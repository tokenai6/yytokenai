package staking_v2

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"XWFrame/internal/frame/consts"
	"XWFrame/internal/repository"
	"XWFrame/internal/service/shared"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

const (
	stakingV2LeaderRewardChangeType = consts.ChangeTypeStakingV2LeaderReward
	stakingV2LeaderVertexUserID     = int64(1)
)

type leaderRewardLevelRule struct {
	Key         string
	Name        string
	DailyShares int64
	RewardRate  decimal.Decimal
}

type leaderRewardLevelHit struct {
	UserID    int64
	TeamStake decimal.Decimal
	Rule      leaderRewardLevelRule
}

type leaderRewardStakeRow struct {
	UserID    int64           `json:"user_id"`
	TeamStake decimal.Decimal `json:"team_stake"`
}

type leaderRewardEdgeRow struct {
	ParentID int64 `json:"parent_id"`
	ChildID  int64 `json:"child_id"`
}

type leaderRewardWalletRow struct {
	ID            int64  `json:"id"`
	WalletAddress string `json:"wallet_address"`
}

type leaderRewardDistributionItem struct {
	UserID       int64
	Wallet       string
	LevelKey     string
	LevelName    string
	TeamStake    decimal.Decimal
	LevelRate    decimal.Decimal
	ChildMaxRate decimal.Decimal
	DiffRate     decimal.Decimal
	RewardAmount decimal.Decimal
	IsVertexSink bool
}

