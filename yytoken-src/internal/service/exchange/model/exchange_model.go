package model

import (
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

// ExchangeRecord 兑换记录
type ExchangeRecord struct {
	Id                int64           `json:"id"`
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

// ExchangeReq 兑换请求
type ExchangeReq struct {
	UserId            int64           `json:"userId" v:"required#用户ID不能为空"`
	FromAccountTypeId int64           `json:"fromAccountTypeId" v:"required#兑出账户类型ID不能为空"`
	ToAccountTypeId   int64           `json:"toAccountTypeId" v:"required#兑入账户类型ID不能为空"`
	FromAmount        decimal.Decimal `json:"fromAmount" v:"required#兑出金额不能为空"`
}

// ExchangeListReq 兑换列表请求
type ExchangeListReq struct {
	UserId       int64 `json:"userId"`
	ExchangeType *int  `json:"exchangeType"`
	Page         int   `json:"page" v:"required|min:1#页码不能为空|页码必须大于0"`
	PageSize     int   `json:"pageSize" v:"required|min:1|max:100#每页数量不能为空|每页数量必须大于0|每页数量不能超过100"`
}

// ExchangeListRes 兑换列表响应
type ExchangeListRes struct {
	List     []*ExchangeRecord `json:"list"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
}

// ExchangeRateRes 兑换汇率响应
type ExchangeRateRes struct {
	FromSymbol   string          `json:"fromSymbol"`
	ToSymbol     string          `json:"toSymbol"`
	ExchangeRate decimal.Decimal `json:"exchangeRate"`
	FeeRate      decimal.Decimal `json:"feeRate"`
}
