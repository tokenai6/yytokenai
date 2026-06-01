package dao

import (
	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/shopspring/decimal"
)

// IApgBurnRecordDao APG销毁记录数据访问接口
type IApgBurnRecordDao interface {
	// 基础CRUD操作
	Insert(ctx context.Context, record *entity.ApgBurnRecord) error
	InsertWithTx(ctx context.Context, tx gdb.TX, record *entity.ApgBurnRecord) error

	// 查询操作
	GetByTxHash(ctx context.Context, txHash string, eventIndex int) (*entity.ApgBurnRecord, error)
	ExistsByTxHash(ctx context.Context, txHash string, eventIndex int) (bool, error)
	GetByFromAddress(ctx context.Context, fromAddress string, limit int) ([]*entity.ApgBurnRecord, error)

	// 统计查询
	SumBurnAmountByDateRange(ctx context.Context, startTime, endTime time.Time) (decimal.Decimal, error)
	GetByDateRange(ctx context.Context, startTime, endTime time.Time) ([]*entity.ApgBurnRecord, error)
}

// ApgBurnRecordDao APG销毁记录DAO
type ApgBurnRecordDao struct {
	table string
	db    gdb.DB
}

// NewApgBurnRecordDao 创建APG销毁记录DAO
func NewApgBurnRecordDao() *ApgBurnRecordDao {
	return &ApgBurnRecordDao{
		table: "apg_burn_records",
		db:    db.GetDB(),
	}
}

// Insert 插入APG销毁记录
func (d *ApgBurnRecordDao) Insert(ctx context.Context, record *entity.ApgBurnRecord) error {
	_, err := d.db.Model(d.table).Ctx(ctx).FieldsEx("id").Insert(record)
	return err
}

// InsertWithTx 在事务中插入APG销毁记录
func (d *ApgBurnRecordDao) InsertWithTx(ctx context.Context, tx gdb.TX, record *entity.ApgBurnRecord) error {
	_, err := tx.Model(d.table).Ctx(ctx).FieldsEx("id").Insert(record)
	return err
}

// GetByTxHash 根据交易哈希和事件索引查询记录
func (d *ApgBurnRecordDao) GetByTxHash(ctx context.Context, txHash string, eventIndex int) (*entity.ApgBurnRecord, error) {
	var record entity.ApgBurnRecord
	err := d.db.Model(d.table).Ctx(ctx).
		Where("tx_hash = ?", txHash).
		Where("event_index = ?", eventIndex).
		Scan(&record)

	if err != nil {
		return nil, err
	}

	return &record, nil
}

// ExistsByTxHash 检查交易是否已处理
func (d *ApgBurnRecordDao) ExistsByTxHash(ctx context.Context, txHash string, eventIndex int) (bool, error) {
	count, err := d.db.Model(d.table).Ctx(ctx).
		Where("tx_hash = ?", txHash).
		Where("event_index = ?", eventIndex).
		Count()

	return count > 0, err
}

// GetByFromAddress 根据发起者地址查询记录列表
func (d *ApgBurnRecordDao) GetByFromAddress(ctx context.Context, fromAddress string, limit int) ([]*entity.ApgBurnRecord, error) {
	var records []*entity.ApgBurnRecord
	err := d.db.Model(d.table).Ctx(ctx).
		Where("from_address = ?", fromAddress).
		Order("created_at DESC").
		Limit(limit).
		Scan(&records)

	return records, err
}

// SumBurnAmountByDateRange 统计时间范围内的销毁总量
func (d *ApgBurnRecordDao) SumBurnAmountByDateRange(ctx context.Context, startTime, endTime time.Time) (decimal.Decimal, error) {
	var result struct {
		Total decimal.Decimal `json:"total"`
	}

	err := d.db.Model(d.table).Ctx(ctx).
		Fields("COALESCE(SUM(burn_amount), 0) as total").
		Where("block_timestamp >= ?", startTime).
		Where("block_timestamp < ?", endTime).
		Scan(&result)

	if err != nil {
		return decimal.Zero, err
	}

	return result.Total, nil
}

// GetByDateRange 获取时间范围内的销毁记录
func (d *ApgBurnRecordDao) GetByDateRange(ctx context.Context, startTime, endTime time.Time) ([]*entity.ApgBurnRecord, error) {
	var records []*entity.ApgBurnRecord
	err := d.db.Model(d.table).Ctx(ctx).
		Where("block_timestamp >= ?", startTime).
		Where("block_timestamp < ?", endTime).
		Order("block_timestamp ASC").
		Scan(&records)

	return records, err
}
