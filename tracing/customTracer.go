package tracing

import (
	"bytes"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"math/big"
	"reflect"
	"strings"
)

// CustomTracer handles tracing functionality with structured path recording
type CustomTracer struct {
	contractABIs         map[common.Address]*abi.ABI
	currentPath          *StructuredExecutionPath
	allPaths             []*StructuredExecutionPath
	legacyCurrentPath    *ExecutionPath // Keep legacy path for backward compatibility
	legacyAllPaths       []*ExecutionPath
	callStack            []*FunctionCall
	currentCall          *FunctionCall
	pathCounter          int
	storageReads         map[string]map[string]string
	storageWrites        map[string]map[string]string
	storageState         map[common.Address]map[common.Hash]common.Hash // Track storage state for old values
	lastOpCode           vm.OpCode
	lastPC               uint64
	branchPoints         map[uint64]bool
	executionDepth       int
	recordStructuredPath bool // Flag to enable structured path recording
	isTraceInitialized   bool // Flag to track if trace has been initialized
}

// NewCustomTracer creates a new custom tracer
func NewCustomTracer() *CustomTracer {
	return &CustomTracer{
		contractABIs:         make(map[common.Address]*abi.ABI),
		allPaths:             make([]*StructuredExecutionPath, 0),
		legacyAllPaths:       make([]*ExecutionPath, 0),
		callStack:            make([]*FunctionCall, 0),
		pathCounter:          0,
		storageReads:         make(map[string]map[string]string),
		storageWrites:        make(map[string]map[string]string),
		storageState:         make(map[common.Address]map[common.Hash]common.Hash),
		branchPoints:         make(map[uint64]bool),
		recordStructuredPath: true, // Enable structured path by default
		isTraceInitialized:   false,
	}
}

// EnableStructuredPath enables structured path recording
func (t *CustomTracer) EnableStructuredPath(enable bool) {
	t.recordStructuredPath = enable
}

// AddContractABI adds ABI information for a contract
func (t *CustomTracer) AddContractABI(address common.Address, contractABI *abi.ABI) {
	t.contractABIs[address] = contractABI
}

// getStackLen safely gets stack length from stack data slice
func (t *CustomTracer) getStackLen(stackData []uint256.Int) int {
	return len(stackData)
}

// safeStackBack safely gets stack element (from the back)
func (t *CustomTracer) safeStackBack(stackData []uint256.Int, n int) *uint256.Int {
	stackLen := len(stackData)
	if stackLen == 0 || n >= stackLen {
		return uint256.NewInt(0)
	}
	return &stackData[stackLen-1-n]
}

// extractStackValues extracts current stack values as common.Hash slice
func (t *CustomTracer) extractStackValues(stackData []uint256.Int, maxValues int) []common.Hash {
	stackLen := len(stackData)
	if stackLen == 0 {
		return []common.Hash{}
	}

	// Limit the number of values to extract
	count := stackLen
	if maxValues > 0 && count > maxValues {
		count = maxValues
	}

	values := make([]common.Hash, count)
	for i := 0; i < count; i++ {
		values[i] = common.BigToHash(stackData[stackLen-1-i].ToBig())
	}

	return values
}

// extractCallParameters extracts parameters from input data using ABI
func (t *CustomTracer) extractCallParameters(input []byte, contractAddr common.Address) [][]byte {
	if len(input) < 4 {
		return [][]byte{}
	}

	if contractABI, exists := t.contractABIs[contractAddr]; exists {
		selector := input[:4]
		for _, method := range contractABI.Methods {
			if bytes.Equal(method.ID[:4], selector) {
				if len(input) > 4 {
					// Try to unpack the parameters
					values, err := method.Inputs.Unpack(input[4:])
					if err == nil {
						params := make([][]byte, len(values))
						for i, value := range values {
							// Convert each parameter to bytes
							params[i] = t.valueToBytes(value)
						}
						return params
					}
				}
				break
			}
		}
	}

	// If ABI not available or unpacking failed, just return raw parameter data
	if len(input) > 4 {
		return [][]byte{input[4:]}
	}

	return [][]byte{}
}

