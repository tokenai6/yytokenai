package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

type ApgReferralReward struct {
	Id              int64       `json:"id" orm:"id,primary"`
	ReceiverUserId  int64       `json:"-" orm:"receiver_user_id"`
	SenderUserId    int64       `json:"-" orm:"sender_user_id"`
	SenderAddress   string      `json:"sender_address" orm:"-"`
	MatchId         int64       `json:"match_id" orm:"match_id"`
	PlayerId        int64       `json:"player_id" orm:"player_id"`
	Level           int         `json:"level" orm:"level"`
	RewardRate      string      `json:"reward_rate" orm:"reward_rate"`
	RewardTokenId   int         `json:"reward_token_id" orm:"reward_token_id"`
	RewardToken     string      `json:"reward_token" orm:"reward_token"`
	RewardAmount    string      `json:"reward_amount" orm:"reward_amount"`
	BalanceChangeId int64       `json:"-" orm:"balance_change_id"`
	ClaimStatus     string      `json:"claim_status" orm:"claim_status"`
	ClaimNonce      string      `json:"claim_nonce" orm:"claim_nonce"`
	ClaimTxHash     string      `json:"claim_tx_hash" orm:"claim_tx_hash"`
	ClaimTime       *gtime.Time `json:"claim_time" orm:"claim_time"`
	CreatedAt       *gtime.Time `json:"created_at" orm:"created_at"`
	UpdatedAt       *gtime.Time `json:"updated_at" orm:"updated_at"`
}

const (
	ReferralClaimStatusUnclaimed = "unclaimed"
	ReferralClaimStatusClaiming  = "claiming"
	ReferralClaimStatusClaimed   = "claimed"
	ReferralClaimStatusFailed    = "failed"
	ReferralClaimStatusTimeout   = "timeout"
)
