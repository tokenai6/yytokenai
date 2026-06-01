package cobo

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// TaxService 税务概览服务
type TaxService interface {
	GetTaxOverview(ctx context.Context, userID int64) (*TaxOverview, error)
}

// TaxOverview 用户税务概览
type TaxOverview struct {
	NodePower              string `json:"node_power" dc:"节点算力"`
	TaxDeductionRemaining  string `json:"tax_deduction_remaining" dc:"本月抵扣剩余额度"`
	PoolPending            string `json:"pool_pending" dc:"预估待收pool"`
	PoolDistributed        string `json:"pool_distributed" dc:"已分配pool总额"`
	GiftNodeActive         bool   `json:"gift_node_active" dc:"赠送节点抵扣额度是否已激活"`
}

type taxService struct{}

// NewTaxService 创建税务概览服务
func NewTaxService() TaxService {
	return &taxService{}
}

func (s *taxService) GetTaxOverview(ctx context.Context, userID int64) (*TaxOverview, error) {
	// 1. 节点算力
	nodePower, err := s.getNodePower(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 2. 抵扣额度（复用 loadWithdrawTaxQuota）
	quota, err := loadWithdrawTaxQuota(ctx, nil, userID, "USDT")
	if err != nil {
		return nil, err
	}
	remainingQuota := quota.NodeQuota.Sub(quota.TaxDeductionUsed)
	if remainingQuota.LessThan(decimal.Zero) {
		remainingQuota = decimal.Zero
	}

	// 3. pool 待分配 = 当前待结算 pool 总金额 × 用户权重 / 全网权重
	poolPending := decimal.Zero
	if nodePower.GreaterThan(decimal.Zero) {
		poolPending, err = s.getPoolPending(ctx, nodePower)
		if err != nil {
			return nil, err
		}
	}

	// 4. pool 已分配
	poolDistributed, err := s.getPoolDistributed(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 5. 赠送节点抵扣额度是否已激活（伞下业绩 >= 赠送节点总额 × 10）
	giftNodeActive, err := s.getGiftNodeActive(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &TaxOverview{
		NodePower:             nodePower.String(),
		TaxDeductionRemaining: remainingQuota.String(),
		PoolPending:           poolPending.String(),
		PoolDistributed:       poolDistributed.String(),
		GiftNodeActive:        giftNodeActive,
	}, nil
}

func (s *taxService) getGiftNodeActive(ctx context.Context, userID int64) (bool, error) {
	giftRow, err := getOneRecord(ctx, nil, `
		SELECT COALESCE(SUM(CASE WHEN p.is_gift = 1 THEN p.amount ELSE 0 END), 0)::numeric(28,8) AS gift_amount_total
		FROM cobo_node_purchase p
		JOIN node_info n ON n.node_type = p.node_type
		WHERE p.user_id = ?
	`, userID)
	if err != nil {
		return false, err
	}
	if giftRow == nil {
		return true, nil
	}
	giftAmountTotal, _ := decimal.NewFromString(giftRow["gift_amount_total"].String())
	if giftAmountTotal.LessThanOrEqual(decimal.Zero) {
		return true, nil
	}
	qualified, _, _, _, err := checkGiftNodePerformanceQualified(ctx, nil, userID, giftAmountTotal)
	if err != nil {
		return false, err
	}
	return qualified, nil
}

func (s *taxService) getNodePower(ctx context.Context, userID int64) (decimal.Decimal, error) {
	row, err := getOneRecord(ctx, nil, `
		SELECT COALESCE(SUM(p.amount * n.power_multiplier), 0) AS power
		FROM cobo_node_purchase p
		JOIN node_info n ON n.node_type = p.node_type
		WHERE p.user_id = ?
	`, userID)
	if err != nil {
		return decimal.Zero, err
	}
	if row == nil {
		return decimal.Zero, nil
	}
	power, _ := decimal.NewFromString(row["power"].String())
	return power, nil
}

func (s *taxService) getPoolPending(ctx context.Context, userPower decimal.Decimal) (decimal.Decimal, error) {
	// 全网待结算 pool 总额
	poolRow, err := getOneRecord(ctx, nil, `
		SELECT COALESCE(SUM(amount), 0) AS total
		FROM us_stock_reward_pool
		WHERE status = 0
	`)
	if err != nil {
		return decimal.Zero, err
	}
	poolTotal, _ := decimal.NewFromString(poolRow["total"].String())
	if poolTotal.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, nil
	}

	// 全网节点权重
	weightRow, err := getOneRecord(ctx, nil, `
		SELECT COALESCE(SUM(p.amount * n.power_multiplier), 0) AS total
		FROM cobo_node_purchase p
		JOIN node_info n ON n.node_type = p.node_type
	`)
	if err != nil {
		return decimal.Zero, err
	}
	totalWeight, _ := decimal.NewFromString(weightRow["total"].String())
	if totalWeight.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, nil
	}

	return poolTotal.Mul(userPower).DivRound(totalWeight, 8), nil
}

func (s *taxService) getPoolDistributed(ctx context.Context, userID int64) (decimal.Decimal, error) {
	row, err := getOneRecord(ctx, nil, `
		SELECT COALESCE(SUM(amount), 0) AS total
		FROM us_stock_reward_settlement
		WHERE user_id = ?
	`, userID)
	if err != nil {
		return decimal.Zero, err
	}
	if row == nil {
		return decimal.Zero, nil
	}
	total, _ := decimal.NewFromString(row["total"].String())
	return total, nil
}

// TaxOverviewAPI 用于 API 层的税务概览响应
func (s *taxService) TaxOverviewAPI(ctx context.Context, userID int64) (*TaxOverview, error) {
	return s.GetTaxOverview(ctx, userID)
}

// userIDFromCtx 从上下文获取用户ID
func userIDFromCtx(ctx context.Context) int64 {
	req := g.RequestFromCtx(ctx)
	if req == nil {
		return 0
	}
	return req.GetCtxVar("user_id").Int64()
}

// TaxOverviewReq 请求结构（供 api 层使用）
type TaxOverviewReq struct{}

// TaxOverviewRes 响应结构（供 api 层使用）
type TaxOverviewRes struct {
	NodePower             string `json:"node_power"`
	TaxDeductionRemaining string `json:"tax_deduction_remaining"`
	PoolPending           string `json:"pool_pending"`
	PoolDistributed       string `json:"pool_distributed"`
	GiftNodeActive        bool   `json:"gift_node_active"`
}

// GetTaxOverview 获取税务概览（供 api 层调用）
func GetTaxOverview(ctx context.Context, userID int64) (*TaxOverviewRes, error) {
	svc := NewTaxService()
	overview, err := svc.GetTaxOverview(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &TaxOverviewRes{
		NodePower:             overview.NodePower,
		TaxDeductionRemaining: overview.TaxDeductionRemaining,
		PoolPending:           overview.PoolPending,
		PoolDistributed:       overview.PoolDistributed,
		GiftNodeActive:        overview.GiftNodeActive,
	}, nil
}
