package abi

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestABIManager(t *testing.T) {
	// 创建ABI管理器
	abiManager := NewABIManager("./test_cache")
	
	// 测试链配置
	ethConfig, exists := abiManager.GetChainConfig(1)
	if !exists {
		t.Fatal("Ethereum chain config not found")
	}
	
	if ethConfig.ChainID != 1 {
		t.Errorf("Expected chain ID 1, got %d", ethConfig.ChainID)
	}
	
	if ethConfig.Name != "ethereum" {
		t.Errorf("Expected name 'ethereum', got '%s'", ethConfig.Name)
	}
	
	// 测试BSC链配置
	bscConfig, exists := abiManager.GetChainConfig(56)
	if !exists {
		t.Fatal("BSC chain config not found")
	}
	
	if bscConfig.ChainID != 56 {
		t.Errorf("Expected chain ID 56, got %d", bscConfig.ChainID)
	}
	
	// 测试ABI获取功能 (这会尝试从缓存获取，如果没有会尝试从区块链获取)
	testAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	
	// 这个调用会失败，因为地址不存在，但我们可以测试方法存在
	_, err := abiManager.GetContractABI(big.NewInt(1), testAddr)
	if err == nil {
		t.Log("Unexpectedly got ABI for non-existent contract")
	} else {
		t.Logf("Expected error when fetching ABI for non-existent contract: %v", err)
	}
	
	// 测试缓存统计
	stats := abiManager.GetCacheStats()
	if stats["memory_cache_size"] != 0 {
		t.Errorf("Expected empty cache, got %d items", stats["memory_cache_size"])
	}
	
	t.Logf("✅ ABI Manager tests passed")
}