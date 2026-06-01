package teamstats

import (
	"context"
	"time"

	coboEntity "XWFrame/internal/entity/cobo"
	frameCache "XWFrame/internal/frame/cache"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/service/shared"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcache"
	"github.com/shopspring/decimal"
)

const globalDistrictStatsCacheTTL = 60 * time.Second

type OverviewAgg struct {
	TeamTotalUserCount      int
	TeamTodayNewUserCount   int
	DirectCount             int
	BigTeamCount            int
	SmallTeamSum            int
	BigTeamPerformance      decimal.Decimal
	SmallTeamPerformance    decimal.Decimal
	TeamPerformance         decimal.Decimal
	TeamYYSum               decimal.Decimal
	TodayNewPerformance     decimal.Decimal
	YesterdayNewPerformance decimal.Decimal
}

type ITeamStatsService interface {
	GetOverviewAggByInviteCode(ctx context.Context, inviteCode string) (*OverviewAgg, error)
	GetL1L2PurchaseTotalByInviteCode(ctx context.Context, inviteCode string) (l1Total, l2Total decimal.Decimal, err error)
	GetGlobalDistrictStats(ctx context.Context) (districtUserCount int, totalDistrictPerformance decimal.Decimal, err error)
}

type teamStatsService struct {
	memoryCache *gcache.Cache
	redisCache  *gcache.Cache
}

func NewTeamStatsService() ITeamStatsService {
	return &teamStatsService{
		memoryCache: frameCache.GetMemoryCache(),
		redisCache:  frameCache.GetRedisCache(),
	}
}

type globalDistrictStatsCacheData struct {
	DistrictUserCount        int    `json:"district_user_count"`
	TotalDistrictPerformance string `json:"total_district_performance"`
}

func (s *teamStatsService) getGlobalDistrictStatsCache(ctx context.Context) (int, decimal.Decimal, bool) {
	cacheKey := frameCache.CacheKey{}.Team().GlobalDistrictStats()

	if s.memoryCache != nil {
		if value, err := s.memoryCache.Get(ctx, cacheKey); err == nil && value != nil && !value.IsNil() {
			var cached globalDistrictStatsCacheData
			if err := value.Struct(&cached); err == nil {
				total, parseErr := decimal.NewFromString(cached.TotalDistrictPerformance)
				if parseErr == nil {
					return cached.DistrictUserCount, total, true
				}
			}
		}
	}

	if s.redisCache != nil {
		if value, err := s.redisCache.Get(ctx, cacheKey); err == nil && value != nil && !value.IsNil() {
			var cached globalDistrictStatsCacheData
			if err := value.Struct(&cached); err == nil {
				total, parseErr := decimal.NewFromString(cached.TotalDistrictPerformance)
				if parseErr == nil {
					if s.memoryCache != nil {
						_ = s.memoryCache.Set(ctx, cacheKey, &cached, globalDistrictStatsCacheTTL)
					}
					return cached.DistrictUserCount, total, true
				}
			}
		}
	}

	return 0, decimal.Zero, false
}

func (s *teamStatsService) setGlobalDistrictStatsCache(ctx context.Context, districtUserCount int, totalDistrictPerformance decimal.Decimal) {
	cacheKey := frameCache.CacheKey{}.Team().GlobalDistrictStats()
	data := &globalDistrictStatsCacheData{
		DistrictUserCount:        districtUserCount,
		TotalDistrictPerformance: totalDistrictPerformance.String(),
	}

	if s.memoryCache != nil {
		_ = s.memoryCache.Set(ctx, cacheKey, data, globalDistrictStatsCacheTTL)
	}
	if s.redisCache != nil {
		_ = s.redisCache.Set(ctx, cacheKey, data, globalDistrictStatsCacheTTL)
	}
}

