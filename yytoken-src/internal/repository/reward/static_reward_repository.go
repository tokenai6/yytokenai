package reward

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	rewardDao "XWFrame/internal/dao/reward"
	"XWFrame/internal/dao"
	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/entity"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/service/balance/model"
	repo "XWFrame/internal/repository"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// PackageUpdate 算力包更新信息
type PackageUpdate struct {
	PackageID          int64
	NewReleasedStatic  decimal.Decimal
	NewDaysElapsed     decimal.Decimal
	NewRemainingDays   decimal.Decimal
	ShouldUpdateMax    bool
	NewMaxStaticRelease decimal.Decimal
	ShouldUpdateStatus bool
	NewStatus          int
}

// IStaticRewardRepository 静态收益仓储接口（仅提供数据访问）
type IStaticRewardRepository interface {
	// GetLastStaticRewardTime 获取最后一次静态收益发放时间
	GetLastStaticRewardTime(ctx context.Context) (time.Time, error)

	// GetActivePackages 获取所有运行中的算力包
	GetActivePackages(ctx context.Context) ([]*rewardEntity.StakingPackageEntity, error)

	// GetActivePackagesByRecordTime 获取运行中的算力包，且开始时间小于等于指定时间（用于静态奖励计算）
	GetActivePackagesByRecordTime(ctx context.Context, recordTime time.Time) ([]*rewardEntity.StakingPackageEntity, error)

	// BatchGetQuotas 批量获取用户额度
	BatchGetQuotas(ctx context.Context, userIDs []int64) (map[int64]*rewardEntity.UserQuotaEntity, error)

	// BatchCheckExists 批量检查幂等性
	BatchCheckExists(ctx context.Context, userIDs []int64, date time.Time) (map[int64]bool, error)

	// BatchExecuteStaticReward 批量执行静态收益（创建资产记录和更新算力包）
	BatchExecuteStaticReward(ctx context.Context, assetRecords []*rewardEntity.AssetRecordEntity, packageUpdates []PackageUpdate) error
}

// staticRewardRepository 静态收益仓储实现
type staticRewardRepository struct {
	stakingDao          rewardDao.IStakingPackageDao
	quotaRepo           IQuotaRepository
	assetRecordDao      rewardDao.IAssetRecordDao
	balanceDao          dao.IAccountBalanceDao
	balanceChangeLogDao dao.IBalanceChangeLogDao
	accountTypeRepo     repo.IAccountTypeRepository
}

// NewStaticRewardRepository 创建静态收益仓储实例
func NewStaticRewardRepository() IStaticRewardRepository {
	return &staticRewardRepository{
		stakingDao:          rewardDao.NewStakingPackageDao(),
		quotaRepo:           NewQuotaRepository(),
		assetRecordDao:      rewardDao.NewAssetRecordDao(),
		balanceDao:          dao.NewAccountBalanceDao(),
		balanceChangeLogDao: dao.NewBalanceChangeLogDao(),
		accountTypeRepo:     repo.NewAccountTypeRepository(),
	}
}

// GetLastStaticRewardTime 获取最后一次静态收益发放时间
func (r *staticRewardRepository) GetLastStaticRewardTime(ctx context.Context) (time.Time, error) {
	return r.assetRecordDao.GetLastRecordTimeByBusinessType(ctx, consts.AssetBusinessTypeRewardStatic)
}

// GetActivePackages 获取所有运行中的算力包
func (r *staticRewardRepository) GetActivePackages(ctx context.Context) ([]*rewardEntity.StakingPackageEntity, error) {
	return r.stakingDao.GetActivePackages(ctx)
}

// GetActivePackagesByRecordTime 获取运行中的算力包，且开始时间小于等于指定时间（用于静态奖励计算）
func (r *staticRewardRepository) GetActivePackagesByRecordTime(ctx context.Context, recordTime time.Time) ([]*rewardEntity.StakingPackageEntity, error) {
	return r.stakingDao.GetActivePackagesByRecordTime(ctx, recordTime)
}

// BatchGetQuotas 批量获取用户额度
func (r *staticRewardRepository) BatchGetQuotas(ctx context.Context, userIDs []int64) (map[int64]*rewardEntity.UserQuotaEntity, error) {
	return r.quotaRepo.GetByUserIDs(ctx, userIDs)
}

// BatchCheckExists 批量检查幂等性
func (r *staticRewardRepository) BatchCheckExists(ctx context.Context, userIDs []int64, date time.Time) (map[int64]bool, error) {
	return r.assetRecordDao.BatchCheckExists(ctx, userIDs, date, consts.AssetBusinessTypeRewardStatic)
}

