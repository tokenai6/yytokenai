package reward

import (
	"XWFrame/internal/frame/model"
	"time"
)

// RewardTaskBatchEntity 奖励任务批次记录实体
// 用于记录每日奖励调度的各个阶段执行状态
// batch_date 从 record_time 提取日期部分手动设置
//
// 唯一键设计：
// 由于每个 record_time 下有8个不同的任务，所以唯一键是 (record_time, task_name) 的组合
// 这确保了：
// - 同一批次(record_time)下，每个任务(task_name)只有一条记录
// - 允许多个批次在同一时间戳下运行不同的任务
//
// 示例：
// record_time=2025-10-23 20:11:32 可以有8条记录：
//   - (2025-10-23 20:11:32, performance_calculation, ...)
//   - (2025-10-23 20:11:32, static_reward, ...)
//   - ...
type RewardTaskBatchEntity struct {
	model.BaseEntity
	RecordTime   time.Time `json:"record_time" gorm:"type:timestamp;not null;index:idx_reward_batch_record_task,priority:1;comment:批次执行时间"`
	BatchDate    time.Time `json:"batch_date" gorm:"type:date;not null;index:idx_reward_batch_date_status;comment:批次日期（由record_time生成）"`
	TaskSequence int       `json:"task_sequence" gorm:"not null;comment:任务执行序号"`
	TaskName     string    `json:"task_name" gorm:"size:50;not null;uniqueIndex:idx_reward_batch_record_task,priority:2;index:idx_reward_batch_date_task;comment:任务名称"`
	Status       string    `json:"status" gorm:"size:20;not null;default:pending;index:idx_reward_batch_date_status;comment:执行状态"`
	ErrorMessage string    `json:"error_message" gorm:"type:text;comment:失败原因详情"`
}

// TableName 指定表名
func (RewardTaskBatchEntity) TableName() string {
	return "reward_task_batch"
}
