package tracing

import (
	"github.com/DQYXACML/autopatch/txmgr/ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ccrypto "github.com/ethereum/go-ethereum/crypto"
	"math/big"
	"time"
)

// ExecutionJump represents a jump instruction execution
type ExecutionJump struct {
	ContractAddress common.Address `json:"contractAddress"`
	JumpFrom        uint64         `json:"jumpFrom"`
	JumpDest        uint64         `json:"jumpDest"`
}

// ExecutionPath represents the execution path of a transaction
type ExecutionPath struct {
	Jumps []ExecutionJump `json:"jumps"`
}

// ComputePathHash computes a simple hash for path comparison
func (p *ExecutionPath) ComputePathHash() [32]byte {
	data := make([]byte, 0)
	for _, jump := range p.Jumps {
		data = append(data, jump.ContractAddress.Bytes()...)
		jumpFromBytes := make([]byte, 8)
		jumpDestBytes := make([]byte, 8)
		for i := 0; i < 8; i++ {
			jumpFromBytes[i] = byte(jump.JumpFrom >> (8 * (7 - i)))
			jumpDestBytes[i] = byte(jump.JumpDest >> (8 * (7 - i)))
		}
		data = append(data, jumpFromBytes...)
		data = append(data, jumpDestBytes...)
	}
	return [32]byte(ccrypto.Keccak256Hash(data))
}

// IsSimilar compares two execution paths and returns similarity percentage
func (p *ExecutionPath) IsSimilar(other *ExecutionPath, threshold float64) bool {
	if other == nil {
		return false
	}

	if len(p.Jumps) == 0 && len(other.Jumps) == 0 {
		return true
	}

	if len(p.Jumps) == 0 || len(other.Jumps) == 0 {
		return false
	}

	// 计算相同跳转的数量
	matches := 0
	minLen := len(p.Jumps)
	if len(other.Jumps) < minLen {
		minLen = len(other.Jumps)
	}

	for i := 0; i < minLen; i++ {
		if p.Jumps[i].ContractAddress == other.Jumps[i].ContractAddress &&
			p.Jumps[i].JumpFrom == other.Jumps[i].JumpFrom &&
			p.Jumps[i].JumpDest == other.Jumps[i].JumpDest {
			matches++
		}
	}

	maxLen := len(p.Jumps)
	if len(other.Jumps) > maxLen {
		maxLen = len(other.Jumps)
	}

	similarity := float64(matches) / float64(maxLen)
	return similarity >= threshold
}

// Account represents an Ethereum account state
type Account struct {
	Balance *hexutil.Big                `json:"balance,omitempty"`
	Code    hexutil.Bytes               `json:"code,omitempty"`
	Nonce   uint64                      `json:"nonce,omitempty"`
	Storage map[common.Hash]common.Hash `json:"storage,omitempty"`
}

// PrestateResult represents the result from prestateTracer
type PrestateResult map[common.Address]*Account

// StorageChange represents a single storage slot change
type StorageChange struct {
	Slot     common.Hash `json:"slot"`
	OldValue common.Hash `json:"oldValue"`
	NewValue common.Hash `json:"newValue"`
	Changed  bool        `json:"changed"`
}

// StateModification represents modifications to contract state
type StateModification struct {
	ContractAddress common.Address              `json:"contractAddress"`
	StorageChanges  map[common.Hash]common.Hash `json:"storageChanges"` // slot -> new value
	InputData       []byte                      `json:"inputData"`
}

// ReplayResult represents the result of transaction replay
type ReplayResult struct {
	OriginalPath    *ExecutionPath     `json:"originalPath"`
	ModifiedPath    *ExecutionPath     `json:"modifiedPath"`
	Similarity      float64            `json:"similarity"`
	IsAttackPattern bool               `json:"isAttackPattern"`
	Modifications   *StateModification `json:"modifications"`
}

// ========== 并发修改相关结构体 ==========

// ModificationCandidate 修改候选
type ModificationCandidate struct {
	ID             string                      `json:"id"`
	InputData      []byte                      `json:"inputData"`
	StorageChanges map[common.Hash]common.Hash `json:"storageChanges"`
	ModType        string                      `json:"modType"` // "input", "storage", "both"
	Priority       int                         `json:"priority"`
	ExpectedImpact string                      `json:"expectedImpact"`
	GeneratedAt    time.Time                   `json:"generatedAt"`
}

