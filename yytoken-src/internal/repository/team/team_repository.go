package team

import (
	"context"

	"XWFrame/internal/dao"
	entity "XWFrame/internal/entity"
)

// ITeamRepository 团队仓储接口
type ITeamRepository interface {
	// GetUserByID 获取用户信息
	GetUserByID(ctx context.Context, userID int64) (*entity.UserEntity, error)

	// GetDirectReferralsByWalletAddress 获取直推用户列表（分页）
	GetDirectReferralsByWalletAddress(ctx context.Context, walletAddress string, page, pageSize int, startDate, endDate string) ([]*entity.UserEntity, int, error)

	// GetDirectCountByUserID 获取直推人数（实时查询）
	GetDirectCountByUserID(ctx context.Context, userID int64) (int, error)

	// GetTeamTotalCount 获取团队总人数
	GetTeamTotalCount(ctx context.Context, userID int64) (int, error)
}

// teamRepository 团队仓储实现
type teamRepository struct {
	userDao dao.IUserDao
}

// NewTeamRepository 创建团队仓储实例
func NewTeamRepository() ITeamRepository {
	return &teamRepository{
		userDao: dao.NewUserDao(),
	}
}

// GetUserByID 获取用户信息
func (r *teamRepository) GetUserByID(ctx context.Context, userID int64) (*entity.UserEntity, error) {
	return r.userDao.GetById(ctx, userID)
}

// GetDirectReferralsByWalletAddress 获取直推用户列表（分页）
func (r *teamRepository) GetDirectReferralsByWalletAddress(ctx context.Context, walletAddress string, page, pageSize int, startDate, endDate string) ([]*entity.UserEntity, int, error) {
	return r.userDao.GetDirectReferralsByWalletAddress(ctx, walletAddress, page, pageSize, startDate, endDate)
}

// GetDirectCountByUserID 获取直推人数（实时查询）
func (r *teamRepository) GetDirectCountByUserID(ctx context.Context, userID int64) (int, error) {
	return r.userDao.GetDirectCountByUserID(ctx, userID)
}

// GetTeamTotalCount 获取团队总人数
func (r *teamRepository) GetTeamTotalCount(ctx context.Context, userID int64) (int, error) {
	return r.userDao.GetTeamTotalCount(ctx, userID)
}
