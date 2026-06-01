package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/repository"
	apgRepo "XWFrame/internal/repository/apg"
	apgService "XWFrame/internal/service/apg"
	"XWFrame/pkg/utils"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcache"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

const (
	JoinStatusSuccess = 0
	JoinStatusFailed  = 1
)

// GroupEvent 拼团事件（用于测试API手动添加玩家）
type GroupEvent struct {
	TokenName   string
	TokenAmount string
	LpAmount    string
	UserAddress string
	TxHash      string
	BlockNumber uint64
}

// ApgGroupListener 简单的拼团监听器（用于测试API）
type ApgGroupListener struct {
	apgRepo  apgRepo.IApgRepository
	userRepo repository.IUserRepository
}

// NewApgGroupListener 创建简单拼团监听器
func NewApgGroupListener() *ApgGroupListener {
	return &ApgGroupListener{
		apgRepo:  apgRepo.NewApgRepository(),
		userRepo: repository.NewUserRepository(),
	}
}

// HandleGroupEvent 处理拼团事件（测试用）
func (l *ApgGroupListener) HandleGroupEvent(ctx context.Context, event *GroupEvent) (int64, error) {
	g.Log().Debug(ctx, "[APG拼团事件] 收到事件",
		"token", event.TokenName,
		"amount", event.TokenAmount,
		"user", event.UserAddress,
		"txHash", event.TxHash,
	)

	exists, err := l.apgRepo.CheckTxHashExists(ctx, event.TxHash)
	if err != nil {
		g.Log().Error(ctx, "[APG拼团事件] 检查交易哈希失败", "error", err)
		return 0, fmt.Errorf("检查交易哈希失败: %w", err)
	}

	if exists {
		g.Log().Debug(ctx, "[APG拼团事件] 交易已处理，跳过", "txHash", event.TxHash)
		return 0, nil
	}

	user, err := l.userRepo.GetUserByWalletAddress(ctx, event.UserAddress)
	if err != nil {
		g.Log().Error(ctx, "[APG拼团事件] 查询用户失败", "address", event.UserAddress, "error", err)
		return 0, fmt.Errorf("查询用户失败: %w", err)
	}

	if user == nil {
		g.Log().Error(ctx, "[APG拼团事件] 用户不存在", "address", event.UserAddress)
		return 0, fmt.Errorf("用户不存在: %s", event.UserAddress)
	}

	now := utils.GetShanghaiTime()
	poolType := apgService.PoolType100U

	// 直接查询当前时间落在 start_time 和 finish_time 之间的场次
	match, err := l.apgRepo.GetCurrentActiveMatch(ctx, poolType)
	if err != nil {
		g.Log().Error(ctx, "[APG拼团事件] 查询当前场次失败",
			"pool", poolType,
			"error", err,
		)
		return 0, fmt.Errorf("查询场次失败: %w", err)
	}

	if match == nil {
		g.Log().Error(ctx, "[APG拼团事件] 当前没有活跃的场次",
			"pool", poolType,
		)
		return 0, fmt.Errorf("当前没有活跃的场次")
	}

	lockTime := match.LockTime.Time.In(utils.GetShanghaiLocation())
	if now.After(lockTime) {
		return 0, fmt.Errorf("已封盘")
	}

	poolConfig, err := l.apgRepo.GetPoolConfig(ctx, poolType)
	if err != nil {
		g.Log().Error(ctx, "[APG拼团事件] 获取池子配置失败", "error", err)
		return 0, fmt.Errorf("获取池子配置失败: %w", err)
	}

	joinStatus := JoinStatusSuccess
	successCount, err := l.apgRepo.CountUserMatchSuccessfulJoins(ctx, user.Id, match.Id)
	if err != nil {
		g.Log().Error(ctx, "[APG拼团事件] 查询用户参与次数失败", "error", err)
		return 0, fmt.Errorf("查询用户参与次数失败: %w", err)
	}

	maxBuy := poolConfig.MaxBuyPerRound
	if maxBuy <= 0 {
		maxBuy = 21
	}
	if successCount >= maxBuy {
		joinStatus = JoinStatusFailed
		g.Log().Warning(ctx, "[APG拼团事件] 用户超过最大参与限制",
			"userId", user.Id,
			"matchId", match.Id,
			"current", successCount,
			"max", maxBuy,
		)
	}

	contractId, _ := apgService.NewTokenService().GetActiveContractId(ctx)
	player := &entity.ApgPlayer{
		UserId:        user.Id,
		WalletAddress: strings.ToLower(event.UserAddress),
		MatchId:       match.Id,
		PoolType:      poolType,
		Round:         match.Round,
		GroupId:       0,
		PaymentToken:  event.TokenName,
		PaymentAmount: event.TokenAmount,
		LpAmount:      event.LpAmount,
		TxHash:        event.TxHash,
		ContractId:    &contractId,
		IsWinner:      apgService.StatusNotDrawn,
		JoinStatus:    joinStatus,
		CreatedAt:     gtime.Now(),
		UpdatedAt:     gtime.Now(),
	}

	if err := l.apgRepo.CreatePlayer(ctx, player); err != nil {
		g.Log().Error(ctx, "[APG拼团事件] 创建玩家记录失败", "error", err)
		return 0, fmt.Errorf("创建玩家记录失败: %w", err)
	}

	if joinStatus == JoinStatusSuccess {
		if err := l.apgRepo.UpdateMatch(ctx, match.Id, map[string]interface{}{
			"player_count": gdb.Raw("player_count + 1"),
		}); err != nil {
			g.Log().Warning(ctx, "[APG拼团事件] 更新场次人数失败", "matchId", match.Id, "error", err)
		}
	}

	statusText := "成功"
	if joinStatus == JoinStatusFailed {
		statusText = "失败(超过限制)"
	}
	g.Log().Info(ctx, "[APG拼团事件] 处理完成",
		"playerId", player.Id,
		"userId", user.Id,
		"matchId", match.Id,
		"txHash", event.TxHash,
		"joinStatus", statusText,
	)

	return player.Id, nil
}

