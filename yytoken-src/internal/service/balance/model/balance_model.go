package model

import (
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

// AccountBalance 账户余额
type AccountBalance struct {
	Id            int64           `json:"id"`
	UserId        int64           `json:"userId"`
	AccountTypeId int64           `json:"accountTypeId"`
	Symbol        string          `json:"symbol"`
	Balance       decimal.Decimal `json:"balance"`
	FrozenBalance decimal.Decimal `json:"frozenBalance"`
	TotalAmount   decimal.Decimal `json:"totalAmount"`
	Version       int             `json:"version"`
	CreatedAt     *gtime.Time     `json:"createdAt"`
	UpdatedAt     *gtime.Time     `json:"updatedAt"`
}

// BalanceChangeLog 资金变动日志
type BalanceChangeLog struct {
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

// BalanceChangeReq 资金变动请求
type BalanceChangeReq struct {
	UserID         int64           `json:"userId"`
	AccountTypeID  int64           `json:"accountTypeId"`
	Symbol         string          `json:"symbol"`
	Amount         decimal.Decimal `json:"amount"`
	ChangeType     string          `json:"changeType"`
	RelatedOrderNo string          `json:"relatedOrderNo"`
	RelatedID      int64           `json:"relatedId"`
	Remark         string          `json:"remark"`
	OperatorID     int64           `json:"operatorId"`
	OperatorType   string          `json:"operatorType"`
	Tx             gdb.TX          `json:"-"` // 事务对象（可选）
}

// TransferReq 转账请求
type TransferReq struct {
	FromUserID        int64           `json:"fromUserId"`
	ToUserID          int64           `json:"toUserId"`
	FromAccountTypeID int64           `json:"fromAccountTypeId"`
	ToAccountTypeID   int64           `json:"toAccountTypeId"`
	Symbol            string          `json:"symbol"`
	Amount            decimal.Decimal `json:"amount"`
	RelatedOrderNo    string          `json:"relatedOrderNo"`
	Remark            string          `json:"remark"`
	OperatorID        int64           `json:"operatorId"`
	OperatorType      string          `json:"operatorType"`
	Tx                gdb.TX          `json:"-"` // 事务对象（可选）
}

// BurnAssetReq 资产销毁请求
type BurnAssetReq struct {
	UserID        int64           `json:"userId"`
	AssetTypeID   int64           `json:"assetTypeId"`
	AccountTypeID int64           `json:"accountTypeId"`
	Symbol        string          `json:"symbol"`
	Amount        decimal.Decimal `json:"amount"`
	OrderNo       string          `json:"orderNo"`
	Remark        string          `json:"remark"`
	Tx            gdb.TX          `json:"-"`
}

// QueryBalanceLogsReq 查询余额变动日志请求
type QueryBalanceLogsReq struct {
	UserID        int64  `json:"userId"`
	AccountTypeID *int64 `json:"accountTypeId"` // 可选
	Symbol        string `json:"symbol"`        // 可选
	ChangeType    string `json:"changeType"`    // 可选
	Page          int    `json:"page"`
	PageSize      int    `json:"pageSize"`
}
