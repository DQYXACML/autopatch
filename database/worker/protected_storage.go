package worker

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"math/big"
	"strings"
)

type ProtectedStorage struct {
	GUID             uuid.UUID      `gorm:"primaryKey" json:"guid"`
	ProtectedAddress common.Address `gorm:"serializer:bytes" json:"protected_address"`
	StorageKey       string         `gorm:"type:varchar(255)" json:"storage_key"`
	StorageValue     string         `gorm:"type:varchar(255)" json:"storage_value"`
	Number           *big.Int       `gorm:"serializer:u256" json:"number"`
}

type ProtectedStorageView interface {
	QueryProtectedStorage(common.Address) ([]ProtectedStorage, error)
	QueryProtectedStorageWithHeader(address common.Address, header *big.Int) ([]ProtectedStorage, error)
}

type ProtectedStorageDB interface {
	ProtectedStorageView

	StoreProtectedStorage([]ProtectedStorage) error
}

type protectedStorageDB struct {
	gorm *gorm.DB
}

func (p *protectedStorageDB) QueryProtectedStorageWithHeader(address common.Address, header *big.Int) ([]ProtectedStorage, error) {
	log.Info("Querying protected storage with header for address", "address", strings.ToLower(address.Hex()), "number", header)
	var protectedStorages []ProtectedStorage
	err := p.gorm.Table("protected_storage").Where("protected_address = ? AND number = ?", strings.ToLower(address.Hex()), header.String()).Find(&protectedStorages).Error
	if err != nil {
		return nil, fmt.Errorf("query protected storage with header failed: %w", err)
	}

	return protectedStorages, nil
}

func (p *protectedStorageDB) QueryProtectedStorage(targetAddress common.Address) ([]ProtectedStorage, error) {
	log.Info("Querying protected storage for address", "address", strings.ToLower(targetAddress.Hex()))
	var protectedStorages []ProtectedStorage
	err := p.gorm.Table("protected_storage").Where("protected_address = ?", strings.ToLower(targetAddress.Hex())).Find(&protectedStorages).Error
	if err != nil {
		return nil, fmt.Errorf("query protected storage failed: %w", err)
	}

	return protectedStorages, nil
}

func (p *protectedStorageDB) StoreProtectedStorage(storages []ProtectedStorage) error {
	result := p.gorm.Table("protected_storage").CreateInBatches(&storages, len(storages))
	return result.Error
}

func NewProtectedStorageDB(db *gorm.DB) ProtectedStorageDB {
	return &protectedStorageDB{
		gorm: db,
	}
}
