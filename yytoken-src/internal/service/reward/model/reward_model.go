package model

type RewardOverview struct {
	YesterdayTotal    string `json:"yesterday_total"`
	YesterdayStatic   string `json:"yesterday_static"`
	YesterdayDistrict string `json:"yesterday_district"`
	YesterdayReferral string `json:"yesterday_referral"`
	YesterdayWeighted string `json:"yesterday_weighted"`
	YesterdayNode     string `json:"yesterday_node"`
	TotalEarnings     string `json:"total_earnings"`
}

type RewardLogItem struct {
	Date    string `json:"date"`
	Amount  string `json:"amount"`
	Actual  string `json:"actual"`
	OrderNo string `json:"order_no"`
	Extra   string `json:"extra"`
}
