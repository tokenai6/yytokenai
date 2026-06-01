package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"XWFrame/internal/dao/apg"
	"XWFrame/internal/entity"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/repository"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gcache"
	"github.com/gogf/gf/v2/os/glog"
)

// ApgReferralRewardEventListener 推荐奖励领取事件监听器
// 监听 RefundReferralReward 事件，更新推荐奖励领取状态
type ApgReferralRewardEventListener struct {
	ctx               context.Context
	db                gdb.DB
	cache             *gcache.Cache
	rpcClient         *RpcClient
	config            *EventListenerConfig
	contractAddress   common.Address
	eventID           common.Hash
	referralRewardDao apg.IReferralRewardDao
	userRepo          repository.IUserRepository
}

// RefundReferralRewardEvent RefundReferralReward 事件数据结构
// 事件签名: RefundReferralReward(address indexed sender, address indexed token, uint256 amount, uint256 nonce)
type RefundReferralRewardEvent struct {
	Sender      common.Address // 领取用户地址 (indexed, 在Topics[1])
	Token       common.Address // 代币地址 (indexed, 在Topics[2])
	Amount      *big.Int       // 领取数量
	Nonce       *big.Int       // nonce
	TxHash      string         // 交易哈希
	BlockNumber uint64         // 区块号
}

// NewApgReferralRewardEventListener 创建推荐奖励领取事件监听器
func NewApgReferralRewardEventListener(ctx context.Context, db gdb.DB, cache *gcache.Cache, rpcClient *RpcClient, config *EventListenerConfig) *ApgReferralRewardEventListener {
	contractAddress := common.HexToAddress(config.ContractMintGroup)
	eventID := common.HexToHash(consts.BlockchainEventIDApgReferralReward)

	glog.Infof(ctx, "[推荐奖励领取监听] 初始化 - 合约地址: %s, 事件ID: %s",
		contractAddress.Hex(), eventID.Hex())

	return &ApgReferralRewardEventListener{
		ctx:               ctx,
		db:                db,
		cache:             cache,
		rpcClient:         rpcClient,
		config:            config,
		contractAddress:   contractAddress,
		eventID:           eventID,
		referralRewardDao: apg.ReferralReward,
		userRepo:          repository.NewUserRepository(),
	}
}

// processApgReferralRewardEvent 处理单个推荐奖励领取事件
func (l *ApgReferralRewardEventListener) processApgReferralRewardEvent(ctx context.Context, event types.Log) error {
	glog.Infof(ctx, "[推荐奖励领取监听] 开始处理事件: tx=%s, logIndex=%d", event.TxHash.Hex(), event.Index)

	rewardEvent, err := l.parseRefundReferralRewardEvent(event)
	if err != nil {
		return fmt.Errorf("解析事件失败: %v", err)
	}

	glog.Infof(ctx, "[推荐奖励领取监听] 事件详情: sender=%s, token=%s, amount=%s, nonce=%s, tx=%s",
		rewardEvent.Sender.Hex(),
		rewardEvent.Token.Hex(),
		rewardEvent.Amount.String(),
		rewardEvent.Nonce.String(),
		rewardEvent.TxHash)

	if err := l.updateReferralRewardStatus(ctx, rewardEvent); err != nil {
		return fmt.Errorf("更新推荐奖励状态失败: %v", err)
	}

	glog.Infof(ctx, "[推荐奖励领取监听] 事件处理完成: tx=%s", rewardEvent.TxHash)

	return nil
}

// parseRefundReferralRewardEvent 解析 RefundReferralReward 事件
// 事件结构: RefundReferralReward(address indexed sender, address indexed token, uint256 amount, uint256 nonce)
// Topics[0]: 事件签名, Topics[1]: sender, Topics[2]: token
// Data: amount (32 bytes) + nonce (32 bytes) = 64 bytes
func (l *ApgReferralRewardEventListener) parseRefundReferralRewardEvent(event types.Log) (*RefundReferralRewardEvent, error) {
	if len(event.Topics) < 3 {
		return nil, fmt.Errorf("Topics数量不足: 期望3, 实际%d", len(event.Topics))
	}

	data := event.Data
	if len(data) < 64 {
		return nil, fmt.Errorf("事件数据长度不足: 期望64, 实际%d", len(data))
	}

	sender := common.HexToAddress(event.Topics[1].Hex())
	token := common.HexToAddress(event.Topics[2].Hex())
	amount := new(big.Int).SetBytes(data[0:32])
	nonce := new(big.Int).SetBytes(data[32:64])

	return &RefundReferralRewardEvent{
		Sender:      sender,
		Token:       token,
		Amount:      amount,
		Nonce:       nonce,
		TxHash:      event.TxHash.Hex(),
		BlockNumber: event.BlockNumber,
	}, nil
}

// updateReferralRewardStatus 更新推荐奖励领取状态
// 使用 userId + nonce 精确匹配签名时标记的记录
func (l *ApgReferralRewardEventListener) updateReferralRewardStatus(ctx context.Context, event *RefundReferralRewardEvent) error {
	walletAddress := strings.ToLower(event.Sender.Hex())
	nonce := event.Nonce.String()
	txHash := event.TxHash

	// 查询用户
	user, err := l.userRepo.GetUserByWalletAddress(ctx, walletAddress)
	if err != nil {
		return fmt.Errorf("查询用户失败: %v", err)
	}
	if user == nil {
		glog.Warningf(ctx, "[推荐奖励领取监听] 用户不存在，跳过: wallet=%s, tx=%s", walletAddress, txHash)
		return nil
	}

	// 通过 userId + nonce 更新奖励状态
	rowsAffected, err := l.referralRewardDao.UpdateClaimStatusByNonce(ctx, user.Id, nonce, entity.ReferralClaimStatusClaimed, txHash)
	if err != nil {
		return fmt.Errorf("更新奖励状态失败: %v", err)
	}

	if rowsAffected == 0 {
		glog.Warningf(ctx, "[推荐奖励领取监听] 未找到匹配记录，跳过: userId=%d, nonce=%s, tx=%s", user.Id, nonce, txHash)
		return nil
	}

	glog.Infof(ctx, "[推荐奖励领取监听] 成功更新 %d 条奖励记录状态为已领取: userId=%d, nonce=%s, tx=%s",
		rowsAffected, user.Id, nonce, txHash)

	return nil
}
