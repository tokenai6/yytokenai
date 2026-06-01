package apg

import (
	"context"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gtime"
)

type IPlayerDao interface {
	Create(ctx context.Context, player *entity.ApgPlayer) error
	BatchCreate(ctx context.Context, tx gdb.TX, players []*entity.ApgPlayer) error
	GetById(ctx context.Context, id int64) (*entity.ApgPlayer, error)
	GetByTxHash(ctx context.Context, txHash string) (*entity.ApgPlayer, error)
	ExistsByTxHash(ctx context.Context, txHash string) (bool, error)
	ExistsByContractIndexAndTxHash(ctx context.Context, contractIndex int64, txHash string) (bool, error)
	GetByContractIndex(ctx context.Context, contractIndex int64) (*entity.ApgPlayer, error)
	GetMatchPlayers(ctx context.Context, matchId int64) ([]*entity.ApgPlayer, error)
	GetMatchSuccessPlayers(ctx context.Context, matchId int64) ([]*entity.ApgPlayer, error)
	GetGroupLoser(ctx context.Context, matchId int64, groupId int) (*entity.ApgPlayer, error)
	UpdateById(ctx context.Context, id int64, data gdb.Map) error
	BatchUpdate(ctx context.Context, tx gdb.TX, players []*entity.ApgPlayer) error
	GetUserRecords(ctx context.Context, userId int64, page, pageSize int) ([]*entity.ApgPlayer, int, error)
	GetUserRecordsByContractId(ctx context.Context, userId int64, contractId int64, page, pageSize int) ([]*entity.ApgPlayer, int, error)
	GetUserRecordsByContractIds(ctx context.Context, userId int64, contractIds []int64, page, pageSize int) ([]*entity.ApgPlayer, int, error)
	GetUserTodayRecords(ctx context.Context, userId int64, poolType int, startTime, endTime *gtime.Time) ([]*entity.ApgPlayer, error)
	CountUserTodayParticipation(ctx context.Context, userId int64, poolType int, startTime, endTime *gtime.Time) (int, error)
	CountUserMatchSuccessfulJoins(ctx context.Context, userId int64, matchId int64) (int, error)
	GetUserStats(ctx context.Context, userId int64) (map[string]interface{}, error)
	GetUserRefundRecords(ctx context.Context, userId int64, page, pageSize int) ([]*entity.ApgPlayer, int, error)
	GetUserTotalRefunds(ctx context.Context, userId int64) (map[string]string, error)
	GetUserMatchPlayers(ctx context.Context, userId int64, matchId int64) ([]*entity.ApgPlayer, error)
	GetMatchTotalPoolByToken(ctx context.Context, matchId int64) (map[string]string, error)
	GetMatchWinners(ctx context.Context, matchId int64, page, pageSize int) ([]*entity.ApgPlayer, int, error)
	GetMatchLosers(ctx context.Context, matchId int64) ([]*entity.ApgPlayer, error)
	// Admin APIs
	GetPlayerList(ctx context.Context, matchId int64, walletAddress string, isWinner, groupId, page, pageSize int) ([]*entity.ApgPlayer, int, error)
	GetClaimedRecords(ctx context.Context, matchId int64, walletAddress string, page, pageSize int) ([]*entity.ApgPlayer, int, error)
	GetStakedRecords(ctx context.Context, matchId int64, walletAddress string, page, pageSize int) ([]*entity.ApgPlayer, int, error)
	// Test APIs
	DeleteByMatchId(ctx context.Context, matchId int64) (int64, error)
	ResetPlayersForRedraw(ctx context.Context, matchId int64) (int64, error)
	// User Detail APIs
	GetUserRecordsByWalletAddress(ctx context.Context, walletAddress string, page, pageSize int) ([]*entity.ApgPlayer, int, error)
}

type playerDao struct{}

var Player IPlayerDao = &playerDao{}

