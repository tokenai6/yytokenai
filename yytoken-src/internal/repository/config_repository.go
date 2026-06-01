package repository

import (
	"context"

	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
)

// IConfigRepository 系统配置仓储接口（业务逻辑层）
type IConfigRepository interface {
	// GetByKeyName 根据键名获取配置
	GetByKeyName(ctx context.Context, keyName string) (*entity.ConfigEntity, error)

	// GetById 根据ID获取配置详情
	GetById(ctx context.Context, id int64) (*entity.ConfigEntity, error)

	// GetList 获取配置列表（分页）
	GetList(ctx context.Context, keyName string, page, pageSize int) ([]*entity.ConfigEntity, int, error)

	// UpdateById 根据ID更新配置
	UpdateById(ctx context.Context, tx gdb.TX, id int64, keyValue, description string) error
}

// configRepository 系统配置仓储实现
type configRepository struct {
	configDao dao.IConfigDao
}

// NewConfigRepository 创建系统配置仓储实例
func NewConfigRepository() IConfigRepository {
	return &configRepository{
		configDao: dao.NewConfigDao(),
	}
}

// GetByKeyName 根据键名获取配置
func (r *configRepository) GetByKeyName(ctx context.Context, keyName string) (*entity.ConfigEntity, error) {
	return r.configDao.GetByKeyName(ctx, keyName)
}

// GetById 根据ID获取配置详情
func (r *configRepository) GetById(ctx context.Context, id int64) (*entity.ConfigEntity, error) {
	return r.configDao.GetById(ctx, id)
}

// GetList 获取配置列表（分页）
func (r *configRepository) GetList(ctx context.Context, keyName string, page, pageSize int) ([]*entity.ConfigEntity, int, error) {
	return r.configDao.GetList(ctx, keyName, page, pageSize)
}

// UpdateById 根据ID更新配置
func (r *configRepository) UpdateById(ctx context.Context, tx gdb.TX, id int64, keyValue, description string) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		// 检查配置是否存在
		config, err := r.configDao.GetById(ctx, id)
		if err != nil {
			return err
		}
		if config == nil {
			return gerror.New("配置不存在")
		}

		// 更新配置
		data := map[string]interface{}{
			"key_value": keyValue,
		}
		if description != "" {
			data["description"] = description
		}

		return r.configDao.UpdateById(ctx, tx, id, data)
	})
}