// SimulationResult 模拟执行结果
type SimulationResult struct {
	Candidate   *ModificationCandidate `json:"candidate"`
	Similarity  float64                `json:"similarity"`
	Success     bool                   `json:"success"`
	Error       error                  `json:"error"`
	ExecutePath *ExecutionPath         `json:"executePath"`
	GasUsed     uint64                 `json:"gasUsed"`
	Duration    time.Duration          `json:"duration"`
}

// TransactionPackage 便于打包成交易的结构体
type TransactionPackage struct {
	ID              string          `json:"id"`
	ContractAddress common.Address  `json:"contractAddress"`
	InputUpdates    []InputUpdate   `json:"inputUpdates"`
	StorageUpdates  []StorageUpdate `json:"storageUpdates"`
	Similarity      float64         `json:"similarity"`
	OriginalTxHash  common.Hash     `json:"originalTxHash"`
	CreatedAt       time.Time       `json:"createdAt"`
	Priority        int             `json:"priority"`
}

// InputUpdate 输入数据更新
type InputUpdate struct {
	FunctionSelector [4]byte     `json:"functionSelector"`
	FunctionName     string      `json:"functionName"`
	OriginalInput    []byte      `json:"originalInput"`
	ModifiedInput    []byte      `json:"modifiedInput"`
	ParameterIndex   int         `json:"parameterIndex"`
	ParameterType    string      `json:"parameterType"`
	ParameterName    string      `json:"parameterName"`
	OriginalValue    interface{} `json:"originalValue"`
	ModifiedValue    interface{} `json:"modifiedValue"`
}

// StorageUpdate 存储更新
type StorageUpdate struct {
	Slot          common.Hash `json:"slot"`
	OriginalValue common.Hash `json:"originalValue"`
	ModifiedValue common.Hash `json:"modifiedValue"`
	SlotType      string      `json:"slotType"` // "uint", "int", "bool", "address", "bytes", "string", "mapping", "array"
	ValueType     string      `json:"valueType"`
	Description   string      `json:"description"`
}

// ConcurrentModificationConfig 并发修改配置
type ConcurrentModificationConfig struct {
	MaxWorkers          int           `json:"maxWorkers"`
	SimilarityThreshold float64       `json:"similarityThreshold"`
	MaxCandidates       int           `json:"maxCandidates"`
	GenerationTimeout   time.Duration `json:"generationTimeout"`
	SimulationTimeout   time.Duration `json:"simulationTimeout"`
	ChannelBufferSize   int           `json:"channelBufferSize"`
	BatchSize           int           `json:"batchSize"`
}

// WorkerResult 工作协程结果
type WorkerResult struct {
	WorkerID int               `json:"workerId"`
	Result   *SimulationResult `json:"result"`
	Error    error             `json:"error"`
}

// TransactionRequest 交易请求
type TransactionRequest struct {
	Package     *TransactionPackage `json:"package"`
	PrivateKey  string              `json:"privateKey"`
	ChainID     *big.Int            `json:"chainId"`
	GasLimit    uint64              `json:"gasLimit"`
	GasPrice    *big.Int            `json:"gasPrice"`
	Nonce       uint64              `json:"nonce"`
	RequestID   string              `json:"requestId"`
	RequestedAt time.Time           `json:"requestedAt"`
}

// TransactionResponse 交易响应
type TransactionResponse struct {
	RequestID   string      `json:"requestId"`
	TxHash      common.Hash `json:"txHash"`
	Success     bool        `json:"success"`
	Error       error       `json:"error"`
	GasUsed     uint64      `json:"gasUsed"`
	BlockNumber *big.Int    `json:"blockNumber"`
	SentAt      time.Time   `json:"sentAt"`
}

// ========== 链上防护相关结构体 ==========

