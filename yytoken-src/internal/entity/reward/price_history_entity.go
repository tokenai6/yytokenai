package reward

import (
	"time"

	"github.com/shopspring/decimal"
)

// PriceHistoryEntity 价格历史实体
type PriceHistoryEntity struct {
	Id           int64           `json:"id" orm:"id,primary"`
	PriceTime    time.Time       `json:"price_time" orm:"price_time"`
	RexPrice     decimal.Decimal `json:"rex_price" orm:"rex_price"`
	ApgPrice     decimal.Decimal `json:"apg_price" orm:"apg_price"`
	ExchangeRate decimal.Decimal `json:"exchange_rate" orm:"exchange_rate"`
	Source       string          `json:"source" orm:"source"`
	CreatedAt    time.Time       `json:"created_at" orm:"created_at"`
}
