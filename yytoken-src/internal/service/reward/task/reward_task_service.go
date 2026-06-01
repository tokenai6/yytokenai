package task

import (
	"context"
	"time"

	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/repository/reward"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

// IRewardTaskService 奖励任务批次服务接口
type IRewardTaskService interface {
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

	// CheckPreviousBatchCompleted 检查是否存在未完成批次
	// 返回：true 表示存在未完成批次（应优先续跑该批次）；false 表示可执行新批次
	CheckPreviousBatchCompleted(ctx context.Context) (bool, error)

	// GetLatestIncompleteRecordTime 获取最近一个未完成批次的 record_time
	GetLatestIncompleteRecordTime(ctx context.Context) (*time.Time, error)

	// ResetTaskStatus 重置任务状态为 pending（用于强制重新执行）
	ResetTaskStatus(ctx context.Context, recordTime time.Time, taskName string) error

	// InitializeSingleTask 初始化单个任务（用于只执行特定任务的场景）
	InitializeSingleTask(ctx context.Context, recordTime time.Time, taskName string, taskSequence int) error

	// GetLatestIncompleteRecordTimeByTask 获取指定任务的最近一个未完成批次的 record_time
	GetLatestIncompleteRecordTimeByTask(ctx context.Context, taskName string) (*time.Time, error)
}

// rewardTaskService 奖励任务批次服务实现
type rewardTaskService struct {
	batchRepo reward.IRewardTaskBatchRepository
}

// NewRewardTaskService 创建奖励任务批次服务实例
func NewRewardTaskService() IRewardTaskService {
	return &rewardTaskService{
		batchRepo: reward.NewRewardTaskBatchRepository(),
	}
}

// InitializeBatch 初始化批次记录
func (s *rewardTaskService) InitializeBatch(ctx context.Context, recordTime time.Time) error {
	// 检查当前批次是否已初始化
	initialized, err := s.IsCurrentBatchInitialized(ctx, recordTime)
	if err != nil {
		return gerror.Wrap(err, "检查批次初始化状态失败")
	}

	if initialized {
		g.Log().Infof(ctx, "[RewardTaskService] 批次已存在，时间: %s", recordTime.Format("2006-01-02 15:04:05"))
		return nil
	}

	// 如果未初始化，进行初始化
	if err := s.batchRepo.InitializeBatch(ctx, recordTime); err != nil {
		return gerror.Wrap(err, "初始化批次失败")
	}

	g.Log().Infof(ctx, "[RewardTaskService] 批次已初始化，时间: %s", recordTime.Format("2006-01-02 15:04:05"))
	return nil
}

// GetLastCompletedBatch 获取最后一个完成的批次
func (s *rewardTaskService) GetLastCompletedBatch(ctx context.Context) (*rewardEntity.RewardTaskBatchEntity, error) {
	return s.batchRepo.GetLastCompletedBatch(ctx)
}

// IsCurrentBatchInitialized 检查当前批次是否已初始化
func (s *rewardTaskService) IsCurrentBatchInitialized(ctx context.Context, recordTime time.Time) (bool, error) {
	return s.batchRepo.IsCurrentBatchInitialized(ctx, recordTime)
}

// IsTaskCompleted 检查任务是否已完成
func (s *rewardTaskService) IsTaskCompleted(ctx context.Context, recordTime time.Time, taskName string) (bool, error) {
	return s.batchRepo.IsTaskCompleted(ctx, recordTime, taskName)
}

// UpdateTaskStatus 更新任务状态
func (s *rewardTaskService) UpdateTaskStatus(ctx context.Context, recordTime time.Time, taskName string, status string, errorMessage *string) error {
	return s.batchRepo.UpdateTaskStatus(ctx, recordTime, taskName, status, errorMessage)
}

// CheckPreviousBatchCompleted 检查是否存在未完成的批次
// 返回值：true 表示存在未完成批次（应优先续跑该批次）；false 表示不存在未完成批次
func (s *rewardTaskService) CheckPreviousBatchCompleted(ctx context.Context) (bool, error) {
	incRT, err := s.batchRepo.GetLatestIncompleteRecordTime(ctx)
	if err != nil {
		// 兼容性处理：部分驱动在无数据时可能返回 no rows 错误
		if err.Error() == "sql: no rows in result set" {
			g.Log().Info(ctx, "[RewardTaskService] 未发现未完成批次（no rows）")
			return false, nil
		}
		g.Log().Errorf(ctx, "[RewardTaskService] 查询未完成批次失败: %v", err)
		return false, err
	}
	if incRT != nil && !incRT.IsZero() {
		g.Log().Warningf(ctx, "[RewardTaskService] 检测到未完成批次，record_time=%s", incRT.Format("2006-01-02 15:04:05"))
		return true, nil
	}
	g.Log().Info(ctx, "[RewardTaskService] 未发现未完成批次，可执行新的批次")
	return false, nil
}

// GetLatestIncompleteRecordTime 获取最近一个未完成批次的 record_time
func (s *rewardTaskService) GetLatestIncompleteRecordTime(ctx context.Context) (*time.Time, error) {
	return s.batchRepo.GetLatestIncompleteRecordTime(ctx)
}

// ResetTaskStatus 重置任务状态为 pending（用于强制重新执行）
func (s *rewardTaskService) ResetTaskStatus(ctx context.Context, recordTime time.Time, taskName string) error {
	return s.batchRepo.ResetTaskStatus(ctx, recordTime, taskName)
}

// InitializeSingleTask 初始化单个任务（用于只执行特定任务的场景）
func (s *rewardTaskService) InitializeSingleTask(ctx context.Context, recordTime time.Time, taskName string, taskSequence int) error {
	return s.batchRepo.InitializeSingleTask(ctx, recordTime, taskName, taskSequence)
}

// GetLatestIncompleteRecordTimeByTask 获取指定任务的最近一个未完成批次的 record_time
func (s *rewardTaskService) GetLatestIncompleteRecordTimeByTask(ctx context.Context, taskName string) (*time.Time, error) {
	return s.batchRepo.GetLatestIncompleteRecordTimeByTask(ctx, taskName)
}
