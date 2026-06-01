package apg

import (
	"context"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

type ReferralRewardSummary struct {
	TokenId         int    `json:"token_id"`
	RewardToken     string `json:"reward_token"`
	ClaimedAmount   string `json:"claimed_amount"`
	UnclaimedAmount string `json:"unclaimed_amount"`
}

type IReferralRewardDao interface {
	Create(ctx context.Context, reward *entity.ApgReferralReward) error
	BatchCreate(ctx context.Context, tx gdb.TX, rewards []*entity.ApgReferralReward) error
	GetById(ctx context.Context, id int64) (*entity.ApgReferralReward, error)
	GetUserRewards(ctx context.Context, userId int64, page, pageSize int) ([]*entity.ApgReferralReward, int, error)
	GetMatchRewards(ctx context.Context, matchId int64) ([]*entity.ApgReferralReward, error)
	GetUserTotalRewards(ctx context.Context, userId int64) (map[string]string, error)
	GetUserUnclaimedTotal(ctx context.Context, userId int64) (map[string]string, error)
	GetUserUnclaimedTotalByTokenId(ctx context.Context, userId int64, tokenId int) (string, error)
	GetUserRewardSummary(ctx context.Context, userId int64) ([]*ReferralRewardSummary, error)
	GetAdminReferralList(ctx context.Context, matchId int64, receiverWalletAddress string, page, pageSize int) ([]*entity.ApgReferralReward, int, error)
	DeleteByMatchId(ctx context.Context, matchId int64) (int64, error)
	UpdateById(ctx context.Context, id int64, data gdb.Map) error
	UpdateClaimStatusByUserAndTokenId(ctx context.Context, userId int64, tokenId int, status, txHash string) (int64, error)
	GetUserUnclaimedByTokenId(ctx context.Context, userId int64, tokenId int) ([]*entity.ApgReferralReward, error)
	UpdateClaimStatusByIds(ctx context.Context, ids []int64, status string) error
	UpdateClaimStatusAndNonceByIds(ctx context.Context, ids []int64, status, nonce string) error
	GetClaimingByNonce(ctx context.Context, nonce string) ([]*entity.ApgReferralReward, error)
	UpdateClaimStatusByNonce(ctx context.Context, userId int64, nonce, status, txHash string) (int64, error)
	ResetClaimingTimeout(ctx context.Context, timeoutMinutes int) (int64, error)
}

type referralRewardDao struct{}

var ReferralReward IReferralRewardDao = &referralRewardDao{}

func (d *referralRewardDao) Create(ctx context.Context, reward *entity.ApgReferralReward) error {
	_, err := db.GetDB().Model("apg_mint_referral_reward").Insert(reward)
	return err
}

func (d *referralRewardDao) BatchCreate(ctx context.Context, tx gdb.TX, rewards []*entity.ApgReferralReward) error {
	if len(rewards) == 0 {
		return nil
	}
	_, err := tx.Model("apg_mint_referral_reward").FieldsEx("id").Insert(rewards)
	return err
}

func (d *referralRewardDao) GetById(ctx context.Context, id int64) (*entity.ApgReferralReward, error) {
	var reward *entity.ApgReferralReward
	err := db.GetDB().Model("apg_mint_referral_reward").Where("id", id).Scan(&reward)
	return reward, err
}

func (d *referralRewardDao) GetUserRewards(ctx context.Context, userId int64, page, pageSize int) ([]*entity.ApgReferralReward, int, error) {
	total, err := db.GetDB().Model("apg_mint_referral_reward").Where("receiver_user_id", userId).Count()
	if err != nil {
		return nil, 0, err
	}

	var rewards []*entity.ApgReferralReward
	err = db.GetDB().Model("apg_mint_referral_reward r").
		LeftJoin("user_info u", "r.sender_user_id = u.id").
		Where("r.receiver_user_id", userId).
		Fields("r.*, u.wallet_address as sender_address").
		Order("r.created_at DESC").
		Page(page, pageSize).
		Scan(&rewards)

	return rewards, total, err
}

func (d *referralRewardDao) GetMatchRewards(ctx context.Context, matchId int64) ([]*entity.ApgReferralReward, error) {
	var rewards []*entity.ApgReferralReward
	err := db.GetDB().Model("apg_mint_referral_reward").
		Where("match_id", matchId).
		Scan(&rewards)
	return rewards, err
}

func (d *referralRewardDao) GetUserTotalRewards(ctx context.Context, userId int64) (map[string]string, error) {
	var rows []struct {
		RewardToken string `json:"reward_token"`
		TotalAmount string `json:"total_amount"`
	}
	err := db.GetDB().Model("apg_mint_referral_reward").
		Where("receiver_user_id", userId).
		Fields("reward_token", "SUM(reward_amount) as total_amount").
		Group("reward_token").
		Scan(&rows)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, row := range rows {
		result[row.RewardToken] = row.TotalAmount
	}
	return result, nil
}

func (d *referralRewardDao) GetUserUnclaimedTotal(ctx context.Context, userId int64) (map[string]string, error) {
	var rows []struct {
		RewardToken string `json:"reward_token"`
		TotalAmount string `json:"total_amount"`
	}
	err := db.GetDB().Model("apg_mint_referral_reward").
		Where("receiver_user_id", userId).
		Where("claim_status", entity.ReferralClaimStatusUnclaimed).
		Fields("reward_token", "SUM(reward_amount) as total_amount").
		Group("reward_token").
		Scan(&rows)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, row := range rows {
		result[row.RewardToken] = row.TotalAmount
	}
	return result, nil
}

func (d *referralRewardDao) GetUserRewardSummary(ctx context.Context, userId int64) ([]*ReferralRewardSummary, error) {
	var rows []*ReferralRewardSummary
	err := db.GetDB().Model("apg_mint_referral_reward").
		Where("receiver_user_id", userId).
		Fields(`
			reward_token_id as token_id,
			reward_token,
			SUM(CASE WHEN claim_status = 'claimed' THEN reward_amount ELSE 0 END) as claimed_amount,
			SUM(CASE WHEN claim_status = 'unclaimed' THEN reward_amount ELSE 0 END) as unclaimed_amount
		`).
		Group("reward_token_id, reward_token").
		Scan(&rows)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (d *referralRewardDao) GetUserUnclaimedTotalByTokenId(ctx context.Context, userId int64, tokenId int) (string, error) {
	var result struct {
		TotalAmount string `json:"total_amount"`
	}
	err := db.GetDB().Model("apg_mint_referral_reward").
		Where("receiver_user_id", userId).
		Where("reward_token_id", tokenId).
		Where("claim_status", entity.ReferralClaimStatusUnclaimed).
		Fields("SUM(reward_amount) as total_amount").
		Scan(&result)
	if err != nil {
		return "0", err
	}
	if result.TotalAmount == "" {
		return "0", nil
	}
	return result.TotalAmount, nil
}

func (d *referralRewardDao) GetAdminReferralList(ctx context.Context, matchId int64, receiverWalletAddress string, page, pageSize int) ([]*entity.ApgReferralReward, int, error) {
	model := db.GetDB().Model("apg_mint_referral_reward r").
		LeftJoin("user_info u", "r.receiver_user_id = u.id")

	if matchId > 0 {
		model = model.Where("r.match_id", matchId)
	}
	if receiverWalletAddress != "" {
		model = model.WhereLike("u.wallet_address", "%"+receiverWalletAddress+"%")
	}

	total, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	var rewards []*entity.ApgReferralReward
	err = model.Fields("r.*").
		Order("r.id DESC").
		Page(page, pageSize).
		Scan(&rewards)

	return rewards, total, err
}

func (d *referralRewardDao) DeleteByMatchId(ctx context.Context, matchId int64) (int64, error) {
	result, err := db.GetDB().Model("apg_mint_referral_reward").
		Where("match_id", matchId).
		Delete()
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *referralRewardDao) UpdateById(ctx context.Context, id int64, data gdb.Map) error {
	_, err := db.GetDB().Model("apg_mint_referral_reward").Where("id", id).Data(data).Update()
	return err
}

func (d *referralRewardDao) UpdateClaimStatusByUserAndTokenId(ctx context.Context, userId int64, tokenId int, status, txHash string) (int64, error) {
	data := gdb.Map{
		"claim_status": status,
	}
	if txHash != "" {
		data["claim_tx_hash"] = txHash
	}
	if status == entity.ReferralClaimStatusClaimed {
		data["claim_time"] = "now()"
	}

	result, err := db.GetDB().Model("apg_mint_referral_reward").
		Where("receiver_user_id", userId).
		Where("reward_token_id", tokenId).
		Where("claim_status", entity.ReferralClaimStatusUnclaimed).
		Data(data).
		Update()
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *referralRewardDao) GetUserUnclaimedByTokenId(ctx context.Context, userId int64, tokenId int) ([]*entity.ApgReferralReward, error) {
	var rewards []*entity.ApgReferralReward
	err := db.GetDB().Model("apg_mint_referral_reward").
		Where("receiver_user_id", userId).
		Where("reward_token_id", tokenId).
		Where("claim_status", entity.ReferralClaimStatusUnclaimed).
		Scan(&rewards)
	return rewards, err
}

func (d *referralRewardDao) UpdateClaimStatusByIds(ctx context.Context, ids []int64, status string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := db.GetDB().Model("apg_mint_referral_reward").
		WhereIn("id", ids).
		Data(gdb.Map{"claim_status": status}).
		Update()
	return err
}

func (d *referralRewardDao) UpdateClaimStatusAndNonceByIds(ctx context.Context, ids []int64, status, nonce string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := db.GetDB().Model("apg_mint_referral_reward").
		WhereIn("id", ids).
		Data(gdb.Map{
			"claim_status": status,
			"claim_nonce":  nonce,
		}).
		Update()
	return err
}

func (d *referralRewardDao) GetClaimingByNonce(ctx context.Context, nonce string) ([]*entity.ApgReferralReward, error) {
	var rewards []*entity.ApgReferralReward
	err := db.GetDB().Model("apg_mint_referral_reward").
		Where("claim_nonce", nonce).
		Where("claim_status", entity.ReferralClaimStatusClaiming).
		Scan(&rewards)
	return rewards, err
}

func (d *referralRewardDao) UpdateClaimStatusByNonce(ctx context.Context, userId int64, nonce, status, txHash string) (int64, error) {
	data := gdb.Map{
		"claim_status": status,
	}
	if txHash != "" {
		data["claim_tx_hash"] = txHash
	}
	if status == entity.ReferralClaimStatusClaimed {
		data["claim_time"] = "now()"
	}

	result, err := db.GetDB().Model("apg_mint_referral_reward").
		Where("receiver_user_id", userId).
		Where("claim_nonce", nonce).
		Data(data).
		Update()
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *referralRewardDao) ResetClaimingTimeout(ctx context.Context, timeoutMinutes int) (int64, error) {
	result, err := db.GetDB().Model("apg_mint_referral_reward").
		Where("claim_status", entity.ReferralClaimStatusClaiming).
		Wheref("updated_at < NOW() - INTERVAL '%d minutes'", timeoutMinutes).
		Data(gdb.Map{"claim_status": entity.ReferralClaimStatusUnclaimed}).
		Update()
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
