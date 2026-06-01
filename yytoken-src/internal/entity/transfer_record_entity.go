package entity

import (
	"time"

	"github.com/shopspring/decimal"
)

// InternalTransferRecordEntity 内部转账记录实体
type InternalTransferRecordEntity struct {
	Id         int64           `json:"id" orm:"id,primary"`
	FromUserID int64           `json:"from_user_id" orm:"from_user_id"`
	ToUserID   int64           `json:"to_user_id" orm:"to_user_id"`
	Symbol     string          `json:"symbol" orm:"symbol"`
	Amount     decimal.Decimal `json:"amount" orm:"amount"`
	RequestID  string          `json:"request_id" orm:"request_id"`
	CreatedAt  time.Time       `json:"created_at" orm:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at" orm:"updated_at"`
}

// TableName 指定表名
func (InternalTransferRecordEntity) TableName() string {
	return "internal_transfer_record"
}
