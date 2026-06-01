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

// IFailedEventDao 失败事件DAO接口
type IFailedEventDao interface {
	// 失败事件管理
	CreateFailedEvent(ctx context.Context, event *entity.FailedEvent) error
	GetFailedEvent(ctx context.Context, eventID string) (*entity.FailedEvent, error)
	UpdateFailedEvent(ctx context.Context, eventID string, updates map[string]interface{}) error
	DeleteFailedEvent(ctx context.Context, eventID string) error
	GetFailedEvents(ctx context.Context, req *GetFailedEventsReq) ([]*entity.FailedEvent, int, error)

	// 重试相关
	GetEventsForRetry(ctx context.Context, handlerName string, beforeTime time.Time) ([]*entity.FailedEvent, error)
	UpdateRetryInfo(ctx context.Context, eventID string, retryCount int, nextRetryAt time.Time, errorMsg string) error
	MarkAsCompleted(ctx context.Context, eventID string) error
	MarkAsAbandoned(ctx context.Context, eventID string) error

	// 统计相关
	GetFailedEventStats(ctx context.Context, handlerName string) (*FailedEventStats, error)
	CleanupExpiredEvents(ctx context.Context, beforeTime time.Time, batchSize int) (int, error)
}

// GetFailedEventsReq 获取失败事件请求
type GetFailedEventsReq struct {
	HandlerName string `json:"handler_name"` // 处理器名称
	Status      string `json:"status"`       // 状态
	Page        int    `json:"page"`         // 页码
	PageSize    int    `json:"page_size"`    // 页大小
	StartTime   string `json:"start_time"`   // 开始时间
	EndTime     string `json:"end_time"`     // 结束时间
}

// FailedEventStats 失败事件统计
type FailedEventStats struct {
	TotalCount     int `json:"total_count"`     // 总数量
	PendingCount   int `json:"pending_count"`   // 等待重试数量
	RetryingCount  int `json:"retrying_count"`  // 正在重试数量
	CompletedCount int `json:"completed_count"` // 已完成数量
	AbandonedCount int `json:"abandoned_count"` // 已放弃数量
}

// failedEventDao 失败事件DAO实现
type failedEventDao struct {
	db gdb.DB
}

// NewFailedEventDao 创建失败事件DAO
func NewFailedEventDao() IFailedEventDao {
	return &failedEventDao{
		db: db.GetDB(),
	}
}

// CreateFailedEvent 创建失败事件
func (d *failedEventDao) CreateFailedEvent(ctx context.Context, event *entity.FailedEvent) error {
	// 使用 INSERT ... ON CONFLICT 处理重复插入
	// 注意：failed_events表的id字段不是自增的，是VARCHAR(64) PRIMARY KEY
	_, err := d.db.Model("failed_events").Ctx(ctx).Insert(event)
	if err != nil {
		// 如果是主键冲突，检查是否已存在
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			// 检查记录是否已存在
			existing, checkErr := d.GetFailedEvent(ctx, event.ID)
			if checkErr == nil && existing != nil {
				// 记录已存在，返回成功
				return nil
			}
		}
		return err
	}
	return nil
}

// GetFailedEvent 获取失败事件
func (d *failedEventDao) GetFailedEvent(ctx context.Context, eventID string) (*entity.FailedEvent, error) {
	var event entity.FailedEvent
	err := d.db.Model("failed_events").Ctx(ctx).Where("id", eventID).Scan(&event)
	if err != nil {
		return nil, err
	}
	return &event, nil
}

// UpdateFailedEvent 更新失败事件
func (d *failedEventDao) UpdateFailedEvent(ctx context.Context, eventID string, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()
	_, err := d.db.Model("failed_events").Ctx(ctx).Where("id", eventID).Update(updates)
	return err
}

// DeleteFailedEvent 删除失败事件
func (d *failedEventDao) DeleteFailedEvent(ctx context.Context, eventID string) error {
	_, err := d.db.Model("failed_events").Ctx(ctx).Where("id", eventID).Delete()
	return err
}

