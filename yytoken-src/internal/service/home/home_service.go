package home

import (
	"context"
	"fmt"
	"strings"
	"time"

	teamDao "XWFrame/internal/dao/team"
	coboEntity "XWFrame/internal/entity/cobo"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/frame/db"
	"XWFrame/internal/repository"
	"XWFrame/internal/repository/home"
	"XWFrame/internal/repository/price"
	rewardRepo "XWFrame/internal/repository/reward"
	"XWFrame/internal/service/home/model"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

type IHomeService interface {
	GetOverview(ctx context.Context) (*model.HomeOverview, error)
	GetPriceSeries(ctx context.Context, token string) (*model.PriceSeries, error)
	GetHomeStats(ctx context.Context) (*model.HomeStats, error)
}

type homeService struct {
	homeRepo        home.IHomeRepository
	priceRepo       price.IPriceRepository
	assetRecordRepo rewardRepo.IAssetRecordRepository
	userRepo        repository.IUserRepository
	teamDao         teamDao.ITeamDao
	teamStatsDao    teamDao.ITeamStatsDao
}

var homeServiceInstance *homeService

func Svc() IHomeService {
	if homeServiceInstance == nil {
		homeServiceInstance = &homeService{
			homeRepo:        home.NewHomeRepository(),
			priceRepo:       price.NewPriceRepository(),
			assetRecordRepo: rewardRepo.NewAssetRecordRepository(),
			userRepo:        repository.NewUserRepository(),
			teamDao:         teamDao.NewTeamDao(),
			teamStatsDao:    teamDao.NewTeamStatsDao(),
		}
	}
	return homeServiceInstance
}

func (s *homeService) GetOverview(ctx context.Context) (*model.HomeOverview, error) {
	// 数据验证：从上下文获取当前用户ID
	userID := ctx.Value("user_id")
	if userID == nil {
		return nil, gerror.New("未获取到用户信息")
	}
	uid, ok := userID.(int64)
	if !ok {
		return nil, gerror.New("用户ID类型错误")
	}

	// 1. 获取昨日收益（业务逻辑：聚合计算）
	offset0 := 0
	yesterdayIncome, err := s.assetRecordRepo.GetUserRewardTotalByOffset(ctx, uid, consts.RewardTypesForStatistics, &offset0)
	if err != nil {
		g.Log().Warningf(ctx, "[首页概览] 获取昨日收益失败: %v", err)
		yesterdayIncome = decimal.Zero
	}

	// 2. 获取个人总算力（业务逻辑：筛选和聚合）
	personalPower, err := s.homeRepo.GetUserPersonalPower(ctx, uid)
	if err != nil {
		g.Log().Warningf(ctx, "[首页概览] 获取算力包失败: %v", err)
		personalPower = decimal.Zero
	}

	// 3. 获取最新价格（业务逻辑：格式化）
	rexPrice := "0.00"
	apgPrice := "0.00"
	latestPrice, err := s.homeRepo.GetLatestPrice(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "[首页概览] 获取最新价格失败: %v", err)
	} else if latestPrice != nil {
		rexPrice = utils.FormatDecimal(latestPrice.RexPrice)
		apgPrice = utils.FormatDecimal(latestPrice.ApgPrice)
	}

	// 4. 获取用户服务中心信息
	isServiceCenter := false
	serviceCenterRate := "0.00"
	userInfo, err := s.userRepo.GetUserById(ctx, uid)
	if err != nil {
		g.Log().Warningf(ctx, "[首页概览] 获取用户信息失败: %v", err)
	} else if userInfo != nil {
		isServiceCenter = userInfo.IsServiceCenter
		serviceCenterRate = utils.FormatDecimal(userInfo.ServiceCenterRate)
	}

	// Service层组装model结构体（业务逻辑：数据格式化）
	return &model.HomeOverview{
		YesterdayIncome:   utils.FormatDecimal(yesterdayIncome),
		PersonalPower:     utils.FormatDecimal(personalPower),
		RexPrice:          rexPrice,
		ApgPrice:          apgPrice,
		IsServiceCenter:   isServiceCenter,
		ServiceCenterRate: serviceCenterRate,
	}, nil
}

