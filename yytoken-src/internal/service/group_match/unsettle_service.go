package group_match

import (
	"context"
	"fmt"
	"time"

	"XWFrame/internal/frame/consts"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

const groupMatchUnsettleLockTTLSeconds = 180

func (s *groupMatchService) UnsettleSessionByID(ctx context.Context, sessionID int64) error {
	lockKey := fmt.Sprintf("group_match:unsettle:session:%d", sessionID)
	token := fmt.Sprintf("%d", time.Now().UnixNano())
	locked, err := acquireSimpleRedisLock(ctx, lockKey, token, groupMatchUnsettleLockTTLSeconds)
	if err != nil {
		return err
	}
	if !locked {
		return gerror.New("session is busy with another operation, please retry later")
	}
	defer releaseSimpleRedisLock(ctx, lockKey, token)

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		type sessionRow struct {
			ID                  int64           `json:"id"`
			Status              int             `json:"status"`
			SessionDate         time.Time       `json:"session_date"`
			LoserPrincipalTotal decimal.Decimal `json:"loser_principal_total"`
		}
		session := &sessionRow{}
		if err := tx.Model("group_match_session").Ctx(ctx).
			Fields("id,status,session_date,loser_principal_total").
			Where("id", sessionID).
			LockUpdate().
			Scan(session); err != nil {
			return gerror.Wrap(err, "query session failed")
		}
		if session.ID == 0 {
			return gerror.New("session not found")
		}
		if session.Status != consts.GroupMatchSessionStatusCompleted {
			return gerror.New("only settled sessions can be unsettled")
		}

		releaseCount, err := tx.GetValue(`
			SELECT COUNT(1)
			FROM group_match_loser_comp_release_log l
			JOIN group_match_loser_compensation c ON c.id = l.compensation_id
			WHERE c.session_id = ?
		`, sessionID)
		if err != nil {
			return gerror.Wrap(err, "check compensation release records failed")
		}
		if releaseCount.Int64() > 0 {
			return gerror.New("session has compensation release records, unsettlement not allowed")
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
			WhereIn("change_type", []string{
				groupMatchChangeTypeWinnerPayout,
				groupMatchChangeTypeFlowRefund,
				groupMatchChangeTypeTicketRefund,
				groupMatchChangeTypeTicketBurnSink,
				groupMatchChangeTypeTeamReward,
			}).
			OrderAsc("id").
			Scan(&balanceRows); err != nil {
			return gerror.Wrap(err, "query rollback balance logs failed")
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
			k := balanceKey{userID: row.UserID, symbol: row.Symbol}
			rollbackMap[k] = rollbackMap[k].Add(row.Amount)
		}

		for k, rollbackAmount := range rollbackMap {
			if !rollbackAmount.GreaterThan(decimal.Zero) {
				continue
			}
			balance, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, k.userID, k.symbol)
			if err != nil {
				return gerror.Wrap(err, "query balance before rollback failed")
			}
			if balance == nil || balance.AvailableAmount.LessThan(rollbackAmount) {
				return gerror.Newf("insufficient user balance for rollback: user_id=%d symbol=%s need=%s", k.userID, k.symbol, rollbackAmount.String())
			}
			if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, k.userID, k.symbol, rollbackAmount.Neg(), decimal.Zero); err != nil {
				return gerror.Wrap(err, "rollback user balance failed")
			}
		}

		leaderBatchID := "SV2-LDR-" + session.SessionDate.Format("20060102")
		leaderRewardRows := make([]*balanceLogRow, 0)
		if err := tx.Model("cobo_balance_change_log").Ctx(ctx).
			Fields("user_id,symbol,amount").
			Where("change_type", consts.ChangeTypeStakingV2LeaderReward).
			Where("related_order_no", leaderBatchID).
			OrderAsc("id").
			Scan(&leaderRewardRows); err != nil {
			return gerror.Wrap(err, "query staking v2 leader reward rollback logs failed")
		}

		leaderRollbackMap := make(map[balanceKey]decimal.Decimal)
		for _, row := range leaderRewardRows {
			if row == nil || row.UserID <= 0 || !row.Amount.GreaterThan(decimal.Zero) {
				continue
			}
			k := balanceKey{userID: row.UserID, symbol: row.Symbol}
			leaderRollbackMap[k] = leaderRollbackMap[k].Add(row.Amount)
		}

		for k, rollbackAmount := range leaderRollbackMap {
			if !rollbackAmount.GreaterThan(decimal.Zero) {
				continue
			}
			balance, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, k.userID, k.symbol)
			if err != nil {
				return gerror.Wrap(err, "query leader reward balance before rollback failed")
			}
			if balance == nil || balance.AvailableAmount.LessThan(rollbackAmount) {
				return gerror.Newf("insufficient user balance for leader reward rollback: user_id=%d symbol=%s need=%s", k.userID, k.symbol, rollbackAmount.String())
			}
			if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, k.userID, k.symbol, rollbackAmount.Neg(), decimal.Zero); err != nil {
				return gerror.Wrap(err, "rollback staking v2 leader reward balance failed")
			}
		}

		if _, err := tx.Model("cobo_balance_change_log").Ctx(ctx).
			Where("related_id", sessionID).
			WhereIn("change_type", []string{
				groupMatchChangeTypeWinnerPayout,
				groupMatchChangeTypeFlowRefund,
				groupMatchChangeTypeTicketRefund,
				groupMatchChangeTypeTicketBurnSink,
				groupMatchChangeTypeTeamReward,
			}).
			Delete(); err != nil {
			return gerror.Wrap(err, "delete balance change logs failed")
		}

		if _, err := tx.Model("cobo_balance_change_log").Ctx(ctx).
			Where("change_type", consts.ChangeTypeStakingV2LeaderReward).
			Where("related_order_no", leaderBatchID).
			Delete(); err != nil {
			return gerror.Wrap(err, "delete staking v2 leader reward balance logs failed")
		}

		if _, err := tx.Model("staking_v2_leader_reward_detail").Ctx(ctx).
			Where("biz_date", session.SessionDate).
			Delete(); err != nil {
			return gerror.Wrap(err, "delete staking v2 leader reward detail failed")
		}

		if _, err := tx.Model("group_match_team_reward_distribution").Ctx(ctx).Where("session_id", sessionID).Delete(); err != nil {
			return gerror.Wrap(err, "delete team reward details failed")
		}

		if session.LoserPrincipalTotal.GreaterThan(decimal.Zero) {
			rateValue, err := tx.GetValue("SELECT rate FROM group_purchase_leadership_pool WHERE biz_date = ? AND source_type = 'group_match_loser' LIMIT 1", session.SessionDate)
			if err != nil {
				return gerror.Wrap(err, "query leadership reward pool ratio failed")
			}
			if !rateValue.IsNil() {
				rate, parseErr := decimal.NewFromString(rateValue.String())
				if parseErr != nil {
					return gerror.Wrap(parseErr, "parse leadership reward pool ratio failed")
				}
				deductSourceAmount := session.LoserPrincipalTotal
				deductPoolAmount := session.LoserPrincipalTotal.Mul(rate)
				if _, err := tx.Exec(`
					UPDATE group_purchase_leadership_pool
					SET source_amount = GREATEST(source_amount - ?, 0),
						amount = GREATEST(amount - ?, 0),
						updated_at = ?
					WHERE biz_date = ? AND source_type = 'group_match_loser'
				`, deductSourceAmount, deductPoolAmount, time.Now(), session.SessionDate); err != nil {
					return gerror.Wrap(err, "rollback leadership reward pool amount failed")
				}
			}
		}

		if _, err := tx.Exec(`
			DELETE FROM group_match_loser_comp_release_log
			WHERE compensation_id IN (
				SELECT id FROM group_match_loser_compensation WHERE session_id = ?
			)
		`, sessionID); err != nil {
			return gerror.Wrap(err, "delete compensation release logs failed")
		}

		if _, err := tx.Model("group_match_loser_compensation").Ctx(ctx).Where("session_id", sessionID).Delete(); err != nil {
			return gerror.Wrap(err, "delete loser compensation records failed")
		}

		if _, err := tx.Exec(`
			DELETE FROM group_match_group_member
			WHERE group_id IN (
				SELECT id FROM group_match_group WHERE session_id = ?
			)
		`, sessionID); err != nil {
			return gerror.Wrap(err, "delete group match member records failed")
		}

		if _, err := tx.Model("group_match_group").Ctx(ctx).Where("session_id", sessionID).Delete(); err != nil {
			return gerror.Wrap(err, "delete group match draw records failed")
		}

		if _, err := tx.Model("group_match_order").Ctx(ctx).Where("session_id", sessionID).Data(g.Map{
			"status":                  consts.GroupMatchOrderStatusPending,
			"group_id":                nil,
			"is_winner":               nil,
			"reward_amount":           decimal.Zero,
			"refund_amount":           decimal.Zero,
			"ticket_return_amount":    decimal.Zero,
			"ticket_burn_amount":      decimal.Zero,
			"loser_comp_usdt_value":   decimal.Zero,
			"loser_comp_symbol":       "",
			"loser_comp_amount":       decimal.Zero,
			"loser_comp_release_days": 0,
			"updated_at":              time.Now(),
		}).Update(); err != nil {
			return gerror.Wrap(err, "reset order status failed")
		}

		if _, err := tx.Model("group_match_session").Ctx(ctx).Where("id", sessionID).Data(g.Map{
			"status":                consts.GroupMatchSessionStatusActive,
			"total_groups":          0,
			"total_flow_user_count": 0,
			"loser_principal_total": decimal.Zero,
			"settled_at":            nil,
			"updated_at":            time.Now(),
		}).Update(); err != nil {
			return gerror.Wrap(err, "reset session status failed")
		}

		return nil
	})
	if err == nil {
		s.markGroupPerformanceDirty(ctx, sessionID)
		s.asyncRefreshGroupPerformance(ctx, sessionID)
	}
	return err
}
