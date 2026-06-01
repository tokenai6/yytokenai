package entity

import (
	"XWFrame/internal/frame/model"
	"time"
)

// NodePurchaseRecordEntity 节点购买记录实体
type NodePurchaseRecordEntity struct {
	model.BaseEntity
	UserID          int64     `json:"user_id" g:"not null;index;comment:用户ID"`
	UserAddress     string    `json:"user_address" g:"size:100;not null;comment:用户地址"`
	FromAddress     string    `json:"from_address" g:"size:100;not null;comment:充值来源地址"`
	Amount          string    `json:"amount" g:"size:50;not null;comment:充值金额"`
	NodeLevel       string    `json:"node_level" g:"size:50;comment:节点等级"`
	PowerMultiplier float64   `json:"power_multiplier" g:"type:decimal(10,2);comment:算力倍数"`
	PowerValue      int64     `json:"power_value" g:"comment:算力值"`
	TransactionHash string    `json:"transaction_hash" g:"size:100;uniqueIndex;not null;comment:交易哈希"`
	TransactionTime time.Time `json:"transaction_time" g:"comment:交易完成时间"`
	EventID         string    `json:"event_id" g:"size:100;uniqueIndex;not null;comment:事件ID"`
	TransactionID   string    `json:"transaction_id" g:"size:100;not null;comment:交易ID"`
	BlockNumber     int64     `json:"block_number" g:"comment:区块号"`
}