// OnChainProtectionRule 链上防护规则
type OnChainProtectionRule struct {
	RuleID          string                  `json:"ruleId"`
	TxHash          common.Hash             `json:"txHash"`          // 原始攻击交易哈希
	ContractAddress common.Address          `json:"contractAddress"` // 目标合约地址
	Similarity      float64                 `json:"similarity"`      // 相似度
	InputRules      []InputProtectionRule   `json:"inputRules"`      // 输入数据保护规则
	StorageRules    []StorageProtectionRule `json:"storageRules"`    // 存储保护规则
	CreatedAt       time.Time               `json:"createdAt"`
	IsActive        bool                    `json:"isActive"`
}

// InputProtectionRule 输入数据保护规则
type InputProtectionRule struct {
	FunctionSelector [4]byte               `json:"functionSelector"` // 函数选择器
	FunctionName     string                `json:"functionName"`     // 函数名称
	OriginalInput    []byte                `json:"originalInput"`    // 原始输入数据
	ModifiedInput    []byte                `json:"modifiedInput"`    // 修改后的输入数据
	ParameterRules   []ParameterProtection `json:"parameterRules"`   // 参数保护规则
	InputHash        common.Hash           `json:"inputHash"`        // 输入数据哈希
}

// ParameterProtection 参数保护规则
type ParameterProtection struct {
	Index         int         `json:"index"`         // 参数索引
	Name          string      `json:"name"`          // 参数名称
	Type          string      `json:"type"`          // 参数类型
	OriginalValue interface{} `json:"originalValue"` // 原始值
	ModifiedValue interface{} `json:"modifiedValue"` // 修改后的值
	MinValue      *big.Int    `json:"minValue"`      // 允许的最小值
	MaxValue      *big.Int    `json:"maxValue"`      // 允许的最大值
	CheckType     string      `json:"checkType"`     // 检查类型: "exact", "range", "pattern"
}

// StorageProtectionRule 存储保护规则
type StorageProtectionRule struct {
	ContractAddress common.Address `json:"contractAddress"` // 被检查的合约地址
	StorageSlot     common.Hash    `json:"storageSlot"`     // 存储槽位
	OriginalValue   common.Hash    `json:"originalValue"`   // 原始值
	ModifiedValue   common.Hash    `json:"modifiedValue"`   // 修改后的值
	MinValue        *big.Int       `json:"minValue"`        // 允许的最小值
	MaxValue        *big.Int       `json:"maxValue"`        // 允许的最大值
	CheckType       string         `json:"checkType"`       // 检查类型: "exact", "range", "delta"
	SlotType        string         `json:"slotType"`        // 槽位类型: "mapping", "array", "simple"
}

// ProtectionRuleSet 保护规则集合
type ProtectionRuleSet struct {
	Rules       []OnChainProtectionRule `json:"rules"`
	TotalRules  int                     `json:"totalRules"`
	ActiveRules int                     `json:"activeRules"`
	CreatedAt   time.Time               `json:"createdAt"`
	UpdatedAt   time.Time               `json:"updatedAt"`
}

// ========== 简化的重放结果结构体 ==========

// SimplifiedReplayResult 简化的重放结果
type SimplifiedReplayResult struct {
	OriginalPath      *ExecutionPath          `json:"originalPath"`
	SuccessfulRules   []OnChainProtectionRule `json:"successfulRules"`
	FailedVariations  []ModificationVariation `json:"failedVariations"`
	TotalVariations   int                     `json:"totalVariations"`
	HighestSimilarity float64                 `json:"highestSimilarity"`
	ProcessingTime    time.Duration           `json:"processingTime"`
	Statistics        *SimpleReplayStatistics `json:"statistics"`
}

// SimpleReplayStatistics 简化的重放统计信息
type SimpleReplayStatistics struct {
	SuccessCount      int            `json:"successCount"`
	FailCount         int            `json:"failCount"`
	AverageSimilarity float64        `json:"averageSimilarity"`
	StrategyResults   map[string]int `json:"strategyResults"`
	ErrorDistribution map[string]int `json:"errorDistribution"`
}

// ModificationVariation 修改变体（保留用于兼容性）
type ModificationVariation struct {
	ID              string                 `json:"id"`
	InputMod        *InputModification     `json:"inputMod"`
	StorageMod      *StorageModification   `json:"storageMod"`
	ExpectedImpact  string                 `json:"expectedImpact"`
	ModificationSet map[string]interface{} `json:"modificationSet"`
}

