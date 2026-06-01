package entity

import (
	"time"
)

// FailedEvent 失败事件实体
type FailedEvent struct {
	ID            string     `json:"id" gorm:"primaryKey;type:varchar(128)"`              // 事件唯一标识
	HandlerName   string     `json:"handler_name" gorm:"type:varchar(100);not null"`      // 处理器名称
	EventData     string     `json:"event_data" gorm:"type:jsonb;not null"`               // 原始事件数据
	Status        string     `json:"status" gorm:"type:varchar(20);default:'pending'"`    // 状态
	RetryCount    int        `json:"retry_count" gorm:"default:0"`                        // 已重试次数
	MaxRetries    int        `json:"max_retries" gorm:"default:0"`                        // 最大重试次数
	NextRetryAt   *time.Time `json:"next_retry_at" gorm:"type:timestamp"`                 // 下次重试时间
	RetryInterval string     `json:"retry_interval" gorm:"type:varchar(20);default:'5m'"` // 重试间隔
	ErrorMsg      string     `json:"error_msg" gorm:"type:text"`                          // 错误信息
	ErrorType     string     `json:"error_type" gorm:"type:varchar(50)"`                  // 错误类型
	CreatedAt     time.Time  `json:"created_at" gorm:"type:timestamptz;default:current_timestamp"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"type:timestamptz;default:current_timestamp"`
	CompletedAt   *time.Time `json:"completed_at" gorm:"type:timestamptz"` // 完成时间
}

// TableName 返回表名
func (FailedEvent) TableName() string {
	return "failed_events"
}

// EventProcessingRecord 事件处理记录实体
type EventProcessingRecord struct {
	ID              int       `json:"id" gorm:"primaryKey;autoIncrement"`                             // 主键ID
	EventID         string    `json:"event_id" gorm:"type:varchar(128);not null"`                     // 事件ID
	AttemptNumber   int       `json:"attempt_number" gorm:"not null"`                                 // 尝试次数
	AttemptType     string    `json:"attempt_type" gorm:"type:varchar(20);not null"`                  // 尝试类型
	AttemptResult   string    `json:"attempt_result" gorm:"type:varchar(20);not null"`                // 尝试结果
	ErrorMsg        string    `json:"error_msg" gorm:"type:text"`                                     // 错误信息
	ErrorType       string    `json:"error_type" gorm:"type:varchar(50)"`                             // 错误类型
	RetryDurationMs int       `json:"retry_duration_ms" gorm:"type:integer"`                          // 重试耗时
	AttemptedBy     string    `json:"attempted_by" gorm:"type:varchar(100)"`                          // 操作者
	AttemptedAt     time.Time `json:"attempted_at" gorm:"type:timestamptz;default:current_timestamp"` // 尝试时间
}

// TableName 返回表名
func (EventProcessingRecord) TableName() string {
	return "event_processing_records"
}
