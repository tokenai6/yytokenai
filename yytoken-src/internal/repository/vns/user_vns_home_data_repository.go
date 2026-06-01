package vns

import (
	"context"

	vnsDao "XWFrame/internal/dao/vns"
	vnsEntity "XWFrame/internal/entity/vns"

	"github.com/gogf/gf/v2/database/gdb"
)

// IUserVnsHomeDataRepository VNS首页数据仓储接口
type IUserVnsHomeDataRepository interface {
	// GetByUserID 根据用户ID获取VNS首页数据（如果不存在则创建）
	GetByUserID(ctx context.Context, userID int64) (*vnsEntity.UserVnsHomeDataEntity, error)

	// GetBaseFieldsByUserIDs 批量查询 VNS首页数据基础字段（仅 user_id/admin_level/reward_share/vip_level）
	GetBaseFieldsByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*vnsEntity.UserVnsHomeDataEntity, error)

	// CreateOrUpdate 创建或更新VNS首页数据
	CreateOrUpdate(ctx context.Context, tx gdb.TX, data *vnsEntity.UserVnsHomeDataEntity) error
}

// userVnsHomeDataRepository VNS首页数据仓储实现
type userVnsHomeDataRepository struct {
	homeDataDao vnsDao.IUserVnsHomeDataDao
}

// NewUserVnsHomeDataRepository 创建VNS首页数据仓储实例
func NewUserVnsHomeDataRepository() IUserVnsHomeDataRepository {
	return &userVnsHomeDataRepository{
		homeDataDao: vnsDao.NewUserVnsHomeDataDao(),
	}
}

// GetByUserID 根据用户ID获取VNS首页数据（如果不存在则创建）
func (r *userVnsHomeDataRepository) GetByUserID(ctx context.Context, userID int64) (*vnsEntity.UserVnsHomeDataEntity, error) {
	// 先查询
	data, err := r.homeDataDao.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 如果不存在，则创建默认记录
	if data == nil {
		data = &vnsEntity.UserVnsHomeDataEntity{
			UserId:                   userID,
			VipLevel:                 0,
			AdminLevel:               0,
			TeamTotalPerformance:     0,
			NextLevelUpgradeRequired: 0,
			TeamMemberCount:          0,
			DirectMemberCount:        0,
			MaxDistrictCount:         0,
			MinDistrictCount:         0,
			MinDistrictPerformance:   0,
			RewardShare:              0,
			VsBalance:                0,
			UsdtBalance:              0,
		}
		if err := r.homeDataDao.Create(ctx, nil, data); err != nil {
			return nil, err
		}
	}

	return data, nil
}

func (r *userVnsHomeDataRepository) GetBaseFieldsByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*vnsEntity.UserVnsHomeDataEntity, error) {
	return r.homeDataDao.GetBaseFieldsByUserIDs(ctx, userIDs)
}

// CreateOrUpdate 创建或更新VNS首页数据
func (r *userVnsHomeDataRepository) CreateOrUpdate(ctx context.Context, tx gdb.TX, data *vnsEntity.UserVnsHomeDataEntity) error {
	return r.homeDataDao.Upsert(ctx, tx, data)
}
