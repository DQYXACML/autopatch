package worker

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"math/big"
	"time"
)

// AttackTxStatus 定义攻击交易状态
const (
	StatusPending    = 0 // 待处理
	StatusProcessing = 1 // 处理中
	StatusSuccess    = 2 // 处理成功
	StatusFailed     = 3 // 处理失败
	StatusSkipped    = 4 // 跳过
)

// AttackTx 攻击交易结构体 - 基于新的设计模式
type AttackTx struct {
	GUID            uuid.UUID      `gorm:"primaryKey" json:"guid"`
	TxHash          common.Hash    `gorm:"serializer:bytes" json:"tx_hash"`
	BlockNumber     *big.Int       `gorm:"serializer:u256" json:"block_number" `
	BlockHash       common.Hash    `gorm:"serializer:bytes" json:"block_hash"`
	ContractAddress common.Address `gorm:"serializer:bytes;index" json:"contract_address"`
	FromAddress     common.Address `gorm:"serializer:bytes"  json:"from_address"`
	ToAddress       common.Address `gorm:"serializer:bytes"  json:"to_address"`
	Value           *big.Int       `gorm:"serializer:u256"  json:"value" `
	GasUsed         *big.Int       `gorm:"serializer:u256"  json:"gas_used"`
	GasPrice        *big.Int       `gorm:"serializer:u256;column:gas_price" json:"gas_price"`
	Status          uint8          `gorm:"default:0;index" json:"status"`
	AttackType      string         `json:"attack_type"`
	ErrorMessage    string         `json:"error_message"`
	Timestamp       uint64         `json:"timestamp"`
	CreatedAt       time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (AttackTx) TableName() string {
	return "attack_tx"
}

// AttackTxQueryParams 查询参数
type AttackTxQueryParams struct {
	Status       *uint8
	AttackType   *string
	ContractAddr *common.Address
	FromAddr     *common.Address
	ToAddr       *common.Address
	BlockNumber  *big.Int
	MinValue     *big.Int
	MaxValue     *big.Int
	Limit        int
	Offset       int
	OrderBy      string
}

// AttackTxView 查询接口
type AttackTxView interface {
	// 基本查询
	QueryAttackTx(params AttackTxQueryParams) ([]AttackTx, error)
	QueryAttackTxByHash(txHash common.Hash) (*AttackTx, error)
	QueryAttackTxByGUID(guid uuid.UUID) (*AttackTx, error)
	QueryAttackTxByStatus(status uint8) ([]AttackTx, error)
	QueryAttackTxByContract(contractAddr common.Address) ([]AttackTx, error)
	QueryAttackTxByBlockRange(startBlock, endBlock *big.Int) ([]AttackTx, error)
	CountAttackTx(params AttackTxQueryParams) (int64, error)

	// 辅助查询方法
	GetPendingAttackTx(limit int) ([]AttackTx, error)
	GetAttackTxByStatusAndType(status uint8, attackType string, limit int) ([]AttackTx, error)
	GetAttackTxByValueRange(minValue, maxValue *big.Int, limit int) ([]AttackTx, error)

	// 统计方法
	GetStatusStatistics() (map[uint8]int64, error)
	GetAttackTypeStatistics() (map[string]int64, error)
}

// AttackTxModifier 修改接口
type AttackTxModifier interface {
	// 基本操作
	StoreAttackTx(txs []AttackTx) error
	UpdateAttackTxStatus(guid uuid.UUID, status uint8, errorMsg string) error
	UpdateAttackTxByHash(txHash common.Hash, updates map[string]interface{}) error
	DeleteAttackTx(guid uuid.UUID) error
	DeleteAttackTxByHash(txHash common.Hash) error

	// 批量操作
	BatchUpdateStatus(guids []uuid.UUID, status uint8) error
	BatchUpdateByHashes(txHashes []common.Hash, status uint8) error

	// 状态标记方法 - 基于GUID
	MarkAsProcessing(guid uuid.UUID) error
	MarkAsSuccess(guid uuid.UUID) error
	MarkAsFailed(guid uuid.UUID, errorMsg string) error
	MarkAsSkipped(guid uuid.UUID, reason string) error

	// 状态标记方法 - 基于Hash
	MarkAsProcessingByHash(txHash common.Hash) error
	MarkAsSuccessByHash(txHash common.Hash) error
	MarkAsFailedByHash(txHash common.Hash, errorMsg string) error

	// 维护方法
	CleanupOldRecords(olderThan time.Time) (int64, error)
}

// AttackTxDB 完整的数据库操作接口
type AttackTxDB interface {
	AttackTxView
	AttackTxModifier
}

// attackTxDB 实现
type attackTxDB struct {
	db *gorm.DB
}

// ===== 基本查询方法 =====

// QueryAttackTx 根据参数查询攻击交易
func (a *attackTxDB) QueryAttackTx(params AttackTxQueryParams) ([]AttackTx, error) {
	var txs []AttackTx
	query := a.db.Model(&AttackTx{})

	// 构建查询条件
	if params.Status != nil {
		query = query.Where("status = ?", *params.Status)
	}
	if params.AttackType != nil {
		query = query.Where("attack_type = ?", *params.AttackType)
	}
	if params.ContractAddr != nil {
		query = query.Where("contract_address = ?", params.ContractAddr.Bytes())
	}
	if params.FromAddr != nil {
		query = query.Where("from_address = ?", params.FromAddr.Bytes())
	}
	if params.ToAddr != nil {
		query = query.Where("to_address = ?", params.ToAddr.Bytes())
	}
	if params.BlockNumber != nil {
		query = query.Where("block_number = ?", params.BlockNumber.Bytes())
	}
	if params.MinValue != nil {
		query = query.Where("value >= ?", params.MinValue.Bytes())
	}
	if params.MaxValue != nil {
		query = query.Where("value <= ?", params.MaxValue.Bytes())
	}

	// 排序
	if params.OrderBy != "" {
		query = query.Order(params.OrderBy)
	} else {
		query = query.Order("created_at DESC")
	}

	// 分页
	if params.Limit > 0 {
		query = query.Limit(params.Limit)
	}
	if params.Offset > 0 {
		query = query.Offset(params.Offset)
	}

	err := query.Find(&txs).Error
	return txs, err
}

// QueryAttackTxByHash 根据交易哈希查询
func (a *attackTxDB) QueryAttackTxByHash(txHash common.Hash) (*AttackTx, error) {
	var tx AttackTx
	err := a.db.Table("attack_tx").Where("tx_hash", txHash.String()).First(&tx).Error
	if err != nil {
		return nil, err
	}
	return &tx, nil
}

// QueryAttackTxByGUID 根据GUID查询
func (a *attackTxDB) QueryAttackTxByGUID(guid uuid.UUID) (*AttackTx, error) {
	var tx AttackTx
	err := a.db.Where("guid = ?", guid).First(&tx).Error
	if err != nil {
		return nil, err
	}
	return &tx, nil
}

// QueryAttackTxByStatus 根据状态查询
func (a *attackTxDB) QueryAttackTxByStatus(status uint8) ([]AttackTx, error) {
	var txs []AttackTx
	err := a.db.Where("status = ?", status).Find(&txs).Error
	return txs, err
}

// QueryAttackTxByContract 根据合约地址查询
func (a *attackTxDB) QueryAttackTxByContract(contractAddr common.Address) ([]AttackTx, error) {
	var txs []AttackTx
	err := a.db.Where("contract_address = ?", contractAddr.Bytes()).Find(&txs).Error
	return txs, err
}

// QueryAttackTxByBlockRange 根据区块范围查询
func (a *attackTxDB) QueryAttackTxByBlockRange(startBlock, endBlock *big.Int) ([]AttackTx, error) {
	var txs []AttackTx
	query := a.db.Model(&AttackTx{})

	if startBlock != nil {
		query = query.Where("block_number >= ?", startBlock.Bytes())
	}
	if endBlock != nil {
		query = query.Where("block_number <= ?", endBlock.Bytes())
	}

	err := query.Order("block_number ASC").Find(&txs).Error
	return txs, err
}

// CountAttackTx 统计数量
func (a *attackTxDB) CountAttackTx(params AttackTxQueryParams) (int64, error) {
	var count int64
	query := a.db.Model(&AttackTx{})

	if params.Status != nil {
		query = query.Where("status = ?", *params.Status)
	}
	if params.AttackType != nil {
		query = query.Where("attack_type = ?", *params.AttackType)
	}
	if params.ContractAddr != nil {
		query = query.Where("contract_address = ?", params.ContractAddr.Bytes())
	}
	if params.FromAddr != nil {
		query = query.Where("from_address = ?", params.FromAddr.Bytes())
	}
	if params.ToAddr != nil {
		query = query.Where("to_address = ?", params.ToAddr.Bytes())
	}
	if params.BlockNumber != nil {
		query = query.Where("block_number = ?", params.BlockNumber.Bytes())
	}
	if params.MinValue != nil {
		query = query.Where("value >= ?", params.MinValue.Bytes())
	}
	if params.MaxValue != nil {
		query = query.Where("value <= ?", params.MaxValue.Bytes())
	}

	err := query.Count(&count).Error
	return count, err
}

// ===== 辅助查询方法 =====

// GetPendingAttackTx 获取待处理的攻击交易
func (a *attackTxDB) GetPendingAttackTx(limit int) ([]AttackTx, error) {
	status := uint8(StatusPending)
	params := AttackTxQueryParams{
		Status:  &status,
		Limit:   limit,
		OrderBy: "created_at ASC",
	}
	return a.QueryAttackTx(params)
}

// GetAttackTxByStatusAndType 根据状态和攻击类型获取交易
func (a *attackTxDB) GetAttackTxByStatusAndType(status uint8, attackType string, limit int) ([]AttackTx, error) {
	params := AttackTxQueryParams{
		Status:     &status,
		AttackType: &attackType,
		Limit:      limit,
		OrderBy:    "created_at ASC",
	}
	return a.QueryAttackTx(params)
}

// GetAttackTxByValueRange 根据价值范围获取交易
func (a *attackTxDB) GetAttackTxByValueRange(minValue, maxValue *big.Int, limit int) ([]AttackTx, error) {
	params := AttackTxQueryParams{
		MinValue: minValue,
		MaxValue: maxValue,
		Limit:    limit,
		OrderBy:  "value DESC",
	}
	return a.QueryAttackTx(params)
}

// ===== 统计方法 =====

// GetStatusStatistics 获取状态统计信息
func (a *attackTxDB) GetStatusStatistics() (map[uint8]int64, error) {
	type StatusCount struct {
		Status uint8 `json:"status"`
		Count  int64 `json:"count"`
	}

	var results []StatusCount
	err := a.db.Model(&AttackTx{}).
		Select("status, count(*) as count").
		Group("status").
		Find(&results).Error

	if err != nil {
		return nil, err
	}

	stats := make(map[uint8]int64)
	for _, result := range results {
		stats[result.Status] = result.Count
	}

	return stats, nil
}

// GetAttackTypeStatistics 获取攻击类型统计信息
func (a *attackTxDB) GetAttackTypeStatistics() (map[string]int64, error) {
	type TypeCount struct {
		AttackType string `json:"attack_type"`
		Count      int64  `json:"count"`
	}

	var results []TypeCount
	err := a.db.Model(&AttackTx{}).
		Select("attack_type, count(*) as count").
		Group("attack_type").
		Find(&results).Error

	if err != nil {
		return nil, err
	}

	stats := make(map[string]int64)
	for _, result := range results {
		stats[result.AttackType] = result.Count
	}

	return stats, nil
}

// ===== 基本操作方法 =====

// StoreAttackTx 批量存储攻击交易
func (a *attackTxDB) StoreAttackTx(txs []AttackTx) error {
	// 为每个交易生成 GUID
	for i := range txs {
		if txs[i].GUID == uuid.Nil {
			txs[i].GUID = uuid.New()
		}
		// 如果没有设置时间戳，使用当前时间
		if txs[i].Timestamp == 0 {
			txs[i].Timestamp = uint64(time.Now().Unix())
		}
	}

	// 使用批量插入，忽略重复的记录
	return a.db.Create(&txs).Error
}

// UpdateAttackTxStatus 更新攻击交易状态
func (a *attackTxDB) UpdateAttackTxStatus(guid uuid.UUID, status uint8, errorMsg string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if errorMsg != "" {
		updates["error_message"] = errorMsg
	}

	return a.db.Model(&AttackTx{}).Where("guid = ?", guid).Updates(updates).Error
}

// UpdateAttackTxByHash 根据交易哈希更新
func (a *attackTxDB) UpdateAttackTxByHash(txHash common.Hash, updates map[string]interface{}) error {
	if updates == nil {
		updates = make(map[string]interface{})
	}
	updates["updated_at"] = time.Now()

	return a.db.Model(&AttackTx{}).Where("tx_hash = ?", txHash.Bytes()).Updates(updates).Error
}

// DeleteAttackTx 删除攻击交易
func (a *attackTxDB) DeleteAttackTx(guid uuid.UUID) error {
	return a.db.Where("guid = ?", guid).Delete(&AttackTx{}).Error
}

// DeleteAttackTxByHash 根据交易哈希删除
func (a *attackTxDB) DeleteAttackTxByHash(txHash common.Hash) error {
	return a.db.Where("tx_hash = ?", txHash.Bytes()).Delete(&AttackTx{}).Error
}

// ===== 批量操作方法 =====

// BatchUpdateStatus 批量更新状态
func (a *attackTxDB) BatchUpdateStatus(guids []uuid.UUID, status uint8) error {
	return a.db.Model(&AttackTx{}).
		Where("guid IN ?", guids).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		}).Error
}

