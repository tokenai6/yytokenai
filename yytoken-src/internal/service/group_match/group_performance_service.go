package group_match

import (
	"context"
	"fmt"
	"strings"
	"time"

	"XWFrame/internal/frame/consts"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

const (
	groupPerformanceDirtyKeyPrefix = "group_performance:dirty"
	groupPerformanceMaxDepth       = 100
)

func GroupPerformanceDirtyKeyForSession(sessionID int64) string {
	return fmt.Sprintf("%s:%d", groupPerformanceDirtyKeyPrefix, sessionID)
}

type groupPerfUserRow struct {
	ID               int64  `json:"id"`
	WalletAddress    string `json:"wallet_address"`
	InviteCode       string `json:"invite_code"`
	ParentInviteCode string `json:"parent_invite_code"`
	Status           int    `json:"status"`
}

type groupPerfOrderRow struct {
	UserID     int64           `json:"user_id"`
	Amount     decimal.Decimal `json:"amount"`
	Cnt        int             `json:"cnt"`
	MaxOrderID int64           `json:"max_order_id"`
}

type groupPerfNode struct {
	userID           int64
	walletAddress    string
	inviteCode       string
	parentInviteCode string
	status           int
	personalAmount   decimal.Decimal
	orderCount       int
	maxOrderID       int64
	children         []*groupPerfNode
	teamAmount       decimal.Decimal
	smallTeamAmount  decimal.Decimal
	teamTotalCount   int
	bigTeamCount     int
	smallTeamSum     int
}

func (s *groupMatchService) ResolveCurrentSessionID(ctx context.Context) (int64, error) {
	type sessionRow struct {
		ID int64 `json:"id"`
	}

	var row sessionRow
	err := g.DB().Model("group_match_session").Ctx(ctx).
		Where("status = ?", consts.GroupMatchSessionStatusActive).
		Where("start_time <= NOW()").
		Where("end_time > NOW()").
		OrderAsc("start_time").
		Limit(1).
		Scan(&row)
	if err != nil && !strings.Contains(err.Error(), "no rows in result set") {
		return 0, err
	}
	if row.ID > 0 {
		return row.ID, nil
	}

	err = g.DB().Model("group_match_session").Ctx(ctx).
		Where("start_time <= NOW()").
		OrderDesc("start_time").
		Limit(1).
		Scan(&row)
	if err != nil && !strings.Contains(err.Error(), "no rows in result set") {
		return 0, err
	}
	return row.ID, nil
}

func (s *groupMatchService) RefreshGroupPerformanceCache(ctx context.Context) error {
	sessionID, err := s.ResolveCurrentSessionID(ctx)
	if err != nil {
		return err
	}
	if sessionID == 0 {
		return nil
	}
	return s.refreshGroupPerformanceBySession(ctx, sessionID)
}

func (s *groupMatchService) refreshGroupPerformanceBySession(ctx context.Context, sessionID int64) error {
	lockKey := fmt.Sprintf("group_match:group_performance:refresh:%d", sessionID)
	token := time.Now().Format("20060102150405.000000000")
	locked, err := acquireSimpleRedisLock(ctx, lockKey, token, 300)
	if err != nil {
		return err
	}
	if !locked {
		return nil
	}
	defer releaseSimpleRedisLock(ctx, lockKey, token)

	needRefresh, err := s.needRefreshGroupPerformance(ctx, sessionID)
	if err != nil {
		return err
	}
	if !needRefresh {
		return nil
	}

	if err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		return s.rebuildGroupPerformanceBySessionTx(ctx, tx, sessionID)
	}); err != nil {
		return err
	}

	s.clearGroupPerformanceDirty(ctx, sessionID)
	return nil
}

func (s *groupMatchService) needRefreshGroupPerformance(ctx context.Context, sessionID int64) (bool, error) {
	if s.isGroupPerformanceDirty(ctx, sessionID) {
		return true, nil
	}

	var current struct {
		MaxID       int64           `json:"max_id"`
		OrderCount  int64           `json:"order_count"`
		TotalAmount decimal.Decimal `json:"total_amount"`
	}
	err := g.DB().Ctx(ctx).Raw(`
		SELECT COALESCE(MAX(id), 0) AS max_id,
		       COUNT(*) AS order_count,
		       COALESCE(SUM(amount), 0) AS total_amount
		FROM group_match_order
		WHERE session_id = ?
	`, sessionID).Scan(&current)
	if err != nil {
		return false, gerror.Wrap(err, "query latest group_match_order id failed")
	}

	var cached struct {
		MaxID       int64           `json:"max_id"`
		RowCount    int64           `json:"row_count"`
		OrderCount  int64           `json:"order_count"`
		TotalAmount decimal.Decimal `json:"total_amount"`
	}
	err = g.DB().Ctx(ctx).Raw(`
		SELECT COALESCE(MAX(max_order_id), 0) AS max_id,
		       COUNT(*) AS row_count,
		       COALESCE(SUM(order_count), 0) AS order_count,
		       COALESCE(SUM(personal_group_amount), 0) AS total_amount
		FROM group_performance
		WHERE session_id = ?
	`, sessionID).Scan(&cached)
	if err != nil {
		return false, gerror.Wrap(err, "query cached group_performance failed")
	}

	if cached.RowCount == 0 {
		return true, nil
	}
	if current.MaxID != cached.MaxID {
		return true, nil
	}
	if current.OrderCount != cached.OrderCount {
		return true, nil
	}
	return !current.TotalAmount.Equal(cached.TotalAmount), nil
}

