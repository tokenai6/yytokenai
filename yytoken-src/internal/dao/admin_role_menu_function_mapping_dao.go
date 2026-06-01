package dao

import (
	"context"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// IAdminRoleMenuFunctionMappingDao 角色与菜单功能映射数据访问接口（只包含基础增删改查）
type IAdminRoleMenuFunctionMappingDao interface {
	// GetDataByRoleId 根据角色id获取数据
	GetDataByRoleId(ctx context.Context, roleId int64) ([]*entity.AdminRoleMenuFunctionMappingEntity, error)

	// BatchCreate 批量创建角色与功能映射
	BatchCreate(ctx context.Context, tx gdb.TX, roleId int64, functionIds []int64) error

	// DeleteByRoleId 根据角色id删除映射数据
	DeleteByRoleId(ctx context.Context, tx gdb.TX, roleId int64) error
}

// adminRoleMenuFunctionMappingDao 映射数据访问实现
type adminRoleMenuFunctionMappingDao struct {
	db gdb.DB
}

// NewAdminRoleMenuFunctionMappingDao 创建映射数据访问实例
func NewAdminRoleMenuFunctionMappingDao() IAdminRoleMenuFunctionMappingDao {
	return &adminRoleMenuFunctionMappingDao{
		db: db.GetDB(),
	}
}

// GetDataByRoleId 根据角色id获取数据
func (d *adminRoleMenuFunctionMappingDao) GetDataByRoleId(ctx context.Context, roleId int64) ([]*entity.AdminRoleMenuFunctionMappingEntity, error) {
	var list []*entity.AdminRoleMenuFunctionMappingEntity
	err := d.db.Ctx(ctx).Model("admin_role_menu_function_mapping").
		WhereNull("deleted_at").
		Where("role_id", roleId).
		Order("id ASC").
		Scan(&list)
	return list, err
}

// BatchCreate 批量创建角色与功能映射
func (d *adminRoleMenuFunctionMappingDao) BatchCreate(ctx context.Context, tx gdb.TX, roleId int64, functionIds []int64) error {
	if len(functionIds) == 0 {
		return nil
	}
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		for _, functionId := range functionIds {
			_, err := tx.Model("admin_role_menu_function_mapping").
				FieldsEx("id", "created_at", "updated_at").
				Data(gdb.Map{
					"role_id":     roleId,
					"function_id": functionId,
				}).
				Insert()
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteByRoleId 根据角色id删除映射数据
func (d *adminRoleMenuFunctionMappingDao) DeleteByRoleId(ctx context.Context, tx gdb.TX, roleId int64) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Model("admin_role_menu_function_mapping").
			Where("role_id", roleId).
			Delete()
		return err
	})
}
