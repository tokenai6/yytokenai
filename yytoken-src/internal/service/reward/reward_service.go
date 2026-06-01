package reward

import (
	"context"
	"fmt"
	"time"

	"XWFrame/internal/frame/consts"
	rewardRepo "XWFrame/internal/repository/reward"
	"XWFrame/internal/service/reward/model"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

type IRewardService interface {
	GetOverview(ctx context.Context) (*model.RewardOverview, error)
	GetLogs(ctx context.Context, type_ string, page, pageSize int) ([]*model.RewardLogItem, int, error)
}

type rewardService struct {
	rewardRepo      rewardRepo.IRewardRepository
	assetRecordRepo rewardRepo.IAssetRecordRepository
}

var rewardServiceInstance *rewardService

func Svc() IRewardService {
	if rewardServiceInstance == nil {
		rewardServiceInstance = &rewardService{
			rewardRepo:      rewardRepo.NewRewardRepository(),
			assetRecordRepo: rewardRepo.NewAssetRecordRepository(),
		}
	}
	return rewardServiceInstance
}

func (s *rewardService) GetOverview(ctx context.Context) (*model.RewardOverview, error) {
	// 数据验证：从上下文获取当前用户ID
	userID := ctx.Value("user_id")
	if userID == nil {
		return nil, gerror.New("未获取到用户信息")
	}
	uid, ok := userID.(int64)
	if !ok {
		return nil, gerror.New("用户ID类型错误")
	}

	// 业务逻辑：获取用户最新的奖励记录时间
	latestRecordTime, err := s.rewardRepo.GetUserLatestRewardRecordTime(ctx, uid, consts.RewardTypesForStatistics)
	if err != nil {
		g.Log().Warningf(ctx, "[奖励概览] 获取最新奖励记录时间失败: %v", err)
		latestRecordTime = time.Time{}
	}

	// 业务逻辑：获取各类型奖励金额（聚合计算）
	var yesterdayStatic, yesterdayDistrict, yesterdayReferral, yesterdayWeighted, yesterdayNode decimal.Decimal
	if latestRecordTime.IsZero() {
		g.Log().Infof(ctx, "[奖励概览] 用户 %d 没有奖励记录，所有奖励为0", uid)
	} else {
		rewardTotals, err := s.rewardRepo.GetUserRewardTotalByOffsetGroupByType(ctx, uid, consts.RewardTypesForStatistics, latestRecordTime)
		if err != nil {
			g.Log().Warningf(ctx, "[奖励概览] 获取最新记录失败: %v", err)
		} else {
			yesterdayStatic = rewardTotals[consts.AssetBusinessTypeRewardStatic]
			yesterdayDistrict = rewardTotals[consts.AssetBusinessTypeRewardDistrict]
			yesterdayReferral = rewardTotals[consts.AssetBusinessTypeRewardReferral]
			yesterdayWeighted = rewardTotals[consts.AssetBusinessTypeRewardWeighted]
			yesterdayNode = rewardTotals[consts.AssetBusinessTypeRewardNode]
		}
	}

	// 业务逻辑：计算昨日总奖励（各类型相加）
	yesterdayTotal := yesterdayStatic.Add(yesterdayDistrict).Add(yesterdayReferral).Add(yesterdayWeighted).Add(yesterdayNode)

	// 业务逻辑：获取累计总收益
	totalEarnings, err := s.assetRecordRepo.GetUserTotalIncome(ctx, uid, consts.RewardStatusSettled)
	if err != nil {
		g.Log().Warningf(ctx, "[奖励概览] 获取累计总收益失败: %v", err)
		totalEarnings = decimal.Zero
	}

	// 业务逻辑：格式化数据
	return &model.RewardOverview{
		YesterdayTotal:    utils.FormatDecimal(yesterdayTotal),
		YesterdayStatic:   utils.FormatDecimal(yesterdayStatic),
		YesterdayDistrict: utils.FormatDecimal(yesterdayDistrict),
		YesterdayReferral: utils.FormatDecimal(yesterdayReferral),
		YesterdayWeighted: utils.FormatDecimal(yesterdayWeighted),
		YesterdayNode:     utils.FormatDecimal(yesterdayNode),
		TotalEarnings:     utils.FormatDecimal(totalEarnings),
	}, nil
}

func (s *rewardService) GetLogs(ctx context.Context, type_ string, page, pageSize int) ([]*model.RewardLogItem, int, error) {
	// 数据验证：从上下文获取当前用户ID
	userID := ctx.Value("user_id")
	if userID == nil {
		return nil, 0, gerror.New("未获取到用户信息")
	}
	uid, ok := userID.(int64)
	if !ok {
		return nil, 0, gerror.New("用户ID类型错误")
	}

	// 数据验证：分页参数
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 调用Repository层获取原始数据
	records, total, err := s.rewardRepo.GetPagedAssetRecords(ctx, uid, type_, "", page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	// 业务逻辑：数据转换和格式化
	items := make([]*model.RewardLogItem, 0, len(records))
	for _, record := range records {
		// 格式化金额（业务逻辑：添加正负号）
		amount := utils.FormatDecimal(record.Amount)
		if record.Amount.GreaterThan(decimal.Zero) {
			amount = "+" + amount
		}

		items = append(items, &model.RewardLogItem{
			Date:    record.CreatedAt.Format("2006-01-02"), // 业务逻辑：日期格式化
			Amount:  amount,                                // 业务逻辑：金额格式化
			Actual:  amount,                                // 实际金额与显示金额相同
			OrderNo: fmt.Sprintf("%d", record.BusinessID),  // 业务逻辑：订单号转换
			Extra:   record.Metadata,                       // 元数据
		})
	}

	return items, total, nil
}