func (s *stakingV2Service) DistributeLeaderRewards(ctx context.Context, bizDate time.Time) (int, error) {
	bizDate = dateOnlyInChina(bizDate)
	lockKey := fmt.Sprintf("staking_v2:leader_reward:dist:%s", bizDate.Format("2006-01-02"))
	token := fmt.Sprintf("%d", time.Now().UnixNano())
	locked, err := acquireSimpleRedisLock(ctx, lockKey, token, 180)
	if err != nil {
		return 0, err
	}
	if !locked {
		return 0, nil
	}
	defer releaseSimpleRedisLock(ctx, lockKey, token)

	rewardRate := s.mustReadDecimalConfig(ctx, "staking_v2_leader_reward_rate", "0.03")
	enableProfit := s.mustReadBoolConfig(ctx, "staking_v2_leader_enable_profit", false)

	dailyStake, err := s.sumDailyStakeAmountForLeader(ctx, bizDate)
	if err != nil {
		return 0, err
	}

	poolAmount := dailyStake.Mul(rewardRate)

	now := time.Now()
	distributedUsers := 0

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if poolAmount.GreaterThan(decimal.Zero) {
			count, err := s.distributeStakingLeaderRewardTx(ctx, tx, bizDate, dailyStake, poolAmount, now, enableProfit)
			if err != nil {
				return err
			}
			distributedUsers += count
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	g.Log().Infof(ctx, "[StakingV2LeaderReward] biz_date=%s 领导奖分配完成: users=%d, daily_stake=%s, pool=%s",
		bizDate.Format("2006-01-02"), distributedUsers, dailyStake.String(), poolAmount.String())
	if !enableProfit {
		g.Log().Infof(ctx, "[StakingV2LeaderReward] biz_date=%s 领导奖发放开关关闭，仅写分配明细不入账", bizDate.Format("2006-01-02"))
	}
	return distributedUsers, nil
}

func (s *stakingV2Service) distributeStakingLeaderRewardTx(ctx context.Context, tx gdb.TX, bizDate time.Time, sourceStakeAmount, poolAmount decimal.Decimal, now time.Time, enableProfit bool) (int, error) {
	already, err := tx.Model("staking_v2_leader_reward_detail").Ctx(ctx).Where("biz_date", bizDate).Count()
	if err != nil {
		return 0, gerror.Wrap(err, "query staking v2 leader reward detail failed")
	}
	if already > 0 {
		g.Log().Infof(ctx, "[StakingV2LeaderReward] biz_date=%s 已分配，跳过", bizDate.Format("2006-01-02"))
		return 0, nil
	}

	hits, rateByUser, err := s.resolveLeadershipLevelsByStakeDate(ctx, bizDate)
	if err != nil {
		return 0, err
	}

	maxChildRate, err := s.resolveMaxChildLevelRate(ctx, rateByUser)
	if err != nil {
		return 0, err
	}

	items, err := s.buildLeaderRewardDistributionItems(ctx, hits, rateByUser, maxChildRate, poolAmount)
	if err != nil {
		return 0, err
	}

	if len(items) == 0 {
		return 0, nil
	}

	batchID := "SV2-LDR-" + bizDate.Format("20060102")
	distributedUsers := 0

	for _, item := range items {
		if !item.RewardAmount.GreaterThan(decimal.Zero) {
			continue
		}

		res, err := tx.Exec(`
			INSERT INTO staking_v2_leader_reward_detail
			(biz_date, batch_id, user_id, wallet_address, level_key, level_name, team_stake_amount, level_rate, child_max_rate, diff_rate,
			 source_stake_amount, total_pool_amount, reward_amount, is_vertex_sink, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (biz_date, user_id) DO NOTHING
		`, bizDate, batchID, item.UserID, strings.ToLower(item.Wallet), item.LevelKey, item.LevelName, item.TeamStake,
			item.LevelRate, item.ChildMaxRate, item.DiffRate, sourceStakeAmount, poolAmount, item.RewardAmount,
			ternaryInt(item.IsVertexSink, 1, 0), now)
		if err != nil {
			return 0, gerror.Wrap(err, "write staking v2 leader reward detail failed")
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			continue
		}

		if enableProfit {
			granted, err := s.ApplyRewardWithCapTx(ctx, tx, item.UserID, item.RewardAmount)
			if err != nil {
				return 0, gerror.Wrap(err, "staking v2 leader reward cap validation failed")
			}
			if granted.GreaterThan(decimal.Zero) {
				if err := s.creditBalanceAndLogTx(ctx, tx, item.UserID, "USDT", granted, stakingV2LeaderRewardChangeType, batchID, 0, "staking v2 leader reward"); err != nil {
					return 0, err
				}
			}
			overflow := item.RewardAmount.Sub(granted)
			if overflow.GreaterThan(decimal.Zero) {
				if err := s.creditBalanceAndLogTx(ctx, tx, stakingV2LeaderVertexUserID, "USDT", overflow, stakingV2LeaderRewardChangeType, batchID, 0, "staking v2 leader reward overflow to vertex"); err != nil {
					return 0, err
				}
			}
			if _, err := tx.Exec(`UPDATE staking_v2_leader_reward_detail SET granted_amount = ?, overflow_amount = ? WHERE biz_date = ? AND batch_id = ? AND user_id = ?`, granted, overflow, bizDate, batchID, item.UserID); err != nil {
				return 0, gerror.Wrap(err, "update staking v2 leader reward granted/overflow failed")
			}
		}
		distributedUsers++
	}

	return distributedUsers, nil
}

func (s *stakingV2Service) resolveLeadershipLevelsByStakeDate(ctx context.Context, bizDate time.Time) (map[int64]*leaderRewardLevelHit, map[int64]decimal.Decimal, error) {
	rules, err := s.loadLeaderRewardLevelRules(ctx)
	if err != nil {
		return nil, nil, err
	}
	if len(rules) == 0 {
		return map[int64]*leaderRewardLevelHit{}, map[int64]decimal.Decimal{}, nil
	}

	dayStart := dateOnlyInChina(bizDate)
	dayEnd := dayStart.AddDate(0, 0, 1)
	rows := make([]*leaderRewardStakeRow, 0)
	err = g.DB().Ctx(ctx).Raw(`
		WITH RECURSIVE downline AS (
			SELECT u.id AS ancestor_id, c.id AS descendant_id
			FROM user_info u
			JOIN user_info c ON c.parent_invite_code = u.invite_code
			WHERE u.status = 1 AND c.status = 1
			UNION ALL
			SELECT d.ancestor_id, c.id
			FROM downline d
			JOIN user_info p ON p.id = d.descendant_id
			JOIN user_info c ON c.parent_invite_code = p.invite_code
			WHERE c.status = 1
		), daily_stake AS (
			SELECT o.user_id, COALESCE(SUM(o.amount), 0) AS stake_amount
			FROM staking_v2_order o
			WHERE o.created_at >= ? AND o.created_at < ? AND o.status = 1 AND o.is_gift = 0
			GROUP BY o.user_id
		)
		SELECT d.ancestor_id AS user_id, COALESCE(SUM(ds.stake_amount), 0) AS team_stake
		FROM downline d
		JOIN daily_stake ds ON ds.user_id = d.descendant_id
		GROUP BY d.ancestor_id
	`, dayStart, dayEnd).Scan(&rows)
	if err != nil {
		return nil, nil, gerror.Wrap(err, "query user team daily stake amount failed")
	}

	hits := make(map[int64]*leaderRewardLevelHit)
	rateByUser := make(map[int64]decimal.Decimal)
	for _, row := range rows {
		if row == nil || row.UserID <= 0 || !row.TeamStake.GreaterThan(decimal.Zero) {
			continue
		}
		matched := leaderRewardLevelRule{}
		for _, rule := range rules {
			if row.TeamStake.GreaterThanOrEqual(decimal.NewFromInt(rule.DailyShares)) {
				matched = rule
			}
		}
		if matched.Key == "" || !matched.RewardRate.GreaterThan(decimal.Zero) {
			continue
		}
		hits[row.UserID] = &leaderRewardLevelHit{UserID: row.UserID, TeamStake: row.TeamStake, Rule: matched}
		rateByUser[row.UserID] = matched.RewardRate
	}
	return hits, rateByUser, nil
}

func (s *stakingV2Service) resolveMaxChildLevelRate(ctx context.Context, rateByUser map[int64]decimal.Decimal) (map[int64]decimal.Decimal, error) {
	if len(rateByUser) == 0 {
		return map[int64]decimal.Decimal{}, nil
	}
	parentIDs := make([]int64, 0, len(rateByUser))
	for userID := range rateByUser {
		parentIDs = append(parentIDs, userID)
	}
	edges := make([]*leaderRewardEdgeRow, 0)
	err := g.DB().Model("user_info p").Ctx(ctx).
		LeftJoin("user_info c", "c.parent_invite_code = p.invite_code AND c.status = 1").
		Fields("p.id AS parent_id", "c.id AS child_id").
		Where("p.status", 1).
		WhereIn("p.id", parentIDs).
		Scan(&edges)
	if err != nil {
		return nil, gerror.Wrap(err, "query user referral relationship failed")
	}

	maxRateByUser := make(map[int64]decimal.Decimal)
	for _, edge := range edges {
		if edge == nil {
			continue
		}
		childRate, ok := rateByUser[edge.ChildID]
		if !ok {
			continue
		}
		cur := maxRateByUser[edge.ParentID]
		if childRate.GreaterThan(cur) {
			maxRateByUser[edge.ParentID] = childRate
		}
	}
	return maxRateByUser, nil
}

func (s *stakingV2Service) buildLeaderRewardDistributionItems(ctx context.Context, hits map[int64]*leaderRewardLevelHit, rateByUser map[int64]decimal.Decimal, maxChildRate map[int64]decimal.Decimal, totalPool decimal.Decimal) ([]*leaderRewardDistributionItem, error) {
	items := make([]*leaderRewardDistributionItem, 0)
	if len(hits) == 0 || !totalPool.GreaterThan(decimal.Zero) {
		return s.buildLeaderRewardVertexSinkItem(ctx, totalPool)
	}

	userIDs := make([]int64, 0, len(hits))
	totalDiff := decimal.Zero
	for userID, hit := range hits {
		childRate := maxChildRate[userID]
		diffRate := hit.Rule.RewardRate.Sub(childRate)
		if diffRate.LessThan(decimal.Zero) {
			diffRate = decimal.Zero
		}
		if !diffRate.GreaterThan(decimal.Zero) {
			continue
		}
		userIDs = append(userIDs, userID)
		totalDiff = totalDiff.Add(diffRate)
		items = append(items, &leaderRewardDistributionItem{
			UserID:       userID,
			LevelKey:     hit.Rule.Key,
			LevelName:    hit.Rule.Name,
			TeamStake:    hit.TeamStake,
			LevelRate:    hit.Rule.RewardRate,
			ChildMaxRate: childRate,
			DiffRate:     diffRate,
		})
	}

	if len(items) == 0 || !totalDiff.GreaterThan(decimal.Zero) {
		return s.buildLeaderRewardVertexSinkItem(ctx, totalPool)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].UserID < items[j].UserID
	})

	walletMap, err := s.getLeaderRewardWalletMap(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	allocated := decimal.Zero
	for i := range items {
		if i == len(items)-1 {
			items[i].RewardAmount = totalPool.Sub(allocated)
		} else {
			items[i].RewardAmount = totalPool.Mul(items[i].DiffRate).Div(totalDiff).RoundDown(18)
			allocated = allocated.Add(items[i].RewardAmount)
		}
		items[i].Wallet = walletMap[items[i].UserID]
	}

	return items, nil
}

