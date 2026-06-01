package cobo

import (
	"time"

	"github.com/shopspring/decimal"
)

// RechargeStatus 充值状态
const (
	RechargeStatusPending   = 0 // 待确认
	RechargeStatusConfirmed = 1 // 已确认
	RechargeStatusFailed    = 2 // 失败
	RechargeStatusRejected  = 3 // 非白名单充币合约，拒绝入账
)

// RechargeEntity Cobo充值记录实体
type RechargeEntity struct {
	ID             int64           `json:"id" orm:"id"`
	UserID         int64           `json:"user_id" orm:"user_id"`
	CoboAddress    string          `json:"cobo_address" orm:"cobo_address"`     // Cobo接收地址
	TxHash         string          `json:"tx_hash" orm:"tx_hash"`               // 链上交易哈希
	FromAddress    string          `json:"from_address" orm:"from_address"`     // 用户打款地址
	Symbol         string          `json:"symbol" orm:"symbol"`                 // 币种
	Amount         decimal.Decimal `json:"amount" orm:"amount"`                 // 金额
	Status         int             `json:"status" orm:"status"`                 // 状态
	Confirmations  int             `json:"confirmations" orm:"confirmations"`   // 确认数
	WebhookID      string          `json:"webhook_id" orm:"webhook_id"`         // Webhook ID（幂等）
	WebhookData    string          `json:"webhook_data" orm:"webhook_data"`     // 原始webhook数据(JSON)
	ChainID        string          `json:"chain_id" orm:"chain_id"`             // 链ID
	LogIndex       int64           `json:"log_index" orm:"log_index"`           // 事件日志索引
	ConfirmedAt    *time.Time      `json:"confirmed_at" orm:"confirmed_at"`     // 确认时间
	CreatedAt      time.Time       `json:"created_at" orm:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at" orm:"updated_at"`
}

// TableName 表名
func (e *RechargeEntity) TableName() string {
	return "cobo_recharge_record"
}

// IsConfirmed 是否已确认
func (e *RechargeEntity) IsConfirmed() bool {
	return e.Status == RechargeStatusConfirmed
}

// IsPending 是否待确认
func (e *RechargeEntity) IsPending() bool {
	return e.Status == RechargeStatusPending
}
