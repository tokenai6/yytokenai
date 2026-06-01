package repository

import (
	"context"

	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
)

// ITokenWhitelistRepository 代币白名单仓储接口
type ITokenWhitelistRepository interface {
	// GetAll 获取所有白名单代币
	GetAll(ctx context.Context) ([]*entity.TokenWhitelistEntity, error)

	// GetBySymbol 根据代币符号获取白名单记录
	GetBySymbol(ctx context.Context, symbol string) (*entity.TokenWhitelistEntity, error)
}

// tokenWhitelistRepository 代币白名单仓储实现
type tokenWhitelistRepository struct {
	tokenWhitelistDao dao.ITokenWhitelistDao
}

// NewTokenWhitelistRepository 创建代币白名单仓储实例
func NewTokenWhitelistRepository() ITokenWhitelistRepository {
	return &tokenWhitelistRepository{
		tokenWhitelistDao: dao.NewTokenWhitelistDao(),
	}
}

// GetAll 获取所有白名单代币
func (r *tokenWhitelistRepository) GetAll(ctx context.Context) ([]*entity.TokenWhitelistEntity, error) {
	return r.tokenWhitelistDao.GetAll(ctx)
}

// GetBySymbol 根据代币符号获取白名单记录
func (r *tokenWhitelistRepository) GetBySymbol(ctx context.Context, symbol string) (*entity.TokenWhitelistEntity, error) {
	return r.tokenWhitelistDao.GetBySymbol(ctx, symbol)
}
