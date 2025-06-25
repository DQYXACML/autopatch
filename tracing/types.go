package tracing

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"math/big"
)

// Account represents an Ethereum account state
type Account struct {
	Balance *hexutil.Big                `json:"balance,omitempty"`
	Code    hexutil.Bytes               `json:"code,omitempty"`
	Nonce   uint64                      `json:"nonce,omitempty"`
	Storage map[common.Hash]common.Hash `json:"storage,omitempty"`
}

// PrestateResult represents the result from prestateTracer
type PrestateResult map[common.Address]*Account

// StorageChange represents a single storage slot change
type StorageChange struct {
	Slot     common.Hash `json:"slot"`
	OldValue common.Hash `json:"oldValue"`
	NewValue common.Hash `json:"newValue"`
	Changed  bool        `json:"changed"`
}

// ContractStorageChanges represents all storage changes for a contract
type ContractStorageChanges struct {
	Address        common.Address  `json:"address"`
	Changes        []StorageChange `json:"changes"`
	NewSlots       []StorageChange `json:"newSlots"`
	ModifiedSlots  []StorageChange `json:"modifiedSlots"`
	UnchangedSlots []StorageChange `json:"unchangedSlots"`
}

// FunctionCall represents a function call in the execution trace
type FunctionCall struct {
	Contract      common.Address  `json:"contract"`
	FunctionName  string          `json:"functionName"`
	FunctionSig   string          `json:"functionSig"`
	Selector      string          `json:"selector"`
	InputData     string          `json:"inputData"`
	DecodedInputs []interface{}   `json:"decodedInputs,omitempty"`
	Depth         int             `json:"depth"`
	CallType      string          `json:"callType"` // CALL, DELEGATECALL, STATICCALL, CREATE, CREATE2
	From          common.Address  `json:"from"`
	To            common.Address  `json:"to"`
	Value         *big.Int        `json:"value"`
	Gas           uint64          `json:"gas"`
	GasUsed       uint64          `json:"gasUsed"`
	Success       bool            `json:"success"`
	Error         string          `json:"error,omitempty"`
	ReturnData    string          `json:"returnData,omitempty"`
	Children      []*FunctionCall `json:"children,omitempty"`
}

// PathConstraint represents execution path constraints
type PathConstraint struct {
	Type        string          `json:"type"`              // "storage", "input", "stack", "memory", "jump"
	Description string          `json:"description"`       // Human readable description
	Address     *common.Address `json:"address,omitempty"` // Contract address for storage/call constraints
	Slot        *common.Hash    `json:"slot,omitempty"`    // Storage slot
	Value       interface{}     `json:"value"`             // The actual value
	PC          uint64          `json:"pc"`                // Program counter
	OpCode      string          `json:"opCode"`            // The opcode that generated this constraint
	StackDepth  int             `json:"stackDepth,omitempty"`
}

// ExecutionPath represents a complete execution path (legacy, keep for compatibility)
type ExecutionPath struct {
	PathID        string                       `json:"pathId"`
	CallStack     []*FunctionCall              `json:"callStack"`
	Constraints   []PathConstraint             `json:"constraints"`
	StartPC       uint64                       `json:"startPc"`
	EndPC         uint64                       `json:"endPc"`
	Success       bool                         `json:"success"`
	GasUsed       uint64                       `json:"gasUsed"`
	StorageReads  map[string]map[string]string `json:"storageReads"`  // contract -> slot -> value
	StorageWrites map[string]map[string]string `json:"storageWrites"` // contract -> slot -> value
}

// === NEW STRUCTURED PATH REPRESENTATION ===

// PathNodeType represents the type of path node
type PathNodeType int

const (
	NodeTypeCall PathNodeType = iota
	NodeTypeStorage
	NodeTypeStack
	NodeTypeJump
)

// StorageOpType represents the type of storage operation
type StorageOpType int

const (
	StorageRead StorageOpType = iota
	StorageWrite
)

// CallNode represents a function call in the execution path
type CallNode struct {
	Selector     [4]byte        `json:"selector"`
	Parameters   [][]byte       `json:"parameters"` // Raw parameter data
	ContractAddr common.Address `json:"contractAddr"`
	FromAddr     common.Address `json:"fromAddr"`
	Depth        int            `json:"depth"`
	Gas          uint64         `json:"gas"`
	Value        *big.Int       `json:"value"`
	CallType     string         `json:"callType"`
}

// StorageNode represents a storage operation
type StorageNode struct {
	ContractAddr common.Address `json:"contractAddr"`
	Slot         common.Hash    `json:"slot"`
	OldValue     common.Hash    `json:"oldValue"`
	NewValue     common.Hash    `json:"newValue"`
	OpType       StorageOpType  `json:"opType"`
}

// StackNode represents stack state at a specific point
type StackNode struct {
	Values []common.Hash `json:"values"`
	Depth  int           `json:"depth"`
	OpCode string        `json:"opCode"`
}

// JumpNode represents a conditional jump
type JumpNode struct {
	Destination common.Hash `json:"destination"`
	Condition   common.Hash `json:"condition"`
	Taken       bool        `json:"taken"`
}

// PathNode represents a single node in the execution path
type PathNode struct {
	NodeType    PathNodeType `json:"nodeType"`
	CallNode    *CallNode    `json:"callNode,omitempty"`
	StorageNode *StorageNode `json:"storageNode,omitempty"`
	StackNode   *StackNode   `json:"stackNode,omitempty"`
	JumpNode    *JumpNode    `json:"jumpNode,omitempty"`
	PC          uint64       `json:"pc"`
	GasUsed     uint64       `json:"gasUsed"`
}

// ParameterModification represents a parameter modification rule
type ParameterModification struct {
	ParameterIndex int         // 参数索引 (从0开始)
	ParameterName  string      // 参数名称 (可选，用于验证)
	NewValue       interface{} // 新的参数值
}

// FunctionModification represents modifications for a specific function
type FunctionModification struct {
	FunctionName      string                  // 函数名称
	FunctionSignature string                  // 函数签名，如 "transfer(address,uint256)"
	NewFunctionName   string                  // 新函数名称 (如果要修改函数选择器)
	ParameterMods     []ParameterModification // 参数修改列表
}
