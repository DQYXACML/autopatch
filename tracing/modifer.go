package tracing

import (
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"math/big"
	"reflect"
	"strings"
)

// InputModifier modifies transaction input data
type InputModifier struct {
	contractABI     abi.ABI
	modifications   map[[4]byte]*FunctionModification
	originalInput   []byte
	originalStorage map[common.Hash]common.Hash
	modStrategy     ModificationStrategy
	stepConfig      *StepMutationConfig // 新增：步长变异配置
}

// StepMutationConfig 步长变异配置
type StepMutationConfig struct {
	IntSteps       []int64  `json:"intSteps"`       // 整数类型步长: [1, 10, 100, -1, -10, -100]
	UintSteps      []uint64 `json:"uintSteps"`      // 无符号整数步长: [1, 10, 100, 1000]
	AddressSteps   []int    `json:"addressSteps"`   // 地址变异步长（修改最后几字节）: [1, 2, 5, -1, -2, -5]
	BytesSteps     []int    `json:"bytesSteps"`     // 字节数组步长: [1, 2, 4, 8]
	StorageSteps   []int64  `json:"storageSteps"`   // 存储值步长: [1, 5, 10, 100, -1, -5, -10, -100]
	EnableNearby   bool     `json:"enableNearby"`   // 是否启用附近值变异
	EnableBoundary bool     `json:"enableBoundary"` // 是否启用边界值
	MaxChanges     int      `json:"maxChanges"`     // 最大同时变异数量
}

// DefaultStepMutationConfig 默认步长变异配置
func DefaultStepMutationConfig() *StepMutationConfig {
	return &StepMutationConfig{
		IntSteps:       []int64{1, 10, 100, 1000, -1, -10, -100, -1000, 5, -5},
		UintSteps:      []uint64{1, 10, 100, 1000, 5, 50, 500},
		AddressSteps:   []int{1, 2, 5, 10, -1, -2, -5, -10},
		BytesSteps:     []int{1, 2, 4, 8, 16},
		StorageSteps:   []int64{1, 10, 100, 1000, -1, -10, -100, -1000, 5, -5, 50, -50},
		EnableNearby:   true,
		EnableBoundary: false, // 禁用边界值，专注于步长变异
		MaxChanges:     3,
	}
}

// FunctionModification represents modifications for a specific function
type FunctionModification struct {
	FunctionName      string                  `json:"functionName"`
	FunctionSignature string                  `json:"functionSignature"`
	ParameterMods     []ParameterModification `json:"parameterMods"`
}

// ParameterModification represents a parameter modification rule
type ParameterModification struct {
	ParameterIndex int         `json:"parameterIndex"`
	ParameterName  string      `json:"parameterName"`
	NewValue       interface{} `json:"newValue"`
	ModType        string      `json:"modType"`   // "step_add", "step_sub", "step_mul", "nearby"
	StepValue      int64       `json:"stepValue"` // 步长值
}

// ModificationStrategy 修改策略
type ModificationStrategy struct {
	InputStrategy   string  `json:"inputStrategy"`   // "step_based", "nearby_values", "boundary_values"
	StorageStrategy string  `json:"storageStrategy"` // "step_incremental", "step_proportional", "nearby_original"
	Aggressiveness  float64 `json:"aggressiveness"`  // 0.0-1.0, 修改的激进程度
	MaxChanges      int     `json:"maxChanges"`      // 最大修改数量
}

// SmartModificationSet 智能修改集合
type SmartModificationSet struct {
	Variations []ModificationVariation `json:"variations"`
	Strategy   ModificationStrategy    `json:"strategy"`
	Original   *OriginalState          `json:"original"`
}

// OriginalState 原始状态
type OriginalState struct {
	InputData []byte                      `json:"inputData"`
	Storage   map[common.Hash]common.Hash `json:"storage"`
	Function  *abi.Method                 `json:"function"`
	Args      []interface{}               `json:"args"`
}

