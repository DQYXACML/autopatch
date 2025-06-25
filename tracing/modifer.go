package tracing

import (
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"math/big"
	"reflect"
	"strings"
)

// AdvancedInputModifier handles both function selector and parameter modifications
type AdvancedInputModifier struct {
	contractABI  abi.ABI
	functionMods map[string]*FunctionModification  // key: function signature
	selectorMods map[[4]byte]*FunctionModification // key: function selector
}

// NewAdvancedInputModifierFromBinding creates a new AdvancedInputModifier from abigen binding
func NewAdvancedInputModifierFromBinding(contractMetaData *bind.MetaData) (*AdvancedInputModifier, error) {
	// 从binding的MetaData获取ABI
	contractABI, err := contractMetaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("failed to get ABI from binding: %v", err)
	}

	if contractABI == nil {
		return nil, fmt.Errorf("ABI is nil")
	}

	return &AdvancedInputModifier{
		contractABI:  *contractABI,
		functionMods: make(map[string]*FunctionModification),
		selectorMods: make(map[[4]byte]*FunctionModification),
	}, nil
}

// NewAdvancedInputModifier 保留原有的方法用于向后兼容
func NewAdvancedInputModifier(abiJSON string) (*AdvancedInputModifier, error) {
	contractABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %v", err)
	}

	return &AdvancedInputModifier{
		contractABI:  contractABI,
		functionMods: make(map[string]*FunctionModification),
		selectorMods: make(map[[4]byte]*FunctionModification),
	}, nil
}

// convertToABIType 根据ABI类型转换值
func (m *AdvancedInputModifier) convertToABIType(value interface{}, abiType abi.Type) (interface{}, error) {
	if value == nil {
		return nil, fmt.Errorf("value is nil")
	}

	fmt.Printf("Converting value %v (type: %T) to ABI type: %s\n", value, value, abiType.String())

	switch abiType.T {
	case abi.IntTy:
		return m.convertToInt(value, abiType.Size)
	case abi.UintTy:
		return m.convertToUint(value, abiType.Size)
	case abi.BoolTy:
		return m.convertToBool(value)
	case abi.StringTy:
		return m.convertToString(value)
	case abi.AddressTy:
		return m.convertToAddress(value)
	case abi.BytesTy:
		return m.convertToBytes(value)
	case abi.FixedBytesTy:
		return m.convertToFixedBytes(value, abiType.Size)
	case abi.SliceTy:
		return m.convertToSlice(value, abiType)
	case abi.ArrayTy:
		return m.convertToArray(value, abiType)
	default:
		// 如果无法转换，返回原值
		fmt.Printf("Warning: unsupported ABI type %s, using original value\n", abiType.String())
		return value, nil
	}
}

// convertToInt 转换为有符号整数
func (m *AdvancedInputModifier) convertToInt(value interface{}, size int) (interface{}, error) {
	var bigIntValue *big.Int

	switch v := value.(type) {
	case *big.Int:
		bigIntValue = v
	case int:
		bigIntValue = big.NewInt(int64(v))
	case int8:
		bigIntValue = big.NewInt(int64(v))
	case int16:
		bigIntValue = big.NewInt(int64(v))
	case int32:
		bigIntValue = big.NewInt(int64(v))
	case int64:
		bigIntValue = big.NewInt(v)
	case uint:
		bigIntValue = big.NewInt(int64(v))
	case uint8:
		bigIntValue = big.NewInt(int64(v))
	case uint16:
		bigIntValue = big.NewInt(int64(v))
	case uint32:
		bigIntValue = big.NewInt(int64(v))
	case uint64:
		bigIntValue = big.NewInt(int64(v))
	case string:
		var ok bool
		bigIntValue, ok = new(big.Int).SetString(v, 0)
		if !ok {
			return nil, fmt.Errorf("cannot parse string %s as integer", v)
		}
	default:
		return nil, fmt.Errorf("cannot convert %T to int%d", value, size)
	}

	// 根据大小返回相应的类型
	switch size {
	case 8:
		if bigIntValue.BitLen() > 7 || bigIntValue.Cmp(big.NewInt(-128)) < 0 || bigIntValue.Cmp(big.NewInt(127)) > 0 {
			return nil, fmt.Errorf("value %s out of range for int8", bigIntValue.String())
		}
		return int8(bigIntValue.Int64()), nil
	case 16:
		if bigIntValue.BitLen() > 15 || bigIntValue.Cmp(big.NewInt(-32768)) < 0 || bigIntValue.Cmp(big.NewInt(32767)) > 0 {
			return nil, fmt.Errorf("value %s out of range for int16", bigIntValue.String())
		}
		return int16(bigIntValue.Int64()), nil
	case 32:
		if bigIntValue.BitLen() > 31 || bigIntValue.Cmp(big.NewInt(-2147483648)) < 0 || bigIntValue.Cmp(big.NewInt(2147483647)) > 0 {
			return nil, fmt.Errorf("value %s out of range for int32", bigIntValue.String())
		}
		return int32(bigIntValue.Int64()), nil
	case 64:
		return bigIntValue.Int64(), nil
	case 256:
		return bigIntValue, nil
	default:
		return nil, fmt.Errorf("unsupported int size: %d", size)
	}
}

