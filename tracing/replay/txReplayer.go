package replay

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/DQYXACML/autopatch/database"
	"github.com/DQYXACML/autopatch/database/common"
	"github.com/DQYXACML/autopatch/database/utils"
	"github.com/DQYXACML/autopatch/synchronizer/node"
	"github.com/DQYXACML/autopatch/txmgr/ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	abiPkg "github.com/DQYXACML/autopatch/tracing/abi"
	"github.com/DQYXACML/autopatch/tracing/analysis"
	"github.com/DQYXACML/autopatch/tracing/core"
	"github.com/DQYXACML/autopatch/tracing/mutation"
	"github.com/DQYXACML/autopatch/tracing/state"
	tracingUtils "github.com/DQYXACML/autopatch/tracing/utils"
)

var (
	EthGasLimit          uint64 = 21000
	TokenGasLimit        uint64 = 120000
	maxFeePerGas                = big.NewInt(2900000000)
	maxPriorityFeePerGas        = big.NewInt(2600000000)
)

// bytesEqual 比较两个字节数组是否相等
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// AttackReplayer handles attack transaction replay and analysis
type AttackReplayer struct {
	client              *ethclient.Client
	nodeClient          node.EthClient
	jumpTracer          *tracingUtils.JumpTracer
	inputModifier       *mutation.InputModifier
	db                  *database.DB
	addressesDB         common.AddressesDB
	similarityThreshold float64
	maxVariations       int

	// Concurrent modification related fields
	concurrentConfig  *tracingUtils.ConcurrentModificationConfig
	transactionSender *ethereum.BatchTransactionSender
	privateKey        string
	privateKeyECDSA   *ecdsa.PrivateKey  // Parsed private key
	fromAddress       gethCommon.Address // Sender address
	chainID           *big.Int

	// Manager components
	mutationManager *mutation.MutationManager
	stateManager    *state.StateManager
	prestateManager *state.PrestateManager
	executionEngine *core.ExecutionEngine
	
	// Smart mutation components
	smartStrategy   *mutation.SmartMutationStrategy
	abiManager      *abiPkg.ABIManager
	typeAwareMutator *mutation.TypeAwareMutator
	storageAnalyzer *analysis.StorageAnalyzer
	storageTypeMutator *analysis.StorageTypeMutator
}

// NewAttackReplayer creates a new attack replayer
func NewAttackReplayer(rpcURL string, db *database.DB, contractsMetadata *bind.MetaData) (*AttackReplayer, error) {
	client, err := ethclient.Dial(rpcURL)
	nodeClient, err := node.DialEthClient(context.Background(), rpcURL)
	if err != nil {
		return nil, err
	}

	// Get chain ID
	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %v", err)
	}

	// Create ABI manager
	abiManager := abiPkg.NewABIManager("./abi_cache")
	fmt.Printf("🔧 ABI Manager created for chain %s\n", chainID.String())

	// Create type-aware mutator
	typeAwareMutator := mutation.NewTypeAwareMutator(chainID, abiManager)
	fmt.Printf("🧬 Type-aware mutator created\n")

	// Create InputModifier (using original approach first)
	inputModifier, err := mutation.NewInputModifier(contractsMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create input modifier: %v", err)
	}

	// Create batch transaction sender
	batchSender, err := ethereum.NewBatchTransactionSender(nodeClient, 4)
	if err != nil {
		return nil, fmt.Errorf("failed to create batch transaction sender: %v", err)
	}

	// Load private key from environment variable for security
	privateKeyHex := os.Getenv("AUTOPATCH_PRIVATE_KEY")
	if privateKeyHex == "" {
		return nil, fmt.Errorf("AUTOPATCH_PRIVATE_KEY environment variable not set - required for transaction signing")
	}

	// Parse private key
	privateKeyECDSA, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %v", err)
	}

	// Calculate sender address
	fromAddress := crypto.PubkeyToAddress(privateKeyECDSA.PublicKey)

	fmt.Printf("=== ATTACK REPLAYER INITIALIZED ===\n")
	fmt.Printf("From Address: %s\n", fromAddress.Hex())
	fmt.Printf("Chain ID: %s\n", chainID.String())
	fmt.Printf("RPC URL: %s\n", rpcURL)

	// Create smart mutation components
	smartStrategy := mutation.NewSmartMutationStrategy(0.8)
	storageAnalyzer := analysis.NewStorageAnalyzer(abiManager, chainID)
	storageTypeMutator := analysis.NewStorageTypeMutator(storageAnalyzer, typeAwareMutator)
	
	fmt.Printf("🧠 Smart mutation strategy created\n")
	fmt.Printf("🔍 Storage analyzer created\n")

	// Create component managers
	jumpTracer := tracingUtils.NewJumpTracer()
	stateManager := state.NewStateManager(jumpTracer)
	prestateManager := state.NewPrestateManager(client)
	executionEngine := core.NewExecutionEngine(client, nodeClient, stateManager, jumpTracer)
	mutationManager := mutation.NewMutationManager(mutation.DefaultMutationConfig(), inputModifier)

	replayer := &AttackReplayer{
		client:              client,
		nodeClient:          nodeClient,
		jumpTracer:          jumpTracer,
		inputModifier:       inputModifier,
		db:                  db,
		addressesDB:         db.Addresses,
		similarityThreshold: 0.8,
		maxVariations:       20,
		concurrentConfig:    tracingUtils.DefaultConcurrentModificationConfig(),
		transactionSender:   batchSender,
		privateKey:          privateKeyHex,
		privateKeyECDSA:     privateKeyECDSA,
		fromAddress:         fromAddress,
		chainID:             chainID,
		mutationManager:     mutationManager,
		stateManager:        stateManager,
		prestateManager:     prestateManager,
		executionEngine:     executionEngine,
		// Smart mutation components
		smartStrategy:      smartStrategy,
		abiManager:         abiManager,
		typeAwareMutator:   typeAwareMutator,
		storageAnalyzer:    storageAnalyzer,
		storageTypeMutator: storageTypeMutator,
	}

	// Initialize ABI manager API keys
	replayer.initializeABIManager(abiManager, typeAwareMutator)

	return replayer, nil
}

