package dao

import (
	"context"
	"database/sql"
	"errors"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// ITokenUserHideDao 用户隐藏代币数据访问接口
type ITokenUserHideDao interface {
	// GetByUserIDAndSymbol 根据用户和代币符号查询
	GetByUserIDAndSymbol(ctx context.Context, userID int64, symbol string) (*entity.TokenUserHideEntity, error)

	// GetByUserID 查询用户所有隐藏代币
	GetByUserID(ctx context.Context, userID int64) ([]*entity.TokenUserHideEntity, error)

	// Create 创建用户隐藏代币配置
	Create(ctx context.Context, tx gdb.TX, item *entity.TokenUserHideEntity) error

	// DeleteByUserIDAndSymbol 删除用户隐藏代币配置
	DeleteByUserIDAndSymbol(ctx context.Context, tx gdb.TX, userID int64, symbol string) error
}

type tokenUserHideDao struct {
	db gdb.DB
}

// NewTokenUserHideDao 创建实例
func NewTokenUserHideDao() ITokenUserHideDao {
	return &tokenUserHideDao{db: db.GetDB()}
}

// GetByUserIDAndSymbol 根据用户和代币符号查询
func (d *tokenUserHideDao) GetByUserIDAndSymbol(ctx context.Context, userID int64, symbol string) (*entity.TokenUserHideEntity, error) {
	var item entity.TokenUserHideEntity
	err := d.db.Model("token_user_hide").Ctx(ctx).
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

// GetByUserID 查询用户所有隐藏代币
func (d *tokenUserHideDao) GetByUserID(ctx context.Context, userID int64) ([]*entity.TokenUserHideEntity, error) {
	var list []*entity.TokenUserHideEntity
	err := d.db.Model("token_user_hide").Ctx(ctx).
		Where("user_id = ?", userID).
		OrderAsc("id").
		Scan(&list)
	if err != nil {
		return nil, err
	}
	return list, nil
}

// Create 创建用户隐藏代币配置
func (d *tokenUserHideDao) Create(ctx context.Context, tx gdb.TX, item *entity.TokenUserHideEntity) error {
	model := d.db.Model("token_user_hide")
	if tx != nil {
		model = tx.Model("token_user_hide")
	}

	_, err := model.Ctx(ctx).
		FieldsEx("id", "created_at", "updated_at").
		Data(item).
		Insert()
	return err
}

// DeleteByUserIDAndSymbol 删除用户隐藏代币配置
func (d *tokenUserHideDao) DeleteByUserIDAndSymbol(ctx context.Context, tx gdb.TX, userID int64, symbol string) error {
	model := d.db.Model("token_user_hide")
	if tx != nil {
		model = tx.Model("token_user_hide")
	}

	_, err := model.Ctx(ctx).
		Where("user_id = ?", userID).
		Where("symbol = ?", symbol).
		Delete()
	return err
}
