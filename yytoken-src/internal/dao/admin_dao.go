package dao

import (
	"context"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// IAdminDao 管理员数据访问接口（只包含基础增删改查）
type IAdminDao interface {
	// GetByUsername 根据用户名获取管理员
	GetByUsername(ctx context.Context, username string) (*entity.AdminInfoEntity, error)

	// GetByWalletAddress 根据钱包地址获取管理员
	GetByWalletAddress(ctx context.Context, walletAddress string) (*entity.AdminInfoEntity, error)

	// GetById 根据ID获取管理员
	GetById(ctx context.Context, id int64) (*entity.AdminInfoEntity, error)

	// UpdateLastLoginAt 更新最后登录时间
	UpdateLastLoginAt(ctx context.Context, id int64) error

	// GetList 分页获取管理员列表（可按用户名模糊查询）
	GetList(ctx context.Context, page, pageSize int, username string) ([]*entity.AdminInfoEntity, int, error)

	// Create 创建管理员
	Create(ctx context.Context, admin *entity.AdminInfoEntity) error

	// Update 更新管理员信息
	Update(ctx context.Context, id int64, data map[string]interface{}) error

	// GetDataByIds 根据ID列表获取管理员列表
	GetDataByIds(ctx context.Context, ids []int64) ([]*entity.AdminInfoEntity, error)
}

// adminDao 管理员数据访问实现
type adminDao struct {
	db gdb.DB
}

// NewAdminDao 创建管理员数据访问实例
func NewAdminDao() IAdminDao {
	return &adminDao{
		db: db.GetDB(),
	}
}

// GetByUsername 根据用户名获取管理员
func (d *adminDao) GetByUsername(ctx context.Context, username string) (*entity.AdminInfoEntity, error) {
	var admin entity.AdminInfoEntity
	err := d.db.Ctx(ctx).Model("admin_info").
		Where("username", username).
		Scan(&admin)
	if err != nil {
		return nil, err
	}
	if admin.Id == 0 {
		return nil, nil
	}
	return &admin, nil
}

// GetByWalletAddress 根据钱包地址获取管理员
func (d *adminDao) GetByWalletAddress(ctx context.Context, walletAddress string) (*entity.AdminInfoEntity, error) {
	var admin entity.AdminInfoEntity
	err := d.db.Ctx(ctx).Model("admin_info").
		Where("wallet_address", walletAddress).
		Scan(&admin)
	if err != nil {
		return nil, err
	}
	if admin.Id == 0 {
		return nil, nil
	}
	return &admin, nil
}

// GetById 根据ID获取管理员
func (d *adminDao) GetById(ctx context.Context, id int64) (*entity.AdminInfoEntity, error) {
	var admin entity.AdminInfoEntity
	err := d.db.Ctx(ctx).Model("admin_info").
		Where("id", id).
		Scan(&admin)
	if err != nil {
		return nil, err
	}
	if admin.Id == 0 {
		return nil, nil
	}
	return &admin, nil
}

// UpdateLastLoginAt 更新最后登录时间
func (d *adminDao) UpdateLastLoginAt(ctx context.Context, id int64) error {
	_, err := d.db.Ctx(ctx).Model("admin_info").
		Where("id", id).
		Data(gdb.Map{
			"last_login_at": gdb.Raw("NOW()"),
		}).
		Update()
	return err
}

// GetList 分页获取管理员列表（可按用户名模糊查询）
func (d *adminDao) GetList(ctx context.Context, page, pageSize int, username string) ([]*entity.AdminInfoEntity, int, error) {
	var list []*entity.AdminInfoEntity
	m := d.db.Ctx(ctx).Model("admin_info")
	if username != "" {
		m = m.WhereLike("username", "%"+username+"%")
	}
	total, err := m.Count()
	if err != nil {
		return nil, 0, err
	}
	err = m.Order("id DESC").
		Limit((page-1)*pageSize, pageSize).
		Scan(&list)
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// Create 创建管理员
func (d *adminDao) Create(ctx context.Context, admin *entity.AdminInfoEntity) error {
	data := gdb.Map{
		"username":       admin.Username,
		"wallet_address": admin.WalletAddress,
		"role":           admin.Role,
		"role_id":        admin.RoleId,
		"status":         admin.Status,
	}
	if admin.Email != "" {
		data["email"] = admin.Email
	}

	result, err := d.db.Ctx(ctx).Model("admin_info").
		Data(data).
		InsertAndGetId()
	if err != nil {
		return err
	}
	admin.Id = result
	return nil
}

// Update 更新管理员信息
func (d *adminDao) Update(ctx context.Context, id int64, data map[string]interface{}) error {
	_, err := d.db.Ctx(ctx).Model("admin_info").
		Where("id", id).
		Data(data).
		Update()
	return err
}

// GetDataByIds 根据ID列表获取管理员列表
func (d *adminDao) GetDataByIds(ctx context.Context, ids []int64) ([]*entity.AdminInfoEntity, error) {
	if len(ids) == 0 {
		return []*entity.AdminInfoEntity{}, nil
	}

	const batchSize = 30000
	var (
		admins []*entity.AdminInfoEntity
		result = make([]*entity.AdminInfoEntity, 0, len(ids))
	)

	for start := 0; start < len(ids); start += batchSize {
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}

		admins = admins[:0]
		if err := d.db.Model("admin_info").Ctx(ctx).WhereIn("id", ids[start:end]).Scan(&admins); err != nil {
			return nil, err
		}
		if len(admins) > 0 {
			result = append(result, admins...)
		}
	}

	return result, nil
}
