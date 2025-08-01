package mutation

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/DQYXACML/autopatch/tracing/utils"
)

// MutationResult Mutation result
type MutationResult struct {
	Variant           int                              `json:"variant"`
	ExecutionPath     []string                         `json:"executionPath"`     // Execution path (triples)
	SimilarityScore   float64                          `json:"similarityScore"`   // Similarity score
	ExecutionTime     time.Duration                    `json:"executionTime"`     // Execution time
	Success          bool                             `json:"success"`           // Whether execution was successful
	InputData        []byte                           `json:"inputData"`         // Mutated input data
	StorageChanges   map[common.Hash]common.Hash      `json:"storageChanges"`    // Storage changes
	MutationType     string                           `json:"mutationType"`      // Mutation type
	TargetSlot       *common.Hash                     `json:"targetSlot,omitempty"` // Target slot (Storage mutation)
	TargetArgIndex   *int                             `json:"targetArgIndex,omitempty"` // Target parameter index (InputData mutation)
}

// MutationStrategy Mutation strategy
type MutationStrategy struct {
	Name               string    `json:"name"`
	Priority           int       `json:"priority"`           // Priority (1-10)
	SuccessRate        float64   `json:"successRate"`        // Success rate
	AverageSimilarity  float64   `json:"averageSimilarity"`  // Average similarity
	TotalAttempts      int       `json:"totalAttempts"`      // Total attempts
	SuccessfulAttempts int       `json:"successfulAttempts"` // Successful attempts
	LastUsed          time.Time `json:"lastUsed"`           // Last used time
}

// SmartMutationStrategy Smart mutation strategy manager with enhanced concurrency control
type SmartMutationStrategy struct {
	strategies           map[string]*MutationStrategy
	recentResults        []MutationResult
	maxRecentResults     int
	similarityThreshold  float64
	adaptiveBatchSize    int
	minBatchSize         int
	maxBatchSize         int
	
	// Learning parameters
	learningRate         float64
	decayFactor          float64
	
	// Statistics
	totalMutations       int
	highSimilarityCount  int
	
	// Enhanced concurrency control
	concurrencyManager   *utils.ConcurrencyManager
	resultCache         *utils.SafeCache
	
	// Legacy mutex for backward compatibility (will be phased out)
	mu sync.RWMutex
}

// NewSmartMutationStrategy Create smart mutation strategy manager with enhanced concurrency
func NewSmartMutationStrategy(threshold float64) *SmartMutationStrategy {
	sms := &SmartMutationStrategy{
		strategies:          make(map[string]*MutationStrategy),
		recentResults:       make([]MutationResult, 0),
		maxRecentResults:    1000,
		similarityThreshold: threshold,
		adaptiveBatchSize:   50,
		minBatchSize:        10,
		maxBatchSize:        200,
		learningRate:        0.1,
		decayFactor:         0.95,
		// Initialize enhanced concurrency control
		concurrencyManager: utils.NewConcurrencyManager(10), // Max 10 concurrent operations
		resultCache:       utils.NewSafeCache(5 * time.Minute), // 5-minute TTL
	}
	
	// Initialize base strategies
	sms.initializeBaseStrategies()
	
	return sms
}

// initializeBaseStrategies Initialize base strategies
func (sms *SmartMutationStrategy) initializeBaseStrategies() {
	baseStrategies := []struct {
		name     string
		priority int
	}{
		// InputData mutation strategies
		{"address_known_substitution", 8},
		{"address_nearby_mutation", 6},
		{"uint256_boundary_values", 7},
		{"uint256_step_increment", 5},
		{"uint256_multiplier", 4},
		{"bool_flip", 9},
		{"bytes_pattern_fill", 3},
		{"string_length_mutation", 6},
		
		// Storage mutation strategies
		{"storage_address_mutation", 8},
		{"storage_balance_scaling", 9},
		{"storage_bool_flip", 9},
		{"storage_counter_increment", 6},
		{"storage_mapping_key_mutation", 7},
		{"storage_array_length_mutation", 5},
		
		// Combined strategies
		{"multi_slot_coordinated", 4},
		{"dependency_aware_mutation", 6},
		{"execution_path_guided", 7},
	}
	
	for _, strategy := range baseStrategies {
		sms.strategies[strategy.name] = &MutationStrategy{
			Name:              strategy.name,
			Priority:          strategy.priority,
			SuccessRate:       0.5, // Initial success rate
			AverageSimilarity: 0.3, // Initial similarity
			TotalAttempts:     0,
			SuccessfulAttempts: 0,
			LastUsed:          time.Now(),
		}
	}
}

