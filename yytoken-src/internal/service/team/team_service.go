package team

import (
	"context"
	"fmt"
	"strings"
	"time"

	frameCache "XWFrame/internal/frame/cache"
	"XWFrame/internal/frame/consts"
	coboRepo "XWFrame/internal/repository/cobo"
	"XWFrame/internal/repository/team"
	"XWFrame/internal/service/group_match"
	"XWFrame/internal/service/shared"
	"XWFrame/internal/service/team/model"
	"XWFrame/internal/service/teamstats"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcache"
	"github.com/shopspring/decimal"
)

const teamOverviewCacheTTL = 30 * time.Second

type ITeamService interface {
	GetOverview(ctx context.Context) (*model.TeamOverview, error)
	GetDirect(ctx context.Context, page, pageSize int, startDate, endDate string) ([]*model.TeamDirectItem, int, error)
	GetLastBizDayRewardSummary(ctx context.Context) (*model.LastBizDayRewardSummary, error)
	GetAllTimeStatsByInviteCode(ctx context.Context, inviteCode string, excludeIDs []int64) (*model.TeamAllTimeStats, error)
}

type teamService struct {
	teamRepo     team.ITeamRepository
	coboPerfRepo coboRepo.IPerformanceRepository
	teamStatsSvc teamstats.ITeamStatsService
	memoryCache  *gcache.Cache
	redisCache   *gcache.Cache
}

type todayGroupPerfRow struct {
	PersonalGroupAmount  decimal.Decimal `json:"personal_group_amount"`
	SmallTeamGroupAmount decimal.Decimal `json:"small_team_group_amount"`
	TeamGroupAmount      decimal.Decimal `json:"team_group_amount"`
	TotalSmallTeamAmount decimal.Decimal `json:"total_small_team_amount"`
	DistrictUserCount    int             `json:"district_user_count"`
	DirectCount          int             `json:"direct_count"`
	TeamTotalCount       int             `json:"team_total_count"`
	BigTeamCount         int             `json:"big_team_count"`
	SmallTeamSum         int             `json:"small_team_sum"`
}

type directNodePerfRow struct {
	TeamPerformance decimal.Decimal `json:"team_performance"`
}

// staking_v2 业务上即"购买三倍券"，source_type=1 为手动购买三倍券
type directTriplePerfRow struct {
	TeamTriplePerformance decimal.Decimal `json:"team_triple_performance"`
}

func normalizeRewardLevel(level int) int {
	if level <= 0 {
		return 0
	}
	if level > 5 {
		return 5
	}
	return level
}

func resolveLeadershipLevelByShares(levelsCount int, shares int64, levelsDailyShares []int64) int {
	if levelsCount <= 0 || shares <= 0 {
		return 0
	}
	calculated := 0
	for i := 0; i < levelsCount && i < len(levelsDailyShares); i++ {
		if shares >= levelsDailyShares[i] {
			calculated = i + 1
		}
	}
	return normalizeRewardLevel(calculated)
}

var teamServiceInstance *teamService

func (s *teamService) resolveCurrentSessionID(ctx context.Context) (int64, error) {
	return group_match.NewGroupMatchService().ResolveCurrentSessionID(ctx)
}

func Svc() ITeamService {
	if teamServiceInstance == nil {
		teamServiceInstance = &teamService{
			teamRepo:     team.NewTeamRepository(),
			coboPerfRepo: coboRepo.NewPerformanceRepository(),
			teamStatsSvc: teamstats.NewTeamStatsService(),
			memoryCache:  frameCache.GetMemoryCache(),
			redisCache:   frameCache.GetRedisCache(),
		}
	}
	return teamServiceInstance
}

func (s *teamService) getOverviewCache(ctx context.Context, userID int64) (*model.TeamOverview, bool) {
	cacheKey := frameCache.CacheKey{}.Team().Overview(userID)

	if s.memoryCache != nil {
		if value, err := s.memoryCache.Get(ctx, cacheKey); err == nil && value != nil && !value.IsNil() {
			var cached model.TeamOverview
			if err := value.Struct(&cached); err == nil {
				return &cached, true
			}
		}
	}

	if s.redisCache != nil {
		if value, err := s.redisCache.Get(ctx, cacheKey); err == nil && value != nil && !value.IsNil() {
			var cached model.TeamOverview
			if err := value.Struct(&cached); err == nil {
				if s.memoryCache != nil {
					_ = s.memoryCache.Set(ctx, cacheKey, &cached, teamOverviewCacheTTL)
				}
				return &cached, true
			}
		}
	}

	return nil, false
}

func (s *teamService) setOverviewCache(ctx context.Context, userID int64, overview *model.TeamOverview) {
	if overview == nil {
		return
	}
	cacheKey := frameCache.CacheKey{}.Team().Overview(userID)
	if s.memoryCache != nil {
		_ = s.memoryCache.Set(ctx, cacheKey, overview, teamOverviewCacheTTL)
	}
	if s.redisCache != nil {
		_ = s.redisCache.Set(ctx, cacheKey, overview, teamOverviewCacheTTL)
	}
}

