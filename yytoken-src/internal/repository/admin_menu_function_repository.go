package repository

import (
	"context"

	"XWFrame/internal/dao"
	"XWFrame/internal/entity"

	"github.com/gogf/gf/v2/database/gdb"
)

// IAdminMenuFunctionRepository 菜单/功能仓储接口（业务逻辑层）
type IAdminMenuFunctionRepository interface {
	// GetById 根据ID获取信息
	GetById(ctx context.Context, id int64) (*entity.AdminMenuFunctionEntity, error)

	// GetByResourceName 根据资源名称获取信息（限定同一父级）
	GetByResourceName(ctx context.Context, resourceName string, excludeId int64, parentId int64) (*entity.AdminMenuFunctionEntity, error)

	// GetByRoute 根据API路由获取信息
	GetByRoute(ctx context.Context, route string, excludeId int64) (*entity.AdminMenuFunctionEntity, error)

	// GetByFunctionKey 根据功能键名获取信息
	GetByFunctionKey(ctx context.Context, functionKey string, excludeId int64) (*entity.AdminMenuFunctionEntity, error)

	// UpdateSort 修改排序号，传入id 和新的排序号
	UpdateSort(ctx context.Context, id int64, newSort int) error

	// Create 创建菜单/功能
	Create(ctx context.Context, data *entity.AdminMenuFunctionEntity) (int64, error)

	// UpdateById 通用更新方法（更新所有字段）
	UpdateById(ctx context.Context, id int64, resourceName, pageUrl, route, functionKey, icon string, sort int) error

	// GetDataByIds 根据多个ID获取数据
	GetDataByIds(ctx context.Context, ids []int64) ([]*entity.AdminMenuFunctionEntity, error)

	// SoftDeleteByIds 软删除多个菜单/功能
	SoftDeleteByIds(ctx context.Context, tx gdb.TX, ids []int64) error

	// GetTopLevelMenus 获取一级菜单
	GetTopLevelMenus(ctx context.Context) ([]*entity.AdminMenuFunctionEntity, error)

	// GetByParentIds 根据父级ID列表获取子项
	GetByParentIds(ctx context.Context, parentIds []int64) ([]*entity.AdminMenuFunctionEntity, error)

	// GetFunctionsByIds 根据多个ID获取功能数据（未删除，且is_menu_function=2）
	GetFunctionsByIds(ctx context.Context, ids []int64) ([]*entity.AdminMenuFunctionEntity, error)

	// GetFunctionsByConditions 查询功能数据（deleted_at is null and function_key != "" and is_menu_function = MenuTypeFunction and parent_id != 0）
	GetFunctionsByConditions(ctx context.Context) ([]*entity.AdminMenuFunctionEntity, error)
}

// adminMenuFunctionRepository 菜单/功能仓储实现
type adminMenuFunctionRepository struct {
	menuFuncDao dao.IAdminMenuFunctionDao
}

// NewAdminMenuFunctionRepository 创建菜单/功能仓储实例
func NewAdminMenuFunctionRepository() IAdminMenuFunctionRepository {
	return &adminMenuFunctionRepository{
		menuFuncDao: dao.NewAdminMenuFunctionDao(),
	}
}

// GetById 根据ID获取信息
func (r *adminMenuFunctionRepository) GetById(ctx context.Context, id int64) (*entity.AdminMenuFunctionEntity, error) {
	return r.menuFuncDao.GetById(ctx, id)
}

// GetByResourceName 根据资源名称获取信息
func (r *adminMenuFunctionRepository) GetByResourceName(ctx context.Context, resourceName string, excludeId int64, parentId int64) (*entity.AdminMenuFunctionEntity, error) {
	return r.menuFuncDao.GetByResourceName(ctx, resourceName, excludeId, parentId)
}

// GetByRoute 根据API路由获取信息
func (r *adminMenuFunctionRepository) GetByRoute(ctx context.Context, route string, excludeId int64) (*entity.AdminMenuFunctionEntity, error) {
	return r.menuFuncDao.GetByRoute(ctx, route, excludeId)
}

// GetByFunctionKey 根据功能键名获取信息
func (r *adminMenuFunctionRepository) GetByFunctionKey(ctx context.Context, functionKey string, excludeId int64) (*entity.AdminMenuFunctionEntity, error) {
	return r.menuFuncDao.GetByFunctionKey(ctx, functionKey, excludeId)
}

// UpdateSort 修改排序号，传入id 和新的排序号
func (r *adminMenuFunctionRepository) UpdateSort(ctx context.Context, id int64, newSort int) error {
	return r.menuFuncDao.UpdateSort(ctx, id, newSort)
}

// Create 创建菜单/功能
func (r *adminMenuFunctionRepository) Create(ctx context.Context, data *entity.AdminMenuFunctionEntity) (int64, error) {
	return r.menuFuncDao.Create(ctx, nil, data)
}

// UpdateById 通用更新方法（更新所有字段）
func (r *adminMenuFunctionRepository) UpdateById(ctx context.Context, id int64, resourceName, pageUrl, route, functionKey, icon string, sort int) error {
	return r.menuFuncDao.UpdateById(ctx, nil, id, resourceName, pageUrl, route, functionKey, icon, sort)
}

// GetDataByIds 根据多个ID获取数据
func (r *adminMenuFunctionRepository) GetDataByIds(ctx context.Context, ids []int64) ([]*entity.AdminMenuFunctionEntity, error) {
	return r.menuFuncDao.GetDataByIds(ctx, ids)
}

// SoftDeleteByIds 软删除多个菜单/功能
func (r *adminMenuFunctionRepository) SoftDeleteByIds(ctx context.Context, tx gdb.TX, ids []int64) error {
	return r.menuFuncDao.SoftDeleteByIds(ctx, tx, ids)
}

// GetTopLevelMenus 获取一级菜单
func (r *adminMenuFunctionRepository) GetTopLevelMenus(ctx context.Context) ([]*entity.AdminMenuFunctionEntity, error) {
	return r.menuFuncDao.GetTopLevelMenus(ctx)
}

// GetByParentIds 根据父级ID列表获取子项
func (r *adminMenuFunctionRepository) GetByParentIds(ctx context.Context, parentIds []int64) ([]*entity.AdminMenuFunctionEntity, error) {
	return r.menuFuncDao.GetByParentIds(ctx, parentIds)
}

// GetFunctionsByIds 根据多个ID获取功能数据（未删除，且is_menu_function=2）
func (r *adminMenuFunctionRepository) GetFunctionsByIds(ctx context.Context, ids []int64) ([]*entity.AdminMenuFunctionEntity, error) {
	return r.menuFuncDao.GetFunctionsByIds(ctx, ids)
}

// GetFunctionsByConditions 查询功能数据（deleted_at is null and function_key != "" and is_menu_function = MenuTypeFunction and parent_id != 0）
func (r *adminMenuFunctionRepository) GetFunctionsByConditions(ctx context.Context) ([]*entity.AdminMenuFunctionEntity, error) {
	return r.menuFuncDao.GetFunctionsByConditions(ctx)
}
