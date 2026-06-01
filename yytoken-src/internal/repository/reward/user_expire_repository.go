package reward

import (
	"context"
	"time"

	"XWFrame/internal/dao"
	rewardDao "XWFrame/internal/dao/reward"
	rewardEntity "XWFrame/internal/entity/reward"

	"github.com/gogf/gf/v2/database/gdb"
)

// IUserExpireRepository 用户出局检查仓储接口（仅提供数据访问）
type IUserExpireRepository interface {
	// GetExpiredUsers 查询所有已出局的用户
	GetExpiredUsers(ctx context.Context) ([]*rewardEntity.UserQuotaEntity, error)

	// GetExpiredPackagesByUserID 查询用户所有已出局的算力包
	GetExpiredPackagesByUserID(ctx context.Context, tx gdb.TX, userID int64) ([]*rewardEntity.StakingPackageEntity, error)

	// GetQuotaByUserID 获取用户额度
	GetQuotaByUserID(ctx context.Context, userID int64) (*rewardEntity.UserQuotaEntity, error)

	// GetUserVIPByOffset 获取用户VIP等级记录
	GetUserVIPByOffset(ctx context.Context, userID int64, offset *int) ([]*rewardEntity.UserVipLevelEntity, error)

	// UpdatePerformanceToZero 清零业绩
	UpdatePerformanceToZero(ctx context.Context, tx gdb.TX, userID int64, date time.Time) error

	// UpdatePerformanceToZeroWithAudit 清零业绩并记录备注与元数据
	UpdatePerformanceToZeroWithAudit(ctx context.Context, tx gdb.TX, userID int64, date time.Time, remark string, metadata string) error

	// UpdateVIP 更新VIP等级
	UpdateVIP(ctx context.Context, tx gdb.TX, vip *rewardEntity.UserVipLevelEntity) error

	// CreateVIP 创建VIP记录
	CreateVIP(ctx context.Context, tx gdb.TX, vip *rewardEntity.UserVipLevelEntity) error

	// UpdatePackageStatusToExpired 更新算力包状态为出局
	UpdatePackageStatusToExpired(ctx context.Context, tx gdb.TX, userID int64, status int) error

	// ResetCurrentStakeToZero 出局后重置当前统计字段为0
	ResetCurrentStakeToZero(ctx context.Context, tx gdb.TX, userID int64) error

	// GetPerformanceByUserAndDate 根据用户ID与日期获取业绩
	GetPerformanceByUserAndDate(ctx context.Context, userID int64, date time.Time) (*rewardEntity.UserPerformanceEntity, error)
}

// userExpireRepository 用户出局检查仓储实现
type userExpireRepository struct {
	quotaDao       rewardDao.IUserQuotaDao
	quotaRepo      IQuotaRepository
	stakingDao     rewardDao.IStakingPackageDao
	performanceDao rewardDao.IUserPerformanceDao
	vipDao         rewardDao.IUserVipLevelDao
	userDao        dao.IUserDao
}

// NewUserExpireRepository 创建用户出局检查仓储实例
func NewUserExpireRepository() IUserExpireRepository {
	return &userExpireRepository{
		quotaDao:       rewardDao.NewUserQuotaDao(),
		quotaRepo:      NewQuotaRepository(),
		stakingDao:     rewardDao.NewStakingPackageDao(),
		performanceDao: rewardDao.NewUserPerformanceDao(),
		vipDao:         rewardDao.NewUserVipLevelDao(),
		userDao:        dao.NewUserDao(),
	}
}

// GetExpiredUsers 查询所有已出局的用户
func (r *userExpireRepository) GetExpiredUsers(ctx context.Context) ([]*rewardEntity.UserQuotaEntity, error) {
	return r.quotaRepo.GetExpiredUsers(ctx)
}

// GetExpiredPackagesByUserID 查询用户所有已出局的算力包
func (r *userExpireRepository) GetExpiredPackagesByUserID(ctx context.Context, tx gdb.TX, userID int64) ([]*rewardEntity.StakingPackageEntity, error) {
	return r.stakingDao.GetExpiredByUserID(ctx, tx, userID)
}

// GetQuotaByUserID 获取用户额度
func (r *userExpireRepository) GetQuotaByUserID(ctx context.Context, userID int64) (*rewardEntity.UserQuotaEntity, error) {
	return r.quotaRepo.GetByUserID(ctx, userID)
}

// GetUserVIPByOffset 获取用户VIP等级记录
func (r *userExpireRepository) GetUserVIPByOffset(ctx context.Context, userID int64, offset *int) ([]*rewardEntity.UserVipLevelEntity, error) {
	return r.vipDao.GetUserVipLevelByOffset(ctx, userID, offset)
}

// UpdatePerformanceToZero 清零业绩
func (r *userExpireRepository) UpdatePerformanceToZero(ctx context.Context, tx gdb.TX, userID int64, date time.Time) error {
	return r.performanceDao.UpdateToZero(ctx, tx, userID, date)
}

// UpdatePerformanceToZeroWithAudit 清零业绩并记录备注与元数据
func (r *userExpireRepository) UpdatePerformanceToZeroWithAudit(ctx context.Context, tx gdb.TX, userID int64, date time.Time, remark string, metadata string) error {
	return r.performanceDao.UpdateToZeroWithAudit(ctx, tx, userID, date, remark, metadata)
}

// UpdateVIP 更新VIP等级
func (r *userExpireRepository) UpdateVIP(ctx context.Context, tx gdb.TX, vip *rewardEntity.UserVipLevelEntity) error {
	return r.vipDao.Update(ctx, tx, vip)
}

// CreateVIP 创建VIP记录
func (r *userExpireRepository) CreateVIP(ctx context.Context, tx gdb.TX, vip *rewardEntity.UserVipLevelEntity) error {
	return r.vipDao.Create(ctx, tx, vip)
}

// UpdatePackageStatusToExpired 更新算力包状态为出局
func (r *userExpireRepository) UpdatePackageStatusToExpired(ctx context.Context, tx gdb.TX, userID int64, status int) error {
	return r.stakingDao.UpdateStatusToExpired(ctx, tx, userID, status)
}

// ResetCurrentStakeToZero 出局后重置当前统计字段为0
func (r *userExpireRepository) ResetCurrentStakeToZero(ctx context.Context, tx gdb.TX, userID int64) error {
	return r.quotaRepo.ResetCurrentStakeToZero(ctx, tx, userID)
}

// GetPerformanceByUserAndDate 根据用户ID与日期获取业绩
func (r *userExpireRepository) GetPerformanceByUserAndDate(ctx context.Context, userID int64, date time.Time) (*rewardEntity.UserPerformanceEntity, error) {
	return r.performanceDao.GetByUserAndDate(ctx, userID, date)
}
