package node

import (
	"context"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/signer/storage"
	"math/big"
	"time"
)

const (
	defaultDialTimeout = 5 * time.Second

	defaultRequestTimeout = 100 * time.Second
)

type myClient struct {
	rpc RPC
}

func (m *myClient) SendRawTransaction(rawTx string) error {
	ctxwt, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()
	if err := m.rpc.CallContext(ctxwt, nil, "eth_sendRawTransaction", rawTx); err != nil {
		return err
	}
	log.Info("send tx to ethereum success")
	return nil
}

func (m *myClient) TraceCallPath(hash common.Hash) (*NodecallFrame, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	var root NodecallFrame
	cfg := map[string]any{"tracer": "callTracer"}
	if err := m.rpc.CallContext(ctx, &root, "debug_traceTransaction", hash, cfg); err != nil {
		return nil, err
	}
	return &root, nil
}

type structLog struct {
	Pc      uint64 `json:"pc"`
	Op      string `json:"op"`
	Gas     uint64 `json:"gas"`
	GasCost uint64 `json:"gasCost"`
	Depth   int    `json:"depth"`
}

type vmTraceResult struct {
	StructLogs []structLog `json:"structLogs"`
}

func (m *myClient) TraceOpcodes(hash common.Hash) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	var res vmTraceResult
	// 不指定 tracer，Geth 默认返回 structLogs
	if err := m.rpc.CallContext(ctx, &res, "debug_traceTransaction", hash); err != nil {
		return nil, err
	}

	ops := make([]map[string]interface{}, len(res.StructLogs))
	for i, slog := range res.StructLogs {
		ops[i] = make(map[string]interface{})
		ops[i]["op"] = slog.Op
		ops[i]["pc"] = slog.Pc
	}
	return ops, nil
}

func (m *myClient) TransactionsToAtBlock(addr common.Address, blockNumber *big.Int) ([]*types.Transaction, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()
	var block *types.Block
	if err := m.rpc.CallContext(
		ctx,
		&block,
		"eth_getBlockByNumber",
		toBlockNumArg(blockNumber), // 参数 1：区块号
		true,                       // 参数 2：返回完整交易对象
	); err != nil {
		return nil, err
	}
	if block == nil {
		return nil, fmt.Errorf("block %s not found", blockNumber.String())
	}

	/* -------- 2. 遍历过滤 To 地址 -------- */
	var hits []*types.Transaction
	for _, tx := range block.Transactions() {
		if to := tx.To(); to != nil && *to == addr {
			hits = append(hits, tx)
		}
	}

	return hits, nil
}

func (m *myClient) TxReceiptByHash(hash common.Hash) (*types.Receipt, error) {
	ctxwt, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	var txReceipt *types.Receipt
	err := m.rpc.CallContext(ctxwt, &txReceipt, "eth_getTransactionReceipt", hash)
	if err != nil {
		return nil, err
	} else if txReceipt == nil {
		return nil, ethereum.NotFound
	}

	return txReceipt, nil
}

func (m *myClient) TxCountByAddress(address common.Address) (hexutil.Uint64, error) {
	ctxwt, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()
	var nonce hexutil.Uint64
	err := m.rpc.CallContext(ctxwt, &nonce, "eth_getTransactionCount", address, "latest")
	if err != nil {
		log.Error("Call eth_getTransactionCount method fail", "err", err)
		return 0, err
	}
	log.Info("get nonce by address success", "nonce", nonce)
	return nonce, err
}

func (m *myClient) DebugTraceTransaction(hash common.Hash) ([]byte, error) {
	log.Info("DebugTraceTransaction called", "hash: ", hash.Hex())
	ctxwt, _ := context.WithTimeout(context.Background(), defaultRequestTimeout)
	err := m.rpc.CallContext(ctxwt, nil, "debug_traceTransaction", hash)
	if err != nil {
		return nil, err
	}
	var traceResult interface{}
	tracerCfg := map[string]interface{}{
		"tracer": "callTracer",
	}
	err = m.rpc.CallContext(
		ctxwt,
		&traceResult,
		"debug_traceTransaction",
		hash,
		tracerCfg,
	)
	if err != nil {
		return nil, err
	}

	//log.Info("traceResult", "result: ", traceResult)
	fmt.Println("traceResult: ", traceResult)

	return nil, nil
}

func (m *myClient) DebugTraceAll(address common.Address) ([]byte, error) {
	log.Info("DebugTraceAll called", "address: ", address.Hex())
	return nil, fmt.Errorf("DebugTraceAll is not implemented")
}

