package repository

import (
	"context"
	"strings"

	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
)

// ITokenUserDisplayRepository 用户代币显示配置仓储接口
type ITokenUserDisplayRepository interface {
	// GetByUserIDAndSymbol 根据用户和代币符号查询
	GetByUserIDAndSymbol(ctx context.Context, userID int64, symbol string) (*entity.TokenUserDisplayEntity, error)

	// GetShownSymbolMap 获取用户已显示代币集合
	GetShownSymbolMap(ctx context.Context, userID int64) (map[string]bool, error)

	// GetShownSymbolsInAddedOrder 获取用户已显示代币（按添加时间正序）
	GetShownSymbolsInAddedOrder(ctx context.Context, userID int64) ([]string, error)

	// Create 创建用户显示代币配置
	Create(ctx context.Context, userID int64, symbol string) error

	// DeleteByUserIDAndSymbol 删除用户显示代币配置
	DeleteByUserIDAndSymbol(ctx context.Context, userID int64, symbol string) error
}

type tokenUserDisplayRepository struct {
	tokenUserDisplayDao dao.ITokenUserDisplayDao
}

// NewTokenUserDisplayRepository 创建实例
func NewTokenUserDisplayRepository() ITokenUserDisplayRepository {
	return &tokenUserDisplayRepository{
		tokenUserDisplayDao: dao.NewTokenUserDisplayDao(),
	}
}

// GetByUserIDAndSymbol 根据用户和代币符号查询
func (r *tokenUserDisplayRepository) GetByUserIDAndSymbol(ctx context.Context, userID int64, symbol string) (*entity.TokenUserDisplayEntity, error) {
	return r.tokenUserDisplayDao.GetByUserIDAndSymbol(ctx, userID, strings.ToUpper(strings.TrimSpace(symbol)))
}

// GetShownSymbolMap 获取用户已显示代币集合
func (r *tokenUserDisplayRepository) GetShownSymbolMap(ctx context.Context, userID int64) (map[string]bool, error) {
	list, err := r.tokenUserDisplayDao.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(list))
	for _, item := range list {
		m[strings.ToUpper(item.Symbol)] = true
	}
	return m, nil
}

// GetShownSymbolsInAddedOrder 获取用户已显示代币（按添加时间正序）
func (r *tokenUserDisplayRepository) GetShownSymbolsInAddedOrder(ctx context.Context, userID int64) ([]string, error) {
	list, err := r.tokenUserDisplayDao.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(list))
	for _, item := range list {
		symbol := strings.ToUpper(strings.TrimSpace(item.Symbol))
		if symbol == "" {
			continue
		}
		result = append(result, symbol)
	}
	return result, nil
}

// Create 创建用户显示代币配置
func (r *tokenUserDisplayRepository) Create(ctx context.Context, userID int64, symbol string) error {
	return r.tokenUserDisplayDao.Create(ctx, nil, &entity.TokenUserDisplayEntity{
		UserId: userID,
		Symbol: strings.ToUpper(strings.TrimSpace(symbol)),
	})
}

// DeleteByUserIDAndSymbol 删除用户显示代币配置
func (r *tokenUserDisplayRepository) DeleteByUserIDAndSymbol(ctx context.Context, userID int64, symbol string) error {
	return r.tokenUserDisplayDao.DeleteByUserIDAndSymbol(ctx, nil, userID, strings.ToUpper(strings.TrimSpace(symbol)))
}
