package group_match

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"XWFrame/internal/frame/consts"
	"XWFrame/internal/service/group_match/model"
	"XWFrame/internal/service/shared"
	"XWFrame/internal/service/staking_v2"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

const (
	groupMatchChangeTypeLeadershipReward       = consts.ChangeTypeGroupMatchLeadershipReward
	groupMatchChangeTypeLeadershipWeightReward = consts.ChangeTypeGroupMatchLeadershipWeightReward
)

type leadershipLevelRule struct {
	Key         string
	Name        string
	DailyShares int64
	RewardRate  decimal.Decimal
}

type leadershipLevelHit struct {
	UserID     int64
	TeamShares int64
	Rule       leadershipLevelRule
}

type leadershipTeamShareRow struct {
	UserID     int64 `json:"user_id"`
	TeamShares int64 `json:"team_shares"`
}

type leadershipEdgeRow struct {
	ParentID int64 `json:"parent_id"`
	ChildID  int64 `json:"child_id"`
}

type leadershipWalletRow struct {
	ID            int64  `json:"id"`
	WalletAddress string `json:"wallet_address"`
}

type leadershipUserRelationRow struct {
	ID               int64  `json:"id"`
	InviteCode       string `json:"invite_code"`
	ParentInviteCode string `json:"parent_invite_code"`
}

type leadershipLoserOrderRow struct {
	ID     int64           `json:"id"`
	UserID int64           `json:"user_id"`
	Amount decimal.Decimal `json:"amount"`
}

type leadershipDistributionItem struct {
	UserID       int64
	Wallet       string
	LevelKey     string
	LevelName    string
	TeamShares   int64
	LevelRate    decimal.Decimal
	ChildMaxRate decimal.Decimal
	DiffRate     decimal.Decimal
	RewardAmount decimal.Decimal
	IsVertexSink bool
}

func leadershipTicketWeightBaseByLevel(levelKey string) int64 {
	switch strings.ToLower(strings.TrimSpace(levelKey)) {
	case "white":
		return 30
	case "yellow":
		return 25
	case "red":
		return 20
	case "blue":
		return 15
	case "black":
		return 10
	default:
		return 0
	}
}

func leadershipTicketWeightByLevel(levelKey string) int64 {
	levelKey = strings.ToLower(strings.TrimSpace(levelKey))
	levelOrder := []string{"white", "yellow", "red", "blue", "black"}
	idx := -1
	for i, key := range levelOrder {
		if key == levelKey {
			idx = i
			break
		}
	}
	if idx < 0 {
		return 0
	}

	var total int64
	for i := 0; i <= idx; i++ {
		total += leadershipTicketWeightBaseByLevel(levelOrder[i])
	}
	return total
}

func (s *groupMatchService) GetLeadershipLevelsConfig(ctx context.Context) (*model.LeadershipLevelsConfigRes, error) {
	rules, err := s.loadLeadershipLevelRules(ctx)
	if err != nil {
		return nil, err
	}

	levels := make([]*model.LeadershipLevelConfigItem, 0, len(rules))
	for _, rule := range rules {
		levels = append(levels, &model.LeadershipLevelConfigItem{
			Key:          rule.Key,
			Name:         rule.Name,
			DailyShares:  rule.DailyShares,
			RewardRate:   rule.RewardRate.String(),
			TicketWeight: leadershipTicketWeightByLevel(rule.Key),
		})
	}

	return &model.LeadershipLevelsConfigRes{
		Levels:             levels,
		LeadershipPoolRate: s.mustReadDecimalConfig(ctx, "group_purchase_leadership_pool_rate", "0.12").String(),
		TicketWeightRate:   s.mustReadDecimalConfig(ctx, "group_purchase_ticket_weight_rate", "0.03").String(),
	}, nil
}