// GetFailedEvents 获取失败事件列表
func (d *failedEventDao) GetFailedEvents(ctx context.Context, req *GetFailedEventsReq) ([]*entity.FailedEvent, int, error) {
	model := d.db.Model("failed_events").Ctx(ctx)

	// 构建查询条件
	if req.HandlerName != "" {
		model = model.Where("handler_name", req.HandlerName)
	}
	if req.Status != "" {
		model = model.Where("status", req.Status)
	}
	if req.StartTime != "" {
		model = model.Where("created_at >= ?", req.StartTime)
	}
	if req.EndTime != "" {
		model = model.Where("created_at <= ?", req.EndTime)
	}

	// 获取总数
	total, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	var events []*entity.FailedEvent
	err = model.Order("created_at DESC").Page(page, pageSize).Scan(&events)
	if err != nil {
		return nil, 0, err
	}

	return events, total, nil
}

// GetEventsForRetry 获取需要重试的事件
func (d *failedEventDao) GetEventsForRetry(ctx context.Context, handlerName string, beforeTime time.Time) ([]*entity.FailedEvent, error) {
	var events []*entity.FailedEvent
	err := d.db.Model("failed_events").Ctx(ctx).
		Where("handler_name", handlerName).
		Where("status", consts.FailedEventStatusPending).
		Where("next_retry_at <= ?", beforeTime).
		Order("next_retry_at ASC").
		Scan(&events)
	return events, err
}

// UpdateRetryInfo 更新重试信息
func (d *failedEventDao) UpdateRetryInfo(ctx context.Context, eventID string, retryCount int, nextRetryAt time.Time, errorMsg string) error {
	updates := map[string]interface{}{
		"retry_count":   retryCount,
		"next_retry_at": nextRetryAt,
		"error_msg":     errorMsg,
		"status":        consts.FailedEventStatusPending,
		"updated_at":    time.Now(),
	}
	return d.UpdateFailedEvent(ctx, eventID, updates)
}

// MarkAsCompleted 标记为完成
func (d *failedEventDao) MarkAsCompleted(ctx context.Context, eventID string) error {
	updates := map[string]interface{}{
		"status":       consts.FailedEventStatusCompleted,
		"completed_at": time.Now(),
		"updated_at":   time.Now(),
	}
	return d.UpdateFailedEvent(ctx, eventID, updates)
}

// MarkAsAbandoned 标记为放弃
func (d *failedEventDao) MarkAsAbandoned(ctx context.Context, eventID string) error {
	updates := map[string]interface{}{
		"status":     consts.FailedEventStatusAbandoned,
		"updated_at": time.Now(),
	}
	return d.UpdateFailedEvent(ctx, eventID, updates)
}

// GetFailedEventStats 获取失败事件统计
func (d *failedEventDao) GetFailedEventStats(ctx context.Context, handlerName string) (*FailedEventStats, error) {
	model := d.db.Model("failed_events").Ctx(ctx)
	if handlerName != "" {
		model = model.Where("handler_name", handlerName)
	}

	stats := &FailedEventStats{}

	// 总数量
	total, err := model.Count()
	if err != nil {
		return nil, err
	}
	stats.TotalCount = total

	// 各状态数量
	statusCounts := []struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
	}{}

	err = model.Fields("status, COUNT(*) as count").Group("status").Scan(&statusCounts)
	if err != nil {
		return nil, err
	}

	for _, sc := range statusCounts {
		switch sc.Status {
		case consts.FailedEventStatusPending:
			stats.PendingCount = sc.Count
		case consts.FailedEventStatusRetrying:
			stats.RetryingCount = sc.Count
		case consts.FailedEventStatusCompleted:
			stats.CompletedCount = sc.Count
		case consts.FailedEventStatusAbandoned:
			stats.AbandonedCount = sc.Count
		}
	}

	return stats, nil
}

// CleanupExpiredEvents 清理过期事件
func (d *failedEventDao) CleanupExpiredEvents(ctx context.Context, beforeTime time.Time, batchSize int) (int, error) {
	if batchSize <= 0 {
		batchSize = 100
	}

	// 先查询需要删除的ID列表，避免PostgreSQL不支持DELETE ... LIMIT语法
	var ids []string
	err := d.db.Model("failed_events").Ctx(ctx).
		Fields("id").
		Where("status IN (?)", []string{consts.FailedEventStatusCompleted, consts.FailedEventStatusAbandoned}).
		Where("updated_at < ?", beforeTime).
		Order("updated_at ASC").
		Limit(batchSize).
		Scan(&ids)
	if err != nil {
		return 0, err
	}

	if len(ids) == 0 {
		return 0, nil
	}

	result, err := d.db.Model("failed_events").Ctx(ctx).
		WhereIn("id", ids).
		Delete()
	if err != nil {
		return 0, err
	}

	affected, _ := result.RowsAffected()
	return int(affected), nil
}