// valueToBytes converts an interface value to bytes
func (t *CustomTracer) valueToBytes(value interface{}) []byte {
	switch v := value.(type) {
	case common.Address:
		return v.Bytes()
	case common.Hash:
		return v.Bytes()
	case *big.Int:
		return v.Bytes()
	case []byte:
		return v
	case string:
		return []byte(v)
	case bool:
		if v {
			return []byte{1}
		}
		return []byte{0}
	default:
		// Use reflection for other types
		val := reflect.ValueOf(value)
		switch val.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return big.NewInt(val.Int()).Bytes()
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return big.NewInt(int64(val.Uint())).Bytes()
		default:
			return []byte(fmt.Sprintf("%v", value))
		}
	}
}

// addPathNode adds a new node to the current structured path
func (t *CustomTracer) addPathNode(node PathNode) {
	if t.recordStructuredPath && t.currentPath != nil {
		t.currentPath.Nodes = append(t.currentPath.Nodes, node)
	}
}

// initializePaths initializes the execution paths (equivalent to onTxStart)
func (t *CustomTracer) initializePaths() {
	fmt.Printf("=== TRACER: Execution started ===\n")

	// Initialize legacy path
	t.legacyCurrentPath = &ExecutionPath{
		PathID:        fmt.Sprintf("path_%d", t.pathCounter),
		CallStack:     make([]*FunctionCall, 0),
		Constraints:   make([]PathConstraint, 0),
		StorageReads:  make(map[string]map[string]string),
		StorageWrites: make(map[string]map[string]string),
		Success:       true,
	}

	// Initialize structured path
	if t.recordStructuredPath {
		t.currentPath = &StructuredExecutionPath{
			PathID:  fmt.Sprintf("structured_path_%d", t.pathCounter),
			Nodes:   make([]PathNode, 0),
			Success: true,
		}
	}

	// Reset storage tracking
	t.storageReads = make(map[string]map[string]string)
	t.storageWrites = make(map[string]map[string]string)

	t.pathCounter++
	t.isTraceInitialized = true
}

// finalizePaths finalizes the execution paths (equivalent to onTxEnd)
func (t *CustomTracer) finalizePaths(gasUsed uint64, err error, reverted bool) {
	if t.legacyCurrentPath != nil {
		t.legacyCurrentPath.GasUsed = gasUsed
		t.legacyCurrentPath.StorageReads = t.storageReads
		t.legacyCurrentPath.StorageWrites = t.storageWrites
		if err != nil || reverted {
			t.legacyCurrentPath.Success = false
		}
		t.legacyAllPaths = append(t.legacyAllPaths, t.legacyCurrentPath)
		fmt.Printf("=== TRACER: Legacy path completed: %s ===\n", t.legacyCurrentPath.PathID)
	}

	if t.recordStructuredPath && t.currentPath != nil {
		t.currentPath.GasUsed = gasUsed
		if err != nil || reverted {
			t.currentPath.Success = false
		}

		// Compute path hash
		t.currentPath.PathHash = t.currentPath.ComputePathHash()

		t.allPaths = append(t.allPaths, t.currentPath)
		fmt.Printf("=== TRACER: Structured path completed: %s ===\n", t.currentPath.PathID)
		fmt.Printf("    Nodes: %d, Hash: %x\n", len(t.currentPath.Nodes), t.currentPath.PathHash[:8])
	}

	// Reset for next trace
	t.isTraceInitialized = false
	t.legacyCurrentPath = nil
	t.currentPath = nil
}

// onTxStart handles transaction start (now mainly for transaction-level tracing)
func (t *CustomTracer) onTxStart(vm *tracing.VMContext, tx *types.Transaction, from common.Address) {
	// Initialize paths if not already done (in case this is called from transaction tracing)
	if !t.isTraceInitialized {
		t.initializePaths()
	}
}

// onTxEnd handles transaction end (now mainly for transaction-level tracing)
func (t *CustomTracer) onTxEnd(receipt *types.Receipt, err error) {
	// Only finalize if we have an active trace and no call stack
	if t.isTraceInitialized && len(t.callStack) == 0 {
		gasUsed := uint64(0)
		if receipt != nil {
			gasUsed = receipt.GasUsed
		}
		t.finalizePaths(gasUsed, err, false)
	}
}

