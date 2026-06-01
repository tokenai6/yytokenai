package scheduler

import (
	"time"
)

// TaskExecutionLogs 任务执行日志实体
type TaskExecutionLogs struct {
	Id        int64      `json:"id" orm:"id"`
	ExecId    string     `json:"exec_id" orm:"exec_id"`
	TaskId    int64      `json:"task_id" orm:"task_id"`
	TaskName  string     `json:"task_name" orm:"task_name"`
	Status    int        `json:"status" orm:"status"`
	StartTime time.Time  `json:"start_time" orm:"start_time"`
	EndTime   *time.Time `json:"end_time" orm:"end_time"`
	Duration  int64      `json:"duration" orm:"duration"`
	Attempt   int        `json:"attempt" orm:"attempt"`
	ErrorMsg  string     `json:"error_msg" orm:"error_msg"`
	CreatedAt time.Time  `json:"created_at" orm:"created_at"`
}

// TableName 指定表名
func (TaskExecutionLogs) TableName() string {
	return "task_execution_logs"
}