// NewInputModifier creates a new input modifier from binding metadata
func NewInputModifier(metaData *bind.MetaData) (*InputModifier, error) {
	contractABI, err := metaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("failed to get ABI: %v", err)
	}

	return &InputModifier{
		contractABI:   *contractABI,
		modifications: make(map[[4]byte]*FunctionModification),
		modStrategy: ModificationStrategy{
			InputStrategy:   "step_based",
			StorageStrategy: "step_incremental",
			Aggressiveness:  0.3,
			MaxChanges:      5,
		},
		stepConfig: DefaultStepMutationConfig(), // 新增：默认步长配置
	}, nil
}


// SetOriginalState 设置原始状态用于智能修改
func (m *InputModifier) SetOriginalState(inputData []byte, storage map[common.Hash]common.Hash) error {
	m.originalInput = inputData
	m.originalStorage = storage

	if len(inputData) >= 4 {
		// 解析原始函数调用
		var selector [4]byte
		copy(selector[:], inputData[:4])

		for _, method := range m.contractABI.Methods {
			if len(method.ID) >= 4 && bytesEqual(method.ID[:4], selector[:]) {
				args, err := method.Inputs.Unpack(inputData[4:])
				if err == nil {
					fmt.Printf("Parsed original function: %s with %d args\n", method.Name, len(args))
					for i, arg := range args {
						fmt.Printf("  Arg[%d] (%s): %v\n", i, method.Inputs[i].Type.String(), arg)
					}
				}
				break
			}
		}
	}

	return nil
}


// generateStepBasedInputModification 生成基于步长的输入修改
func (m *InputModifier) generateStepBasedInputModification(strategy string, variant int) (*InputModification, error) {
	if len(m.originalInput) < 4 {
		return nil, fmt.Errorf("invalid input data")
	}

	var selector [4]byte
	copy(selector[:], m.originalInput[:4])

	var method *abi.Method
	for _, methodItem := range m.contractABI.Methods {
		if len(methodItem.ID) >= 4 && bytesEqual(methodItem.ID[:4], selector[:]) {
			method = &methodItem
			break
		}
	}

	if method == nil {
		return nil, fmt.Errorf("method not found")
	}

	// 解析原始参数
	originalArgs, err := method.Inputs.Unpack(m.originalInput[4:])
	if err != nil {
		return nil, fmt.Errorf("failed to unpack args: %v", err)
	}

	// 根据策略修改参数（使用步长变异）
	modifiedArgs := make([]interface{}, len(originalArgs))
	copy(modifiedArgs, originalArgs)
	paramChanges := make([]ParameterChange, 0)

	// 确保至少修改一个参数
	hasChanges := false
	for i, arg := range originalArgs {
		newArg, changed := m.modifyArgumentByStepStrategy(arg, method.Inputs[i].Type, strategy, variant)
		if changed {
			paramChanges = append(paramChanges, ParameterChange{
				Index:       i,
				Name:        method.Inputs[i].Name,
				Type:        method.Inputs[i].Type.String(),
				Original:    arg,
				Modified:    newArg,
				ChangeType:  m.determineChangeType(arg, newArg),
				ChangeRatio: m.calculateChangeRatio(arg, newArg),
			})
			modifiedArgs[i] = newArg
			hasChanges = true
			fmt.Printf("🔧 Step-based parameter change: %s[%d] %v -> %v\n",
				method.Inputs[i].Name, i, arg, newArg)
		}
	}

	// 如果没有变化，强制修改第一个参数（使用步长）
	if !hasChanges && len(originalArgs) > 0 {
		firstArg := originalArgs[0]
		modifiedArg := m.forceStepModifyArgument(firstArg, method.Inputs[0].Type, variant)

		paramChanges = append(paramChanges, ParameterChange{
			Index:       0,
			Name:        method.Inputs[0].Name,
			Type:        method.Inputs[0].Type.String(),
			Original:    firstArg,
			Modified:    modifiedArg,
			ChangeType:  "forced_step_modify",
			ChangeRatio: 0.1,
		})
		modifiedArgs[0] = modifiedArg
		fmt.Printf("🔧 Forced step-based parameter change: %s[0] %v -> %v\n",
			method.Inputs[0].Name, firstArg, modifiedArg)
	}

	if len(paramChanges) == 0 {
		return nil, nil // 仍然没有修改
	}

	// 重新打包参数
	packedArgs, err := method.Inputs.Pack(modifiedArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to pack modified args: %v", err)
	}

	modifiedInput := append(selector[:], packedArgs...)

	inputMod := &InputModification{
		OriginalInput:    m.originalInput,
		ModifiedInput:    modifiedInput,
		FunctionSelector: selector,
		FunctionName:     method.Name,
		ParameterChanges: paramChanges,
		ModificationHash: ComputeModificationHash(m.originalInput, modifiedInput),
	}

	return inputMod, nil
}

