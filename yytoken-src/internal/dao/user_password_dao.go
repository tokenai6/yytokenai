package dao

import (
	"context"
	"database/sql"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// IUserPasswordDao 用户密码数据访问接口
type IUserPasswordDao interface {
	// Upsert 创建或更新密码
	Upsert(ctx context.Context, userID int64, passwordHash string) error
	// GetByUserID 根据用户ID获取密码
	GetByUserID(ctx context.Context, userID int64) (*entity.UserPasswordEntity, error)
	// ExistsByUserID 检查用户是否设置了密码
	ExistsByUserID(ctx context.Context, userID int64) (bool, error)
}

// userPasswordDao 用户密码数据访问实现
type userPasswordDao struct {
	db gdb.DB
}

// NewUserPasswordDao 创建用户密码数据访问实例
func NewUserPasswordDao() IUserPasswordDao {
	return &userPasswordDao{
		db: db.GetDB(),
	}
}

// Upsert 创建或更新密码
func (d *userPasswordDao) Upsert(ctx context.Context, userID int64, passwordHash string) error {
	_, err := d.db.Exec(
		ctx,
		`INSERT INTO user_password (user_id, password_hash, created_at, updated_at)
		 VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 ON CONFLICT (user_id)
		 DO UPDATE SET password_hash = EXCLUDED.password_hash, updated_at = CURRENT_TIMESTAMP`,
		userID,
		passwordHash,
	)
	return err
}

// GetByUserID 根据用户ID获取密码
func (d *userPasswordDao) GetByUserID(ctx context.Context, userID int64) (*entity.UserPasswordEntity, error) {
	var entity entity.UserPasswordEntity
	err := d.db.Model("user_password").Ctx(ctx).Where("user_id", userID).Scan(&entity)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if entity.ID == 0 {
		return nil, nil
	}
	return &entity, nil
}

// ExistsByUserID 检查用户是否设置了密码
func (d *userPasswordDao) ExistsByUserID(ctx context.Context, userID int64) (bool, error) {
	count, err := d.db.Model("user_password").Ctx(ctx).Where("user_id", userID).Count()
	return count > 0, err
}
