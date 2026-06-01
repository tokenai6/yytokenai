package dao

import (
	"context"
	"database/sql"
	"errors"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// ITransferRecordDao 内部转账记录数据访问接口
type ITransferRecordDao interface {
	Create(ctx context.Context, tx gdb.TX, record *entity.InternalTransferRecordEntity) error
	GetByFromUserAndRequestID(ctx context.Context, fromUserID int64, requestID string) (*entity.InternalTransferRecordEntity, error)
	ListByUserID(ctx context.Context, userID int64, direction string, page, pageSize int) ([]*entity.InternalTransferRecordEntity, int, error)
}

// transferRecordDao 内部转账记录数据访问实现
type transferRecordDao struct {
	db gdb.DB
}

// NewTransferRecordDao 创建内部转账记录数据访问实例
func NewTransferRecordDao() ITransferRecordDao {
	return &transferRecordDao{
		db: db.GetDB(),
	}
}

// Create 创建转账记录
func (d *transferRecordDao) Create(ctx context.Context, tx gdb.TX, record *entity.InternalTransferRecordEntity) error {
	model := d.db.Model("internal_transfer_record")
	if tx != nil {
		model = tx.Model("internal_transfer_record")
	}
	id, err := model.Ctx(ctx).FieldsEx("id", "created_at").Data(record).InsertAndGetId()
	if err != nil {
		return err
	}
	record.Id = id
	return nil
}

// GetByFromUserAndRequestID 根据转出方用户ID和请求ID获取记录（幂等检查）
func (d *transferRecordDao) GetByFromUserAndRequestID(ctx context.Context, fromUserID int64, requestID string) (*entity.InternalTransferRecordEntity, error) {
	if requestID == "" {
		return nil, nil
	}
	var record entity.InternalTransferRecordEntity
	err := d.db.Model("internal_transfer_record").Ctx(ctx).
		Where("from_user_id = ? AND request_id = ?", fromUserID, requestID).
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

// ListByUserID 查询用户的内部转账记录
// direction: in(转入) / out(转出) / all(全部)
func (d *transferRecordDao) ListByUserID(ctx context.Context, userID int64, direction string, page, pageSize int) ([]*entity.InternalTransferRecordEntity, int, error) {
	model := d.db.Model("internal_transfer_record").Ctx(ctx)
	switch direction {
	case "in":
		model = model.Where("to_user_id = ?", userID)
	case "out":
		model = model.Where("from_user_id = ?", userID)
	default:
		model = model.Where("from_user_id = ? OR to_user_id = ?", userID, userID)
	}

	var total int
	countModel := model
	count, err := countModel.Count()
	if err != nil {
		return nil, 0, err
	}
	total = count

	var records []*entity.InternalTransferRecordEntity
	err = model.Order("created_at DESC, id DESC").Page(page, pageSize).Scan(&records)
	if err != nil {
		return nil, 0, err
	}
	return records, total, nil
}
