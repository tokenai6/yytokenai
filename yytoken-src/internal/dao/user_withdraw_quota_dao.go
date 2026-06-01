package dao

import (
	"context"
	"database/sql"
	"errors"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/shopspring/decimal"
)

// IUserWithdrawQuotaDao 用户提现额度数据访问接口
type IUserWithdrawQuotaDao interface {
	// GetByUserIDAndMonth 根据用户ID和月份获取额度记录
	GetByUserIDAndMonth(ctx context.Context, userID int64, month string) (*entity.UserWithdrawQuotaEntity, error)

	// Create 创建额度记录
	Create(ctx context.Context, tx gdb.TX, record *entity.UserWithdrawQuotaEntity) error

	// Update 更新额度记录
	Update(ctx context.Context, tx gdb.TX, userID int64, month string, data map[string]interface{}) error

	// GetOrCreate 获取或创建额度记录
	GetOrCreate(ctx context.Context, tx gdb.TX, userID int64, month string) (*entity.UserWithdrawQuotaEntity, error)
}

// userWithdrawQuotaDao 用户提现额度数据访问实现
type userWithdrawQuotaDao struct {
	db gdb.DB
}

// NewUserWithdrawQuotaDao 创建用户提现额度数据访问实例
func NewUserWithdrawQuotaDao() IUserWithdrawQuotaDao {
	return &userWithdrawQuotaDao{
		db: db.GetDB(),
	}
}

// GetByUserIDAndMonth 根据用户ID和月份获取额度记录
func (d *userWithdrawQuotaDao) GetByUserIDAndMonth(ctx context.Context, userID int64, month string) (*entity.UserWithdrawQuotaEntity, error) {
	var record entity.UserWithdrawQuotaEntity
	err := d.db.Model("user_withdraw_quota").Ctx(ctx).
		Where("user_id = ? AND month = ?", userID, month).
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

// Create 创建额度记录
func (d *userWithdrawQuotaDao) Create(ctx context.Context, tx gdb.TX, record *entity.UserWithdrawQuotaEntity) error {
	model := d.db.Model("user_withdraw_quota")
	if tx != nil {
		model = tx.Model("user_withdraw_quota")
	}

	result, err := model.Ctx(ctx).
		FieldsEx("id", "created_at", "updated_at").
		Data(record).
		InsertAndGetId()
	if err != nil {
		return err
	}
	record.Id = result
	return nil
}

// Update 更新额度记录
func (d *userWithdrawQuotaDao) Update(ctx context.Context, tx gdb.TX, userID int64, month string, data map[string]interface{}) error {
	model := d.db.Model("user_withdraw_quota")
	if tx != nil {
		model = tx.Model("user_withdraw_quota")
	}

	_, err := model.Ctx(ctx).
		Where("user_id = ? AND month = ?", userID, month).
		Data(data).
		Update()
	return err
}

// GetOrCreate 获取或创建额度记录
func (d *userWithdrawQuotaDao) GetOrCreate(ctx context.Context, tx gdb.TX, userID int64, month string) (*entity.UserWithdrawQuotaEntity, error) {
	record, err := d.GetByUserIDAndMonth(ctx, userID, month)
	if err != nil {
		return nil, err
	}
	if record != nil {
		return record, nil
	}

	record = &entity.UserWithdrawQuotaEntity{
		UserID:           userID,
		Month:            month,
		WithdrawOffset:   decimal.Zero,
		ExtraQuota:       decimal.Zero,
		TaxDeductionUsed: decimal.Zero,
		Remark:           "",
	}

	if err := d.Create(ctx, tx, record); err != nil {
		return nil, err
	}
	return record, nil
}
