package model

import (
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

// RechargeRecord 充值记录
type RechargeRecord struct {
	Id              int64           `json:"id"`
	UserId          int64           `json:"userId"`
	AccountTypeId   int64           `json:"accountTypeId"`
	Symbol          string          `json:"symbol"`
	OrderNo         string          `json:"orderNo"`
	RechargeAddress string          `json:"rechargeAddress"`
	ChainType       string          `json:"chainType"`
	Amount          decimal.Decimal `json:"amount"`
	TxHash          string          `json:"txHash"`
	BlockNumber     int64           `json:"blockNumber"`
	Confirmations   int             `json:"confirmations"`
	Status          int             `json:"status"`
	Remark          string          `json:"remark"`
	ConfirmedAt     *gtime.Time     `json:"confirmedAt"`
	CreatedAt       *gtime.Time     `json:"createdAt"`
	UpdatedAt       *gtime.Time     `json:"updatedAt"`
}

// RechargeReq 充值请求
type RechargeReq struct {
	UserId          int64           `json:"userId" v:"required#用户ID不能为空"`
	AccountTypeId   int64           `json:"accountTypeId" v:"required#账户类型ID不能为空"`
	RechargeAddress string          `json:"rechargeAddress" v:"required#充值地址不能为空"`
	ChainType       string          `json:"chainType" v:"required#链类型不能为空"`
	Amount          decimal.Decimal `json:"amount" v:"required#充值金额不能为空"`
	TxHash          string          `json:"txHash" v:"required#交易哈希不能为空"`
	BlockNumber     int64           `json:"blockNumber"`
}

// RechargeListReq 充值列表请求
type RechargeListReq struct {
	UserId   int64  `json:"userId"`
	Symbol   string `json:"symbol"`
	Status   *int   `json:"status"`
	Page     int    `json:"page" v:"required|min:1#页码不能为空|页码必须大于0"`
	PageSize int    `json:"pageSize" v:"required|min:1|max:100#每页数量不能为空|每页数量必须大于0|每页数量不能超过100"`
}

// RechargeListRes 充值列表响应
type RechargeListRes struct {
	List     []*RechargeRecord `json:"list"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
}
