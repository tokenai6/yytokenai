package reward

import (
	"XWFrame/internal/frame/model"
	"time"

	"github.com/shopspring/decimal"
)

// RewardStaticEntity 静态收益记录实体
type RewardStaticEntity struct {
	model.BaseEntity
	UserID        int64           `json:"user_id" gorm:"not null;index:idx_reward_static_user_date"`
	PackageID     int64           `json:"package_id" gorm:"not null;index:idx_reward_static_package"`
	RewardDate    time.Time       `json:"reward_date" gorm:"type:date;not null;index:idx_reward_static_user_date,priority:2;index:idx_reward_static_date"`
	StakeAmount   decimal.Decimal `json:"stake_amount" gorm:"type:decimal(28,18);not null"`
	PowerValue    decimal.Decimal `json:"power_value" gorm:"type:decimal(28,18);not null"`
	YieldRate     decimal.Decimal `json:"yield_rate" gorm:"type:decimal(10,6);not null"`
	RewardAmount  decimal.Decimal `json:"reward_amount" gorm:"type:decimal(28,18);not null"`
	ActualReward  decimal.Decimal `json:"actual_reward" gorm:"type:decimal(28,18);not null"`
	QuotaBefore   decimal.Decimal `json:"quota_before" gorm:"type:decimal(28,18);not null"`
	QuotaAfter    decimal.Decimal `json:"quota_after" gorm:"type:decimal(28,18);not null"`
	QuotaDeducted decimal.Decimal `json:"quota_deducted" gorm:"type:decimal(28,18);not null"`
	Status        int             `json:"status" gorm:"not null;default:1;comment:1-已发放 2-已削减"`
	OrderNo       string          `json:"order_no" gorm:"uniqueIndex;size:64;not null"`
}

// TableName 指定表名
func (RewardStaticEntity) TableName() string {
	return "reward_static"
}
