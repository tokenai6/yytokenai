package dao

import (
	"context"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// ITokenWhitelistDao 代币白名单数据访问接口
type ITokenWhitelistDao interface {
	// GetAll 获取所有白名单代币
	GetAll(ctx context.Context) ([]*entity.TokenWhitelistEntity, error)

	// GetBySymbol 根据代币符号获取白名单记录
	GetBySymbol(ctx context.Context, symbol string) (*entity.TokenWhitelistEntity, error)

	// Create 创建白名单记录
	Create(ctx context.Context, tx gdb.TX, entity *entity.TokenWhitelistEntity) error
}

// tokenWhitelistDao 代币白名单数据访问实现
type tokenWhitelistDao struct {
	db gdb.DB
}

// NewTokenWhitelistDao 创建代币白名单数据访问实例
func NewTokenWhitelistDao() ITokenWhitelistDao {
	return &tokenWhitelistDao{
		db: db.GetDB(),
	}
}

// GetAll 获取所有白名单代币
func (d *tokenWhitelistDao) GetAll(ctx context.Context) ([]*entity.TokenWhitelistEntity, error) {
	var list []*entity.TokenWhitelistEntity
	err := d.db.Model("token_whitelist").Ctx(ctx).
		OrderAsc("id").
		Scan(&list)
	if err != nil {
		return nil, err
	}
	return list, nil
}

// GetBySymbol 根据代币符号获取白名单记录
func (d *tokenWhitelistDao) GetBySymbol(ctx context.Context, symbol string) (*entity.TokenWhitelistEntity, error) {
	var entity entity.TokenWhitelistEntity
	err := d.db.Model("token_whitelist").Ctx(ctx).
		Where("LOWER(symbol) = LOWER(?)", symbol).
		Scan(&entity)
	if err != nil {
		return nil, err
	}
	if entity.Id == 0 {
		return nil, nil
	}
	return &entity, nil
}

// Create 创建白名单记录
func (d *tokenWhitelistDao) Create(ctx context.Context, tx gdb.TX, entity *entity.TokenWhitelistEntity) error {
	model := d.db.Model("token_whitelist")
	if tx != nil {
		model = tx.Model("token_whitelist")
	}

	_, err := model.Ctx(ctx).
		Data(entity).
		Insert()
	return err
}
