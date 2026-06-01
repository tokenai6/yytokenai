package reward

import (
	"time"

	"github.com/shopspring/decimal"
)

// SettlementDetailEntity 结算明细表实体
type SettlementDetailEntity struct {
	Id             int64           `json:"id" orm:"id,primary"`
	SettlementID   int64           `json:"settlement_id" orm:"settlement_id"`
	UserID         int64           `json:"user_id" orm:"user_id"`
	SettlementTime time.Time       `json:"settlement_time" orm:"record_time"`
	SourceTable    string          `json:"source_table" orm:"source_table"`
	SourceID       int64           `json:"source_id" orm:"source_id"`
	Amount         decimal.Decimal `json:"amount" orm:"amount"`
	BusinessType   string          `json:"business_type" orm:"business_type"`
	Status         int             `json:"status" orm:"status"`
	CreatedAt      time.Time       `json:"created_at" orm:"created_at"`
}
