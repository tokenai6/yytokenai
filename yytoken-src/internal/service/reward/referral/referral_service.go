package referral

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/repository"
	rewardRepo "XWFrame/internal/repository/reward"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// QuotaUpdate 用户额度更新信息
type QuotaUpdate struct {
	UserID            int64
	NewRemainingQuota decimal.Decimal
}

// LayerStaticReward 层级静态收益详情
type LayerStaticReward struct {
	UserID int64           `json:"user_id"`
	Amount decimal.Decimal `json:"amount"`
}

// IReferralService 推荐奖励服务接口
type IReferralService interface {
	// DistributeReferralReward 发放推荐奖励
	DistributeReferralReward(ctx context.Context, date time.Time) error
}

// referralService 推荐奖励服务实现
type referralService struct {
	referralRepo       rewardRepo.IReferralRewardRepository
	userRepo           repository.IUserRepository
	stakingPackageRepo rewardRepo.IStakingPackageRepository
}

// NewReferralService 创建推荐奖励服务实例
func NewReferralService() IReferralService {
	return &referralService{
		referralRepo:       rewardRepo.NewReferralRewardRepository(),
		userRepo:           repository.NewUserRepository(),
		stakingPackageRepo: rewardRepo.NewStakingPackageRepository(),
	}
}

// DistributeReferralReward 发放推荐奖励（业务逻辑层）
func (s *referralService) DistributeReferralReward(ctx context.Context, date time.Time) error {
	g.Log().Infof(ctx, "[ReferralReward] 开始发放推荐奖励，日期: %s", date.Format(consts.TimeFormatDate))

	// 1. 获取有效用户
	validUsers, err := s.referralRepo.GetValidUsers(ctx, date)
	if err != nil {
		g.Log().Errorf(ctx, "[ReferralReward] 查询有效用户失败: %v", err)
		return err
	}

	if len(validUsers) == 0 {
		g.Log().Info(ctx, "[ReferralReward] 没有有效用户，跳过推荐奖励发放")
		return nil
	}

	g.Log().Infof(ctx, "[ReferralReward] 找到 %d 个有效用户", len(validUsers))

	// 2. 提取用户ID列表用于幂等性检查
	userIDs := make([]int64, len(validUsers))
	for i, user := range validUsers {
		userIDs[i] = user.UserID
	}

	// 3. 批量检查幂等性（分批处理）
	existsMap, err := s.batchCheckIdempotency(ctx, userIDs, date)
	if err != nil {
		return err
	}

	// 4. 批量处理推荐奖励
	var allAssetRecords []*rewardEntity.AssetRecordEntity
	var allQuotaUpdates []QuotaUpdate
	totalProcessed := 0
	totalReward := decimal.Zero
	totalOutOfGame := 0

	for _, user := range validUsers {
		// 检查幂等性
		if existsMap[user.UserID] {
			g.Log().Debugf(ctx, "[ReferralReward] 用户 %d 在时刻 %s 的推荐奖励已发放，跳过", user.UserID, date.Format(consts.TimeFormatDateTime))
			continue
		}

		// 计算用户推荐奖励
		userReward, userCount, layerStaticRewards, validDirectReferrals, layerCount, teamTotalPerformance, layerValidPerformance, err := s.calculateUserReferralReward(ctx, user.UserID, date)
		if err != nil {
			g.Log().Errorf(ctx, "[ReferralReward] 计算用户 %d 推荐奖励失败: %v", user.UserID, err)
			continue
		}

		if userReward.IsZero() {
			clearMap(layerStaticRewards)
			continue
		}

		// 创建资金记录
		assetRecord := s.createReferralAssetRecord(user.UserID, userReward, date, userCount, layerStaticRewards, validDirectReferrals, layerCount, teamTotalPerformance, layerValidPerformance)
		allAssetRecords = append(allAssetRecords, assetRecord)

		clearMap(layerStaticRewards)

		// 记录额度更新
		newRemainingQuota := user.Quota.RemainingQuota.Sub(userReward)
		allQuotaUpdates = append(allQuotaUpdates, QuotaUpdate{
			UserID:            user.UserID,
			NewRemainingQuota: newRemainingQuota,
		})

		totalProcessed++
		totalReward = totalReward.Add(userReward)

		// 检查是否出局
		if newRemainingQuota.LessThanOrEqual(decimal.Zero) {
			totalOutOfGame++
		}
	}

	if len(allAssetRecords) == 0 {
		g.Log().Info(ctx, "[ReferralReward] 没有需要发放的推荐奖励")
		return nil
	}

	// 5. 任务完成后清理缓存（使用 defer 确保即使发生错误也会清理）
	defer func() {
		// 清空所有缓存
		validUsers = nil
		userIDs = nil
		allAssetRecords = nil
		allQuotaUpdates = nil
		clearMap(existsMap)

		g.Log().Debugf(ctx, "[ReferralReward] 缓存已清理")
	}()

	// 6. 批量执行数据库操作
	// 转换QuotaUpdate类型
	repoQuotaUpdates := make([]rewardRepo.QuotaUpdate, len(allQuotaUpdates))
	for i, update := range allQuotaUpdates {
		repoQuotaUpdates[i] = rewardRepo.QuotaUpdate{
			UserID:            update.UserID,
			NewRemainingQuota: update.NewRemainingQuota,
		}
	}

	if err := s.referralRepo.BatchExecuteReferralReward(ctx, allAssetRecords, repoQuotaUpdates); err != nil {
		g.Log().Errorf(ctx, "[ReferralReward] 批量执行推荐奖励失败: %v", err)
		return err
	}

	g.Log().Infof(ctx, "[ReferralReward] 推荐奖励发放完成 - 处理用户: %d, 发放金额: %s, 出局用户: %d",
		totalProcessed, totalReward.String(), totalOutOfGame)

	return nil
}

