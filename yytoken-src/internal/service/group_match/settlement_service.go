package group_match

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"XWFrame/internal/frame/consts"
	"XWFrame/internal/service/cobo"
	"XWFrame/internal/service/staking_v2"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

const (
	groupMatchSettleLockTTLSeconds = 180

	groupMatchChangeTypeWinnerPayout   = consts.ChangeTypeGroupMatchSettleWinner
	groupMatchChangeTypeFlowRefund     = consts.ChangeTypeGroupMatchSettleFlowRefund
	groupMatchChangeTypeTicketRefund   = consts.ChangeTypeGroupMatchSettleTicketRefund
	groupMatchChangeTypeTicketBurnSink = consts.ChangeTypeGroupMatchSettleTicketBurnToVtx
	groupMatchChangeTypeLoserComp      = consts.ChangeTypeGroupMatchLoserCompRelease
	groupMatchLoserCompStatusReleasing = 0
	groupMatchLoserCompStatusDone      = 1
	groupMatchVertexUserID             = int64(1)
)

type settlementOrderRow struct {
	ID              int64           `json:"id"`
	UserID          int64           `json:"user_id"`
	OrderNo         string          `json:"order_no"`
	Amount          decimal.Decimal `json:"amount"`
	TicketSymbol    string          `json:"ticket_symbol"`
	TicketAmount    decimal.Decimal `json:"ticket_amount"`
	TicketUSDTValue decimal.Decimal `json:"ticket_usdt_value" orm:"ticket_usdt_value"`
	CreatedAt       time.Time       `json:"created_at"`
	CurrentStatus   int             `json:"status"`
	CurrentGroupID  int64           `json:"group_id"`
}

type loserCompensationRow struct {
	ID             int64           `json:"id"`
	OrderID        int64           `json:"order_id"`
	UserID         int64           `json:"user_id"`
	TokenSymbol    string          `json:"token_symbol"`
	PendingAmount  decimal.Decimal `json:"pending_amount"`
	ReleasedAmount decimal.Decimal `json:"released_amount"`
	ReleaseDays    int             `json:"release_days"`
	StartDate      time.Time       `json:"start_date"`
	EndDate        time.Time       `json:"end_date"`
	Status         int             `json:"status"`
	NextReleaseAt  *time.Time      `json:"next_release_at"`
	WalletAddress  string          `json:"wallet_address"`
	CompUSDTValue  decimal.Decimal `json:"usdt_value"`
}

type balanceCacheKey struct {
	UserID int64
	Symbol string
}

type balanceCacheState struct {
	Available decimal.Decimal
}

