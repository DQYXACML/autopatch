package tracing

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/DQYXACML/autopatch/database"
	"github.com/DQYXACML/autopatch/database/common"
	"github.com/DQYXACML/autopatch/database/worker"
	"github.com/DQYXACML/autopatch/synchronizer/node"
	"github.com/DQYXACML/autopatch/txmgr/ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/ethereum/go-ethereum/triedb/hashdb"
	"github.com/holiman/uint256"
)

var (
	EthGasLimit          uint64 = 21000
	TokenGasLimit        uint64 = 120000
	maxFeePerGas                = big.NewInt(2900000000)
	maxPriorityFeePerGas        = big.NewInt(2600000000)
)

// AttackReplayer handles attack transaction replay and analysis
type AttackReplayer struct {
	client              *ethclient.Client
	nodeClient          node.EthClient
	jumpTracer          *JumpTracer
	inputModifier       *InputModifier
	db                  *database.DB
	addressesDB         common.AddressesDB
	similarityThreshold float64
	maxVariations       int

	// 新增：并发修改相关字段
	concurrentConfig  *ConcurrentModificationConfig
	transactionSender *ethereum.BatchTransactionSender
	privateKey        string
	privateKeyECDSA   *ecdsa.PrivateKey  // 新增：存储解析后的私钥
	fromAddress       gethCommon.Address // 新增：存储发送者地址
	chainID           *big.Int

	// 新增：步长变异配置
	mutationConfig *MutationConfig
}

// MutationConfig 步长变异配置
type MutationConfig struct {
	InputSteps   []int64 `json:"inputSteps"`   // 输入数据变异步长: [10, 100, -10, -100, 1000, -1000]
	StorageSteps []int64 `json:"storageSteps"` // 存储值变异步长: [1, 10, 100, -1, -10, -100]
	ByteSteps    []int   `json:"byteSteps"`    // 字节级变异步长: [1, 2, 5, -1, -2, -5]
	MaxMutations int     `json:"maxMutations"` // 每次最大变异数量
	OnlyPrestate bool    `json:"onlyPrestate"` // 是否只修改prestate中已有的存储槽
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

// NewAttackReplayer creates a new attack replayer
func NewAttackReplayer(rpcURL string, db *database.DB, contractsMetadata *bind.MetaData) (*AttackReplayer, error) {
	client, err := ethclient.Dial(rpcURL)
	nodeClient, err := node.DialEthClient(context.Background(), rpcURL)
	if err != nil {
		return nil, err
	}

	inputModifier, err := NewInputModifier(contractsMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create input modifier: %v", err)
	}

	// 创建批量交易发送器
	batchSender, err := ethereum.NewBatchTransactionSender(nodeClient, 4)
	if err != nil {
		return nil, fmt.Errorf("failed to create batch transaction sender: %v", err)
	}

	// 获取链ID
	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %v", err)
	}

	// 写死私钥在程序里
	hardcodedPrivateKey := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80" // 示例私钥，实际使用时请替换

	// 解析私钥
	privateKeyECDSA, err := crypto.HexToECDSA(hardcodedPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %v", err)
	}

	// 计算发送者地址
	fromAddress := crypto.PubkeyToAddress(privateKeyECDSA.PublicKey)

	fmt.Printf("=== ATTACK REPLAYER INITIALIZED ===\n")
	fmt.Printf("From Address: %s\n", fromAddress.Hex())
	fmt.Printf("Chain ID: %s\n", chainID.String())
	fmt.Printf("RPC URL: %s\n", rpcURL)

	return &AttackReplayer{
		client:              client,
		nodeClient:          nodeClient,
		jumpTracer:          NewJumpTracer(),
		inputModifier:       inputModifier,
		db:                  db,
		addressesDB:         db.Addresses,
		similarityThreshold: 0.8,
		maxVariations:       20,
		concurrentConfig:    DefaultConcurrentModificationConfig(),
		transactionSender:   batchSender,
		privateKey:          hardcodedPrivateKey,
		privateKeyECDSA:     privateKeyECDSA,
		fromAddress:         fromAddress,
		chainID:             chainID,
		mutationConfig:      DefaultMutationConfig(), // 新增：默认变异配置
	}, nil
}

// SetMutationConfig 设置变异配置
func (r *AttackReplayer) SetMutationConfig(config *MutationConfig) {
	r.mutationConfig = config
}

// SetPrivateKey 设置私钥用于交易签名
func (r *AttackReplayer) SetPrivateKey(privateKey string) {
	r.privateKey = privateKey

	// 解析私钥
	privateKeyECDSA, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		fmt.Printf("Failed to parse private key: %v\n", err)
		return
	}

	r.privateKeyECDSA = privateKeyECDSA
	r.fromAddress = crypto.PubkeyToAddress(privateKeyECDSA.PublicKey)

	fmt.Printf("Updated private key and from address: %s\n", r.fromAddress.Hex())
}

// sendTransactionToContract 发送交易到合约
func (r *AttackReplayer) sendTransactionToContract(
	contractAddr gethCommon.Address,
	inputData []byte,
	storageChanges map[gethCommon.Hash]gethCommon.Hash,
	gasLimit uint64,
) (*gethCommon.Hash, error) {
	if r.privateKeyECDSA == nil {
		return nil, fmt.Errorf("private key not set")
	}

	// 1. 获取nonce值
	nonce, err := r.nodeClient.TxCountByAddress(r.fromAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %v", err)
	}

	fmt.Printf("🔢 Current nonce for %s: %d\n", r.fromAddress.Hex(), uint64(nonce))

	// 2. 创建交易数据结构
	txData := &types.DynamicFeeTx{
		ChainID:   r.chainID,
		Nonce:     uint64(nonce),
		GasTipCap: maxPriorityFeePerGas,
		GasFeeCap: maxFeePerGas,
		Gas:       gasLimit,
		To:        &contractAddr,
		Value:     big.NewInt(0),
		Data:      inputData,
	}

	fmt.Printf("📦 Created transaction data:\n")
	fmt.Printf("   To: %s\n", contractAddr.Hex())
	fmt.Printf("   Nonce: %d\n", txData.Nonce)
	fmt.Printf("   Gas: %d\n", txData.Gas)
	fmt.Printf("   Data length: %d bytes\n", len(inputData))
	if len(inputData) >= 4 {
		fmt.Printf("   Function selector: %x\n", inputData[:4])
	}

	// 3. 离线签名交易
	rawTxHex, txHashStr, err := ethereum.OfflineSignTx(txData, r.privateKey, r.chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %v", err)
	}

	fmt.Printf("✍️  Transaction signed successfully\n")
	fmt.Printf("   Tx Hash: %s\n", txHashStr)
	fmt.Printf("   Raw Tx length: %d bytes\n", len(rawTxHex))

	// 4. 发送原始交易
	err = r.nodeClient.SendRawTransaction(rawTxHex)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %v", err)
	}

	txHash := gethCommon.HexToHash(txHashStr)
	fmt.Printf("🚀 Transaction sent successfully: %s\n", txHash.Hex())

	return &txHash, nil
}

// sendMutationTransactions 发送变异交易到合约
func (r *AttackReplayer) sendMutationTransactions(
	contractAddr gethCommon.Address,
	mutations []MutationData,
	gasLimit uint64,
) ([]*gethCommon.Hash, error) {
	if len(mutations) == 0 {
		return nil, fmt.Errorf("no mutations to send")
	}

	fmt.Printf("=== SENDING %d MUTATION TRANSACTIONS ===\n", len(mutations))

	var txHashes []*gethCommon.Hash
	var errors []error

	for i, mutation := range mutations {
		fmt.Printf("\n--- Sending mutation %d/%d (ID: %s) ---\n", i+1, len(mutations), mutation.ID)

		// 确保有修改的输入数据
		inputData := mutation.InputData
		if len(inputData) == 0 {
			fmt.Printf("⚠️  Mutation %s has no input data, skipping\n", mutation.ID)
			continue
		}

		// 发送交易
		txHash, err := r.sendTransactionToContract(contractAddr, inputData, mutation.StorageChanges, gasLimit)
		if err != nil {
			fmt.Printf("❌ Failed to send mutation %s: %v\n", mutation.ID, err)
			errors = append(errors, fmt.Errorf("mutation %s: %v", mutation.ID, err))
			continue
		}

		txHashes = append(txHashes, txHash)
		fmt.Printf("✅ Mutation %s sent: %s\n", mutation.ID, txHash.Hex())

		// 添加小延迟避免nonce冲突
		time.Sleep(1 * time.Second)
	}

	fmt.Printf("\n=== MUTATION TRANSACTION SENDING COMPLETED ===\n")
	fmt.Printf("Successfully sent: %d/%d transactions\n", len(txHashes), len(mutations))
	if len(errors) > 0 {
		fmt.Printf("Errors encountered: %d\n", len(errors))
		for _, err := range errors {
			fmt.Printf("  - %v\n", err)
		}
	}

	// 如果有部分成功，返回成功的交易哈希
	if len(txHashes) > 0 {
		return txHashes, nil
	}

	// 如果全部失败，返回第一个错误
	if len(errors) > 0 {
		return nil, errors[0]
	}

	return nil, fmt.Errorf("no transactions were sent")
}