// convertToUint 转换为无符号整数
func (m *AdvancedInputModifier) convertToUint(value interface{}, size int) (interface{}, error) {
	var bigIntValue *big.Int

	switch v := value.(type) {
	case *big.Int:
		bigIntValue = v
	case int:
		bigIntValue = big.NewInt(int64(v))
	case int8:
		bigIntValue = big.NewInt(int64(v))
	case int16:
		bigIntValue = big.NewInt(int64(v))
	case int32:
		bigIntValue = big.NewInt(int64(v))
	case int64:
		bigIntValue = big.NewInt(v)
	case uint:
		bigIntValue = big.NewInt(int64(v))
	case uint8:
		bigIntValue = big.NewInt(int64(v))
	case uint16:
		bigIntValue = big.NewInt(int64(v))
	case uint32:
		bigIntValue = big.NewInt(int64(v))
	case uint64:
		bigIntValue = new(big.Int).SetUint64(v)
	case string:
		var ok bool
		bigIntValue, ok = new(big.Int).SetString(v, 0)
		if !ok {
			return nil, fmt.Errorf("cannot parse string %s as integer", v)
		}
	default:
		return nil, fmt.Errorf("cannot convert %T to uint%d", value, size)
	}

	if bigIntValue.Sign() < 0 {
		return nil, fmt.Errorf("negative value %s cannot be converted to uint%d", bigIntValue.String(), size)
	}

	// 根据大小返回相应的类型
	switch size {
	case 8:
		if bigIntValue.Cmp(big.NewInt(255)) > 0 {
			return nil, fmt.Errorf("value %s out of range for uint8", bigIntValue.String())
		}
		return uint8(bigIntValue.Uint64()), nil
	case 16:
		if bigIntValue.Cmp(big.NewInt(65535)) > 0 {
			return nil, fmt.Errorf("value %s out of range for uint16", bigIntValue.String())
		}
		return uint16(bigIntValue.Uint64()), nil
	case 32:
		if bigIntValue.Cmp(big.NewInt(4294967295)) > 0 {
			return nil, fmt.Errorf("value %s out of range for uint32", bigIntValue.String())
		}
		return uint32(bigIntValue.Uint64()), nil
	case 64:
		maxUint64 := new(big.Int).SetUint64(^uint64(0))
		if bigIntValue.Cmp(maxUint64) > 0 {
			return nil, fmt.Errorf("value %s out of range for uint64", bigIntValue.String())
		}
		return bigIntValue.Uint64(), nil
	case 128:
		// uint128 通常用 *big.Int 表示
		maxUint128 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
		if bigIntValue.Cmp(maxUint128) > 0 {
			return nil, fmt.Errorf("value %s out of range for uint128", bigIntValue.String())
		}
		return bigIntValue, nil
	case 256:
		return bigIntValue, nil
	default:
		return nil, fmt.Errorf("unsupported uint size: %d", size)
	}
}

// convertToBool 转换为布尔值
func (m *AdvancedInputModifier) convertToBool(value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return reflect.ValueOf(v).Int() != 0, nil
	case *big.Int:
		return v.Sign() != 0, nil
	case string:
		lower := strings.ToLower(v)
		if lower == "true" || lower == "1" {
			return true, nil
		} else if lower == "false" || lower == "0" {
			return false, nil
		}
		return nil, fmt.Errorf("cannot parse string %s as bool", v)
	default:
		return nil, fmt.Errorf("cannot convert %T to bool", value)
	}
}

// convertToString 转换为字符串
func (m *AdvancedInputModifier) convertToString(value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

// convertToAddress 转换为地址
func (m *AdvancedInputModifier) convertToAddress(value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case common.Address:
		return v, nil
	case string:
		if !common.IsHexAddress(v) {
			return nil, fmt.Errorf("invalid address format: %s", v)
		}
		return common.HexToAddress(v), nil
	case []byte:
		if len(v) != 20 {
			return nil, fmt.Errorf("invalid address length: %d", len(v))
		}
		return common.BytesToAddress(v), nil
	default:
		return nil, fmt.Errorf("cannot convert %T to address", value)
	}
}

