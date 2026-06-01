package staking_v2

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/repository"
	coboRepo "XWFrame/internal/repository/cobo"
	"XWFrame/internal/service/staking_v2/model"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

const (
	defaultMinStakeAmount                  = "50"
	defaultRewardLimitMultiplier           = "4"
	stakeV2UserLockTTLSeconds              = 10
	stakeV2BalanceChangeTypeStake          = consts.ChangeTypeStakingV2Stake
	stakeV2BalanceChangeTypeDirectReward   = consts.ChangeTypeStakingV2ReferralDirect
	stakeV2BalanceChangeTypeIndirectReward = consts.ChangeTypeStakingV2ReferralIndirect
	stakeV2BalanceChangeTypeBurnToVertex   = consts.ChangeTypeStakingV2ReferralBurnToVertex

	stakeV2RewardTypeDirect   = "direct"
	stakeV2RewardTypeIndirect = "indirect"
	// Reward cap multiplier is fixed at 4x by business rule.
	stakeV2FixedRewardLimitMultiplier = int64(4)
)

var errStakeIdempotentConflict = errors.New("stake idempotent conflict")

type IStakingV2Service interface {
	CreateStake(ctx context.Context, req *model.CreateStakeReq) (*model.CreateStakeRes, error)
	GetOverview(ctx context.Context, userID int64) (*model.StakeOverviewRes, error)
	GetMyStakes(ctx context.Context, req *model.StakeListReq) (*model.StakeListRes, error)
	HasActiveStake(ctx context.Context, userID int64) (bool, error)
	ApplyRewardWithCapTx(ctx context.Context, tx gdb.TX, userID int64, wantedAmount decimal.Decimal) (decimal.Decimal, error)
	GetActiveStakeQuote(ctx context.Context, req *model.ActiveStakeQuoteReq) (*model.ActiveStakeQuoteRes, error)
	DistributeLeaderRewards(ctx context.Context, bizDate time.Time) (int, error)
}

type stakingV2Service struct {
	balanceRepo coboRepo.IBalanceRepository
	userRepo    repository.IUserRepository
}

func NewStakingV2Service() IStakingV2Service {
	return &stakingV2Service{
		balanceRepo: coboRepo.NewBalanceRepository(),
		userRepo:    repository.NewUserRepository(),
	}
}

