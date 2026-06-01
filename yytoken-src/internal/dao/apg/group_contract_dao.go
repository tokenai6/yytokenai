package apg

import (
	"context"
	"strings"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/os/gtime"
)

type IGroupContractDao interface {
	GetAll(ctx context.Context) ([]*entity.ApgGroupContract, error)
	GetByAddress(ctx context.Context, address string) (*entity.ApgGroupContract, error)
	GetActive(ctx context.Context) (*entity.ApgGroupContract, error)
	GetLatestTwo(ctx context.Context) ([]*entity.ApgGroupContract, error)
	Create(ctx context.Context, contract *entity.ApgGroupContract) error
	SetActive(ctx context.Context, id int64) error
	DeactivateAll(ctx context.Context) error
}

type groupContractDao struct{}

var GroupContract IGroupContractDao = &groupContractDao{}

func (d *groupContractDao) GetAll(ctx context.Context) ([]*entity.ApgGroupContract, error) {
	var contracts []*entity.ApgGroupContract
	err := db.GetDB().Model("apg_group_contract").
		Order("id DESC").
		Scan(&contracts)
	return contracts, err
}

func (d *groupContractDao) GetByAddress(ctx context.Context, address string) (*entity.ApgGroupContract, error) {
	var contract *entity.ApgGroupContract
	err := db.GetDB().Model("apg_group_contract").
		Where("LOWER(contract_address) = LOWER(?)", address).
		Scan(&contract)
	return contract, err
}

func (d *groupContractDao) GetActive(ctx context.Context) (*entity.ApgGroupContract, error) {
	var contract *entity.ApgGroupContract
	err := db.GetDB().Model("apg_group_contract").
		Where("is_active", true).
		Limit(1).
		Scan(&contract)
	return contract, err
}

func (d *groupContractDao) GetLatestTwo(ctx context.Context) ([]*entity.ApgGroupContract, error) {
	var contracts []*entity.ApgGroupContract
	err := db.GetDB().Model("apg_group_contract").
		Order("created_at DESC").
		Limit(2).
		Scan(&contracts)
	return contracts, err
}

func (d *groupContractDao) Create(ctx context.Context, contract *entity.ApgGroupContract) error {
	now := gtime.Now()
	data := map[string]interface{}{
		"contract_address": strings.ToLower(contract.ContractAddress),
		"is_active":        contract.IsActive,
		"activated_at":     now,
		"created_at":       now,
		"updated_at":       now,
	}

	id, err := db.GetDB().Model("apg_group_contract").InsertAndGetId(data)
	if err != nil {
		return err
	}
	contract.Id = id
	contract.ActivatedAt = now
	contract.CreatedAt = now
	contract.UpdatedAt = now
	return nil
}

func (d *groupContractDao) SetActive(ctx context.Context, id int64) error {
	_, err := db.GetDB().Model("apg_group_contract").
		Where("id", id).
		Update(map[string]interface{}{
			"is_active":  true,
			"updated_at": gtime.Now(),
		})
	return err
}

func (d *groupContractDao) DeactivateAll(ctx context.Context) error {
	_, err := db.GetDB().Model("apg_group_contract").
		Where("is_active", true).
		Update(map[string]interface{}{
			"is_active":      false,
			"deactivated_at": gtime.Now(),
			"updated_at":     gtime.Now(),
		})
	return err
}