// generateFallbackStepBasedInputModification 生成后备的基于步长的输入修改
func (m *InputModifier) generateFallbackStepBasedInputModification(variant int) *InputModification {
	if len(m.originalInput) < 4 {
		return nil
	}

	var selector [4]byte
	copy(selector[:], m.originalInput[:4])

	var method *abi.Method
	for _, methodItem := range m.contractABI.Methods {
		if len(methodItem.ID) >= 4 && bytesEqual(methodItem.ID[:4], selector[:]) {
			method = &methodItem
			break
		}
	}

	if method == nil {
		return nil
	}

	// 创建一个基于步长的修改：对输入数据的特定字节位置应用步长变异
	modifiedInput := make([]byte, len(m.originalInput))
	copy(modifiedInput, m.originalInput)

	// 选择步长
	stepIndex := variant % len(m.stepConfig.BytesSteps)
	step := m.stepConfig.BytesSteps[stepIndex]

	// 对4字节之后的数据应用步长变异
	if len(modifiedInput) > 4 {
		paramData := modifiedInput[4:]
		m.applyStepMutationToParameterBytes(paramData, step, variant)
	}

	// 创建一个虚拟的参数变化
	paramChanges := []ParameterChange{
		{
			Index:       0,
			Name:        "fallback_step_param",
			Type:        "bytes",
			Original:    m.originalInput,
			Modified:    modifiedInput,
			ChangeType:  "step_based_fallback",
			ChangeRatio: 0.1,
		},
	}

	return &InputModification{
		OriginalInput:    m.originalInput,
		ModifiedInput:    modifiedInput,
		FunctionSelector: selector,
		FunctionName:     method.Name,
		ParameterChanges: paramChanges,
		ModificationHash: ComputeModificationHash(m.originalInput, modifiedInput),
	}
}

// generateFallbackStepBasedStorageModification 生成后备的基于步长的存储修改
func (m *InputModifier) generateFallbackStepBasedStorageModification(variant int) *StorageModification {
	changes := make([]StorageSlotChange, 0)

	// 选择步长
	stepIndex := variant % len(m.stepConfig.StorageSteps)
	step := m.stepConfig.StorageSteps[stepIndex]

	// 如果有原始存储，对其应用步长修改
	if m.originalStorage != nil && len(m.originalStorage) > 0 {
		count := 0
		for slot, originalValue := range m.originalStorage {
			// 应用步长变异
			newValue := m.applyStepToStorageValue(originalValue, step)

			change := StorageSlotChange{
				Slot:        slot,
				Original:    originalValue,
				Modified:    newValue,
				Delta:       big.NewInt(step),
				ChangeType:  "step_increment",
				ChangeRatio: m.calculateStorageChangeRatio(originalValue, newValue),
				SlotType:    ExtractSlotType(slot),
			}

			changes = append(changes, change)
			count++
			if count >= m.stepConfig.MaxChanges { // 限制修改数量
				break
			}
		}
	} else {
		// 如果没有原始存储，创建一些基于步长的虚拟存储修改
		for i := 0; i < 2; i++ {
			slot := common.BigToHash(big.NewInt(int64(i)))
			originalValue := common.Hash{}
			stepValue := step * int64(i+1)
			newValue := common.BigToHash(big.NewInt(stepValue))

			change := StorageSlotChange{
				Slot:        slot,
				Original:    originalValue,
				Modified:    newValue,
				Delta:       big.NewInt(stepValue),
				ChangeType:  "step_set",
				ChangeRatio: 1.0,
				SlotType:    "simple",
			}

			changes = append(changes, change)
		}
	}

	if len(changes) == 0 {
		return nil
	}

	// 计算修改哈希
	hashData := make([]byte, 0)
	for _, change := range changes {
		hashData = append(hashData, change.Slot.Bytes()...)
		hashData = append(hashData, change.Modified.Bytes()...)
	}

	return &StorageModification{
		Changes:          changes,
		ModificationHash: crypto.Keccak256Hash(hashData),
	}
}