// initializeABIManager Initialize ABI manager
func (r *AttackReplayer) initializeABIManager(abiManager *abiPkg.ABIManager, typeAwareMutator *mutation.TypeAwareMutator) {
	// Set API keys from environment variables or config file
	etherscanKey := os.Getenv("ETHERSCAN_API_KEY")
	bscscanKey := os.Getenv("BSCSCAN_API_KEY")
	
	if etherscanKey != "" {
		abiManager.SetAPIKey(1, etherscanKey) // Ethereum
		fmt.Printf("🔑 Etherscan API key configured\n")
	} else {
		fmt.Printf("⚠️  No Etherscan API key found in environment\n")
	}
	
	if bscscanKey != "" {
		abiManager.SetAPIKey(56, bscscanKey) // BSC
		fmt.Printf("🔑 BscScan API key configured\n")
	} else {
		fmt.Printf("⚠️  No BscScan API key found in environment\n")
	}
	
	// Display ABI manager status
	stats := abiManager.GetCacheStats()
	fmt.Printf("📋 ABI Cache: %d in memory, %d in files\n", 
		stats["memory_cache_size"], stats["file_cache_size"])
}

// EnableTypeAwareMutation Enable type-aware mutation for specific contract
func (r *AttackReplayer) EnableTypeAwareMutation(contractAddr gethCommon.Address) error {
	if r.inputModifier == nil {
		return fmt.Errorf("input modifier not initialized")
	}

	// Create ABI manager and type-aware mutator (if not already created)
	abiManager := abiPkg.NewABIManager("./abi_cache")
	typeAwareMutator := mutation.NewTypeAwareMutator(r.chainID, abiManager)
	
	// Initialize API keys
	r.initializeABIManager(abiManager, typeAwareMutator)

	// Enable type-aware mutation
	r.inputModifier.EnableTypeAwareMutation(abiManager, typeAwareMutator, r.chainID, contractAddr)
	
	fmt.Printf("✅ Type-aware mutation enabled for contract %s\n", contractAddr.Hex())
	return nil
}

// DisableTypeAwareMutation Disable type-aware mutation
func (r *AttackReplayer) DisableTypeAwareMutation() {
	if r.inputModifier != nil {
		r.inputModifier.DisableTypeAwareMutation()
	}
}

// GetContractABI Get contract ABI (helper method)
func (r *AttackReplayer) GetContractABI(contractAddr gethCommon.Address) (*abi.ABI, error) {
	abiManager := abiPkg.NewABIManager("./abi_cache")
	r.initializeABIManager(abiManager, nil)
	
	return abiManager.GetContractABI(r.chainID, contractAddr)
}

// AnalyzeContract Analyze contract structure
func (r *AttackReplayer) AnalyzeContract(contractAddr gethCommon.Address) (*ContractAnalysis, error) {
	contractABI, err := r.GetContractABI(contractAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get contract ABI: %v", err)
	}

	analysis := &ContractAnalysis{
		Address: contractAddr,
		ChainID: r.chainID,
		ABI:     contractABI,
		Methods: make([]MethodAnalysis, 0, len(contractABI.Methods)),
	}

	// Analyze each method
	for name, method := range contractABI.Methods {
		methodAnalysis := MethodAnalysis{
			Name:      name,
			Signature: method.Sig,
			Inputs:    make([]ParameterAnalysis, len(method.Inputs)),
		}

		// Analyze each parameter
		for i, input := range method.Inputs {
			importance := r.calculateParameterImportanceScore(input)
			methodAnalysis.Inputs[i] = ParameterAnalysis{
				Name:        input.Name,
				Type:        input.Type.String(),
				Importance:  importance,
				Strategies:  r.getMutationStrategies(input.Type),
			}
		}

		analysis.Methods = append(analysis.Methods, methodAnalysis)
	}

	return analysis, nil
}

// calculateParameterImportanceScore Calculate parameter importance score
func (r *AttackReplayer) calculateParameterImportanceScore(input abi.Argument) float64 {
	// 基本重要性评分逻辑
	importance := 0.5 // 默认分数
	
	// 根据参数类型增加重要性
	switch input.Type.T {
	case abi.AddressTy:
		importance += 0.3 // 地址类型通常很重要
	case abi.UintTy, abi.IntTy:
		if input.Type.Size >= 256 {
			importance += 0.2 // 大整数类型
		} else {
			importance += 0.1 // 小整数类型
		}
	case abi.BoolTy:
		importance += 0.1 // 布尔类型
	case abi.StringTy, abi.BytesTy:
		importance += 0.15 // 字符串和字节类型
	}
	
	// 根据参数名称增加重要性
	nameBoost := r.calculateNameImportance(input.Name)
	importance += nameBoost
	
	// 确保分数在合理范围内
	if importance > 1.0 {
		importance = 1.0
	}
	if importance < 0.1 {
		importance = 0.1
	}
	
	return importance
}

// calculateNameImportance 根据参数名称计算重要性加成
func (r *AttackReplayer) calculateNameImportance(name string) float64 {
	// 转换为小写进行匹配
	lowerName := strings.ToLower(name)
	
	// 高重要性关键词
	highImportance := []string{"amount", "value", "price", "balance", "token", "address", "owner", "admin"}
	for _, keyword := range highImportance {
		if strings.Contains(lowerName, keyword) {
			return 0.3
		}
	}
	
	// 中等重要性关键词
	mediumImportance := []string{"id", "index", "count", "limit", "max", "min"}
	for _, keyword := range mediumImportance {
		if strings.Contains(lowerName, keyword) {
			return 0.2
		}
	}
	
	// 低重要性关键词
	lowImportance := []string{"data", "info", "meta", "extra"}
	for _, keyword := range lowImportance {
		if strings.Contains(lowerName, keyword) {
			return 0.1
		}
	}
	
	return 0.0 // 无匹配
}

