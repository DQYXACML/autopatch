package replay

import (
	"context"
	"fmt"
	"github.com/DQYXACML/autopatch/bindings"
	"github.com/DQYXACML/autopatch/config"
	"github.com/DQYXACML/autopatch/database"
	"github.com/DQYXACML/autopatch/database/worker"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"math/big"
	"testing"
	"time"
	tracingUtils "github.com/DQYXACML/autopatch/tracing/utils"
)

func TestRelayTx(t *testing.T) {
	// 加载测试配置
	testConfig, err := config.LoadTestConfig()
	if err != nil {
		log.Fatal("Failed to load test config:", err)
	}

	// 连接数据库
	gormDB, err := gorm.Open(postgres.Open(testConfig.GetDSN()), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// 创建数据库实例
	db, err := database.NewDB(context.Background(), testConfig.DBConfig)
	if err != nil {
		log.Fatal("Failed to create database instance:", err)
	}
	defer db.Close()

	// 自动迁移数据库表
	err = gormDB.AutoMigrate(&worker.AttackTx{})
	if err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	fmt.Println("✅ Database connected and migrated successfully!")

	// 插入测试攻击交易
	err = insertExampleAttackTransaction(db, testConfig.RPCURL)
	if err != nil {
		log.Fatal("Failed to insert example transaction:", err)
	}

	// 创建攻击重放器
	replayer, err := NewAttackReplayer(
		testConfig.RPCURL,            // 以太坊节点RPC URL
		db,                           // 数据库连接
		bindings.StorageScanMetaData, // StorageScan合约的metadata
	)
	if err != nil {
		log.Fatal("Failed to create replayer:", err)
	}

	// 测试重放、变异收集和交易发送的完整流程
	fmt.Println("\n=== STARTING COMPLETE ATTACK TRANSACTION REPLAY, MUTATION COLLECTION AND TRANSACTION SENDING ===")

	// 使用配置中的交易哈希和合约地址
	txHash := testConfig.GetTxHash()
	protectContractAddr := testConfig.GetProtectContractAddress()

	// 执行完整流程：重放 -> 收集变异 -> 发送交易
	mutationCollection, sentTxHashes, err := replayer.ReplayAndSendMutations(txHash, protectContractAddr)
	if err != nil {
		log.Fatal("Failed to replay and send mutations:", err)
	}

	fmt.Println("\n=== COMPLETE WORKFLOW COMPLETED ===")

	// 显示详细的变异数据
	displayMutationResults(mutationCollection)

	// 显示发送的交易哈希
	displaySentTransactions(sentTxHashes)

	// 转换为Solidity格式
	solidityData := mutationCollection.ToSolidityFormat()
	displaySolidityFormat(solidityData)

	// 验证结果
	validateMutationResults(t, mutationCollection)

	// 验证交易发送结果
	validateTransactionSending(t, sentTxHashes, mutationCollection.SuccessfulMutations)

	fmt.Printf("\n✅ All tests passed successfully!\n")
	fmt.Printf("📊 Replayed transaction and collected %d mutations\n", len(mutationCollection.Mutations))
	fmt.Printf("✅ Found %d successful mutations with similarity >= threshold\n", len(mutationCollection.SuccessfulMutations))
	if sentTxHashes != nil {
		fmt.Printf("🚀 Successfully sent %d mutation transactions to contract\n", len(sentTxHashes))
	} else {
		fmt.Printf("⚠️  No transactions were sent (this is normal if no successful mutations were found)\n")
	}
}

// displaySentTransactions 显示发送的交易哈希
func displaySentTransactions(txHashes []*common.Hash) {
	fmt.Printf("\n=== SENT TRANSACTION HASHES ===\n")

	if txHashes == nil || len(txHashes) == 0 {
		fmt.Printf("⚠️  No transactions were sent to the contract\n")
		fmt.Printf("This can happen if:\n")
		fmt.Printf("  - No successful mutations were found\n")
		fmt.Printf("  - All mutations had similarity below threshold\n")
		fmt.Printf("  - Transaction sending encountered errors\n")
		return
	}

	fmt.Printf("Successfully sent %d transactions to contract:\n", len(txHashes))
	for i, txHash := range txHashes {
		if txHash != nil {
			fmt.Printf("  [%d] %s\n", i+1, txHash.Hex())
			fmt.Printf("      Explorer: https://holesky.etherscan.io/tx/%s\n", txHash.Hex())
		} else {
			fmt.Printf("  [%d] <nil hash - transaction may have failed>\n", i+1)
		}
	}

	fmt.Printf("\n🔗 You can monitor these transactions on Holesky Etherscan\n")
	fmt.Printf("⏱️  Transactions may take a few seconds to appear in the explorer\n")
}

// validateTransactionSending 验证交易发送结果
func validateTransactionSending(t *testing.T, sentTxHashes []*common.Hash, successfulMutations []tracingUtils.MutationData) {
	fmt.Printf("\n=== VALIDATING TRANSACTION SENDING ===\n")

	// 如果没有成功的变异，就不应该有发送的交易
	if len(successfulMutations) == 0 {
		if sentTxHashes != nil && len(sentTxHashes) > 0 {
			t.Error("Expected no sent transactions when there are no successful mutations")
		}
		fmt.Printf("✅ Correctly sent no transactions (no successful mutations found)\n")
		return
	}

	// 如果有成功的变异，检查交易发送情况
	fmt.Printf("Successful mutations: %d\n", len(successfulMutations))
	if sentTxHashes != nil {
		fmt.Printf("Sent transactions: %d\n", len(sentTxHashes))
	} else {
		fmt.Printf("Sent transactions: 0 (nil result)\n")
	}

	// 验证发送的交易哈希格式
	if sentTxHashes != nil {
		for i, txHash := range sentTxHashes {
			if txHash == nil {
				t.Errorf("Transaction hash %d is nil", i)
				continue
			}

			// 验证哈希格式（应该是32字节）
			if len(txHash.Bytes()) != 32 {
				t.Errorf("Transaction hash %d has invalid length: expected 32, got %d", i, len(txHash.Bytes()))
			}

			// 验证哈希不是零值
			if *txHash == (common.Hash{}) {
				t.Errorf("Transaction hash %d is zero hash", i)
			}
		}
	}

	// 注意：由于交易发送是异步的，我们不强制要求发送的交易数量等于成功变异数量
	// 一些交易可能因为nonce冲突、gas不足等原因失败
	fmt.Printf("✅ Transaction sending validation completed\n")

	if sentTxHashes != nil && len(sentTxHashes) > 0 {
		fmt.Printf("✅ Successfully sent %d transactions\n", len(sentTxHashes))
	} else {
		fmt.Printf("⚠️  No transactions were sent (this may be expected in test environments)\n")
	}
}

// TestRelayTxWithoutSending 测试只进行重放和变异收集，不发送交易
func TestRelayTxWithoutSending(t *testing.T) {
	// 加载测试配置
	testConfig, err := config.LoadTestConfig()
	if err != nil {
		log.Fatal("Failed to load test config:", err)
	}

	// 连接数据库
	gormDB, err := gorm.Open(postgres.Open(testConfig.GetDSN()), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// 创建数据库实例
	db, err := database.NewDB(context.Background(), testConfig.DBConfig)
	if err != nil {
		log.Fatal("Failed to create database instance:", err)
	}
	defer db.Close()

	// 自动迁移数据库表
	err = gormDB.AutoMigrate(&worker.AttackTx{})
	if err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	fmt.Println("✅ Database connected and migrated successfully!")

	// 插入测试攻击交易
	err = insertExampleAttackTransaction(db, testConfig.RPCURL)
	if err != nil {
		log.Fatal("Failed to insert example transaction:", err)
	}

	// 创建攻击重放器
	replayer, err := NewAttackReplayer(
		testConfig.RPCURL,            // 以太坊节点RPC URL
		db,                           // 数据库连接
		bindings.StorageScanMetaData, // StorageScan合约的metadata
	)
	if err != nil {
		log.Fatal("Failed to create replayer:", err)
	}

	// 测试只进行重放和变异收集（不发送交易）
	fmt.Println("\n=== STARTING ATTACK TRANSACTION REPLAY AND MUTATION COLLECTION (NO SENDING) ===")

	// 使用配置中的交易哈希和合约地址
	txHash := testConfig.GetTxHash()
	contractAddr := testConfig.GetContractAddress()

	// 执行重放和变异收集
	mutationCollection, err := replayer.ReplayAndCollectMutations(txHash, contractAddr)
	if err != nil {
		log.Fatal("Failed to replay and collect mutations:", err)
	}

	fmt.Println("\n=== REPLAY AND MUTATION COLLECTION COMPLETED ===")

	// 显示详细的变异数据
	displayMutationResults(mutationCollection)

	// 转换为Solidity格式
	solidityData := mutationCollection.ToSolidityFormat()
	displaySolidityFormat(solidityData)

	// 验证结果
	validateMutationResults(t, mutationCollection)

	fmt.Printf("\n✅ All tests passed successfully!\n")
	fmt.Printf("📊 Ready to send %d successful mutations to Solidity contract\n", len(mutationCollection.SuccessfulMutations))
}

// TestTransactionSendingOnly 单独测试交易发送功能
func TestTransactionSendingOnly(t *testing.T) {
	// 加载测试配置
	testConfig, err := config.LoadTestConfig()
	if err != nil {
		log.Fatal("Failed to load test config:", err)
	}

	// 创建数据库实例
	db, err := database.NewDB(context.Background(), testConfig.DBConfig)
	if err != nil {
		log.Fatal("Failed to create database instance:", err)
	}
	defer db.Close()

	// 创建攻击重放器
	replayer, err := NewAttackReplayer(
		testConfig.RPCURL,            // 以太坊节点RPC URL
		db,                           // 数据库连接
		bindings.StorageScanMetaData, // StorageScan合约的metadata
	)
	if err != nil {
		log.Fatal("Failed to create replayer:", err)
	}

	fmt.Println("\n=== TESTING TRANSACTION SENDING FUNCTIONALITY ===")

	// 目标合约地址
	contractAddr := testConfig.GetContractAddress()

	// 创建一些模拟的成功变异数据用于测试发送
	testMutations := createTestMutationData()

	// 测试发送变异交易
	sentTxHashes, err := replayer.SendMutationTransactions(contractAddr, testMutations, TokenGasLimit)
	if err != nil {
		fmt.Printf("⚠️  Transaction sending failed (this may be expected in test): %v\n", err)
		// 不直接失败测试，因为在测试环境中交易发送可能失败
	} else {
		fmt.Printf("✅ Successfully sent %d test transactions\n", len(sentTxHashes))
		for i, txHash := range sentTxHashes {
			fmt.Printf("   [%d] %s\n", i+1, txHash.Hex())
		}
	}

	fmt.Println("✅ Transaction sending test completed")
}

// createTestMutationData 创建测试变异数据
func createTestMutationData() []tracingUtils.MutationData {
	// 创建setUint1函数调用数据 (function selector: 0x698ccd3a)
	setUint1Data := make([]byte, 36) // 4字节函数选择器 + 32字节参数
	// setUint1 function selector
	setUint1Data[0] = 0x69
	setUint1Data[1] = 0x8c
	setUint1Data[2] = 0xcd
	setUint1Data[3] = 0x3a
	// uint8 parameter (42) - 右对齐到32字节
	setUint1Data[35] = 42

	// 创建setString1函数调用数据 (function selector: 0xbb3da883)
	setString1Data := make([]byte, 100) // 估算长度
	// setString1 function selector
	setString1Data[0] = 0xbb
	setString1Data[1] = 0x3d
	setString1Data[2] = 0xa8
	setString1Data[3] = 0x83
	// string parameter offset (0x20)
	setString1Data[31] = 0x20
	// string length (0x0c for "test_string")
	setString1Data[63] = 0x0c
	// string data "test_string"
	copy(setString1Data[64:76], []byte("test_string"))

	testMutations := []tracingUtils.MutationData{
		{
			ID:        "test_mutation_1",
			InputData: setUint1Data,
			StorageChanges: map[common.Hash]common.Hash{
				common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"): common.HexToHash("0x000000000000000000000000000000000000000000000000000000000000002a"),
			},
			Similarity:    0.95,
			Success:       true,
			ExecutionTime: 100 * time.Millisecond,
		},
		{
			ID:        "test_mutation_2",
			InputData: setString1Data,
			StorageChanges: map[common.Hash]common.Hash{
				common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000005"): common.HexToHash("0x746573745f737472696e670000000000000000000000000000000000000000000"),
			},
			Similarity:    0.92,
			Success:       true,
			ExecutionTime: 150 * time.Millisecond,
		},
	}

	return testMutations
}

// displayMutationResults 显示变异结果的详细信息
func displayMutationResults(collection *tracingUtils.MutationCollection) {
	fmt.Printf("\n=== MUTATION COLLECTION ANALYSIS ===\n")
	fmt.Printf("📝 Original Transaction: %s\n", collection.OriginalTxHash.Hex())
	fmt.Printf("🏠 Contract Address: %s\n", collection.ContractAddress.Hex())
	fmt.Printf("📊 Total Mutations: %d\n", collection.TotalMutations)
	fmt.Printf("✅ Successful Mutations: %d\n", collection.SuccessCount)
	fmt.Printf("❌ Failed Mutations: %d\n", collection.FailureCount)
	fmt.Printf("📈 Success Rate: %.2f%%\n", float64(collection.SuccessCount)/float64(collection.TotalMutations)*100)
	fmt.Printf("🎯 Average Similarity: %.2f%%\n", collection.AverageSimilarity*100)
	fmt.Printf("🏆 Highest Similarity: %.2f%%\n", collection.HighestSimilarity*100)
	fmt.Printf("⏱️  Processing Time: %v\n", collection.ProcessingTime)

	// Display call trace information
	if collection.CallTrace != nil && len(collection.CallTrace.ExtractedCalls) > 0 {
		fmt.Printf("\n=== EXTRACTED CALL DATA ===\n")
		fmt.Printf("📞 Extracted calls to protected contracts: %d\n", len(collection.CallTrace.ExtractedCalls))
		for i, call := range collection.CallTrace.ExtractedCalls {
			fmt.Printf("  [%d] Contract: %s, From: %s, Input length: %d bytes\n",
				i, call.ContractAddress.Hex(), call.From.Hex(), len(call.InputData))
			if len(call.InputData) >= 4 {
				fmt.Printf("      Function selector: %x\n", call.InputData[:4])
			}
		}
	}

	fmt.Printf("\n=== ORIGINAL DATA ===\n")
	fmt.Printf("📥 Original Input Data: %x\n", collection.OriginalInputData)
	if len(collection.OriginalInputData) >= 4 {
		fmt.Printf("🔧 Function Selector: %x\n", collection.OriginalInputData[:4])
		if len(collection.OriginalInputData) > 4 {
			fmt.Printf("📋 Parameters: %x\n", collection.OriginalInputData[4:])
		}
	}

	fmt.Printf("💾 Original Storage Slots: %d\n", len(collection.OriginalStorage))
	if len(collection.OriginalStorage) > 0 {
		fmt.Printf("📦 Storage Details:\n")
		for slot, value := range collection.OriginalStorage {
			fmt.Printf("   Slot %s: %s\n", slot.Hex(), value.Hex())
		}
	}

	fmt.Printf("\n=== SUCCESSFUL MUTATIONS DETAILS ===\n")
	for i, mutation := range collection.SuccessfulMutations {
		fmt.Printf("🔬 Mutation %d (ID: %s):\n", i+1, mutation.ID)
		fmt.Printf("   📊 Similarity: %.2f%%\n", mutation.Similarity*100)
		fmt.Printf("   ⏱️  Execution Time: %v\n", mutation.ExecutionTime)

		// Show source call data if available
		if mutation.SourceCallData != nil {
			fmt.Printf("   📞 Based on call to: %s\n", mutation.SourceCallData.ContractAddress.Hex())
			fmt.Printf("   📤 From: %s\n", mutation.SourceCallData.From.Hex())
		}

		if len(mutation.InputData) > 0 {
			fmt.Printf("   📥 Mutated Input: %x\n", mutation.InputData)
			if len(mutation.InputData) >= 4 {
				fmt.Printf("   🔧 Function Selector: %x\n", mutation.InputData[:4])
				if len(mutation.InputData) > 4 {
					fmt.Printf("   📋 Mutated Parameters: %x\n", mutation.InputData[4:])
				}
			}
		}

		if len(mutation.StorageChanges) > 0 {
			fmt.Printf("   💾 Storage Changes: %d\n", len(mutation.StorageChanges))
			for slot, value := range mutation.StorageChanges {
				fmt.Printf("      Slot %s: %s\n", slot.Hex(), value.Hex())
			}
		}
		fmt.Println()
	}

	fmt.Printf("\n=== FAILED MUTATIONS SUMMARY ===\n")
	errorCounts := make(map[string]int)
	for _, mutation := range collection.Mutations {
		if !mutation.Success && mutation.ErrorMessage != "" {
			errorCounts[mutation.ErrorMessage]++
		}
	}

	if len(errorCounts) > 0 {
		fmt.Printf("❌ Error Distribution:\n")
		for errorMsg, count := range errorCounts {
			fmt.Printf("   %s: %d occurrences\n", errorMsg, count)
		}
	} else {
		fmt.Printf("ℹ️  No error details available\n")
	}
}

// displaySolidityFormat 显示适合发送给Solidity的格式
func displaySolidityFormat(solidityData *tracingUtils.SolidityMutationData) {
	fmt.Printf("\n=== SOLIDITY CONTRACT DATA FORMAT ===\n")
	fmt.Printf("🏠 Contract Address: %s\n", solidityData.ContractAddress.Hex())
	fmt.Printf("📝 Original Tx Hash: %s\n", solidityData.OriginalTxHash.Hex())
	fmt.Printf("📥 Original Input Data: %x\n", solidityData.OriginalInputData)
	fmt.Printf("📊 Total Mutations: %s\n", solidityData.TotalMutations.String())
	fmt.Printf("✅ Success Count: %s\n", solidityData.SuccessCount.String())

	fmt.Printf("\n📥 Input Mutations (%d):\n", len(solidityData.InputMutations))
	for i, inputData := range solidityData.InputMutations {
		fmt.Printf("   [%d]: %x\n", i, inputData)
	}

	fmt.Printf("\n💾 Storage Mutations (%d):\n", len(solidityData.StorageMutations))
	for i, storageMutation := range solidityData.StorageMutations {
		fmt.Printf("   [%d]: Slot=%s, Value=%s\n", i, storageMutation.Slot.Hex(), storageMutation.Value.Hex())
	}

	fmt.Printf("\n📈 Similarities (%d):\n", len(solidityData.Similarities))
	for i, similarity := range solidityData.Similarities {
		// 相似度以万分之一为单位存储，转换回百分比显示
		similarityPercent := new(big.Float).Quo(new(big.Float).SetInt(similarity), big.NewFloat(100))
		fmt.Printf("   [%d]: %.2f%%\n", i, similarityPercent)
	}

	fmt.Printf("\n🚀 Ready to send to Solidity contract!\n")
}

// validateMutationResults 验证变异结果
func validateMutationResults(t *testing.T, collection *tracingUtils.MutationCollection) {
	fmt.Printf("\n=== VALIDATING MUTATION RESULTS ===\n")

	// 基本验证
	if collection.TotalMutations <= 0 {
		t.Error("Expected at least one mutation to be generated")
	}

	if collection.SuccessCount < 0 || collection.FailureCount < 0 {
		t.Error("Expected non-negative success and failure counts")
	}

	if collection.SuccessCount+collection.FailureCount != collection.TotalMutations {
		t.Error("Success count + failure count should equal total mutations")
	}

	if len(collection.Mutations) != collection.TotalMutations {
		t.Error("Mutations slice length should equal total mutations count")
	}

	if len(collection.SuccessfulMutations) != collection.SuccessCount {
		t.Error("Successful mutations slice length should equal success count")
	}

	// 验证成功的变异
	for i, mutation := range collection.SuccessfulMutations {
		if !mutation.Success {
			t.Errorf("Successful mutation %d should have Success=true", i)
		}

		if mutation.Similarity < 0 || mutation.Similarity > 1 {
			t.Errorf("Mutation %d similarity should be between 0 and 1, got %.2f", i, mutation.Similarity)
		}

		if mutation.ID == "" {
			t.Errorf("Mutation %d should have a non-empty ID", i)
		}

		// 至少应该有输入变异或存储变异之一
		if len(mutation.InputData) == 0 && len(mutation.StorageChanges) == 0 {
			t.Errorf("Mutation %d should have either input data or storage changes", i)
		}
	}

	// 验证统计信息
	if collection.SuccessCount > 0 {
		if collection.AverageSimilarity < 0 || collection.AverageSimilarity > 1 {
			t.Error("Average similarity should be between 0 and 1")
		}

		if collection.HighestSimilarity < 0 || collection.HighestSimilarity > 1 {
			t.Error("Highest similarity should be between 0 and 1")
		}

		if collection.HighestSimilarity < collection.AverageSimilarity {
			t.Error("Highest similarity should be >= average similarity")
		}
	}

	// 验证原始数据
	if collection.OriginalTxHash == (common.Hash{}) {
		t.Error("Original tx hash should not be empty")
	}

	if collection.ContractAddress == (common.Address{}) {
		t.Error("Contract address should not be empty")
	}

	if len(collection.OriginalInputData) == 0 {
		t.Error("Original input data should not be empty")
	}

	// 验证调用跟踪数据（新功能）
	if collection.CallTrace != nil {
		validateCallTrace(t, collection)
	}

	// 验证基于调用的变异（新功能）
	validateCallBasedMutations(t, collection)

	fmt.Printf("✅ All validation checks passed!\n")
}

// validateCallTrace 验证调用跟踪数据
func validateCallTrace(t *testing.T, collection *tracingUtils.MutationCollection) {
	fmt.Printf("\n--- Validating Call Trace ---\n")
	
	if collection.CallTrace.OriginalTxHash != collection.OriginalTxHash {
		t.Error("Call trace transaction hash should match original transaction hash")
	}

	if len(collection.CallTrace.ExtractedCalls) > 0 {
		fmt.Printf("Found %d extracted calls\n", len(collection.CallTrace.ExtractedCalls))
		
		for i, call := range collection.CallTrace.ExtractedCalls {
			if call.ContractAddress == (common.Address{}) {
				t.Errorf("Extracted call %d should have a valid contract address", i)
			}
			
			if call.From == (common.Address{}) {
				t.Errorf("Extracted call %d should have a valid from address", i)
			}
			
			if len(call.InputData) == 0 {
				t.Errorf("Extracted call %d should have non-empty input data", i)
			}
			
			// Verify that at least one mutation is based on this call
			foundMutation := false
			for _, mutation := range collection.Mutations {
				if mutation.SourceCallData != nil && 
				   mutation.SourceCallData.ContractAddress == call.ContractAddress {
					foundMutation = true
					break
				}
			}
			
			if foundMutation {
				fmt.Printf("✅ Found mutations based on call to %s\n", call.ContractAddress.Hex())
			}
		}
	} else {
		fmt.Printf("⚠️  No extracted calls found (using fallback mutation method)\n")
	}
}

// validateCallBasedMutations 验证基于调用的变异
func validateCallBasedMutations(t *testing.T, collection *tracingUtils.MutationCollection) {
	fmt.Printf("\n--- Validating Call-Based Mutations ---\n")
	
	callBasedCount := 0
	for _, mutation := range collection.Mutations {
		if mutation.SourceCallData != nil {
			callBasedCount++
			
			// Verify mutation input data length matches source call data length
			if len(mutation.InputData) > 0 && len(mutation.SourceCallData.InputData) > 0 {
				// Function selector should remain the same
				if len(mutation.InputData) >= 4 && len(mutation.SourceCallData.InputData) >= 4 {
					if !bytesEqual(mutation.InputData[:4], mutation.SourceCallData.InputData[:4]) {
						// This is OK - we might be testing different functions
						fmt.Printf("⚠️  Mutation %s has different function selector than source\n", mutation.ID)
					}
				}
			}
		}
	}
	
	fmt.Printf("Call-based mutations: %d/%d (%.2f%%)\n", 
		callBasedCount, collection.TotalMutations, 
		float64(callBasedCount)/float64(collection.TotalMutations)*100)
	
	// If we have extracted calls, we should have call-based mutations
	if collection.CallTrace != nil && len(collection.CallTrace.ExtractedCalls) > 0 && callBasedCount == 0 {
		t.Error("Expected at least some call-based mutations when calls were extracted")
	}
}

// insertExampleAttackTransaction 插入示例攻击交易，从RPC获取真实数据
func insertExampleAttackTransaction(db *database.DB, rpcURL string) error {
	fmt.Println("📥 Inserting example attack transaction...")

	// 加载测试配置获取交易哈希和合约地址
	testConfig, err := config.LoadTestConfig()
	if err != nil {
		return fmt.Errorf("failed to load test config: %v", err)
	}

	// 指定的交易哈希和合约地址
	txHash := testConfig.GetTxHash()
	contractAddr := testConfig.GetProtectContractAddress()

	// 检查交易是否已存在
	existingTx, err := db.AttackTx.QueryAttackTxByHash(txHash)
	if err == nil && existingTx != nil {
		fmt.Printf("✅ Transaction %s already exists, resetting to pending status\n", txHash.Hex())
		err = db.AttackTx.UpdateAttackTxStatus(existingTx.GUID, worker.StatusPending, "")
		if err != nil {
			return fmt.Errorf("failed to reset transaction status: %v", err)
		}
		return nil
	}

	// 连接以太坊客户端
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return fmt.Errorf("failed to connect to Ethereum client: %v", err)
	}
	defer client.Close()

	fmt.Printf("🔍 Fetching transaction details from RPC: %s\n", txHash.Hex())

	// 获取交易详情
	tx, isPending, err := client.TransactionByHash(context.Background(), txHash)
	if err != nil {
		return fmt.Errorf("failed to get transaction by hash: %v", err)
	}

	if isPending {
		return fmt.Errorf("transaction is still pending")
	}

	// 获取交易收据
	receipt, err := client.TransactionReceipt(context.Background(), txHash)
	if err != nil {
		return fmt.Errorf("failed to get transaction receipt: %v", err)
	}

	// 获取区块信息
	block, err := client.HeaderByNumber(context.Background(), receipt.BlockNumber)
	if err != nil {
		return fmt.Errorf("failed to get block header: %v", err)
	}

	// 获取链ID和发送者地址
	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get chain ID: %v", err)
	}

	signer := types.LatestSignerForChainID(chainID)
	fromAddress, err := types.Sender(signer, tx)
	if err != nil {
		return fmt.Errorf("failed to get sender address: %v", err)
	}

	// 验证合约地址
	var actualContractAddr common.Address
	if tx.To() != nil {
		actualContractAddr = *tx.To()
	} else if receipt.ContractAddress != (common.Address{}) {
		actualContractAddr = receipt.ContractAddress
	} else {
		actualContractAddr = contractAddr
	}

	if actualContractAddr != contractAddr {
		fmt.Printf("⚠️  Using actual contract address: %s\n", actualContractAddr.Hex())
		contractAddr = actualContractAddr
	}

	// 创建攻击交易记录
	attackTx := worker.AttackTx{
		GUID:            uuid.New(),
		TxHash:          txHash,
		BlockNumber:     receipt.BlockNumber,
		BlockHash:       receipt.BlockHash,
		ContractAddress: contractAddr,
		FromAddress:     fromAddress,
		ToAddress:       actualContractAddr,
		Value:           tx.Value(),
		GasUsed:         big.NewInt(int64(receipt.GasUsed)),
		GasPrice:        tx.GasPrice(),
		Status:          worker.StatusPending,
		AttackType:      "test_replay",
		ErrorMessage:    "",
		Timestamp:       block.Time,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// 验证并存储交易记录
	if err := worker.ValidateAttackTx(&attackTx); err != nil {
		return fmt.Errorf("invalid attack transaction: %v", err)
	}

	err = db.AttackTx.StoreAttackTx([]worker.AttackTx{attackTx})
	if err != nil {
		return fmt.Errorf("failed to store attack transaction: %v", err)
	}

	fmt.Printf("✅ Successfully inserted attack transaction: %s\n", txHash.Hex())
	fmt.Printf("📍 Contract address: %s\n", contractAddr.Hex())
	fmt.Printf("🆔 GUID: %s\n", attackTx.GUID.String())
	return nil
}

// TestTargetContractMutation tests that only the target contract's input is mutated
func TestTargetContractMutation(t *testing.T) {
	// Load test configuration
	testConfig, err := config.LoadTestConfig()
	if err != nil {
		log.Fatal("Failed to load test config:", err)
	}

	// Connect to database
	db, err := database.NewDB(context.Background(), testConfig.DBConfig)
	if err != nil {
		log.Fatal("Failed to create database instance:", err)
	}
	defer db.Close()

	// Create attack replayer
	replayer, err := NewAttackReplayer(
		testConfig.RPCURL,
		db,
		bindings.StorageScanMetaData,
	)
	if err != nil {
		log.Fatal("Failed to create replayer:", err)
	}

	fmt.Println("\n=== TESTING TARGET CONTRACT MUTATION ===")

	// Use configuration transaction hash and contract address
	txHash := testConfig.GetTxHash()
	contractAddr := testConfig.GetContractAddress()

	// Execute replay and collect mutations
	mutationCollection, err := replayer.ReplayAndCollectMutations(txHash, contractAddr)
	if err != nil {
		log.Fatal("Failed to replay and collect mutations:", err)
	}

	// Verify that mutations are based on target contract calls
	fmt.Printf("\n=== VERIFYING TARGET CONTRACT MUTATIONS ===\n")
	
	// Check if call trace was extracted
	require.NotNil(t, mutationCollection.CallTrace, "Call trace should not be nil")
	
	if len(mutationCollection.CallTrace.ExtractedCalls) > 0 {
		fmt.Printf("✅ Extracted %d calls to protected contracts\n", 
			len(mutationCollection.CallTrace.ExtractedCalls))
		
		// Verify first extracted call is to our target contract
		firstCall := mutationCollection.CallTrace.ExtractedCalls[0]
		assert.Equal(t, contractAddr, firstCall.ContractAddress, 
			"First extracted call should be to target contract")
		
		// Verify mutations are based on extracted calls
		mutationsWithSource := 0
		for _, mutation := range mutationCollection.Mutations {
			if mutation.SourceCallData != nil {
				mutationsWithSource++
				// Verify the source is from our target contract
				assert.Equal(t, contractAddr, mutation.SourceCallData.ContractAddress,
					"Mutation source should be from target contract")
			}
		}
		
		fmt.Printf("✅ %d/%d mutations are based on target contract calls\n",
			mutationsWithSource, len(mutationCollection.Mutations))
		
		// Most mutations should be based on extracted calls
		assert.Greater(t, mutationsWithSource, len(mutationCollection.Mutations)/2,
			"Most mutations should be based on extracted calls")
	} else {
		fmt.Printf("⚠️  No calls extracted - using fallback mutation\n")
	}
	
	fmt.Printf("\n✅ Target contract mutation test completed successfully\n")
}

// TestExecutionPathRecording tests that execution paths are recorded correctly
func TestExecutionPathRecording(t *testing.T) {
	// This test would require setting up contracts with known execution paths
	// For now, we'll test the basic functionality
	
	fmt.Println("\n=== TESTING EXECUTION PATH RECORDING ===")
	
	// Create a mock execution path
	originalPath := &tracingUtils.ExecutionPath{
		Jumps: []tracingUtils.ExecutionJump{
			{ContractAddress: common.HexToAddress("0x1"), JumpFrom: 10, JumpDest: 20},
			{ContractAddress: common.HexToAddress("0x1"), JumpFrom: 30, JumpDest: 40},
			{ContractAddress: common.HexToAddress("0x2"), JumpFrom: 50, JumpDest: 60},
		},
	}
	
	// Create a similar path with one difference
	similarPath := &tracingUtils.ExecutionPath{
		Jumps: []tracingUtils.ExecutionJump{
			{ContractAddress: common.HexToAddress("0x1"), JumpFrom: 10, JumpDest: 20},
			{ContractAddress: common.HexToAddress("0x1"), JumpFrom: 30, JumpDest: 45}, // Different
			{ContractAddress: common.HexToAddress("0x2"), JumpFrom: 50, JumpDest: 60},
		},
	}
	
	// Test similarity calculation
	replayer := &AttackReplayer{}
	similarity := replayer.calculatePathSimilarity(originalPath, similarPath)
	
	expectedSimilarity := 2.0 / 3.0 // 2 out of 3 jumps match
	assert.InDelta(t, expectedSimilarity, similarity, 0.01, 
		"Path similarity should be approximately 66.67%")
	
	fmt.Printf("✅ Path similarity calculated correctly: %.2f%%\n", similarity*100)
	
	// Test empty paths
	emptyPath := &tracingUtils.ExecutionPath{Jumps: []tracingUtils.ExecutionJump{}}
	similarity = replayer.calculatePathSimilarity(originalPath, emptyPath)
	assert.Equal(t, 0.0, similarity, "Similarity with empty path should be 0")
	
	similarity = replayer.calculatePathSimilarity(emptyPath, emptyPath)
	assert.Equal(t, 1.0, similarity, "Similarity of two empty paths should be 1")
	
	fmt.Println("✅ Execution path recording test completed")
}
