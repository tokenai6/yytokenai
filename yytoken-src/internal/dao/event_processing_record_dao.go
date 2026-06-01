package dao

import (
	"XWFrame/internal/entity"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/frame/db"
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
)

// IEventProcessingRecordDao 事件处理记录DAO接口
type IEventProcessingRecordDao interface {
	// 记录管理
	CreateRecord(ctx context.Context, record *entity.EventProcessingRecord) error
	GetRecordsByEventID(ctx context.Context, eventID string) ([]*entity.EventProcessingRecord, error)
	GetRecordsByEventIDAndType(ctx context.Context, eventID string, attemptType string) ([]*entity.EventProcessingRecord, error)

	// 统计相关
	GetRetryStats(ctx context.Context, req *GetRetryStatsReq) (*RetryStats, error)
	GetRetryStatsByHandler(ctx context.Context, handlerName string, startTime, endTime time.Time) (*RetryStats, error)
}

// GetRetryStatsReq 获取重试统计请求
type GetRetryStatsReq struct {
	EventID     string    `json:"event_id"`     // 事件ID
	HandlerName string    `json:"handler_name"` // 处理器名称
	AttemptType string    `json:"attempt_type"` // 尝试类型
	StartTime   time.Time `json:"start_time"`   // 开始时间
	EndTime     time.Time `json:"end_time"`     // 结束时间
}

// RetryStats 重试统计
type RetryStats struct {
	TotalAttempts      int     `json:"total_attempts"`      // 总尝试次数
	SuccessfulAttempts int     `json:"successful_attempts"` // 成功次数
	FailedAttempts     int     `json:"failed_attempts"`     // 失败次数
	SuccessRate        float64 `json:"success_rate"`        // 成功率
	AvgDurationMs      float64 `json:"avg_duration_ms"`     // 平均耗时
	MaxDurationMs      int     `json:"max_duration_ms"`     // 最大耗时
	MinDurationMs      int     `json:"min_duration_ms"`     // 最小耗时

	// 按类型统计
	ImmediateAttempts int `json:"immediate_attempts"` // 立即重试次数
	ScheduledAttempts int `json:"scheduled_attempts"` // 定时重试次数
	ManualAttempts    int `json:"manual_attempts"`    // 手动重试次数
	BatchAttempts     int `json:"batch_attempts"`     // 批量重试次数
	RecoveryAttempts  int `json:"recovery_attempts"`  // 系统恢复重试次数
}

// eventProcessingRecordDao 事件处理记录DAO实现
type eventProcessingRecordDao struct {
	db gdb.DB
}

// NewEventProcessingRecordDao 创建事件处理记录DAO
func NewEventProcessingRecordDao() IEventProcessingRecordDao {
	return &eventProcessingRecordDao{
		db: db.GetDB(),
	}
}

// CreateRecord 创建记录
func (d *eventProcessingRecordDao) CreateRecord(ctx context.Context, record *entity.EventProcessingRecord) error {
	// 排除自增ID字段，让数据库自动生成
	_, err := d.db.Model("event_processing_records").Ctx(ctx).FieldsEx("id").Insert(record)
	if err != nil {
		// 该表用于“重试记录观测”，不应影响主流程。
		// 测试/新环境若未执行初始化SQL，会出现类似：
		// - The table "event_processing_records" may not exist, or the table contains no fields
		// - relation "event_processing_records" does not exist
		// 这里直接忽略，避免统一扫块器日志刷屏。
		errStr := err.Error()
		if strings.Contains(errStr, "event_processing_records") &&
			(strings.Contains(errStr, "may not exist") || strings.Contains(errStr, "does not exist")) {
			return nil
		}
		return err
	}
	return nil
}

// GetRecordsByEventID 根据事件ID获取记录
func (d *eventProcessingRecordDao) GetRecordsByEventID(ctx context.Context, eventID string) ([]*entity.EventProcessingRecord, error) {
	var records []*entity.EventProcessingRecord
	err := d.db.Model("event_processing_records").Ctx(ctx).
		Where("event_id", eventID).
		Order("attempted_at ASC").
		Scan(&records)
	return records, err
}

