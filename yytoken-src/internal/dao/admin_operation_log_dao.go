package dao

import (
	"context"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// IAdminOperationLogDao 后台管理操作日志数据访问接口
type IAdminOperationLogDao interface {
	// Create 创建操作日志
	Create(ctx context.Context, log *entity.AdminOperationLogEntity) error

	// GetList 获取操作日志列表（分页）
	GetList(ctx context.Context, page, pageSize int, operationType, operationTimeStart, operationTimeEnd string) ([]*entity.AdminOperationLogEntity, int, error)
}

// adminOperationLogDao 后台管理操作日志数据访问实现
type adminOperationLogDao struct {
	db gdb.DB
}

// NewAdminOperationLogDao 创建后台管理操作日志数据访问实例
func NewAdminOperationLogDao() IAdminOperationLogDao {
	return &adminOperationLogDao{
		db: db.GetDB(),
	}
}

// Create 创建操作日志
func (d *adminOperationLogDao) Create(ctx context.Context, log *entity.AdminOperationLogEntity) error {
	_, err := d.db.Ctx(ctx).Model("admin_operation_log").
		FieldsEx("id", "created_at").
		Data(log).
		Insert()
	return err
}

// GetList 获取操作日志列表（分页）
func (d *adminOperationLogDao) GetList(ctx context.Context, page, pageSize int, operationType, operationTimeStart, operationTimeEnd string) ([]*entity.AdminOperationLogEntity, int, error) {
	var list []*entity.AdminOperationLogEntity
	query := d.db.Ctx(ctx).Model("admin_operation_log")

	// 操作类型模糊查询
	if operationType != "" {
		query = query.WhereLike("operation_type", "%"+operationType+"%")
	}

	// 操作时间范围查询
	if operationTimeStart != "" {
		// 将日期字符串转换为当天的开始时间：2025-01-01 -> 2025-01-01 00:00:00
		// 支持 YYYY-MM-DD 格式或完整时间格式
		if len(operationTimeStart) == 10 {
			startTime := operationTimeStart + " 00:00:00"
			query = query.WhereGTE("operation_time", startTime)
		} else {
			query = query.WhereGTE("operation_time", operationTimeStart)
		}
	}
	if operationTimeEnd != "" {
		// 将日期字符串转换为当天的结束时间：2025-01-01 -> 2025-01-01 23:59:59
		// 支持 YYYY-MM-DD 格式或完整时间格式
		if len(operationTimeEnd) == 10 {
			endTime := operationTimeEnd + " 23:59:59"
			query = query.WhereLTE("operation_time", endTime)
		} else {
			query = query.WhereLTE("operation_time", operationTimeEnd)
		}
	}

	// 获取总数
	total, err := query.Count()
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	err = query.Order("operation_time DESC, id DESC").
		Limit((page-1)*pageSize, pageSize).
		Scan(&list)
	if err != nil {
		return nil, 0, err
	}

	return list, total, nil
}