func (s *homeService) GetPriceSeries(ctx context.Context, token string) (*model.PriceSeries, error) {
	// 调用Repository层获取原始数据（用于图表展示）
	prices, err := s.priceRepo.GetLatestN(ctx, 7)
	if err != nil {
		return nil, err
	}

	// 业务逻辑：如果数据不足，返回空数据
	if len(prices) == 0 {
		return &model.PriceSeries{
			Points:     []*model.PricePoint{},
			GrowthRate: "0.0%",
			RexPrice:   "0.00",
			ApgPrice:   "0.0000",
		}, nil
	}

	// 获取最新价格（prices[0] 是最新的记录）
	latestPrice := prices[0]
	rexPrice := utils.FormatDecimal(latestPrice.RexPrice)
	apgPrice := utils.FormatDecimal(latestPrice.ApgPrice)

	// 业务逻辑：转换为前端需要的格式（需要反转顺序，因为查询是倒序的）
	items := make([]*model.PricePoint, 0, len(prices))
	for i := len(prices) - 1; i >= 0; i-- {
		p := prices[i]
		var priceValue string
		// 根据token类型选择对应的价格
		if token == "apg" || token == "APG" {
			priceValue = utils.FormatDecimal(p.ApgPrice)
		} else {
			// 默认返回REX价格
			priceValue = utils.FormatDecimal(p.RexPrice)
		}

		items = append(items, &model.PricePoint{
			T: p.PriceTime.Format(consts.TimeFormatDateTime),
			P: priceValue,
		})
	}

	// 业务逻辑：计算当日涨跌幅（基于今日开盘价）
	// 合约在 UTC 00:00 调用 recordDailyOpenPrice
	growth := decimal.Zero
	now := time.Now().UTC()

	// 获取今日开盘价（source='manual_daily_open'）
	dailyOpenPrice, err := s.priceRepo.GetDailyOpenPrice(ctx, now)
	if err != nil {
		g.Log().Warningf(ctx, "[价格曲线] 获取今日开盘价失败: %v", err)
	}

	if dailyOpenPrice != nil {
		// 使用今日开盘价和当前最新价格计算涨跌幅
		var openPriceValue, latestPriceValue decimal.Decimal
		if token == "apg" || token == "APG" {
			openPriceValue = dailyOpenPrice.ApgPrice
			latestPriceValue = latestPrice.ApgPrice
		} else {
			openPriceValue = dailyOpenPrice.RexPrice
			latestPriceValue = latestPrice.RexPrice
		}
		if openPriceValue.GreaterThan(decimal.Zero) {
			growth = latestPriceValue.Sub(openPriceValue).DivRound(openPriceValue, 4).Mul(decimal.NewFromInt(100))
		}
	} else {
		// 降级方案：如果今日开盘价不存在，使用现有数据的第一条和最后一条
		if len(prices) >= 2 {
			var first, last decimal.Decimal
			if token == "apg" || token == "APG" {
				first = prices[len(prices)-1].ApgPrice
				last = prices[0].ApgPrice
			} else {
				first = prices[len(prices)-1].RexPrice
				last = prices[0].RexPrice
			}
			if first.GreaterThan(decimal.Zero) {
				growth = last.Sub(first).DivRound(first, 4).Mul(decimal.NewFromInt(100))
			}
		}
	}

	// Service层组装model结构体
	return &model.PriceSeries{
		Points:     items,
		GrowthRate: utils.FormatDecimal(growth) + "%",
		RexPrice:   rexPrice,
		ApgPrice:   apgPrice,
	}, nil
}

