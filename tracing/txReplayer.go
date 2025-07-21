package tracing

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"math/big"
	"sync"
	"time"

	"github.com/DQYXACML/autopatch/database"
	"github.com/DQYXACML/autopatch/database/common"
	"github.com/DQYXACML/autopatch/synchronizer/node"
	"github.com/DQYXACML/autopatch/txmgr/ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
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

	// 管理器组件
	mutationManager *MutationManager
	stateManager    *StateManager
	prestateManager *PrestateManager
	executionEngine *ExecutionEngine
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

	// 创建组件管理器
	jumpTracer := NewJumpTracer()
	stateManager := NewStateManager(jumpTracer)
	prestateManager := NewPrestateManager(client)
	executionEngine := NewExecutionEngine(client, nodeClient, stateManager, jumpTracer)
	mutationManager := NewMutationManager(DefaultMutationConfig(), inputModifier)

	return &AttackReplayer{
		client:              client,
		nodeClient:          nodeClient,
		jumpTracer:          jumpTracer,
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
		mutationManager:     mutationManager,
		stateManager:        stateManager,
		prestateManager:     prestateManager,
		executionEngine:     executionEngine,
	}, nil
}

// SetMutationConfig 设置变异配置

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

// SendMutationTransactions 发送变异交易到合约
func (r *AttackReplayer) SendMutationTransactions(
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

	txHashes, err := r.SendMutationTransactions(contractAddr, mutationCollection.SuccessfulMutations, TokenGasLimit)
	if err != nil {
		fmt.Printf("❌ Failed to send some or all mutation transactions: %v\n", err)
		// 不返回错误，因为收集变异是成功的
	}

	return mutationCollection, txHashes, nil
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
	return r.mutationManager.GenerateStepBasedInputData(originalInput, variant)
}