// onEnter handles call start
func (t *CustomTracer) onEnter(depth int, typ byte, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	t.executionDepth = depth

	// Initialize paths if this is the first call (depth 0) and not yet initialized
	if depth == 0 && !t.isTraceInitialized {
		t.initializePaths()
	}

	var callType string
	var contractAddr common.Address

	switch vm.OpCode(typ) {
	case vm.CREATE, vm.CREATE2:
		callType = "CREATE"
		contractAddr = crypto.CreateAddress(from, 0) // Simplified
	default:
		callType = "CALL"
		contractAddr = to
	}

	// Create legacy function call
	call := &FunctionCall{
		Contract:  contractAddr,
		From:      from,
		To:        to,
		CallType:  callType,
		InputData: hexutil.Encode(input),
		Value:     new(big.Int).Set(value),
		Gas:       gas,
		Depth:     depth,
		Children:  make([]*FunctionCall, 0),
		Success:   true,
	}

	// Decode function call if we have ABI
	if callType == "CALL" && len(input) >= 4 {
		if contractABI, exists := t.contractABIs[contractAddr]; exists {
			t.decodeFunctionCall(call, input, contractABI)
		} else {
			// Try to identify function by selector
			selector := hexutil.Encode(input[:4])
			call.Selector = selector
			call.FunctionName = "unknown"
			call.FunctionSig = "unknown()"
		}
	}

	// Add to call stack
	t.callStack = append(t.callStack, call)
	t.currentCall = call

	// Add as child to parent if exists
	if len(t.callStack) > 1 {
		parent := t.callStack[len(t.callStack)-2]
		parent.Children = append(parent.Children, call)
	}

	// Add to legacy current path
	if t.legacyCurrentPath != nil {
		t.legacyCurrentPath.CallStack = append(t.legacyCurrentPath.CallStack, call)
	}

	// Create structured call node
	if t.recordStructuredPath && callType == "CALL" && len(input) >= 4 {
		var selector [4]byte
		copy(selector[:], input[:4])

		callNode := &CallNode{
			Selector:     selector,
			Parameters:   t.extractCallParameters(input, contractAddr),
			ContractAddr: contractAddr,
			FromAddr:     from,
			Depth:        depth,
			Gas:          gas,
			Value:        new(big.Int).Set(value),
			CallType:     callType,
		}

		pathNode := PathNode{
			NodeType: NodeTypeCall,
			CallNode: callNode,
			PC:       0, // Call start doesn't have specific PC
			GasUsed:  0,
		}

		t.addPathNode(pathNode)

		fmt.Printf("TRACER: Structured call node added - %s, selector: %x, params: %d\n",
			contractAddr.Hex(), selector, len(callNode.Parameters))
	}

	fmt.Printf("TRACER: Call started - %s to %s, function: %s, depth: %d, input: %s\n",
		callType, contractAddr.Hex(), call.FunctionName, depth, common.Bytes2Hex(input))
}

// onExit handles call end
func (t *CustomTracer) onExit(depth int, output []byte, gasUsed uint64, err error, reverted bool) {
	if len(t.callStack) > 0 {
		call := t.callStack[len(t.callStack)-1]
		call.GasUsed = gasUsed
		call.ReturnData = hexutil.Encode(output)

		if err != nil || reverted {
			call.Success = false
			if err != nil {
				call.Error = err.Error()
			}
			if t.legacyCurrentPath != nil {
				t.legacyCurrentPath.Success = false
			}
			if t.recordStructuredPath && t.currentPath != nil {
				t.currentPath.Success = false
			}
		}

		// Remove from call stack
		t.callStack = t.callStack[:len(t.callStack)-1]
		if len(t.callStack) > 0 {
			t.currentCall = t.callStack[len(t.callStack)-1]
		} else {
			t.currentCall = nil
		}

		fmt.Printf("TRACER: Call ended - %s, success: %t, gasUsed: %d\n",
			call.FunctionName, call.Success, gasUsed)

		// If this is the root level call (depth 0) and we're back to empty call stack,
		// finalize the paths (equivalent to onTxEnd for local EVM calls)
		if depth == 0 && len(t.callStack) == 0 && t.isTraceInitialized {
			t.finalizePaths(gasUsed, err, reverted)
		}
	}
}