// RecordMutationResult Record mutation result with enhanced thread safety
func (sms *SmartMutationStrategy) RecordMutationResult(result MutationResult) {
	// Use enhanced concurrency control
	err := sms.concurrencyManager.WithStrategyLock(func() error {
		// Add to recent results
		sms.recentResults = append(sms.recentResults, result)
		if len(sms.recentResults) > sms.maxRecentResults {
			sms.recentResults = sms.recentResults[1:]
		}
		
		// Update strategy statistics
		if strategy, exists := sms.strategies[result.MutationType]; exists {
			strategy.TotalAttempts++
			strategy.LastUsed = time.Now()
			
			if result.Success {
				strategy.SuccessfulAttempts++
				// Update success rate (exponential moving average)
				newSuccessRate := float64(strategy.SuccessfulAttempts) / float64(strategy.TotalAttempts)
				strategy.SuccessRate = strategy.SuccessRate*(1-sms.learningRate) + newSuccessRate*sms.learningRate
				
				// Update average similarity
				strategy.AverageSimilarity = strategy.AverageSimilarity*(1-sms.learningRate) + result.SimilarityScore*sms.learningRate
			}
		}
		
		// Update global statistics
		sms.totalMutations++
		if result.SimilarityScore >= sms.similarityThreshold {
			sms.highSimilarityCount++
		}
		
		// Dynamically adjust batch size (called within lock)
		sms.adaptBatchSize()
		
		// Cache the result for potential reuse
		cacheKey := fmt.Sprintf("result_%s_%d", result.MutationType, result.Variant)
		sms.resultCache.Set(cacheKey, result)
		
		return nil
	}, false) // Write lock
	
	if err != nil {
		// Log error but don't fail the operation
		fmt.Printf("⚠️  Error recording mutation result: %v\n", err)
	}
}

// adaptBatchSize Dynamically adjust batch size
func (sms *SmartMutationStrategy) adaptBatchSize() {
	if sms.totalMutations < 10 {
		return // Insufficient data, do not adjust yet
	}
	
	// Calculate recent success rate
	recentSuccessRate := float64(sms.highSimilarityCount) / float64(sms.totalMutations)
	
	// Adjust batch size based on success rate
	if recentSuccessRate > 0.3 {
		// High success rate, can increase batch size
		sms.adaptiveBatchSize = int(float64(sms.adaptiveBatchSize) * 1.1)
	} else if recentSuccessRate < 0.1 {
		// Low success rate, reduce batch size
		sms.adaptiveBatchSize = int(float64(sms.adaptiveBatchSize) * 0.9)
	}
	
	// Limit to reasonable range
	if sms.adaptiveBatchSize < sms.minBatchSize {
		sms.adaptiveBatchSize = sms.minBatchSize
	}
	if sms.adaptiveBatchSize > sms.maxBatchSize {
		sms.adaptiveBatchSize = sms.maxBatchSize
	}
}

// GetOptimalMutationPlan Get optimal mutation plan
func (sms *SmartMutationStrategy) GetOptimalMutationPlan(
	contractAddr common.Address,
	slotInfos []utils.StorageSlotInfo,
	inputDataLength int,
) *MutationPlan {
	sms.mu.RLock()
	defer sms.mu.RUnlock()
	
	plan := &MutationPlan{
		ContractAddress:  contractAddr,
		TotalVariants:    sms.adaptiveBatchSize,
		StorageMutations: make([]StorageMutationPlan, 0),
		InputMutations:   make([]InputMutationPlan, 0),
		PriorityOrder:    make([]string, 0),
	}
	
	// Get sorted strategies
	sortedStrategies := sms.getRankedStrategies()
	
	// Allocate mutation plans
	variantCount := 0
	maxVariantsPerStrategy := sms.adaptiveBatchSize / len(sortedStrategies)
	if maxVariantsPerStrategy < 1 {
		maxVariantsPerStrategy = 1  
	}
	
	for _, strategy := range sortedStrategies {
		if variantCount >= sms.adaptiveBatchSize {
			break
		}
		
		strategyVariants := maxVariantsPerStrategy
		if variantCount+strategyVariants > sms.adaptiveBatchSize {
			strategyVariants = sms.adaptiveBatchSize - variantCount
		}
		
		// Generate specific mutation plans based on strategy type
		if sms.isStorageStrategy(strategy.Name) {
			storagePlans := sms.generateStorageMutationPlans(&strategy, slotInfos, strategyVariants)
			plan.StorageMutations = append(plan.StorageMutations, storagePlans...)
		} else {
			inputPlans := sms.generateInputMutationPlans(&strategy, inputDataLength, strategyVariants)
			plan.InputMutations = append(plan.InputMutations, inputPlans...)
		}
		
		plan.PriorityOrder = append(plan.PriorityOrder, strategy.Name)
		variantCount += strategyVariants
	}
	
	return plan
}

