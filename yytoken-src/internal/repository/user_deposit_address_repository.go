package repository

import (
	"context"

	"XWFrame/internal/dao"
	"XWFrame/internal/entity"

	"github.com/gogf/gf/v2/frame/g"
)

// UserDepositAddressRepository 用户充值地址仓储
type UserDepositAddressRepository struct {
	dao *dao.UserDepositAddressDao
}

// NewUserDepositAddressRepository 创建用户充值地址仓储
func NewUserDepositAddressRepository() *UserDepositAddressRepository {
	return &UserDepositAddressRepository{
		dao: dao.NewUserDepositAddressDao(),
	}
}

// GetUserDepositAddress 获取用户有效的充值地址
func (r *UserDepositAddressRepository) GetUserDepositAddress(ctx context.Context, userID int64) (*entity.UserDepositAddressEntity, error) {
	address, err := r.dao.GetByUserId(ctx, userID)
	if err != nil {
		return nil, err
	}
	return address, nil
}

// CreateDepositAddress 创建充值地址记录
func (r *UserDepositAddressRepository) CreateDepositAddress(ctx context.Context, userID int64, chainID, address string) (*entity.UserDepositAddressEntity, error) {
	entity := &entity.UserDepositAddressEntity{
		UserID:  userID,
		ChainID: chainID,
		Address: address,
		IsValid: true,
	}

	err := r.dao.Create(ctx, entity)
	if err != nil {
		return nil, err
	}

	g.Log().Infof(ctx, "为用户 %d 创建充值地址: %s", userID, address)
	return entity, nil
}

// GetByAddress 根据地址查询
func (r *UserDepositAddressRepository) GetByAddress(ctx context.Context, address string) (*entity.UserDepositAddressEntity, error) {
	entity, err := r.dao.GetByAddress(ctx, address)
	if err != nil {
		return nil, err
	}
	return entity, nil
}