func (s *stakingV2Service) buildLeaderRewardVertexSinkItem(ctx context.Context, totalPool decimal.Decimal) ([]*leaderRewardDistributionItem, error) {
	if !totalPool.GreaterThan(decimal.Zero) {
		return []*leaderRewardDistributionItem{}, nil
	}
	walletMap, err := s.getLeaderRewardWalletMap(ctx, []int64{stakingV2LeaderVertexUserID})
	if err != nil {
		return nil, err
	}
	return []*leaderRewardDistributionItem{{
		UserID:       stakingV2LeaderVertexUserID,
		Wallet:       walletMap[stakingV2LeaderVertexUserID],
		LevelKey:     "vertex",
		LevelName:    "首码",
		TeamStake:    decimal.Zero,
		LevelRate:    decimal.Zero,
		ChildMaxRate: decimal.Zero,
		DiffRate:     decimal.NewFromInt(1),
		RewardAmount: totalPool,
		IsVertexSink: true,
	}}, nil
}

func (s *stakingV2Service) getLeaderRewardWalletMap(ctx context.Context, userIDs []int64) (map[int64]string, error) {
	if len(userIDs) == 0 {
		return map[int64]string{}, nil
	}
	rows := make([]*leaderRewardWalletRow, 0)
	err := g.DB().Model("user_info").Ctx(ctx).
		Fields("id,wallet_address").
		WhereIn("id", userIDs).
		Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "query user wallet address failed")
	}
	walletMap := make(map[int64]string, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		walletMap[row.ID] = strings.ToLower(strings.TrimSpace(row.WalletAddress))
	}
	return walletMap, nil
}

