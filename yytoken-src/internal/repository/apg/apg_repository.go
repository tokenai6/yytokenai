package apg

import (
	"context"
	"time"

	"XWFrame/internal/dao/apg"
	"XWFrame/internal/entity"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gtime"
)

type IApgRepository interface {
	GetPoolConfig(ctx context.Context, poolType int) (*entity.ApgPoolConfig, error)
	GetAllPoolConfigs(ctx context.Context) ([]*entity.ApgPoolConfig, error)
	GetAllowedTokens(ctx context.Context, poolType int) ([]string, error)
	GetTokenInfoByAddress(ctx context.Context, address string) (*entity.ApgMintTokenInfo, error)

	CreateMatch(ctx context.Context, match *entity.ApgMatch) error
	GetMatch(ctx context.Context, id int64) (*entity.ApgMatch, error)
	GetMatchByDateRoundPool(ctx context.Context, matchDate string, round, poolType int) (*entity.ApgMatch, error)
	GetUnfinishedMatch(ctx context.Context, matchDate string, round, poolType int) (*entity.ApgMatch, error)
	GetCurrentActiveMatch(ctx context.Context, poolType int) (*entity.ApgMatch, error)
	GetMatchByTime(ctx context.Context, poolType int, t time.Time) (*entity.ApgMatch, error)
	GetNextMatch(ctx context.Context, poolType int) (*entity.ApgMatch, error)
	LockMatch(ctx context.Context, matchId int64) (bool, error)
	UpdateMatch(ctx context.Context, id int64, data gdb.Map) error

	CreatePlayer(ctx context.Context, player *entity.ApgPlayer) error
	GetPlayer(ctx context.Context, id int64) (*entity.ApgPlayer, error)
	CheckTxHashExists(ctx context.Context, txHash string) (bool, error)
	CheckContractIndexAndTxHashExists(ctx context.Context, contractIndex int64, txHash string) (bool, error)
	GetPlayerByContractIndex(ctx context.Context, contractIndex int64) (*entity.ApgPlayer, error)
	GetMatchPlayers(ctx context.Context, matchId int64) ([]*entity.ApgPlayer, error)
	GetMatchSuccessPlayers(ctx context.Context, matchId int64) ([]*entity.ApgPlayer, error)
	UpdatePlayer(ctx context.Context, id int64, data gdb.Map) error
	GetUserRecords(ctx context.Context, userId int64, page, pageSize int) ([]*entity.ApgPlayer, int, error)
	GetUserRecordsByContractId(ctx context.Context, userId int64, contractId int64, page, pageSize int) ([]*entity.ApgPlayer, int, error)
	GetUserRecordsByContractIds(ctx context.Context, userId int64, contractIds []int64, page, pageSize int) ([]*entity.ApgPlayer, int, error)
	GetUserTodayParticipationCount(ctx context.Context, userId int64, poolType int) (int, error)
	CountUserMatchSuccessfulJoins(ctx context.Context, userId int64, matchId int64) (int, error)
	GetUserRefundRecords(ctx context.Context, userId int64, page, pageSize int) ([]*entity.ApgPlayer, int, error)
	GetUserTotalRefunds(ctx context.Context, userId int64) (map[string]string, error)
	GetUserMatchPlayers(ctx context.Context, userId int64, matchId int64) ([]*entity.ApgPlayer, error)
	GetMatchTotalPoolByToken(ctx context.Context, matchId int64) (map[string]string, error)

	CreateReferralReward(ctx context.Context, tx gdb.TX, reward *entity.ApgReferralReward) error
	BatchCreateReferralRewards(ctx context.Context, tx gdb.TX, rewards []*entity.ApgReferralReward) error
	GetUserReferralRewards(ctx context.Context, userId int64, page, pageSize int) ([]*entity.ApgReferralReward, int, error)
	GetUserReferralRewardTotal(ctx context.Context, userId int64) (map[string]string, error)
	GetUserUnclaimedReferralTotal(ctx context.Context, userId int64) (map[string]string, error)
	GetUserUnclaimedReferralTotalByTokenId(ctx context.Context, userId int64, tokenId int) (string, error)
	GetUserReferralRewardSummary(ctx context.Context, userId int64) ([]*apg.ReferralRewardSummary, error)
	UpdateReferralClaimStatusByUserAndTokenId(ctx context.Context, userId int64, tokenId int, status, txHash string) (int64, error)
	ResetReferralClaimingTimeout(ctx context.Context, timeoutMinutes int) (int64, error)

	CreatePowerRecord(ctx context.Context, tx gdb.TX, record *entity.ApgPowerRecord) error
	GetUserPowerRecords(ctx context.Context, userId int64, page, pageSize int) ([]*entity.ApgPowerRecord, int, error)
	GetUserTotalPower(ctx context.Context, userId int64) (map[string]string, error)

	GetMatchList(ctx context.Context, poolType int, startDate, endDate string, page, pageSize int) ([]*entity.ApgMatch, int, error)
	GetMatchWinners(ctx context.Context, matchId int64, page, pageSize int) ([]*entity.ApgPlayer, int, error)
	GetMatchLosers(ctx context.Context, matchId int64) ([]*entity.ApgPlayer, error)

	// Admin APIs
	GetAdminMatchList(ctx context.Context, matchDate string, poolType, isFinished, page, pageSize int) ([]*entity.ApgMatch, int, error)
	GetAdminPlayerList(ctx context.Context, matchId int64, walletAddress string, isWinner, groupId, page, pageSize int) ([]*entity.ApgPlayer, int, error)
	GetAdminReferralList(ctx context.Context, matchId int64, receiverWalletAddress string, page, pageSize int) ([]*entity.ApgReferralReward, int, error)
	GetClaimedRecords(ctx context.Context, matchId int64, walletAddress string, page, pageSize int) ([]*entity.ApgPlayer, int, error)
	GetStakedRecords(ctx context.Context, matchId int64, walletAddress string, page, pageSize int) ([]*entity.ApgPlayer, int, error)
	GetMatchStatistics(ctx context.Context, matchId int64) ([]*entity.ApgMatchStatistics, error)
	BatchCreateMatchStatistics(ctx context.Context, statistics []*entity.ApgMatchStatistics) error

	// Test APIs
	DeleteMatchData(ctx context.Context, matchId int64) (map[string]int64, error)
	ResetMatchForRedraw(ctx context.Context, matchId int64) (map[string]int64, error)
	DeletePlayersByMatchId(ctx context.Context, matchId int64) (int64, error)
}

