package scheduler

import (
	"time"
)

// ScheduledTask 定时任务实体
type ScheduledTask struct {
	Id                   int64     `json:"id" g:"primaryKey;autoIncrement"`
	Name                 string    `json:"name" g:"column:name;type:varchar(100);uniqueIndex;not null;comment:任务名称"`
	HandlerName          string    `json:"handler_name" g:"column:handler_name;type:varchar(100);not null;comment:处理器名称"`
	CronExpression       string    `json:"cron_expression" g:"column:cron_expression;type:varchar(100);not null;comment:Cron表达式"`
	Timeout              int       `json:"timeout" g:"column:timeout;type:int;default:300;comment:超时时间(秒)"`
	Status               int       `json:"status" g:"column:status;type:int;default:1;comment:任务状态:0=禁用,1=启用"`
	DependencyTaskName   *string   `json:"dependency_task_name" g:"column:dependency_task_name;type:varchar(100);comment:依赖的任务名称"`
	DependencyTimeWindow int       `json:"dependency_time_window" g:"column:dependency_time_window;type:int;default:30;comment:依赖时间窗口(分钟)"`
	Params               *string   `json:"params" g:"column:params;type:jsonb;comment:任务参数"`
	Description          *string   `json:"description" g:"column:description;type:text;comment:任务描述"`
	CreatedAt            time.Time `json:"created_at" g:"column:created_at;autoCreateTime;comment:创建时间"`
	UpdatedAt            time.Time `json:"updated_at" g:"column:updated_at;autoUpdateTime;comment:更新时间"`
}

// TableName 指定表名
func (ScheduledTask) TableName() string {
	return "scheduled_tasks"
}
