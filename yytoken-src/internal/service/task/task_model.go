package task

import (
	"time"
)

// TaskCreateReq 创建任务请求
type TaskCreateReq struct {
	Name                 string  `json:"name" v:"required#任务名称不能为空"`
	HandlerName          string  `json:"handler_name" v:"required#处理器名称不能为空"`
	CronExpression       string  `json:"cron_expression" v:"required#Cron表达式不能为空"`
	Timeout              int     `json:"timeout" v:"min:1#超时时间必须大于0"`
	Status               int     `json:"status" v:"in:0,1#状态值无效"`
	DependencyTaskName   *string `json:"dependency_task_name"`
	DependencyTimeWindow int     `json:"dependency_time_window"`
	Params               *string `json:"params"`
	Description          *string `json:"description"`
}

// TaskUpdateReq 更新任务请求
type TaskUpdateReq struct {
	Id                   int64   `json:"id" v:"required#任务ID不能为空"`
	Name                 string  `json:"name" v:"required#任务名称不能为空"`
	HandlerName          string  `json:"handler_name" v:"required#处理器名称不能为空"`
	CronExpression       string  `json:"cron_expression" v:"required#Cron表达式不能为空"`
	Timeout              int     `json:"timeout" v:"min:1#超时时间必须大于0"`
	Status               int     `json:"status" v:"in:0,1#状态值无效"`
	DependencyTaskName   *string `json:"dependency_task_name"`
	DependencyTimeWindow int     `json:"dependency_time_window"`
	Params               *string `json:"params"`
	Description          *string `json:"description"`
}

// TaskListReq 任务列表请求
type TaskListReq struct {
	Page int `json:"page" v:"min:1#页码必须大于0"`
	Size int `json:"size" v:"min:1#每页大小必须大于0"`
}

// TaskListRes 任务列表响应
type TaskListRes struct {
	Total    int             `json:"total"`     // 总数量
	Page     int             `json:"page"`      // 当前页码
	PageSize int             `json:"page_size"` // 每页数量
	Pages    int             `json:"pages"`     // 总页数
	List     []TaskDetailRes `json:"list"`      // 任务列表
}

// TaskDetailRes 任务详情响应
type TaskDetailRes struct {
	Id                   int64     `json:"id"`
	Name                 string    `json:"name"`
	HandlerName          string    `json:"handler_name"`
	CronExpression       string    `json:"cron_expression"`
	Timeout              int       `json:"timeout"`
	Status               int       `json:"status"`
	DependencyTaskName   *string   `json:"dependency_task_name"`
	DependencyTimeWindow int       `json:"dependency_time_window"`
	Params               *string   `json:"params"`
	Description          *string   `json:"description"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// TaskRunReq 立即执行任务请求
type TaskRunReq struct {
	Id int64 `json:"id" v:"required#任务ID不能为空"`
}

// TaskStatusReq 任务状态操作请求
type TaskStatusReq struct {
	Id int64 `json:"id" v:"required#任务ID不能为空"`
}

// ExecutionLogListReq 执行日志列表请求
type ExecutionLogListReq struct {
	Page     int    `json:"page" v:"min:1#页码必须大于0"`
	Size     int    `json:"size" v:"min:1#每页大小必须大于0"`
	TaskName string `json:"task_name"`
}

// ExecutionLogListRes 执行日志列表响应
type ExecutionLogListRes struct {
	Total    int            `json:"total"`     // 总数量
	Page     int            `json:"page"`      // 当前页码
	PageSize int            `json:"page_size"` // 每页数量
	Pages    int            `json:"pages"`     // 总页数
	List     []ExecutionLog `json:"list"`      // 日志列表
}

// ExecutionLog 执行日志项
type ExecutionLog struct {
	Id        int64      `json:"id"`
	ExecId    string     `json:"exec_id"`
	TaskId    int64      `json:"task_id"`
	TaskName  string     `json:"task_name"`
	Status    int        `json:"status"`
	StartTime time.Time  `json:"start_time"`
	EndTime   *time.Time `json:"end_time"`
	Duration  int64      `json:"duration"`
	Attempt   int        `json:"attempt"`
	ErrorMsg  string     `json:"error_msg"`
	CreatedAt time.Time  `json:"created_at"`
}

// LogCleanReq 日志清理请求
type LogCleanReq struct {
	Days int `json:"days" v:"min:1#天数必须大于0"`
}

// LogCleanRes 日志清理响应
type LogCleanRes struct {
	DeletedCount int64 `json:"deleted_count"`
}
