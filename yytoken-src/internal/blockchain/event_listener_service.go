package blockchain

import (
	"XWFrame/internal/frame/consts"
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gcache"
	"github.com/gogf/gf/v2/os/glog"
)

// EventListenerService 事件监听服务
type EventListenerService struct {
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	rpcClient *RpcClient
	db        gdb.DB
	cache     *gcache.Cache
	config    *EventListenerConfig

	// 统一扫块器（主扫块器：Stake/Mint/Burn等）
	blockScanner *UnifiedBlockScanner

	// APG Mint 专用扫块器（独立扫块，更快响应）
	apgMintScanner *ApgMintBlockScanner

	// 监听器（用于创建事件处理器）
	stakeListener             *StakeEventListener
	burnListener              *BurnEventListener
	nodeDividendUsdtListener  *NodeDividendUsdtListener
	mintStakeListener         *MintStakeEventListener
	apgGroupListener          *ApgGroupEventListener
	apgRefundListener         *ApgRefundEventListener
	apgReferralRewardListener *ApgReferralRewardEventListener

	// 监听器状态
	status   map[string]*ListenerStatus
	statusMu sync.RWMutex
}

// ListenerStatus 监听器状态
type ListenerStatus struct {
	Name           string    // 监听器名称
	Running        bool      // 是否运行中
	LastExecTime   time.Time // 最后执行时间
	LastBlockNum   uint64    // 最后处理的区块号
	CurrentBlock   uint64    // 当前链上最新块
	BlocksBehind   uint64    // 落后的区块数
	FailCount      int       // 连续失败次数
	LastError      string    // 最后错误信息
	ProcessedCount uint64    // 已处理事件数
}

// NewEventListenerService 创建事件监听服务
func NewEventListenerService(ctx context.Context, db gdb.DB, cache *gcache.Cache) (*EventListenerService, error) {
	// 加载配置（自动填充默认值）
	config, err := LoadEventListenerConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("加载配置失败: %v", err)
	}

	// 创建子上下文
	subCtx, cancel := context.WithCancel(ctx)

	// 创建RPC客户端
	rpcClient, err := NewRpcClient(
		config.RPCEndpoints,
		config.RPCTimeout,
		config.RequestsPerSecond,
		config.Burst,
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("创建RPC客户端失败: %v", err)
	}

	service := &EventListenerService{
		ctx:       subCtx,
		cancel:    cancel,
		db:        db,
		cache:     cache,
		rpcClient: rpcClient,
		config:    config,
		status:    make(map[string]*ListenerStatus),
	}

	// 创建监听器
	service.stakeListener = NewStakeEventListener(subCtx, db, cache, rpcClient, config)
	service.burnListener = NewBurnEventListener(subCtx, db, cache, rpcClient, config)
	service.nodeDividendUsdtListener = NewNodeDividendUsdtListener(subCtx, db, cache, rpcClient, config)
	service.mintStakeListener = NewMintStakeEventListener(subCtx, db, cache, rpcClient, config)
	service.apgGroupListener = NewApgGroupEventListener(subCtx, db, cache, rpcClient, config)
	service.apgRefundListener = NewApgRefundEventListener(subCtx, db, cache, rpcClient, config)
	service.apgReferralRewardListener = NewApgReferralRewardEventListener(subCtx, db, cache, rpcClient, config)

	// 创建统一扫块器（主扫块器：处理 Stake/Mint/Burn 等事件）
	service.blockScanner = NewUnifiedBlockScanner(subCtx, cache, rpcClient, config)

	// 注册主扫块器的事件处理器
	service.blockScanner.RegisterHandler(NewStakeEventHandler(service.stakeListener))
	service.blockScanner.RegisterHandler(NewBurnEventHandler(service.burnListener))
	service.blockScanner.RegisterHandler(NewNodeDividendUsdtEventHandler(service.nodeDividendUsdtListener))
	service.blockScanner.RegisterHandler(NewCombinationStakeEventHandler(service.stakeListener))

	// 创建 APG Mint 专用扫块器（独立扫块，处理 MintGroup 合约事件）
	service.apgMintScanner = NewApgMintBlockScanner(subCtx, cache, rpcClient, config)

	// 注册 APG Mint 扫块器的事件处理器
	service.apgMintScanner.RegisterHandler(NewApgGroupEventHandler(service.apgGroupListener))
	service.apgMintScanner.RegisterHandler(NewApgRefundEventHandler(service.apgRefundListener))
	service.apgMintScanner.RegisterHandler(NewMintStakeEventHandler(service.mintStakeListener))
	service.apgMintScanner.RegisterHandler(NewApgReferralRewardEventHandler(service.apgReferralRewardListener))

	// 初始化状态
	service.status[consts.BlockchainListenerNameStake] = &ListenerStatus{Name: consts.BlockchainListenerNameStake}
	service.status[consts.BlockchainListenerNameBurn] = &ListenerStatus{Name: consts.BlockchainListenerNameBurn}
	service.status[consts.BlockchainListenerNameNodeDividendUsdt] = &ListenerStatus{Name: consts.BlockchainListenerNameNodeDividendUsdt}
	service.status[consts.BlockchainListenerNameMintStake] = &ListenerStatus{Name: consts.BlockchainListenerNameMintStake}
	service.status[consts.BlockchainListenerNameCombinationStake] = &ListenerStatus{Name: consts.BlockchainListenerNameCombinationStake}
	service.status[consts.BlockchainListenerNameApgGroup] = &ListenerStatus{Name: consts.BlockchainListenerNameApgGroup}
	service.status[consts.BlockchainListenerNameApgRefund] = &ListenerStatus{Name: consts.BlockchainListenerNameApgRefund}
	service.status[consts.BlockchainListenerNameApgReferralReward] = &ListenerStatus{Name: consts.BlockchainListenerNameApgReferralReward}

	return service, nil
}

