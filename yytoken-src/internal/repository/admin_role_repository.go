package repository

import (
	"context"

	"XWFrame/internal/dao"
	"XWFrame/internal/entity"

	"github.com/gogf/gf/v2/database/gdb"
)

// IAdminRoleRepository 角色仓储接口（业务逻辑层）
type IAdminRoleRepository interface {
	// GetById 根据ID获取信息
	GetById(ctx context.Context, id int64) (*entity.AdminRoleEntity, error)

	// GetByRoleName 根据角色名称获取信息
	GetByRoleName(ctx context.Context, roleName string, excludeId int64) (*entity.AdminRoleEntity, error)

	// GetList 获取列表（分页）
	GetList(ctx context.Context, page, pageSize int) ([]*entity.AdminRoleEntity, int, error)

	// Create 创建角色
	Create(ctx context.Context, tx gdb.TX, roleName string) (int64, error)

	// UpdateById 根据ID更新角色名称
	UpdateById(ctx context.Context, tx gdb.TX, id int64, roleName string) error

	// GetDataByIds 根据多个ID获取数据
	GetDataByIds(ctx context.Context, ids []int64) ([]*entity.AdminRoleEntity, error)

	// SoftDeleteByIds 软删除多个角色
	SoftDeleteByIds(ctx context.Context, tx gdb.TX, ids []int64) error
}

// adminRoleRepository 角色仓储实现
type adminRoleRepository struct {
	roleDao dao.IAdminRoleDao
}

// NewAdminRoleRepository 创建角色仓储实例
func NewAdminRoleRepository() IAdminRoleRepository {
	return &adminRoleRepository{
		roleDao: dao.NewAdminRoleDao(),
	}
}

// GetById 根据ID获取信息
func (r *adminRoleRepository) GetById(ctx context.Context, id int64) (*entity.AdminRoleEntity, error) {
	return r.roleDao.GetById(ctx, id)
}

// GetByRoleName 根据角色名称获取信息
func (r *adminRoleRepository) GetByRoleName(ctx context.Context, roleName string, excludeId int64) (*entity.AdminRoleEntity, error) {
	return r.roleDao.GetByRoleName(ctx, roleName, excludeId)
}

// GetList 获取列表（分页）
func (r *adminRoleRepository) GetList(ctx context.Context, page, pageSize int) ([]*entity.AdminRoleEntity, int, error) {
	return r.roleDao.GetList(ctx, page, pageSize)
}

// Create 创建角色
func (r *adminRoleRepository) Create(ctx context.Context, tx gdb.TX, roleName string) (int64, error) {
	return r.roleDao.Create(ctx, tx, roleName)
}

// UpdateById 根据ID更新角色名称
func (r *adminRoleRepository) UpdateById(ctx context.Context, tx gdb.TX, id int64, roleName string) error {
	return r.roleDao.UpdateById(ctx, tx, id, roleName)
}

// GetDataByIds 根据多个ID获取数据
func (r *adminRoleRepository) GetDataByIds(ctx context.Context, ids []int64) ([]*entity.AdminRoleEntity, error) {
	return r.roleDao.GetDataByIds(ctx, ids)
}

// SoftDeleteByIds 软删除多个角色
func (r *adminRoleRepository) SoftDeleteByIds(ctx context.Context, tx gdb.TX, ids []int64) error {
	return r.roleDao.SoftDeleteByIds(ctx, tx, ids)
}
