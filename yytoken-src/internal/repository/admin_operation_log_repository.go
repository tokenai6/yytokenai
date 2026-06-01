package repository

import (
	"context"

	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
)

// IAdminOperationLogRepository 后台管理操作日志仓储接口
type IAdminOperationLogRepository interface {
	// CreateLog 创建操作日志
	CreateLog(ctx context.Context, log *entity.AdminOperationLogEntity) error

	// GetList 获取操作日志列表（分页）
	GetList(ctx context.Context, page, pageSize int, operationType, operationTimeStart, operationTimeEnd string) ([]*entity.AdminOperationLogEntity, int, error)
}

// adminOperationLogRepository 后台管理操作日志仓储实现
type adminOperationLogRepository struct {
	logDao dao.IAdminOperationLogDao
}

// NewAdminOperationLogRepository 创建后台管理操作日志仓储实例
func NewAdminOperationLogRepository() IAdminOperationLogRepository {
	return &adminOperationLogRepository{
		logDao: dao.NewAdminOperationLogDao(),
	}
}

// CreateLog 创建操作日志
func (r *adminOperationLogRepository) CreateLog(ctx context.Context, log *entity.AdminOperationLogEntity) error {
	return r.logDao.Create(ctx, log)
}

// GetList 获取操作日志列表（分页）
func (r *adminOperationLogRepository) GetList(ctx context.Context, page, pageSize int, operationType, operationTimeStart, operationTimeEnd string) ([]*entity.AdminOperationLogEntity, int, error) {
	return r.logDao.GetList(ctx, page, pageSize, operationType, operationTimeStart, operationTimeEnd)
}