func (s *stakingV2Service) CreateStake(ctx context.Context, req *model.CreateStakeReq) (*model.CreateStakeRes, error) {
	if req == nil || req.UserID <= 0 {
		return nil, gerror.New("参数错误")
	}

	requestID := strings.TrimSpace(req.RequestID)
	if len(requestID) > 64 {
		return nil, gerror.New("request_id 长度不能超过64")
	}

	if requestID != "" {
		existing, err := s.getStakeOrderByRequestID(ctx, req.UserID, requestID)
		if err != nil {
			return nil, gerror.Wrap(err, "查询幂等记录失败")
		}
		if existing != nil {
			return existing, nil
		}
	}

	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		return nil, gerror.New("质押金额格式错误")
	}

	minAmount := s.mustReadDecimalConfig(ctx, "staking_v2_min_amount", defaultMinStakeAmount)
	if amount.LessThan(minAmount) {
		return nil, gerror.Newf("最低质押金额为 %s USDT", minAmount.String())
	}

	// Fixed business rule: max reward = principal * 4.
	rewardLimitMultiplier := decimal.NewFromInt(stakeV2FixedRewardLimitMultiplier)

	user, err := s.userRepo.GetUserById(ctx, req.UserID)
	if err != nil {
		return nil, gerror.Wrap(err, "查询用户失败")
	}
	if user == nil {
		return nil, gerror.New("用户不存在")
	}

	hasActive, err := s.HasActiveStake(ctx, req.UserID)
	if err != nil {
		return nil, gerror.Wrap(err, "查询有效质押失败")
	}
	if hasActive {
		return nil, gerror.New("You have an active stake order. Please wait until it completes before creating a new one.")
	}

	locked, lockErr := s.acquireUserStakeLock(ctx, req.UserID)
	if lockErr != nil {
		return nil, gerror.Wrap(lockErr, "获取质押锁失败")
	}
	if !locked {
		return nil, gerror.New("质押请求过于频繁，请稍后重试")
	}
	defer s.releaseUserStakeLock(ctx, req.UserID)

	rewardLimitIncrement := amount.Mul(rewardLimitMultiplier)
	directRate := s.mustReadDecimalConfig(ctx, "referral_direct_rate", "0.08")
	indirectRate := s.mustReadDecimalConfig(ctx, "referral_indirect_rate", "0.04")
	teamPoolRate := s.mustReadDecimalConfig(ctx, "referral_team_pool_rate", "0.03")

	now := time.Now()
	res := &model.CreateStakeRes{}
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		balance, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, req.UserID, "USDT")
		if err != nil {
			return gerror.Wrap(err, "查询余额失败")
		}
		if balance == nil || balance.AvailableAmount.LessThan(amount) {
			return gerror.New("余额不足")
		}

		if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, req.UserID, "USDT", amount.Neg(), decimal.Zero); err != nil {
			return gerror.Wrap(err, "扣除USDT失败")
		}

		balanceAfterRow, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, req.UserID, "USDT")
		if err != nil {
			return gerror.Wrap(err, "查询扣减后余额失败")
		}
		if balanceAfterRow == nil {
			return gerror.New("余额记录不存在")
		}

		orderData := g.Map{
			"user_id":                 req.UserID,
			"request_id":              requestID,
			"amount":                  amount,
			"source_type":             consts.StakingV2SourceTypeManualStake,
			"status":                  consts.StakingV2StatusActive,
			"total_reward":            decimal.Zero,
			"reward_limit_multiplier": rewardLimitMultiplier,
			"created_at":              now,
			"updated_at":              now,
		}

		orderID, err := tx.Model("staking_v2_order").Ctx(ctx).Data(orderData).InsertAndGetId()
		if err != nil {
			if requestID != "" && strings.Contains(strings.ToLower(err.Error()), "idx_staking_v2_order_user_request_id") {
				return errStakeIdempotentConflict
			}
			return gerror.Wrap(err, "创建质押订单失败")
		}

		if err := s.createCoboBalanceChangeLogTx(ctx, tx, req.UserID, "USDT", amount.Neg(), balance.AvailableAmount, balanceAfterRow.AvailableAmount, requestID, orderID); err != nil {
			return gerror.Wrap(err, "写入余额审计日志失败")
		}

		if err := s.distributeReferralRewardsTx(ctx, tx, user, orderID, amount, directRate, indirectRate, teamPoolRate); err != nil {
			return gerror.Wrap(err, "发放推荐奖励失败")
		}

		_, err = tx.Exec(`
			INSERT INTO staking_v2_user_stats (
				user_id, total_stake_amount, total_reward_earned, reward_limit, is_capped, created_at, updated_at
			)
			VALUES (?, ?, 0, ?, FALSE, ?, ?)
			ON CONFLICT (user_id) DO UPDATE SET
				total_stake_amount = staking_v2_user_stats.total_stake_amount + EXCLUDED.total_stake_amount,
				reward_limit = staking_v2_user_stats.reward_limit + EXCLUDED.reward_limit,
				is_capped = FALSE,
				capped_at = NULL,
				updated_at = EXCLUDED.updated_at
		`, req.UserID, amount, rewardLimitIncrement, now, now)
		if err != nil {
			return gerror.Wrap(err, "更新用户质押统计失败")
		}

		res.OrderID = orderID
		res.Amount = amount.String()
		res.Status = consts.StakingV2StatusActive
		res.SourceType = consts.StakingV2SourceTypeManualStake
		res.CreatedAt = now.Format("2006-01-02 15:04:05")
		res.BalanceLeft = balanceAfterRow.AvailableAmount.String()

		return nil
	})
	if err != nil {
		if errors.Is(err, errStakeIdempotentConflict) {
			existing, getErr := s.getStakeOrderByRequestID(ctx, req.UserID, requestID)
			if getErr != nil {
				return nil, gerror.Wrap(getErr, "读取幂等记录失败")
			}
			if existing == nil {
				return nil, gerror.New("幂等请求记录不存在")
			}
			return existing, nil
		}
		return nil, err
	}

	return res, nil
}

