package repository

import (
	"context"

	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
)

// IAdminRepository 管理员仓储接口（业务逻辑层）
type IAdminRepository interface {
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

// adminRepository 管理员仓储实现
type adminRepository struct {
	adminDao dao.IAdminDao
}

// NewAdminRepository 创建管理员仓储实例
func NewAdminRepository() IAdminRepository {
	return &adminRepository{
		adminDao: dao.NewAdminDao(),
	}
}

// GetByUsername 根据用户名获取管理员
func (r *adminRepository) GetByUsername(ctx context.Context, username string) (*entity.AdminInfoEntity, error) {
	return r.adminDao.GetByUsername(ctx, username)
}

// GetByWalletAddress 根据钱包地址获取管理员
func (r *adminRepository) GetByWalletAddress(ctx context.Context, walletAddress string) (*entity.AdminInfoEntity, error) {
	return r.adminDao.GetByWalletAddress(ctx, walletAddress)
}

// GetById 根据ID获取管理员
func (r *adminRepository) GetById(ctx context.Context, id int64) (*entity.AdminInfoEntity, error) {
	return r.adminDao.GetById(ctx, id)
}

// UpdateLastLoginAt 更新最后登录时间
func (r *adminRepository) UpdateLastLoginAt(ctx context.Context, id int64) error {
	return r.adminDao.UpdateLastLoginAt(ctx, id)
}

// GetList 分页获取管理员列表（可按用户名模糊查询）
func (r *adminRepository) GetList(ctx context.Context, page, pageSize int, username string) ([]*entity.AdminInfoEntity, int, error) {
	return r.adminDao.GetList(ctx, page, pageSize, username)
}

// Create 创建管理员
func (r *adminRepository) Create(ctx context.Context, admin *entity.AdminInfoEntity) error {
	return r.adminDao.Create(ctx, admin)
}

// Update 更新管理员信息
func (r *adminRepository) Update(ctx context.Context, id int64, data map[string]interface{}) error {
	return r.adminDao.Update(ctx, id, data)
}

// GetDataByIds 根据ID列表获取管理员列表
func (r *adminRepository) GetDataByIds(ctx context.Context, ids []int64) ([]*entity.AdminInfoEntity, error) {
	return r.adminDao.GetDataByIds(ctx, ids)
}
