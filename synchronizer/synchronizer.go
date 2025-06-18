package synchronizer

import (
	"context"
	"fmt"
	"github.com/DQYXACML/autopatch/common/tasks"
	"github.com/DQYXACML/autopatch/config"
	"github.com/DQYXACML/autopatch/database"
	common2 "github.com/DQYXACML/autopatch/database/common"
	"github.com/DQYXACML/autopatch/database/utils"
	"github.com/DQYXACML/autopatch/database/worker"
	sutils "github.com/DQYXACML/autopatch/storage/utils"
	"github.com/DQYXACML/autopatch/synchronizer/node"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"math/big"
	"os"
	"time"
)

var (
	storageLayoutJson = ``
)

type Synchronizer struct {
	ethClient node.EthClient
	db        *database.DB
	chainCfg  *config.ChainConfig
	tasks     tasks.Group

	headers         []types.Header
	latestHeader    *types.Header
	headerTraversal *node.HeaderTraversal
}

func NewSynchronizer(cfg *config.Config, db *database.DB, client node.EthClient) (*Synchronizer, error) {
	latestHeader, err := db.Blocks.LatestBlockHeader()
	if err != nil {
		log.Error("query latest block header fail", "err", err)
		return nil, err
	}

	var fromHeader *types.Header
	if latestHeader != nil {
		fromHeader = latestHeader.RLPHeader.Header()
		log.Info("Header from DB", "header", fromHeader.Number)
	} else if cfg.Chain.StartingHeight > 0 {
		header, err := client.BlockHeaderByNumber(big.NewInt(int64(cfg.Chain.StartingHeight)))
		if err != nil {
			log.Error("get block from chain fail", "err", err)
			return nil, err
		}
		fromHeader = header
		log.Info("Header from env", "header", header.Number)
	} else {
		log.Info("no eth block indexed state")
	}

	headerTraversal := node.NewHeaderTraversal(client, fromHeader, big.NewInt(0), cfg.Chain.ChainId)
	return &Synchronizer{
		ethClient:       client,
		db:              db,
		chainCfg:        &cfg.Chain,
		headerTraversal: headerTraversal,
		latestHeader:    fromHeader,
		tasks:           tasks.Group{},
	}, nil
}

func (syncer *Synchronizer) Start() error {
	log.Info("Starting synchronizer")
	tickerSyncer := time.NewTicker(time.Second * 10)
	syncer.tasks.Go(func() error {
		for range tickerSyncer.C {
			newHeaders, err := syncer.headerTraversal.NextHeaders(syncer.chainCfg.BlockStep)
			log.Info("NewHeaders", "newHeaders", len(newHeaders))
			if err != nil {
				log.Error("error querying for header", "err", err)
				continue
			} else if len(newHeaders) == 0 {
				log.Warn("no new header, sync at head")
			} else {
				syncer.headers = newHeaders
			}
			latestHeader := syncer.headerTraversal.LatestHeader()
			if latestHeader != nil {
				log.Info("Latest header", "latestHeader", latestHeader.Number)
			}
			err = syncer.processBatch(syncer.headers)
			if err == nil {
				syncer.headers = nil
			}
		}
		return nil
	})
	return nil
}

func (syncer *Synchronizer) Close() error {
	log.Info("Closing synchronizer")
	return nil
}