type apgRepository struct{}

func NewApgRepository() IApgRepository {
	return &apgRepository{}
}

func (r *apgRepository) GetPoolConfig(ctx context.Context, poolType int) (*entity.ApgPoolConfig, error) {
	return apg.PoolConfig.GetByPoolType(ctx, poolType)
}

func (r *apgRepository) GetAllPoolConfigs(ctx context.Context) ([]*entity.ApgPoolConfig, error) {
	return apg.PoolConfig.GetAllEnabled(ctx)
}

func (r *apgRepository) GetAllowedTokens(ctx context.Context, poolType int) ([]string, error) {
	tokenInfos, err := apg.TokenInfo.GetEnabled(ctx)
	if err != nil {
		return nil, err
	}

	tokens := make([]string, 0, len(tokenInfos))
	for _, t := range tokenInfos {
		tokens = append(tokens, t.Symbol)
	}
	return tokens, nil
}

func (r *apgRepository) GetTokenInfoByAddress(ctx context.Context, address string) (*entity.ApgMintTokenInfo, error) {
	return apg.TokenInfo.GetByAddress(ctx, address)
}

func (r *apgRepository) CreateMatch(ctx context.Context, match *entity.ApgMatch) error {
	return apg.Match.Create(ctx, match)
}

func (r *apgRepository) GetMatch(ctx context.Context, id int64) (*entity.ApgMatch, error) {
	return apg.Match.GetById(ctx, id)
}

func (r *apgRepository) GetMatchByDateRoundPool(ctx context.Context, matchDate string, round, poolType int) (*entity.ApgMatch, error) {
	return apg.Match.GetByDateRoundPool(ctx, matchDate, round, poolType)
}

func (r *apgRepository) GetUnfinishedMatch(ctx context.Context, matchDate string, round, poolType int) (*entity.ApgMatch, error) {
	return apg.Match.GetUnfinishedMatch(ctx, matchDate, round, poolType)
}

