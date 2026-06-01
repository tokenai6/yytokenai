package reward

import (
	"context"

	rewardDao "XWFrame/internal/dao/reward"
	rewardEntity "XWFrame/internal/entity/reward"
)

// IUserVipLevelRepository 用户VIP等级仓储接口
type IUserVipLevelRepository interface {
	// GetUserVipLevelByOffset 按offset获取用户VIP等级记录
	GetUserVipLevelByOffset(ctx context.Context, userID int64, offset *int) ([]*rewardEntity.UserVipLevelEntity, error)

	// GetCurrentVipLevelsByUserIDs 根据用户ID列表获取当前VIP等级记录
	GetCurrentVipLevelsByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*rewardEntity.UserVipLevelEntity, error)
}

// userVipLevelRepository 用户VIP等级仓储实现
type userVipLevelRepository struct {
	vipDao rewardDao.IUserVipLevelDao
}

// NewUserVipLevelRepository 创建用户VIP等级仓储实例
func NewUserVipLevelRepository() IUserVipLevelRepository {
	return &userVipLevelRepository{
		vipDao: rewardDao.NewUserVipLevelDao(),
	}
}

// GetUserVipLevelByOffset 按offset获取用户VIP等级记录
func (r *userVipLevelRepository) GetUserVipLevelByOffset(ctx context.Context, userID int64, offset *int) ([]*rewardEntity.UserVipLevelEntity, error) {
	return r.vipDao.GetUserVipLevelByOffset(ctx, userID, offset)
}

// GetCurrentVipLevelsByUserIDs 根据用户ID列表获取当前VIP等级记录
func (r *userVipLevelRepository) GetCurrentVipLevelsByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*rewardEntity.UserVipLevelEntity, error) {
	if len(userIDs) == 0 {
		return map[int64]*rewardEntity.UserVipLevelEntity{}, nil
	}

	records, err := r.vipDao.GetCurrentVipLevelsByUserIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	result := make(map[int64]*rewardEntity.UserVipLevelEntity, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		if existing, ok := result[record.UserID]; ok {
			if record.RecordTime.After(existing.RecordTime) {
				result[record.UserID] = record
			}
			continue
		}
		result[record.UserID] = record
	}

	return result, nil
}