// ApgGroupEventListener APG拼团事件监听器
// 监听 HashPowerGroup 事件，创建玩家记录
type ApgGroupEventListener struct {
	ctx             context.Context
	db              gdb.DB
	cache           *gcache.Cache
	rpcClient       *RpcClient
	config          *EventListenerConfig
	contractAddress common.Address
	eventID         common.Hash
	apgRepo         apgRepo.IApgRepository
	userRepo        repository.IUserRepository
}

// HashPowerGroupEvent HashPowerGroup 事件数据结构
type HashPowerGroupEvent struct {
	User        common.Address // indexed - 用户地址
	Token       common.Address // indexed - 支付代币地址
	Index       *big.Int       // indexed - 合约索引（重要，用于后续事件关联）
	GroupType   *big.Int       // 拼团类型
	Liquidity   *big.Int       // 流动性
	UsdtValue   *big.Int       // USDT等值（下注价值）
	TokenAmount *big.Int       // 代币数量
	Timestamp   int64          // 时间戳
	TxHash      string         // 交易哈希
	BlockNumber uint64         // 区块号
}

// NewApgGroupEventListener 创建APG拼团事件监听器
func NewApgGroupEventListener(ctx context.Context, db gdb.DB, cache *gcache.Cache, rpcClient *RpcClient, config *EventListenerConfig) *ApgGroupEventListener {
	contractAddress := common.HexToAddress(config.ContractMintGroup)
	eventID := common.HexToHash(consts.BlockchainEventIDApgGroup)

	glog.Infof(ctx, "[APG拼团监听] 初始化 - 合约地址: %s, 事件ID: %s",
		contractAddress.Hex(), eventID.Hex())

	return &ApgGroupEventListener{
		ctx:             ctx,
		db:              db,
		cache:           cache,
		rpcClient:       rpcClient,
		config:          config,
		contractAddress: contractAddress,
		eventID:         eventID,
		apgRepo:         apgRepo.NewApgRepository(),
		userRepo:        repository.NewUserRepository(),
	}
}

// processApgGroupEvent 处理单个APG拼团事件
func (l *ApgGroupEventListener) processApgGroupEvent(ctx context.Context, event types.Log) error {
	glog.Infof(ctx, "[APG拼团监听] 开始处理拼团事件: tx=%s, logIndex=%d", event.TxHash.Hex(), event.Index)

	groupEvent, err := l.parseHashPowerGroupEvent(event)
	if err != nil {
		return fmt.Errorf("解析事件失败: %v", err)
	}

	glog.Infof(ctx, "[APG拼团监听] 事件详情: user=%s, token=%s, index=%s, groupType=%s, usdtValue=%s, tx=%s",
		groupEvent.User.Hex(),
		groupEvent.Token.Hex(),
		groupEvent.Index.String(),
		groupEvent.GroupType.String(),
		l.convertFromWei(groupEvent.UsdtValue).String(),
		groupEvent.TxHash)

	if err := l.createPlayerRecord(ctx, groupEvent); err != nil {
		return fmt.Errorf("创建玩家记录失败: %v", err)
	}

	glog.Infof(ctx, "[APG拼团监听] 事件处理完成: tx=%s, index=%s",
		groupEvent.TxHash, groupEvent.Index.String())

	return nil
}