// getRankedStrategies Get strategies sorted by priority
func (sms *SmartMutationStrategy) getRankedStrategies() []MutationStrategy {
	strategies := make([]MutationStrategy, 0, len(sms.strategies))
	
	for _, strategy := range sms.strategies {
		// Calculate comprehensive score
		score := sms.calculateStrategyScore(strategy)
		strategyCopy := *strategy
		strategyCopy.Priority = int(score * 10) // Convert to 1-10 priority
		strategies = append(strategies, strategyCopy)
	}
	
	// Sort by comprehensive score
	sort.Slice(strategies, func(i, j int) bool {
		return strategies[i].Priority > strategies[j].Priority
	})
	
	return strategies
}

// calculateStrategyScore Calculate strategy comprehensive score
func (sms *SmartMutationStrategy) calculateStrategyScore(strategy *MutationStrategy) float64 {
	// Base score from success rate and similarity
	baseScore := strategy.SuccessRate*0.4 + strategy.AverageSimilarity*0.4
	
	// Time decay factor (recently used strategies have slightly higher priority)
	timeSinceLastUse := time.Since(strategy.LastUsed)
	timeDecay := math.Exp(-timeSinceLastUse.Hours() / 24) // 24-hour decay
	
	// Exploration factor (give strategies with fewer attempts a chance)
	explorationBonus := 0.0
	if strategy.TotalAttempts < 10 {
		explorationBonus = 0.2
	}
	
	// Comprehensive score
	score := baseScore*0.6 + timeDecay*0.2 + explorationBonus*0.2
	
	// Ensure score is within reasonable range
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}
	
	return score
}

// isStorageStrategy Determine if it's a storage strategy
func (sms *SmartMutationStrategy) isStorageStrategy(strategyName string) bool {
	storageStrategies := map[string]bool{
		"storage_address_mutation":    true,
		"storage_balance_scaling":     true,
		"storage_bool_flip":           true,
		"storage_counter_increment":   true,
		"storage_mapping_key_mutation": true,
		"storage_array_length_mutation": true,
		"multi_slot_coordinated":      true,
		"dependency_aware_mutation":   true,
	}
	
	return storageStrategies[strategyName]
}

// generateStorageMutationPlans Generate storage mutation plans
func (sms *SmartMutationStrategy) generateStorageMutationPlans(
	strategy *MutationStrategy,
	slotInfos []utils.StorageSlotInfo,
	variants int,
) []StorageMutationPlan {
	plans := make([]StorageMutationPlan, 0)
	
	// Select appropriate slots based on strategy
	targetSlots := sms.selectTargetSlots(strategy.Name, slotInfos)
	
	for i := 0; i < variants && i < len(targetSlots); i++ {
		plan := StorageMutationPlan{
			Strategy:    strategy.Name,
			TargetSlot:  targetSlots[i].Slot,
			SlotType:    targetSlots[i].SlotType,
			Variant:     i,
			Priority:    strategy.Priority,
		}
		plans = append(plans, plan)
	}
	
	return plans
}

// generateInputMutationPlans Generate input mutation plans
func (sms *SmartMutationStrategy) generateInputMutationPlans(
	strategy *MutationStrategy,
	inputDataLength int,
	variants int,
) []InputMutationPlan {
	plans := make([]InputMutationPlan, 0)
	
	// Select appropriate parameter positions based on strategy
	targetPositions := sms.selectTargetPositions(strategy.Name, inputDataLength)
	
	for i := 0; i < variants && i < len(targetPositions); i++ {
		plan := InputMutationPlan{
			Strategy:       strategy.Name,
			TargetArgIndex: targetPositions[i],
			Variant:        i,
			Priority:       strategy.Priority,
		}
		plans = append(plans, plan)
	}
	
	return plans
}

