package dao

import (
	"context"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// IAdminMenuFunctionDao 菜单/功能数据访问接口（只包含基础增删改查）
type IAdminMenuFunctionDao interface {
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
	Create(ctx context.Context, tx gdb.TX, data *entity.AdminMenuFunctionEntity) (int64, error)

	// UpdateById 通用更新方法（更新所有字段）
	UpdateById(ctx context.Context, tx gdb.TX, id int64, resourceName, pageUrl, route, functionKey, icon string, sort int) error

	// GetDataByIds 根据多个ID获取数据（未删除）
	GetDataByIds(ctx context.Context, ids []int64) ([]*entity.AdminMenuFunctionEntity, error)

	// SoftDeleteByIds 软删除多个菜单/功能
	SoftDeleteByIds(ctx context.Context, tx gdb.TX, ids []int64) error

	// GetTopLevelMenus 获取一级菜单（parent_id=0 and is_menu_function=1）
	GetTopLevelMenus(ctx context.Context) ([]*entity.AdminMenuFunctionEntity, error)

	// GetByParentIds 根据父级ID列表获取子项（未删除）
	GetByParentIds(ctx context.Context, parentIds []int64) ([]*entity.AdminMenuFunctionEntity, error)

	// GetFunctionsByIds 根据多个ID获取功能数据（未删除）
	GetFunctionsByIds(ctx context.Context, ids []int64) ([]*entity.AdminMenuFunctionEntity, error)

	// GetFunctionsByConditions 查询功能数据（deleted_at is null and function_key != "" and is_menu_function = MenuTypeFunction and parent_id != 0）
	GetFunctionsByConditions(ctx context.Context) ([]*entity.AdminMenuFunctionEntity, error)
}

// adminMenuFunctionDao 菜单/功能数据访问实现
type adminMenuFunctionDao struct {
	db gdb.DB
}

const adminMenuFunctionSelectFields = "id,created_at,updated_at,parent_id,resource_name,page_url,route,function_key,is_menu_function,icon,sort,deleted_at"

// NewAdminMenuFunctionDao 创建菜单/功能数据访问实例
func NewAdminMenuFunctionDao() IAdminMenuFunctionDao {
	return &adminMenuFunctionDao{
		db: db.GetDB(),
	}
}

// GetById 根据ID获取信息
func (d *adminMenuFunctionDao) GetById(ctx context.Context, id int64) (*entity.AdminMenuFunctionEntity, error) {
	var data entity.AdminMenuFunctionEntity
	err := d.db.Ctx(ctx).Model("admin_menu_function").
		Fields(adminMenuFunctionSelectFields).
		WhereNull("deleted_at").
		Where("id", id).
		Scan(&data)
	if err != nil {
		return nil, err
	}
	if data.Id == 0 {
		return nil, nil
	}
	return &data, nil
}

// GetByResourceName 根据资源名称获取信息
func (d *adminMenuFunctionDao) GetByResourceName(ctx context.Context, resourceName string, excludeId int64, parentId int64) (*entity.AdminMenuFunctionEntity, error) {
	var data entity.AdminMenuFunctionEntity
	query := d.db.Ctx(ctx).Model("admin_menu_function").WhereNull("deleted_at").Where("resource_name", resourceName).Where("parent_id", parentId)
	if excludeId > 0 {
		query = query.Where("id <> ?", excludeId)
	}
	err := query.Fields(adminMenuFunctionSelectFields).Scan(&data)
	if err != nil {
		return nil, err
	}
	if data.Id == 0 {
		return nil, nil
	}
	return &data, nil
}

// GetByRoute 根据API路由获取信息
func (d *adminMenuFunctionDao) GetByRoute(ctx context.Context, route string, excludeId int64) (*entity.AdminMenuFunctionEntity, error) {
	var data entity.AdminMenuFunctionEntity
	query := d.db.Ctx(ctx).Model("admin_menu_function").WhereNull("deleted_at").Where("route", route)
	if excludeId > 0 {
		query = query.Where("id <> ?", excludeId)
	}
	err := query.Fields(adminMenuFunctionSelectFields).Scan(&data)
	if err != nil {
		return nil, err
	}
	if data.Id == 0 {
		return nil, nil
	}
	return &data, nil
}

// GetByFunctionKey 根据功能键名获取信息
func (d *adminMenuFunctionDao) GetByFunctionKey(ctx context.Context, functionKey string, excludeId int64) (*entity.AdminMenuFunctionEntity, error) {
	var data entity.AdminMenuFunctionEntity
	query := d.db.Ctx(ctx).Model("admin_menu_function").WhereNull("deleted_at").Where("function_key", functionKey)
	if excludeId > 0 {
		query = query.Where("id <> ?", excludeId)
	}
	err := query.Fields(adminMenuFunctionSelectFields).Scan(&data)
	if err != nil {
		return nil, err
	}
	if data.Id == 0 {
		return nil, nil
	}
	return &data, nil
}

// UpdateSort 修改排序号，传入id 和新的排序号
func (d *adminMenuFunctionDao) UpdateSort(ctx context.Context, id int64, newSort int) error {
	_, err := d.db.Ctx(ctx).Model("admin_menu_function").
		Where("id", id).
		Data(gdb.Map{
			"sort": newSort,
		}).
		Update()
	return err
}

// Create 创建菜单/功能
func (d *adminMenuFunctionDao) Create(ctx context.Context, tx gdb.TX, data *entity.AdminMenuFunctionEntity) (int64, error) {
	var newId int64
	err := db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		id, err := tx.Model("admin_menu_function").
			FieldsEx("id", "created_at", "updated_at").
			Data(gdb.Map{
				"parent_id":        data.ParentId,
				"resource_name":    data.ResourceName,
				"page_url":         data.PageUrl,
				"route":            data.Route,
				"function_key":     data.FunctionKey,
				"is_menu_function": data.IsMenuFunction,
				"icon":             data.Icon,
				"sort":             data.Sort,
			}).
			InsertAndGetId()
		if err != nil {
			return err
		}
		newId = id
		return nil
	})
	if err != nil {
		return 0, err
	}
	return newId, nil
}