func (syncer *Synchronizer) processBatch(headers []types.Header) error {
	if len(headers) == 0 {
		return nil
	}
	firstHeader, lastHeader := headers[0], headers[len(headers)-1]
	log.Info("sync batch", "size", len(headers), "startBlock", firstHeader.Number, "endBlock", lastHeader.Number)

	headerMap := make(map[common.Hash]*types.Header, len(headers))
	for i := range headers {
		header := headers[i]
		headerMap[header.Hash()] = &header
	}
	var addressList []common.Address

	addressList, err := syncer.db.Protected.QueryProtectedAddAddressList()
	if err != nil {
		log.Error("QueryProtectedAddAddressList fail", "err", err)
		return err
	}

	log.Info("Protected address list", "addresses", addressList)

	// 并发准备
	const maxWorkers = 16
	g, _ := errgroup.WithContext(context.Background())
	sem := make(chan struct{}, maxWorkers)
	// 存储header结构
	blockHeaders := make([]common2.BlockHeader, 0, len(headers))
	for i := range headers {
		if headers[i].Number == nil {
			continue
		}
		bHeader := common2.BlockHeader{
			Hash:       headers[i].Hash(),
			ParentHash: headers[i].ParentHash,
			Number:     headers[i].Number,
			Timestamp:  headers[i].Time,
			RLPHeader:  (*utils.RLPHeader)(&headers[i]),
		}
		blockHeaders = append(blockHeaders, bHeader)
		h := headers[i]
		sem <- struct{}{}
		// --- Storage ---
		g.Go(func() error {
			defer func() { <-sem }()
			return syncer.parseStorage(&h)
		})
		// --- Tx ---
		sem <- struct{}{}
		g.Go(func() error {
			defer func() { <-sem }()
			return syncer.parseTx(addressList[0], &h)
		})
		//// 写StorageState入库
		//log.Info("write StorageState to db", "header", headers[i].Number)
		//if err := syncer.parseStorage(&headers[i]); err != nil {
		//	log.Error("parseStorage fail", "err", err)
		//	return err
		//}
		//// 写tx 入库
		//log.Info("write tx to db", "txs", len(addressList), "blockNumber", headers[i].Number)
		//if err := syncer.parseTx(addressList[0], &headers[i]); err != nil {
		//	log.Error("parseTx fail", "err", err)
		//	return err
		//}
		return g.Wait()
	}

	// 写blockHeader入库
	log.Info("Process Batch Write blockHeaders in DB")
	if err := syncer.db.Transaction(func(tx *database.DB) error {
		if err := tx.Blocks.StoreBlockHeaders(blockHeaders); err != nil {
			return err
		}
		return nil
	}); err != nil {
		log.Error("StoreBlockHeaders fail", "err", err)
		return err
	}

	return nil
}

func (syncer *Synchronizer) parseTx(addr common.Address, header *types.Header) error {
	txs, err := syncer.ethClient.TransactionsToAtBlock(addr, header.Number)
	log.Info("parseTx log", "txs", len(txs))
	if err != nil {
		return err
	}
	protectedTxs := make([]worker.ProtectedTx, 0, len(txs))
	for _, tx := range txs {
		protectedTx := worker.ProtectedTx{
			GUID:             uuid.New(),
			BlockHash:        header.Hash(),
			BlockNumber:      header.Number,
			Hash:             tx.Hash(),
			ProtectedAddress: addr,
			InputData:        tx.Data(),
		}
		protectedTxs = append(protectedTxs, protectedTx)
	}
	err = syncer.db.ProtectedTx.StoreProtectedTx(protectedTxs, uint64(len(protectedTxs)))
	if err != nil {
		log.Error("StoreProtectedTx fail", "err", err)
		return err
	}
	return nil
}

func (syncer *Synchronizer) parseStorage(header *types.Header) error {
	// 写StorageState入库
	c := sutils.NewContract(common.HexToAddress("0xCcdaC991C3AB71dA4bB2510E79eA4B90e41128CB"), syncer.chainCfg.ChainRpcUrl)
	fileContent, err := os.ReadFile("./synchronizer/StorageScan.json")
	if err != nil {
		log.Error("Read Json file failure:", err)
	}
	storageLayoutJson = string(fileContent)
	err = c.ParseByStorageLayout(storageLayoutJson)
	if err != nil {
		log.Error("Parse Storage Layout Error: ", err)
	}
	storages, err := fetchOnChainValue(c, header)
	if err != nil {
		log.Error("Fetch Storages error: ", err)
		return err
	}
	// 写storages入库
	if err := syncer.db.Transaction(func(tx *database.DB) error {
		if err := tx.ProtectedStorage.StoreProtectedStorage(storages); err != nil {
			return err
		}
		return nil
	}); err != nil {
		log.Error("StoreProtectedStorage fail", "err", err)
		return err
	}
	return nil
}

func fetchOnChainValue(c *sutils.Contract, header *types.Header) ([]worker.ProtectedStorage, error) {
	vars := c.GetAllVariables()
	storages := make([]worker.ProtectedStorage, 0, len(vars))
	for _, v := range vars {
		val := c.GetVariableValue(v.Name)
		storage := worker.ProtectedStorage{
			GUID:             uuid.New(),
			ProtectedAddress: c.Address,
			StorageKey:       v.Name,
			StorageValue:     fmt.Sprintf("%v", val),
			Number:           header.Number,
		}
		storages = append(storages, storage)
	}
	return storages, nil
}
