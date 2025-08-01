package core

import (
	"context"
	"fmt"
	"math/big"

	"github.com/DQYXACML/autopatch/database/utils"
	"github.com/DQYXACML/autopatch/synchronizer/node"
	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/holiman/uint256"
	tracingUtils "github.com/DQYXACML/autopatch/tracing/utils"
)

// StateManagerInterface 状态管理器接口
type StateManagerInterface interface {
	CreateStateFromPrestate(prestate tracingUtils.PrestateResult) (*state.StateDB, error)
	CreateEVMWithTracer(stateDB *state.StateDB, block *types.Header, chainID *big.Int) (*vm.EVM, error)
	CreateInterceptingEVM(stateDB *state.StateDB, block *types.Header, chainID *big.Int, targetCalls map[gethCommon.Address][]byte) (*tracingUtils.InterceptingEVM, error)
}

// ExecutionEngine 执行引擎，负责交易执行和路径分析
type ExecutionEngine struct {
	client       *ethclient.Client
	nodeClient   node.EthClient
	stateManager StateManagerInterface
	jumpTracer   *tracingUtils.JumpTracer
}

// NewExecutionEngine 创建执行引擎
func NewExecutionEngine(client *ethclient.Client, nodeClient node.EthClient, stateManager StateManagerInterface, jumpTracer *tracingUtils.JumpTracer) *ExecutionEngine {
	return &ExecutionEngine{
		client:       client,
		nodeClient:   nodeClient,
		stateManager: stateManager,
		jumpTracer:   jumpTracer,
	}
}

// ExecuteTransactionWithContext 使用预获取的上下文执行交易并进行跟踪
func (e *ExecutionEngine) ExecuteTransactionWithContext(ctx *tracingUtils.ExecutionContext, modifiedInput []byte, storageMods map[gethCommon.Hash]gethCommon.Hash) (*tracingUtils.ExecutionPath, error) {
	stateDB, err := e.stateManager.CreateStateFromPrestate(ctx.Prestate)
	if err != nil {
		return nil, fmt.Errorf("failed to create state: %v", err)
	}

	if storageMods != nil && ctx.Transaction.To() != nil {
		for slot, value := range storageMods {
			stateDB.SetState(*ctx.Transaction.To(), slot, value)
		}
		fmt.Printf("Applied %d storage modifications\n", len(storageMods))
	}

	evm, err := e.stateManager.CreateEVMWithTracer(stateDB, ctx.Block, ctx.ChainID)
	if err != nil {
		return nil, fmt.Errorf("failed to create EVM: %v", err)
	}

	txCtx := vm.TxContext{
		Origin:   ctx.From,
		GasPrice: ctx.Transaction.GasPrice(),
	}
	evm.SetTxContext(txCtx)

	inputData := ctx.Transaction.Data()
	if modifiedInput != nil {
		inputData = modifiedInput
	}

	e.jumpTracer.StartTrace()

	if ctx.Transaction.To() == nil {
		_, _, _, err = evm.Create(
			ctx.From,
			inputData,
			ctx.Transaction.Gas(),
			uint256.MustFromBig(ctx.Transaction.Value()),
		)
	} else {
		_, _, err = evm.Call(
			ctx.From,
			*ctx.Transaction.To(),
			inputData,
			ctx.Transaction.Gas(),
			uint256.MustFromBig(ctx.Transaction.Value()),
		)
	}

	path := e.jumpTracer.StopTrace()

	if err != nil {
		fmt.Printf("Transaction execution failed: %v\n", err)
	}

	return path, nil
}

