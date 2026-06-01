package entity

import (
	"time"

	"github.com/shopspring/decimal"
)

// StockPriceEntity 股票价格历史实体
type StockPriceEntity struct {
	Id           int64           `json:"id" orm:"id,primary"`
	Symbol       string          `json:"symbol" orm:"symbol"`             // 股票代码，如 YYAI
	Price        decimal.Decimal `json:"price" orm:"price"`               // 当前价格
	Currency     string          `json:"currency" orm:"currency"`         // 货币单位，如 USD
	OpenPrice    decimal.Decimal `json:"open_price" orm:"open_price"`     // 开盘价
	HighPrice    decimal.Decimal `json:"high_price" orm:"high_price"`     // 最高价
	LowPrice     decimal.Decimal `json:"low_price" orm:"low_price"`       // 最低价
	PrevClose    decimal.Decimal `json:"prev_close" orm:"prev_close"`     // 昨收价
	Volume       int64           `json:"volume" orm:"volume"`             // 成交量
	PriceTime    time.Time       `json:"price_time" orm:"price_time"`     // 价格时间
	Source       string          `json:"source" orm:"source"`             // 数据来源，如 yahoo_finance
	AdminId      int64           `json:"admin_id" orm:"admin_id"`         // 操作管理员ID
	CreatedAt    time.Time       `json:"created_at" orm:"created_at"`     // 创建时间
}

// TableName 指定表名
func (StockPriceEntity) TableName() string {
	return "stock_price"
}