// onOpcode handles opcode tracing
func (t *CustomTracer) onOpcode(pc uint64, opcode byte, gas uint64, cost uint64, scope tracing.OpContext, rData []byte, depth int, err error) {
	if t.legacyCurrentPath == nil {
		return
	}

	op := vm.OpCode(opcode)
	t.lastPC = pc
	t.lastOpCode = op

	// Get contract address
	contractAddr := scope.Address()

	// Get stack data
	stackData := scope.StackData()
	stackLen := t.getStackLen(stackData)

	// Track different types of constraints based on opcodes
	switch op {
	case vm.SLOAD:
		// Storage read
		if stackLen > 0 {
			slot := t.safeStackBack(stackData, 0)
			slotHash := common.BigToHash(slot.ToBig())

			// Legacy constraint
			constraint := PathConstraint{
				Type:        "storage",
				Description: fmt.Sprintf("Storage read from slot %s", slotHash.Hex()),
				Address:     &contractAddr,
				Slot:        &slotHash,
				PC:          pc,
				OpCode:      op.String(),
			}
			t.legacyCurrentPath.Constraints = append(t.legacyCurrentPath.Constraints, constraint)

			// Track storage read
			addrStr := contractAddr.Hex()
			if t.storageReads[addrStr] == nil {
				t.storageReads[addrStr] = make(map[string]string)
			}

			// Structured storage node
			if t.recordStructuredPath {
				// Get old value (assuming 0 if not tracked)
				oldValue := common.Hash{}
				if addrStorage, exists := t.storageState[contractAddr]; exists {
					if val, exists := addrStorage[slotHash]; exists {
						oldValue = val
					}
				}

				storageNode := &StorageNode{
					ContractAddr: contractAddr,
					Slot:         slotHash,
					OldValue:     oldValue,
					NewValue:     oldValue, // Same for read
					OpType:       StorageRead,
				}

				pathNode := PathNode{
					NodeType:    NodeTypeStorage,
					StorageNode: storageNode,
					PC:          pc,
					GasUsed:     cost,
				}

				t.addPathNode(pathNode)
			}
		}

	case vm.SSTORE:
		// Storage write
		if stackLen >= 2 {
			slot := t.safeStackBack(stackData, 0)
			value := t.safeStackBack(stackData, 1)
			slotHash := common.BigToHash(slot.ToBig())
			newValue := common.BigToHash(value.ToBig())

			// Legacy constraint
			constraint := PathConstraint{
				Type:        "storage",
				Description: fmt.Sprintf("Storage write to slot %s = %s", slotHash.Hex(), value.Hex()),
				Address:     &contractAddr,
				Slot:        &slotHash,
				Value:       value.Hex(),
				PC:          pc,
				OpCode:      op.String(),
			}
			t.legacyCurrentPath.Constraints = append(t.legacyCurrentPath.Constraints, constraint)

			// Track storage write
			addrStr := contractAddr.Hex()
			if t.storageWrites[addrStr] == nil {
				t.storageWrites[addrStr] = make(map[string]string)
			}
			t.storageWrites[addrStr][slotHash.Hex()] = value.Hex()

			// Update storage state tracking
			if t.storageState[contractAddr] == nil {
				t.storageState[contractAddr] = make(map[common.Hash]common.Hash)
			}
			oldValue := t.storageState[contractAddr][slotHash]
			t.storageState[contractAddr][slotHash] = newValue

			// Structured storage node
			if t.recordStructuredPath {
				storageNode := &StorageNode{
					ContractAddr: contractAddr,
					Slot:         slotHash,
					OldValue:     oldValue,
					NewValue:     newValue,
					OpType:       StorageWrite,
				}

				pathNode := PathNode{
					NodeType:    NodeTypeStorage,
					StorageNode: storageNode,
					PC:          pc,
					GasUsed:     cost,
				}

				t.addPathNode(pathNode)
			}
		}

	case vm.JUMPI:
		// Conditional jump - important for path analysis
		if stackLen >= 2 {
			dest := t.safeStackBack(stackData, 0)
			condition := t.safeStackBack(stackData, 1)

			// Legacy constraint
			constraint := PathConstraint{
				Type:        "jump",
				Description: fmt.Sprintf("Conditional jump to %s based on condition %s", dest.Hex(), condition.Hex()),
				Value:       condition.Hex(),
				PC:          pc,
				OpCode:      op.String(),
				StackDepth:  stackLen,
			}
			t.legacyCurrentPath.Constraints = append(t.legacyCurrentPath.Constraints, constraint)
			t.branchPoints[pc] = true

			// Structured jump node
			if t.recordStructuredPath {
				jumpNode := &JumpNode{
					Destination: common.BigToHash(dest.ToBig()),
					Condition:   common.BigToHash(condition.ToBig()),
					Taken:       condition.Sign() != 0,
				}

				pathNode := PathNode{
					NodeType: NodeTypeJump,
					JumpNode: jumpNode,
					PC:       pc,
					GasUsed:  cost,
				}

				t.addPathNode(pathNode)
			}
		}

	case vm.EQ, vm.LT, vm.GT, vm.SLT, vm.SGT, vm.ISZERO:
		// Comparison operations that might affect control flow
		if stackLen >= 1 {
			var values []string
			if op == vm.ISZERO && stackLen >= 1 {
				values = append(values, t.safeStackBack(stackData, 0).Hex())
			} else if stackLen >= 2 {
				values = append(values, t.safeStackBack(stackData, 0).Hex(), t.safeStackBack(stackData, 1).Hex())
			}

			// Legacy constraint
			constraint := PathConstraint{
				Type:        "comparison",
				Description: fmt.Sprintf("Comparison operation %s with values %v", op.String(), values),
				Value:       values,
				PC:          pc,
				OpCode:      op.String(),
				StackDepth:  stackLen,
			}
			t.legacyCurrentPath.Constraints = append(t.legacyCurrentPath.Constraints, constraint)

			// Structured stack node for important comparisons
			if t.recordStructuredPath {
				stackValues := t.extractStackValues(stackData, 5) // Extract top 5 stack values
				stackNode := &StackNode{
					Values: stackValues,
					Depth:  stackLen,
					OpCode: op.String(),
				}

				pathNode := PathNode{
					NodeType:  NodeTypeStack,
					StackNode: stackNode,
					PC:        pc,
					GasUsed:   cost,
				}

				t.addPathNode(pathNode)
			}
		}

	case vm.REVERT, vm.INVALID, vm.SELFDESTRUCT:
		// Error conditions
		constraint := PathConstraint{
			Type:        "error",
			Description: fmt.Sprintf("Execution terminated with %s", op.String()),
			PC:          pc,
			OpCode:      op.String(),
		}
		t.legacyCurrentPath.Constraints = append(t.legacyCurrentPath.Constraints, constraint)
		t.legacyCurrentPath.Success = false

		if t.recordStructuredPath && t.currentPath != nil {
			t.currentPath.Success = false
		}

	case vm.CALLDATALOAD, vm.CALLDATASIZE:
		// Input data access
		constraint := PathConstraint{
			Type:        "input",
			Description: fmt.Sprintf("Calldata access with %s", op.String()),
			PC:          pc,
			OpCode:      op.String(),
		}

		if op == vm.CALLDATALOAD && stackLen > 0 {
			offset := t.safeStackBack(stackData, 0)
			constraint.Description = fmt.Sprintf("Calldata load at offset %s", offset.Hex())
			constraint.Value = offset.Hex()
		}

		t.legacyCurrentPath.Constraints = append(t.legacyCurrentPath.Constraints, constraint)
	}
}