func (s *groupMatchService) ActivatePendingSessions(ctx context.Context) (int64, error) {
	result, err := g.DB().Exec(ctx, `
		UPDATE group_match_session
		SET status = ?
		WHERE status = ? AND start_time <= ?
	`, consts.GroupMatchSessionStatusActive, consts.GroupMatchSessionStatusPending, time.Now())
	if err != nil {
		return 0, gerror.Wrap(err, "update session status failed")
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

func (s *groupMatchService) SettleDueSessions(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 20
	}
	g.Log().Infof(ctx, "[GroupMatch] settle due sessions start: limit=%d now=%s", limit, time.Now().Format(time.RFC3339))

	type sessionRow struct {
		ID int64 `json:"id"`
	}
	rows := make([]*sessionRow, 0)
	err := g.DB().Model("group_match_session").Ctx(ctx).
		Fields("id").
		Where("status IN (?, ?)", consts.GroupMatchSessionStatusActive, consts.GroupMatchSessionStatusSettling).
		Where("end_time <= ?", time.Now()).
		OrderAsc("end_time").
		Limit(limit).
		Scan(&rows)
	if err != nil {
		return 0, gerror.Wrap(err, "query pending settlement sessions failed")
	}
	if len(rows) == 0 {
		g.Log().Infof(ctx, "[GroupMatch] settle due sessions skip: no due session")
		return 0, nil
	}
	g.Log().Infof(ctx, "[GroupMatch] settle due sessions found: count=%d", len(rows))

	settled := 0
	for _, row := range rows {
		if row == nil || row.ID <= 0 {
			continue
		}
		g.Log().Infof(ctx, "[GroupMatch] settle due session begin: session_id=%d", row.ID)
		didSettle, err := s.settleSingleSession(ctx, row.ID, false)
		if err != nil {
			return settled, err
		}
		if didSettle {
			settled++
			g.Log().Infof(ctx, "[GroupMatch] settle due session done: session_id=%d", row.ID)
		} else {
			g.Log().Infof(ctx, "[GroupMatch] settle due session skipped: session_id=%d", row.ID)
		}
	}
	g.Log().Infof(ctx, "[GroupMatch] settle due sessions finished: settled=%d", settled)

	return settled, nil
}

func (s *groupMatchService) ReleaseLoserCompensations(ctx context.Context, limit int) (int, error) {
	return s.releaseLoserCompensationsWithMinNextReleaseAt(ctx, limit, nil)
}

func (s *groupMatchService) releaseLoserCompensationsAtNow(ctx context.Context, limit int, now time.Time) (int, error) {
	if limit <= 0 {
		limit = 500
	}

	type idRow struct {
		ID int64 `json:"id"`
	}
	rows := make([]*idRow, 0)
	err := g.DB().Model("group_match_loser_compensation").Ctx(ctx).
		Fields("id").
		Where("status = ?", groupMatchLoserCompStatusReleasing).
		Where("next_release_at IS NULL OR next_release_at <= ?", now).
		OrderAsc("id").
		Limit(limit).
		Scan(&rows)
	if err != nil {
		return 0, gerror.Wrap(err, "query pending compensation release failed")
	}

	releasedCount := 0
	for _, row := range rows {
		if row == nil || row.ID <= 0 {
			continue
		}
		ok, releaseErr := s.releaseSingleCompensationWithBaseNow(ctx, row.ID, now, true)
		if releaseErr != nil {
			return releasedCount, releaseErr
		}
		if ok {
			releasedCount++
		}
	}
	return releasedCount, nil
}

func (s *groupMatchService) ReleaseLoserCompensationsSince(ctx context.Context, limit int, minNextReleaseAt time.Time) (int, error) {
	return s.releaseLoserCompensationsWithMinNextReleaseAt(ctx, limit, &minNextReleaseAt)
}

func (s *groupMatchService) ReleaseLoserCompensationsBefore(ctx context.Context, limit int, maxNextReleaseAt time.Time) (int, error) {
	return s.releaseLoserCompensationsWithMaxNextReleaseAt(ctx, limit, &maxNextReleaseAt)
}

func (s *groupMatchService) releaseLoserCompensationsWithMaxNextReleaseAt(ctx context.Context, limit int, maxNextReleaseAt *time.Time) (int, error) {
	if limit <= 0 {
		limit = 500
	}

	type idRow struct {
		ID int64 `json:"id"`
	}
	rows := make([]*idRow, 0)
	m := g.DB().Model("group_match_loser_compensation").Ctx(ctx).
		Fields("id").
		Where("status = ?", groupMatchLoserCompStatusReleasing).
		Where("next_release_at IS NULL OR next_release_at <= ?", time.Now()).
		OrderAsc("id")
	if maxNextReleaseAt != nil {
		m = m.Where("next_release_at < ?", *maxNextReleaseAt)
	}
	err := m.Limit(limit).Scan(&rows)
	if err != nil {
		return 0, gerror.Wrap(err, "query pending compensation release failed")
	}

	releasedCount := 0
	for _, row := range rows {
		if row == nil || row.ID <= 0 {
			continue
		}
		ok, releaseErr := s.releaseSingleCompensation(ctx, row.ID)
		if releaseErr != nil {
			g.Log().Errorf(ctx, "[GroupMatch] releaseSingleCompensation failed: compensation_id=%d err=%v", row.ID, releaseErr)
			_, updateErr := g.DB().Model("group_match_loser_compensation").Ctx(ctx).
				Where("id", row.ID).
				Where("status = ?", groupMatchLoserCompStatusReleasing).
				Data(g.Map{"next_release_at": time.Now().Add(time.Hour), "updated_at": time.Now()}).
				Update()
			if updateErr != nil {
				g.Log().Errorf(ctx, "[GroupMatch] push back failed compensation record failed: compensation_id=%d err=%v", row.ID, updateErr)
			}
			continue
		}
		if ok {
			releasedCount++
		}
	}
	return releasedCount, nil
}

func (s *groupMatchService) releaseLoserCompensationsWithMinNextReleaseAt(ctx context.Context, limit int, minNextReleaseAt *time.Time) (int, error) {
	if limit <= 0 {
		limit = 500
	}

	type idRow struct {
		ID int64 `json:"id"`
	}
	rows := make([]*idRow, 0)
	m := g.DB().Model("group_match_loser_compensation").Ctx(ctx).
		Fields("id").
		Where("status = ?", groupMatchLoserCompStatusReleasing).
		Where("next_release_at IS NULL OR next_release_at <= ?", time.Now()).
		OrderAsc("id")
	if minNextReleaseAt != nil {
		m = m.Where("next_release_at >= ?", *minNextReleaseAt)
	}
	err := m.Limit(limit).Scan(&rows)
	if err != nil {
		return 0, gerror.Wrap(err, "query pending compensation release failed")
	}

	releasedCount := 0
	for _, row := range rows {
		if row == nil || row.ID <= 0 {
			continue
		}
		ok, releaseErr := s.releaseSingleCompensation(ctx, row.ID)
		if releaseErr != nil {
			g.Log().Errorf(ctx, "[GroupMatch] releaseSingleCompensation failed: compensation_id=%d err=%v", row.ID, releaseErr)
			_, updateErr := g.DB().Model("group_match_loser_compensation").Ctx(ctx).
				Where("id", row.ID).
				Where("status = ?", groupMatchLoserCompStatusReleasing).
				Data(g.Map{"next_release_at": time.Now().Add(time.Hour), "updated_at": time.Now()}).
				Update()
			if updateErr != nil {
				g.Log().Errorf(ctx, "[GroupMatch] push back failed compensation record failed: compensation_id=%d err=%v", row.ID, updateErr)
			}
			continue
		}
		if ok {
			releasedCount++
		}
	}
	return releasedCount, nil
}

func (s *groupMatchService) settleSingleSession(ctx context.Context, sessionID int64, skipEndTimeCheck bool) (bool, error) {
	lockKey := fmt.Sprintf("group_match:settle:session:%d", sessionID)
	token := utils.GenerateSnowflakeId()
	locked, err := acquireSimpleRedisLock(ctx, lockKey, token, groupMatchSettleLockTTLSeconds)
	if err != nil {
		return false, err
	}
	if !locked {
		g.Log().Infof(ctx, "[GroupMatch] settle skipped by lock: session_id=%d lock_key=%s", sessionID, lockKey)
		return false, nil
	}
	g.Log().Infof(ctx, "[GroupMatch] settle lock acquired: session_id=%d lock_key=%s", sessionID, lockKey)
	defer releaseSimpleRedisLock(ctx, lockKey, token)

	winnerRate := s.mustReadDecimalConfig(ctx, "group_match_winner_reward_rate", "0.02")
	winnerTicketTax := s.mustReadDecimalConfig(ctx, "group_match_winner_ticket_tax", "0.3")
	loserCompUSDT := s.mustReadDecimalConfig(ctx, "group_match_loser_compensation_usdt", "150")
	loserReleaseDays := s.mustReadIntConfig(ctx, "group_match_loser_release_days", 45)
	batchGroups := s.mustReadIntConfig(ctx, "group_match_settlement_batch_groups", 20)
	teamPoolRate := s.mustReadDecimalConfig(ctx, "group_purchase_leadership_pool_rate", "0.12")
	if loserReleaseDays <= 0 {
		loserReleaseDays = 45
	}
	if batchGroups <= 0 {
		batchGroups = 20
	}
	g.Log().Infof(ctx, "[GroupMatch] settle config loaded: session_id=%d winner_rate=%s winner_ticket_tax_usdt=%s loser_comp_usdt=%s loser_release_days=%d leadership_pool_rate=%s",
		sessionID,
		winnerRate.String(),
		winnerTicketTax.String(),
		loserCompUSDT.String(),
		loserReleaseDays,
		teamPoolRate.String(),
	)

	canSettle, err := s.prepareSessionForSettlement(ctx, sessionID, skipEndTimeCheck)
	if err != nil {
		return false, err
	}
	if !canSettle {
		g.Log().Infof(ctx, "[GroupMatch] settle skipped by status/time: session_id=%d", sessionID)
		return false, nil
	}

	type sessionMeta struct {
		SessionName string    `json:"session_name"`
		SessionDate time.Time `json:"session_date"`
	}
	meta := &sessionMeta{}
	if err := g.DB().Model("group_match_session").Ctx(ctx).Fields("session_name,session_date").Where("id", sessionID).Scan(meta); err != nil {
		return false, gerror.Wrap(err, "query session meta failed")
	}

	plan, err := s.buildSettlementPlan(ctx, sessionID)
	if err != nil {
		return false, err
	}
	totalPendingOrders := 0
	for _, p := range plan {
		totalPendingOrders += len(p.Orders)
	}
	g.Log().Infof(ctx, "[GroupMatch] settlement plan built: session_id=%d total_orders=%d total_plans=%d", sessionID, totalPendingOrders, len(plan))

	go func(sid int64, name string, orders, plans int) {
		notifyCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cobo.GetTelegramNotifyService(notifyCtx).NotifyGroupMatchSettlementStarted(notifyCtx, sid, name, orders, plans)
	}(sessionID, meta.SessionName, totalPendingOrders, len(plan))

	compNextReleaseAt, err := s.resolveCompensationInitialNextReleaseAt(ctx, meta.SessionDate)
	if err != nil {
		return false, err
	}

	queue := newPlanQueue(plan)
	if totalPendingOrders == 0 {
		g.Log().Infof(ctx, "[GroupMatch] settle no pending orders: session_id=%d", sessionID)
	}

	totalProcessedOrders := 0
	totalGroups := 0
	totalFlowUsers := 0
	settleStart := time.Now()
	var processedCounter atomic.Int64

	workerCount := 2
	g.Log().Infof(ctx, "[GroupMatch] settle start parallel: session_id=%d workers=%d batch_groups=%d", sessionID, workerCount, batchGroups)

	type workerResult struct {
		orders int
		groups int
		flow   int
		err    error
	}
	resultCh := make(chan workerResult, workerCount)
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			g.Log().Infof(ctx, "[GroupMatch] settle worker started: session_id=%d worker=%d", sessionID, workerID)
			wOrders, wGroups, wFlow := 0, 0, 0
			for {
				batch := queue.take(batchGroups)
				if len(batch) == 0 {
					g.Log().Infof(ctx, "[GroupMatch] settle worker finished: session_id=%d worker=%d orders=%d groups=%d flow=%d", sessionID, workerID, wOrders, wGroups, wFlow)
					resultCh <- workerResult{orders: wOrders, groups: wGroups, flow: wFlow}
					return
				}
				bOrders, bGroups, bFlow, err := s.settleGroupBatchTx(ctx, sessionID, batch, winnerRate, winnerTicketTax, loserCompUSDT, loserReleaseDays, teamPoolRate, compNextReleaseAt)
				if err != nil {
					resultCh <- workerResult{err: err}
					return
				}
				wOrders += bOrders
				wGroups += bGroups
				wFlow += bFlow
				processedNow := int(processedCounter.Add(int64(bOrders)))
				remainingOrders := totalPendingOrders - processedNow
				if remainingOrders < 0 {
					remainingOrders = 0
				}
				elapsed := time.Since(settleStart)
				eta := "unknown"
				if processedNow > 0 {
					avgPerOrder := elapsed / time.Duration(processedNow)
					eta = (avgPerOrder * time.Duration(remainingOrders)).String()
				}
				g.Log().Infof(ctx, "[GroupMatch] settle progress: session_id=%d worker=%d batch_orders=%d batch_groups=%d batch_flow=%d worker_total=%d processed=%d/%d remaining=%d eta=%s elapsed=%s",
					sessionID, workerID, bOrders, bGroups, bFlow, wOrders, processedNow, totalPendingOrders, remainingOrders, eta, elapsed)
			}
		}(i)
	}
	go func() {
		wg.Wait()
		close(resultCh)
	}()
	for r := range resultCh {
		if r.err != nil {
			return false, r.err
		}
		totalProcessedOrders += r.orders
		totalGroups += r.groups
		totalFlowUsers += r.flow
	}
	g.Log().Infof(ctx, "[GroupMatch] settle batches finished: session_id=%d workers=%d total_groups=%d total_flow=%d total_orders=%d elapsed=%s",
		sessionID, workerCount, totalGroups, totalFlowUsers, totalProcessedOrders, time.Since(settleStart))

	if _, err := g.DB().Exec(ctx, `
		UPDATE group_match_session
		SET total_groups = (SELECT COUNT(*) FROM group_match_group WHERE session_id = ?),
		    total_flow_user_count = (SELECT COUNT(*) FROM group_match_order WHERE session_id = ? AND status = ?),
		    loser_principal_total = COALESCE((SELECT SUM(amount) FROM group_match_order WHERE session_id = ? AND status = ?), 0)
		WHERE id = ? AND status = ?
	`, sessionID, sessionID, consts.GroupMatchOrderStatusFlowOut, sessionID, consts.GroupMatchOrderStatusLoser, sessionID, consts.GroupMatchSessionStatusSettling); err != nil {
		return false, gerror.Wrap(err, "update session final stats failed")
	}

	if err := s.distributeTeamReward(ctx, sessionID); err != nil {
		return false, err
	}

	if err := s.completeSessionSettlement(ctx, sessionID); err != nil {
		return false, err
	}
	s.markGroupPerformanceDirty(ctx, sessionID)

	go func(sid int64, name string, orders, groups, flow int, start time.Time) {
		notifyCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cobo.GetTelegramNotifyService(notifyCtx).NotifyGroupMatchSettlementCompleted(notifyCtx, sid, name, orders, groups, flow, time.Since(start))
	}(sessionID, meta.SessionName, totalProcessedOrders, totalGroups, totalFlowUsers, settleStart)

	if meta.SessionName == "Evening" {
		// YY 未中奖补偿释放：Evening 结算后异步批量循环释放，不走 StakingV2 封顶截断。
		g.Log().Infof(ctx, "[GroupMatch] Evening session %d settled, triggering loser compensation release (async)", sessionID)
		go func(sid int64, sName string) {
			defer func() {
				if r := recover(); r != nil {
					g.Log().Errorf(context.Background(), "[GroupMatch] loser compensation release panic recovered: session_id=%d panic=%v", sid, r)
				}
			}()
			asyncCtx := context.Background()
			const batchLimit = 500
			releaseBaseNow := time.Now()
			totalReleased := 0
			releaseRounds := 0
			releaseStartAt := time.Now()
			for {
				releaseCount, err := s.releaseLoserCompensationsAtNow(asyncCtx, batchLimit, releaseBaseNow)
				if err != nil {
					g.Log().Errorf(asyncCtx, "[GroupMatch] loser compensation release failed after Evening session %d settled: round=%d total=%d err=%v", sid, releaseRounds+1, totalReleased, err)
					break
				}
				releaseRounds++
				totalReleased += releaseCount
				if releaseCount == 0 {
					break
				}
			}
			g.Log().Infof(asyncCtx, "[GroupMatch] loser compensation release completed after Evening session %d: total=%d rounds=%d batch_limit=%d elapsed=%s", sid, totalReleased, releaseRounds, batchLimit, time.Since(releaseStartAt))
			notifyCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cobo.GetTelegramNotifyService(notifyCtx).NotifyGroupMatchLoserCompReleaseCompleted(notifyCtx, sid, sName, totalReleased, time.Since(releaseStartAt))
		}(sessionID, meta.SessionName)

		g.Log().Infof(ctx, "[GroupMatch] Evening session %d settled, triggering leadership reward distribution", sessionID)
		// 领导奖（拼团体系）：同一业务日内同时结算级差 + 加权（加权资金来源为当日新增质押）。
		leadershipStart := time.Now()
		count, err := s.DistributeLeadershipRewards(ctx, meta.SessionDate)
		if err != nil {
			g.Log().Errorf(ctx, "[GroupMatch] leadership reward distribution failed after Evening session %d settled: err=%v", sessionID, err)
		} else {
			g.Log().Infof(ctx, "[GroupMatch] leadership reward distribution completed after Evening session %d: users=%d", sessionID, count)
			go func(sid int64, sName string, c int, start time.Time) {
				notifyCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				cobo.GetTelegramNotifyService(notifyCtx).NotifyGroupMatchLeadershipRewardDistributed(notifyCtx, sid, sName, c, time.Since(start))
			}(sessionID, meta.SessionName, count, leadershipStart)
		}

		// Staking V2 独立领导奖：历史保留的另一套奖励链路，和拼团领导奖明细表分开。
		g.Log().Infof(ctx, "[GroupMatch] Evening session %d settled, triggering staking_v2 leader reward distribution", sessionID)
		stakingStart := time.Now()
		stakingCount, err := staking_v2.NewStakingV2Service().DistributeLeaderRewards(ctx, meta.SessionDate)
		if err != nil {
			g.Log().Errorf(ctx, "[GroupMatch] staking_v2 leader reward distribution failed after Evening session %d settled: err=%v", sessionID, err)
		} else {
			g.Log().Infof(ctx, "[GroupMatch] staking_v2 leader reward distribution completed after Evening session %d: users=%d", sessionID, stakingCount)
			go func(sid int64, sName string, c int, start time.Time) {
				notifyCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				cobo.GetTelegramNotifyService(notifyCtx).NotifyGroupMatchStakingV2LeaderRewardDistributed(notifyCtx, sid, sName, c, time.Since(start))
			}(sessionID, meta.SessionName, stakingCount, stakingStart)
		}
	}

	return true, nil
}