func (s *groupMatchService) isGroupPerformanceDirty(ctx context.Context, sessionID int64) bool {
	key := GroupPerformanceDirtyKeyForSession(sessionID)
	v, err := g.Redis().Get(ctx, key)
	if err != nil {
		g.Log().Warningf(ctx, "[GroupPerformance] check dirty failed: key=%s err=%v", key, err)
		return false
	}
	if v == nil || v.IsNil() {
		return false
	}
	return strings.TrimSpace(v.String()) != ""
}

func (s *groupMatchService) rebuildGroupPerformanceBySessionTx(ctx context.Context, tx gdb.TX, sessionID int64) error {
	var sessionRow struct {
		SessionDate time.Time `json:"session_date"`
	}
	err := tx.Raw("SELECT session_date FROM group_match_session WHERE id = ?", sessionID).Scan(&sessionRow)
	if err != nil {
		return gerror.Wrap(err, "query session biz_date failed")
	}
	bizDate := sessionRow.SessionDate
	bizDate = dateOnlyInChina(bizDate)

	users := make([]groupPerfUserRow, 0)
	if err := tx.Model("user_info").Ctx(ctx).
		Fields("id, wallet_address, invite_code, parent_invite_code, status").
		Scan(&users); err != nil {
		return gerror.Wrap(err, "query users for group_performance failed")
	}

	joinOrders := make([]groupPerfOrderRow, 0)
	err = tx.Raw(`
		SELECT user_id,
			COALESCE(SUM(amount), 0) AS amount,
			COUNT(*) AS cnt,
			COALESCE(MAX(id), 0) AS max_order_id
		FROM group_match_order
		WHERE session_id = ?
		GROUP BY user_id
	`, sessionID).Scan(&joinOrders)
	if err != nil {
		return gerror.Wrap(err, "query group join orders by session failed")
	}

	nodeMap := make(map[string]*groupPerfNode, len(users))
	idMap := make(map[int64]*groupPerfNode, len(users))
	for _, u := range users {
		n := &groupPerfNode{userID: u.ID, walletAddress: u.WalletAddress, inviteCode: u.InviteCode, parentInviteCode: u.ParentInviteCode, status: u.Status}
		nodeMap[u.InviteCode] = n
		idMap[u.ID] = n
	}
	for _, od := range joinOrders {
		if n, ok := idMap[od.UserID]; ok {
			n.personalAmount = od.Amount
			n.orderCount = od.Cnt
			n.maxOrderID = od.MaxOrderID
		}
	}

	roots := make([]*groupPerfNode, 0)
	for _, n := range nodeMap {
		if n.parentInviteCode != "" {
			if p, ok := nodeMap[n.parentInviteCode]; ok {
				p.children = append(p.children, n)
				continue
			}
		}
		roots = append(roots, n)
	}

	visitState := make(map[*groupPerfNode]int, len(users))
	var dfs func(*groupPerfNode, int)
	dfs = func(n *groupPerfNode, depth int) {
		if n == nil {
			return
		}
		if visitState[n] == 2 {
			return
		}
		if visitState[n] == 1 {
			g.Log().Warningf(ctx, "[GroupPerformance] invite cycle detected, user_id=%d invite_code=%s", n.userID, n.inviteCode)
			return
		}
		if depth > groupPerformanceMaxDepth {
			g.Log().Warningf(ctx, "[GroupPerformance] max performance depth reached, max_depth=%d truncated_at_user_id=%d invite_code=%s", groupPerformanceMaxDepth, n.userID, n.inviteCode)
			return
		}
		visitState[n] = 1
		for _, c := range n.children {
			dfs(c, depth+1)
		}
		branch := make([]decimal.Decimal, 0, len(n.children))
		branchCounts := make([]int, 0, len(n.children))
		teamAmount := decimal.Zero
		teamTotalCount := 0
		for _, c := range n.children {
			if visitState[c] != 2 {
				continue
			}
			bp := c.personalAmount.Add(c.teamAmount)
			branch = append(branch, bp)
			branchCounts = append(branchCounts, c.teamTotalCount+1)
			teamAmount = teamAmount.Add(bp)
			teamTotalCount += c.teamTotalCount + 1
		}
		n.teamAmount = teamAmount
		n.teamTotalCount = teamTotalCount
		if len(branch) > 0 {
			maxB := decimal.Zero
			maxIdx := 0
			for i, b := range branch {
				if b.GreaterThan(maxB) {
					maxB = b
					maxIdx = i
				}
			}
			n.smallTeamAmount = n.teamAmount.Sub(maxB)
			n.bigTeamCount = branchCounts[maxIdx]
			n.smallTeamSum = n.teamTotalCount - branchCounts[maxIdx]
		}
		visitState[n] = 2
	}
	for _, r := range roots {
		dfs(r, 0)
	}
	for _, n := range idMap {
		dfs(n, 0)
	}

	if _, err := tx.Exec("DELETE FROM group_performance WHERE session_id = ?", sessionID); err != nil {
		return gerror.Wrap(err, "clear group_performance by session failed")
	}

	const batchSize = 500
	const valueTuple = "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)"
	const insertHead = `INSERT INTO group_performance (
		session_id, biz_date, user_id, wallet_address, invite_code,
		personal_group_amount, team_group_amount, small_team_group_amount,
		direct_count, team_total_count, order_count, max_order_id,
		big_team_count, small_team_sum,
		updated_at, created_at
	) VALUES `
	const insertTail = ` ON CONFLICT (session_id, user_id) DO UPDATE SET
		biz_date = EXCLUDED.biz_date,
		wallet_address = EXCLUDED.wallet_address,
		invite_code = EXCLUDED.invite_code,
		personal_group_amount = EXCLUDED.personal_group_amount,
		team_group_amount = EXCLUDED.team_group_amount,
		small_team_group_amount = EXCLUDED.small_team_group_amount,
		direct_count = EXCLUDED.direct_count,
		team_total_count = EXCLUDED.team_total_count,
		order_count = EXCLUDED.order_count,
		max_order_id = EXCLUDED.max_order_id,
		big_team_count = EXCLUDED.big_team_count,
		small_team_sum = EXCLUDED.small_team_sum,
		updated_at = CURRENT_TIMESTAMP`

	totalSmallTeamAmount := decimal.Zero
	districtUserCount := 0
	tuples := make([]string, 0, batchSize)
	args := make([]any, 0, batchSize*15)

	flush := func() error {
		if len(tuples) == 0 {
			return nil
		}
		sql := insertHead + strings.Join(tuples, ",") + insertTail
		if _, err := tx.Exec(sql, args...); err != nil {
			return gerror.Wrap(err, "batch insert group_performance failed")
		}
		tuples = tuples[:0]
		args = args[:0]
		return nil
	}

	for _, u := range users {
		if u.Status != 1 {
			continue
		}
		n := nodeMap[u.InviteCode]
		if n == nil {
			continue
		}
		tuples = append(tuples, valueTuple)
		args = append(args,
			sessionID, bizDate, n.userID, n.walletAddress, n.inviteCode,
			n.personalAmount, n.teamAmount, n.smallTeamAmount,
			len(n.children), n.teamTotalCount, n.orderCount, n.maxOrderID,
			n.bigTeamCount, n.smallTeamSum,
		)
		if n.smallTeamAmount.GreaterThan(decimal.Zero) {
			totalSmallTeamAmount = totalSmallTeamAmount.Add(n.smallTeamAmount)
			districtUserCount++
		}
		if len(tuples) >= batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := flush(); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO group_performance_summary (session_id, biz_date, total_small_team_amount, district_user_count, updated_at, created_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (session_id) DO UPDATE SET
			biz_date = EXCLUDED.biz_date,
			total_small_team_amount = EXCLUDED.total_small_team_amount,
			district_user_count = EXCLUDED.district_user_count,
			updated_at = CURRENT_TIMESTAMP
	`, sessionID, bizDate, totalSmallTeamAmount, districtUserCount); err != nil {
		return gerror.Wrap(err, "upsert group_performance_summary failed")
	}
	return nil
}

func (s *groupMatchService) markGroupPerformanceDirty(ctx context.Context, sessionID int64) {
	key := GroupPerformanceDirtyKeyForSession(sessionID)
	err := g.Redis().SetEX(ctx, key, 1, 300)
	if err != nil {
		g.Log().Warningf(ctx, "[GroupPerformance] mark dirty failed: %v", err)
	}
}

func (s *groupMatchService) clearGroupPerformanceDirty(ctx context.Context, sessionID int64) {
	key := GroupPerformanceDirtyKeyForSession(sessionID)
	_, _ = g.Redis().Del(ctx, key)
}

func (s *groupMatchService) asyncRefreshGroupPerformance(ctx context.Context, sessionID int64) {
	lockKey := fmt.Sprintf("group_match:group_performance:async_refresh:%d", sessionID)
	token := time.Now().Format("20060102150405.000000000")
	locked, err := acquireSimpleRedisLock(ctx, lockKey, token, 30)
	if err != nil {
		g.Log().Warningf(ctx, "[GroupPerformance] async refresh lock failed: session_id=%d err=%v", sessionID, err)
		return
	}
	if !locked {
		return
	}

	go func() {
		gCtx := context.Background()
		defer releaseSimpleRedisLock(gCtx, lockKey, token)
		if err := s.refreshGroupPerformanceBySession(gCtx, sessionID); err != nil {
			g.Log().Warningf(gCtx, "[GroupPerformance] async refresh failed: session_id=%d err=%v", sessionID, err)
		} else {
			g.Log().Infof(gCtx, "[GroupPerformance] async refresh success: session_id=%d", sessionID)
		}
		s.clearGroupPerformanceDirty(gCtx, sessionID)
	}()
}