func (s *groupMatchService) DistributeLeadershipRewards(ctx context.Context, bizDate time.Time) (int, error) {
	bizDate = dateOnlyInChina(bizDate)
	lockKey := fmt.Sprintf("group_match:leadership:dist:%s", bizDate.Format("2006-01-02"))
	token := fmt.Sprintf("%d", time.Now().UnixNano())
	locked, err := acquireSimpleRedisLock(ctx, lockKey, token, 180)
	if err != nil {
		return 0, err
	}
	if !locked {
		return 0, nil
	}
	defer releaseSimpleRedisLock(ctx, lockKey, token)

	leadershipRate := s.mustReadDecimalConfig(ctx, "group_purchase_leadership_pool_rate", "0.12")
	ticketRate := s.mustReadDecimalConfig(ctx, "group_purchase_ticket_weight_rate", "0.03")
	enableProfit := s.mustReadBoolConfig(ctx, "group_purchase_leadership_enable_profit", false)

	loserPrincipal, err := s.sumLoserPrincipalByBizDate(ctx, bizDate)
	if err != nil {
		return 0, err
	}
	dailyStake, err := s.sumDailyStakeAmount(ctx, bizDate)
	if err != nil {
		return 0, err
	}

	loserPool := loserPrincipal.Mul(leadershipRate)
	ticketPool := dailyStake.Mul(ticketRate)

	now := time.Now()
	distributedUsers := 0

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 写入奖池来源（两个池子都记录）
		if loserPool.GreaterThan(decimal.Zero) {
			if err := s.upsertLeadershipPoolSourceTx(ctx, tx, bizDate, "group_match_loser", loserPrincipal, leadershipRate, loserPool, now); err != nil {
				return err
			}
		}
		if ticketPool.GreaterThan(decimal.Zero) {
			if err := s.upsertLeadershipPoolSourceTx(ctx, tx, bizDate, "daily_staking", dailyStake, ticketRate, ticketPool, now); err != nil {
				return err
			}
		}

		// 池子一：极差分配（loser 12%）
		if loserPool.GreaterThan(decimal.Zero) {
			count, err := s.distributeLoserLeadershipRewardTx(ctx, tx, bizDate, loserPool, now, enableProfit)
			if err != nil {
				return err
			}
			distributedUsers += count
		}

		// 池子二：加权分配（新增业绩 3%）
		if ticketPool.GreaterThan(decimal.Zero) {
			count, err := s.distributeTicketWeightRewardTx(ctx, tx, bizDate, ticketPool, now, enableProfit)
			if err != nil {
				return err
			}
			distributedUsers += count
		}

		// 更新奖池状态为已分配
		if distributedUsers > 0 {
			if _, err := tx.Model("group_purchase_leadership_pool").Ctx(ctx).
				Where("biz_date", bizDate).
				Data(g.Map{"status": 1, "distributed_at": now, "updated_at": now}).Update(); err != nil {
				return gerror.Wrap(err, "update leadership reward pool status failed")
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	g.Log().Infof(ctx, "[LeadershipReward] biz_date=%s 领导奖分配完成: users=%d, loser_pool=%s, ticket_pool=%s",
		bizDate.Format("2006-01-02"), distributedUsers, loserPool.String(), ticketPool.String())
	if !enableProfit {
		g.Log().Infof(ctx, "[LeadershipReward] biz_date=%s 领导奖发放开关关闭，仅写分配明细不入账", bizDate.Format("2006-01-02"))
	}
	return distributedUsers, nil
}

func (s *groupMatchService) distributeLoserLeadershipRewardTx(ctx context.Context, tx gdb.TX, bizDate time.Time, loserPool decimal.Decimal, now time.Time, enableProfit bool) (int, error) {
	already, err := tx.Model("group_purchase_leadership_reward_detail").Ctx(ctx).Where("biz_date", bizDate).Count()
	if err != nil {
		return 0, gerror.Wrap(err, "query range leadership reward detail failed")
	}
	if already > 0 {
		g.Log().Infof(ctx, "[LeadershipReward] biz_date=%s 极差领导奖已分配，跳过", bizDate.Format("2006-01-02"))
		return 0, nil
	}

	hits, rateByUser, err := s.resolveLeadershipLevelsByDate(ctx, bizDate)
	if err != nil {
		return 0, err
	}
	poolRate := s.mustReadDecimalConfig(ctx, "group_purchase_leadership_pool_rate", "0.12")
	if !poolRate.GreaterThan(decimal.Zero) {
		return 0, gerror.New("invalid leadership pool rate")
	}

	loserOrders, err := s.listLoserOrdersByBizDate(ctx, bizDate)
	if err != nil {
		return 0, err
	}
	if len(loserOrders) == 0 {
		return 0, nil
	}

	parentByUser, err := s.loadParentUserMap(ctx)
	if err != nil {
		return 0, err
	}

	maxChildRate, err := s.resolveMaxChildLevelRate(ctx, rateByUser)
	if err != nil {
		return 0, err
	}

	totalLoserPrincipal := decimal.Zero
	for _, order := range loserOrders {
		totalLoserPrincipal = totalLoserPrincipal.Add(order.Amount)
	}
	if !totalLoserPrincipal.GreaterThan(decimal.Zero) {
		return 0, nil
	}

	rewardByUser := make(map[int64]decimal.Decimal)
	allocatedPool := decimal.Zero
	for i, order := range loserOrders {
		if order == nil || order.UserID <= 0 || !order.Amount.GreaterThan(decimal.Zero) {
			continue
		}

		orderPool := decimal.Zero
		if i == len(loserOrders)-1 {
			orderPool = loserPool.Sub(allocatedPool)
		} else {
			orderPool = loserPool.Mul(order.Amount).Div(totalLoserPrincipal).RoundDown(18)
			allocatedPool = allocatedPool.Add(orderPool)
		}
		if !orderPool.GreaterThan(decimal.Zero) {
			continue
		}

		type chainPiece struct {
			leaderID int64
			diffRate decimal.Decimal
		}
		pieces := make([]*chainPiece, 0)
		paidRate := decimal.Zero

		for parentID := parentByUser[order.UserID]; parentID > 0; parentID = parentByUser[parentID] {
			leaderRate, ok := rateByUser[parentID]
			if !ok || !leaderRate.GreaterThan(decimal.Zero) {
				continue
			}
			if leaderRate.GreaterThan(poolRate) {
				leaderRate = poolRate
			}
			if !leaderRate.GreaterThan(paidRate) {
				continue
			}
			diffRate := leaderRate.Sub(paidRate)
			if !diffRate.GreaterThan(decimal.Zero) {
				continue
			}
			pieces = append(pieces, &chainPiece{leaderID: parentID, diffRate: diffRate})
			paidRate = leaderRate
			if paidRate.Equal(poolRate) {
				break
			}
		}

		orderAllocated := decimal.Zero
		for _, piece := range pieces {
			reward := orderPool.Mul(piece.diffRate).Div(poolRate).RoundDown(18)
			if !reward.GreaterThan(decimal.Zero) {
				continue
			}
			rewardByUser[piece.leaderID] = rewardByUser[piece.leaderID].Add(reward)
			orderAllocated = orderAllocated.Add(reward)
		}

		leftover := orderPool.Sub(orderAllocated)
		if leftover.GreaterThan(decimal.Zero) {
			rewardByUser[groupMatchVertexUserID] = rewardByUser[groupMatchVertexUserID].Add(leftover)
		}
	}

	if len(rewardByUser) == 0 {
		return 0, nil
	}

	leaderIDs := make([]int64, 0, len(rewardByUser))
	for userID := range rewardByUser {
		leaderIDs = append(leaderIDs, userID)
	}
	sort.Slice(leaderIDs, func(i, j int) bool {
		return leaderIDs[i] < leaderIDs[j]
	})

	walletMap, err := s.getWalletMap(ctx, leaderIDs)
	if err != nil {
		return 0, err
	}
	teamTodayOrderCountByAddr := make(map[string]int64)
	for userID, hit := range hits {
		addr := strings.ToLower(strings.TrimSpace(walletMap[userID]))
		if addr == "" {
			continue
		}
		teamTodayOrderCountByAddr[addr] = hit.TeamShares
	}

	batchID := "GLR-DIFF-" + bizDate.Format("20060102")
	detailItems := make([]*diffDetailItem, 0, len(leaderIDs))
	for _, userID := range leaderIDs {
		rewardAmount := rewardByUser[userID]
		if !rewardAmount.GreaterThan(decimal.Zero) {
			continue
		}

		wallet := strings.ToLower(strings.TrimSpace(walletMap[userID]))
		if wallet == "" {
			continue
		}

		teamShares := teamTodayOrderCountByAddr[wallet]
		levelKey := "vertex"
		levelName := "首码"
		levelRate := decimal.Zero
		childMaxRate := decimal.Zero
		diffRate := decimal.Zero
		isVertexSink := 0

		if userID == groupMatchVertexUserID {
			isVertexSink = 1
		} else if hit, ok := hits[userID]; ok && hit != nil {
			levelKey = hit.Rule.Key
			levelName = hit.Rule.Name
			levelRate = hit.Rule.RewardRate
			childMaxRate = maxChildRate[userID]
			diffRate = levelRate.Sub(childMaxRate)
			if diffRate.LessThan(decimal.Zero) {
				diffRate = decimal.Zero
			}
		}

		detailItems = append(detailItems, &diffDetailItem{
			userID: userID, wallet: wallet, levelKey: levelKey, levelName: levelName,
			teamShares: teamShares, levelRate: levelRate, childMaxRate: childMaxRate,
			diffRate: diffRate, rewardAmount: rewardAmount, isVertexSink: isVertexSink,
		})
	}

	if len(detailItems) > 0 {
		insertedUserIDs, err := s.batchInsertDiffLeadershipRewardDetailTx(ctx, tx, bizDate, batchID, now, totalLoserPrincipal, loserPool, detailItems)
		if err != nil {
			return 0, err
		}
		detailItems = filterDiffDetailItemsByUserIDs(detailItems, insertedUserIDs)
	}

	stakingSvc := staking_v2.NewStakingV2Service()
	distributedUsers := 0
	for _, item := range detailItems {
		if enableProfit {
			granted, err := stakingSvc.ApplyRewardWithCapTx(ctx, tx, item.userID, item.rewardAmount)
			if err != nil {
				return 0, gerror.Wrap(err, "leadership diff reward limit validation failed")
			}
			if granted.GreaterThan(decimal.Zero) {
				if err := s.creditBalanceAndLogTx(ctx, tx, item.userID, "USDT", granted, groupMatchChangeTypeLeadershipReward, batchID, 0, "group_match leadership diff reward"); err != nil {
					return 0, err
				}
			}
			overflow := item.rewardAmount.Sub(granted)
			if overflow.GreaterThan(decimal.Zero) {
				if err := s.creditBalanceAndLogTx(ctx, tx, groupMatchVertexUserID, "USDT", overflow, groupMatchChangeTypeLeadershipReward, batchID, 0, "group_match leadership diff reward overflow to vertex"); err != nil {
					return 0, err
				}
			}
			if _, err := tx.Exec(`UPDATE group_purchase_leadership_reward_detail SET granted_amount = ?, overflow_amount = ? WHERE biz_date = ? AND batch_id = ? AND user_id = ?`, granted, overflow, bizDate, batchID, item.userID); err != nil {
				return 0, gerror.Wrap(err, "update leadership diff reward granted/overflow failed")
			}
		}
		distributedUsers++
	}

	return distributedUsers, nil
}

func (s *groupMatchService) loadParentUserMap(ctx context.Context) (map[int64]int64, error) {
	rows := make([]*leadershipUserRelationRow, 0)
	err := g.DB().Model("user_info").Ctx(ctx).
		Fields("id, invite_code, parent_invite_code").
		Where("status", 1).
		Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "query user relation failed")
	}

	inviteToID := make(map[string]int64, len(rows))
	for _, row := range rows {
		if row == nil || row.ID <= 0 {
			continue
		}
		inviteCode := strings.TrimSpace(row.InviteCode)
		if inviteCode == "" {
			continue
		}
		inviteToID[inviteCode] = row.ID
	}

	parentByUser := make(map[int64]int64, len(rows))
	for _, row := range rows {
		if row == nil || row.ID <= 0 {
			continue
		}
		parentInviteCode := strings.TrimSpace(row.ParentInviteCode)
		if parentInviteCode == "" {
			continue
		}
		parentByUser[row.ID] = inviteToID[parentInviteCode]
	}
	return parentByUser, nil
}