// forceStepModifyArgument 强制使用步长修改参数
func (m *InputModifier) forceStepModifyArgument(arg interface{}, argType abi.Type, variant int) interface{} {
	// 根据变异索引选择步长
	switch argType.T {
	case abi.UintTy:
		if val, ok := arg.(*big.Int); ok && val != nil {
			stepIndex := variant % len(m.stepConfig.UintSteps)
			step := m.stepConfig.UintSteps[stepIndex]
			return new(big.Int).Add(val, new(big.Int).SetUint64(step))
		}
		if val, ok := arg.(uint8); ok {
			stepIndex := variant % len(m.stepConfig.UintSteps)
			step := m.stepConfig.UintSteps[stepIndex]
			if step <= 255 {
				newVal := uint64(val) + step
				if newVal > 255 {
					newVal = 255
				}
				return uint8(newVal)
			}
			return val + 1
		}
	case abi.IntTy:
		if val, ok := arg.(*big.Int); ok && val != nil {
			stepIndex := variant % len(m.stepConfig.IntSteps)
			step := m.stepConfig.IntSteps[stepIndex]
			return new(big.Int).Add(val, big.NewInt(step))
		}
		if val, ok := arg.(int8); ok {
			stepIndex := variant % len(m.stepConfig.IntSteps)
			step := m.stepConfig.IntSteps[stepIndex]
			newVal := int64(val) + step
			if newVal > 127 {
				newVal = 127
			} else if newVal < -128 {
				newVal = -128
			}
			return int8(newVal)
		}
	case abi.BoolTy:
		if val, ok := arg.(bool); ok {
			return !val
		}
	case abi.StringTy:
		if val, ok := arg.(string); ok {
			return val + "_step_modified"
		}
	case abi.AddressTy:
		if val, ok := arg.(common.Address); ok {
			newAddr := val
			stepIndex := variant % len(m.stepConfig.AddressSteps)
			step := m.stepConfig.AddressSteps[stepIndex]
			// 修改地址的最后字节
			newAddr[19] = byte((int(newAddr[19]) + step) % 256)
			return newAddr
		}
	}
	return arg
}

// calculateChangeRatio 计算变化比例
func (m *InputModifier) calculateChangeRatio(original, modified interface{}) float64 {
	switch orig := original.(type) {
	case *big.Int:
		if mod, ok := modified.(*big.Int); ok {
			if orig.Sign() == 0 {
				return 1.0
			}
			delta := new(big.Int).Sub(mod, orig)
			ratio := new(big.Float).Quo(new(big.Float).SetInt(delta), new(big.Float).SetInt(orig))
			result, _ := ratio.Float64()
			return result
		}
	case uint8:
		if mod, ok := modified.(uint8); ok {
			if orig == 0 {
				return 1.0
			}
			return float64(int(mod)-int(orig)) / float64(orig)
		}
	case bool:
		return 1.0 // 布尔值变化视为100%变化
	case string:
		if mod, ok := modified.(string); ok {
			if len(orig) == 0 {
				return 1.0
			}
			return float64(len(mod)-len(orig)) / float64(len(orig))
		}
	}
	return 0.1 // 默认变化比例
}

// modifyArgumentByStepStrategy 根据步长策略修改参数
func (m *InputModifier) modifyArgumentByStepStrategy(arg interface{}, argType abi.Type, strategy string, variant int) (interface{}, bool) {
	switch strategy {
	case "step_based":
		return m.modifyWithSteps(arg, argType, variant)
	case "nearby_values":
		return m.modifyNearbyWithSteps(arg, argType, variant)
	case "boundary_values":
		if m.stepConfig.EnableBoundary {
			return m.modifyWithBoundaryValues(arg, argType, variant)
		}
		return m.modifyWithSteps(arg, argType, variant)
	default:
		return m.modifyWithSteps(arg, argType, variant)
	}
}

