package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
	"XWFrame/internal/frame/consts"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/gogf/gf/v2/os/gcache"
	"github.com/gogf/gf/v2/os/glog"
)

// EventHandler 事件处理器接口
type EventHandler interface {
	// GetEventID 获取监听的事件ID
	GetEventID() common.Hash
	// GetContractAddress 获取监听的合约地址
	GetContractAddress() common.Address
	// ProcessEvent 处理单个事件
	ProcessEvent(ctx context.Context, event types.Log) error
	// GetName 获取处理器名称
	GetName() string
}

// UnifiedBlockScanner 统一区块扫描器
type UnifiedBlockScanner struct {
	ctx                      context.Context
	cache                    *gcache.Cache
	rpcClient                *RpcClient
	config                   *EventListenerConfig
	handlers                 []EventHandler                // 事件处理器列表
	faultToleranceService    *FaultToleranceService        // 容错性服务
	eventProcessingRecordDao dao.IEventProcessingRecordDao // 事件处理记录DAO
}

// NewUnifiedBlockScanner 创建统一区块扫描器
func NewUnifiedBlockScanner(
	ctx context.Context,
	cache *gcache.Cache,
	rpcClient *RpcClient,
	config *EventListenerConfig,
) *UnifiedBlockScanner {
	// 加载容错性配置
	faultToleranceConfig, err := LoadFaultToleranceConfig(ctx)
	if err != nil {
		glog.Errorf(ctx, "加载容错性配置失败: %v", err)
		faultToleranceConfig = &FaultToleranceConfig{} // 使用默认配置
	}
	faultToleranceService := NewFaultToleranceService(faultToleranceConfig)

	scanner := &UnifiedBlockScanner{
		ctx:                      ctx,
		cache:                    cache,
		rpcClient:                rpcClient,
		config:                   config,
		handlers:                 make([]EventHandler, 0),
		faultToleranceService:    faultToleranceService,
		eventProcessingRecordDao: dao.NewEventProcessingRecordDao(),
	}

	// 启动容错性服务
	if err := faultToleranceService.Start(ctx); err != nil {
		glog.Errorf(ctx, "启动容错性服务失败: %v", err)
	}

	return scanner
}

// RegisterHandler 注册事件处理器
func (s *UnifiedBlockScanner) RegisterHandler(handler EventHandler) {
	s.handlers = append(s.handlers, handler)
	glog.Infof(s.ctx, "[统一扫块器] 注册事件处理器: %s", handler.GetName())

	// 如果处理器实现了RetryEventHandler接口，也注册到容错性服务
	if retryHandler, ok := handler.(RetryEventHandler); ok {
		s.faultToleranceService.RegisterHandler(retryHandler)
	}
}