// convertToBytes 转换为动态字节数组
func (m *AdvancedInputModifier) convertToBytes(value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case []byte:
		return v, nil
	case string:
		if strings.HasPrefix(v, "0x") {
			return hexutil.Decode(v)
		}
		return []byte(v), nil
	default:
		return nil, fmt.Errorf("cannot convert %T to bytes", value)
	}
}

// convertToFixedBytes 转换为固定长度字节数组
func (m *AdvancedInputModifier) convertToFixedBytes(value interface{}, size int) (interface{}, error) {
	var bytes []byte

	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		if strings.HasPrefix(v, "0x") {
			var err error
			bytes, err = hexutil.Decode(v)
			if err != nil {
				return nil, err
			}
		} else {
			bytes = []byte(v)
		}
	default:
		return nil, fmt.Errorf("cannot convert %T to fixed bytes", value)
	}

	if len(bytes) > size {
		bytes = bytes[:size]
	} else if len(bytes) < size {
		// 右填充零
		padded := make([]byte, size)
		copy(padded, bytes)
		bytes = padded
	}

	// 根据大小返回相应的数组类型
	switch size {
	case 32:
		var result [32]byte
		copy(result[:], bytes)
		return result, nil
	default:
		// 对于其他大小，使用反射创建相应的数组类型
		arrayType := reflect.ArrayOf(size, reflect.TypeOf(byte(0)))
		arrayValue := reflect.New(arrayType).Elem()
		reflect.Copy(arrayValue, reflect.ValueOf(bytes))
		return arrayValue.Interface(), nil
	}
}

// convertToSlice 转换为切片
func (m *AdvancedInputModifier) convertToSlice(value interface{}, abiType abi.Type) (interface{}, error) {
	// 这里需要根据实际需求实现切片转换
	// 目前返回原值
	return value, nil
}

// convertToArray 转换为数组
func (m *AdvancedInputModifier) convertToArray(value interface{}, abiType abi.Type) (interface{}, error) {
	// 这里需要根据实际需求实现数组转换
	// 目前返回原值
	return value, nil
}

func (m *AdvancedInputModifier) AddFunctionModification(mod *FunctionModification) error {
	// 验证函数是否存在于ABI中
	method, exists := m.contractABI.Methods[mod.FunctionName]
	if !exists {
		return fmt.Errorf("function %s not found in ABI", mod.FunctionName)
	}

	// 验证参数修改
	for _, paramMod := range mod.ParameterMods {
		if paramMod.ParameterIndex >= len(method.Inputs) {
			return fmt.Errorf("parameter index %d out of range for function %s",
				paramMod.ParameterIndex, mod.FunctionName)
		}

		// 如果提供了参数名称，验证是否匹配
		if paramMod.ParameterName != "" {
			expectedName := method.Inputs[paramMod.ParameterIndex].Name
			if expectedName != paramMod.ParameterName {
				return fmt.Errorf("parameter name mismatch: expected %s, got %s",
					expectedName, paramMod.ParameterName)
			}
		}
	}

	// 存储修改规则
	signature := mod.FunctionSignature
	if signature == "" {
		signature = method.Sig
	}

	m.functionMods[signature] = mod

	// 同时按选择器存储
	selector := method.ID
	var selectorArray [4]byte
	copy(selectorArray[:], selector[:4])
	m.selectorMods[selectorArray] = mod

	return nil
}

