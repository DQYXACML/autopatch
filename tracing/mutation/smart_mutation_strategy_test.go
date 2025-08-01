package mutation

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/DQYXACML/autopatch/tracing/utils"
)

func TestSmartMutationStrategy(t *testing.T) {
	// 创建智能变异策略管理器
	sms := NewSmartMutationStrategy(0.8)
	
	// 验证初始化
	if sms.similarityThreshold != 0.8 {
		t.Errorf("Expected similarity threshold 0.8, got %f", sms.similarityThreshold)
	}
	
	if sms.adaptiveBatchSize != 50 {
		t.Errorf("Expected initial batch size 50, got %d", sms.adaptiveBatchSize)
	}
	
	// 验证策略初始化
	stats := sms.GetStrategyStats()
	if len(stats) == 0 {
		t.Error("No strategies initialized")
	}
	
	// 检查特定策略
	if strategy, exists := stats["address_known_substitution"]; !exists {
		t.Error("address_known_substitution strategy not found")
	} else {
		if strategy.Priority != 8 {
			t.Errorf("Expected priority 8 for address_known_substitution, got %d", strategy.Priority)
		}
	}
	
	t.Logf("✅ Smart mutation strategy initialization test passed")
}

func TestMutationResultRecording(t *testing.T) {
	sms := NewSmartMutationStrategy(0.7)
	
	// 记录一些测试结果
	results := []MutationResult{
		{
			Variant:         0,
			ExecutionPath:   []string{"CALL", "SSTORE", "RETURN"},
			SimilarityScore: 0.9,
			ExecutionTime:   time.Millisecond * 100,
			Success:         true,
			MutationType:    "address_known_substitution",
		},
		{
			Variant:         1,
			ExecutionPath:   []string{"CALL", "REVERT"},
			SimilarityScore: 0.3,
			ExecutionTime:   time.Millisecond * 50,
			Success:         false,
			MutationType:    "address_known_substitution",
		},
		{
			Variant:         2,
			ExecutionPath:   []string{"CALL", "SSTORE", "SLOAD", "RETURN"},
			SimilarityScore: 0.85,
			ExecutionTime:   time.Millisecond * 120,
			Success:         true,
			MutationType:    "uint256_boundary_values",
		},
	}
	
	// 记录结果
	for _, result := range results {
		sms.RecordMutationResult(result)
	}
	
	// 验证统计更新
	stats := sms.GetStrategyStats()
	
	addressStrategy := stats["address_known_substitution"]
	if addressStrategy.TotalAttempts != 2 {
		t.Errorf("Expected 2 attempts for address strategy, got %d", addressStrategy.TotalAttempts)
	}
	
	if addressStrategy.SuccessfulAttempts != 1 {
		t.Errorf("Expected 1 successful attempt for address strategy, got %d", addressStrategy.SuccessfulAttempts)
	}
	
	uint256Strategy := stats["uint256_boundary_values"]
	if uint256Strategy.TotalAttempts != 1 {
		t.Errorf("Expected 1 attempt for uint256 strategy, got %d", uint256Strategy.TotalAttempts)
	}
	
	// 验证总体统计
	overallStats := sms.GetOverallStats()
	totalMutations := overallStats["total_mutations"].(int)
	if totalMutations != 3 {
		t.Errorf("Expected 3 total mutations, got %d", totalMutations)
	}
	
	highSimilarityCount := overallStats["high_similarity_count"].(int)
	if highSimilarityCount != 2 {
		t.Errorf("Expected 2 high similarity results, got %d", highSimilarityCount)
	}
	
	t.Logf("✅ Mutation result recording test passed")
}