// Scan 执行区块扫描
func (s *UnifiedBlockScanner) Scan(ctx context.Context) error {
	if len(s.handlers) == 0 {
		glog.Warning(ctx, "[统一扫块器] 没有注册任何事件处理器")
		return nil
	}

	// 1. 获取上次处理的区块高度（从Redis）
	lastBlock := s.getLastProcessedBlock(ctx)

	// 2. 获取当前最新区块（减去确认块数）
	latestBlock, err := s.rpcClient.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("获取最新区块失败: %v", err)
	}
	latestBlock = latestBlock - uint64(s.config.ConfirmBlocks)

	// 检查是否有新区块
	if lastBlock >= latestBlock {
		glog.Debugf(ctx, "[统一扫块器] 无新区块需要处理，当前: %d, 最新: %d", lastBlock, latestBlock)
		return nil
	}

	glog.Infof(ctx, "[统一扫块器] 开始扫块: 从 %d 到 %d (共 %d 个区块)", lastBlock+1, latestBlock, latestBlock-lastBlock)

	// 3. 收集所有需要监听的合约地址和事件ID
	contractAddresses := make(map[common.Address]bool)
	eventIDs := make(map[common.Hash]bool)
	for _, handler := range s.handlers {
		contractAddresses[handler.GetContractAddress()] = true
		eventIDs[handler.GetEventID()] = true
	}

	// 转换为切片
	addresses := make([]common.Address, 0, len(contractAddresses))
	for addr := range contractAddresses {
		addresses = append(addresses, addr)
	}

	topics := make([]common.Hash, 0, len(eventIDs))
	for topic := range eventIDs {
		topics = append(topics, topic)
	}

	// 4. 批量扫描区块
	for fromBlock := lastBlock + 1; fromBlock <= latestBlock; fromBlock += uint64(s.config.BatchSize) {
		toBlock := min(fromBlock+uint64(s.config.BatchSize)-1, latestBlock)

		// 限制单次查询范围（防止RPC超时）
		if toBlock-fromBlock > uint64(s.config.MaxBlocksPerQuery) {
			toBlock = fromBlock + uint64(s.config.MaxBlocksPerQuery) - 1
		}

		glog.Debugf(ctx, "[统一扫块器] 查询区块范围: %d - %d", fromBlock, toBlock)

		events, err := s.rpcClient.FilterLogs(ctx, ethereum.FilterQuery{
			FromBlock: big.NewInt(int64(fromBlock)),
			ToBlock:   big.NewInt(int64(toBlock)),
			Addresses: addresses,               // 所有合约地址
			Topics:    [][]common.Hash{topics}, // 所有事件ID
		})

		if err != nil {
			// 检查是否是上下文取消的错误
			if isContextCanceledError(err) {
				glog.Infof(ctx, "[统一扫块器] 上下文已取消，停止扫块")
				return nil // 正常退出，不报错
			}

			// 检查是否是历史数据被修剪的错误
			if isPrunedError(err) {
				glog.Warningf(ctx, "[统一扫块器] ⚠️ 历史区块 [%d-%d] 数据已被修剪", fromBlock, toBlock)
				glog.Warningf(ctx, "[统一扫块器] ⚠️ 公共RPC节点通常只保留最近7-30天的历史数据")
				glog.Warningf(ctx, "[统一扫块器] ⚠️ 建议使用归档节点或调整起始区块配置")

				// 获取最新区块并重置进度
				latestBlock, err := s.rpcClient.BlockNumber(ctx)
				if err != nil {
					return fmt.Errorf("获取最新区块失败: %v", err)
				}

				// 计算安全的起始区块（从最新区块减去确认块数）
				newStartBlock := latestBlock - uint64(s.config.ConfirmBlocks)
				if newStartBlock < latestBlock {
					s.saveLastProcessedBlock(ctx, newStartBlock)
					glog.Infof(ctx, "[统一扫块器] ✅ 已重置扫块进度到安全区块: %d (最新: %d)", newStartBlock, latestBlock)
					// 返回 nil，等待下次扫描从新位置开始
					return nil
				} else {
					glog.Warningf(ctx, "[统一扫块器] ⚠️ 无法计算安全起始区块，跳过本次扫描")
					return nil
				}
			}

			// 其他类型的错误，记录详细信息
			glog.Errorf(ctx, "[统一扫块器] ❌ 查询事件失败 [%d-%d]: %v", fromBlock, toBlock, err)
			return fmt.Errorf("查询事件失败 [%d-%d]: %v", fromBlock, toBlock, err)
		}

		glog.Infof(ctx, "[统一扫块器] 发现 %d 个事件 [区块 %d-%d]", len(events), fromBlock, toBlock)

		// 5. 分发事件到对应的处理器
		eventCount := make(map[string]int) // 统计每个处理器处理的事件数
		for _, event := range events {
			// 找到对应的处理器
			for _, handler := range s.handlers {
				if event.Address == handler.GetContractAddress() &&
					len(event.Topics) > 0 && event.Topics[0] == handler.GetEventID() {

					// 先尝试3次重试，失败后进入容错性重试队列
					var err error
					if retryHandler, ok := handler.(RetryEventHandler); ok {
						// 先执行3次重试
						err = s.processEventWithRetry(ctx, retryHandler, event)
						if err != nil {
							// 3次重试都失败，进入容错性重试队列
							glog.Warningf(ctx, "[统一扫块器] %s 3次重试失败，进入容错性重试队列 (tx: %s): %v",
								handler.GetName(), event.TxHash.Hex(), err)
							err = s.faultToleranceService.ProcessEvent(ctx, retryHandler, event)
						}
					} else {
						// 直接处理（无容错性）
						err = handler.ProcessEvent(ctx, event)
					}

					if err != nil {
						glog.Errorf(ctx, "[统一扫块器] %s 处理事件失败 (tx: %s): %v",
							handler.GetName(), event.TxHash.Hex(), err)
						continue
					}
					eventCount[handler.GetName()]++
					break
				}
			}
		}

		// 输出统计信息
		for name, count := range eventCount {
			if count > 0 {
				glog.Infof(ctx, "[统一扫块器] %s 处理了 %d 个事件", name, count)
			}
		}

		// 6. 保存扫描进度
		s.saveLastProcessedBlock(ctx, toBlock)

		// 7. 限流（防止RPC节点限流）
		if s.config.RateLimitEnabled && s.config.RequestsPerSecond > 0 {
			time.Sleep(time.Second / time.Duration(s.config.RequestsPerSecond))
		}
	}

	glog.Infof(ctx, "[统一扫块器] 扫块完成: 处理到区块 %d", latestBlock)
	return nil
}