func (s *teamService) GetOverview(ctx context.Context) (*model.TeamOverview, error) {
	// 当前团队概览业绩口径（2026-04）：仅统计 cobo_node_purchase，
	// 不再使用历史 user_performance 快照口径。此处为实时查询，不走快照表。
	userID := ctx.Value("user_id")
	if userID == nil {
		return nil, gerror.New("未获取到用户信息")
	}
	uid, ok := userID.(int64)
	if !ok {
		return nil, gerror.New("用户ID类型错误")
	}

	if cached, ok := s.getOverviewCache(ctx, uid); ok {
		return cached, nil
	}

	selfUser, err := s.teamRepo.GetUserByID(ctx, uid)
	if err != nil {
		return nil, err
	}
	if selfUser == nil {
		empty := &model.TeamOverview{RewardLevel: 0, TeamPerformance: "0.00", MyDistrictShareRatio: "0.000000", TeamUserCount: 0}
		s.setOverviewCache(ctx, uid, empty)
		return empty, nil
	}

	sessionID, err := s.resolveCurrentSessionID(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "[团队概览] 获取当前场次失败: %v", err)
		sessionID = 0
	}
	bizToday := s.resolveCurrentBizDate(ctx, sessionID)
	bizYesterday := bizToday.AddDate(0, 0, -1)

	partStats, err := s.getParticipationDistrictStats(ctx, uid, sessionID)
	if err != nil {
		g.Log().Warningf(ctx, "[团队概览] 查询参与口径小区统计失败: %v", err)
		partStats = &participationDistrictStats{}
	}

	todayPerf, todayPerfErr := s.getSessionGroupPerformance(ctx, uid, sessionID)
	if todayPerfErr != nil {
		g.Log().Warningf(ctx, "[团队概览] 查询group_performance失败: %v", todayPerfErr)
		todayPerf = &todayGroupPerfRow{}
	}

	mintAndStake, err := s.getTeamMintAndStakeStatsByBizDate(ctx, selfUser.InviteCode, bizToday, bizYesterday)
	if err != nil {
		g.Log().Warningf(ctx, "[团队概览] 查询团队铸币/新增统计失败: %v", err)
		mintAndStake = &teamMintAndStakeStats{}
	}

	rewardStats, err := s.getTeamRewardStats(ctx, uid)
	if err != nil {
		g.Log().Warningf(ctx, "[团队概览] 查询领导奖/团队奖励统计失败: %v", err)
		rewardStats = &teamRewardStats{}
	}

	agg, err := s.teamStatsSvc.GetOverviewAggByInviteCode(ctx, selfUser.InviteCode)
	if err != nil {
		return nil, err
	}
	networkDistrictUserCount, networkDistrictPerformance, err := s.teamStatsSvc.GetGlobalDistrictStats(ctx)
	if err != nil {
		return nil, err
	}

	if agg.BigTeamCount > 0 || agg.SmallTeamSum > 0 {
		partStats.BigTeamCount = agg.BigTeamCount
		partStats.SmallTeamSum = agg.SmallTeamSum
	}
	// /team/overview 大小区业绩展示使用 Staking V2 团队业绩口径。
	partStats.BigTeamPerformance = agg.BigTeamPerformance
	partStats.SmallTeamPerformance = agg.SmallTeamPerformance
	partStats.NetworkDistrictUserCount = networkDistrictUserCount
	partStats.NetworkDistrictPerformance = networkDistrictPerformance
	if selfUser.UseFullPerf {
		partStats.NetworkDistrictPerformance = partStats.NetworkDistrictPerformance.Add(agg.BigTeamPerformance)
	}
	if partStats.NetworkDistrictPerformance.LessThan(partStats.SmallTeamPerformance) {
		partStats.NetworkDistrictPerformance = partStats.SmallTeamPerformance
	}
	if partStats.NetworkDistrictPerformance.GreaterThan(decimal.Zero) {
		partStats.MyDistrictShareRatio = partStats.SmallTeamPerformance.Div(partStats.NetworkDistrictPerformance)
		if partStats.MyDistrictShareRatio.GreaterThan(decimal.NewFromInt(1)) {
			partStats.MyDistrictShareRatio = decimal.NewFromInt(1)
		}
	} else {
		partStats.MyDistrictShareRatio = decimal.Zero
	}

	teamTotalUserCount := agg.TeamTotalUserCount
	directCount := agg.DirectCount
	if teamTotalUserCount == 0 || directCount == 0 {
		if teamTotalUserCount == 0 {
			teamTotalUserCount = todayPerf.TeamTotalCount
		}
		if directCount == 0 {
			directCount = todayPerf.DirectCount
		}
	}

	rewardLevel := normalizeRewardLevel(selfUser.VipLevel)
	if cfg, cfgErr := group_match.NewGroupMatchService().GetLeadershipLevelsConfig(ctx); cfgErr != nil {
		g.Log().Warningf(ctx, "[团队概览] 读取领导奖等级配置失败: %v", cfgErr)
	} else {
		thresholds := make([]int64, 0, len(cfg.Levels))
		for _, level := range cfg.Levels {
			if level == nil {
				continue
			}
			thresholds = append(thresholds, level.DailyShares)
		}
		shares := todayPerf.TeamGroupAmount.Div(decimal.NewFromInt(100)).Floor().IntPart()
		calculatedLevel := resolveLeadershipLevelByShares(len(thresholds), shares, thresholds)
		if calculatedLevel > rewardLevel {
			rewardLevel = calculatedLevel
		}
	}

	res := &model.TeamOverview{
		TeamTotalUserCount:         teamTotalUserCount,
		TeamUserCount:              teamTotalUserCount,
		TeamTodayNewUserCount:      agg.TeamTodayNewUserCount,
		DirectCount:                directCount,
		RewardLevel:                rewardLevel,
		BigTeamCount:               partStats.BigTeamCount,
		SmallTeamSum:               partStats.SmallTeamSum,
		BigTeamPerformance:         utils.FormatDecimal(partStats.BigTeamPerformance),
		SmallTeamPerformance:       utils.FormatDecimal(partStats.SmallTeamPerformance),
		NetworkDistrictUserCount:   partStats.NetworkDistrictUserCount,
		NetworkDistrictPerformance: utils.FormatDecimal(partStats.NetworkDistrictPerformance),
		MyDistrictShareRatio:       partStats.MyDistrictShareRatio.StringFixed(6),
		TeamPerformance:            utils.FormatDecimal(mintAndStake.TeamMintFlow),
		TeamTodayMint:              utils.FormatDecimal(mintAndStake.TeamTodayMint),
		TeamYesterdayMint:          utils.FormatDecimal(mintAndStake.TeamYesterdayMint),
		TeamYYSum:                  utils.FormatDecimal(agg.TeamYYSum),
		TodayNewPerformance:        utils.FormatDecimal(mintAndStake.TodayStakeNew),
		YesterdayNewPerformance:    utils.FormatDecimal(mintAndStake.YesterdayStakeNew),
		LeadershipReward:           utils.FormatDecimal(rewardStats.LeadershipReward),
		LeadershipWeightReward:     utils.FormatDecimal(rewardStats.LeadershipWeightReward),
		TeamReward:                 utils.FormatDecimal(rewardStats.TeamReward),
	}
	s.setOverviewCache(ctx, uid, res)
	return res, nil
}