func (s *teamStatsService) GetOverviewAggByInviteCode(ctx context.Context, inviteCode string) (*OverviewAgg, error) {
	type row struct {
		TeamTotalUserCount      int    `json:"team_total_user_count"`
		TeamTodayNewUserCount   int    `json:"team_today_new_user_count"`
		DirectCount             int    `json:"direct_count"`
		BigTeamCount            int    `json:"big_team_count"`
		SmallTeamSum            int    `json:"small_team_sum"`
		BigTeamPerformance      string `json:"big_team_performance"`
		SmallTeamPerformance    string `json:"small_team_performance"`
		TeamPerformance         string `json:"team_performance"`
		TeamYYSum               string `json:"team_yy_sum"`
		TodayNewPerformance     string `json:"today_new_performance"`
		YesterdayNewPerformance string `json:"yesterday_new_performance"`
	}

	sql := `
WITH RECURSIVE team_tree AS (
	SELECT
		u.id,
		u.invite_code,
		u.created_at,
		u.id AS root_id,
		1 AS level
	FROM user_info u
	WHERE u.parent_invite_code = ?

	UNION ALL

	SELECT
		c.id,
		c.invite_code,
		c.created_at,
		t.root_id,
		t.level + 1
	FROM user_info c
	INNER JOIN team_tree t ON c.parent_invite_code = t.invite_code
),
branch_sizes AS (
	SELECT root_id, COUNT(*)::int AS cnt
	FROM team_tree
	GROUP BY root_id
),
branch_performance AS (
	SELECT
		t.root_id,
		COALESCE(SUM(np.amount), 0) AS perf
	FROM team_tree t
	LEFT JOIN cobo_node_purchase np ON np.user_id = t.id AND np.is_gift = 0
	GROUP BY t.root_id
),
	big_branch AS (
		SELECT root_id
		FROM branch_performance
		ORDER BY perf DESC
		LIMIT 1
	),
team_perf AS (
	SELECT COALESCE(SUM(np.amount), 0) AS total
	FROM cobo_node_purchase np
	INNER JOIN team_tree t ON t.id = np.user_id
	WHERE np.is_gift = 0
),
team_yy AS (
	SELECT COALESCE(SUM(np.amount), 0) AS total
	FROM cobo_node_purchase np
	INNER JOIN team_tree t ON t.id = np.user_id
	WHERE np.is_gift = 0
),
today_perf AS (
	SELECT COALESCE(SUM(o.amount), 0) AS total
	FROM staking_v2_order o
	INNER JOIN team_tree t ON t.id = o.user_id
	WHERE o.source_type = ?
	  AND o.is_gift = 0
	  AND o.created_at >= ?
	  AND o.created_at < ?
),
yesterday_perf AS (
	SELECT COALESCE(SUM(o.amount), 0) AS total
	FROM staking_v2_order o
	INNER JOIN team_tree t ON t.id = o.user_id
	WHERE o.source_type = ?
	  AND o.is_gift = 0
	  AND o.created_at >= ?
	  AND o.created_at < ?
)
SELECT
	(SELECT COUNT(*)::int FROM team_tree) AS team_total_user_count,
	(SELECT COUNT(*)::int
	 FROM team_tree
	 WHERE created_at >= date_trunc('day', timezone('Asia/Shanghai', now()))
	   AND created_at < date_trunc('day', timezone('Asia/Shanghai', now())) + interval '1 day'
	) AS team_today_new_user_count,
	(SELECT COUNT(*)::int FROM team_tree WHERE level = 1) AS direct_count,
	COALESCE((SELECT cnt FROM branch_sizes WHERE root_id = (SELECT root_id FROM big_branch)), 0)::int AS big_team_count,
	COALESCE(
		(SELECT SUM(cnt) FROM branch_sizes)
		- COALESCE((SELECT cnt FROM branch_sizes WHERE root_id = (SELECT root_id FROM big_branch)), 0),
		0
	)::int AS small_team_sum,
	COALESCE((SELECT MAX(perf)::text FROM branch_performance), '0') AS big_team_performance,
	COALESCE((SELECT SUM(perf)::numeric FROM branch_performance), 0)
	  - COALESCE((SELECT MAX(perf)::numeric FROM branch_performance), 0) AS small_team_performance,
	COALESCE((SELECT total::text FROM team_perf), '0') AS team_performance,
	COALESCE((SELECT total::text FROM team_yy), '0') AS team_yy_sum,
	COALESCE((SELECT total::text FROM today_perf), '0') AS today_new_performance,
	COALESCE((SELECT total::text FROM yesterday_perf), '0') AS yesterday_new_performance
`

	cst := time.FixedZone("CST", 8*3600)
	nowCST := time.Now().In(cst)
	todayBizDate := time.Date(nowCST.Year(), nowCST.Month(), nowCST.Day(), 0, 0, 0, 0, cst)
	yesterdayBizDate := todayBizDate.AddDate(0, 0, -1)

	todayStart, todayEnd, err := shared.ResolveSessionDayWindow(ctx, todayBizDate)
	if err != nil {
		return nil, err
	}
	if todayStart.IsZero() || todayEnd.IsZero() || !todayEnd.After(todayStart) {
		todayStart = todayBizDate
		todayEnd = todayBizDate.AddDate(0, 0, 1)
	}

	yesterdayStart, yesterdayEnd, err := shared.ResolveSessionDayWindow(ctx, yesterdayBizDate)
	if err != nil {
		return nil, err
	}
	if yesterdayStart.IsZero() || yesterdayEnd.IsZero() || !yesterdayEnd.After(yesterdayStart) {
		yesterdayStart = yesterdayBizDate
		yesterdayEnd = todayBizDate
	}

	var r row
	if err := g.DB().Ctx(ctx).Raw(
		sql,
		inviteCode,
		consts.StakingV2SourceTypeManualStake, todayStart, todayEnd,
		consts.StakingV2SourceTypeManualStake, yesterdayStart, yesterdayEnd,
	).Scan(&r); err != nil {
		return nil, err
	}

	teamPerformance, err := decimal.NewFromString(r.TeamPerformance)
	if err != nil {
		teamPerformance = decimal.Zero
	}
	teamYYSum, err := decimal.NewFromString(r.TeamYYSum)
	if err != nil {
		teamYYSum = decimal.Zero
	}
	todayNewPerformance, err := decimal.NewFromString(r.TodayNewPerformance)
	if err != nil {
		todayNewPerformance = decimal.Zero
	}
	yesterdayNewPerformance, err := decimal.NewFromString(r.YesterdayNewPerformance)
	if err != nil {
		yesterdayNewPerformance = decimal.Zero
	}
	bigTeamPerformance, err := decimal.NewFromString(r.BigTeamPerformance)
	if err != nil {
		bigTeamPerformance = decimal.Zero
	}
	smallTeamPerformance, err := decimal.NewFromString(r.SmallTeamPerformance)
	if err != nil {
		smallTeamPerformance = decimal.Zero
	}

	// /team/overview 的大小区业绩按 Staking V2 团队业绩口径计算。
	var sv2 struct {
		BigTeamCount         int    `json:"big_team_count"`
		SmallTeamSum         int    `json:"small_team_sum"`
		BigTeamPerformance   string `json:"big_team_performance"`
		SmallTeamPerformance string `json:"small_team_performance"`
	}
	sv2Err := g.DB().Ctx(ctx).Raw(`
		WITH RECURSIVE team_tree AS (
			SELECT u.id, u.invite_code, u.id AS root_id, 1 AS level
			FROM user_info u
			WHERE u.parent_invite_code = ?

			UNION ALL

			SELECT c.id, c.invite_code, t.root_id, t.level + 1
			FROM user_info c
			INNER JOIN team_tree t ON c.parent_invite_code = t.invite_code
		), branch_sizes AS (
			SELECT root_id, COUNT(*)::int AS cnt
			FROM team_tree
			GROUP BY root_id
		), branch_performance AS (
			SELECT
				direct.id AS root_id,
				COALESCE(sv2.personal_performance, 0) + COALESCE(sv2.team_performance, 0) AS perf
			FROM user_info direct
			LEFT JOIN staking_v2_performance sv2 ON sv2.user_id = direct.id
			WHERE direct.parent_invite_code = ?
		), big_branch AS (
			SELECT root_id
			FROM branch_performance
			ORDER BY perf DESC, root_id ASC
			LIMIT 1
		)
		SELECT
			COALESCE((SELECT cnt FROM branch_sizes WHERE root_id = (SELECT root_id FROM big_branch)), 0)::int AS big_team_count,
			COALESCE((SELECT SUM(cnt) FROM branch_sizes), 0)::int
				- COALESCE((SELECT cnt FROM branch_sizes WHERE root_id = (SELECT root_id FROM big_branch)), 0)::int AS small_team_sum,
			COALESCE((SELECT MAX(perf)::text FROM branch_performance), '0') AS big_team_performance,
			(COALESCE((SELECT SUM(perf) FROM branch_performance), 0)
				- COALESCE((SELECT MAX(perf) FROM branch_performance), 0))::text AS small_team_performance
	`, inviteCode, inviteCode).Scan(&sv2)
	if sv2Err != nil {
		return nil, sv2Err
	}
	if sv2.SmallTeamPerformance != "" {
		r.BigTeamCount = sv2.BigTeamCount
		r.SmallTeamSum = sv2.SmallTeamSum
		bigTeamPerformance, err = decimal.NewFromString(sv2.BigTeamPerformance)
		if err != nil {
			return nil, err
		}
		smallTeamPerformance, err = decimal.NewFromString(sv2.SmallTeamPerformance)
		if err != nil {
			return nil, err
		}
	}

	useFullPerfValue, _ := g.DB().Model("user_info").Ctx(ctx).
		Fields("use_full_perf").
		Where("invite_code = ?", inviteCode).
		Value()
	if useFullPerfValue.Bool() {
		smallTeamPerformance = smallTeamPerformance.Add(bigTeamPerformance)
		bigTeamPerformance = decimal.Zero
	}

	return &OverviewAgg{
		TeamTotalUserCount:      r.TeamTotalUserCount,
		TeamTodayNewUserCount:   r.TeamTodayNewUserCount,
		DirectCount:             r.DirectCount,
		BigTeamCount:            r.BigTeamCount,
		SmallTeamSum:            r.SmallTeamSum,
		BigTeamPerformance:      bigTeamPerformance,
		SmallTeamPerformance:    smallTeamPerformance,
		TeamPerformance:         teamPerformance,
		TeamYYSum:               teamYYSum,
		TodayNewPerformance:     todayNewPerformance,
		YesterdayNewPerformance: yesterdayNewPerformance,
	}, nil
}