// parseHashPowerGroupEvent 解析 HashPowerGroup 事件
// 事件结构: HashPowerGroup(address indexed user, address indexed token, uint256 indexed index,
//           uint256 groupType, uint256 amount, uint256 liquidity, uint256 usdtValue, uint256 tokenAmount, uint256 timestamp)
// data 共 192 bytes (6个字段)
func (l *ApgGroupEventListener) parseHashPowerGroupEvent(event types.Log) (*HashPowerGroupEvent, error) {
	if len(event.Topics) < 4 {
		return nil, fmt.Errorf("事件Topics数量不足: 期望4, 实际%d", len(event.Topics))
	}

	user := common.BytesToAddress(event.Topics[1].Bytes())
	token := common.BytesToAddress(event.Topics[2].Bytes())
	index := new(big.Int).SetBytes(event.Topics[3].Bytes())

	data := event.Data
	if len(data) < 192 {
		return nil, fmt.Errorf("事件数据长度不足: 期望192, 实际%d", len(data))
	}

	groupType := new(big.Int).SetBytes(data[0:32])
	// amount := new(big.Int).SetBytes(data[32:64]) // 暂未使用
	liquidity := new(big.Int).SetBytes(data[64:96])
	usdtValue := new(big.Int).SetBytes(data[96:128])
	tokenAmount := new(big.Int).SetBytes(data[128:160])
	timestamp := new(big.Int).SetBytes(data[160:192]).Int64()

	return &HashPowerGroupEvent{
		User:        user,
		Token:       token,
		Index:       index,
		GroupType:   groupType,
		Liquidity:   liquidity,
		UsdtValue:   usdtValue,
		TokenAmount: tokenAmount,
		Timestamp:   timestamp,
		TxHash:      event.TxHash.Hex(),
		BlockNumber: event.BlockNumber,
	}, nil
}