func (s *groupMatchService) prepareSessionForSettlement(ctx context.Context, sessionID int64, skipEndTimeCheck bool) (bool, error) {
	type sessionRow struct {
		Status  int       `json:"status"`
		EndTime time.Time `json:"end_time"`
	}

	canSettle := false
	now := time.Now()
	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		row := &sessionRow{}
		if err := tx.Model("group_match_session").Ctx(ctx).Fields("status,end_time").Where("id", sessionID).LockUpdate().Scan(row); err != nil {
			return gerror.Wrap(err, "query session status failed")
		}

		if row.Status == consts.GroupMatchSessionStatusCompleted {
			canSettle = false
			return nil
		}
		if row.Status != consts.GroupMatchSessionStatusActive && row.Status != consts.GroupMatchSessionStatusSettling {
			canSettle = false
			return nil
		}

		if row.Status == consts.GroupMatchSessionStatusActive {
			// lib/pq 将 timestamp without time zone 扫描为 UTC，而 time.Now() 是本地时区(CST)
			// 必须按本地时区重新构造 end_time 再比较，否则跨时区比较会得到错误结果
			endTimeLocal := time.Date(row.EndTime.Year(), row.EndTime.Month(), row.EndTime.Day(),
				row.EndTime.Hour(), row.EndTime.Minute(), row.EndTime.Second(), row.EndTime.Nanosecond(), time.Local)
			if !skipEndTimeCheck && now.Before(endTimeLocal) {
				canSettle = false
				return nil
			}
			if _, err := tx.Model("group_match_session").Ctx(ctx).Where("id", sessionID).Data(g.Map{"status": consts.GroupMatchSessionStatusSettling}).Update(); err != nil {
				return gerror.Wrap(err, "update session to settling failed")
			}
		}

		canSettle = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return canSettle, nil
}

