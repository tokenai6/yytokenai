package cobo

import (
	"time"

	"github.com/shopspring/decimal"
)

// NodePurchaseStatus 节点购买状态
const (
	NodeStatusRunning       = 1 // 运行中
	NodeStatusStaticExpired = 2 // 静态完成
	NodeStatusQuotaExpired  = 3 // 额度耗尽
	NodeStatusForceExpired  = 4 // 强制出局
)

// NodePurchaseEntity Cobo节点购买记录实体
type NodePurchaseEntity struct {
	ID                 int64           `json:"id" orm:"id"`
	UserID             int64           `json:"user_id" orm:"user_id"`
	PackageNo          string          `json:"package_no" orm:"package_no"`                       // 包号
	RequestID          string          `json:"request_id" orm:"request_id"`                       // 幂等请求ID
	NodeType           int             `json:"node_type" orm:"node_type"`                         // 节点类型 1-4
	Amount             decimal.Decimal `json:"amount" orm:"amount"`                               // 购买金额
	PowerValue         decimal.Decimal `json:"power_value" orm:"power_value"`                     // 算力值
	StakeAmount        decimal.Decimal `json:"stake_amount" orm:"stake_amount"`                   // 质押金额
	Status             int             `json:"status" orm:"status"`                               // 状态
	DirectRewardAmount decimal.Decimal `json:"direct_reward_amount" orm:"direct_reward_amount"`   // 直推奖励金额
	DirectRewardUserID int64           `json:"direct_reward_user_id" orm:"direct_reward_user_id"` // 获得直推奖励的用户ID
	TeamRewardTotal    decimal.Decimal `json:"team_reward_total" orm:"team_reward_total"`         // 团队奖励总计
	LeaderRewardTotal  decimal.Decimal `json:"leader_reward_total" orm:"leader_reward_total"`     // 领导奖励总计
	StartTime          time.Time       `json:"start_time" orm:"start_time"`
	ExpectedEndTime    *time.Time      `json:"expected_end_time" orm:"expected_end_time"`
	ActualEndTime      *time.Time      `json:"actual_end_time" orm:"actual_end_time"`
	CreatedAt          time.Time       `json:"created_at" orm:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at" orm:"updated_at"`
	// 赠送节点相关字段
	IsGift      int    `json:"is_gift" orm:"is_gift"`             // 是否赠送节点: 0-正常购买, 1-后台赠送
	GiftBy      int64  `json:"gift_by" orm:"gift_by"`             // 赠送操作人ID (admin_info.id)
	GiftRemark  string `json:"gift_remark" orm:"gift_remark"`     // 赠送备注
	Locale      string `json:"locale" orm:"locale"`               // 用户购买时的语言环境
}

// TableName 表名
func (e *NodePurchaseEntity) TableName() string {
	return "cobo_node_purchase"
}

// IsRunning 是否运行中
func (e *NodePurchaseEntity) IsRunning() bool {
	return e.Status == NodeStatusRunning
}
