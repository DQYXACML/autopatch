package tracing

import (
	"math/big"

	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// ExecutionContext 包含交易执行过程中不变的上下文信息
// 通过缓存这些信息，避免在多次变异执行中重复获取
type ExecutionContext struct {
	// 交易相关信息
	Transaction *types.Transaction
	TxHash      gethCommon.Hash
	From        gethCommon.Address
	
	// 链上信息
	Receipt     *types.Receipt
	Block       *types.Header
	ChainID     *big.Int
	
	// 签名器（基于 chainID 创建）
	Signer      types.Signer
	
	// 预状态信息
	Prestate    PrestateResult
	AllContractsStorage map[gethCommon.Address]map[gethCommon.Hash]gethCommon.Hash
}

// NewExecutionContext 创建新的执行上下文
func NewExecutionContext(
	tx *types.Transaction,
	receipt *types.Receipt,
	block *types.Header,
	chainID *big.Int,
	prestate PrestateResult,
	allContractsStorage map[gethCommon.Address]map[gethCommon.Hash]gethCommon.Hash,
) (*ExecutionContext, error) {
	signer := types.LatestSignerForChainID(chainID)
	from, err := types.Sender(signer, tx)
	if err != nil {
		return nil, err
	}
	
	return &ExecutionContext{
		Transaction: tx,
		TxHash:      tx.Hash(),
		From:        from,
		Receipt:     receipt,
		Block:       block,
		ChainID:     chainID,
		Signer:      signer,
		Prestate:    prestate,
		AllContractsStorage: allContractsStorage,
	}, nil
}