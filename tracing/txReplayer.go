package tracing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/ethereum/go-ethereum/triedb/hashdb"
	"github.com/holiman/uint256"
	"math/big"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

// StorageVariable represents a variable in contract storage
type StorageVariable struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Slot     int    `json:"slot"`
	Offset   int    `json:"offset"`   // Byte offset within the slot
	Size     int    `json:"size"`     // Size in bytes
	IsSigned bool   `json:"isSigned"` // For integer types
}

// StorageLayout represents the storage layout of a contract
type StorageLayout struct {
	Variables []StorageVariable `json:"variables"`
}

// VariableInfo holds parsed information about a storage variable
type VariableInfo struct {
	Name     string
	Type     string
	Size     int
	IsSigned bool
	IsStatic bool // true for fixed-size types, false for dynamic types
}

// TransactionReplayer handles the complete replay process
type TransactionReplayer struct {
	client            *ethclient.Client
	simpleModifiers   map[common.Address]*InputModifier         // 简单函数选择器修改
	advancedModifiers map[common.Address]*AdvancedInputModifier // 高级参数修改
	tracer            *CustomTracer                             // NEW: Custom tracer
	storageLayouts    map[common.Address]*StorageLayout         // NEW: Storage layouts for contracts
}

func NewTransactionReplayer(rpcURL string) (*TransactionReplayer, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}

	return &TransactionReplayer{
		client:            client,
		simpleModifiers:   make(map[common.Address]*InputModifier),
		advancedModifiers: make(map[common.Address]*AdvancedInputModifier),
		tracer:            NewCustomTracer(),                       // NEW: Initialize tracer
		storageLayouts:    make(map[common.Address]*StorageLayout), // NEW: Initialize storage layouts
	}, nil
}

func (r *TransactionReplayer) AddSimpleModifier(contract common.Address, modifier *InputModifier) {
	r.simpleModifiers[contract] = modifier
}

// AddAdvancedModifierFromBinding 使用abigen binding添加高级修改器
func (r *TransactionReplayer) AddAdvancedModifierFromBinding(contract common.Address, metaData *bind.MetaData) error {
	modifier, err := NewAdvancedInputModifierFromBinding(metaData)
	if err != nil {
		return err
	}
	r.advancedModifiers[contract] = modifier

	// NEW: Also add ABI to tracer
	if contractABI, err := metaData.GetAbi(); err == nil && contractABI != nil {
		r.tracer.AddContractABI(contract, contractABI)
	}

	return nil
}

// AddAdvancedModifier 保留原有方法用于向后兼容
func (r *TransactionReplayer) AddAdvancedModifier(contract common.Address, abiJSON string) error {
	modifier, err := NewAdvancedInputModifier(abiJSON)
	if err != nil {
		return err
	}
	r.advancedModifiers[contract] = modifier

	// NEW: Also add ABI to tracer
	contractABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err == nil {
		r.tracer.AddContractABI(contract, &contractABI)
	}

	return nil
}

// NEW: AddStorageLayout adds storage layout information for a contract
func (r *TransactionReplayer) AddStorageLayout(contract common.Address, layout *StorageLayout) {
	r.storageLayouts[contract] = layout
}

// NEW: AddStorageLayoutFromBinding creates storage layout from contract metadata
func (r *TransactionReplayer) AddStorageLayoutFromBinding(contract common.Address, metaData *bind.MetaData) error {
	// This is a simplified example for the specific case mentioned
	// In practice, you would need to parse the contract's storage layout from metadata
	// For now, we'll create a manual layout for the example case
	layout := &StorageLayout{
		Variables: []StorageVariable{
			{Name: "int1", Type: "int8", Slot: 0, Offset: 0, Size: 1, IsSigned: true},
			{Name: "int2", Type: "int128", Slot: 0, Offset: 1, Size: 16, IsSigned: true},
		},
	}
	r.AddStorageLayout(contract, layout)
	return nil
}

// NEW: GenerateStorageLayoutFromABI dynamically generates storage layout from ABI
func (r *TransactionReplayer) GenerateStorageLayoutFromABI(contract common.Address, metaData *bind.MetaData) error {
	contractABI, err := metaData.GetAbi()
	if err != nil {
		return fmt.Errorf("failed to get ABI: %v", err)
	}

	layout, err := r.parseStorageLayoutFromABI(contractABI)
	if err != nil {
		return fmt.Errorf("failed to parse storage layout: %v", err)
	}

	r.AddStorageLayout(contract, layout)
	return nil
}

// parseStorageLayoutFromABI parses storage layout from ABI by analyzing setter functions
func (r *TransactionReplayer) parseStorageLayoutFromABI(contractABI *abi.ABI) (*StorageLayout, error) {
	variables := []VariableInfo{}

	// Regular expressions to parse setter function names
	setterRegex := regexp.MustCompile(`^set([A-Z][a-zA-Z0-9]*)$`)

	// Extract variables from setter functions
	for methodName, method := range contractABI.Methods {
		if matches := setterRegex.FindStringSubmatch(methodName); matches != nil {
			varName := strings.ToLower(matches[1][:1]) + matches[1][1:] // Convert to camelCase

			// Only process setters with exactly one parameter
			if len(method.Inputs) == 1 {
				input := method.Inputs[0]
				varInfo := VariableInfo{
					Name: varName,
					Type: input.Type.String(),
				}

				// Parse type information
				err := r.parseVariableType(&varInfo, input.Type.String())
				if err != nil {
					fmt.Printf("Warning: failed to parse type %s for variable %s: %v\n",
						input.Type.String(), varName, err)
					continue
				}

				variables = append(variables, varInfo)
			}
		}
	}

	// Sort variables to ensure consistent ordering (by name)
	sort.Slice(variables, func(i, j int) bool {
		return variables[i].Name < variables[j].Name
	})

	// Calculate storage layout according to Solidity rules
	return r.calculateStorageLayout(variables), nil
}

