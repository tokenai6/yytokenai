package reward

import (
	"time"

	"github.com/shopspring/decimal"
)

// QuotaChangeLogEntity 额度变动日志实体
type QuotaChangeLogEntity struct {
	Id             int64           `json:"id" orm:"id,primary"`
	UserID         int64           `json:"user_id" orm:"user_id"`
	ChangeType     string          `json:"change_type" orm:"change_type"`
	ChangeAmount   decimal.Decimal `json:"change_amount" orm:"change_amount"`
	QuotaBefore    decimal.Decimal `json:"quota_before" orm:"quota_before"`
	QuotaAfter     decimal.Decimal `json:"quota_after" orm:"quota_after"`
	RelatedOrderNo string          `json:"related_order_no" orm:"related_order_no"`
	RelatedID      int64           `json:"related_id" orm:"related_id"`
	RewardType     string          `json:"reward_type" orm:"reward_type"`
	Remark         string          `json:"remark" orm:"remark"`
	CreatedAt      time.Time       `json:"created_at" orm:"created_at"`
}
