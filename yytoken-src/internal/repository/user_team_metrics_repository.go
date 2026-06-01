package repository

import (
	"context"
	"time"

	"XWFrame/internal/dao"
	"XWFrame/internal/entity"

	"github.com/shopspring/decimal"
)

// IUserTeamMetricsRepository 用户团队指标仓储接口
type IUserTeamMetricsRepository interface {
	// Create 创建用户团队指标
	Create(ctx context.Context, metrics *entity.UserTeamMetricsEntity) error

	// GetById 根据ID获取用户团队指标
	GetById(ctx context.Context, id int64) (*entity.UserTeamMetricsEntity, error)

	// GetByAddress 根据地址获取用户团队指标
	GetByAddress(ctx context.Context, address string) (*entity.UserTeamMetricsEntity, error)

	// GetByUserId 根据用户ID获取用户团队指标
	GetByUserId(ctx context.Context, userId int64) (*entity.UserTeamMetricsEntity, error)

	// GetList 分页获取用户团队指标列表
	GetList(ctx context.Context, page, pageSize int, address string) ([]*entity.UserTeamMetricsEntity, int, error)

	// UpdateById 根据ID更新用户团队指标
	UpdateById(ctx context.Context, id int64, data map[string]interface{}) error

	// DeleteById 根据ID删除用户团队指标
	DeleteById(ctx context.Context, id int64) error

	// GetAllMetrics 获取所有用户团队指标
	GetAllMetrics(ctx context.Context) ([]*entity.UserTeamMetricsEntity, error)

	// GetUserTeamNewByCycle 根据周期获取用户团队新增
	GetUserTeamNewByCycle(ctx context.Context, userId int64, cycleStartTime, cycleEndTime time.Time) (decimal.Decimal, error)

	// GetTeamStaticReleaseByCycle 获取团队成员（包括自己）的未出局总质押金额
	GetTeamStaticReleaseByCycle(ctx context.Context, userId int64, cycleStartTime, cycleEndTime time.Time) (decimal.Decimal, error)
}

// userTeamMetricsRepository 用户团队指标仓储实现
type userTeamMetricsRepository struct {
	metricsDao dao.IUserTeamMetricsDao
}

// NewUserTeamMetricsRepository 创建用户团队指标仓储实例
func NewUserTeamMetricsRepository() IUserTeamMetricsRepository {
	return &userTeamMetricsRepository{
		metricsDao: dao.NewUserTeamMetricsDao(),
	}
}

// Create 创建用户团队指标
func (r *userTeamMetricsRepository) Create(ctx context.Context, metrics *entity.UserTeamMetricsEntity) error {
	return r.metricsDao.Create(ctx, metrics)
}

// GetById 根据ID获取用户团队指标
func (r *userTeamMetricsRepository) GetById(ctx context.Context, id int64) (*entity.UserTeamMetricsEntity, error) {
	return r.metricsDao.GetById(ctx, id)
}

// GetByAddress 根据地址获取用户团队指标
func (r *userTeamMetricsRepository) GetByAddress(ctx context.Context, address string) (*entity.UserTeamMetricsEntity, error) {
	return r.metricsDao.GetByAddress(ctx, address)
}

// GetByUserId 根据用户ID获取用户团队指标
func (r *userTeamMetricsRepository) GetByUserId(ctx context.Context, userId int64) (*entity.UserTeamMetricsEntity, error) {
	return r.metricsDao.GetByUserId(ctx, userId)
}

// GetList 分页获取用户团队指标列表
func (r *userTeamMetricsRepository) GetList(ctx context.Context, page, pageSize int, address string) ([]*entity.UserTeamMetricsEntity, int, error) {
	return r.metricsDao.GetList(ctx, page, pageSize, address)
}

// UpdateById 根据ID更新用户团队指标
func (r *userTeamMetricsRepository) UpdateById(ctx context.Context, id int64, data map[string]interface{}) error {
	return r.metricsDao.UpdateById(ctx, id, data)
}

// DeleteById 根据ID删除用户团队指标
func (r *userTeamMetricsRepository) DeleteById(ctx context.Context, id int64) error {
	return r.metricsDao.DeleteById(ctx, id)
}

// GetAllMetrics 获取所有用户团队指标
func (r *userTeamMetricsRepository) GetAllMetrics(ctx context.Context) ([]*entity.UserTeamMetricsEntity, error) {
	return r.metricsDao.GetAllMetrics(ctx)
}

// GetUserTeamNewByCycle 根据周期获取用户团队新增
func (r *userTeamMetricsRepository) GetUserTeamNewByCycle(ctx context.Context, userId int64, cycleStartTime, cycleEndTime time.Time) (decimal.Decimal, error) {
	return r.metricsDao.GetUserTeamNewByCycle(ctx, userId, cycleStartTime, cycleEndTime)
}

// GetTeamStaticReleaseByCycle 获取团队成员（包括自己）的未出局总质押金额
func (r *userTeamMetricsRepository) GetTeamStaticReleaseByCycle(ctx context.Context, userId int64, cycleStartTime, cycleEndTime time.Time) (decimal.Decimal, error) {
	return r.metricsDao.GetTeamStaticReleaseByCycle(ctx, userId, cycleStartTime, cycleEndTime)
}