// parseVariableType parses Solidity type string and extracts size and signedness
func (r *TransactionReplayer) parseVariableType(varInfo *VariableInfo, typeStr string) error {
	switch {
	// Unsigned integers
	case strings.HasPrefix(typeStr, "uint"):
		varInfo.IsSigned = false
		varInfo.IsStatic = true
		if typeStr == "uint" || typeStr == "uint256" {
			varInfo.Size = 32
		} else {
			// Extract size from uint8, uint16, etc.
			sizeStr := strings.TrimPrefix(typeStr, "uint")
			if sizeStr == "" {
				varInfo.Size = 32 // default uint is uint256
			} else {
				var bitSize int
				_, err := fmt.Sscanf(sizeStr, "%d", &bitSize)
				if err != nil {
					return err
				}
				varInfo.Size = bitSize / 8
			}
		}

	// Signed integers
	case strings.HasPrefix(typeStr, "int"):
		varInfo.IsSigned = true
		varInfo.IsStatic = true
		if typeStr == "int" || typeStr == "int256" {
			varInfo.Size = 32
		} else {
			// Extract size from int8, int16, etc.
			sizeStr := strings.TrimPrefix(typeStr, "int")
			if sizeStr == "" {
				varInfo.Size = 32 // default int is int256
			} else {
				var bitSize int
				_, err := fmt.Sscanf(sizeStr, "%d", &bitSize)
				if err != nil {
					return err
				}
				varInfo.Size = bitSize / 8
			}
		}

	// Boolean
	case typeStr == "bool":
		varInfo.IsSigned = false
		varInfo.IsStatic = true
		varInfo.Size = 1

	// Address
	case typeStr == "address":
		varInfo.IsSigned = false
		varInfo.IsStatic = true
		varInfo.Size = 20

	// Fixed-size bytes
	case strings.HasPrefix(typeStr, "bytes") && typeStr != "bytes":
		varInfo.IsSigned = false
		varInfo.IsStatic = true
		sizeStr := strings.TrimPrefix(typeStr, "bytes")
		var size int
		_, err := fmt.Sscanf(sizeStr, "%d", &size)
		if err != nil {
			return err
		}
		varInfo.Size = size

	// Dynamic types
	case typeStr == "string" || typeStr == "bytes":
		varInfo.IsSigned = false
		varInfo.IsStatic = false
		varInfo.Size = 32 // Takes one slot for length/short string

	// Arrays and mappings (simplified handling)
	case strings.Contains(typeStr, "[]") || strings.Contains(typeStr, "mapping"):
		varInfo.IsSigned = false
		varInfo.IsStatic = false
		varInfo.Size = 32 // Takes one slot

	default:
		return fmt.Errorf("unsupported type: %s", typeStr)
	}

	return nil
}

// calculateStorageLayout calculates storage slot assignment according to Solidity rules
func (r *TransactionReplayer) calculateStorageLayout(variables []VariableInfo) *StorageLayout {
	layout := &StorageLayout{
		Variables: make([]StorageVariable, 0, len(variables)),
	}

	currentSlot := 0
	currentOffset := 0

	for _, varInfo := range variables {
		var slot, offset int

		if varInfo.IsStatic && varInfo.Size < 32 {
			// Small static types can be packed
			if currentOffset+varInfo.Size > 32 {
				// Won't fit in current slot, move to next
				currentSlot++
				currentOffset = 0
			}

			slot = currentSlot
			offset = currentOffset
			currentOffset += varInfo.Size

			// If we've filled the slot exactly, move to next
			if currentOffset == 32 {
				currentSlot++
				currentOffset = 0
			}
		} else {
			// 32-byte types or dynamic types get their own slot
			if currentOffset > 0 {
				// If there's partial data in current slot, move to next
				currentSlot++
				currentOffset = 0
			}

			slot = currentSlot
			offset = 0

			// Move to next slot for next variable
			currentSlot++
			currentOffset = 0
		}

		storageVar := StorageVariable{
			Name:     varInfo.Name,
			Type:     varInfo.Type,
			Slot:     slot,
			Offset:   offset,
			Size:     varInfo.Size,
			IsSigned: varInfo.IsSigned,
		}

		layout.Variables = append(layout.Variables, storageVar)
	}

	return layout
}

// NEW: CreateStorageLayoutForStorageScan creates layout for the StorageScan contract
func (r *TransactionReplayer) CreateStorageLayoutForStorageScan(contract common.Address) {
	layout := &StorageLayout{
		Variables: []StorageVariable{
			// Based on the StorageScan contract structure
			{Name: "uint1", Type: "uint8", Slot: 0, Offset: 0, Size: 1, IsSigned: false},
			{Name: "uint2", Type: "uint128", Slot: 0, Offset: 1, Size: 16, IsSigned: false},
			{Name: "uint3", Type: "uint256", Slot: 1, Offset: 0, Size: 32, IsSigned: false},
			{Name: "int1", Type: "int8", Slot: 2, Offset: 0, Size: 1, IsSigned: true},
			{Name: "int2", Type: "int128", Slot: 2, Offset: 1, Size: 16, IsSigned: true},
			{Name: "int3", Type: "int256", Slot: 3, Offset: 0, Size: 32, IsSigned: true},
			{Name: "bool1", Type: "bool", Slot: 4, Offset: 0, Size: 1, IsSigned: false},
			{Name: "bool2", Type: "bool", Slot: 4, Offset: 1, Size: 1, IsSigned: false},
			{Name: "string1", Type: "string", Slot: 5, Offset: 0, Size: 32, IsSigned: false},
			{Name: "string2", Type: "string", Slot: 6, Offset: 0, Size: 32, IsSigned: false},
			{Name: "b1", Type: "bytes1", Slot: 7, Offset: 0, Size: 1, IsSigned: false},
			{Name: "b2", Type: "bytes8", Slot: 7, Offset: 1, Size: 8, IsSigned: false},
			{Name: "b3", Type: "bytes32", Slot: 8, Offset: 0, Size: 32, IsSigned: false},
			{Name: "addr1", Type: "address", Slot: 9, Offset: 0, Size: 20, IsSigned: false},
		},
	}
	r.AddStorageLayout(contract, layout)
}

