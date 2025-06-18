package worker

import (
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"math/big"
	"strings"
)

type ProtectedTx struct {
	GUID             uuid.UUID      `gorm:"primaryKey" json:"guid"`
	BlockHash        common.Hash    `gorm:"column:block_hash;serializer:bytes"  db:"block_hash" json:"block_hash"`
	BlockNumber      *big.Int       `gorm:"serializer:u256;column:block_number" db:"block_number" json:"block_number" form:"block_number"`
	Hash             common.Hash    `gorm:"column:hash;serializer:bytes"  db:"hash" json:"hash"`
	ProtectedAddress common.Address `gorm:"serializer:bytes" json:"protected_address"`
	InputData        []byte         `gorm:"serializer:bytes;column:input_data" db:"input_data" json:"input_data"`
}

type ProtectedTxVIew interface {
	QueryProtectedTxWithHeaderAndAddress(address common.Address, number *big.Int) ([]ProtectedTx, error)
}

type ProtectedTxDB interface {
	ProtectedTxVIew

	StoreProtectedTx([]ProtectedTx, uint64) error
}

type protectedTxDB struct {
	gorm *gorm.DB
}

func (p *protectedTxDB) QueryProtectedTxWithHeaderAndAddress(address common.Address, number *big.Int) ([]ProtectedTx, error) {
	var tx []ProtectedTx
	err := p.gorm.Table("protected_txs").
		Where("protected_address = ? AND block_number = ?", strings.ToLower(address.Hex()), number.String()).
		Find(&tx).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // No records found
		}
		return nil, err // Return the error if something else went wrong
	}

	return tx, nil
}

func (p *protectedTxDB) StoreProtectedTx(txList []ProtectedTx, txLength uint64) error {
	result := p.gorm.Table("protected_txs").CreateInBatches(&txList, int(txLength))
	return result.Error
}

func NewProtectedTxDB(db *gorm.DB) ProtectedTxDB {
	return &protectedTxDB{
		gorm: db,
	}
}