// batchCheckIdempotency 批量检查幂等性（分批处理）
func (s *referralService) batchCheckIdempotency(ctx context.Context, userIDs []int64, date time.Time) (map[int64]bool, error) {
	existsMap := make(map[int64]bool)
	const maxBatchSize = 30000

	if len(userIDs) > maxBatchSize {
		for i := 0; i < len(userIDs); i += maxBatchSize {
			end := i + maxBatchSize
			if end > len(userIDs) {
				end = len(userIDs)
			}
			batch := userIDs[i:end]

			batchExistsMap, err := s.referralRepo.BatchCheckExists(ctx, batch, date)
			if err != nil {
				return nil, err
			}

			for userID, exists := range batchExistsMap {
				existsMap[userID] = exists
			}
		}
	} else {
		var err error
		existsMap, err = s.referralRepo.BatchCheckExists(ctx, userIDs, date)
		if err != nil {
			return nil, err
		}
	}

	return existsMap, nil
}

// calculateUserReferralReward 计算用户推荐奖励（业务逻辑）
func (s *referralService) calculateUserReferralReward(ctx context.Context, userID int64, date time.Time) (decimal.Decimal, int, map[int][]LayerStaticReward, int, int, decimal.Decimal, decimal.Decimal, error) {
	// 1. 获取30层推荐关系
	layerMap, err := s.referralRepo.Get30LayerReferralsByWalletAddress(ctx, userID)
	if err != nil {
		return decimal.Zero, 0, nil, 0, 0, decimal.Zero, decimal.Zero, err
	}

	if len(layerMap) == 0 {
		// g.Log().Debugf(ctx, "[ReferralReward] 用户 %d 没有推荐下级，跳过", userID)
		return decimal.Zero, 0, nil, 0, 0, decimal.Zero, decimal.Zero, nil
	}

	// 2. 收集所有30层用户ID
	allUserIDs := make([]int64, 0)
	for _, userIDs := range layerMap {
		allUserIDs = append(allUserIDs, userIDs...)
	}

	// 3. 批量查询静态收益
	rawStaticRewards, err := s.referralRepo.GetBatchUserSumByBusiness(ctx, allUserIDs, consts.AssetBusinessTypeRewardStatic, date)
	if err != nil {
		return decimal.Zero, 0, nil, 0, 0, decimal.Zero, decimal.Zero, err
	}

	// 4. 批量查询业绩（检查有效性）
	perfMap, err := s.referralRepo.GetPerformanceByUserIDs(ctx, allUserIDs, date)
	if err != nil {
		return decimal.Zero, 0, nil, 0, 0, decimal.Zero, decimal.Zero, err
	}

	// 5. 获取所有下级用户ID（用于计算团队总业绩）
	allDescendantIDs, err := s.userRepo.GetAllDescendantIDs(ctx, userID)
	if err != nil {
		return decimal.Zero, 0, nil, 0, 0, decimal.Zero, decimal.Zero, err
	}

	// 6. 计算团队总业绩（所有下级的质押金额，status=1）
	var teamTotalPerformance decimal.Decimal
	if len(allDescendantIDs) > 0 {
		teamTotalPerformance, err = s.stakingPackageRepo.SumStakeAmountByUserIDsWithStatus(ctx, allDescendantIDs)
		if err != nil {
			return decimal.Zero, 0, nil, 0, 0, decimal.Zero, decimal.Zero, err
		}
	}

	// 7. 计算层级奖励有效业绩（rawStaticRewards 中所有用户ID的质押金额，status=1）
	var layerValidPerformance decimal.Decimal
	if len(rawStaticRewards) > 0 {
		layerValidUserIDs := make([]int64, 0, len(rawStaticRewards))
		for uid := range rawStaticRewards {
			layerValidUserIDs = append(layerValidUserIDs, uid)
		}
		layerValidPerformance, err = s.stakingPackageRepo.SumStakeAmountByUserIDsWithStatus(ctx, layerValidUserIDs)
		if err != nil {
			return decimal.Zero, 0, nil, 0, 0, decimal.Zero, decimal.Zero, err
		}
	}

	// 8. 方法返回前清理局部缓存
	defer func() {
		clearMap(layerMap)
		clearMap(perfMap)
		clearMap(rawStaticRewards)
		allUserIDs = nil
		allDescendantIDs = nil
	}()

	// 9. 按层级计算奖励
	totalReward := decimal.Zero
	layerStaticRewards := make(map[int][]LayerStaticReward, len(layerMap))
	validDirectReferrals := 0

	for level := 1; level <= 30; level++ {
		userIDs, exists := layerMap[level]
		if !exists {
			continue
		}

		layerStaticTotal := decimal.Zero
		layerDetails := make([]LayerStaticReward, 0, len(userIDs))

		for _, uid := range userIDs {
			// 检查下级是否有效（个人业绩≥100）
			perf, exists := perfMap[uid]
			if !exists || perf.PersonalPerformance.LessThan(consts.GetMinPerformanceThreshold()) {
				continue
			}

			// 累加该层静态收益
			if staticReward, exists := rawStaticRewards[uid]; exists && !staticReward.IsZero() {
				layerStaticTotal = layerStaticTotal.Add(staticReward)
				layerDetails = append(layerDetails, LayerStaticReward{
					UserID: uid,
					Amount: staticReward,
				})

				// 统计有效直推用户数量（仅第一层）
				if level == 1 {
					validDirectReferrals++
				}
			}
		}

		if layerStaticTotal.IsZero() || len(layerDetails) == 0 {
			continue
		}

		if level <= validDirectReferrals {
			// 计算该层推荐奖励（1%）
			layerReward := layerStaticTotal.Mul(consts.GetReferralRewardRate())
			totalReward = totalReward.Add(layerReward)
			layerStaticRewards[level] = layerDetails
		}
	}

	// 10. 计算拿了多少层
	layerCount := len(layerStaticRewards)

	return totalReward, len(allUserIDs), layerStaticRewards, validDirectReferrals, layerCount, teamTotalPerformance, layerValidPerformance, nil
}

