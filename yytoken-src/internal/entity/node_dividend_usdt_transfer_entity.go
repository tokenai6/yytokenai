package entity

import (
	"time"

	"github.com/shopspring/decimal"
)

// NodeDividendUsdtTransfer 节点分红USDT转账记录实体
type NodeDividendUsdtTransfer struct {
	Id             int64           `json:"id" orm:"id,primary"`
	FromAddress    string          `json:"from_address" orm:"from_address"`       // 发起地址（国库合约）
	ToAddress      string          `json:"to_address" orm:"to_address"`           // 接收地址（节点收税地址）
	Amount         decimal.Decimal `json:"amount" orm:"amount"`                   // 转账金额（USDT）
	TxHash         string          `json:"tx_hash" orm:"tx_hash"`                 // 交易哈希
	BlockNumber    int64           `json:"block_number" orm:"block_number"`       // 区块号
	EventIndex     int             `json:"event_index" orm:"event_index"`         // 事件索引
	BlockTimestamp time.Time       `json:"block_timestamp" orm:"block_timestamp"` // 区块时间
	CreatedAt      time.Time       `json:"created_at" orm:"created_at"`           // 创建时间
}
