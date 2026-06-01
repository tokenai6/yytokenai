package reward

import (
	"time"

	"github.com/shopspring/decimal"
)

// SettlementRecordEntity 结算表实体
type SettlementRecordEntity struct {
	Id               int64           `json:"id" orm:"id,primary"`
	UserID           int64           `json:"user_id" orm:"user_id"`
	SettlementTime   time.Time       `json:"settlement_time" orm:"record_time"`
	SettlementAmount decimal.Decimal `json:"settlement_amount" orm:"settlement_amount"`
	DetailCount      int             `json:"detail_count" orm:"detail_count"`
	CreatedAt        time.Time       `json:"created_at" orm:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at" orm:"updated_at"`
}