func (s *stakingV2Service) distributeReferralRewardsTx(
	ctx context.Context,
	tx gdb.TX,
	buyer *entity.UserEntity,
	stakeOrderID int64,
	stakeAmount decimal.Decimal,
	directRate decimal.Decimal,
	indirectRate decimal.Decimal,
	teamPoolRate decimal.Decimal,
) error {
	if buyer == nil || stakeOrderID <= 0 || !stakeAmount.GreaterThan(decimal.Zero) {
		return nil
	}

	buyerWallet := strings.ToLower(strings.TrimSpace(buyer.WalletAddress))
	l1, err := s.getParentUserByWalletAddressTx(ctx, tx, buyer.ParentWalletAddress)
	if err != nil {
		return err
	}
	if l1 != nil && l1.Id > 0 && l1.Id != buyer.Id && directRate.GreaterThan(decimal.Zero) {
		reward := stakeAmount.Mul(directRate)
		if err := s.issueReferralRewardTx(ctx, tx, l1, buyer, stakeOrderID, stakeAmount, directRate, reward, stakeV2RewardTypeDirect, stakeV2BalanceChangeTypeDirectReward); err != nil {
			return err
		}

		l2, getL2Err := s.getParentUserByWalletAddressTx(ctx, tx, l1.ParentWalletAddress)
		if getL2Err != nil {
			return getL2Err
		}
		if l2 != nil && l2.Id > 0 && l2.Id != buyer.Id && l2.Id != l1.Id && indirectRate.GreaterThan(decimal.Zero) {
			indirectReward := stakeAmount.Mul(indirectRate)
			if err := s.issueReferralRewardTx(ctx, tx, l2, buyer, stakeOrderID, stakeAmount, indirectRate, indirectReward, stakeV2RewardTypeIndirect, stakeV2BalanceChangeTypeIndirectReward); err != nil {
				return err
			}
		} else if indirectRate.GreaterThan(decimal.Zero) {
			// 间推上级不存在，间推奖励作为烧伤给顶号
			indirectReward := stakeAmount.Mul(indirectRate)
			if err := s.transferOverflowToVertexTx(ctx, tx, buyer, stakeOrderID, indirectReward, stakeV2RewardTypeIndirect); err != nil {
				return err
			}
		}
	}

	if teamPoolRate.GreaterThan(decimal.Zero) {
		poolAmount := stakeAmount.Mul(teamPoolRate)
		if poolAmount.GreaterThan(decimal.Zero) {
			_, err := tx.Exec(`
				INSERT INTO public_team_pool (source_type, source_id, user_id, amount, status, created_at)
				VALUES (?, ?, ?, ?, 0, ?)
			`, "stake_reward", stakeOrderID, buyer.Id, poolAmount, time.Now())
			if err != nil {
				return gerror.Wrap(err, "写入团队池失败")
			}
			g.Log().Infof(ctx, "[staking_v2] 团队池入账: user_id=%d wallet=%s order_id=%d amount=%s", buyer.Id, buyerWallet, stakeOrderID, poolAmount.String())
		}
	}

	return nil
}

