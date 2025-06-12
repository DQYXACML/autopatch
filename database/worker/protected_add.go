package worker

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProtectedAdd struct {
	GUID             uuid.UUID      `gorm:"primaryKey" json:"guid"`
	ProtectedAddress common.Address `gorm:"serializer:bytes" json:"protected_address"`
	ContractName     string         `gorm:"type:varchar(255)" json:"contract_name"`
}

type ProtectedAddView interface {
	QueryProtectedAddAddressList() ([]common.Address, error)
}

type ProtectedAddDB interface {
	ProtectedAddView

	StoreProtectedAdd([]ProtectedAdd) error
}

type protectedAddDB struct {
	gorm *gorm.DB
}

func (p *protectedAddDB) QueryProtectedAddAddressList() ([]common.Address, error) {
	var protectedAdds []ProtectedAdd
	err := p.gorm.Table("protected_created").Find(&protectedAdds).Error
	if err != nil {
		return nil, fmt.Errorf("query proxy created failed: %w", err)
	}

	var addressList []common.Address
	for _, protectedAdd := range protectedAdds {
		addressList = append(addressList, protectedAdd.ProtectedAddress)
	}
	return addressList, nil
}

func (p *protectedAddDB) StoreProtectedAdd(adds []ProtectedAdd) error {
	result := p.gorm.Table("protected_created").CreateInBatches(&adds, len(adds))
	return result.Error
}

func NewProtectedAddDB(db *gorm.DB) ProtectedAddDB {
	return &protectedAddDB{
		gorm: db,
	}
}
