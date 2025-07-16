package database

import (
	"context"
	"fmt"
	"github.com/DQYXACML/autopatch/database/worker"
	"github.com/pkg/errors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"os"
	"path/filepath"

	"github.com/DQYXACML/autopatch/config"
	"github.com/DQYXACML/autopatch/database/common"
	_ "github.com/DQYXACML/autopatch/database/utils/serializers"
)

type DB struct {
	gorm *gorm.DB

	Blocks           common.BlocksDB
	Addresses        common.AddressesDB
	Protected        worker.ProtectedAddDB
	AttackTx         worker.AttackTxDB
	ProtectedStorage worker.ProtectedStorageDB
	ProtectedTx      worker.ProtectedTxDB
}

func NewDB(ctx context.Context, dbConfig config.DBConfig) (*DB, error) {
	dsn := fmt.Sprintf("host=%s dbname=%s sslmode=disable", dbConfig.Host, dbConfig.Name)
	if dbConfig.Port != 0 {
		dsn += fmt.Sprintf(" port=%d", dbConfig.Port)
	}
	if dbConfig.User != "" {
		dsn += fmt.Sprintf(" user=%s", dbConfig.User)
	}
	if dbConfig.Password != "" {
		dsn += fmt.Sprintf(" password=%s", dbConfig.Password)
	}

	gormConfig := gorm.Config{
		SkipDefaultTransaction: true,
		CreateBatchSize:        3_000,
	}
	gorm, err := gorm.Open(postgres.Open(dsn), &gormConfig)

	if err != nil {
		return nil, err
	}

	db := &DB{
		gorm:             gorm,
		Blocks:           common.NewBlocksDB(gorm),
		Addresses:        common.NewAddressesDB(gorm),
		AttackTx:         worker.NewAttackTxDB(gorm),
		Protected:        worker.NewProtectedAddDB(gorm),
		ProtectedStorage: worker.NewProtectedStorageDB(gorm),
		ProtectedTx:      worker.NewProtectedTxDB(gorm),
	}
	return db, nil
}

func (db *DB) Transaction(fn func(db *DB) error) error {
	return db.gorm.Transaction(func(tx *gorm.DB) error {
		txDB := &DB{
			gorm:             tx,
			Blocks:           common.NewBlocksDB(tx),
			Addresses:        common.NewAddressesDB(tx),
			AttackTx:         worker.NewAttackTxDB(tx),
			Protected:        worker.NewProtectedAddDB(tx),
			ProtectedStorage: worker.NewProtectedStorageDB(tx),
			ProtectedTx:      worker.NewProtectedTxDB(tx),
		}
		return fn(txDB)
	})
}

func (db *DB) Close() error {
	sql, err := db.gorm.DB()
	if err != nil {
		return err
	}
	return sql.Close()
}

func (db *DB) ExecuteSQLMigration(migrationsFolder string) error {
	err := filepath.Walk(migrationsFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("Failed to process migration file: %s", path))
		}
		if info.IsDir() {
			return nil
		}
		fileContent, readErr := os.ReadFile(path)
		if readErr != nil {
			return errors.Wrap(readErr, fmt.Sprintf("Error reading SQL file: %s", path))
		}

		execErr := db.gorm.Exec(string(fileContent)).Error
		if execErr != nil {
			return errors.Wrap(execErr, fmt.Sprintf("Error executing SQL script: %s", path))
		}
		return nil
	})
	return err
}
