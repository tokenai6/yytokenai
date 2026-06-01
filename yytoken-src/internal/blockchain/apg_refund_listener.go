package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"XWFrame/internal/dao"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/frame/db"
	"XWFrame/internal/repository"
	apgRepo "XWFrame/internal/repository/apg"
	apgService "XWFrame/internal/service/apg"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gcache"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

// ApgRefundEventListener APG领取/退款事件监听器
// 监听 RefundUserLp 事件，更新玩家领取状态
type ApgRefundEventListener struct {
	ctx             context.Context
	db              gdb.DB
	cache           *gcache.Cache
	rpcClient       *RpcClient
	config          *EventListenerConfig
	contractAddress common.Address
	eventID         common.Hash
	apgRepo         apgRepo.IApgRepository
	balanceRepo     repository.IBalanceRepository
	accountTypeDao  dao.IAccountTypeDao
}

// RefundUserLpEvent RefundUserLp 事件数据结构
// 事件签名: RefundUserLp(address indexed user, address indexed token, address indexed rewardToken,
//           uint256 index, uint256 lpAmount, uint256 tokenAmount, uint256 rewardAmount, uint8 isWin)
type RefundUserLpEvent struct {
	User         common.Address // indexed - 用户地址
	Token        common.Address // indexed - 领取代币地址
	RewardToken  common.Address // indexed - 奖励代币地址
	Index        *big.Int       // data - 合约索引
	LpAmount     *big.Int       // data - LP数量
	TokenAmount  *big.Int       // data - 代币数量
	RewardAmount *big.Int       // data - 奖励数量
	IsWin        uint8          // data - 是否赢家
	TxHash       string         // 交易哈希
	BlockNumber  uint64         // 区块号
}

// NewApgRefundEventListener 创建APG领取/退款事件监听器
func NewApgRefundEventListener(ctx context.Context, db gdb.DB, cache *gcache.Cache, rpcClient *RpcClient, config *EventListenerConfig) *ApgRefundEventListener {
	contractAddress := common.HexToAddress(config.ContractMintGroup)
	eventID := common.HexToHash(consts.BlockchainEventIDApgRefund)

	glog.Infof(ctx, "[APG领取监听] 初始化 - 合约地址: %s, 事件ID: %s",
		contractAddress.Hex(), eventID.Hex())

	return &ApgRefundEventListener{
		ctx:             ctx,
		db:              db,
		cache:           cache,
		rpcClient:       rpcClient,
		config:          config,
		contractAddress: contractAddress,
		eventID:         eventID,
		apgRepo:         apgRepo.NewApgRepository(),
		balanceRepo:     repository.NewBalanceRepository(),
		accountTypeDao:  dao.NewAccountTypeDao(),
	}
}

// processApgRefundEvent 处理单个APG领取/退款事件
func (l *ApgRefundEventListener) processApgRefundEvent(ctx context.Context, event types.Log) error {
	glog.Infof(ctx, "[APG领取监听] 开始处理领取事件: tx=%s, logIndex=%d", event.TxHash.Hex(), event.Index)

	refundEvent, err := l.parseRefundUserLpEvent(event)
	if err != nil {
		return fmt.Errorf("解析事件失败: %v", err)
	}

	glog.Infof(ctx, "[APG领取监听] 事件详情: user=%s, index=%s, lpAmount=%s, tokenAmount=%s, rewardAmount=%s, tx=%s",
		refundEvent.User.Hex(),
		refundEvent.Index.String(),
		l.convertFromWei(refundEvent.LpAmount).String(),
		l.convertFromWei(refundEvent.TokenAmount).String(),
		l.convertFromWei(refundEvent.RewardAmount).String(),
		refundEvent.TxHash)

	if err := l.updatePlayerRefundStatus(ctx, refundEvent); err != nil {
		return fmt.Errorf("更新玩家领取状态失败: %v", err)
	}

	glog.Infof(ctx, "[APG领取监听] 事件处理完成: tx=%s, index=%s",
		refundEvent.TxHash, refundEvent.Index.String())

	return nil
}

