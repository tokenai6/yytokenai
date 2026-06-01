package entity

import (
	"XWFrame/internal/frame/model"

	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

// AccountBalanceEntity 账户余额实体
type AccountBalanceEntity struct {
	model.BaseEntity
	UserId        int64           `json:"userId"`
	AccountTypeId int64           `json:"accountTypeId"`
	AccountType   string          `json:"accountType"` // 账户类型（冗余字段，如apg_user_balance）
	Symbol        string          `json:"symbol"`
	Balance       decimal.Decimal `json:"balance"`
	FrozenBalance decimal.Decimal `json:"frozenBalance"`
	TotalAmount   decimal.Decimal `json:"totalAmount"`
	Version       int             `json:"version"`
	CreatedAt     *gtime.Time     `json:"createdAt"`
	UpdatedAt     *gtime.Time     `json:"updatedAt"`
}

// BalanceChangeLogEntity 资金变动日志实体
type BalanceChangeLogEntity struct {
	Id             int64           `json:"id"`
	UserId         int64           `json:"userId"`
	AccountTypeId  int64           `json:"accountTypeId"`
	Symbol         string          `json:"symbol"`
	ChangeType     string          `json:"changeType"`
	Amount         decimal.Decimal `json:"amount"`
	BeforeBalance  decimal.Decimal `json:"beforeBalance"`
	AfterBalance   decimal.Decimal `json:"afterBalance"`
	RelatedOrderNo string          `json:"relatedOrderNo"`
	RelatedId      int64           `json:"relatedId"`
	Remark         string          `json:"remark"`
	OperatorId     int64           `json:"operatorId"`
	OperatorType   string          `json:"operatorType"`
	CreatedAt      *gtime.Time     `json:"createdAt"`
}
