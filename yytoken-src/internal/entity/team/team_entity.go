package team

import (
	"XWFrame/internal/frame/model"

	"github.com/shopspring/decimal"
)

// TeamEntity 团队实体
type TeamEntity struct {
	model.BaseEntity
	Name                string `json:"name" gorm:"size:50;not null;comment:团队名称"`
	LeaderWalletAddress string `json:"leader_wallet_address" gorm:"size:42;not null;uniqueIndex;comment:团队长钱包地址"`
	Status              int    `json:"status" gorm:"default:1;comment:状态 1-启用 0-禁用"`
	ParentId            *int64 `json:"parent_id" gorm:"index;comment:父团队ID（NULL表示顶级团队）"`
}

func (TeamEntity) TableName() string {
	return "team"
}

// TeamStatsEntity 团队统计实体
type TeamStatsEntity struct {
	model.BaseEntity
	TeamID             int64           `json:"team_id" gorm:"not null;index;comment:团队ID"`
	StakeAmount48h     decimal.Decimal `json:"stake_amount_48h" gorm:"type:decimal(28,18);default:0;comment:48小时新增质押金额"`
	WithdrawAmount48h  decimal.Decimal `json:"withdraw_amount_48h" gorm:"type:decimal(28,18);default:0;comment:48小时提现金额"`
	StakeCount48h      int             `json:"stake_count_48h" gorm:"default:0;comment:48小时新增质押笔数"`
	WithdrawCount48h   int             `json:"withdraw_count_48h" gorm:"default:0;comment:48小时提现笔数"`
	TotalMemberCount   int             `json:"total_member_count" gorm:"default:0;comment:团队总人数"`
}

func (TeamStatsEntity) TableName() string {
	return "team_stats"
}

type StakeDetailItem struct {
	Id            int64           `json:"id"`
	UserId        int64           `json:"user_id"`
	WalletAddress string          `json:"wallet_address"`
	StakeAmount   decimal.Decimal `json:"stake_amount"`
	StakeUsdt     decimal.Decimal `json:"stake_usdt"`
	StakeType     int             `json:"stake_type"`
	TxHash        string          `json:"tx_hash"`
	CreatedAt     string          `json:"created_at"`
}

type WithdrawDetailItem struct {
	Id              int64           `json:"id"`
	UserId          int64           `json:"user_id"`
	WalletAddress   string          `json:"wallet_address"`
	Amount          decimal.Decimal `json:"amount"`
	Fee             decimal.Decimal `json:"fee"`
	ActualAmount    decimal.Decimal `json:"actual_amount"`
	Symbol          string          `json:"symbol"`
	Status          string          `json:"status"`
	WithdrawAddress string          `json:"withdraw_address"`
	TxHash          string          `json:"tx_hash"`
	CreatedAt       string          `json:"created_at"`
}

type DailyTrendItem struct {
	Date   string          `json:"date"`
	Amount decimal.Decimal `json:"amount"`
}
