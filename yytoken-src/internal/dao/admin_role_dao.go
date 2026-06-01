package dao

import (
	"context"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// IAdminRoleDao 角色数据访问接口（只包含基础增删改查）
type IAdminRoleDao interface {
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

	// GetDataByIds 根据多个ID获取数据（未删除）
	GetDataByIds(ctx context.Context, ids []int64) ([]*entity.AdminRoleEntity, error)

	// SoftDeleteByIds 软删除多个角色
	SoftDeleteByIds(ctx context.Context, tx gdb.TX, ids []int64) error
}

// adminRoleDao 角色数据访问实现
type adminRoleDao struct {
	db gdb.DB
}

// NewAdminRoleDao 创建角色数据访问实例
func NewAdminRoleDao() IAdminRoleDao {
	return &adminRoleDao{
		db: db.GetDB(),
	}
}

// GetById 根据ID获取信息
func (d *adminRoleDao) GetById(ctx context.Context, id int64) (*entity.AdminRoleEntity, error) {
	var data entity.AdminRoleEntity
	err := d.db.Ctx(ctx).Model("admin_role").
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

// GetByRoleName 根据角色名称获取信息
func (d *adminRoleDao) GetByRoleName(ctx context.Context, roleName string, excludeId int64) (*entity.AdminRoleEntity, error) {
	var data entity.AdminRoleEntity
	query := d.db.Ctx(ctx).Model("admin_role").WhereNull("deleted_at").Where("role_name", roleName)
	if excludeId > 0 {
		query = query.Where("id <> ?", excludeId)
	}
	err := query.Scan(&data)
	if err != nil {
		return nil, err
	}
	if data.Id == 0 {
		return nil, nil
	}
	return &data, nil
}

// GetList 获取列表（分页）
func (d *adminRoleDao) GetList(ctx context.Context, page, pageSize int) ([]*entity.AdminRoleEntity, int, error) {
	var list []*entity.AdminRoleEntity
	query := d.db.Ctx(ctx).Model("admin_role").WhereNull("deleted_at")
	total, err := query.Count()
	if err != nil {
		return nil, 0, err
	}
	err = query.Order("id DESC").
		Limit((page-1)*pageSize, pageSize).
		Scan(&list)
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// Create 创建角色
func (d *adminRoleDao) Create(ctx context.Context, tx gdb.TX, roleName string) (int64, error) {
	var newId int64
	err := db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		id, err := tx.Model("admin_role").
			FieldsEx("id", "created_at", "updated_at").
			Data(gdb.Map{
				"role_name": roleName,
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

// UpdateById 根据ID更新角色名称
func (d *adminRoleDao) UpdateById(ctx context.Context, tx gdb.TX, id int64, roleName string) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Model("admin_role").
			Where("id", id).
			Data(gdb.Map{
				"role_name": roleName,
			}).
			Update()
		return err
	})
}

// GetDataByIds 根据多个ID获取数据（未删除）
func (d *adminRoleDao) GetDataByIds(ctx context.Context, ids []int64) ([]*entity.AdminRoleEntity, error) {
	if len(ids) == 0 {
		return []*entity.AdminRoleEntity{}, nil
	}
	var list []*entity.AdminRoleEntity
	err := d.db.Ctx(ctx).Model("admin_role").
		WhereNull("deleted_at").
		WhereIn("id", ids).
		Scan(&list)
	return list, err
}

// SoftDeleteByIds 软删除多个角色
func (d *adminRoleDao) SoftDeleteByIds(ctx context.Context, tx gdb.TX, ids []int64) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Model("admin_role").
			WhereIn("id", ids).
			Data(gdb.Map{
				"deleted_at": gdb.Raw("NOW()"),
			}).
			Update()
		return err
	})
}