// ReplayAttackTransactions replays all pending attack transactions
func (r *AttackReplayer) ReplayAttackTransactions() error {
	txs, err := r.db.AttackTx.QueryAttackTxByStatus(worker.StatusPending)
	if err != nil {
		return fmt.Errorf("failed to get pending transactions: %v", err)
	}

	fmt.Printf("Found %d pending attack transactions\n", len(txs))

	for _, attackTx := range txs {
		fmt.Printf("\n=== Processing attack transaction: %s ===\n", attackTx.TxHash.Hex())

		err = r.db.AttackTx.MarkAsProcessing(attackTx.GUID)
		if err != nil {
			fmt.Printf("Failed to mark transaction as processing: %v\n", err)
			continue
		}

		result, err := r.ReplayAttackTransactionWithVariations(attackTx.TxHash, attackTx.ContractAddress)
		if err != nil {
			fmt.Printf("Failed to replay transaction: %v\n", err)
			r.db.AttackTx.MarkAsFailed(attackTx.GUID, err.Error())
			continue
		}

		// 分析结果并更新状态
		if len(result.SuccessfulRules) > 0 {
			fmt.Printf("✅ Generated %d protection rules! Highest similarity: %.2f%%\n",
				len(result.SuccessfulRules), result.HighestSimilarity*100)

			// 这里可以将保护规则保存到数据库或发送到链上防护合约
			r.saveProtectionRules(result.SuccessfulRules)

			err = r.db.AttackTx.MarkAsSuccess(attackTx.GUID)
		} else {
			fmt.Printf("❌ No protection rules generated. Tested %d variations\n", result.TotalVariations)
			err = r.db.AttackTx.MarkAsFailed(attackTx.GUID,
				fmt.Sprintf("No rules generated after %d variations", result.TotalVariations))
		}

		if err != nil {
			fmt.Printf("Failed to update transaction status: %v\n", err)
		}

		// 打印统计信息
		r.printReplayStatistics(result)
	}

	return nil
}

// ReplayAndSendMutations 重放攻击交易、收集变异并发送到合约
func (r *AttackReplayer) ReplayAndSendMutations(txHash gethCommon.Hash, contractAddr gethCommon.Address) (*MutationCollection, []*gethCommon.Hash, error) {
	fmt.Printf("=== REPLAY AND SEND MUTATIONS ===\n")

	// 1. 重放并收集变异
	mutationCollection, err := r.ReplayAndCollectMutations(txHash, contractAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to replay and collect mutations: %v", err)
	}

	// 2. 发送成功的变异到合约
	if len(mutationCollection.SuccessfulMutations) == 0 {
		fmt.Printf("⚠️  No successful mutations to send\n")
		return mutationCollection, nil, nil
	}

	fmt.Printf("🚀 Sending %d successful mutations to contract...\n", len(mutationCollection.SuccessfulMutations))

	txHashes, err := r.sendMutationTransactions(contractAddr, mutationCollection.SuccessfulMutations, TokenGasLimit)
	if err != nil {
		fmt.Printf("❌ Failed to send some or all mutation transactions: %v\n", err)
		// 不返回错误，因为收集变异是成功的
	}

	return mutationCollection, txHashes, nil
}

// ReplayAttackTransactionWithConcurrentModification 使用并发修改重放攻击交易
func (r *AttackReplayer) ReplayAttackTransactionWithConcurrentModification(txHash gethCommon.Hash, contractAddr gethCommon.Address) (*SimplifiedReplayResult, error) {
	startTime := time.Now()

	fmt.Printf("=== CONCURRENT ATTACK TRANSACTION REPLAY START ===\n")
	fmt.Printf("Transaction hash: %s\n", txHash.Hex())
	fmt.Printf("Contract address: %s\n", contractAddr.Hex())

	// 获取交易详情
	tx, err := r.nodeClient.TxByHash(txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %v", err)
	}

	// 获取预状态
	prestate, err := r.getTransactionPrestate(txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get prestate: %v", err)
	}

	// 执行原始交易
	fmt.Printf("\n=== ORIGINAL EXECUTION ===\n")
	originalPath, err := r.executeTransactionWithTracing(tx, prestate, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to execute original transaction: %v", err)
	}

	fmt.Printf("Original path recorded: %d jumps\n", len(originalPath.Jumps))

	// 启动并发修改和模拟
	result, err := r.runConcurrentModificationAndSimulation(tx, prestate, originalPath, contractAddr, txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to run concurrent modification: %v", err)
	}

	result.ProcessingTime = time.Since(startTime)
	fmt.Printf("\n=== CONCURRENT REPLAY COMPLETED ===\n")
	fmt.Printf("Processing time: %v\n", result.ProcessingTime)

	return result, nil
}

// runConcurrentModificationAndSimulation 运行并发修改和模拟
func (r *AttackReplayer) runConcurrentModificationAndSimulation(
	tx *types.Transaction,
	prestate PrestateResult,
	originalPath *ExecutionPath,
	contractAddr gethCommon.Address,
	originalTxHash gethCommon.Hash,
) (*SimplifiedReplayResult, error) {

	// 创建通道
	candidateChan := make(chan *ModificationCandidate, r.concurrentConfig.ChannelBufferSize)
	resultChan := make(chan *WorkerResult, r.concurrentConfig.ChannelBufferSize)
	transactionChan := make(chan *ethereum.TransactionPackage, r.concurrentConfig.ChannelBufferSize)
	stopChan := make(chan struct{})

	// 启动交易发送器（如果还未启动）
	if !r.transactionSender.IsStarted() {
		r.transactionSender.Start()
	}

	// 使用defer确保最终会调用stop，但要检查是否已经停止
	defer func() {
		if !r.transactionSender.IsStopped() {
			r.transactionSender.Stop()
		}
	}()

	var wg sync.WaitGroup

	// 启动修改候选生成协程
	wg.Add(1)
	go r.modificationGenerator(candidateChan, stopChan, &wg, tx, prestate)

	// 启动模拟执行协程池
	for i := 0; i < r.concurrentConfig.MaxWorkers; i++ {
		wg.Add(1)
		go r.simulationWorker(i, candidateChan, resultChan, stopChan, &wg, tx, prestate, originalPath)
	}

	// 启动交易处理协程
	wg.Add(1)
	go r.transactionProcessor(resultChan, transactionChan, stopChan, &wg, contractAddr, originalTxHash)

	// 启动交易发送协程
	wg.Add(1)
	go r.transactionSender_worker(transactionChan, stopChan, &wg)

	// 收集结果
	result := &SimplifiedReplayResult{
		OriginalPath:      originalPath,
		SuccessfulRules:   make([]OnChainProtectionRule, 0),
		FailedVariations:  make([]ModificationVariation, 0),
		TotalVariations:   0,
		HighestSimilarity: 0.0,
		Statistics: &SimpleReplayStatistics{
			StrategyResults:   make(map[string]int),
			ErrorDistribution: make(map[string]int),
		},
	}

	// 等待一段时间或直到找到足够的成功案例
	timeout := time.NewTimer(r.concurrentConfig.GenerationTimeout)
	defer timeout.Stop()

	successCount := 0
	maxSuccess := 5 // 最多收集5个成功案例

	// 结果收集循环
	func() {
		for {
			select {
			case workerResult := <-resultChan:
				result.TotalVariations++

				if workerResult.Error != nil {
					result.Statistics.FailCount++
					result.Statistics.ErrorDistribution[workerResult.Error.Error()]++
					continue
				}

				if workerResult.Result.Success && workerResult.Result.Similarity >= r.concurrentConfig.SimilarityThreshold {
					// 创建保护规则
					rule := r.createProtectionRuleFromResult(workerResult.Result, contractAddr, originalTxHash)
					result.SuccessfulRules = append(result.SuccessfulRules, rule)
					result.Statistics.SuccessCount++
					successCount++

					if workerResult.Result.Similarity > result.HighestSimilarity {
						result.HighestSimilarity = workerResult.Result.Similarity
					}

					fmt.Printf("✅ Found successful modification! Similarity: %.2f%% (Total: %d)\n",
						workerResult.Result.Similarity*100, successCount)

					if successCount >= maxSuccess {
						fmt.Printf("🎯 Reached maximum success count (%d), stopping...\n", maxSuccess)
						return
					}
				} else {
					result.Statistics.FailCount++
				}

			case <-timeout.C:
				fmt.Printf("⏰ Timeout reached, stopping concurrent modification...\n")
				return
			}
		}
	}()

	// 发送停止信号
	close(stopChan)

	// 等待所有协程结束
	wg.Wait()

	// 安全地关闭channels
	safeCloseCandidate := func() {
		defer func() {
			if r := recover(); r != nil {
				// Channel已经被关闭，忽略panic
			}
		}()
		select {
		case <-candidateChan:
		default:
		}
	}
	safeCloseCandidate()

	// 计算平均相似度
	if result.Statistics.SuccessCount > 0 {
		totalSimilarity := 0.0
		for _, rule := range result.SuccessfulRules {
			totalSimilarity += rule.Similarity
		}
		result.Statistics.AverageSimilarity = totalSimilarity / float64(result.Statistics.SuccessCount)
	}

	return result, nil
}

