package dao

import (
	"context"
	"database/sql"
	"errors"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// ITokenUserDisplayDao 用户代币显示配置数据访问接口
type ITokenUserDisplayDao interface {
	// GetByUserIDAndSymbol 根据用户和代币符号查询
	GetByUserIDAndSymbol(ctx context.Context, userID int64, symbol string) (*entity.TokenUserDisplayEntity, error)

	// GetByUserID 查询用户所有显示代币
	GetByUserID(ctx context.Context, userID int64) ([]*entity.TokenUserDisplayEntity, error)

	// Create 创建用户显示代币配置
	Create(ctx context.Context, tx gdb.TX, item *entity.TokenUserDisplayEntity) error

	// DeleteByUserIDAndSymbol 删除用户显示代币配置
	DeleteByUserIDAndSymbol(ctx context.Context, tx gdb.TX, userID int64, symbol string) error
}

type tokenUserDisplayDao struct {
	db gdb.DB
}

// NewTokenUserDisplayDao 创建实例
func NewTokenUserDisplayDao() ITokenUserDisplayDao {
	return &tokenUserDisplayDao{db: db.GetDB()}
}

// GetByUserIDAndSymbol 根据用户和代币符号查询
func (d *tokenUserDisplayDao) GetByUserIDAndSymbol(ctx context.Context, userID int64, symbol string) (*entity.TokenUserDisplayEntity, error) {
	var item entity.TokenUserDisplayEntity
	err := d.db.Model("token_user_display").Ctx(ctx).
		Where("user_id = ?", userID).
		Where("symbol = ?", symbol).
		Scan(&item)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	if item.Id == 0 {
		return nil, nil
	}
	return &item, nil
}

// GetByUserID 查询用户所有显示代币
func (d *tokenUserDisplayDao) GetByUserID(ctx context.Context, userID int64) ([]*entity.TokenUserDisplayEntity, error) {
	var list []*entity.TokenUserDisplayEntity
	err := d.db.Model("token_user_display").Ctx(ctx).
		Where("user_id = ?", userID).
		OrderAsc("id").
		Scan(&list)
	if err != nil {
		return nil, err
	}
	return list, nil
}

// Create 创建用户显示代币配置
func (d *tokenUserDisplayDao) Create(ctx context.Context, tx gdb.TX, item *entity.TokenUserDisplayEntity) error {
	model := d.db.Model("token_user_display")
	if tx != nil {
		model = tx.Model("token_user_display")
	}

	_, err := model.Ctx(ctx).
		FieldsEx("id", "created_at", "updated_at").
		Data(item).
		Insert()
	return err
}

// DeleteByUserIDAndSymbol 删除用户显示代币配置
func (d *tokenUserDisplayDao) DeleteByUserIDAndSymbol(ctx context.Context, tx gdb.TX, userID int64, symbol string) error {
	model := d.db.Model("token_user_display")
	if tx != nil {
		model = tx.Model("token_user_display")
	}

	_, err := model.Ctx(ctx).
		Where("user_id = ?", userID).
		Where("symbol = ?", symbol).
		Delete()
	return err
}