// settleGroupBatchTx 在单个事务内处理多个 plan。
// 每个 plan 已在 buildSettlementPlan 中按用户全局预分桶，不同 plan 之间订单 ID 互不相交，
// 因此 worker 之间无需 SKIP LOCKED 抢占；锁仅用于在事务内确认订单仍处于 pending。
func (s *groupMatchService) settleGroupBatchTx(
	ctx context.Context,
	sessionID int64,
	batch []*settlementPlan,
	winnerRate decimal.Decimal,
	winnerTicketTax decimal.Decimal,
	loserCompUSDT decimal.Decimal,
	loserReleaseDays int,
	teamPoolRate decimal.Decimal,
	compNextReleaseAt time.Time,
) (int, int, int, error) {
	if len(batch) == 0 {
		return 0, 0, 0, nil
	}
	processedOrders := 0
	groupsClosed := 0
	flowOrders := 0
	stakingSvc := staking_v2.NewStakingV2Service()

	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now := time.Now()
		balanceCache := make(map[balanceCacheKey]*balanceCacheState, 128)
		planIndependent := 0
		planMixed := 0
		planFlow := 0
		for _, p := range batch {
			if p == nil {
				continue
			}
			switch p.Kind {
			case planKindIndependent:
				planIndependent++
			case planKindMixed:
				planMixed++
			case planKindFlow:
				planFlow++
			}
		}
		g.Log().Infof(ctx, "[GroupMatch] settle batch tx begin: session_id=%d plans=%d independent=%d mixed=%d flow=%d", sessionID, len(batch), planIndependent, planMixed, planFlow)
		for _, plan := range batch {
			if plan == nil || len(plan.Orders) == 0 {
				continue
			}
			orderIDs := make([]int64, 0, len(plan.Orders))
			for _, o := range plan.Orders {
				orderIDs = append(orderIDs, o.ID)
			}
			locked := make([]*settlementOrderRow, 0, len(plan.Orders))
			if err := tx.Model("group_match_order").Ctx(ctx).
				Fields("id,user_id,order_no,amount,ticket_symbol,ticket_amount,ticket_usdt_value,created_at,status,group_id").
				WhereIn("id", orderIDs).
				Where("session_id = ?", sessionID).
				Where("status = ?", consts.GroupMatchOrderStatusPending).
				OrderAsc("id").
				LockUpdate().
				Scan(&locked); err != nil {
				return gerror.Wrap(err, "lock plan orders failed")
			}
			if len(locked) != len(plan.Orders) {
				g.Log().Warningf(ctx, "[GroupMatch] plan order count mismatch, skip: session_id=%d plan_kind=%d expect=%d got=%d", sessionID, plan.Kind, len(plan.Orders), len(locked))
				continue
			}
			lockedByID := make(map[int64]*settlementOrderRow, len(locked))
			for _, row := range locked {
				lockedByID[row.ID] = row
			}
			orderedLocked := make([]*settlementOrderRow, 0, len(plan.Orders))
			for _, planned := range plan.Orders {
				row, ok := lockedByID[planned.ID]
				if !ok {
					return gerror.Newf("plan order missing after lock: session_id=%d order_id=%d", sessionID, planned.ID)
				}
				orderedLocked = append(orderedLocked, row)
			}
			locked = orderedLocked

			switch plan.Kind {
			case planKindIndependent:
				loserIndex, err := secureRandInt(len(locked))
				if err != nil {
					return gerror.Wrap(err, "pick loser index failed")
				}
				if _, err := s.settleCompleteGroupTx(ctx, tx, sessionID, locked, winnerRate, winnerTicketTax, loserCompUSDT, loserReleaseDays, teamPoolRate, compNextReleaseAt, now, stakingSvc, loserIndex, balanceCache); err != nil {
					return err
				}
				groupsClosed++
			case planKindMixed:
				if plan.LoserIndex < 0 || plan.LoserIndex >= len(locked) {
					return gerror.Newf("invalid mixed loser index: %d (orders=%d)", plan.LoserIndex, len(locked))
				}
				if _, err := s.settleCompleteGroupTx(ctx, tx, sessionID, locked, winnerRate, winnerTicketTax, loserCompUSDT, loserReleaseDays, teamPoolRate, compNextReleaseAt, now, stakingSvc, plan.LoserIndex, balanceCache); err != nil {
					return err
				}
				groupsClosed++
			case planKindFlow:
				if err := s.flowRefundGroupTx(ctx, tx, sessionID, locked, now, balanceCache); err != nil {
					return err
				}
				flowOrders += len(locked)
			default:
				return gerror.Newf("unknown plan kind: %d", plan.Kind)
			}
			processedOrders += len(locked)
		}
		g.Log().Infof(ctx, "[GroupMatch] settle batch tx done: session_id=%d processed_orders=%d groups=%d flow_orders=%d", sessionID, processedOrders, groupsClosed, flowOrders)
		return nil
	})
	if err != nil {
		return 0, 0, 0, err
	}
	return processedOrders, groupsClosed, flowOrders, nil
}