// parseRefundUserLpEvent 解析 RefundUserLp 事件
// 事件结构: RefundUserLp(address indexed user, address indexed token, address indexed rewardToken,
//
//	uint256 index, uint256 lpAmount, uint256 tokenAmount, uint256 rewardAmount, uint8 isWin)
//
// data 共 160 bytes (5个字段)
func (l *ApgRefundEventListener) parseRefundUserLpEvent(event types.Log) (*RefundUserLpEvent, error) {
	if len(event.Topics) < 4 {
		return nil, fmt.Errorf("事件Topics数量不足: 期望4, 实际%d", len(event.Topics))
	}

	user := common.BytesToAddress(event.Topics[1].Bytes())
	token := common.BytesToAddress(event.Topics[2].Bytes())
	rewardToken := common.BytesToAddress(event.Topics[3].Bytes())

	data := event.Data
	if len(data) < 160 {
		return nil, fmt.Errorf("事件数据长度不足: 期望160, 实际%d", len(data))
	}

	index := new(big.Int).SetBytes(data[0:32])
	lpAmount := new(big.Int).SetBytes(data[32:64])
	tokenAmount := new(big.Int).SetBytes(data[64:96])
	rewardAmount := new(big.Int).SetBytes(data[96:128])
	isWin := uint8(new(big.Int).SetBytes(data[128:160]).Uint64())

	return &RefundUserLpEvent{
		User:         user,
		Token:        token,
		RewardToken:  rewardToken,
		Index:        index,
		LpAmount:     lpAmount,
		TokenAmount:  tokenAmount,
		RewardAmount: rewardAmount,
		IsWin:        isWin,
		TxHash:       event.TxHash.Hex(),
		BlockNumber:  event.BlockNumber,
	}, nil
}

// updatePlayerRefundStatus 更新玩家领取状态
func (l *ApgRefundEventListener) updatePlayerRefundStatus(ctx context.Context, event *RefundUserLpEvent) error {
	contractIndex := event.Index.Int64()
	txHash := event.TxHash

	player, err := l.apgRepo.GetPlayerByContractIndex(ctx, contractIndex)
	if err != nil {
		return fmt.Errorf("查询玩家记录失败: %v", err)
	}
	if player == nil {
		glog.Warningf(ctx, "[APG领取监听] 玩家记录不存在，跳过: contractIndex=%d, tx=%s", contractIndex, txHash)
		return nil
	}

	if player.HasRefund {
		glog.Debugf(ctx, "[APG领取监听] 玩家已领取，跳过: playerId=%d, contractIndex=%d", player.Id, contractIndex)
		return nil
	}

	return db.GetDB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 赢家需要扣减奖励余额
		if player.IsWinner == apgService.StatusWinner && player.WinnerRewardAmount != "" {
			rewardAmount, err := decimal.NewFromString(player.WinnerRewardAmount)
			if err != nil {
				return fmt.Errorf("解析奖励金额失败: %v", err)
			}

			if rewardAmount.GreaterThan(decimal.Zero) {
				accountTypeID, err := l.accountTypeDao.GetIdByTypeAndSymbol(ctx, apgService.MintRewardAccountType, player.WinnerRewardToken)
				if err != nil {
					return fmt.Errorf("查询账户类型失败: %v", err)
				}
				if accountTypeID == 0 {
					glog.Warningf(ctx, "[APG领取监听] 账户类型不存在，跳过扣减: %s/%s", apgService.MintRewardAccountType, player.WinnerRewardToken)
				} else {
					withdrawParams := &repository.BalanceOperationParams{
						UserID:         player.UserId,
						AccountTypeID:  accountTypeID,
						AccountType:    apgService.MintRewardAccountType,
						Symbol:         player.WinnerRewardToken,
						Amount:         rewardAmount,
						ChangeType:     "apg_mint_claim",
						RelatedOrderNo: fmt.Sprintf("CLAIM-%d", player.Id),
						RelatedId:      player.Id,
						Remark:         fmt.Sprintf("APG铸币领取奖励(事件);playerId:%d", player.Id),
						OperatorType:   "system",
					}
					if err := l.balanceRepo.Withdraw(ctx, tx, withdrawParams); err != nil {
						glog.Errorf(ctx, "[APG领取监听] 扣减奖励余额失败: %v, playerId=%d", err, player.Id)
					}
				}
			}
		}

		// 更新玩家记录
		updateData := gdb.Map{
			"has_refund":         true,
			"refund_tx_hash":     txHash,
			"refund_time":        gtime.Now(),
			"has_claimed_reward": true,
		}

		walletAddress := strings.ToLower(event.User.Hex())
		if strings.ToLower(player.WalletAddress) != walletAddress {
			glog.Warningf(ctx, "[APG领取监听] 领取地址与玩家地址不匹配: player=%s, event=%s",
				player.WalletAddress, walletAddress)
		}

		return l.apgRepo.UpdatePlayer(ctx, player.Id, updateData)
	})
}

// convertFromWei 金额精度转换
func (l *ApgRefundEventListener) convertFromWei(weiValue *big.Int) decimal.Decimal {
	return decimal.NewFromBigInt(weiValue, -int32(l.config.Decimals))
}