// getMutationStrategies Get mutation strategies for type
func (r *AttackReplayer) getMutationStrategies(argType abi.Type) []string {
	strategies := []string{"step_based"}
	
	switch argType.T {
	case abi.AddressTy:
		strategies = append(strategies, "known_addresses", "nearby_addresses", "zero_address")
	case abi.UintTy, abi.IntTy:
		strategies = append(strategies, "boundary_values", "bit_patterns", "multiplication")
	case abi.BoolTy:
		strategies = append(strategies, "toggle")
	case abi.StringTy:
		strategies = append(strategies, "length_mutation", "encoding_mutation", "special_chars")
	case abi.BytesTy:
		strategies = append(strategies, "byte_flip", "length_change", "pattern_fill")
	}
	
	return strategies
}

// SetMutationConfig Set mutation configuration

// sendTransactionToContract Send transaction to contract
func (r *AttackReplayer) sendTransactionToContract(
	contractAddr gethCommon.Address,
	inputData []byte,
	storageChanges map[gethCommon.Hash]gethCommon.Hash,
	gasLimit uint64,
) (*gethCommon.Hash, error) {
	if r.privateKeyECDSA == nil {
		return nil, fmt.Errorf("private key not set")
	}

	// 1. Get nonce value
	nonce, err := r.nodeClient.TxCountByAddress(r.fromAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %v", err)
	}

	fmt.Printf("🔢 Current nonce for %s: %d\n", r.fromAddress.Hex(), uint64(nonce))

	// 2. Create transaction data structure
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

	// 3. Sign transaction offline
	rawTxHex, txHashStr, err := ethereum.OfflineSignTx(txData, r.privateKey, r.chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %v", err)
	}

	fmt.Printf("✍️  Transaction signed successfully\n")
	fmt.Printf("   Tx Hash: %s\n", txHashStr)
	fmt.Printf("   Raw Tx length: %d bytes\n", len(rawTxHex))

	// 4. Send raw transaction
	err = r.nodeClient.SendRawTransaction(rawTxHex)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %v", err)
	}

	txHash := gethCommon.HexToHash(txHashStr)
	fmt.Printf("🚀 Transaction sent successfully: %s\n", txHash.Hex())

	return &txHash, nil
}

// SendMutationTransactions Send mutation transactions to contract
func (r *AttackReplayer) SendMutationTransactions(
	contractAddr gethCommon.Address,
	mutations []tracingUtils.MutationData,
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

		// Ensure there is modified input data
		inputData := mutation.InputData
		if len(inputData) == 0 {
			fmt.Printf("⚠️  Mutation %s has no input data, skipping\n", mutation.ID)
			continue
		}

		// Send transaction
		txHash, err := r.sendTransactionToContract(contractAddr, inputData, mutation.StorageChanges, gasLimit)
		if err != nil {
			fmt.Printf("❌ Failed to send mutation %s: %v\n", mutation.ID, err)
			errors = append(errors, fmt.Errorf("mutation %s: %v", mutation.ID, err))
			continue
		}

		txHashes = append(txHashes, txHash)
		fmt.Printf("✅ Mutation %s sent: %s\n", mutation.ID, txHash.Hex())

		// Add small delay to avoid nonce conflict
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

	// If partially successful, return successful transaction hashes
	if len(txHashes) > 0 {
		return txHashes, nil
	}

	// If all failed, return the first error
	if len(errors) > 0 {
		return nil, errors[0]
	}

	return nil, fmt.Errorf("no transactions were sent")
}

// ReplayAndSendMutations Replay attack transaction, collect mutations and send to contract
func (r *AttackReplayer) ReplayAndSendMutations(txHash gethCommon.Hash, contractAddr gethCommon.Address) (*tracingUtils.MutationCollection, []*gethCommon.Hash, error) {
	fmt.Printf("=== REPLAY AND SEND MUTATIONS ===\n")

	// 1. Replay and collect mutations
	mutationCollection, err := r.ReplayAndCollectMutations(txHash, contractAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to replay and collect mutations: %v", err)
	}

	// 2. Send successful mutations to contract
	if len(mutationCollection.SuccessfulMutations) == 0 {
		fmt.Printf("⚠️  No successful mutations to send\n")
		return mutationCollection, nil, nil
	}

	fmt.Printf("🚀 Sending %d successful mutations to contract...\n", len(mutationCollection.SuccessfulMutations))

	txHashes, err := r.SendMutationTransactions(contractAddr, mutationCollection.SuccessfulMutations, TokenGasLimit)
	if err != nil {
		fmt.Printf("❌ Failed to send some or all mutation transactions: %v\n", err)
		// Don't return error because mutation collection was successful
	}

	return mutationCollection, txHashes, nil
}

