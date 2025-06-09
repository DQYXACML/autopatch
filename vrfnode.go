package vrf_node

import (
	"context"
	"github.com/DQYXACML/vrf-node/config"
	"github.com/DQYXACML/vrf-node/synchronizer"
	"github.com/DQYXACML/vrf-node/synchronizer/node"
	"github.com/ethereum/go-ethereum/log"
	"sync/atomic"
)

type VrfNode struct {
	synchronizer *synchronizer.Synchronizer
	stopped      atomic.Bool
}

func NewVrfNode(ctx context.Context, cfg *config.Config) (*VrfNode, error) {
	ethClient, err := node.DialEthClient(ctx, cfg.Chain.ChainRpcUrl)
	if err != nil {
		log.Error("new eth client fail", "err", err)
		return nil, err
	}
	newSynchronizer, err := synchronizer.NewSynchronizer(cfg, ethClient)
	if err != nil {
		return nil, err
	}
	vrfNode := &VrfNode{
		synchronizer: newSynchronizer,
	}
	return vrfNode, nil
}

func (vn *VrfNode) Start(ctx context.Context) error {
	vn.synchronizer.Start()
	return nil
}

func (vn *VrfNode) Stop(ctx context.Context) error {
	vn.synchronizer.Close()
	return nil
}

func (vn *VrfNode) Stopped() bool {
	return vn.stopped.Load()
}