type teamMintAndStakeStats struct {
	TeamMintFlow      decimal.Decimal `json:"team_mint_flow"`
	TeamTodayMint     decimal.Decimal `json:"team_today_mint"`
	TeamYesterdayMint decimal.Decimal `json:"team_yesterday_mint"`
	TodayStakeNew     decimal.Decimal `json:"today_stake_new"`
	YesterdayStakeNew decimal.Decimal `json:"yesterday_stake_new"`
}

func (s *teamService) getTeamMintAndStakeStats(ctx context.Context, inviteCode string) (*teamMintAndStakeStats, error) {
	return s.getTeamMintAndStakeStatsByBizDate(ctx, inviteCode, shanghaiDateOnly(time.Now()), shanghaiDateOnly(time.Now()).AddDate(0, 0, -1))
}

func (s *teamService) getTeamMintAndStakeStatsByBizDate(ctx context.Context, inviteCode string, bizToday time.Time, bizYesterday time.Time) (*teamMintAndStakeStats, error) {
	todayStart, todayEnd, err := resolveSessionWindowByBizDate(ctx, bizToday)
	if err != nil {
		return nil, err
	}
	yesterdayStart, yesterdayEnd, err := resolveSessionWindowByBizDate(ctx, bizYesterday)
	if err != nil {
		return nil, err
	}

	row := &teamMintAndStakeStats{}
	err = g.DB().Ctx(ctx).Raw(`
		WITH RECURSIVE team_tree AS (
			SELECT id, invite_code
			FROM user_info
			WHERE parent_invite_code = ?
			UNION ALL
			SELECT u.id, u.invite_code
			FROM user_info u
			JOIN team_tree t ON u.parent_invite_code = t.invite_code
		)
		SELECT
			COALESCE((
				SELECT SUM(o.amount)
				FROM group_match_order o
				JOIN team_tree t ON t.id = o.user_id
			), 0) AS team_mint_flow,
			COALESCE((
				SELECT SUM(o.amount)
				FROM group_match_order o
				JOIN group_match_session sess ON sess.id = o.session_id
				JOIN team_tree t ON t.id = o.user_id
				WHERE sess.session_date = ?::date
			), 0) AS team_today_mint,
			COALESCE((
				SELECT SUM(o.amount)
				FROM group_match_order o
				JOIN group_match_session sess ON sess.id = o.session_id
				JOIN team_tree t ON t.id = o.user_id
				WHERE sess.session_date = ?::date
			), 0) AS team_yesterday_mint,
			COALESCE((
				SELECT SUM(o.amount)
				FROM staking_v2_order o
				JOIN team_tree t ON t.id = o.user_id
				WHERE o.created_at >= ?
				  AND o.created_at < ?
			), 0) AS today_stake_new,
			COALESCE((
				SELECT SUM(o.amount)
				FROM staking_v2_order o
				JOIN team_tree t ON t.id = o.user_id
				WHERE o.created_at >= ?
				  AND o.created_at < ?
			), 0) AS yesterday_stake_new
	`, inviteCode, bizToday.Format("2006-01-02"), bizYesterday.Format("2006-01-02"), todayStart, todayEnd, yesterdayStart, yesterdayEnd).Scan(row)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *teamService) GetAllTimeStatsByInviteCode(ctx context.Context, inviteCode string, excludeIDs []int64) (*model.TeamAllTimeStats, error) {
	type row struct {
		PersonalTripleRaw  string `json:"personal_triple_raw"`
		TeamTripleRaw      string `json:"team_triple_raw"`
		PersonalGiftTriple string `json:"personal_gift_triple"`
		TeamGiftTriple     string `json:"team_gift_triple"`
		PersonalMint       string `json:"personal_mint"`
		TeamMint           string `json:"team_mint"`
	}

	var excludeCond1, excludeCond2 string
	if len(excludeIDs) > 0 {
		var sb strings.Builder
		sb.WriteString(" AND u.id NOT IN (")
		for i, id := range excludeIDs {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf("%d", id))
		}
		sb.WriteString(")")
		excludeCond1 = sb.String()

		var sb2 strings.Builder
		sb2.WriteString(" AND c.id NOT IN (")
		for i, id := range excludeIDs {
			if i > 0 {
				sb2.WriteString(",")
			}
			sb2.WriteString(fmt.Sprintf("%d", id))
		}
		sb2.WriteString(")")
		excludeCond2 = sb2.String()
	}

	sql := fmt.Sprintf(`
		WITH RECURSIVE team_tree AS (
			SELECT id, invite_code
			FROM user_info
			WHERE parent_invite_code = ?%s
			UNION ALL
			SELECT u.id, u.invite_code
			FROM user_info u
			JOIN team_tree t ON u.parent_invite_code = t.invite_code
			WHERE 1=1%s
		),
		me AS (
			SELECT id FROM user_info WHERE invite_code = ?
		)
		SELECT
			COALESCE((
				SELECT SUM(o.amount)
				FROM staking_v2_order o
				WHERE o.user_id = (SELECT id FROM me)
			), 0)::text AS personal_triple_raw,
			COALESCE((
				SELECT SUM(o.amount)
				FROM staking_v2_order o
				WHERE o.user_id IN (SELECT id FROM team_tree)
			), 0)::text AS team_triple_raw,
			COALESCE((
				SELECT SUM(g.grant_amount::numeric)
				FROM cobo_node_token_grant g
				WHERE g.user_id = (SELECT id FROM me)
				  AND g.token_type = 'Triple'
				  AND g.status = 1
			), 0)::text AS personal_gift_triple,
			COALESCE((
				SELECT SUM(g.grant_amount::numeric)
				FROM cobo_node_token_grant g
				WHERE g.user_id IN (SELECT id FROM team_tree)
				  AND g.token_type = 'Triple'
				  AND g.status = 1
			), 0)::text AS team_gift_triple,
			COALESCE((
				SELECT COUNT(*)
				FROM group_match_order o
				WHERE o.user_id = (SELECT id FROM me)
			), 0)::text AS personal_mint,
			COALESCE((
				SELECT COUNT(*)
				FROM group_match_order o
				WHERE o.user_id IN (SELECT id FROM team_tree)
			), 0)::text AS team_mint
	`, excludeCond1, excludeCond2)

	var r row
	if err := g.DB().Ctx(ctx).Raw(sql, inviteCode, inviteCode).Scan(&r); err != nil {
		return nil, err
	}

	personalTripleRaw, _ := decimal.NewFromString(r.PersonalTripleRaw)
	teamTripleRaw, _ := decimal.NewFromString(r.TeamTripleRaw)
	personalGift, _ := decimal.NewFromString(r.PersonalGiftTriple)
	teamGift, _ := decimal.NewFromString(r.TeamGiftTriple)

	personalTriple := personalTripleRaw.Sub(personalGift)
	if personalTriple.LessThan(decimal.Zero) {
		personalTriple = decimal.Zero
	}
	teamTriple := teamTripleRaw.Sub(teamGift)
	if teamTriple.LessThan(decimal.Zero) {
		teamTriple = decimal.Zero
	}

	return &model.TeamAllTimeStats{
		PersonalTriple:     personalTriple.StringFixed(2),
		TeamTriple:         teamTriple.StringFixed(2),
		PersonalGiftTriple: r.PersonalGiftTriple,
		TeamGiftTriple:     r.TeamGiftTriple,
		PersonalMint:       r.PersonalMint,
		TeamMint:           r.TeamMint,
	}, nil
}