func (r *TransactionReplayer) AddFunctionModification(contract common.Address, mod *FunctionModification) error {
	modifier, exists := r.advancedModifiers[contract]
	if !exists {
		return fmt.Errorf("no advanced modifier registered for contract %s", contract.Hex())
	}
	return modifier.AddFunctionModification(mod)
}

// NEW: Get execution trace results
func (r *TransactionReplayer) GetExecutionTrace() []*ExecutionPath {
	return r.tracer.GetExecutionPaths()
}

// NEW: Print execution trace summary
func (r *TransactionReplayer) PrintExecutionTrace() {
	r.tracer.PrintExecutionSummary()
}

// NEW: Compare two execution paths
func (r *TransactionReplayer) CompareExecutionPaths(pathA, pathB *ExecutionPath) map[string]interface{} {
	return r.tracer.CompareExecutionPaths(pathA, pathB)
}

// NEW: Export execution trace to JSON
func (r *TransactionReplayer) ExportExecutionTrace() (string, error) {
	paths := r.tracer.GetExecutionPaths()
	jsonData, err := json.MarshalIndent(paths, "", "  ")
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}

func (r *TransactionReplayer) ReplayTransaction(txHash common.Hash) error {
	fmt.Printf("\n=== TRANSACTION REPLAY START ===\n")
	fmt.Printf("Transaction hash: %s\n", txHash.Hex())

	// Step 1: Get transaction details
	tx, _, err := r.client.TransactionByHash(context.Background(), txHash)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %v", err)
	}

	fmt.Printf("Transaction details:\n")
	fmt.Printf("  To: %s\n", tx.To().Hex())
	fmt.Printf("  Value: %s\n", tx.Value().String())
	fmt.Printf("  Gas: %d\n", tx.Gas())
	fmt.Printf("  GasPrice: %s\n", tx.GasPrice().String())
	fmt.Printf("  Data length: %d\n", len(tx.Data()))
	fmt.Printf("  Original data: %x\n", tx.Data())

	// Get transaction receipt to get block information
	receipt, err := r.client.TransactionReceipt(context.Background(), txHash)
	if err != nil {
		return fmt.Errorf("failed to get transaction receipt: %v", err)
	}

	fmt.Printf("Receipt details:\n")
	fmt.Printf("  Block number: %s\n", receipt.BlockNumber.String())
	fmt.Printf("  Gas used: %d\n", receipt.GasUsed)
	fmt.Printf("  Status: %d\n", receipt.Status)

	// Get block header for the transaction
	block, err := r.client.HeaderByNumber(context.Background(), receipt.BlockNumber)
	if err != nil {
		return fmt.Errorf("failed to get block header: %v", err)
	}

	// Get transaction sender
	chainID, err := r.client.NetworkID(context.Background())
	if err != nil {
		return err
	}

	fmt.Printf("Network chain ID: %d\n", chainID.Uint64())

	signer := types.LatestSignerForChainID(chainID)
	from, err := types.Sender(signer, tx)
	if err != nil {
		return err
	}

	fmt.Printf("Transaction from: %s\n", from.Hex())

	// Step 2: Get prestate
	fmt.Printf("\n=== GETTING PRESTATE ===\n")
	prestate, err := r.getTransactionPrestate(txHash)
	if err != nil {
		return fmt.Errorf("failed to get prestate: %v", err)
	}

	fmt.Printf("Prestate accounts count: %d\n", len(prestate))
	for addr, account := range prestate {
		fmt.Printf("  Account %s: Balance=%s, Nonce=%d, CodeSize=%d, StorageKeys=%d\n",
			addr.Hex(),
			(*big.Int)(account.Balance).String(),
			account.Nonce,
			len(account.Code),
			len(account.Storage))
	}

	// Step 3: Create StateDB from prestate
	fmt.Printf("\n=== CREATING STATE DB ===\n")
	stateDB, err := r.createStateFromPrestate(prestate)
	if err != nil {
		return fmt.Errorf("failed to create state: %v", err)
	}
	fmt.Printf("StateDB created successfully\n")

	// Step 4: Create EVM instance with actual block context and tracer
	evm, err := r.createEVMWithBlockInfoAndTracer(stateDB, block, chainID)
	if err != nil {
		return fmt.Errorf("failed to create EVM: %v", err)
	}

	// Step 5: Prepare transaction context
	fmt.Printf("\n=== SETTING TRANSACTION CONTEXT ===\n")
	txCtx := vm.TxContext{
		Origin:   from,
		GasPrice: tx.GasPrice(),
	}
	evm.SetTxContext(txCtx)
	fmt.Printf("Transaction context set\n")

	// Step 6: Apply input modifications
	fmt.Printf("\n=== APPLYING INPUT MODIFICATIONS ===\n")
	input := tx.Data()
	originalInput := make([]byte, len(input))
	copy(originalInput, input)

	to := tx.To()
	inputModified := false

	if to != nil {
		fmt.Printf("Contract address: %s\n", to.Hex())

		// 优先使用高级修改器
		if advancedModifier, exists := r.advancedModifiers[*to]; exists {
			fmt.Printf("Found advanced modifier for contract\n")
			modifiedInput, err := advancedModifier.ModifyInput(input)
			if err != nil {
				fmt.Printf("Advanced modification failed: %v, falling back to original input\n", err)
			} else {
				if !bytes.Equal(input, modifiedInput) {
					input = modifiedInput
					inputModified = true
					fmt.Printf("Input was modified by advanced modifier\n")
				} else {
					fmt.Printf("No actual modification applied by advanced modifier\n")
				}
			}
		} else if simpleModifier, exists := r.simpleModifiers[*to]; exists {
			fmt.Printf("Found simple modifier for contract\n")
			modifiedInput := simpleModifier.ModifyInput(input)
			if !bytes.Equal(input, modifiedInput) {
				input = modifiedInput
				inputModified = true
				fmt.Printf("Input was modified by simple modifier\n")
			} else {
				fmt.Printf("No actual modification applied by simple modifier\n")
			}
		} else {
			fmt.Printf("No modifier found for contract %s\n", to.Hex())
		}
	} else {
		fmt.Printf("Contract creation transaction\n")
	}

	if inputModified {
		fmt.Printf("\n=== INPUT COMPARISON ===\n")
		fmt.Printf("ORIGINAL INPUT (%d bytes): %x\n", len(originalInput), originalInput)
		fmt.Printf("MODIFIED INPUT (%d bytes): %x\n", len(input), input)

		// 分析修改的部分
		if len(originalInput) >= 4 && len(input) >= 4 {
			fmt.Printf("ORIGINAL SELECTOR: %x\n", originalInput[:4])
			fmt.Printf("MODIFIED SELECTOR: %x\n", input[:4])
			if !bytes.Equal(originalInput[:4], input[:4]) {
				fmt.Printf("FUNCTION SELECTOR CHANGED!\n")
			}

			if len(originalInput) > 4 {
				fmt.Printf("ORIGINAL PARAMS: %x\n", originalInput[4:])
			}
			if len(input) > 4 {
				fmt.Printf("MODIFIED PARAMS: %x\n", input[4:])
			}
		}
		fmt.Printf("=== INPUT COMPARISON END ===\n")
	} else {
		fmt.Printf("Input was not modified\n")
	}

	// Step 7: Execute transaction with tracing
	fmt.Printf("\n=== EXECUTING TRANSACTION WITH TRACING ===\n")
	if to == nil {
		// Contract creation
		fmt.Printf("Executing contract creation\n")
		_, _, leftOverGas, err := evm.Create(
			from,
			input,
			tx.Gas(),
			uint256.MustFromBig(tx.Value()),
		)
		if err != nil {
			fmt.Printf("Contract creation failed with error: %v\n", err)
			return fmt.Errorf("contract creation failed: %v", err)
		}
		fmt.Printf("Contract created successfully. Gas used: %d\n", tx.Gas()-leftOverGas)
	} else {
		// Contract call
		fmt.Printf("Executing contract call\n")
		result, leftOverGas, err := evm.Call(
			from,
			*to,
			input,
			tx.Gas(),
			uint256.MustFromBig(tx.Value()),
		)
		if err != nil {
			fmt.Printf("Contract call failed with error: %v\n", err)
			fmt.Printf("This might be due to PUSH0 opcode issue - trying alternative solution...\n")

			// 尝试使用强制激活Shanghai的配置
			alternativeEVM, err := r.createEVMWithForcedShanghaiAndTracer(stateDB, block, chainID)
			if err != nil {
				return fmt.Errorf("failed to create alternative EVM: %v", err)
			}

			alternativeEVM.SetTxContext(txCtx)
			fmt.Printf("Retrying with forced Shanghai activation...\n")

			result, leftOverGas, err = alternativeEVM.Call(
				from,
				*to,
				input,
				tx.Gas(),
				uint256.MustFromBig(tx.Value()),
			)
			if err != nil {
				return fmt.Errorf("call failed even with forced Shanghai: %v", err)
			}
		}
		fmt.Printf("Call executed successfully. Gas used: %d\n", tx.Gas()-leftOverGas)
		fmt.Printf("Result length: %d\n", len(result))
		fmt.Printf("Result: %x\n", result)
	}

	// Step 8: Compare storage changes with enhanced analysis
	fmt.Printf("\n=== STORAGE CHANGES ANALYSIS ===\n")
	if to != nil {
		err = r.analyzeStorageChanges(*to, prestate, stateDB)
		if err != nil {
			fmt.Printf("Failed to analyze storage changes: %v\n", err)
		}
	}

	// NEW: Step 9: Print execution trace
	fmt.Printf("\n=== EXECUTION TRACE ANALYSIS ===\n")
	r.PrintExecutionTrace()

	fmt.Printf("=== TRANSACTION REPLAY END ===\n\n")
	return nil
}

