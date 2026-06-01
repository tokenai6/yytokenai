package cobo

import (
	"time"

	"github.com/shopspring/decimal"
)

// BalanceEntity 用户Cobo余额实体
type BalanceEntity struct {
	ID               int64           `json:"id" orm:"id"`
	UserID           int64           `json:"user_id" orm:"user_id"`
	Symbol           string          `json:"symbol" orm:"symbol"`
	AvailableAmount  decimal.Decimal `json:"available_amount" orm:"available_amount"` // 可用余额
	FrozenAmount     decimal.Decimal `json:"frozen_amount" orm:"frozen_amount"`       // 冻结金额（提现中）
	CreatedAt        time.Time       `json:"created_at" orm:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at" orm:"updated_at"`
}

// TableName 表名
func (e *BalanceEntity) TableName() string {
	return "cobo_balance"
}

// GetTotalAmount 获取总金额（可用+冻结）
func (e *BalanceEntity) GetTotalAmount() decimal.Decimal {
	return e.AvailableAmount.Add(e.FrozenAmount)
}

// HasEnoughBalance 检查余额是否充足
func (e *BalanceEntity) HasEnoughBalance(amount decimal.Decimal) bool {
	return e.AvailableAmount.GreaterThanOrEqual(amount)
}

// Freeze 冻结金额
func (e *BalanceEntity) Freeze(amount decimal.Decimal) bool {
	if !e.HasEnoughBalance(amount) {
		return false
	}
	e.AvailableAmount = e.AvailableAmount.Sub(amount)
	e.FrozenAmount = e.FrozenAmount.Add(amount)
	return true
}

// Unfreeze 解冻金额
func (e *BalanceEntity) Unfreeze(amount decimal.Decimal) {
	if e.FrozenAmount.GreaterThanOrEqual(amount) {
		e.FrozenAmount = e.FrozenAmount.Sub(amount)
		e.AvailableAmount = e.AvailableAmount.Add(amount)
	}
}

// Deduct 扣除冻结金额（提现成功时调用）
func (e *BalanceEntity) Deduct(amount decimal.Decimal) bool {
	if e.FrozenAmount.GreaterThanOrEqual(amount) {
		e.FrozenAmount = e.FrozenAmount.Sub(amount)
		return true
	}
	return false
}
