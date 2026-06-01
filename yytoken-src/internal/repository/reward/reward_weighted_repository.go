package reward

import (
	"context"
	"time"

	rewardDao "XWFrame/internal/dao/reward"
	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/consts"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// IRewardWeightedRepository 加权奖励仓储接口（仅提供数据访问）
type IRewardWeightedRepository interface {
	// GetDailySumByBusiness 获取指定日期业务类型的总和
	GetDailySumByBusiness(ctx context.Context, date time.Time, businessType string) (decimal.Decimal, error)

	// GetTopByNewDirectPerformance 获取直推新增业绩前N名（limit<=0 表示不限制）
	GetTopByNewDirectPerformance(ctx context.Context, date time.Time, limit int) ([]*rewardEntity.UserPerformanceEntity, error)

	// GetNewDirectPerformanceSum 计算用户直推新增业绩总和
	GetNewDirectPerformanceSum(ctx context.Context, userIDs []int64, date time.Time) (decimal.Decimal, error)

	// BatchGetQuotas 批量获取用户额度
	BatchGetQuotas(ctx context.Context, userIDs []int64) (map[int64]*rewardEntity.UserQuotaEntity, error)

	// BatchCheckExists 批量检查幂等性
	BatchCheckExists(ctx context.Context, userIDs []int64, date time.Time) (map[int64]bool, error)

	// BatchCreateAssetRecords 批量创建资产记录
	BatchCreateAssetRecords(ctx context.Context, records []*rewardEntity.AssetRecordEntity) error
}

// rewardWeightedRepository 加权奖励仓储实现
type rewardWeightedRepository struct {
	assetRecordDao rewardDao.IAssetRecordDao
	performanceDao rewardDao.IUserPerformanceDao
	quotaDao       rewardDao.IUserQuotaDao
}

// NewRewardWeightedRepository 创建加权奖励仓储实例
func NewRewardWeightedRepository() IRewardWeightedRepository {
	return &rewardWeightedRepository{
		assetRecordDao: rewardDao.NewAssetRecordDao(),
		performanceDao: rewardDao.NewUserPerformanceDao(),
		quotaDao:       rewardDao.NewUserQuotaDao(),
	}
}

// GetDailySumByBusiness 获取指定日期业务类型的总和
func (r *rewardWeightedRepository) GetDailySumByBusiness(ctx context.Context, date time.Time, businessType string) (decimal.Decimal, error) {
	return r.assetRecordDao.GetDailySumByBusiness(ctx, date, businessType, 0)
}

// GetTopByNewDirectPerformance 获取直推新增业绩前N名（limit<=0 表示不限制）
func (r *rewardWeightedRepository) GetTopByNewDirectPerformance(ctx context.Context, date time.Time, limit int) ([]*rewardEntity.UserPerformanceEntity, error) {
	return r.performanceDao.GetTopByNewDirectPerformance(ctx, date, limit)
}

// GetNewDirectPerformanceSum 计算用户直推新增业绩总和
func (r *rewardWeightedRepository) GetNewDirectPerformanceSum(ctx context.Context, userIDs []int64, date time.Time) (decimal.Decimal, error) {
	return r.performanceDao.GetNewDirectPerformanceSum(ctx, userIDs, date)
}

// BatchGetQuotas 批量获取用户额度
func (r *rewardWeightedRepository) BatchGetQuotas(ctx context.Context, userIDs []int64) (map[int64]*rewardEntity.UserQuotaEntity, error) {
	quotas, err := r.quotaDao.BatchGetByUserIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	quotaMap := make(map[int64]*rewardEntity.UserQuotaEntity)
	for _, quota := range quotas {
		quotaMap[quota.UserID] = quota
	}
	return quotaMap, nil
}

// BatchCheckExists 批量检查幂等性
func (r *rewardWeightedRepository) BatchCheckExists(ctx context.Context, userIDs []int64, date time.Time) (map[int64]bool, error) {
	return r.assetRecordDao.BatchCheckExists(ctx, userIDs, date, consts.AssetBusinessTypeRewardWeighted)
}

// BatchCreateAssetRecords 批量创建资产记录
func (r *rewardWeightedRepository) BatchCreateAssetRecords(ctx context.Context, records []*rewardEntity.AssetRecordEntity) error {
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		return r.assetRecordDao.BatchCreate(ctx, tx, records)
	})
}
