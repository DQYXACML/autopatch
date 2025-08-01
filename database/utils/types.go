package utils

import (
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

type RLPHeader types.Header

func (h *RLPHeader) EncodeRLP(w io.Writer) error {
	return types.NewBlockWithHeader((*types.Header)(h)).EncodeRLP(w)
}

func (h *RLPHeader) DecodeRLP(s *rlp.Stream) error {
	block := new(types.Block)
	err := block.DecodeRLP(s)
	if err != nil {
		return err
	}

	header := block.Header()
	*h = (RLPHeader)(*header)
	return nil
}

func (h *RLPHeader) Header() *types.Header {
	return (*types.Header)(h)
}

func (h *RLPHeader) Hash() common.Hash {
	return h.Header().Hash()
}

type Bytes []byte

func (b Bytes) Bytes() []byte {
	return b[:]
}
func (b *Bytes) SetBytes(bytes []byte) {
	*b = bytes
}

// ContractState 表示合约在特定时刻的状态
type ContractState struct {
	Address common.Address                 `json:"address"`
	Storage map[common.Hash]common.Hash    `json:"storage"`
	Code    []byte                         `json:"code"`
	Balance *big.Int                       `json:"balance"`
	Nonce   uint64                         `json:"nonce"`
}
