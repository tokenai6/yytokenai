package blockchain

import (
	"XWFrame/internal/frame/consts"
	"context"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// FaultToleranceConfig 容错性配置
type FaultToleranceConfig struct {
	// 基础配置
	Enabled bool // 是否启用容错性功能

	// 重试机制配置（包含失败事件管理）
	Retry RetryConfig

	// 熔断器配置
	CircuitBreaker CircuitBreakerConfig
}

// RetryConfig 重试机制配置（包含失败事件管理）
type RetryConfig struct {
	Enabled                       bool  // 启用重试机制
	ImmediateRetryEnabled         bool  // 启用立即重试
	ImmediateRetryMaxAttempts     int   // 立即重试最大次数
	ImmediateRetryTimeout         int   // 立即重试超时时间（秒）
	ScheduledRetryEnabled         bool  // 启用定时重试
	ScheduledRetryIntervals       []int // 定时重试间隔（秒）
	ManualRetryEnabled            bool  // 启用手动重试
	BatchRetryEnabled             bool  // 启用批量重试
	BatchRetryMaxSize             int   // 批量重试最大数量
	RecoveryRetryEnabled          bool  // 启用系统恢复重试
	RecoveryRetryTriggerOnStartup bool  // 系统启动时触发恢复重试

	// 失败事件管理配置（合并到重试机制中）
	FailedEventsTTL                 int64 // 失败事件保留时间（秒）
	FailedEventsMaxRetries          int   // 最大重试次数（0=无限制）
	FailedEventsAutoRetryEnabled    bool  // 是否启用自动重试
	FailedEventsAutoRetryInterval   int   // 自动重试间隔（秒）
	FailedEventsAutoRetryLimit      int   // 自动重试次数限制
	FailedEventsStatusCheckInterval int   // 状态检查间隔（秒）
	FailedEventsCleanupInterval     int   // 清理间隔（秒）
	FailedEventsCleanupBatchSize    int   // 清理批量大小
}

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	Enabled                bool        // 启用熔断器
	FailureThreshold       int         // 失败阈值（百分比）
	SuccessThreshold       int         // 成功阈值（百分比）
	Timeout                int         // 熔断超时时间（秒）
	RequestVolumeThreshold int         // 请求量阈值
	SleepWindow            int         // 睡眠窗口（秒）
	TestRequestCount       int         // 测试请求数
	NetworkLayer           LayerConfig // 网络层熔断配置
	ServiceLayer           LayerConfig // 服务层熔断配置
	BusinessLayer          LayerConfig // 业务层熔断配置
}

// LayerConfig 分层熔断配置
type LayerConfig struct {
	Enabled          bool // 启用该层熔断
	FailureThreshold int  // 失败阈值
	Timeout          int  // 超时时间（秒）
}

