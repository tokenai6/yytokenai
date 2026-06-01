package entity

import (
	"time"

	"github.com/shopspring/decimal"
)

// ApgBurnRecord APG销毁记录实体
type ApgBurnRecord struct {
	Id              int64           `json:"id" orm:"id,primary"`                     // 主键ID
	FromAddress     string          `json:"from_address" orm:"from_address"`         // 发起者地址
	BurnAmount      decimal.Decimal `json:"burn_amount" orm:"burn_amount"`           // 销毁数量（用于节点分红）
	MarketingAmount decimal.Decimal `json:"marketing_amount" orm:"marketing_amount"` // 市场奖励
	OperationAmount decimal.Decimal `json:"operation_amount" orm:"operation_amount"` // 运营奖励
	TxHash          string          `json:"tx_hash" orm:"tx_hash"`                   // 交易哈希
	BlockNumber     int64           `json:"block_number" orm:"block_number"`         // 区块号
	EventIndex      int             `json:"event_index" orm:"event_index"`           // 事件索引
	BlockTimestamp  time.Time       `json:"block_timestamp" orm:"block_timestamp"`   // 区块时间
	CreatedAt       time.Time       `json:"created_at" orm:"created_at"`             // 创建时间
}
