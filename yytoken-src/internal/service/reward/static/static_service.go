package static

import (
	"context"
	"encoding/json"
	"time"

	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/consts"
	rewardRepo "XWFrame/internal/repository/reward"

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

// PackageRewardResult 算力包收益计算结果
type PackageRewardResult struct {
	Reward              decimal.Decimal // 增量静态收益
	ElapsedSeconds      int64           // 本次增量计算的经过秒数
	TotalElapsedSeconds int64           // 从开始时间到结束时间的总经过秒数（用于计算days_elapsed）
	EndTime             time.Time       // 实际计算的结束时间
	DailyRate           decimal.Decimal // 日收益率
}

// PackageRewardDetail 算力包奖励明细
type PackageRewardDetail struct {
	Package      *rewardEntity.StakingPackageEntity
	RewardResult PackageRewardResult
}

// IStaticService 静态收益服务接口
type IStaticService interface {
	// DistributeStaticReward 发放静态收益
	DistributeStaticReward(ctx context.Context, date time.Time) error
}

// staticService 静态收益服务实现
type staticService struct {
	staticRepo rewardRepo.IStaticRewardRepository
}

// NewStaticService 创建静态收益服务实例
func NewStaticService() IStaticService {
	return &staticService{
		staticRepo: rewardRepo.NewStaticRewardRepository(),
	}
}

// DistributeStaticReward 发放静态收益（业务逻辑层）
func (s *staticService) DistributeStaticReward(ctx context.Context, date time.Time) error {
	g.Log().Infof(ctx, "[StaticReward] 开始发放静态收益，执行时刻: %s", date.Format(consts.TimeFormatDateTime))

	// 1. 获取上次执行时间
	lastRecordTime, err := s.staticRepo.GetLastStaticRewardTime(ctx)
	if err != nil {
		g.Log().Errorf(ctx, "[StaticReward] 查询上次执行时间失败: %v", err)
		return err
	}

	if !lastRecordTime.IsZero() {
		normalizedDate := date.In(lastRecordTime.Location())
		g.Log().Infof(ctx, "[StaticReward] 上次执行时刻: %s，本次执行时刻: %s，时间差: %v",
			lastRecordTime.Format(consts.TimeFormatDateTime),
			normalizedDate.Format(consts.TimeFormatDateTime),
			normalizedDate.Sub(lastRecordTime))
	} else {
		g.Log().Infof(ctx, "[StaticReward] 首次执行，将从各算力包的开始时间开始计算")
	}

	// =========================
	// 【旧逻辑 - 保留（按天释放）】
	// - 条件：status=1 且 start_time <= date（record_time）
	// - 每天固定释放：stake_amount * daily_yield_rate
	// - released_static 逐日累加，达到 max_static_release → status=2（静态完成出局）
	//
	// ⚠️ 需求变更：现阶段要“一次性释放剩余静态收益 + 给上级奖励 + 所有质押记录状态改成3”，
	// 所以旧逻辑主流程先整体注释保留，后续可能会恢复。
	// =========================
	//
	// packages, err := s.staticRepo.GetActivePackagesByRecordTime(ctx, date)
	// if err != nil {
	// 	g.Log().Errorf(ctx, "[StaticReward] 查询运行中算力包失败: %v", err)
	// 	return err
	// }

	// 2. 【新逻辑】获取所有运行中的算力包（一次性释放：覆盖“所有质押记录”口径）
	// 条件：status=1
	packages, err := s.staticRepo.GetActivePackages(ctx)
	if err != nil {
		g.Log().Errorf(ctx, "[StaticReward] 查询运行中算力包失败: %v", err)
		return err
	}

	if len(packages) == 0 {
		g.Log().Info(ctx, "[StaticReward] 没有运行中的算力包，跳过静态收益发放")
		return nil
	}

	g.Log().Infof(ctx, "[StaticReward] 找到 %d 个运行中的算力包", len(packages))

	// 3. 按用户分组
	userPackages := make(map[int64][]*rewardEntity.StakingPackageEntity)
	for _, pkg := range packages {
		userPackages[pkg.UserID] = append(userPackages[pkg.UserID], pkg)
	}

	// 4. 获取用户ID列表
	userIDs := make([]int64, 0, len(userPackages))
	for userID := range userPackages {
		userIDs = append(userIDs, userID)
	}

	// 5. 批量检查幂等性（分批处理避免PostgreSQL参数限制）
	existsMap, err := s.batchCheckIdempotency(ctx, userIDs, date)
	if err != nil {
		return err
	}

	// 6. 计算静态收益
	var allAssetRecords []*rewardEntity.AssetRecordEntity
	var allPackageUpdates []PackageUpdate
	var allPackageRewardDetails []PackageRewardDetail
	totalProcessed := 0
	totalReward := decimal.Zero

	for userID, userPkgs := range userPackages {
		// 计算用户静态收益（一次性释放剩余金额：stake_amount - released_static）
		// - 本次释放金额=总金额-已释放部分，一次性解锁
		// - 同时将所有运行中算力包 status 置为 3（额度耗尽出局）
		userReward, packageUpdates, packageRewardDetails, dailyRate, totalStakeAmount, err := s.calculateUserStaticRewardOneTime(userPkgs, date)
		if err != nil {
			g.Log().Errorf(ctx, "[StaticReward] 计算用户 %d 静态收益失败: %v", userID, err)
			continue
		}

		// 无论是否生成资金记录，都需要更新算力包（释放剩余并置 status=3）
		if len(packageUpdates) > 0 {
			allPackageUpdates = append(allPackageUpdates, packageUpdates...)
		}

		// 检查幂等性：资金记录按用户维度幂等；算力包状态更新不受幂等限制
		if existsMap[userID] {
			g.Log().Debugf(ctx, "[StaticReward] 用户 %d 在时刻 %s 的静态收益资金记录已存在，跳过创建资金记录（仍会更新算力包）", userID, date.Format(consts.TimeFormatDateTime))
			continue
		}

		if userReward.LessThanOrEqual(decimal.Zero) {
			continue
		}

		// 创建资金记录
		assetRecord := s.createAssetRecord(userID, userReward, packageRewardDetails, dailyRate, totalStakeAmount, date)
		allAssetRecords = append(allAssetRecords, assetRecord)
		allPackageRewardDetails = append(allPackageRewardDetails, packageRewardDetails...)

		totalProcessed++
		totalReward = totalReward.Add(userReward)
	}

	if len(allAssetRecords) == 0 && len(allPackageUpdates) == 0 {
		g.Log().Info(ctx, "[StaticReward] 没有需要处理的静态收益/算力包更新")
		return nil
	}

	g.Log().Infof(ctx, "[StaticReward] 准备发放静态收益 - 资产记录数: %d, 算力包更新数: %d", len(allAssetRecords), len(allPackageUpdates))

	// 6.5 任务完成后清理缓存（使用 defer 确保即使发生错误也会清理）
	defer func() {
		// 清空所有缓存
		packages = nil
		userPackages = nil
		userIDs = nil
		allAssetRecords = nil
		allPackageUpdates = nil
		allPackageRewardDetails = nil
		clearMap(existsMap)

		g.Log().Debugf(ctx, "[StaticReward] 缓存已清理")
	}()

	// 7. 批量执行数据库操作
	// 转换PackageUpdate类型
	repoPackageUpdates := make([]rewardRepo.PackageUpdate, len(allPackageUpdates))
	for i, update := range allPackageUpdates {
		repoPackageUpdates[i] = rewardRepo.PackageUpdate{
			PackageID:          update.PackageID,
			NewReleasedStatic:  update.NewReleasedStatic,
			NewDaysElapsed:     update.NewDaysElapsed,
			NewRemainingDays:   update.NewRemainingDays,
			ShouldUpdateMax:    update.ShouldUpdateMax,
			NewMaxStaticRelease: update.NewMaxStaticRelease,
			ShouldUpdateStatus: update.ShouldUpdateStatus,
			NewStatus:          update.NewStatus,
		}
	}

	if err := s.staticRepo.BatchExecuteStaticReward(ctx, allAssetRecords, repoPackageUpdates); err != nil {
		g.Log().Errorf(ctx, "[StaticReward] 批量执行静态收益失败: %v", err)
		return err
	}

	g.Log().Infof(ctx, "[StaticReward] 静态收益发放完成 - 处理用户: %d, 发放金额: %s",
		totalProcessed, totalReward.String())

	return nil
}

// batchCheckIdempotency 批量检查幂等性（不查询额度）
func (s *staticService) batchCheckIdempotency(ctx context.Context, userIDs []int64, date time.Time) (map[int64]bool, error) {
	existsMap := make(map[int64]bool)

	// 分批处理
	if len(userIDs) > consts.MaxBatchSizeUserQuery {
		// 分批检查幂等性
		for i := 0; i < len(userIDs); i += consts.MaxBatchSizeUserQuery {
			end := i + consts.MaxBatchSizeUserQuery
			if end > len(userIDs) {
				end = len(userIDs)
			}
			batch := userIDs[i:end]

			batchExistsMap, err := s.staticRepo.BatchCheckExists(ctx, batch, date)
			if err != nil {
				return nil, err
			}
			for userID, exists := range batchExistsMap {
				existsMap[userID] = exists
			}
		}
	} else {
		var err error
		existsMap, err = s.staticRepo.BatchCheckExists(ctx, userIDs, date)
		if err != nil {
			return nil, err
		}
	}

	return existsMap, nil
}

// calculateUserStaticRewardOneTime 计算用户静态收益（一次性释放剩余金额）
// 规则：
// - 本次释放金额 = stake_amount - released_static（小于等于0则不释放）
// - released_static 更新为 stake_amount（若 released_static 已大于 stake_amount，则保持不变）
// - 所有运行中算力包统一置为 status=3（额度耗尽出局）
func (s *staticService) calculateUserStaticRewardOneTime(packages []*rewardEntity.StakingPackageEntity, date time.Time) (decimal.Decimal, []PackageUpdate, []PackageRewardDetail, decimal.Decimal, decimal.Decimal, error) {
	totalStaticReward := decimal.Zero
	var packageUpdates []PackageUpdate
	var packageRewardDetails []PackageRewardDetail
	var dailyRate decimal.Decimal
	totalStakeAmount := decimal.Zero

	for _, pkg := range packages {
		// 记录日收益率（仅用于metadata展示）
		if dailyRate.IsZero() {
			dailyRate = pkg.DailyYieldRate
		}

		// 本次释放剩余金额：stake_amount - released_static
		remain := pkg.StakeAmount.Sub(pkg.ReleasedStatic)
		if remain.LessThan(decimal.Zero) {
			remain = decimal.Zero
		}

		// 更新 released_static：至少推进到 stake_amount（不回退）
		newReleasedStatic := pkg.ReleasedStatic
		if remain.GreaterThan(decimal.Zero) {
			newReleasedStatic = pkg.ReleasedStatic.Add(remain)
		}
		if newReleasedStatic.LessThan(pkg.StakeAmount) {
			newReleasedStatic = pkg.StakeAmount
		}

		// days_elapsed / remaining_days：一次性释放后统一归零剩余天数（仅用于展示字段）
		newDaysElapsed := pkg.TotalDays
		if newDaysElapsed.LessThanOrEqual(decimal.Zero) {
			newDaysElapsed = pkg.DaysElapsed
		}
		newRemainingDays := decimal.Zero

		// 统一置为 status=3（额度耗尽出局）
		packageUpdates = append(packageUpdates, PackageUpdate{
			PackageID:          pkg.Id,
			NewReleasedStatic:  newReleasedStatic,
			NewDaysElapsed:     newDaysElapsed,
			NewRemainingDays:   newRemainingDays,
			ShouldUpdateMax:    true,
			NewMaxStaticRelease: pkg.StakeAmount,
			ShouldUpdateStatus: true,
			NewStatus:          consts.StakingStatusQuotaExpired,
		})

		// 只有本次释放金额>0才产生资金记录明细
		if remain.GreaterThan(decimal.Zero) {
			totalStaticReward = totalStaticReward.Add(remain)

			totalElapsedSeconds := int64(date.Sub(pkg.StartTime).Seconds())
			if totalElapsedSeconds < 0 {
				totalElapsedSeconds = 0
			}

			rewardResult := PackageRewardResult{
				Reward:              remain,
				ElapsedSeconds:      0,
				TotalElapsedSeconds: totalElapsedSeconds,
				EndTime:             date,
				DailyRate:           pkg.DailyYieldRate,
			}

			packageRewardDetails = append(packageRewardDetails, PackageRewardDetail{
				Package:      pkg,
				RewardResult: rewardResult,
			})
		}

		// 累加总质押金额
		totalStakeAmount = totalStakeAmount.Add(pkg.StakeAmount)
	}

	return totalStaticReward, packageUpdates, packageRewardDetails, dailyRate, totalStakeAmount, nil
}

// calculateUserStaticReward 计算用户静态收益（业务逻辑，不检查额度限制）
func (s *staticService) calculateUserStaticReward(packages []*rewardEntity.StakingPackageEntity, date time.Time, lastRecordTime time.Time) (decimal.Decimal, []PackageUpdate, []PackageRewardDetail, decimal.Decimal, decimal.Decimal, error) {
	totalStaticReward := decimal.Zero
	var packageUpdates []PackageUpdate
	var packageRewardDetails []PackageRewardDetail
	var dailyRate decimal.Decimal
	totalStakeAmount := decimal.Zero

	for _, pkg := range packages {
		// 注意：时间条件已在数据库查询层面过滤（start_time <= date），此处无需再次检查

		// 计算单个算力包的增量静态收益
		rewardResult := s.calculatePackageStaticRewardIncrement(pkg, lastRecordTime, date)
		staticReward := rewardResult.Reward

		// 记录日收益率（所有包使用相同的日收益率）
		if dailyRate.IsZero() {
			dailyRate = rewardResult.DailyRate
		}

		// 检查静态上限限制（直接使用包记录中存储的 max_static_release，已根据类型正确计算）
		maxStaticRelease := pkg.MaxStaticRelease
		if pkg.ReleasedStatic.Add(staticReward).GreaterThan(maxStaticRelease) {
			staticReward = maxStaticRelease.Sub(pkg.ReleasedStatic)
			if staticReward.LessThanOrEqual(decimal.Zero) {
				continue
			}
		}

		// 不再检查剩余额度限制，直接发放奖励
		if staticReward.LessThanOrEqual(decimal.Zero) {
			continue
		}

		// 计算新的已释放静态收益
		newReleasedStatic := pkg.ReleasedStatic.Add(staticReward)

		// 计算已释放天数（每天释放一次，所以天数+1）
		// 如果这是第一次释放，从0开始；否则在原有天数基础上+1
		newDaysElapsed := pkg.DaysElapsed.Add(decimal.NewFromInt(1))

		// 计算剩余天数
		totalDays := pkg.TotalDays
		newRemainingDays := totalDays.Sub(newDaysElapsed)
		if newRemainingDays.LessThan(decimal.Zero) {
			newRemainingDays = decimal.Zero
		}

		// 检查是否达到静态出局条件
		shouldUpdateStatus := newReleasedStatic.GreaterThanOrEqual(maxStaticRelease)
		newStatus := consts.StakingStatusRunning
		if shouldUpdateStatus {
			newStatus = consts.StakingStatusStaticExpired
		}

		packageUpdates = append(packageUpdates, PackageUpdate{
			PackageID:          pkg.Id,
			NewReleasedStatic:  newReleasedStatic,
			NewDaysElapsed:     newDaysElapsed,
			NewRemainingDays:   newRemainingDays,
			ShouldUpdateStatus: shouldUpdateStatus,
			NewStatus:          newStatus,
		})

		totalStaticReward = totalStaticReward.Add(staticReward)

		rewardResult.Reward = staticReward
		packageRewardDetails = append(packageRewardDetails, PackageRewardDetail{
			Package:      pkg,
			RewardResult: rewardResult,
		})

		// 累加总质押金额
		totalStakeAmount = totalStakeAmount.Add(pkg.StakeAmount)
	}

	return totalStaticReward, packageUpdates, packageRewardDetails, dailyRate, totalStakeAmount, nil
}

// calculatePackageStaticRewardIncrement 计算单个算力包每天的静态收益（业务逻辑）
// 每天固定释放：stake_amount * daily_yield_rate（不涉及时间差计算）
func (s *staticService) calculatePackageStaticRewardIncrement(pkg *rewardEntity.StakingPackageEntity, lastRecordTime time.Time, currentRecordTime time.Time) PackageRewardResult {
	// 获取日收益率
	dailyRate := pkg.DailyYieldRate

	// 每天固定释放金额：stake_amount * daily_yield_rate（固定值，不计算时间差）
	dailyReward := pkg.StakeAmount.Mul(dailyRate)

	// 以下字段仅用于记录到metadata，不影响释放金额计算
	// 每天固定86400秒（仅用于记录）
	elapsedSeconds := int64(consts.SecondsPerDay)
	// 总经过秒数（仅用于记录，不影响释放金额）
	totalElapsedSeconds := int64(currentRecordTime.Sub(pkg.StartTime).Seconds())
	if totalElapsedSeconds < 0 {
		totalElapsedSeconds = 0
	}

	return PackageRewardResult{
		Reward:              dailyReward,
		ElapsedSeconds:      elapsedSeconds,
		TotalElapsedSeconds: totalElapsedSeconds,
		EndTime:             currentRecordTime,
		DailyRate:           dailyRate,
	}
}

// createAssetRecord 创建资金记录（业务逻辑）
func (s *staticService) createAssetRecord(userID int64, totalReward decimal.Decimal, packageRewardDetails []PackageRewardDetail, dailyRate decimal.Decimal, totalStakeAmount decimal.Decimal, normalizedDate time.Time) *rewardEntity.AssetRecordEntity {
	var packageDetails []map[string]interface{}
	for _, detail := range packageRewardDetails {
		pkg := detail.Package
		rewardResult := detail.RewardResult

		packageDetail := map[string]interface{}{
			"package_id":   pkg.Id,
			"stake_amount": pkg.StakeAmount.String(),
			"power_value":  pkg.PowerValue.String(),
			// "yield_rate":            consts.StaticRewardDailyRate,
			"yield_rate":            pkg.DailyYieldRate.String(),
			"reward_amount":         rewardResult.Reward.String(),
			"elapsed_seconds":       rewardResult.ElapsedSeconds,
			"total_elapsed_seconds": rewardResult.TotalElapsedSeconds,
			"end_time":              rewardResult.EndTime.Format(consts.TimeFormatDateTime),
		}
		packageDetails = append(packageDetails, packageDetail)
	}

	metadata := map[string]interface{}{
		"package_count":      len(packageRewardDetails),
		"total_reward":       totalReward.String(),
		"daily_rate":         dailyRate.String(),
		"total_stake_amount": totalStakeAmount.String(),
		"packages":           packageDetails,
	}
	metadataJSON, _ := json.Marshal(metadata)

	return &rewardEntity.AssetRecordEntity{
		UserID:       userID,
		AssetType:    consts.AssetTypeAPGUserBalance,
		RecordTime:   normalizedDate,
		Amount:       totalReward,
		FlowType:     consts.AssetFlowTypeIncome,
		Status:       consts.AssetRecordStatusPending,
		BusinessType: consts.AssetBusinessTypeRewardStatic,
		BusinessID:   0,
		Remark:       "静态收益",
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
