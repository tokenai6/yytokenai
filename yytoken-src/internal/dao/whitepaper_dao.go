package dao

import (
	"context"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"
)

type IWhitepaperDao interface {
	Create(ctx context.Context, data map[string]interface{}) (int64, error)
	GetEnabledList(ctx context.Context) ([]*entity.WhitepaperEntity, error)
	GetList(ctx context.Context) ([]*entity.WhitepaperEntity, error)
	GetById(ctx context.Context, id int64) (*entity.WhitepaperEntity, error)
	UpdateById(ctx context.Context, id int64, data map[string]interface{}) error
	DeleteById(ctx context.Context, id int64) error
}

type whitepaperDao struct{}

func NewWhitepaperDao() IWhitepaperDao {
	return &whitepaperDao{}
}

func (d *whitepaperDao) Create(ctx context.Context, data map[string]interface{}) (int64, error) {
	result, err := db.GetDB().Model("whitepaper").Ctx(ctx).
		Data(data).
		InsertAndGetId()
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (d *whitepaperDao) GetEnabledList(ctx context.Context) ([]*entity.WhitepaperEntity, error) {
	var list []*entity.WhitepaperEntity
	err := db.GetDB().Model("whitepaper").Ctx(ctx).
		Where("is_enabled = ?", true).
		OrderAsc("sort_order").
		OrderAsc("id").
		Scan(&list)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (d *whitepaperDao) GetList(ctx context.Context) ([]*entity.WhitepaperEntity, error) {
	var list []*entity.WhitepaperEntity
	err := db.GetDB().Model("whitepaper").Ctx(ctx).
		OrderAsc("sort_order").
		OrderAsc("id").
		Scan(&list)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (d *whitepaperDao) GetById(ctx context.Context, id int64) (*entity.WhitepaperEntity, error) {
	var row entity.WhitepaperEntity
	err := db.GetDB().Model("whitepaper").Ctx(ctx).
		Where("id = ?", id).
		Scan(&row)
	if err != nil {
		return nil, err
	}
	if row.Id == 0 {
		return nil, nil
	}
	return &row, nil
}

func (d *whitepaperDao) UpdateById(ctx context.Context, id int64, data map[string]interface{}) error {
	_, err := db.GetDB().Model("whitepaper").Ctx(ctx).
		Where("id = ?", id).
		Data(data).
		Update()
	return err
}

func (d *whitepaperDao) DeleteById(ctx context.Context, id int64) error {
	_, err := db.GetDB().Model("whitepaper").Ctx(ctx).
		Where("id = ?", id).
		Delete()
	return err
}