func (s *homeService) GetHomeStats(ctx context.Context) (*model.HomeStats, error) {
	baseStats, err := s.homeRepo.GetHomeBaseStats(ctx)
	if err != nil {
		return nil, err
	}

	result := &model.HomeStats{}
	if baseStats != nil {
		result.UserCount = g.NewVar(baseStats.UserCount).String()

		result.TotalStakeUSDT = utils.FormatDecimal(baseStats.TotalStakeUSDT)
		result.TotalWithdrawUSDT = utils.FormatDecimal(baseStats.TotalWithdrawUSDT)

		result.YesterdayStakeUSDT = utils.FormatDecimal(baseStats.YesterdayStakeUSDT)
		result.YesterdayWithdrawUSDT = utils.FormatDecimal(baseStats.YesterdayWithdrawUSDT)

		result.NodeUserCount = g.NewVar(baseStats.NodeUserCount).String()
		result.StakeUserCount = g.NewVar(baseStats.StakeUserCount).String()

		result.TotalGiftStakeUSDT = utils.FormatDecimal(baseStats.TotalGiftStakeUSDT)
		result.YesterdayGiftStakeUSDT = utils.FormatDecimal(baseStats.YesterdayGiftStakeUSDT)
		result.RealPurchasedStakeValue = utils.FormatDecimal(baseStats.RealPurchasedStakeValue)

		result.TodayGroupMatchUserCount = g.NewVar(baseStats.TodayGroupMatchUserCount).String()
		result.TodayGroupMatchAmount = utils.FormatDecimal(baseStats.TodayGroupMatchAmount)
		result.PendingWithdrawCount = g.NewVar(baseStats.PendingWithdrawCount).String()
		result.TodayActiveUserCount = g.NewVar(baseStats.TodayActiveUserCount).String()

		result.TotalRechargeUSDT = utils.FormatDecimal(baseStats.TotalRechargeUSDT)
		result.TotalYYBalance = utils.FormatDecimal(baseStats.TotalYYBalance)
		result.TodaySwapYY = utils.FormatDecimal(baseStats.TodaySwapYY)
		result.TodayBurnYY = utils.FormatDecimal(baseStats.TodayBurnYY)
		result.TodayReleaseYY = utils.FormatDecimal(baseStats.TodayReleaseYY)
		result.YesterdaySwapYY = utils.FormatDecimal(baseStats.YesterdaySwapYY)
		result.YesterdayBurnYY = utils.FormatDecimal(baseStats.YesterdayBurnYY)
		result.HistoricalBurnYY = utils.FormatDecimal(baseStats.HistoricalBurnYY)
		result.HistoricalBurnJU = utils.FormatDecimal(baseStats.HistoricalBurnJU)
		result.HistoricalBurnSZPN = utils.FormatDecimal(baseStats.HistoricalBurnSZPN)
		result.TotalYYSwapFeeUSDT = utils.FormatDecimal(baseStats.TotalYYSwapFeeUSDT)
	}

	// 顶级团队24小时质押/提现：按 /pr 口径，以 R-ABxx 为根统计子团队
	topTeamStats, err := s.getTopTeam24hStats(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "[首页统计] 获取顶级团队24h数据失败: err=%v", err)
	} else {
		result.TopTeam24hStats = topTeamStats
	}

	return result, nil
}

// prTargetLeaderAddresses /pr 日报的4个顶级领导人地址
var prTargetLeaderAddresses = []string{
	"0x239fd41551f83f91aa2500be987a6a7f12994b1d", // R-AB11
	"0x52681178a52491ff51e1ba416448fda2f3300588", // R-AB12
	"0xda23d3ee389999ac10a4161f2a531dc1ffd12b72", // R-AB21
	"0x73f0ed351e91bc649ac675ca2cda40af396a1e24", // R-AB22
}

type topTeam24hRow struct {
	TeamId         int64           `json:"team_id"`
	TeamName       string          `json:"team_name"`
	StakeAmount    decimal.Decimal `json:"stake_amount"`
	WithdrawAmount decimal.Decimal `json:"withdraw_amount"`
}

