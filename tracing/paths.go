package tracing

import (
	"crypto/sha256"
	"encoding/binary"
)

// StructuredExecutionPath represents an execution path using structured data
type StructuredExecutionPath struct {
	PathID   string     `json:"pathId"`
	Nodes    []PathNode `json:"nodes"`
	PathHash [32]byte   `json:"pathHash"` // Hash for quick comparison
	Success  bool       `json:"success"`
	GasUsed  uint64     `json:"gasUsed"`
}

// ComputePathHash computes a hash of the execution path for comparison
func (p *StructuredExecutionPath) ComputePathHash() [32]byte {
	hasher := sha256.New()

	for _, node := range p.Nodes {
		// Write node type
		hasher.Write([]byte{byte(node.NodeType)})

		// Write PC
		pcBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(pcBytes, node.PC)
		hasher.Write(pcBytes)

		switch node.NodeType {
		case NodeTypeCall:
			if node.CallNode != nil {
				hasher.Write(node.CallNode.Selector[:])
				hasher.Write(node.CallNode.ContractAddr[:])
				hasher.Write(node.CallNode.FromAddr[:])
				for _, param := range node.CallNode.Parameters {
					hasher.Write(param)
				}
			}
		case NodeTypeStorage:
			if node.StorageNode != nil {
				hasher.Write(node.StorageNode.ContractAddr[:])
				hasher.Write(node.StorageNode.Slot[:])
				hasher.Write(node.StorageNode.NewValue[:])
				hasher.Write([]byte{byte(node.StorageNode.OpType)})
			}
		case NodeTypeStack:
			if node.StackNode != nil {
				for _, value := range node.StackNode.Values {
					hasher.Write(value[:])
				}
				hasher.Write([]byte(node.StackNode.OpCode))
			}
		case NodeTypeJump:
			if node.JumpNode != nil {
				hasher.Write(node.JumpNode.Destination[:])
				hasher.Write(node.JumpNode.Condition[:])
				if node.JumpNode.Taken {
					hasher.Write([]byte{1})
				} else {
					hasher.Write([]byte{0})
				}
			}
		}
	}

	return sha256.Sum256(hasher.Sum(nil))
}

// IsEqual compares two structured execution paths
func (p *StructuredExecutionPath) IsEqual(other *StructuredExecutionPath) bool {
	if other == nil {
		return false
	}
	return p.PathHash == other.PathHash
}

// IsSimilar compares paths with tolerance for minor differences
func (p *StructuredExecutionPath) IsSimilar(other *StructuredExecutionPath, tolerance float64) bool {
	if other == nil {
		return false
	}

	if len(p.Nodes) == 0 && len(other.Nodes) == 0 {
		return true
	}

	minLen := len(p.Nodes)
	if len(other.Nodes) < minLen {
		minLen = len(other.Nodes)
	}

	if minLen == 0 {
		return false
	}

	matches := 0
	for i := 0; i < minLen; i++ {
		if p.compareNodes(&p.Nodes[i], &other.Nodes[i]) {
			matches++
		}
	}

	similarity := float64(matches) / float64(len(p.Nodes))
	return similarity >= tolerance
}

// compareNodes compares two path nodes
func (p *StructuredExecutionPath) compareNodes(n1, n2 *PathNode) bool {
	if n1.NodeType != n2.NodeType {
		return false
	}

	switch n1.NodeType {
	case NodeTypeCall:
		if n1.CallNode == nil || n2.CallNode == nil {
			return n1.CallNode == n2.CallNode
		}
		return n1.CallNode.Selector == n2.CallNode.Selector &&
			n1.CallNode.ContractAddr == n2.CallNode.ContractAddr
	case NodeTypeStorage:
		if n1.StorageNode == nil || n2.StorageNode == nil {
			return n1.StorageNode == n2.StorageNode
		}
		return n1.StorageNode.ContractAddr == n2.StorageNode.ContractAddr &&
			n1.StorageNode.Slot == n2.StorageNode.Slot &&
			n1.StorageNode.OpType == n2.StorageNode.OpType
	case NodeTypeJump:
		if n1.JumpNode == nil || n2.JumpNode == nil {
			return n1.JumpNode == n2.JumpNode
		}
		return n1.JumpNode.Taken == n2.JumpNode.Taken
	}

	return true
}
