package repository

import (
	schedulerDao "XWFrame/internal/dao/scheduler"
	schedulerEntity "XWFrame/internal/entity/scheduler"
	"XWFrame/internal/frame/db"
	"XWFrame/internal/frame/model"
	"context"
	"time"
)

// PageRes 分页响应类型别名
type PageRes = model.PageRes

// TaskListRes 任务列表响应
type TaskListRes struct {
	model.PageRes
	List []*schedulerEntity.ScheduledTask `json:"list"`
}

// TaskRepository 任务仓储层
type TaskRepository struct {
	taskDao *schedulerDao.TaskDao
	logDao  *schedulerDao.TaskExecutionLogsDao
}

// NewTaskRepository 创建任务仓储实例
func NewTaskRepository() *TaskRepository {
	return &TaskRepository{
		taskDao: schedulerDao.NewTaskDao(),
		logDao:  schedulerDao.NewTaskExecutionLogsDao(),
	}
}

// CreateTask 创建任务
func (r *TaskRepository) CreateTask(ctx context.Context, task *schedulerEntity.ScheduledTask) error {
	return r.taskDao.Insert(ctx, task)
}

// UpdateTask 更新任务
func (r *TaskRepository) UpdateTask(ctx context.Context, task *schedulerEntity.ScheduledTask) error {
	return r.taskDao.Update(ctx, task)
}

// DeleteTask 删除任务
func (r *TaskRepository) DeleteTask(ctx context.Context, id int64) error {
	return r.taskDao.Delete(ctx, id)
}

// GetTaskById 根据ID获取任务
func (r *TaskRepository) GetTaskById(ctx context.Context, id int64) (*schedulerEntity.ScheduledTask, error) {
	return r.taskDao.GetById(ctx, id)
}

// GetTaskByName 根据名称获取任务
func (r *TaskRepository) GetTaskByName(ctx context.Context, name string) (*schedulerEntity.ScheduledTask, error) {
	return r.taskDao.GetByName(ctx, name)
}

// GetEnabledTasks 获取所有启用的任务
func (r *TaskRepository) GetEnabledTasks(ctx context.Context) ([]*schedulerEntity.ScheduledTask, error) {
	return r.taskDao.GetEnabledTasks(ctx)
}

// GetAllTasks 获取所有任务（包括启用和禁用的）
func (r *TaskRepository) GetAllTasks(ctx context.Context) ([]*schedulerEntity.ScheduledTask, error) {
	return r.taskDao.GetAllTasks(ctx)
}

// GetTaskList 分页查询任务列表
func (r *TaskRepository) GetTaskList(ctx context.Context, req *model.PageReq) (*TaskListRes, error) {
	// 构建查询模型
	dbQuery := db.GetDB().Model(&schedulerEntity.ScheduledTask{})

	// 兜底保护，防止除零和非法页码
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	// 获取总数
	total, err := dbQuery.Count()
	if err != nil {
		return nil, err
	}

	// 分页查询
	var tasks []*schedulerEntity.ScheduledTask
	err = dbQuery.Page(page, pageSize).Order("created_at DESC").Scan(&tasks)

	if err != nil {
		return nil, err
	}

	// 计算总页数
	pages := (total + pageSize - 1) / pageSize

	return &TaskListRes{
		PageRes: model.PageRes{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
			Pages:    pages,
		},
		List: tasks,
	}, nil
}

// CheckDependency 检查任务依赖
func (r *TaskRepository) CheckDependency(ctx context.Context, dependTaskName string, timeWindow int) (bool, error) {
	if dependTaskName == "" {
		return true, nil
	}
	return r.logDao.HasSuccessInWindow(ctx, dependTaskName, timeWindow)
}

// CreateExecutionLog 创建执行日志
func (r *TaskRepository) CreateExecutionLog(ctx context.Context, log *schedulerEntity.TaskExecutionLogs) error {
	return r.logDao.Insert(ctx, log)
}

// UpdateExecutionLog 更新执行日志
func (r *TaskRepository) UpdateExecutionLog(ctx context.Context, log *schedulerEntity.TaskExecutionLogs) error {
	return r.logDao.Update(ctx, log)
}

// ExecutionLogListRes 执行日志列表响应
type ExecutionLogListRes struct {
	model.PageRes
	List []*schedulerEntity.TaskExecutionLogs `json:"list"`
}

// GetExecutionLogs 分页查询执行日志
func (r *TaskRepository) GetExecutionLogs(ctx context.Context, req *model.PageReq, taskName string) ([]*schedulerEntity.TaskExecutionLogs, int, error) {
	// 构建查询 Model
	query := db.GetDB().Ctx(ctx).Model(&schedulerEntity.TaskExecutionLogs{})
	if taskName != "" {
		query = query.Where("task_name", taskName)
	}

	// 获取总数
	count, err := query.Count()
	if err != nil {
		return nil, 0, err
	}
	total := int(count)

	// 兜底保护，防止除零和非法页码
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	// 分页查询
	var logs []*schedulerEntity.TaskExecutionLogs
	err = query.Page(page, pageSize).Order("created_at DESC").Scan(&logs)
	if err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// CleanOldLogs 清理旧日志
func (r *TaskRepository) CleanOldLogs(ctx context.Context, days int) (int64, error) {
	return r.logDao.DeleteOldLogs(ctx, days)
}

// HasSuccessLogInDateRange 检查指定任务在时间范围内是否有成功执行记录
func (r *TaskRepository) HasSuccessLogInDateRange(ctx context.Context, taskName string, startTime, endTime time.Time) (bool, error) {
	return r.logDao.HasSuccessInDateRange(ctx, taskName, startTime, endTime)
}

// GetRunningExecutionLog 获取正在运行的执行记录
func (r *TaskRepository) GetRunningExecutionLog(ctx context.Context, taskId int64) (*schedulerEntity.TaskExecutionLogs, error) {
	var log schedulerEntity.TaskExecutionLogs
	err := db.GetDB().Ctx(ctx).Model(&schedulerEntity.TaskExecutionLogs{}).
		Where("task_id", taskId).
		Where("status", "1"). // 1表示运行中，数据库字段是varchar类型
		Order("start_time DESC").
		Scan(&log)

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil // 没有找到记录
		}
		return nil, err
	}
	return &log, nil
}
