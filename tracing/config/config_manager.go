package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// TypeAwareMutationConfig 类型感知变异配置
type TypeAwareMutationConfig struct {
	// 基础配置
	EnableTypeAware   bool `json:"enableTypeAware" yaml:"enableTypeAware"`
	FallbackToGeneric bool `json:"fallbackToGeneric" yaml:"fallbackToGeneric"`
	MaxMutations      int  `json:"maxMutations" yaml:"maxMutations"`
	
	// 链配置
	Chains map[string]*ChainMutationConfig `json:"chains" yaml:"chains"`
	
	// 类型特定配置
	AddressMutation AddressMutationConfig `json:"addressMutation" yaml:"addressMutation"`
	NumberMutation  NumberMutationConfig  `json:"numberMutation" yaml:"numberMutation"`
	StringMutation  StringMutationConfig  `json:"stringMutation" yaml:"stringMutation"`
	
	// 执行配置
	Execution ExecutionConfig `json:"execution" yaml:"execution"`
}

// ChainMutationConfig 链特定变异配置
type ChainMutationConfig struct {
	ChainID           int64  `json:"chainId" yaml:"chainId"`
	Name              string `json:"name" yaml:"name"`
	ExplorerAPIKey    string `json:"explorerApiKey" yaml:"explorerApiKey"`
	ExplorerAPI       string `json:"explorerApi" yaml:"explorerApi"`
	KnownAddresses    []string `json:"knownAddresses" yaml:"knownAddresses"`
	EnableTypeAware   bool   `json:"enableTypeAware" yaml:"enableTypeAware"`
}

// AddressMutationConfig 地址变异配置
type AddressMutationConfig struct {
	UseKnownAddresses bool    `json:"useKnownAddresses" yaml:"useKnownAddresses"`
	FlipBytes         []int   `json:"flipBytes" yaml:"flipBytes"`
	NearbyRange       int64   `json:"nearbyRange" yaml:"nearbyRange"`
	ZeroAddressRatio  float64 `json:"zeroAddressRatio" yaml:"zeroAddressRatio"`
}

// NumberMutationConfig 数值变异配置
type NumberMutationConfig struct {
	BoundaryValues  bool    `json:"boundaryValues" yaml:"boundaryValues"`
	StepSizes       []int64 `json:"stepSizes" yaml:"stepSizes"`
	MultiplierRatio float64 `json:"multiplierRatio" yaml:"multiplierRatio"`
	BitPatterns     bool    `json:"bitPatterns" yaml:"bitPatterns"`
}

// StringMutationConfig 字符串变异配置
type StringMutationConfig struct {
	MaxLength     int    `json:"maxLength" yaml:"maxLength"`
	SpecialChars  bool   `json:"specialChars" yaml:"specialChars"`
	EncodingTests bool   `json:"encodingTests" yaml:"encodingTests"`
	Truncation    bool   `json:"truncation" yaml:"truncation"`
}

// ExecutionConfig 执行配置
type ExecutionConfig struct {
	MaxConcurrentWorkers int     `json:"maxConcurrentWorkers" yaml:"maxConcurrentWorkers"`
	BatchSize            int     `json:"batchSize" yaml:"batchSize"`
	TimeoutSeconds       int     `json:"timeoutSeconds" yaml:"timeoutSeconds"`
	SimilarityThreshold  float64 `json:"similarityThreshold" yaml:"similarityThreshold"`
	EnableEarlyPruning   bool    `json:"enableEarlyPruning" yaml:"enableEarlyPruning"`
	CacheSize            int     `json:"cacheSize" yaml:"cacheSize"`
}

