package model

type TeamOverview struct {
	TeamTotalUserCount       int    `json:"team_total_user_count"`
	TeamUserCount            int    `json:"team_user_count"`
	TeamTodayNewUserCount    int    `json:"team_today_new_user_count"`
	DirectCount              int    `json:"direct_count"`
	RewardLevel              int    `json:"reward_level"`
	BigTeamCount             int    `json:"big_team_count"`
	SmallTeamSum             int    `json:"small_team_sum"`
	BigTeamPerformance       string `json:"big_team_performance"`
	SmallTeamPerformance     string `json:"small_team_performance"`
	NetworkDistrictUserCount int    `json:"network_district_user_count"`
	NetworkDistrictPerformance string `json:"network_district_performance"`
	MyDistrictShareRatio     string `json:"my_district_share_ratio"`
	TeamPerformance          string `json:"team_performance"`
	TeamTodayMint            string `json:"team_today_mint"`
	TeamYesterdayMint        string `json:"team_yesterday_mint"`
	TeamYYSum                string `json:"team_yy_sum"`
	TodayNewPerformance      string `json:"today_new_performance"`
	YesterdayNewPerformance  string `json:"yesterday_new_performance"`
	LeadershipReward         string `json:"leadership_reward"`
	LeadershipWeightReward   string `json:"leadership_weight_reward"`
	TeamReward               string `json:"team_reward"`
}

type TeamDirectItem struct {
	Address             string `json:"address"`
	PersonalPerformance string `json:"personal_performance"`
	TeamPerformance     string `json:"team_performance"`
	TeamNodePerformance string `json:"team_node_performance"`
	TeamTriplePerformance string `json:"team_triple_performance"`
	CreatedAt           string `json:"created_at"` // 用户创建时间
}

type LastBizDayRewardSummary struct {
	SettlementTime   string `json:"settlement_time"`
	GroupPerformance string `json:"group_performance"`
	LeaderNew        string `json:"leader_new"`
	AllReward        string `json:"all_reward"`
}

type TeamAllTimeStats struct {
	PersonalTriple     string `json:"personal_triple"`      // 个人实际购买三倍券
	TeamTriple         string `json:"team_triple"`          // 团队实际购买三倍券
	PersonalGiftTriple string `json:"personal_gift_triple"` // 个人赠送三倍券
	TeamGiftTriple     string `json:"team_gift_triple"`     // 团队赠送三倍券
	PersonalMint       string `json:"personal_mint"`
	TeamMint           string `json:"team_mint"`
}