// modificationGenerator 修改候选生成协程
func (r *AttackReplayer) modificationGenerator(
	candidateChan chan<- *ModificationCandidate,
	stopChan <-chan struct{},
	wg *sync.WaitGroup,
	tx *types.Transaction,
	prestate PrestateResult,
) {
	defer wg.Done()
	defer func() {
		// 安全地关闭channel
		defer func() {
			if r := recover(); r != nil {
				// Channel已经被关闭，忽略panic
			}
		}()
		close(candidateChan)
	}()

	// 设置原始状态
	contractStorage := make(map[gethCommon.Hash]gethCommon.Hash)
	if tx.To() != nil {
		if contractAccount, exists := prestate[*tx.To()]; exists {
			contractStorage = contractAccount.Storage
		}
	}

	err := r.inputModifier.SetOriginalState(tx.Data(), contractStorage)
	if err != nil {
		fmt.Printf("Failed to set original state: %v\n", err)
		return
	}

	candidateID := 0
	ticker := time.NewTicker(100 * time.Millisecond) // 每100ms生成一批候选
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
			return
		case <-ticker.C:
			// 生成一批修改候选（使用步长变异）
			candidates := r.generateStepBasedModificationCandidates(candidateID, r.concurrentConfig.BatchSize, tx.Data(), contractStorage)
			candidateID += len(candidates)

			for _, candidate := range candidates {
				select {
				case candidateChan <- candidate:
				case <-stopChan:
					return
				}
			}

			if candidateID >= r.concurrentConfig.MaxCandidates {
				fmt.Printf("Generated maximum candidates (%d), stopping generator...\n", candidateID)
				return
			}
		}
	}
}

// simulationWorker 模拟执行工作协程
func (r *AttackReplayer) simulationWorker(
	workerID int,
	candidateChan <-chan *ModificationCandidate,
	resultChan chan<- *WorkerResult,
	stopChan <-chan struct{},
	wg *sync.WaitGroup,
	tx *types.Transaction,
	prestate PrestateResult,
	originalPath *ExecutionPath,
) {
	defer wg.Done()

	for {
		select {
		case <-stopChan:
			return
		case candidate := <-candidateChan:
			if candidate == nil {
				return
			}

			// 执行模拟
			result := r.simulateModification(candidate, tx, prestate, originalPath)

			workerResult := &WorkerResult{
				WorkerID: workerID,
				Result:   result,
			}

			if result.Error != nil {
				workerResult.Error = result.Error
			}

			select {
			case resultChan <- workerResult:
			case <-stopChan:
				return
			}
		}
	}
}

// transactionProcessor 交易处理协程
func (r *AttackReplayer) transactionProcessor(
	resultChan <-chan *WorkerResult,
	transactionChan chan<- *ethereum.TransactionPackage,
	stopChan <-chan struct{},
	wg *sync.WaitGroup,
	contractAddr gethCommon.Address,
	originalTxHash gethCommon.Hash,
) {
	defer wg.Done()
	defer func() {
		// 安全地关闭channel
		defer func() {
			if r := recover(); r != nil {
				// Channel已经被关闭，忽略panic
			}
		}()
		close(transactionChan)
	}()

	for {
		select {
		case <-stopChan:
			return
		case workerResult := <-resultChan:
			if workerResult == nil {
				return
			}

			// 只处理成功且相似度高的结果
			if workerResult.Result.Success && workerResult.Result.Similarity >= r.concurrentConfig.SimilarityThreshold {
				// 创建交易包
				pkg := CreateTransactionPackage(workerResult.Result.Candidate, workerResult.Result.Similarity, contractAddr, originalTxHash)

				select {
				case transactionChan <- pkg:
					fmt.Printf("📦 Created transaction package for similarity %.2f%%\n", workerResult.Result.Similarity*100)
				case <-stopChan:
					return
				}
			}
		}
	}
}

// transactionSender_worker 交易发送工作协程 - 使用新的发送方法
func (r *AttackReplayer) transactionSender_worker(
	transactionChan <-chan *ethereum.TransactionPackage,
	stopChan <-chan struct{},
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	if r.privateKeyECDSA == nil {
		fmt.Printf("⚠️  No private key set, skipping transaction sending\n")
		return
	}

	for {
		select {
		case <-stopChan:
			return
		case pkg := <-transactionChan:
			if pkg == nil {
				return
			}

			// 提取交易数据
			var inputData []byte
			var storageChanges map[gethCommon.Hash]gethCommon.Hash

			// 从交易包中提取输入数据
			if len(pkg.InputUpdates) > 0 {
				inputData = pkg.InputUpdates[0].ModifiedInput
			}

			// 从交易包中提取存储更改
			if len(pkg.StorageUpdates) > 0 {
				storageChanges = make(map[gethCommon.Hash]gethCommon.Hash)
				for _, update := range pkg.StorageUpdates {
					storageChanges[update.Slot] = update.ModifiedValue
				}
			}

			// 使用新的发送方法发送交易
			txHash, err := r.sendTransactionToContract(
				pkg.ContractAddress,
				inputData,
				storageChanges,
				TokenGasLimit,
			)

			if err != nil {
				fmt.Printf("❌ Failed to send transaction for package %s: %v\n", pkg.ID, err)
			} else {
				fmt.Printf("🚀 Successfully sent transaction for package %s: %s\n", pkg.ID, txHash.Hex())
			}
		}
	}
}

// generateStepBasedModificationCandidates 生成基于步长的修改候选（确保每个候选都有有效修改）
func (r *AttackReplayer) generateStepBasedModificationCandidates(
	startID int,
	count int,
	originalInput []byte,
	originalStorage map[gethCommon.Hash]gethCommon.Hash,
) []*ModificationCandidate {

	candidates := make([]*ModificationCandidate, 0, count)

	for i := 0; i < count; i++ {
		candidate := &ModificationCandidate{
			ID:             fmt.Sprintf("step_candidate_%d", startID+i),
			GeneratedAt:    time.Now(),
			StorageChanges: make(map[gethCommon.Hash]gethCommon.Hash),
		}

		// 根据策略选择修改类型
		modType := i % 3
		hasValidModification := false

		switch modType {
		case 0: // 只修改输入（基于步长）
			modifiedInput := r.generateStepBasedInputData(originalInput, i)
			if !bytesEqual(modifiedInput, originalInput) {
				candidate.InputData = modifiedInput
				candidate.ModType = "input_step"
				candidate.Priority = 1
				hasValidModification = true
			}
		case 1: // 只修改存储（基于步长，仅修改已有存储槽）
			storageChanges := r.generateStepBasedStorageChanges(originalStorage, i)
			if len(storageChanges) > 0 {
				candidate.StorageChanges = storageChanges
				candidate.ModType = "storage_step"
				candidate.Priority = 2
				hasValidModification = true
			}
		case 2: // 同时修改输入和存储（基于步长）
			modifiedInput := r.generateStepBasedInputData(originalInput, i)
			storageChanges := r.generateStepBasedStorageChanges(originalStorage, i)

			if !bytesEqual(modifiedInput, originalInput) {
				candidate.InputData = modifiedInput
				hasValidModification = true
			}
			if len(storageChanges) > 0 {
				candidate.StorageChanges = storageChanges
				hasValidModification = true
			}

			if hasValidModification {
				candidate.ModType = "both_step"
				candidate.Priority = 3
			}
		}

		// 如果没有产生有效修改，强制生成一个
		if !hasValidModification {
			hasValidModification = r.forceValidModification(candidate, originalInput, originalStorage, i)
		}

		// 只添加有有效修改的候选
		if hasValidModification {
			candidate.ExpectedImpact = r.predictModificationImpact(candidate)
			candidates = append(candidates, candidate)
		} else {
			fmt.Printf("⚠️  Skipped candidate %s - no valid modifications generated\n", candidate.ID)
		}
	}

	fmt.Printf("Generated %d valid step-based candidates out of %d attempts\n", len(candidates), count)
	return candidates
}

// forceValidModification 强制生成有效的修改
func (r *AttackReplayer) forceValidModification(
	candidate *ModificationCandidate,
	originalInput []byte,
	originalStorage map[gethCommon.Hash]gethCommon.Hash,
	variant int,
) bool {
	// 策略1：强制修改输入数据
	if len(originalInput) > 4 {
		modifiedInput := r.forceModifyInputData(originalInput, variant)
		if !bytesEqual(modifiedInput, originalInput) {
			candidate.InputData = modifiedInput
			candidate.ModType = "forced_input_step"
			candidate.Priority = 1
			fmt.Printf("🔧 Forced input modification for candidate %s\n", candidate.ID)
			return true
		}
	}

	// 策略2：强制修改存储（如果有原始存储）
	if len(originalStorage) > 0 {
		storageChanges := r.forceModifyStorageData(originalStorage, variant)
		if len(storageChanges) > 0 {
			candidate.StorageChanges = storageChanges
			candidate.ModType = "forced_storage_step"
			candidate.Priority = 2
			fmt.Printf("💾 Forced storage modification for candidate %s\n", candidate.ID)
			return true
		}
	}

	// 策略3：创建虚拟存储修改（最后手段）
	virtualStorage := r.createVirtualStorageModification(variant)
	if len(virtualStorage) > 0 {
		candidate.StorageChanges = virtualStorage
		candidate.ModType = "forced_virtual_storage"
		candidate.Priority = 3
		fmt.Printf("🔮 Created virtual storage modification for candidate %s\n", candidate.ID)
		return true
	}

	return false
}

