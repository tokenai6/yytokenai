package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

type ApgPlayer struct {
	Id                    int64       `json:"id" orm:"id,primary"`
	UserId                int64       `json:"user_id" orm:"user_id"`
	WalletAddress         string      `json:"wallet_address" orm:"wallet_address"`
	MatchId               int64       `json:"match_id" orm:"match_id"`
	PoolType              int         `json:"pool_type" orm:"pool_type"`
	Round                 int         `json:"round" orm:"round"`
	GroupId               int         `json:"group_id" orm:"group_id"`
	PaymentTokenId        int64       `json:"payment_token_id" orm:"payment_token_id"`
	PaymentToken          string      `json:"payment_token" orm:"payment_token"`
	PaymentAmount         string      `json:"payment_amount" orm:"payment_amount"`
	TxHash                string      `json:"tx_hash" orm:"tx_hash"`
	IsWinner              int         `json:"is_winner" orm:"is_winner"`
	JoinStatus            int         `json:"join_status" orm:"join_status"`
	ResultTime            *gtime.Time `json:"result_time" orm:"result_time"`
	RefundToken           string      `json:"refund_token" orm:"refund_token"`
	RefundAmount          string      `json:"refund_amount" orm:"refund_amount"`
	HasRefund             bool        `json:"has_refund" orm:"has_refund"`
	RefundTxHash          string      `json:"refund_tx_hash" orm:"refund_tx_hash"`
	RefundTime            *gtime.Time `json:"refund_time" orm:"refund_time"`
	WinnerRewardToken     string      `json:"winner_reward_token" orm:"winner_reward_token"`
	WinnerRewardAmount    string      `json:"winner_reward_amount" orm:"winner_reward_amount"`
	ReferralPoolToken     string      `json:"referral_pool_token" orm:"referral_pool_token"`
	ReferralPoolAmount    string      `json:"referral_pool_amount" orm:"referral_pool_amount"`
	PowerPurchaseToken    string      `json:"power_purchase_token" orm:"power_purchase_token"`
	PowerPurchaseAmount   string      `json:"power_purchase_amount" orm:"power_purchase_amount"`
	CoboReserveToken      string      `json:"cobo_reserve_token" orm:"cobo_reserve_token"`
	CoboReserveAmount     string      `json:"cobo_reserve_amount" orm:"cobo_reserve_amount"`
	HasClaimedReward      bool        `json:"has_claimed_reward" orm:"has_claimed_reward"`
	ContractIndex         *int64      `json:"contract_index" orm:"contract_index"`
	ContractId            *int64      `json:"contract_id" orm:"contract_id"`
	LpAmount              string      `json:"lp_amount" orm:"lp_amount"`
	StakeStatus           int         `json:"stake_status" orm:"stake_status"`
	StakeTxHash           string      `json:"stake_tx_hash" orm:"stake_tx_hash"`
	StakingPackageId      *int64      `json:"staking_package_id" orm:"staking_package_id"`
	StakeTime             *gtime.Time `json:"stake_time" orm:"stake_time"`
	CreatedAt             *gtime.Time `json:"created_at" orm:"created_at"`
	UpdatedAt             *gtime.Time `json:"updated_at" orm:"updated_at"`
}
