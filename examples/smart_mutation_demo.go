package main

import (
	"fmt"
	"log"
	"math/big"
	"os"

	"github.com/DQYXACML/autopatch/tracing/abi"
	"github.com/DQYXACML/autopatch/tracing/analysis"
	"github.com/DQYXACML/autopatch/tracing/config"
	"github.com/DQYXACML/autopatch/tracing/mutation"
	"github.com/ethereum/go-ethereum/common"
)

func main() {
	fmt.Printf("=== 智能变异系统演示 ===\n\n")

	// 1. 创建配置管理器
	configManager := config.NewConfigManager("./config/mutation_config.json")
	
	// 设置环境变量（演示用）
	os.Setenv("ETHERSCAN_API_KEY", "YOUR_ETHERSCAN_KEY")
	os.Setenv("BSCSCAN_API_KEY", "YOUR_BSCSCAN_KEY")
	
	// 加载配置
	if err := configManager.LoadConfig(); err != nil {
		log.Printf("Failed to load config: %v, using defaults", err)
	}
	
	// 打印配置
	configManager.PrintConfig()

	// 2. 创建ABI管理器
	abiManager := abi.NewABIManager("./abi_cache")
	chainID := big.NewInt(1) // Ethereum mainnet
	
	fmt.Printf("\n=== ABI管理器演示 ===\n")
	
	// 获取USDC合约的ABI（如果可用）
	usdcAddr := common.HexToAddress("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")
	if abi, err := abiManager.GetContractABI(chainID, usdcAddr); err == nil {
		fmt.Printf("✅ Successfully retrieved ABI for USDC contract\n")
		fmt.Printf("   Methods: %d\n", len(abi.Methods))
		fmt.Printf("   Events: %d\n", len(abi.Events))
	} else {
		fmt.Printf("⚠️  Could not retrieve ABI for USDC: %v\n", err)
	}

	// 3. 创建类型感知变异器
	fmt.Printf("\n=== 类型感知变异器演示 ===\n")
	
	typeAwareMutator := mutation.NewTypeAwareMutator(chainID, abiManager)
	
	// 演示地址变异
	originalAddr := common.HexToAddress("0x742d35Cc6634C0532925a3b8D5c5C1c1b5c85C7c")
	mutatedAddr, err := typeAwareMutator.MutateAddress(originalAddr, 0)
	if err == nil {
		fmt.Printf("地址变异: %s -> %s\n", originalAddr.Hex(), mutatedAddr.Hex())
	}
	
	// 演示大整数变异
	originalValue := big.NewInt(1000000000000000000) // 1 ETH in wei
	mutatedValue := typeAwareMutator.MutateBigUint(originalValue, 1)
	fmt.Printf("数值变异: %s -> %s\n", originalValue.String(), mutatedValue.String())
	
	// 演示字符串变异
	originalString := "Hello"
	mutatedString := typeAwareMutator.MutateString(originalString, 2)
	fmt.Printf("字符串变异: '%s' -> '%s'\n", originalString, mutatedString)

	// 4. 创建Storage分析器
	fmt.Printf("\n=== Storage分析器演示 ===\n")
	
	storageAnalyzer := analysis.NewStorageAnalyzer(abiManager, chainID)
	
	// 创建示例存储数据
	sampleStorage := map[common.Hash]common.Hash{
		// 地址槽位
		common.BigToHash(big.NewInt(0)): common.HexToHash("0x000000000000000000000000742d35Cc6634C0532925a3b8D5c5C1c1b5c85C7c"),
		// 余额槽位
		common.BigToHash(big.NewInt(1)): common.HexToHash("0x0000000000000000000000000000000000000000000000000de0b6b3a7640000"), // 1 ETH
		// 布尔槽位
		common.BigToHash(big.NewInt(2)): common.BigToHash(big.NewInt(1)),
		// 计数器槽位
		common.BigToHash(big.NewInt(3)): common.BigToHash(big.NewInt(100)),
	}
	
	contractAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	
	// 分析存储
	slotInfos, err := storageAnalyzer.AnalyzeContractStorage(contractAddr, sampleStorage)
	if err != nil {
		log.Printf("Storage analysis failed: %v", err)
		return
	}
	
	fmt.Printf("分析了 %d 个存储槽:\n", len(slotInfos))
	for _, slotInfo := range slotInfos {
		fmt.Printf("  槽位 %s: 类型=%s, 重要性=%.2f, 策略=%d个\n",
			slotInfo.Slot.Hex()[:10]+"...",
			slotInfo.SlotType,
			slotInfo.ImportanceScore,
			len(slotInfo.MutationStrategies))
	}

	// 5. 创建Storage变异器并演示变异
	fmt.Printf("\n=== Storage变异器演示 ===\n")
	
	storageTypeMutator := analysis.NewStorageTypeMutator(storageAnalyzer, typeAwareMutator)
	
	// 变异存储
	mutatedStorage, err := storageTypeMutator.MutateStorage(contractAddr, sampleStorage, 0)
	if err != nil {
		log.Printf("Storage mutation failed: %v", err)
		return
	}
	
	fmt.Printf("存储变异结果:\n")
	for slot, originalValue := range sampleStorage {
		if mutatedValue, exists := mutatedStorage[slot]; exists && mutatedValue != originalValue {
			fmt.Printf("  槽位 %s: %s -> %s\n",
				slot.Hex()[:10]+"...",
				originalValue.Hex()[:10]+"...",
				mutatedValue.Hex()[:10]+"...")
		}
	}

	// 6. 显示缓存统计
	fmt.Printf("\n=== 缓存统计 ===\n")
	stats := abiManager.GetCacheStats()
	fmt.Printf("ABI缓存: 内存中%d个, 文件中%d个\n", 
		stats["memory_cache_size"], stats["file_cache_size"])

	fmt.Printf("\n=== 演示完成 ===\n")
	fmt.Printf("✅ 智能变异系统已成功实现以下功能:\n")
	fmt.Printf("   1. 双链ABI管理（Ethereum + BSC）\n")
	fmt.Printf("   2. 类型感知的参数变异\n")
	fmt.Printf("   3. 智能Storage分析和变异\n")
	fmt.Printf("   4. 配置化的变异策略\n")
	fmt.Printf("   5. 缓存优化和性能提升\n")
}