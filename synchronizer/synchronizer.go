package synchronizer

import (
	"errors"
	"github.com/DQYXACML/vrf-node/common/tasks"
	"github.com/DQYXACML/vrf-node/config"
	"github.com/DQYXACML/vrf-node/synchronizer/node"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"math/big"
	"time"
)

type Synchronizer struct {
	ethClient node.EthClient
	chainCfg  *config.ChainConfig
	tasks     tasks.Group

	headers         []types.Header
	headerTraversal *node.HeaderTraversal
}

func NewSynchronizer(cfg *config.Config, client node.EthClient) (*Synchronizer, error) {
	var fromHeader *types.Header
	if cfg.Chain.StartingHeight > 0 {
		header, err := client.BlockHeaderByNumber(big.NewInt(int64(cfg.Chain.StartingHeight)))
		if err != nil {
			log.Error("get block from chain fail", "err", err)
			return nil, err
		}
		fromHeader = header
	}

	headerTraversal := node.NewHeaderTraversal(client, fromHeader, big.NewInt(0), cfg.Chain.ChainId)
	return &Synchronizer{
		ethClient:       client,
		chainCfg:        &cfg.Chain,
		headerTraversal: headerTraversal,
		tasks:           tasks.Group{},
	}, nil
}

func (syncer *Synchronizer) Start() error {
	log.Info("Starting synchronizer")
	tickerSyncer := time.NewTicker(time.Second * 1)
	syncer.tasks.Go(func() error {
		for range tickerSyncer.C {
			newHeaders, err := syncer.headerTraversal.NextHeaders(syncer.chainCfg.BlockStep)
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
	firstHeader, lastHead := headers[0], headers[len(headers)-1]
	log.Info("sync batch", "size", len(headers), "startBlock", firstHeader.Number, "endBlock", lastHead.Number)

	headerMap := make(map[common.Hash]*types.Header, len(headers))
	for i := range headers {
		header := headers[i]
		headerMap[header.Hash()] = &header
	}
	var addressList []common.Address
	addressList = append(addressList, common.HexToAddress("0x2bf417A46a595Facd902111c13008Cb3ECD536b7"))
	addressList = append(addressList, common.HexToAddress("0x21EA59025C4a16E948224D100D97c3a24706C728"))

	filterQuery := ethereum.FilterQuery{
		FromBlock: firstHeader.Number,
		ToBlock:   lastHead.Number,
		Addresses: addressList,
	}
	logs, err := syncer.ethClient.FilterLogs(filterQuery)
	if err != nil {
		log.Error("filter logs fail", "err", err)
		return err
	}

	if logs.ToBlockHeader.Number.Cmp(lastHead.Number) != 0 {
		return errors.New("mismatch in filter#toBlock numer")
	} else if logs.ToBlockHeader.Hash() != lastHead.Hash() {
		return errors.New("mismatch in filter#toBlock hash")
	}

	if len(logs.Logs) > 0 {
		log.Info("detected logs", "size", len(logs.Logs))
	}

	for i := range logs.Logs {
		logEvent := logs.Logs[i]
		if _, ok := headerMap[logEvent.BlockHash]; !ok {
			continue
		}
		timestamp := headerMap[logEvent.BlockHash].Time
		log.Info("event logs", "address", logs.Logs[i].Address, "timestamp", timestamp)
	}
	return nil
}
