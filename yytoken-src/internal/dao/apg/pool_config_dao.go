package apg

import (
	"context"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

type IPoolConfigDao interface {
	GetByPoolType(ctx context.Context, poolType int) (*entity.ApgPoolConfig, error)
	GetAllEnabled(ctx context.Context) ([]*entity.ApgPoolConfig, error)
	UpdateStatus(ctx context.Context, poolType int, status int) error
}

type poolConfigDao struct{}

var PoolConfig IPoolConfigDao = &poolConfigDao{}

func (d *poolConfigDao) GetByPoolType(ctx context.Context, poolType int) (*entity.ApgPoolConfig, error) {
	var config entity.ApgPoolConfig
	err := db.GetDB().Model("apg_mint_pool_config").
		Where("pool_type", poolType).
		Scan(&config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (d *poolConfigDao) GetAllEnabled(ctx context.Context) ([]*entity.ApgPoolConfig, error) {
	var configs []*entity.ApgPoolConfig
	err := db.GetDB().Model("apg_mint_pool_config").
		Where("status", 1).
		Order("pool_type ASC").
		Scan(&configs)
	return configs, err
}

func (d *poolConfigDao) UpdateStatus(ctx context.Context, poolType int, status int) error {
	_, err := db.GetDB().Model("apg_mint_pool_config").
		Where("pool_type", poolType).
		Update(gdb.Map{"status": status})
	return err
}
