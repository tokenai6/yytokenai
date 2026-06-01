package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"XWFrame/internal/frame/consts"
	apgRepo "XWFrame/internal/repository/apg"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gcache"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/shopspring/decimal"
)

// MintStakeEventListener APG Mint 质押事件监听器
// 监听 StakeFromMintGame 事件（输家60%代币兑换U购买算力）
type MintStakeEventListener struct {
	ctx             context.Context
	db              gdb.DB
	cache           *gcache.Cache
	rpcClient       *RpcClient
	config          *EventListenerConfig
	contractAddress common.Address
	eventID         common.Hash
	stakeListener   *StakeEventListener
	apgRepo         apgRepo.IApgRepository
}

// StakeFromMintGameEvent StakeFromMintGame 事件数据结构
type StakeFromMintGameEvent struct {
	User       common.Address // indexed - 用户地址
	StakeIndex *big.Int       // indexed - 质押索引
	GroupType  *big.Int       // indexed - 拼团类型
	GroupIndex *big.Int       // data - 合约索引（对应 player 的 contract_index）
	UsdtAmount *big.Int       // data - USDT 金额
	StakeType  uint8          // data - 质押类型
	RexPrice   *big.Int       // data - REX 价格
	Timestamp  int64          // data - 时间戳
	TxHash     string         // 交易哈希
	BlockNumber uint64        // 区块号
	EventIndex uint           // 事件索引
}

// NewMintStakeEventListener 创建 APG Mint 质押事件监听器
func NewMintStakeEventListener(
	ctx context.Context,
	db gdb.DB,
	cache *gcache.Cache,
	rpcClient *RpcClient,
	config *EventListenerConfig,
) *MintStakeEventListener {
	contractAddress := common.HexToAddress(config.ContractMintGroup)
	eventID := common.HexToHash(consts.BlockchainEventIDMintStake)

	glog.Infof(ctx, "[APG Mint质押监听] 初始化 - 合约地址: %s, 事件ID: %s",
		contractAddress.Hex(), eventID.Hex())

	return &MintStakeEventListener{
		ctx:             ctx,
		db:              db,
		cache:           cache,
		rpcClient:       rpcClient,
		config:          config,
		contractAddress: contractAddress,
		eventID:         eventID,
		stakeListener:   NewStakeEventListener(ctx, db, cache, rpcClient, config),
		apgRepo:         apgRepo.NewApgRepository(),
	}
}

// processMintStakeEvent 处理 APG Mint 质押事件
// 事件签名: StakeFromMintGame(address indexed user, uint256 indexed stakeIndex, uint256 indexed groupType,
//
//	uint256 groupIndex, uint256 usdtAmount, uint8 stakeType, uint256 rexPrice, uint256 timestamp)
func (l *MintStakeEventListener) processMintStakeEvent(ctx context.Context, event types.Log) error {
	glog.Infof(ctx, "[APG Mint质押监听] 开始处理质押事件: tx=%s, logIndex=%d", event.TxHash.Hex(), event.Index)

	stakeEvent, err := l.parseStakeFromMintGameEvent(event)
	if err != nil {
		return fmt.Errorf("解析事件失败: %v", err)
	}

	glog.Infof(ctx, "[APG Mint质押监听] 事件详情: user=%s, groupIndex=%s, usdtAmount=%s, stakeType=%d, tx=%s",
		stakeEvent.User.Hex(),
		stakeEvent.GroupIndex.String(),
		l.convertFromWei(stakeEvent.UsdtAmount).String(),
		stakeEvent.StakeType,
		stakeEvent.TxHash)

	// 检查是否已处理
	exists, err := l.checkIfProcessed(ctx, event.TxHash.Hex(), event.BlockNumber)
	if err != nil {
		return fmt.Errorf("检查重复失败: %v", err)
	}
	if exists {
		glog.Debugf(ctx, "[APG Mint质押监听] 事件已处理，跳过: tx=%s, block=%d", event.TxHash.Hex(), event.BlockNumber)
		return nil
	}

	if err := l.createStakeForMintGame(ctx, stakeEvent); err != nil {
		return fmt.Errorf("创建质押失败: %v", err)
	}

	glog.Infof(ctx, "[APG Mint质押监听] 事件处理完成: tx=%s, groupIndex=%s",
		stakeEvent.TxHash, stakeEvent.GroupIndex.String())

	return nil
}