// InputModification 输入修改详情
type InputModification struct {
	OriginalInput    []byte            `json:"originalInput"`
	ModifiedInput    []byte            `json:"modifiedInput"`
	FunctionSelector [4]byte           `json:"functionSelector"`
	FunctionName     string            `json:"functionName"`
	ParameterChanges []ParameterChange `json:"parameterChanges"`
	ModificationHash common.Hash       `json:"modificationHash"`
}

// ParameterChange 参数变化详情
type ParameterChange struct {
	Index       int         `json:"index"`
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Original    interface{} `json:"original"`
	Modified    interface{} `json:"modified"`
	Delta       interface{} `json:"delta"`
	ChangeType  string      `json:"changeType"`
	ChangeRatio float64     `json:"changeRatio"`
}

// StorageModification 存储修改详情
type StorageModification struct {
	Changes          []StorageSlotChange `json:"changes"`
	ModificationHash common.Hash         `json:"modificationHash"`
}

// StorageSlotChange 存储槽变化详情
type StorageSlotChange struct {
	Slot        common.Hash `json:"slot"`
	Original    common.Hash `json:"original"`
	Modified    common.Hash `json:"modified"`
	Delta       *big.Int    `json:"delta"`
	ChangeType  string      `json:"changeType"`
	ChangeRatio float64     `json:"changeRatio"`
	SlotType    string      `json:"slotType"`
}

// 在 types.go 文件的适当位置添加以下结构体：

// StepBasedModificationCandidate 基于步长的修改候选
type StepBasedModificationCandidate struct {
	*ModificationCandidate
	InputSteps   []int64 `json:"inputSteps"`   // 应用的输入步长
	StorageSteps []int64 `json:"storageSteps"` // 应用的存储步长
	StepStrategy string  `json:"stepStrategy"` // 步长策略：add, sub, mul, xor
	BaselineUsed bool    `json:"baselineUsed"` // 是否基于原始值
}

// StepMutationResult 步长变异结果
type StepMutationResult struct {
	OriginalValue interface{} `json:"originalValue"`
	ModifiedValue interface{} `json:"modifiedValue"`
	AppliedStep   int64       `json:"appliedStep"`
	StepType      string      `json:"stepType"` // add, sub, mul, xor
	ChangeRatio   float64     `json:"changeRatio"`
	Success       bool        `json:"success"`
}

// ========== 变异数据收集结构 ==========

// MutationData 单个变异的数据
type MutationData struct {
	ID             string                      `json:"id"`
	InputData      []byte                      `json:"inputData"`
	StorageChanges map[common.Hash]common.Hash `json:"storageChanges"`
	Similarity     float64                     `json:"similarity"`
	Success        bool                        `json:"success"`
	ErrorMessage   string                      `json:"errorMessage"`
	ExecutionTime  time.Duration               `json:"executionTime"`
}

// MutationCollection 变异数据集合，用于发送给链上处理
type MutationCollection struct {
	OriginalTxHash      common.Hash                 `json:"originalTxHash"`
	ContractAddress     common.Address              `json:"contractAddress"`
	OriginalInputData   []byte                      `json:"originalInputData"`
	OriginalStorage     map[common.Hash]common.Hash `json:"originalStorage"`
	Mutations           []MutationData              `json:"mutations"`
	SuccessfulMutations []MutationData              `json:"successfulMutations"`
	TotalMutations      int                         `json:"totalMutations"`
	SuccessCount        int                         `json:"successCount"`
	FailureCount        int                         `json:"failureCount"`
	AverageSimilarity   float64                     `json:"averageSimilarity"`
	HighestSimilarity   float64                     `json:"highestSimilarity"`
	ProcessingTime      time.Duration               `json:"processingTime"`
	CreatedAt           time.Time                   `json:"createdAt"`
}