// GetRecordsByEventIDAndType 根据事件ID和类型获取记录
func (d *eventProcessingRecordDao) GetRecordsByEventIDAndType(ctx context.Context, eventID string, attemptType string) ([]*entity.EventProcessingRecord, error) {
	var records []*entity.EventProcessingRecord
	err := d.db.Model("event_processing_records").Ctx(ctx).
		Where("event_id", eventID).
		Where("attempt_type", attemptType).
		Order("attempted_at ASC").
		Scan(&records)
	return records, err
}

// GetRetryStats 获取重试统计
func (d *eventProcessingRecordDao) GetRetryStats(ctx context.Context, req *GetRetryStatsReq) (*RetryStats, error) {
	model := d.db.Model("event_processing_records").Ctx(ctx)

	// 构建查询条件
	if req.EventID != "" {
		model = model.Where("event_id", req.EventID)
	}
	if req.HandlerName != "" {
		// 需要通过JOIN查询
		model = model.LeftJoin("failed_events fe", "event_processing_records.event_id = fe.id").
			Where("fe.handler_name", req.HandlerName)
	}
	if req.AttemptType != "" {
		model = model.Where("attempt_type", req.AttemptType)
	}
	if !req.StartTime.IsZero() {
		model = model.Where("attempted_at >= ?", req.StartTime)
	}
	if !req.EndTime.IsZero() {
		model = model.Where("attempted_at <= ?", req.EndTime)
	}

	stats := &RetryStats{}

	// 总尝试次数
	total, err := model.Count()
	if err != nil {
		return nil, err
	}
	stats.TotalAttempts = total

	if total == 0 {
		return stats, nil
	}

	// 成功和失败次数
	var result struct {
		SuccessCount int `json:"success_count"`
		FailedCount  int `json:"failed_count"`
	}

	err = model.Fields("COUNT(CASE WHEN attempt_result = 'success' THEN 1 END) as success_count, COUNT(CASE WHEN attempt_result = 'failed' THEN 1 END) as failed_count").
		Scan(&result)
	if err != nil {
		return nil, err
	}

	stats.SuccessfulAttempts = result.SuccessCount
	stats.FailedAttempts = result.FailedCount
	stats.SuccessRate = float64(result.SuccessCount) / float64(total) * 100

	// 耗时统计
	var durationStats struct {
		AvgDuration float64 `json:"avg_duration"`
		MaxDuration int     `json:"max_duration"`
		MinDuration int     `json:"min_duration"`
	}

	err = model.Fields("AVG(retry_duration_ms) as avg_duration, MAX(retry_duration_ms) as max_duration, MIN(retry_duration_ms) as min_duration").
		Where("retry_duration_ms IS NOT NULL").
		Scan(&durationStats)
	if err != nil {
		return nil, err
	}

	stats.AvgDurationMs = durationStats.AvgDuration
	stats.MaxDurationMs = durationStats.MaxDuration
	stats.MinDurationMs = durationStats.MinDuration

	// 按类型统计
	typeStats := []struct {
		AttemptType string `json:"attempt_type"`
		Count       int    `json:"count"`
	}{}

	err = model.Fields("attempt_type, COUNT(*) as count").Group("attempt_type").Scan(&typeStats)
	if err != nil {
		return nil, err
	}

	for _, ts := range typeStats {
		switch ts.AttemptType {
		case consts.AttemptTypeImmediate:
			stats.ImmediateAttempts = ts.Count
		case consts.AttemptTypeScheduled:
			stats.ScheduledAttempts = ts.Count
		case consts.AttemptTypeManual:
			stats.ManualAttempts = ts.Count
		case consts.AttemptTypeBatch:
			stats.BatchAttempts = ts.Count
		case consts.AttemptTypeRecovery:
			stats.RecoveryAttempts = ts.Count
		}
	}

	return stats, nil
}

// GetRetryStatsByHandler 根据处理器获取重试统计
func (d *eventProcessingRecordDao) GetRetryStatsByHandler(ctx context.Context, handlerName string, startTime, endTime time.Time) (*RetryStats, error) {
	req := &GetRetryStatsReq{
		HandlerName: handlerName,
		StartTime:   startTime,
		EndTime:     endTime,
	}
	return d.GetRetryStats(ctx, req)
}