func (s *groupMatchService) listLoserOrdersByBizDate(ctx context.Context, bizDate time.Time) ([]*leadershipLoserOrderRow, error) {
	rows := make([]*leadershipLoserOrderRow, 0)
	err := g.DB().Ctx(ctx).Raw(`
		SELECT o.id, o.user_id, o.amount
		FROM group_match_order o
		JOIN group_match_session s ON s.id = o.session_id
		WHERE o.status = ?
		  AND s.session_date = ?::date
		ORDER BY o.id ASC
	`, consts.GroupMatchOrderStatusLoser, bizDate.Format("2006-01-02")).Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "query loser orders failed")
	}
	return rows, nil
}

func (s *groupMatchService) distributeTicketWeightRewardTx(ctx context.Context, tx gdb.TX, bizDate time.Time, ticketPool decimal.Decimal, now time.Time, enableProfit bool) (int, error) {
	already, err := tx.Model("group_purchase_leadership_weight_reward_detail").Ctx(ctx).Where("biz_date", bizDate).Count()
	if err != nil {
		return 0, gerror.Wrap(err, "query weighted leadership reward detail failed")
	}
	if already > 0 {
		g.Log().Infof(ctx, "[LeadershipReward] biz_date=%s 加权领导奖已分配，跳过", bizDate.Format("2006-01-02"))
		return 0, nil
	}

	// 查询当日所有有团队拼购份数的用户
	hits, _, err := s.resolveLeadershipLevelsByDate(ctx, bizDate)
	if err != nil {
		return 0, err
	}

	if !ticketPool.GreaterThan(decimal.Zero) {
		return 0, nil
	}
	if len(hits) == 0 {
		return s.sinkTicketWeightRewardToVertexTx(ctx, tx, bizDate, ticketPool, now, enableProfit)
	}

	// 收集用户ID和加权总份数
	userIDs := make([]int64, 0, len(hits))
	totalShares := int64(0)
	totalWeightedShares := int64(0)
	type weightItem struct {
		userID        int64
		hit           *leadershipLevelHit
		weightedShare int64
	}
	weightItems := make([]*weightItem, 0)
	for userID, hit := range hits {
		totalShares += hit.TeamShares
		lw := leadershipTicketWeightByLevel(hit.Rule.Key)
		if lw <= 0 {
			continue
		}
		userIDs = append(userIDs, userID)
		weightedShare := hit.TeamShares * lw
		totalWeightedShares += weightedShare
		weightItems = append(weightItems, &weightItem{userID: userID, hit: hit, weightedShare: weightedShare})
	}

	if totalWeightedShares <= 0 {
		// 无有效加权份数，沉淀到顶号
		return s.sinkTicketWeightRewardToVertexTx(ctx, tx, bizDate, ticketPool, now, enableProfit)
	}

	walletMap, err := s.getWalletMap(ctx, userIDs)
	if err != nil {
		return 0, err
	}

	batchID := "GLR-WEIGHT-" + bizDate.Format("20060102")
	allocated := decimal.Zero

	// 按 userID 排序以保证确定性
	sort.Slice(weightItems, func(i, j int) bool {
		return weightItems[i].userID < weightItems[j].userID
	})

	detailItems := make([]*weightDetailItem, 0, len(weightItems))
	for i, item := range weightItems {
		hit := item.hit
		if hit == nil || hit.TeamShares <= 0 {
			continue
		}

		var reward decimal.Decimal
		if i == len(weightItems)-1 {
			reward = ticketPool.Sub(allocated)
		} else {
			reward = ticketPool.Mul(decimal.NewFromInt(item.weightedShare)).Div(decimal.NewFromInt(totalWeightedShares)).RoundDown(18)
			allocated = allocated.Add(reward)
		}
		if !reward.GreaterThan(decimal.Zero) {
			continue
		}

		wallet := strings.ToLower(walletMap[item.userID])
		if wallet == "" {
			continue
		}
		detailItems = append(detailItems, &weightDetailItem{
			userID: item.userID, wallet: wallet, levelName: hit.Rule.Name, teamShares: hit.TeamShares,
			totalShares: totalWeightedShares, poolAmount: ticketPool, rewardAmount: reward,
		})
	}

	if len(detailItems) > 0 {
		insertedUserIDs, err := s.batchInsertWeightLeadershipRewardDetailTx(ctx, tx, bizDate, batchID, now, detailItems)
		if err != nil {
			return 0, err
		}
		detailItems = filterWeightDetailItemsByUserIDs(detailItems, insertedUserIDs)
	}

	stakingSvc := staking_v2.NewStakingV2Service()
	distributedUsers := 0
	for _, item := range detailItems {
		if enableProfit {
			granted, err := stakingSvc.ApplyRewardWithCapTx(ctx, tx, item.userID, item.rewardAmount)
			if err != nil {
				return 0, gerror.Wrap(err, "leadership weight reward limit validation failed")
			}
			if granted.GreaterThan(decimal.Zero) {
				if err := s.creditBalanceAndLogTx(ctx, tx, item.userID, "USDT", granted, groupMatchChangeTypeLeadershipWeightReward, batchID, 0, "group_match leadership weight reward"); err != nil {
					return 0, err
				}
			}
			overflow := item.rewardAmount.Sub(granted)
			if overflow.GreaterThan(decimal.Zero) {
				if err := s.creditBalanceAndLogTx(ctx, tx, groupMatchVertexUserID, "USDT", overflow, groupMatchChangeTypeLeadershipWeightReward, batchID, 0, "group_match leadership weight reward overflow to vertex"); err != nil {
					return 0, err
				}
			}
			if _, err := tx.Exec(`UPDATE group_purchase_leadership_weight_reward_detail SET granted_amount = ?, overflow_amount = ? WHERE biz_date = ? AND batch_id = ? AND user_id = ?`, granted, overflow, bizDate, batchID, item.userID); err != nil {
				return 0, gerror.Wrap(err, "update leadership weight reward granted/overflow failed")
			}
		}
		distributedUsers++
	}

	return distributedUsers, nil
}

