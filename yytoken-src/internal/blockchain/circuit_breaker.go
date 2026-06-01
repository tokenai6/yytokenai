package blockchain

import (
	"XWFrame/internal/frame/consts"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CircuitBreakerState 熔断器状态
type CircuitBreakerState int

const (
	CircuitBreakerStateClosed   CircuitBreakerState = iota // 关闭状态
	CircuitBreakerStateOpen                                // 开启状态
	CircuitBreakerStateHalfOpen                            // 半开状态
)

// String 返回状态字符串
func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitBreakerStateClosed:
		return "closed"
	case CircuitBreakerStateOpen:
		return "open"
	case CircuitBreakerStateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	config *CircuitBreakerConfig
	state  CircuitBreakerState
	mutex  sync.RWMutex

	// 统计信息
	requestCount    int64     // 请求总数
	successCount    int64     // 成功数
	failureCount    int64     // 失败数
	lastFailureTime time.Time // 最后失败时间

	// 状态变更时间
	stateChangeTime time.Time
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config:          config,
		state:           CircuitBreakerStateClosed,
		stateChangeTime: time.Now(),
	}
}

// Execute 执行操作
func (cb *CircuitBreaker) Execute(ctx context.Context, operation func() error) error {
	// 检查熔断器状态
	if !cb.allowRequest() {
		return fmt.Errorf("熔断器开启，拒绝请求")
	}

	// 执行操作
	err := operation()

	// 记录结果
	cb.recordResult(err)

	return err
}

// allowRequest 检查是否允许请求
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	switch cb.state {
	case CircuitBreakerStateClosed:
		return true
	case CircuitBreakerStateOpen:
		// 检查是否可以进入半开状态
		if time.Since(cb.lastFailureTime) >= time.Duration(cb.config.Timeout)*time.Second {
			cb.mutex.RUnlock()
			cb.mutex.Lock()
			if cb.state == CircuitBreakerStateOpen &&
				time.Since(cb.lastFailureTime) >= time.Duration(cb.config.Timeout)*time.Second {
				cb.setState(CircuitBreakerStateHalfOpen)
			}
			cb.mutex.Unlock()
			cb.mutex.RLock()
		}
		return cb.state == CircuitBreakerStateHalfOpen
	case CircuitBreakerStateHalfOpen:
		return true
	default:
		return false
	}
}

// recordResult 记录操作结果
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.requestCount++

	if err == nil {
		cb.successCount++
		cb.onSuccess()
	} else {
		cb.failureCount++
		cb.lastFailureTime = time.Now()
		cb.onFailure()
	}
}

// onSuccess 成功回调
func (cb *CircuitBreaker) onSuccess() {
	switch cb.state {
	case CircuitBreakerStateHalfOpen:
		// 半开状态下成功，检查是否可以关闭熔断器
		if cb.successCount >= int64(cb.config.SuccessThreshold) {
			cb.setState(CircuitBreakerStateClosed)
		}
	}
}

// onFailure 失败回调
func (cb *CircuitBreaker) onFailure() {
	switch cb.state {
	case CircuitBreakerStateClosed:
		// 关闭状态下失败，检查是否需要开启熔断器
		if cb.shouldOpen() {
			cb.setState(CircuitBreakerStateOpen)
		}
	case CircuitBreakerStateHalfOpen:
		// 半开状态下失败，立即开启熔断器
		cb.setState(CircuitBreakerStateOpen)
	}
}

// shouldOpen 检查是否应该开启熔断器
func (cb *CircuitBreaker) shouldOpen() bool {
	// 检查请求量阈值
	if cb.requestCount < int64(cb.config.RequestVolumeThreshold) {
		return false
	}

	// 计算失败率
	failureRate := float64(cb.failureCount) / float64(cb.requestCount) * 100
	return failureRate >= float64(cb.config.FailureThreshold)
}

// setState 设置状态
func (cb *CircuitBreaker) setState(newState CircuitBreakerState) {
	if cb.state != newState {
		oldState := cb.state
		cb.state = newState
		cb.stateChangeTime = time.Now()

		// 重置计数器
		if newState == CircuitBreakerStateClosed {
			cb.resetCounters()
		}

		g.Log().Infof(context.Background(), "熔断器状态变更: %s -> %s", oldState.String(), newState.String())
	}
}