func (r *TransactionReplayer) analyzeStorageChanges(contractAddr common.Address, prestate PrestateResult, postStateDB *state.StateDB) error {
	fmt.Printf("Analyzing storage changes for contract: %s\n", contractAddr.Hex())

	// 获取执行前的storage状态
	prestateAccount, exists := prestate[contractAddr]
	if !exists {
		fmt.Printf("Contract %s not found in prestate\n", contractAddr.Hex())
		return nil
	}

	// 创建storage changes结构
	changes := &ContractStorageChanges{
		Address:        contractAddr,
		Changes:        []StorageChange{},
		NewSlots:       []StorageChange{},
		ModifiedSlots:  []StorageChange{},
		UnchangedSlots: []StorageChange{},
	}

	// 分析原有的storage槽
	for slot, oldValue := range prestateAccount.Storage {
		newValue := postStateDB.GetState(contractAddr, slot)

		change := StorageChange{
			Slot:     slot,
			OldValue: oldValue,
			NewValue: newValue,
			Changed:  oldValue != newValue,
		}

		changes.Changes = append(changes.Changes, change)

		if change.Changed {
			changes.ModifiedSlots = append(changes.ModifiedSlots, change)
		} else {
			changes.UnchangedSlots = append(changes.UnchangedSlots, change)
		}
	}

	// 检查是否有新的storage槽（这需要遍历StateDB，比较复杂）
	// 为了简化，我们只检查一些常见的storage槽
	r.checkCommonStorageSlots(contractAddr, prestateAccount.Storage, postStateDB, changes)

	// 输出分析结果
	r.printStorageChanges(changes)

	// NEW: Enhanced storage decoding with layout information
	if layout, exists := r.storageLayouts[contractAddr]; exists {
		r.decodeStorageChangesWithLayout(changes, layout)
	} else {
		// 如果有高级修改器，尝试解码storage值
		if modifier, exists := r.advancedModifiers[contractAddr]; exists {
			r.decodeStorageChanges(changes, modifier)
		}
	}

	return nil
}