func (s *groupMatchService) sinkTicketWeightRewardToVertexTx(ctx context.Context, tx gdb.TX, bizDate time.Time, ticketPool decimal.Decimal, now time.Time, enableProfit bool) (int, error) {
	g.Log().Infof(ctx, "[LeadershipReward] biz_date=%s 加权领导奖无达标用户，沉淀到顶号", bizDate.Format("2006-01-02"))
	batchID := "GLR-WEIGHT-" + bizDate.Format("20060102")

	walletMap, err := s.getWalletMap(ctx, []int64{groupMatchVertexUserID})
	if err != nil {
		return 0, err
	}

	res, err := tx.Exec(`
		INSERT INTO group_purchase_leadership_weight_reward_detail
		(biz_date, batch_id, user_id, wallet_address, level_name, team_shares, total_shares, pool_amount, reward_amount, is_vertex_sink, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (biz_date, user_id) DO NOTHING
	`, bizDate, batchID, groupMatchVertexUserID, strings.ToLower(walletMap[groupMatchVertexUserID]), "", 0, 0, ticketPool, ticketPool, 1, now)
	if err != nil {
		return 0, gerror.Wrap(err, "write weighted leadership reward cap accumulation detail failed")
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return 0, nil
	}

	if enableProfit {
		if err := s.creditBalanceAndLogTx(ctx, tx, groupMatchVertexUserID, "USDT", ticketPool, groupMatchChangeTypeLeadershipWeightReward, batchID, 0, "group_match leadership weight reward sink to vertex"); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(`UPDATE group_purchase_leadership_weight_reward_detail SET granted_amount = ?, overflow_amount = 0 WHERE biz_date = ? AND batch_id = ? AND user_id = ?`, ticketPool, bizDate, batchID, groupMatchVertexUserID); err != nil {
			return 0, gerror.Wrap(err, "update sinked leadership weight reward granted failed")
		}
	}
	return 1, nil
}

