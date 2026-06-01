package entity

import (
	"time"

	"github.com/shopspring/decimal"
)

// SwapRecordEntity Swap兑换记录实体
type SwapRecordEntity struct {
	Id         int64           `json:"id" orm:"id,primary"`
	UserID     int64           `json:"user_id" orm:"user_id"`
	FromSymbol string          `json:"from_symbol" orm:"from_symbol"`
	ToSymbol   string          `json:"to_symbol" orm:"to_symbol"`
	FromAmount decimal.Decimal `json:"from_amount" orm:"from_amount"`
	ToAmount   decimal.Decimal `json:"to_amount" orm:"to_amount"`
	Fee        decimal.Decimal `json:"fee" orm:"fee"`
	Price      decimal.Decimal `json:"price" orm:"price"`
	RequestID   string          `json:"request_id" orm:"request_id"`
	BurnTxHash  string          `json:"burn_tx_hash" orm:"burn_tx_hash"`
	CreatedAt   time.Time       `json:"created_at" orm:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" orm:"updated_at"`
}

// TableName 指定表名
func (SwapRecordEntity) TableName() string {
	return "swap_record"
}
