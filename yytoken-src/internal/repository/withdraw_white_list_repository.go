package repository

import (
	"context"

	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
)

// IWithdrawWhiteListRepository 提现地址白名单仓储接口
type IWithdrawWhiteListRepository interface {
	// GetByUserID 根据用户ID获取白名单记录
	GetByUserID(ctx context.Context, userID int64) (*entity.WithdrawWhiteListEntity, error)

	// Create 创建白名单记录
	Create(ctx context.Context, userID int64, remark string) error

	// Delete 删除白名单记录
	Delete(ctx context.Context, userID int64) error
}

// withdrawWhiteListRepository 提现地址白名单仓储实现
type withdrawWhiteListRepository struct {
	withdrawWhiteListDao dao.IWithdrawWhiteListDao
}

// NewWithdrawWhiteListRepository 创建提现地址白名单仓储实例
func NewWithdrawWhiteListRepository() IWithdrawWhiteListRepository {
	return &withdrawWhiteListRepository{
		withdrawWhiteListDao: dao.NewWithdrawWhiteListDao(),
	}
}

// GetByUserID 根据用户ID获取白名单记录
func (r *withdrawWhiteListRepository) GetByUserID(ctx context.Context, userID int64) (*entity.WithdrawWhiteListEntity, error) {
	return r.withdrawWhiteListDao.GetByUserID(ctx, userID)
}

// Create 创建白名单记录
func (r *withdrawWhiteListRepository) Create(ctx context.Context, userID int64, remark string) error {
	entity := &entity.WithdrawWhiteListEntity{
		UserID: userID,
		Remark: remark,
	}
	return r.withdrawWhiteListDao.Create(ctx, nil, entity)
}

// Delete 删除白名单记录
func (r *withdrawWhiteListRepository) Delete(ctx context.Context, userID int64) error {
	return r.withdrawWhiteListDao.Delete(ctx, nil, userID)
}
