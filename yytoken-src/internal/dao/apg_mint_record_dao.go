package dao

import (
	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"
	"context"

	"github.com/gogf/gf/v2/database/gdb"
)

// ApgMintRecordDao APG提取记录DAO
type ApgMintRecordDao struct {
	table string
	db    gdb.DB
}

// NewApgMintRecordDao 创建APG提取记录DAO
func NewApgMintRecordDao() *ApgMintRecordDao {
	return &ApgMintRecordDao{
		table: "apg_mint_records",
		db:    db.GetDB(),
	}
}

// Insert 插入APG提取记录
func (d *ApgMintRecordDao) Insert(ctx context.Context, record *entity.ApgMintRecord) error {
	_, err := d.db.Model(d.table).Ctx(ctx).FieldsEx("id").Data(record).Insert()
	return err
}

// InsertWithTx 在事务中插入APG提取记录
func (d *ApgMintRecordDao) InsertWithTx(ctx context.Context, tx gdb.TX, record *entity.ApgMintRecord) error {
	_, err := tx.Model(d.table).Ctx(ctx).FieldsEx("id").Data(record).Insert()
	return err
}

// GetByTxHash 根据交易哈希和事件索引查询记录
func (d *ApgMintRecordDao) GetByTxHash(ctx context.Context, txHash string, eventIndex int) (*entity.ApgMintRecord, error) {
	var record entity.ApgMintRecord
	err := d.db.Model(d.table).Ctx(ctx).
		Where("tx_hash = ?", txHash).
		Where("event_index = ?", eventIndex).
		Scan(&record)

	if err != nil {
		return nil, err
	}

	return &record, nil
}

// GetByNonce 根据nonce查询记录
func (d *ApgMintRecordDao) GetByNonce(ctx context.Context, nonce int64) (*entity.ApgMintRecord, error) {
	var record entity.ApgMintRecord
	err := d.db.Model(d.table).Ctx(ctx).
		Where("nonce = ?", nonce).
		Scan(&record)

	if err != nil {
		return nil, err
	}

	return &record, nil
}

// GetByUserId 根据用户ID查询记录列表
func (d *ApgMintRecordDao) GetByUserId(ctx context.Context, userId int64, limit int) ([]*entity.ApgMintRecord, error) {
	var records []*entity.ApgMintRecord
	err := d.db.Model(d.table).Ctx(ctx).
		Where("user_id = ?", userId).
		Order("created_at DESC").
		Limit(limit).
		Scan(&records)

	return records, err
}

// ExistsByTxHash 检查交易是否已处理
func (d *ApgMintRecordDao) ExistsByTxHash(ctx context.Context, txHash string, eventIndex int) (bool, error) {
	count, err := d.db.Model(d.table).Ctx(ctx).
		Where("tx_hash = ?", txHash).
		Where("event_index = ?", eventIndex).
		Count()

	return count > 0, err
}