// BatchUpdateByHashes 根据交易哈希批量更新状态
func (a *attackTxDB) BatchUpdateByHashes(txHashes []common.Hash, status uint8) error {
	hashBytes := make([][]byte, len(txHashes))
	for i, hash := range txHashes {
		hashBytes[i] = hash.Bytes()
	}

	return a.db.Model(&AttackTx{}).
		Where("tx_hash IN ?", hashBytes).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		}).Error
}

// ===== 状态标记方法 - 基于GUID =====

// MarkAsProcessing 标记为处理中
func (a *attackTxDB) MarkAsProcessing(guid uuid.UUID) error {
	return a.UpdateAttackTxStatus(guid, StatusProcessing, "")
}

// MarkAsSuccess 标记为成功
func (a *attackTxDB) MarkAsSuccess(guid uuid.UUID) error {
	return a.UpdateAttackTxStatus(guid, StatusSuccess, "")
}

// MarkAsFailed 标记为失败
func (a *attackTxDB) MarkAsFailed(guid uuid.UUID, errorMsg string) error {
	return a.UpdateAttackTxStatus(guid, StatusFailed, errorMsg)
}

// MarkAsSkipped 标记为跳过
func (a *attackTxDB) MarkAsSkipped(guid uuid.UUID, reason string) error {
	return a.UpdateAttackTxStatus(guid, StatusSkipped, reason)
}