// ExecuteTransactionWithTracing 执行交易并进行跟踪（保留原方法以兼容）
func (e *ExecutionEngine) ExecuteTransactionWithTracing(tx *types.Transaction, prestate tracingUtils.PrestateResult, modifiedInput []byte, storageMods map[gethCommon.Hash]gethCommon.Hash) (*tracingUtils.ExecutionPath, error) {
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

// ExecuteWithInterceptedCalls executes a transaction with intercepted and modified calls to specific contracts
func (e *ExecutionEngine) ExecuteWithInterceptedCalls(
	ctx *tracingUtils.ExecutionContext,
	targetCalls map[gethCommon.Address][]byte,
) (*tracingUtils.ExecutionPath, error) {
	// Create state from prestate
	stateDB, err := e.stateManager.CreateStateFromPrestate(ctx.Prestate)
	if err != nil {
		return nil, fmt.Errorf("failed to create state: %v", err)
	}
	
	// Apply any storage modifications from all contracts
	if ctx.AllContractsStorage != nil {
		for contractAddr, storage := range ctx.AllContractsStorage {
			for slot, value := range storage {
				stateDB.SetState(contractAddr, slot, value)
			}
		}
	}
	
	// Set target contract for the jump tracer
	if len(targetCalls) > 0 {
		// Use the first target contract as the primary one for tracing
		for addr := range targetCalls {
			e.jumpTracer.SetTargetContract(addr)
			break
		}
	}
	
	// Create intercepting EVM
	interceptingEVM, err := e.stateManager.CreateInterceptingEVM(
		stateDB, 
		ctx.Block, 
		ctx.ChainID,
		targetCalls,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create intercepting EVM: %v", err)
	}
	
	// Set transaction context
	txCtx := vm.TxContext{
		Origin:   ctx.From,
		GasPrice: ctx.Transaction.GasPrice(),
	}
	interceptingEVM.SetTxContext(txCtx)
	
	// Start tracing
	e.jumpTracer.StartTrace()
	
	// Execute transaction with original input data
	// The InterceptingEVM will replace input data for target contracts
	inputData := ctx.Transaction.Data()
	
	fmt.Printf("Executing transaction with intercepted calls\n")
	fmt.Printf("  Original input data length: %d\n", len(inputData))
	fmt.Printf("  Target contracts: %d\n", len(targetCalls))
	
	if ctx.Transaction.To() == nil {
		// Contract creation
		_, _, _, err = interceptingEVM.Create(
			ctx.From,
			inputData,
			ctx.Transaction.Gas(),
			uint256.MustFromBig(ctx.Transaction.Value()),
		)
	} else {
		// Regular call
		_, _, err = interceptingEVM.Call(
			ctx.From,
			*ctx.Transaction.To(),
			inputData,
			ctx.Transaction.Gas(),
			uint256.MustFromBig(ctx.Transaction.Value()),
		)
	}
	
	// Stop tracing and get the path
	path := e.jumpTracer.StopTrace()
	
	if err != nil {
		fmt.Printf("Transaction execution with interception failed: %v\n", err)
	} else {
		fmt.Printf("Transaction execution with interception succeeded\n")
		fmt.Printf("  Recorded jumps: %d\n", len(path.Jumps))
	}
	
	return path, nil
}

// ExecuteMutationBatch 并行执行变异批次

// CalculatePathSimilarity 计算路径相似度
func (e *ExecutionEngine) CalculatePathSimilarity(path1, path2 *tracingUtils.ExecutionPath) float64 {
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

// ExecuteTransaction 执行交易并返回执行路径（简化版本）
func (e *ExecutionEngine) ExecuteTransaction(
	tx *types.Transaction,
	prestate map[gethCommon.Address]*utils.ContractState,
) ([]string, error) {
	// 这是一个简化的实现，实际应该使用完整的EVM执行
	// 这里只是为了满足接口需求
	
	if tx.To() == nil {
		return []string{"CREATE"}, nil
	}
	
	// 基于交易类型和目标地址生成简单的执行路径
	path := []string{"CALL"}
	
	// 检查是否有存储访问
	if contractState, exists := prestate[*tx.To()]; exists && len(contractState.Storage) > 0 {
		path = append(path, "SLOAD")
		if len(tx.Data()) > 0 {
			path = append(path, "SSTORE")
		}
	}
	
	// 根据输入数据判断可能的操作
	if len(tx.Data()) >= 4 {
		selector := gethCommon.Bytes2Hex(tx.Data()[:4])
		switch selector {
		case "a9059cbb": // transfer(address,uint256)
			path = append(path, "TRANSFER", "RETURN")
		case "095ea7b3": // approve(address,uint256)
			path = append(path, "APPROVE", "RETURN")
		case "23b872dd": // transferFrom(address,address,uint256)
			path = append(path, "TRANSFERFROM", "RETURN")
		default:
			path = append(path, "RETURN")
		}
	} else {
		path = append(path, "RETURN")
	}
	
	return path, nil
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
