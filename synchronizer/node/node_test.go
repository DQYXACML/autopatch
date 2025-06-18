// node_test.go
package node

import (
	"context"
	"github.com/ethereum/go-ethereum"
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

/* -------------------------------------------------------------------------- */
/*                                  Mock RPC                                  */
/* -------------------------------------------------------------------------- */

type mockRPC struct{ mock.Mock }

func (m *mockRPC) Close() {}

func (m *mockRPC) CallContext(ctx context.Context, result any, method string, args ...any) error {
	return m.Called(ctx, result, method, args).Error(0)
}

func (m *mockRPC) BatchCallContext(ctx context.Context, b []rpc.BatchElem) error {
	return m.Called(ctx, b).Error(0)
}

/* -------------------------------------------------------------------------- */
/*                               DebugTrace test                              */
/* -------------------------------------------------------------------------- */

func TestDebugTraceTransaction(t *testing.T) {
	mrpc := new(mockRPC)
	cli := &myClient{rpc: mrpc}

	hash := common.HexToHash("0xaaf64b10913ae54c9430cb6c6043acecac6801c52b909291be19f76f35a5e4bc")
	tracerCfg := map[string]interface{}{"tracer": "callTracer"}

	// 第 1 次调：result==nil，只有 txHash
	mrpc.On(
		"CallContext",
		mock.Anything,
		nil,
		"debug_traceTransaction",
		[]interface{}{hash},
	).Return(nil).Once()

	// 第 2 次调：带 tracerCfg，需要把 trace 写进 result
	mrpc.On(
		"CallContext",
		mock.Anything,
		mock.Anything,
		"debug_traceTransaction",
		[]interface{}{hash, tracerCfg},
	).Run(func(args mock.Arguments) {
		if resPtr, ok := args.Get(1).(*interface{}); ok {
			*resPtr = map[string]any{"type": "CALL"}
		}
	}).Return(nil).Once()

	out, err := cli.DebugTraceTransaction(hash)
	assert.NoError(t, err)
	assert.Nil(t, out) // 当前实现固定返回 nil
	mrpc.AssertExpectations(t)
}

/* -------------------------------------------------------------------------- */
/*                          BlockHeadersByRange   test                        */
/* -------------------------------------------------------------------------- */

func TestBlockHeadersByRange(t *testing.T) {
	mrpc := new(mockRPC)
	cli := &myClient{rpc: mrpc}

	start := big.NewInt(10)
	end := big.NewInt(11) // 范围包含 10 和 11，共 2 个

	mrpc.On("BatchCallContext", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			elems := args.Get(1).([]rpc.BatchElem)
			// 伪造返回的 Header
			for i := range elems {
				h := types.Header{Number: big.NewInt(start.Int64() + int64(i))}
				*elems[i].Result.(*types.Header) = h
			}
		}).Return(nil).Once()

	headers, err := cli.BlockHeadersByRange(start, end, 1)
	assert.NoError(t, err)
	assert.Len(t, headers, 2)
	assert.EqualValues(t, 10, headers[0].Number.Int64())
	assert.EqualValues(t, 11, headers[1].Number.Int64())
	mrpc.AssertExpectations(t)
}

/* -------------------------------------------------------------------------- */
/*                               FilterLogs test                              */
/* -------------------------------------------------------------------------- */

func TestFilterLogs(t *testing.T) {
	mrpc := new(mockRPC)
	cli := &myClient{rpc: mrpc}

	addr := common.HexToAddress("0xc0ffee")
	toBlk := big.NewInt(100)

	mrpc.On("BatchCallContext", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			elems := args.Get(1).([]rpc.BatchElem)

			// elems[0] -> eth_getBlockByNumber
			h := types.Header{Number: big.NewInt(100)}
			*elems[0].Result.(*types.Header) = h

			// elems[1] -> eth_getLogs
			logs := []types.Log{
				{Address: addr, BlockNumber: 100},
			}
			*elems[1].Result.(*[]types.Log) = logs
		}).Return(nil).Once()

	query := ethereum.FilterQuery{
		Addresses: []common.Address{addr},
		ToBlock:   toBlk,
	}

	res, err := cli.FilterLogs(query)
	assert.NoError(t, err)
	assert.Equal(t, uint64(100), res.ToBlockHeader.Number.Uint64())
	assert.Len(t, res.Logs, 1)
	assert.Equal(t, addr, res.Logs[0].Address)
	mrpc.AssertExpectations(t)
}

/* -------------------------------------------------------------------------- */
/*                              StorageHash test                              */
/* -------------------------------------------------------------------------- */

func TestStorageHash(t *testing.T) {
	mrpc := new(mockRPC)
	cli := &myClient{rpc: mrpc}

	account := common.HexToAddress("0xdeadbeef")
	block := big.NewInt(123)
	wantHash := common.HexToHash("0xfeedface")

	mrpc.On(
		"CallContext",
		mock.Anything,
		mock.Anything,
		"eth_getProof",
		[]interface{}{account, nil, toBlockNumArg(block)},
	).Run(func(args mock.Arguments) {
		// 通过反射给匿名 struct 写字段
		proofPtr := args.Get(1)
		v := reflect.ValueOf(proofPtr).Elem()
		field := v.FieldByName("StorageHash")
		if field.IsValid() && field.CanSet() {
			field.Set(reflect.ValueOf(wantHash))
		}
	}).Return(nil).Once()

	got, err := cli.StorageHash(account, block)
	assert.NoError(t, err)
	assert.Equal(t, wantHash, got)
	mrpc.AssertExpectations(t)
}
