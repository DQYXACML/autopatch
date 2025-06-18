package auto_patch

import (
	"context"
	"github.com/DQYXACML/autopatch/config"
	"github.com/DQYXACML/autopatch/database"
	"github.com/DQYXACML/autopatch/storage"
	"github.com/DQYXACML/autopatch/synchronizer"
	"github.com/DQYXACML/autopatch/synchronizer/node"
	"github.com/DQYXACML/autopatch/tracing"
	common2 "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"math/big"
	"sync/atomic"
)

const BlockSize = 3000

type AutoPatch struct {
	db            *database.DB
	synchronizer  *synchronizer.Synchronizer
	storageparser *storage.StorageParser
	stopped       atomic.Bool
}

func NewAutoPatch(ctx context.Context, cfg *config.Config) (*AutoPatch, error) {
	ethClient, err := node.DialEthClient(ctx, cfg.Chain.ChainRpcUrl)
	if err != nil {
		log.Error("new eth client fail", "err", err)
		return nil, err
	}
	db, err := database.NewDB(ctx, cfg.MasterDB)
	if err != nil {
		log.Error("new database fail", "err", err)
		return nil, err
	}

	mytracer := tracing.NewTracer(ethClient)
	err = mytracer.TraceTransaction(common2.HexToHash("0x9e63085271890a141297039b3b711913699f1ee4db1acb667ad7ce304772036b"))
	if err != nil {
		log.Error("trace transaction fail", "err", err)
		return nil, err
	}

	newSynchronizer, err := synchronizer.NewSynchronizer(cfg, db, ethClient)
	if err != nil {
		return nil, err
	}

	spConfig := &storage.StorageParserConfig{
		EventLoopInterval: cfg.Chain.EventInterval,
		StartHeight:       big.NewInt(int64(cfg.Chain.StartingHeight)),
		BlockSize:         BlockSize,
	}

	storageParser, err := storage.NewStorageParser(db, ethClient, spConfig)
	if err != nil {
		return nil, err
	}

	autoPatch := &AutoPatch{
		synchronizer:  newSynchronizer,
		storageparser: storageParser,
		db:            db,
	}
	return autoPatch, nil
}

func (ap *AutoPatch) Start(ctx context.Context) error {
	err := ap.synchronizer.Start()
	if err != nil {
		return err
	}
	err = ap.storageparser.Start()
	if err != nil {
		return err
	}
	return nil
}

func (ap *AutoPatch) Stop(ctx context.Context) error {
	err := ap.synchronizer.Close()
	if err != nil {
		return err
	}
	err = ap.storageparser.Close()
	if err != nil {
		return err
	}
	return nil
}

func (ap *AutoPatch) Stopped() bool {
	return ap.stopped.Load()
}