// createPlayerRecord 创建玩家记录
func (l *ApgGroupEventListener) createPlayerRecord(ctx context.Context, event *HashPowerGroupEvent) error {
	walletAddress := strings.ToLower(event.User.Hex())
	tokenAddress := strings.ToLower(event.Token.Hex())
	txHash := event.TxHash
	contractIndex := event.Index.Int64()

	// 检查 (contract_index, tx_hash) 是否已处理
	exists, err := l.apgRepo.CheckContractIndexAndTxHashExists(ctx, contractIndex, txHash)
	if err != nil {
		return fmt.Errorf("检查contract_index失败: %v", err)
	}
	if exists {
		glog.Debugf(ctx, "[APG拼团监听] contract_index已处理，跳过: index=%d, tx=%s", contractIndex, txHash)
		return nil
	}

	// 查询支付代币信息
	tokenInfo, err := l.apgRepo.GetTokenInfoByAddress(ctx, tokenAddress)
	if err != nil {
		return fmt.Errorf("查询代币信息失败: %v", err)
	}
	if tokenInfo == nil {
		glog.Warningf(ctx, "[APG拼团监听] 代币不存在，跳过: token=%s, tx=%s", tokenAddress, txHash)
		return nil
	}

	// 查询用户
	user, err := l.userRepo.GetUserByWalletAddress(ctx, walletAddress)
	if err != nil {
		return fmt.Errorf("查询用户失败: %v", err)
	}
	if user == nil {
		glog.Warningf(ctx, "[APG拼团监听] 用户不存在，跳过: address=%s", walletAddress)
		return nil
	}

	// 根据事件时间戳确定场次（直接从数据库查询事件时间落在哪个场次）
	eventTime := time.Unix(event.Timestamp, 0).In(utils.GetShanghaiLocation())

	// groupType 对应 pool_type
	poolType := int(event.GroupType.Int64())

	// 直接查询事件时间落在 start_time 和 finish_time 之间的场次
	match, err := l.apgRepo.GetMatchByTime(ctx, poolType, eventTime)
	if err != nil {
		return fmt.Errorf("查询场次失败: %v", err)
	}
	if match == nil {
		glog.Warningf(ctx, "[APG拼团监听] 事件时间没有对应的场次: time=%v, pool=%d", eventTime, poolType)
		return nil
	}

	// 获取池子配置
	poolConfig, err := l.apgRepo.GetPoolConfig(ctx, poolType)
	if err != nil {
		return fmt.Errorf("获取池子配置失败: %v", err)
	}

	maxBuy := poolConfig.MaxBuyPerRound
	if maxBuy <= 0 {
		maxBuy = 21
	}

	// 计算支付金额和 LP 数量
	tokenAmount := l.convertFromWei(event.TokenAmount)
	lpAmount := l.convertFromWei(event.Liquidity)

	activeContractId, _ := apgService.NewTokenService().GetActiveContractId(ctx)

	// 如果场次已结束，标记为退款状态（迟到参与）
	isWinner := apgService.StatusNotDrawn
	if match.IsFinished != 0 {
		isWinner = apgService.StatusRefunded
		glog.Infof(ctx, "[APG拼团监听] 场次已结束，标记为退款: matchId=%d, eventTime=%v", match.Id, eventTime)
	}

	player := &entity.ApgPlayer{
		UserId:         user.Id,
		WalletAddress:  walletAddress,
		MatchId:        match.Id,
		PoolType:       poolType,
		Round:          match.Round,
		GroupId:        0,
		PaymentTokenId: tokenInfo.Id,
		PaymentToken:   tokenInfo.Symbol,
		PaymentAmount:  tokenAmount.String(),
		LpAmount:       lpAmount.String(),
		TxHash:         txHash,
		ContractIndex:  &contractIndex,
		ContractId:     &activeContractId,
		IsWinner:       isWinner,
		JoinStatus:     0,
		CreatedAt:      gtime.Now(),
		UpdatedAt:      gtime.Now(),
	}

	// 如果场次已结束，设置退款字段
	if match.IsFinished != 0 {
		player.RefundToken = tokenInfo.Symbol
		player.RefundAmount = tokenAmount.String()
		player.ResultTime = gtime.Now()
	}

	// 事务处理：检查重复 -> 创建玩家 -> 检查限制 -> 更新场次人数
	var joinStatus int
	err = l.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 在事务内再次检查重复（防止并发）
		exists, err := l.apgRepo.CheckContractIndexAndTxHashExists(ctx, contractIndex, txHash)
		if err != nil {
			return fmt.Errorf("检查contract_index失败: %v", err)
		}
		if exists {
			glog.Debugf(ctx, "[APG拼团监听] contract_index已处理（事务内检查），跳过: index=%d, tx=%s", contractIndex, txHash)
			return nil
		}

		if err := l.apgRepo.CreatePlayer(ctx, player); err != nil {
			return fmt.Errorf("创建玩家记录失败: %v", err)
		}

		// 如果场次已结束，直接返回（无需检查限制和更新人数）
		if match.IsFinished != 0 {
			glog.Infof(ctx, "[APG拼团监听] 已结束场次，跳过限制检查: playerId=%d, matchId=%d", player.Id, match.Id)
			return nil
		}

		// 插入后统计总数（包含刚插入的记录）
		successCount, err := l.apgRepo.CountUserMatchSuccessfulJoins(ctx, user.Id, match.Id)
		if err != nil {
			return fmt.Errorf("查询用户参与次数失败: %v", err)
		}

		if successCount > maxBuy {
			joinStatus = 1
			if err := l.apgRepo.UpdatePlayer(ctx, player.Id, gdb.Map{"join_status": 1}); err != nil {
				return fmt.Errorf("更新玩家状态失败: %v", err)
			}
			glog.Warningf(ctx, "[APG拼团监听] 用户超过最大参与限制: userId=%d, matchId=%d, current=%d, max=%d",
				user.Id, match.Id, successCount, maxBuy)
		} else {
			// 更新场次人数（原子操作）
			if err := l.apgRepo.UpdateMatch(ctx, match.Id, map[string]interface{}{
				"player_count": gdb.Raw("player_count + 1"),
			}); err != nil {
				return fmt.Errorf("更新场次人数失败: %v", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	statusText := "成功"
	if joinStatus == 1 {
		statusText = "失败(超过限制)"
	}
	glog.Infof(ctx, "[APG拼团监听] 创建玩家记录成功: playerId=%d, userId=%d, matchId=%d, contractIndex=%d, status=%s",
		player.Id, user.Id, match.Id, contractIndex, statusText)

	return nil
}

// getBlockTimestamp 获取区块时间戳
func (l *ApgGroupEventListener) getBlockTimestamp(ctx context.Context, blockNumber uint64) (time.Time, error) {
	blockNum := new(big.Int).SetUint64(blockNumber)
	block, err := l.rpcClient.BlockByNumber(ctx, blockNum)
	if err != nil {
		return time.Time{}, fmt.Errorf("获取区块信息失败: %v", err)
	}
	return time.Unix(int64(block.Time()), 0), nil
}

// convertFromWei 金额精度转换
func (l *ApgGroupEventListener) convertFromWei(weiValue *big.Int) decimal.Decimal {
	return decimal.NewFromBigInt(weiValue, -int32(l.config.Decimals))
}