func (s *teamStatsService) GetL1L2PurchaseTotalByInviteCode(ctx context.Context, inviteCode string) (l1Total, l2Total decimal.Decimal, err error) {
	type row struct {
		L1Total string `json:"l1_total"`
		L2Total string `json:"l2_total"`
	}

	sql := `
WITH l1 AS (
	SELECT id, invite_code
	FROM user_info
	WHERE parent_invite_code = ?
),
l2 AS (
	SELECT u.id
	FROM user_info u
	INNER JOIN l1 ON u.parent_invite_code = l1.invite_code
),
l1_sum AS (
	SELECT COALESCE(SUM(amount), 0) AS total
	FROM cobo_node_purchase
	WHERE user_id IN (SELECT id FROM l1)
	  AND is_gift = 0
	  AND status = ?
),
l2_sum AS (
	SELECT COALESCE(SUM(amount), 0) AS total
	FROM cobo_node_purchase
	WHERE user_id IN (SELECT id FROM l2)
	  AND is_gift = 0
	  AND status = ?
)
SELECT
	COALESCE((SELECT total::text FROM l1_sum), '0') AS l1_total,
	COALESCE((SELECT total::text FROM l2_sum), '0') AS l2_total
`

	var r row
	if err = g.DB().Ctx(ctx).Raw(sql, inviteCode, coboEntity.NodeStatusRunning, coboEntity.NodeStatusRunning).Scan(&r); err != nil {
		return decimal.Zero, decimal.Zero, err
	}

	l1Total, err = decimal.NewFromString(r.L1Total)
	if err != nil {
		return decimal.Zero, decimal.Zero, err
	}
	l2Total, err = decimal.NewFromString(r.L2Total)
	if err != nil {
		return decimal.Zero, decimal.Zero, err
	}

	return l1Total, l2Total, nil
}

