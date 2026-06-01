package node

import (
	"time"
)

// NodeInfo 节点信息（C端返回模型，不包含敏感字段）
type NodeInfo struct {
	Id              int64     `json:"id"`
	NodeType        int       `json:"node_type"`        // 节点类型(用于下单)
	NodeLevel       string    `json:"node_level"`       // 节点等级
	Price           string    `json:"price"`            // 节点价格(USDT)
	Status          int       `json:"status"`           // 状态
	PowerMultiplier float64   `json:"power_multiplier"` // 算力倍数
	TotalShares     int64     `json:"total_shares"`     // 总份额
	SoldShares      int64     `json:"sold_shares"`      // 已销售份额
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// GetNodeInfoRes 获取节点信息响应
type GetNodeInfoRes struct {
	Nodes []NodeInfo `json:"nodes"`
}

// PurchaseRecordInfo 购买记录信息
type PurchaseRecordInfo struct {
	TransactionTime int64   `json:"transaction_time"` // 交易时间戳（秒）
	NodeType        int     `json:"node_type"`        // 节点类型
	Amount          string  `json:"amount"`           // 金额
	PowerMultiplier float64 `json:"power_multiplier"` // 算力倍数
	PowerValue      int64   `json:"power_value"`      // 算力值
	TransactionHash string  `json:"transaction_hash"` // 交易哈希
	IsGift          int     `json:"is_gift"`          // 是否赠送节点: 0-正常购买, 1-后台赠送
	GiftRemark      string  `json:"gift_remark"`      // 赠送备注
}

// GetUserPurchaseRecordsRes 获取用户购买记录响应
type GetUserPurchaseRecordsRes struct {
	Records []*PurchaseRecordInfo `json:"records"`
}

// ReferralUserStats 推荐用户统计
type ReferralUserStats struct {
	UserAddress  string    `json:"user_address"`  // 用户地址
	RegisterTime time.Time `json:"register_time"` // 注册时间
	TotalPower   int64     `json:"total_power"`   // 总算力
}

// GetReferralPurchaseStatsRes 获取推荐用户购买统计响应
type GetReferralPurchaseStatsRes struct {
	Users []*ReferralUserStats `json:"users"`
}
