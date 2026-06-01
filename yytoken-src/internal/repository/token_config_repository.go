package repository

import (
	"context"

	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
)

// ITokenConfigRepository 代币配置仓储接口
type ITokenConfigRepository interface {
	// GetBySymbol 根据代币符号获取配置
	GetBySymbol(ctx context.Context, symbol string) (*entity.TokenConfigEntity, error)

	// GetAllEnabled 获取所有启用的代币配置
	GetAllEnabled(ctx context.Context) ([]*entity.TokenConfigEntity, error)

	// GetAll 获取所有代币配置（含禁用）
	GetAll(ctx context.Context) ([]*entity.TokenConfigEntity, error)

	// SearchBySymbol 根据代币符号模糊搜索（忽略大小写）
	SearchBySymbol(ctx context.Context, symbol string) ([]*entity.TokenConfigEntity, error)

	// UpdateById 根据ID更新配置
	UpdateById(ctx context.Context, id int64, data map[string]interface{}) error

	// GetTicketTokens 获取所有可作为拼团门票的代币配置
	GetTicketTokens(ctx context.Context) ([]*entity.TokenConfigEntity, error)
}

// tokenConfigRepository 代币配置仓储实现
type tokenConfigRepository struct {
	tokenConfigDao dao.ITokenConfigDao
}

// NewTokenConfigRepository 创建代币配置仓储实例
func NewTokenConfigRepository() ITokenConfigRepository {
	return &tokenConfigRepository{
		tokenConfigDao: dao.NewTokenConfigDao(),
	}
}

// GetBySymbol 根据代币符号获取配置
func (r *tokenConfigRepository) GetBySymbol(ctx context.Context, symbol string) (*entity.TokenConfigEntity, error) {
	return r.tokenConfigDao.GetBySymbol(ctx, symbol)
}

// GetAllEnabled 获取所有启用的代币配置
func (r *tokenConfigRepository) GetAllEnabled(ctx context.Context) ([]*entity.TokenConfigEntity, error) {
	return r.tokenConfigDao.GetAllEnabled(ctx)
}

// GetAll 获取所有代币配置（含禁用）
func (r *tokenConfigRepository) GetAll(ctx context.Context) ([]*entity.TokenConfigEntity, error) {
	return r.tokenConfigDao.GetAll(ctx)
}

// SearchBySymbol 根据代币符号模糊搜索（忽略大小写）
func (r *tokenConfigRepository) SearchBySymbol(ctx context.Context, symbol string) ([]*entity.TokenConfigEntity, error) {
	return r.tokenConfigDao.SearchBySymbol(ctx, symbol)
}

// UpdateById 根据ID更新配置
func (r *tokenConfigRepository) UpdateById(ctx context.Context, id int64, data map[string]interface{}) error {
	return r.tokenConfigDao.UpdateById(ctx, nil, id, data)
}

// GetTicketTokens 获取所有可作为拼团门票的代币配置
func (r *tokenConfigRepository) GetTicketTokens(ctx context.Context) ([]*entity.TokenConfigEntity, error) {
	return r.tokenConfigDao.GetTicketTokens(ctx)
}
