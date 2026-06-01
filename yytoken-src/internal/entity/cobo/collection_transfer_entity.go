package cobo

import (
	"time"

	"github.com/shopspring/decimal"
)

// CollectionTransferEntity 归集转账记录实体
type CollectionTransferEntity struct {
	ID            int64           `json:"id" orm:"id"`
	TransactionID string          `json:"transaction_id" orm:"transaction_id"`
	TokenID       string          `json:"token_id" orm:"token_id"`
	Amount        decimal.Decimal `json:"amount" orm:"amount"`
	ToAddress     string          `json:"to_address" orm:"to_address"`
	TxHash        string          `json:"tx_hash" orm:"tx_hash"`
	Status        string          `json:"status" orm:"status"`
	TransferTime  *time.Time      `json:"transfer_time" orm:"transfer_time"`
	CreatedAt     time.Time       `json:"created_at" orm:"created_at"`
}

// TableName 表名
func (e *CollectionTransferEntity) TableName() string {
	return "cobo_collection_transfer"
}
