package reward

import (
	"time"

	"github.com/shopspring/decimal"
)

// RewardReferralEntity 推荐奖励实体
type RewardReferralEntity struct {
	Id            int64           `json:"id" orm:"id,primary"`
	UserID        int64           `json:"user_id" orm:"user_id"`
	RewardDate    time.Time       `json:"reward_date" orm:"reward_date"`
	TotalLayers   int             `json:"total_layers" orm:"total_layers"`
	RewardDetails string          `json:"reward_details" orm:"reward_details"`
	RewardAmount  decimal.Decimal `json:"reward_amount" orm:"reward_amount"`
	ActualReward  decimal.Decimal `json:"actual_reward" orm:"actual_reward"`
	QuotaBefore   decimal.Decimal `json:"quota_before" orm:"quota_before"`
	QuotaAfter    decimal.Decimal `json:"quota_after" orm:"quota_after"`
	QuotaDeducted decimal.Decimal `json:"quota_deducted" orm:"quota_deducted"`
	Status        int             `json:"status" orm:"status"`
	OrderNo       string          `json:"order_no" orm:"order_no"`
	CreatedAt     time.Time       `json:"created_at" orm:"created_at"`
}