func (s *teamStatsService) GetGlobalDistrictStats(ctx context.Context) (districtUserCount int, totalDistrictPerformance decimal.Decimal, err error) {
	if cachedCount, cachedTotal, ok := s.getGlobalDistrictStatsCache(ctx); ok {
		return cachedCount, cachedTotal, nil
	}

	type row struct {
		DistrictUserCount        int    `json:"district_user_count"`
		TotalDistrictPerformance string `json:"total_district_performance"`
	}

	const sql = `
WITH direct_branch AS (
	SELECT
		parent.id AS user_id,
		COALESCE(sv2.personal_performance, 0) + COALESCE(sv2.team_performance, 0) AS branch_perf
	FROM user_info parent
	INNER JOIN user_info child ON child.parent_invite_code = parent.invite_code
	LEFT JOIN staking_v2_performance sv2 ON sv2.user_id = child.id
), user_district AS (
	SELECT
		user_id,
		COALESCE(SUM(branch_perf), 0) - COALESCE(MAX(branch_perf), 0) AS small_perf
	FROM direct_branch
	GROUP BY user_id
)
SELECT
	COUNT(*) FILTER (WHERE small_perf > 0)::int AS district_user_count,
	COALESCE(SUM(CASE WHEN small_perf > 0 THEN small_perf ELSE 0 END)::text, '0') AS total_district_performance
FROM user_district
`

	var r row
	if err = g.DB().Ctx(ctx).Raw(sql).Scan(&r); err != nil {
		return 0, decimal.Zero, err
	}

	totalDistrictPerformance, err = decimal.NewFromString(r.TotalDistrictPerformance)
	if err != nil {
		return 0, decimal.Zero, err
	}

	s.setGlobalDistrictStatsCache(ctx, r.DistrictUserCount, totalDistrictPerformance)

	return r.DistrictUserCount, totalDistrictPerformance, nil
}