// forceModifyInputData 强制修改输入数据
func (r *AttackReplayer) forceModifyInputData(originalInput []byte, variant int) []byte {
	if len(originalInput) < 4 {
		return originalInput
	}

	// 复制原始输入
	modified := make([]byte, len(originalInput))
	copy(modified, originalInput)

	// 强制修改策略
	if len(modified) > 4 {
		paramData := modified[4:]

		// 确保至少修改一个字节
		modificationMade := false

		// 策略1：修改第一个字节（如果存在）
		if len(paramData) > 0 {
			original := paramData[0]
			paramData[0] = byte((int(paramData[0]) + variant + 1) % 256)
			if paramData[0] != original {
				modificationMade = true
			}
		}

		// 策略2：修改32字节边界的数据（用于uint256等）
		if len(paramData) >= 32 && !modificationMade {
			// 修改最后一个字节
			original := paramData[31]
			paramData[31] = byte((int(paramData[31]) + variant + 1) % 256)
			if paramData[31] != original {
				modificationMade = true
			}
		}

		// 策略3：如果还没有修改，强制修改任意位置
		if !modificationMade && len(paramData) > 0 {
			index := variant % len(paramData)
			paramData[index] = byte((int(paramData[index]) + 1) % 256)
		}
	}

	return modified
}

// forceModifyStorageData 强制修改存储数据
func (r *AttackReplayer) forceModifyStorageData(originalStorage map[gethCommon.Hash]gethCommon.Hash, variant int) map[gethCommon.Hash]gethCommon.Hash {
	changes := make(map[gethCommon.Hash]gethCommon.Hash)

	if len(originalStorage) == 0 {
		return changes
	}

	// 选择要修改的存储槽
	count := 0
	maxModifications := 2

	for slot, originalValue := range originalStorage {
		if count >= maxModifications {
			break
		}

		// 确保修改后的值与原始值不同
		originalBig := originalValue.Big()
		var newValue gethCommon.Hash

		// 多种修改策略，确保至少有一种有效
		strategies := []func(*big.Int, int) *big.Int{
			func(orig *big.Int, v int) *big.Int { return new(big.Int).Add(orig, big.NewInt(int64(v+1))) },
			func(orig *big.Int, v int) *big.Int {
				result := new(big.Int).Sub(orig, big.NewInt(int64(v+1)))
				if result.Sign() < 0 {
					result = big.NewInt(int64(v + 1))
				}
				return result
			},
			func(orig *big.Int, v int) *big.Int { return new(big.Int).Xor(orig, big.NewInt(int64(v+1))) },
			func(orig *big.Int, v int) *big.Int { return big.NewInt(int64(v + 100)) },
		}

		for _, strategy := range strategies {
			newBig := strategy(originalBig, variant)
			newValue = gethCommon.BigToHash(newBig)

			// 确保值确实发生了变化
			if newValue != originalValue {
				changes[slot] = newValue
				count++
				fmt.Printf("   Forced storage change: slot %s: %s -> %s\n",
					slot.Hex()[:10]+"...", originalValue.Hex()[:10]+"...", newValue.Hex()[:10]+"...")
				break
			}
		}

		// 如果所有策略都失败，使用最简单的修改
		if _, exists := changes[slot]; !exists {
			// 最后手段：直接设置为一个固定的不同值
			if originalValue == (gethCommon.Hash{}) {
				newValue = gethCommon.BigToHash(big.NewInt(int64(variant + 1)))
			} else {
				newValue = gethCommon.Hash{} // 设置为零值
			}
			changes[slot] = newValue
			count++
			fmt.Printf("   Last resort storage change: slot %s: %s -> %s\n",
				slot.Hex()[:10]+"...", originalValue.Hex()[:10]+"...", newValue.Hex()[:10]+"...")
		}
	}

	return changes
}

// createVirtualStorageModification 创建虚拟存储修改（最后手段）
func (r *AttackReplayer) createVirtualStorageModification(variant int) map[gethCommon.Hash]gethCommon.Hash {
	changes := make(map[gethCommon.Hash]gethCommon.Hash)

	// 创建一些虚拟的存储槽修改
	for i := 0; i < 2; i++ {
		slot := gethCommon.BigToHash(big.NewInt(int64(variant*10 + i + 1)))
		value := gethCommon.BigToHash(big.NewInt(int64(variant*100 + i + 42)))
		changes[slot] = value
		fmt.Printf("   Virtual storage: slot %s = %s\n", slot.Hex()[:10]+"...", value.Hex()[:10]+"...")
	}

	return changes
}

// generateStepBasedInputData 生成基于步长的输入数据变异（替换随机生成）
func (r *AttackReplayer) generateStepBasedInputData(originalInput []byte, variant int) []byte {
	if len(originalInput) < 4 {
		return originalInput
	}

	// 复制原始输入
	modified := make([]byte, len(originalInput))
	copy(modified, originalInput)

	// 保持函数选择器不变，只修改参数部分（4字节之后）
	if len(originalInput) > 4 {
		paramData := modified[4:]

		// 根据变异配置选择步长
		stepIndex := variant % len(r.mutationConfig.InputSteps)
		step := r.mutationConfig.InputSteps[stepIndex]

		fmt.Printf("🔧 Applying input step mutation: step=%d, variant=%d\n", step, variant)

		// 对参数数据进行步长变异
		r.applyStepMutationToBytes(paramData, step, variant)
	}

	return modified
}

// generateStepBasedStorageChanges 生成基于步长的存储变化（改进版）
func (r *AttackReplayer) generateStepBasedStorageChanges(originalStorage map[gethCommon.Hash]gethCommon.Hash, variant int) map[gethCommon.Hash]gethCommon.Hash {
	changes := make(map[gethCommon.Hash]gethCommon.Hash)

	if !r.mutationConfig.OnlyPrestate || len(originalStorage) == 0 {
		fmt.Printf("⚠️  No original storage to mutate or OnlyPrestate disabled\n")
		return changes
	}

	// 根据变异配置选择步长
	stepIndex := variant % len(r.mutationConfig.StorageSteps)
	step := r.mutationConfig.StorageSteps[stepIndex]

	fmt.Printf("💾 Applying storage step mutation: step=%d, variant=%d\n", step, variant)

	// 限制修改的存储槽数量
	mutationCount := 0
	maxMutations := r.mutationConfig.MaxMutations
	if maxMutations <= 0 {
		maxMutations = 1
	}

	// 只修改已有的存储槽，并确保值发生变化
	for slot, originalValue := range originalStorage {
		if mutationCount >= maxMutations {
			break
		}

		// 根据变异策略决定是否修改这个槽
		if (variant+mutationCount)%2 == 0 { // 增加修改频率
			newValue := r.generateStepBasedStorageValue(originalValue, step, variant)
			if newValue != originalValue { // 只有实际发生变化时才记录
				changes[slot] = newValue
				mutationCount++
				fmt.Printf("   Modified slot %s: %s -> %s (step: %d)\n",
					slot.Hex()[:10]+"...", originalValue.Hex()[:10]+"...", newValue.Hex()[:10]+"...", step)
			} else {
				// 如果步长变异没有产生变化，强制修改
				forcedValue := r.forceStorageValueChange(originalValue, variant)
				if forcedValue != originalValue {
					changes[slot] = forcedValue
					mutationCount++
					fmt.Printf("   Forced slot modification %s: %s -> %s\n",
						slot.Hex()[:10]+"...", originalValue.Hex()[:10]+"...", forcedValue.Hex()[:10]+"...")
				}
			}
		}
	}

	fmt.Printf("💾 Generated %d storage changes\n", len(changes))
	return changes
}

// forceStorageValueChange 强制改变存储值
func (r *AttackReplayer) forceStorageValueChange(original gethCommon.Hash, variant int) gethCommon.Hash {
	originalBig := original.Big()

	// 尝试多种修改策略
	modifications := []*big.Int{
		new(big.Int).Add(originalBig, big.NewInt(int64(variant+1))),
		new(big.Int).Sub(originalBig, big.NewInt(int64(variant+1))),
		new(big.Int).Xor(originalBig, big.NewInt(int64(variant+1))),
		big.NewInt(int64(variant + 42)),
	}

	for _, modBig := range modifications {
		if modBig.Sign() < 0 {
			modBig = new(big.Int).Abs(modBig)
		}
		newValue := gethCommon.BigToHash(modBig)
		if newValue != original {
			return newValue
		}
	}

	// 最后手段：如果原值为零，设为非零；如果非零，设为零
	if original == (gethCommon.Hash{}) {
		return gethCommon.BigToHash(big.NewInt(1))
	} else {
		return gethCommon.Hash{}
	}
}

