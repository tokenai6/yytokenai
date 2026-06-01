package cobo

import (
	"time"

	"github.com/shopspring/decimal"
)

// BalanceSnapshotEntity Cobo钱包余额快照实体
type BalanceSnapshotEntity struct {
	ID            int64           `json:"id" orm:"id"`
	WalletBalance decimal.Decimal `json:"wallet_balance" orm:"wallet_balance"`
	SnapshotAt    time.Time       `json:"snapshot_at" orm:"snapshot_at"`
	CreatedAt     time.Time       `json:"created_at" orm:"created_at"`
}

// TableName 表名
func (e *BalanceSnapshotEntity) TableName() string {
	return "cobo_balance_snapshot"
}
