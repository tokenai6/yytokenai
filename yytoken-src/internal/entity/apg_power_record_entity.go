package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

type ApgPowerRecord struct {
	Id             int64       `json:"id" orm:"id,primary"`
	UserId         int64       `json:"user_id" orm:"user_id"`
	SourceType     string      `json:"source_type" orm:"source_type"`
	SourceId       int64       `json:"source_id" orm:"source_id"`
	PurchaseToken  string      `json:"purchase_token" orm:"purchase_token"`
	PurchaseAmount string      `json:"purchase_amount" orm:"purchase_amount"`
	PowerAmount    string      `json:"power_amount" orm:"power_amount"`
	ConversionRate string      `json:"conversion_rate" orm:"conversion_rate"`
	Notes          string      `json:"notes" orm:"notes"`
	CreatedAt      *gtime.Time `json:"created_at" orm:"created_at"`
}
