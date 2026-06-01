package model

type HomeOverview struct {
	YesterdayIncome   string `json:"yesterday_income"`
	PersonalPower     string `json:"personal_power"`
	RexPrice          string `json:"rex_price"`
	ApgPrice          string `json:"apg_price"`
	IsServiceCenter   bool   `json:"is_service_center"`
	ServiceCenterRate string `json:"service_center_rate"`
}

type PricePoint struct {
	T string `json:"t"`
	P string `json:"p"`
}

// PriceSeries 价格序列及增长率
type PriceSeries struct {
	Points     []*PricePoint `json:"points"`
	GrowthRate string        `json:"growth_rate"`
	RexPrice   string        `json:"rex_price"`
	ApgPrice   string        `json:"apg_price"`
}

// HomeStats 首页统计数据
type HomeStats struct {
	UserCount string `json:"user_count"` // 用户数量

	TotalStakeUSDT    string `json:"total_stake_usdt"`    // 总质押USDT（排除赠送节点）
	TotalWithdrawUSDT string `json:"total_withdraw_usdt"` // USDT提取总量

	YesterdayStakeUSDT    string `json:"yesterday_stake_usdt"`    // 昨日质押USDT（排除赠送节点）
	YesterdayWithdrawUSDT string `json:"yesterday_withdraw_usdt"` // 昨日USDT提取

	NodeUserCount  string `json:"node_user_count"`  // 兼容旧字段：质押用户数量
	StakeUserCount string `json:"stake_user_count"` // 质押用户数量

	TotalGiftStakeUSDT      string `json:"total_gift_stake_usdt"`      // 赠送质押总USDT
	YesterdayGiftStakeUSDT  string `json:"yesterday_gift_stake_usdt"`  // 昨日赠送质押USDT
	RealPurchasedStakeValue string `json:"real_purchased_stake_value"` // 真实购买质押总USDT

	TodayGroupMatchUserCount string `json:"today_group_match_user_count"` // 今日参与拼团人数
	TodayGroupMatchAmount    string `json:"today_group_match_amount"`     // 今日参与拼团金额
	PendingWithdrawCount     string `json:"pending_withdraw_count"`       // 待处理提现数
	TodayActiveUserCount     string `json:"today_active_user_count"`      // 今日活跃用户

	TotalRechargeUSDT string `json:"total_recharge_usdt"` // 平台总充值USDT

	TotalYYBalance     string `json:"total_yy_balance"`       // 用户YY余额总计
	TodaySwapYY        string `json:"today_swap_yy"`          // 今日兑换YY数量
	TodayBurnYY        string `json:"today_burn_yy"`          // 今日销毁YY数量
	TodayReleaseYY     string `json:"today_release_yy"`       // 今日释放YY数量
	YesterdaySwapYY    string `json:"yesterday_swap_yy"`      // 昨日兑换YY数量
	YesterdayBurnYY    string `json:"yesterday_burn_yy"`      // 昨日销毁YY数量
	HistoricalBurnYY   string `json:"historical_burn_yy"`     // 历史YY销毁数量
	HistoricalBurnJU   string `json:"historical_burn_ju"`     // 历史JU销毁数量
	HistoricalBurnSZPN string `json:"historical_burn_szpn"`   // 历史SZPN销毁数量
	TotalYYSwapFeeUSDT string `json:"total_yy_swap_fee_usdt"` // YY兑换手续费总计(USDT)

	TopTeam24hStats []*TopTeam24hStats `json:"top_team_24h_stats"` // 顶级团队24小时质押/提现（仅2个顶级领导人团队）
}

// TopTeam24hStats 顶级团队24小时统计
type TopTeam24hStats struct {
	TeamId         int64  `json:"team_id"`
	TeamName       string `json:"team_name"`
	StakeAmount    string `json:"stake_amount"`
	WithdrawAmount string `json:"withdraw_amount"`
}
