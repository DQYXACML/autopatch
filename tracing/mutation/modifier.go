package mutation

import (
	"bytes"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"math/big"
	"reflect"
	"strings"
	abiPkg "github.com/DQYXACML/autopatch/tracing/abi"
	"github.com/DQYXACML/autopatch/tracing/utils"
)

// InputModifier modifies transaction input data
type InputModifier struct {
	contractABI      abi.ABI
	modifications    map[[4]byte]*FunctionModification
	originalInput    []byte
	originalStorage  map[common.Hash]common.Hash
	modStrategy      ModificationStrategy
	stepConfig       *StepMutationConfig // 新增：步长变异配置
	
	// 新增：ABI增强功能
	abiManager       *abiPkg.ABIManager
	typeAwareMutator *TypeAwareMutator
	chainID          *big.Int
	contractAddr     *common.Address  // 修改为指针类型
	enableTypeAware  bool             // 修改字段名
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
	Variations []utils.ModificationVariation `json:"variations"`
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
		stepConfig:      DefaultStepMutationConfig(), // 新增：默认步长配置
		enableTypeAware: false,                       // 默认不启用类型感知
	}, nil
}

// NewInputModifierWithABI creates a new input modifier with ABI enhancement
func NewInputModifierWithABI(
	metaData *bind.MetaData,
	abiManager *abiPkg.ABIManager,
	typeAwareMutator *TypeAwareMutator,
	chainID *big.Int,
	contractAddr common.Address,
) (*InputModifier, error) {
	contractABI, err := metaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("failed to get ABI: %v", err)
	}

	return &InputModifier{
		contractABI:   *contractABI,
		modifications: make(map[[4]byte]*FunctionModification),
		modStrategy: ModificationStrategy{
			InputStrategy:   "type_aware",
			StorageStrategy: "step_incremental",
			Aggressiveness:  0.3,
			MaxChanges:      5,
		},
		stepConfig:       DefaultStepMutationConfig(),
		abiManager:       abiManager,
		typeAwareMutator: typeAwareMutator,
		chainID:          chainID,
		contractAddr:     &contractAddr,
		enableTypeAware:  true,
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
func (m *InputModifier) generateStepBasedInputModification(strategy string, variant int) (*utils.InputModification, error) {
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
	paramChanges := make([]utils.ParameterChange, 0)

	// 确保至少修改一个参数
	hasChanges := false
	for i, arg := range originalArgs {
		newArg, changed := m.modifyArgumentByStepStrategy(arg, method.Inputs[i].Type, strategy, variant)
		if changed {
			paramChanges = append(paramChanges, utils.ParameterChange{
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

		paramChanges = append(paramChanges, utils.ParameterChange{
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

	inputMod := &utils.InputModification{
		OriginalInput:    m.originalInput,
		ModifiedInput:    modifiedInput,
		FunctionSelector: selector,
		FunctionName:     method.Name,
		ParameterChanges: paramChanges,
		ModificationHash: utils.ComputeModificationHash(m.originalInput, modifiedInput),
	}

	return inputMod, nil
}

// generateFallbackStepBasedInputModification 生成后备的基于步长的输入修改
func (m *InputModifier) generateFallbackStepBasedInputModification(variant int) *utils.InputModification {
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
	paramChanges := []utils.ParameterChange{
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

	return &utils.InputModification{
		OriginalInput:    m.originalInput,
		ModifiedInput:    modifiedInput,
		FunctionSelector: selector,
		FunctionName:     method.Name,
		ParameterChanges: paramChanges,
		ModificationHash: utils.ComputeModificationHash(m.originalInput, modifiedInput),
	}
}

// generateFallbackStepBasedStorageModification 生成后备的基于步长的存储修改
func (m *InputModifier) generateFallbackStepBasedStorageModification(variant int) *utils.StorageModification {
	changes := make([]utils.StorageSlotChange, 0)

	// 选择步长
	stepIndex := variant % len(m.stepConfig.StorageSteps)
	step := m.stepConfig.StorageSteps[stepIndex]

	// 如果有原始存储，对其应用步长修改
	if m.originalStorage != nil && len(m.originalStorage) > 0 {
		count := 0
		for slot, originalValue := range m.originalStorage {
			// 应用步长变异
			newValue := m.applyStepToStorageValue(originalValue, step)

			change := utils.StorageSlotChange{
				Slot:        slot,
				Original:    originalValue,
				Modified:    newValue,
				Delta:       big.NewInt(step),
				ChangeType:  "step_increment",
				ChangeRatio: m.calculateStorageChangeRatio(originalValue, newValue),
				SlotType:    utils.ExtractSlotType(slot),
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

			change := utils.StorageSlotChange{
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

	return &utils.StorageModification{
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
	// 如果启用了类型感知变异，优先使用
	if m.enableTypeAware && m.typeAwareMutator != nil {
		if strategy == "type_aware" || strategy == "step_based" {
			modified, err := m.typeAwareMutator.MutateByType(argType, arg, variant)
			if err == nil && !isEqual(modified, arg) {
				fmt.Printf("🔧 Type-aware mutation: %s %v -> %v\n", argType.String(), arg, modified)
				return modified, true
			}
			fmt.Printf("⚠️  Type-aware mutation failed: %v, falling back to step-based\n", err)
		}
	}

	// 回退到原有的步长策略
	switch strategy {
	case "step_based", "type_aware":
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
func (m *InputModifier) generateStepBasedStorageModification(strategy string, variant int) *utils.StorageModification {
	changes := make([]utils.StorageSlotChange, 0)

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

				change := utils.StorageSlotChange{
					Slot:        slot,
					Original:    originalValue,
					Modified:    newValue,
					Delta:       new(big.Int).Sub(modifiedBig, originalBig),
					ChangeType:  utils.DetermineChangeType(originalBig, modifiedBig),
					ChangeRatio: utils.CalculateChangeRatio(originalBig, modifiedBig),
					SlotType:    utils.ExtractSlotType(slot),
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

	storageMod := &utils.StorageModification{
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

// EnableTypeAwareMutation 启用类型感知变异
func (m *InputModifier) EnableTypeAwareMutation(
	abiManager *abiPkg.ABIManager,
	typeAwareMutator *TypeAwareMutator,
	chainID *big.Int,
	contractAddr common.Address,
) {
	m.abiManager = abiManager
	m.typeAwareMutator = typeAwareMutator
	m.chainID = chainID
	m.contractAddr = &contractAddr
	m.enableTypeAware = true
	m.modStrategy.InputStrategy = "type_aware"
	
	fmt.Printf("✅ Type-aware mutation enabled for contract %s on chain %s\n", 
		contractAddr.Hex(), chainID.String())
}

// DisableTypeAwareMutation 禁用类型感知变异
func (m *InputModifier) DisableTypeAwareMutation() {
	m.enableTypeAware = false
	m.modStrategy.InputStrategy = "step_based"
	fmt.Printf("⚠️  Type-aware mutation disabled, falling back to step-based\n")
}

// RefreshContractABI 刷新合约ABI（从区块浏览器重新获取）
func (m *InputModifier) RefreshContractABI() error {
	if !m.enableTypeAware || m.abiManager == nil {
		return fmt.Errorf("type-aware mutation not enabled")
	}

	newABI, err := m.abiManager.GetContractABI(m.chainID, *m.contractAddr)
	if err != nil {
		return fmt.Errorf("failed to refresh ABI: %v", err)
	}

	m.contractABI = *newABI
	fmt.Printf("✅ Contract ABI refreshed for %s\n", m.contractAddr.Hex())
	return nil
}

// AnalyzeParameterImportance 分析参数重要性
func (m *InputModifier) AnalyzeParameterImportance(methodName string) ([]ParameterImportance, error) {
	method, exists := m.contractABI.Methods[methodName]
	if !exists {
		return nil, fmt.Errorf("method %s not found in ABI", methodName)
	}

	importance := make([]ParameterImportance, len(method.Inputs))
	
	for i, input := range method.Inputs {
		score := m.calculateParameterImportance(input)
		importance[i] = ParameterImportance{
			ParamIndex:      i,
			ParamName:       input.Name,
			ParamType:       input.Type.String(),
			ImportanceScore: score,
		}
	}

	// 按重要性排序
	for i := 0; i < len(importance)-1; i++ {
		for j := i + 1; j < len(importance); j++ {
			if importance[i].ImportanceScore < importance[j].ImportanceScore {
				importance[i], importance[j] = importance[j], importance[i]
			}
		}
	}

	return importance, nil
}

// calculateParameterImportance 计算单个参数的重要性分数
func (m *InputModifier) calculateParameterImportance(input abi.Argument) float64 {
	score := 0.5 // 基础分数

	// 根据参数名称判断重要性
	name := strings.ToLower(input.Name)
	switch {
	case strings.Contains(name, "amount") || strings.Contains(name, "value"):
		score = 0.95 // 金额参数最重要
	case strings.Contains(name, "to") || strings.Contains(name, "recipient"):
		score = 0.9 // 接收者地址很重要
	case strings.Contains(name, "token") || strings.Contains(name, "asset"):
		score = 0.85 // 代币地址很重要
	case strings.Contains(name, "deadline") || strings.Contains(name, "timestamp"):
		score = 0.8 // 时间参数重要
	case strings.Contains(name, "fee") || strings.Contains(name, "slippage"):
		score = 0.75 // 费用相关参数
	case strings.Contains(name, "enable") || strings.Contains(name, "allow"):
		score = 0.7 // 权限控制参数
	}

	// 根据类型调整重要性
	switch input.Type.T {
	case abi.AddressTy:
		score += 0.1 // 地址通常很重要
	case abi.UintTy:
		if input.Type.Size >= 256 {
			score += 0.05 // 大整数可能是金额
		}
	case abi.BoolTy:
		score += 0.02 // 布尔值相对简单
	}

	// 确保分数在0-1范围内
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}

	return score
}

// GetOptimalMutationStrategy 获取最优变异策略
func (m *InputModifier) GetOptimalMutationStrategy(inputData []byte) (string, error) {
	if len(inputData) < 4 {
		return "generic", nil
	}

	if m.enableTypeAware && m.typeAwareMutator != nil {
		// 解析函数选择器
		var selector [4]byte
		copy(selector[:], inputData[:4])

		// 查找对应的方法
		for _, method := range m.contractABI.Methods {
			if len(method.ID) >= 4 && bytesEqual(method.ID[:4], selector[:]) {
				// 分析参数复杂度
				complexity := m.analyzeParameterComplexity(method.Inputs)
				
				if complexity > 0.7 {
					return "type_aware", nil
				} else if complexity > 0.4 {
					return "step_based", nil
				} else {
					return "nearby_values", nil
				}
			}
		}
	}

	return "step_based", nil
}

// analyzeParameterComplexity 分析参数复杂度
func (m *InputModifier) analyzeParameterComplexity(inputs abi.Arguments) float64 {
	if len(inputs) == 0 {
		return 0.0
	}

	totalComplexity := 0.0
	for _, input := range inputs {
		complexity := 0.0
		
		switch input.Type.T {
		case abi.AddressTy:
			complexity = 0.8
		case abi.UintTy, abi.IntTy:
			complexity = 0.6
		case abi.StringTy, abi.BytesTy:
			complexity = 0.7
		case abi.ArrayTy, abi.SliceTy:
			complexity = 0.9
		case abi.BoolTy:
			complexity = 0.3
		default:
			complexity = 0.5
		}
		
		totalComplexity += complexity
	}

	return totalComplexity / float64(len(inputs))
}

// ParameterImportance 参数重要性分析结果
type ParameterImportance struct {
	ParamIndex      int     `json:"paramIndex"`
	ParamName       string  `json:"paramName"`
	ParamType       string  `json:"paramType"`
	ImportanceScore float64 `json:"importanceScore"`
}

// isEqual 检查两个值是否相等
func isEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	
	// 使用reflect进行深度比较
	return reflect.DeepEqual(a, b)
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

// ModifyInputDataByStrategy 根据策略名称变异输入数据
func (m *InputModifier) ModifyInputDataByStrategy(inputData []byte, strategy string, variant int) ([]byte, error) {
	if len(inputData) < 4 {
		return nil, fmt.Errorf("input data too short")
	}
	
	// 如果启用了类型感知变异且有ABI信息，使用类型感知方法
	if m.enableTypeAware && m.abiManager != nil && m.contractAddr != nil {
		return m.modifyInputDataWithStrategy(inputData, strategy, variant)
	}
	
	// 否则使用基础策略
	return m.modifyInputDataBasicStrategy(inputData, strategy, variant)
}

// modifyInputDataWithStrategy 使用类型感知的策略变异
func (m *InputModifier) modifyInputDataWithStrategy(inputData []byte, strategy string, variant int) ([]byte, error) {
	// 解析函数选择器和参数
	if len(inputData) < 4 {
		return nil, fmt.Errorf("input data too short")
	}
	
	functionSelector := inputData[:4]
	paramData := inputData[4:]
	
	// 获取ABI信息
	contractABI, err := m.abiManager.GetContractABI(m.chainID, *m.contractAddr)
	if err != nil {
		// 回退到基础策略
		return m.modifyInputDataBasicStrategy(inputData, strategy, variant)
	}
	
	// 根据函数选择器找到对应的方法
	var targetMethod *abi.Method
	for _, method := range contractABI.Methods {
		if bytes.Equal(method.ID, functionSelector) {
			targetMethod = &method
			break
		}
	}
	
	if targetMethod == nil {
		// 找不到方法，使用基础策略
		return m.modifyInputDataBasicStrategy(inputData, strategy, variant)
	}
	
	// 解析参数
	values, err := targetMethod.Inputs.Unpack(paramData)
	if err != nil {
		// 解析失败，使用基础策略
		return m.modifyInputDataBasicStrategy(inputData, strategy, variant)
	}
	
	// 根据策略变异参数
	mutatedValues, err := m.mutateParametersByStrategy(targetMethod.Inputs, values, strategy, variant)
	if err != nil {
		return nil, fmt.Errorf("failed to mutate parameters: %v", err)
	}
	
	// 重新打包参数
	mutatedParamData, err := targetMethod.Inputs.Pack(mutatedValues...)
	if err != nil {
		return nil, fmt.Errorf("failed to pack mutated parameters: %v", err)
	}
	
	// 组合函数选择器和变异后的参数
	result := make([]byte, 4+len(mutatedParamData))
	copy(result[:4], functionSelector)
	copy(result[4:], mutatedParamData)
	
	return result, nil
}

// modifyInputDataBasicStrategy 基础策略变异
func (m *InputModifier) modifyInputDataBasicStrategy(inputData []byte, strategy string, variant int) ([]byte, error) {
	if len(inputData) < 4 {
		return nil, fmt.Errorf("input data too short")
	}
	
	result := make([]byte, len(inputData))
	copy(result, inputData)
	
	// 跳过函数选择器，只变异参数部分
	// paramData := result[4:] // 暂时不使用，避免编译错误
	
	switch strategy {
	case "address_known_substitution":
		return m.mutateAddressesInParams(result, variant)
	case "address_nearby_mutation":
		return m.mutateAddressesNearby(result, variant)
	case "uint256_boundary_values":
		return m.mutateUintBoundary(result, variant)
	case "uint256_step_increment":
		return m.mutateUintStep(result, variant)
	case "uint256_multiplier":
		return m.mutateUintMultiplier(result, variant)
	case "bool_flip":
		return m.mutateBoolFlip(result, variant)
	case "bytes_pattern_fill":
		return m.mutateBytePatterns(result, variant)
	case "string_length_mutation":
		return m.mutateStringLength(result, variant)
	default:
		// 默认使用现有的变异方法
		return m.ModifyInputDataDefault(inputData, variant)
	}
}

// mutateParametersByStrategy 根据策略变异参数
func (m *InputModifier) mutateParametersByStrategy(
	inputs abi.Arguments,
	values []interface{},
	strategy string,
	variant int,
) ([]interface{}, error) {
	mutatedValues := make([]interface{}, len(values))
	copy(mutatedValues, values)
	
	// 根据策略选择要变异的参数
	targetParamIndex := variant % len(values)
	
	if targetParamIndex >= len(inputs) {
		return mutatedValues, nil
	}
	
	targetArg := inputs[targetParamIndex]
	targetValue := values[targetParamIndex]
	
	// 根据策略和参数类型进行变异
	mutatedValue, err := m.mutateValueByStrategy(targetArg.Type, targetValue, strategy, variant)
	if err != nil {
		return nil, err
	}
	
	mutatedValues[targetParamIndex] = mutatedValue
	return mutatedValues, nil
}

// mutateValueByStrategy 根据策略变异单个值
func (m *InputModifier) mutateValueByStrategy(
	argType abi.Type,
	value interface{},
	strategy string,
	variant int,
) (interface{}, error) {
	// 使用类型感知变异器
	if m.typeAwareMutator != nil {
		return m.typeAwareMutator.MutateByType(argType, value, variant)
	}
	
	// 回退到基础变异
	return m.mutateValueBasic(argType, value, variant)
}

// mutateValueBasic 基础值变异
func (m *InputModifier) mutateValueBasic(argType abi.Type, value interface{}, variant int) (interface{}, error) {
	switch argType.T {
	case abi.AddressTy:
		if addr, ok := value.(common.Address); ok {
			// 简单的地址变异
			newAddr := addr
			newAddr[19] = byte((int(newAddr[19]) + variant) % 256)
			return newAddr, nil
		}
	case abi.UintTy, abi.IntTy:
		if bigInt, ok := value.(*big.Int); ok {
			// 简单的数值变异
			step := big.NewInt(int64(variant + 1))
			result := new(big.Int).Add(bigInt, step)
			return result, nil
		}
	case abi.BoolTy:
		if b, ok := value.(bool); ok {
			return !b, nil
		}
	}
	
	return value, nil
}

// 辅助方法：各种基础策略的实现
func (m *InputModifier) mutateAddressesInParams(inputData []byte, variant int) ([]byte, error) {
	// 实现地址替换策略
	result := make([]byte, len(inputData))
	copy(result, inputData)
	
	// 查找可能的地址位置（32字节对齐，前12字节为0）
	for i := 4; i+32 <= len(result); i += 32 {
		// 检查是否看起来像地址
		isZeroPrefix := true
		for j := 0; j < 12; j++ {
			if result[i+j] != 0 {
				isZeroPrefix = false
				break
			}
		}
		
		if isZeroPrefix {
			// 修改地址
			result[i+31] = byte((int(result[i+31]) + variant) % 256)
		}
	}
	
	return result, nil
}

func (m *InputModifier) mutateAddressesNearby(inputData []byte, variant int) ([]byte, error) {
	result := make([]byte, len(inputData))
	copy(result, inputData)
	
	// 类似于mutateAddressesInParams，但使用不同的变异策略
	for i := 4; i+32 <= len(result); i += 32 {
		isZeroPrefix := true
		for j := 0; j < 12; j++ {
			if result[i+j] != 0 {
				isZeroPrefix = false
				break
			}
		}
		
		if isZeroPrefix {
			// 在范围内随机变异
			range_ := 1000 + variant*100
			result[i+30] = byte((int(result[i+30]) + range_) % 256)
			result[i+31] = byte((int(result[i+31]) + variant) % 256)
		}
	}
	
	return result, nil
}

func (m *InputModifier) mutateUintBoundary(inputData []byte, variant int) ([]byte, error) {
	result := make([]byte, len(inputData))
	copy(result, inputData)
	
	// 边界值变异
	boundaryValues := [][]byte{
		make([]byte, 32), // 0
		{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, // max uint256
	}
	
	boundaryIndex := variant % len(boundaryValues)
	targetParam := (variant / len(boundaryValues)) % ((len(result) - 4) / 32)
	
	if targetParam*32+36 <= len(result) {
		copy(result[4+targetParam*32:4+(targetParam+1)*32], boundaryValues[boundaryIndex])
	}
	
	return result, nil
}

func (m *InputModifier) mutateUintStep(inputData []byte, variant int) ([]byte, error) {
	result := make([]byte, len(inputData))
	copy(result, inputData)
	
	steps := []int64{1, -1, 10, -10, 100, -100, 1000, -1000}
	step := steps[variant%len(steps)]
	targetParam := (variant / len(steps)) % ((len(result) - 4) / 32)
	
	if targetParam*32+36 <= len(result) {
		// 读取当前值
		paramStart := 4 + targetParam*32
		currentValue := new(big.Int).SetBytes(result[paramStart:paramStart+32])
		
		// 应用步长
		newValue := new(big.Int).Add(currentValue, big.NewInt(step))
		if newValue.Sign() < 0 {
			newValue = big.NewInt(0)
		}
		
		// 写回结果
		valueBytes := newValue.Bytes()
		copy(result[paramStart:paramStart+32], make([]byte, 32)) // 清零
		copy(result[paramStart+32-len(valueBytes):paramStart+32], valueBytes)
	}
	
	return result, nil
}

func (m *InputModifier) mutateUintMultiplier(inputData []byte, variant int) ([]byte, error) {
	result := make([]byte, len(inputData))
	copy(result, inputData)
	
	multipliers := []int64{2, 10, 100, 1000}
	multiplier := multipliers[variant%len(multipliers)]
	targetParam := (variant / len(multipliers)) % ((len(result) - 4) / 32)
	
	if targetParam*32+36 <= len(result) {
		paramStart := 4 + targetParam*32
		currentValue := new(big.Int).SetBytes(result[paramStart:paramStart+32])
		
		newValue := new(big.Int).Mul(currentValue, big.NewInt(multiplier))
		
		valueBytes := newValue.Bytes()
		copy(result[paramStart:paramStart+32], make([]byte, 32))
		if len(valueBytes) <= 32 {
			copy(result[paramStart+32-len(valueBytes):paramStart+32], valueBytes)
		}
	}
	
	return result, nil
}

func (m *InputModifier) mutateBoolFlip(inputData []byte, variant int) ([]byte, error) {
	result := make([]byte, len(inputData))
	copy(result, inputData)
	
	targetParam := variant % ((len(result) - 4) / 32)
	
	if targetParam*32+36 <= len(result) {
		paramStart := 4 + targetParam*32
		// 假设布尔值存储在最后一个字节
		if result[paramStart+31] == 0 {
			result[paramStart+31] = 1
		} else {
			result[paramStart+31] = 0
		}
	}
	
	return result, nil
}

func (m *InputModifier) mutateBytePatterns(inputData []byte, variant int) ([]byte, error) {
	result := make([]byte, len(inputData))
	copy(result, inputData)
	
	patterns := []byte{0x00, 0xFF, 0xAA, 0x55, 0xDE, 0xAD, 0xBE, 0xEF}
	pattern := patterns[variant%len(patterns)]
	
	// 填充某个参数位置
	targetParam := (variant / len(patterns)) % ((len(result) - 4) / 32)
	
	if targetParam*32+36 <= len(result) {
		paramStart := 4 + targetParam*32
		for i := paramStart; i < paramStart+32; i++ {
			result[i] = pattern
		}
	}
	
	return result, nil
}

func (m *InputModifier) mutateStringLength(inputData []byte, variant int) ([]byte, error) {
	// 字符串长度变异比较复杂，这里做简化处理
	result := make([]byte, len(inputData))
	copy(result, inputData)
	
	// 假设某个位置存储的是字符串长度
	targetParam := variant % ((len(result) - 4) / 32)
	
	if targetParam*32+36 <= len(result) {
		paramStart := 4 + targetParam*32
		// 修改长度值
		lengthVariations := []int64{0, 1, 32, 64, 1000, 10000}
		newLength := lengthVariations[variant%len(lengthVariations)]
		
		lengthBytes := big.NewInt(newLength).Bytes()
		copy(result[paramStart:paramStart+32], make([]byte, 32))
		copy(result[paramStart+32-len(lengthBytes):paramStart+32], lengthBytes)
	}
	
	return result, nil
}

// ModifyInputDataDefault 默认输入数据变异方法
func (m *InputModifier) ModifyInputDataDefault(inputData []byte, variant int) ([]byte, error) {
	if len(inputData) < 4 {
		return nil, fmt.Errorf("input data too short")
	}
	
	result := make([]byte, len(inputData))
	copy(result, inputData)
	
	// 简单的变异策略：修改参数部分的一些字节
	if len(result) > 4 {
		// 跳过函数选择器，修改参数
		paramStart := 4
		paramLength := len(result) - 4
		
		// 根据variant选择修改位置
		modifyIndex := paramStart + (variant % paramLength)
		
		// 修改字节值
		result[modifyIndex] = byte((int(result[modifyIndex]) + variant + 1) % 256)
		
		// 可能修改多个字节
		if variant > 10 && paramLength > 32 {
			secondIndex := paramStart + ((variant * 7) % paramLength)
			result[secondIndex] = byte((int(result[secondIndex]) + variant*2) % 256)
		}
	}
	
	return result, nil
}
