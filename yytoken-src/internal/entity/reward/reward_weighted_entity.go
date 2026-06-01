package reward

import (
	"XWFrame/internal/frame/model"
	"time"

	"github.com/shopspring/decimal"
)

// RewardWeightedEntity 加权奖励记录实体
type RewardWeightedEntity struct {
	model.BaseEntity
	UserID                int64           `json:"user_id" gorm:"not null;index:idx_reward_weighted_user_date"`
	RewardDate            time.Time       `json:"reward_date" gorm:"type:date;not null;index:idx_reward_weighted_user_date,priority:2;index:idx_reward_weighted_date_ranking"`
	Ranking               int             `json:"ranking" gorm:"not null;index:idx_reward_weighted_date_ranking,priority:2"`
	NewDirectPerformance  decimal.Decimal `json:"new_direct_performance" gorm:"type:decimal(28,18);not null"`
	Top50TotalPerformance decimal.Decimal `json:"top50_total_performance" gorm:"type:decimal(28,18);not null"`
	NetworkTotalOutput    decimal.Decimal `json:"network_total_output" gorm:"type:decimal(28,18);not null"`
	WeightRatio           decimal.Decimal `json:"weight_ratio" gorm:"type:decimal(10,6);not null"`
	RewardAmount          decimal.Decimal `json:"reward_amount" gorm:"type:decimal(28,18);not null"`
	ActualReward          decimal.Decimal `json:"actual_reward" gorm:"type:decimal(28,18);not null"`
	QuotaBefore           decimal.Decimal `json:"quota_before" gorm:"type:decimal(28,18);not null"`
	QuotaAfter            decimal.Decimal `json:"quota_after" gorm:"type:decimal(28,18);not null"`
	QuotaDeducted         decimal.Decimal `json:"quota_deducted" gorm:"type:decimal(28,18);not null"`
	Status                int             `json:"status" gorm:"not null;default:1;comment:1-已发放 2-已削减"`
	OrderNo               string          `json:"order_no" gorm:"uniqueIndex;size:64;not null"`
}

// TableName 指定表名
func (RewardWeightedEntity) TableName() string {
	return "reward_weighted"
}