// onFault handles execution faults
func (t *CustomTracer) onFault(pc uint64, opcode byte, gas uint64, cost uint64, scope tracing.OpContext, depth int, err error) {
	if t.legacyCurrentPath != nil {
		op := vm.OpCode(opcode)
		constraint := PathConstraint{
			Type:        "fault",
			Description: fmt.Sprintf("Execution fault at PC %d: %s", pc, err.Error()),
			PC:          pc,
			OpCode:      op.String(),
			Value:       err.Error(),
		}

		t.legacyCurrentPath.Constraints = append(t.legacyCurrentPath.Constraints, constraint)
		t.legacyCurrentPath.Success = false
	}

	if t.recordStructuredPath && t.currentPath != nil {
		t.currentPath.Success = false
	}
}

// onGasChange handles gas changes (optional, can be empty)
func (t *CustomTracer) onGasChange(old, new uint64, reason tracing.GasChangeReason) {}

// onBalanceChange handles balance changes (optional, can be empty)
func (t *CustomTracer) onBalanceChange(a common.Address, prev, new *big.Int, reason tracing.BalanceChangeReason) {
}

// onNonceChange handles nonce changes (optional, can be empty)
func (t *CustomTracer) onNonceChange(a common.Address, prev, new uint64) {}

// onStorageChange handles storage changes
func (t *CustomTracer) onStorageChange(a common.Address, k, prev, new common.Hash) {
	// Track storage changes
	addrStr := a.Hex()
	if t.storageWrites[addrStr] == nil {
		t.storageWrites[addrStr] = make(map[string]string)
	}
	t.storageWrites[addrStr][k.Hex()] = new.Hex()

	// Update storage state tracking
	if t.storageState[a] == nil {
		t.storageState[a] = make(map[common.Hash]common.Hash)
	}
	t.storageState[a][k] = new
}

