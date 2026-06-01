package group_match

import (
	"context"
	"fmt"
	"strings"
	"time"

	"XWFrame/internal/frame/consts"
	"XWFrame/internal/service/staking_v2"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

const (
	groupMatchChangeTypeTeamReward = consts.ChangeTypeGroupMatchTeamReward
)

type teamRewardUserRow struct {
	UserID                int64           `json:"user_id"`
	WalletAddress         string          `json:"wallet_address"`
	SmallTeamPerformance  decimal.Decimal `json:"small_team_performance"`
	TeamRewardPerformance decimal.Decimal `json:"team_reward_performance"`
}

func (s *groupMatchService) DistributeTeamReward(ctx context.Context, sessionID int64) error {
	return s.distributeTeamReward(ctx, sessionID)
}

func (s *groupMatchService) RedistributeTeamReward(ctx context.Context, sessionID int64, forceRefreshPerformance bool, allowZeroPerformance bool) error {
	if sessionID <= 0 {
		return gerror.New("invalid session_id")
	}
	lockKey := fmt.Sprintf("group_match:team_reward:redistribute:%d", sessionID)
	token := fmt.Sprintf("%d", time.Now().UnixNano())
	locked, err := acquireSimpleRedisLock(ctx, lockKey, token, 300)
	if err != nil {
		return err
	}
	if !locked {
		return gerror.New("team reward redistribution is busy, please retry later")
	}
	defer releaseSimpleRedisLock(ctx, lockKey, token)

	if forceRefreshPerformance {
		if _, err := g.DB().Model("group_performance").Ctx(ctx).Where("session_id", sessionID).Delete(); err != nil {
			return gerror.Wrap(err, "clear group performance cache failed")
		}
	}
	if err := s.refreshGroupPerformanceBySession(ctx, sessionID); err != nil {
		return err
	}

	if err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var session struct {
			ID int64 `json:"id"`
		}
		if err := tx.Model("group_match_session").Ctx(ctx).Fields("id").Where("id", sessionID).LockUpdate().Scan(&session); err != nil {
			return gerror.Wrap(err, "query session failed")
		}
		if session.ID == 0 {
			return gerror.Newf("session not found: %d", sessionID)
		}

		_, totalPerformance, err := s.getTeamRewardUsersTx(ctx, tx, sessionID)
		if err != nil {
			return err
		}
		if !allowZeroPerformance && !totalPerformance.GreaterThan(decimal.Zero) {
			return gerror.Newf("session %d team reward performance is zero; refuse to redistribute to avoid sinking to vertex", sessionID)
		}

		if err := s.rollbackExistingTeamRewardTx(ctx, tx, sessionID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	return s.distributeTeamReward(ctx, sessionID)
}

func (s *groupMatchService) distributeTeamReward(ctx context.Context, sessionID int64) error {
	lockKey := fmt.Sprintf("group_match:team_reward:session:%d", sessionID)
	token := fmt.Sprintf("%d", time.Now().UnixNano())
	locked, err := acquireSimpleRedisLock(ctx, lockKey, token, 180)
	if err != nil {
		return err
	}
	if !locked {
		return nil
	}
	defer releaseSimpleRedisLock(ctx, lockKey, token)

	teamRewardRate := s.mustReadDecimalConfig(ctx, "group_match_team_reward_rate", "0.20")
	enableProfit := s.mustReadBoolConfig(ctx, "group_match_team_reward_enable_profit", false)
	if !teamRewardRate.GreaterThan(decimal.Zero) {
		g.Log().Infof(ctx, "[TeamReward] session_id=%d 团队奖比例为0，跳过", sessionID)
		return nil
	}

	if err := s.refreshGroupPerformanceBySession(ctx, sessionID); err != nil {
		return err
	}

	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 幂等性检查
		already, err := tx.Model("group_match_team_reward_distribution").Ctx(ctx).Where("session_id", sessionID).Count()
		if err != nil {
			return gerror.Wrap(err, "query team reward distribution records failed")
		}
		if already > 0 {
			g.Log().Infof(ctx, "[TeamReward] session_id=%d 团队奖已分配，跳过", sessionID)
			return nil
		}

		// 1. 统计该场次所有 loser 订单本金
		loserPrincipal, err := s.sumSessionLoserPrincipalTx(ctx, tx, sessionID)
		if err != nil {
			return err
		}
		if !loserPrincipal.GreaterThan(decimal.Zero) {
			g.Log().Infof(ctx, "[TeamReward] session_id=%d 无 loser 本金，跳过", sessionID)
			return nil
		}

		poolAmount := loserPrincipal.Mul(teamRewardRate)
		if !poolAmount.GreaterThan(decimal.Zero) {
			g.Log().Infof(ctx, "[TeamReward] session_id=%d 奖池为0，跳过", sessionID)
			return nil
		}

		// 2. 查询所有团队奖业绩 > 0 的用户
		users, totalPerformance, err := s.getTeamRewardUsersTx(ctx, tx, sessionID)
		if err != nil {
			return err
		}

		// 3. 分配
		if totalPerformance.GreaterThan(decimal.Zero) && len(users) > 0 {
			allocated := decimal.Zero
			now := time.Now()
			stakingSvc := staking_v2.NewStakingV2Service()
			for i, u := range users {
				var reward decimal.Decimal
				if i == len(users)-1 {
					reward = poolAmount.Sub(allocated)
				} else {
					reward = poolAmount.Mul(u.TeamRewardPerformance).Div(totalPerformance).RoundDown(18)
					allocated = allocated.Add(reward)
				}
				if !reward.GreaterThan(decimal.Zero) {
					continue
				}

				granted := decimal.Zero
				overflow := decimal.Zero
				if enableProfit {
					grantedAmount, err := stakingSvc.ApplyRewardWithCapTx(ctx, tx, u.UserID, reward)
					if err != nil {
						return gerror.Wrap(err, "team reward limit validation failed")
					}
					granted = grantedAmount
					overflow = reward.Sub(grantedAmount)
				}

				if _, err := tx.Exec(`
					INSERT INTO group_match_team_reward_distribution
					(session_id, user_id, wallet_address, small_team_performance, team_reward_performance, total_small_team_performance, pool_amount, reward_amount, granted_amount, overflow_amount, created_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				`, sessionID, u.UserID, strings.ToLower(u.WalletAddress), u.SmallTeamPerformance, u.TeamRewardPerformance, totalPerformance, poolAmount, reward, granted, overflow, now); err != nil {
					return gerror.Wrap(err, "write team reward distribution detail failed")
				}

				if enableProfit {
					if granted.GreaterThan(decimal.Zero) {
						if err := s.creditBalanceAndLogTx(ctx, tx, u.UserID, "USDT", granted, groupMatchChangeTypeTeamReward, fmt.Sprintf("TEAM-%d", sessionID), sessionID, "group_match team reward"); err != nil {
							return err
						}
					}
					if overflow.GreaterThan(decimal.Zero) {
						if err := s.creditBalanceAndLogTx(ctx, tx, groupMatchVertexUserID, "USDT", overflow, groupMatchChangeTypeTeamReward, fmt.Sprintf("TEAM-%d", sessionID), sessionID, "group_match team reward overflow to vertex"); err != nil {
							return err
						}
					}
				}
			}
		} else {
			// 全网团队奖业绩为0，沉淀到顶号
			g.Log().Infof(ctx, "[TeamReward] session_id=%d 全网团队奖业绩为0，奖池 %s 沉淀到顶号", sessionID, poolAmount.String())
			vertexWallet, err := s.getWalletAddressByUserID(ctx, tx, groupMatchVertexUserID)
			if err != nil {
				return err
			}
			now := time.Now()
			granted := decimal.Zero
			overflow := decimal.Zero
			if enableProfit {
				granted = poolAmount
			}
			if _, err := tx.Exec(`
				INSERT INTO group_match_team_reward_distribution
				(session_id, user_id, wallet_address, small_team_performance, team_reward_performance, total_small_team_performance, pool_amount, reward_amount, granted_amount, overflow_amount, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, sessionID, groupMatchVertexUserID, strings.ToLower(vertexWallet), decimal.Zero, decimal.Zero, decimal.Zero, poolAmount, poolAmount, granted, overflow, now); err != nil {
				return gerror.Wrap(err, "write team reward accumulation detail failed")
			}
			if enableProfit {
				if err := s.creditBalanceAndLogTx(ctx, tx, groupMatchVertexUserID, "USDT", poolAmount, groupMatchChangeTypeTeamReward, fmt.Sprintf("TEAM-%d", sessionID), sessionID, "group_match team reward sink to vertex"); err != nil {
					return err
				}
			}
		}

		if !enableProfit {
			g.Log().Infof(ctx, "[TeamReward] session_id=%d 团队奖发放开关关闭，仅写分配明细不入账", sessionID)
		}

		g.Log().Infof(ctx, "[TeamReward] session_id=%d 团队奖分配完成: loser_principal=%s, pool=%s, users=%d, total_perf=%s",
			sessionID, loserPrincipal.String(), poolAmount.String(), len(users), totalPerformance.String())
		return nil
	})
}

func (s *groupMatchService) rollbackExistingTeamRewardTx(ctx context.Context, tx gdb.TX, sessionID int64) error {
	type distributionRow struct {
		UserID        int64           `json:"user_id"`
		GrantedAmount decimal.Decimal `json:"granted_amount"`
	}
	distributions := make([]*distributionRow, 0)
	if err := tx.Model("group_match_team_reward_distribution").Ctx(ctx).
		Fields("user_id, granted_amount").
		Where("session_id", sessionID).
		OrderAsc("id").
		Scan(&distributions); err != nil {
		return gerror.Wrap(err, "query existing team reward distributions failed")
	}

	grantedByUser := make(map[int64]decimal.Decimal)
	for _, row := range distributions {
		if row == nil || row.UserID <= 0 || !row.GrantedAmount.GreaterThan(decimal.Zero) {
			continue
		}
		grantedByUser[row.UserID] = grantedByUser[row.UserID].Add(row.GrantedAmount)
	}
	for userID, amount := range grantedByUser {
		if err := s.rollbackStakingRewardCapTx(ctx, tx, userID, amount); err != nil {
			return err
		}
	}

	type balanceLogRow struct {
		UserID int64           `json:"user_id"`
		Symbol string          `json:"symbol"`
		Amount decimal.Decimal `json:"amount"`
	}
	balanceRows := make([]*balanceLogRow, 0)
	if err := tx.Model("cobo_balance_change_log").Ctx(ctx).
		Fields("user_id,symbol,amount").
		Where("related_id", sessionID).
		Where("change_type", groupMatchChangeTypeTeamReward).
		OrderAsc("id").
		Scan(&balanceRows); err != nil {
		return gerror.Wrap(err, "query existing team reward balance logs failed")
	}

	type balanceKey struct {
		userID int64
		symbol string
	}
	rollbackMap := make(map[balanceKey]decimal.Decimal)
	for _, row := range balanceRows {
		if row == nil || row.UserID <= 0 || !row.Amount.GreaterThan(decimal.Zero) {
			continue
		}
		key := balanceKey{userID: row.UserID, symbol: row.Symbol}
		rollbackMap[key] = rollbackMap[key].Add(row.Amount)
	}
	for key, amount := range rollbackMap {
		balance, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, key.userID, key.symbol)
		if err != nil {
			return gerror.Wrap(err, "query balance before team reward rollback failed")
		}
		if balance == nil || balance.AvailableAmount.LessThan(amount) {
			return gerror.Newf("insufficient balance for team reward rollback: user_id=%d symbol=%s need=%s", key.userID, key.symbol, amount.String())
		}
		if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, key.userID, key.symbol, amount.Neg(), decimal.Zero); err != nil {
			return gerror.Wrap(err, "rollback team reward balance failed")
		}
	}

	if _, err := tx.Model("cobo_balance_change_log").Ctx(ctx).
		Where("related_id", sessionID).
		Where("change_type", groupMatchChangeTypeTeamReward).
		Delete(); err != nil {
		return gerror.Wrap(err, "delete existing team reward balance logs failed")
	}
	if _, err := tx.Model("group_match_team_reward_distribution").Ctx(ctx).Where("session_id", sessionID).Delete(); err != nil {
		return gerror.Wrap(err, "delete existing team reward distributions failed")
	}
	return nil
}

func (s *groupMatchService) rollbackStakingRewardCapTx(ctx context.Context, tx gdb.TX, userID int64, amount decimal.Decimal) error {
	if userID <= 0 || !amount.GreaterThan(decimal.Zero) {
		return nil
	}

	type orderRow struct {
		ID          int64           `json:"id"`
		Amount      decimal.Decimal `json:"amount"`
		TotalReward decimal.Decimal `json:"total_reward"`
	}
	orders := make([]*orderRow, 0)
	if err := tx.Model("staking_v2_order").Ctx(ctx).
		Fields("id,amount,total_reward").
		Where("user_id = ? AND total_reward > 0", userID).
		OrderDesc("id").
		LockUpdate().
		Scan(&orders); err != nil {
		return gerror.Wrap(err, "query staking orders before cap rollback failed")
	}

	remaining := amount
	for _, order := range orders {
		if order == nil || !remaining.GreaterThan(decimal.Zero) {
			break
		}
		rollback := remaining
		if rollback.GreaterThan(order.TotalReward) {
			rollback = order.TotalReward
		}
		newTotalReward := order.TotalReward.Sub(rollback)
		orderLimit := order.Amount.Mul(decimal.NewFromInt(4))
		status := consts.StakingV2StatusActive
		if !newTotalReward.LessThan(orderLimit) {
			status = consts.StakingV2StatusCapped
		}
		if _, err := tx.Model("staking_v2_order").Ctx(ctx).Where("id", order.ID).Data(g.Map{
			"total_reward": newTotalReward,
			"status":       status,
			"updated_at":   time.Now(),
		}).Update(); err != nil {
			return gerror.Wrap(err, "rollback staking order cap failed")
		}
		remaining = remaining.Sub(rollback)
	}
	if remaining.GreaterThan(decimal.Zero) {
		return gerror.Newf("staking cap rollback exceeds recorded order reward: user_id=%d amount=%s remaining=%s", userID, amount.String(), remaining.String())
	}

	var stats struct {
		TotalRewardEarned decimal.Decimal `json:"total_reward_earned"`
		RewardLimit       decimal.Decimal `json:"reward_limit"`
	}
	if err := tx.Model("staking_v2_user_stats").Ctx(ctx).
		Fields("total_reward_earned,reward_limit").
		Where("user_id", userID).
		LockUpdate().
		Scan(&stats); err != nil {
		return gerror.Wrap(err, "query staking stats before cap rollback failed")
	}
	newRewardEarned := stats.TotalRewardEarned.Sub(amount)
	if newRewardEarned.LessThan(decimal.Zero) {
		newRewardEarned = decimal.Zero
	}
	isCapped := stats.RewardLimit.GreaterThan(decimal.Zero) && !newRewardEarned.LessThan(stats.RewardLimit)
	data := g.Map{
		"total_reward_earned": newRewardEarned,
		"is_capped":           isCapped,
		"updated_at":          time.Now(),
	}
	if !isCapped {
		data["capped_at"] = nil
	}
	if _, err := tx.Model("staking_v2_user_stats").Ctx(ctx).Where("user_id", userID).Data(data).Update(); err != nil {
		return gerror.Wrap(err, "rollback staking stats cap failed")
	}
	return nil
}

func (s *groupMatchService) sumSessionLoserPrincipalTx(ctx context.Context, tx gdb.TX, sessionID int64) (decimal.Decimal, error) {
	row, err := tx.GetOne(`
		SELECT loser_principal_total AS amount
		FROM group_match_session
		WHERE id = ?
	`, sessionID)
	if err != nil {
		return decimal.Zero, gerror.Wrap(err, "query session loser principal failed")
	}
	amount, _ := decimal.NewFromString(row["amount"].String())
	return amount, nil
}

func (s *groupMatchService) getTeamRewardUsersTx(ctx context.Context, tx gdb.TX, sessionID int64) ([]*teamRewardUserRow, decimal.Decimal, error) {
	result, err := tx.GetAll(`
		SELECT gp.user_id,
		       COALESCE(gp.wallet_address, '') AS wallet_address,
		       COALESCE(gp.small_team_group_amount, 0) AS small_team_performance,
		       COALESCE(CASE WHEN u.use_full_perf THEN gp.team_group_amount ELSE gp.small_team_group_amount END, 0) AS team_reward_performance
		FROM group_performance gp
		JOIN user_info u ON u.id = gp.user_id
		WHERE gp.session_id = ?
		  AND u.status = 1
		  AND COALESCE(CASE WHEN u.use_full_perf THEN gp.team_group_amount ELSE gp.small_team_group_amount END, 0) > 0
		ORDER BY gp.user_id ASC
	`, sessionID)
	if err != nil {
		return nil, decimal.Zero, gerror.Wrap(err, "query team loser performance users failed")
	}

	rows := make([]*teamRewardUserRow, 0, len(result))
	for _, record := range result {
		smallPerf, err := decimal.NewFromString(record["small_team_performance"].String())
		if err != nil {
			return nil, decimal.Zero, gerror.Wrapf(err, "parse small team performance failed: user_id=%d", record["user_id"].Int64())
		}
		rewardPerf, err := decimal.NewFromString(record["team_reward_performance"].String())
		if err != nil {
			return nil, decimal.Zero, gerror.Wrapf(err, "parse team reward performance failed: user_id=%d", record["user_id"].Int64())
		}
		rows = append(rows, &teamRewardUserRow{
			UserID:                record["user_id"].Int64(),
			WalletAddress:         record["wallet_address"].String(),
			SmallTeamPerformance:  smallPerf,
			TeamRewardPerformance: rewardPerf,
		})
	}

	total := decimal.Zero
	for _, r := range rows {
		total = total.Add(r.TeamRewardPerformance)
	}
	return rows, total, nil
}

func (s *groupMatchService) getSessionBizDate(ctx context.Context, sessionID int64) (time.Time, error) {
	row, err := g.DB().Ctx(ctx).Raw("SELECT session_date FROM group_match_session WHERE id = ?", sessionID).One()
	if err != nil {
		return time.Time{}, gerror.Wrap(err, "query session date failed")
	}
	if row.IsEmpty() {
		return time.Time{}, gerror.Newf("session not found: %d", sessionID)
	}
	return dateOnlyInChina(row["session_date"].Time()), nil
}
