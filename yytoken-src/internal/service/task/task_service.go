package task

import (
	schedulerEntity "XWFrame/internal/entity/scheduler"
	"XWFrame/internal/frame/model"
	"XWFrame/internal/repository"
	"XWFrame/internal/scheduler"
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
)

// ITaskService 任务服务接口
type ITaskService interface {
	// CreateTask 创建任务
	CreateTask(ctx context.Context, req *TaskCreateReq) error
	// UpdateTask 更新任务
	UpdateTask(ctx context.Context, req *TaskUpdateReq) error
	// DeleteTask 删除任务
	DeleteTask(ctx context.Context, id int64) error
	// GetTaskList 获取任务列表
	GetTaskList(ctx context.Context, req *TaskListReq) (*TaskListRes, error)
	// GetTaskDetail 获取任务详情
	GetTaskDetail(ctx context.Context, id int64) (*TaskDetailRes, error)
	// EnableTask 启用任务
	EnableTask(ctx context.Context, req *TaskStatusReq) error
	// DisableTask 禁用任务
	DisableTask(ctx context.Context, req *TaskStatusReq) error
	// RunTask 立即执行任务
	RunTask(ctx context.Context, req *TaskRunReq) error
	// GetExecutionLogs 获取执行日志
	GetExecutionLogs(ctx context.Context, req *ExecutionLogListReq) (*ExecutionLogListRes, error)
	// CleanLogs 清理日志
	CleanLogs(ctx context.Context, req *LogCleanReq) (*LogCleanRes, error)
}

// taskService 任务服务实现
type taskService struct {
	repository *repository.TaskRepository
}

// NewTaskService 创建任务服务实例
func NewTaskService() ITaskService {
	return &taskService{
		repository: repository.NewTaskRepository(),
	}
}

// CreateTask 创建任务
func (s *taskService) CreateTask(ctx context.Context, req *TaskCreateReq) error {
	// 检查任务名称是否已存在
	existingTask, err := s.repository.GetTaskByName(ctx, req.Name)
	if err == nil && existingTask != nil {
		return fmt.Errorf("任务名称已存在: %s", req.Name)
	}

	// 检查依赖任务是否存在
	if req.DependencyTaskName != nil && *req.DependencyTaskName != "" {
		dependTask, err := s.repository.GetTaskByName(ctx, *req.DependencyTaskName)
		if err != nil || dependTask == nil {
			return fmt.Errorf("依赖任务不存在: %s", *req.DependencyTaskName)
		}
	}

	// 创建任务实体
	task := &schedulerEntity.ScheduledTask{
		Name:                 req.Name,
		HandlerName:          req.HandlerName,
		CronExpression:       req.CronExpression,
		Timeout:              req.Timeout,
		Status:               req.Status,
		DependencyTaskName:   req.DependencyTaskName,
		DependencyTimeWindow: req.DependencyTimeWindow,
		Params:               req.Params,
		Description:          req.Description,
	}

	return s.repository.CreateTask(ctx, task)
}

// UpdateTask 更新任务
func (s *taskService) UpdateTask(ctx context.Context, req *TaskUpdateReq) error {
	// 检查任务是否存在
	task, err := s.repository.GetTaskById(ctx, req.Id)
	if err != nil || task == nil {
		return fmt.Errorf("任务不存在")
	}

	// 检查任务名称是否已被其他任务使用
	if req.Name != task.Name {
		existingTask, err := s.repository.GetTaskByName(ctx, req.Name)
		if err == nil && existingTask != nil {
			return fmt.Errorf("任务名称已存在: %s", req.Name)
		}
	}

	// 检查依赖任务是否存在
	if req.DependencyTaskName != nil && *req.DependencyTaskName != "" {
		dependTask, err := s.repository.GetTaskByName(ctx, *req.DependencyTaskName)
		if err != nil || dependTask == nil {
			return fmt.Errorf("依赖任务不存在: %s", *req.DependencyTaskName)
		}
		// 不能依赖自己
		if *req.DependencyTaskName == req.Name {
			return fmt.Errorf("任务不能依赖自己")
		}
	}

	// 记录旧的状态和 Cron 表达式
	oldStatus := task.Status
	oldCronExpression := task.CronExpression

	// 更新任务实体
	task.Name = req.Name
	task.HandlerName = req.HandlerName
	task.CronExpression = req.CronExpression
	task.Timeout = req.Timeout
	task.Status = req.Status
	task.DependencyTaskName = req.DependencyTaskName
	task.DependencyTimeWindow = req.DependencyTimeWindow
	task.Params = req.Params
	task.Description = req.Description

	// 更新数据库
	if err := s.repository.UpdateTask(ctx, task); err != nil {
		return err
	}

	// 如果状态或 Cron 表达式发生变化，需要同步更新调度器
	if oldStatus != task.Status || oldCronExpression != task.CronExpression {
		if task.Status == 1 {
			// 启用任务：重新调度
			if err := scheduler.EnableTask(ctx, task.Id); err != nil {
				g.Log().Errorf(ctx, "[TaskService] 启用任务失败: %v", err)
				return fmt.Errorf("更新调度器失败: %v", err)
			}
		} else if task.Status == 0 {
			// 禁用任务
			if err := scheduler.DisableTask(ctx, task.Id); err != nil {
				g.Log().Errorf(ctx, "[TaskService] 禁用任务失败: %v", err)
				return fmt.Errorf("更新调度器失败: %v", err)
			}
		}
	}

	return nil
}