// onCodeChange handles code changes (optional, can be empty)
func (t *CustomTracer) onCodeChange(a common.Address, prevCodeHash common.Hash, prev []byte, codeHash common.Hash, code []byte) {
}

// onLog handles log events (optional, can be empty)
func (t *CustomTracer) onLog(log *types.Log) {}

// onSystemCallStart handles system call start (optional, can be empty)
func (t *CustomTracer) onSystemCallStart() {}

// onSystemCallEnd handles system call end (optional, can be empty)
func (t *CustomTracer) onSystemCallEnd() {}

// decodeFunctionCall decodes function call using ABI
func (t *CustomTracer) decodeFunctionCall(call *FunctionCall, input []byte, contractABI *abi.ABI) {
	if len(input) < 4 {
		return
	}

	selector := input[:4]
	call.Selector = hexutil.Encode(selector)

	// Find method by selector
	for _, method := range contractABI.Methods {
		if bytes.Equal(method.ID[:4], selector) {
			call.FunctionName = method.Name
			call.FunctionSig = method.Sig

			// Decode parameters
			if len(input) > 4 {
				values, err := method.Inputs.Unpack(input[4:])
				if err == nil {
					call.DecodedInputs = values

					// Add input constraints
					for i, value := range values {
						constraint := PathConstraint{
							Type:        "input",
							Description: fmt.Sprintf("Function parameter %d (%s) = %v", i, method.Inputs[i].Name, value),
							Value:       value,
							PC:          0, // Set at call start
							OpCode:      "CALL",
						}

						if t.legacyCurrentPath != nil {
							t.legacyCurrentPath.Constraints = append(t.legacyCurrentPath.Constraints, constraint)
						}
					}
				}
			}
			return
		}
	}

	// If not found, mark as unknown
	call.FunctionName = "unknown"
	call.FunctionSig = "unknown()"
}

