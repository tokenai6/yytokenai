package blockchain

import (
	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
	"XWFrame/internal/frame/consts"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

// RetryEventHandler 重试事件处理器接口
type RetryEventHandler interface {
	ProcessEvent(ctx context.Context, event types.Log) error
	GetHandlerName() string
}

// RetryManager 重试管理器
type RetryManager struct {
	config                   *FaultToleranceConfig
	failedEventDao           dao.IFailedEventDao
	eventProcessingRecordDao dao.IEventProcessingRecordDao
	handlers                 map[string]RetryEventHandler // 处理器映射
}

// NewRetryManager 创建重试管理器
func NewRetryManager(config *FaultToleranceConfig) *RetryManager {
	return &RetryManager{
		config:                   config,
		failedEventDao:           dao.NewFailedEventDao(),
		eventProcessingRecordDao: dao.NewEventProcessingRecordDao(),
		handlers:                 make(map[string]RetryEventHandler),
	}
}

// RegisterHandler 注册处理器
func (rm *RetryManager) RegisterHandler(handler RetryEventHandler) {
	rm.handlers[handler.GetHandlerName()] = handler
}

// ProcessEvent 处理事件（容错性重试，不记录重复的处理记录）
func (rm *RetryManager) ProcessEvent(ctx context.Context, handler RetryEventHandler, event types.Log) error {
	// 直接加入失败事件表，不重复处理
	// 因为事件已经在 processEventWithRetry 中处理过了
	eventID := rm.generateEventID(event)

	// 检查是否已经存在失败事件记录
	existingEvent, err := rm.failedEventDao.GetFailedEvent(ctx, eventID)
	if err == nil && existingEvent != nil {
		// 记录已存在，直接返回错误
		g.Log().Infof(ctx, "失败事件记录已存在: %s", eventID)
		return gerror.New("事件处理失败，已加入容错性重试队列")
	}

	// 创建失败事件记录
	if addErr := rm.addToFailedEvents(ctx, handler, event, gerror.New("3次重试失败，进入容错性重试队列")); addErr != nil {
		g.Log().Errorf(ctx, "添加失败事件失败: %v", addErr)
		return addErr
	}

	return gerror.New("事件处理失败，已加入容错性重试队列")
}

// ProcessScheduledRetries 处理定时重试
func (rm *RetryManager) ProcessScheduledRetries(ctx context.Context, handlerName string) error {
	if !rm.config.Retry.ScheduledRetryEnabled {
		return nil
	}

	// 查询需要重试的事件
	failedEvents, err := rm.failedEventDao.GetEventsForRetry(ctx, handlerName, time.Now())
	if err != nil {
		g.Log().Errorf(ctx, "查询失败事件失败: handler=%s, error=%v", handlerName, err)
		return err
	}

	// 如果有需要重试的事件，记录日志
	if len(failedEvents) > 0 {
		g.Log().Infof(ctx, "找到 %d 个需要重试的事件 (handler: %s)", len(failedEvents), handlerName)
	}

	for _, event := range failedEvents {
		// 更新状态为正在重试
		rm.failedEventDao.UpdateFailedEvent(ctx, event.ID, map[string]interface{}{
			"status": consts.FailedEventStatusRetrying,
		})

		// 重试事件
		err := rm.retryEvent(ctx, event)
		if err != nil {
			// 重试失败，更新重试信息
			g.Log().Warningf(ctx, "事件重试失败: %s, error: %v", event.ID, err)
			rm.updateRetryInfo(ctx, event, err)
		} else {
			// 重试成功，标记为完成
			g.Log().Infof(ctx, "事件重试成功: %s", event.ID)
			rm.markAsCompleted(ctx, event)
		}
	}

	return nil
}

// ManualRetryEvent 手动重试事件
func (rm *RetryManager) ManualRetryEvent(ctx context.Context, eventID string) error {
	if !rm.config.Retry.ManualRetryEnabled {
		return fmt.Errorf("手动重试未启用")
	}

	event, err := rm.failedEventDao.GetFailedEvent(ctx, eventID)
	if err != nil {
		return err
	}
	if event == nil {
		return fmt.Errorf("事件不存在")
	}

	// 更新状态为正在重试
	rm.failedEventDao.UpdateFailedEvent(ctx, eventID, map[string]interface{}{
		"status": consts.FailedEventStatusRetrying,
	})

	// 重试事件
	err = rm.retryEvent(ctx, event)
	if err != nil {
		// 重试失败，更新重试信息
		rm.updateRetryInfo(ctx, event, err)
		return err
	}

	// 重试成功，标记为完成
	rm.markAsCompleted(ctx, event)
	return nil
}

// BatchRetryEvents 批量重试事件
func (rm *RetryManager) BatchRetryEvents(ctx context.Context, eventIDs []string) error {
	if !rm.config.Retry.BatchRetryEnabled {
		return fmt.Errorf("批量重试未启用")
	}

	successCount := 0
	failCount := 0

	for _, eventID := range eventIDs {
		err := rm.ManualRetryEvent(ctx, eventID)
		if err != nil {
			g.Log().Errorf(ctx, "重试事件 %s 失败: %v", eventID, err)
			failCount++
		} else {
			successCount++
		}
	}

	g.Log().Infof(ctx, "批量重试完成: 成功 %d, 失败 %d", successCount, failCount)
	return nil
}

// addToFailedEvents 添加失败事件到失败事件表
func (rm *RetryManager) addToFailedEvents(ctx context.Context, handler RetryEventHandler, event types.Log, err error) error {
	eventData, _ := json.Marshal(event)
	eventID := rm.generateEventID(event)

	nextRetryTime := rm.getNextRetryTime(0)
	failedEvent := &entity.FailedEvent{
		ID:            eventID,
		HandlerName:   handler.GetHandlerName(),
		EventData:     string(eventData),
		Status:        consts.FailedEventStatusPending,
		RetryCount:    0,
		MaxRetries:    rm.config.Retry.FailedEventsMaxRetries,
		NextRetryAt:   &nextRetryTime,
		RetryInterval: rm.getRetryInterval(0),
		ErrorMsg:      err.Error(),
		ErrorType:     rm.classifyError(err),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	g.Log().Infof(ctx, "[重试管理器] 添加失败事件: eventID=%s, handler=%s", eventID, handler.GetHandlerName())

	createErr := rm.failedEventDao.CreateFailedEvent(ctx, failedEvent)
	if createErr != nil {
		g.Log().Errorf(ctx, "[重试管理器] 创建失败事件失败: eventID=%s, error=%v", eventID, createErr)
		return createErr
	}

	g.Log().Infof(ctx, "[重试管理器] 成功添加失败事件: eventID=%s", eventID)
	return nil
}

// retryEvent 重试事件
func (rm *RetryManager) retryEvent(ctx context.Context, failedEvent *entity.FailedEvent) error {
	// 解析事件数据
	var event types.Log
	err := json.Unmarshal([]byte(failedEvent.EventData), &event)
	if err != nil {
		return fmt.Errorf("解析事件数据失败: %v", err)
	}

	// 获取处理器（这里需要根据handlerName获取对应的处理器）
	handler := rm.getHandlerByName(failedEvent.HandlerName)
	if handler == nil {
		return fmt.Errorf("未找到处理器: %s", failedEvent.HandlerName)
	}

	// 重试事件
	startTime := time.Now()
	err = handler.ProcessEvent(ctx, event)
	duration := time.Since(startTime)

	// 记录重试记录
	record := &entity.EventProcessingRecord{
		EventID:         failedEvent.ID,
		AttemptNumber:   failedEvent.RetryCount + 1,
		AttemptType:     consts.AttemptTypeScheduled,
		AttemptResult:   consts.AttemptResultSuccess,
		RetryDurationMs: int(duration.Milliseconds()),
		AttemptedBy:     "system",
		AttemptedAt:     time.Now(),
	}

	if err != nil {
		record.AttemptResult = consts.AttemptResultFailed
		record.ErrorMsg = err.Error()
		record.ErrorType = rm.classifyError(err)
	}

	// 保存重试记录
	rm.eventProcessingRecordDao.CreateRecord(ctx, record)

	return err
}

// updateRetryInfo 更新重试信息
func (rm *RetryManager) updateRetryInfo(ctx context.Context, event *entity.FailedEvent, err error) {
	retryCount := event.RetryCount + 1
	nextRetryAt := rm.getNextRetryTime(retryCount)

	rm.failedEventDao.UpdateRetryInfo(ctx, event.ID, retryCount, nextRetryAt, err.Error())
}

// markAsCompleted 标记为完成
func (rm *RetryManager) markAsCompleted(ctx context.Context, event *entity.FailedEvent) {
	rm.failedEventDao.MarkAsCompleted(ctx, event.ID)
}

// generateEventID 生成事件ID
func (rm *RetryManager) generateEventID(event types.Log) string {
	return fmt.Sprintf("%s_%d_%d", event.TxHash.Hex(), event.Index, event.BlockNumber)
}

// getNextRetryTime 获取下次重试时间
func (rm *RetryManager) getNextRetryTime(retryCount int) time.Time {
	interval := rm.getRetryIntervalDuration(retryCount)
	return time.Now().Add(interval)
}

// getRetryInterval 获取重试间隔字符串
func (rm *RetryManager) getRetryInterval(retryCount int) string {
	if retryCount >= len(rm.config.Retry.ScheduledRetryIntervals) {
		retryCount = len(rm.config.Retry.ScheduledRetryIntervals) - 1
	}

	interval := rm.config.Retry.ScheduledRetryIntervals[retryCount]
	return fmt.Sprintf("%ds", interval)
}

// getRetryIntervalDuration 获取重试间隔时长
func (rm *RetryManager) getRetryIntervalDuration(retryCount int) time.Duration {
	if retryCount >= len(rm.config.Retry.ScheduledRetryIntervals) {
		retryCount = len(rm.config.Retry.ScheduledRetryIntervals) - 1
	}

	interval := rm.config.Retry.ScheduledRetryIntervals[retryCount]
	return time.Duration(interval) * time.Second
}

// classifyError 分类错误
func (rm *RetryManager) classifyError(err error) string {
	errStr := err.Error()

	// 网络错误
	if contains(errStr, []string{"connection refused", "connection timeout", "network unreachable", "i/o timeout"}) {
		return consts.ErrorTypeNetwork
	}

	// 数据库错误
	if contains(errStr, []string{"database", "sql", "constraint", "duplicate key"}) {
		return consts.ErrorTypeDatabase
	}

	// 服务错误
	if contains(errStr, []string{"service unavailable", "internal server error", "service error", "rpc error"}) {
		return consts.ErrorTypeService
	}

	// 超时错误
	if contains(errStr, []string{"timeout", "deadline exceeded", "context deadline"}) {
		return consts.ErrorTypeTimeout
	}

	// 验证错误
	if contains(errStr, []string{"validation", "invalid", "required", "format"}) {
		return consts.ErrorTypeValidation
	}

	// 业务错误
	if contains(errStr, []string{"business", "rule", "permission", "not found"}) {
		return consts.ErrorTypeBusiness
	}

	return consts.ErrorTypeUnknown
}

// contains 检查字符串是否包含任一子字符串
func contains(str string, substrs []string) bool {
	for _, substr := range substrs {
		if len(str) >= len(substr) && str[:len(substr)] == substr {
			return true
		}
	}
	return false
}

// getHandlerByName 根据名称获取处理器
func (rm *RetryManager) getHandlerByName(handlerName string) RetryEventHandler {
	handler, exists := rm.handlers[handlerName]
	if !exists {
		return nil
	}
	return handler
}
