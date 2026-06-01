package group_match

import (
	"context"
	"fmt"
	"time"

	"XWFrame/internal/frame/consts"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

const groupMatchClearLockTTLSeconds = 180

func (s *groupMatchService) ClearSessionByID(ctx context.Context, sessionID int64) error {
	lockKey := fmt.Sprintf("group_match:clear:session:%d", sessionID)
	token := fmt.Sprintf("%d", time.Now().UnixNano())
	locked, err := acquireSimpleRedisLock(ctx, lockKey, token, groupMatchClearLockTTLSeconds)
	if err != nil {
		return err
	}
	if !locked {
		return gerror.New("session is busy with another operation, please retry later")
	}
	defer releaseSimpleRedisLock(ctx, lockKey, token)

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		type sessionRow struct {
			ID     int64 `json:"id"`
			Status int   `json:"status"`
		}
		session := &sessionRow{}
		if err := tx.Model("group_match_session").Ctx(ctx).
			Fields("id,status").
			Where("id", sessionID).
			LockUpdate().
			Scan(session); err != nil {
			return gerror.Wrap(err, "query session failed")
		}
		if session.ID == 0 {
			return gerror.New("session not found")
		}
		if session.Status == consts.GroupMatchSessionStatusSettling {
			return gerror.New("cannot clear a session while settling")
		}

		if _, err := tx.Exec(`
			WITH delta AS (
				SELECT user_id, symbol, SUM(amount) AS net_amount
				FROM cobo_balance_change_log
				WHERE related_id = ?
				  AND change_type LIKE 'group_match_%'
				GROUP BY user_id, symbol
			)
			UPDATE cobo_balance b
			SET available_amount = b.available_amount - d.net_amount,
			    updated_at = CURRENT_TIMESTAMP
			FROM delta d
			WHERE b.user_id = d.user_id
			  AND b.symbol = d.symbol
		`, sessionID); err != nil {
			return gerror.Wrap(err, "rollback group match balances failed")
		}

		if _, err := tx.Model("cobo_balance_change_log").Ctx(ctx).
			Where("related_id", sessionID).
			WhereLike("change_type", "group_match_%").
			Delete(); err != nil {
			return gerror.Wrap(err, "delete group match balance logs failed")
		}

		if _, err := tx.Model("group_match_team_reward_distribution").Ctx(ctx).Where("session_id", sessionID).Delete(); err != nil {
			return gerror.Wrap(err, "delete team reward details failed")
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
			return gerror.Wrap(err, "delete group member records failed")
		}

		if _, err := tx.Model("group_match_group").Ctx(ctx).Where("session_id", sessionID).Delete(); err != nil {
			return gerror.Wrap(err, "delete group records failed")
		}

		if _, err := tx.Model("group_match_order").Ctx(ctx).Where("session_id", sessionID).Delete(); err != nil {
			return gerror.Wrap(err, "delete order records failed")
		}

		if _, err := tx.Model("group_match_session").Ctx(ctx).Where("id", sessionID).Data(g.Map{
			"status":                consts.GroupMatchSessionStatusPending,
			"total_orders":          0,
			"total_groups":          0,
			"total_flow_user_count": 0,
			"loser_principal_total": 0,
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