// resetCounters 重置计数器
func (cb *CircuitBreaker) resetCounters() {
	cb.requestCount = 0
	cb.successCount = 0
	cb.failureCount = 0
}

// GetState 获取当前状态
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// GetStats 获取统计信息
func (cb *CircuitBreaker) GetStats() *CircuitBreakerStats {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	stats := &CircuitBreakerStats{
		State:           cb.state.String(),
		RequestCount:    cb.requestCount,
		SuccessCount:    cb.successCount,
		FailureCount:    cb.failureCount,
		LastFailureTime: cb.lastFailureTime,
		StateChangeTime: cb.stateChangeTime,
	}

	if cb.requestCount > 0 {
		stats.SuccessRate = float64(cb.successCount) / float64(cb.requestCount) * 100
		stats.FailureRate = float64(cb.failureCount) / float64(cb.requestCount) * 100
	}

	return stats
}

// CircuitBreakerStats 熔断器统计信息
type CircuitBreakerStats struct {
	State           string    `json:"state"`             // 当前状态
	RequestCount    int64     `json:"request_count"`     // 请求总数
	SuccessCount    int64     `json:"success_count"`     // 成功数
	FailureCount    int64     `json:"failure_count"`     // 失败数
	SuccessRate     float64   `json:"success_rate"`      // 成功率
	FailureRate     float64   `json:"failure_rate"`      // 失败率
	LastFailureTime time.Time `json:"last_failure_time"` // 最后失败时间
	StateChangeTime time.Time `json:"state_change_time"` // 状态变更时间
}

// CircuitBreakerManager 熔断器管理器
type CircuitBreakerManager struct {
	config          *FaultToleranceConfig
	circuitBreakers map[string]*CircuitBreaker
	mutex           sync.RWMutex
}

// NewCircuitBreakerManager 创建熔断器管理器
func NewCircuitBreakerManager(config *FaultToleranceConfig) *CircuitBreakerManager {
	return &CircuitBreakerManager{
		config:          config,
		circuitBreakers: make(map[string]*CircuitBreaker),
	}
}

// GetCircuitBreaker 获取熔断器
func (cbm *CircuitBreakerManager) GetCircuitBreaker(name string) *CircuitBreaker {
	cbm.mutex.RLock()
	cb, exists := cbm.circuitBreakers[name]
	cbm.mutex.RUnlock()

	if !exists {
		cbm.mutex.Lock()
		defer cbm.mutex.Unlock()

		// 双重检查
		cb, exists = cbm.circuitBreakers[name]
		if !exists {
			// 根据名称选择配置
			config := cbm.getConfigForName(name)
			cb = NewCircuitBreaker(config)
			cbm.circuitBreakers[name] = cb
		}
	}

	return cb
}

// getConfigForName 根据名称获取配置
func (cbm *CircuitBreakerManager) getConfigForName(name string) *CircuitBreakerConfig {
	// 从配置映射中获取配置
	if configData, exists := consts.CircuitBreakerConfigs[name]; exists {
		return &CircuitBreakerConfig{
			Enabled:                cbm.config.CircuitBreaker.Enabled,
			FailureThreshold:       configData.FailureThreshold,
			SuccessThreshold:       configData.SuccessThreshold,
			Timeout:                configData.Timeout,
			RequestVolumeThreshold: configData.RequestVolumeThreshold,
		}
	}

	// 使用默认配置
	return &cbm.config.CircuitBreaker
}

// GetAllStats 获取所有熔断器统计信息
func (cbm *CircuitBreakerManager) GetAllStats() map[string]*CircuitBreakerStats {
	cbm.mutex.RLock()
	defer cbm.mutex.RUnlock()

	stats := make(map[string]*CircuitBreakerStats)
	for name, cb := range cbm.circuitBreakers {
		stats[name] = cb.GetStats()
	}

	return stats
}
