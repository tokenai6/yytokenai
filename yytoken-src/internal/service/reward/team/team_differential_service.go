package team

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

// ITeamDifferentialService 团队极差奖励服务接口
type ITeamDifferentialService interface {
	// DistributeTeamDifferentialReward 发放团队极差奖励
	DistributeTeamDifferentialReward(ctx context.Context, date time.Time) error
}

type teamDifferentialService struct {
	teamRepo rewardRepo.ITeamDifferentialRepository
}

func NewTeamDifferentialService() ITeamDifferentialService {
	return &teamDifferentialService{
		teamRepo: rewardRepo.NewTeamDifferentialRepository(),
	}
}

func (s *teamDifferentialService) DistributeTeamDifferentialReward(ctx context.Context, date time.Time) error {
	g.Log().Infof(ctx, "[TeamDifferentialReward] 开始发放团队极差奖励，日期: %s", date.Format(consts.TimeFormatDate))

	// 1. 获取所有产生静态收益的用户及其收益金额
	staticRewards, err := s.teamRepo.GetStaticRewardsByDate(ctx, date)
	if err != nil {
		g.Log().Errorf(ctx, "[TeamDifferentialReward] 获取静态收益记录失败: %v", err)
		return err
	}

	if len(staticRewards) == 0 {
		g.Log().Info(ctx, "[TeamDifferentialReward] 没有静态收益记录，跳过")
		return nil
	}
	g.Log().Infof(ctx, "[TeamDifferentialReward] 静态收益用户数: %d", len(staticRewards))

	// 2. 收集所有涉及到的用户（产生收益的用户 + 他们的祖先）
	earnerIDs := make([]int64, 0, len(staticRewards))
	for userID := range staticRewards {
		earnerIDs = append(earnerIDs, userID)
	}

	// 3. 批量获取祖先链
	ancestorMap, err := s.teamRepo.GetAncestorsByUserIDs(ctx, earnerIDs)
	if err != nil {
		g.Log().Errorf(ctx, "[TeamDifferentialReward] 获取祖先链失败: %v", err)
		return err
	}
	g.Log().Infof(ctx, "[TeamDifferentialReward] 祖先链获取完成，涉及用户数: %d", len(ancestorMap))

	// 收集所有需要查询等级的用户ID（所有祖先）
	allAncestorIDsMap := make(map[int64]bool)
	for _, ancestors := range ancestorMap {
		for _, ancestorID := range ancestors {
			allAncestorIDsMap[ancestorID] = true
		}
	}
	allAncestorIDs := make([]int64, 0, len(allAncestorIDsMap))
	for ancestorID := range allAncestorIDsMap {
		allAncestorIDs = append(allAncestorIDs, ancestorID)
	}
	g.Log().Infof(ctx, "[TeamDifferentialReward] 需要查询等级的用户数: %d", len(allAncestorIDs))

	// 4. 批量查询等级信息 (vip_level, admin_level)
	vnsHomeDataMap, err := s.teamRepo.GetVnsHomeDataByUserIDs(ctx, allAncestorIDs)
	if err != nil {
		g.Log().Errorf(ctx, "[TeamDifferentialReward] 获取等级信息失败: %v", err)
		return err
	}
	g.Log().Infof(ctx, "[TeamDifferentialReward] 已获取等级数据条数: %d", len(vnsHomeDataMap))

	// 5. 幂等性检查
	// 针对每一个祖先，检查是否已经发放过该日期的团队奖励
	// 注意：如果中途失败重试，这种检查方式可能导致部分发放
	// 但在 BatchExecuteTeamReward 中使用事务可以保证原子性
	existsMap, err := s.teamRepo.BatchCheckExists(ctx, allAncestorIDs, date)
	if err != nil {
		g.Log().Errorf(ctx, "[TeamDifferentialReward] 检查幂等性失败: %v", err)
		return err
	}

	ancestorTotalRewards := make(map[int64]decimal.Decimal)
	ancestorRewardDetails := make(map[int64][]map[string]interface{})

	for earnerID, rewardAmount := range staticRewards {
		ancestors, exists := ancestorMap[earnerID]
		if !exists || len(ancestors) <= 1 { // ancestors[0] 是 earner 自己
			continue
		}

		currentMaxRate := decimal.Zero
		// 从 parent 开始往上找 (ancestors[0] 是 earner)
		for i := 1; i < len(ancestors); i++ {
			ancestorID := ancestors[i]

			// 检查该祖先是否已发放过今日奖励
			if existsMap[ancestorID] {
				continue
			}

			homeData, exists := vnsHomeDataMap[ancestorID]
			if !exists {
				continue
			}

			// VIP等级取 vip_level 和 admin_level 的最大值
			level := homeData.VipLevel
			if homeData.AdminLevel > level {
				level = homeData.AdminLevel
			}

			if level <= 0 {
				continue
			}

			rate := consts.GetTeamDifferentialRewardRate(level)
			if rate.GreaterThan(currentMaxRate) {
				diffRate := rate.Sub(currentMaxRate)
				diffReward := rewardAmount.Mul(diffRate)

				if diffReward.GreaterThan(decimal.Zero) {
					ancestorTotalRewards[ancestorID] = ancestorTotalRewards[ancestorID].Add(diffReward)

					detail := map[string]interface{}{
						"earner_id":      earnerID,
						"static_reward":  rewardAmount.String(),
						"ancestor_level": level,
						"rate":           rate.String(),
						"diff_rate":      diffRate.String(),
						"reward":         diffReward.String(),
					}
					ancestorRewardDetails[ancestorID] = append(ancestorRewardDetails[ancestorID], detail)
				}
				currentMaxRate = rate
			}

			// 如果已经达到最高比例，不再往上发
			if currentMaxRate.GreaterThanOrEqual(consts.GetTeamDifferentialRewardRate(9)) {
				break
			}
		}
	}

	// 6. 执行发放
	var allAssetRecords []*rewardEntity.AssetRecordEntity
	for ancestorID, totalAmount := range ancestorTotalRewards {
		if totalAmount.IsZero() {
			continue
		}

		metadata := map[string]interface{}{
			"total_reward": totalAmount.String(),
			"details":      ancestorRewardDetails[ancestorID],
		}
		metadataJSON, _ := json.Marshal(metadata)

		allAssetRecords = append(allAssetRecords, &rewardEntity.AssetRecordEntity{
			UserID:       ancestorID,
			AssetType:    consts.AssetTypeAPGUserBalance,
			RecordTime:   date,
			Amount:       totalAmount,
			FlowType:     consts.AssetFlowTypeIncome,
			Status:       consts.AssetRecordStatusPending,
			BusinessType: consts.AssetBusinessTypeTeamReward,
			Remark:       "团队极差奖励",
			Metadata:     string(metadataJSON),
		})
	}

	if len(allAssetRecords) > 0 {
		if err := s.teamRepo.BatchExecuteTeamReward(ctx, allAssetRecords); err != nil {
			g.Log().Errorf(ctx, "[TeamDifferentialReward] 批量执行奖励发放失败: %v", err)
			return err
		}
	}

	g.Log().Infof(ctx, "[TeamDifferentialReward] 团队极差奖励发放完成，发放人数: %d", len(allAssetRecords))
	return nil
}