func (s *teamService) resolveCurrentBizDate(ctx context.Context, sessionID int64) time.Time {
	if sessionID <= 0 {
		return shanghaiDateOnly(time.Now())
	}

	row, err := g.DB().GetOne(ctx, "SELECT session_date FROM group_match_session WHERE id = ?", sessionID)
	if err != nil {
		g.Log().Warningf(ctx, "[团队概览] 查询当前场次业务日失败: sessionID=%d err=%v", sessionID, err)
		return shanghaiDateOnly(time.Now())
	}
	if row == nil {
		return shanghaiDateOnly(time.Now())
	}
	bizDate := row["session_date"].Time()
	if bizDate.IsZero() {
		return shanghaiDateOnly(time.Now())
	}
	return shanghaiDateOnly(bizDate)
}

func resolveSessionWindowByBizDate(ctx context.Context, bizDate time.Time) (time.Time, time.Time, error) {
	windowStart, windowEnd, err := shared.ResolveSessionDayWindow(ctx, bizDate)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if windowStart.IsZero() || windowEnd.IsZero() || !windowEnd.After(windowStart) {
		return time.Time{}, time.Time{}, nil
	}
	return windowStart, windowEnd, nil
}

func shanghaiDateOnly(t time.Time) time.Time {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	}
	inLoc := t.In(loc)
	return time.Date(inLoc.Year(), inLoc.Month(), inLoc.Day(), 0, 0, 0, 0, loc)
}

