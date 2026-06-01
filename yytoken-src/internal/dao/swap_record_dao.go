package dao

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ISwapRecordDao Swap记录数据访问接口
type ISwapRecordDao interface {
	Create(ctx context.Context, tx gdb.TX, record *entity.SwapRecordEntity) error
	GetByUserAndRequestID(ctx context.Context, userID int64, requestID string) (*entity.SwapRecordEntity, error)
	UpdateBurnTxHash(ctx context.Context, id int64, burnTxHash string) error
}

// swapRecordDao Swap记录数据访问实现
type swapRecordDao struct {
	db gdb.DB
}

// NewSwapRecordDao 创建Swap记录数据访问实例
func NewSwapRecordDao() ISwapRecordDao {
	return &swapRecordDao{
		db: db.GetDB(),
	}
}

// Create 创建Swap记录
func (d *swapRecordDao) Create(ctx context.Context, tx gdb.TX, record *entity.SwapRecordEntity) error {
	now := time.Now()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = now
	}

	model := d.db.Model("swap_record")
	if tx != nil {
		model = tx.Model("swap_record")
	}
	id, err := model.Ctx(ctx).FieldsEx("id").Data(record).InsertAndGetId()
	if err != nil {
		return err
	}
	record.Id = id
	return nil
}

// UpdateBurnTxHash 更新 burn_tx_hash
func (d *swapRecordDao) UpdateBurnTxHash(ctx context.Context, id int64, burnTxHash string) error {
	_, err := d.db.Model("swap_record").Ctx(ctx).
		Where("id", id).
		Data(g.Map{"burn_tx_hash": burnTxHash}).
		Update()
	return err
}

// GetByUserAndRequestID 根据用户ID和请求ID获取记录（幂等检查）
func (d *swapRecordDao) GetByUserAndRequestID(ctx context.Context, userID int64, requestID string) (*entity.SwapRecordEntity, error) {
	if requestID == "" {
		return nil, nil
	}
	var record entity.SwapRecordEntity
	err := d.db.Model("swap_record").Ctx(ctx).
		Where("user_id = ? AND request_id = ?", userID, requestID).
		Scan(&record)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if record.Id == 0 {
		return nil, nil
	}
	return &record, nil
}