func (s *groupMatchService) GetUserLeadershipLevel(ctx context.Context, userID int64, bizDate time.Time) (*model.UserLeadershipLevel, error) {
	if userID <= 0 {
		return nil, gerror.New("invalid user_id parameter")
	}
	bizDate = dateOnlyInChina(bizDate)
	hits, _, err := s.resolveLeadershipLevelsByDate(ctx, bizDate)
	if err != nil {
		return nil, err
	}
	hit, ok := hits[userID]
	if !ok || hit == nil {
		return &model.UserLeadershipLevel{
			BizDate:    bizDate.Format("2006-01-02"),
			UserID:     userID,
			LevelKey:   "",
			LevelName:  "",
			TeamShares: 0,
			RewardRate: "0",
		}, nil
	}
	return &model.UserLeadershipLevel{
		BizDate:    bizDate.Format("2006-01-02"),
		UserID:     userID,
		LevelKey:   hit.Rule.Key,
		LevelName:  hit.Rule.Name,
		TeamShares: hit.TeamShares,
		RewardRate: hit.Rule.RewardRate.String(),
	}, nil
}

func (s *groupMatchService) GetUserLeadershipRewardPreview(ctx context.Context, userID int64, bizDate time.Time) (*model.UserLeadershipRewardPreview, error) {
	if userID <= 0 {
		return nil, gerror.New("invalid user_id parameter")
	}
	bizDate = dateOnlyInChina(bizDate)

	teamShares, teamAmount, err := s.getUserTeamStatsByDate(ctx, userID, bizDate)
	if err != nil {
		return nil, err
	}

	rules, err := s.loadLeadershipLevelRules(ctx)
	if err != nil {
		return nil, err
	}
	vipLevel, err := s.getUserVipLevel(ctx, userID)
	if err != nil {
		return nil, err
	}

	var hit *leadershipLevelHit
	for _, rule := range rules {
		if teamShares >= rule.DailyShares {
			hit = &leadershipLevelHit{UserID: userID, TeamShares: teamShares, Rule: rule}
		}
	}

	rewardLevel := normalizeLeadershipLevel(vipLevel)
	calculatedLevel := 0
	if hit != nil {
		for idx, rule := range rules {
			if hit.TeamShares >= rule.DailyShares {
				calculatedLevel = idx + 1
			}
		}
	}
	calculatedLevel = normalizeLeadershipLevel(calculatedLevel)
	if calculatedLevel > rewardLevel {
		rewardLevel = calculatedLevel
	}

	levelRate := decimal.Zero
	childMaxRate := decimal.Zero
	diffRate := decimal.Zero
	levelKey := ""
	levelName := ""

	if hit != nil {
		levelKey = hit.Rule.Key
		levelName = hit.Rule.Name
		levelRate = hit.Rule.RewardRate

		rateByUser := map[int64]decimal.Decimal{userID: levelRate}
		maxChildRate, err := s.resolveMaxChildLevelRate(ctx, rateByUser)
		if err != nil {
			return nil, err
		}
		childMaxRate = maxChildRate[userID]
		diffRate = levelRate.Sub(childMaxRate)
		if diffRate.LessThan(decimal.Zero) {
			diffRate = decimal.Zero
		}
	}

	loserPrincipal, err := s.sumLoserPrincipalByBizDate(ctx, bizDate)
	if err != nil {
		return nil, err
	}
	leadershipRate := s.mustReadDecimalConfig(ctx, "group_purchase_leadership_pool_rate", "0.12")
	loserPool := loserPrincipal.Mul(leadershipRate)

	totalDiffRate := decimal.Zero
	estimatedReward := decimal.Zero

	if diffRate.GreaterThan(decimal.Zero) && loserPool.GreaterThan(decimal.Zero) {
		hits, rateByUser, err := s.resolveLeadershipLevelsByDate(ctx, bizDate)
		if err != nil {
			return nil, err
		}
		maxChildRateAll, err := s.resolveMaxChildLevelRate(ctx, rateByUser)
		if err != nil {
			return nil, err
		}
		for uid, h := range hits {
			childRate := maxChildRateAll[uid]
			d := h.Rule.RewardRate.Sub(childRate)
			if d.LessThan(decimal.Zero) {
				d = decimal.Zero
			}
			if d.GreaterThan(decimal.Zero) {
				totalDiffRate = totalDiffRate.Add(d)
			}
		}
		if totalDiffRate.GreaterThan(decimal.Zero) {
			estimatedReward = loserPool.Mul(diffRate).Div(totalDiffRate).RoundDown(18)
		}
	}

	return &model.UserLeadershipRewardPreview{
		BizDate:         bizDate.Format("2006-01-02"),
		UserID:          userID,
		RewardLevel:     rewardLevel,
		LevelKey:        levelKey,
		LevelName:       levelName,
		TeamShares:      teamShares,
		TeamAmount:      teamAmount.String(),
		LevelRate:       levelRate.String(),
		ChildMaxRate:    childMaxRate.String(),
		DiffRate:        diffRate.String(),
		LoserPoolAmount: loserPool.String(),
		TotalDiffRate:   totalDiffRate.String(),
		EstimatedReward: estimatedReward.String(),
	}, nil
}

