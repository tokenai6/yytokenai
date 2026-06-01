package cobo

import (
	"context"
	"encoding/json"
	"time"

	entityCobo "XWFrame/internal/entity/cobo"
	repo "XWFrame/internal/repository/cobo"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

const founderFeeDividendType = "founder_fee_dividend"

// FounderDividendService 创始股手续费分红服务
type FounderDividendService interface {
	DistributeDaily(ctx context.Context, runAt time.Time) error
}

type founderDividendService struct {
	balanceRepo repo.IBalanceRepository
}

// NewFounderDividendService 创建创始股手续费分红服务
func NewFounderDividendService() FounderDividendService {
	return &founderDividendService{balanceRepo: repo.NewBalanceRepository()}
}

type founderUserEquity struct {
	UserID  int64
	Equity  decimal.Decimal
	Reward  decimal.Decimal
	Percent decimal.Decimal
}

// DistributeDaily 按日发放创始股手续费分红（手续费池50%）
func (s *founderDividendService) DistributeDaily(ctx context.Context, runAt time.Time) error {
	lastDistAt, err := s.getLastDistributionTime(ctx)
	if err != nil {
		return err
	}

	feeTotal, err := s.getFeeTotal(ctx, lastDistAt, runAt)
	if err != nil {
		return err
	}
	if feeTotal.LessThanOrEqual(decimal.Zero) {
		g.Log().Info(ctx, "[CoboFounderDividend] 无新增手续费，跳过")
		return nil
	}

	pool := feeTotal.Mul(decimal.NewFromFloat(0.5))
	if pool.LessThanOrEqual(decimal.Zero) {
		g.Log().Info(ctx, "[CoboFounderDividend] 分红池为0，跳过")
		return nil
	}

	users, totalEquity, err := s.getFounderUsers(ctx)
	if err != nil {
		return err
	}
	if len(users) == 0 || totalEquity.LessThanOrEqual(decimal.Zero) {
		g.Log().Info(ctx, "[CoboFounderDividend] 无创始股用户，跳过")
		return nil
	}

	for i := range users {
		users[i].Percent = users[i].Equity.DivRound(totalEquity, 18)
		users[i].Reward = pool.Mul(users[i].Percent)
	}

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		for _, u := range users {
			if u.Reward.LessThanOrEqual(decimal.Zero) {
				continue
			}

			if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, u.UserID, "USDT", u.Reward, decimal.Zero); err != nil {
				return err
			}

			meta := map[string]interface{}{
				"fee_total":      feeTotal.String(),
				"pool_rate":      "0.5",
				"dividend_pool":  pool.String(),
				"equity":         u.Equity.String(),
				"equity_percent": u.Percent.String(),
				"period_start_at": func() string {
					if lastDistAt.IsZero() {
						return ""
					}
					return lastDistAt.Format(time.RFC3339)
				}(),
				"period_end_at": runAt.Format(time.RFC3339),
			}
			metaJSON, _ := json.Marshal(meta)

			_, err := tx.Model("cobo_reward_record").Ctx(ctx).Data(g.Map{
				"user_id":               u.UserID,
				"reward_type":           founderFeeDividendType,
				"amount":                u.Reward,
				"symbol":                "USDT",
				"source_user_id":        0,
				"source_wallet_address": "",
				"purchase_package_no":   "",
				"purchase_amount":       "",
				"reward_rate":           "0.50",
				"metadata":              string(metaJSON),
				"created_at":            time.Now(),
				"updated_at":            time.Now(),
			}).Insert()
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	g.Log().Infof(ctx, "[CoboFounderDividend] 发放完成: 用户=%d, 手续费总额=%s, 分红池=%s", len(users), feeTotal.String(), pool.String())
	return nil
}

func (s *founderDividendService) getLastDistributionTime(ctx context.Context) (time.Time, error) {
	v, err := g.DB().Model("cobo_reward_record").Ctx(ctx).
		Fields("MAX(created_at)").
		Where("reward_type = ?", founderFeeDividendType).
		Value()
	if err != nil {
		return time.Time{}, err
	}
	if v == nil || v.IsNil() || v.String() == "" {
		return time.Time{}, nil
	}
	if v.GTime() == nil {
		return time.Time{}, nil
	}
	return v.GTime().Time, nil
}

func (s *founderDividendService) getFeeTotal(ctx context.Context, start, end time.Time) (decimal.Decimal, error) {
	model := g.DB().Model("cobo_withdraw_request").Ctx(ctx).
		Fields("COALESCE(SUM(fee_amount), 0)").
		Where("status = ?", entityCobo.WithdrawStatusSuccess).
		Where("fee_amount > 0").
		Where("updated_at <= ?", end)

	if !start.IsZero() {
		model = model.Where("updated_at > ?", start)
	}

	v, err := model.Value()
	if err != nil {
		return decimal.Zero, err
	}
	if v == nil || v.IsNil() || v.String() == "" {
		return decimal.Zero, nil
	}
	return decimal.NewFromString(v.String())
}

func (s *founderDividendService) getFounderUsers(ctx context.Context) ([]founderUserEquity, decimal.Decimal, error) {
	rows, err := g.DB().Model("cobo_node_purchase").Ctx(ctx).
		Fields("user_id, COALESCE(SUM(stake_amount), 0) AS equity").
		Where("status = ?", entityCobo.NodeStatusRunning).
		Group("user_id").
		Order("user_id ASC").
		All()
	if err != nil {
		return nil, decimal.Zero, err
	}

	result := make([]founderUserEquity, 0, len(rows))
	total := decimal.Zero
	for _, row := range rows {
		equity, convErr := decimal.NewFromString(row["equity"].String())
		if convErr != nil || equity.LessThanOrEqual(decimal.Zero) {
			continue
		}
		u := founderUserEquity{UserID: row["user_id"].Int64(), Equity: equity}
		result = append(result, u)
		total = total.Add(equity)
	}

	return result, total, nil
}
