package tracing

import (
	"context"
	"fmt"
	"github.com/DQYXACML/autopatch/bindings"
	"github.com/DQYXACML/autopatch/synchronizer/node"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/ethdb/leveldb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/triedb"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// MyTracer 通过 RPC 调用 debug_traceTransaction 来追踪交易
type MyTracer struct {
	rpcClient node.EthClient
}

// NewTracer 用 Holesky RPC URL 构造 Tracer
func NewTracer(client node.EthClient) *MyTracer {
	return &MyTracer{
		rpcClient: client,
	}
}

// TraceTransaction 调用 debug_traceTransaction
func (t *MyTracer) TraceTransaction(txHash common.Hash) error {
	// 获取Opcode
	opcodes, err := t.rpcClient.TraceOpcodes(txHash)
	if err != nil {
		log.Error("failed to trace transaction", "txHash", txHash.Hex(), "error", err)
		return err
	}
	for _, v := range opcodes {
		log.Info("For", "opcode", v["op"], "Pc", v["pc"])
	}

	// 获取CallTrace
	path, err := t.rpcClient.TraceCallPath(txHash)
	if err != nil {
		log.Error("failed to trace call path", "txHash", txHash.Hex(), "error", err)
		return err
	}
	log.Info("call path calls: ", "calls", path.Calls)
	return nil
}

// Triple 新的tracer
type Triple struct {
	Contract common.Address
	PC       uint64
	Opcode   string
}

type jumpdestTracer struct {
	Triples []Triple
}

func (j *jumpdestTracer) opcodeHook(pc uint64, op byte, _gas, _cost uint64,
	ctx tracing.OpContext, _rdata []byte, _depth int, _err error) {

	if op == byte(vm.JUMPDEST) {
		j.Triples = append(j.Triples, Triple{
			Contract: ctx.Address(),
			PC:       pc,
			Opcode:   "JUMPDEST",
		})
	}
}

type miniCtx struct{ db ethdb.Database }

func (m miniCtx) Config() *params.ChainConfig {
	// 创建支持所有硬分叉的配置，而不是使用默认的 HoleskyChainConfig
	return createFullySupportedChainConfig(17000) // 17000 是 Holesky 的链ID
}

func (m miniCtx) Engine() consensus.Engine { return nil }

func (m miniCtx) GetHeader(_ context.Context, h common.Hash, n uint64) *types.Header {
	return rawdb.ReadHeader(m.db, h, n)
}

// createFullySupportedChainConfig 创建完全支持所有硬分叉的链配置
func createFullySupportedChainConfig(chainID uint64) *params.ChainConfig {
	var baseConfig *params.ChainConfig
	switch chainID {
	case 1:
		baseConfig = params.MainnetChainConfig
	case 17000:
		baseConfig = params.HoleskyChainConfig
	case 11155111:
		baseConfig = params.SepoliaChainConfig
	default:
		baseConfig = params.HoleskyChainConfig
	}

	// 创建新的配置，确保所有硬分叉都已激活
	config := &params.ChainConfig{
		ChainID:                 big.NewInt(int64(chainID)),
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
		TerminalTotalDifficulty: baseConfig.TerminalTotalDifficulty,
		Ethash:                  baseConfig.Ethash,
		Clique:                  baseConfig.Clique,
	}

	// 设置基于时间的硬分叉
	switch chainID {
	case 1: // 主网
		shanghaiTime := uint64(1681338455) // 主网Shanghai时间
		cancunTime := uint64(1710338135)   // 主网Cancun时间
		config.ShanghaiTime = &shanghaiTime
		config.CancunTime = &cancunTime
	case 17000: // Holesky测试网
		// Holesky在创世时就支持所有硬分叉
		genesisTime := uint64(0)
		config.ShanghaiTime = &genesisTime
		config.CancunTime = &genesisTime
	case 11155111: // Sepolia测试网
		genesisTime := uint64(0)
		config.ShanghaiTime = &genesisTime
		config.CancunTime = &genesisTime
	default:
		// 默认启用所有硬分叉
		genesisTime := uint64(0)
		config.ShanghaiTime = &genesisTime
		config.CancunTime = &genesisTime
	}

	fmt.Printf("Created chain config for chainID %d with Shanghai time: %v (PUSH0 enabled)\n",
		chainID, config.ShanghaiTime)

	return config
}

func die(err error, msg string) {
	if err != nil {
		log.Error("Fatal error", "msg", msg, "error", err)
	}
}

func patchCalldata(raw []byte) ([]byte, error) {
	abiObj, err := bindings.StorageScanMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	method, err := abiObj.MethodById(raw[:4])
	if err != nil {
		return nil, err
	}
	args, err := method.Inputs.Unpack(raw[4:])
	if err != nil {
		return nil, err
	}
	if _, ok := args[0].(uint8); !ok {
		return nil, fmt.Errorf("arg0 is not uint8")
	}
	args[0] = uint8(42) // 示例修改
	packed, err := method.Inputs.Pack(args...)
	if err != nil {
		return nil, err
	}
	return append(method.ID, packed...), nil
}

func RelayTx(tx1 common.Hash) {
	const (
		dataDir = "/home/dqy/go-project/test2/holesky/geth/chaindata"
		rpcURL  = "https://lb.drpc.org/ogrpc?network=holesky&dkey=Avduh2iIjEAksBUYtd4wP1NUPObEnwYR76WEFhW5UfFk"
	)
	txHash := common.HexToHash("0x928e413a5effed2f9389d8c5a12aaf5d63edd0668dab98c67927f51a29205e0a")

	/* 1. 链上数据 */
	client, err := ethclient.Dial(rpcURL)
	die(err, "dial RPC")
	tx, pending, err := client.TransactionByHash(context.Background(), txHash)
	die(err, "get tx")
	if pending {
		log.Error("Tx not packed yet", "txHash", txHash.Hex())
	}
	receipt, err := client.TransactionReceipt(context.Background(), txHash)
	die(err, "get receipt")
	chainID, err := client.NetworkID(context.Background())
	die(err, "network id")
	signer := types.LatestSignerForChainID(chainID)

	/* 2. 修改 calldata */
	newCalldata, err := patchCalldata(tx.Data())
	die(err, "patch calldata")

	/* 3. 本地 chaindata & 状态树 */
	ldb, err := leveldb.New(dataDir, 0, 0, "", false)
	die(err, "open leveldb")
	defer ldb.Close()
	rdb := rawdb.NewDatabase(ldb)

	block := rawdb.ReadBlock(rdb, common.Hash{}, receipt.BlockNumber.Uint64())
	if block == nil {
		log.Error("block not found", "blockNumber", receipt.BlockNumber.Uint64())
	}
	header := block.Header()

	tdb := triedb.NewDatabase(rdb, &triedb.Config{})
	stateDB, err := state.New(header.ParentHash, state.NewDatabase(tdb, nil))
	die(err, "state.New")

	gp := new(core.GasPool)
	gp.AddGas(header.GasLimit)

	// 修复：使用支持所有硬分叉的链配置
	chainConfig := createFullySupportedChainConfig(chainID.Uint64())

	blockCtx := vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		Coinbase:    header.Coinbase,
		BlockNumber: header.Number,
		Time:        header.Time,
		Difficulty:  header.Difficulty,
		GasLimit:    header.GasLimit,
		BaseFee:     header.BaseFee,
	}

	/* 4. 回放前置交易 */
	for _, orig := range block.Transactions() {
		if orig.Hash() == txHash {
			break
		}
		from, err := types.Sender(signer, orig)
		die(err, "sender pre-tx")
		evm := vm.NewEVM(blockCtx, stateDB, chainConfig, vm.Config{})
		evm.SetTxContext(vm.TxContext{Origin: from, GasPrice: orig.GasPrice()})
		var used uint64
		_, err = core.ApplyTransaction(evm, gp, stateDB, header, orig, &used)
		die(err, "apply pre-tx")
	}

	/* 5. 模拟修改后的交易并追踪 JUMPDEST */
	jt := &jumpdestTracer{}
	hooks := &tracing.Hooks{
		OnOpcode: jt.opcodeHook, // 只填我们关心的钩子
	}
	vmConf := vm.Config{Tracer: hooks}

	from, err := types.Sender(signer, tx)
	die(err, "sender new-tx")
	evm := vm.NewEVM(blockCtx, stateDB, chainConfig, vmConf)
	evm.SetTxContext(vm.TxContext{Origin: from, GasPrice: tx.GasPrice()})

	newTx := types.NewTransaction(tx.Nonce(), *tx.To(), tx.Value(),
		tx.Gas(), tx.GasPrice(), newCalldata)

	var used2 uint64
	newRcpt, err := core.ApplyTransaction(evm, gp, stateDB, header, newTx, &used2)
	die(err, "apply new-tx")

	fmt.Printf("Manipulate GasUsed: %d\n", newRcpt.GasUsed)
	for _, t := range jt.Triples {
		fmt.Printf("JUMPDEST @ %s:%d\n", t.Contract.Hex(), t.PC)
	}
}