// modifyWithSteps 使用步长进行修改
func (m *InputModifier) modifyWithSteps(arg interface{}, argType abi.Type, variant int) (interface{}, bool) {
	switch argType.T {
	case abi.UintTy:
		if val, ok := arg.(*big.Int); ok && val != nil {
			stepIndex := variant % len(m.stepConfig.UintSteps)
			step := m.stepConfig.UintSteps[stepIndex]
			newVal := new(big.Int).Add(val, new(big.Int).SetUint64(step))
			return newVal, true
		}
		if val, ok := arg.(uint8); ok {
			stepIndex := variant % len(m.stepConfig.UintSteps)
			step := m.stepConfig.UintSteps[stepIndex]
			if step <= 255 {
				newVal := uint64(val) + step
				if newVal > 255 {
					newVal = 255
				}
				return uint8(newVal), true
			}
		}

	case abi.IntTy:
		if val, ok := arg.(*big.Int); ok && val != nil {
			stepIndex := variant % len(m.stepConfig.IntSteps)
			step := m.stepConfig.IntSteps[stepIndex]
			newVal := new(big.Int).Add(val, big.NewInt(step))
			return newVal, true
		}
		if val, ok := arg.(int8); ok {
			stepIndex := variant % len(m.stepConfig.IntSteps)
			step := m.stepConfig.IntSteps[stepIndex]
			newVal := int64(val) + step
			if newVal > 127 {
				newVal = 127
			} else if newVal < -128 {
				newVal = -128
			}
			return int8(newVal), true
		}

	case abi.BoolTy:
		if val, ok := arg.(bool); ok {
			return !val, true
		}

	case abi.StringTy:
		if val, ok := arg.(string); ok {
			return val + "_step", true
		}

	case abi.AddressTy:
		if val, ok := arg.(common.Address); ok {
			stepIndex := variant % len(m.stepConfig.AddressSteps)
			step := m.stepConfig.AddressSteps[stepIndex]
			newAddr := val
			// 修改地址的最后几个字节
			for i := 0; i < 2 && i+step < 20; i++ {
				byteIndex := 19 - i
				newAddr[byteIndex] = byte((int(newAddr[byteIndex]) + step) % 256)
			}
			return newAddr, true
		}
	}

	return arg, false
}

// modifyNearbyWithSteps 在原始值附近使用步长修改
func (m *InputModifier) modifyNearbyWithSteps(arg interface{}, argType abi.Type, variant int) (interface{}, bool) {
	if !m.stepConfig.EnableNearby {
		return m.modifyWithSteps(arg, argType, variant)
	}

	switch argType.T {
	case abi.UintTy:
		if val, ok := arg.(*big.Int); ok && val != nil {
			stepIndex := variant % len(m.stepConfig.UintSteps)
			step := m.stepConfig.UintSteps[stepIndex]

			// 在附近进行小幅度变异
			if variant%2 == 0 {
				return new(big.Int).Add(val, new(big.Int).SetUint64(step)), true
			} else {
				result := new(big.Int).Sub(val, new(big.Int).SetUint64(step))
				if result.Sign() < 0 {
					result = big.NewInt(0)
				}
				return result, true
			}
		}

	case abi.IntTy:
		if val, ok := arg.(*big.Int); ok && val != nil {
			stepIndex := variant % len(m.stepConfig.IntSteps)
			step := m.stepConfig.IntSteps[stepIndex]
			// 交替加减
			if variant%2 == 0 {
				return new(big.Int).Add(val, big.NewInt(step)), true
			} else {
				return new(big.Int).Sub(val, big.NewInt(step)), true
			}
		}
	}

	// 对于其他类型，回退到标准步长修改
	return m.modifyWithSteps(arg, argType, variant)
}