func TestOptimalMutationPlan(t *testing.T) {
	sms := NewSmartMutationStrategy(0.75)
	
	// 创建测试数据
	contractAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	
	// 创建测试存储槽位信息
	slotInfos := []utils.StorageSlotInfo{
		{
			Slot:        common.BigToHash(big.NewInt(0)),
			SlotType:    utils.StorageTypeAddress,
			Value:       common.HexToHash("0x000000000000000000000000742d35Cc6634C0532925a3b8D5c5C1c1b5c85C7c"),
			ImportanceScore: 0.9,
		},
		{
			Slot:        common.BigToHash(big.NewInt(1)),
			SlotType:    utils.StorageTypeUint256,
			Value:       common.HexToHash("0x0000000000000000000000000000000000000000000000000de0b6b3a7640000"),
			ImportanceScore: 0.95,
		},
		{
			Slot:        common.BigToHash(big.NewInt(2)),
			SlotType:    utils.StorageTypeBool,
			Value:       common.BigToHash(big.NewInt(1)),
			ImportanceScore: 0.6,
		},
	}
	
	// 生成变异计划
	plan := sms.GetOptimalMutationPlan(contractAddr, slotInfos, 100) // 假设100字节输入数据
	
	// 验证计划
	if plan.ContractAddress != contractAddr {
		t.Error("Contract address mismatch in plan")
	}
	
	if plan.TotalVariants != sms.GetBatchSize() {
		t.Errorf("Expected %d total variants, got %d", sms.GetBatchSize(), plan.TotalVariants)
	}
	
	if len(plan.StorageMutations) == 0 && len(plan.InputMutations) == 0 {
		t.Error("No mutations planned")
	}
	
	if len(plan.PriorityOrder) == 0 {
		t.Error("No priority order defined")
	}
	
	// 打印计划（用于调试）
	t.Logf("Generated mutation plan with %d storage mutations and %d input mutations", 
		len(plan.StorageMutations), len(plan.InputMutations))
	
	t.Logf("✅ Optimal mutation plan test passed")
}

func TestStrategyRanking(t *testing.T) {
	sms := NewSmartMutationStrategy(0.8)
	
	// 记录一些结果来影响排名
	highPerformanceResults := []MutationResult{
		{
			Variant:         0,
			SimilarityScore: 0.95,
			Success:         true,
			MutationType:    "storage_balance_scaling",
		},
		{
			Variant:         1,
			SimilarityScore: 0.92,
			Success:         true,
			MutationType:    "storage_balance_scaling",
		},
		{
			Variant:         2,
			SimilarityScore: 0.88,
			Success:         true,
			MutationType:    "storage_balance_scaling",
		},
	}
	
	lowPerformanceResults := []MutationResult{
		{
			Variant:         0,
			SimilarityScore: 0.2,
			Success:         false,
			MutationType:    "bytes_pattern_fill",
		},
		{
			Variant:         1,
			SimilarityScore: 0.1,
			Success:         false,
			MutationType:    "bytes_pattern_fill",
		},
	}
	
	// 记录高性能结果
	for _, result := range highPerformanceResults {
		sms.RecordMutationResult(result)
	}
	
	// 记录低性能结果
	for _, result := range lowPerformanceResults {
		sms.RecordMutationResult(result)
	}
	
	// 生成一个变异计划来测试排名
	contractAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	slotInfos := []utils.StorageSlotInfo{
		{
			Slot:        common.BigToHash(big.NewInt(0)),
			SlotType:    utils.StorageTypeUint256,
			Value:       common.HexToHash("0x0000000000000000000000000000000000000000000000000de0b6b3a7640000"),
			ImportanceScore: 0.95,
		},
	}
	
	plan := sms.GetOptimalMutationPlan(contractAddr, slotInfos, 100)
	
	// 验证高性能策略应该排在前面
	highPerfStrategyFound := false
	lowPerfStrategyPosition := -1
	
	for i, strategy := range plan.PriorityOrder {
		if strategy == "storage_balance_scaling" {
			highPerfStrategyFound = true
		}
		if strategy == "bytes_pattern_fill" {
			lowPerfStrategyPosition = i
		}
	}
	
	if !highPerfStrategyFound {
		t.Error("High performance strategy not found in priority order")
	}
	
	// 低性能策略应该排在后面（如果存在的话）
	if lowPerfStrategyPosition != -1 && lowPerfStrategyPosition < 3 {
		t.Errorf("Low performance strategy ranked too high at position %d", lowPerfStrategyPosition)
	}
	
	t.Logf("✅ Strategy ranking test passed")
}

func TestAdaptiveBatchSize(t *testing.T) {
	sms := NewSmartMutationStrategy(0.7)
	
	initialBatchSize := sms.GetBatchSize()
	
	// 记录一系列高成功率的结果
	for i := 0; i < 20; i++ {
		result := MutationResult{
			Variant:         i,
			SimilarityScore: 0.85, // 高于阈值
			Success:         true,
			MutationType:    "address_known_substitution",
		}
		sms.RecordMutationResult(result)
	}
	
	// 批次大小应该增加
	newBatchSize := sms.GetBatchSize()
	if newBatchSize <= initialBatchSize {
		t.Errorf("Expected batch size to increase from %d, got %d", initialBatchSize, newBatchSize)
	}
	
	// 记录一系列低成功率的结果
	for i := 0; i < 30; i++ {
		result := MutationResult{
			Variant:         i,
			SimilarityScore: 0.3, // 低于阈值
			Success:         false,
			MutationType:    "bytes_pattern_fill",
		}
		sms.RecordMutationResult(result)
	}
	
	// 批次大小应该减少
	finalBatchSize := sms.GetBatchSize()
	if finalBatchSize >= newBatchSize {
		t.Errorf("Expected batch size to decrease from %d, got %d", newBatchSize, finalBatchSize)
	}
	
	t.Logf("Batch size adaptation: %d -> %d -> %d", initialBatchSize, newBatchSize, finalBatchSize)
	t.Logf("✅ Adaptive batch size test passed")
}

