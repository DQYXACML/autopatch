package analysis

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	abiPkg "github.com/DQYXACML/autopatch/tracing/abi"
	"github.com/DQYXACML/autopatch/tracing/mutation"
	"github.com/DQYXACML/autopatch/tracing/utils"
)

// StorageAnalyzer 存储分析器
type StorageAnalyzer struct {
	abiManager *abiPkg.ABIManager
	chainID    *big.Int
}

// NewStorageAnalyzer 创建存储分析器
func NewStorageAnalyzer(abiManager *abiPkg.ABIManager, chainID *big.Int) *StorageAnalyzer {
	return &StorageAnalyzer{
		abiManager: abiManager,
		chainID:    chainID,
	}
}

// AnalyzeContractStorage 分析合约存储
func (sa *StorageAnalyzer) AnalyzeContractStorage(
	contractAddr common.Address,
	storage map[common.Hash]common.Hash,
) ([]utils.StorageSlotInfo, error) {
	var slots []utils.StorageSlotInfo
	
	// 获取合约ABI
	contractABI, err := sa.abiManager.GetContractABI(sa.chainID, contractAddr)
	if err != nil {
		// 如果无法获取ABI，使用启发式分析
		fmt.Printf("⚠️  Could not get ABI for %s, using heuristic analysis\n", contractAddr.Hex())
		return sa.analyzeStorageHeuristically(storage), nil
	}
	
	// 基于ABI分析存储
	slots = sa.analyzeStorageWithABI(contractABI, storage)
	
	// 为每个槽位计算重要性分数和变异策略
	for i := range slots {
		slots[i].ImportanceScore = sa.calculateSlotImportance(slots[i])
		slots[i].MutationStrategies = sa.getMutationStrategiesForSlot(slots[i])
	}
	
	return slots, nil
}

// analyzeStorageWithABI 基于ABI分析存储
func (sa *StorageAnalyzer) analyzeStorageWithABI(
	contractABI *abi.ABI,
	storage map[common.Hash]common.Hash,
) []utils.StorageSlotInfo {
	var slots []utils.StorageSlotInfo
	
	// TODO: 这里需要更复杂的存储布局分析
	// 目前先进行基础分析
	for slot, value := range storage {
		slotInfo := utils.StorageSlotInfo{
			Slot:     slot,
			Value:    value,
			SlotType: sa.inferSlotTypeFromValue(value),
		}
		
		// 尝试从ABI推断更精确的类型
		sa.enhanceSlotInfoWithABI(contractABI, &slotInfo)
		
		slots = append(slots, slotInfo)
	}
	
	return slots
}

// analyzeStorageHeuristically 启发式分析存储
func (sa *StorageAnalyzer) analyzeStorageHeuristically(
	storage map[common.Hash]common.Hash,
) []utils.StorageSlotInfo {
	var slots []utils.StorageSlotInfo
	
	for slot, value := range storage {
		slotInfo := utils.StorageSlotInfo{
			Slot:        slot,
			Value:       value,
			SlotType:    sa.inferSlotTypeFromValue(value),
			Description: sa.generateSlotDescription(slot, value),
		}
		
		slots = append(slots, slotInfo)
	}
	
	return slots
}

// inferSlotTypeFromValue 从值推断槽位类型
func (sa *StorageAnalyzer) inferSlotTypeFromValue(value common.Hash) utils.StorageSlotType {
	// 检查是否为完全空的Hash
	if value == (common.Hash{}) {
		return utils.StorageTypeEmpty
	}
	
	valueBig := value.Big()
	
	// 检查是否为布尔值（0或1）
	if valueBig.Cmp(big.NewInt(0)) == 0 {
		return utils.StorageTypeBool
	}
	if valueBig.Cmp(big.NewInt(1)) == 0 {
		return utils.StorageTypeBool
	}
	
	// 检查是否为地址（非零值且在地址范围内）
	if sa.looksLikeAddress(value) {
		return utils.StorageTypeAddress
	}
	
	// 检查是否为小整数（可能是计数器、标志等）
	if valueBig.Cmp(big.NewInt(1000000)) < 0 {
		return utils.StorageTypeUint256
	}
	
	// 检查是否看起来像字符串长度或数组长度
	if sa.looksLikeLength(valueBig) {
		return utils.StorageTypeUint256
	}
	
	// 默认为bytes32
	return utils.StorageTypeBytes32
}