func (r *apgRepository) GetCurrentActiveMatch(ctx context.Context, poolType int) (*entity.ApgMatch, error) {
	now := gtime.NewFromTime(utils.GetShanghaiTime())
	return apg.Match.GetCurrentActiveMatch(ctx, poolType, now)
}

func (r *apgRepository) GetMatchByTime(ctx context.Context, poolType int, t time.Time) (*entity.ApgMatch, error) {
	gt := gtime.NewFromTime(t)
	return apg.Match.GetCurrentActiveMatch(ctx, poolType, gt)
}

func (r *apgRepository) GetNextMatch(ctx context.Context, poolType int) (*entity.ApgMatch, error) {
	now := gtime.NewFromTime(utils.GetShanghaiTime())
	return apg.Match.GetNextMatch(ctx, poolType, now)
}

func (r *apgRepository) LockMatch(ctx context.Context, matchId int64) (bool, error) {
	affected, err := apg.Match.LockMatch(ctx, matchId)
	return affected > 0, err
}

func (r *apgRepository) UpdateMatch(ctx context.Context, id int64, data gdb.Map) error {
	return apg.Match.UpdateById(ctx, id, data)
}

func (r *apgRepository) CreatePlayer(ctx context.Context, player *entity.ApgPlayer) error {
	return apg.Player.Create(ctx, player)
}

func (r *apgRepository) GetPlayer(ctx context.Context, id int64) (*entity.ApgPlayer, error) {
	return apg.Player.GetById(ctx, id)
}

func (r *apgRepository) CheckTxHashExists(ctx context.Context, txHash string) (bool, error) {
	return apg.Player.ExistsByTxHash(ctx, txHash)
}

func (r *apgRepository) CheckContractIndexAndTxHashExists(ctx context.Context, contractIndex int64, txHash string) (bool, error) {
	return apg.Player.ExistsByContractIndexAndTxHash(ctx, contractIndex, txHash)
}

func (r *apgRepository) GetPlayerByContractIndex(ctx context.Context, contractIndex int64) (*entity.ApgPlayer, error) {
	return apg.Player.GetByContractIndex(ctx, contractIndex)
}

func (r *apgRepository) GetMatchPlayers(ctx context.Context, matchId int64) ([]*entity.ApgPlayer, error) {
	return apg.Player.GetMatchPlayers(ctx, matchId)
}

func (r *apgRepository) GetMatchSuccessPlayers(ctx context.Context, matchId int64) ([]*entity.ApgPlayer, error) {
	return apg.Player.GetMatchSuccessPlayers(ctx, matchId)
}

func (r *apgRepository) UpdatePlayer(ctx context.Context, id int64, data gdb.Map) error {
	return apg.Player.UpdateById(ctx, id, data)
}

func (r *apgRepository) GetUserRecords(ctx context.Context, userId int64, page, pageSize int) ([]*entity.ApgPlayer, int, error) {
	return apg.Player.GetUserRecords(ctx, userId, page, pageSize)
}

func (r *apgRepository) GetUserRecordsByContractId(ctx context.Context, userId int64, contractId int64, page, pageSize int) ([]*entity.ApgPlayer, int, error) {
	return apg.Player.GetUserRecordsByContractId(ctx, userId, contractId, page, pageSize)
}

func (r *apgRepository) GetUserRecordsByContractIds(ctx context.Context, userId int64, contractIds []int64, page, pageSize int) ([]*entity.ApgPlayer, int, error) {
	return apg.Player.GetUserRecordsByContractIds(ctx, userId, contractIds, page, pageSize)
}

func (r *apgRepository) GetUserTodayParticipationCount(ctx context.Context, userId int64, poolType int) (int, error) {
	now := utils.GetShanghaiTime()
	startOfDayStr := now.Format("2006-01-02") + " 00:00:00"
	startOfDay := gtime.NewFromStr(startOfDayStr)
	endOfDay := startOfDay.Add(gtime.D)

	return apg.Player.CountUserTodayParticipation(ctx, userId, poolType, startOfDay, endOfDay)
}

func (r *apgRepository) CountUserMatchSuccessfulJoins(ctx context.Context, userId int64, matchId int64) (int, error) {
	return apg.Player.CountUserMatchSuccessfulJoins(ctx, userId, matchId)
}

func (r *apgRepository) CreateReferralReward(ctx context.Context, tx gdb.TX, reward *entity.ApgReferralReward) error {
	return apg.ReferralReward.Create(ctx, reward)
}