// getLastProcessedBlock 获取上次处理的区块号
func (s *UnifiedBlockScanner) getLastProcessedBlock(ctx context.Context) uint64 {
	// 从Redis获取统一的扫块进度
	value, err := s.cache.Get(ctx, consts.BlockchainCacheKeyUnifiedLastBlock)
	if err == nil && value != nil {
		if blockNum, err := strconv.ParseUint(value.String(), 10, 64); err == nil {
			return blockNum
		}
	}

	// Redis中没有，处理起始区块逻辑
	if s.config.StartBlock == 0 {
		// 如果配置为0，表示从当前最新区块开始（避免历史区块被裁剪）
		latestBlock, err := s.rpcClient.BlockNumber(ctx)
		if err != nil {
			glog.Errorf(ctx, "[统一扫块器] 获取最新区块失败: %v", err)
			return 0 // 如果获取失败，返回0作为兜底
		}
		// 从当前最新区块开始，减去确认块数
		confirmBlocks := uint64(s.config.ConfirmBlocks)
		var startBlock uint64
		if latestBlock > confirmBlocks {
			startBlock = latestBlock - confirmBlocks
		} else {
			startBlock = 0
		}
		glog.Infof(ctx, "[统一扫块器] 首次启动，从当前最新区块开始: %d (最新: %d)", startBlock, latestBlock)
		return startBlock
	}

	// 使用配置的起始区块，但先验证是否可用
	configuredStart := s.config.StartBlock
	latestBlock, err := s.rpcClient.BlockNumber(ctx)
	if err != nil {
		glog.Warningf(ctx, "[统一扫块器] 无法验证配置的起始区块，使用配置值: %d", configuredStart)
		return configuredStart
	}

	// 如果配置的起始区块太老（超过30天），自动调整
	maxHistoryBlocks := uint64(864000) // 约30天的区块数（BSC测试网：3秒/块）
	if latestBlock > maxHistoryBlocks && configuredStart < latestBlock-maxHistoryBlocks {
		glog.Warningf(ctx, "[统一扫块器] ⚠️ 配置的起始区块 %d 距离最新区块 %d 超过30天（%d个区块）",
			configuredStart, latestBlock, latestBlock-configuredStart)
		glog.Warningf(ctx, "[统一扫块器] ⚠️ 公共RPC节点可能不保留这么久的历史数据")
		glog.Warningf(ctx, "[统一扫块器] ⚠️ 建议设置 start_block: 0 或使用归档节点")

		// 尝试从7天前开始（较为保守的策略）
		safeStartBlock := latestBlock - uint64(201600) // 约7天
		glog.Infof(ctx, "[统一扫块器] 自动调整起始区块到7天前: %d", safeStartBlock)
		return safeStartBlock
	}

	glog.Infof(ctx, "[统一扫块器] 使用配置的起始区块: %d (最新: %d, 间隔: %d个区块)",
		configuredStart, latestBlock, latestBlock-configuredStart)
	return configuredStart
}

// saveLastProcessedBlock 保存处理进度
func (s *UnifiedBlockScanner) saveLastProcessedBlock(ctx context.Context, blockNum uint64) {
	err := s.cache.Set(ctx, consts.BlockchainCacheKeyUnifiedLastBlock, blockNum, 0) // 永久保存
	if err != nil {
		glog.Errorf(ctx, "[统一扫块器] 保存进度失败: %v", err)
	}
}

// isContextCanceledError 检查是否是上下文取消的错误
func isContextCanceledError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()

	// 检查上下文取消相关的错误信息
	canceledKeywords := []string{
		"context canceled",
		"context cancelled",
		"operation was canceled",
		"operation was cancelled",
		"request canceled",
		"request cancelled",
	}

	for _, keyword := range canceledKeywords {
		if len(errMsg) > 0 && len(keyword) > 0 {
			// 不区分大小写的字符串包含检查
			if containsIgnoreCase(errMsg, keyword) {
				return true
			}
		}
	}
	return false
}

// isPrunedError 检查是否是历史数据被修剪的错误
func isPrunedError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()

	// 检查常见的 pruned 错误信息
	prunedKeywords := []string{
		"pruned",
		"History has been pruned",
		"missing trie node",
		"header not found",
		"block not found",
		"node not found",
		"trie node missing",
		"state not found",
		"archive node",
		"full node",
		"dedicated full node",
		"allnodes.com",
	}

	for _, keyword := range prunedKeywords {
		if len(errMsg) > 0 && len(keyword) > 0 {
			// 不区分大小写的字符串包含检查
			if containsIgnoreCase(errMsg, keyword) {
				return true
			}
		}
	}
	return false
}