// UpdateById 通用更新方法（更新所有字段）
func (d *adminMenuFunctionDao) UpdateById(ctx context.Context, tx gdb.TX, id int64, resourceName, pageUrl, route, functionKey, icon string, sort int) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Model("admin_menu_function").
			Where("id", id).
			Data(gdb.Map{
				"resource_name": resourceName,
				"page_url":      pageUrl,
				"route":         route,
				"function_key":  functionKey,
				"icon":          icon,
				"sort":          sort,
			}).
			Update()
		return err
	})
}

// GetDataByIds 根据多个ID获取数据（未删除）
func (d *adminMenuFunctionDao) GetDataByIds(ctx context.Context, ids []int64) ([]*entity.AdminMenuFunctionEntity, error) {
	if len(ids) == 0 {
		return []*entity.AdminMenuFunctionEntity{}, nil
	}
	var list []*entity.AdminMenuFunctionEntity
	err := d.db.Ctx(ctx).Model("admin_menu_function").
		Fields(adminMenuFunctionSelectFields).
		WhereNull("deleted_at").
		WhereIn("id", ids).
		Scan(&list)
	return list, err
}

// SoftDeleteByIds 软删除多个菜单/功能
func (d *adminMenuFunctionDao) SoftDeleteByIds(ctx context.Context, tx gdb.TX, ids []int64) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Model("admin_menu_function").
			WhereIn("id", ids).
			Data(gdb.Map{
				"deleted_at": gdb.Raw("NOW()"),
			}).
			Update()
		return err
	})
}

// GetTopLevelMenus 获取一级菜单（parent_id=0 and is_menu_function=1）
func (d *adminMenuFunctionDao) GetTopLevelMenus(ctx context.Context) ([]*entity.AdminMenuFunctionEntity, error) {
	var list []*entity.AdminMenuFunctionEntity
	err := d.db.Ctx(ctx).Model("admin_menu_function").
		Fields(adminMenuFunctionSelectFields).
		WhereNull("deleted_at").
		Where("parent_id", 0).
		Where("is_menu_function", consts.MenuTypeMenu).
		Order("sort ASC, id ASC").
		Scan(&list)
	return list, err
}

// GetByParentIds 根据父级ID列表获取子项（未删除）
func (d *adminMenuFunctionDao) GetByParentIds(ctx context.Context, parentIds []int64) ([]*entity.AdminMenuFunctionEntity, error) {
	if len(parentIds) == 0 {
		return []*entity.AdminMenuFunctionEntity{}, nil
	}
	var list []*entity.AdminMenuFunctionEntity
	err := d.db.Ctx(ctx).Model("admin_menu_function").
		Fields(adminMenuFunctionSelectFields).
		WhereNull("deleted_at").
		WhereIn("parent_id", parentIds).
		Order("sort ASC, id ASC").
		Scan(&list)
	return list, err
}

// GetFunctionsByIds 根据多个ID获取功能数据（未删除）
func (d *adminMenuFunctionDao) GetFunctionsByIds(ctx context.Context, ids []int64) ([]*entity.AdminMenuFunctionEntity, error) {
	if len(ids) == 0 {
		return []*entity.AdminMenuFunctionEntity{}, nil
	}
	var list []*entity.AdminMenuFunctionEntity
	err := d.db.Ctx(ctx).Model("admin_menu_function").
		Fields(adminMenuFunctionSelectFields).
		WhereNull("deleted_at").
		WhereIn("id", ids).
		Scan(&list)
	return list, err
}

// GetFunctionsByConditions 查询功能数据（deleted_at is null and function_key != "" and is_menu_function = MenuTypeFunction and parent_id != 0）
func (d *adminMenuFunctionDao) GetFunctionsByConditions(ctx context.Context) ([]*entity.AdminMenuFunctionEntity, error) {
	var list []*entity.AdminMenuFunctionEntity
	err := d.db.Ctx(ctx).Model("admin_menu_function").
		Fields(adminMenuFunctionSelectFields).
		WhereNull("deleted_at").
		Where("function_key <> ?", "").
		Where("is_menu_function", consts.MenuTypeFunction).
		Where("parent_id <> ?", 0).
		Scan(&list)
	return list, err
}
