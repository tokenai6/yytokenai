package dao

import (
	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// UserDepositAddressDao 用户充值地址DAO
type UserDepositAddressDao struct {
	db gdb.DB
}

// NewUserDepositAddressDao 创建用户充值地址DAO
func NewUserDepositAddressDao() *UserDepositAddressDao {
	return &UserDepositAddressDao{
		db: db.GetDB(),
	}
}

// GetByUserId 根据用户ID查询地址
func (d *UserDepositAddressDao) GetByUserId(ctx context.Context, userID int64) (*entity.UserDepositAddressEntity, error) {
	// 使用All方法获取结果，然后手动转换类型
	result, err := d.db.Model("user_deposit_address").Where("user_id = ? AND is_valid = ?", userID, true).All()
	if err != nil {
		if g.IsNil(err) {
			return nil, nil
		}
		return nil, err
	}

	if len(result) == 0 {
		return nil, nil
	}

	// 手动转换数据
	record := result[0]
	entity := &entity.UserDepositAddressEntity{
		UserID:  record["user_id"].Int64(),
		ChainID: record["chain_id"].String(),
		Address: record["address"].String(),
		IsValid: record["is_valid"].Bool(),
	}

	// 设置基础字段
	entity.Id = record["id"].Int64()
	entity.CreatedAt = record["created_at"].Time()
	entity.UpdatedAt = record["updated_at"].Time()

	return entity, nil
}

// GetByAddress 根据地址查询记录
func (d *UserDepositAddressDao) GetByAddress(ctx context.Context, address string) (*entity.UserDepositAddressEntity, error) {
	// 使用All方法获取结果，然后手动转换类型
	result, err := d.db.Model("user_deposit_address").Where("address = ?", address).All()
	if err != nil {
		if g.IsNil(err) {
			return nil, nil
		}
		return nil, err
	}

	if len(result) == 0 {
		return nil, nil
	}

	// 手动转换数据
	record := result[0]
	entity := &entity.UserDepositAddressEntity{
		UserID:  record["user_id"].Int64(),
		ChainID: record["chain_id"].String(),
		Address: record["address"].String(),
		IsValid: record["is_valid"].Bool(),
	}

	// 设置基础字段
	entity.Id = record["id"].Int64()
	entity.CreatedAt = record["created_at"].Time()
	entity.UpdatedAt = record["updated_at"].Time()

	return entity, nil
}

// Create 创建地址记录
func (d *UserDepositAddressDao) Create(ctx context.Context, entity *entity.UserDepositAddressEntity) error {
	_, err := d.db.Model("user_deposit_address").Data(entity).FieldsEx("id", "created_at", "updated_at").Insert()
	return err
}

// UpdateIsValid 更新有效状态
func (d *UserDepositAddressDao) UpdateIsValid(ctx context.Context, id int64, isValid bool) error {
	_, err := d.db.Model("user_deposit_address").Where("id = ?", id).Data(map[string]interface{}{"is_valid": isValid}).Update()
	return err
}
