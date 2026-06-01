package entity

import (
	"XWFrame/internal/frame/model"

	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

// RechargeRecordEntity 充值记录实体
type RechargeRecordEntity struct {
	model.BaseEntity
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