// selectTargetSlots Select target slots
func (sms *SmartMutationStrategy) selectTargetSlots(strategyName string, slotInfos []utils.StorageSlotInfo) []utils.StorageSlotInfo {
	switch strategyName {
	case "storage_address_mutation":
		return sms.filterSlotsByType(slotInfos, utils.StorageTypeAddress)
	case "storage_balance_scaling":
		return sms.filterSlotsLikeBalance(slotInfos)
	case "storage_bool_flip":
		return sms.filterSlotsByType(slotInfos, utils.StorageTypeBool)
	case "storage_counter_increment":
		return sms.filterSlotsLikeCounter(slotInfos)
	case "storage_mapping_key_mutation":
		return sms.filterSlotsByType(slotInfos, utils.StorageTypeMapping)
	case "storage_array_length_mutation":
		return sms.filterSlotsByType(slotInfos, utils.StorageTypeArray)
	default:
		// Sort by importance and return
		sortedSlots := make([]utils.StorageSlotInfo, len(slotInfos))
		copy(sortedSlots, slotInfos)
		sort.Slice(sortedSlots, func(i, j int) bool {
			return sortedSlots[i].ImportanceScore > sortedSlots[j].ImportanceScore
		})
		return sortedSlots
	}
}

// filterSlotsByType Filter slots by type
func (sms *SmartMutationStrategy) filterSlotsByType(slotInfos []utils.StorageSlotInfo, slotType utils.StorageSlotType) []utils.StorageSlotInfo {
	filtered := make([]utils.StorageSlotInfo, 0)
	for _, slot := range slotInfos {
		if slot.SlotType == slotType {
			filtered = append(filtered, slot)
		}
	}
	return filtered
}

// filterSlotsLikeBalance Filter slots that look like balances
func (sms *SmartMutationStrategy) filterSlotsLikeBalance(slotInfos []utils.StorageSlotInfo) []utils.StorageSlotInfo {
	filtered := make([]utils.StorageSlotInfo, 0)
	for _, slot := range slotInfos {
		if slot.SlotType == utils.StorageTypeUint256 {
			// Check if it looks like a balance
			valueBig := slot.Value.Big()
			minBalance := new(big.Int).Exp(big.NewInt(10), big.NewInt(15), nil)
			maxBalance := new(big.Int).Exp(big.NewInt(10), big.NewInt(27), nil)
			
			if valueBig.Cmp(minBalance) >= 0 && valueBig.Cmp(maxBalance) <= 0 {
				filtered = append(filtered, slot)
			}
		}
	}
	return filtered
}

// filterSlotsLikeCounter Filter slots that look like counters
func (sms *SmartMutationStrategy) filterSlotsLikeCounter(slotInfos []utils.StorageSlotInfo) []utils.StorageSlotInfo {
	filtered := make([]utils.StorageSlotInfo, 0)
	for _, slot := range slotInfos {
		if slot.SlotType == utils.StorageTypeUint256 {
			valueBig := slot.Value.Big()
			// Counters are usually small positive integers
			if valueBig.Cmp(big.NewInt(0)) >= 0 && valueBig.Cmp(big.NewInt(1000000)) <= 0 {
				filtered = append(filtered, slot)
			}
		}
	}
	return filtered
}

// selectTargetPositions Select target parameter positions
func (sms *SmartMutationStrategy) selectTargetPositions(strategyName string, inputDataLength int) []int {
	positions := make([]int, 0)
	
	// Skip function selector (first 4 bytes)
	if inputDataLength <= 4 {
		return positions
	}
	
	// Calculate number of 32-byte parameters
	paramCount := (inputDataLength - 4) / 32
	if paramCount == 0 {
		return positions
	}
	
	switch strategyName {
	case "address_known_substitution", "address_nearby_mutation":
		// Addresses are usually at specific positions
		for i := 0; i < paramCount; i++ {
			positions = append(positions, i)
		}
	case "uint256_boundary_values", "uint256_step_increment", "uint256_multiplier":
		// Numeric parameters
		for i := 0; i < paramCount; i++ {
			positions = append(positions, i)
		}
	case "bool_flip":
		// Boolean parameters
		for i := 0; i < paramCount; i++ {
			positions = append(positions, i)
		}
	default:
		// Default all parameter positions
		for i := 0; i < paramCount; i++ {
			positions = append(positions, i)
		}
	}
	
	return positions
}

// GetBatchSize Get current recommended batch size
func (sms *SmartMutationStrategy) GetBatchSize() int {
	sms.mu.RLock()
	defer sms.mu.RUnlock()
	return sms.adaptiveBatchSize
}