// DeleteTask 删除任务
func (s *taskService) DeleteTask(ctx context.Context, id int64) error {
	// 检查任务是否存在
	task, err := s.repository.GetTaskById(ctx, id)
	if err != nil || task == nil {
		return fmt.Errorf("任务不存在")
	}

	return s.repository.DeleteTask(ctx, id)
}

// GetTaskList 获取任务列表
func (s *taskService) GetTaskList(ctx context.Context, req *TaskListReq) (*TaskListRes, error) {

	// 规范化分页参数，防止除零
	page := req.Page
	if page <= 0 {
		page = 1
	}
	size := req.Size
	if size <= 0 {
		size = 100
	}

	pageReq := &model.PageReq{
		Page:     page,
		PageSize: size,
	}
	repoRes, err := s.repository.GetTaskList(ctx, pageReq)
	if err != nil {
		return nil, err
	}

	// 转换实体为响应结构
	var taskList []TaskDetailRes
	for _, task := range repoRes.List {
		taskList = append(taskList, TaskDetailRes{
			Id:                   task.Id,
			Name:                 task.Name,
			HandlerName:          task.HandlerName,
			CronExpression:       task.CronExpression,
			Timeout:              task.Timeout,
			Status:               task.Status,
			DependencyTaskName:   task.DependencyTaskName,
			DependencyTimeWindow: task.DependencyTimeWindow,
			Params:               task.Params,
			Description:          task.Description,
			CreatedAt:            task.CreatedAt,
			UpdatedAt:            task.UpdatedAt,
		})
	}

	return &TaskListRes{
		Total:    repoRes.Total,
		Page:     repoRes.Page,
		PageSize: repoRes.PageSize,
		Pages:    repoRes.Pages,
		List:     taskList,
	}, nil
}

// GetTaskDetail 获取任务详情
func (s *taskService) GetTaskDetail(ctx context.Context, id int64) (*TaskDetailRes, error) {
	task, err := s.repository.GetTaskById(ctx, id)
	if err != nil || task == nil {
		return nil, fmt.Errorf("任务不存在")
	}

	return &TaskDetailRes{
		Id:                   task.Id,
		Name:                 task.Name,
		HandlerName:          task.HandlerName,
		CronExpression:       task.CronExpression,
		Timeout:              task.Timeout,
		Status:               task.Status,
		DependencyTaskName:   task.DependencyTaskName,
		DependencyTimeWindow: task.DependencyTimeWindow,
		Params:               task.Params,
		Description:          task.Description,
		CreatedAt:            task.CreatedAt,
		UpdatedAt:            task.UpdatedAt,
	}, nil
}

// EnableTask 启用任务
func (s *taskService) EnableTask(ctx context.Context, req *TaskStatusReq) error {
	task, err := s.repository.GetTaskById(ctx, req.Id)
	if err != nil || task == nil {
		return fmt.Errorf("任务不存在")
	}

	task.Status = 1
	return s.repository.UpdateTask(ctx, task)
}

// DisableTask 禁用任务
func (s *taskService) DisableTask(ctx context.Context, req *TaskStatusReq) error {
	task, err := s.repository.GetTaskById(ctx, req.Id)
	if err != nil || task == nil {
		return fmt.Errorf("任务不存在")
	}

	task.Status = 0
	return s.repository.UpdateTask(ctx, task)
}

// RunTask 立即执行任务
func (s *taskService) RunTask(ctx context.Context, req *TaskRunReq) error {
	task, err := s.repository.GetTaskById(ctx, req.Id)
	if err != nil || task == nil {
		return fmt.Errorf("任务不存在")
	}

	// 这里需要调用调度器来执行任务
	// 暂时返回成功，实际实现会在调度器中处理
	g.Log().Infof(ctx, "手动执行任务: %s", task.Name)
	return nil
}

// GetExecutionLogs 获取执行日志
func (s *taskService) GetExecutionLogs(ctx context.Context, req *ExecutionLogListReq) (*ExecutionLogListRes, error) {
	pageReq := &model.PageReq{
		Page:     req.Page,
		PageSize: req.Size,
	}
	logs, total, err := s.repository.GetExecutionLogs(ctx, pageReq, req.TaskName)
	if err != nil {
		return nil, err
	}

	// 规范化分页参数
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.Size
	if pageSize <= 0 {
		pageSize = 10
	}

	// 转换实体为响应结构
	var logList []ExecutionLog
	for _, log := range logs {
		logList = append(logList, ExecutionLog{
			Id:        log.Id,
			ExecId:    log.ExecId,
			TaskId:    log.TaskId,
			TaskName:  log.TaskName,
			Status:    log.Status,
			StartTime: log.StartTime,
			EndTime:   log.EndTime,
			Duration:  log.Duration,
			Attempt:   log.Attempt,
			ErrorMsg:  log.ErrorMsg,
			CreatedAt: log.CreatedAt,
		})
	}

	// 计算总页数
	pages := (total + pageSize - 1) / pageSize

	return &ExecutionLogListRes{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Pages:    pages,
		List:     logList,
	}, nil
}

// CleanLogs 清理日志
func (s *taskService) CleanLogs(ctx context.Context, req *LogCleanReq) (*LogCleanRes, error) {
	deletedCount, err := s.repository.CleanOldLogs(ctx, req.Days)
	if err != nil {
		return nil, err
	}

	return &LogCleanRes{
		DeletedCount: deletedCount,
	}, nil
}
