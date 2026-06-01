package reward

import (
	"context"
	"time"

	"XWFrame/internal/dao"
	rewardDao "XWFrame/internal/dao/reward"
	"XWFrame/internal/entity"
	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/consts"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// IRewardDistrictRepository 小区奖励仓储接口（仅提供数据访问）
type IRewardDistrictRepository interface {
	// GetAllUsers 获取所有活跃用户
	GetAllUsers(ctx context.Context) ([]*entity.UserEntity, error)

	// GetAllCurrentVIPs 获取所有当前VIP等级
	GetAllCurrentVIPs(ctx context.Context) ([]*rewardEntity.UserVipLevelEntity, error)

	// GetAllStaticRecords 获取指定日期的所有静态收益记录
	GetAllStaticRecords(ctx context.Context, date time.Time) ([]*rewardEntity.AssetRecordEntity, error)

	// GetPerformanceByDate 获取指定日期的用户业绩
	GetPerformanceByDate(ctx context.Context, date time.Time) ([]*rewardEntity.UserPerformanceEntity, error)

	// GetQuotaByUserID 获取用户额度
	GetQuotaByUserID(ctx context.Context, userID int64) (*rewardEntity.UserQuotaEntity, error)

	// GetQuotasByUserIDs 批量获取用户额度
	GetQuotasByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*rewardEntity.UserQuotaEntity, error)

	// CheckExists 检查记录是否存在（幂等性检查）
	CheckExists(ctx context.Context, userID int64, date time.Time) (bool, error)

	// CreateAssetRecord 创建资金记录
	CreateAssetRecord(ctx context.Context, record *rewardEntity.AssetRecordEntity) error
}

// rewardDistrictRepository 小区奖励仓储实现
type rewardDistrictRepository struct {
	assetRecordDao rewardDao.IAssetRecordDao
	vipDao         rewardDao.IUserVipLevelDao
	quotaRepo      IQuotaRepository
	performanceDao rewardDao.IUserPerformanceDao
	userDao        dao.IUserDao
}

// NewRewardDistrictRepository 创建小区奖励仓储实例
func NewRewardDistrictRepository() IRewardDistrictRepository {
	return &rewardDistrictRepository{
		assetRecordDao: rewardDao.NewAssetRecordDao(),
		vipDao:         rewardDao.NewUserVipLevelDao(),
		quotaRepo:      NewQuotaRepository(),
		performanceDao: rewardDao.NewUserPerformanceDao(),
		userDao:        dao.NewUserDao(),
	}
}

// GetAllUsers 获取所有活跃用户
func (r *rewardDistrictRepository) GetAllUsers(ctx context.Context) ([]*entity.UserEntity, error) {
	return r.userDao.GetActiveUsers(ctx)
}

// GetAllCurrentVIPs 获取所有当前VIP等级
func (r *rewardDistrictRepository) GetAllCurrentVIPs(ctx context.Context) ([]*rewardEntity.UserVipLevelEntity, error) {
	return r.vipDao.GetAllCurrent(ctx)
}

// GetAllStaticRecords 获取指定日期的所有静态收益记录
func (r *rewardDistrictRepository) GetAllStaticRecords(ctx context.Context, date time.Time) ([]*rewardEntity.AssetRecordEntity, error) {
	return r.assetRecordDao.GetAllByDateAndBusiness(ctx, date, consts.AssetBusinessTypeRewardStatic, consts.AssetFlowTypeIncome)
}

// GetPerformanceByDate 获取指定日期的用户业绩
func (r *rewardDistrictRepository) GetPerformanceByDate(ctx context.Context, date time.Time) ([]*rewardEntity.UserPerformanceEntity, error) {
	return r.performanceDao.GetByDate(ctx, date)
}

// GetQuotaByUserID 获取用户额度
func (r *rewardDistrictRepository) GetQuotaByUserID(ctx context.Context, userID int64) (*rewardEntity.UserQuotaEntity, error) {
	return r.quotaRepo.GetByUserID(ctx, userID)
}

// GetQuotasByUserIDs 批量获取用户额度
func (r *rewardDistrictRepository) GetQuotasByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*rewardEntity.UserQuotaEntity, error) {
	return r.quotaRepo.GetByUserIDs(ctx, userIDs)
}

// CheckExists 检查记录是否存在（幂等性检查）
func (r *rewardDistrictRepository) CheckExists(ctx context.Context, userID int64, date time.Time) (bool, error) {
	return r.assetRecordDao.CheckExists(ctx, userID, date, consts.AssetBusinessTypeRewardDistrict)
}

// CreateAssetRecord 创建资金记录
func (r *rewardDistrictRepository) CreateAssetRecord(ctx context.Context, record *rewardEntity.AssetRecordEntity) error {
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		return r.assetRecordDao.Create(ctx, tx, record)
	})
}