func (r *apgRepository) BatchCreateReferralRewards(ctx context.Context, tx gdb.TX, rewards []*entity.ApgReferralReward) error {
	return apg.ReferralReward.BatchCreate(ctx, tx, rewards)
}

func (r *apgRepository) GetUserReferralRewards(ctx context.Context, userId int64, page, pageSize int) ([]*entity.ApgReferralReward, int, error) {
	return apg.ReferralReward.GetUserRewards(ctx, userId, page, pageSize)
}

func (r *apgRepository) GetUserReferralRewardTotal(ctx context.Context, userId int64) (map[string]string, error) {
	return apg.ReferralReward.GetUserTotalRewards(ctx, userId)
}

func (r *apgRepository) GetUserUnclaimedReferralTotal(ctx context.Context, userId int64) (map[string]string, error) {
	return apg.ReferralReward.GetUserUnclaimedTotal(ctx, userId)
}

func (r *apgRepository) GetUserUnclaimedReferralTotalByTokenId(ctx context.Context, userId int64, tokenId int) (string, error) {
	return apg.ReferralReward.GetUserUnclaimedTotalByTokenId(ctx, userId, tokenId)
}

func (r *apgRepository) GetUserReferralRewardSummary(ctx context.Context, userId int64) ([]*apg.ReferralRewardSummary, error) {
	return apg.ReferralReward.GetUserRewardSummary(ctx, userId)
}

func (r *apgRepository) UpdateReferralClaimStatusByUserAndTokenId(ctx context.Context, userId int64, tokenId int, status, txHash string) (int64, error) {
	return apg.ReferralReward.UpdateClaimStatusByUserAndTokenId(ctx, userId, tokenId, status, txHash)
}

func (r *apgRepository) ResetReferralClaimingTimeout(ctx context.Context, timeoutMinutes int) (int64, error) {
	return apg.ReferralReward.ResetClaimingTimeout(ctx, timeoutMinutes)
}

func (r *apgRepository) CreatePowerRecord(ctx context.Context, tx gdb.TX, record *entity.ApgPowerRecord) error {
	return apg.PowerRecord.Create(ctx, record)
}

func (r *apgRepository) GetUserPowerRecords(ctx context.Context, userId int64, page, pageSize int) ([]*entity.ApgPowerRecord, int, error) {
	return apg.PowerRecord.GetUserRecords(ctx, userId, page, pageSize)
}

func (r *apgRepository) GetUserTotalPower(ctx context.Context, userId int64) (map[string]string, error) {
	return apg.PowerRecord.GetUserTotalPower(ctx, userId)
}

func (r *apgRepository) GetMatchList(ctx context.Context, poolType int, startDate, endDate string, page, pageSize int) ([]*entity.ApgMatch, int, error) {
	return apg.Match.GetMatchList(ctx, poolType, startDate, endDate, page, pageSize)
}

func (r *apgRepository) GetMatchWinners(ctx context.Context, matchId int64, page, pageSize int) ([]*entity.ApgPlayer, int, error) {
	return apg.Player.GetMatchWinners(ctx, matchId, page, pageSize)
}

func (r *apgRepository) GetMatchLosers(ctx context.Context, matchId int64) ([]*entity.ApgPlayer, error) {
	return apg.Player.GetMatchLosers(ctx, matchId)
}

func (r *apgRepository) GetUserRefundRecords(ctx context.Context, userId int64, page, pageSize int) ([]*entity.ApgPlayer, int, error) {
	return apg.Player.GetUserRefundRecords(ctx, userId, page, pageSize)
}

func (r *apgRepository) GetUserTotalRefunds(ctx context.Context, userId int64) (map[string]string, error) {
	return apg.Player.GetUserTotalRefunds(ctx, userId)
}

func (r *apgRepository) GetUserMatchPlayers(ctx context.Context, userId int64, matchId int64) ([]*entity.ApgPlayer, error) {
	return apg.Player.GetUserMatchPlayers(ctx, userId, matchId)
}

func (r *apgRepository) GetMatchTotalPoolByToken(ctx context.Context, matchId int64) (map[string]string, error) {
	return apg.Player.GetMatchTotalPoolByToken(ctx, matchId)
}