// containsIgnoreCase 不区分大小写的字符串包含检查
func containsIgnoreCase(s, substr string) bool {
	sLower := ""
	substrLower := ""
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			sLower += string(r + 32)
		} else {
			sLower += string(r)
		}
	}
	for _, r := range substr {
		if r >= 'A' && r <= 'Z' {
			substrLower += string(r + 32)
		} else {
			substrLower += string(r)
		}
	}
	// 简单的字符串包含检查
	if len(substrLower) > len(sLower) {
		return false
	}
	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		if sLower[i:i+len(substrLower)] == substrLower {
			return true
		}
	}
	return false
}

// GetFaultToleranceService 获取容错性服务
func (s *UnifiedBlockScanner) GetFaultToleranceService() *FaultToleranceService {
	return s.faultToleranceService
}

// processEventWithRetry 处理事件（带3次重试）
func (s *UnifiedBlockScanner) processEventWithRetry(ctx context.Context, handler RetryEventHandler, event types.Log) error {
	const maxRetry = 3
	const retryDelay = 5 * time.Second

	var lastErr error

	for i := 0; i < maxRetry; i++ {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 记录重试开始时间
		startTime := time.Now()

		// 执行事件处理
		err := handler.ProcessEvent(ctx, event)
		duration := time.Since(startTime)

		// 记录每次重试的详细信息
		retryRecord := &entity.EventProcessingRecord{
			EventID:         s.generateEventID(event),
			AttemptNumber:   i + 1,
			AttemptType:     consts.AttemptTypeImmediate,
			AttemptResult:   consts.AttemptResultSuccess,
			RetryDurationMs: int(duration.Milliseconds()),
			AttemptedBy:     "system",
			AttemptedAt:     time.Now(),
		}

		if err != nil {
			retryRecord.AttemptResult = consts.AttemptResultFailed
			retryRecord.ErrorMsg = err.Error()
			retryRecord.ErrorType = s.classifyError(err)
			lastErr = err
		}

		// 保存重试记录
		if createErr := s.eventProcessingRecordDao.CreateRecord(ctx, retryRecord); createErr != nil {
			g.Log().Errorf(ctx, "创建重试记录失败: %v", createErr)
		}

		if err == nil {
			// 成功，返回
			if i > 0 {
				glog.Infof(ctx, "[统一扫块器] %s 事件处理成功 (第%d次尝试)", handler.GetHandlerName(), i+1)
			}
			return nil
		}

		// 检查是否是上下文取消的错误
		if isContextCanceledError(err) {
			glog.Infof(ctx, "[统一扫块器] %s 上下文已取消，停止重试", handler.GetHandlerName())
			return err
		}

		// 如果不是最后一次，等待后重试（指数退避）
		if i < maxRetry-1 {
			delay := retryDelay * time.Duration(1<<i) // 指数退避：5s, 10s, 20s
			glog.Warningf(ctx, "[统一扫块器] %s 事件处理失败 (第%d/%d次): %v，等待 %v 后重试...",
				handler.GetHandlerName(), i+1, maxRetry, err, delay)

			// 使用可中断的等待
			select {
			case <-ctx.Done():
				glog.Infof(ctx, "[统一扫块器] %s 上下文已取消，停止重试", handler.GetHandlerName())
				return ctx.Err()
			case <-time.After(delay):
				// 继续重试
			}
		}
	}

	// 所有重试失败
	glog.Errorf(ctx, "[统一扫块器] %s 所有重试失败 (%d次): %v", handler.GetHandlerName(), maxRetry, lastErr)
	return lastErr
}

// generateEventID 生成事件ID
func (s *UnifiedBlockScanner) generateEventID(event types.Log) string {
	return fmt.Sprintf("%s_%d_%d", event.TxHash.Hex(), event.Index, event.BlockNumber)
}

// classifyError 分类错误类型
func (s *UnifiedBlockScanner) classifyError(err error) string {
	if err == nil {
		return ""
	}

	errStr := err.Error()

	// 网络相关错误
	if strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "dial") {
		return consts.ErrorTypeNetwork
	}

	// 数据库相关错误
	if strings.Contains(errStr, "sql:") ||
		strings.Contains(errStr, "database") ||
		strings.Contains(errStr, "constraint") ||
		strings.Contains(errStr, "duplicate key") {
		return consts.ErrorTypeDatabase
	}

	// 服务相关错误
	if strings.Contains(errStr, "service") ||
		strings.Contains(errStr, "internal") ||
		strings.Contains(errStr, "server") {
		return consts.ErrorTypeService
	}

	// 业务相关错误
	if strings.Contains(errStr, "business") ||
		strings.Contains(errStr, "validation") ||
		strings.Contains(errStr, "invalid") {
		return consts.ErrorTypeBusiness
	}

	// 配置相关错误
	if strings.Contains(errStr, "config") ||
		strings.Contains(errStr, "configuration") {
		return consts.ErrorTypeValidation
	}

	// 默认返回未知错误
	return consts.ErrorTypeUnknown
}
