package reward

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"XWFrame/internal/dao"
	rewardDao "XWFrame/internal/dao/reward"
	vnsDao "XWFrame/internal/dao/vns"
	"XWFrame/internal/entity"
	rewardEntity "XWFrame/internal/entity/reward"
	vnsEntity "XWFrame/internal/entity/vns"
	"XWFrame/internal/frame/consts"
	balanceModel "XWFrame/internal/service/balance/model"
	repo "XWFrame/internal/repository"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// ITeamDifferentialRepository 团队极差奖励仓储接口
type ITeamDifferentialRepository interface {
	// GetStaticRewardsByDate 获取指定日期的静态收益记录
	GetStaticRewardsByDate(ctx context.Context, date time.Time) (map[int64]decimal.Decimal, error)

	// BatchCheckExists 批量检查幂等性
	BatchCheckExists(ctx context.Context, userIDs []int64, date time.Time) (map[int64]bool, error)

	// GetAncestorsByUserIDs 批量获取祖先链
	GetAncestorsByUserIDs(ctx context.Context, userIDs []int64) (map[int64][]int64, error)

	// GetVnsHomeDataByUserIDs 批量获取用户VNS首页数据（等级信息）
	GetVnsHomeDataByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*vnsEntity.UserVnsHomeDataEntity, error)

	// BatchExecuteTeamReward 批量执行奖励发放
	BatchExecuteTeamReward(ctx context.Context, assetRecords []*rewardEntity.AssetRecordEntity) error
}

type teamDifferentialRepository struct {
	userDao            dao.IUserDao
	assetRecordDao     rewardDao.IAssetRecordDao
	userVnsHomeDataDao vnsDao.IUserVnsHomeDataDao
	balanceDao          dao.IAccountBalanceDao
	balanceChangeLogDao dao.IBalanceChangeLogDao
	accountTypeRepo     repo.IAccountTypeRepository
}

func NewTeamDifferentialRepository() ITeamDifferentialRepository {
	return &teamDifferentialRepository{
		userDao:            dao.NewUserDao(),
		assetRecordDao:     rewardDao.NewAssetRecordDao(),
		userVnsHomeDataDao: vnsDao.NewUserVnsHomeDataDao(),
		balanceDao:          dao.NewAccountBalanceDao(),
		balanceChangeLogDao: dao.NewBalanceChangeLogDao(),
		accountTypeRepo:     repo.NewAccountTypeRepository(),
	}
}

func (r *teamDifferentialRepository) GetStaticRewardsByDate(ctx context.Context, date time.Time) (map[int64]decimal.Decimal, error) {
	return r.assetRecordDao.GetAllSumsByBusiness(ctx, consts.AssetBusinessTypeRewardStatic, 0, date)
}

func (r *teamDifferentialRepository) BatchCheckExists(ctx context.Context, userIDs []int64, date time.Time) (map[int64]bool, error) {
	result := make(map[int64]bool)
	if len(userIDs) == 0 {
		return result, nil
	}

	// 分批 IN，避免参数过大/查询过慢
	const batchSize = 5000
	for i := 0; i < len(userIDs); i += batchSize {
		end := i + batchSize
		if end > len(userIDs) {
			end = len(userIDs)
		}
		batch := userIDs[i:end]

		existsMap, err := r.assetRecordDao.BatchCheckExists(ctx, batch, date, consts.AssetBusinessTypeTeamReward)
		if err != nil {
			return nil, err
		}
		for uid, exists := range existsMap {
			result[uid] = exists
		}
	}

	// 为所有用户ID设置默认值（不存在）
	for _, uid := range userIDs {
		if _, ok := result[uid]; !ok {
			result[uid] = false
		}
	}

	return result, nil
}

func (r *teamDifferentialRepository) GetAncestorsByUserIDs(ctx context.Context, userIDs []int64) (map[int64][]int64, error) {
	return r.userDao.GetAncestorsByUserIDs(ctx, userIDs)
}

func (r *teamDifferentialRepository) GetVnsHomeDataByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*vnsEntity.UserVnsHomeDataEntity, error) {
	return r.userVnsHomeDataDao.GetBaseFieldsByUserIDs(ctx, userIDs)
}

func (r *teamDifferentialRepository) BatchExecuteTeamReward(ctx context.Context, assetRecords []*rewardEntity.AssetRecordEntity) error {
	if len(assetRecords) == 0 {
		return nil
	}
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 获取APG用户余额账户类型ID
		apgAccountTypeID := r.accountTypeRepo.GetAPGUserBalanceID(ctx)

		// 1. 写入资金流水（直接置为已结算，保证“立即入账”体验与静态收益一致）
		for _, record := range assetRecords {
			record.Status = consts.AssetRecordStatusSettled
		}
		if err := r.assetRecordDao.BatchCreate(ctx, tx, assetRecords); err != nil {
			return err
		}

		// 2. 按用户汇总入账，并写账变
		userRewards := make(map[int64]decimal.Decimal)
		recordTime := assetRecords[0].RecordTime
		for _, record := range assetRecords {
			userRewards[record.UserID] = userRewards[record.UserID].Add(record.Amount)
		}

		for userID, totalReward := range userRewards {
			if totalReward.LessThanOrEqual(decimal.Zero) {
				continue
			}

			// 查询或创建账户余额（带悲观锁）
			existingBalance, err := r.balanceDao.GetByUserAndAccountTypeForUpdate(ctx, tx, userID, apgAccountTypeID)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("查询用户 %d 账户余额失败: %w", userID, err)
			}

			var beforeBalance, afterBalance decimal.Decimal
			if existingBalance == nil {
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

			orderNo := fmt.Sprintf("TEAM_DIFF_%s_%d", recordTime.Format("20060102"), userID)
			if len(orderNo) > 64 {
				orderNo = orderNo[len(orderNo)-64:]
			}

			changeLog := &balanceModel.BalanceChangeLog{
				UserId:         userID,
				AccountTypeId:  apgAccountTypeID,
				Symbol:         "APG",
				ChangeType:     consts.ChangeTypeRewardSettlement,
				Amount:         totalReward,
				BeforeBalance:  beforeBalance,
				AfterBalance:   afterBalance,
				RelatedOrderNo: orderNo,
				RelatedId:      0,
				Remark:         fmt.Sprintf("团队极差奖励（%s）", recordTime.Format(consts.TimeFormatDate)),
				OperatorId:     0,
				OperatorType:   consts.OperatorTypeSystem,
			}
			if err := r.balanceChangeLogDao.CreateLog(ctx, tx, changeLog); err != nil {
				return fmt.Errorf("创建用户 %d 账变记录失败: %w", userID, err)
			}
		}

		return nil
	})
}