// DefaultTypeAwareMutationConfig 创建默认类型感知变异配置
func DefaultTypeAwareMutationConfig() *TypeAwareMutationConfig {
	return &TypeAwareMutationConfig{
		EnableTypeAware:   true,
		FallbackToGeneric: true,
		MaxMutations:      1000,
		
		Chains: map[string]*ChainMutationConfig{
			"ethereum": {
				ChainID:         1,
				Name:            "ethereum",
				ExplorerAPIKey:  "", // 从环境变量获取
				ExplorerAPI:     "https://api.etherscan.io/api",
				KnownAddresses: []string{
					"0x0000000000000000000000000000000000000000", // Zero address
					"0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2", // WETH
					"0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", // USDC
					"0xdAC17F958D2ee523a2206206994597C13D831ec7", // USDT
					"0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D", // Uniswap Router
				},
				EnableTypeAware: true,
			},
			"bsc": {
				ChainID:         56,
				Name:            "bsc",
				ExplorerAPIKey:  "", // 从环境变量获取
				ExplorerAPI:     "https://api.bscscan.com/api",
				KnownAddresses: []string{
					"0x0000000000000000000000000000000000000000", // Zero address
					"0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c", // WBNB
					"0xe9e7CEA3DedcA5984780Bafc599bD69ADd087D56", // BUSD
					"0x10ED43C718714eb63d5aA57B78B54704E256024E", // PancakeSwap Router
					"0x8894E0a0c962CB723c1976a4421c95949bE2D4E3", // BETO
				},
				EnableTypeAware: true,
			},
		},
		
		AddressMutation: AddressMutationConfig{
			UseKnownAddresses: true,
			FlipBytes:         []int{1, 2, 4, 8},
			NearbyRange:       1000,
			ZeroAddressRatio:  0.1,
		},
		
		NumberMutation: NumberMutationConfig{
			BoundaryValues:  true,
			StepSizes:       []int64{1, 10, 100, 1000, 10000, -1, -10, -100, -1000},
			MultiplierRatio: 0.2,
			BitPatterns:     true,
		},
		
		StringMutation: StringMutationConfig{
			MaxLength:     1000,
			SpecialChars:  true,
			EncodingTests: true,
			Truncation:    true,
		},
		
		Execution: ExecutionConfig{
			MaxConcurrentWorkers: 8,
			BatchSize:            100,
			TimeoutSeconds:       30,
			SimilarityThreshold:  0.8,
			EnableEarlyPruning:   true,
			CacheSize:            10000,
		},
	}
}

// ConfigManager 配置管理器
type ConfigManager struct {
	config     *TypeAwareMutationConfig
	configPath string
}

// NewConfigManager 创建配置管理器
func NewConfigManager(configPath string) *ConfigManager {
	return &ConfigManager{
		config:     DefaultTypeAwareMutationConfig(),
		configPath: configPath,
	}
}

// LoadConfig 加载配置
func (cm *ConfigManager) LoadConfig() error {
	if cm.configPath == "" {
		fmt.Printf("⚠️  No config path provided, using default configuration\n")
		return cm.loadFromEnvironment()
	}

	// 检查配置文件是否存在
	if _, err := os.Stat(cm.configPath); os.IsNotExist(err) {
		fmt.Printf("⚠️  Config file not found: %s, creating default config\n", cm.configPath)
		return cm.SaveConfig()
	}

	// 读取配置文件
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	// 解析JSON配置
	var config TypeAwareMutationConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config file: %v", err)
	}

	cm.config = &config
	
	// 从环境变量补充API密钥
	err = cm.loadFromEnvironment()
	if err != nil {
		fmt.Printf("⚠️  Failed to load environment variables: %v\n", err)
	}

	fmt.Printf("✅ Configuration loaded from %s\n", cm.configPath)
	return nil
}

