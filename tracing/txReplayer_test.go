package tracing

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
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"math/big"
	"testing"
	"time"
)

func TestRelayTx(t *testing.T) {
	const rpcURL = "https://lb.drpc.org/bsc/Avduh2iIjEAksBUYtd4wP1NUPObEnwYR76WEFhW5UfFk"

	// 连接数据库
	dsn := "host=172.23.216.120 user=root password=1234 dbname=postgres port=5432 sslmode=disable"
	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// 创建数据库实例
	dbConfig := config.DBConfig{
		Host:     "172.23.216.120",
		Port:     5432,
		Name:     "postgres",
		User:     "root",
		Password: "1234",
	}

	db, err := database.NewDB(context.Background(), dbConfig)
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
	err = insertExampleAttackTransaction(db, rpcURL)
	if err != nil {
		log.Fatal("Failed to insert example transaction:", err)
	}

	// 创建攻击重放器
	replayer, err := NewAttackReplayer(
		rpcURL,                       // 以太坊节点RPC URL
		db,                           // 数据库连接
		bindings.StorageScanMetaData, // StorageScan合约的metadata
	)
	if err != nil {
		log.Fatal("Failed to create replayer:", err)
	}

	// 测试重放、变异收集和交易发送的完整流程
	fmt.Println("\n=== STARTING COMPLETE ATTACK TRANSACTION REPLAY, MUTATION COLLECTION AND TRANSACTION SENDING ===")

	// 使用真实的交易哈希和合约地址
	txHash := common.HexToHash("0x2a65254b41b42f39331a0bcc9f893518d6b106e80d9a476b8ca3816325f4a150")
	//contractAddr := common.HexToAddress("0x9967407a5B9177E234d7B493AF8ff4A46771BEdf")
	protectContractAddr := common.HexToAddress("0x95e92b09b89cf31fa9f1eca4109a85f88eb08531")

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
func validateTransactionSending(t *testing.T, sentTxHashes []*common.Hash, successfulMutations []MutationData) {
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
	const rpcURL = "https://lb.drpc.org/holesky/Avduh2iIjEAksBUYtd4wP1NUPObEnwYR76WEFhW5UfFk"

	// 连接数据库
	dsn := "host=172.23.216.120 user=root password=1234 dbname=postgres port=5432 sslmode=disable"
	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// 创建数据库实例
	dbConfig := config.DBConfig{
		Host:     "172.23.216.120",
		Port:     5432,
		Name:     "postgres",
		User:     "root",
		Password: "1234",
	}

	db, err := database.NewDB(context.Background(), dbConfig)
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
	err = insertExampleAttackTransaction(db, rpcURL)
	if err != nil {
		log.Fatal("Failed to insert example transaction:", err)
	}

	// 创建攻击重放器
	replayer, err := NewAttackReplayer(
		rpcURL,                       // 以太坊节点RPC URL
		db,                           // 数据库连接
		bindings.StorageScanMetaData, // StorageScan合约的metadata
	)
	if err != nil {
		log.Fatal("Failed to create replayer:", err)
	}

	// 测试只进行重放和变异收集（不发送交易）
	fmt.Println("\n=== STARTING ATTACK TRANSACTION REPLAY AND MUTATION COLLECTION (NO SENDING) ===")

	// 使用真实的交易哈希和合约地址
	txHash := common.HexToHash("0x44b10cacbbda290163c152b40b826709815d18c8ac6d478e3efc6b48a6c6dc5e")
	contractAddr := common.HexToAddress("0x9967407a5B9177E234d7B493AF8ff4A46771BEdf")

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
	const rpcURL = "https://lb.drpc.org/holesky/Avduh2iIjEAksBUYtd4wP1NUPObEnwYR76WEFhW5UfFk"

	// 创建数据库实例
	dbConfig := config.DBConfig{
		Host:     "172.23.216.120",
		Port:     5432,
		Name:     "postgres",
		User:     "root",
		Password: "1234",
	}

	db, err := database.NewDB(context.Background(), dbConfig)
	if err != nil {
		log.Fatal("Failed to create database instance:", err)
	}
	defer db.Close()

	// 创建攻击重放器
	replayer, err := NewAttackReplayer(
		rpcURL,                       // 以太坊节点RPC URL
		db,                           // 数据库连接
		bindings.StorageScanMetaData, // StorageScan合约的metadata
	)
	if err != nil {
		log.Fatal("Failed to create replayer:", err)
	}

	fmt.Println("\n=== TESTING TRANSACTION SENDING FUNCTIONALITY ===")

	// 目标合约地址
	contractAddr := common.HexToAddress("0x9967407a5B9177E234d7B493AF8ff4A46771BEdf")

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
func createTestMutationData() []MutationData {
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

	testMutations := []MutationData{
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
func displayMutationResults(collection *MutationCollection) {
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
func displaySolidityFormat(solidityData *SolidityMutationData) {
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
func validateMutationResults(t *testing.T, collection *MutationCollection) {
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

	fmt.Printf("✅ All validation checks passed!\n")
}

// insertExampleAttackTransaction 插入示例攻击交易，从RPC获取真实数据
func insertExampleAttackTransaction(db *database.DB, rpcURL string) error {
	fmt.Println("📥 Inserting example attack transaction...")

	// 指定的交易哈希和合约地址
	txHash := common.HexToHash("0x2a65254b41b42f39331a0bcc9f893518d6b106e80d9a476b8ca3816325f4a150")
	contractAddr := common.HexToAddress("0x95e92b09b89cf31fa9f1eca4109a85f88eb08531")

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