func (s *groupMatchService) completeSessionSettlement(ctx context.Context, sessionID int64) error {
	result, err := g.DB().Exec(ctx, `
		UPDATE group_match_session
		SET status = ?, settled_at = COALESCE(settled_at, ?)
		WHERE id = ? AND status = ?
		AND NOT EXISTS (
			SELECT 1 FROM group_match_order WHERE session_id = ? AND status = ?
		)
	`, consts.GroupMatchSessionStatusCompleted, time.Now(), sessionID, consts.GroupMatchSessionStatusSettling, sessionID, consts.GroupMatchOrderStatusPending)
	if err != nil {
		return gerror.Wrap(err, "update session settlement completed status failed")
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return gerror.Newf("session settlement incomplete: session_id=%d, pending orders or status abnormal", sessionID)
	}
	return nil
}

func (s *groupMatchService) settleCompleteGroupTx(
	ctx context.Context,
	tx gdb.TX,
	sessionID int64,
	orders []*settlementOrderRow,
	winnerRate decimal.Decimal,
	winnerTicketTax decimal.Decimal,
	loserCompUSDT decimal.Decimal,
	loserReleaseDays int,
	teamPoolRate decimal.Decimal,
	compNextReleaseAt time.Time,
	now time.Time,
	stakingSvc staking_v2.IStakingV2Service,
	loserIndex int,
	balanceCache map[balanceCacheKey]*balanceCacheState,
) (decimal.Decimal, error) {
	if loserIndex < 0 || loserIndex >= len(orders) {
		return decimal.Zero, gerror.Newf("invalid loser index: %d (orders=%d)", loserIndex, len(orders))
	}
	groupNo := "GMG-" + utils.GenerateSnowflakeId()
	randomSeed := fmt.Sprintf("%d", time.Now().UnixNano())

	totalPrincipal := decimal.Zero
	winnerRewardTotal := decimal.Zero
	totalTicketReturn := decimal.Zero
	totalTicketBurn := decimal.Zero
	for _, order := range orders {
		totalPrincipal = totalPrincipal.Add(order.Amount)
	}

	loserUserID := orders[loserIndex].UserID
	loserWallet, err := s.getWalletAddressByUserID(ctx, tx, loserUserID)
	if err != nil {
		return decimal.Zero, err
	}

	groupID, err := tx.Model("group_match_group").Ctx(ctx).Data(g.Map{
		"session_id":           sessionID,
		"group_no":             groupNo,
		"status":               1,
		"member_count":         len(orders),
		"loser_user_id":        loserUserID,
		"loser_wallet_address": loserWallet,
		"random_seed":          randomSeed,
		"total_principal":      totalPrincipal,
		"drawn_at":             now,
		"created_at":           now,
	}).InsertAndGetId()
	if err != nil {
		return decimal.Zero, gerror.Wrap(err, "create group match record failed")
	}

	if teamPoolRate.GreaterThan(decimal.Zero) {
		poolAmount := orders[loserIndex].Amount.Mul(teamPoolRate)
		if poolAmount.GreaterThan(decimal.Zero) {
			_, err = tx.Exec(`
				INSERT INTO group_purchase_leadership_pool
				(biz_date, source_type, source_amount, rate, amount, status, created_at, updated_at)
				SELECT DATE(s.start_time), 'group_match_loser', ?, ?, ?, 0, ?, ?
				FROM group_match_session s
				WHERE s.id = ?
				ON CONFLICT (biz_date, source_type)
				DO UPDATE SET
					source_amount = group_purchase_leadership_pool.source_amount + EXCLUDED.source_amount,
					amount = group_purchase_leadership_pool.amount + EXCLUDED.amount,
					rate = EXCLUDED.rate,
					updated_at = EXCLUDED.updated_at
			`, orders[loserIndex].Amount, teamPoolRate, poolAmount, now, now, sessionID)
			if err != nil {
				return decimal.Zero, gerror.Wrap(err, "write leadership reward pool record failed")
			}
		}
	}

	compTokenAmount, _, err := s.calculateCompTokenAmount(ctx, loserCompUSDT)
	if err != nil {
		return decimal.Zero, err
	}
	if stakingSvc == nil {
		stakingSvc = staking_v2.NewStakingV2Service()
	}

	for idx, order := range orders {
		isWinner := idx != loserIndex
		rewardAmount := decimal.Zero
		refundAmount := decimal.Zero
		var ticketReturnAmount decimal.Decimal
		var ticketBurnAmount decimal.Decimal

		if isWinner {
			rewardAmount = order.Amount.Mul(winnerRate)
			// 奖励封顶在 staking_v2 内固定按 4 倍本金处理。
			grantedReward, err := stakingSvc.ApplyRewardWithCapTx(ctx, tx, order.UserID, rewardAmount)
			if err != nil {
				return decimal.Zero, gerror.Wrap(err, "apply 4x reward cap failed")
			}
			rewardAmount = grantedReward
			refundAmount = order.Amount
			winnerRewardTotal = winnerRewardTotal.Add(rewardAmount)

			if err := s.creditBalanceAndLogTx(ctx, tx, order.UserID, "USDT", refundAmount.Add(rewardAmount), groupMatchChangeTypeWinnerPayout, order.OrderNo, sessionID, "group_match winner settlement", balanceCache); err != nil {
				return decimal.Zero, err
			}

			// 中奖者门票扣除盈余税（USDT价值），按订单门票价格换算为代币数量
			var taxTokenAmount decimal.Decimal
			if order.TicketUSDTValue.GreaterThan(decimal.Zero) {
				taxTokenAmount = winnerTicketTax.Mul(order.TicketAmount).Div(order.TicketUSDTValue)
			}
			if taxTokenAmount.GreaterThan(order.TicketAmount) {
				taxTokenAmount = order.TicketAmount
			}
			ticketBurnAmount = taxTokenAmount
			ticketReturnAmount = order.TicketAmount.Sub(taxTokenAmount)
			g.Log().Debugf(ctx, "[GroupMatch] winner ticket settlement: session_id=%d group_id=%d order_no=%s user_id=%d symbol=%s ticket_amount=%s ticket_usdt_value=%s tax_usdt=%s burn_token=%s return_token=%s",
				sessionID,
				groupID,
				order.OrderNo,
				order.UserID,
				order.TicketSymbol,
				order.TicketAmount.String(),
				order.TicketUSDTValue.String(),
				winnerTicketTax.String(),
				ticketBurnAmount.String(),
				ticketReturnAmount.String(),
			)
		} else {
			if err := s.createLoserCompensationTx(ctx, tx, sessionID, groupID, order, loserCompUSDT, loserReleaseDays, now, compNextReleaseAt); err != nil {
				return decimal.Zero, err
			}
			if _, err := tx.Model("group_match_order").Ctx(ctx).Where("id", order.ID).Data(g.Map{
				"loser_comp_usdt_value":   loserCompUSDT,
				"loser_comp_symbol":       "YY",
				"loser_comp_amount":       compTokenAmount,
				"loser_comp_release_days": loserReleaseDays,
				"updated_at":              now,
			}).Update(); err != nil {
				return decimal.Zero, gerror.Wrap(err, "update loser compensation field failed")
			}
			// g.Log().Infof(ctx, "[GroupMatch] loser补偿建档: session_id=%d, order_no=%s, usdt=%s, yy_price=%s, yy_amount=%s", sessionID, order.OrderNo, loserCompUSDT.String(), compPrice.String(), compTokenAmount.String())

			// 未中奖者门票全额退回
			ticketBurnAmount = decimal.Zero
			ticketReturnAmount = order.TicketAmount
		}

		if ticketReturnAmount.GreaterThan(decimal.Zero) {
			if err := s.creditBalanceAndLogTx(ctx, tx, order.UserID, order.TicketSymbol, ticketReturnAmount, groupMatchChangeTypeTicketRefund, order.OrderNo, sessionID, "group_match ticket refund", balanceCache); err != nil {
				return decimal.Zero, err
			}
		}

		if ticketBurnAmount.GreaterThan(decimal.Zero) {
			if err := s.creditBalanceAndLogTx(ctx, tx, groupMatchVertexUserID, order.TicketSymbol, ticketBurnAmount, groupMatchChangeTypeTicketBurnSink, order.OrderNo, sessionID, "group_match winner ticket burn to vertex", balanceCache); err != nil {
				return decimal.Zero, err
			}
			g.Log().Debugf(ctx, "[GroupMatch] ticket burn sink credited: session_id=%d order_no=%s symbol=%s burn_token=%s vertex_user_id=%d", sessionID, order.OrderNo, order.TicketSymbol, ticketBurnAmount.String(), groupMatchVertexUserID)
		}

		totalTicketReturn = totalTicketReturn.Add(ticketReturnAmount)
		totalTicketBurn = totalTicketBurn.Add(ticketBurnAmount)

		if _, err := tx.Model("group_match_group_member").Ctx(ctx).Data(g.Map{
			"group_id":      groupID,
			"order_id":      order.ID,
			"user_id":       order.UserID,
			"is_winner":     isWinner,
			"reward_amount": rewardAmount,
			"created_at":    now,
		}).Insert(); err != nil {
			return decimal.Zero, gerror.Wrap(err, "create group match member record failed")
		}

		if _, err := tx.Model("group_match_order").Ctx(ctx).Where("id", order.ID).Data(g.Map{
			"status":               ternaryInt(isWinner, consts.GroupMatchOrderStatusWinner, consts.GroupMatchOrderStatusLoser),
			"group_id":             groupID,
			"is_winner":            isWinner,
			"reward_amount":        rewardAmount,
			"refund_amount":        refundAmount,
			"ticket_return_amount": ticketReturnAmount,
			"ticket_burn_amount":   ticketBurnAmount,
			"updated_at":           now,
		}).Update(); err != nil {
			return decimal.Zero, gerror.Wrap(err, "update group match order status failed")
		}
	}

	_, err = tx.Model("group_match_group").Ctx(ctx).Where("id", groupID).Data(g.Map{
		"winner_reward_total": winnerRewardTotal,
	}).Update()
	if err != nil {
		return decimal.Zero, gerror.Wrap(err, "update group match winner reward stats failed")
	}

	g.Log().Debugf(ctx, "[GroupMatch] group settle summary: session_id=%d group_id=%d member_count=%d loser_user_id=%d winner_reward_total=%s total_ticket_return=%s total_ticket_burn=%s",
		sessionID,
		groupID,
		len(orders),
		loserUserID,
		winnerRewardTotal.String(),
		totalTicketReturn.String(),
		totalTicketBurn.String(),
	)

	return orders[loserIndex].Amount, nil
}