// SaveConfig 保存配置
func (cm *ConfigManager) SaveConfig() error {
	if cm.configPath == "" {
		return fmt.Errorf("no config path provided")
	}

	// 创建目录
	dir := filepath.Dir(cm.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	// 序列化配置
	data, err := json.MarshalIndent(cm.config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize config: %v", err)
	}

	// 写入文件
	if err := os.WriteFile(cm.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	fmt.Printf("✅ Configuration saved to %s\n", cm.configPath)
	return nil
}

// loadFromEnvironment 从环境变量加载配置
func (cm *ConfigManager) loadFromEnvironment() error {
	// 加载API密钥
	if etherscanKey := os.Getenv("ETHERSCAN_API_KEY"); etherscanKey != "" {
		if ethChain := cm.config.Chains["ethereum"]; ethChain != nil {
			ethChain.ExplorerAPIKey = etherscanKey
		}
	}
	
	if bscscanKey := os.Getenv("BSCSCAN_API_KEY"); bscscanKey != "" {
		if bscChain := cm.config.Chains["bsc"]; bscChain != nil {
			bscChain.ExplorerAPIKey = bscscanKey
		}
	}

	return nil
}

// GetConfig 获取配置
func (cm *ConfigManager) GetConfig() *TypeAwareMutationConfig {
	return cm.config
}

// GetChainConfig 获取指定链的配置
func (cm *ConfigManager) GetChainConfig(chainName string) (*ChainMutationConfig, bool) {
	config, exists := cm.config.Chains[chainName]
	return config, exists
}

// GetChainConfigByID 根据链ID获取配置
func (cm *ConfigManager) GetChainConfigByID(chainID int64) (*ChainMutationConfig, bool) {
	for _, config := range cm.config.Chains {
		if config.ChainID == chainID {
			return config, true
		}
	}
	return nil, false
}

// SetChainAPIKey 设置链的API密钥
func (cm *ConfigManager) SetChainAPIKey(chainName, apiKey string) error {
	if chain, exists := cm.config.Chains[chainName]; exists {
		chain.ExplorerAPIKey = apiKey
		return nil
	}
	return fmt.Errorf("chain %s not found", chainName)
}

// UpdateExecutionConfig 更新执行配置
func (cm *ConfigManager) UpdateExecutionConfig(execConfig ExecutionConfig) {
	cm.config.Execution = execConfig
}

// ValidateConfig 验证配置
func (cm *ConfigManager) ValidateConfig() error {
	if cm.config.MaxMutations <= 0 {
		return fmt.Errorf("maxMutations must be positive")
	}
	
	if cm.config.Execution.MaxConcurrentWorkers <= 0 {
		return fmt.Errorf("maxConcurrentWorkers must be positive")
	}
	
	if cm.config.Execution.BatchSize <= 0 {
		return fmt.Errorf("batchSize must be positive")
	}
	
	if cm.config.Execution.SimilarityThreshold < 0 || cm.config.Execution.SimilarityThreshold > 1 {
		return fmt.Errorf("similarityThreshold must be between 0 and 1")
	}
	
	// 验证每个链的配置
	for name, chainConfig := range cm.config.Chains {
		if chainConfig.ChainID <= 0 {
			return fmt.Errorf("invalid chainID for %s", name)
		}
		
		if chainConfig.ExplorerAPI == "" {
			return fmt.Errorf("explorerAPI is required for %s", name)
		}
	}
	
	return nil
}

// PrintConfig 打印配置摘要
func (cm *ConfigManager) PrintConfig() {
	fmt.Printf("=== MUTATION CONFIGURATION ===\n")
	fmt.Printf("Type-aware mutation: %v\n", cm.config.EnableTypeAware)
	fmt.Printf("Fallback to generic: %v\n", cm.config.FallbackToGeneric)
	fmt.Printf("Max mutations: %d\n", cm.config.MaxMutations)
	
	fmt.Printf("\n=== CHAIN CONFIGURATIONS ===\n")
	for name, chainConfig := range cm.config.Chains {
		fmt.Printf("Chain: %s (ID: %d)\n", name, chainConfig.ChainID)
		fmt.Printf("  Explorer API: %s\n", chainConfig.ExplorerAPI)
		fmt.Printf("  API Key: %s\n", maskAPIKey(chainConfig.ExplorerAPIKey))
		fmt.Printf("  Known addresses: %d\n", len(chainConfig.KnownAddresses))
		fmt.Printf("  Type-aware: %v\n", chainConfig.EnableTypeAware)
	}
	
	fmt.Printf("\n=== EXECUTION CONFIGURATION ===\n")
	fmt.Printf("Concurrent workers: %d\n", cm.config.Execution.MaxConcurrentWorkers)
	fmt.Printf("Batch size: %d\n", cm.config.Execution.BatchSize)
	fmt.Printf("Timeout: %ds\n", cm.config.Execution.TimeoutSeconds)
	fmt.Printf("Similarity threshold: %.2f\n", cm.config.Execution.SimilarityThreshold)
	fmt.Printf("Early pruning: %v\n", cm.config.Execution.EnableEarlyPruning)
	fmt.Printf("Cache size: %d\n", cm.config.Execution.CacheSize)
}

// maskAPIKey 遮蔽API密钥显示
func maskAPIKey(apiKey string) string {
	if apiKey == "" {
		return "not set"
	}
	if len(apiKey) <= 8 {
		return "***"
	}
	return apiKey[:4] + "***" + apiKey[len(apiKey)-4:]
}