package repository

import (
	"context"

	"XWFrame/internal/dao"
	"XWFrame/internal/entity"

	"github.com/gogf/gf/v2/database/gdb"
)

// IAdminRoleMenuFunctionMappingRepository 角色与菜单功能映射仓储接口（业务逻辑层）
type IAdminRoleMenuFunctionMappingRepository interface {
	// GetDataByRoleId 根据角色id获取数据
	GetDataByRoleId(ctx context.Context, roleId int64) ([]*entity.AdminRoleMenuFunctionMappingEntity, error)

	// BatchCreate 批量创建角色与功能映射
	BatchCreate(ctx context.Context, tx gdb.TX, roleId int64, functionIds []int64) error

	// DeleteByRoleId 根据角色id删除映射数据
	DeleteByRoleId(ctx context.Context, tx gdb.TX, roleId int64) error
}

// adminRoleMenuFunctionMappingRepository 映射仓储实现
type adminRoleMenuFunctionMappingRepository struct {
	mappingDao dao.IAdminRoleMenuFunctionMappingDao
}

// NewAdminRoleMenuFunctionMappingRepository 创建映射仓储实例
func NewAdminRoleMenuFunctionMappingRepository() IAdminRoleMenuFunctionMappingRepository {
	return &adminRoleMenuFunctionMappingRepository{
		mappingDao: dao.NewAdminRoleMenuFunctionMappingDao(),
	}
}

// GetDataByRoleId 根据角色id获取数据
func (r *adminRoleMenuFunctionMappingRepository) GetDataByRoleId(ctx context.Context, roleId int64) ([]*entity.AdminRoleMenuFunctionMappingEntity, error) {
	return r.mappingDao.GetDataByRoleId(ctx, roleId)
}

// BatchCreate 批量创建角色与功能映射
func (r *adminRoleMenuFunctionMappingRepository) BatchCreate(ctx context.Context, tx gdb.TX, roleId int64, functionIds []int64) error {
	return r.mappingDao.BatchCreate(ctx, tx, roleId, functionIds)
}

// DeleteByRoleId 根据角色id删除映射数据
func (r *adminRoleMenuFunctionMappingRepository) DeleteByRoleId(ctx context.Context, tx gdb.TX, roleId int64) error {
	return r.mappingDao.DeleteByRoleId(ctx, tx, roleId)
}
