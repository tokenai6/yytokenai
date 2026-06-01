package reward

import (
	"XWFrame/internal/frame/model"
	"time"

	"github.com/shopspring/decimal"
)

// RewardDistrictEntity 小区奖励记录实体
type RewardDistrictEntity struct {
	model.BaseEntity
	UserID                 int64           `json:"user_id" gorm:"not null;index:idx_reward_district_user_date"`
	RewardDate             time.Time       `json:"reward_date" gorm:"type:date;not null;index:idx_reward_district_user_date,priority:2;index:idx_reward_district_date"`
	VipLevel               int             `json:"vip_level" gorm:"not null"`
	RewardRate             decimal.Decimal `json:"reward_rate" gorm:"type:decimal(10,6);not null"`
	DistrictPerformance    decimal.Decimal `json:"district_performance" gorm:"type:decimal(28,18);not null"`
	SubordinateStaticTotal decimal.Decimal `json:"subordinate_static_total" gorm:"type:decimal(28,18);not null"`
	RewardDetails          string          `json:"reward_details" gorm:"type:jsonb"`
	RewardAmount           decimal.Decimal `json:"reward_amount" gorm:"type:decimal(28,18);not null"`
	ActualReward           decimal.Decimal `json:"actual_reward" gorm:"type:decimal(28,18);not null"`
	QuotaBefore            decimal.Decimal `json:"quota_before" gorm:"type:decimal(28,18);not null"`
	QuotaAfter             decimal.Decimal `json:"quota_after" gorm:"type:decimal(28,18);not null"`
	QuotaDeducted          decimal.Decimal `json:"quota_deducted" gorm:"type:decimal(28,18);not null"`
	Status                 int             `json:"status" gorm:"not null;default:1;comment:1-已发放 2-已削减"`
	OrderNo                string          `json:"order_no" gorm:"uniqueIndex;size:64;not null"`
}

// TableName 指定表名
func (RewardDistrictEntity) TableName() string {
	return "reward_district"
}
