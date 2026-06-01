package home

import (
	"context"
	"time"

	"XWFrame/internal/dao/reward"
	coboEntity "XWFrame/internal/entity/cobo"
	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/db"
	rewardRepo "XWFrame/internal/repository/reward"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/shopspring/decimal"
)

// IHomeRepository 首页仓储接口
type IHomeRepository interface {
	// GetUserYesterdayIncome 获取用户昨日收益
	GetUserYesterdayIncome(ctx context.Context, userID int64) (decimal.Decimal, error)

	// GetUserPersonalPower 获取用户个人总算力
	GetUserPersonalPower(ctx context.Context, userID int64) (decimal.Decimal, error)

	// GetLatestPrice 获取最新价格
	GetLatestPrice(ctx context.Context) (*rewardEntity.PriceHistoryEntity, error)

	// GetHomeBaseStats 获取首页基础统计数据
	GetHomeBaseStats(ctx context.Context) (*HomeBaseStats, error)
}

// HomeBaseStats 首页统计数据（Repository层）
type HomeBaseStats struct {
	UserCount int `json:"user_count"` // 用户数量

	TotalStakeUSDT    decimal.Decimal `json:"total_stake_usdt"`    // 总质押USDT（staking_v2_order）
	TotalWithdrawUSDT decimal.Decimal `json:"total_withdraw_usdt"` // USDT提取总量

	YesterdayStakeUSDT    decimal.Decimal `json:"yesterday_stake_usdt"`    // 昨日质押USDT（staking_v2_order）
	YesterdayWithdrawUSDT decimal.Decimal `json:"yesterday_withdraw_usdt"` // 昨日USDT提取

	NodeUserCount  int `json:"node_user_count"`  // 兼容旧字段：质押用户数量（有过 staking_v2_order 的去重用户数）
	StakeUserCount int `json:"stake_user_count"` // 质押用户数量（有过 staking_v2_order 的去重用户数）

	TotalGiftStakeUSDT      decimal.Decimal `json:"total_gift_stake_usdt"`      // 赠送质押总USDT（staking_v2_order）
	YesterdayGiftStakeUSDT  decimal.Decimal `json:"yesterday_gift_stake_usdt"`  // 昨日赠送质押USDT（staking_v2_order）
	RealPurchasedStakeValue decimal.Decimal `json:"real_purchased_stake_value"` // 真实购买质押总USDT（staking_v2_order.is_gift=0）

	TodayGroupMatchUserCount int             `json:"today_group_match_user_count"` // 今日参与拼团人数
	TodayGroupMatchAmount    decimal.Decimal `json:"today_group_match_amount"`     // 今日参与拼团金额
	PendingWithdrawCount     int             `json:"pending_withdraw_count"`       // 待处理提现数
	TodayActiveUserCount     int             `json:"today_active_user_count"`      // 今日活跃用户

	TotalRechargeUSDT decimal.Decimal `json:"total_recharge_usdt"` // 平台总充值USDT（排除测试用户）

	TotalYYBalance     decimal.Decimal `json:"total_yy_balance"`       // 平台用户YY总余额（可用+冻结，排除测试用户）
	TodaySwapYY        decimal.Decimal `json:"today_swap_yy"`          // 今日兑换YY数量
	TodayBurnYY        decimal.Decimal `json:"today_burn_yy"`          // 今日销毁YY数量
	TodayReleaseYY     decimal.Decimal `json:"today_release_yy"`       // 今日释放YY数量
	YesterdaySwapYY    decimal.Decimal `json:"yesterday_swap_yy"`      // 昨日兑换YY数量
	YesterdayBurnYY    decimal.Decimal `json:"yesterday_burn_yy"`      // 昨日销毁YY数量
	HistoricalBurnYY   decimal.Decimal `json:"historical_burn_yy"`     // 历史YY销毁数量
	HistoricalBurnJU   decimal.Decimal `json:"historical_burn_ju"`     // 历史JU销毁数量
	HistoricalBurnSZPN decimal.Decimal `json:"historical_burn_szpn"`   // 历史SZPN销毁数量
	TotalYYSwapFeeUSDT decimal.Decimal `json:"total_yy_swap_fee_usdt"` // YY兑换手续费总计(USDT)
}