// GetStrategyStats Get strategy statistics
func (sms *SmartMutationStrategy) GetStrategyStats() map[string]*MutationStrategy {
	sms.mu.RLock()
	defer sms.mu.RUnlock()
	
	stats := make(map[string]*MutationStrategy)
	for name, strategy := range sms.strategies {
		strategyCopy := *strategy
		stats[name] = &strategyCopy
	}
	
	return stats
}

// GetOverallStats Get overall statistics
func (sms *SmartMutationStrategy) GetOverallStats() map[string]interface{} {
	sms.mu.RLock()
	defer sms.mu.RUnlock()
	
	stats := map[string]interface{}{
		"total_mutations":         sms.totalMutations,
		"high_similarity_count":   sms.highSimilarityCount,
		"current_batch_size":      sms.adaptiveBatchSize,
		"success_rate":           float64(sms.highSimilarityCount) / float64(sms.totalMutations),
		"recent_results_count":    len(sms.recentResults),
		"similarity_threshold":    sms.similarityThreshold,
	}
	
	return stats
}

// UpdateSimilarityThreshold Update similarity threshold
func (sms *SmartMutationStrategy) UpdateSimilarityThreshold(threshold float64) {
	sms.mu.Lock()
	defer sms.mu.Unlock()
	sms.similarityThreshold = threshold
}

// ResetStrategies Reset strategy statistics (for new experiments)
func (sms *SmartMutationStrategy) ResetStrategies() {
	sms.mu.Lock()
	defer sms.mu.Unlock()
	
	for _, strategy := range sms.strategies {
		strategy.TotalAttempts = 0
		strategy.SuccessfulAttempts = 0
		strategy.SuccessRate = 0.5
		strategy.AverageSimilarity = 0.3
	}
	
	sms.recentResults = make([]MutationResult, 0)
	sms.totalMutations = 0
	sms.highSimilarityCount = 0
	sms.adaptiveBatchSize = 50
}

// MutationPlan Overall mutation plan
type MutationPlan struct {
	ContractAddress  common.Address        `json:"contractAddress"`
	TotalVariants    int                   `json:"totalVariants"`
	StorageMutations []StorageMutationPlan `json:"storageMutations"`
	InputMutations   []InputMutationPlan   `json:"inputMutations"`
	PriorityOrder    []string              `json:"priorityOrder"`
}

// StorageMutationPlan Storage mutation plan
type StorageMutationPlan struct {
	Strategy    string             `json:"strategy"`
	TargetSlot  common.Hash        `json:"targetSlot"`
	SlotType    utils.StorageSlotType    `json:"slotType"`
	Variant     int                `json:"variant"`
	Priority    int                `json:"priority"`
}

// InputMutationPlan Input mutation plan
type InputMutationPlan struct {
	Strategy       string `json:"strategy"`
	TargetArgIndex int    `json:"targetArgIndex"`
	Variant        int    `json:"variant"`
	Priority       int    `json:"priority"`
}

// PrintPlan Print mutation plan
func (mp *MutationPlan) PrintPlan() {
	fmt.Printf("=== Mutation Plan ===\n")
	fmt.Printf("Contract Address: %s\n", mp.ContractAddress.Hex())
	fmt.Printf("Total Mutations: %d\n", mp.TotalVariants)
	fmt.Printf("Storage Mutations: %d\n", len(mp.StorageMutations))
	fmt.Printf("Input Mutations: %d\n", len(mp.InputMutations))
	
	fmt.Printf("\nStrategy Priority Order:\n")
	for i, strategy := range mp.PriorityOrder {
		fmt.Printf("  %d. %s\n", i+1, strategy)
	}
	
	if len(mp.StorageMutations) > 0 {
		fmt.Printf("\nStorage Mutation Plans:\n")
		for i, plan := range mp.StorageMutations {
			if i >= 5 { // 只显示前5个
			fmt.Printf("  ... %d more storage mutations\n", len(mp.StorageMutations)-5)
				break
			}
			fmt.Printf("  %s -> Slot %s (Type: %s)\n", 
				plan.Strategy, plan.TargetSlot.Hex()[:10]+"...", plan.SlotType)
		}
	}
	
	if len(mp.InputMutations) > 0 {
		fmt.Printf("\nInput Mutation Plans:\n")
		for i, plan := range mp.InputMutations {
			if i >= 5 { // 只显示前5个
			fmt.Printf("  ... %d more input mutations\n", len(mp.InputMutations)-5)
				break
			}
			fmt.Printf("  %s -> Parameter %d\n", plan.Strategy, plan.TargetArgIndex)
		}
	}
}