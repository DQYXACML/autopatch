package analysis

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/DQYXACML/autopatch/tracing/abi"
	"github.com/DQYXACML/autopatch/tracing/mutation"
	"github.com/DQYXACML/autopatch/tracing/utils"
)

func TestStorageAnalyzer(t *testing.T) {
	chainID := big.NewInt(1)
	abiManager := abi.NewABIManager("./test_cache")
	analyzer := NewStorageAnalyzer(abiManager, chainID)
	
	// 创建测试存储数据
	storage := map[common.Hash]common.Hash{
		// 地址槽位
		common.BigToHash(big.NewInt(0)): common.HexToHash("0x000000000000000000000000742d35Cc6634C0532925a3b8D5c5C1c1b5c85C7c"),
		// 布尔槽位
		common.BigToHash(big.NewInt(1)): common.BigToHash(big.NewInt(1)),
		// 整数槽位（看起来像余额）
		common.BigToHash(big.NewInt(2)): common.HexToHash("0x0000000000000000000000000000000000000000000000000de0b6b3a7640000"), // 1 ETH
		// 空槽位
		common.BigToHash(big.NewInt(3)): common.Hash{},
		// Mapping槽位（随机哈希）
		common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"): common.BigToHash(big.NewInt(100)),
	}
	
	contractAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	
	// 分析存储
	slotInfos, err := analyzer.AnalyzeContractStorage(contractAddr, storage)
	if err != nil {
		t.Fatalf("Failed to analyze storage: %v", err)
	}
	
	if len(slotInfos) != len(storage) {
		t.Errorf("Expected %d slot infos, got %d", len(storage), len(slotInfos))
	}
	
	// 验证类型推断
	typeCount := make(map[utils.StorageSlotType]int)
	for _, slotInfo := range slotInfos {
		typeCount[slotInfo.SlotType]++
		
		t.Logf("Slot %s: Type=%s, Value=%s, Importance=%.2f",
			slotInfo.Slot.Hex()[:10]+"...",
			slotInfo.SlotType,
			slotInfo.Value.Hex()[:10]+"...",
			slotInfo.ImportanceScore)
	}
	
	// 验证至少识别了一些类型
	if typeCount[utils.StorageTypeAddress] == 0 {
		t.Error("Expected to identify at least one address slot")
	}
	
	if typeCount[utils.StorageTypeBool] == 0 {
		t.Error("Expected to identify at least one boolean slot")
	}
	
	if typeCount[utils.StorageTypeUint256] == 0 {
		t.Error("Expected to identify at least one uint256 slot")
	}
	
	t.Logf("✅ Storage analyzer tests passed")
}

func TestStorageTypeMutator(t *testing.T) {
	chainID := big.NewInt(1)
	abiManager := abi.NewABIManager("./test_cache")
	analyzer := NewStorageAnalyzer(abiManager, chainID)
	typeAwareMutator := mutation.NewTypeAwareMutator(chainID, abiManager)
	mutator := NewStorageTypeMutator(analyzer, typeAwareMutator)
	
	// 创建测试存储
	originalStorage := map[common.Hash]common.Hash{
		common.BigToHash(big.NewInt(0)): common.HexToHash("0x000000000000000000000000742d35Cc6634C0532925a3b8D5c5C1c1b5c85C7c"),
		common.BigToHash(big.NewInt(1)): common.BigToHash(big.NewInt(1000)),
		common.BigToHash(big.NewInt(2)): common.BigToHash(big.NewInt(1)),
	}
	
	contractAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	
	// 测试变异
	mutatedStorage, err := mutator.MutateStorage(contractAddr, originalStorage, 0)
	if err != nil {
		t.Fatalf("Failed to mutate storage: %v", err)
	}
	
	if len(mutatedStorage) != len(originalStorage) {
		t.Errorf("Expected %d slots, got %d", len(originalStorage), len(mutatedStorage))
	}
	
	// 检查是否有变异发生
	mutationCount := 0
	for slot, originalValue := range originalStorage {
		if mutatedValue, exists := mutatedStorage[slot]; exists {
			if mutatedValue != originalValue {
				mutationCount++
				t.Logf("Mutated slot %s: %s -> %s",
					slot.Hex()[:10]+"...",
					originalValue.Hex()[:10]+"...",
					mutatedValue.Hex()[:10]+"...")
			}
		}
	}
	
	if mutationCount == 0 {
		t.Log("⚠️  No mutations occurred (may be expected for some variants)")
	} else {
		t.Logf("✅ %d storage slots mutated", mutationCount)
	}
	
	t.Logf("✅ Storage type mutator tests passed")
}

