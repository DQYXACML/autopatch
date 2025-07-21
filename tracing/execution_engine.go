package tracing

import (
	"context"
	"fmt"

	"github.com/DQYXACML/autopatch/synchronizer/node"
	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/holiman/uint256"
)

// ExecutionEngine 执行引擎，负责交易执行和路径分析
type ExecutionEngine struct {
	client       *ethclient.Client
	nodeClient   node.EthClient
	stateManager *StateManager
	jumpTracer   *JumpTracer
}

// NewExecutionEngine 创建执行引擎
func NewExecutionEngine(client *ethclient.Client, nodeClient node.EthClient, stateManager *StateManager, jumpTracer *JumpTracer) *ExecutionEngine {
	return &ExecutionEngine{
		client:       client,
		nodeClient:   nodeClient,
		stateManager: stateManager,
		jumpTracer:   jumpTracer,
	}
}

// ExecuteTransactionWithTracing 执行交易并进行跟踪
func (e *ExecutionEngine) ExecuteTransactionWithTracing(tx *types.Transaction, prestate PrestateResult, modifiedInput []byte, storageMods map[gethCommon.Hash]gethCommon.Hash) (*ExecutionPath, error) {
	stateDB, err := e.stateManager.CreateStateFromPrestate(prestate)
	if err != nil {
		return nil, fmt.Errorf("failed to create state: %v", err)
	}

	if storageMods != nil && tx.To() != nil {
		for slot, value := range storageMods {
			stateDB.SetState(*tx.To(), slot, value)
		}
		fmt.Printf("Applied %d storage modifications\n", len(storageMods))
	}

	receipt, err := e.nodeClient.TxReceiptByHash(tx.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get receipt: %v", err)
	}

	block, err := e.nodeClient.BlockHeaderByNumber(receipt.BlockNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get block: %v", err)
	}

	chainID, err := e.client.NetworkID(context.Background())
	if err != nil {
		return nil, err
	}

	evm, err := e.stateManager.CreateEVMWithTracer(stateDB, block, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to create EVM: %v", err)
	}

	signer := types.LatestSignerForChainID(chainID)
	from, err := types.Sender(signer, tx)
	if err != nil {
		return nil, err
	}

	txCtx := vm.TxContext{
		Origin:   from,
		GasPrice: tx.GasPrice(),
	}
	evm.SetTxContext(txCtx)

	inputData := tx.Data()
	if modifiedInput != nil {
		inputData = modifiedInput
	}

	e.jumpTracer.StartTrace()

	if tx.To() == nil {
		_, _, _, err = evm.Create(
			from,
			inputData,
			tx.Gas(),
			uint256.MustFromBig(tx.Value()),
		)
	} else {
		_, _, err = evm.Call(
			from,
			*tx.To(),
			inputData,
			tx.Gas(),
			uint256.MustFromBig(tx.Value()),
		)
	}

	path := e.jumpTracer.StopTrace()

	if err != nil {
		fmt.Printf("Transaction execution failed: %v\n", err)
	}

	return path, nil
}

// ExecuteMutationBatch 并行执行变异批次

// CalculatePathSimilarity 计算路径相似度
func (e *ExecutionEngine) CalculatePathSimilarity(path1, path2 *ExecutionPath) float64 {
	if path1 == nil || path2 == nil {
		return 0.0
	}

	if len(path1.Jumps) == 0 && len(path2.Jumps) == 0 {
		return 1.0
	}

	if len(path1.Jumps) == 0 || len(path2.Jumps) == 0 {
		return 0.0
	}

	matches := 0
	minLen := len(path1.Jumps)
	if len(path2.Jumps) < minLen {
		minLen = len(path2.Jumps)
	}

	for i := 0; i < minLen; i++ {
		if path1.Jumps[i].ContractAddress == path2.Jumps[i].ContractAddress &&
			path1.Jumps[i].JumpFrom == path2.Jumps[i].JumpFrom &&
			path1.Jumps[i].JumpDest == path2.Jumps[i].JumpDest {
			matches++
		}
	}

	maxLen := len(path1.Jumps)
	if len(path2.Jumps) > maxLen {
		maxLen = len(path2.Jumps)
	}

	return float64(matches) / float64(maxLen)
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