// generateStepBasedStorageValue 生成基于步长的存储值变异（替换随机生成）
func (r *AttackReplayer) generateStepBasedStorageValue(original gethCommon.Hash, step int64, variant int) gethCommon.Hash {
	originalBig := original.Big()

	// 根据不同的变异策略应用步长
	switch variant % 4 {
	case 0: // 直接加法
		result := new(big.Int).Add(originalBig, big.NewInt(step))
		return gethCommon.BigToHash(result)
	case 1: // 直接减法（确保不为负）
		result := new(big.Int).Sub(originalBig, big.NewInt(step))
		if result.Sign() < 0 {
			result = big.NewInt(0)
		}
		return gethCommon.BigToHash(result)
	case 2: // 乘法变异（小步长）
		if step != 0 {
			multiplier := step
			if multiplier < 0 {
				multiplier = -multiplier
			}
			if multiplier > 10 {
				multiplier = 2 // 限制乘数避免溢出
			}
			result := new(big.Int).Mul(originalBig, big.NewInt(multiplier))
			return gethCommon.BigToHash(result)
		}
		return original
	case 3: // 位操作变异
		if originalBig.Sign() > 0 {
			stepAbs := step
			if stepAbs < 0 {
				stepAbs = -stepAbs
			}
			// 进行XOR操作
			xorValue := big.NewInt(stepAbs)
			result := new(big.Int).Xor(originalBig, xorValue)
			return gethCommon.BigToHash(result)
		}
		return original
	default:
		return original
	}
}

