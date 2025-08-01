package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigManager(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.json")
	
	// 创建配置管理器
	cm := NewConfigManager(configPath)
	
	// 测试默认配置
	config := cm.GetConfig()
	if config == nil {
		t.Fatal("Default config is nil")
	}
	
	if !config.EnableTypeAware {
		t.Error("Expected EnableTypeAware to be true")
	}
	
	if config.MaxMutations != 1000 {
		t.Errorf("Expected MaxMutations 1000, got %d", config.MaxMutations)
	}
	
	// 测试链配置
	ethConfig, exists := cm.GetChainConfig("ethereum")
	if !exists {
		t.Fatal("Ethereum config not found")
	}
	
	if ethConfig.ChainID != 1 {
		t.Errorf("Expected Ethereum chain ID 1, got %d", ethConfig.ChainID)
	}
	
	bscConfig, exists := cm.GetChainConfig("bsc")
	if !exists {
		t.Fatal("BSC config not found")
	}
	
	if bscConfig.ChainID != 56 {
		t.Errorf("Expected BSC chain ID 56, got %d", bscConfig.ChainID)
	}
	
	// 测试根据ID获取配置
	ethByID, exists := cm.GetChainConfigByID(1)
	if !exists {
		t.Fatal("Ethereum config not found by ID")
	}
	
	if ethByID.Name != "ethereum" {
		t.Errorf("Expected name 'ethereum', got '%s'", ethByID.Name)
	}
	
	// 测试保存配置
	err := cm.SaveConfig()
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}
	
	// 验证文件是否创建
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}
	
	// 测试加载配置
	cm2 := NewConfigManager(configPath)
	err = cm2.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	
	// 验证加载的配置
	loadedConfig := cm2.GetConfig()
	if loadedConfig.MaxMutations != config.MaxMutations {
		t.Error("Loaded config differs from original")
	}
	
	// 测试配置验证
	err = cm.ValidateConfig()
	if err != nil {
		t.Errorf("Config validation failed: %v", err)
	}
	
	// 测试API密钥设置
	err = cm.SetChainAPIKey("ethereum", "test_key_123")
	if err != nil {
		t.Errorf("Failed to set API key: %v", err)
	}
	
	ethConfig, _ = cm.GetChainConfig("ethereum")
	if ethConfig.ExplorerAPIKey != "test_key_123" {
		t.Error("API key was not set correctly")
	}
	
	t.Logf("✅ Config manager tests passed")
}

func TestEnvironmentVariables(t *testing.T) {
	// 设置环境变量
	os.Setenv("ETHERSCAN_API_KEY", "test_etherscan_key")
	os.Setenv("BSCSCAN_API_KEY", "test_bscscan_key")
	defer func() {
		os.Unsetenv("ETHERSCAN_API_KEY")
		os.Unsetenv("BSCSCAN_API_KEY")
	}()
	
	// 创建配置管理器
	cm := NewConfigManager("")
	err := cm.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config from environment: %v", err)
	}
	
	// 验证API密钥是否从环境变量加载
	ethConfig, _ := cm.GetChainConfig("ethereum")
	if ethConfig.ExplorerAPIKey != "test_etherscan_key" {
		t.Errorf("Expected Etherscan API key 'test_etherscan_key', got '%s'", ethConfig.ExplorerAPIKey)
	}
	
	bscConfig, _ := cm.GetChainConfig("bsc")
	if bscConfig.ExplorerAPIKey != "test_bscscan_key" {
		t.Errorf("Expected BscScan API key 'test_bscscan_key', got '%s'", bscConfig.ExplorerAPIKey)
	}
	
	t.Logf("✅ Environment variables tests passed")
}

func TestConfigValidation(t *testing.T) {
	cm := NewConfigManager("")
	
	// 测试无效配置
	config := cm.GetConfig()
	
	// 测试无效的最大变异数
	originalMaxMutations := config.MaxMutations
	config.MaxMutations = 0
	err := cm.ValidateConfig()
	if err == nil {
		t.Error("Expected validation error for maxMutations = 0")
	}
	config.MaxMutations = originalMaxMutations
	
	// 测试无效的并发工作者数量
	originalWorkers := config.Execution.MaxConcurrentWorkers
	config.Execution.MaxConcurrentWorkers = 0
	err = cm.ValidateConfig()
	if err == nil {
		t.Error("Expected validation error for maxConcurrentWorkers = 0")
	}
	config.Execution.MaxConcurrentWorkers = originalWorkers
	
	// 测试无效的相似度阈值
	originalThreshold := config.Execution.SimilarityThreshold
	config.Execution.SimilarityThreshold = 1.5
	err = cm.ValidateConfig()
	if err == nil {
		t.Error("Expected validation error for similarityThreshold > 1")
	}
	config.Execution.SimilarityThreshold = originalThreshold
	
	// 测试有效配置
	err = cm.ValidateConfig()
	if err != nil {
		t.Errorf("Valid config failed validation: %v", err)
	}
	
	t.Logf("✅ Config validation tests passed")
}

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "not set"},
		{"short", "***"},
		{"1234567890abcdef", "1234***cdef"},
		{"very_long_api_key_for_testing", "very***ting"},
	}
	
	for _, test := range tests {
		result := maskAPIKey(test.input)
		if result != test.expected {
			t.Errorf("maskAPIKey(%s) = %s, expected %s", test.input, result, test.expected)
		}
	}
	
	t.Logf("✅ API key masking tests passed")
}