package reward

import (
	"context"
	"time"

	rewardDao "XWFrame/internal/dao/reward"
	rewardEntity "XWFrame/internal/entity/reward"

	"github.com/gogf/gf/v2/database/gdb"
)

// IStakeStatisticsRepository 质押统计仓储接口（仅提供数据访问）
type IStakeStatisticsRepository interface {
	// GetByDate 获取指定日期的统计
	GetByDate(ctx context.Context, date time.Time) (*rewardEntity.DailyStakeStatisticsEntity, error)

	// GetUserStatsByDate 获取指定日期的用户统计
	GetUserStatsByDate(ctx context.Context, date time.Time) ([]*rewardEntity.UserDailyStakeStatisticsEntity, error)

	// CalculateGlobalStats 统计全网数据
	CalculateGlobalStats(ctx context.Context, date time.Time) (*rewardEntity.DailyStakeStatisticsEntity, error)

	// CalculateUserStats 统计每个用户数据
	CalculateUserStats(ctx context.Context, date time.Time) ([]*rewardEntity.UserDailyStakeStatisticsEntity, error)

	// CreateDailyStats 创建全网统计
	CreateDailyStats(ctx context.Context, tx gdb.TX, stats *rewardEntity.DailyStakeStatisticsEntity) error

	// BatchCreateUserStats 批量创建用户统计
	BatchCreateUserStats(ctx context.Context, tx gdb.TX, stats []*rewardEntity.UserDailyStakeStatisticsEntity) error
}

// stakeStatisticsRepository 质押统计仓储实现
type stakeStatisticsRepository struct {
	dailyStatsDao rewardDao.IDailyStakeStatisticsDao
	userStatsDao  rewardDao.IUserDailyStakeStatisticsDao
}

// NewStakeStatisticsRepository 创建质押统计仓储实例
func NewStakeStatisticsRepository() IStakeStatisticsRepository {
	return &stakeStatisticsRepository{
		dailyStatsDao: rewardDao.NewDailyStakeStatisticsDao(),
		userStatsDao:  rewardDao.NewUserDailyStakeStatisticsDao(),
	}
}

// GetByDate 获取指定日期的统计
func (r *stakeStatisticsRepository) GetByDate(ctx context.Context, date time.Time) (*rewardEntity.DailyStakeStatisticsEntity, error) {
	return r.dailyStatsDao.GetByDate(ctx, date)
}

// GetUserStatsByDate 获取指定日期的用户统计
func (r *stakeStatisticsRepository) GetUserStatsByDate(ctx context.Context, date time.Time) ([]*rewardEntity.UserDailyStakeStatisticsEntity, error) {
	return r.userStatsDao.GetByDate(ctx, date)
}

// CalculateGlobalStats 统计全网数据
func (r *stakeStatisticsRepository) CalculateGlobalStats(ctx context.Context, date time.Time) (*rewardEntity.DailyStakeStatisticsEntity, error) {
	return r.dailyStatsDao.CalculateGlobalStats(ctx, date)
}

// CalculateUserStats 统计每个用户数据
func (r *stakeStatisticsRepository) CalculateUserStats(ctx context.Context, date time.Time) ([]*rewardEntity.UserDailyStakeStatisticsEntity, error) {
	return r.userStatsDao.CalculateUserStats(ctx, date)
}

// CreateDailyStats 创建全网统计
func (r *stakeStatisticsRepository) CreateDailyStats(ctx context.Context, tx gdb.TX, stats *rewardEntity.DailyStakeStatisticsEntity) error {
	return r.dailyStatsDao.Create(ctx, tx, stats)
}

// BatchCreateUserStats 批量创建用户统计
func (r *stakeStatisticsRepository) BatchCreateUserStats(ctx context.Context, tx gdb.TX, stats []*rewardEntity.UserDailyStakeStatisticsEntity) error {
	return r.userStatsDao.BatchCreate(ctx, tx, stats)
}
