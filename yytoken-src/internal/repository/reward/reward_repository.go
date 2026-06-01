package reward

import (
	"context"
	"time"

	rewardEntity "XWFrame/internal/entity/reward"

	"github.com/shopspring/decimal"
)

// IRewardRepository 奖励仓储接口
type IRewardRepository interface {
	// GetUserLatestRewardRecordTime 获取用户最新的奖励类型record_time
	GetUserLatestRewardRecordTime(ctx context.Context, userID int64, rewardTypes []string) (time.Time, error)

	// GetUserRewardTotalByOffsetGroupByType 按recordTime获取用户奖励类型记录的总金额（按业务类型分组）
	GetUserRewardTotalByOffsetGroupByType(ctx context.Context, userID int64, rewardTypes []string, recordTime time.Time) (map[string]decimal.Decimal, error)

	// GetUserTotalIncome 获取用户累计总收入
	GetUserTotalIncome(ctx context.Context, userID int64, status int) (decimal.Decimal, error)

	// GetPagedAssetRecords 分页获取用户资金记录
	GetPagedAssetRecords(ctx context.Context, userID int64, businessType, recordType string, page, pageSize int) ([]*rewardEntity.AssetRecordEntity, int, error)
}

// rewardRepository 奖励仓储实现
type rewardRepository struct {
	assetRecordRepo IAssetRecordRepository
}

// NewRewardRepository 创建奖励仓储实例
func NewRewardRepository() IRewardRepository {
	return &rewardRepository{
		assetRecordRepo: NewAssetRecordRepository(),
	}
}

// GetUserLatestRewardRecordTime 获取用户最新的奖励类型record_time
func (r *rewardRepository) GetUserLatestRewardRecordTime(ctx context.Context, userID int64, rewardTypes []string) (time.Time, error) {
	return r.assetRecordRepo.GetUserLatestRewardRecordTime(ctx, userID, rewardTypes)
}

// GetUserRewardTotalByOffsetGroupByType 按recordTime获取用户奖励类型记录的总金额（按业务类型分组）
func (r *rewardRepository) GetUserRewardTotalByOffsetGroupByType(ctx context.Context, userID int64, rewardTypes []string, recordTime time.Time) (map[string]decimal.Decimal, error) {
	return r.assetRecordRepo.GetUserRewardTotalByOffsetGroupByType(ctx, userID, rewardTypes, recordTime)
}

// GetUserTotalIncome 获取用户累计总收入
func (r *rewardRepository) GetUserTotalIncome(ctx context.Context, userID int64, status int) (decimal.Decimal, error) {
	return r.assetRecordRepo.GetUserTotalIncome(ctx, userID, status)
}

// GetPagedAssetRecords 分页获取用户资金记录
func (r *rewardRepository) GetPagedAssetRecords(ctx context.Context, userID int64, businessType, recordType string, page, pageSize int) ([]*rewardEntity.AssetRecordEntity, int, error) {
	return r.assetRecordRepo.GetPagedByUserID(ctx, userID, businessType, recordType, page, pageSize)
}