func (s *groupMatchService) getUserVipLevel(ctx context.Context, userID int64) (int, error) {
	if userID <= 0 {
		return 0, nil
	}
	type userVipRow struct {
		VipLevel int `json:"vip_level"`
	}
	var row userVipRow
	err := g.DB().Model("user_info").Ctx(ctx).
		Fields("vip_level").
		Where("id", userID).
		Scan(&row)
	if err != nil {
		return 0, gerror.Wrap(err, "query user vip_level failed")
	}
	return row.VipLevel, nil
}

func (s *groupMatchService) getUserTeamStatsByDate(ctx context.Context, userID int64, bizDate time.Time) (int64, decimal.Decimal, error) {
	type row struct {
		TeamShares int64           `json:"team_shares"`
		TeamAmount decimal.Decimal `json:"team_amount"`
	}
	var r row
	err := g.DB().Ctx(ctx).Raw(`
		WITH RECURSIVE downline AS (
			SELECT u.id AS ancestor_id, c.id AS descendant_id
			FROM user_info u
			JOIN user_info c ON c.parent_invite_code = u.invite_code
			WHERE u.id = ? AND u.status = 1 AND c.status = 1
			UNION ALL
			SELECT d.ancestor_id, c.id
			FROM downline d
			JOIN user_info p ON p.id = d.descendant_id
			JOIN user_info c ON c.parent_invite_code = p.invite_code
			WHERE c.status = 1
		)
		SELECT COALESCE(COUNT(o.id), 0) AS team_shares, COALESCE(SUM(o.amount), 0) AS team_amount
		FROM downline d
		JOIN group_match_order o ON o.user_id = d.descendant_id
		JOIN group_match_session s ON s.id = o.session_id
		WHERE s.session_date = ?::date
	`, userID, bizDate.Format("2006-01-02")).Scan(&r)
	if err != nil {
		return 0, decimal.Zero, gerror.Wrap(err, "query user team stats failed")
	}
	return r.TeamShares, r.TeamAmount, nil
}