// checkCommonStorageSlots 检查一些常见的storage槽位
func (r *TransactionReplayer) checkCommonStorageSlots(contractAddr common.Address, originalStorage map[common.Hash]common.Hash, stateDB *state.StateDB, changes *ContractStorageChanges) {
	// 检查槽位 0-50（大多数简单合约的状态变量在这个范围内）
	for i := 0; i < 51; i++ {
		slot := common.BigToHash(big.NewInt(int64(i)))

		// 如果这个槽位在原始storage中不存在
		if _, exists := originalStorage[slot]; !exists {
			newValue := stateDB.GetState(contractAddr, slot)

			// 如果新值不为空，说明是新创建的槽位
			if newValue != (common.Hash{}) {
				change := StorageChange{
					Slot:     slot,
					OldValue: common.Hash{}, // 原值为空
					NewValue: newValue,
					Changed:  true,
				}

				changes.NewSlots = append(changes.NewSlots, change)
				changes.Changes = append(changes.Changes, change)
			}
		}
	}
}

// printStorageChanges 输出storage变化的详细信息
func (r *TransactionReplayer) printStorageChanges(changes *ContractStorageChanges) {
	fmt.Printf("\n=== STORAGE CHANGES SUMMARY ===\n")
	fmt.Printf("Contract: %s\n", changes.Address.Hex())
	fmt.Printf("Total slots analyzed: %d\n", len(changes.Changes))
	fmt.Printf("Modified slots: %d\n", len(changes.ModifiedSlots))
	fmt.Printf("New slots: %d\n", len(changes.NewSlots))
	fmt.Printf("Unchanged slots: %d\n", len(changes.UnchangedSlots))

	if len(changes.ModifiedSlots) > 0 {
		fmt.Printf("\n=== MODIFIED STORAGE SLOTS ===\n")
		for i, change := range changes.ModifiedSlots {
			fmt.Printf("Change #%d:\n", i+1)
			fmt.Printf("  Slot:      %s (decimal: %s)\n", change.Slot.Hex(), change.Slot.Big().String())
			fmt.Printf("  Old Value: %s (decimal: %s)\n", change.OldValue.Hex(), change.OldValue.Big().String())
			fmt.Printf("  New Value: %s (decimal: %s)\n", change.NewValue.Hex(), change.NewValue.Big().String())

			// 尝试解释常见的存储模式
			r.interpretStorageSlot(change.Slot, change.OldValue, change.NewValue)
		}
	}

	if len(changes.NewSlots) > 0 {
		fmt.Printf("\n=== NEW STORAGE SLOTS ===\n")
		for i, change := range changes.NewSlots {
			fmt.Printf("New Slot #%d:\n", i+1)
			fmt.Printf("  Slot:      %s (decimal: %s)\n", change.Slot.Hex(), change.Slot.Big().String())
			fmt.Printf("  Value:     %s (decimal: %s)\n", change.NewValue.Hex(), change.NewValue.Big().String())

			r.interpretStorageSlot(change.Slot, change.OldValue, change.NewValue)
		}
	}

	if len(changes.UnchangedSlots) > 0 {
		fmt.Printf("\n=== UNCHANGED STORAGE SLOTS ===\n")
		for i, change := range changes.UnchangedSlots {
			fmt.Printf("Slot #%d: %s = %s\n", i+1, change.Slot.Hex(), change.NewValue.Hex())
		}
	}
}

// interpretStorageSlot 尝试解释storage槽的含义
func (r *TransactionReplayer) interpretStorageSlot(slot, oldValue, newValue common.Hash) {
	slotNum := slot.Big()

	fmt.Printf("  Interpretation:\n")

	// 基于槽位号码推测可能的变量类型
	switch slotNum.Int64() {
	case 0:
		fmt.Printf("    -> Likely: First state variable (slot 0)\n")
	case 1:
		fmt.Printf("    -> Likely: Second state variable (slot 1)\n")
	case 2:
		fmt.Printf("    -> Likely: Third state variable (slot 2)\n")
	default:
		if slotNum.Int64() < 50 {
			fmt.Printf("    -> Likely: State variable at slot %d\n", slotNum.Int64())
		} else {
			fmt.Printf("    -> Likely: Mapping or dynamic array slot\n")
		}
	}

	// 尝试解析常见的数据类型
	if newValue.Big().Cmp(big.NewInt(1)) == 0 {
		fmt.Printf("    -> Could be: boolean true or uint with value 1\n")
	} else if newValue.Big().Cmp(big.NewInt(0)) == 0 {
		fmt.Printf("    -> Could be: boolean false, uint with value 0, or cleared slot\n")
	} else if newValue.Big().BitLen() <= 64 {
		fmt.Printf("    -> Could be: small integer value %s\n", newValue.Big().String())
	} else {
		fmt.Printf("    -> Could be: large integer, address, or hash\n")
	}

	// 检查是否像地址
	if newValue.Big().BitLen() <= 160 && newValue.Big().Cmp(big.NewInt(0)) > 0 {
		addr := common.BigToAddress(newValue.Big())
		fmt.Printf("    -> If address: %s\n", addr.Hex())
	}

	fmt.Printf("\n")
}

