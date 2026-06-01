package entity

import (
	"XWFrame/internal/frame/model"

	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

// ExchangeRecordEntity 兑换记录实体
type ExchangeRecordEntity struct {
	model.BaseEntity
	UserId            int64           `json:"userId"`
	OrderNo           string          `json:"orderNo"`
	ExchangeType      int             `json:"exchangeType"`
	FromAccountTypeId int64           `json:"fromAccountTypeId"`
	FromSymbol        string          `json:"fromSymbol"`
	FromAmount        decimal.Decimal `json:"fromAmount"`
	ToAccountTypeId   int64           `json:"toAccountTypeId"`
	ToSymbol          string          `json:"toSymbol"`
	ToAmount          decimal.Decimal `json:"toAmount"`
	ExchangeRate      decimal.Decimal `json:"exchangeRate"`
	Fee               decimal.Decimal `json:"fee"`
	FeeRate           decimal.Decimal `json:"feeRate"`
	Status            int             `json:"status"`
	Remark            string          `json:"remark"`
	CreatedAt         *gtime.Time     `json:"createdAt"`
	UpdatedAt         *gtime.Time     `json:"updatedAt"`
}