func (m *myClient) GetStorageAt(hash common.Hash) (storage.Storage, error) {
	log.Info("GetStorageAt called", "hash: ", hash.Hex())
	return nil, fmt.Errorf("GetStorageAt is not implemented")
}

func (m *myClient) BlockHeadersByRange(startHeight *big.Int, engHeight *big.Int, chainId uint) ([]types.Header, error) {
	if startHeight.Cmp(engHeight) == 0 {
		header, err := m.BlockHeaderByNumber(startHeight)
		if err != nil {
			return nil, err
		}
		return []types.Header{*header}, nil
	}

	count := new(big.Int).Sub(engHeight, startHeight).Uint64() + 1
	headers := make([]types.Header, count)
	batchElems := make([]rpc.BatchElem, count)

	for i := uint64(0); i < count; i++ {
		height := new(big.Int).Add(startHeight, new(big.Int).SetUint64(i))
		batchElems[i] = rpc.BatchElem{
			Method: "eth_getBlockByNumber",
			Args:   []interface{}{toBlockNumArg(height), false},
			Result: &headers[i],
		}
	}

	ctxwt, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	err := m.rpc.BatchCallContext(ctxwt, batchElems)
	if err != nil {
		return nil, err
	}

	size := 0
	for i, batchElem := range batchElems {
		header, ok := batchElem.Result.(*types.Header)
		if !ok {
			return nil, fmt.Errorf("unable to transform rpc response %v into utils.Header", batchElem.Result)
		}
		headers[i] = *header

		size = size + 1
	}
	headers = headers[:size]

	return headers, nil
}

func (m *myClient) BlockHeaderByNumber(b *big.Int) (*types.Header, error) {
	ctxwt, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	var header *types.Header
	err := m.rpc.CallContext(ctxwt, &header, "eth_getBlockByNumber", toBlockNumArg(b), false)
	if err != nil {
		log.Error("Call eth_getBlockByNumber method fail", "err", err)
		return nil, err
	} else if header == nil {
		log.Error("header not found")
		return nil, ethereum.NotFound
	}
	return header, nil
}

func (m *myClient) LatestSafeBlockHeader() (*types.Header, error) {
	ctxwt, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	var header *types.Header
	err := m.rpc.CallContext(ctxwt, &header, "eth_getBlockByNumber", "safe", false)
	if err != nil {
		return nil, err
	} else if header == nil {
		return nil, ethereum.NotFound
	}

	return header, nil
}

func (m *myClient) LatestFinalizedBlockHeader() (*types.Header, error) {
	ctxwt, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	var header *types.Header
	err := m.rpc.CallContext(ctxwt, &header, "eth_getBlockByNumber", "finalized", false)
	if err != nil {
		return nil, err
	} else if header == nil {
		return nil, ethereum.NotFound
	}

	return header, nil
}

func (m *myClient) BlockHeaderByHash(hash common.Hash) (*types.Header, error) {
	ctxwt, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	var header *types.Header
	err := m.rpc.CallContext(ctxwt, &header, "eth_getBlockByHash", hash, false)
	if err != nil {
		return nil, err
	} else if header == nil {
		return nil, ethereum.NotFound
	}

	if header.Hash() != hash {
		return nil, errors.New("header mismatch")
	}

	return header, nil
}

func (m *myClient) TxByHash(hash common.Hash) (*types.Transaction, error) {
	ctxwt, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	var tx *types.Transaction
	err := m.rpc.CallContext(ctxwt, &tx, "eth_getTransactionByHash", hash)
	if err != nil {
		return nil, err
	} else if tx == nil {
		return nil, ethereum.NotFound
	}

	log.Info("Transaction Data is: ", tx.Data())

	return tx, nil
}

func (m *myClient) StorageHash(address common.Address, b *big.Int) (common.Hash, error) {
	ctxwt, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	proof := struct{ StorageHash common.Hash }{}
	err := m.rpc.CallContext(ctxwt, &proof, "eth_getProof", address, nil, toBlockNumArg(b))
	if err != nil {
		return common.Hash{}, err
	}

	return proof.StorageHash, nil
}