// looksLikeAddress 检查值是否看起来像地址
func (sa *StorageAnalyzer) looksLikeAddress(value common.Hash) bool {
	bytes := value.Bytes()
	
	// 检查前12字节是否为0
	for i := 0; i < 12; i++ {
		if bytes[i] != 0 {
			return false
		}
	}
	
	// 检查后20字节是否不全为0（除非是零地址）
	hasNonZero := false
	nonZeroCount := 0
	for i := 12; i < 32; i++ {
		if bytes[i] != 0 {
			hasNonZero = true
			nonZeroCount++
		}
	}
	
	// 必须有非零字节才能是地址
	if !hasNonZero {
		return false
	}
	
	// 如果只有最后几个字节非零，可能是小整数而不是地址
	// 地址通常有更多的非零字节
	if nonZeroCount < 4 {
		valueBig := value.Big()
		// 如果数值小于2^32，很可能是整数而不是地址
		if valueBig.Cmp(big.NewInt(4294967296)) < 0 {
			return false
		}
	}
	
	return true
}

// looksLikeLength 检查值是否看起来像长度
func (sa *StorageAnalyzer) looksLikeLength(value *big.Int) bool {
	// 长度通常是合理的小值
	return value.Cmp(big.NewInt(0)) >= 0 && value.Cmp(big.NewInt(10000)) <= 0
}

// enhanceSlotInfoWithABI 使用ABI增强槽位信息
func (sa *StorageAnalyzer) enhanceSlotInfoWithABI(contractABI *abi.ABI, slotInfo *utils.StorageSlotInfo) {
	// TODO: 实现更精确的ABI映射
	// 这需要分析合约的存储布局
	
	// 目前只做基础的类型映射
	switch slotInfo.SlotType {
	case utils.StorageTypeAddress:
		addressType, _ := abi.NewType("address", "", nil)
		slotInfo.AbiType = &addressType
	case utils.StorageTypeUint256:
		uint256Type, _ := abi.NewType("uint256", "", nil)
		slotInfo.AbiType = &uint256Type
	case utils.StorageTypeBool:
		boolType, _ := abi.NewType("bool", "", nil)
		slotInfo.AbiType = &boolType
	case utils.StorageTypeBytes32:
		bytes32Type, _ := abi.NewType("bytes32", "", nil)
		slotInfo.AbiType = &bytes32Type
	}
}

// generateSlotDescription 生成槽位描述
func (sa *StorageAnalyzer) generateSlotDescription(slot, value common.Hash) string {
	slotNum := slot.Big()
	
	// 基于槽位号生成描述
	switch {
	case slotNum.Cmp(big.NewInt(10)) < 0:
		return fmt.Sprintf("Storage slot %s (simple variable)", slotNum.String())
	case sa.looksLikeMapping(slot):
		return fmt.Sprintf("Mapping entry (slot hash: %s)", slot.Hex()[:10]+"...")
	case sa.looksLikeArray(slot):
		return fmt.Sprintf("Array element (slot: %s)", slot.Hex()[:10]+"...")
	default:
		return fmt.Sprintf("Storage slot %s", slot.Hex()[:10]+"...")
	}
}

// looksLikeMapping 检查是否看起来像mapping条目
func (sa *StorageAnalyzer) looksLikeMapping(slot common.Hash) bool {
	// Mapping的槽位通常是keccak256哈希的结果
	// 这些值通常看起来很随机
	bytes := slot.Bytes()
	
	// 简单的随机性检查：至少一半的字节不为0
	nonZeroCount := 0
	for _, b := range bytes {
		if b != 0 {
			nonZeroCount++
		}
	}
	
	return nonZeroCount > len(bytes)/2
}

// looksLikeArray 检查是否看起来像数组元素
func (sa *StorageAnalyzer) looksLikeArray(slot common.Hash) bool {
	// 数组元素的槽位通常基于基础槽位计算
	// 这里做简化处理
	slotBig := slot.Big()
	
	// 如果槽位号很大但有规律，可能是数组
	return slotBig.Cmp(big.NewInt(1000)) > 0
}

// calculateSlotImportance 计算槽位重要性
func (sa *StorageAnalyzer) calculateSlotImportance(slotInfo utils.StorageSlotInfo) float64 {
	importance := 0.5 // 基础分数
	
	// 根据类型调整重要性
	switch slotInfo.SlotType {
	case utils.StorageTypeAddress:
		importance = 0.9 // 地址很重要
	case utils.StorageTypeUint256:
		// 判断是否为余额或计数器
		if sa.looksLikeBalance(slotInfo.Value) {
			importance = 0.95 // 余额最重要
		} else {
			importance = 0.7 // 普通数值
		}
	case utils.StorageTypeBool:
		importance = 0.6 // 布尔值中等重要
	case utils.StorageTypeMapping:
		importance = 0.85 // Mapping很重要
	case utils.StorageTypeArray:
		importance = 0.75 // 数组比较重要
	case utils.StorageTypeEmpty:
		importance = 0.1 // 空值不重要
	}
	
	// 根据槽位位置调整重要性（前几个槽位通常更重要）
	slotNum := slotInfo.Slot.Big()
	if slotNum.Cmp(big.NewInt(10)) < 0 {
		importance += 0.1 // 前10个槽位加分
	}
	
	// 确保分数在0-1范围内
	if importance > 1.0 {
		importance = 1.0
	}
	if importance < 0.0 {
		importance = 0.0
	}
	
	return importance
}

