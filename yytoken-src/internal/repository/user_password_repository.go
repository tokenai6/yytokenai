package repository

import (
	"context"

	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/errors/gerror"
)

// IUserPasswordRepository 用户密码仓储接口
type IUserPasswordRepository interface {
	// SetPassword 设置用户密码
	SetPassword(ctx context.Context, userID int64, passwordHash string) error
	// GetPassword 获取用户密码
	GetPassword(ctx context.Context, userID int64) (*entity.UserPasswordEntity, error)
	// HasPassword 检查用户是否设置了密码
	HasPassword(ctx context.Context, userID int64) (bool, error)
}

// userPasswordRepository 用户密码仓储实现
type userPasswordRepository struct {
	passwordDao dao.IUserPasswordDao
}

// NewUserPasswordRepository 创建用户密码仓储实例
func NewUserPasswordRepository() IUserPasswordRepository {
	return &userPasswordRepository{
		passwordDao: dao.NewUserPasswordDao(),
	}
}

// SetPassword 设置用户密码
func (r *userPasswordRepository) SetPassword(ctx context.Context, userID int64, passwordHash string) error {
	return r.passwordDao.Upsert(ctx, userID, passwordHash)
}

// GetPassword 获取用户密码
func (r *userPasswordRepository) GetPassword(ctx context.Context, userID int64) (*entity.UserPasswordEntity, error) {
	return r.passwordDao.GetByUserID(ctx, userID)
}

// HasPassword 检查用户是否设置了密码
func (r *userPasswordRepository) HasPassword(ctx context.Context, userID int64) (bool, error) {
	return r.passwordDao.ExistsByUserID(ctx, userID)
}

// VerifyUserPassword 验证用户密码（未设置密码则拒绝）
func VerifyUserPassword(ctx context.Context, repo IUserPasswordRepository, userID int64, password string) error {
	hasPassword, err := repo.HasPassword(ctx, userID)
	if err != nil {
		return gerror.Wrap(err, "check password failed")
	}
	if !hasPassword {
		return gerror.New("please set password first")
	}
	if password == "" {
		return gerror.New("password is required")
	}
	passwordEntity, err := repo.GetPassword(ctx, userID)
	if err != nil {
		return gerror.Wrap(err, "get password failed")
	}
	if passwordEntity == nil {
		return gerror.New("password not set")
	}
	if !utils.VerifyPassword(password, passwordEntity.PasswordHash) {
		return gerror.New("invalid password")
	}
	return nil
}