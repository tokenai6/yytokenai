package reward

import (
	"context"
	"time"

	rewardDao "XWFrame/internal/dao/reward"
	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/consts"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

// IRewardTaskBatchRepository 奖励任务批次仓储接口
type IRewardTaskBatchRepository interface {
	// InitializeBatch 初始化批次记录
	InitializeBatch(ctx context.Context, recordTime time.Time) error

	// GetLastCompletedBatch 获取最后一个完成的批次
	GetLastCompletedBatch(ctx context.Context) (*rewardEntity.RewardTaskBatchEntity, error)

	// IsCurrentBatchInitialized 检查当前批次是否已初始化
	IsCurrentBatchInitialized(ctx context.Context, recordTime time.Time) (bool, error)

	// IsTaskCompleted 检查任务是否已完成
	IsTaskCompleted(ctx context.Context, recordTime time.Time, taskName string) (bool, error)

	// UpdateTaskStatus 更新任务状态
	UpdateTaskStatus(ctx context.Context, recordTime time.Time, taskName string, status string, errorMessage *string) error

	// GetLatestIncompleteRecordTime 获取最近一个未完成批次的 record_time
	GetLatestIncompleteRecordTime(ctx context.Context) (*time.Time, error)

	// GetLatestRecordTime 获取最新的 record_time（不管是否完成）
	GetLatestRecordTime(ctx context.Context) (*time.Time, error)

	// CheckBatchAllCompleted 检查指定批次的所有任务是否都已完成
	CheckBatchAllCompleted(ctx context.Context, recordTime time.Time) (bool, error)

	// ResetTaskStatus 重置任务状态为 pending（用于强制重新执行）
	ResetTaskStatus(ctx context.Context, recordTime time.Time, taskName string) error

	// InitializeSingleTask 初始化单个任务（用于只执行特定任务的场景）
	InitializeSingleTask(ctx context.Context, recordTime time.Time, taskName string, taskSequence int) error

	// GetLatestIncompleteRecordTimeByTask 获取指定任务的最近一个未完成批次的 record_time
	GetLatestIncompleteRecordTimeByTask(ctx context.Context, taskName string) (*time.Time, error)
}

// rewardTaskBatchRepository 奖励任务批次仓储实现
type rewardTaskBatchRepository struct {
	batchDao rewardDao.IRewardTaskBatchDao
}

// NewRewardTaskBatchRepository 创建奖励任务批次仓储实例
func NewRewardTaskBatchRepository() IRewardTaskBatchRepository {
	return &rewardTaskBatchRepository{
		batchDao: rewardDao.NewRewardTaskBatchDao(),
	}
}

// InitializeBatch 初始化批次记录 - 创建9个pending任务记录
// 参数 recordTime 会被插入到 record_time 字段
// batch_date 从 record_time 提取日期部分手动设置
func (r *rewardTaskBatchRepository) InitializeBatch(ctx context.Context, recordTime time.Time) error {
	tasks := []string{
		consts.RewardTaskPerformanceCalculation,
		consts.RewardTaskStaticReward,
		consts.RewardTaskCommunityReward,
		consts.RewardTaskReferralReward,
		consts.RewardTaskWeightedReward,
		consts.RewardTaskNodeDividend,
		consts.RewardTaskFounderFeeDividend,
		consts.RewardTaskServiceCenterReward, // 服务中心奖励
		consts.RewardTaskSettlementSummary,
		consts.RewardTaskUserExpireCheck,
	}

	// 从 record_time 提取日期部分作为 batch_date
	batchDate := time.Date(recordTime.Year(), recordTime.Month(), recordTime.Day(), 0, 0, 0, 0, recordTime.Location())

	batches := make([]*rewardEntity.RewardTaskBatchEntity, len(tasks))
	for i, taskName := range tasks {
		batches[i] = &rewardEntity.RewardTaskBatchEntity{
			RecordTime:   recordTime,
			BatchDate:    batchDate,
			TaskSequence: i + 1,
			TaskName:     taskName,
			Status:       "pending",
		}
	}

	err := r.batchDao.BatchCreate(ctx, nil, batches)
	if err != nil {
		return gerror.Wrap(err, "初始化批次失败")
	}

	g.Log().Infof(ctx, "[奖励批次] 初始化批次: record_time=%s, 任务数=%d", recordTime.Format("2006-01-02 15:04:05"), len(tasks))
	return nil
}

// GetLastCompletedBatch 获取最后一个完成的批次
func (r *rewardTaskBatchRepository) GetLastCompletedBatch(ctx context.Context) (*rewardEntity.RewardTaskBatchEntity, error) {
	batch, err := r.batchDao.GetLastCompletedBatch(ctx)
	if err != nil {
		// 不包装"no rows"错误，让调用方直接处理
		return nil, err
	}
	return batch, nil
}

// IsCurrentBatchInitialized 检查当前批次是否已初始化
func (r *rewardTaskBatchRepository) IsCurrentBatchInitialized(ctx context.Context, recordTime time.Time) (bool, error) {
	count, err := r.batchDao.CountByRecordTime(ctx, recordTime)
	if err != nil {
		return false, gerror.Wrap(err, "检查批次初始化状态失败")
	}
	return count > 0, nil
}

// IsTaskCompleted 检查任务是否已完成
func (r *rewardTaskBatchRepository) IsTaskCompleted(ctx context.Context, recordTime time.Time, taskName string) (bool, error) {
	completed, err := r.batchDao.IsTaskCompleted(ctx, recordTime, taskName)
	if err != nil {
		return false, gerror.Wrap(err, "检查任务完成状态失败")
	}
	return completed, nil
}

// UpdateTaskStatus 更新任务状态
func (r *rewardTaskBatchRepository) UpdateTaskStatus(ctx context.Context, recordTime time.Time, taskName string, status string, errorMessage *string) error {
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if err := r.batchDao.UpdateStatus(ctx, tx, recordTime, taskName, status, errorMessage); err != nil {
			return gerror.Wrap(err, "更新任务状态失败")
		}

		if errorMessage != nil {
			g.Log().Infof(ctx, "[奖励批次] 任务%s: %s (错误: %s)", taskName, status, *errorMessage)
		} else {
			g.Log().Infof(ctx, "[奖励批次] 任务%s: %s", taskName, status)
		}
		return nil
	})
}