// getTopTeam24hStats 按 /pr 口径统计子团队24h质押/提现
// 以 R-ABxx 为顶级领导人，递归统计其直推子团队长（排除首码/R-）及其下级的24h数据
func (s *homeService) getTopTeam24hStats(ctx context.Context) ([]*model.TopTeam24hStats, error) {
	quoted := make([]string, 0, len(prTargetLeaderAddresses))
	for _, addr := range prTargetLeaderAddresses {
		quoted = append(quoted, fmt.Sprintf("'%s'", strings.ToLower(addr)))
	}
	targetLeaders := strings.Join(quoted, ",")

	var rows []topTeam24hRow
	sql := fmt.Sprintf(`
		WITH RECURSIVE target_teams AS (
			SELECT
				t.id AS team_id,
				t.name AS team_name,
				LOWER(t.leader_wallet_address) AS leader_address,
				leader.invite_code AS leader_invite_code
			FROM team t
			JOIN user_info leader ON LOWER(leader.wallet_address) = LOWER(t.leader_wallet_address)
			WHERE t.status = 1
			  AND LOWER(t.leader_wallet_address) IN (%s)
		),
		direct_members AS (
			SELECT
				tt.team_id AS parent_team_id,
				tt.team_name AS parent_team_name,
				u.id AS user_id,
				LOWER(u.wallet_address) AS wallet_address,
				t2.id AS child_team_id,
				t2.name AS child_team_name
			FROM target_teams tt
			JOIN user_info u ON u.parent_invite_code = tt.leader_invite_code
			JOIN team t2 ON LOWER(t2.leader_wallet_address) = LOWER(u.wallet_address) AND t2.status = 1
			WHERE u.status = 1
			  AND COALESCE(u.is_test, 0) = 0
			  AND t2.name <> '首码'
			  AND t2.name NOT LIKE 'R-%%'
		),
		member_chain AS (
			SELECT
				dm.child_team_id AS team_id,
				dm.child_team_name AS team_name,
				dm.user_id
			FROM direct_members dm

			UNION ALL

			SELECT
				mc.team_id,
				mc.team_name,
				u.id AS user_id
			FROM member_chain mc
			JOIN user_info p ON p.id = mc.user_id
			JOIN user_info u ON u.parent_invite_code = p.invite_code
			WHERE u.status = 1
			  AND COALESCE(u.is_test, 0) = 0
		),
		stake_stats AS (
			SELECT mc.team_id, mc.team_name, COALESCE(SUM(svo.amount), 0) AS stake_amount
			FROM member_chain mc
			JOIN staking_v2_order svo ON svo.user_id = mc.user_id
				AND svo.created_at >= NOW() - INTERVAL '24 hours'
				AND svo.status > 0
			GROUP BY mc.team_id, mc.team_name
		),
		withdraw_stats AS (
			SELECT mc.team_id, mc.team_name, COALESCE(SUM(cwr.actual_amount), 0) AS withdraw_amount
			FROM member_chain mc
			JOIN cobo_withdraw_request cwr ON cwr.user_id = mc.user_id
				AND cwr.created_at >= NOW() - INTERVAL '24 hours'
				AND cwr.symbol = 'USDT'
				AND cwr.status = %d
			GROUP BY mc.team_id, mc.team_name
		)
		SELECT
			COALESCE(s.team_id, w.team_id) AS team_id,
			COALESCE(s.team_name, w.team_name) AS team_name,
			COALESCE(s.stake_amount, 0) AS stake_amount,
			COALESCE(w.withdraw_amount, 0) AS withdraw_amount
		FROM stake_stats s
		FULL OUTER JOIN withdraw_stats w ON s.team_id = w.team_id
		ORDER BY COALESCE(s.stake_amount, 0) + COALESCE(w.withdraw_amount, 0) DESC
	`, targetLeaders, coboEntity.WithdrawStatusSuccess)

	if err := db.GetDB().Ctx(ctx).Raw(sql).Scan(&rows); err != nil {
		return nil, err
	}

	out := make([]*model.TopTeam24hStats, 0, len(rows))
	for _, r := range rows {
		out = append(out, &model.TopTeam24hStats{
			TeamId:         r.TeamId,
			TeamName:       r.TeamName,
			StakeAmount:    utils.FormatDecimal(r.StakeAmount),
			WithdrawAmount: utils.FormatDecimal(r.WithdrawAmount),
		})
	}
	return out, nil
}
