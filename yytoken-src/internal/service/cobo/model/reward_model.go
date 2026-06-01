package model

// RewardType 奖励类型
const (
	RewardTypeDirect           = "direct"             // 直推奖励
	RewardTypeIndirect         = "indirect"           // 间推奖励
	RewardTypeTeam             = "team"               // 团队奖励
	RewardTypeLeadership       = "leadership"         // 领导奖励（极差+加权合并）
	RewardTypeLeadershipDiff   = "leadership_diff"    // 领导极差（等级奖励）
	RewardTypeLeadershipWeight = "leadership_weight"  // 领导加权（等级分红）
	RewardTypeMatchReward      = "match_reward"       // 拼团收益（按场次聚合，不含本金）
	RewardTypeUSStock          = "us_stock"           // US Stock Reward 每日结算收益
)

// GetRewardRecordsReq 获取奖励记录请求
type GetRewardRecordsReq struct {
	UserID     int64  `json:"user_id"`     // 当前用户ID（从JWT获取）
	Page       int    `json:"page"`        // 页码，默认1
	PageSize   int    `json:"page_size"`   // 每页数量，默认20
	RewardType string `json:"reward_type"` // 奖励类型筛选，direct/indirect/founder_fee_dividend
}

// RewardRecord 奖励记录
type RewardRecord struct {
	ID                  int64  `json:"id"`                    // 记录ID
	RewardType          string `json:"reward_type"`           // 奖励类型：direct/indirect/founder_fee_dividend
	RewardTypeText      string `json:"reward_type_text"`      // 奖励类型文案（按locale）
	Amount              string `json:"amount"`                // 奖励金额
	Symbol              string `json:"symbol"`                // 币种
	SourceUserID        int64  `json:"source_user_id"`        // 触发奖励的下级用户ID
	SourceWalletAddress string `json:"source_wallet_address"` // 触发奖励的下级钱包地址
	PurchasePackageNo   string `json:"purchase_package_no"`   // 关联购买包号
	PurchaseAmount      string `json:"purchase_amount"`       // 触发奖励的购买金额
	RewardRate          string `json:"reward_rate"`           // 奖励比例
	CreatedAt           int64  `json:"created_at"`            // 奖励时间（Unix时间戳）
}

// GetRewardRecordsRes 获取奖励记录响应
type GetRewardRecordsRes struct {
	Page     int             `json:"page"`      // 页码
	PageSize int             `json:"page_size"` // 每页数量
	Total    int64           `json:"total"`     // 总数
	Pages    int             `json:"pages"`     // 总页数
	List     []*RewardRecord `json:"list"`      // 奖励记录列表
}