// ToSolidityFormat 转换为适合发送给Solidity的格式
func (mc *MutationCollection) ToSolidityFormat() *SolidityMutationData {
	solidityData := &SolidityMutationData{
		OriginalTxHash:    mc.OriginalTxHash,
		ContractAddress:   mc.ContractAddress,
		OriginalInputData: mc.OriginalInputData,
		InputMutations:    make([][]byte, 0),
		StorageMutations:  make([]SolidityStorageMutation, 0),
		Similarities:      make([]*big.Int, 0),
		TotalMutations:    big.NewInt(int64(mc.TotalMutations)),
		SuccessCount:      big.NewInt(int64(mc.SuccessCount)),
	}

	for _, mutation := range mc.SuccessfulMutations {
		// 收集输入变异
		if len(mutation.InputData) > 0 {
			solidityData.InputMutations = append(solidityData.InputMutations, mutation.InputData)
		}

		// 收集存储变异
		for slot, value := range mutation.StorageChanges {
			storageMutation := SolidityStorageMutation{
				Slot:  slot,
				Value: value,
			}
			solidityData.StorageMutations = append(solidityData.StorageMutations, storageMutation)
		}

		// 收集相似度（转换为百分比的整数）
		similarityPercent := new(big.Int).SetInt64(int64(mutation.Similarity * 10000)) // 保留4位小数
		solidityData.Similarities = append(solidityData.Similarities, similarityPercent)
	}

	return solidityData
}

// SolidityMutationData 适合发送给Solidity的数据格式
type SolidityMutationData struct {
	OriginalTxHash    common.Hash               `json:"originalTxHash"`
	ContractAddress   common.Address            `json:"contractAddress"`
	OriginalInputData []byte                    `json:"originalInputData"`
	InputMutations    [][]byte                  `json:"inputMutations"`
	StorageMutations  []SolidityStorageMutation `json:"storageMutations"`
	Similarities      []*big.Int                `json:"similarities"`
	TotalMutations    *big.Int                  `json:"totalMutations"`
	SuccessCount      *big.Int                  `json:"successCount"`
}

// SolidityStorageMutation 存储变异的Solidity格式
type SolidityStorageMutation struct {
	Slot  common.Hash `json:"slot"`
	Value common.Hash `json:"value"`
}

// ========== 辅助函数 ==========

// GenerateRuleID 生成保护规则ID
func GenerateRuleID(txHash common.Hash, contractAddr common.Address, timestamp time.Time) string {
	data := append(txHash.Bytes(), contractAddr.Bytes()...)
	data = append(data, big.NewInt(timestamp.Unix()).Bytes()...)
	hash := ccrypto.Keccak256Hash(data)
	return hash.Hex()[:16] // 取前16位作为ID
}

// ComputeModificationHash 计算修改的哈希值
func ComputeModificationHash(original, modified []byte) common.Hash {
	data := append(original, modified...)
	return ccrypto.Keccak256Hash(data)
}

// CalculateChangeRatio 计算变化比例
func CalculateChangeRatio(original, modified *big.Int) float64 {
	if original.Sign() == 0 {
		if modified.Sign() == 0 {
			return 0.0
		}
		return 1.0 // 从0变为非0，视为100%变化
	}

	delta := new(big.Int).Sub(modified, original)
	ratio := new(big.Float).Quo(new(big.Float).SetInt(delta), new(big.Float).SetInt(original))
	result, _ := ratio.Float64()
	return result
}

// DetermineChangeType 确定变化类型
func DetermineChangeType(original, modified *big.Int) string {
	cmp := original.Cmp(modified)
	switch {
	case cmp == 0:
		return "unchanged"
	case cmp > 0:
		return "decrement"
	case cmp < 0:
		return "increment"
	default:
		return "replace"
	}
}

// ExtractSlotType 根据存储槽位置推断类型
func ExtractSlotType(slot common.Hash) string {
	// 简单的启发式方法判断槽类型
	slotBig := slot.Big()

	// 如果是小整数，可能是简单变量
	if slotBig.Cmp(big.NewInt(100)) < 0 {
		return "simple"
	}

	// 如果是大数且有特定模式，可能是映射
	slotBytes := slot.Bytes()
	zeroCount := 0
	for _, b := range slotBytes {
		if b == 0 {
			zeroCount++
		}
	}

	if zeroCount > 20 {
		return "mapping"
	}

	return "array"
}

