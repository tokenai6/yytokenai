package model

type CurrentSessionRes struct {
	ID          int64  `json:"id"`
	SessionDate string `json:"session_date"`
	SessionName string `json:"session_name"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	Status      int    `json:"status"`
}

type SessionTimeRangeItem struct {
	SessionName string `json:"session_name"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
}

type SessionTimeRangesRes struct {
	Timezone string                  `json:"timezone"`
	List     []*SessionTimeRangeItem `json:"list"`
}

type SessionItem struct {
	SessionID          int64  `json:"session_id"`
	SessionDate        string `json:"session_date"`
	SessionName        string `json:"session_name"`
	StartTime          string `json:"start_time"`
	EndTime            string `json:"end_time"`
	Status             int    `json:"status"`
	TotalOrders        int    `json:"total_orders"`
	TotalGroups        int    `json:"total_groups"`
	TotalFlowUserCount int    `json:"total_flow_user_count"`
}

type GetSessionListReq struct {
	Page     int
	PageSize int
}

type GetSessionListRes struct {
	List     []*SessionItem `json:"list"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

type GroupMemberItem struct {
	UserID        int64  `json:"user_id"`
	WalletAddress string `json:"wallet_address"`
	IsWinner      bool   `json:"is_winner"`
	RewardAmount  string `json:"reward_amount"`
}

type GroupDetailItem struct {
	GroupID            int64              `json:"group_id"`
	GroupNo            string             `json:"group_no"`
	Status             int                `json:"status"`
	MemberCount        int                `json:"member_count"`
	LoserUserID        int64              `json:"loser_user_id"`
	LoserWalletAddress string             `json:"loser_wallet_address"`
	TotalPrincipal     string             `json:"total_principal"`
	WinnerRewardTotal  string             `json:"winner_reward_total"`
	DrawnAt            string             `json:"drawn_at"`
	Members            []*GroupMemberItem `json:"members"`
}

type GetSessionDetailReq struct {
	SessionID int64
}

type UserSessionItem struct {
	UserID        int64  `json:"user_id"`
	WalletAddress string `json:"wallet_address"`
	OrderCount    int    `json:"order_count"`
	GroupCount    int    `json:"group_count"`
	TotalAmount   string `json:"total_amount"`
	TotalReward   string `json:"total_reward"`
	TotalRefund   string `json:"total_refund"`
	WinCount      int    `json:"win_count"`
	LoseCount     int    `json:"lose_count"`
}

type GetSessionDetailRes struct {
	Session *SessionItem       `json:"session"`
	Users   []*UserSessionItem `json:"users"`
}

type GetSessionLoserDetailReq struct {
	SessionID int64
}

type LoserDetailItem struct {
	LoserAddress string `json:"loser_address"`
	LoseCount    int    `json:"lose_count"`
	CreatedAt    string `json:"created_at"`
}

type GetSessionLoserDetailRes struct {
	List []*LoserDetailItem `json:"list"`
}

type JoinGroupReq struct {
	UserID       int64
	SessionID    int64
	TicketSymbol string
	Count        int
	RequestID    string
	Locale       string
}

type JoinOrderItem struct {
	OrderNo      string `json:"order_no"`
	Amount       string `json:"amount"`
	TicketSymbol string `json:"ticket_symbol"`
	TicketAmount string `json:"ticket_amount"`
	Status       int    `json:"status"`
}

type JoinGroupRes struct {
	SessionID     int64            `json:"session_id"`
	Orders        []*JoinOrderItem `json:"orders"`
	TotalUSDT     string           `json:"total_usdt"`
	TotalTicket   string           `json:"total_ticket"`
	UsdtLeft      string           `json:"usdt_left"`
	TicketLeft    string           `json:"ticket_left"`
	TicketSymbol  string           `json:"ticket_symbol"`
	TicketPrice   string           `json:"ticket_price"`
	TicketUSDTVal string           `json:"ticket_usdt_value"`
}

type GetMyOrdersReq struct {
	UserID    int64
	SessionID int64
	Status    int
	Page      int
	PageSize  int
}

type MyOrderItem struct {
	SessionID      int64 `json:"session_id"`
	Status         int   `json:"status"`
	TotalOrderCnt  int   `json:"total_order_count"`
	WinnerOrderCnt int   `json:"win_count"`
}

type GetMyOrdersRes struct {
	List     []*MyOrderItem `json:"list"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

type UserLeadershipLevel struct {
	BizDate    string `json:"biz_date"`
	UserID     int64  `json:"user_id"`
	LevelKey   string `json:"level_key"`
	LevelName  string `json:"level_name"`
	TeamShares int64  `json:"team_shares"`
	RewardRate string `json:"reward_rate"`
}

type UserLeadershipRewardPreview struct {
	BizDate         string `json:"biz_date"`
	UserID          int64  `json:"user_id"`
	RewardLevel     int    `json:"reward_level"`
	LevelKey        string `json:"level_key"`
	LevelName       string `json:"level_name"`
	TeamShares      int64  `json:"team_shares"`
	TeamAmount      string `json:"team_amount"`
	LevelRate       string `json:"level_rate"`
	ChildMaxRate    string `json:"child_max_rate"`
	DiffRate        string `json:"diff_rate"`
	LoserPoolAmount string `json:"loser_pool_amount"`
	TotalDiffRate   string `json:"total_diff_rate"`
	EstimatedReward string `json:"estimated_reward"`
}

type HistoryStatsItem struct {
	SessionID    int64  `json:"session_id"`
	SessionDate  string `json:"session_date"`
	SessionName  string `json:"session_name"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	TotalOrders  int    `json:"total_orders"`
	WinnerCount  int    `json:"winner_count"`
	LoserCount   int    `json:"loser_count"`
	FlowOutCount int    `json:"flow_out_count"`
}

type GetHistoryStatsReq struct {
	Page     int
	PageSize int
}

type GetHistoryStatsRes struct {
	List     []*HistoryStatsItem `json:"list"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

type LatestJoinItem struct {
	UserID       int64  `json:"user_id"`
	Address      string `json:"address"`
	Amount       string `json:"amount"`
	TicketSymbol string `json:"ticket_symbol"`
	TicketAmount string `json:"ticket_amount"`
	Count        int    `json:"count"`
	CreatedAt    string `json:"created_at"`
}

type GetLatestJoinsReq struct{}

type GetLatestJoinsRes struct {
	SessionID   int64             `json:"session_id"`
	SessionName string            `json:"session_name"`
	List        []*LatestJoinItem `json:"list"`
}

type TopJoinerItem struct {
	Wallet string `json:"wallet"`
	Count  int    `json:"count"`
}

type GetTopJoinersReq struct {
	Top int
}

type GetTopJoinersRes struct {
	BizDate string           `json:"biz_date"`
	List    []*TopJoinerItem `json:"list"`
}

type GetParticipationStatsReq struct {
	UserID int64
}

type GetParticipationStatsRes struct {
	MyCount   int64 `json:"my_count"`
	TeamCount int64 `json:"team_count"`
}

type GetCurrentParticipationStatsReq struct {
	UserID    int64
	SessionID int64
}

type GetCurrentParticipationStatsRes struct {
	SessionType     string `json:"session_type"`
	TotalOrderCount int64  `json:"total_order_count"`
	TeamOrderCount  int64  `json:"team_order_count"`
}

type GetBalanceChangesReq struct {
	UserID   int64
	Symbol   string
	BizType  string // all / winner / loser_release
	Locale   string
	Page     int
	PageSize int
}

type BalanceChangeItem struct {
	ID             int64  `json:"id"`
	Symbol         string `json:"symbol"`
	ChangeType     string `json:"change_type"`
	Amount         string `json:"amount"`
	BeforeBalance  string `json:"before_balance"`
	AfterBalance   string `json:"after_balance"`
	RelatedOrderNo string `json:"related_order_no"`
	RelatedID      int64  `json:"related_id"`
	Remark         string `json:"remark"`
	CreatedAt      string `json:"created_at"`
}

type GetBalanceChangesRes struct {
	List     []*BalanceChangeItem `json:"list"`
	Total    int                  `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

type LeadershipLevelConfigItem struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	DailyShares  int64  `json:"daily_shares"`
	RewardRate   string `json:"reward_rate"`
	TicketWeight int64  `json:"ticket_weight"`
}

type LeadershipLevelsConfigRes struct {
	Levels             []*LeadershipLevelConfigItem `json:"levels"`
	LeadershipPoolRate string                       `json:"leadership_pool_rate"`
	TicketWeightRate   string                       `json:"ticket_weight_rate"`
}

type GetTodayRewardReq struct {
	UserID int64
	Date   string
}

type GetTodayRewardRes struct {
	Date                    string `json:"date"`
	SettledRewardUSDT       string `json:"settled_reward_usdt"`
	ReleaseYYUSDTValue      string `json:"release_yy_usdt_value"`
	TotalSettledRewardUSDT  string `json:"total_settled_reward_usdt"`
	TotalReleaseYYUSDTValue string `json:"total_release_yy_usdt_value"`
}

type LoserOutputItem struct {
	Amount    string `json:"amount"`     // YY 数量
	USDTValue string `json:"usdt_value"` // USDT 价值
}

type GetLoserOutputStatsReq struct {
	UserID int64
}

type GetLoserOutputStatsRes struct {
	DailyOutput     *LoserOutputItem `json:"daily_output"`     // 日产出（设计释放量）
	PendingOutput   *LoserOutputItem `json:"pending_output"`   // 待产出
	CompletedOutput *LoserOutputItem `json:"completed_output"` // 已产出
}

type LoserReleaseLogItem struct {
	ID               int64  `json:"id"`
	CompensationID   int64  `json:"compensation_id"`
	OrderID          int64  `json:"order_id"`
	ReleaseDate      string `json:"release_date"`
	ReleaseAmount    string `json:"release_amount"`
	ReleaseUSDTValue string `json:"release_usdt_value"`
	CreatedAt        string `json:"created_at"`
}

type GetLoserReleaseLogReq struct {
	UserID   int64
	Page     int
	PageSize int
}

type GetLoserReleaseLogRes struct {
	List     []*LoserReleaseLogItem `json:"list"`
	Total    int                    `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}