func (s *stakingV2Service) issueReferralRewardTx(
	ctx context.Context,
	tx gdb.TX,
	receiver *entity.UserEntity,
	fromUser *entity.UserEntity,
	stakeOrderID int64,
	stakeAmount decimal.Decimal,
	rewardRate decimal.Decimal,
	wantedAmount decimal.Decimal,
	rewardType string,
	changeType string,
) error {
	if receiver == nil || fromUser == nil || !wantedAmount.GreaterThan(decimal.Zero) {
		return nil
	}

	grantedAmount, statsBefore, statsAfter, err := s.applyRewardWithCapTx(ctx, tx, receiver.Id, wantedAmount)
	if err != nil {
		return err
	}
	if !grantedAmount.GreaterThan(decimal.Zero) {
		return s.transferOverflowToVertexTx(ctx, tx, fromUser, stakeOrderID, wantedAmount, rewardType)
	}

	if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, receiver.Id, "USDT", grantedAmount, decimal.Zero); err != nil {
		return gerror.Wrap(err, "发放推荐奖励入账失败")
	}

	balanceAfterRow, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, receiver.Id, "USDT")
	if err != nil {
		return gerror.Wrap(err, "查询推荐奖励后余额失败")
	}
	receiverBalanceAfter := decimal.Zero
	if balanceAfterRow != nil {
		receiverBalanceAfter = balanceAfterRow.AvailableAmount
	}

	relatedOrderNo := "STAKEV2-REF-" + utils.GenerateSnowflakeId()
	_, err = tx.Exec(`
		INSERT INTO cobo_balance_change_log
		(user_id, symbol, change_type, amount, before_balance, after_balance, related_order_no, related_id, remark, operator_type, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, receiver.Id, "USDT", changeType, grantedAmount, receiverBalanceAfter.Sub(grantedAmount), receiverBalanceAfter, relatedOrderNo, stakeOrderID, "staking_v2 referral reward", consts.OperatorTypeSystem, time.Now())
	if err != nil {
		return gerror.Wrap(err, "写入推荐奖励审计日志失败")
	}

	_, err = tx.Exec(`
		INSERT INTO staking_v2_reward_record
		(user_id, wallet_address, from_user_id, from_wallet_address, reward_type, stake_order_id, stake_amount, reward_rate, reward_amount, symbol, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'USDT', 1, ?)
	`, receiver.Id, strings.ToLower(strings.TrimSpace(receiver.WalletAddress)), fromUser.Id, strings.ToLower(strings.TrimSpace(fromUser.WalletAddress)), rewardType, stakeOrderID, stakeAmount, rewardRate, grantedAmount, time.Now())
	if err != nil {
		return gerror.Wrap(err, "写入推荐奖励记录失败")
	}

	g.Log().Infof(ctx, "[staking_v2] 推荐奖励发放: type=%s receiver=%d from=%d wanted=%s granted=%s stats_before=%s stats_after=%s", rewardType, receiver.Id, fromUser.Id, wantedAmount.String(), grantedAmount.String(), statsBefore.String(), statsAfter.String())
	overflow := wantedAmount.Sub(grantedAmount)
	if overflow.GreaterThan(decimal.Zero) {
		if err := s.transferOverflowToVertexTx(ctx, tx, fromUser, stakeOrderID, overflow, rewardType); err != nil {
			return err
		}
	}
	return nil
}

func (s *stakingV2Service) transferOverflowToVertexTx(ctx context.Context, tx gdb.TX, fromUser *entity.UserEntity, stakeOrderID int64, overflow decimal.Decimal, rewardType string) error {
	if !overflow.GreaterThan(decimal.Zero) {
		return nil
	}
	vertexUser, err := s.getVertexUserTx(ctx, tx)
	if err != nil {
		return err
	}
	if vertexUser == nil || vertexUser.Id <= 0 {
		return gerror.New("未配置首码地址用户，无法接收烧伤奖金")
	}

	if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, vertexUser.Id, "USDT", overflow, decimal.Zero); err != nil {
		return gerror.Wrap(err, "烧伤奖金转入首码地址失败")
	}
	balanceAfterRow, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, vertexUser.Id, "USDT")
	if err != nil {
		return gerror.Wrap(err, "查询首码地址余额失败")
	}
	balanceAfter := decimal.Zero
	if balanceAfterRow != nil {
		balanceAfter = balanceAfterRow.AvailableAmount
	}
	relatedOrderNo := "STAKEV2-BURN-" + utils.GenerateSnowflakeId()
	_, err = tx.Exec(`
		INSERT INTO cobo_balance_change_log
		(user_id, symbol, change_type, amount, before_balance, after_balance, related_order_no, related_id, remark, operator_type, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, vertexUser.Id, "USDT", stakeV2BalanceChangeTypeBurnToVertex, overflow, balanceAfter.Sub(overflow), balanceAfter, relatedOrderNo, stakeOrderID, "staking_v2 overflow burn to vertex", consts.OperatorTypeSystem, time.Now())
	if err != nil {
		return gerror.Wrap(err, "写入烧伤奖金审计日志失败")
	}

	fromUserID := int64(0)
	fromWallet := ""
	if fromUser != nil {
		fromUserID = fromUser.Id
		fromWallet = strings.ToLower(strings.TrimSpace(fromUser.WalletAddress))
	}
	_, err = tx.Exec(`
		INSERT INTO staking_v2_reward_record
		(user_id, wallet_address, from_user_id, from_wallet_address, reward_type, stake_order_id, stake_amount, reward_rate, reward_amount, symbol, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'USDT', 1, ?)
	`, vertexUser.Id, strings.ToLower(strings.TrimSpace(vertexUser.WalletAddress)), fromUserID, fromWallet, "burn_to_vertex_"+rewardType, stakeOrderID, decimal.Zero, decimal.Zero, overflow, time.Now())
	if err != nil {
		return gerror.Wrap(err, "写入烧伤奖金记录失败")
	}

	g.Log().Infof(ctx, "[staking_v2] 烧伤奖金入首码地址: reward_type=%s from_user=%d vertex_user=%d overflow=%s", rewardType, fromUserID, vertexUser.Id, overflow.String())
	return nil
}