// generateStepBasedModificationCandidates Generate step-based modification candidates (ensure each candidate has valid modifications)
func (r *AttackReplayer) generateStepBasedModificationCandidates(
	startID int,
	count int,
	originalInput []byte,
	originalStorage map[gethCommon.Hash]gethCommon.Hash,
) []*tracingUtils.ModificationCandidate {

	candidates := make([]*tracingUtils.ModificationCandidate, 0, count)

	for i := 0; i < count; i++ {
		candidate := &tracingUtils.ModificationCandidate{
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
	candidate *tracingUtils.ModificationCandidate,
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

// simulateModificationWithContext 使用执行上下文模拟修改
func (r *AttackReplayer) simulateModificationWithContext(
	candidate *tracingUtils.ModificationCandidate,
	ctx *tracingUtils.ExecutionContext,
	originalPath *tracingUtils.ExecutionPath,
) *tracingUtils.SimulationResult {

	startTime := time.Now()
	result := &tracingUtils.SimulationResult{
		Candidate: candidate,
		Success:   false,
	}

	// Create target calls map for intercepted execution
	targetCalls := make(map[gethCommon.Address][]byte)
	
	// If candidate has source call data, use its contract address
	if candidate.SourceCallData != nil && len(candidate.InputData) > 0 {
		targetCalls[candidate.SourceCallData.ContractAddress] = candidate.InputData
	} else if len(candidate.InputData) > 0 {
		// Fallback: use the main contract address from ctx
		if ctx.Transaction.To() != nil {
			targetCalls[*ctx.Transaction.To()] = candidate.InputData
		}
	}
	
	// Apply storage modifications to target calls
	// Note: storage mods are applied in ExecuteWithInterceptedCalls via ctx.AllContractsStorage

	// Execute with intercepted calls
	modifiedPath, err := r.executionEngine.ExecuteWithInterceptedCalls(ctx, targetCalls)
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

// simulateModification 模拟修改（保留以兼容旧代码）
func (r *AttackReplayer) simulateModification(
	candidate *tracingUtils.ModificationCandidate,
	tx *types.Transaction,
	prestate tracingUtils.PrestateResult,
	originalPath *tracingUtils.ExecutionPath,
) *tracingUtils.SimulationResult {

	startTime := time.Now()
	result := &tracingUtils.SimulationResult{
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
func (r *AttackReplayer) predictModificationImpact(candidate *tracingUtils.ModificationCandidate) string {
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

// executeMutationBatchWithContext 使用执行上下文并行执行一批变异
func (r *AttackReplayer) executeMutationBatchWithContext(candidates []*tracingUtils.ModificationCandidate, ctx *tracingUtils.ExecutionContext, originalPath *tracingUtils.ExecutionPath) []*tracingUtils.SimulationResult {
	results := make([]*tracingUtils.SimulationResult, len(candidates))

	// 使用goroutine并行执行
	var wg sync.WaitGroup
	for i, candidate := range candidates {
		wg.Add(1)
		go func(index int, cand *tracingUtils.ModificationCandidate) {
			defer wg.Done()
			results[index] = r.simulateModificationWithContext(cand, ctx, originalPath)
		}(i, candidate)
	}

	wg.Wait()
	return results
}

// executeMutationBatch 并行执行一批变异（保留以兼容旧代码）
func (r *AttackReplayer) executeMutationBatch(candidates []*tracingUtils.ModificationCandidate, tx *types.Transaction, prestate tracingUtils.PrestateResult, originalPath *tracingUtils.ExecutionPath) []*tracingUtils.SimulationResult {
	results := make([]*tracingUtils.SimulationResult, len(candidates))

	// 使用goroutine并行执行
	var wg sync.WaitGroup
	for i, candidate := range candidates {
		wg.Add(1)
		go func(index int, cand *tracingUtils.ModificationCandidate) {
			defer wg.Done()
			results[index] = r.simulateModification(cand, tx, prestate, originalPath)
		}(i, candidate)
	}

	wg.Wait()
	return results
}

// validateProtectionRule 验证保护规则的有效性
func (r *AttackReplayer) validateProtectionRule(rule *tracingUtils.OnChainProtectionRule) bool {
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
	similarity float64, variation *tracingUtils.ModificationVariation) tracingUtils.OnChainProtectionRule {

	ruleID := tracingUtils.GenerateRuleID(txHash, contractAddr, time.Now())

	rule := tracingUtils.OnChainProtectionRule{
		RuleID:          ruleID,
		TxHash:          txHash,
		ContractAddress: contractAddr,
		Similarity:      similarity,
		InputRules:      make([]tracingUtils.InputProtectionRule, 0),
		StorageRules:    make([]tracingUtils.StorageProtectionRule, 0),
		CreatedAt:       time.Now(),
		IsActive:        true,
	}

	// 生成输入保护规则
	if variation.InputMod != nil {
		inputRule := tracingUtils.CreateInputProtectionRule(variation.InputMod)
		// 验证输入规则是否有效
		if len(inputRule.FunctionSelector) == 4 && len(inputRule.ParameterRules) > 0 {
			rule.InputRules = append(rule.InputRules, inputRule)
		}
	}

	// 生成存储保护规则
	if variation.StorageMod != nil {
		storageRules := tracingUtils.CreateStorageProtectionRules(variation.StorageMod, contractAddr)
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
	similarity float64, variation *tracingUtils.ModificationVariation) tracingUtils.OnChainProtectionRule {

	ruleID := tracingUtils.GenerateRuleID(txHash, contractAddr, time.Now())

	rule := tracingUtils.OnChainProtectionRule{
		RuleID:          ruleID,
		TxHash:          txHash,
		ContractAddress: contractAddr,
		Similarity:      similarity,
		InputRules:      make([]tracingUtils.InputProtectionRule, 0),
		StorageRules:    make([]tracingUtils.StorageProtectionRule, 0),
		CreatedAt:       time.Now(),
		IsActive:        true,
	}

	// 创建一个基本的存储保护规则
	basicStorageRule := tracingUtils.StorageProtectionRule{
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

// executeTransactionWithContext 使用执行上下文执行交易
func (r *AttackReplayer) executeTransactionWithContext(ctx *tracingUtils.ExecutionContext, modifiedInput []byte, storageMods map[gethCommon.Hash]gethCommon.Hash) (*tracingUtils.ExecutionPath, error) {
	return r.executionEngine.ExecuteTransactionWithContext(ctx, modifiedInput, storageMods)
}

// 保持原有的辅助方法
func (r *AttackReplayer) executeTransactionWithTracing(tx *types.Transaction, prestate tracingUtils.PrestateResult, modifiedInput []byte, storageMods map[gethCommon.Hash]gethCommon.Hash) (*tracingUtils.ExecutionPath, error) {
	return r.executionEngine.ExecuteTransactionWithTracing(tx, prestate, modifiedInput, storageMods)
}

func (r *AttackReplayer) calculatePathSimilarity(path1, path2 *tracingUtils.ExecutionPath) float64 {
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
func (r *AttackReplayer) getTransactionCallTrace(txHash gethCommon.Hash, protectedContracts []gethCommon.Address) (*tracingUtils.CallTrace, error) {
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
	callTrace := &tracingUtils.CallTrace{
		OriginalTxHash:     txHash,
		RootCall:           rootCall,
		ExtractedCalls:     make([]tracingUtils.ExtractedCallData, 0),
		ProtectedContracts: protectedContracts,
	}

	// 递归提取与被保护合约相关的调用数据，只提取第一个匹配的
	r.extractProtectedContractCalls(rootCall, protectedContracts, &callTrace.ExtractedCalls, 0)

	fmt.Printf("Extracted %d calls from protected contracts\n", len(callTrace.ExtractedCalls))
	for i, extractedCall := range callTrace.ExtractedCalls {
		fmt.Printf("  [%d] Contract: %s, From: %s, Input length: %d bytes\n",
			i, extractedCall.ContractAddress.Hex(), extractedCall.From.Hex(), len(extractedCall.InputData))
	}

	return callTrace, nil
}

// convertCallFrame 将 node.callFrame 转换为 tracing.CallFrame
func (r *AttackReplayer) convertCallFrame(nodeFrame *node.NodecallFrame) *tracingUtils.CallFrame {
	if nodeFrame == nil {
		return nil
	}

	frame := &tracingUtils.CallFrame{
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
		frame.Calls = make([]tracingUtils.CallFrame, len(nodeFrame.Calls))
		for i, subCall := range nodeFrame.Calls {
			convertedSubCall := r.convertCallFrame(&subCall)
			if convertedSubCall != nil {
				frame.Calls[i] = *convertedSubCall
			}
		}
	}

	return frame
}

// extractProtectedContractCalls 递归提取与被保护合约相关的调用数据，只找第一个匹配的
func (r *AttackReplayer) extractProtectedContractCalls(frame *tracingUtils.CallFrame, protectedContracts []gethCommon.Address, extractedCalls *[]tracingUtils.ExtractedCallData, depth int) bool {
	if frame == nil {
		return false
	}

	// 检查调用目标是否为被保护的合约
	fromAddr := gethCommon.HexToAddress(frame.From)
	toAddr := gethCommon.HexToAddress(frame.To)

	// 检查 to 字段，如果调用目标是被保护合约，记录调用数据
	for _, protectedAddr := range protectedContracts {
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

			extractedCall := tracingUtils.ExtractedCallData{
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
			return true // 找到第一个匹配就返回
		}
	}

	// 递归处理子调用，如果找到匹配就立即返回
	for _, subCall := range frame.Calls {
		if r.extractProtectedContractCalls(&subCall, protectedContracts, extractedCalls, depth+1) {
			return true // 子调用找到匹配，立即返回
		}
	}
	
	return false // 没有找到匹配
}

// getTransactionPrestateWithAllContracts 获取交易的预状态，保存所有合约的存储
func (r *AttackReplayer) getTransactionPrestateWithAllContracts(txHash gethCommon.Hash) (tracingUtils.PrestateResult, map[gethCommon.Address]map[gethCommon.Hash]gethCommon.Hash, error) {
	return r.prestateManager.GetTransactionPrestateWithAllContracts(txHash)
}

// generateStepBasedModificationCandidatesFromCalls 根据提取的调用数据生成基于步长的修改候选
func (r *AttackReplayer) generateStepBasedModificationCandidatesFromCalls(
	startID int,
	count int,
	extractedCalls []tracingUtils.ExtractedCallData,
	originalStorage map[gethCommon.Address]map[gethCommon.Hash]gethCommon.Hash,
) []*tracingUtils.ModificationCandidate {

	candidates := make([]*tracingUtils.ModificationCandidate, 0, count)

	if len(extractedCalls) == 0 {
		fmt.Printf("⚠️  No extracted calls available for generating candidates\n")
		return candidates
	}

	for i := 0; i < count; i++ {
		candidate := &tracingUtils.ModificationCandidate{
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
		config := mutation.DefaultMutationConfig()
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

	config := mutation.DefaultMutationConfig()
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
	candidate *tracingUtils.ModificationCandidate,
	selectedCall tracingUtils.ExtractedCallData,
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
func (r *AttackReplayer) createVirtualStorageModificationFromCall(selectedCall tracingUtils.ExtractedCallData, variant int) map[gethCommon.Hash]gethCommon.Hash {
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
func (r *AttackReplayer) predictModificationImpactFromCall(candidate *tracingUtils.ModificationCandidate, selectedCall tracingUtils.ExtractedCallData) string {
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
func (r *AttackReplayer) ReplayAndCollectMutations(txHash gethCommon.Hash, contractAddr gethCommon.Address) (*tracingUtils.MutationCollection, error) {
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

	// 创建执行上下文 - 一次性获取所有需要的信息
	fmt.Printf("\n=== CREATING EXECUTION CONTEXT ===\n")
	receipt, err := r.nodeClient.TxReceiptByHash(txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get receipt: %v", err)
	}

	block, err := r.nodeClient.BlockHeaderByNumber(receipt.BlockNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get block: %v", err)
	}

	chainID, err := r.client.NetworkID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %v", err)
	}

	execCtx, err := tracingUtils.NewExecutionContext(tx, receipt, block, chainID, prestate, allContractsStorage)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution context: %v", err)
	}
	fmt.Printf("✅ Execution context created: ChainID=%s, Block=%d\n", chainID.String(), block.Number.Uint64())

	// 创建变异数据集合
	mutationCollection := &tracingUtils.MutationCollection{
		OriginalTxHash:      txHash,
		ContractAddress:     contractAddr,
		OriginalInputData:   tx.Data(), // 保留原始交易的输入数据作为参考
		OriginalStorage:     make(map[gethCommon.Hash]gethCommon.Hash),
		Mutations:           make([]tracingUtils.MutationData, 0),
		SuccessfulMutations: make([]tracingUtils.MutationData, 0),
		CreatedAt:           time.Now(),
		CallTrace:           callTrace,
		AllContractsStorage: allContractsStorage,
	}

	// 提取主要保护合约的原始存储状态
	if contractAccount, exists := prestate[contractAddr]; exists {
		mutationCollection.OriginalStorage = contractAccount.Storage
		fmt.Printf("Original storage slots for main contract: %d\n", len(contractAccount.Storage))
	}

	// 执行原始交易 - 使用 InterceptingEVM 以确保只记录目标合约的跳转
	fmt.Printf("\n=== ORIGINAL EXECUTION ===\n")
	
	// 设置目标合约，使用空的 targetCalls（不修改输入）
	targetCalls := make(map[gethCommon.Address][]byte)
	// 通知 InterceptingEVM 哪些是目标合约，但不修改输入
	for _, protectedAddr := range protectedContracts {
		targetCalls[protectedAddr] = nil // nil 表示不修改输入
	}
	
	originalPath, err := r.executionEngine.ExecuteWithInterceptedCalls(execCtx, targetCalls)
	if err != nil {
		return nil, fmt.Errorf("failed to execute original transaction: %v", err)
	}
	fmt.Printf("Original execution path: %d jumps (target contract only)\n", len(originalPath.Jumps))

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
	config := mutation.DefaultMutationConfig()
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

		var candidates []*tracingUtils.ModificationCandidate

		// 如果有提取的调用数据，使用基于调用的变异
		if len(callTrace.ExtractedCalls) > 0 {
			candidates = r.generateStepBasedModificationCandidatesFromCalls(i, currentBatchSize, callTrace.ExtractedCalls, allContractsStorage)
		} else {
			// 回退到原始方法
			candidates = r.generateStepBasedModificationCandidates(i, currentBatchSize, tx.Data(), mutationCollection.OriginalStorage)
		}

		// 并行执行这批变异 - 使用执行上下文
		mutationResults := r.executeMutationBatchWithContext(candidates, execCtx, originalPath)

		// 收集结果
		for _, result := range mutationResults {
			mutationData := tracingUtils.MutationData{
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

// ContractAnalysis 合约分析结果
type ContractAnalysis struct {
	Address gethCommon.Address `json:"address"`
	ChainID *big.Int           `json:"chainId"`
	ABI     *abi.ABI           `json:"abi"`
	Methods []MethodAnalysis   `json:"methods"`
}

// MethodAnalysis 方法分析结果
type MethodAnalysis struct {
	Name      string              `json:"name"`
	Signature string              `json:"signature"`
	Inputs    []ParameterAnalysis `json:"inputs"`
}

// ParameterAnalysis 参数分析结果
type ParameterAnalysis struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Importance float64  `json:"importance"`
	Strategies []string `json:"strategies"`
}

// ExecuteSmartMutationCampaign 执行智能变异活动
func (r *AttackReplayer) ExecuteSmartMutationCampaign(
	txHash gethCommon.Hash,
	targetContracts []gethCommon.Address,
) (*SmartMutationCampaignResult, error) {
	fmt.Printf("\n=== 开始智能变异活动 ===\n")
	fmt.Printf("交易哈希: %s\n", txHash.Hex())
	fmt.Printf("目标合约数量: %d\n", len(targetContracts))
	
	startTime := time.Now()
	
	// 获取原始交易信息
	originalTx, _, err := r.client.TransactionByHash(context.Background(), txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get original transaction: %v", err)
	}
	
	// 获取prestate
	prestate, err := r.prestateManager.GetPrestate(txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get prestate: %v", err)
	}
	
	// 分析目标合约
	contractAnalyses := make(map[gethCommon.Address]*ContractAnalysis)
	allSlotInfos := make(map[gethCommon.Address][]tracingUtils.StorageSlotInfo)
	
	for _, contractAddr := range targetContracts {
		// 启用类型感知变异
		if err := r.EnableTypeAwareMutation(contractAddr); err != nil {
			fmt.Printf("⚠️  Failed to enable type-aware mutation for %s: %v\n", contractAddr.Hex(), err)
			continue
		}
		
		// 分析合约
		analysis, err := r.AnalyzeContract(contractAddr)
		if err != nil {
			fmt.Printf("⚠️  Failed to analyze contract %s: %v\n", contractAddr.Hex(), err)
			continue
		}
		contractAnalyses[contractAddr] = analysis
		
		// 分析存储
		if contractStorage, exists := prestate[contractAddr]; exists {
			slotInfos, err := r.storageAnalyzer.AnalyzeContractStorage(contractAddr, contractStorage.Storage)
			if err != nil {
				fmt.Printf("⚠️  Failed to analyze storage for %s: %v\n", contractAddr.Hex(), err)
				continue
			}
			allSlotInfos[contractAddr] = slotInfos
		}
	}
	
	// 生成智能变异计划
	mutationPlans := make([]*mutation.MutationPlan, 0)
	for contractAddr, slotInfos := range allSlotInfos {
		plan := r.smartStrategy.GetOptimalMutationPlan(contractAddr, slotInfos, len(originalTx.Data()))
		mutationPlans = append(mutationPlans, plan)
		
		fmt.Printf("\n📋 为合约 %s 生成变异计划:\n", contractAddr.Hex()[:10]+"...")
		plan.PrintPlan()
	}
	
	// 执行变异计划
	campaignResult := &SmartMutationCampaignResult{
		TransactionHash:    txHash,
		TargetContracts:   targetContracts,
		ContractAnalyses:  contractAnalyses,
		MutationPlans:     mutationPlans,
		Results:           make([]*SmartMutationResult, 0),
		StartTime:         startTime,
	}
	
	// 执行每个计划
	for _, plan := range mutationPlans {
		planResults, err := r.executeMutationPlan(originalTx, plan, prestate)
		if err != nil {
			fmt.Printf("⚠️  Failed to execute plan for %s: %v\n", plan.ContractAddress.Hex(), err)
			continue
		}
		
		campaignResult.Results = append(campaignResult.Results, planResults...)
		
		// 记录结果到智能策略中
		for _, result := range planResults {
			mutationResult := mutation.MutationResult{
				Variant:         result.Variant,
				ExecutionPath:   result.ExecutionPath,
				SimilarityScore: result.SimilarityScore,
				ExecutionTime:   result.ExecutionTime,
				Success:         result.Success,
				InputData:       result.MutatedInputData,
				StorageChanges:  result.StorageChanges,
				MutationType:    result.Strategy,
			}
			r.smartStrategy.RecordMutationResult(mutationResult)
		}
	}
	
	// 计算总体统计
	campaignResult.EndTime = time.Now()
	campaignResult.TotalDuration = campaignResult.EndTime.Sub(campaignResult.StartTime)
	campaignResult.TotalMutations = len(campaignResult.Results)
	
	successCount := 0
	totalSimilarity := 0.0
	highestSimilarity := 0.0
	
	for _, result := range campaignResult.Results {
		if result.Success {
			successCount++
			totalSimilarity += result.SimilarityScore
			if result.SimilarityScore > highestSimilarity {
				highestSimilarity = result.SimilarityScore
			}
		}
	}
	
	campaignResult.SuccessCount = successCount
	campaignResult.SuccessRate = float64(successCount) / float64(campaignResult.TotalMutations)
	if successCount > 0 {
		campaignResult.AverageSimilarity = totalSimilarity / float64(successCount)
	}
	campaignResult.HighestSimilarity = highestSimilarity
	
	// 打印活动结果
	fmt.Printf("\n=== 智能变异活动完成 ===\n")
	fmt.Printf("总变异数: %d\n", campaignResult.TotalMutations)
	fmt.Printf("成功变异: %d\n", campaignResult.SuccessCount)
	fmt.Printf("成功率: %.2f%%\n", campaignResult.SuccessRate*100)
	fmt.Printf("平均相似度: %.2f%%\n", campaignResult.AverageSimilarity*100)
	fmt.Printf("最高相似度: %.2f%%\n", campaignResult.HighestSimilarity*100)
	fmt.Printf("总耗时: %v\n", campaignResult.TotalDuration)
	
	// 显示策略统计
	fmt.Printf("\n=== 策略性能统计 ===\n")
	strategyStats := r.smartStrategy.GetStrategyStats()
	for name, stats := range strategyStats {
		if stats.TotalAttempts > 0 {
			fmt.Printf("%s: 成功率=%.2f%%, 平均相似度=%.2f%%, 尝试次数=%d\n",
				name, stats.SuccessRate*100, stats.AverageSimilarity*100, stats.TotalAttempts)
		}
	}
	
	return campaignResult, nil
}

// executeMutationPlan 执行单个变异计划
func (r *AttackReplayer) executeMutationPlan(
	originalTx *types.Transaction,
	plan *mutation.MutationPlan,
	prestate map[gethCommon.Address]*utils.ContractState,
) ([]*SmartMutationResult, error) {
	results := make([]*SmartMutationResult, 0)
	
	// 执行存储变异
	for _, storagePlan := range plan.StorageMutations {
		result, err := r.executeStorageMutation(originalTx, storagePlan, prestate)
		if err != nil {
			fmt.Printf("⚠️  Storage mutation failed: %v\n", err)
			continue
		}
		results = append(results, result)
	}
	
	// 执行输入数据变异
	for _, inputPlan := range plan.InputMutations {
		result, err := r.executeInputMutation(originalTx, inputPlan, prestate)
		if err != nil {
			fmt.Printf("⚠️  Input mutation failed: %v\n", err)
			continue
		}
		results = append(results, result)
	}
	
	return results, nil
}

// executeStorageMutation 执行存储变异
func (r *AttackReplayer) executeStorageMutation(
	originalTx *types.Transaction,
	plan mutation.StorageMutationPlan,
	prestate map[gethCommon.Address]*utils.ContractState,
) (*SmartMutationResult, error) {
	startTime := time.Now()
	
	// 复制原始存储状态
	mutatedPrestate := r.copyPrestate(prestate)
	
	// 找到对应的合约地址（这里需要根据计划找到正确的合约）
	var targetContractAddr gethCommon.Address
	for addr, contractState := range mutatedPrestate {
		if len(contractState.Storage) > 0 {
			targetContractAddr = addr
			break // 简化处理，选择第一个有存储的合约
		}
	}
	
	// 获取目标合约的存储
	contractState, exists := mutatedPrestate[targetContractAddr]
	if !exists {
		return nil, fmt.Errorf("contract state not found for storage mutation")
	}
	
	// 变异存储
	mutatedStorage, err := r.storageTypeMutator.MutateStorage(
		targetContractAddr,
		contractState.Storage,
		plan.Variant,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to mutate storage: %v", err)
	}
	
	// 更新预状态
	contractState.Storage = mutatedStorage
	
	// 执行变异后的交易
	mutatedTx := originalTx // 存储变异不改变交易本身
	trace, err := r.executionEngine.ExecuteTransaction(mutatedTx, mutatedPrestate)
	if err != nil {
		return &SmartMutationResult{
			Strategy:          plan.Strategy,
			Variant:           plan.Variant,
			Success:           false,
			ExecutionTime:     time.Since(startTime),
			Error:             err.Error(),
			TargetSlot:        &plan.TargetSlot,
		}, nil
	}
	
	// 计算相似度
	originalTrace, _ := r.executionEngine.ExecuteTransaction(originalTx, prestate)
	similarity := r.jumpTracer.CalculateSimilarity(originalTrace, trace)
	
	result := &SmartMutationResult{
		Strategy:          plan.Strategy,
		Variant:           plan.Variant,
		Success:           true,
		SimilarityScore:   similarity,
		ExecutionTime:     time.Since(startTime),
		ExecutionPath:     trace,
		StorageChanges:    mutatedStorage,
		TargetSlot:        &plan.TargetSlot,
		MutatedInputData:  originalTx.Data(),
	}
	
	return result, nil
}

// executeInputMutation 执行输入数据变异
func (r *AttackReplayer) executeInputMutation(
	originalTx *types.Transaction,
	plan mutation.InputMutationPlan,
	prestate map[gethCommon.Address]*utils.ContractState,
) (*SmartMutationResult, error) {
	startTime := time.Now()
	
	// 变异输入数据
	mutatedInputData, err := r.inputModifier.ModifyInputDataByStrategy(originalTx.Data(), plan.Strategy, plan.Variant)
	if err != nil {
		return &SmartMutationResult{
			Strategy:          plan.Strategy,
			Variant:           plan.Variant,
			Success:           false,
			ExecutionTime:     time.Since(startTime),
			Error:             err.Error(),
			TargetArgIndex:    &plan.TargetArgIndex,
		}, nil
	}
	
	// 创建变异后的交易
	mutatedTx := types.NewTransaction(
		originalTx.Nonce(),
		*originalTx.To(),
		originalTx.Value(),
		originalTx.Gas(),
		originalTx.GasPrice(),
		mutatedInputData,
	)
	
	// 执行变异后的交易
	trace, err := r.executionEngine.ExecuteTransaction(mutatedTx, prestate)
	if err != nil {
		return &SmartMutationResult{
			Strategy:          plan.Strategy,
			Variant:           plan.Variant,
			Success:           false,
			ExecutionTime:     time.Since(startTime),
			Error:             err.Error(),
			TargetArgIndex:    &plan.TargetArgIndex,
			MutatedInputData:  mutatedInputData,
		}, nil
	}
	
	// 计算相似度
	originalTrace, _ := r.executionEngine.ExecuteTransaction(originalTx, prestate)
	similarity := r.jumpTracer.CalculateSimilarity(originalTrace, trace)
	
	result := &SmartMutationResult{
		Strategy:          plan.Strategy,
		Variant:           plan.Variant,
		Success:           true,
		SimilarityScore:   similarity,
		ExecutionTime:     time.Since(startTime),
		ExecutionPath:     trace,
		MutatedInputData:  mutatedInputData,
		TargetArgIndex:    &plan.TargetArgIndex,
	}
	
	return result, nil
}

// copyPrestate 复制预状态
func (r *AttackReplayer) copyPrestate(prestate map[gethCommon.Address]*utils.ContractState) map[gethCommon.Address]*utils.ContractState {
	copied := make(map[gethCommon.Address]*utils.ContractState)
	
	for addr, state := range prestate {
		copiedStorage := make(map[gethCommon.Hash]gethCommon.Hash)
		for slot, value := range state.Storage {
			copiedStorage[slot] = value
		}
		
		copied[addr] = &utils.ContractState{
			Storage: copiedStorage,
			Code:    state.Code,
			Balance: state.Balance,
			Nonce:   state.Nonce,
		}
	}
	
	return copied
}

// GetSmartStrategyStats 获取智能策略统计
func (r *AttackReplayer) GetSmartStrategyStats() map[string]interface{} {
	if r.smartStrategy == nil {
		return map[string]interface{}{"error": "smart strategy not initialized"}
	}
	
	return r.smartStrategy.GetOverallStats()
}

// UpdateSmartStrategyThreshold 更新智能策略的相似度阈值
func (r *AttackReplayer) UpdateSmartStrategyThreshold(threshold float64) {
	if r.smartStrategy != nil {
		r.smartStrategy.UpdateSimilarityThreshold(threshold)
		fmt.Printf("🎯 Smart strategy similarity threshold updated to %.2f\n", threshold)
	}
}

// ResetSmartStrategy 重置智能策略（用于新实验）
func (r *AttackReplayer) ResetSmartStrategy() {
	if r.smartStrategy != nil {
		r.smartStrategy.ResetStrategies()
		fmt.Printf("🔄 Smart strategy reset completed\n")
	}
}

// SmartMutationCampaignResult 智能变异活动结果 
type SmartMutationCampaignResult struct {
	TransactionHash   gethCommon.Hash                           `json:"transactionHash"` 
	TargetContracts   []gethCommon.Address                      `json:"targetContracts"`
	ContractAnalyses  map[gethCommon.Address]*ContractAnalysis  `json:"contractAnalyses"`
	MutationPlans     []*mutation.MutationPlan                           `json:"mutationPlans"`
	Results           []*SmartMutationResult                    `json:"results"`
	StartTime         time.Time                                 `json:"startTime"`
	EndTime           time.Time                                 `json:"endTime"`
	TotalDuration     time.Duration                             `json:"totalDuration"`
	TotalMutations    int                                       `json:"totalMutations"`
	SuccessCount      int                                       `json:"successCount"`
	SuccessRate       float64                                   `json:"successRate"`
	AverageSimilarity float64                                   `json:"averageSimilarity"`
	HighestSimilarity float64                                   `json:"highestSimilarity"`
}

// SmartMutationResult 智能变异结果
type SmartMutationResult struct {
	Strategy          string                           `json:"strategy"`
	Variant           int                              `json:"variant"`
	Success           bool                             `json:"success"`
	SimilarityScore   float64                          `json:"similarityScore"`
	ExecutionTime     time.Duration                    `json:"executionTime"`
	ExecutionPath     []string                         `json:"executionPath"`
	Error             string                           `json:"error,omitempty"`
	
	// 变异数据
	MutatedInputData  []byte                           `json:"mutatedInputData,omitempty"`
	StorageChanges    map[gethCommon.Hash]gethCommon.Hash `json:"storageChanges,omitempty"`
	
	// 目标信息
	TargetSlot        *gethCommon.Hash                 `json:"targetSlot,omitempty"`
	TargetArgIndex    *int                             `json:"targetArgIndex,omitempty"`
}