func (s *groupMatchService) flowRefundGroupTx(ctx context.Context, tx gdb.TX, sessionID int64, orders []*settlementOrderRow, now time.Time, balanceCache map[balanceCacheKey]*balanceCacheState) error {
	for _, order := range orders {
		if err := s.creditBalanceAndLogTx(ctx, tx, order.UserID, "USDT", order.Amount, groupMatchChangeTypeFlowRefund, order.OrderNo, sessionID, "group_match flow refund", balanceCache); err != nil {
			return err
		}
		if err := s.creditBalanceAndLogTx(ctx, tx, order.UserID, order.TicketSymbol, order.TicketAmount, groupMatchChangeTypeTicketRefund, order.OrderNo, sessionID, "group_match flow ticket refund", balanceCache); err != nil {
			return err
		}

		_, err := tx.Model("group_match_order").Ctx(ctx).Where("id", order.ID).Data(g.Map{
			"status":               consts.GroupMatchOrderStatusFlowOut,
			"is_winner":            nil,
			"reward_amount":        decimal.Zero,
			"refund_amount":        order.Amount,
			"ticket_return_amount": order.TicketAmount,
			"ticket_burn_amount":   decimal.Zero,
			"updated_at":           now,
		}).Update()
		if err != nil {
			return gerror.Wrap(err, "update flow-out order status failed")
		}
	}
	return nil
}