// createReferralAssetRecord 创建推荐奖励资金记录（业务逻辑）
func (s *referralService) createReferralAssetRecord(userID int64, totalReward decimal.Decimal, date time.Time, userCount int, layerStaticRewards map[int][]LayerStaticReward, validDirectReferrals int, layerCount int, teamTotalPerformance decimal.Decimal, layerValidPerformance decimal.Decimal) *rewardEntity.AssetRecordEntity {
	var staticRewardsData map[string][]LayerStaticReward
	if len(layerStaticRewards) > 0 {
		staticRewardsData = make(map[string][]LayerStaticReward, len(layerStaticRewards))
		for layer, rewards := range layerStaticRewards {
			if len(rewards) == 0 {
				continue
			}
			staticRewardsData[strconv.Itoa(layer)] = rewards
		}
	}

	metadata := map[string]interface{}{
		"total_reward":            totalReward.String(),
		"reward_type":             "referral",
		"user_count":              userCount,
		"static_rewards":          staticRewardsData,
		"valid_direct_referrals":  validDirectReferrals,
		"layer_count":             layerCount,
		"team_total_performance":  teamTotalPerformance.String(),
		"layer_valid_performance": layerValidPerformance.String(),
	}
	metadataJSON, _ := json.Marshal(metadata)

	return &rewardEntity.AssetRecordEntity{
		UserID:       userID,
		AssetType:    consts.AssetTypeAPGUserBalance,
		RecordTime:   date,
		Amount:       totalReward,
		FlowType:     consts.AssetFlowTypeIncome,
		Status:       consts.AssetRecordStatusPending,
		BusinessType: consts.AssetBusinessTypeRewardReferral,
		BusinessID:   0,
		Remark:       "推荐奖励",
		Metadata:     string(metadataJSON),
	}
}

// clearMap 清空 map（辅助函数，用于释放内存）
func clearMap[K comparable, V any](m map[K]V) {
	if m == nil {
		return
	}
	for k := range m {
		delete(m, k)
	}
}