// GetLatestIncompleteRecordTime 获取最近一个未完成批次的 record_time
func (r *rewardTaskBatchRepository) GetLatestIncompleteRecordTime(ctx context.Context) (*time.Time, error) {
	return r.batchDao.GetLatestIncompleteRecordTime(ctx)
}

// GetLatestRecordTime 获取最新的 record_time（不管是否完成）
func (r *rewardTaskBatchRepository) GetLatestRecordTime(ctx context.Context) (*time.Time, error) {
	return r.batchDao.GetLatestRecordTime(ctx)
}

// CheckBatchAllCompleted 检查指定批次的所有任务是否都已完成
func (r *rewardTaskBatchRepository) CheckBatchAllCompleted(ctx context.Context, recordTime time.Time) (bool, error) {
	return r.batchDao.CheckBatchAllCompleted(ctx, recordTime)
}

// ResetTaskStatus 重置任务状态为 pending（用于强制重新执行）
func (r *rewardTaskBatchRepository) ResetTaskStatus(ctx context.Context, recordTime time.Time, taskName string) error {
	return r.UpdateTaskStatus(ctx, recordTime, taskName, "pending", nil)
}

// InitializeSingleTask 初始化单个任务（用于只执行特定任务的场景）
func (r *rewardTaskBatchRepository) InitializeSingleTask(ctx context.Context, recordTime time.Time, taskName string, taskSequence int) error {
	// 检查任务是否已存在
	existing, err := r.batchDao.GetByRecordTimeAndTask(ctx, recordTime, taskName)
	if err != nil {
		return gerror.Wrap(err, "检查任务是否存在失败")
	}
	if existing != nil {
		// 任务已存在，无需初始化
		return nil
	}

	// 从 record_time 提取日期部分作为 batch_date
	batchDate := time.Date(recordTime.Year(), recordTime.Month(), recordTime.Day(), 0, 0, 0, 0, recordTime.Location())

	batch := &rewardEntity.RewardTaskBatchEntity{
		RecordTime:   recordTime,
		BatchDate:    batchDate,
		TaskSequence: taskSequence,
		TaskName:     taskName,
		Status:       "pending",
	}

	err = r.batchDao.Create(ctx, nil, batch)
	if err != nil {
		return gerror.Wrap(err, "初始化单个任务失败")
	}

	g.Log().Infof(ctx, "[奖励批次] 初始化单个任务: record_time=%s, task_name=%s", recordTime.Format("2006-01-02 15:04:05"), taskName)
	return nil
}

// GetLatestIncompleteRecordTimeByTask 获取指定任务的最近一个未完成批次的 record_time
func (r *rewardTaskBatchRepository) GetLatestIncompleteRecordTimeByTask(ctx context.Context, taskName string) (*time.Time, error) {
	return r.batchDao.GetLatestIncompleteRecordTimeByTask(ctx, taskName)
}