// Start 启动事件监听服务
func (s *EventListenerService) Start() error {
	glog.Infof(s.ctx, "[事件监听服务] 正在启动...")
	glog.Infof(s.ctx, "[事件监听服务] 区块链网络: %s (Chain ID: %d)", s.config.ChainName, s.config.ChainID)
	glog.Infof(s.ctx, "[事件监听服务] 扫块间隔: %v", s.config.Interval)
	glog.Infof(s.ctx, "[事件监听服务] 区块确认数: %d", s.config.ConfirmBlocks)
	glog.Infof(s.ctx, "[事件监听服务] 批量大小: %d", s.config.BatchSize)

	// 启动主扫块器（处理 Stake/Mint/Burn 等事件）
	s.wg.Add(1)
	go s.runListenerWithRetry(s.ctx, "主扫块器", s.blockScanner.Scan)

	// APG Mint 扫块器已迁移到独立进程 (tools/group_listener)
	// 如需使用主服务集成的 APG Mint 扫块器，请取消下面的注释
	// s.wg.Add(1)
	// go s.runListenerWithRetry(s.ctx, "APG Mint扫块器", s.apgMintScanner.Scan)

	glog.Infof(s.ctx, "[事件监听服务] ⚠️ APG Mint 扫块器已禁用，使用独立监听器（apg_mint_listener）")

	// 启动心跳更新
	if s.config.HeartbeatInterval > 0 {
		s.wg.Add(1)
		go s.runHeartbeat()
	}

	glog.Infof(s.ctx, "[事件监听服务] ✅ 启动成功")

	return nil
}

// Stop 停止事件监听服务
func (s *EventListenerService) Stop() {
	glog.Infof(s.ctx, "[事件监听服务] 正在停止...")

	// 取消上下文
	s.cancel()

	// 等待所有协程退出（带超时）
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	// 等待协程退出，最多等待30秒
	select {
	case <-done:
		glog.Infof(s.ctx, "[事件监听服务] 所有协程已正常退出")
	case <-time.After(30 * time.Second):
		glog.Warningf(s.ctx, "[事件监听服务] ⚠️ 协程退出超时，强制停止")
	}

	// 关闭RPC客户端
	s.rpcClient.Close()

	glog.Infof(s.ctx, "[事件监听服务] ✅ 已停止")
}

// runListenerWithRetry 运行监听器（带自动重试和恢复）
func (s *EventListenerService) runListenerWithRetry(
	ctx context.Context,
	name string,
	listenerFunc func(context.Context) error,
) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.Interval)
	defer ticker.Stop()

	glog.Infof(ctx, "[%s] 启动，执行间隔: %v", name, s.config.Interval)

	for {
		select {
		case <-ctx.Done():
			glog.Infof(ctx, "[%s] 收到退出信号", name)
			return

		case <-ticker.C:
			// 执行监听逻辑（带恢复）
			s.executeWithRecovery(ctx, name, listenerFunc)
		}
	}
}

