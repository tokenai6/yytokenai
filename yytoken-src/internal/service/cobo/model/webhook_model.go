package model

import "strings"

// WebhookPayload Cobo Webhook 请求体
type WebhookPayload struct {
	ID        string          `json:"id"`        // Webhook ID（幂等键）
	Type      string          `json:"type"`      // 事件类型
	Status    string          `json:"status"`    // 状态
	Data      WebhookData     `json:"data"`      // 数据
	Timestamp int64           `json:"timestamp"` // 时间戳
}

// WebhookData Webhook 数据详情
type WebhookData struct {
	TxHash        string `json:"tx_hash"`         // 交易哈希
	ToAddress     string `json:"to_address"`      // 接收地址（Cobo地址）
	FromAddress   string `json:"from_address"`    // 发送地址（用户地址）
	Asset         string `json:"asset"`           // 资产符号
	Amount        string `json:"amount"`          // 金额
	Confirmations int    `json:"confirmations"`   // 确认数
	BlockNumber   int64  `json:"block_number"`    // 区块高度
	WalletID      string `json:"wallet_id"`       // 钱包ID
	RequestID     string `json:"request_id"`      // 请求ID（提现用）
}

// WebhookResponse Webhook 响应
type WebhookResponse struct {
	Code    int    `json:"code"`    // 0=成功
	Message string `json:"message"` // 错误信息
}

// IsWithdrawEvent 是否为提现事件
func (p *WebhookPayload) IsWithdrawEvent() bool {
	// Cobo 交易事件类型: transaction.created, transaction.updated, transaction.completed, transaction.failed
	t := strings.ToLower(p.Type)
	if t == "transaction.updated" || t == "transaction.completed" || t == "transaction.failed" {
		return true
	}
	return strings.Contains(t, "withdraw")
}

// IsRechargeEvent 是否为充值事件
func (p *WebhookPayload) IsRechargeEvent() bool {
	// Cobo 充值事件
	t := strings.ToLower(p.Type)
	if t == "deposit.completed" || t == "deposit.confirming" {
		return true
	}
	return strings.Contains(t, "deposit")
}