func (m *AdvancedInputModifier) ModifyInput(input []byte) ([]byte, error) {
	fmt.Printf("\n=== INPUT MODIFICATION START ===\n")
	fmt.Printf("Original input length: %d\n", len(input))
	fmt.Printf("Original input data: %x\n", input)

	if len(input) < 4 {
		fmt.Printf("Input too short, returning original\n")
		fmt.Printf("=== INPUT MODIFICATION END ===\n\n")
		return input, nil // 不是函数调用
	}

	// 提取函数选择器
	var selector [4]byte
	copy(selector[:], input[:4])
	fmt.Printf("Function selector: %x\n", selector)

	// 查找修改规则
	mod, exists := m.selectorMods[selector]
	if !exists {
		fmt.Printf("No modification rule found for selector %x\n", selector)
		fmt.Printf("=== INPUT MODIFICATION END ===\n\n")
		return input, nil // 没有修改规则
	}

	fmt.Printf("Found modification rule for function: %s\n", mod.FunctionName)

	// 获取对应的ABI方法
	method, exists := m.contractABI.Methods[mod.FunctionName]
	if !exists {
		return nil, fmt.Errorf("method %s not found in ABI", mod.FunctionName)
	}

	fmt.Printf("Method signature: %s\n", method.Sig)
	fmt.Printf("Method inputs count: %d\n", len(method.Inputs))

	// 解析原始参数
	args, err := method.Inputs.Unpack(input[4:])
	if err != nil {
		return nil, fmt.Errorf("failed to unpack input: %v", err)
	}

	fmt.Printf("Original args for %s: %+v\n", mod.FunctionName, args)
	for i, arg := range args {
		fmt.Printf("  [%d] %s = %v (type: %T)\n", i, method.Inputs[i].Name, arg, arg)
	}

	// 应用参数修改，包含类型转换
	modifiedArgs := make([]interface{}, len(args))
	copy(modifiedArgs, args)

	fmt.Printf("Applying %d parameter modifications:\n", len(mod.ParameterMods))
	for _, paramMod := range mod.ParameterMods {
		if paramMod.ParameterIndex < len(modifiedArgs) {
			oldValue := modifiedArgs[paramMod.ParameterIndex]

			// 获取目标参数的ABI类型
			abiType := method.Inputs[paramMod.ParameterIndex].Type

			// 转换新值到正确的类型
			convertedValue, err := m.convertToABIType(paramMod.NewValue, abiType)
			if err != nil {
				return nil, fmt.Errorf("failed to convert parameter %d: %v", paramMod.ParameterIndex, err)
			}

			modifiedArgs[paramMod.ParameterIndex] = convertedValue

			fmt.Printf("Modified parameter %d (%s): %v (type: %T) -> %v (type: %T)\n",
				paramMod.ParameterIndex,
				method.Inputs[paramMod.ParameterIndex].Name,
				oldValue, oldValue,
				convertedValue, convertedValue)
		}
	}

	// 确定目标函数方法
	var targetMethod abi.Method
	if mod.NewFunctionName != "" {
		// 使用新函数名
		var exists bool
		targetMethod, exists = m.contractABI.Methods[mod.NewFunctionName]
		if !exists {
			return nil, fmt.Errorf("target method %s not found in ABI", mod.NewFunctionName)
		}
		fmt.Printf("Changed function: %s -> %s\n", mod.FunctionName, mod.NewFunctionName)
		fmt.Printf("New function selector: %x\n", targetMethod.ID[:4])
	} else {
		// 使用原函数名
		targetMethod = method
		fmt.Printf("Keeping original function: %s\n", mod.FunctionName)
	}

	// 重新打包参数
	fmt.Printf("Repacking args: %+v\n", modifiedArgs)
	packedArgs, err := targetMethod.Inputs.Pack(modifiedArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to pack modified args: %v", err)
	}

	// 组合新的input data
	result := make([]byte, 4+len(packedArgs))
	copy(result[:4], targetMethod.ID[:4])
	copy(result[4:], packedArgs)

	fmt.Printf("Modified input length: %d\n", len(result))
	fmt.Printf("Modified input data: %x\n", result)
	fmt.Printf("=== INPUT MODIFICATION END ===\n\n")

	return result, nil
}

// InputModifier 保留原有的简单功能以保持向后兼容
// InputModifier modifies transaction input data
type InputModifier struct {
	targetSelector      [4]byte
	replacementSelector [4]byte
}

func NewInputModifier(targetFunc, replacementFunc string) *InputModifier {
	return &InputModifier{
		targetSelector:      getFunctionSelector(targetFunc),
		replacementSelector: getFunctionSelector(replacementFunc),
	}
}

func getFunctionSelector(signature string) [4]byte {
	hash := crypto.Keccak256([]byte(signature))
	var selector [4]byte
	copy(selector[:], hash[:4])
	return selector
}

func (m *InputModifier) ModifyInput(input []byte) []byte {
	if len(input) < 4 {
		return input
	}

	var currentSelector [4]byte
	copy(currentSelector[:], input[:4])

	if currentSelector == m.targetSelector {
		modified := make([]byte, len(input))
		copy(modified, input)
		copy(modified[:4], m.replacementSelector[:])

		fmt.Printf("Modified function selector: %x -> %x\n",
			m.targetSelector, m.replacementSelector)
		return modified
	}

	return input
}
