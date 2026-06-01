package blockchain

import (
	"XWFrame/internal/frame/consts"
	"context"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/gogf/gf/v2/frame/g"
)

// HealthStatus 健康状态
type HealthStatus struct {
	Overall string            `json:"overall"` // healthy, warning, critical
	Details map[string]string `json:"details"` // 各组件状态
}

// FaultToleranceService 容错性服务
type FaultToleranceService struct {
	config                *FaultToleranceConfig
	retryManager          *RetryManager
	circuitBreakerManager *CircuitBreakerManager
}

// NewFaultToleranceService 创建容错性服务
func NewFaultToleranceService(config *FaultToleranceConfig) *FaultToleranceService {
	retryManager := NewRetryManager(config)
	circuitBreakerManager := NewCircuitBreakerManager(config)
	return &FaultToleranceService{
		config:                config,
		retryManager:          retryManager,
		circuitBreakerManager: circuitBreakerManager,
	}
}

// Start 启动容错性服务
func (fts *FaultToleranceService) Start(ctx context.Context) error {
	if !fts.config.Enabled {
		g.Log().Info(ctx, "容错性功能未启用，跳过启动")
		return nil
	}

	g.Log().Info(ctx, "启动容错性服务")

	// 启动定时重试任务
	if fts.config.Retry.ScheduledRetryEnabled {
		go fts.startScheduledRetry(ctx)
	}

	// 启动清理任务
	go fts.startCleanupTask(ctx)

	return nil
}

// RegisterHandler 注册事件处理器
func (fts *FaultToleranceService) RegisterHandler(handler RetryEventHandler) {
	fts.retryManager.RegisterHandler(handler)
	g.Log().Infof(context.Background(), "注册事件处理器: %s", handler.GetHandlerName())
}

// ProcessEvent 处理事件（带容错性）
func (fts *FaultToleranceService) ProcessEvent(ctx context.Context, handler RetryEventHandler, event types.Log) error {
	if !fts.config.Enabled {
		// 容错性未启用，直接处理
		return handler.ProcessEvent(ctx, event)
	}

	// 获取熔断器
	circuitBreaker := fts.circuitBreakerManager.GetCircuitBreaker(handler.GetHandlerName())

	// 使用熔断器执行
	return circuitBreaker.Execute(ctx, func() error {
		return fts.retryManager.ProcessEvent(ctx, handler, event)
	})
}

// startScheduledRetry 启动定时重试任务
func (fts *FaultToleranceService) startScheduledRetry(ctx context.Context) {
	interval := fts.config.Retry.FailedEventsAutoRetryInterval
	g.Log().Infof(ctx, "🚀 启动定时重试任务，执行间隔: %d秒", interval)

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			g.Log().Info(ctx, "⏹️ 定时重试任务已停止")
			return
		case <-ticker.C:
			fts.processScheduledRetries(ctx)
		}
	}
}

// processScheduledRetries 处理定时重试
func (fts *FaultToleranceService) processScheduledRetries(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			g.Log().Errorf(ctx, "processScheduledRetries panic恢复: %v", r)
		}
	}()

	// 处理各个处理器的重试
	handlers := []string{consts.BlockchainHandlerNameMint, consts.BlockchainHandlerNameBurn, consts.BlockchainHandlerNameStake}

	for _, handlerName := range handlers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					g.Log().Errorf(ctx, "处理器 %s 定时重试 panic恢复: %v", handlerName, r)
				}
			}()

			if err := fts.retryManager.ProcessScheduledRetries(ctx, handlerName); err != nil {
				g.Log().Errorf(ctx, "处理定时重试失败: handler=%s, error=%v", handlerName, err)
			}
		}()
	}
}

// startCleanupTask 启动清理任务
func (fts *FaultToleranceService) startCleanupTask(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(fts.config.Retry.FailedEventsCleanupInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fts.cleanupExpiredEvents(ctx)
		}
	}
}

// cleanupExpiredEvents 清理过期事件
func (fts *FaultToleranceService) cleanupExpiredEvents(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			g.Log().Errorf(ctx, "cleanupExpiredEvents panic恢复: %v", r)
		}
	}()

	// 清理7天前的已完成和已放弃事件
	beforeTime := time.Now().Add(-time.Duration(fts.config.Retry.FailedEventsTTL) * time.Second)

	affected, err := fts.retryManager.failedEventDao.CleanupExpiredEvents(ctx, beforeTime, fts.config.Retry.FailedEventsCleanupBatchSize)
	if err != nil {
		g.Log().Errorf(ctx, "清理过期事件失败: %v", err)
		return
	}

	if affected > 0 {
		g.Log().Infof(ctx, "清理过期事件完成: 清理了 %d 条记录", affected)
	}
}

// GetRetryManager 获取重试管理器
func (fts *FaultToleranceService) GetRetryManager() *RetryManager {
	return fts.retryManager
}

// GetCircuitBreakerManager 获取熔断器管理器
func (fts *FaultToleranceService) GetCircuitBreakerManager() *CircuitBreakerManager {
	return fts.circuitBreakerManager
}

// GetHealthStatus 获取健康状态
func (fts *FaultToleranceService) GetHealthStatus() *HealthStatus {
	// 简化的健康状态检查
	return &HealthStatus{
		Overall: "healthy",
		Details: map[string]string{
			"retry_manager":           "healthy",
			"circuit_breaker_manager": "healthy",
		},
	}
}
