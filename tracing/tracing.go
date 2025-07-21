package tracing

import (
	"github.com/DQYXACML/autopatch/synchronizer/node"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
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
