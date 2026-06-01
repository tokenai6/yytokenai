package weighted

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/consts"
	rewardRepo "XWFrame/internal/repository/reward"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// IWeightedService 加权奖励服务接口
type IWeightedService interface {
	// DistributeWeightedReward 发放加权奖励
	DistributeWeightedReward(ctx context.Context, date time.Time) error
}

// weightedService 加权奖励服务实现
type weightedService struct {
	weightedRepo rewardRepo.IRewardWeightedRepository
}

// NewWeightedService 创建加权奖励服务实例
func NewWeightedService() IWeightedService {
	return &weightedService{
		weightedRepo: rewardRepo.NewRewardWeightedRepository(),
	}
}

// DistributeWeightedReward 发放加权奖励（业务逻辑层）
func (s *weightedService) DistributeWeightedReward(ctx context.Context, date time.Time) error {
	g.Log().Infof(ctx, "[加权奖励计算] 开始计算加权奖励: date=%s", date.Format(consts.TimeFormatDate))

	// 1. 获取全网总产出
	networkTotalOutput, err := s.weightedRepo.GetDailySumByBusiness(ctx, date, consts.AssetBusinessTypeRewardStatic)
	if err != nil {
		return gerror.Wrap(err, "获取全网总产出失败")
	}

	if networkTotalOutput.IsZero() {
		g.Log().Info(ctx, "[加权奖励] 全网总产出为0，无需发放")
		return nil
	}

	// 2. 获取全部直推新增业绩榜单（按业绩降序）
	targetCount := consts.WeightedRewardTop50Count

	var (
		candidates []*rewardEntity.UserPerformanceEntity

		filteredUsers           []*rewardEntity.UserPerformanceEntity
		totalSkippedExisting    int
		totalSkippedQuota       int
		totalSkippedPerformance int
	)

	candidates, err = s.weightedRepo.GetTopByNewDirectPerformance(ctx, date, 0)
	if err != nil {
		return gerror.Wrap(err, "获取直推新增业绩榜单失败")
	}

	if len(candidates) == 0 {
		g.Log().Info(ctx, "[加权奖励] 无用户有直推新增业绩，无需发放")
		return nil
	}

	// 批量获取额度与幂等信息
	userIDs := make([]int64, 0, len(candidates))
	for _, user := range candidates {
		userIDs = append(userIDs, user.UserID)
	}

	quotaMap, err := s.weightedRepo.BatchGetQuotas(ctx, userIDs)
	if err != nil {
		return gerror.Wrap(err, "批量获取用户额度失败")
	}

	existsMap, err := s.weightedRepo.BatchCheckExists(ctx, userIDs, date)
	if err != nil {
		return gerror.Wrap(err, "批量幂等检查失败")
	}

	filteredUsers = make([]*rewardEntity.UserPerformanceEntity, 0, targetCount)

	for _, userPerf := range candidates {
		if len(filteredUsers) >= targetCount {
			break
		}

		if existsMap[userPerf.UserID] {
			totalSkippedExisting++
			continue
		}

		quota, ok := quotaMap[userPerf.UserID]
		if !ok || quota == nil || quota.RemainingQuota.LessThanOrEqual(decimal.Zero) {
			totalSkippedQuota++
			continue
		}

		if userPerf.PersonalPerformance.LessThan(consts.GetMinPerformanceThreshold()) {
			totalSkippedPerformance++
			continue
		}

		filteredUsers = append(filteredUsers, userPerf)
	}

	if len(filteredUsers) == 0 {
		g.Log().Infof(ctx, "[加权奖励] 无符合条件的候选用户（总候选=%d，已存在=%d，额度不足=%d，业绩不足=%d）",
			len(candidates), totalSkippedExisting, totalSkippedQuota, totalSkippedPerformance)
		return nil
	}

	if len(filteredUsers) > targetCount {
		filteredUsers = filteredUsers[:targetCount]
	}

	if len(filteredUsers) < targetCount {
		g.Log().Infof(ctx, "[加权奖励] 符合条件的用户不足%d名，实际=%d，已存在=%d，额度不足=%d，业绩不足=%d",
			targetCount, len(filteredUsers), totalSkippedExisting, totalSkippedQuota, totalSkippedPerformance)
	} else {
		g.Log().Infof(ctx, "[加权奖励] 已筛选出满足条件的前%d名用户，已存在=%d，额度不足=%d，业绩不足=%d",
			targetCount, totalSkippedExisting, totalSkippedQuota, totalSkippedPerformance)
	}
	// 3. 计算前50名总新增业绩
	top50Total := decimal.Zero
	for _, user := range filteredUsers {
		top50Total = top50Total.Add(user.NewDirectPerformance)
	}

	if top50Total.IsZero() {
		g.Log().Info(ctx, "[加权奖励] 符合条件用户的直推新增总额为0，无需发放")
		return nil
	}

	// 4. 计算分红池（业务逻辑）
	dividendPool := networkTotalOutput.Mul(consts.GetWeightedRewardPoolRate())

	g.Log().Infof(ctx, "[加权奖励] 全网总产出=%s, 前50名总新增=%s, 分红池=%s",
		networkTotalOutput.String(), top50Total.String(), dividendPool.String())

	// 5. 计算并准备批量创建数据（业务逻辑）
	recordsToCreate := make([]*rewardEntity.AssetRecordEntity, 0, len(filteredUsers))
	skipExistsCount := 0

	for rank, userPerf := range filteredUsers {
		// 幂等检查
		if existsMap[userPerf.UserID] {
			skipExistsCount++
			continue
		}

		// 检查用户额度
		quota, exists := quotaMap[userPerf.UserID]
		if !exists || quota == nil || quota.RemainingQuota.LessThanOrEqual(decimal.Zero) {
			continue
		}

		// 检查个人业绩门槛（100u）
		if userPerf.PersonalPerformance.LessThan(consts.GetMinPerformanceThreshold()) {
			continue
		}

		// 业务逻辑：计算权重比例和奖励
		weightRatio := userPerf.NewDirectPerformance.DivRound(top50Total, consts.TokenPrecision)
		rewardAmount := dividendPool.Mul(weightRatio)

		// 构建元数据
		weightedMetadata := rewardEntity.WeightedRewardMetadata{
			Ranking:               rank + 1,
			NewDirectPerformance:  userPerf.NewDirectPerformance,
			Top50TotalPerformance: top50Total,
			NetworkTotalOutput:    networkTotalOutput,
			WeightRatio:           weightRatio,
		}
		metadataJSON, _ := json.Marshal(weightedMetadata)

		// 创建资产记录
		recordsToCreate = append(recordsToCreate, &rewardEntity.AssetRecordEntity{
			UserID:       userPerf.UserID,
			AssetType:    consts.AssetTypeAPGUserBalance,
			RecordTime:   date,
			Amount:       rewardAmount,
			FlowType:     consts.AssetFlowTypeIncome,
			Status:       consts.AssetRecordStatusPending,
			BusinessType: consts.AssetBusinessTypeRewardWeighted,
			BusinessID:   0,
			Remark:       fmt.Sprintf("加权奖励-排名第%d", rank+1),
			Metadata:     string(metadataJSON),
		})
	}

	if skipExistsCount > 0 {
		g.Log().Infof(ctx, "[加权奖励] 最终阶段跳过已存在记录数: %d", skipExistsCount)
	}

	if len(recordsToCreate) == 0 {
		g.Log().Infof(ctx, "[加权奖励] 最终无可发放的用户（筛选用户=%d, 已存在=%d）", len(filteredUsers), skipExistsCount)
		return nil
	}

	// 7. 批量创建资金记录
	if err := s.weightedRepo.BatchCreateAssetRecords(ctx, recordsToCreate); err != nil {
		return gerror.Wrap(err, "批量发放失败")
	}

	g.Log().Infof(ctx, "[加权奖励] 发放完成: 总用户=%d, 实际发放=%d, 最终跳过=%d",
		len(filteredUsers), len(recordsToCreate), skipExistsCount)

	return nil
}