func (s *stakingV2Service) sumDailyStakeAmountForLeader(ctx context.Context, bizDate time.Time) (decimal.Decimal, error) {
	windowStart, windowEnd, err := shared.ResolveSessionDayWindow(ctx, bizDate)
	if err != nil {
		return decimal.Zero, err
	}
	if windowStart.IsZero() || windowEnd.IsZero() || !windowEnd.After(windowStart) {
		return decimal.Zero, nil
	}
	row, err := g.DB().GetOne(ctx, `
		SELECT COALESCE(SUM(amount), 0) AS amount
		FROM staking_v2_order
		WHERE created_at >= ?
		  AND created_at < ?
		  AND is_gift = 0
	`, windowStart, windowEnd)
	if err != nil {
		return decimal.Zero, gerror.Wrap(err, "count daily new stake for leader reward failed")
	}
	amount, _ := decimal.NewFromString(row["amount"].String())
	return amount, nil
}
func (s *stakingV2Service) loadLeaderRewardLevelRules(ctx context.Context) ([]leaderRewardLevelRule, error) {
	defaultRules := `{"version":1,"levels":[{"key":"white","name":"白钻","daily_shares":1000,"reward_rate":0.04},{"key":"yellow","name":"黄钻","daily_shares":5000,"reward_rate":0.06},{"key":"red","name":"红钻","daily_shares":10000,"reward_rate":0.08},{"key":"blue","name":"蓝钻","daily_shares":50000,"reward_rate":0.10},{"key":"black","name":"黑钻","daily_shares":100000,"reward_rate":0.12}]}`
	rulesJSON := s.mustReadStringConfig(ctx, "group_purchase_leadership_level_rules", defaultRules)

	var raw struct {
		Levels []struct {
			Key         string          `json:"key"`
			Name        string          `json:"name"`
			DailyShares int64           `json:"daily_shares"`
			RewardRate  decimal.Decimal `json:"reward_rate"`
		} `json:"levels"`
	}
	if err := gjson.DecodeTo(rulesJSON, &raw); err != nil {
		return nil, gerror.Wrap(err, "parse leader reward level rules failed")
	}
	rules := make([]leaderRewardLevelRule, 0, len(raw.Levels))
	for _, level := range raw.Levels {
		if level.DailyShares <= 0 || strings.TrimSpace(level.Key) == "" || !level.RewardRate.GreaterThan(decimal.Zero) {
			continue
		}
		rules = append(rules, leaderRewardLevelRule{
			Key:         strings.TrimSpace(level.Key),
			Name:        strings.TrimSpace(level.Name),
			DailyShares: level.DailyShares,
			RewardRate:  level.RewardRate,
		})
	}
	if len(rules) == 0 {
		return nil, gerror.New("leader reward level rules are empty")
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].DailyShares < rules[j].DailyShares
	})
	return rules, nil
}