// BatchExecuteStaticReward 批量执行静态收益（直接更新余额和创建账变记录）
func (r *staticRewardRepository) BatchExecuteStaticReward(ctx context.Context, assetRecords []*rewardEntity.AssetRecordEntity, packageUpdates []PackageUpdate) error {
	g.Log().Infof(ctx, "[BatchExecuteStaticReward] 开始执行 - 资产记录数: %d, 算力包更新数: %d", len(assetRecords), len(packageUpdates))
	
	// 获取APG用户余额账户类型ID
	apgAccountTypeID := r.accountTypeRepo.GetAPGUserBalanceID(ctx)

	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 1. 批量创建资产记录（状态为已结算，分批处理，避免PostgreSQL参数限制65535）
		if len(assetRecords) > 0 {
			// 将状态设置为已结算
			for _, record := range assetRecords {
				record.Status = consts.AssetRecordStatusSettled
			}

			batchSize := consts.MaxBatchSizeRecordInsert
			if batchSize <= 0 || batchSize > 5000 {
				batchSize = 5000
			}

			for i := 0; i < len(assetRecords); i += batchSize {
				end := i + batchSize
				if end > len(assetRecords) {
					end = len(assetRecords)
				}
				if err := r.assetRecordDao.BatchCreate(ctx, tx, assetRecords[i:end]); err != nil {
					return err
				}
			}
		}

		// 2. 批量更新算力包的 released_static
		for _, update := range packageUpdates {
			if err := r.stakingDao.UpdateReleasedStatic(ctx, tx, update.PackageID, update.NewReleasedStatic, update.NewDaysElapsed, update.NewRemainingDays); err != nil {
				return err
			}

			// 一次性释放口径：将 max_static_release 同步为 stake_amount（1倍释放上限）
			if update.ShouldUpdateMax {
				if err := r.stakingDao.UpdateMaxStaticRelease(ctx, tx, update.PackageID, update.NewMaxStaticRelease); err != nil {
					return err
				}
			}

			if update.ShouldUpdateStatus {
				if err := r.stakingDao.UpdatePackageStatus(ctx, tx, update.PackageID, update.NewStatus); err != nil {
					return err
				}
			}
		}

		// 3. 按用户分组，增加余额并创建账变记录
		userRewardsCount := 0
		if len(assetRecords) > 0 {
			userRewards := make(map[int64]decimal.Decimal) // userID -> totalReward
			recordTime := assetRecords[0].RecordTime
			
			for _, record := range assetRecords {
				userRewards[record.UserID] = userRewards[record.UserID].Add(record.Amount)
			}

			// 4. 为每个用户增加余额并创建账变记录
			g.Log().Infof(ctx, "[BatchExecuteStaticReward] 准备为 %d 个用户增加余额", len(userRewards))
			for userID, totalReward := range userRewards {
				userRewardsCount++
				if totalReward.LessThanOrEqual(decimal.Zero) {
					g.Log().Warningf(ctx, "[BatchExecuteStaticReward] 用户 %d 的奖励金额为0或负数，跳过", userID)
					continue
				}
				g.Log().Infof(ctx, "[BatchExecuteStaticReward] 处理用户 %d，奖励金额: %s", userID, totalReward.String())

				// 4.1 查询或创建账户余额（带悲观锁）
				existingBalance, err := r.balanceDao.GetByUserAndAccountTypeForUpdate(ctx, tx, userID, apgAccountTypeID)
				if err != nil && !errors.Is(err, sql.ErrNoRows) {
					return fmt.Errorf("查询用户 %d 账户余额失败: %w", userID, err)
				}

				var beforeBalance, afterBalance decimal.Decimal

				if existingBalance == nil {
					// 账户不存在则创建
					newBalance := &entity.AccountBalanceEntity{
						UserId:        userID,
						AccountTypeId: apgAccountTypeID,
						AccountType:   consts.AssetTypeAPGUserBalance,
						Symbol:        "APG",
						Balance:       totalReward,
						FrozenBalance: decimal.Zero,
						TotalAmount:   totalReward,
						Version:       0,
					}
					if err := r.balanceDao.Create(ctx, tx, newBalance); err != nil {
						return fmt.Errorf("创建用户 %d 账户余额失败: %w", userID, err)
					}
					beforeBalance = decimal.Zero
					afterBalance = totalReward
				} else {
					// 账户已存在则增加余额
					beforeBalance = existingBalance.Balance
					afterBalance = existingBalance.Balance.Add(totalReward)

					updateData := map[string]interface{}{
						"balance":      afterBalance,
						"total_amount": existingBalance.TotalAmount.Add(totalReward),
						"version":      gdb.Raw("version + 1"),
					}
					if err := r.balanceDao.UpdateById(ctx, tx, existingBalance.Id, updateData); err != nil {
						return fmt.Errorf("更新用户 %d 账户余额失败: %w", userID, err)
					}
				}

				// 4.2 创建账变记录
				orderNo := fmt.Sprintf("STATIC_%s_%d", recordTime.Format("20060102"), userID)
				// 处理订单号长度（最多64字符）
				if len(orderNo) > 64 {
					orderNo = orderNo[len(orderNo)-64:]
				}

				changeLog := &model.BalanceChangeLog{
					UserId:         userID,
					AccountTypeId:  apgAccountTypeID,
					Symbol:         "APG",
					ChangeType:     consts.ChangeTypeRewardSettlement,
					Amount:         totalReward,
					BeforeBalance:  beforeBalance,
					AfterBalance:   afterBalance,
					RelatedOrderNo: orderNo,
					RelatedId:      0,
					Remark:         fmt.Sprintf("静态收益释放（%s）", recordTime.Format(consts.TimeFormatDate)),
					OperatorId:     0,
					OperatorType:   consts.OperatorTypeSystem,
				}
				if err := r.balanceChangeLogDao.CreateLog(ctx, tx, changeLog); err != nil {
					return fmt.Errorf("创建用户 %d 账变记录失败: %w", userID, err)
				}
				g.Log().Infof(ctx, "[BatchExecuteStaticReward] 用户 %d 余额更新成功 - 前余额: %s, 后余额: %s, 增加: %s", userID, beforeBalance.String(), afterBalance.String(), totalReward.String())
			}
		}

		g.Log().Infof(ctx, "[BatchExecuteStaticReward] 执行完成 - 资产记录: %d, 算力包更新: %d, 用户余额更新: %d", len(assetRecords), len(packageUpdates), userRewardsCount)
		return nil
	})
}
