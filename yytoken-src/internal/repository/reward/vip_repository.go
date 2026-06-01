package reward

import (
	"context"
	"time"

	rewardDao "XWFrame/internal/dao/reward"
	rewardEntity "XWFrame/internal/entity/reward"

	"github.com/gogf/gf/v2/database/gdb"
)

// IVipRepository VIP等级仓储接口（仅提供数据查询和保存）
type IVipRepository interface {
	// GetPerformanceByDate 获取指定日期的所有用户业绩
	GetPerformanceByDate(ctx context.Context, date time.Time) ([]*rewardEntity.UserPerformanceEntity, error)

	// BatchGetCurrentVIPs 批量获取当前VIP等级
	BatchGetCurrentVIPs(ctx context.Context, userIDs []int64) ([]*rewardEntity.UserVipLevelEntity, error)

	// BatchCreateVIPs 批量创建VIP记录
	BatchCreateVIPs(ctx context.Context, vips []*rewardEntity.UserVipLevelEntity) error

	// SetNotCurrent 设置VIP记录为非当前（在事务中）
	SetNotCurrent(ctx context.Context, tx gdb.TX, vipID int64) error

	// CreateVIP 创建VIP记录（在事务中）
	CreateVIP(ctx context.Context, tx gdb.TX, vip *rewardEntity.UserVipLevelEntity) error
}

// vipRepository VIP等级仓储实现
type vipRepository struct {
	vipDao         rewardDao.IUserVipLevelDao
	performanceDao rewardDao.IUserPerformanceDao
}

// NewVipRepository 创建VIP等级仓储实例
func NewVipRepository() IVipRepository {
	return &vipRepository{
		vipDao:         rewardDao.NewUserVipLevelDao(),
		performanceDao: rewardDao.NewUserPerformanceDao(),
	}
}

// GetPerformanceByDate 获取指定日期的所有用户业绩
func (r *vipRepository) GetPerformanceByDate(ctx context.Context, date time.Time) ([]*rewardEntity.UserPerformanceEntity, error) {
	return r.performanceDao.GetByDate(ctx, date)
}

// BatchGetCurrentVIPs 批量获取当前VIP等级
func (r *vipRepository) BatchGetCurrentVIPs(ctx context.Context, userIDs []int64) ([]*rewardEntity.UserVipLevelEntity, error) {
	return r.vipDao.BatchGetCurrentVIPs(ctx, userIDs)
}

// BatchCreateVIPs 批量创建VIP记录
func (r *vipRepository) BatchCreateVIPs(ctx context.Context, vips []*rewardEntity.UserVipLevelEntity) error {
	return r.vipDao.BatchCreate(ctx, nil, vips)
}

// SetNotCurrent 设置VIP记录为非当前（在事务中）
func (r *vipRepository) SetNotCurrent(ctx context.Context, tx gdb.TX, vipID int64) error {
	return r.vipDao.SetNotCurrent(ctx, tx, vipID)
}

// CreateVIP 创建VIP记录（在事务中）
func (r *vipRepository) CreateVIP(ctx context.Context, tx gdb.TX, vip *rewardEntity.UserVipLevelEntity) error {
	return r.vipDao.Create(ctx, tx, vip)
}