// looksLikeBalance 检查值是否看起来像余额
func (sa *StorageAnalyzer) looksLikeBalance(value common.Hash) bool {
	valueBig := value.Big()
	
	// 余额通常是较大的数值，但不会太大
	minBalance := new(big.Int).Exp(big.NewInt(10), big.NewInt(15), nil) // 0.001 ETH in wei
	maxBalance := new(big.Int).Exp(big.NewInt(10), big.NewInt(27), nil) // 1B ETH in wei
	
	return valueBig.Cmp(minBalance) >= 0 && valueBig.Cmp(maxBalance) <= 0
}

// getMutationStrategiesForSlot 获取槽位的变异策略
func (sa *StorageAnalyzer) getMutationStrategiesForSlot(slotInfo utils.StorageSlotInfo) []string {
	var strategies []string
	
	// 基础策略
	strategies = append(strategies, "step_increment", "step_decrement")
	
	// 根据类型添加特定策略
	switch slotInfo.SlotType {
	case utils.StorageTypeAddress:
		strategies = append(strategies, "known_addresses", "zero_address", "flip_bytes")
	case utils.StorageTypeUint256:
		strategies = append(strategies, "boundary_values", "multiplication", "bit_patterns")
		if sa.looksLikeBalance(slotInfo.Value) {
			strategies = append(strategies, "balance_scaling", "zero_balance")
		}
	case utils.StorageTypeBool:
		strategies = append(strategies, "boolean_flip")
	case utils.StorageTypeBytes32:
		strategies = append(strategies, "byte_flip", "pattern_fill", "hash_collision")
	case utils.StorageTypeMapping:
		strategies = append(strategies, "key_mutation", "value_mutation")
	case utils.StorageTypeArray:
		strategies = append(strategies, "length_mutation", "element_mutation")
	}
	
	// 根据重要性添加特殊策略
	if slotInfo.ImportanceScore > 0.8 {
		strategies = append(strategies, "conservative_mutation") // 重要槽位使用保守变异
	}
	
	return strategies
}

// StorageTypeMutator 存储类型变异器
type StorageTypeMutator struct {
	analyzer       *StorageAnalyzer
	typeAwareMutator *mutation.TypeAwareMutator
}

// NewStorageTypeMutator 创建存储类型变异器
func NewStorageTypeMutator(analyzer *StorageAnalyzer, typeAwareMutator *mutation.TypeAwareMutator) *StorageTypeMutator {
	return &StorageTypeMutator{
		analyzer:         analyzer,
		typeAwareMutator: typeAwareMutator,
	}
}

// MutateStorage 基于类型变异存储
func (stm *StorageTypeMutator) MutateStorage(
	contractAddr common.Address,
	originalStorage map[common.Hash]common.Hash,
	variant int,
) (map[common.Hash]common.Hash, error) {
	// 分析存储结构
	slotInfos, err := stm.analyzer.AnalyzeContractStorage(contractAddr, originalStorage)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze storage: %v", err)
	}
	
	mutatedStorage := make(map[common.Hash]common.Hash)
	
	// 选择要变异的槽位（基于重要性）
	slotsToMutate := stm.selectSlotsForMutation(slotInfos, variant)
	
	for _, slotInfo := range slotInfos {
		if stm.shouldMutateSlot(slotInfo, slotsToMutate) {
			mutatedValue := stm.mutateSlotValue(slotInfo, variant)
			mutatedStorage[slotInfo.Slot] = mutatedValue
		} else {
			mutatedStorage[slotInfo.Slot] = slotInfo.Value
		}
	}
	
	return mutatedStorage, nil
}

// selectSlotsForMutation 选择要变异的槽位
func (stm *StorageTypeMutator) selectSlotsForMutation(slotInfos []utils.StorageSlotInfo, variant int) map[common.Hash]bool {
	selected := make(map[common.Hash]bool)
	
	// 根据变异策略选择槽位
	maxSlots := 3 // 最多变异3个槽位
	if len(slotInfos) < maxSlots {
		maxSlots = len(slotInfos)
	}
	
	// 按重要性排序（简化版）
	importantSlots := make([]utils.StorageSlotInfo, 0)
	for _, slot := range slotInfos {
		if slot.ImportanceScore > 0.5 {
			importantSlots = append(importantSlots, slot)
		}
	}
	
	// 选择前几个重要槽位
	count := 0
	for _, slot := range importantSlots {
		if count >= maxSlots {
			break
		}
		if (variant+count)%2 == 0 { // 增加随机性
			selected[slot.Slot] = true
			count++
		}
	}
	
	// 如果没有选到足够的槽位，随机选择一些
	if count == 0 && len(slotInfos) > 0 {
		selectedSlot := slotInfos[variant%len(slotInfos)]
		selected[selectedSlot.Slot] = true
	}
	
	return selected
}

