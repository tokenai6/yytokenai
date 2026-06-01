package scheduler

import (
	schedulerEntity "XWFrame/internal/entity/scheduler"
	"XWFrame/internal/frame/db"
	"context"

	"github.com/gogf/gf/v2/database/gdb"
)

// TaskDao 定时任务DAO
type TaskDao struct{}

// NewTaskDao 创建DAO实例
func NewTaskDao() *TaskDao {
	return &TaskDao{}
}

// Insert 插入任务
func (d *TaskDao) Insert(ctx context.Context, task *schedulerEntity.ScheduledTask) error {
	_, err := db.GetDB().Ctx(ctx).Model(&schedulerEntity.ScheduledTask{}).FieldsEx("id", "created_at", "updated_at").Insert(task)
	return err
}

// Update 更新任务
func (d *TaskDao) Update(ctx context.Context, task *schedulerEntity.ScheduledTask) error {
	_, err := db.GetDB().Ctx(ctx).Model(&schedulerEntity.ScheduledTask{}).FieldsEx("id", "created_at").Where("id", task.Id).Update(task)
	return err
}

// Delete 删除任务
func (d *TaskDao) Delete(ctx context.Context, id int64) error {
	_, err := db.GetDB().Ctx(ctx).Model(&schedulerEntity.ScheduledTask{}).Where("id", id).Delete()
	return err
}

// GetById 根据ID获取任务
func (d *TaskDao) GetById(ctx context.Context, id int64) (*schedulerEntity.ScheduledTask, error) {
	var task schedulerEntity.ScheduledTask
	err := db.GetDB().Ctx(ctx).Model(&schedulerEntity.ScheduledTask{}).Where("id", id).Scan(&task)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// GetByName 根据名称获取任务
func (d *TaskDao) GetByName(ctx context.Context, name string) (*schedulerEntity.ScheduledTask, error) {
	var task schedulerEntity.ScheduledTask
	err := db.GetDB().Ctx(ctx).Model(&schedulerEntity.ScheduledTask{}).Where("name", name).Scan(&task)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// GetEnabledTasks 获取所有启用的任务
func (d *TaskDao) GetEnabledTasks(ctx context.Context) ([]*schedulerEntity.ScheduledTask, error) {
	var tasks []*schedulerEntity.ScheduledTask
	err := db.GetDB().Ctx(ctx).Model(&schedulerEntity.ScheduledTask{}).Where("status", 1).Scan(&tasks)
	return tasks, err
}

// GetAllTasks 获取所有任务（包括启用和禁用的）
func (d *TaskDao) GetAllTasks(ctx context.Context) ([]*schedulerEntity.ScheduledTask, error) {
	var tasks []*schedulerEntity.ScheduledTask
	err := db.GetDB().Ctx(ctx).Model(&schedulerEntity.ScheduledTask{}).Scan(&tasks)
	return tasks, err
}

// Query 查询任务列表
func (d *TaskDao) Query(ctx context.Context, query *gdb.Model) ([]*schedulerEntity.ScheduledTask, error) {
	var tasks []*schedulerEntity.ScheduledTask
	err := query.Scan(&tasks)
	return tasks, err
}

// Count 统计任务数量
func (d *TaskDao) Count(ctx context.Context, query *gdb.Model) (int, error) {
	count, err := query.Count()
	return count, err
}