// modifyWithBoundaryValues 使用边界值修改
func (m *InputModifier) modifyWithBoundaryValues(arg interface{}, argType abi.Type, variant int) (interface{}, bool) {
	switch argType.T {
	case abi.UintTy:
		switch argType.Size {
		case 8:
			values := []uint8{0, 1, 127, 255}
			if variant < len(values) {
				return values[variant], true
			}
		case 256:
			values := []*big.Int{
				big.NewInt(0),
				big.NewInt(1),
				new(big.Int).Exp(big.NewInt(2), big.NewInt(128), nil),
				new(big.Int).Sub(new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil), big.NewInt(1)),
			}
			if variant < len(values) {
				return values[variant], true
			}
		}

	case abi.IntTy:
		switch argType.Size {
		case 8:
			values := []int8{-128, -1, 0, 1, 127}
			if variant < len(values) {
				return values[variant], true
			}
		case 256:
			values := []*big.Int{
				new(big.Int).Neg(new(big.Int).Exp(big.NewInt(2), big.NewInt(255), nil)),
				big.NewInt(-1),
				big.NewInt(0),
				big.NewInt(1),
				new(big.Int).Sub(new(big.Int).Exp(big.NewInt(2), big.NewInt(255), nil), big.NewInt(1)),
			}
			if variant < len(values) {
				return values[variant], true
			}
		}

	case abi.BoolTy:
		return variant%2 == 0, true

	case abi.StringTy:
		values := []string{"", "a", strings.Repeat("a", 1000)}
		if variant < len(values) {
			return values[variant], true
		}
	}

	return arg, false
}

// generateStepBasedStorageModification 生成基于步长的存储修改
func (m *InputModifier) generateStepBasedStorageModification(strategy string, variant int) *StorageModification {
	changes := make([]StorageSlotChange, 0)

	if m.originalStorage != nil && len(m.originalStorage) > 0 {
		// 选择步长
		stepIndex := variant % len(m.stepConfig.StorageSteps)
		step := m.stepConfig.StorageSteps[stepIndex]

		fmt.Printf("💾 Applying step-based storage modification: step=%d, strategy=%s\n", step, strategy)

		// 只修改原始存储中已有的槽
		count := 0
		for slot, originalValue := range m.originalStorage {
			if count >= m.stepConfig.MaxChanges {
				break
			}

			newValue := m.modifyStorageByStepStrategy(originalValue, slot, strategy, step, variant)
			if newValue != originalValue {
				originalBig := originalValue.Big()
				modifiedBig := newValue.Big()

				change := StorageSlotChange{
					Slot:        slot,
					Original:    originalValue,
					Modified:    newValue,
					Delta:       new(big.Int).Sub(modifiedBig, originalBig),
					ChangeType:  DetermineChangeType(originalBig, modifiedBig),
					ChangeRatio: CalculateChangeRatio(originalBig, modifiedBig),
					SlotType:    ExtractSlotType(slot),
				}

				changes = append(changes, change)
				count++
				fmt.Printf("   Step-modified slot %s: %s -> %s (step: %d)\n",
					slot.Hex()[:10]+"...", originalValue.Hex()[:10]+"...", newValue.Hex()[:10]+"...", step)
			}
		}
	}

	if len(changes) == 0 {
		return nil
	}

	storageMod := &StorageModification{
		Changes: changes,
	}

	// 计算修改哈希
	hashData := make([]byte, 0)
	for _, change := range changes {
		hashData = append(hashData, change.Slot.Bytes()...)
		hashData = append(hashData, change.Modified.Bytes()...)
	}
	storageMod.ModificationHash = crypto.Keccak256Hash(hashData)

	return storageMod
}

// modifyStorageByStepStrategy 根据步长策略修改存储值
func (m *InputModifier) modifyStorageByStepStrategy(original common.Hash, slot common.Hash, strategy string, step int64, variant int) common.Hash {
	originalBig := original.Big()

	switch strategy {
	case "step_based":
		// 直接应用步长
		return m.applyStepToStorageValue(original, step)

	case "nearby_values":
		if m.stepConfig.EnableNearby {
			// 在原值附近使用小步长
			smallStep := step / 10
			if smallStep == 0 {
				smallStep = 1
			}
			if variant%2 == 0 {
				return common.BigToHash(new(big.Int).Add(originalBig, big.NewInt(smallStep)))
			} else {
				result := new(big.Int).Sub(originalBig, big.NewInt(smallStep))
				if result.Sign() < 0 {
					result = big.NewInt(0)
				}
				return common.BigToHash(result)
			}
		}
		return m.applyStepToStorageValue(original, step)

	case "boundary_values":
		if m.stepConfig.EnableBoundary {
			values := []*big.Int{
				big.NewInt(0),
				big.NewInt(1),
				big.NewInt(0xffffffff),
				new(big.Int).Sub(new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil), big.NewInt(1)),
			}
			if variant < len(values) {
				return common.BigToHash(values[variant])
			}
		}
		return m.applyStepToStorageValue(original, step)

	default:
		return m.applyStepToStorageValue(original, step)
	}
}

