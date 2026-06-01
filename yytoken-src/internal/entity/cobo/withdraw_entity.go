package cobo

import (
	"time"

	"github.com/shopspring/decimal"
)

// WithdrawStatus 提现状态
const (
	WithdrawStatusPending    = 0 // 待审核
	WithdrawStatusApproved   = 1 // 审核通过
	WithdrawStatusRejected   = 2 // 审核拒绝
	WithdrawStatusProcessing = 3 // 处理中
	WithdrawStatusSuccess    = 4 // 成功
	WithdrawStatusFailed     = 5 // 失败
)

// RiskLevel 风控等级
const (
	RiskLevelNormal  = 0 // 正常
	RiskLevelWarning = 1 // 警告
	RiskLevelHigh    = 2 // 高风险
)

// WithdrawEntity Cobo提现申请实体
type WithdrawEntity struct {
	ID            int64           `json:"id" orm:"id"`
	UserID        int64           `json:"user_id" orm:"user_id"`
	OrderNo       string          `json:"order_no" orm:"order_no"`               // 订单号
	Symbol        string          `json:"symbol" orm:"symbol"`                   // 币种
	Amount        decimal.Decimal `json:"amount" orm:"amount"`                   // 金额
	FeeRate       decimal.Decimal `json:"fee_rate" orm:"fee_rate"`               // 手续费费率（如 0.1 = 10%）
	FeeAmount     decimal.Decimal `json:"fee_amount" orm:"fee_amount"`           // 手续费金额
	ActualAmount  decimal.Decimal `json:"actual_amount" orm:"actual_amount"`     // 实际到账金额（扣除手续费与税款后）
	TaxAmount          decimal.Decimal `json:"tax_amount" orm:"tax_amount"`                    // 实际税收（USDT），转入税收接收方
	TaxDeductionAmount decimal.Decimal `json:"tax_deduction_amount" orm:"tax_deduction_amount"` // 本次税款被节点额度抵扣金额
	TaxYyaiUsdtAmount  decimal.Decimal `json:"tax_yyai_usdt_amount" orm:"tax_yyai_usdt_amount"`  // 税款中兑换YYAI的USDT等值
	TaxYyaiAmount      decimal.Decimal `json:"tax_yyai_amount" orm:"tax_yyai_amount"`            // 兑换发放的YYAI数量
	TaxRecipient  string          `json:"tax_recipient" orm:"tax_recipient"`     // 税款接收方钱包地址（小写）
	ToAddress     string          `json:"to_address" orm:"to_address"`           // 目标地址
	Chain         string          `json:"chain" orm:"chain"`                     // 链
	Status        int             `json:"status" orm:"status"`                   // 状态
	AutoApproved  bool            `json:"auto_approved" orm:"auto_approved"`     // 是否自动审核
	AuditBy       int64           `json:"audit_by" orm:"audit_by"`               // 审核人ID
	AuditAt       *time.Time      `json:"audit_at" orm:"audit_at"`               // 审核时间
	AuditRemark   string          `json:"audit_remark" orm:"audit_remark"`       // 审核备注
	CoboRequestID string          `json:"cobo_request_id" orm:"cobo_request_id"` // Cobo请求ID
	CoboResponse  string          `json:"cobo_response" orm:"cobo_response"`     // Cobo响应
	TxHash        string          `json:"tx_hash" orm:"tx_hash"`                 // 链上交易哈希
	WebhookID     string          `json:"webhook_id" orm:"webhook_id"`           // Webhook ID（幂等键）
	RiskLevel     int             `json:"risk_level" orm:"risk_level"`           // 风控等级
	RiskReason    string          `json:"risk_reason" orm:"risk_reason"`         // 风控原因
	CreatedAt     time.Time       `json:"created_at" orm:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at" orm:"updated_at"`
}

// TableName 表名
func (e *WithdrawEntity) TableName() string {
	return "cobo_withdraw_request"
}

// IsPending 是否待审核
func (e *WithdrawEntity) IsPending() bool {
	return e.Status == WithdrawStatusPending
}

// IsApproved 是否已审核通过
func (e *WithdrawEntity) IsApproved() bool {
	return e.Status == WithdrawStatusApproved || e.Status == WithdrawStatusProcessing || e.Status == WithdrawStatusSuccess
}

// IsFinal 是否最终状态
func (e *WithdrawEntity) IsFinal() bool {
	return e.Status == WithdrawStatusSuccess || e.Status == WithdrawStatusFailed || e.Status == WithdrawStatusRejected
}