func (s *groupMatchService) createLoserCompensationTx(
	ctx context.Context,
	tx gdb.TX,
	sessionID, groupID int64,
	order *settlementOrderRow,
	usdtValue decimal.Decimal,
	releaseDays int,
	now time.Time,
	nextReleaseAt time.Time,
) error {
	wallet, err := s.getWalletAddressByUserID(ctx, tx, order.UserID)
	if err != nil {
		return err
	}
	if wallet != "" {
		wallet = strings.ToLower(wallet)
	}

	startDate := dateOnlyInChina(nextReleaseAt)
	endDate := startDate.AddDate(0, 0, releaseDays-1)
	_, err = tx.Model("group_match_loser_compensation").Ctx(ctx).Data(g.Map{
		"order_id":        order.ID,
		"session_id":      sessionID,
		"group_id":        groupID,
		"user_id":         order.UserID,
		"wallet_address":  wallet,
		"token_symbol":    "YY",
		"usdt_value":      usdtValue,
		"release_days":    releaseDays,
		"released_amount": decimal.Zero,
		"pending_amount":  usdtValue,
		"status":          groupMatchLoserCompStatusReleasing,
		"start_date":      startDate,
		"end_date":        endDate,
		"next_release_at": nextReleaseAt,
		"created_at":      now,
		"updated_at":      now,
	}).Insert()
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil
		}
		return gerror.Wrap(err, "create loser compensation record failed")
	}
	return nil
}

func (s *groupMatchService) resolveCompensationInitialNextReleaseAt(ctx context.Context, sessionDate time.Time) (time.Time, error) {
	type eveningRow struct {
		EndTime time.Time `json:"end_time"`
	}
	row := &eveningRow{}
	err := g.DB().Model("group_match_session").Ctx(ctx).
		Fields("end_time").
		Where("session_date = ?", dateOnlyInChina(sessionDate)).
		Where("session_name = ?", "Evening").
		OrderAsc("id").
		Limit(1).
		Scan(row)
	if err != nil {
		return time.Time{}, gerror.Wrap(err, "query evening session end_time failed")
	}
	if row.EndTime.IsZero() {
		return time.Now(), nil
	}
	return time.Date(
		row.EndTime.Year(), row.EndTime.Month(), row.EndTime.Day(),
		row.EndTime.Hour(), row.EndTime.Minute(), row.EndTime.Second(), row.EndTime.Nanosecond(),
		time.FixedZone("CST", 8*3600),
	), nil
}

func (s *groupMatchService) calculateCompTokenAmount(ctx context.Context, usdtValue decimal.Decimal) (decimal.Decimal, decimal.Decimal, error) {
	// YY 使用内部 stock_price 定义的最新价格
	price, err := s.getTicketPrice(ctx, "YY")
	if err != nil {
		return decimal.Zero, decimal.Zero, gerror.Wrap(err, "query YY price failed")
	}
	if !price.GreaterThan(decimal.Zero) {
		return decimal.Zero, decimal.Zero, gerror.New("YY price unavailable")
	}
	return usdtValue.Div(price), price, nil
}

func (s *groupMatchService) releaseSingleCompensation(ctx context.Context, compensationID int64) (bool, error) {
	return s.releaseSingleCompensationWithBaseNow(ctx, compensationID, time.Time{}, false)
}

func (s *groupMatchService) releaseSingleCompensationWithBaseNow(ctx context.Context, compensationID int64, baseNow time.Time, forceBaseNow bool) (bool, error) {
	now := time.Now()
	didRelease := false

	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		row := &loserCompensationRow{}
		err := tx.Model("group_match_loser_compensation").Ctx(ctx).
			Where("id = ?", compensationID).
			Where("status = ?", groupMatchLoserCompStatusReleasing).
			Where("next_release_at IS NULL OR next_release_at <= ?", now).
			LockUpdate().
			Scan(row)
		if err != nil {
			return gerror.Wrap(err, "query compensation records failed")
		}
		if row.ID == 0 || !row.PendingAmount.GreaterThan(decimal.Zero) {
			return nil
		}

		releaseAt := now
		if forceBaseNow {
			releaseAt = baseNow
		} else if row.NextReleaseAt != nil && !row.NextReleaseAt.IsZero() {
			correctedNext := fixScanTimeToCST(*row.NextReleaseAt)
			if correctedNext.Before(now) {
				releaseAt = correctedNext
			}
		}
		releaseDate := dateOnlyInChina(releaseAt)

		releaseUSDT := s.calculateTodayReleaseAmount(row, releaseDate)
		if !releaseUSDT.GreaterThan(decimal.Zero) {
			return gerror.Newf("compensation release amount abnormal: compensation_id=%d, pending_amount=%s", row.ID, row.PendingAmount.String())
		}

		yyPrice, err := s.getTicketPrice(ctx, "YY")
		if err != nil {
			return gerror.Wrap(err, "query YY price failed")
		}
		if !yyPrice.GreaterThan(decimal.Zero) {
			return gerror.New("YY price unavailable")
		}
		grantedUSDT := releaseUSDT
		grantedYY := releaseUSDT.Div(yyPrice)

		insertRes, err := tx.Exec(`
			INSERT INTO group_match_loser_comp_release_log
			(compensation_id, order_id, user_id, wallet_address, token_symbol, release_date, release_usdt_value, release_amount, status, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?)
			ON CONFLICT (compensation_id, release_date) DO NOTHING
		`, row.ID, row.OrderID, row.UserID, strings.ToLower(row.WalletAddress), row.TokenSymbol, releaseDate, grantedUSDT, grantedYY, now)
		if err != nil {
			return gerror.Wrap(err, "write compensation release log failed")
		}
		inserted, _ := insertRes.RowsAffected()
		if inserted == 0 {
			return nil
		}

		if grantedYY.GreaterThan(decimal.Zero) {
			if err := s.creditBalanceAndLogTx(ctx, tx, row.UserID, row.TokenSymbol, grantedYY, groupMatchChangeTypeLoserComp, fmt.Sprintf("COMP-%d", row.ID), row.OrderID, "group_match loser compensation release", nil); err != nil {
				return err
			}
		}

		newReleased := row.ReleasedAmount.Add(releaseUSDT)
		newPending := row.PendingAmount.Sub(releaseUSDT)
		if newPending.LessThan(decimal.Zero) {
			newPending = decimal.Zero
		}

		status := groupMatchLoserCompStatusReleasing
		if !newPending.GreaterThan(decimal.Zero) {
			status = groupMatchLoserCompStatusDone
		}

		data := g.Map{
			"released_amount": newReleased,
			"pending_amount":  newPending,
			"last_release_at": now,
			"updated_at":      now,
			"status":          status,
		}
		if status == groupMatchLoserCompStatusDone {
			data["next_release_at"] = nil
		} else {
			// 锚定到当前 next_release_at（=Evening end_time）+ 24h，避免每天用 time.Now() 漂移
			var nextReleaseAt time.Time
			if row.NextReleaseAt != nil && !row.NextReleaseAt.IsZero() {
				nextReleaseAt = fixScanTimeToCST(*row.NextReleaseAt).Add(24 * time.Hour)
			} else {
				nextReleaseAt = releaseAt.Add(24 * time.Hour)
			}
			data["next_release_at"] = nextReleaseAt
		}

		_, err = tx.Model("group_match_loser_compensation").Ctx(ctx).Where("id", row.ID).Data(data).Update()
		if err != nil {
			return gerror.Wrap(err, "update compensation status failed")
		}
		didRelease = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return didRelease, nil
}