// homeRepository 首页仓储实现
type homeRepository struct {
	assetRecordRepo rewardRepo.IAssetRecordRepository
	stakingDao      reward.IStakingPackageDao
	priceDao        reward.IPriceHistoryDao
}

// NewHomeRepository 创建首页仓储实例
func NewHomeRepository() IHomeRepository {
	return &homeRepository{
		assetRecordRepo: rewardRepo.NewAssetRecordRepository(),
		stakingDao:      reward.NewStakingPackageDao(),
		priceDao:        reward.NewPriceHistoryDao(),
	}
}

// GetUserYesterdayIncome 获取用户昨日收益
// 注意：此方法已废弃，业务逻辑应在 service 层处理
func (r *homeRepository) GetUserYesterdayIncome(ctx context.Context, userID int64) (decimal.Decimal, error) {
	return decimal.Zero, gerror.New("请使用 service 层方法")
}

// GetUserPersonalPower 获取用户个人总算力
func (r *homeRepository) GetUserPersonalPower(ctx context.Context, userID int64) (decimal.Decimal, error) {
	packages, err := r.stakingDao.GetActivePackages(ctx)
	if err != nil {
		return decimal.Zero, err
	}

	personalPower := decimal.Zero
	for _, pkg := range packages {
		if pkg.UserID == userID {
			personalPower = personalPower.Add(pkg.PowerValue)
		}
	}
	return personalPower, nil
}

// GetLatestPrice 获取最新价格
func (r *homeRepository) GetLatestPrice(ctx context.Context) (*rewardEntity.PriceHistoryEntity, error) {
	return r.priceDao.GetLatest(ctx)
}