func (s *stakingV2Service) creditBalanceAndLogTx(
	ctx context.Context,
	tx gdb.TX,
	userID int64,
	symbol string,
	amount decimal.Decimal,
	changeType string,
	relatedOrderNo string,
	relatedID int64,
	remark string,
) error {
	if !amount.GreaterThan(decimal.Zero) {
		return nil
	}

	before, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, userID, symbol)
	if err != nil {
		return gerror.Wrap(err, "query balance before distribution failed")
	}
	beforeAmount := decimal.Zero
	if before != nil {
		beforeAmount = before.AvailableAmount
	}

	if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, userID, symbol, amount, decimal.Zero); err != nil {
		return gerror.Wrap(err, "update balance failed")
	}

	after, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, userID, symbol)
	if err != nil {
		return gerror.Wrap(err, "query balance after distribution failed")
	}
	afterAmount := decimal.Zero
	if after != nil {
		afterAmount = after.AvailableAmount
	}

	_, err = tx.Exec(`
		INSERT INTO cobo_balance_change_log
		(user_id, symbol, change_type, amount, before_balance, after_balance, related_order_no, related_id, remark, operator_type, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, userID, symbol, changeType, amount, beforeAmount, afterAmount, relatedOrderNo, relatedID, remark, consts.OperatorTypeSystem, time.Now())
	if err != nil {
		return gerror.Wrap(err, "write cobo balance change log failed")
	}
	return nil
}

func (s *stakingV2Service) mustReadStringConfig(ctx context.Context, key, fallback string) string {
	cfg, err := repository.NewConfigRepository().GetByKeyName(ctx, key)
	if err == nil && cfg != nil && strings.TrimSpace(cfg.KeyValue) != "" {
		return strings.TrimSpace(cfg.KeyValue)
	}
	return fallback
}

func (s *stakingV2Service) mustReadBoolConfig(ctx context.Context, key string, fallback bool) bool {
	fallbackStr := "false"
	if fallback {
		fallbackStr = "true"
	}
	val := strings.ToLower(strings.TrimSpace(s.mustReadStringConfig(ctx, key, fallbackStr)))
	switch val {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func dateOnlyInChina(t time.Time) time.Time {
	loc := time.FixedZone("CST", 8*3600)
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

func ternaryInt(cond bool, yes, no int) int {
	if cond {
		return yes
	}
	return no
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
		g.Log().Warningf(ctx, "[StakingV2LeaderReward] release distributed lock failed: key=%s err=%v", key, err)
	}
}