// ===== 状态标记方法 - 基于Hash =====

// MarkAsProcessingByHash 根据交易哈希标记为处理中
func (a *attackTxDB) MarkAsProcessingByHash(txHash common.Hash) error {
	updates := map[string]interface{}{
		"status": StatusProcessing,
	}
	return a.UpdateAttackTxByHash(txHash, updates)
}

// MarkAsSuccessByHash 根据交易哈希标记为成功
func (a *attackTxDB) MarkAsSuccessByHash(txHash common.Hash) error {
	updates := map[string]interface{}{
		"status": StatusSuccess,
	}
	return a.UpdateAttackTxByHash(txHash, updates)
}

// MarkAsFailedByHash 根据交易哈希标记为失败
func (a *attackTxDB) MarkAsFailedByHash(txHash common.Hash, errorMsg string) error {
	updates := map[string]interface{}{
		"status":        StatusFailed,
		"error_message": errorMsg,
	}
	return a.UpdateAttackTxByHash(txHash, updates)
}

// ===== 维护方法 =====

// CleanupOldRecords 清理老旧记录
func (a *attackTxDB) CleanupOldRecords(olderThan time.Time) (int64, error) {
	result := a.db.Where("created_at < ?", olderThan).Delete(&AttackTx{})
	return result.RowsAffected, result.Error
}

