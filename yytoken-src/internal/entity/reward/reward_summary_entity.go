package reward

import (
	"XWFrame/internal/frame/model"
	"time"

	"github.com/shopspring/decimal"
)

// RewardSummaryEntity 奖励汇总实体
type RewardSummaryEntity struct {
	model.BaseEntity
	UserID             int64           `json:"user_id" gorm:"not null;index:idx_reward_summary_user_date"`
	StatDate           time.Time       `json:"stat_date" gorm:"type:date;not null;index:idx_reward_summary_user_date,priority:2;index:idx_reward_summary_date"`
	StaticReward       decimal.Decimal `json:"static_reward" gorm:"type:decimal(28,18);default:0"`
	DistrictReward     decimal.Decimal `json:"district_reward" gorm:"type:decimal(28,18);default:0"`
	ReferralReward     decimal.Decimal `json:"referral_reward" gorm:"type:decimal(28,18);default:0"`
	WeightedReward     decimal.Decimal `json:"weighted_reward" gorm:"type:decimal(28,18);default:0"`
	NodeReward         decimal.Decimal `json:"node_reward" gorm:"type:decimal(28,18);default:0"`
	TotalReward        decimal.Decimal `json:"total_reward" gorm:"type:decimal(28,18);default:0"`
	TotalQuotaDeducted decimal.Decimal `json:"total_quota_deducted" gorm:"type:decimal(28,18);default:0"`
	TotalRewardReduced decimal.Decimal `json:"total_reward_reduced" gorm:"type:decimal(28,18);default:0"` // 总削减奖励（额度不足时）
}

// TableName 指定表名
func (RewardSummaryEntity) TableName() string {
	return "reward_summary"
}