func (m *myClient) FilterLogs(query ethereum.FilterQuery) (Logs, error) {
	args, err := toFilterLog(query)
	if err != nil {
		return Logs{}, err
	}
	var header types.Header
	var logs []types.Log

	batchElems := make([]rpc.BatchElem, 2)

	batchElems[0] = rpc.BatchElem{
		Method: "eth_getBlockByNumber",
		Args:   []interface{}{toBlockNumArg(query.ToBlock), false},
		Result: &header,
	}
	batchElems[1] = rpc.BatchElem{
		Method: "eth_getLogs",
		Args:   []interface{}{args},
		Result: &logs,
	}
	ctxwt, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	err = m.rpc.BatchCallContext(ctxwt, batchElems)
	if err != nil {
		return Logs{}, err
	}
	if batchElems[0].Error != nil {
		return Logs{}, fmt.Errorf("unable to query for the `FilterQuery#ToBlock` header: %w", batchElems[0].Error)
	}
	if batchElems[1].Error != nil {
		return Logs{}, fmt.Errorf("unable to query logs: %w", batchElems[1].Error)
	}

	return Logs{Logs: logs, ToBlockHeader: &header}, nil
}

func (m *myClient) Close() {
	m.rpc.Close()
}

type RPC interface {
	Close()
	CallContext(ctx context.Context, result any, method string, args ...any) error
	BatchCallContext(ctx context.Context, b []rpc.BatchElem) error
}

type Logs struct {
	Logs          []types.Log
	ToBlockHeader *types.Header
}

type NodecallFrame struct {
	Type    string          `json:"type"`
	From    string          `json:"from"`
	To      string          `json:"to"`
	Input   string          `json:"input"`
	Calls   []NodecallFrame `json:"calls,omitempty"`
	Gas     string          `json:"gas"`
	GasUsed string          `json:"gas_used"`
	Value   string          `json:"value"`
}

type EthClient interface {
	BlockHeaderByNumber(*big.Int) (*types.Header, error)
	LatestSafeBlockHeader() (*types.Header, error)
	LatestFinalizedBlockHeader() (*types.Header, error)
	BlockHeaderByHash(hash common.Hash) (*types.Header, error)
	BlockHeadersByRange(*big.Int, *big.Int, uint) ([]types.Header, error)

	TxByHash(hash common.Hash) (*types.Transaction, error)
	TxReceiptByHash(common.Hash) (*types.Receipt, error)
	TransactionsToAtBlock(addr common.Address, blockNumber *big.Int) ([]*types.Transaction, error)

	StorageHash(common.Address, *big.Int) (common.Hash, error)
	FilterLogs(query ethereum.FilterQuery) (Logs, error)

	TxCountByAddress(common.Address) (hexutil.Uint64, error)
	SendRawTransaction(rawTx string) error

	DebugTraceAll(common.Address) ([]byte, error)
	DebugTraceTransaction(hash common.Hash) ([]byte, error)

	TraceCallPath(hash common.Hash) (*NodecallFrame, error)
	TraceOpcodes(hash common.Hash) ([]map[string]interface{}, error)

	GetStorageAt(common.Hash) (storage.Storage, error)

	Close()
}

func DialEthClient(ctx context.Context, rpcUrl string) (EthClient, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultDialTimeout)
	defer cancel()

	rpcClient, err := rpc.DialContext(ctx, rpcUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to dial address (%s): %w", rpcUrl, err)
	}

	return &myClient{
		rpc: NewRPC(rpcClient),
	}, nil
}

type rpcClient struct {
	rpc *rpc.Client
}

func NewRPC(client *rpc.Client) RPC {
	return &rpcClient{client}
}

func (c *rpcClient) Close() {
	c.rpc.Close()
}

func (c *rpcClient) CallContext(ctx context.Context, result any, method string, args ...any) error {
	err := c.rpc.CallContext(ctx, result, method, args...)
	return err
}

func (c *rpcClient) BatchCallContext(ctx context.Context, b []rpc.BatchElem) error {
	err := c.rpc.BatchCallContext(ctx, b)
	return err
}

func toBlockNumArg(b *big.Int) string {
	if b == nil {
		return "latest"
	}
	if b.Sign() >= 0 {
		return hexutil.EncodeBig(b)
	}
	return rpc.BlockNumber(b.Int64()).String()
}

func toFilterLog(q ethereum.FilterQuery) (interface{}, error) {
	arg := map[string]interface{}{"address": q.Addresses, "topics": q.Topics}
	if q.BlockHash != nil {
		arg["blockHash"] = *q.BlockHash
		if q.FromBlock != nil || q.ToBlock != nil {
			return nil, errors.New("cannot specify both BlockHash and FromBlock/ToBlock")
		}
	} else {
		if q.FromBlock != nil {
			arg["fromBlock"] = toBlockNumArg(q.FromBlock)
		} else {
			arg["fromBlock"] = "0x0"
		}
		if q.ToBlock != nil {
			arg["toBlock"] = toBlockNumArg(q.ToBlock)
		}
	}
	return arg, nil
}