type teamRewardStats struct {
	LeadershipReward       decimal.Decimal `json:"leadership_reward"`
	LeadershipWeightReward decimal.Decimal `json:"leadership_weight_reward"`
	TeamReward             decimal.Decimal `json:"team_reward"`
}

type participationDistrictStats struct {
	BigTeamCount               int             `json:"big_team_count"`
	SmallTeamSum               int             `json:"small_team_sum"`
	BigTeamPerformance         decimal.Decimal `json:"big_team_performance"`
	SmallTeamPerformance       decimal.Decimal `json:"small_team_performance"`
	TeamGroupAmount            decimal.Decimal `json:"team_group_amount"`
	NetworkDistrictUserCount   int             `json:"network_district_user_count"`
	NetworkDistrictPerformance decimal.Decimal `json:"network_district_performance"`
	MyDistrictShareRatio       decimal.Decimal `json:"my_district_share_ratio"`
}

func (s *teamService) getParticipationDistrictStats(ctx context.Context, userID int64, sessionID int64) (*participationDistrictStats, error) {
	if sessionID == 0 {
		return &participationDistrictStats{}, nil
	}

	type row struct {
		BigTeamCount               int             `json:"big_team_count"`
		SmallTeamSum               int             `json:"small_team_sum"`
		TeamGroupAmount            decimal.Decimal `json:"team_group_amount"`
		SmallTeamPerformance       decimal.Decimal `json:"small_team_performance"`
		NetworkDistrictUserCount   int             `json:"network_district_user_count"`
		NetworkDistrictPerformance decimal.Decimal `json:"network_district_performance"`
	}

	var r row
	err := g.DB().Ctx(ctx).Raw(`
		SELECT
			COALESCE(gp.big_team_count, 0) AS big_team_count,
			COALESCE(gp.small_team_sum, 0) AS small_team_sum,
			COALESCE(gp.team_group_amount, 0) AS team_group_amount,
			COALESCE(gp.small_team_group_amount, 0) AS small_team_performance,
			COALESCE(s.district_user_count, 0) AS network_district_user_count,
			COALESCE(s.total_small_team_amount, 0) AS network_district_performance
		FROM (SELECT 1) AS pivot
		LEFT JOIN group_performance gp ON gp.session_id = ? AND gp.user_id = ?
		LEFT JOIN group_performance_summary s ON s.session_id = ?
	`, sessionID, userID, sessionID).Scan(&r)
	if err != nil {
		return nil, err
	}

	bigPerf := r.TeamGroupAmount.Sub(r.SmallTeamPerformance)
	smallPerf := r.SmallTeamPerformance
	networkPerf := r.NetworkDistrictPerformance

	share := decimal.Zero
	if networkPerf.LessThan(smallPerf) {
		networkPerf = smallPerf
	}
	if networkPerf.GreaterThan(decimal.Zero) {
		share = smallPerf.Div(networkPerf)
		if share.LessThan(decimal.Zero) {
			share = decimal.Zero
		}
		if share.GreaterThan(decimal.NewFromInt(1)) {
			share = decimal.NewFromInt(1)
		}
	}

	return &participationDistrictStats{
		BigTeamCount:               r.BigTeamCount,
		SmallTeamSum:               r.SmallTeamSum,
		BigTeamPerformance:         bigPerf,
		SmallTeamPerformance:       smallPerf,
		TeamGroupAmount:            r.TeamGroupAmount,
		NetworkDistrictUserCount:   r.NetworkDistrictUserCount,
		NetworkDistrictPerformance: networkPerf,
		MyDistrictShareRatio:       share,
	}, nil
}