func (d *playerDao) Create(ctx context.Context, player *entity.ApgPlayer) error {
	data := map[string]interface{}{
		"user_id":          player.UserId,
		"wallet_address":   player.WalletAddress,
		"match_id":         player.MatchId,
		"pool_type":        player.PoolType,
		"round":            player.Round,
		"group_id":         player.GroupId,
		"payment_token_id": player.PaymentTokenId,
		"payment_token":    player.PaymentToken,
		"payment_amount":   player.PaymentAmount,
		"tx_hash":          player.TxHash,
		"contract_index":   player.ContractIndex,
		"contract_id":      player.ContractId,
		"is_winner":        player.IsWinner,
		"join_status":      player.JoinStatus,
		"has_refund":       player.HasRefund,
		"created_at":       player.CreatedAt,
		"updated_at":       player.UpdatedAt,
	}
	if player.LpAmount != "" {
		data["lp_amount"] = player.LpAmount
	}
	if player.RefundToken != "" {
		data["refund_token"] = player.RefundToken
	}
	if player.RefundAmount != "" {
		data["refund_amount"] = player.RefundAmount
	}
	if player.ResultTime != nil {
		data["result_time"] = player.ResultTime
	}
	id, err := db.GetDB().Model("apg_mint_player").InsertAndGetId(data)
	if err != nil {
		return err
	}
	player.Id = id
	return nil
}

func (d *playerDao) BatchCreate(ctx context.Context, tx gdb.TX, players []*entity.ApgPlayer) error {
	_, err := tx.Model("apg_mint_player").OmitEmptyData().Insert(players)
	return err
}

func (d *playerDao) GetById(ctx context.Context, id int64) (*entity.ApgPlayer, error) {
	var player *entity.ApgPlayer
	err := db.GetDB().Model("apg_mint_player").Where("id", id).Scan(&player)
	if err != nil {
		return nil, err
	}
	return player, nil
}

func (d *playerDao) GetByTxHash(ctx context.Context, txHash string) (*entity.ApgPlayer, error) {
	var player *entity.ApgPlayer
	err := db.GetDB().Model("apg_mint_player").Where("tx_hash", txHash).Scan(&player)
	if err != nil {
		return nil, err
	}
	return player, nil
}

func (d *playerDao) ExistsByTxHash(ctx context.Context, txHash string) (bool, error) {
	count, err := db.GetDB().Model("apg_mint_player").Where("tx_hash", txHash).Count()
	return count > 0, err
}

func (d *playerDao) ExistsByContractIndexAndTxHash(ctx context.Context, contractIndex int64, txHash string) (bool, error) {
	count, err := db.GetDB().Model("apg_mint_player").
		Where("contract_index", contractIndex).
		Where("tx_hash", txHash).
		Count()
	return count > 0, err
}

func (d *playerDao) GetByContractIndex(ctx context.Context, contractIndex int64) (*entity.ApgPlayer, error) {
	var player *entity.ApgPlayer
	err := db.GetDB().Model("apg_mint_player").Where("contract_index", contractIndex).Scan(&player)
	return player, err
}

func (d *playerDao) GetMatchPlayers(ctx context.Context, matchId int64) ([]*entity.ApgPlayer, error) {
	var players []*entity.ApgPlayer
	err := db.GetDB().Model("apg_mint_player").
		Where("match_id", matchId).
		Order("id ASC").
		Scan(&players)
	return players, err
}

func (d *playerDao) GetMatchSuccessPlayers(ctx context.Context, matchId int64) ([]*entity.ApgPlayer, error) {
	var players []*entity.ApgPlayer
	err := db.GetDB().Model("apg_mint_player").
		Where("match_id", matchId).
		Where("join_status", 0).
		Order("id ASC").
		Scan(&players)
	return players, err
}

func (d *playerDao) GetGroupLoser(ctx context.Context, matchId int64, groupId int) (*entity.ApgPlayer, error) {
	var player *entity.ApgPlayer
	err := db.GetDB().Model("apg_mint_player").
		Where("match_id", matchId).
		Where("group_id", groupId).
		Where("is_winner", 2). // StatusLoser = 2
		Limit(1).
		Scan(&player)
	return player, err
}

func (d *playerDao) UpdateById(ctx context.Context, id int64, data gdb.Map) error {
	data["updated_at"] = gtime.Now()
	_, err := db.GetDB().Model("apg_mint_player").Where("id", id).Update(data)
	return err
}