// applyStepToStorageValue 对存储值应用步长
func (m *InputModifier) applyStepToStorageValue(original common.Hash, step int64) common.Hash {
	originalBig := original.Big()

	if step > 0 {
		// 正步长：加法
		return common.BigToHash(new(big.Int).Add(originalBig, big.NewInt(step)))
	} else if step < 0 {
		// 负步长：减法（确保不为负）
		result := new(big.Int).Sub(originalBig, big.NewInt(-step))
		if result.Sign() < 0 {
			result = big.NewInt(0)
		}
		return common.BigToHash(result)
	}

	return original // step为0时不修改
}

// calculateStorageChangeRatio 计算存储变化比例
func (m *InputModifier) calculateStorageChangeRatio(original, modified common.Hash) float64 {
	originalBig := original.Big()
	modifiedBig := modified.Big()

	if originalBig.Sign() == 0 {
		if modifiedBig.Sign() == 0 {
			return 0.0
		}
		return 1.0
	}

	delta := new(big.Int).Sub(modifiedBig, originalBig)
	ratio := new(big.Float).Quo(new(big.Float).SetInt(delta), new(big.Float).SetInt(originalBig))
	result, _ := ratio.Float64()
	return result
}

// applyStepMutationToParameterBytes 对参数字节应用步长变异
func (m *InputModifier) applyStepMutationToParameterBytes(data []byte, step int, variant int) {
	if len(data) == 0 {
		return
	}

	// 根据数据长度和变异类型选择修改策略
	switch variant % 3 {
	case 0: // 修改整个32字节块（如果足够长）
		if len(data) >= 32 {
			// 将前32字节作为big.Int处理
			value := new(big.Int).SetBytes(data[:32])
			newValue := new(big.Int).Add(value, big.NewInt(int64(step)))

			// 将结果写回，保持32字节长度
			newBytes := newValue.Bytes()
			// 清零前32字节
			for i := 0; i < 32; i++ {
				data[i] = 0
			}
			// 复制新值，右对齐
			copy(data[32-len(newBytes):32], newBytes)
		}
	case 1: // 修改特定位置的字节
		byteIndex := variant % len(data)
		newByte := (int(data[byteIndex]) + step) % 256
		if newByte < 0 {
			newByte = 256 + newByte
		}
		data[byteIndex] = byte(newByte)
	case 2: // 修改多个字节位置
		maxChanges := 3
		if len(data) < maxChanges {
			maxChanges = len(data)
		}

		for i := 0; i < maxChanges; i++ {
			byteIndex := (variant + i) % len(data)
			newByte := (int(data[byteIndex]) + step) % 256
			if newByte < 0 {
				newByte = 256 + newByte
			}
			data[byteIndex] = byte(newByte)
		}
	}
}

// 辅助函数
func (m *InputModifier) determineChangeType(original, modified interface{}) string {
	origValue := reflect.ValueOf(original)
	modValue := reflect.ValueOf(modified)

	if origValue.Type() != modValue.Type() {
		return "type_change"
	}

	switch origValue.Kind() {
	case reflect.String:
		if origValue.String() == modValue.String() {
			return "unchanged"
		}
		return "step_replace"
	case reflect.Bool:
		return "toggle"
	default:
		return "step_modify"
	}
}

func (m *InputModifier) getExpectedImpact(strategy string) string {
	switch strategy {
	case "step_based":
		return "step_based_behavior_change"
	case "nearby_values":
		return "nearby_value_exploitation"
	case "boundary_values":
		return "boundary_value_exploitation"
	default:
		return "unknown_step_based"
	}
}

// bytesEqual 比较字节数组是否相等
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
