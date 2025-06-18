package storage

import (
	"github.com/DQYXACML/autopatch/common/tasks"
	"github.com/DQYXACML/autopatch/database"
	"github.com/DQYXACML/autopatch/database/common"
	"github.com/DQYXACML/autopatch/synchronizer/node"
	common2 "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"gorm.io/gorm"
	"math/big"
	"time"
)

type StorageParserConfig struct {
	DappLinkVrfAddress        string
	DappLinkVrfFactoryAddress string
	EventLoopInterval         time.Duration
	StartHeight               *big.Int
	BlockSize                 uint64
}

type StorageParser struct {
	db                *database.DB
	ethClient         node.EthClient
	spConf            *StorageParserConfig
	latestBlockHeader *common.BlockHeader
	tasks             tasks.Group
}

func NewStorageParser(db *database.DB, client node.EthClient, spConf *StorageParserConfig) (*StorageParser, error) {
	return &StorageParser{
		db:        db,
		spConf:    spConf,
		ethClient: client,
		tasks: tasks.Group{HandleCrit: func(err error) {
			log.Error("critical error in storage parser:" + err.Error())
		}},
	}, nil
}

func (sp *StorageParser) Start() error {
	tickerSyncer := time.NewTicker(sp.spConf.EventLoopInterval)
	sp.tasks.Go(func() error {
		for range tickerSyncer.C {
			log.Info("start parse storage logs")
			err := sp.ProcessStorage()
			if err != nil {
				log.Info("process storage error", "err", err)
				return err
			}
		}
		return nil
	})
	return nil
}

func (sp *StorageParser) ProcessStorage() error {
	lastBlockNumber := sp.spConf.StartHeight
	if sp.latestBlockHeader != nil {
		lastBlockNumber = sp.latestBlockHeader.Number
	}
	log.Info("process storage latest block number", "lastBlockNumber", lastBlockNumber)
	latestHeaderScope := func(db *gorm.DB) *gorm.DB {
		newQuery := db.Session(&gorm.Session{NewDB: true})
		headers := newQuery.Model(common.BlockHeader{}).Where("number > ?", lastBlockNumber)
		return db.Where("number = (?)", newQuery.Table("(?) as block_numbers", headers.Order("number ASC").Limit(int(sp.spConf.BlockSize))).Select("MAX(number)"))
	}
	if latestHeaderScope == nil {
		return nil
	}
	latestBlockHeader, err := sp.db.Blocks.BlockHeaderWithScope(latestHeaderScope)
	if err != nil {
		log.Error("get latest block header with scope fail", "err", err)
		return err
	} else if latestBlockHeader == nil {
		log.Debug("no new block for process event")
		return nil
	}

	// 从db里读取配置的被保护合约地址
	contractAddresses, err := sp.db.Protected.QueryProtectedAddAddressList()
	if err != nil {
		log.Error("query protected contract addresses fail", "err", err)
		return err
	}
	log.Info("Get Protected Contract Addresses", "addresses", contractAddresses)
	// 根据地址查询
	storages, err := sp.db.ProtectedStorage.QueryProtectedStorage(common2.HexToAddress("0xCcdaC991C3AB71dA4bB2510E79eA4B90e41128CB"))
	if err != nil {
		return err
	}
	//log.Info("Get Protected Storage", "storages", storages)
	// 遍历storages
	for _, storage := range storages {
		// 打印每个storage的key和value
		log.Info("Protected Storage Key-Value", "key", storage.StorageKey, "value", storage.StorageValue)
	}
	// 读取db里的不变量，结合storage和参数，判断不变量是否被打破
	protectedStorageWithHeaders, err := sp.db.ProtectedStorage.QueryProtectedStorageWithHeader(common2.HexToAddress("0xCcdaC991C3AB71dA4bB2510E79eA4B90e41128CB"), latestBlockHeader.Number)
	if err != nil {
		return err
	}
	for _, storage := range protectedStorageWithHeaders {

		log.Info("protected Storage With Headers Key-Value", "key", storage.StorageKey, "value", storage.StorageValue)
	}
	// 如果被打破，进行相应的处理
	// 获取打破前后的区块高度.
	beforeAttackHeaderNumber, attackHeaderNumber := new(big.Int).Sub(latestBlockHeader.Number, big.NewInt(1)), latestBlockHeader.Number
	log.Info("Attack Header Number", "beforeAttackHeaderNumber", beforeAttackHeaderNumber, "afterAttackHeaderNumber", attackHeaderNumber)
	// 根据区块高度、保护合约的地址，查询获取打破前后的交易
	attackTxs, err := sp.db.ProtectedTx.QueryProtectedTxWithHeaderAndAddress(common2.HexToAddress("0xCcdaC991C3AB71dA4bB2510E79eA4B90e41128CB"), attackHeaderNumber)
	if err != nil {
		log.Error("query protected tx with header and address fail", "err", err)
		return err
	}
	if len(attackTxs) == 1 {
		attacktx := attackTxs[0] // 获取攻击交易
		// 从交易中获取一些特征：
		log.Info("attack tx", "attackTx", attacktx)
	} else {
		for _, tx := range attackTxs {
			log.Info("Attack Tx", "tx", tx)
		}
	}

	return nil
}

func (sp *StorageParser) Close() error {
	return sp.tasks.Wait()
}
