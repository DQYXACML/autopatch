package tracing

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/ethereum/go-ethereum/triedb/hashdb"
	"github.com/holiman/uint256"
)

// StateManager 管理状态数据库和EVM创建
type StateManager struct {
	jumpTracer *JumpTracer
}

// NewStateManager 创建状态管理器
func NewStateManager(jumpTracer *JumpTracer) *StateManager {
	return &StateManager{
		jumpTracer: jumpTracer,
	}
}

// CreateStateFromPrestate 从预状态创建状态数据库
func (sm *StateManager) CreateStateFromPrestate(prestate PrestateResult) (*state.StateDB, error) {
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

// CreateEVMWithTracer 创建包含tracer的EVM - 修复PUSH0操作码支持
func (sm *StateManager) CreateEVMWithTracer(stateDB *state.StateDB, blockHeader *types.Header, chainID *big.Int) (*vm.EVM, error) {
	// 创建支持所有硬分叉的链配置
	chainConfig := sm.CreateChainConfigWithAllForks(chainID, blockHeader)

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
		Tracer:                  sm.jumpTracer.ToTracingHooks(),
	}

	evm := vm.NewEVM(blockCtx, stateDB, chainConfig, vmConfig)

	// 验证Shanghai是否已激活
	isShanghai := chainConfig.IsShanghai(blockHeader.Number, blockTime)
	fmt.Printf("Shanghai activated: %v (block time: %d, shanghai time: %v)\n",
		isShanghai, blockTime, chainConfig.ShanghaiTime)

	return evm, nil
}

// CreateChainConfigWithAllForks 创建包含所有硬分叉的链配置 - 修复PUSH0支持
func (sm *StateManager) CreateChainConfigWithAllForks(chainID *big.Int, blockHeader *types.Header) *params.ChainConfig {
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