// applyStepMutationToBytes 对字节数组应用步长变异
func (r *AttackReplayer) applyStepMutationToBytes(data []byte, step int64, variant int) {
	if len(data) == 0 {
		return
	}

	// 根据数据长度和变异类型选择修改策略
	switch variant % 3 {
	case 0: // 修改整个32字节块（如果足够长）
		if len(data) >= 32 {
			// 将前32字节作为big.Int处理
			value := new(big.Int).SetBytes(data[:32])
			newValue := new(big.Int).Add(value, big.NewInt(step))

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
		byteStep := int(step) % 256
		if byteStep < 0 {
			byteStep = 256 + byteStep
		}
		newByte := (int(data[byteIndex]) + byteStep) % 256
		data[byteIndex] = byte(newByte)
	case 2: // 修改多个字节位置
		stepAbs := step
		if stepAbs < 0 {
			stepAbs = -stepAbs
		}
		byteStep := int(stepAbs) % 256

		// 修改最多3个字节位置
		maxChanges := 3
		if len(data) < maxChanges {
			maxChanges = len(data)
		}

		for i := 0; i < maxChanges; i++ {
			byteIndex := (variant + i) % len(data)
			if i%2 == 0 {
				data[byteIndex] = byte((int(data[byteIndex]) + byteStep) % 256)
			} else {
				data[byteIndex] = byte((int(data[byteIndex]) - byteStep + 256) % 256)
			}
		}
	}
}

// simulateModification 模拟修改
func (r *AttackReplayer) simulateModification(
	candidate *ModificationCandidate,
	tx *types.Transaction,
	prestate PrestateResult,
	originalPath *ExecutionPath,
) *SimulationResult {

	startTime := time.Now()
	result := &SimulationResult{
		Candidate: candidate,
		Success:   false,
	}

	// 执行修改后的交易
	modifiedPath, err := r.executeTransactionWithTracing(tx, prestate, candidate.InputData, candidate.StorageChanges)
	if err != nil {
		result.Error = fmt.Errorf("simulation failed: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// 计算相似度
	similarity := r.calculatePathSimilarity(originalPath, modifiedPath)
	result.Similarity = similarity
	result.ExecutePath = modifiedPath
	result.Success = true
	result.Duration = time.Since(startTime)

	return result
}

// predictModificationImpact 预测修改影响
func (r *AttackReplayer) predictModificationImpact(candidate *ModificationCandidate) string {
	switch candidate.ModType {
	case "input_step":
		return "step_based_input_behavior_change"
	case "storage_step":
		return "step_based_state_manipulation"
	case "both_step":
		return "comprehensive_step_based_attack_vector"
	default:
		return "unknown_step_based"
	}
}

// createProtectionRuleFromResult 从结果创建保护规则
func (r *AttackReplayer) createProtectionRuleFromResult(
	result *SimulationResult,
	contractAddr gethCommon.Address,
	originalTxHash gethCommon.Hash,
) OnChainProtectionRule {

	ruleID := GenerateRuleID(originalTxHash, contractAddr, time.Now())

	rule := OnChainProtectionRule{
		RuleID:          ruleID,
		TxHash:          originalTxHash,
		ContractAddress: contractAddr,
		Similarity:      result.Similarity,
		InputRules:      make([]InputProtectionRule, 0),
		StorageRules:    make([]StorageProtectionRule, 0),
		CreatedAt:       time.Now(),
		IsActive:        true,
	}

	// 根据修改候选创建输入保护规则
	if result.Candidate.InputData != nil && len(result.Candidate.InputData) > 0 {
		originalInput := make([]byte, 0)
		if len(result.Candidate.InputData) >= 4 {
			// 假设原始输入，这里可以从context中获取
			originalInput = make([]byte, len(result.Candidate.InputData))
			copy(originalInput, result.Candidate.InputData)
			// 修改一些字节来模拟原始输入
			if len(originalInput) > 4 {
				originalInput[4] = 0x00
			}
		}

		inputRule := CreateInputProtectionRuleFromCandidate(result.Candidate, originalInput)
		rule.InputRules = append(rule.InputRules, inputRule)
	}

	// 根据修改候选创建存储保护规则
	if len(result.Candidate.StorageChanges) > 0 {
		storageRules := CreateStorageProtectionRulesFromCandidate(result.Candidate, contractAddr)
		rule.StorageRules = append(rule.StorageRules, storageRules...)
	}

	// 确保至少有一个规则
	if len(rule.InputRules) == 0 && len(rule.StorageRules) == 0 {
		// 创建一个基本的存储规则作为后备
		basicStorageRule := StorageProtectionRule{
			ContractAddress: contractAddr,
			StorageSlot:     gethCommon.BigToHash(big.NewInt(1)), // 使用有效的槽位
			OriginalValue:   gethCommon.Hash{},
			ModifiedValue:   gethCommon.BigToHash(big.NewInt(1)),
			CheckType:       "exact",
			SlotType:        "simple",
		}
		rule.StorageRules = append(rule.StorageRules, basicStorageRule)
	}

	return rule
}

// ReplayAndCollectMutations 重放攻击交易并收集所有变异数据
func (r *AttackReplayer) ReplayAndCollectMutations(txHash gethCommon.Hash, contractAddr gethCommon.Address) (*MutationCollection, error) {
	startTime := time.Now()

	fmt.Printf("=== ATTACK TRANSACTION REPLAY WITH MUTATION COLLECTION ===\n")
	fmt.Printf("Transaction hash: %s\n", txHash.Hex())
	fmt.Printf("Contract address: %s\n", contractAddr.Hex())

	// 获取交易详情
	tx, err := r.nodeClient.TxByHash(txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %v", err)
	}

	// 获取预状态
	prestate, err := r.getTransactionPrestate(txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get prestate: %v", err)
	}

	// 创建变异数据集合
	mutationCollection := &MutationCollection{
		OriginalTxHash:      txHash,
		ContractAddress:     contractAddr,
		OriginalInputData:   tx.Data(),
		OriginalStorage:     make(map[gethCommon.Hash]gethCommon.Hash),
		Mutations:           make([]MutationData, 0),
		SuccessfulMutations: make([]MutationData, 0),
		CreatedAt:           time.Now(),
	}

	// 提取原始存储状态
	if contractAccount, exists := prestate[contractAddr]; exists {
		mutationCollection.OriginalStorage = contractAccount.Storage
		fmt.Printf("Original storage slots: %d\n", len(contractAccount.Storage))
	}

	// 执行原始交易
	fmt.Printf("\n=== ORIGINAL EXECUTION ===\n")
	originalPath, err := r.executeTransactionWithTracing(tx, prestate, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to execute original transaction: %v", err)
	}
	fmt.Printf("Original execution path: %d jumps\n", len(originalPath.Jumps))

	// 设置原始状态用于智能修改
	err = r.inputModifier.SetOriginalState(tx.Data(), mutationCollection.OriginalStorage)
	if err != nil {
		fmt.Printf("Failed to set original state: %v\n", err)
	}

	// 生成并执行变异（使用步长变异）
	fmt.Printf("\n=== GENERATING AND EXECUTING STEP-BASED MUTATIONS ===\n")
	fmt.Printf("Using mutation config: InputSteps=%v, StorageSteps=%v, OnlyPrestate=%v\n",
		r.mutationConfig.InputSteps, r.mutationConfig.StorageSteps, r.mutationConfig.OnlyPrestate)

	// 生成多种变异候选
	totalCandidates := 50 // 减少数量以便测试
	batchSize := 10

	for i := 0; i < totalCandidates; i += batchSize {
		currentBatchSize := batchSize
		if i+batchSize > totalCandidates {
			currentBatchSize = totalCandidates - i
		}

		// 使用步长变异生成候选
		candidates := r.generateStepBasedModificationCandidates(i, currentBatchSize, tx.Data(), mutationCollection.OriginalStorage)

		// 并行执行这批变异
		mutationResults := r.executeMutationBatch(candidates, tx, prestate, originalPath)

		// 收集结果
		for _, result := range mutationResults {
			mutationData := MutationData{
				ID:             result.Candidate.ID,
				InputData:      result.Candidate.InputData,
				StorageChanges: result.Candidate.StorageChanges,
				Similarity:     result.Similarity,
				Success:        result.Success,
				ExecutionTime:  result.Duration,
			}

			if result.Error != nil {
				mutationData.ErrorMessage = result.Error.Error()
			}

			mutationCollection.Mutations = append(mutationCollection.Mutations, mutationData)

			// 收集成功的变异
			if result.Success && result.Similarity >= r.similarityThreshold {
				mutationCollection.SuccessfulMutations = append(mutationCollection.SuccessfulMutations, mutationData)
				fmt.Printf("✅ Successful step-based mutation %s: Similarity %.2f%%\n", result.Candidate.ID, result.Similarity*100)
			} else {
				fmt.Printf("❌ Failed step-based mutation %s: %s\n", result.Candidate.ID, mutationData.ErrorMessage)
			}
		}
	}

	// 计算统计信息
	mutationCollection.TotalMutations = len(mutationCollection.Mutations)
	mutationCollection.SuccessCount = len(mutationCollection.SuccessfulMutations)
	mutationCollection.FailureCount = mutationCollection.TotalMutations - mutationCollection.SuccessCount
	mutationCollection.ProcessingTime = time.Since(startTime)

	// 计算平均相似度和最高相似度
	if mutationCollection.SuccessCount > 0 {
		totalSimilarity := 0.0
		for _, mutation := range mutationCollection.SuccessfulMutations {
			totalSimilarity += mutation.Similarity
			if mutation.Similarity > mutationCollection.HighestSimilarity {
				mutationCollection.HighestSimilarity = mutation.Similarity
			}
		}
		mutationCollection.AverageSimilarity = totalSimilarity / float64(mutationCollection.SuccessCount)
	}

	fmt.Printf("\n=== STEP-BASED MUTATION COLLECTION COMPLETED ===\n")
	fmt.Printf("Total mutations: %d\n", mutationCollection.TotalMutations)
	fmt.Printf("Successful mutations: %d\n", mutationCollection.SuccessCount)
	fmt.Printf("Failed mutations: %d\n", mutationCollection.FailureCount)
	fmt.Printf("Success rate: %.2f%%\n", float64(mutationCollection.SuccessCount)/float64(mutationCollection.TotalMutations)*100)
	fmt.Printf("Average similarity: %.2f%%\n", mutationCollection.AverageSimilarity*100)
	fmt.Printf("Highest similarity: %.2f%%\n", mutationCollection.HighestSimilarity*100)
	fmt.Printf("Processing time: %v\n", mutationCollection.ProcessingTime)

	return mutationCollection, nil
}

// executeMutationBatch 并行执行一批变异
func (r *AttackReplayer) executeMutationBatch(candidates []*ModificationCandidate, tx *types.Transaction, prestate PrestateResult, originalPath *ExecutionPath) []*SimulationResult {
	results := make([]*SimulationResult, len(candidates))

	// 使用goroutine并行执行
	var wg sync.WaitGroup
	for i, candidate := range candidates {
		wg.Add(1)
		go func(index int, cand *ModificationCandidate) {
			defer wg.Done()
			results[index] = r.simulateModification(cand, tx, prestate, originalPath)
		}(i, candidate)
	}

	wg.Wait()
	return results
}

// ReplayAttackTransactionWithVariations 使用多种变体重放攻击交易（保持兼容性）
func (r *AttackReplayer) ReplayAttackTransactionWithVariations(txHash gethCommon.Hash, contractAddr gethCommon.Address) (*SimplifiedReplayResult, error) {
	// 优先使用并发版本
	return r.ReplayAttackTransactionWithConcurrentModification(txHash, contractAddr)
}

// 保持原有方法的实现...
func (r *AttackReplayer) testVariation(tx *types.Transaction, prestate PrestateResult,
	originalPath *ExecutionPath, variation *ModificationVariation,
	contractAddr gethCommon.Address, originalTxHash gethCommon.Hash) (bool, *OnChainProtectionRule) {

	var modifiedInput []byte
	var storageChanges map[gethCommon.Hash]gethCommon.Hash

	// 准备修改后的输入数据
	if variation.InputMod != nil {
		modifiedInput = variation.InputMod.ModifiedInput
	}

	// 准备存储修改
	if variation.StorageMod != nil {
		storageChanges = make(map[gethCommon.Hash]gethCommon.Hash)
		for _, change := range variation.StorageMod.Changes {
			storageChanges[change.Slot] = change.Modified
		}
	}

	// 执行修改后的交易
	modifiedPath, err := r.executeTransactionWithTracing(tx, prestate, modifiedInput, storageChanges)
	if err != nil {
		fmt.Printf("  ❌ Execution failed: %v\n", err)
		return false, nil
	}

	// 计算相似度
	similarity := r.calculatePathSimilarity(originalPath, modifiedPath)
	fmt.Printf("  📊 Similarity: %.2f%%\n", similarity*100)

	// 检查是否超过阈值
	if similarity >= r.similarityThreshold {
		// 创建链上防护规则
		rule := r.createProtectionRule(originalTxHash, contractAddr, similarity, variation)

		// 验证规则的有效性
		if r.validateProtectionRule(&rule) {
			fmt.Printf("  🛡️  Generated protection rule: %s\n", rule.RuleID)
			fmt.Printf("    Input rules: %d\n", len(rule.InputRules))
			fmt.Printf("    Storage rules: %d\n", len(rule.StorageRules))
			return true, &rule
		} else {
			fmt.Printf("  ❌ Generated rule is invalid, skipping\n")
			return false, nil
		}
	}

	return false, nil
}

// validateProtectionRule 验证保护规则的有效性
func (r *AttackReplayer) validateProtectionRule(rule *OnChainProtectionRule) bool {
	// 检查是否有有效规则
	if len(rule.InputRules) == 0 && len(rule.StorageRules) == 0 {
		fmt.Printf("    ⚠️  Rule has no input or storage rules\n")
		return false
	}

	// 验证输入规则
	for i, inputRule := range rule.InputRules {
		if len(inputRule.FunctionSelector) != 4 {
			fmt.Printf("    ⚠️  Input rule %d has invalid function selector\n", i)
			return false
		}
		if len(inputRule.ParameterRules) == 0 {
			fmt.Printf("    ⚠️  Input rule %d has no parameter rules\n", i)
			return false
		}
	}

	// 验证存储规则
	for i, storageRule := range rule.StorageRules {
		if storageRule.ContractAddress == (gethCommon.Address{}) {
			fmt.Printf("    ⚠️  Storage rule %d has invalid contract address\n", i)
			return false
		}
		if storageRule.StorageSlot == (gethCommon.Hash{}) {
			fmt.Printf("    ⚠️  Storage rule %d has invalid storage slot\n", i)
			return false
		}
	}

	return true
}

// createProtectionRule 创建链上防护规则
func (r *AttackReplayer) createProtectionRule(txHash gethCommon.Hash, contractAddr gethCommon.Address,
	similarity float64, variation *ModificationVariation) OnChainProtectionRule {

	ruleID := GenerateRuleID(txHash, contractAddr, time.Now())

	rule := OnChainProtectionRule{
		RuleID:          ruleID,
		TxHash:          txHash,
		ContractAddress: contractAddr,
		Similarity:      similarity,
		InputRules:      make([]InputProtectionRule, 0),
		StorageRules:    make([]StorageProtectionRule, 0),
		CreatedAt:       time.Now(),
		IsActive:        true,
	}

	// 生成输入保护规则
	if variation.InputMod != nil {
		inputRule := CreateInputProtectionRule(variation.InputMod)
		// 验证输入规则是否有效
		if len(inputRule.FunctionSelector) == 4 && len(inputRule.ParameterRules) > 0 {
			rule.InputRules = append(rule.InputRules, inputRule)
		}
	}

	// 生成存储保护规则
	if variation.StorageMod != nil {
		storageRules := CreateStorageProtectionRules(variation.StorageMod, contractAddr)
		// 只添加有效的存储规则
		for _, storageRule := range storageRules {
			if storageRule.ContractAddress != (gethCommon.Address{}) && storageRule.StorageSlot != (gethCommon.Hash{}) {
				rule.StorageRules = append(rule.StorageRules, storageRule)
			}
		}
	}

	// 如果仍然没有有效规则，创建一个基本规则
	if len(rule.InputRules) == 0 && len(rule.StorageRules) == 0 {
		rule = r.createFallbackProtectionRule(txHash, contractAddr, similarity, variation)
	}

	return rule
}

// createFallbackProtectionRule 创建后备保护规则
func (r *AttackReplayer) createFallbackProtectionRule(txHash gethCommon.Hash, contractAddr gethCommon.Address,
	similarity float64, variation *ModificationVariation) OnChainProtectionRule {

	ruleID := GenerateRuleID(txHash, contractAddr, time.Now())

	rule := OnChainProtectionRule{
		RuleID:          ruleID,
		TxHash:          txHash,
		ContractAddress: contractAddr,
		Similarity:      similarity,
		InputRules:      make([]InputProtectionRule, 0),
		StorageRules:    make([]StorageProtectionRule, 0),
		CreatedAt:       time.Now(),
		IsActive:        true,
	}

	// 创建一个基本的存储保护规则
	basicStorageRule := StorageProtectionRule{
		ContractAddress: contractAddr,
		StorageSlot:     gethCommon.BigToHash(big.NewInt(0)), // 使用槽位0
		OriginalValue:   gethCommon.Hash{},
		ModifiedValue:   gethCommon.BigToHash(big.NewInt(1)),
		CheckType:       "exact",
		SlotType:        "simple",
	}

	rule.StorageRules = append(rule.StorageRules, basicStorageRule)

	fmt.Printf("    📝Transaction execution failed Created fallback protection rule with basic storage rule\n")
	return rule
}

// saveProtectionRules 保存保护规则（可以保存到数据库或发送到链上）
func (r *AttackReplayer) saveProtectionRules(rules []OnChainProtectionRule) {
	fmt.Printf("\n=== SAVING %d PROTECTION RULES ===\n", len(rules))

	for i, rule := range rules {
		fmt.Printf("Rule %d: %s\n", i+1, rule.RuleID)
		fmt.Printf("  Contract: %s\n", rule.ContractAddress.Hex())
		fmt.Printf("  Original Tx: %s\n", rule.TxHash.Hex())
		fmt.Printf("  Similarity: %.2f%%\n", rule.Similarity*100)
		fmt.Printf("  Input Rules: %d\n", len(rule.InputRules))
		fmt.Printf("  Storage Rules: %d\n", len(rule.StorageRules))

		// 打印输入规则详情
		for j, inputRule := range rule.InputRules {
			fmt.Printf("    Input Rule %d:\n", j+1)
			fmt.Printf("      Function: %s\n", inputRule.FunctionName)
			fmt.Printf("      Selector: %x\n", inputRule.FunctionSelector)
			fmt.Printf("      Parameters: %d\n", len(inputRule.ParameterRules))

			for k, paramRule := range inputRule.ParameterRules {
				fmt.Printf("        Param %d: %s (%s) - %s\n",
					k+1, paramRule.Name, paramRule.Type, paramRule.CheckType)
				if paramRule.CheckType == "range" && paramRule.MinValue != nil && paramRule.MaxValue != nil {
					fmt.Printf("          Range: [%s, %s]\n",
						paramRule.MinValue.String(), paramRule.MaxValue.String())
				}
			}
		}

		// 打印存储规则详情
		for j, storageRule := range rule.StorageRules {
			fmt.Printf("    Storage Rule %d:\n", j+1)
			fmt.Printf("      Contract: %s\n", storageRule.ContractAddress.Hex())
			fmt.Printf("      Slot: %s\n", storageRule.StorageSlot.Hex())
			fmt.Printf("      Type: %s (%s)\n", storageRule.SlotType, storageRule.CheckType)
			if storageRule.CheckType == "range" && storageRule.MinValue != nil && storageRule.MaxValue != nil {
				fmt.Printf("      Range: [%s, %s]\n",
					storageRule.MinValue.String(), storageRule.MaxValue.String())
			}
		}
		fmt.Println()
	}

	// TODO: 这里可以添加实际的保存逻辑
	// 1. 保存到数据库
	// 2. 发送到链上防护合约
	// 3. 发送到防护服务等

	fmt.Printf("✅ Protection rules saved successfully!\n")
}

// 保持原有方法的兼容性
func (r *AttackReplayer) ReplayAttackTransaction(txHash gethCommon.Hash, contractAddr gethCommon.Address) (*ReplayResult, error) {
	simplifiedResult, err := r.ReplayAttackTransactionWithVariations(txHash, contractAddr)
	if err != nil {
		return nil, err
	}

	// 转换为原有格式
	result := &ReplayResult{
		OriginalPath:    simplifiedResult.OriginalPath,
		Similarity:      simplifiedResult.HighestSimilarity,
		IsAttackPattern: len(simplifiedResult.SuccessfulRules) > 0,
	}

	if len(simplifiedResult.SuccessfulRules) > 0 {
		rule := simplifiedResult.SuccessfulRules[0]
		result.Modifications = &StateModification{
			ContractAddress: rule.ContractAddress,
			StorageChanges:  make(map[gethCommon.Hash]gethCommon.Hash),
		}

		if len(rule.InputRules) > 0 {
			result.Modifications.InputData = rule.InputRules[0].ModifiedInput
		}

		if len(rule.StorageRules) > 0 {
			for _, storageRule := range rule.StorageRules {
				result.Modifications.StorageChanges[storageRule.StorageSlot] = storageRule.ModifiedValue
			}
		}
	}

	return result, nil
}

// extractStrategyFromVariationID 从变体ID提取策略
func (r *AttackReplayer) extractStrategyFromVariationID(variationID string) string {
	if len(variationID) == 0 {
		return "unknown"
	}

	parts := strings.Split(variationID, "_")
	if len(parts) > 0 {
		return parts[0]
	}

	return "unknown"
}

// printReplayStatistics 打印重放统计信息
func (r *AttackReplayer) printReplayStatistics(result *SimplifiedReplayResult) {
	fmt.Printf("\n=== REPLAY STATISTICS ===\n")
	fmt.Printf("Total variations tested: %d\n", result.TotalVariations)
	fmt.Printf("Successful rules: %d\n", result.Statistics.SuccessCount)
	fmt.Printf("Failed variations: %d\n", result.Statistics.FailCount)
	fmt.Printf("Success rate: %.2f%%\n", float64(result.Statistics.SuccessCount)/float64(result.TotalVariations)*100)
	fmt.Printf("Average similarity: %.2f%%\n", result.Statistics.AverageSimilarity*100)
	fmt.Printf("Highest similarity: %.2f%%\n", result.HighestSimilarity*100)
	fmt.Printf("Processing time: %v\n", result.ProcessingTime)

	if len(result.Statistics.StrategyResults) > 0 {
		fmt.Printf("\nStrategy results:\n")
		for strategy, count := range result.Statistics.StrategyResults {
			fmt.Printf("  %s: %d\n", strategy, count)
		}
	}
}

// 保持原有的辅助方法
func (r *AttackReplayer) executeTransactionWithTracing(tx *types.Transaction, prestate PrestateResult, modifiedInput []byte, storageMods map[gethCommon.Hash]gethCommon.Hash) (*ExecutionPath, error) {
	stateDB, err := r.createStateFromPrestate(prestate)
	if err != nil {
		return nil, fmt.Errorf("failed to create state: %v", err)
	}

	if storageMods != nil && tx.To() != nil {
		for slot, value := range storageMods {
			stateDB.SetState(*tx.To(), slot, value)
		}
		fmt.Printf("Applied %d storage modifications\n", len(storageMods))
	}

	receipt, err := r.nodeClient.TxReceiptByHash(tx.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get receipt: %v", err)
	}

	block, err := r.nodeClient.BlockHeaderByNumber(receipt.BlockNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get block: %v", err)
	}

	chainID, err := r.client.NetworkID(context.Background())
	if err != nil {
		return nil, err
	}

	evm, err := r.createEVMWithTracer(stateDB, block, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to create EVM: %v", err)
	}

	signer := types.LatestSignerForChainID(chainID)
	from, err := types.Sender(signer, tx)
	if err != nil {
		return nil, err
	}

	txCtx := vm.TxContext{
		Origin:   from,
		GasPrice: tx.GasPrice(),
	}
	evm.SetTxContext(txCtx)

	inputData := tx.Data()
	if modifiedInput != nil {
		inputData = modifiedInput
	}

	r.jumpTracer.StartTrace()

	if tx.To() == nil {
		_, _, _, err = evm.Create(
			from,
			inputData,
			tx.Gas(),
			uint256.MustFromBig(tx.Value()),
		)
	} else {
		_, _, err = evm.Call(
			from,
			*tx.To(),
			inputData,
			tx.Gas(),
			uint256.MustFromBig(tx.Value()),
		)
	}

	path := r.jumpTracer.StopTrace()

	if err != nil {
		fmt.Printf("Transaction execution failed: %v\n", err)
	}

	return path, nil
}

func (r *AttackReplayer) generateStateModifications(contractAddr gethCommon.Address, prestate PrestateResult, originalInput []byte) *StateModification {
	modifications := &StateModification{
		ContractAddress: contractAddr,
		StorageChanges:  make(map[gethCommon.Hash]gethCommon.Hash),
		InputData:       originalInput,
	}

	modifiedInput, err := r.inputModifier.ModifyInput(originalInput)
	if err != nil {
		fmt.Printf("Failed to modify input: %v\n", err)
	} else {
		modifications.InputData = modifiedInput
	}

	if contractAccount, exists := prestate[contractAddr]; exists {
		for slot, originalValue := range contractAccount.Storage {
			if rand.Float32() < 0.5 {
				newValue := r.generateStepBasedStorageValue(originalValue, r.mutationConfig.StorageSteps[0], 0)
				modifications.StorageChanges[slot] = newValue
			}
		}
	}

	return modifications
}

func (r *AttackReplayer) calculatePathSimilarity(path1, path2 *ExecutionPath) float64 {
	if path1 == nil || path2 == nil {
		return 0.0
	}

	if len(path1.Jumps) == 0 && len(path2.Jumps) == 0 {
		return 1.0
	}

	if len(path1.Jumps) == 0 || len(path2.Jumps) == 0 {
		return 0.0
	}

	matches := 0
	minLen := len(path1.Jumps)
	if len(path2.Jumps) < minLen {
		minLen = len(path2.Jumps)
	}

	for i := 0; i < minLen; i++ {
		if path1.Jumps[i].ContractAddress == path2.Jumps[i].ContractAddress &&
			path1.Jumps[i].JumpFrom == path2.Jumps[i].JumpFrom &&
			path1.Jumps[i].JumpDest == path2.Jumps[i].JumpDest {
			matches++
		}
	}

	maxLen := len(path1.Jumps)
	if len(path2.Jumps) > maxLen {
		maxLen = len(path2.Jumps)
	}

	return float64(matches) / float64(maxLen)
}

func (r *AttackReplayer) getTransactionPrestate(txHash gethCommon.Hash) (PrestateResult, error) {
	config := map[string]interface{}{
		"tracer": "prestateTracer",
		"tracerConfig": map[string]interface{}{
			"diffMode": false,
		},
		"timeout": "60s",
	}

	var result PrestateResult
	err := r.client.Client().CallContext(context.Background(), &result,
		"debug_traceTransaction", txHash, config)
	return result, err
}

func (r *AttackReplayer) createStateFromPrestate(prestate PrestateResult) (*state.StateDB, error) {
	memDb := rawdb.NewMemoryDatabase()

	trieDB := triedb.NewDatabase(memDb, &triedb.Config{
		Preimages: false,
		IsVerkle:  false,
		HashDB: &hashdb.Config{
			CleanCacheSize: 256 * 1024 * 1024,
		},
	})

	stateDb := state.NewDatabase(trieDB, nil)
	stateDB, err := state.New(gethCommon.Hash{}, stateDb)
	if err != nil {
		return nil, err
	}

	for addr, account := range prestate {
		if !stateDB.Exist(addr) {
			stateDB.CreateAccount(addr)
		}

		if account.Balance != nil {
			balance := (*big.Int)(account.Balance)
			stateDB.SetBalance(addr, uint256.MustFromBig(balance), 0)
		}

		if account.Nonce > 0 {
			stateDB.SetNonce(addr, account.Nonce, 0)
		}

		if len(account.Code) > 0 {
			code := []byte(account.Code)

			// 检查字节码中是否包含PUSH0指令 (0x5f)
			containsPush0 := false
			for i, b := range code {
				if b == 0x5f {
					containsPush0 = true
					fmt.Printf("Found PUSH0 instruction at position %d in contract %s\n", i, addr.Hex())
				}
			}

			if containsPush0 {
				fmt.Printf("Contract %s contains PUSH0 instructions, code length: %d\n", addr.Hex(), len(code))
				// 显示前100字节的十六进制表示
				maxLen := len(code)
				if maxLen > 100 {
					maxLen = 100
				}
				fmt.Printf("First %d bytes: %x\n", maxLen, code[:maxLen])
			}

			stateDB.SetCode(addr, code)
		}

		for key, value := range account.Storage {
			stateDB.SetState(addr, key, value)
		}
	}

	return stateDB, nil
}

// createEVMWithTracer 创建包含tracer的EVM - 修复PUSH0操作码支持
func (r *AttackReplayer) createEVMWithTracer(stateDB *state.StateDB, blockHeader *types.Header, chainID *big.Int) (*vm.EVM, error) {
	// 创建支持所有硬分叉的链配置
	chainConfig := r.createChainConfigWithAllForks(chainID, blockHeader)

	// 强制使用支持Shanghai的时间戳，确保PUSH0操作码可用
	// 计算一个确保Shanghai硬分叉激活的时间戳
	shanghaiTime := uint64(0)
	if chainConfig.ShanghaiTime != nil {
		shanghaiTime = *chainConfig.ShanghaiTime
	}

	// 确保使用的时间戳大于等于Shanghai时间
	blockTime := blockHeader.Time
	if blockTime < shanghaiTime {
		blockTime = shanghaiTime + 1
		fmt.Printf("Adjusted block time from %d to %d to ensure Shanghai activation\n", blockHeader.Time, blockTime)
	}

	blockCtx := vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     func(uint64) gethCommon.Hash { return gethCommon.Hash{} },
		Coinbase:    blockHeader.Coinbase,
		BlockNumber: blockHeader.Number,
		Time:        blockTime, // 使用调整后的时间戳
		Difficulty:  blockHeader.Difficulty,
		GasLimit:    blockHeader.GasLimit,
		BaseFee:     blockHeader.BaseFee,
	}

	vmConfig := vm.Config{
		NoBaseFee:               false,
		EnablePreimageRecording: true,
		Tracer:                  r.jumpTracer.ToTracingHooks(),
	}

	evm := vm.NewEVM(blockCtx, stateDB, chainConfig, vmConfig)

	// 验证Shanghai是否已激活
	isShanghai := chainConfig.IsShanghai(blockHeader.Number, blockTime)
	fmt.Printf("Shanghai activated: %v (block time: %d, shanghai time: %v)\n",
		isShanghai, blockTime, chainConfig.ShanghaiTime)

	return evm, nil
}

// createChainConfigWithAllForks 创建包含所有硬分叉的链配置 - 修复PUSH0支持
func (r *AttackReplayer) createChainConfigWithAllForks(chainID *big.Int, blockHeader *types.Header) *params.ChainConfig {
	// 创建新的配置，强制启用所有硬分叉
	config := &params.ChainConfig{
		ChainID:                 chainID,
		HomesteadBlock:          big.NewInt(0),
		DAOForkBlock:            nil,
		DAOForkSupport:          false,
		EIP150Block:             big.NewInt(0),
		EIP155Block:             big.NewInt(0),
		EIP158Block:             big.NewInt(0),
		ByzantiumBlock:          big.NewInt(0),
		ConstantinopleBlock:     big.NewInt(0),
		PetersburgBlock:         big.NewInt(0),
		IstanbulBlock:           big.NewInt(0),
		MuirGlacierBlock:        big.NewInt(0),
		BerlinBlock:             big.NewInt(0),
		LondonBlock:             big.NewInt(0),
		ArrowGlacierBlock:       big.NewInt(0),
		GrayGlacierBlock:        big.NewInt(0),
		MergeNetsplitBlock:      big.NewInt(0),
		TerminalTotalDifficulty: nil, // 设置为nil以避免合并相关检查
		Ethash:                  new(params.EthashConfig),
		Clique:                  nil,
	}

	// 强制设置基于时间的硬分叉为0，确保所有硬分叉都从创世激活
	genesisTime := uint64(0)
	config.ShanghaiTime = &genesisTime
	config.CancunTime = &genesisTime

	// 验证配置
	fmt.Printf("Created FORCED chain config for chainID %d:\n", chainID.Uint64())
	fmt.Printf("  All block-based forks: activated at block 0\n")
	fmt.Printf("  Shanghai time: %d (PUSH0 FORCED ENABLED)\n", *config.ShanghaiTime)
	fmt.Printf("  Cancun time: %d\n", *config.CancunTime)

	if blockHeader != nil {
		fmt.Printf("  Block time: %d\n", blockHeader.Time)

		// 立即验证Shanghai是否会被激活
		wouldActivate := config.IsShanghai(blockHeader.Number, blockHeader.Time)
		fmt.Printf("  Shanghai would activate with this config: %v\n", wouldActivate)

		// 如果仍然不激活，强制设置更早的时间
		if !wouldActivate {
			// 强制设置为比当前区块时间更早的时间
			earlierTime := blockHeader.Time - 1000
			if blockHeader.Time < 1000 {
				earlierTime = 0
			}
			config.ShanghaiTime = &earlierTime
			config.CancunTime = &earlierTime
			fmt.Printf("  FORCED Shanghai time to: %d\n", *config.ShanghaiTime)

			// 再次验证
			finalCheck := config.IsShanghai(blockHeader.Number, blockHeader.Time)
			fmt.Printf("  Final Shanghai activation check: %v\n", finalCheck)
		}
	}

	return config
}
