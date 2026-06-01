package dao

import (
	"context"
	"database/sql"
	"errors"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// ITokenConfigDao 代币配置数据访问接口
type ITokenConfigDao interface {
	// GetBySymbol 根据代币符号获取配置
	GetBySymbol(ctx context.Context, symbol string) (*entity.TokenConfigEntity, error)

	// GetAllEnabled 获取所有启用的代币配置
	GetAllEnabled(ctx context.Context) ([]*entity.TokenConfigEntity, error)

	// GetAll 获取所有代币配置（含禁用），按 sort_order 升序
	GetAll(ctx context.Context) ([]*entity.TokenConfigEntity, error)

	// Create 创建代币配置
	Create(ctx context.Context, tx gdb.TX, config *entity.TokenConfigEntity) error

	// UpdateById 根据ID更新配置
	UpdateById(ctx context.Context, tx gdb.TX, id int64, data map[string]interface{}) error

	// SearchBySymbol 根据代币符号模糊搜索（忽略大小写）
	SearchBySymbol(ctx context.Context, symbol string) ([]*entity.TokenConfigEntity, error)

	// GetTicketTokens 获取所有可作为拼团门票的代币配置
	GetTicketTokens(ctx context.Context) ([]*entity.TokenConfigEntity, error)
}

// tokenConfigDao 代币配置数据访问实现
type tokenConfigDao struct {
	db gdb.DB
}

// NewTokenConfigDao 创建代币配置数据访问实例
func NewTokenConfigDao() ITokenConfigDao {
	return &tokenConfigDao{
		db: db.GetDB(),
	}
}

// GetBySymbol 根据代币符号获取配置
func (d *tokenConfigDao) GetBySymbol(ctx context.Context, symbol string) (*entity.TokenConfigEntity, error) {
	var config entity.TokenConfigEntity
	err := d.db.Model("token_config").Ctx(ctx).
		Where("symbol = ?", symbol).
		Where("is_enabled = ?", true).
		Scan(&config)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if config.Id == 0 {
		return nil, nil
	}
	return &config, nil
}

// GetAllEnabled 获取所有启用的代币配置
func (d *tokenConfigDao) GetAllEnabled(ctx context.Context) ([]*entity.TokenConfigEntity, error) {
	var configs []*entity.TokenConfigEntity
	err := d.db.Model("token_config").Ctx(ctx).
		Where("is_enabled = ?", true).
		OrderAsc("sort_order").
		Scan(&configs)
	if err != nil {
		return nil, err
	}
	return configs, nil
}

// GetAll 获取所有代币配置（含禁用），按 sort_order 升序
func (d *tokenConfigDao) GetAll(ctx context.Context) ([]*entity.TokenConfigEntity, error) {
	var configs []*entity.TokenConfigEntity
	err := d.db.Model("token_config").Ctx(ctx).
		OrderAsc("sort_order").
		Scan(&configs)
	if err != nil {
		return nil, err
	}
	return configs, nil
}

// Create 创建代币配置
func (d *tokenConfigDao) Create(ctx context.Context, tx gdb.TX, config *entity.TokenConfigEntity) error {
	model := d.db.Model("token_config")
	if tx != nil {
		model = tx.Model("token_config")
	}

	result, err := model.Ctx(ctx).
		FieldsEx("id", "created_at", "updated_at").
		Data(config).
		InsertAndGetId()
	if err != nil {
		return err
	}
	config.Id = result
	return nil
}

// UpdateById 根据ID更新配置
func (d *tokenConfigDao) UpdateById(ctx context.Context, tx gdb.TX, id int64, data map[string]interface{}) error {
	model := d.db.Model("token_config")
	if tx != nil {
		model = tx.Model("token_config")
	}

	_, err := model.Ctx(ctx).
		Where("id = ?", id).
		Data(data).
		Update()
	return err
}

// GetTicketTokens 获取所有可作为拼团门票的代币配置
func (d *tokenConfigDao) GetTicketTokens(ctx context.Context) ([]*entity.TokenConfigEntity, error) {
	var configs []*entity.TokenConfigEntity
	err := d.db.Model("token_config").Ctx(ctx).
		Where("is_enabled = ?", true).
		Where("is_ticket_token = ?", true).
		OrderAsc("sort_order").
		Scan(&configs)
	if err != nil {
		return nil, err
	}
	return configs, nil
}

// SearchBySymbol 根据代币符号模糊搜索（忽略大小写）
func (d *tokenConfigDao) SearchBySymbol(ctx context.Context, symbol string) ([]*entity.TokenConfigEntity, error) {
	var configs []*entity.TokenConfigEntity
	err := d.db.Model("token_config").Ctx(ctx).
		Where("is_enabled = ?", true).
		Where("LOWER(symbol) LIKE LOWER(?)", "%"+symbol+"%").
		OrderAsc("sort_order").
		Scan(&configs)
	if err != nil {
		return nil, err
	}
	return configs, nil
}