// parseStakeFromMintGameEvent 解析 StakeFromMintGame 事件
func (l *MintStakeEventListener) parseStakeFromMintGameEvent(event types.Log) (*StakeFromMintGameEvent, error) {
	if len(event.Topics) < 4 {
		return nil, fmt.Errorf("事件Topics数量不足: 期望4, 实际%d", len(event.Topics))
	}

	user := common.BytesToAddress(event.Topics[1].Bytes())
	stakeIndex := new(big.Int).SetBytes(event.Topics[2].Bytes())
	groupType := new(big.Int).SetBytes(event.Topics[3].Bytes())

	data := event.Data
	if len(data) < 160 {
		return nil, fmt.Errorf("事件数据长度不足: 期望160, 实际%d", len(data))
	}

	groupIndex := new(big.Int).SetBytes(data[0:32])
	usdtAmount := new(big.Int).SetBytes(data[32:64])
	stakeType := uint8(new(big.Int).SetBytes(data[64:96]).Uint64())
	rexPrice := new(big.Int).SetBytes(data[96:128])
	timestamp := new(big.Int).SetBytes(data[128:160]).Int64()

	return &StakeFromMintGameEvent{
		User:        user,
		StakeIndex:  stakeIndex,
		GroupType:   groupType,
		GroupIndex:  groupIndex,
		UsdtAmount:  usdtAmount,
		StakeType:   stakeType,
		RexPrice:    rexPrice,
		Timestamp:   timestamp,
		TxHash:      event.TxHash.Hex(),
		BlockNumber: event.BlockNumber,
		EventIndex:  event.Index,
	}, nil
}

// createStakeForMintGame 为 APG Mint 游戏创建质押
func (l *MintStakeEventListener) createStakeForMintGame(ctx context.Context, event *StakeFromMintGameEvent) error {
	contractIndex := event.GroupIndex.Int64()
	walletAddress := strings.ToLower(event.User.Hex())

	// 根据 contract_index 查询玩家记录
	player, err := l.apgRepo.GetPlayerByContractIndex(ctx, contractIndex)
	if err != nil {
		return fmt.Errorf("查询玩家记录失败: %v", err)
	}
	if player == nil {
		glog.Warningf(ctx, "[APG Mint质押监听] 玩家记录不存在，跳过: contractIndex=%d, tx=%s", contractIndex, event.TxHash)
		return nil
	}

	// 验证钱包地址
	if strings.ToLower(player.WalletAddress) != walletAddress {
		glog.Warningf(ctx, "[APG Mint质押监听] 钱包地址不匹配: player=%s, event=%s",
			player.WalletAddress, walletAddress)
	}

	// 检查是否已创建质押包（staking_package_id 有值才表示真正完成）
	// 注意：前端 claim API 会先设置 stake_status=1，但不会创建质押包
	// 所以这里需要检查 staking_package_id 而不是 stake_status
	if player.StakingPackageId != nil && *player.StakingPackageId > 0 {
		glog.Debugf(ctx, "[APG Mint质押监听] 该记录已创建质押包，跳过: playerId=%d, contractIndex=%d, stakingPackageId=%d",
			player.Id, contractIndex, *player.StakingPackageId)
		return nil
	}

	usdtAmount := l.convertFromWei(event.UsdtAmount)
	rexPrice := l.convertFromWei(event.RexPrice)

	return l.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 创建质押包
		stakeEventData := &StakeEventData{
			UserID:         player.UserId,
			WalletAddress:  walletAddress,
			StakeAmount:    usdtAmount,
			StakeType:      consts.StakeTypeMintGame,
			REXPrice:       rexPrice,
			TxHash:         event.TxHash,
			BlockNumber:    event.BlockNumber,
			EventIndex:     int(event.EventIndex),
			BlockTimestamp: time.Unix(event.Timestamp, 0),
			StakeUsdt:      usdtAmount,
			TokenAmount:    decimal.Zero,
			TokenContract:  "",
			TokenSymbol:    "",
		}

		if err := l.stakeListener.createStakingPackage(ctx, tx, stakeEventData); err != nil {
			return fmt.Errorf("创建质押包失败: %v", err)
		}

		// 获取刚创建的质押包ID
		stakingPackageId, err := tx.Model("staking_package").Ctx(ctx).
			Where("tx_hash = ?", event.TxHash).
			Fields("id").
			Value()
		if err != nil || stakingPackageId.Int64() == 0 {
			return fmt.Errorf("获取质押包ID失败: %v", err)
		}

		// 更新玩家记录
		_, err = tx.Model("apg_mint_player").Ctx(ctx).
			Where("id = ?", player.Id).
			Data(gdb.Map{
				"stake_status":       1,
				"stake_tx_hash":      event.TxHash,
				"staking_package_id": stakingPackageId.Int64(),
				"stake_time":         time.Now(),
				"updated_at":         time.Now(),
			}).
			Update()

		if err != nil {
			return fmt.Errorf("更新玩家记录失败: %v", err)
		}

		return nil
	})
}

// convertFromWei 金额精度转换
func (l *MintStakeEventListener) convertFromWei(weiValue *big.Int) decimal.Decimal {
	return decimal.NewFromBigInt(weiValue, -int32(l.config.Decimals))
}

// checkIfProcessed 检查事件是否已处理
func (l *MintStakeEventListener) checkIfProcessed(ctx context.Context, txHash string, blockNumber uint64) (bool, error) {
	count, err := l.db.Model("staking_package").Ctx(ctx).
		Where("tx_hash = ?", txHash).
		Where("block_number = ?", blockNumber).
		Count()
	return count > 0, err
}