// shouldMutateSlot 判断是否应该变异该槽位
func (stm *StorageTypeMutator) shouldMutateSlot(slotInfo utils.StorageSlotInfo, selected map[common.Hash]bool) bool {
	return selected[slotInfo.Slot]
}

// mutateSlotValue 变异槽位值
func (stm *StorageTypeMutator) mutateSlotValue(slotInfo utils.StorageSlotInfo, variant int) common.Hash {
	// 根据槽位类型选择变异策略
	switch slotInfo.SlotType {
	case utils.StorageTypeAddress:
		return stm.mutateAddressSlot(slotInfo.Value, variant)
	case utils.StorageTypeUint256:
		return stm.mutateUintSlot(slotInfo.Value, variant)
	case utils.StorageTypeBool:
		return stm.mutateBoolSlot(slotInfo.Value, variant)
	case utils.StorageTypeBytes32:
		return stm.mutateBytesSlot(slotInfo.Value, variant)
	default:
		return stm.mutateGenericSlot(slotInfo.Value, variant)
	}
}

// mutateAddressSlot 变异地址槽位
func (stm *StorageTypeMutator) mutateAddressSlot(value common.Hash, variant int) common.Hash {
	// 提取地址部分（后20字节）
	addr := common.BytesToAddress(value.Bytes()[12:])
	
	// 使用简单的地址变异逻辑
	if stm.typeAwareMutator != nil {
		// 提取地址并创建ABI类型
		addrType, _ := abi.NewType("address", "", nil)
		mutatedValue, err := stm.typeAwareMutator.MutateByType(addrType, addr, variant)
		if err == nil {
			if mutatedAddr, ok := mutatedValue.(common.Address); ok {
				// 将地址转换回hash格式
				var result common.Hash
				copy(result[12:], mutatedAddr.Bytes())
				return result
			}
		}
	}
	
	// 回退到简单变异
	return stm.mutateGenericSlot(value, variant)
}

// mutateUintSlot 变异整数槽位
func (stm *StorageTypeMutator) mutateUintSlot(value common.Hash, variant int) common.Hash {
	valueBig := value.Big()
	
	// 使用简单的整数变异逻辑
	if stm.typeAwareMutator != nil {
		// 创建uint256类型
		uint256Type, _ := abi.NewType("uint256", "", nil)
		mutatedValue, err := stm.typeAwareMutator.MutateByType(uint256Type, valueBig, variant)
		if err == nil {
			if mutatedBig, ok := mutatedValue.(*big.Int); ok {
				return common.BigToHash(mutatedBig)
			}
		}
	}
	
	// 回退到简单变异
	return stm.mutateGenericSlot(value, variant)
}

// mutateBoolSlot 变异布尔槽位
func (stm *StorageTypeMutator) mutateBoolSlot(value common.Hash, variant int) common.Hash {
	valueBig := value.Big()
	
	// 布尔值翻转
	if valueBig.Cmp(big.NewInt(0)) == 0 {
		return common.BigToHash(big.NewInt(1))
	} else {
		return common.BigToHash(big.NewInt(0))
	}
}

// mutateBytesSlot 变异字节槽位
func (stm *StorageTypeMutator) mutateBytesSlot(value common.Hash, variant int) common.Hash {
	bytes := value.Bytes()
	mutatedBytes := make([]byte, len(bytes))
	copy(mutatedBytes, bytes)
	
	// 翻转一些字节
	flipCount := (variant % 3) + 1
	for i := 0; i < flipCount; i++ {
		index := (variant + i) % len(mutatedBytes)
		mutatedBytes[index] ^= 0xFF
	}
	
	return common.BytesToHash(mutatedBytes)
}

// mutateGenericSlot 通用槽位变异
func (stm *StorageTypeMutator) mutateGenericSlot(value common.Hash, variant int) common.Hash {
	valueBig := value.Big()
	
	// 简单的数值变异
	steps := []*big.Int{
		big.NewInt(1), big.NewInt(-1), big.NewInt(10), big.NewInt(-10),
		big.NewInt(100), big.NewInt(-100), big.NewInt(1000), big.NewInt(-1000),
	}
	
	step := steps[variant%len(steps)]
	result := new(big.Int).Add(valueBig, step)
	
	// 确保非负
	if result.Sign() < 0 {
		result = big.NewInt(0)
	}
	
	return common.BigToHash(result)
}