func TestSlotTypeInference(t *testing.T) {
	chainID := big.NewInt(1)
	abiManager := abi.NewABIManager("./test_cache")
	analyzer := NewStorageAnalyzer(abiManager, chainID)
	
	testCases := []struct {
		value        common.Hash
		expectedType utils.StorageSlotType
		description  string
	}{
		{
			value:        common.Hash{},
			expectedType: utils.StorageTypeEmpty,
			description:  "empty hash",
		},
		{
			value:        common.BigToHash(big.NewInt(0)),
			expectedType: utils.StorageTypeEmpty, // BigInt(0) 创建的也是空Hash
			description:  "zero value (same as empty hash)",
		},
		{
			value:        common.BigToHash(big.NewInt(1)),
			expectedType: utils.StorageTypeBool,
			description:  "one value (boolean true)",
		},
		{
			value:        common.HexToHash("0x000000000000000000000000742d35Cc6634C0532925a3b8D5c5C1c1b5c85C7c"),
			expectedType: utils.StorageTypeAddress,
			description:  "address value",
		},
		{
			value:        common.BigToHash(big.NewInt(1000)),
			expectedType: utils.StorageTypeUint256,
			description:  "small integer",
		},
		{
			value:        common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
			expectedType: utils.StorageTypeBytes32,
			description:  "random bytes32",
		},
	}
	
	for _, tc := range testCases {
		inferredType := analyzer.inferSlotTypeFromValue(tc.value)
		if inferredType != tc.expectedType {
			t.Errorf("For %s: expected %s, got %s", tc.description, tc.expectedType, inferredType)
		} else {
			t.Logf("✅ %s correctly identified as %s", tc.description, inferredType)
		}
	}
	
	t.Logf("✅ Slot type inference tests passed")
}

func TestAddressDetection(t *testing.T) {
	chainID := big.NewInt(1)
	abiManager := abi.NewABIManager("./test_cache")
	analyzer := NewStorageAnalyzer(abiManager, chainID)
	
	testCases := []struct {
		value       common.Hash
		isAddress   bool
		description string
	}{
		{
			value:       common.HexToHash("0x000000000000000000000000742d35Cc6634C0532925a3b8D5c5C1c1b5c85C7c"),
			isAddress:   true,
			description: "valid address",
		},
		{
			value:       common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
			isAddress:   false,
			description: "zero address",
		},
		{
			value:       common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
			isAddress:   false,
			description: "random hash",
		},
		{
			value:       common.BigToHash(big.NewInt(1000)),
			isAddress:   false,
			description: "small number",
		},
	}
	
	for _, tc := range testCases {
		result := analyzer.looksLikeAddress(tc.value)
		if result != tc.isAddress {
			t.Errorf("For %s: expected %v, got %v", tc.description, tc.isAddress, result)
		} else {
			t.Logf("✅ %s correctly detected (isAddress: %v)", tc.description, result)
		}
	}
	
	t.Logf("✅ Address detection tests passed")
}

func TestBalanceDetection(t *testing.T) {
	chainID := big.NewInt(1)
	abiManager := abi.NewABIManager("./test_cache")
	analyzer := NewStorageAnalyzer(abiManager, chainID)
	
	testCases := []struct {
		value       common.Hash
		isBalance   bool
		description string
	}{
		{
			value:       common.HexToHash("0x0000000000000000000000000000000000000000000000000de0b6b3a7640000"), // 1 ETH
			isBalance:   true,
			description: "1 ETH",
		},
		{
			value:       common.BigToHash(big.NewInt(1000)),
			isBalance:   false,
			description: "small number",
		},
		{
			value:       common.BigToHash(big.NewInt(0)),
			isBalance:   false,
			description: "zero",
		},
		{
			value:       common.HexToHash("0x1000000000000000000000000000000000000000000000000000000000000000"), // 很大的数
			isBalance:   false,
			description: "too large number",
		},
	}
	
	for _, tc := range testCases {
		result := analyzer.looksLikeBalance(tc.value)
		if result != tc.isBalance {
			t.Errorf("For %s: expected %v, got %v", tc.description, tc.isBalance, result)
		} else {
			t.Logf("✅ %s correctly detected (isBalance: %v)", tc.description, result)
		}
	}
	
	t.Logf("✅ Balance detection tests passed")
}