// CreateInputProtectionRule 创建输入保护规则
func CreateInputProtectionRule(inputMod *InputModification) InputProtectionRule {
	if inputMod == nil {
		// 返回一个带有默认值的规则而不是空规则
		return InputProtectionRule{
			FunctionSelector: [4]byte{0x00, 0x00, 0x00, 0x00},
			FunctionName:     "unknown",
			OriginalInput:    []byte{},
			ModifiedInput:    []byte{},
			ParameterRules:   []ParameterProtection{},
			InputHash:        common.Hash{},
		}
	}

	paramRules := make([]ParameterProtection, 0)
	for _, paramChange := range inputMod.ParameterChanges {
		paramRule := ParameterProtection{
			Index:         paramChange.Index,
			Name:          paramChange.Name,
			Type:          paramChange.Type,
			OriginalValue: paramChange.Original,
			ModifiedValue: paramChange.Modified,
			CheckType:     determineCheckType(paramChange),
		}

		// 为数值类型设置范围
		if paramRule.CheckType == "range" {
			paramRule.MinValue, paramRule.MaxValue = calculateValueRange(paramChange)
		}

		paramRules = append(paramRules, paramRule)
	}

	// 如果没有参数规则，创建一个默认的
	if len(paramRules) == 0 {
		paramRules = append(paramRules, ParameterProtection{
			Index:         0,
			Name:          "default_param",
			Type:          "bytes",
			OriginalValue: inputMod.OriginalInput,
			ModifiedValue: inputMod.ModifiedInput,
			CheckType:     "exact",
		})
	}

	return InputProtectionRule{
		FunctionSelector: inputMod.FunctionSelector,
		FunctionName:     inputMod.FunctionName,
		OriginalInput:    inputMod.OriginalInput,
		ModifiedInput:    inputMod.ModifiedInput,
		ParameterRules:   paramRules,
		InputHash:        inputMod.ModificationHash,
	}
}

// CreateStorageProtectionRules 创建存储保护规则
func CreateStorageProtectionRules(storageMod *StorageModification, contractAddr common.Address) []StorageProtectionRule {
	if storageMod == nil {
		// 返回一个包含默认规则的列表而不是nil
		defaultRule := StorageProtectionRule{
			ContractAddress: contractAddr,
			StorageSlot:     common.BigToHash(big.NewInt(0)), // 使用槽位0作为默认
			OriginalValue:   common.Hash{},
			ModifiedValue:   common.BigToHash(big.NewInt(1)),
			CheckType:       "exact",
			SlotType:        "simple",
		}
		return []StorageProtectionRule{defaultRule}
	}

	rules := make([]StorageProtectionRule, 0)
	for _, change := range storageMod.Changes {
		// 确保槽位不为空
		slot := change.Slot
		if slot == (common.Hash{}) {
			// 如果槽位为空，使用一个默认槽位
			slot = common.BigToHash(big.NewInt(0))
		}

		rule := StorageProtectionRule{
			ContractAddress: contractAddr,
			StorageSlot:     slot,
			OriginalValue:   change.Original,
			ModifiedValue:   change.Modified,
			CheckType:       determineStorageCheckType(change),
			SlotType:        change.SlotType,
		}

		// 为数值类型设置范围
		if rule.CheckType == "range" {
			rule.MinValue, rule.MaxValue = calculateStorageValueRange(change)
		}

		rules = append(rules, rule)
	}

	// 如果没有规则，创建一个默认的
	if len(rules) == 0 {
		defaultRule := StorageProtectionRule{
			ContractAddress: contractAddr,
			StorageSlot:     common.BigToHash(big.NewInt(0)),
			OriginalValue:   common.Hash{},
			ModifiedValue:   common.BigToHash(big.NewInt(1)),
			CheckType:       "exact",
			SlotType:        "simple",
		}
		rules = append(rules, defaultRule)
	}

	return rules
}

// determineCheckType 确定参数检查类型
func determineCheckType(paramChange ParameterChange) string {
	switch paramChange.Type {
	case "uint256", "int256", "uint8", "int8":
		if paramChange.ChangeRatio > 0.1 { // 变化超过10%使用范围检查
			return "range"
		}
		return "exact"
	case "bool":
		return "exact"
	case "string":
		return "pattern"
	case "address":
		return "exact"
	default:
		return "exact"
	}
}