func (s *teamService) getTeamRewardStats(ctx context.Context, userID int64) (*teamRewardStats, error) {
	row := &teamRewardStats{}
	err := g.DB().Ctx(ctx).Raw(`
		SELECT
			COALESCE((SELECT SUM(reward_amount) FROM group_purchase_leadership_reward_detail WHERE user_id = ?), 0) AS leadership_reward,
			COALESCE((SELECT SUM(reward_amount) FROM group_purchase_leadership_weight_reward_detail WHERE user_id = ?), 0) AS leadership_weight_reward,
			COALESCE((SELECT SUM(amount) FROM (
				SELECT direct_reward_amount AS amount FROM cobo_node_purchase WHERE direct_reward_user_id = ? AND direct_reward_amount > 0 AND is_gift = 0
				UNION ALL
				SELECT amount FROM cobo_balance_change_log WHERE user_id = ? AND change_type = 'staking_v2_referral_direct'
				UNION ALL
				SELECT amount FROM cobo_balance_change_log WHERE user_id = ? AND change_type = 'staking_v2_referral_indirect'
				UNION ALL
				SELECT granted_amount FROM group_match_team_reward_distribution WHERE user_id = ? AND COALESCE(granted_amount, 0) > 0
				UNION ALL
				SELECT granted_amount FROM group_purchase_leadership_reward_detail WHERE user_id = ? AND COALESCE(granted_amount, 0) > 0
				UNION ALL
				SELECT granted_amount FROM group_purchase_leadership_weight_reward_detail WHERE user_id = ? AND COALESCE(granted_amount, 0) > 0
				UNION ALL
				SELECT SUM(reward_amount) AS amount FROM group_match_order WHERE user_id = ? AND is_winner = true GROUP BY session_id HAVING COALESCE(SUM(reward_amount), 0) > 0
				UNION ALL
				SELECT amount FROM us_stock_reward_settlement WHERE user_id = ? AND amount > 0
			) t), 0) AS team_reward
	`, userID, userID, userID, userID, userID, userID, userID, userID, userID, userID).Scan(row)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *teamService) getSessionGroupPerformance(ctx context.Context, userID int64, sessionID int64) (*todayGroupPerfRow, error) {
	row := &todayGroupPerfRow{}
	err := g.DB().Ctx(ctx).Raw(`
		WITH me AS (
			SELECT gp.personal_group_amount, gp.small_team_group_amount, gp.team_group_amount,
			       gp.direct_count, gp.team_total_count, gp.big_team_count, gp.small_team_sum
			FROM group_performance gp
			WHERE gp.session_id = ? AND gp.user_id = ?
		), total AS (
			SELECT s.total_small_team_amount, s.district_user_count
			FROM group_performance_summary s
			WHERE s.session_id = ?
		)
		SELECT COALESCE(me.personal_group_amount, 0) AS personal_group_amount,
			COALESCE(me.small_team_group_amount, 0) AS small_team_group_amount,
			COALESCE(me.team_group_amount, 0) AS team_group_amount,
			COALESCE(me.direct_count, 0) AS direct_count,
			COALESCE(me.team_total_count, 0) AS team_total_count,
			COALESCE(me.big_team_count, 0) AS big_team_count,
			COALESCE(me.small_team_sum, 0) AS small_team_sum,
			COALESCE(total.total_small_team_amount, 0) AS total_small_team_amount,
			COALESCE(total.district_user_count, 0) AS district_user_count
		FROM (SELECT 1) AS pivot
		LEFT JOIN total ON true
		LEFT JOIN me ON true
	`, sessionID, userID, sessionID).Scan(row)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *teamService) GetDirect(ctx context.Context, page, pageSize int, startDate, endDate string) ([]*model.TeamDirectItem, int, error) {
	// 数据验证：从上下文获取当前用户ID
	userID := ctx.Value("user_id")
	if userID == nil {
		return nil, 0, gerror.New("未获取到用户信息")
	}
	uid, ok := userID.(int64)
	if !ok {
		return nil, 0, gerror.New("用户ID类型错误")
	}

	// 数据验证：分页参数
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 获取用户信息
	user, err := s.teamRepo.GetUserByID(ctx, uid)
	if err != nil {
		return nil, 0, err
	}
	if user == nil {
		return []*model.TeamDirectItem{}, 0, nil
	}

	// 业务逻辑：获取直推用户列表（按最新 group_performance + 节点/三倍券业绩降序）
	users, total, err := s.teamRepo.GetDirectReferralsByWalletAddress(ctx, user.WalletAddress, page, pageSize, startDate, endDate)
	if err != nil {
		return nil, 0, err
	}

	// 批量查询最新 group_performance 拼团参与业绩
	userIDs := make([]int64, 0, len(users))
	for _, u := range users {
		userIDs = append(userIDs, u.Id)
	}
	perfMap, err := s.getDirectParticipationPerformanceBatch(ctx, userIDs)
	if err != nil {
		g.Log().Warningf(ctx, "[团队直推] 批量查询拼团参与业绩失败: %v", err)
		perfMap = make(map[int64]*todayGroupPerfRow)
	}
	nodePerfMap, err := s.getDirectNodePerformanceBatch(ctx, userIDs)
	if err != nil {
		g.Log().Warningf(ctx, "[团队直推] 批量查询节点业绩失败: %v", err)
		nodePerfMap = make(map[int64]*directNodePerfRow)
	}
	triplePerfMap, err := s.getDirectTeamTriplePerformanceBatch(ctx, userIDs)
	if err != nil {
		g.Log().Warningf(ctx, "[团队直推] 批量查询三倍券业绩失败: %v", err)
		triplePerfMap = make(map[int64]*directTriplePerfRow)
	}

	// 业务逻辑：为每个用户查询业绩并格式化
	items := make([]*model.TeamDirectItem, 0, len(users))
	for _, user := range users {
		personalPerformance := "0.00"
		teamPerformance := "0.00"
		teamNodePerformance := "0.00"
		teamTriplePerformance := "0.00"

		if p, ok := perfMap[user.Id]; ok {
			personalPerformance = p.PersonalGroupAmount.StringFixed(2)
			teamPerformance = p.TeamGroupAmount.StringFixed(2)
		}
		if p, ok := nodePerfMap[user.Id]; ok {
			teamNodePerformance = p.TeamPerformance.StringFixed(2)
		}
		if p, ok := triplePerfMap[user.Id]; ok {
			teamTriplePerformance = p.TeamTriplePerformance.StringFixed(2)
		}

		// 格式化用户创建时间
		createdAt := user.CreatedAt.Format(consts.TimeFormatDateTime)

		items = append(items, &model.TeamDirectItem{
			Address:               user.WalletAddress,
			PersonalPerformance:   personalPerformance,
			TeamPerformance:       teamPerformance,
			TeamNodePerformance:   teamNodePerformance,
			TeamTriplePerformance: teamTriplePerformance,
			CreatedAt:             createdAt,
		})
	}

	return items, total, nil
}