func (s *stakingV2Service) getVertexUserTx(ctx context.Context, tx gdb.TX) (*entity.UserEntity, error) {
	row := &entity.UserEntity{}
	err := tx.Model("user_info").Ctx(ctx).
		Fields("id", "wallet_address").
		Where("id", 1).
		Limit(1).
		Scan(row)
	if err != nil {
		return nil, gerror.Wrap(err, "查询首码地址用户失败")
	}
	if row.Id == 0 {
		return nil, nil
	}
	return row, nil
}

func (s *stakingV2Service) applyRewardWithCapTx(ctx context.Context, tx gdb.TX, userID int64, wantedAmount decimal.Decimal) (decimal.Decimal, decimal.Decimal, decimal.Decimal, error) {
	if !wantedAmount.GreaterThan(decimal.Zero) {
		return decimal.Zero, decimal.Zero, decimal.Zero, nil
	}

	type statsRow struct {
		ID                int64           `json:"id"`
		TotalRewardEarned decimal.Decimal `json:"total_reward_earned"`
		RewardLimit       decimal.Decimal `json:"reward_limit"`
		IsCapped          bool            `json:"is_capped"`
	}
	stats := &statsRow{}
	if err := tx.Model("staking_v2_user_stats").Ctx(ctx).Where("user_id", userID).LockUpdate().Scan(stats); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return decimal.Zero, decimal.Zero, decimal.Zero, nil
		}
		return decimal.Zero, decimal.Zero, decimal.Zero, gerror.Wrap(err, "查询用户收益统计失败")
	}
	if stats.ID == 0 {
		return decimal.Zero, stats.TotalRewardEarned, stats.TotalRewardEarned, nil
	}

	type orderRow struct {
		ID          int64           `json:"id"`
		Amount      decimal.Decimal `json:"amount"`
		TotalReward decimal.Decimal `json:"total_reward"`
	}
	orders := make([]*orderRow, 0)
	if err := tx.Model("staking_v2_order").Ctx(ctx).
		Where("user_id = ? AND status = ?", userID, consts.StakingV2StatusActive).
		OrderAsc("id").
		LockUpdate().
		Scan(&orders); err != nil {
		return decimal.Zero, stats.TotalRewardEarned, stats.TotalRewardEarned, gerror.Wrap(err, "查询可发放订单失败")
	}
	if len(orders) == 0 {
		if _, err := tx.Model("staking_v2_user_stats").Ctx(ctx).Where("user_id", userID).Data(g.Map{"is_capped": true, "capped_at": time.Now(), "updated_at": time.Now()}).Update(); err != nil {
			return decimal.Zero, stats.TotalRewardEarned, stats.TotalRewardEarned, gerror.Wrap(err, "更新封顶状态失败")
		}
		return decimal.Zero, stats.TotalRewardEarned, stats.TotalRewardEarned, nil
	}

	remaining := wantedAmount
	granted := decimal.Zero
	for _, order := range orders {
		if !remaining.GreaterThan(decimal.Zero) {
			break
		}
		// Fixed business rule: each order reward cap is always amount * 4.
		orderLimit := order.Amount.Mul(decimal.NewFromInt(stakeV2FixedRewardLimitMultiplier))
		available := orderLimit.Sub(order.TotalReward)
		if available.LessThanOrEqual(decimal.Zero) {
			_, err := tx.Model("staking_v2_order").Ctx(ctx).Where("id", order.ID).Data(g.Map{"status": consts.StakingV2StatusCapped, "updated_at": time.Now()}).Update()
			if err != nil {
				return decimal.Zero, stats.TotalRewardEarned, stats.TotalRewardEarned, gerror.Wrap(err, "更新订单封顶状态失败")
			}
			continue
		}

		grant := remaining
		if grant.GreaterThan(available) {
			grant = available
		}
		newTotalReward := order.TotalReward.Add(grant)
		status := consts.StakingV2StatusActive
		if !newTotalReward.LessThan(orderLimit) {
			status = consts.StakingV2StatusCapped
		}

		_, err := tx.Model("staking_v2_order").Ctx(ctx).Where("id", order.ID).Data(g.Map{
			"total_reward": newTotalReward,
			"status":       status,
			"updated_at":   time.Now(),
		}).Update()
		if err != nil {
			return decimal.Zero, stats.TotalRewardEarned, stats.TotalRewardEarned, gerror.Wrap(err, "更新订单收益失败")
		}
		remaining = remaining.Sub(grant)
		granted = granted.Add(grant)
	}

	if !granted.GreaterThan(decimal.Zero) {
		remainingActiveCount, err := tx.Model("staking_v2_order").Ctx(ctx).Where("user_id = ? AND status = ?", userID, consts.StakingV2StatusActive).Count()
		if err != nil {
			return decimal.Zero, stats.TotalRewardEarned, stats.TotalRewardEarned, gerror.Wrap(err, "统计有效订单失败")
		}
		if remainingActiveCount == 0 && !stats.IsCapped {
			_, err := tx.Model("staking_v2_user_stats").Ctx(ctx).Where("user_id", userID).Data(g.Map{
				"is_capped":  true,
				"capped_at":  time.Now(),
				"updated_at": time.Now(),
			}).Update()
			if err != nil {
				return decimal.Zero, stats.TotalRewardEarned, stats.TotalRewardEarned, gerror.Wrap(err, "更新用户封顶状态失败")
			}
		}
		return decimal.Zero, stats.TotalRewardEarned, stats.TotalRewardEarned, nil
	}

	newStatsReward := stats.TotalRewardEarned.Add(granted)
	remainingActiveCount, err := tx.Model("staking_v2_order").Ctx(ctx).Where("user_id = ? AND status = ?", userID, consts.StakingV2StatusActive).Count()
	if err != nil {
		return decimal.Zero, stats.TotalRewardEarned, newStatsReward, gerror.Wrap(err, "统计有效订单失败")
	}
	isCapped := remainingActiveCount == 0
	data := g.Map{
		"total_reward_earned": newStatsReward,
		"is_capped":           isCapped,
		"updated_at":          time.Now(),
	}
	if isCapped {
		data["capped_at"] = time.Now()
	}
	_, err = tx.Model("staking_v2_user_stats").Ctx(ctx).Where("user_id", userID).Data(data).Update()
	if err != nil {
		return decimal.Zero, stats.TotalRewardEarned, newStatsReward, gerror.Wrap(err, "更新用户收益统计失败")
	}

	return granted, stats.TotalRewardEarned, newStatsReward, nil
}

