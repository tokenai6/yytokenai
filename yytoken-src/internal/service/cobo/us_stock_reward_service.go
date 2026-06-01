package cobo

import (
	"context"
	"time"

	"XWFrame/internal/frame/consts"
	repo "XWFrame/internal/repository/cobo"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// USStockRewardService US Stock Reward Pool 结算服务
type USStockRewardService interface {
	DistributeDaily(ctx context.Context, runAt time.Time) error
}

type usStockRewardService struct {
	balanceRepo repo.IBalanceRepository
}

// NewUSStockRewardService 创建 US Stock Reward Pool 结算服务
func NewUSStockRewardService() USStockRewardService {
	return &usStockRewardService{balanceRepo: repo.NewBalanceRepository()}
}

type poolUserWeight struct {
	UserID   int64
	Weight   decimal.Decimal
	Reward   decimal.Decimal
	Percent  decimal.Decimal
}

// DistributeDaily 按日结算 US Stock Reward Pool（仅结算昨天的 pool，当天的留给明天）
func (s *usStockRewardService) DistributeDaily(ctx context.Context, runAt time.Time) error {
	yesterday := runAt.In(cnLocation).AddDate(0, 0, -1).Format("2006-01-02")

	// 查询昨天及之前的待结算 pool
	pools, err := s.getPendingPools(ctx, yesterday)
	if err != nil {
		return err
	}
	if len(pools) == 0 {
		g.Log().Info(ctx, "[USStockReward] 无待结算 pool，跳过")
		return nil
	}

	for _, pool := range pools {
		if err := s.settlePool(ctx, pool); err != nil {
			g.Log().Errorf(ctx, "[USStockReward] 结算 pool 失败: date=%s, err=%v", pool.Date, err)
		}
	}

	return nil
}

type poolRecord struct {
	Date   string
	Amount decimal.Decimal
}

func (s *usStockRewardService) getPendingPools(ctx context.Context, beforeDate string) ([]poolRecord, error) {
	rows, err := g.DB().Ctx(ctx).Raw(`
		SELECT pool_date, COALESCE(SUM(amount), 0) AS amount
		FROM us_stock_reward_pool
		WHERE status = ? AND pool_date <= ? AND amount > 0
		GROUP BY pool_date
		ORDER BY pool_date ASC
	`, 0, beforeDate).All()
	if err != nil {
		return nil, err
	}

	result := make([]poolRecord, 0, len(rows))
	for _, row := range rows {
		amt, _ := decimal.NewFromString(row["amount"].String())
		if amt.GreaterThan(decimal.Zero) {
			result = append(result, poolRecord{Date: row["pool_date"].String(), Amount: amt})
		}
	}
	return result, nil
}

func (s *usStockRewardService) settlePool(ctx context.Context, pool poolRecord) error {
	users, totalWeight, err := s.getNodeUsers(ctx)
	if err != nil {
		return err
	}
	if len(users) == 0 || totalWeight.LessThanOrEqual(decimal.Zero) {
		g.Log().Infof(ctx, "[USStockReward] 无节点用户，跳过结算: date=%s", pool.Date)
		return s.markPoolSettled(ctx, pool.Date)
	}

	for i := range users {
		users[i].Percent = users[i].Weight.DivRound(totalWeight, 18)
		users[i].Reward = pool.Amount.Mul(users[i].Percent).Round(8)
	}

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		for _, u := range users {
			if u.Reward.LessThanOrEqual(decimal.Zero) {
				continue
			}

			// 更新用户余额
			if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, u.UserID, "USDT", u.Reward, decimal.Zero); err != nil {
				return err
			}

			// 查询余额用于账变
			bal, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, u.UserID, "USDT")
			if err != nil {
				return err
			}
			before := decimal.Zero
			if bal != nil {
				before = bal.AvailableAmount.Add(bal.FrozenAmount).Sub(u.Reward)
			}
			after := before.Add(u.Reward)

			// 写入账变
			if err := writeBalanceChangeLogTx(ctx, tx, u.UserID, "USDT", consts.ChangeTypeUSStockRewardPool, u.Reward, before, after,
				"", 0, "US Stock Reward daily settlement ("+pool.Date+")", consts.OperatorTypeSystem); err != nil {
				return err
			}

			// 写入结算明细
			_, err = tx.Exec(`
				INSERT INTO us_stock_reward_settlement (pool_date, user_id, amount, node_weight, weight_percent, created_at)
				VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
				ON CONFLICT (pool_date, user_id)
				DO UPDATE SET
					amount = EXCLUDED.amount,
					node_weight = EXCLUDED.node_weight,
					weight_percent = EXCLUDED.weight_percent,
					created_at = CURRENT_TIMESTAMP
			`, pool.Date, u.UserID, u.Reward, u.Weight, u.Percent)
			if err != nil {
				return err
			}
		}

		// 标记 pool 已结算
		return s.markPoolSettledTx(ctx, tx, pool.Date)
	})
	if err != nil {
		return err
	}

	g.Log().Infof(ctx, "[USStockReward] 结算完成: date=%s, pool=%s, users=%d", pool.Date, pool.Amount.String(), len(users))
	return nil
}

func (s *usStockRewardService) getNodeUsers(ctx context.Context) ([]poolUserWeight, decimal.Decimal, error) {
	rows, err := g.DB().Ctx(ctx).Raw(`
		SELECT p.user_id, COALESCE(SUM(p.amount * n.power_multiplier), 0) AS weight
		FROM cobo_node_purchase p
		JOIN node_info n ON n.node_type = p.node_type
		GROUP BY p.user_id
		ORDER BY p.user_id ASC
	`).All()
	if err != nil {
		return nil, decimal.Zero, err
	}

	result := make([]poolUserWeight, 0, len(rows))
	total := decimal.Zero
	for _, row := range rows {
		weight, convErr := decimal.NewFromString(row["weight"].String())
		if convErr != nil || weight.LessThanOrEqual(decimal.Zero) {
			continue
		}
		result = append(result, poolUserWeight{UserID: row["user_id"].Int64(), Weight: weight})
		total = total.Add(weight)
	}

	return result, total, nil
}

func (s *usStockRewardService) markPoolSettled(ctx context.Context, date string) error {
	_, err := g.DB().Model("us_stock_reward_pool").Ctx(ctx).
		Data(g.Map{"status": 1, "settled_at": time.Now(), "updated_at": time.Now()}).
		Where("pool_date = ?", date).
		Update()
	return err
}

func (s *usStockRewardService) markPoolSettledTx(ctx context.Context, tx gdb.TX, date string) error {
	model := tx.Model("us_stock_reward_pool").Ctx(ctx)
	_, err := model.Data(g.Map{"status": 1, "settled_at": time.Now(), "updated_at": time.Now()}).
		Where("pool_date = ?", date).
		Update()
	return err
}
