package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

type ApgMatch struct {
	Id          int64       `json:"id" orm:"id,primary"`
	MatchDate   string      `json:"match_date" orm:"match_date"`
	Round       int         `json:"round" orm:"round"`
	PoolType    int         `json:"pool_type" orm:"pool_type"`
	StartTime   *gtime.Time `json:"start_time" orm:"start_time"`
	LockTime    *gtime.Time `json:"lock_time" orm:"lock_time"`
	FinishTime  *gtime.Time `json:"finish_time" orm:"finish_time"`
	IsFinished  int         `json:"is_finished" orm:"is_finished"`
	PlayerCount int         `json:"player_count" orm:"player_count"`
	GroupCount  int         `json:"group_count" orm:"group_count"`
	CreatedAt   *gtime.Time `json:"created_at" orm:"created_at"`
	UpdatedAt   *gtime.Time `json:"updated_at" orm:"updated_at"`
}

type ApgMatchStatistics struct {
	Id                    int64       `json:"id" orm:"id,primary"`
	MatchId               int64       `json:"match_id" orm:"match_id"`
	Token                 string      `json:"token" orm:"token"`
	WinnerRewardAmount    string      `json:"winner_reward_amount" orm:"winner_reward_amount"`
	ReferralRewardAmount  string      `json:"referral_reward_amount" orm:"referral_reward_amount"`
	PowerPurchaseAmount   string      `json:"power_purchase_amount" orm:"power_purchase_amount"`
	CreatedAt             *gtime.Time `json:"created_at" orm:"created_at"`
	UpdatedAt             *gtime.Time `json:"updated_at" orm:"updated_at"`
}