func (s *groupMatchService) resolveLeadershipLevelsByDate(ctx context.Context, bizDate time.Time) (map[int64]*leadershipLevelHit, map[int64]decimal.Decimal, error) {
	rules, err := s.loadLeadershipLevelRules(ctx)
	if err != nil {
		return nil, nil, err
	}
	if len(rules) == 0 {
		return map[int64]*leadershipLevelHit{}, map[int64]decimal.Decimal{}, nil
	}

	rows := make([]*leadershipTeamShareRow, 0)
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
		), daily_order AS (
			SELECT o.user_id, COUNT(*) AS shares
			FROM group_match_order o
			JOIN group_match_session s ON s.id = o.session_id
			WHERE s.session_date = ?::date
			GROUP BY o.user_id
		)
		SELECT d.ancestor_id AS user_id, COALESCE(SUM(od.shares), 0) AS team_shares
		FROM downline d
		JOIN daily_order od ON od.user_id = d.descendant_id
		GROUP BY d.ancestor_id
	`, bizDate.Format("2006-01-02")).Scan(&rows)
	if err != nil {
		return nil, nil, gerror.Wrap(err, "query user team daily group match count failed")
	}

	hits := make(map[int64]*leadershipLevelHit)
	rateByUser := make(map[int64]decimal.Decimal)
	userIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row == nil || row.UserID <= 0 {
			continue
		}
		userIDs = append(userIDs, row.UserID)
	}

	currentVipLevels := make(map[int64]int)
	if len(userIDs) > 0 {
		type vipRow struct {
			ID       int64 `json:"id"`
			VipLevel int   `json:"vip_level"`
		}

		vipRows := make([]*vipRow, 0, len(userIDs))
		vipErr := g.DB().Model("user_info").Ctx(ctx).
			Fields("id,vip_level").
			WhereIn("id", userIDs).
			Scan(&vipRows)
		if vipErr != nil {
			g.Log().Warningf(ctx, "[LeadershipReward] query user_info vip levels failed, fallback to calculated level: err=%v", vipErr)
		} else {
			for _, row := range vipRows {
				if row == nil || row.ID <= 0 {
					continue
				}
				currentVipLevels[row.ID] = normalizeLeadershipLevel(row.VipLevel)
			}
		}
	}

	for _, row := range rows {
		if row == nil || row.UserID <= 0 || row.TeamShares <= 0 {
			continue
		}
		calculatedLevel := 0
		for idx, rule := range rules {
			if row.TeamShares >= rule.DailyShares {
				calculatedLevel = idx + 1
			}
		}

		effectiveLevel := normalizeLeadershipLevel(calculatedLevel)
		if vipLevel, ok := currentVipLevels[row.UserID]; ok && vipLevel > effectiveLevel {
			effectiveLevel = vipLevel
		}

		matched := resolveLeadershipRuleByLevel(rules, effectiveLevel)
		if matched.Key == "" || !matched.RewardRate.GreaterThan(decimal.Zero) {
			continue
		}
		hits[row.UserID] = &leadershipLevelHit{UserID: row.UserID, TeamShares: row.TeamShares, Rule: matched}
		rateByUser[row.UserID] = matched.RewardRate
	}
	return hits, rateByUser, nil
}

func normalizeLeadershipLevel(level int) int {
	if level <= 0 {
		return 0
	}
	if level > 5 {
		return 5
	}
	return level
}

func resolveLeadershipRuleByLevel(rules []leadershipLevelRule, level int) leadershipLevelRule {
	level = normalizeLeadershipLevel(level)
	if level == 0 || len(rules) == 0 {
		return leadershipLevelRule{}
	}
	if level > len(rules) {
		level = len(rules)
	}
	return rules[level-1]
}

func (s *groupMatchService) resolveMaxChildLevelRate(ctx context.Context, rateByUser map[int64]decimal.Decimal) (map[int64]decimal.Decimal, error) {
	if len(rateByUser) == 0 {
		return map[int64]decimal.Decimal{}, nil
	}
	parentIDs := make([]int64, 0, len(rateByUser))
	for userID := range rateByUser {
		parentIDs = append(parentIDs, userID)
	}
	edges := make([]*leadershipEdgeRow, 0)
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

func (s *groupMatchService) buildLeadershipDistributionItems(ctx context.Context, hits map[int64]*leadershipLevelHit, rateByUser map[int64]decimal.Decimal, maxChildRate map[int64]decimal.Decimal, totalPool decimal.Decimal) ([]*leadershipDistributionItem, error) {
	items := make([]*leadershipDistributionItem, 0)
	if len(hits) == 0 || !totalPool.GreaterThan(decimal.Zero) {
		return s.buildVertexSinkItem(ctx, totalPool)
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
		items = append(items, &leadershipDistributionItem{
			UserID:       userID,
			LevelKey:     hit.Rule.Key,
			LevelName:    hit.Rule.Name,
			TeamShares:   hit.TeamShares,
			LevelRate:    hit.Rule.RewardRate,
			ChildMaxRate: childRate,
			DiffRate:     diffRate,
		})
	}

	if len(items) == 0 || !totalDiff.GreaterThan(decimal.Zero) {
		return s.buildVertexSinkItem(ctx, totalPool)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].UserID < items[j].UserID
	})

	walletMap, err := s.getWalletMap(ctx, userIDs)
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

func (s *groupMatchService) buildVertexSinkItem(ctx context.Context, totalPool decimal.Decimal) ([]*leadershipDistributionItem, error) {
	if !totalPool.GreaterThan(decimal.Zero) {
		return []*leadershipDistributionItem{}, nil
	}
	walletMap, err := s.getWalletMap(ctx, []int64{1})
	if err != nil {
		return nil, err
	}
	return []*leadershipDistributionItem{{
		UserID:       1,
		Wallet:       walletMap[1],
		LevelKey:     "vertex",
		LevelName:    "首码",
		TeamShares:   0,
		LevelRate:    decimal.Zero,
		ChildMaxRate: decimal.Zero,
		DiffRate:     decimal.NewFromInt(1),
		RewardAmount: totalPool,
		IsVertexSink: true,
	}}, nil
}

func (s *groupMatchService) getWalletMap(ctx context.Context, userIDs []int64) (map[int64]string, error) {
	if len(userIDs) == 0 {
		return map[int64]string{}, nil
	}
	rows := make([]*leadershipWalletRow, 0)
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

func (s *groupMatchService) upsertLeadershipPoolSourceTx(ctx context.Context, tx gdb.TX, bizDate time.Time, sourceType string, sourceAmount, rate, amount decimal.Decimal, now time.Time) error {
	_, err := tx.Exec(`
		INSERT INTO group_purchase_leadership_pool
		(biz_date, source_type, source_amount, rate, amount, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?)
		ON CONFLICT (biz_date, source_type)
		DO UPDATE SET source_amount = EXCLUDED.source_amount, rate = EXCLUDED.rate, amount = EXCLUDED.amount, updated_at = EXCLUDED.updated_at
	`, bizDate, sourceType, sourceAmount, rate, amount, now, now)
	if err != nil {
		return gerror.Wrap(err, "write leadership reward pool source failed")
	}
	return nil
}

func (s *groupMatchService) sumLoserPrincipalByBizDate(ctx context.Context, bizDate time.Time) (decimal.Decimal, error) {
	row, err := g.DB().GetOne(ctx, `
		SELECT COALESCE(SUM(o.amount), 0) AS amount
		FROM group_match_order o
		JOIN group_match_session s ON s.id = o.session_id
		WHERE o.status = ?
		  AND s.session_date = ?::date
	`, consts.GroupMatchOrderStatusLoser, bizDate.Format("2006-01-02"))
	if err != nil {
		return decimal.Zero, gerror.Wrap(err, "count non-winner principal failed")
	}
	amount, _ := decimal.NewFromString(row["amount"].String())
	return amount, nil
}

func (s *groupMatchService) sumDailyStakeAmount(ctx context.Context, bizDate time.Time) (decimal.Decimal, error) {
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
	`, windowStart, windowEnd)
	if err != nil {
		return decimal.Zero, gerror.Wrap(err, "count daily new stake failed")
	}
	amount, _ := decimal.NewFromString(row["amount"].String())
	return amount, nil
}
func (s *groupMatchService) loadLeadershipLevelRules(ctx context.Context) ([]leadershipLevelRule, error) {
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
		return nil, gerror.Wrap(err, "parse leadership reward level rules failed")
	}
	rules := make([]leadershipLevelRule, 0, len(raw.Levels))
	for _, level := range raw.Levels {
		if level.DailyShares <= 0 || strings.TrimSpace(level.Key) == "" || !level.RewardRate.GreaterThan(decimal.Zero) {
			continue
		}
		rules = append(rules, leadershipLevelRule{
			Key:         strings.TrimSpace(level.Key),
			Name:        strings.TrimSpace(level.Name),
			DailyShares: level.DailyShares,
			RewardRate:  level.RewardRate,
		})
	}
	if len(rules) == 0 {
		return nil, gerror.New("leadership reward level rules are empty")
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].DailyShares < rules[j].DailyShares
	})
	return rules, nil
}