func (d *playerDao) BatchUpdate(ctx context.Context, tx gdb.TX, players []*entity.ApgPlayer) error {
	for _, player := range players {
		player.UpdatedAt = gtime.Now()
		_, err := tx.Model("apg_mint_player").
			Where("id", player.Id).
			Update(player)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *playerDao) GetUserRecords(ctx context.Context, userId int64, page, pageSize int) ([]*entity.ApgPlayer, int, error) {
	model := db.GetDB().Model("apg_mint_player").Where("user_id", userId)

	total, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	var players []*entity.ApgPlayer
	err = model.Order("created_at DESC").
		Page(page, pageSize).
		Scan(&players)

	return players, total, err
}

func (d *playerDao) GetUserRecordsByContractId(ctx context.Context, userId int64, contractId int64, page, pageSize int) ([]*entity.ApgPlayer, int, error) {
	model := db.GetDB().Model("apg_mint_player").
		Where("user_id", userId).
		Where("contract_id", contractId)

	total, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	var players []*entity.ApgPlayer
	err = model.Order("created_at DESC").
		Page(page, pageSize).
		Scan(&players)

	return players, total, err
}

func (d *playerDao) GetUserRecordsByContractIds(ctx context.Context, userId int64, contractIds []int64, page, pageSize int) ([]*entity.ApgPlayer, int, error) {
	model := db.GetDB().Model("apg_mint_player").
		Where("user_id", userId).
		WhereIn("contract_id", contractIds)

	total, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	var players []*entity.ApgPlayer
	err = model.Order("created_at DESC").
		Page(page, pageSize).
		Scan(&players)

	return players, total, err
}

func (d *playerDao) GetUserTodayRecords(ctx context.Context, userId int64, poolType int, startTime, endTime *gtime.Time) ([]*entity.ApgPlayer, error) {
	var players []*entity.ApgPlayer
	err := db.GetDB().Model("apg_mint_player").
		Where("user_id", userId).
		Where("pool_type", poolType).
		WhereBetween("created_at", startTime, endTime).
		Scan(&players)
	return players, err
}

func (d *playerDao) CountUserTodayParticipation(ctx context.Context, userId int64, poolType int, startTime, endTime *gtime.Time) (int, error) {
	count, err := db.GetDB().Model("apg_mint_player").
		Where("user_id", userId).
		Where("pool_type", poolType).
		WhereBetween("created_at", startTime, endTime).
		Count()
	return count, err
}

func (d *playerDao) CountUserMatchSuccessfulJoins(ctx context.Context, userId int64, matchId int64) (int, error) {
	count, err := db.GetDB().Model("apg_mint_player").
		Where("user_id", userId).
		Where("match_id", matchId).
		Where("join_status", 0).
		Count()
	return count, err
}

func (d *playerDao) GetUserStats(ctx context.Context, userId int64) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := db.GetDB().Model("apg_mint_player").
		Where("user_id", userId).
		Fields(
			"COUNT(*) as total_join_count",
			"SUM(CASE WHEN is_winner = 1 THEN 1 ELSE 0 END) as total_win_count",
			"SUM(CASE WHEN is_winner = 2 THEN 1 ELSE 0 END) as total_lose_count",
			"SUM(CASE WHEN is_winner = 3 THEN 1 ELSE 0 END) as total_refund_count",
		).
		Scan(&result)
	return result, err
}

func (d *playerDao) GetUserRefundRecords(ctx context.Context, userId int64, page, pageSize int) ([]*entity.ApgPlayer, int, error) {
	model := db.GetDB().Model("apg_mint_player").
		Where("user_id", userId).
		Where("is_winner", 3)

	total, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	var records []*entity.ApgPlayer
	err = model.Order("created_at DESC").
		Page(page, pageSize).
		Scan(&records)

	return records, total, err
}

func (d *playerDao) GetUserTotalRefunds(ctx context.Context, userId int64) (map[string]string, error) {
	var result map[string]string
	err := db.GetDB().Model("apg_mint_player").
		Where("user_id", userId).
		Where("is_winner", 3).
		Fields(
			"refund_token",
			"SUM(refund_amount) as total_refund",
		).
		Group("refund_token").
		Scan(&result)
	return result, err
}

func (d *playerDao) GetPlayerList(ctx context.Context, matchId int64, walletAddress string, isWinner, groupId, page, pageSize int) ([]*entity.ApgPlayer, int, error) {
	model := db.GetDB().Model("apg_mint_player")

	if matchId > 0 {
		model = model.Where("match_id", matchId)
	}
	if walletAddress != "" {
		model = model.Where("LOWER(wallet_address) LIKE LOWER(?)", "%"+walletAddress+"%")
	}
	if isWinner >= 0 {
		model = model.Where("is_winner", isWinner)
	}
	if groupId > 0 {
		model = model.Where("group_id", groupId)
	}

	total, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	var players []*entity.ApgPlayer
	err = model.Order("id ASC").
		Page(page, pageSize).
		Scan(&players)

	return players, total, err
}