func (s *teamService) getDirectNodePerformanceBatch(ctx context.Context, userIDs []int64) (map[int64]*directNodePerfRow, error) {
	if len(userIDs) == 0 {
		return make(map[int64]*directNodePerfRow), nil
	}

	placeholders := make([]string, len(userIDs))
	args := make([]interface{}, 0, len(userIDs))
	for i, id := range userIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args = append(args, id)
	}

	rows, err := g.DB().GetAll(ctx, fmt.Sprintf(`
		SELECT user_id,
		       COALESCE(team_performance, 0) AS team_performance,
		       COALESCE(personal_performance, 0) AS personal_performance
		FROM cobo_performance
		WHERE user_id IN (%s)
	`, strings.Join(placeholders, ", ")), args...)
	if err != nil {
		return nil, err
	}

	result := make(map[int64]*directNodePerfRow, len(rows))
	for _, row := range rows {
		uid := row["user_id"].Int64()
		teamPerf, _ := decimal.NewFromString(row["team_performance"].String())
		personalPerf, _ := decimal.NewFromString(row["personal_performance"].String())
		result[uid] = &directNodePerfRow{TeamPerformance: teamPerf.Add(personalPerf)}
	}

	return result, nil
}

func (s *teamService) GetLastBizDayRewardSummary(ctx context.Context) (*model.LastBizDayRewardSummary, error) {
	userID := ctx.Value("user_id")
	if userID == nil {
		return nil, gerror.New("未获取到用户信息")
	}
	uid, ok := userID.(int64)
	if !ok {
		return nil, gerror.New("用户ID类型错误")
	}

	sessionID, err := s.resolveCurrentSessionID(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "[团队奖励汇总] 获取当前场次失败: %v", err)
		sessionID = 0
	}
	bizToday := s.resolveCurrentBizDate(ctx, sessionID)
	bizYesterday := bizToday.AddDate(0, 0, -1)

	windowStart, windowEnd, err := resolveSessionWindowByBizDate(ctx, bizYesterday)
	if err != nil {
		return nil, gerror.Wrap(err, "resolve last business day window failed")
	}

	res := &model.LastBizDayRewardSummary{
		SettlementTime:   "",
		GroupPerformance: "0.00",
		LeaderNew:        "0.00",
		AllReward:        "0.00",
	}
	if !windowEnd.IsZero() {
		res.SettlementTime = windowEnd.Format("2006-01-02 15:04:05")
	}

	selfUser, err := s.teamRepo.GetUserByID(ctx, uid)
	if err != nil {
		return nil, err
	}
	if selfUser == nil {
		return res, nil
	}

	row := struct {
		GroupPerformance decimal.Decimal `json:"group_performance"`
		LeaderNew        decimal.Decimal `json:"leader_new"`
		AllReward        decimal.Decimal `json:"all_reward"`
	}{}

	queryErr := g.DB().Ctx(ctx).Raw(`
		WITH RECURSIVE team_tree AS (
			SELECT u.id, u.invite_code
			FROM user_info u
			WHERE u.parent_invite_code = ?
			UNION ALL
			SELECT c.id, c.invite_code
			FROM user_info c
			JOIN team_tree t ON c.parent_invite_code = t.invite_code
		)
		SELECT
			COALESCE((
				SELECT SUM(o.amount)
				FROM group_match_order o
				JOIN group_match_session s ON s.id = o.session_id
				JOIN team_tree t ON t.id = o.user_id
				WHERE s.session_date = ?::date
			), 0) AS group_performance,
			COALESCE((
				SELECT d.reward_amount
				FROM group_purchase_leadership_weight_reward_detail d
				WHERE d.user_id = ?
				ORDER BY d.biz_date DESC, d.created_at DESC, d.id DESC
				LIMIT 1
			), 0) AS leader_new,
			(
				COALESCE((
					SELECT SUM(o.reward_amount)
					FROM group_match_order o
					JOIN group_match_session s ON s.id = o.session_id
					WHERE o.user_id = ?
					  AND o.is_winner = true
					  AND s.session_date = ?::date
				), 0)
				+
				COALESCE((
					SELECT SUM(d.granted_amount)
					FROM group_match_team_reward_distribution d
					JOIN group_match_session s ON s.id = d.session_id
					WHERE d.user_id = ?
					  AND s.session_date = ?::date
				), 0)
				+
				COALESCE((
					SELECT SUM(d.granted_amount)
					FROM group_purchase_leadership_reward_detail d
					WHERE d.user_id = ?
					  AND d.biz_date = ?::date
				), 0)
				+
				COALESCE((
					SELECT SUM(d.granted_amount)
					FROM group_purchase_leadership_weight_reward_detail d
					WHERE d.user_id = ?
					  AND d.biz_date = ?::date
				), 0)
				+
				COALESCE((
					SELECT SUM(l.amount)
					FROM cobo_balance_change_log l
					WHERE l.user_id = ?
					  AND l.change_type IN ('staking_v2_referral_direct', 'staking_v2_referral_indirect')
					  AND l.created_at >= ?
					  AND l.created_at < ?
				), 0)
				+
				COALESCE((
					SELECT SUM(p.direct_reward_amount)
					FROM cobo_node_purchase p
					WHERE p.direct_reward_user_id = ?
					  AND p.direct_reward_amount > 0
					  AND p.is_gift = 0
					  AND p.created_at >= ?
					  AND p.created_at < ?
				), 0)
			) AS all_reward
	`, selfUser.InviteCode, bizYesterday.Format("2006-01-02"), uid, uid, bizYesterday.Format("2006-01-02"), uid, bizYesterday.Format("2006-01-02"), uid, bizYesterday.Format("2006-01-02"), uid, bizYesterday.Format("2006-01-02"), uid, windowStart, windowEnd, uid, windowStart, windowEnd).Scan(&row)
	if queryErr != nil {
		return nil, gerror.Wrap(queryErr, "query last business day reward summary failed")
	}

	res.GroupPerformance = utils.FormatDecimal(row.GroupPerformance)
	res.LeaderNew = utils.FormatDecimal(row.LeaderNew)
	res.AllReward = utils.FormatDecimal(row.AllReward)
	return res, nil
}

