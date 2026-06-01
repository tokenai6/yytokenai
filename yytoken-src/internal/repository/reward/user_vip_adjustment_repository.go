package reward

import (
	"context"
	"time"

	rewardDao "XWFrame/internal/dao/reward"
	rewardEntity "XWFrame/internal/entity/reward"

	"github.com/gogf/gf/v2/database/gdb"
)

// IUserVipAdjustmentRepository VIP调整记录仓储接口
type IUserVipAdjustmentRepository interface {
	// CreateAdjustment 创建调整记录
	CreateAdjustment(ctx context.Context, tx gdb.TX, adjustment *rewardEntity.UserVipAdjustmentEntity) error

	// GetActiveAdjustment 获取用户有效的调整记录
	GetActiveAdjustment(ctx context.Context, userID int64, now time.Time) (*rewardEntity.UserVipAdjustmentEntity, error)

	// BatchGetActiveAdjustments 批量获取用户有效的调整记录
	BatchGetActiveAdjustments(ctx context.Context, userIDs []int64, now time.Time) (map[int64]*rewardEntity.UserVipAdjustmentEntity, error)

	// GetAdjustmentByID 根据ID获取调整记录
	GetAdjustmentByID(ctx context.Context, id int64) (*rewardEntity.UserVipAdjustmentEntity, error)

	// CancelAdjustment 取消调整记录
	CancelAdjustment(ctx context.Context, tx gdb.TX, id int64, now time.Time) error

	// GetAdjustmentList 获取调整记录列表（分页）
	GetAdjustmentList(ctx context.Context, userID int64, page, pageSize int) ([]*rewardEntity.UserVipAdjustmentEntity, int, error)
}

// userVipAdjustmentRepository VIP调整记录仓储实现
type userVipAdjustmentRepository struct {
	adjustmentDao rewardDao.IUserVipAdjustmentDao
}

// NewUserVipAdjustmentRepository 创建VIP调整记录仓储实例
func NewUserVipAdjustmentRepository() IUserVipAdjustmentRepository {
	return &userVipAdjustmentRepository{
		adjustmentDao: rewardDao.NewUserVipAdjustmentDao(),
	}
}

// CreateAdjustment 创建调整记录
func (r *userVipAdjustmentRepository) CreateAdjustment(ctx context.Context, tx gdb.TX, adjustment *rewardEntity.UserVipAdjustmentEntity) error {
	return r.adjustmentDao.Create(ctx, tx, adjustment)
}

// GetActiveAdjustment 获取用户有效的调整记录
func (r *userVipAdjustmentRepository) GetActiveAdjustment(ctx context.Context, userID int64, now time.Time) (*rewardEntity.UserVipAdjustmentEntity, error) {
	return r.adjustmentDao.GetActiveByUserID(ctx, userID, now)
}

// BatchGetActiveAdjustments 批量获取用户有效的调整记录
func (r *userVipAdjustmentRepository) BatchGetActiveAdjustments(ctx context.Context, userIDs []int64, now time.Time) (map[int64]*rewardEntity.UserVipAdjustmentEntity, error) {
	return r.adjustmentDao.BatchGetActiveByUserIDs(ctx, userIDs, now)
}

// GetAdjustmentByID 根据ID获取调整记录
func (r *userVipAdjustmentRepository) GetAdjustmentByID(ctx context.Context, id int64) (*rewardEntity.UserVipAdjustmentEntity, error) {
	return r.adjustmentDao.GetByID(ctx, id)
}

// CancelAdjustment 取消调整记录
func (r *userVipAdjustmentRepository) CancelAdjustment(ctx context.Context, tx gdb.TX, id int64, now time.Time) error {
	return r.adjustmentDao.Cancel(ctx, tx, id, now)
}

// GetAdjustmentList 获取调整记录列表（分页）
func (r *userVipAdjustmentRepository) GetAdjustmentList(ctx context.Context, userID int64, page, pageSize int) ([]*rewardEntity.UserVipAdjustmentEntity, int, error) {
	return r.adjustmentDao.GetList(ctx, userID, page, pageSize)
}