func (d *playerDao) GetClaimedRecords(ctx context.Context, matchId int64, walletAddress string, page, pageSize int) ([]*entity.ApgPlayer, int, error) {
	model := db.GetDB().Model("apg_mint_player").Where("has_claimed_reward", true)

	if matchId > 0 {
		model = model.Where("match_id", matchId)
	}
	if walletAddress != "" {
		model = model.Where("LOWER(wallet_address) LIKE LOWER(?)", "%"+walletAddress+"%")
	}

	total, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	var players []*entity.ApgPlayer
	err = model.Order("updated_at DESC").
		Page(page, pageSize).
		Scan(&players)

	return players, total, err
}

func (d *playerDao) GetStakedRecords(ctx context.Context, matchId int64, walletAddress string, page, pageSize int) ([]*entity.ApgPlayer, int, error) {
	model := db.GetDB().Model("apg_mint_player").Where("stake_status", 1)

	if matchId > 0 {
		model = model.Where("match_id", matchId)
	}
	if walletAddress != "" {
		model = model.Where("LOWER(wallet_address) LIKE LOWER(?)", "%"+walletAddress+"%")
	}

	total, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	var players []*entity.ApgPlayer
	err = model.Order("stake_time DESC").
		Page(page, pageSize).
		Scan(&players)

	return players, total, err
}

func (d *playerDao) DeleteByMatchId(ctx context.Context, matchId int64) (int64, error) {
	result, err := db.GetDB().Model("apg_mint_player").
		Where("match_id", matchId).
		Delete()
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *playerDao) GetUserMatchPlayers(ctx context.Context, userId int64, matchId int64) ([]*entity.ApgPlayer, error) {
	var players []*entity.ApgPlayer
	err := db.GetDB().Model("apg_mint_player").
		Where("user_id", userId).
		Where("match_id", matchId).
		Where("join_status", 0).
		Order("created_at ASC").
		Scan(&players)
	return players, err
}

func (d *playerDao) GetMatchTotalPoolByToken(ctx context.Context, matchId int64) (map[string]string, error) {
	type PoolSum struct {
		PaymentToken string `json:"payment_token"`
		TotalAmount  string `json:"total_amount"`
	}
	var results []PoolSum
	err := db.GetDB().Model("apg_mint_player").
		Where("match_id", matchId).
		Where("join_status", 0).
		Fields("payment_token", "SUM(payment_amount::numeric) as total_amount").
		Group("payment_token").
		Scan(&results)
	if err != nil {
		return nil, err
	}

	poolMap := make(map[string]string)
	for _, r := range results {
		poolMap[r.PaymentToken] = r.TotalAmount
	}
	return poolMap, nil
}

func (d *playerDao) GetMatchWinners(ctx context.Context, matchId int64, page, pageSize int) ([]*entity.ApgPlayer, int, error) {
	model := db.GetDB().Model("apg_mint_player").
		Where("match_id", matchId).
		Where("is_winner", 1).
		Where("join_status", 0)

	total, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	var winners []*entity.ApgPlayer
	err = model.Order("id ASC").
		Page(page, pageSize).
		Scan(&winners)

	return winners, total, err
}

func (d *playerDao) GetMatchLosers(ctx context.Context, matchId int64) ([]*entity.ApgPlayer, error) {
	var losers []*entity.ApgPlayer
	err := db.GetDB().Model("apg_mint_player").
		Where("match_id", matchId).
		Where("is_winner", 2).
		Where("join_status", 0).
		Order("group_id ASC").
		Scan(&losers)
	return losers, err
}

// ResetPlayersForRedraw 重置玩家开奖状态（用于重新开奖）
func (d *playerDao) ResetPlayersForRedraw(ctx context.Context, matchId int64) (int64, error) {
	result, err := db.GetDB().Model("apg_mint_player").
		Where("match_id", matchId).
		Update(gdb.Map{
			"group_id":              0,
			"is_winner":             0,
			"result_time":           nil,
			"refund_token":          nil,
			"refund_amount":         nil,
			"winner_reward_token":   nil,
			"winner_reward_amount":  nil,
			"referral_pool_token":   nil,
			"referral_pool_amount":  nil,
			"power_purchase_token":  nil,
			"power_purchase_amount": nil,
			"cobo_reserve_token":    nil,
			"cobo_reserve_amount":   nil,
			"updated_at":            gtime.Now(),
		})
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *playerDao) GetUserRecordsByWalletAddress(ctx context.Context, walletAddress string, page, pageSize int) ([]*entity.ApgPlayer, int, error) {
	model := db.GetDB().Model("apg_mint_player").Where("LOWER(wallet_address) = LOWER(?)", walletAddress)

	total, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	var players []*entity.ApgPlayer
	err = model.Order("created_at DESC").
		Page(page, pageSize).
		Scan(&players)

	return players, total, err
}
