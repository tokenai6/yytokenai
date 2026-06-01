package dao

import (
	"context"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// IWithdrawWhiteListDao 提现地址白名单数据访问接口
type IWithdrawWhiteListDao interface {
	// GetByUserID 根据用户ID获取白名单记录
	GetByUserID(ctx context.Context, userID int64) (*entity.WithdrawWhiteListEntity, error)

	// Create 创建白名单记录
	Create(ctx context.Context, tx gdb.TX, entity *entity.WithdrawWhiteListEntity) error

	// Delete 删除白名单记录
	Delete(ctx context.Context, tx gdb.TX, userID int64) error
}

// withdrawWhiteListDao 提现地址白名单数据访问实现
type withdrawWhiteListDao struct {
	db gdb.DB
}

// NewWithdrawWhiteListDao 创建提现地址白名单数据访问实例
func NewWithdrawWhiteListDao() IWithdrawWhiteListDao {
	return &withdrawWhiteListDao{
		db: db.GetDB(),
	}
}

// GetByUserID 根据用户ID获取白名单记录
func (d *withdrawWhiteListDao) GetByUserID(ctx context.Context, userID int64) (*entity.WithdrawWhiteListEntity, error) {
	var entity entity.WithdrawWhiteListEntity
	err := d.db.Model("withdraw_white_list").Ctx(ctx).
		Where("user_id = ?", userID).
		Scan(&entity)
	if err != nil {
		return nil, err
	}
	if entity.UserID == 0 {
		return nil, nil
	}
	return &entity, nil
}

// Create 创建白名单记录
func (d *withdrawWhiteListDao) Create(ctx context.Context, tx gdb.TX, entity *entity.WithdrawWhiteListEntity) error {
	model := d.db.Model("withdraw_white_list")
	if tx != nil {
		model = tx.Model("withdraw_white_list")
	}

	_, err := model.Ctx(ctx).
		Data(entity).
		Insert()
	return err
}

// Delete 删除白名单记录
func (d *withdrawWhiteListDao) Delete(ctx context.Context, tx gdb.TX, userID int64) error {
	model := d.db.Model("withdraw_white_list")
	if tx != nil {
		model = tx.Model("withdraw_white_list")
	}

	_, err := model.Ctx(ctx).
		Where("user_id = ?", userID).
		Delete()
	return err
}