func (s *stakingV2Service) ApplyRewardWithCapTx(ctx context.Context, tx gdb.TX, userID int64, wantedAmount decimal.Decimal) (decimal.Decimal, error) {
	if tx == nil {
		return decimal.Zero, gerror.New("transaction is required")
	}
	granted, _, _, err := s.applyRewardWithCapTx(ctx, tx, userID, wantedAmount)
	if err != nil {
		return decimal.Zero, err
	}
	return granted, nil
}

func (s *stakingV2Service) getParentUserByWalletAddressTx(ctx context.Context, tx gdb.TX, walletAddress string) (*entity.UserEntity, error) {
	wallet := strings.ToLower(strings.TrimSpace(walletAddress))
	if wallet == "" {
		return nil, nil
	}
	parent := &entity.UserEntity{}
	err := tx.Model("user_info").Ctx(ctx).
		Fields("id", "wallet_address", "parent_wallet_address").
		Where("wallet_address", wallet).
		Limit(1).
		Scan(parent)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, gerror.Wrap(err, "查询上级用户失败")
	}
	if parent.Id == 0 {
		return nil, nil
	}
	return parent, nil
}

func (s *stakingV2Service) GetOverview(ctx context.Context, userID int64) (*model.StakeOverviewRes, error) {
	if userID <= 0 {
		return nil, gerror.New("参数错误")
	}

	type userStatsRow struct {
		TotalStakeAmount  decimal.Decimal `json:"total_stake_amount"`
		TotalRewardEarned decimal.Decimal `json:"total_reward_earned"`
		RewardLimit       decimal.Decimal `json:"reward_limit"`
		IsCapped          bool            `json:"is_capped"`
	}

	var row userStatsRow
	err := g.DB().Model("staking_v2_user_stats").Ctx(ctx).
		Where("user_id", userID).
		Scan(&row)
	if err != nil && err != sql.ErrNoRows {
		return nil, gerror.Wrap(err, "查询质押统计失败")
	}

	activeCount, err := g.DB().Model("staking_v2_order").Ctx(ctx).
		Where("user_id = ? AND status = ?", userID, consts.StakingV2StatusActive).
		Count()
	if err != nil {
		return nil, gerror.Wrap(err, "查询有效订单数量失败")
	}

	cappedCount, err := g.DB().Model("staking_v2_order").Ctx(ctx).
		Where("user_id = ? AND status = ?", userID, consts.StakingV2StatusCapped).
		Count()
	if err != nil {
		return nil, gerror.Wrap(err, "查询封顶订单数量失败")
	}

	progress := decimal.Zero
	if row.RewardLimit.GreaterThan(decimal.Zero) {
		progress = row.TotalRewardEarned.Div(row.RewardLimit)
		if progress.GreaterThan(decimal.NewFromInt(1)) {
			progress = decimal.NewFromInt(1)
		}
	}

	return &model.StakeOverviewRes{
		TotalStakeAmount:  row.TotalStakeAmount.String(),
		TotalRewardEarned: row.TotalRewardEarned.String(),
		RewardLimit:       row.RewardLimit.String(),
		RewardProgress:    progress.String(),
		IsCapped:          row.IsCapped,
		ActiveOrderCount:  activeCount,
		CappedOrders:      cappedCount,
	}, nil
}

