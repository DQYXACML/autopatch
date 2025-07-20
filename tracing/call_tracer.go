package tracing

import (
	"github.com/DQYXACML/autopatch/synchronizer/node"
)

// CallTracer 负责调用跟踪和分析
type CallTracer struct {
	nodeClient node.EthClient
}

// NewCallTracer 创建调用跟踪器
func NewCallTracer(nodeClient node.EthClient) *CallTracer {
	return &CallTracer{
		nodeClient: nodeClient,
	}
}