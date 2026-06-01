package dao

import (
	"context"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// IConfigDao 配置数据访问接口（只包含基础增删改查）
type IConfigDao interface {
	// GetByKeyName 根据键名获取配置
	GetByKeyName(ctx context.Context, keyName string) (*entity.ConfigEntity, error)

	// GetById 根据ID获取配置
	GetById(ctx context.Context, id int64) (*entity.ConfigEntity, error)

	// GetList 获取配置列表（分页）
	GetList(ctx context.Context, keyName string, page, pageSize int) ([]*entity.ConfigEntity, int, error)

	// UpdateById 根据ID更新配置
	UpdateById(ctx context.Context, tx gdb.TX, id int64, data map[string]interface{}) error
}

// configDao 配置数据访问实现
type configDao struct {
	db gdb.DB
}

// NewConfigDao 创建配置数据访问实例
func NewConfigDao() IConfigDao {
	return &configDao{
		db: db.GetDB(),
	}
}

// GetByKeyName 根据键名获取配置
func (d *configDao) GetByKeyName(ctx context.Context, keyName string) (*entity.ConfigEntity, error) {
	var config entity.ConfigEntity
	err := d.db.Ctx(ctx).Raw("SELECT key AS key_name, value AS key_value, description, created_at, updated_at FROM system_config WHERE key = ?", keyName).Scan(&config)
	if err != nil {
		return nil, err
	}
	if config.KeyName == "" {
		return nil, nil
	}
	return &config, nil
}

// GetById 根据ID获取配置（system_config 无主键id，通过ROW_NUMBER模拟）
func (d *configDao) GetById(ctx context.Context, id int64) (*entity.ConfigEntity, error) {
	var config entity.ConfigEntity
	err := d.db.Ctx(ctx).Raw(`
		WITH ranked AS (
			SELECT ROW_NUMBER() OVER (ORDER BY key ASC) AS id, key, value, description, created_at, updated_at
			FROM system_config
		)
		SELECT id, key AS key_name, value AS key_value, description, created_at, updated_at
		FROM ranked
		WHERE id = ?
	`, id).Scan(&config)
	if err != nil {
		return nil, err
	}
	if config.KeyName == "" {
		return nil, nil
	}
	return &config, nil
}

// GetList 获取配置列表（分页）
func (d *configDao) GetList(ctx context.Context, keyName string, page, pageSize int) ([]*entity.ConfigEntity, int, error) {
	var total int
	countSql := `
		WITH ranked AS (
			SELECT ROW_NUMBER() OVER (ORDER BY key ASC) AS id, key
			FROM system_config
		)
		SELECT COUNT(*) FROM ranked WHERE 1=1
	`
	listSql := `
		WITH ranked AS (
			SELECT ROW_NUMBER() OVER (ORDER BY key ASC) AS id, key, value, description, created_at, updated_at
			FROM system_config
		)
		SELECT id, key AS key_name, value AS key_value, description, created_at, updated_at
		FROM ranked
		WHERE 1=1
	`
	args := []interface{}{}

	if keyName != "" {
		countSql += " AND key LIKE ?"
		listSql += " AND key LIKE ?"
		args = append(args, "%"+keyName+"%")
	}

	listSql += " ORDER BY key DESC LIMIT ? OFFSET ?"

	record, err := d.db.Ctx(ctx).Raw(countSql, args...).One()
	if err != nil {
		return nil, 0, err
	}
	total = record["count"].Int()

	queryArgs := append([]interface{}{}, args...)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	var list []*entity.ConfigEntity
	err = d.db.Ctx(ctx).Raw(listSql, queryArgs...).Scan(&list)

	return list, total, err
}

// UpdateById 根据ID更新配置
func (d *configDao) UpdateById(ctx context.Context, tx gdb.TX, id int64, data map[string]interface{}) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Exec(`
			WITH ranked AS (
				SELECT ROW_NUMBER() OVER (ORDER BY key ASC) AS id, key
				FROM system_config
			)
			UPDATE system_config
			SET value = ?, description = ?, updated_at = CURRENT_TIMESTAMP
			WHERE key = (SELECT key FROM ranked WHERE id = ?)
		`, data["key_value"], data["description"], id)
		return err
	})
}