// determineStorageCheckType 确定存储检查类型
func determineStorageCheckType(change StorageSlotChange) string {
	if change.ChangeRatio > 0.1 { // 变化超过10%使用范围检查
		return "range"
	}
	return "exact"
}

// calculateValueRange 计算参数值范围
func calculateValueRange(paramChange ParameterChange) (*big.Int, *big.Int) {
	switch v := paramChange.Modified.(type) {
	case *big.Int:
		// 设置±20%的范围
		delta := new(big.Int).Div(v, big.NewInt(5)) // 20%
		minVal := new(big.Int).Sub(v, delta)
		maxVal := new(big.Int).Add(v, delta)

		// 确保最小值不为负数（对于uint类型）
		if minVal.Sign() < 0 {
			minVal = big.NewInt(0)
		}

		return minVal, maxVal
	case uint8:
		delta := uint8(20) // 固定delta
		minVal := big.NewInt(int64(v) - int64(delta))
		maxVal := big.NewInt(int64(v) + int64(delta))

		if minVal.Sign() < 0 {
			minVal = big.NewInt(0)
		}
		if maxVal.Cmp(big.NewInt(255)) > 0 {
			maxVal = big.NewInt(255)
		}

		return minVal, maxVal
	default:
		// 默认返回修改后的值作为精确匹配
		if bigVal, ok := convertToBigInt(v); ok {
			return bigVal, bigVal
		}
		return big.NewInt(0), big.NewInt(0)
	}
}

// calculateStorageValueRange 计算存储值范围
func calculateStorageValueRange(change StorageSlotChange) (*big.Int, *big.Int) {
	modifiedBig := change.Modified.Big()

	// 设置±20%的范围
	delta := new(big.Int).Div(modifiedBig, big.NewInt(5)) // 20%
	minVal := new(big.Int).Sub(modifiedBig, delta)
	maxVal := new(big.Int).Add(modifiedBig, delta)

	// 确保最小值不为负数
	if minVal.Sign() < 0 {
		minVal = big.NewInt(0)
	}

	return minVal, maxVal
}

// convertToBigInt 将任意类型转换为big.Int
func convertToBigInt(value interface{}) (*big.Int, bool) {
	switch v := value.(type) {
	case *big.Int:
		return v, true
	case int64:
		return big.NewInt(v), true
	case uint64:
		return new(big.Int).SetUint64(v), true
	case int:
		return big.NewInt(int64(v)), true
	case uint8:
		return big.NewInt(int64(v)), true
	case int8:
		return big.NewInt(int64(v)), true
	default:
		return nil, false
	}
}

// ========== 并发修改相关辅助函数 ==========

// CreateTransactionPackage 创建交易包
func CreateTransactionPackage(candidate *ModificationCandidate, similarity float64,
	contractAddr common.Address, originalTxHash common.Hash) *ethereum.TransactionPackage {

	pkg := &ethereum.TransactionPackage{
		ID:              candidate.ID,
		ContractAddress: contractAddr,
		InputUpdates:    make([]ethereum.InputUpdate, 0),
		StorageUpdates:  make([]ethereum.StorageUpdate, 0),
		Similarity:      similarity,
		OriginalTxHash:  originalTxHash,
		CreatedAt:       time.Now(),
		Priority:        candidate.Priority,
	}

	// 处理输入更新
	if candidate.InputData != nil && len(candidate.InputData) > 0 {
		inputUpdate := ethereum.InputUpdate{
			ModifiedInput: candidate.InputData,
		}
		if len(candidate.InputData) >= 4 {
			copy(inputUpdate.FunctionSelector[:], candidate.InputData[:4])
		}
		pkg.InputUpdates = append(pkg.InputUpdates, inputUpdate)
	}

	// 处理存储更新
	for slot, value := range candidate.StorageChanges {
		storageUpdate := ethereum.StorageUpdate{
			Slot:          slot,
			ModifiedValue: value,
			SlotType:      ExtractSlotType(slot),
		}
		pkg.StorageUpdates = append(pkg.StorageUpdates, storageUpdate)
	}

	return pkg
}