func (s *stakingV2Service) GetMyStakes(ctx context.Context, req *model.StakeListReq) (*model.StakeListRes, error) {
	if req == nil || req.UserID <= 0 {
		return nil, gerror.New("参数错误")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	m := g.DB().Model("staking_v2_order").Ctx(ctx).Where("user_id", req.UserID)
	if req.Status > 0 {
		m = m.Where("status", req.Status)
	}
	if req.SourceType > 0 {
		m = m.Where("source_type", req.SourceType)
	}

	total, err := m.Count()
	if err != nil {
		return nil, gerror.Wrap(err, "查询质押列表总数失败")
	}

	type stakeRow struct {
		ID          int64           `json:"id"`
		Amount      decimal.Decimal `json:"amount"`
		Status      int             `json:"status"`
		SourceType  int             `json:"source_type"`
		TotalReward decimal.Decimal `json:"total_reward"`
		CreatedAt   time.Time       `json:"created_at"`
		IsGift      int             `json:"is_gift"`
	}

	var rows []*stakeRow
	err = m.OrderDesc("id").Page(req.Page, req.PageSize).Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "查询质押列表失败")
	}

	list := make([]*model.StakeOrderItem, 0, len(rows))
	for _, r := range rows {
		list = append(list, &model.StakeOrderItem{
			ID:          r.ID,
			Amount:      r.Amount.String(),
			Status:      r.Status,
			SourceType:  r.SourceType,
			TotalReward: r.TotalReward.String(),
			// Fixed business rule: displayed cap is always principal * 4.
			RewardLimit: r.Amount.Mul(decimal.NewFromInt(stakeV2FixedRewardLimitMultiplier)).String(),
			CreatedAt:   r.CreatedAt.Format("2006-01-02 15:04:05"),
			IsGift:      r.IsGift,
		})
	}

	return &model.StakeListRes{
		List:     list,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func (s *stakingV2Service) HasActiveStake(ctx context.Context, userID int64) (bool, error) {
	count, err := g.DB().Model("staking_v2_order").Ctx(ctx).
		Where("user_id = ? AND status = ?", userID, consts.StakingV2StatusActive).
		Count()
	if err != nil {
		return false, gerror.Wrap(err, "查询有效质押失败")
	}
	return count > 0, nil
}

func (s *stakingV2Service) GetActiveStakeQuote(ctx context.Context, req *model.ActiveStakeQuoteReq) (*model.ActiveStakeQuoteRes, error) {
	if req == nil || req.UserID <= 0 {
		return nil, gerror.New("参数错误")
	}

	var row struct {
		TotalAmount  decimal.Decimal `json:"total_amount"`
		TotalReward  decimal.Decimal `json:"total_reward"`
	}
	err := g.DB().Model("staking_v2_order").Ctx(ctx).
		Fields("COALESCE(SUM(amount), 0) AS total_amount, COALESCE(SUM(total_reward), 0) AS total_reward").
		Where("user_id = ? AND status = ?", req.UserID, consts.StakingV2StatusActive).
		Scan(&row)
	if err != nil {
		return nil, gerror.Wrap(err, "查询有效质押订单失败")
	}

	if row.TotalAmount.IsZero() {
		return &model.ActiveStakeQuoteRes{HasActive: false}, nil
	}

	limit := row.TotalAmount.Mul(decimal.NewFromInt(stakeV2FixedRewardLimitMultiplier))
	rest := limit.Sub(row.TotalReward)
	if rest.LessThan(decimal.Zero) {
		rest = decimal.Zero
	}

	return &model.ActiveStakeQuoteRes{
		HasActive:   true,
		Principal:   row.TotalAmount.String(),
		RestQuota:   rest.String(),
		TotalReward: row.TotalReward.String(),
	}, nil
}

func (s *stakingV2Service) mustReadDecimalConfig(ctx context.Context, key, fallback string) decimal.Decimal {
	val := fallback
	config, err := repository.NewConfigRepository().GetByKeyName(ctx, key)
	if err == nil && config != nil && strings.TrimSpace(config.KeyValue) != "" {
		val = strings.TrimSpace(config.KeyValue)
	}
	d, err := decimal.NewFromString(val)
	if err != nil {
		d, _ = decimal.NewFromString(fallback)
	}
	return d
}

func (s *stakingV2Service) getStakeOrderByRequestID(ctx context.Context, userID int64, requestID string) (*model.CreateStakeRes, error) {
	type row struct {
		ID         int64           `json:"id"`
		Amount     decimal.Decimal `json:"amount"`
		Status     int             `json:"status"`
		SourceType int             `json:"source_type"`
		CreatedAt  time.Time       `json:"created_at"`
	}

	var order row
	err := g.DB().Model("staking_v2_order").Ctx(ctx).
		Where("user_id = ? AND request_id = ?", userID, requestID).
		OrderDesc("id").
		Limit(1).
		Scan(&order)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if order.ID == 0 {
		return nil, nil
	}

	balance, err := s.balanceRepo.GetOrCreate(ctx, userID, "USDT")
	if err != nil {
		return nil, err
	}

	return &model.CreateStakeRes{
		OrderID:     order.ID,
		Amount:      order.Amount.String(),
		Status:      order.Status,
		SourceType:  order.SourceType,
		CreatedAt:   order.CreatedAt.Format("2006-01-02 15:04:05"),
		BalanceLeft: balance.AvailableAmount.String(),
	}, nil
}

func (s *stakingV2Service) createCoboBalanceChangeLogTx(ctx context.Context, tx gdb.TX, userID int64, symbol string, amount, beforeBalance, afterBalance decimal.Decimal, requestID string, relatedID int64) error {
	relatedOrderNo := requestID
	if relatedOrderNo == "" {
		relatedOrderNo = "STAKEV2-" + time.Now().Format("20060102150405")
	}

	_, err := tx.Exec(`
		INSERT INTO cobo_balance_change_log
			(user_id, symbol, change_type, amount, before_balance, after_balance, related_order_no, related_id, remark, operator_type, created_at)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, userID, symbol, stakeV2BalanceChangeTypeStake, amount, beforeBalance, afterBalance, relatedOrderNo, relatedID, "staking_v2 stake", consts.OperatorTypeUser, time.Now())
	if err != nil {
		g.Log().Errorf(ctx, "[staking_v2] 写入cobo余额变更日志失败: user_id=%d, err=%v", userID, err)
	}
	return err
}

func (s *stakingV2Service) acquireUserStakeLock(ctx context.Context, userID int64) (bool, error) {
	redisClient := g.Redis()
	if redisClient == nil {
		return false, gerror.New("redis not initialized")
	}

	key := fmt.Sprintf("staking_v2:stake:lock:%d", userID)
	result, err := redisClient.Do(ctx, "SET", key, "1", "NX", "EX", stakeV2UserLockTTLSeconds)
	if err != nil {
		return false, err
	}
	if result.IsNil() {
		return false, nil
	}
	return result.String() == "OK", nil
}

func (s *stakingV2Service) releaseUserStakeLock(ctx context.Context, userID int64) {
	redisClient := g.Redis()
	if redisClient == nil {
		return
	}

	key := fmt.Sprintf("staking_v2:stake:lock:%d", userID)
	if _, err := redisClient.Do(ctx, "DEL", key); err != nil {
		g.Log().Warningf(ctx, "[staking_v2] 释放质押锁失败: user_id=%d, err=%v", userID, err)
	}
}