func (s *groupMatchService) calculateTodayReleaseAmount(row *loserCompensationRow, releaseDate time.Time) decimal.Decimal {
	if row == nil || !row.PendingAmount.GreaterThan(decimal.Zero) {
		return decimal.Zero
	}

	if !releaseDate.Before(dateOnlyInChina(row.EndDate).AddDate(0, 0, 1)) {
		return row.PendingAmount
	}

	endDate := dateOnlyInChina(row.EndDate)
	remainDays := int(endDate.Sub(releaseDate).Hours()/24) + 1
	if remainDays <= 1 {
		return row.PendingAmount
	}

	amount := row.PendingAmount.Div(decimal.NewFromInt(int64(remainDays)))
	if amount.LessThanOrEqual(decimal.Zero) {
		return row.PendingAmount
	}
	return amount
}

func (s *groupMatchService) creditBalanceAndLogTx(
	ctx context.Context,
	tx gdb.TX,
	userID int64,
	symbol string,
	amount decimal.Decimal,
	changeType string,
	relatedOrderNo string,
	relatedID int64,
	remark string,
	balanceCaches ...map[balanceCacheKey]*balanceCacheState,
) error {
	if !amount.GreaterThan(decimal.Zero) {
		return nil
	}
	var balanceCache map[balanceCacheKey]*balanceCacheState
	if len(balanceCaches) > 0 {
		balanceCache = balanceCaches[0]
	}
	beforeAmount := decimal.Zero
	cacheKey := balanceCacheKey{UserID: userID, Symbol: strings.ToUpper(symbol)}
	if balanceCache != nil {
		if state, ok := balanceCache[cacheKey]; ok && state != nil {
			beforeAmount = state.Available
		} else {
			before, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, userID, symbol)
			if err != nil {
				return gerror.Wrap(err, "query balance before distribution failed")
			}
			if before != nil {
				beforeAmount = before.AvailableAmount
			}
			balanceCache[cacheKey] = &balanceCacheState{Available: beforeAmount}
		}
	} else {
		before, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, userID, symbol)
		if err != nil {
			return gerror.Wrap(err, "query balance before distribution failed")
		}
		if before != nil {
			beforeAmount = before.AvailableAmount
		}
	}

	if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, userID, symbol, amount, decimal.Zero); err != nil {
		return gerror.Wrap(err, "update balance failed")
	}

	afterAmount := beforeAmount.Add(amount)
	if balanceCache != nil {
		balanceCache[cacheKey] = &balanceCacheState{Available: afterAmount}
	}

	if err := s.createCoboBalanceChangeLogTxWithRemark(ctx, tx, userID, symbol, changeType, amount, beforeAmount, afterAmount, relatedOrderNo, relatedID, remark); err != nil {
		return err
	}
	return nil
}

func (s *groupMatchService) getWalletAddressByUserID(ctx context.Context, tx gdb.TX, userID int64) (string, error) {
	row := struct {
		WalletAddress string `json:"wallet_address"`
	}{}
	err := tx.Model("user_info").Ctx(ctx).Fields("wallet_address").Where("id", userID).Scan(&row)
	if err != nil {
		return "", gerror.Wrap(err, "query user wallet address failed")
	}
	return strings.ToLower(strings.TrimSpace(row.WalletAddress)), nil
}

func (s *groupMatchService) createCoboBalanceChangeLogTxWithRemark(ctx context.Context, tx gdb.TX, userID int64, symbol, changeType string, amount, beforeBalance, afterBalance decimal.Decimal, relatedOrderNo string, relatedID int64, remark string) error {
	_, err := tx.Exec(`INSERT INTO cobo_balance_change_log (user_id, symbol, change_type, amount, before_balance, after_balance, related_order_no, related_id, remark, operator_type, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, userID, symbol, changeType, amount, beforeBalance, afterBalance, relatedOrderNo, relatedID, remark, consts.OperatorTypeSystem, time.Now())
	if err != nil {
		return gerror.Wrap(err, "write cobo balance change log failed")
	}
	return nil
}

func acquireSimpleRedisLock(ctx context.Context, key, token string, ttlSeconds int) (bool, error) {
	redisClient := g.Redis()
	if redisClient == nil {
		return false, gerror.New("redis not initialized")
	}
	result, err := redisClient.Do(ctx, "SET", key, token, "NX", "EX", ttlSeconds)
	if err != nil {
		return false, gerror.Wrap(err, "acquire distributed lock failed")
	}
	if result.IsNil() {
		return false, nil
	}
	return result.String() == "OK", nil
}

func releaseSimpleRedisLock(ctx context.Context, key, token string) {
	redisClient := g.Redis()
	if redisClient == nil {
		return
	}
	_, err := redisClient.Do(ctx, "EVAL", `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) end return 0`, 1, key, token)
	if err != nil {
		g.Log().Warningf(ctx, "[GroupMatch] 释放分布式锁失败: key=%s err=%v", key, err)
	}
}

func dateOnlyInChina(t time.Time) time.Time {
	loc := time.FixedZone("CST", 8*3600)
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

// fixScanTimeToCST 修正 lib/pq 扫描 timestamp without time zone 时默认标记为 UTC 的偏差。
// 数据库中的时间值实际表示 CST 时间，扫描后需重新包装为 CST 时区才能正确比较。
func fixScanTimeToCST(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return time.Date(
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second(), t.Nanosecond(),
		time.FixedZone("CST", 8*3600),
	)
}

func ternaryInt(cond bool, yes, no int) int {
	if cond {
		return yes
	}
	return no
}
