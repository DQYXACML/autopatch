package tracing

import (
	"math/big"

	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
)

// MutationConfig 步长变异配置
type MutationConfig struct {
	InputSteps   []int64 `json:"inputSteps"`   // 输入数据变异步长: [10, 100, -10, -100, 1000, -1000]
	StorageSteps []int64 `json:"storageSteps"` // 存储值变异步长: [1, 10, 100, -1, -10, -100]
	ByteSteps    []int   `json:"byteSteps"`    // 字节级变异步长: [1, 2, 5, -1, -2, -5]
	MaxMutations int     `json:"maxMutations"` // 每次最大变异数量
	OnlyPrestate bool    `json:"onlyPrestate"` // 是否只修改prestate中已有的存储槽
}

// MutationManager 管理数据变异操作
type MutationManager struct {
	config        *MutationConfig
	inputModifier *InputModifier
}

// NewMutationManager 创建变异管理器
func NewMutationManager(config *MutationConfig, inputModifier *InputModifier) *MutationManager {
	return &MutationManager{
		config:        config,
		inputModifier: inputModifier,
	}
}

// DefaultMutationConfig 默认步长变异配置
func DefaultMutationConfig() *MutationConfig {
	return &MutationConfig{
		InputSteps:   []int64{10, 100, 1000, -10, -100, -1000, 1, -1, 50, -50},
		StorageSteps: []int64{1, 10, 100, 1000, -1, -10, -100, -1000, 5, -5},
		ByteSteps:    []int{1, 2, 5, 10, -1, -2, -5, -10},
		MaxMutations: 3,
		OnlyPrestate: true,
	}
}

// GenerateStepBasedInputData 生成基于步长的输入数据
func (m *MutationManager) GenerateStepBasedInputData(originalInput []byte, variant int) []byte {
	if len(originalInput) < 4 {
		return originalInput
	}

	modifiedInput := make([]byte, len(originalInput))
	copy(modifiedInput, originalInput)

	stepIndex := variant % len(m.config.InputSteps)
	step := m.config.InputSteps[stepIndex]

	m.ApplyStepMutationToBytes(modifiedInput[4:], step, variant)

	return modifiedInput
}

// GenerateStepBasedStorageChanges 生成基于步长的存储变更
func (m *MutationManager) GenerateStepBasedStorageChanges(originalStorage map[gethCommon.Hash]gethCommon.Hash, variant int) map[gethCommon.Hash]gethCommon.Hash {
	if len(originalStorage) == 0 {
		return make(map[gethCommon.Hash]gethCommon.Hash)
	}

	modifiedStorage := make(map[gethCommon.Hash]gethCommon.Hash)

	stepIndex := variant % len(m.config.StorageSteps)
	step := m.config.StorageSteps[stepIndex]

	keyIndex := 0
	maxModifications := (variant % m.config.MaxMutations) + 1

	for key, value := range originalStorage {
		if keyIndex >= maxModifications {
			modifiedStorage[key] = value
		} else {
			newValue := m.GenerateStepBasedStorageValue(value, step, variant+keyIndex)
			modifiedStorage[key] = newValue
		}
		keyIndex++
	}

	return modifiedStorage
}

// GenerateStepBasedStorageValue 生成基于步长的存储值
func (m *MutationManager) GenerateStepBasedStorageValue(original gethCommon.Hash, step int64, variant int) gethCommon.Hash {
	originalInt := new(big.Int).SetBytes(original.Bytes())

	delta := big.NewInt(step)
	if variant%3 == 0 {
		delta.Mul(delta, big.NewInt(int64(variant+1)))
	}

	newValue := new(big.Int).Add(originalInt, delta)

	// 处理溢出
	maxValue, _ := uint256.FromBig(new(big.Int).Lsh(big.NewInt(1), 256))
	maxValue.SubUint64(maxValue, 1)

	if newValue.Sign() < 0 {
		newValue.Add(newValue, maxValue.ToBig())
	}

	if newValue.BitLen() > 256 {
		newValue.Mod(newValue, maxValue.ToBig())
	}

	return gethCommon.BigToHash(newValue)
}

// ApplyStepMutationToBytes 应用步长变异到字节数据
func (m *MutationManager) ApplyStepMutationToBytes(data []byte, step int64, variant int) {
	if len(data) == 0 {
		return
	}

	byteStepIndex := variant % len(m.config.ByteSteps)
	byteStep := m.config.ByteSteps[byteStepIndex]

	mutationCount := (variant % m.config.MaxMutations) + 1

	for i := 0; i < mutationCount && i < len(data); i++ {
		offset := (variant + i*17) % len(data)

		// 应用字节级步长变异
		currentByte := int(data[offset])
		newByte := currentByte + byteStep

		// 处理字节范围溢出
		if newByte < 0 {
			newByte = 256 + (newByte % 256)
		} else if newByte > 255 {
			newByte = newByte % 256
		}

		data[offset] = byte(newByte)

		// 应用更大的步长变异（基于输入步长）
		if len(data) >= offset+8 {
			currentValue := new(big.Int).SetBytes(data[offset : offset+8])
			delta := big.NewInt(step * int64(i+1))

			if variant%2 == 1 {
				delta.Neg(delta)
			}

			newValue := new(big.Int).Add(currentValue, delta)
			valueBytes := newValue.Bytes()

			// 确保不超过8字节
			if len(valueBytes) > 8 {
				valueBytes = valueBytes[len(valueBytes)-8:]
			}

			// 清零原有字节并设置新值
			for j := 0; j < 8; j++ {
				data[offset+j] = 0
			}

			startPos := offset + 8 - len(valueBytes)
			copy(data[startPos:offset+8], valueBytes)
		}
	}
}