func TestSlotFiltering(t *testing.T) {
	sms := NewSmartMutationStrategy(0.8)
	
	// 创建测试槽位
	slotInfos := []utils.StorageSlotInfo{
		{
			Slot:        common.BigToHash(big.NewInt(0)),
			SlotType:    utils.StorageTypeAddress,
			Value:       common.HexToHash("0x000000000000000000000000742d35Cc6634C0532925a3b8D5c5C1c1b5c85C7c"),
			ImportanceScore: 0.9,
		},
		{
			Slot:        common.BigToHash(big.NewInt(1)),
			SlotType:    utils.StorageTypeUint256,
			Value:       common.HexToHash("0x0000000000000000000000000000000000000000000000000de0b6b3a7640000"), // 1 ETH
			ImportanceScore: 0.95,
		},
		{
			Slot:        common.BigToHash(big.NewInt(2)),
			SlotType:    utils.StorageTypeBool,
			Value:       common.BigToHash(big.NewInt(1)),
			ImportanceScore: 0.6,
		},
		{
			Slot:        common.BigToHash(big.NewInt(3)),
			SlotType:    utils.StorageTypeUint256,
			Value:       common.BigToHash(big.NewInt(100)), // 小数值（计数器）
			ImportanceScore: 0.7,
		},
	}
	
	// 测试地址过滤
	addressSlots := sms.filterSlotsByType(slotInfos, utils.StorageTypeAddress)
	if len(addressSlots) != 1 {
		t.Errorf("Expected 1 address slot, got %d", len(addressSlots))
	}
	
	// 测试布尔过滤
	boolSlots := sms.filterSlotsByType(slotInfos, utils.StorageTypeBool)
	if len(boolSlots) != 1 {
		t.Errorf("Expected 1 bool slot, got %d", len(boolSlots))
	}
	
	// 测试余额过滤
	balanceSlots := sms.filterSlotsLikeBalance(slotInfos)
	if len(balanceSlots) != 1 {
		t.Errorf("Expected 1 balance-like slot, got %d", len(balanceSlots))
	}
	
	// 测试计数器过滤
	counterSlots := sms.filterSlotsLikeCounter(slotInfos)
	if len(counterSlots) != 1 { // 小数值应该被识别为计数器
		t.Errorf("Expected 1 counter-like slot, got %d", len(counterSlots))
	}
	
	t.Logf("✅ Slot filtering test passed")
}

func TestStrategyReset(t *testing.T) {
	sms := NewSmartMutationStrategy(0.8)
	
	// 记录一些结果
	result := MutationResult{
		Variant:         0,
		SimilarityScore: 0.9,
		Success:         true,
		MutationType:    "address_known_substitution",
	}
	sms.RecordMutationResult(result)
	
	// 验证有统计数据
	stats := sms.GetStrategyStats()
	if stats["address_known_substitution"].TotalAttempts == 0 {
		t.Error("Expected attempts to be recorded before reset")
	}
	
	overallStats := sms.GetOverallStats()
	if overallStats["total_mutations"].(int) == 0 {
		t.Error("Expected total mutations to be recorded before reset")
	}
	
	// 重置策略
	sms.ResetStrategies()
	
	// 验证重置后的状态
	statsAfterReset := sms.GetStrategyStats() 
	if statsAfterReset["address_known_substitution"].TotalAttempts != 0 {
		t.Error("Expected attempts to be reset to 0")
	}
	
	overallStatsAfterReset := sms.GetOverallStats()
	if overallStatsAfterReset["total_mutations"].(int) != 0 {
		t.Error("Expected total mutations to be reset to 0")
	}
	
	if sms.GetBatchSize() != 50 {
		t.Errorf("Expected batch size to be reset to 50, got %d", sms.GetBatchSize())
	}
	
	t.Logf("✅ Strategy reset test passed")
}