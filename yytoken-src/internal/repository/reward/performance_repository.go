package reward

import (
	"context"
	"time"

	"XWFrame/internal/dao"
	rewardDao "XWFrame/internal/dao/reward"
	"XWFrame/internal/entity"
	rewardEntity "XWFrame/internal/entity/reward"

	"github.com/shopspring/decimal"
)

// IPerformanceRepository 业绩统计仓储接口（仅提供数据查询和保存）
type IPerformanceRepository interface {
	// GetActiveUsers 获取所有激活用户
	GetActiveUsers(ctx context.Context) ([]*entity.UserEntity, error)

	// GetActivePackages 获取所有运行中的算力包
	GetActivePackages(ctx context.Context) ([]*rewardEntity.StakingPackageEntity, error)

	// GetActivePackagesByRecordTime 获取运行中的算力包，且开始时间小于等于指定时间（用于业绩统计）
	GetActivePackagesByRecordTime(ctx context.Context, recordTime time.Time) ([]*rewardEntity.StakingPackageEntity, error)

	// GetPackagesByRecordTimeAndStakeType 获取指定质押类型的算力包，且开始时间小于等于指定时间
	// 说明：不按 status 过滤（别管有没有出局），用于独立口径统计（例如只统计 stake_type=12）。
	GetPackagesByRecordTimeAndStakeType(ctx context.Context, recordTime time.Time, stakeType int) ([]*rewardEntity.StakingPackageEntity, error)

	// GetStakeAmountByStartTimeRange 按开始时间区间统计质押金额
	GetStakeAmountByStartTimeRange(ctx context.Context, start, end time.Time) (map[int64]decimal.Decimal, error)

	// GetStakeAmountByActualEndTimeRange 按实际结束时间区间统计质押金额
	GetStakeAmountByActualEndTimeRange(ctx context.Context, start, end time.Time) (map[int64]decimal.Decimal, error)

	// GetPerformanceByDate 获取指定日期的所有用户业绩
	GetPerformanceByDate(ctx context.Context, date time.Time) ([]*rewardEntity.UserPerformanceEntity, error)

	// GetLatestRecordTimeBefore 获取指定时间之前最近的record_time
	GetLatestRecordTimeBefore(ctx context.Context, date time.Time) (time.Time, error)

	// BatchCreatePerformance 批量创建用户业绩记录
	BatchCreatePerformance(ctx context.Context, performances []*rewardEntity.UserPerformanceEntity) error

	// BatchUpdateIncrementalFields 批量更新新增业绩字段
	BatchUpdateIncrementalFields(ctx context.Context, recordTime time.Time, updates []*PerformanceIncrementUpdate) error
}

// performanceRepository 业绩统计仓储实现
type performanceRepository struct {
	performanceDao rewardDao.IUserPerformanceDao
	stakingDao     rewardDao.IStakingPackageDao
	userDao        dao.IUserDao
}

// PerformanceIncrementUpdate 新增业绩字段更新请求
type PerformanceIncrementUpdate struct {
	UserID                 int64
	NewPersonalPerformance decimal.Decimal
	NewDirectPerformance   decimal.Decimal
	NewTeamPerformance     decimal.Decimal
	NewDistrictPerformance decimal.Decimal
	NewExpiredStake        decimal.Decimal
}

// NewPerformanceRepository 创建业绩统计仓储实例
func NewPerformanceRepository() IPerformanceRepository {
	return &performanceRepository{
		performanceDao: rewardDao.NewUserPerformanceDao(),
		stakingDao:     rewardDao.NewStakingPackageDao(),
		userDao:        dao.NewUserDao(),
	}
}

// GetActiveUsers 获取所有激活用户
func (r *performanceRepository) GetActiveUsers(ctx context.Context) ([]*entity.UserEntity, error) {
	return r.userDao.GetActiveUsers(ctx)
}

// GetActivePackages 获取所有运行中的算力包
func (r *performanceRepository) GetActivePackages(ctx context.Context) ([]*rewardEntity.StakingPackageEntity, error) {
	return r.stakingDao.GetActivePackages(ctx)
}

// GetActivePackagesByRecordTime 获取运行中的算力包，且开始时间小于等于指定时间（用于业绩统计）
func (r *performanceRepository) GetActivePackagesByRecordTime(ctx context.Context, recordTime time.Time) ([]*rewardEntity.StakingPackageEntity, error) {
	return r.stakingDao.GetActivePackagesByRecordTime(ctx, recordTime)
}

func (r *performanceRepository) GetPackagesByRecordTimeAndStakeType(ctx context.Context, recordTime time.Time, stakeType int) ([]*rewardEntity.StakingPackageEntity, error) {
	return r.stakingDao.GetPackagesByRecordTimeAndStakeType(ctx, recordTime, stakeType)
}

// GetStakeAmountByStartTimeRange 按开始时间区间统计质押金额
func (r *performanceRepository) GetStakeAmountByStartTimeRange(ctx context.Context, start, end time.Time) (map[int64]decimal.Decimal, error) {
	return r.stakingDao.GetStakeAmountByStartTimeRange(ctx, start, end)
}

// GetStakeAmountByActualEndTimeRange 按实际结束时间区间统计质押金额
func (r *performanceRepository) GetStakeAmountByActualEndTimeRange(ctx context.Context, start, end time.Time) (map[int64]decimal.Decimal, error) {
	return r.stakingDao.GetStakeAmountByActualEndTimeRange(ctx, start, end)
}

// GetPerformanceByDate 获取指定日期的所有用户业绩
func (r *performanceRepository) GetPerformanceByDate(ctx context.Context, date time.Time) ([]*rewardEntity.UserPerformanceEntity, error) {
	return r.performanceDao.GetByDate(ctx, date)
}

// GetLatestRecordTimeBefore 获取指定时间之前最近的record_time
func (r *performanceRepository) GetLatestRecordTimeBefore(ctx context.Context, date time.Time) (time.Time, error) {
	return r.performanceDao.GetLatestRecordTimeBefore(ctx, date)
}

// BatchCreatePerformance 批量创建用户业绩记录
func (r *performanceRepository) BatchCreatePerformance(ctx context.Context, performances []*rewardEntity.UserPerformanceEntity) error {
	return r.performanceDao.BatchCreate(ctx, nil, performances)
}

// BatchUpdateIncrementalFields 批量更新新增业绩字段
func (r *performanceRepository) BatchUpdateIncrementalFields(ctx context.Context, recordTime time.Time, updates []*PerformanceIncrementUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	for _, item := range updates {
		if item == nil {
			continue
		}

		data := map[string]interface{}{
			"new_personal_performance": item.NewPersonalPerformance,
			"new_direct_performance":   item.NewDirectPerformance,
			"new_team_performance":     item.NewTeamPerformance,
			"new_district_performance": item.NewDistrictPerformance,
			"new_expired_stake":        item.NewExpiredStake,
		}

		if err := r.performanceDao.UpdateIncrementalFields(ctx, nil, item.UserID, recordTime, data); err != nil {
			return err
		}
	}

	return nil
}