// ToTracingHooks converts CustomTracer to tracing.Hooks struct
func (t *CustomTracer) ToTracingHooks() *tracing.Hooks {
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

// GetExecutionPaths returns all recorded legacy execution paths
func (t *CustomTracer) GetExecutionPaths() []*ExecutionPath {
	return t.legacyAllPaths
}

// GetStructuredExecutionPaths returns all recorded structured execution paths
func (t *CustomTracer) GetStructuredExecutionPaths() []*StructuredExecutionPath {
	return t.allPaths
}

// PrintExecutionSummary prints a summary of all execution paths
func (t *CustomTracer) PrintExecutionSummary() {
	fmt.Printf("\n=== EXECUTION TRACE SUMMARY ===\n")
	fmt.Printf("Legacy paths recorded: %d\n", len(t.legacyAllPaths))
	fmt.Printf("Structured paths recorded: %d\n", len(t.allPaths))

	// Print structured paths summary
	if len(t.allPaths) > 0 {
		fmt.Printf("\n=== STRUCTURED PATHS ===\n")
		for i, path := range t.allPaths {
			fmt.Printf("Path %d (%s):\n", i+1, path.PathID)
			fmt.Printf("  Success: %t\n", path.Success)
			fmt.Printf("  Gas used: %d\n", path.GasUsed)
			fmt.Printf("  Nodes: %d\n", len(path.Nodes))
			fmt.Printf("  Path hash: %x\n", path.PathHash[:8])

			// Count node types
			nodeTypes := make(map[PathNodeType]int)
			for _, node := range path.Nodes {
				nodeTypes[node.NodeType]++
			}
			fmt.Printf("  Node types: Call=%d, Storage=%d, Stack=%d, Jump=%d\n",
				nodeTypes[NodeTypeCall], nodeTypes[NodeTypeStorage],
				nodeTypes[NodeTypeStack], nodeTypes[NodeTypeJump])
		}
	}

	// Print legacy paths summary
	for i, path := range t.legacyAllPaths {
		fmt.Printf("\nLegacy Path %d (%s):\n", i+1, path.PathID)
		fmt.Printf("  Success: %t\n", path.Success)
		fmt.Printf("  Gas used: %d\n", path.GasUsed)
		fmt.Printf("  Call stack depth: %d\n", len(path.CallStack))
		fmt.Printf("  Constraints: %d\n", len(path.Constraints))

		// Print call chain
		fmt.Printf("  Call chain:\n")
		for j, call := range path.CallStack {
			indent := strings.Repeat("    ", call.Depth)
			fmt.Printf("    %d. %s%s -> %s::%s\n", j+1, indent, call.From.Hex(), call.Contract.Hex(), call.FunctionName)
		}

		// Print key constraints
		fmt.Printf("  Key constraints:\n")
		constraintCounts := make(map[string]int)
		for _, constraint := range path.Constraints {
			constraintCounts[constraint.Type]++
		}

		for cType, count := range constraintCounts {
			fmt.Printf("    %s: %d\n", cType, count)
		}
	}
}

// CompareStructuredPaths compares two structured execution paths
func (t *CustomTracer) CompareStructuredPaths(pathA, pathB *StructuredExecutionPath) map[string]interface{} {
	comparison := make(map[string]interface{})

	if pathA == nil || pathB == nil {
		comparison["error"] = "One or both paths are nil"
		return comparison
	}

	// Basic comparison
	comparison["pathsEqual"] = pathA.IsEqual(pathB)
	comparison["pathsSimilar80"] = pathA.IsSimilar(pathB, 0.8)
	comparison["pathsSimilar60"] = pathA.IsSimilar(pathB, 0.6)

	// Hash comparison
	comparison["hashesEqual"] = pathA.PathHash == pathB.PathHash
	comparison["pathAHash"] = fmt.Sprintf("%x", pathA.PathHash[:8])
	comparison["pathBHash"] = fmt.Sprintf("%x", pathB.PathHash[:8])

	// Node count comparison
	comparison["nodeCountA"] = len(pathA.Nodes)
	comparison["nodeCountB"] = len(pathB.Nodes)
	comparison["nodeCountSame"] = len(pathA.Nodes) == len(pathB.Nodes)

	// Node type distribution
	nodeTypesA := make(map[PathNodeType]int)
	nodeTypesB := make(map[PathNodeType]int)

	for _, node := range pathA.Nodes {
		nodeTypesA[node.NodeType]++
	}
	for _, node := range pathB.Nodes {
		nodeTypesB[node.NodeType]++
	}

	comparison["nodeTypesA"] = nodeTypesA
	comparison["nodeTypesB"] = nodeTypesB

	// Success comparison
	comparison["successA"] = pathA.Success
	comparison["successB"] = pathB.Success
	comparison["successSame"] = pathA.Success == pathB.Success

	return comparison
}

// CompareExecutionPaths compares two legacy execution paths and returns differences
func (t *CustomTracer) CompareExecutionPaths(pathA, pathB *ExecutionPath) map[string]interface{} {
	comparison := make(map[string]interface{})

	// Compare call stacks
	callStackSame := len(pathA.CallStack) == len(pathB.CallStack)
	if callStackSame {
		for i := 0; i < len(pathA.CallStack); i++ {
			if pathA.CallStack[i].FunctionName != pathB.CallStack[i].FunctionName ||
				pathA.CallStack[i].Contract != pathB.CallStack[i].Contract {
				callStackSame = false
				break
			}
		}
	}
	comparison["callStackSame"] = callStackSame

	// Compare constraint types
	constraintsA := make(map[string]int)
	constraintsB := make(map[string]int)

	for _, c := range pathA.Constraints {
		constraintsA[c.Type]++
	}
	for _, c := range pathB.Constraints {
		constraintsB[c.Type]++
	}

	comparison["constraintTypesA"] = constraintsA
	comparison["constraintTypesB"] = constraintsB

	// Compare storage accesses
	comparison["storageAccessSame"] = reflect.DeepEqual(pathA.StorageReads, pathB.StorageReads) &&
		reflect.DeepEqual(pathA.StorageWrites, pathB.StorageWrites)

	return comparison
}