// ===== 构造函数 =====

// NewAttackTxDB 创建新的攻击交易数据库实例
func NewAttackTxDB(db *gorm.DB) AttackTxDB {
	return &attackTxDB{db: db}
}

// ===== 辅助函数 =====

// CreateAttackTxFromRPC 从RPC数据创建AttackTx记录的辅助函数
func CreateAttackTxFromRPC(txHash common.Hash, blockNumber *big.Int, blockHash common.Hash,
	contractAddr, fromAddr, toAddr common.Address, value, gasUsed, gasPrice *big.Int,
	attackType string, timestamp uint64) AttackTx {

	return AttackTx{
		GUID:            uuid.New(),
		TxHash:          txHash,
		BlockNumber:     blockNumber,
		BlockHash:       blockHash,
		ContractAddress: contractAddr,
		FromAddress:     fromAddr,
		ToAddress:       toAddr,
		Value:           value,
		GasUsed:         gasUsed,
		GasPrice:        gasPrice,
		Status:          StatusPending,
		AttackType:      attackType,
		ErrorMessage:    "",
		Timestamp:       timestamp,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

// GetStatusString 获取状态字符串表示
func GetStatusString(status uint8) string {
	switch status {
	case StatusPending:
		return "Pending"
	case StatusProcessing:
		return "Processing"
	case StatusSuccess:
		return "Success"
	case StatusFailed:
		return "Failed"
	case StatusSkipped:
		return "Skipped"
	default:
		return "Unknown"
	}
}

// ValidateAttackTx 验证AttackTx记录的有效性
func ValidateAttackTx(tx *AttackTx) error {
	if tx == nil {
		return fmt.Errorf("attack transaction is nil")
	}

	if tx.TxHash == (common.Hash{}) {
		return fmt.Errorf("transaction hash is empty")
	}

	if tx.BlockNumber == nil || tx.BlockNumber.Sign() < 0 {
		return fmt.Errorf("invalid block number")
	}

	if tx.ContractAddress == (common.Address{}) {
		return fmt.Errorf("contract address is empty")
	}

	if tx.AttackType == "" {
		return fmt.Errorf("attack type is empty")
	}

	return nil
}

// BatchCreateAttackTxFromHashes 批量从交易哈希创建AttackTx记录（需要配合RPC使用）
func BatchCreateAttackTxFromHashes(txHashes []common.Hash, contractAddr common.Address, attackType string) []AttackTx {
	txs := make([]AttackTx, len(txHashes))
	now := time.Now()
	timestamp := uint64(now.Unix())

	for i, txHash := range txHashes {
		txs[i] = AttackTx{
			GUID:            uuid.New(),
			TxHash:          txHash,
			ContractAddress: contractAddr,
			Status:          StatusPending,
			AttackType:      attackType,
			Timestamp:       timestamp,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
	}

	return txs
}
