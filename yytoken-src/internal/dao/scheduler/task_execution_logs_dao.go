package scheduler

import (
	"context"
	"fmt"
	"time"

	schedulerEntity "XWFrame/internal/entity/scheduler"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// TaskExecutionLogsDao 任务执行日志DAO
type TaskExecutionLogsDao struct{}

// NewTaskExecutionLogsDao 创建DAO实例
func NewTaskExecutionLogsDao() *TaskExecutionLogsDao {
	return &TaskExecutionLogsDao{}
}

// Insert 插入执行日志
func (d *TaskExecutionLogsDao) Insert(ctx context.Context, log *schedulerEntity.TaskExecutionLogs) error {
	// 显式指定字段，确保status作为字符串写入（数据库字段是varchar）
	_, err := db.GetDB().Ctx(ctx).Model(&schedulerEntity.TaskExecutionLogs{}).Insert(map[string]interface{}{
		"exec_id":    log.ExecId,
		"task_id":    log.TaskId,
		"task_name":  log.TaskName,
		"status":     fmt.Sprintf("%d", log.Status),
		"start_time": log.StartTime,
		"end_time":   log.EndTime,
		"duration":   log.Duration,
		"attempt":    log.Attempt,
		"error_msg":  log.ErrorMsg,
	})
	return err
}

// Update 更新执行日志
func (d *TaskExecutionLogsDao) Update(ctx context.Context, log *schedulerEntity.TaskExecutionLogs) error {
	_, err := db.GetDB().Ctx(ctx).Model(&schedulerEntity.TaskExecutionLogs{}).
		Where("exec_id", log.ExecId).
		Update(map[string]interface{}{
			"status":    fmt.Sprintf("%d", log.Status), // 转换为字符串，数据库字段是varchar
			"end_time":  log.EndTime,
			"duration":  log.Duration,
			"attempt":   log.Attempt,
			"error_msg": log.ErrorMsg,
		})
	return err
}

// Query 查询执行日志
func (d *TaskExecutionLogsDao) Query(ctx context.Context, query *gdb.Model) ([]*schedulerEntity.TaskExecutionLogs, error) {
	var logs []*schedulerEntity.TaskExecutionLogs
	err := query.Scan(&logs)
	return logs, err
}

// DeleteOldLogs 删除旧日志
func (d *TaskExecutionLogsDao) DeleteOldLogs(ctx context.Context, days int) (int64, error) {
	result, err := db.GetDB().Ctx(ctx).Model(&schedulerEntity.TaskExecutionLogs{}).
		Where(fmt.Sprintf("created_at < NOW() - INTERVAL '%d days'", days)).
		Delete()
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// HasSuccessInWindow 检查指定任务在时间窗口内是否有成功执行记录
func (d *TaskExecutionLogsDao) HasSuccessInWindow(ctx context.Context, taskName string, windowMinutes int) (bool, error) {
	count, err := db.GetDB().Ctx(ctx).Model(&schedulerEntity.TaskExecutionLogs{}).
		Where("task_name", taskName).
		Where("status", "2"). // 2 = 成功
		Where(fmt.Sprintf("end_time >= NOW() - INTERVAL '%d minutes'", windowMinutes)).
		Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// HasSuccessInDateRange 检查指定任务在时间范围内是否有成功执行记录
func (d *TaskExecutionLogsDao) HasSuccessInDateRange(ctx context.Context, taskName string, startTime, endTime time.Time) (bool, error) {
	count, err := db.GetDB().Ctx(ctx).Model(&schedulerEntity.TaskExecutionLogs{}).
		Where("task_name", taskName).
		Where("status", "2"). // 2 = 成功
		WhereBetween("end_time", startTime, endTime).
		Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
