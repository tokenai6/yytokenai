package reward

import (
	"context"
	"time"

	"XWFrame/internal/dao"
	rewardDao "XWFrame/internal/dao/reward"
	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/consts"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// QuotaUpdate 用户额度更新信息
type QuotaUpdate struct {
	UserID            int64
	NewRemainingQuota decimal.Decimal
}

// ValidUserInfo 有效用户信息
type ValidUserInfo struct {
	UserID int64
	Quota  *rewardEntity.UserQuotaEntity
}

// IReferralRewardRepository 推荐奖励仓储接口（仅提供数据访问）
type IReferralRewardRepository interface {
	// GetValidUsers 获取有效用户（个人业绩≥100，未出局）
	GetValidUsers(ctx context.Context, date time.Time) ([]ValidUserInfo, error)

	// BatchCheckExists 批量检查幂等性
	BatchCheckExists(ctx context.Context, userIDs []int64, date time.Time) (map[int64]bool, error)

	// Get30LayerReferralsByWalletAddress 获取30层推荐关系
	Get30LayerReferralsByWalletAddress(ctx context.Context, userID int64) (map[int][]int64, error)

	// GetBatchUserSumByBusiness 批量查询用户业务金额总和
	GetBatchUserSumByBusiness(ctx context.Context, userIDs []int64, businessType string, date time.Time) (map[int64]decimal.Decimal, error)

	// GetPerformanceByUserIDs 批量查询用户业绩
	GetPerformanceByUserIDs(ctx context.Context, userIDs []int64, date time.Time) (map[int64]*rewardEntity.UserPerformanceEntity, error)

	// BatchExecuteReferralReward 批量执行推荐奖励（创建资产记录和更新额度）
	BatchExecuteReferralReward(ctx context.Context, assetRecords []*rewardEntity.AssetRecordEntity, quotaUpdates []QuotaUpdate) error
}

// referralRewardRepository 推荐奖励仓储实现
type referralRewardRepository struct {
	userDao        dao.IUserDao
	quotaRepo      IQuotaRepository
	assetRecordDao rewardDao.IAssetRecordDao
	performanceDao rewardDao.IUserPerformanceDao
}

// NewReferralRewardRepository 创建推荐奖励仓储实例
func NewReferralRewardRepository() IReferralRewardRepository {
	return &referralRewardRepository{
		userDao:        dao.NewUserDao(),
		quotaRepo:      NewQuotaRepository(),
		assetRecordDao: rewardDao.NewAssetRecordDao(),
		performanceDao: rewardDao.NewUserPerformanceDao(),
	}
}

// GetValidUsers 获取有效用户
func (r *referralRewardRepository) GetValidUsers(ctx context.Context, date time.Time) ([]ValidUserInfo, error) {
	// 获取所有个人业绩≥100的用户
	performances, err := r.performanceDao.GetByDate(ctx, date)
	if err != nil {
		return nil, err
	}

	validUserIDs := make([]int64, 0)
	validUserMap := make(map[int64]bool)

	for _, perf := range performances {
		if perf.PersonalPerformance.GreaterThanOrEqual(consts.GetMinPerformanceThreshold()) {
			validUserIDs = append(validUserIDs, perf.UserID)
			validUserMap[perf.UserID] = true
		}
	}

	if len(validUserIDs) == 0 {
		return []ValidUserInfo{}, nil
	}

	// 批量查询用户额度
	quotaMap, err := r.quotaRepo.GetByUserIDs(ctx, validUserIDs)
	if err != nil {
		return nil, err
	}

	var validUsers []ValidUserInfo
	for _, userID := range validUserIDs {
		quota, exists := quotaMap[userID]
		if exists && quota != nil && quota.RemainingQuota.GreaterThan(decimal.Zero) {
			validUsers = append(validUsers, ValidUserInfo{
				UserID: userID,
				Quota:  quota,
			})
		}
	}

	return validUsers, nil
}

// BatchCheckExists 批量检查幂等性
func (r *referralRewardRepository) BatchCheckExists(ctx context.Context, userIDs []int64, date time.Time) (map[int64]bool, error) {
	return r.assetRecordDao.BatchCheckExists(ctx, userIDs, date, consts.AssetBusinessTypeRewardReferral)
}

// Get30LayerReferralsByWalletAddress 获取30层推荐关系
func (r *referralRewardRepository) Get30LayerReferralsByWalletAddress(ctx context.Context, userID int64) (map[int][]int64, error) {
	return r.userDao.Get30LayerReferralsByWalletAddress(ctx, userID)
}

// GetBatchUserSumByBusiness 批量查询用户业务金额总和
func (r *referralRewardRepository) GetBatchUserSumByBusiness(ctx context.Context, userIDs []int64, businessType string, date time.Time) (map[int64]decimal.Decimal, error) {
	return r.assetRecordDao.GetBatchUserSumByBusiness(ctx, userIDs, businessType, 0, date)
}

// GetPerformanceByUserIDs 批量查询用户业绩（自动分批处理，避免内存溢出）
func (r *referralRewardRepository) GetPerformanceByUserIDs(ctx context.Context, userIDs []int64, date time.Time) (map[int64]*rewardEntity.UserPerformanceEntity, error) {
	if len(userIDs) == 0 {
		return make(map[int64]*rewardEntity.UserPerformanceEntity), nil
	}

	const maxBatchSize = 1000 // 每批次最多1000个用户，避免内存溢出
	perfMap := make(map[int64]*rewardEntity.UserPerformanceEntity)

	if len(userIDs) > maxBatchSize {
		// 分批查询
		totalBatches := (len(userIDs) + maxBatchSize - 1) / maxBatchSize
		g.Log().Infof(ctx, "[ReferralRewardRepository] 批量查询用户业绩，数量 %d 超过限制，将分 %d 批处理", len(userIDs), totalBatches)

		for i := 0; i < len(userIDs); i += maxBatchSize {
			end := i + maxBatchSize
			if end > len(userIDs) {
				end = len(userIDs)
			}
			batch := userIDs[i:end]
			batchIndex := i/maxBatchSize + 1

			performances, err := r.performanceDao.GetByUserIDs(ctx, batch, date)
			if err != nil {
				return nil, gerror.Wrapf(err, "批量查询用户业绩失败（第 %d/%d 批）", batchIndex, totalBatches)
			}

			// 合并结果
			for _, perf := range performances {
				perfMap[perf.UserID] = perf
			}
		}
	} else {
		// 数据量不大，直接查询
		performances, err := r.performanceDao.GetByUserIDs(ctx, userIDs, date)
		if err != nil {
			return nil, gerror.Wrapf(err, "批量查询用户业绩失败，用户数量: %d", len(userIDs))
		}

		for _, perf := range performances {
			perfMap[perf.UserID] = perf
		}
	}

	return perfMap, nil
}

// BatchExecuteReferralReward 批量执行推荐奖励
func (r *referralRewardRepository) BatchExecuteReferralReward(ctx context.Context, assetRecords []*rewardEntity.AssetRecordEntity, quotaUpdates []QuotaUpdate) error {
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 批量创建资产记录
		if err := r.assetRecordDao.BatchCreate(ctx, tx, assetRecords); err != nil {
			return err
		}

		// 批量更新额度（使用DeductQuota保证一致性）
		// for _, update := range quotaUpdates {
		// 	quota, err := r.quotaRepo.GetForUpdate(ctx, tx, update.UserID)
		// 	if err != nil {
		// 		return err
		// 	}
		// 	deductAmount := quota.RemainingQuota.Sub(update.NewRemainingQuota)
		// 	if deductAmount.GreaterThan(decimal.Zero) {
		// 		if err := r.quotaRepo.DeductQuota(ctx, tx, update.UserID, deductAmount, consts.ChangeTypeRewardReferral, consts.AssetBusinessTypeRewardReferral, "推荐奖励", 0); err != nil {
		// 			return err
		// 		}
		// 	}
		// }

		return nil
	})
}
