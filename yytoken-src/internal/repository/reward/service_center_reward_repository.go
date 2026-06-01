package reward

import (
	"context"
	"time"

	"XWFrame/internal/dao"
	rewardDao "XWFrame/internal/dao/reward"
	"XWFrame/internal/entity"
	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/consts"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// ServiceCenterNode 服务中心节点（用于树形结构计算）
type ServiceCenterNode struct {
	UserID             int64
	Rate               decimal.Decimal
	Children           []*ServiceCenterNode
	RewardBase         decimal.Decimal // 基础奖励金额（计算中）
	RewardAmount       decimal.Decimal // 实际奖励金额（最终）
	TeamNewPerformance decimal.Decimal // 团队新增业绩（缓存）
	Calculated         bool            // 是否已计算完成
}

// IServiceCenterRewardRepository 服务中心奖励仓储接口（仅提供数据访问）
type IServiceCenterRewardRepository interface {
	// GetAllUsers 获取所有活跃用户
	GetAllUsers(ctx context.Context) ([]*entity.UserEntity, error)

	// GetPerformanceByDate 获取指定日期的用户业绩
	GetPerformanceByDate(ctx context.Context, date time.Time) ([]*rewardEntity.UserPerformanceEntity, error)

	// BatchCheckExists 批量检查幂等性
	BatchCheckExists(ctx context.Context, userIDs []int64, date time.Time) (map[int64]bool, error)

	// BatchCreateAssetRecords 批量创建资产记录
	BatchCreateAssetRecords(ctx context.Context, records []*rewardEntity.AssetRecordEntity) error
}

// serviceCenterRewardRepository 服务中心奖励仓储实现
type serviceCenterRewardRepository struct {
	userDao        dao.IUserDao
	assetRecordDao rewardDao.IAssetRecordDao
	performanceDao rewardDao.IUserPerformanceDao
}

// NewServiceCenterRewardRepository 创建服务中心奖励仓储实例
func NewServiceCenterRewardRepository() IServiceCenterRewardRepository {
	return &serviceCenterRewardRepository{
		userDao:        dao.NewUserDao(),
		assetRecordDao: rewardDao.NewAssetRecordDao(),
		performanceDao: rewardDao.NewUserPerformanceDao(),
	}
}

// GetAllUsers 获取所有活跃用户
func (r *serviceCenterRewardRepository) GetAllUsers(ctx context.Context) ([]*entity.UserEntity, error) {
	return r.userDao.GetActiveUsers(ctx)
}

// GetPerformanceByDate 获取指定日期的用户业绩
func (r *serviceCenterRewardRepository) GetPerformanceByDate(ctx context.Context, date time.Time) ([]*rewardEntity.UserPerformanceEntity, error) {
	return r.performanceDao.GetByDate(ctx, date)
}

// BatchCheckExists 批量检查幂等性
func (r *serviceCenterRewardRepository) BatchCheckExists(ctx context.Context, userIDs []int64, date time.Time) (map[int64]bool, error) {
	return r.assetRecordDao.BatchCheckExists(ctx, userIDs, date, consts.AssetBusinessTypeRewardServiceCenter)
}

// BatchCreateAssetRecords 批量创建资产记录
func (r *serviceCenterRewardRepository) BatchCreateAssetRecords(ctx context.Context, records []*rewardEntity.AssetRecordEntity) error {
	filteredRecords := make([]*rewardEntity.AssetRecordEntity, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		if !record.Amount.GreaterThan(decimal.Zero) {
			continue
		}
		filteredRecords = append(filteredRecords, record)
	}

	if len(filteredRecords) == 0 {
		g.Log().Info(ctx, "[服务中心奖励] 过滤后无有效资产记录需要创建")
		return nil
	}

	const batchSize = 1000

	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		for start := 0; start < len(filteredRecords); start += batchSize {
			end := start + batchSize
			if end > len(filteredRecords) {
				end = len(filteredRecords)
			}

			if err := r.assetRecordDao.BatchCreate(ctx, tx, filteredRecords[start:end]); err != nil {
				return err
			}
		}
		return nil
	})
}