type diffDetailItem struct {
	userID       int64
	wallet       string
	levelKey     string
	levelName    string
	teamShares   int64
	levelRate    decimal.Decimal
	childMaxRate decimal.Decimal
	diffRate     decimal.Decimal
	rewardAmount decimal.Decimal
	isVertexSink int
}

type weightDetailItem struct {
	userID       int64
	wallet       string
	levelName    string
	teamShares   int64
	totalShares  int64
	poolAmount   decimal.Decimal
	rewardAmount decimal.Decimal
}

func (s *groupMatchService) batchInsertDiffLeadershipRewardDetailTx(
	ctx context.Context,
	tx gdb.TX,
	bizDate time.Time,
	batchID string,
	now time.Time,
	totalLoserPrincipal decimal.Decimal,
	loserPool decimal.Decimal,
	items []*diffDetailItem,
) (map[int64]struct{}, error) {
	if len(items) == 0 {
		return map[int64]struct{}{}, nil
	}
	insertedUserIDs := make(map[int64]struct{}, len(items))
	const batchSize = 500
	baseSQL := `INSERT INTO group_purchase_leadership_reward_detail
		(biz_date, batch_id, user_id, wallet_address, level_key, level_name, team_shares, level_rate, child_max_rate, diff_rate,
		 source_loser_amount, source_ticket_amount, total_pool_amount, reward_amount, is_vertex_sink, created_at)
		VALUES `
	for start := 0; start < len(items); start += batchSize {
		end := start + batchSize
		if end > len(items) {
			end = len(items)
		}
		var sb strings.Builder
		sb.WriteString(baseSQL)
		params := make([]interface{}, 0, (end-start)*16)
		for i := start; i < end; i++ {
			item := items[i]
			if i > start {
				sb.WriteString(", ")
			}
			sb.WriteString("(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
			params = append(params,
				bizDate, batchID, item.userID, item.wallet, item.levelKey, item.levelName,
				item.teamShares, item.levelRate, item.childMaxRate, item.diffRate,
				totalLoserPrincipal, decimal.Zero, loserPool, item.rewardAmount,
				item.isVertexSink, now,
			)
		}
		sb.WriteString(" ON CONFLICT (biz_date, user_id) DO NOTHING RETURNING user_id")
		type insertedRow struct {
			UserID int64 `json:"user_id"`
		}
		insertedRows := make([]*insertedRow, 0)
		if err := tx.GetScan(&insertedRows, sb.String(), params...); err != nil {
			return nil, gerror.Wrap(err, "batch insert diff leadership reward detail failed")
		}
		for _, row := range insertedRows {
			if row == nil || row.UserID <= 0 {
				continue
			}
			insertedUserIDs[row.UserID] = struct{}{}
		}
	}
	return insertedUserIDs, nil
}

func (s *groupMatchService) batchInsertWeightLeadershipRewardDetailTx(
	ctx context.Context,
	tx gdb.TX,
	bizDate time.Time,
	batchID string,
	now time.Time,
	items []*weightDetailItem,
) (map[int64]struct{}, error) {
	if len(items) == 0 {
		return map[int64]struct{}{}, nil
	}
	insertedUserIDs := make(map[int64]struct{}, len(items))
	const batchSize = 500
	baseSQL := `INSERT INTO group_purchase_leadership_weight_reward_detail
		(biz_date, batch_id, user_id, wallet_address, level_name, team_shares, total_shares, pool_amount, reward_amount, is_vertex_sink, created_at)
		VALUES `
	for start := 0; start < len(items); start += batchSize {
		end := start + batchSize
		if end > len(items) {
			end = len(items)
		}
		var sb strings.Builder
		sb.WriteString(baseSQL)
		params := make([]interface{}, 0, (end-start)*11)
		for i := start; i < end; i++ {
			item := items[i]
			if i > start {
				sb.WriteString(", ")
			}
			sb.WriteString("(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
			params = append(params,
				bizDate, batchID, item.userID, item.wallet, item.levelName, item.teamShares,
				item.totalShares, item.poolAmount, item.rewardAmount, 0, now,
			)
		}
		sb.WriteString(" ON CONFLICT (biz_date, user_id) DO NOTHING RETURNING user_id")
		type insertedRow struct {
			UserID int64 `json:"user_id"`
		}
		insertedRows := make([]*insertedRow, 0)
		if err := tx.GetScan(&insertedRows, sb.String(), params...); err != nil {
			return nil, gerror.Wrap(err, "batch insert weight leadership reward detail failed")
		}
		for _, row := range insertedRows {
			if row == nil || row.UserID <= 0 {
				continue
			}
			insertedUserIDs[row.UserID] = struct{}{}
		}
	}
	return insertedUserIDs, nil
}

func filterDiffDetailItemsByUserIDs(items []*diffDetailItem, userIDs map[int64]struct{}) []*diffDetailItem {
	if len(items) == 0 || len(userIDs) == 0 {
		return []*diffDetailItem{}
	}
	filtered := make([]*diffDetailItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if _, ok := userIDs[item.userID]; !ok {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func filterWeightDetailItemsByUserIDs(items []*weightDetailItem, userIDs map[int64]struct{}) []*weightDetailItem {
	if len(items) == 0 || len(userIDs) == 0 {
		return []*weightDetailItem{}
	}
	filtered := make([]*weightDetailItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if _, ok := userIDs[item.userID]; !ok {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}