// LoadFaultToleranceConfig 加载容错性配置（自动使用默认值）
func LoadFaultToleranceConfig(ctx context.Context) (*FaultToleranceConfig, error) {
	cfg := &FaultToleranceConfig{}

	// ==========================================
	// 基础配置
	// ==========================================
	cfg.Enabled = g.Cfg().MustGet(ctx, "blockchain.fault_tolerance.enabled", consts.FaultToleranceDefaultRetryEnabled).Bool()

	// ==========================================
	// 重试机制配置（包含失败事件管理，使用总开关，其他使用常量默认值）
	// ==========================================
	cfg.Retry.Enabled = cfg.Enabled // 重试机制跟随总开关
	cfg.Retry.ImmediateRetryEnabled = consts.FaultToleranceDefaultImmediateRetryEnabled
	cfg.Retry.ImmediateRetryMaxAttempts = consts.FaultToleranceDefaultImmediateRetryMaxAttempts
	cfg.Retry.ImmediateRetryTimeout = consts.FaultToleranceDefaultImmediateRetryTimeout
	cfg.Retry.ScheduledRetryEnabled = consts.FaultToleranceDefaultScheduledRetryEnabled
	cfg.Retry.ScheduledRetryIntervals = consts.FaultToleranceDefaultScheduledRetryIntervals
	cfg.Retry.ManualRetryEnabled = consts.FaultToleranceDefaultManualRetryEnabled
	cfg.Retry.BatchRetryEnabled = consts.FaultToleranceDefaultBatchRetryEnabled
	cfg.Retry.BatchRetryMaxSize = consts.FaultToleranceDefaultBatchRetryMaxSize
	cfg.Retry.RecoveryRetryEnabled = consts.FaultToleranceDefaultRecoveryRetryEnabled
	cfg.Retry.RecoveryRetryTriggerOnStartup = consts.FaultToleranceDefaultRecoveryRetryTriggerOnStartup

	// 失败事件管理配置（合并到重试机制中）
	cfg.Retry.FailedEventsTTL = consts.FaultToleranceDefaultFailedEventsTTL
	cfg.Retry.FailedEventsMaxRetries = consts.FaultToleranceDefaultFailedEventsMaxRetries
	cfg.Retry.FailedEventsAutoRetryEnabled = consts.FaultToleranceDefaultFailedEventsAutoRetryEnabled
	cfg.Retry.FailedEventsAutoRetryInterval = consts.FaultToleranceDefaultFailedEventsAutoRetryInterval
	cfg.Retry.FailedEventsAutoRetryLimit = consts.FaultToleranceDefaultFailedEventsAutoRetryLimit
	cfg.Retry.FailedEventsStatusCheckInterval = consts.FaultToleranceDefaultFailedEventsStatusCheckInterval
	cfg.Retry.FailedEventsCleanupInterval = consts.FaultToleranceDefaultFailedEventsCleanupInterval
	cfg.Retry.FailedEventsCleanupBatchSize = consts.FaultToleranceDefaultFailedEventsCleanupBatchSize

	// ==========================================
	// 熔断器配置（只读取启用开关，其他使用常量默认值）
	// ==========================================
	cfg.CircuitBreaker.Enabled = g.Cfg().MustGet(ctx, "blockchain.fault_tolerance.circuit_breaker_enabled", consts.FaultToleranceDefaultCircuitBreakerEnabled).Bool()
	cfg.CircuitBreaker.FailureThreshold = consts.FaultToleranceDefaultFailureThreshold
	cfg.CircuitBreaker.SuccessThreshold = consts.FaultToleranceDefaultSuccessThreshold
	cfg.CircuitBreaker.Timeout = consts.FaultToleranceDefaultCircuitBreakerTimeout
	cfg.CircuitBreaker.RequestVolumeThreshold = consts.FaultToleranceDefaultRequestVolumeThreshold
	cfg.CircuitBreaker.SleepWindow = consts.FaultToleranceDefaultSleepWindow
	cfg.CircuitBreaker.TestRequestCount = consts.FaultToleranceDefaultTestRequestCount

	// 分层熔断配置（使用常量默认值）
	cfg.CircuitBreaker.NetworkLayer.Enabled = consts.FaultToleranceDefaultNetworkLayerEnabled
	cfg.CircuitBreaker.NetworkLayer.FailureThreshold = consts.FaultToleranceDefaultNetworkLayerFailureThreshold
	cfg.CircuitBreaker.NetworkLayer.Timeout = consts.FaultToleranceDefaultNetworkLayerTimeout

	cfg.CircuitBreaker.ServiceLayer.Enabled = consts.FaultToleranceDefaultServiceLayerEnabled
	cfg.CircuitBreaker.ServiceLayer.FailureThreshold = consts.FaultToleranceDefaultServiceLayerFailureThreshold
	cfg.CircuitBreaker.ServiceLayer.Timeout = consts.FaultToleranceDefaultServiceLayerTimeout

	cfg.CircuitBreaker.BusinessLayer.Enabled = consts.FaultToleranceDefaultBusinessLayerEnabled
	cfg.CircuitBreaker.BusinessLayer.FailureThreshold = consts.FaultToleranceDefaultBusinessLayerFailureThreshold
	cfg.CircuitBreaker.BusinessLayer.Timeout = consts.FaultToleranceDefaultBusinessLayerTimeout

	return cfg, nil
}

// GetRetryInterval 获取指定重试次数的间隔时间
func (c *FaultToleranceConfig) GetRetryInterval(retryCount int) time.Duration {
	if retryCount < 0 || retryCount >= len(c.Retry.ScheduledRetryIntervals) {
		// 如果重试次数超出配置范围，使用最后一个间隔
		retryCount = len(c.Retry.ScheduledRetryIntervals) - 1
	}
	return time.Duration(c.Retry.ScheduledRetryIntervals[retryCount]) * time.Second
}

// IsRetryEnabled 检查重试机制是否启用
func (c *FaultToleranceConfig) IsRetryEnabled() bool {
	return c.Enabled && c.Retry.Enabled
}

// IsCircuitBreakerEnabled 检查熔断器是否启用
func (c *FaultToleranceConfig) IsCircuitBreakerEnabled() bool {
	return c.Enabled && c.CircuitBreaker.Enabled
}