// generateStepBasedStorageChanges 生成基于步长的存储变化（改进版）
func (r *AttackReplayer) generateStepBasedStorageChanges(originalStorage map[gethCommon.Hash]gethCommon.Hash, variant int) map[gethCommon.Hash]gethCommon.Hash {
	return r.mutationManager.GenerateStepBasedStorageChanges(originalStorage, variant)
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

// 保持原有方法的兼容性

// 保持原有的辅助方法
func (r *AttackReplayer) executeTransactionWithTracing(tx *types.Transaction, prestate PrestateResult, modifiedInput []byte, storageMods map[gethCommon.Hash]gethCommon.Hash) (*ExecutionPath, error) {
	return r.executionEngine.ExecuteTransactionWithTracing(tx, prestate, modifiedInput, storageMods)
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

// getTransactionCallTrace 获取交易的调用跟踪，提取所有被保护合约的调用数据
func (r *AttackReplayer) getTransactionCallTrace(txHash gethCommon.Hash, protectedContracts []gethCommon.Address) (*CallTrace, error) {
	fmt.Printf("=== EXTRACTING CALL TRACE ===\n")
	fmt.Printf("Transaction hash: %s\n", txHash.Hex())
	fmt.Printf("Protected contracts: %v\n", protectedContracts)

	// 使用 TraceCallPath 获取调用跟踪
	callFrame, err := r.nodeClient.TraceCallPath(txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to trace call path: %v", err)
	}

	// 将 node.callFrame 转换为 tracing.CallFrame
	rootCall := r.convertCallFrame(callFrame)

	// 创建 CallTrace 结构
	callTrace := &CallTrace{
		OriginalTxHash:     txHash,
		RootCall:           rootCall,
		ExtractedCalls:     make([]ExtractedCallData, 0),
		ProtectedContracts: protectedContracts,
	}

	// 递归提取与被保护合约相关的调用数据
	r.extractProtectedContractCalls(rootCall, protectedContracts, &callTrace.ExtractedCalls, 0)

	fmt.Printf("Extracted %d calls from protected contracts\n", len(callTrace.ExtractedCalls))
	for i, extractedCall := range callTrace.ExtractedCalls {
		fmt.Printf("  [%d] Contract: %s, From: %s, Input length: %d bytes\n",
			i, extractedCall.ContractAddress.Hex(), extractedCall.From.Hex(), len(extractedCall.InputData))
	}

	return callTrace, nil
}

// convertCallFrame 将 node.callFrame 转换为 tracing.CallFrame
func (r *AttackReplayer) convertCallFrame(nodeFrame *node.NodecallFrame) *CallFrame {
	if nodeFrame == nil {
		return nil
	}

	frame := &CallFrame{
		Type:    nodeFrame.Type,
		From:    nodeFrame.From,
		To:      nodeFrame.To,
		Input:   nodeFrame.Input,
		Gas:     nodeFrame.Gas,
		GasUsed: nodeFrame.GasUsed,
		Value:   nodeFrame.Value,
	}

	// 递归转换子调用
	if len(nodeFrame.Calls) > 0 {
		frame.Calls = make([]CallFrame, len(nodeFrame.Calls))
		for i, subCall := range nodeFrame.Calls {
			convertedSubCall := r.convertCallFrame(&subCall)
			if convertedSubCall != nil {
				frame.Calls[i] = *convertedSubCall
			}
		}
	}

	return frame
}

// extractProtectedContractCalls 递归提取与被保护合约相关的调用数据
func (r *AttackReplayer) extractProtectedContractCalls(frame *CallFrame, protectedContracts []gethCommon.Address, extractedCalls *[]ExtractedCallData, depth int) {
	if frame == nil {
		return
	}

	// 检查当前调用是否来自被保护的合约
	fromAddr := gethCommon.HexToAddress(frame.From)
	toAddr := gethCommon.HexToAddress(frame.To)

	// 检查 from 字段是否匹配被保护合约
	for _, protectedAddr := range protectedContracts {
		if fromAddr == protectedAddr {
			// 提取调用数据
			inputData, err := hexutil.Decode(frame.Input)
			if err != nil {
				fmt.Printf("Warning: failed to decode input data for call from %s: %v\n", fromAddr.Hex(), err)
				inputData = []byte{}
			}

			gas := uint64(0)
			if gasInt, err := hexutil.DecodeUint64(frame.Gas); err == nil {
				gas = gasInt
			}

			value := big.NewInt(0)
			if frame.Value != "" && frame.Value != "0x0" {
				if valueBig, ok := big.NewInt(0).SetString(frame.Value, 0); ok {
					value = valueBig
				}
			}

			extractedCall := ExtractedCallData{
				ContractAddress: protectedAddr,
				From:            fromAddr,
				InputData:       inputData,
				CallType:        frame.Type,
				Value:           value,
				Gas:             gas,
				Depth:           depth,
			}

			*extractedCalls = append(*extractedCalls, extractedCall)

			fmt.Printf("📞 Extracted call from protected contract %s:\n", protectedAddr.Hex())
			fmt.Printf("   To: %s\n", toAddr.Hex())
			fmt.Printf("   Input: %x (length: %d)\n", inputData, len(inputData))
			fmt.Printf("   Depth: %d\n", depth)
			break
		}

		// 也检查 to 字段，如果调用目标是被保护合约，也可能需要记录
		if toAddr == protectedAddr && frame.Input != "" && frame.Input != "0x" {
			inputData, err := hexutil.Decode(frame.Input)
			if err != nil {
				fmt.Printf("Warning: failed to decode input data for call to %s: %v\n", toAddr.Hex(), err)
				inputData = []byte{}
			}

			gas := uint64(0)
			if gasInt, err := hexutil.DecodeUint64(frame.Gas); err == nil {
				gas = gasInt
			}

			value := big.NewInt(0)
			if frame.Value != "" && frame.Value != "0x0" {
				if valueBig, ok := big.NewInt(0).SetString(frame.Value, 0); ok {
					value = valueBig
				}
			}

			extractedCall := ExtractedCallData{
				ContractAddress: protectedAddr,
				From:            fromAddr,
				InputData:       inputData,
				CallType:        frame.Type,
				Value:           value,
				Gas:             gas,
				Depth:           depth,
			}

			*extractedCalls = append(*extractedCalls, extractedCall)

			fmt.Printf("📞 Extracted call to protected contract %s:\n", protectedAddr.Hex())
			fmt.Printf("   From: %s\n", fromAddr.Hex())
			fmt.Printf("   Input: %x (length: %d)\n", inputData, len(inputData))
			fmt.Printf("   Depth: %d\n", depth)
			break
		}
	}

	// 递归处理子调用
	for _, subCall := range frame.Calls {
		r.extractProtectedContractCalls(&subCall, protectedContracts, extractedCalls, depth+1)
	}
}

// getTransactionPrestateWithAllContracts 获取交易的预状态，保存所有合约的存储
func (r *AttackReplayer) getTransactionPrestateWithAllContracts(txHash gethCommon.Hash) (PrestateResult, map[gethCommon.Address]map[gethCommon.Hash]gethCommon.Hash, error) {
	return r.prestateManager.GetTransactionPrestateWithAllContracts(txHash)
}

// generateStepBasedModificationCandidatesFromCalls 根据提取的调用数据生成基于步长的修改候选
func (r *AttackReplayer) generateStepBasedModificationCandidatesFromCalls(
	startID int,
	count int,
	extractedCalls []ExtractedCallData,
	originalStorage map[gethCommon.Address]map[gethCommon.Hash]gethCommon.Hash,
) []*ModificationCandidate {

	candidates := make([]*ModificationCandidate, 0, count)

	if len(extractedCalls) == 0 {
		fmt.Printf("⚠️  No extracted calls available for generating candidates\n")
		return candidates
	}

	for i := 0; i < count; i++ {
		candidate := &ModificationCandidate{
			ID:             fmt.Sprintf("call_step_candidate_%d", startID+i),
			GeneratedAt:    time.Now(),
			StorageChanges: make(map[gethCommon.Hash]gethCommon.Hash),
		}

		// 选择要变异的调用数据
		callIndex := i % len(extractedCalls)
		selectedCall := extractedCalls[callIndex]

		// 设置来源调用数据
		candidate.SourceCallData = &selectedCall

		// 根据策略选择修改类型
		modType := i % 3
		hasValidModification := false

		switch modType {
		case 0: // 只修改输入（基于步长）
			if len(selectedCall.InputData) > 0 {
				modifiedInput := r.generateStepBasedInputDataFromCall(selectedCall.InputData, i)
				if !bytesEqual(modifiedInput, selectedCall.InputData) {
					candidate.InputData = modifiedInput
					candidate.ModType = "input_step_from_call"
					candidate.Priority = 1
					hasValidModification = true
				}
			}
		case 1: // 只修改存储（基于步长，仅修改相关合约的存储槽）
			if contractStorage, exists := originalStorage[selectedCall.ContractAddress]; exists {
				storageChanges := r.generateStepBasedStorageChangesFromCall(contractStorage, i)
				if len(storageChanges) > 0 {
					candidate.StorageChanges = storageChanges
					candidate.ModType = "storage_step_from_call"
					candidate.Priority = 2
					hasValidModification = true
				}
			}
		case 2: // 同时修改输入和存储（基于步长）
			if len(selectedCall.InputData) > 0 {
				modifiedInput := r.generateStepBasedInputDataFromCall(selectedCall.InputData, i)
				if !bytesEqual(modifiedInput, selectedCall.InputData) {
					candidate.InputData = modifiedInput
					hasValidModification = true
				}
			}
			if contractStorage, exists := originalStorage[selectedCall.ContractAddress]; exists {
				storageChanges := r.generateStepBasedStorageChangesFromCall(contractStorage, i)
				if len(storageChanges) > 0 {
					candidate.StorageChanges = storageChanges
					hasValidModification = true
				}
			}
			if hasValidModification {
				candidate.ModType = "both_step_from_call"
				candidate.Priority = 3
			}
		}

		// 如果没有产生有效修改，强制生成一个
		if !hasValidModification {
			hasValidModification = r.forceValidModificationFromCall(candidate, selectedCall, originalStorage, i)
		}

		// 只添加有有效修改的候选
		if hasValidModification {
			candidate.ExpectedImpact = r.predictModificationImpactFromCall(candidate, selectedCall)
			candidates = append(candidates, candidate)
		} else {
			fmt.Printf("⚠️  Skipped candidate %s - no valid modifications generated from call\n", candidate.ID)
		}
	}

	fmt.Printf("Generated %d valid call-based candidates out of %d attempts\n", len(candidates), count)
	return candidates
}

// generateStepBasedInputDataFromCall 基于调用数据生成步长变异的输入数据
func (r *AttackReplayer) generateStepBasedInputDataFromCall(originalInput []byte, variant int) []byte {
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
		config := r.mutationManager.config
		stepIndex := variant % len(config.InputSteps)
		step := config.InputSteps[stepIndex]

		fmt.Printf("🔧 Applying call-based input step mutation: step=%d, variant=%d\n", step, variant)

		// 对参数数据进行步长变异
		r.applyStepMutationToBytes(paramData, step, variant)
	}

	return modified
}

// generateStepBasedStorageChangesFromCall 基于调用数据生成步长变异的存储变化
func (r *AttackReplayer) generateStepBasedStorageChangesFromCall(originalStorage map[gethCommon.Hash]gethCommon.Hash, variant int) map[gethCommon.Hash]gethCommon.Hash {
	changes := make(map[gethCommon.Hash]gethCommon.Hash)

	config := r.mutationManager.config
	if !config.OnlyPrestate || len(originalStorage) == 0 {
		fmt.Printf("⚠️  No original storage to mutate or OnlyPrestate disabled\n")
		return changes
	}

	// 根据变异配置选择步长
	stepIndex := variant % len(config.StorageSteps)
	step := config.StorageSteps[stepIndex]

	fmt.Printf("💾 Applying call-based storage step mutation: step=%d, variant=%d\n", step, variant)

	// 限制修改的存储槽数量
	mutationCount := 0
	maxMutations := config.MaxMutations
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

	fmt.Printf("💾 Generated %d storage changes from call data\n", len(changes))
	return changes
}

// forceValidModificationFromCall 基于调用数据强制生成有效的修改
func (r *AttackReplayer) forceValidModificationFromCall(
	candidate *ModificationCandidate,
	selectedCall ExtractedCallData,
	originalStorage map[gethCommon.Address]map[gethCommon.Hash]gethCommon.Hash,
	variant int,
) bool {
	// 策略1：强制修改输入数据
	if len(selectedCall.InputData) > 4 {
		modifiedInput := r.forceModifyInputDataFromCall(selectedCall.InputData, variant)
		if !bytesEqual(modifiedInput, selectedCall.InputData) {
			candidate.InputData = modifiedInput
			candidate.ModType = "forced_input_step_from_call"
			candidate.Priority = 1
			fmt.Printf("🔧 Forced input modification for call-based candidate %s\n", candidate.ID)
			return true
		}
	}

	// 策略2：强制修改存储（如果有相关合约的原始存储）
	if contractStorage, exists := originalStorage[selectedCall.ContractAddress]; exists && len(contractStorage) > 0 {
		storageChanges := r.forceModifyStorageDataFromCall(contractStorage, variant)
		if len(storageChanges) > 0 {
			candidate.StorageChanges = storageChanges
			candidate.ModType = "forced_storage_step_from_call"
			candidate.Priority = 2
			fmt.Printf("💾 Forced storage modification for call-based candidate %s\n", candidate.ID)
			return true
		}
	}

	// 策略3：创建虚拟存储修改（最后手段）
	virtualStorage := r.createVirtualStorageModificationFromCall(selectedCall, variant)
	if len(virtualStorage) > 0 {
		candidate.StorageChanges = virtualStorage
		candidate.ModType = "forced_virtual_storage_from_call"
		candidate.Priority = 3
		fmt.Printf("🔮 Created virtual storage modification for call-based candidate %s\n", candidate.ID)
		return true
	}

	return false
}

// forceModifyInputDataFromCall 基于调用数据强制修改输入数据
func (r *AttackReplayer) forceModifyInputDataFromCall(originalInput []byte, variant int) []byte {
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

// forceModifyStorageDataFromCall 基于调用数据强制修改存储数据
func (r *AttackReplayer) forceModifyStorageDataFromCall(originalStorage map[gethCommon.Hash]gethCommon.Hash, variant int) map[gethCommon.Hash]gethCommon.Hash {
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
				fmt.Printf("   Forced call-based storage change: slot %s: %s -> %s\n",
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
			fmt.Printf("   Last resort call-based storage change: slot %s: %s -> %s\n",
				slot.Hex()[:10]+"...", originalValue.Hex()[:10]+"...", newValue.Hex()[:10]+"...")
		}
	}

	return changes
}

// createVirtualStorageModificationFromCall 基于调用数据创建虚拟存储修改
func (r *AttackReplayer) createVirtualStorageModificationFromCall(selectedCall ExtractedCallData, variant int) map[gethCommon.Hash]gethCommon.Hash {
	changes := make(map[gethCommon.Hash]gethCommon.Hash)

	// 基于调用数据的哈希创建虚拟存储槽
	callHash := crypto.Keccak256Hash(selectedCall.InputData, selectedCall.ContractAddress.Bytes())

	// 创建一些虚拟的存储槽修改
	for i := 0; i < 2; i++ {
		slotData := append(callHash.Bytes(), big.NewInt(int64(variant*10+i+1)).Bytes()...)
		slot := crypto.Keccak256Hash(slotData)
		value := gethCommon.BigToHash(big.NewInt(int64(variant*100 + i + 42)))
		changes[slot] = value
		fmt.Printf("   Virtual call-based storage: slot %s = %s\n", slot.Hex()[:10]+"...", value.Hex()[:10]+"...")
	}

	return changes
}

// predictModificationImpactFromCall 基于调用数据预测修改影响
func (r *AttackReplayer) predictModificationImpactFromCall(candidate *ModificationCandidate, selectedCall ExtractedCallData) string {
	impact := candidate.ModType
	if selectedCall.ContractAddress != (gethCommon.Address{}) {
		impact += fmt.Sprintf("_on_contract_%s", selectedCall.ContractAddress.Hex()[:10])
	}
	if len(selectedCall.InputData) >= 4 {
		impact += fmt.Sprintf("_func_%x", selectedCall.InputData[:4])
	}
	return impact
}

// ReplayAndCollectMutations 重放攻击交易并收集所有变异数据（修改版本）
func (r *AttackReplayer) ReplayAndCollectMutations(txHash gethCommon.Hash, contractAddr gethCommon.Address) (*MutationCollection, error) {
	startTime := time.Now()

	fmt.Printf("=== ATTACK TRANSACTION REPLAY WITH MUTATION COLLECTION (ENHANCED) ===\n")
	fmt.Printf("Transaction hash: %s\n", txHash.Hex())
	fmt.Printf("Contract address: %s\n", contractAddr.Hex())

	// 获取交易详情
	tx, err := r.nodeClient.TxByHash(txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %v", err)
	}

	// 设置被保护合约列表（可以包含多个合约）
	protectedContracts := []gethCommon.Address{contractAddr}

	// 获取调用跟踪，提取与被保护合约相关的调用数据
	callTrace, err := r.getTransactionCallTrace(txHash, protectedContracts)
	if err != nil {
		return nil, fmt.Errorf("failed to get call trace: %v", err)
	}

	// 获取预状态，保存所有合约的存储
	prestate, allContractsStorage, err := r.getTransactionPrestateWithAllContracts(txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get prestate: %v", err)
	}

	// 创建变异数据集合
	mutationCollection := &MutationCollection{
		OriginalTxHash:      txHash,
		ContractAddress:     contractAddr,
		OriginalInputData:   tx.Data(), // 保留原始交易的输入数据作为参考
		OriginalStorage:     make(map[gethCommon.Hash]gethCommon.Hash),
		Mutations:           make([]MutationData, 0),
		SuccessfulMutations: make([]MutationData, 0),
		CreatedAt:           time.Now(),
		CallTrace:           callTrace,
		AllContractsStorage: allContractsStorage,
	}

	// 提取主要保护合约的原始存储状态
	if contractAccount, exists := prestate[contractAddr]; exists {
		mutationCollection.OriginalStorage = contractAccount.Storage
		fmt.Printf("Original storage slots for main contract: %d\n", len(contractAccount.Storage))
	}

	// 执行原始交易
	fmt.Printf("\n=== ORIGINAL EXECUTION ===\n")
	originalPath, err := r.executeTransactionWithTracing(tx, prestate, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to execute original transaction: %v", err)
	}
	fmt.Printf("Original execution path: %d jumps\n", len(originalPath.Jumps))

	// 如果没有提取到调用数据，回退到原始方法
	if len(callTrace.ExtractedCalls) == 0 {
		fmt.Printf("\n⚠️  No calls extracted from trace, falling back to original input data mutation\n")
		// 设置原始状态用于智能修改
		err = r.inputModifier.SetOriginalState(tx.Data(), mutationCollection.OriginalStorage)
		if err != nil {
			fmt.Printf("Failed to set original state: %v\n", err)
		}
	} else {
		fmt.Printf("\n=== USING EXTRACTED CALL DATA FOR MUTATIONS ===\n")
		// 使用提取的调用数据设置原始状态
		// 选择第一个提取的调用数据作为主要输入
		mainCallData := callTrace.ExtractedCalls[0].InputData
		err = r.inputModifier.SetOriginalState(mainCallData, mutationCollection.OriginalStorage)
		if err != nil {
			fmt.Printf("Failed to set original state from extracted calls: %v\n", err)
		}
	}

	// 生成并执行变异（使用基于调用数据的步长变异）
	fmt.Printf("\n=== GENERATING AND EXECUTING CALL-BASED STEP MUTATIONS ===\n")
	config := r.mutationManager.config
	fmt.Printf("Using mutation config: InputSteps=%v, StorageSteps=%v, OnlyPrestate=%v\n",
		config.InputSteps, config.StorageSteps, config.OnlyPrestate)

	// 生成多种变异候选
	totalCandidates := 50 // 减少数量以便测试
	batchSize := 10

	for i := 0; i < totalCandidates; i += batchSize {
		currentBatchSize := batchSize
		if i+batchSize > totalCandidates {
			currentBatchSize = totalCandidates - i
		}

		var candidates []*ModificationCandidate

		// 如果有提取的调用数据，使用基于调用的变异
		if len(callTrace.ExtractedCalls) > 0 {
			candidates = r.generateStepBasedModificationCandidatesFromCalls(i, currentBatchSize, callTrace.ExtractedCalls, allContractsStorage)
		} else {
			// 回退到原始方法
			candidates = r.generateStepBasedModificationCandidates(i, currentBatchSize, tx.Data(), mutationCollection.OriginalStorage)
		}

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
				SourceCallData: result.Candidate.SourceCallData, // 保存来源调用数据
			}

			if result.Error != nil {
				mutationData.ErrorMessage = result.Error.Error()
			}

			mutationCollection.Mutations = append(mutationCollection.Mutations, mutationData)

			// 收集成功的变异
			if result.Success && result.Similarity >= r.similarityThreshold {
				mutationCollection.SuccessfulMutations = append(mutationCollection.SuccessfulMutations, mutationData)
				fmt.Printf("✅ Successful call-based mutation %s: Similarity %.2f%%\n", result.Candidate.ID, result.Similarity*100)
				if result.Candidate.SourceCallData != nil {
					fmt.Printf("   Based on call to contract: %s\n", result.Candidate.SourceCallData.ContractAddress.Hex())
				}
			} else {
				fmt.Printf("❌ Failed call-based mutation %s: %s\n", result.Candidate.ID, mutationData.ErrorMessage)
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

	fmt.Printf("\n=== CALL-BASED MUTATION COLLECTION COMPLETED ===\n")
	fmt.Printf("Total mutations: %d\n", mutationCollection.TotalMutations)
	fmt.Printf("Successful mutations: %d\n", mutationCollection.SuccessCount)
	fmt.Printf("Failed mutations: %d\n", mutationCollection.FailureCount)
	fmt.Printf("Success rate: %.2f%%\n", float64(mutationCollection.SuccessCount)/float64(mutationCollection.TotalMutations)*100)
	fmt.Printf("Average similarity: %.2f%%\n", mutationCollection.AverageSimilarity*100)
	fmt.Printf("Highest similarity: %.2f%%\n", mutationCollection.HighestSimilarity*100)
	fmt.Printf("Processing time: %v\n", mutationCollection.ProcessingTime)
	fmt.Printf("Extracted calls used: %d\n", len(callTrace.ExtractedCalls))
	fmt.Printf("Contracts with storage: %d\n", len(allContractsStorage))

	return mutationCollection, nil
}