func (s *teamService) getDirectTeamTriplePerformanceBatch(ctx context.Context, userIDs []int64) (map[int64]*directTriplePerfRow, error) {
	if len(userIDs) == 0 {
		return make(map[int64]*directTriplePerfRow), nil
	}

	placeholders := make([]string, len(userIDs))
	args := make([]interface{}, 0, len(userIDs))
	for i, id := range userIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args = append(args, id)
	}

	rows, err := g.DB().GetAll(ctx, fmt.Sprintf(`
		SELECT user_id,
		       COALESCE(team_performance, 0) AS team_triple_performance,
		       COALESCE(personal_performance, 0) AS personal_triple_performance
		FROM staking_v2_performance
		WHERE user_id IN (%s)
	`, strings.Join(placeholders, ", ")), args...)
	if err != nil {
		return nil, err
	}

	result := make(map[int64]*directTriplePerfRow, len(rows))
	for _, row := range rows {
		uid := row["user_id"].Int64()
		teamTriplePerf, _ := decimal.NewFromString(row["team_triple_performance"].String())
		personalTriplePerf, _ := decimal.NewFromString(row["personal_triple_performance"].String())
		result[uid] = &directTriplePerfRow{TeamTriplePerformance: teamTriplePerf.Add(personalTriplePerf)}
	}

	return result, nil
}

func (s *teamService) getDirectParticipationPerformanceBatch(ctx context.Context, userIDs []int64) (map[int64]*todayGroupPerfRow, error) {
	if len(userIDs) == 0 {
		return make(map[int64]*todayGroupPerfRow), nil
	}

	placeholders := make([]string, len(userIDs))
	args := make([]interface{}, 0, len(userIDs))
	for i, id := range userIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args = append(args, id)
	}

	rows, err := g.DB().GetAll(ctx, fmt.Sprintf(`
		SELECT user_id,
		       COALESCE(personal_group_amount, 0) AS personal_group_amount,
		       COALESCE(small_team_group_amount, 0) AS small_team_group_amount,
		       COALESCE(team_group_amount, 0) AS team_group_amount
		FROM (
			SELECT user_id,
			       COALESCE(SUM(personal_group_amount), 0) AS personal_group_amount,
			       COALESCE(SUM(small_team_group_amount), 0) AS small_team_group_amount,
			       COALESCE(SUM(team_group_amount), 0) AS team_group_amount
			FROM group_performance
			WHERE user_id IN (%s)
			GROUP BY user_id
		) gp
	`, strings.Join(placeholders, ", ")), args...)
	if err != nil {
		return nil, err
	}

	result := make(map[int64]*todayGroupPerfRow, len(rows))
	for _, row := range rows {
		uid := row["user_id"].Int64()
		personal, _ := decimal.NewFromString(row["personal_group_amount"].String())
		small, _ := decimal.NewFromString(row["small_team_group_amount"].String())
		team, _ := decimal.NewFromString(row["team_group_amount"].String())
		result[uid] = &todayGroupPerfRow{
			PersonalGroupAmount:  personal,
			SmallTeamGroupAmount: small,
			TeamGroupAmount:      team,
		}
	}
	return result, nil
}
