package model

type CreateStakeReq struct {
	UserID    int64
	Amount    string
	RequestID string
}

type CreateStakeRes struct {
	OrderID     int64  `json:"order_id"`
	Amount      string `json:"amount"`
	Status      int    `json:"status"`
	SourceType  int    `json:"source_type"`
	CreatedAt   string `json:"created_at"`
	BalanceLeft string `json:"balance_left"`
}

type StakeOverviewRes struct {
	TotalStakeAmount  string `json:"total_stake_amount"`
	TotalRewardEarned string `json:"total_reward_earned"`
	RewardLimit       string `json:"reward_limit"`
	RewardProgress    string `json:"reward_progress"`
	IsCapped          bool   `json:"is_capped"`
	ActiveOrderCount  int    `json:"active_order_count"`
	CappedOrders      int    `json:"capped_orders"`
}

type StakeListReq struct {
	UserID     int64
	Page       int
	PageSize   int
	Status     int
	SourceType int
}

type StakeOrderItem struct {
	ID          int64  `json:"id"`
	Amount      string `json:"amount"`
	Status      int    `json:"status"`
	SourceType  int    `json:"source_type"`
	TotalReward string `json:"total_reward"`
	RewardLimit string `json:"reward_limit"`
	CreatedAt   string `json:"created_at"`
	IsGift      int    `json:"is_gift"`
}

type StakeListRes struct {
	List     []*StakeOrderItem `json:"list"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

type ActiveStakeQuoteReq struct {
	UserID int64
}

type ActiveStakeQuoteRes struct {
	HasActive   bool   `json:"has_active"`
	Principal   string `json:"principal,omitempty"`
	RestQuota   string `json:"rest_quota,omitempty"`
	TotalReward string `json:"total_reward,omitempty"`
}
