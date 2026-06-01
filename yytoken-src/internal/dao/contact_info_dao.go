package dao

import (
	"context"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"
)

type IContactInfoDao interface {
	GetEnabledList(ctx context.Context) ([]*entity.ContactInfoEntity, error)
	GetList(ctx context.Context) ([]*entity.ContactInfoEntity, error)
	GetById(ctx context.Context, id int64) (*entity.ContactInfoEntity, error)
	UpdateById(ctx context.Context, id int64, data map[string]interface{}) error
}

type contactInfoDao struct{}

func NewContactInfoDao() IContactInfoDao {
	return &contactInfoDao{}
}

func (d *contactInfoDao) GetEnabledList(ctx context.Context) ([]*entity.ContactInfoEntity, error) {
	var list []*entity.ContactInfoEntity
	err := db.GetDB().Model("contact_info").Ctx(ctx).
		Where("is_enabled = ?", true).
		OrderAsc("sort_order").
		OrderAsc("id").
		Scan(&list)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (d *contactInfoDao) GetList(ctx context.Context) ([]*entity.ContactInfoEntity, error) {
	var list []*entity.ContactInfoEntity
	err := db.GetDB().Model("contact_info").Ctx(ctx).
		OrderAsc("sort_order").
		OrderAsc("id").
		Scan(&list)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (d *contactInfoDao) GetById(ctx context.Context, id int64) (*entity.ContactInfoEntity, error) {
	var row entity.ContactInfoEntity
	err := db.GetDB().Model("contact_info").Ctx(ctx).
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

func (d *contactInfoDao) UpdateById(ctx context.Context, id int64, data map[string]interface{}) error {
	_, err := db.GetDB().Model("contact_info").Ctx(ctx).
		Where("id = ?", id).
		Data(data).
		Update()
	return err
}