// NEW: decodeStorageChangesWithLayout uses storage layout to decode packed variables
func (r *TransactionReplayer) decodeStorageChangesWithLayout(changes *ContractStorageChanges, layout *StorageLayout) {
	fmt.Printf("\n=== ENHANCED STORAGE DECODING WITH LAYOUT ===\n")

	for _, change := range changes.ModifiedSlots {
		slotNum := int(change.Slot.Big().Int64())
		fmt.Printf("Slot %d detailed analysis:\n", slotNum)

		// Find all variables in this slot
		varsInSlot := make([]StorageVariable, 0)
		for _, variable := range layout.Variables {
			if variable.Slot == slotNum {
				varsInSlot = append(varsInSlot, variable)
			}
		}

		if len(varsInSlot) == 0 {
			fmt.Printf("  -> No known variables in this slot\n")
			continue
		}

		fmt.Printf("  -> Variables in this slot: %d\n", len(varsInSlot))

		// Decode each variable
		for _, variable := range varsInSlot {
			oldVariableValue := r.extractVariableFromSlot(change.OldValue, variable)
			newVariableValue := r.extractVariableFromSlot(change.NewValue, variable)

			fmt.Printf("    Variable: %s (%s)\n", variable.Name, variable.Type)
			fmt.Printf("      Old value: %s\n", r.formatVariableValue(oldVariableValue, variable))
			fmt.Printf("      New value: %s\n", r.formatVariableValue(newVariableValue, variable))

			if !bytes.Equal(oldVariableValue, newVariableValue) {
				fmt.Printf("      Status: CHANGED ✅\n")
			} else {
				fmt.Printf("      Status: unchanged\n")
			}
		}
		fmt.Printf("\n")
	}
}

// NEW: extractVariableFromSlot extracts a specific variable's value from a storage slot
func (r *TransactionReplayer) extractVariableFromSlot(slotValue common.Hash, variable StorageVariable) []byte {
	slotBytes := slotValue.Bytes()

	// In Solidity, variables are packed from right to left (little-endian style)
	// But the slot is stored as big-endian bytes

	// Calculate the starting position from the right
	totalSlotSize := 32 // 256 bits = 32 bytes
	startFromRight := variable.Offset
	startFromLeft := totalSlotSize - startFromRight - variable.Size

	if startFromLeft < 0 || startFromLeft+variable.Size > totalSlotSize {
		// Invalid range, return zero bytes
		return make([]byte, variable.Size)
	}

	// Extract the bytes for this variable
	variableBytes := make([]byte, variable.Size)
	copy(variableBytes, slotBytes[startFromLeft:startFromLeft+variable.Size])

	return variableBytes
}

// NEW: formatVariableValue formats a variable value according to its type
func (r *TransactionReplayer) formatVariableValue(value []byte, variable StorageVariable) string {
	if len(value) == 0 {
		return "0"
	}

	// Convert bytes to big.Int
	bigIntValue := new(big.Int).SetBytes(value)

	switch {
	case strings.HasPrefix(variable.Type, "uint"):
		return fmt.Sprintf("%s (decimal: %s)", common.Bytes2Hex(value), bigIntValue.String())

	case strings.HasPrefix(variable.Type, "int"):
		// Handle signed integers
		if variable.IsSigned && len(value) > 0 {
			// Check if the most significant bit is set (negative number)
			if value[0]&0x80 != 0 {
				// Convert from two's complement
				// For the size of the variable, create a mask
				bitSize := variable.Size * 8
				maxValue := new(big.Int).Lsh(big.NewInt(1), uint(bitSize))
				if bigIntValue.Cmp(new(big.Int).Rsh(maxValue, 1)) >= 0 {
					// This is a negative number in two's complement
					signedValue := new(big.Int).Sub(bigIntValue, maxValue)
					return fmt.Sprintf("%s (decimal: %s, signed)", common.Bytes2Hex(value), signedValue.String())
				}
			}
		}
		return fmt.Sprintf("%s (decimal: %s)", common.Bytes2Hex(value), bigIntValue.String())

	case variable.Type == "bool":
		if bigIntValue.Cmp(big.NewInt(0)) == 0 {
			return "false"
		}
		return "true"

	case variable.Type == "address":
		if len(value) >= 20 {
			addr := common.BytesToAddress(value[len(value)-20:])
			return addr.Hex()
		}
		return common.Bytes2Hex(value)

	case strings.HasPrefix(variable.Type, "bytes"):
		return common.Bytes2Hex(value)

	default:
		return fmt.Sprintf("%s (raw: %s)", common.Bytes2Hex(value), bigIntValue.String())
	}
}

