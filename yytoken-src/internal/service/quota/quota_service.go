package quota

import (
	"context"

	"XWFrame/internal/repository"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

const rewardTypeIndirect = "indirect"

// IQuotaService 额度服务接口
type IQuotaService interface {
	// CalculateRemainingQuota 计算用户剩余推荐额度
	// 额度上限 = ∑自己购买节点价格×倍率（即 power_value）
	// 剩余额度 = 额度上限 - 已获得推荐佣金
	// 备注：推荐奖励计算公式仍是 L1×10%，该公式仅用于额度上限
	CalculateRemainingQuota(ctx context.Context, userID int64) (decimal.Decimal, error)

	// CalculateRewardLimit 计算用户的推荐佣金上限（不包含已获得的）
	CalculateRewardLimit(ctx context.Context, userID int64) (l1Limit, l2Limit decimal.Decimal, err error)

	// GetReceivedReward 获取用户已获得的推荐佣金总额
	GetReceivedReward(ctx context.Context, userID int64) (decimal.Decimal, error)
}

// quotaService 额度服务实现
type quotaService struct {
	userRepo       repository.IUserRepository
	rewardStatRepo rewardStatRepository
}

type rewardStatRepository interface {
	GetDirectRewardSum(ctx context.Context, userID int64) (decimal.Decimal, error)
	GetIndirectRewardSum(ctx context.Context, userID int64) (decimal.Decimal, error)
}

type dbRewardStatRepository struct{}

func (r *dbRewardStatRepository) GetDirectRewardSum(ctx context.Context, userID int64) (decimal.Decimal, error) {
	directRewardVar, err := g.DB().Model("cobo_node_purchase").Ctx(ctx).
		Fields("COALESCE(SUM(direct_reward_amount), 0)").
		Where("direct_reward_user_id = ? AND direct_reward_amount > 0 AND is_gift = 0", userID).
		Value()
	if err != nil {
		return decimal.Zero, err
	}
	return decimal.NewFromString(directRewardVar.String())
}

func (r *dbRewardStatRepository) GetIndirectRewardSum(ctx context.Context, userID int64) (decimal.Decimal, error) {
	indirectRewardVar, err := g.DB().Model("cobo_reward_record").Ctx(ctx).
		Fields("COALESCE(SUM(amount), 0)").
		Where("user_id = ? AND reward_type = ?", userID, rewardTypeIndirect).
		Value()
	if err != nil {
		return decimal.Zero, err
	}
	return decimal.NewFromString(indirectRewardVar.String())
}

// NewQuotaService 创建额度服务
func NewQuotaService() IQuotaService {
	return &quotaService{
		userRepo:       repository.NewUserRepository(),
		rewardStatRepo: &dbRewardStatRepository{},
	}
}

// CalculateRemainingQuota 计算用户剩余推荐额度
func (s *quotaService) CalculateRemainingQuota(ctx context.Context, userID int64) (decimal.Decimal, error) {
	// 获取当前用户
	currentUser, err := s.userRepo.GetUserById(ctx, userID)
	if err != nil {
		return decimal.Zero, err
	}
	if currentUser == nil {
		return decimal.Zero, nil
	}

	// 计算额度上限
	l1Limit, l2Limit, err := s.CalculateRewardLimit(ctx, userID)
	if err != nil {
		return decimal.Zero, err
	}
	rewardLimit := l1Limit.Add(l2Limit)

	// 计算已获得的推荐佣金
	receivedReward, err := s.GetReceivedReward(ctx, userID)
	if err != nil {
		return decimal.Zero, err
	}

	// 剩余额度 = 上限 - 已获得
	remaining := rewardLimit.Sub(receivedReward)
	if remaining.LessThan(decimal.Zero) {
		remaining = decimal.Zero
	}

	return remaining, nil
}

// CalculateRewardLimit 计算用户的推荐佣金上限
// 上限口径：用户自身购买节点的 power_value 之和（仅统计正常购买且运行中）
func (s *quotaService) CalculateRewardLimit(ctx context.Context, userID int64) (l1Limit, l2Limit decimal.Decimal, err error) {
	powerValueVar, err := g.DB().Model("cobo_node_purchase").Ctx(ctx).
		Fields("COALESCE(SUM(power_value), 0)").
		Where("user_id = ? AND is_gift = 0 AND status = 1", userID).
		Value()
	if err != nil {
		g.Log().Warningf(ctx, "[额度计算] 获取用户节点power_value总额失败: user_id=%d, err=%v", userID, err)
		return decimal.Zero, decimal.Zero, err
	}

	selfLimit, parseErr := decimal.NewFromString(powerValueVar.String())
	if parseErr != nil {
		g.Log().Warningf(ctx, "[额度计算] 解析用户节点power_value总额失败: user_id=%d, val=%s, err=%v", userID, powerValueVar.String(), parseErr)
		return decimal.Zero, decimal.Zero, parseErr
	}

	// 兼容现有返回结构：额度全部放在l1Limit，l2Limit固定为0
	return selfLimit, decimal.Zero, nil
}

// GetReceivedReward 获取用户已获得的推荐佣金总额
func (s *quotaService) GetReceivedReward(ctx context.Context, userID int64) (decimal.Decimal, error) {
	directReward, err := s.rewardStatRepo.GetDirectRewardSum(ctx, userID)
	if err != nil {
		g.Log().Warningf(ctx, "[额度计算] 获取已获得直推佣金失败: %v", err)
		return decimal.Zero, err
	}

	indirectReward, err := s.rewardStatRepo.GetIndirectRewardSum(ctx, userID)
	if err != nil {
		g.Log().Warningf(ctx, "[额度计算] 获取已获得间推佣金失败: %v", err)
		return decimal.Zero, err
	}

	return directReward.Add(indirectReward), nil
}

// Svc 获取额度服务实例（单例）
var quotaServiceInstance IQuotaService

func Svc() IQuotaService {
	if quotaServiceInstance == nil {
		quotaServiceInstance = NewQuotaService()
	}
	return quotaServiceInstance
}
