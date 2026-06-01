package repository

import (
	"context"
	"strings"

	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
)

// ITokenUserHideRepository 用户隐藏代币仓储接口
type ITokenUserHideRepository interface {
	// GetByUserIDAndSymbol 根据用户和代币符号查询
	GetByUserIDAndSymbol(ctx context.Context, userID int64, symbol string) (*entity.TokenUserHideEntity, error)

	// GetHiddenSymbolMap 获取用户已隐藏代币集合
	GetHiddenSymbolMap(ctx context.Context, userID int64) (map[string]bool, error)

	// Create 创建用户隐藏代币配置
	Create(ctx context.Context, userID int64, symbol string) error

	// DeleteByUserIDAndSymbol 删除用户隐藏代币配置
	DeleteByUserIDAndSymbol(ctx context.Context, userID int64, symbol string) error
}

type tokenUserHideRepository struct {
	tokenUserHideDao dao.ITokenUserHideDao
}

// NewTokenUserHideRepository 创建实例
func NewTokenUserHideRepository() ITokenUserHideRepository {
	return &tokenUserHideRepository{
		tokenUserHideDao: dao.NewTokenUserHideDao(),
	}
}

// GetByUserIDAndSymbol 根据用户和代币符号查询
func (r *tokenUserHideRepository) GetByUserIDAndSymbol(ctx context.Context, userID int64, symbol string) (*entity.TokenUserHideEntity, error) {
	return r.tokenUserHideDao.GetByUserIDAndSymbol(ctx, userID, strings.ToUpper(strings.TrimSpace(symbol)))
}

// GetHiddenSymbolMap 获取用户已隐藏代币集合
func (r *tokenUserHideRepository) GetHiddenSymbolMap(ctx context.Context, userID int64) (map[string]bool, error) {
	list, err := r.tokenUserHideDao.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(list))
	for _, item := range list {
		m[strings.ToUpper(item.Symbol)] = true
	}
	return m, nil
}

// Create 创建用户隐藏代币配置
func (r *tokenUserHideRepository) Create(ctx context.Context, userID int64, symbol string) error {
	return r.tokenUserHideDao.Create(ctx, nil, &entity.TokenUserHideEntity{
		UserId: userID,
		Symbol: strings.ToUpper(strings.TrimSpace(symbol)),
	})
}

// DeleteByUserIDAndSymbol 删除用户隐藏代币配置
func (r *tokenUserHideRepository) DeleteByUserIDAndSymbol(ctx context.Context, userID int64, symbol string) error {
	return r.tokenUserHideDao.DeleteByUserIDAndSymbol(ctx, nil, userID, strings.ToUpper(strings.TrimSpace(symbol)))
}