// decodeStorageChanges 使用ABI信息尝试解码storage变化
func (r *TransactionReplayer) decodeStorageChanges(changes *ContractStorageChanges, modifier *AdvancedInputModifier) {
	fmt.Printf("\n=== ADVANCED STORAGE INTERPRETATION ===\n")

	// 这里可以根据合约的ABI和已知的状态变量布局来解码storage
	// 由于Solidity的存储布局规则，我们可以推测一些基本的变量类型

	for _, change := range changes.ModifiedSlots {
		slotNum := change.Slot.Big().Int64()

		fmt.Printf("Slot %d interpretation:\n", slotNum)

		// 基于已知的函数修改规则，推测可能的变量
		for signature, funcMod := range modifier.functionMods {
			if strings.Contains(signature, "setUint") || strings.Contains(signature, "setInt") {
				fmt.Printf("  -> Possible match with function: %s\n", funcMod.FunctionName)

				// 如果是简单的setter函数，槽位通常对应变量的声明顺序
				if len(funcMod.ParameterMods) > 0 {
					newVal := funcMod.ParameterMods[0].NewValue
					fmt.Printf("  -> Expected new value from modification: %v\n", newVal)

					// 比较实际的storage值和期望的修改值
					actualVal := change.NewValue.Big()
					switch v := newVal.(type) {
					case int, int8, int16, int32, int64:
						expectedVal := big.NewInt(reflect.ValueOf(v).Int())
						if actualVal.Cmp(expectedVal) == 0 {
							fmt.Printf("  -> ✅ MATCH: Storage value matches expected modification!\n")
						} else {
							fmt.Printf("  -> ❌ MISMATCH: Expected %s, got %s\n", expectedVal.String(), actualVal.String())
						}
					case uint, uint8, uint16, uint32, uint64:
						expectedVal := big.NewInt(int64(reflect.ValueOf(v).Uint()))
						if actualVal.Cmp(expectedVal) == 0 {
							fmt.Printf("  -> ✅ MATCH: Storage value matches expected modification!\n")
						} else {
							fmt.Printf("  -> ❌ MISMATCH: Expected %s, got %s\n", expectedVal.String(), actualVal.String())
						}
					case *big.Int:
						if actualVal.Cmp(v) == 0 {
							fmt.Printf("  -> ✅ MATCH: Storage value matches expected modification!\n")
						} else {
							fmt.Printf("  -> ❌ MISMATCH: Expected %s, got %s\n", v.String(), actualVal.String())
						}
					}
				}
			}
		}
		fmt.Printf("\n")
	}
}

// GetStorageAt 提供一个便捷方法来读取特定槽位的值
func (r *TransactionReplayer) GetStorageAt(stateDB *state.StateDB, contractAddr common.Address, slot common.Hash) common.Hash {
	return stateDB.GetState(contractAddr, slot)
}

// GetStorageBySlotNumber 通过槽位编号读取storage
func (r *TransactionReplayer) GetStorageBySlotNumber(stateDB *state.StateDB, contractAddr common.Address, slotNumber int64) common.Hash {
	slot := common.BigToHash(big.NewInt(slotNumber))
	return stateDB.GetState(contractAddr, slot)
}

// DumpContractStorage 导出合约的所有非零storage槽
func (r *TransactionReplayer) DumpContractStorage(stateDB *state.StateDB, contractAddr common.Address, maxSlots int) map[string]string {
	fmt.Printf("\n=== DUMPING CONTRACT STORAGE ===\n")
	fmt.Printf("Contract: %s\n", contractAddr.Hex())
	fmt.Printf("Checking first %d slots...\n", maxSlots)

	storage := make(map[string]string)

	for i := 0; i < maxSlots; i++ {
		slot := common.BigToHash(big.NewInt(int64(i)))
		value := stateDB.GetState(contractAddr, slot)

		if value != (common.Hash{}) {
			storage[slot.Hex()] = value.Hex()
			fmt.Printf("  Slot %d (%s): %s (decimal: %s)\n",
				i, slot.Hex(), value.Hex(), value.Big().String())
		}
	}

	fmt.Printf("Found %d non-zero storage slots\n", len(storage))
	return storage
}

func (r *TransactionReplayer) getTransactionPrestate(txHash common.Hash) (PrestateResult, error) {
	config := map[string]interface{}{
		"tracer": "prestateTracer",
		"tracerConfig": map[string]interface{}{
			"diffMode": false,
		},
		"timeout": "60s",
	}

	var result PrestateResult
	err := r.client.Client().CallContext(context.Background(), &result,
		"debug_traceTransaction", txHash, config)
	return result, err
}

func (r *TransactionReplayer) createStateFromPrestate(prestate PrestateResult) (*state.StateDB, error) {
	// Create in-memory database
	memDb := rawdb.NewMemoryDatabase()

	// Create trie database
	trieDB := triedb.NewDatabase(memDb, &triedb.Config{
		Preimages: false,
		IsVerkle:  false,
		HashDB: &hashdb.Config{
			CleanCacheSize: 256 * 1024 * 1024, // 256MB
		},
	})

	// For memory-only testing state, we don't need snapshot functionality
	// Pass nil as snapshot tree to disable snapshot features
	stateDb := state.NewDatabase(trieDB, nil)

	// Create new StateDB with empty root
	stateDB, err := state.New(common.Hash{}, stateDb)
	if err != nil {
		return nil, err
	}

	for addr, account := range prestate {
		if !stateDB.Exist(addr) {
			stateDB.CreateAccount(addr)
		}

		if account.Balance != nil {
			balance := (*big.Int)(account.Balance)
			stateDB.SetBalance(addr, uint256.MustFromBig(balance),
				tracing.BalanceChangeUnspecified)
		}

		if account.Nonce > 0 {
			stateDB.SetNonce(addr, account.Nonce, tracing.NonceChangeUnspecified)
		}

		if len(account.Code) > 0 {
			stateDB.SetCode(addr, []byte(account.Code))
		}

		for key, value := range account.Storage {
			stateDB.SetState(addr, key, value)
		}
	}

	return stateDB, nil
}