// executeWithRecovery 带Panic恢复的执行
func (s *EventListenerService) executeWithRecovery(
	ctx context.Context,
	name string,
	listenerFunc func(context.Context) error,
) {
	defer func() {
		if r := recover(); r != nil {
			stackTrace := string(debug.Stack())
			glog.Errorf(ctx, "[%s] Panic恢复: %v\nStack: %s", name, r, stackTrace)
			s.updateStatus(name, false, fmt.Sprintf("panic: %v", r))

			// 如果启用自动重启，等待后重新尝试
			if s.config.AutoRestart {
				time.Sleep(s.config.RetryDelay)
			}
		}
	}()

	// 更新状态：开始执行
	s.updateStatus(name, true, "")
	startTime := time.Now()

	// 执行监听逻辑（带重试）
	var err error
	for i := 0; i < s.config.MaxRetry; i++ {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			glog.Infof(ctx, "[%s] 上下文已取消，停止重试", name)
			return
		default:
		}

		// 创建带超时的子上下文
		execCtx, cancel := context.WithTimeout(ctx, s.config.EventQueryTimeout)
		err = listenerFunc(execCtx)
		cancel()

		if err == nil {
			// 成功，重置失败计数
			s.updateStatus(name, true, "")
			duration := time.Since(startTime)
			glog.Debugf(ctx, "[%s] 执行成功，耗时: %v", name, duration)
			return
		}

		// 检查是否是上下文取消的错误
		if isContextCanceledError(err) {
			glog.Infof(ctx, "[%s] 上下文已取消，停止执行", name)
			return
		}

		// 失败，记录错误
		glog.Errorf(ctx, "[%s] 执行失败 (第%d/%d次): %v", name, i+1, s.config.MaxRetry, err)

		// 如果不是最后一次，等待后重试（指数退避）
		if i < s.config.MaxRetry-1 {
			delay := s.config.RetryDelay * time.Duration(1<<i) // 指数退避
			glog.Infof(ctx, "[%s] 等待 %v 后重试...", name, delay)

			// 使用可中断的等待
			select {
			case <-ctx.Done():
				glog.Infof(ctx, "[%s] 上下文已取消，停止重试", name)
				return
			case <-time.After(delay):
				// 继续重试
			}
		}
	}

	// 所有重试失败
	s.updateStatus(name, false, err.Error())
	glog.Errorf(ctx, "[%s] 所有重试失败，等待下次执行", name)
}

// runHeartbeat 运行心跳更新
func (s *EventListenerService) runHeartbeat() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return

		case <-ticker.C:
			s.updateHeartbeats(s.ctx)
		}
	}
}

// updateHeartbeats 更新所有监听器的心跳
func (s *EventListenerService) updateHeartbeats(ctx context.Context) {
	now := time.Now()

	// 更新质押监听器心跳
	s.cache.Set(ctx, consts.BlockchainCacheKeyStakeListenerHeart, now.Unix(), 10*time.Minute)

	// 更新APG提取监听器心跳
	s.cache.Set(ctx, consts.BlockchainCacheKeyMintListenerHeart, now.Unix(), 10*time.Minute)

	// 更新Swap销毁监听器心跳
	s.cache.Set(ctx, consts.BlockchainCacheKeyBurnListenerHeart, now.Unix(), 10*time.Minute)
}

// updateStatus 更新监听器状态
func (s *EventListenerService) updateStatus(name string, running bool, errorMsg string) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()

	if status, ok := s.status[name]; ok {
		status.Running = running
		status.LastExecTime = time.Now()

		if errorMsg != "" {
			status.FailCount++
			status.LastError = errorMsg
		} else {
			status.FailCount = 0
			status.LastError = ""
		}
	}
}

// GetStatus 获取所有监听器状态
func (s *EventListenerService) GetStatus() map[string]*ListenerStatus {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()

	// 深拷贝返回
	result := make(map[string]*ListenerStatus)
	for k, v := range s.status {
		result[k] = &ListenerStatus{
			Name:           v.Name,
			Running:        v.Running,
			LastExecTime:   v.LastExecTime,
			LastBlockNum:   v.LastBlockNum,
			CurrentBlock:   v.CurrentBlock,
			BlocksBehind:   v.BlocksBehind,
			FailCount:      v.FailCount,
			LastError:      v.LastError,
			ProcessedCount: v.ProcessedCount,
		}
	}

	return result
}

// GetConfig 获取配置
func (s *EventListenerService) GetConfig() *EventListenerConfig {
	return s.config
}

// GetLastProcessedBlock 获取上次处理的区块高度
func (s *EventListenerService) GetLastProcessedBlock(ctx context.Context) (uint64, error) {
	if s.blockScanner == nil {
		return 0, fmt.Errorf("统一扫块器未初始化")
	}
	return s.blockScanner.getLastProcessedBlock(ctx), nil
}

// GetCurrentBlockInfo 获取当前区块信息（上次处理区块、最新区块、落后区块数）
func (s *EventListenerService) GetCurrentBlockInfo(ctx context.Context) (map[string]interface{}, error) {
	if s.blockScanner == nil {
		return nil, fmt.Errorf("统一扫块器未初始化")
	}

	lastBlock := s.blockScanner.getLastProcessedBlock(ctx)
	latestBlock, err := s.rpcClient.BlockNumber(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取最新区块失败: %v", err)
	}
	latestBlock = latestBlock - uint64(s.config.ConfirmBlocks)

	var blocksBehind uint64
	if latestBlock > lastBlock {
		blocksBehind = latestBlock - lastBlock
	}

	return map[string]interface{}{
		"last_processed_block": lastBlock,
		"latest_block":         latestBlock + uint64(s.config.ConfirmBlocks), // 链上最新区块
		"safe_block":           latestBlock,                                  // 安全区块（减去确认数）
		"blocks_behind":        blocksBehind,
		"confirm_blocks":       s.config.ConfirmBlocks,
	}, nil
}