func (r *apgRepository) GetAdminMatchList(ctx context.Context, matchDate string, poolType, isFinished, page, pageSize int) ([]*entity.ApgMatch, int, error) {
	return apg.Match.GetAdminMatchList(ctx, matchDate, poolType, isFinished, page, pageSize)
}

func (r *apgRepository) GetAdminPlayerList(ctx context.Context, matchId int64, walletAddress string, isWinner, groupId, page, pageSize int) ([]*entity.ApgPlayer, int, error) {
	return apg.Player.GetPlayerList(ctx, matchId, walletAddress, isWinner, groupId, page, pageSize)
}

func (r *apgRepository) GetAdminReferralList(ctx context.Context, matchId int64, receiverWalletAddress string, page, pageSize int) ([]*entity.ApgReferralReward, int, error) {
	return apg.ReferralReward.GetAdminReferralList(ctx, matchId, receiverWalletAddress, page, pageSize)
}

func (r *apgRepository) GetClaimedRecords(ctx context.Context, matchId int64, walletAddress string, page, pageSize int) ([]*entity.ApgPlayer, int, error) {
	return apg.Player.GetClaimedRecords(ctx, matchId, walletAddress, page, pageSize)
}

func (r *apgRepository) GetStakedRecords(ctx context.Context, matchId int64, walletAddress string, page, pageSize int) ([]*entity.ApgPlayer, int, error) {
	return apg.Player.GetStakedRecords(ctx, matchId, walletAddress, page, pageSize)
}

func (r *apgRepository) GetMatchStatistics(ctx context.Context, matchId int64) ([]*entity.ApgMatchStatistics, error) {
	return apg.MatchStatistics.GetByMatchId(ctx, matchId)
}

func (r *apgRepository) BatchCreateMatchStatistics(ctx context.Context, statistics []*entity.ApgMatchStatistics) error {
	return apg.MatchStatistics.BatchCreate(ctx, statistics)
}

func (r *apgRepository) DeleteMatchData(ctx context.Context, matchId int64) (map[string]int64, error) {
	result := make(map[string]int64)

	powerCount, err := apg.PowerRecord.DeleteByMatchId(ctx, matchId)
	if err != nil {
		return nil, err
	}
	result["power_records"] = powerCount

	referralCount, err := apg.ReferralReward.DeleteByMatchId(ctx, matchId)
	if err != nil {
		return nil, err
	}
	result["referral_rewards"] = referralCount

	statsCount, err := apg.MatchStatistics.DeleteByMatchId(ctx, matchId)
	if err != nil {
		return nil, err
	}
	result["statistics"] = statsCount

	playerCount, err := apg.Player.DeleteByMatchId(ctx, matchId)
	if err != nil {
		return nil, err
	}
	result["players"] = playerCount

	err = apg.Match.UpdateById(ctx, matchId, gdb.Map{
		"player_count": 0,
		"group_count":  0,
		"is_finished":  0,
		"finish_time":  nil,
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// ResetMatchForRedraw 重置场次以便重新开奖（不删除玩家记录，只重置状态）
func (r *apgRepository) ResetMatchForRedraw(ctx context.Context, matchId int64) (map[string]int64, error) {
	result := make(map[string]int64)

	// 1. 删除推荐奖励记录
	referralCount, err := apg.ReferralReward.DeleteByMatchId(ctx, matchId)
	if err != nil {
		return nil, err
	}
	result["referral_rewards"] = referralCount

	// 2. 删除统计数据
	statsCount, err := apg.MatchStatistics.DeleteByMatchId(ctx, matchId)
	if err != nil {
		return nil, err
	}
	result["statistics"] = statsCount

	// 3. 重置玩家开奖状态（不删除）
	playerCount, err := apg.Player.ResetPlayersForRedraw(ctx, matchId)
	if err != nil {
		return nil, err
	}
	result["players_reset"] = playerCount

	// 4. 重置场次开奖状态
	err = apg.Match.UpdateById(ctx, matchId, gdb.Map{
		"player_count": 0,
		"group_count":  0,
		"is_finished":  0,
		"updated_at":   gtime.Now(),
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (r *apgRepository) DeletePlayersByMatchId(ctx context.Context, matchId int64) (int64, error) {
	return apg.Player.DeleteByMatchId(ctx, matchId)
}
