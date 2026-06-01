package reward

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// IRewardTaskBatchDao 奖励任务批次记录数据访问接口
type IRewardTaskBatchDao interface {
	// Create 创建批次记录
	Create(ctx context.Context, tx gdb.TX, batch *reward.RewardTaskBatchEntity) error

	// BatchCreate 批量创建批次记录
	BatchCreate(ctx context.Context, tx gdb.TX, batches []*reward.RewardTaskBatchEntity) error

	// GetByRecordTime 根据记录时间查询所有批次
	GetByRecordTime(ctx context.Context, recordTime time.Time) ([]*reward.RewardTaskBatchEntity, error)

	// GetByRecordTimeAndTask 根据记录时间和任务名查询
	GetByRecordTimeAndTask(ctx context.Context, recordTime time.Time, taskName string) (*reward.RewardTaskBatchEntity, error)

	// GetLastCompletedBatch 查询最后一个完成的批次
	GetLastCompletedBatch(ctx context.Context) (*reward.RewardTaskBatchEntity, error)

	// UpdateStatus 更新任务状态
	UpdateStatus(ctx context.Context, tx gdb.TX, recordTime time.Time, taskName string, status string, errorMessage *string) error

	// CountByRecordTime 统计指定记录时间的批次数
	CountByRecordTime(ctx context.Context, recordTime time.Time) (int, error)

	// GetFailedTasks 查询失败的任务
	GetFailedTasks(ctx context.Context, recordTime time.Time) ([]*reward.RewardTaskBatchEntity, error)

	// IsTaskCompleted 检查任务是否已完成
	IsTaskCompleted(ctx context.Context, recordTime time.Time, taskName string) (bool, error)

	// GetLatestIncompleteRecordTime 查询最近一个未完成的批次的 record_time
	// 判定标准：该 record_time 下存在 status<>"completed" 的任务
	GetLatestIncompleteRecordTime(ctx context.Context) (*time.Time, error)

	// GetLatestIncompleteRecordTimeByTask 查询指定任务的最近一个未完成批次的 record_time
	GetLatestIncompleteRecordTimeByTask(ctx context.Context, taskName string) (*time.Time, error)

	// GetLatestRecordTime 查询最新的 record_time（不管是否完成）
	GetLatestRecordTime(ctx context.Context) (*time.Time, error)

	// CheckBatchAllCompleted 检查指定批次的所有任务是否都已完成
	CheckBatchAllCompleted(ctx context.Context, recordTime time.Time) (bool, error)
}

// rewardTaskBatchDao 奖励任务批次记录数据访问实现
type rewardTaskBatchDao struct {
	db gdb.DB
}

// NewRewardTaskBatchDao 创建奖励任务批次记录数据访问实例
func NewRewardTaskBatchDao() IRewardTaskBatchDao {
	return &rewardTaskBatchDao{
		db: db.GetDB(),
	}
}

// Create 创建批次记录
// 注意：batch_date 需要手动设置，从 record_time 提取日期部分
func (d *rewardTaskBatchDao) Create(ctx context.Context, tx gdb.TX, batch *reward.RewardTaskBatchEntity) error {
	model := d.db.Model("reward_task_batch")
	if tx != nil {
		model = tx.Model("reward_task_batch")
	}

	result, err := model.Ctx(ctx).
		FieldsEx("id", "created_at", "updated_at").
		Data(batch).
		InsertAndGetId()
	if err != nil {
		return err
	}
	batch.Id = result
	return nil
}

// BatchCreate 批量创建批次记录
// 注意：batch_date 需要手动设置，从 record_time 提取日期部分
func (d *rewardTaskBatchDao) BatchCreate(ctx context.Context, tx gdb.TX, batches []*reward.RewardTaskBatchEntity) error {
	if len(batches) == 0 {
		return nil
	}

	model := d.db.Model("reward_task_batch")
	if tx != nil {
		model = tx.Model("reward_task_batch")
	}

	_, err := model.Ctx(ctx).
		FieldsEx("id", "created_at", "updated_at").
		Data(batches).
		Insert()
	return err
}