// GetHomeBaseStats 获取首页基础统计数据
func (r *homeRepository) GetHomeBaseStats(ctx context.Context) (*HomeBaseStats, error) {
	stats := &HomeBaseStats{
		TotalStakeUSDT:           decimal.Zero,
		TotalWithdrawUSDT:        decimal.Zero,
		YesterdayStakeUSDT:       decimal.Zero,
		YesterdayWithdrawUSDT:    decimal.Zero,
		NodeUserCount:            0,
		StakeUserCount:           0,
		TotalGiftStakeUSDT:       decimal.Zero,
		YesterdayGiftStakeUSDT:   decimal.Zero,
		RealPurchasedStakeValue:  decimal.Zero,
		TodayGroupMatchUserCount: 0,
		TodayGroupMatchAmount:    decimal.Zero,
		PendingWithdrawCount:     0,
		TodayActiveUserCount:     0,
		TotalRechargeUSDT:        decimal.Zero,
		TotalYYBalance:           decimal.Zero,
		TodaySwapYY:              decimal.Zero,
		TodayBurnYY:              decimal.Zero,
		TodayReleaseYY:           decimal.Zero,
		YesterdaySwapYY:          decimal.Zero,
		YesterdayBurnYY:          decimal.Zero,
		HistoricalBurnYY:         decimal.Zero,
		HistoricalBurnJU:         decimal.Zero,
		HistoricalBurnSZPN:       decimal.Zero,
		TotalYYSwapFeeUSDT:       decimal.Zero,
	}

	// 1. 用户数量
	userCount, err := db.GetDB().Ctx(ctx).Model("user_info").Count()
	if err == nil {
		stats.UserCount = userCount
	}

	// 2. 总质押 USDT（staking_v2_order.amount 求和）
	stakeUsdtValue, err := db.GetDB().Ctx(ctx).Model("staking_v2_order").
		Fields("COALESCE(SUM(amount), 0)").
		Value()
	if err == nil && !stakeUsdtValue.IsNil() {
		stats.TotalStakeUSDT, _ = decimal.NewFromString(stakeUsdtValue.String())
	}

	// 3. Cobo 总提现 USDT（cobo_withdraw_request，symbol=USDT，status=success，actual_amount）
	totalWithdrawUsdt, err := db.GetDB().Ctx(ctx).Model("cobo_withdraw_request").
		Where("symbol = ?", "USDT").
		Where("status = ?", coboEntity.WithdrawStatusSuccess).
		Fields("COALESCE(SUM(actual_amount), 0)").
		Value()
	if err == nil && !totalWithdrawUsdt.IsNil() {
		stats.TotalWithdrawUSDT, _ = decimal.NewFromString(totalWithdrawUsdt.String())
	}

	// 4. 昨日时间范围（本地时间）
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	startOfYesterday := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// 5. 昨日质押 USDT（staking_v2_order）
	yStakeUsdt, err := db.GetDB().Ctx(ctx).Model("staking_v2_order").
		Where("created_at >= ?", startOfYesterday).
		Where("created_at < ?", startOfToday).
		Fields("COALESCE(SUM(amount), 0)").
		Value()
	if err == nil && !yStakeUsdt.IsNil() {
		stats.YesterdayStakeUSDT, _ = decimal.NewFromString(yStakeUsdt.String())
	}

	// 6. 昨日 Cobo 提现 USDT
	yWithdrawUsdt, err := db.GetDB().Ctx(ctx).Model("cobo_withdraw_request").
		Where("symbol = ?", "USDT").
		Where("status = ?", coboEntity.WithdrawStatusSuccess).
		Where("created_at >= ?", startOfYesterday).
		Where("created_at < ?", startOfToday).
		Fields("COALESCE(SUM(actual_amount), 0)").
		Value()
	if err == nil && !yWithdrawUsdt.IsNil() {
		stats.YesterdayWithdrawUSDT, _ = decimal.NewFromString(yWithdrawUsdt.String())
	}

	// 7. 质押用户数量（有过 staking_v2_order 的去重用户数）
	stakeUserCountValue, err := db.GetDB().Ctx(ctx).Model("staking_v2_order").
		Fields("COUNT(DISTINCT user_id)").
		Value()
	if err == nil && !stakeUserCountValue.IsNil() {
		stats.StakeUserCount = stakeUserCountValue.Int()
		stats.NodeUserCount = stats.StakeUserCount
	}

	// 8. 赠送质押总 USDT
	giftStakeUsdtValue, err := db.GetDB().Ctx(ctx).Model("staking_v2_order").
		Where("is_gift = ?", 1).
		Fields("COALESCE(SUM(amount), 0)").
		Value()
	if err == nil && !giftStakeUsdtValue.IsNil() {
		stats.TotalGiftStakeUSDT, _ = decimal.NewFromString(giftStakeUsdtValue.String())
	}

	// 8.1 真实购买质押总 USDT
	realPurchasedStakeValue, err := db.GetDB().Ctx(ctx).Model("staking_v2_order").
		Where("is_gift = ?", 0).
		Fields("COALESCE(SUM(amount), 0)").
		Value()
	if err == nil && !realPurchasedStakeValue.IsNil() {
		stats.RealPurchasedStakeValue, _ = decimal.NewFromString(realPurchasedStakeValue.String())
	}

	// 9. 昨日赠送质押 USDT
	yGiftStakeUsdt, err := db.GetDB().Ctx(ctx).Model("staking_v2_order").
		Where("is_gift = ?", 1).
		Where("created_at >= ?", startOfYesterday).
		Where("created_at < ?", startOfToday).
		Fields("COALESCE(SUM(amount), 0)").
		Value()
	if err == nil && !yGiftStakeUsdt.IsNil() {
		stats.YesterdayGiftStakeUSDT, _ = decimal.NewFromString(yGiftStakeUsdt.String())
	}

	// 10. 今日参与拼团人数（group_match_order 当天去重 user_id）
	todayGroupMatchUserCount, err := db.GetDB().Ctx(ctx).Model("group_match_order").
		Where("created_at >= ?", startOfToday).
		Fields("COUNT(DISTINCT user_id)").
		Value()
	if err == nil && !todayGroupMatchUserCount.IsNil() {
		stats.TodayGroupMatchUserCount = todayGroupMatchUserCount.Int()
	}

	// 11. 今日参与拼团金额（group_match_order 当天 amount 总和）
	todayGroupMatchAmount, err := db.GetDB().Ctx(ctx).Model("group_match_order").
		Where("created_at >= ?", startOfToday).
		Fields("COALESCE(SUM(amount), 0)").
		Value()
	if err == nil && !todayGroupMatchAmount.IsNil() {
		stats.TodayGroupMatchAmount, _ = decimal.NewFromString(todayGroupMatchAmount.String())
	}

	// 12. 待处理提现数（cobo_withdraw_request status=0 待审核）
	pendingWithdrawCount, err := db.GetDB().Ctx(ctx).Model("cobo_withdraw_request").
		Where("status = ?", coboEntity.WithdrawStatusPending).
		Count()
	if err == nil {
		stats.PendingWithdrawCount = pendingWithdrawCount
	}

	// 13. 今日活跃用户（user_info last_login_at >= 今天开始）
	todayActiveUserCount, err := db.GetDB().Ctx(ctx).Model("user_info").
		Where("last_login_at >= ?", startOfToday).
		Count()
	if err == nil {
		stats.TodayActiveUserCount = todayActiveUserCount
	}

	// 14. 平台总充值 USDT（cobo_recharge_record 已确认，排除测试用户）
	totalRechargeUsdt, err := db.GetDB().Ctx(ctx).Raw(`
		SELECT COALESCE(SUM(crr.amount), 0)
		FROM cobo_recharge_record crr
		JOIN user_info ui ON crr.user_id = ui.id
		WHERE crr.symbol = 'USDT'
		  AND crr.status = 1
		  AND ui.is_test = 0
	`).Value()
	if err == nil && !totalRechargeUsdt.IsNil() {
		stats.TotalRechargeUSDT, _ = decimal.NewFromString(totalRechargeUsdt.String())
	}

	// 15. 平台用户YY总余额（可用+冻结，排除测试用户）
	totalYYBalance, err := db.GetDB().Ctx(ctx).Raw(`
		SELECT COALESCE(SUM(cb.available_amount + cb.frozen_amount), 0)
		FROM cobo_balance cb
		JOIN user_info ui ON cb.user_id = ui.id
		WHERE cb.symbol = 'YY'
		  AND ui.is_test = 0
	`).Value()
	if err == nil && !totalYYBalance.IsNil() {
		stats.TotalYYBalance, _ = decimal.NewFromString(totalYYBalance.String())
	}

	// 16. 今日兑换YY数量（swap_record.from_symbol=YY）
	todaySwapYY, err := db.GetDB().Ctx(ctx).Model("swap_record").
		Where("from_symbol = ?", "YY").
		Where("created_at >= ?", startOfToday).
		Fields("COALESCE(SUM(from_amount), 0)").
		Value()
	if err == nil && !todaySwapYY.IsNil() {
		stats.TodaySwapYY, _ = decimal.NewFromString(todaySwapYY.String())
	}

	// 17. 今日销毁YY数量（拼团门票销毁日志，symbol=YY）
	todayBurnYY, err := db.GetDB().Ctx(ctx).Model("cobo_balance_change_log").
		Where("symbol = ?", "YY").
		Where("change_type = ?", "group_match_settle_ticket_burn_to_vertex").
		Where("created_at >= ?", startOfToday).
		Fields("COALESCE(SUM(ABS(amount)), 0)").
		Value()
	if err == nil && !todayBurnYY.IsNil() {
		stats.TodayBurnYY, _ = decimal.NewFromString(todayBurnYY.String())
	}

	// 18. 今日释放YY数量（group_match_loser_comp_release_log.release_amount）
	todayReleaseYY, err := db.GetDB().Ctx(ctx).Model("group_match_loser_comp_release_log").
		Where("token_symbol = ?", "YY").
		Where("release_date = ?", startOfToday.Format("2006-01-02")).
		Fields("COALESCE(SUM(release_amount), 0)").
		Value()
	if err == nil && !todayReleaseYY.IsNil() {
		stats.TodayReleaseYY, _ = decimal.NewFromString(todayReleaseYY.String())
	}

	// 19. 昨日兑换YY数量
	yesterdaySwapYY, err := db.GetDB().Ctx(ctx).Model("swap_record").
		Where("from_symbol = ?", "YY").
		Where("created_at >= ?", startOfYesterday).
		Where("created_at < ?", startOfToday).
		Fields("COALESCE(SUM(from_amount), 0)").
		Value()
	if err == nil && !yesterdaySwapYY.IsNil() {
		stats.YesterdaySwapYY, _ = decimal.NewFromString(yesterdaySwapYY.String())
	}

	// 20. 昨日销毁YY数量
	yesterdayBurnYY, err := db.GetDB().Ctx(ctx).Model("cobo_balance_change_log").
		Where("symbol = ?", "YY").
		Where("change_type = ?", "group_match_settle_ticket_burn_to_vertex").
		Where("created_at >= ?", startOfYesterday).
		Where("created_at < ?", startOfToday).
		Fields("COALESCE(SUM(ABS(amount)), 0)").
		Value()
	if err == nil && !yesterdayBurnYY.IsNil() {
		stats.YesterdayBurnYY, _ = decimal.NewFromString(yesterdayBurnYY.String())
	}

	// 21. 历史YY销毁数量
	historicalBurnYY, err := db.GetDB().Ctx(ctx).Model("cobo_balance_change_log").
		Where("symbol = ?", "YY").
		Where("change_type = ?", "group_match_settle_ticket_burn_to_vertex").
		Fields("COALESCE(SUM(ABS(amount)), 0)").
		Value()
	if err == nil && !historicalBurnYY.IsNil() {
		stats.HistoricalBurnYY, _ = decimal.NewFromString(historicalBurnYY.String())
	}

	// 22. 历史JU销毁数量
	historicalBurnJU, err := db.GetDB().Ctx(ctx).Model("cobo_balance_change_log").
		Where("symbol = ?", "JU").
		Where("change_type = ?", "group_match_settle_ticket_burn_to_vertex").
		Fields("COALESCE(SUM(ABS(amount)), 0)").
		Value()
	if err == nil && !historicalBurnJU.IsNil() {
		stats.HistoricalBurnJU, _ = decimal.NewFromString(historicalBurnJU.String())
	}

	// 23. 历史SZPN销毁数量
	historicalBurnSZPN, err := db.GetDB().Ctx(ctx).Model("cobo_balance_change_log").
		Where("symbol = ?", "SZPN").
		Where("change_type = ?", "group_match_settle_ticket_burn_to_vertex").
		Fields("COALESCE(SUM(ABS(amount)), 0)").
		Value()
	if err == nil && !historicalBurnSZPN.IsNil() {
		stats.HistoricalBurnSZPN, _ = decimal.NewFromString(historicalBurnSZPN.String())
	}

	// 24. YY兑换手续费总计（USDT）
	totalYYSwapFeeUSDT, err := db.GetDB().Ctx(ctx).Model("swap_record").
		Where("from_symbol = ?", "YY").
		Where("to_symbol = ?", "USDT").
		Fields("COALESCE(SUM(fee), 0)").
		Value()
	if err == nil && !totalYYSwapFeeUSDT.IsNil() {
		stats.TotalYYSwapFeeUSDT, _ = decimal.NewFromString(totalYYSwapFeeUSDT.String())
	}

	return stats, nil
}