// DefaultConcurrentModificationConfig 默认并发修改配置
func DefaultConcurrentModificationConfig() *ConcurrentModificationConfig {
	return &ConcurrentModificationConfig{
		MaxWorkers:          8,
		SimilarityThreshold: 0.9,
		MaxCandidates:       1000,
		GenerationTimeout:   30 * time.Second,
		SimulationTimeout:   10 * time.Second,
		ChannelBufferSize:   100,
		BatchSize:           10,
	}
}

// CreateInputProtectionRuleFromCandidate 从修改候选创建输入保护规则
func CreateInputProtectionRuleFromCandidate(candidate *ModificationCandidate, originalInput []byte) InputProtectionRule {
	rule := InputProtectionRule{
		OriginalInput:  originalInput,
		ModifiedInput:  candidate.InputData,
		ParameterRules: make([]ParameterProtection, 0),
	}

	// 设置函数选择器
	if len(candidate.InputData) >= 4 {
		copy(rule.FunctionSelector[:], candidate.InputData[:4])
		rule.FunctionName = "modified_function"
	}

	// 创建一个默认的参数保护规则，确保不为空
	defaultParam := ParameterProtection{
		Index:         0,
		Name:          "param_0",
		Type:          "bytes",
		OriginalValue: originalInput,
		ModifiedValue: candidate.InputData,
		CheckType:     "exact",
	}
	rule.ParameterRules = append(rule.ParameterRules, defaultParam)

	// 如果输入数据长度超过4字节，创建更多参数规则
	if len(candidate.InputData) > 4 {
		// 分析输入数据的变化，创建更详细的参数规则
		paramData := candidate.InputData[4:]
		if len(paramData) >= 32 {
			// 假设是uint256参数
			value := new(big.Int).SetBytes(paramData[:32])
			param1 := ParameterProtection{
				Index:         1,
				Name:          "param_1",
				Type:          "uint256",
				OriginalValue: big.NewInt(0),
				ModifiedValue: value,
				CheckType:     "range",
				MinValue:      new(big.Int).Sub(value, big.NewInt(1000)),
				MaxValue:      new(big.Int).Add(value, big.NewInt(1000)),
			}
			rule.ParameterRules = append(rule.ParameterRules, param1)
		}
	}

	rule.InputHash = ComputeModificationHash(originalInput, candidate.InputData)
	return rule
}

// CreateStorageProtectionRulesFromCandidate 从修改候选创建存储保护规则
func CreateStorageProtectionRulesFromCandidate(candidate *ModificationCandidate, contractAddr common.Address) []StorageProtectionRule {
	rules := make([]StorageProtectionRule, 0)

	// 为每个存储变化创建规则
	for slot, value := range candidate.StorageChanges {
		// 确保槽位有效
		if slot == (common.Hash{}) {
			slot = common.BigToHash(big.NewInt(1)) // 使用槽位1而不是0
		}

		rule := StorageProtectionRule{
			ContractAddress: contractAddr,
			StorageSlot:     slot,
			OriginalValue:   common.Hash{},
			ModifiedValue:   value,
			CheckType:       "exact",
			SlotType:        ExtractSlotType(slot),
		}

		// 为数值类型设置范围检查
		if rule.SlotType == "simple" || rule.SlotType == "mapping" {
			valueBig := value.Big()
			if valueBig.Sign() > 0 {
				rule.CheckType = "range"
				delta := new(big.Int).Div(valueBig, big.NewInt(10)) // 10%的范围
				rule.MinValue = new(big.Int).Sub(valueBig, delta)
				rule.MaxValue = new(big.Int).Add(valueBig, delta)

				if rule.MinValue.Sign() < 0 {
					rule.MinValue = big.NewInt(0)
				}
			}
		}

		rules = append(rules, rule)
	}

	// 如果没有存储变化，创建一个默认规则
	if len(rules) == 0 {
		defaultRule := StorageProtectionRule{
			ContractAddress: contractAddr,
			StorageSlot:     common.BigToHash(big.NewInt(1)), // 使用有效的槽位
			OriginalValue:   common.Hash{},
			ModifiedValue:   common.BigToHash(big.NewInt(1)),
			CheckType:       "exact",
			SlotType:        "simple",
		}
		rules = append(rules, defaultRule)
	}

	return rules
}