// GetByRecordTime 根据记录时间查询所有批次
func (d *rewardTaskBatchDao) GetByRecordTime(ctx context.Context, recordTime time.Time) ([]*reward.RewardTaskBatchEntity, error) {
	var batches []*reward.RewardTaskBatchEntity
	err := d.db.Model("reward_task_batch").Ctx(ctx).
		Where("record_time", recordTime).
		Order("task_sequence ASC").
		Scan(&batches)
	if err != nil {
		return nil, err
	}
	return batches, nil
}

// GetByRecordTimeAndTask 根据记录时间和任务名查询
func (d *rewardTaskBatchDao) GetByRecordTimeAndTask(ctx context.Context, recordTime time.Time, taskName string) (*reward.RewardTaskBatchEntity, error) {
	var batch reward.RewardTaskBatchEntity
	err := d.db.Model("reward_task_batch").Ctx(ctx).
		Where("record_time", recordTime).
		Where("task_name", taskName).
		Scan(&batch)
	if err != nil {
		// 如果查询不到记录（sql: no rows in result set），返回 nil, nil（记录不存在，这是正常情况）
		if errors.Is(err, sql.ErrNoRows) || err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	if batch.Id == 0 {
		return nil, nil
	}
	return &batch, nil
}

// GetLastCompletedBatch 查询最后一个完成的批次
func (d *rewardTaskBatchDao) GetLastCompletedBatch(ctx context.Context) (*reward.RewardTaskBatchEntity, error) {
	var batch reward.RewardTaskBatchEntity
	err := d.db.Model("reward_task_batch").Ctx(ctx).
		Where("status", "completed").
		Where("task_name", consts.RewardTaskUserExpireCheck). // 最后一个任务完成表示整个批次完成
		Order("record_time DESC").
		Limit(1).
		Scan(&batch)
	if err != nil {
		// 如果查询不到记录（sql: no rows in result set），返回 nil, nil（没有已完成批次，这是正常情况）
		if errors.Is(err, sql.ErrNoRows) || err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	if batch.Id == 0 {
		return nil, nil
	}
	return &batch, nil
}

// UpdateStatus 更新任务状态
func (d *rewardTaskBatchDao) UpdateStatus(ctx context.Context, tx gdb.TX, recordTime time.Time, taskName string, status string, errorMessage *string) error {
	// 准备更新数据
	data := map[string]interface{}{
		"status": status,
	}
	if errorMessage != nil {
		data["error_message"] = *errorMessage
	}

	// 1) 行级锁：使用独立的Model进行 SELECT ... FOR UPDATE
	lockModel := d.db.Model("reward_task_batch")
	if tx != nil {
		lockModel = tx.Model("reward_task_batch")
	}
	if _, err := lockModel.Ctx(ctx).
		Where("record_time", recordTime).
		Where("task_name", taskName).
		LockUpdate().
		One(); err != nil {
		return err
	}

	// 2) 更新：使用新的Model，避免 FOR UPDATE/Where 状态遗留
	updateModel := d.db.Model("reward_task_batch")
	if tx != nil {
		updateModel = tx.Model("reward_task_batch")
	}
	_, err := updateModel.Ctx(ctx).
		Where("record_time", recordTime).
		Where("task_name", taskName).
		Data(data).
		Update()
	return err
}

// CountByRecordTime 统计指定记录时间的批次数
func (d *rewardTaskBatchDao) CountByRecordTime(ctx context.Context, recordTime time.Time) (int, error) {
	count, err := d.db.Model("reward_task_batch").Ctx(ctx).
		Where("record_time", recordTime).
		Count()
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// GetFailedTasks 查询失败的任务
func (d *rewardTaskBatchDao) GetFailedTasks(ctx context.Context, recordTime time.Time) ([]*reward.RewardTaskBatchEntity, error) {
	var batches []*reward.RewardTaskBatchEntity
	err := d.db.Model("reward_task_batch").Ctx(ctx).
		Where("record_time", recordTime).
		Where("status", "failed").
		Order("task_sequence ASC").
		Scan(&batches)
	if err != nil {
		return nil, err
	}
	return batches, nil
}

// IsTaskCompleted 检查任务是否已完成
func (d *rewardTaskBatchDao) IsTaskCompleted(ctx context.Context, recordTime time.Time, taskName string) (bool, error) {
	count, err := d.db.Model("reward_task_batch").Ctx(ctx).
		Where("record_time", recordTime).
		Where("task_name", taskName).
		Where("status", "completed").
		Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetLatestIncompleteRecordTime 查询最近一个未完成的批次的 record_time
// 逻辑：按 record_time 分组，选择存在未完成任务（pending/failed）的批次，取最新一条
func (d *rewardTaskBatchDao) GetLatestIncompleteRecordTime(ctx context.Context) (*time.Time, error) {
	var result struct {
		RecordTime time.Time `json:"record_time"`
	}
	err := d.db.Model("reward_task_batch").Ctx(ctx).
		Fields("record_time").
		Where("status <> ?", "completed").
		Group("record_time").
		Order("record_time DESC").
		Limit(1).
		Scan(&result)
	if err != nil {
		// 如果查询不到记录（sql: no rows in result set），返回 nil, nil（没有未完成批次，这是正常情况）
		if errors.Is(err, sql.ErrNoRows) || err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	if result.RecordTime.IsZero() {
		return nil, nil
	}
	return &result.RecordTime, nil
}

// GetLatestRecordTime 查询最新的 record_time（不管是否完成）
func (d *rewardTaskBatchDao) GetLatestRecordTime(ctx context.Context) (*time.Time, error) {
	var result struct {
		RecordTime time.Time `json:"record_time"`
	}
	err := d.db.Model("reward_task_batch").Ctx(ctx).
		Fields("record_time").
		Order("record_time DESC").
		Limit(1).
		Scan(&result)
	if err != nil {
		// 如果查询不到记录（sql: no rows in result set），返回 nil, nil（没有批次记录，这是正常情况）
		if errors.Is(err, sql.ErrNoRows) || err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	if result.RecordTime.IsZero() {
		return nil, nil
	}
	return &result.RecordTime, nil
}

// GetLatestIncompleteRecordTimeByTask 查询指定任务的最近一个未完成批次的 record_time
func (d *rewardTaskBatchDao) GetLatestIncompleteRecordTimeByTask(ctx context.Context, taskName string) (*time.Time, error) {
	var result struct {
		RecordTime time.Time `json:"record_time"`
	}
	err := d.db.Model("reward_task_batch").Ctx(ctx).
		Fields("record_time").
		Where("task_name", taskName).
		Where("status <> ?", "completed").
		Order("record_time DESC").
		Limit(1).
		Scan(&result)
	if err != nil {
		// 如果查询不到记录（sql: no rows in result set），返回 nil, nil（没有未完成批次，这是正常情况）
		if errors.Is(err, sql.ErrNoRows) || err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	if result.RecordTime.IsZero() {
		return nil, nil
	}
	return &result.RecordTime, nil
}

// CheckBatchAllCompleted 检查指定批次的所有任务是否都已完成
func (d *rewardTaskBatchDao) CheckBatchAllCompleted(ctx context.Context, recordTime time.Time) (bool, error) {
	// 查询该批次的所有任务
	batches, err := d.GetByRecordTime(ctx, recordTime)
	if err != nil {
		return false, err
	}

	// 如果没有任务记录，认为已完成（可能是首次执行，还没有批次记录）
	if len(batches) == 0 {
		return true, nil
	}

	// 检查是否所有任务都是 completed 状态
	for _, batch := range batches {
		if batch.Status != "completed" {
			return false, nil
		}
	}

	return true, nil
}