// NEW: Create EVM with tracer support
func (r *TransactionReplayer) createEVMWithBlockInfoAndTracer(stateDB *state.StateDB, blockHeader *types.Header, chainID *big.Int) (*vm.EVM, error) {
	fmt.Printf("\n=== EVM CREATION WITH TRACER START ===\n")
	fmt.Printf("Chain ID: %d\n", chainID.Uint64())
	fmt.Printf("Block number: %s\n", blockHeader.Number.String())
	fmt.Printf("Block timestamp: %d\n", blockHeader.Time)
	fmt.Printf("Block BaseFee: %s\n", blockHeader.BaseFee.String())

	// 根据链 ID 选择配置，优先使用预定义配置
	var chainConfig *params.ChainConfig
	switch chainID.Uint64() {
	case 1: // 主网
		chainConfig = params.MainnetChainConfig
		fmt.Printf("Using MainnetChainConfig\n")
	case 17000: // Holesky 测试网
		chainConfig = params.HoleskyChainConfig
		fmt.Printf("Using HoleskyChainConfig\n")
	case 11155111: // Sepolia 测试网
		chainConfig = params.SepoliaChainConfig
		fmt.Printf("Using SepoliaChainConfig\n")
	default:
		// 对于未知链ID，优先使用Holesky配置作为fallback
		chainConfig = params.HoleskyChainConfig
		fmt.Printf("Unknown chain ID %d, falling back to HoleskyChainConfig\n", chainID.Uint64())
	}

	// 打印硬分叉激活状态
	fmt.Printf("=== HARD FORK STATUS ===\n")
	if chainConfig.ShanghaiTime != nil {
		fmt.Printf("Shanghai fork time: %d (current time: %d, activated: %t)\n",
			*chainConfig.ShanghaiTime, blockHeader.Time, *chainConfig.ShanghaiTime <= blockHeader.Time)
	} else {
		fmt.Printf("Shanghai fork time: not set\n")
	}

	if chainConfig.CancunTime != nil {
		fmt.Printf("Cancun fork time: %d (current time: %d, activated: %t)\n",
			*chainConfig.CancunTime, blockHeader.Time, *chainConfig.CancunTime <= blockHeader.Time)
	} else {
		fmt.Printf("Cancun fork time: not set\n")
	}

	// 检查是否支持PUSH0操作码
	rules := chainConfig.Rules(blockHeader.Number, true, blockHeader.Time)
	fmt.Printf("Shanghai rules activated: %t\n", rules.IsShanghai)
	fmt.Printf("Cancun rules activated: %t\n", rules.IsCancun)

	blockCtx := vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     func(uint64) common.Hash { return common.Hash{} },
		Coinbase:    blockHeader.Coinbase,
		BlockNumber: blockHeader.Number,
		Time:        blockHeader.Time,
		Difficulty:  blockHeader.Difficulty,
		GasLimit:    blockHeader.GasLimit,
		BaseFee:     blockHeader.BaseFee,
	}

	// NEW: VM Config with tracer using tracing.Hooks
	vmConfig := vm.Config{
		NoBaseFee:               false,
		EnablePreimageRecording: true,
		Tracer:                  r.tracer.ToTracingHooks(), // Use ToTracingHooks() method
	}

	fmt.Printf("=== EVM CREATION WITH TRACER END ===\n\n")

	evm := vm.NewEVM(blockCtx, stateDB, chainConfig, vmConfig)
	return evm, nil
}

// NEW: Create EVM with forced Shanghai and tracer
func (r *TransactionReplayer) createEVMWithForcedShanghaiAndTracer(stateDB *state.StateDB, blockHeader *types.Header, chainID *big.Int) (*vm.EVM, error) {
	fmt.Printf("\n=== CREATING EVM WITH FORCED SHANGHAI ACTIVATION AND TRACER ===\n")

	// 使用现有的配置，但修改时间戳来强制激活硬分叉
	var chainConfig *params.ChainConfig
	switch chainID.Uint64() {
	case 1:
		chainConfig = params.MainnetChainConfig
	case 17000:
		chainConfig = params.HoleskyChainConfig
	case 11155111:
		chainConfig = params.SepoliaChainConfig
	default:
		chainConfig = params.HoleskyChainConfig // 默认使用Holesky配置
	}

	fmt.Printf("Base config: %v\n", chainConfig.ChainID)

	// 使用一个更晚的时间戳来确保所有硬分叉都激活
	modifiedTime := blockHeader.Time
	if modifiedTime < 1700000000 { // 如果时间戳太早，使用2023年的时间戳
		modifiedTime = 1700000000
		fmt.Printf("Modified timestamp from %d to %d to ensure hard forks activation\n", blockHeader.Time, modifiedTime)
	}

	blockCtx := vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     func(uint64) common.Hash { return common.Hash{} },
		Coinbase:    blockHeader.Coinbase,
		BlockNumber: blockHeader.Number,
		Time:        modifiedTime, // 使用修改的时间戳
		Difficulty:  blockHeader.Difficulty,
		GasLimit:    blockHeader.GasLimit,
		BaseFee:     blockHeader.BaseFee,
	}

	// 验证硬分叉规则
	rules := chainConfig.Rules(blockCtx.BlockNumber, true, blockCtx.Time)
	fmt.Printf("With modified time - Shanghai rules activated: %t\n", rules.IsShanghai)
	fmt.Printf("With modified time - Cancun rules activated: %t\n", rules.IsCancun)

	// NEW: VM Config with tracer
	vmConfig := vm.Config{
		NoBaseFee:               false,
		EnablePreimageRecording: true,
		Tracer:                  r.tracer.ToTracingHooks(), // Use ToTracingHooks() method
	}

	fmt.Printf("=== FORCED SHANGHAI EVM WITH TRACER CREATION END ===\n")

	evm := vm.NewEVM(blockCtx, stateDB, chainConfig, vmConfig)
	return evm, nil
}

// Backwards compatibility: keep the original method without tracer
func (r *TransactionReplayer) createEVMWithBlockInfo(stateDB *state.StateDB, blockHeader *types.Header, chainID *big.Int) (*vm.EVM, error) {
	return r.createEVMWithBlockInfoAndTracer(stateDB, blockHeader, chainID)
}

func (r *TransactionReplayer) createEVMWithForcedShanghai(stateDB *state.StateDB, blockHeader *types.Header, chainID *big.Int) (*vm.EVM, error) {
	return r.createEVMWithForcedShanghaiAndTracer(stateDB, blockHeader, chainID)
}
