package tracing

import (
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"math/big"
)

// JumpTracer handles jump instruction tracing for execution path recording
type JumpTracer struct {
	contractABIs  map[common.Address]*abi.ABI
	executionPath *ExecutionPath
	isTraceActive bool
}

// NewJumpTracer creates a new jump tracer
func NewJumpTracer() *JumpTracer {
	return &JumpTracer{
		contractABIs:  make(map[common.Address]*abi.ABI),
		executionPath: &ExecutionPath{Jumps: make([]ExecutionJump, 0)},
		isTraceActive: false,
	}
}

// AddContractABI adds ABI information for a contract
func (t *JumpTracer) AddContractABI(address common.Address, contractABI *abi.ABI) {
	t.contractABIs[address] = contractABI
}

// StartTrace starts recording execution path
func (t *JumpTracer) StartTrace() {
	t.isTraceActive = true
	t.executionPath = &ExecutionPath{Jumps: make([]ExecutionJump, 0)}
}

// StopTrace stops recording and returns the execution path
func (t *JumpTracer) StopTrace() *ExecutionPath {
	t.isTraceActive = false
	return t.executionPath
}

// GetExecutionPath returns the current execution path
func (t *JumpTracer) GetExecutionPath() *ExecutionPath {
	return t.executionPath
}

// safeStackBack safely gets stack element (from the back)
func (t *JumpTracer) safeStackBack(stackData []uint256.Int, n int) *uint256.Int {
	stackLen := len(stackData)
	if stackLen == 0 || n >= stackLen {
		return uint256.NewInt(0)
	}
	return &stackData[stackLen-1-n]
}

// onTxStart handles transaction start
func (t *JumpTracer) onTxStart(vm *tracing.VMContext, tx *types.Transaction, from common.Address) {
	if t.isTraceActive {
		fmt.Printf("=== JUMP TRACER: Transaction started ===\n")
	}
}

// onTxEnd handles transaction end
func (t *JumpTracer) onTxEnd(receipt *types.Receipt, err error) {
	if t.isTraceActive {
		fmt.Printf("=== JUMP TRACER: Transaction ended, jumps recorded: %d ===\n", len(t.executionPath.Jumps))
	}
}

// onEnter handles call start
func (t *JumpTracer) onEnter(depth int, typ byte, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	// Not used for jump tracing
}

// onExit handles call end
func (t *JumpTracer) onExit(depth int, output []byte, gasUsed uint64, err error, reverted bool) {
	// Not used for jump tracing
}

// onOpcode handles opcode tracing - focus on JUMP and JUMPI
func (t *JumpTracer) onOpcode(pc uint64, opcode byte, gas uint64, cost uint64, scope tracing.OpContext, rData []byte, depth int, err error) {
	if !t.isTraceActive {
		return
	}

	op := vm.OpCode(opcode)
	contractAddr := scope.Address()
	stackData := scope.StackData()
	stackLen := len(stackData)

	switch op {
	case vm.JUMP:
		if stackLen > 0 {
			dest := t.safeStackBack(stackData, 0)
			jump := ExecutionJump{
				ContractAddress: contractAddr,
				JumpFrom:        pc,
				JumpDest:        dest.Uint64(),
			}
			t.executionPath.Jumps = append(t.executionPath.Jumps, jump)
			fmt.Printf("JUMP TRACER: JUMP %s: %d -> %d\n", contractAddr.Hex(), pc, dest.Uint64())
		}

	case vm.JUMPI:
		if stackLen >= 2 {
			dest := t.safeStackBack(stackData, 0)
			condition := t.safeStackBack(stackData, 1)

			// 只有当条件为真时才记录跳转
			if condition.Sign() != 0 {
				jump := ExecutionJump{
					ContractAddress: contractAddr,
					JumpFrom:        pc,
					JumpDest:        dest.Uint64(),
				}
				t.executionPath.Jumps = append(t.executionPath.Jumps, jump)
				fmt.Printf("JUMP TRACER: JUMPI %s: %d -> %d (condition: %s)\n",
					contractAddr.Hex(), pc, dest.Uint64(), condition.Hex())
			}
		}
	}
}

// onFault handles execution faults
func (t *JumpTracer) onFault(pc uint64, opcode byte, gas uint64, cost uint64, scope tracing.OpContext, depth int, err error) {
	// Not used for jump tracing
}

// Empty implementations for required interface methods
func (t *JumpTracer) onGasChange(old, new uint64, reason tracing.GasChangeReason) {}
func (t *JumpTracer) onBalanceChange(a common.Address, prev, new *big.Int, reason tracing.BalanceChangeReason) {
}
func (t *JumpTracer) onNonceChange(a common.Address, prev, new uint64)           {}
func (t *JumpTracer) onStorageChange(a common.Address, k, prev, new common.Hash) {}
func (t *JumpTracer) onCodeChange(a common.Address, prevCodeHash common.Hash, prev []byte, codeHash common.Hash, code []byte) {
}
func (t *JumpTracer) onLog(log *types.Log) {}
func (t *JumpTracer) onSystemCallStart()   {}
func (t *JumpTracer) onSystemCallEnd()     {}

// ToTracingHooks converts JumpTracer to tracing.Hooks struct
func (t *JumpTracer) ToTracingHooks() *tracing.Hooks {
	return &tracing.Hooks{
		OnTxStart:         t.onTxStart,
		OnTxEnd:           t.onTxEnd,
		OnEnter:           t.onEnter,
		OnExit:            t.onExit,
		OnOpcode:          t.onOpcode,
		OnFault:           t.onFault,
		OnGasChange:       t.onGasChange,
		OnBalanceChange:   t.onBalanceChange,
		OnNonceChange:     t.onNonceChange,
		OnCodeChange:      t.onCodeChange,
		OnStorageChange:   t.onStorageChange,
		OnLog:             t.onLog,
		OnSystemCallStart: t.onSystemCallStart,
		OnSystemCallEnd:   t.onSystemCallEnd,
	}
}
