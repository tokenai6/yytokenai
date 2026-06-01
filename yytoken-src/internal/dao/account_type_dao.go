package dao

import (
	"context"

	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
)

// IAccountTypeDao 账户类型数据访问接口
type IAccountTypeDao interface {
	GetById(ctx context.Context, id int64) (*AccountType, error)
	GetSymbolById(ctx context.Context, id int64) (string, error)
	GetAllBySymbol(ctx context.Context, symbol string) ([]*AccountType, error)
	GetIdByTypeAndSymbol(ctx context.Context, accountType, symbol string) (int64, error)
	Create(ctx context.Context, accountType *AccountType) (int64, error)
}

// AccountType 账户类型
type AccountType struct {
	Id          int64  `json:"id"`
	AssetTypeId int64  `json:"assetTypeId"`
	Symbol      string `json:"symbol"`
	Name        string `json:"name"`
	Type        string `json:"type"`
}

// accountTypeDao 账户类型数据访问实现
type accountTypeDao struct {
	db gdb.DB
}

// NewAccountTypeDao 创建账户类型数据访问实例
func NewAccountTypeDao() IAccountTypeDao {
	return &accountTypeDao{
		db: db.GetDB(),
	}
}

// GetById 根据ID查询账户类型
func (d *accountTypeDao) GetById(ctx context.Context, id int64) (*AccountType, error) {
	var accountType AccountType
	err := d.db.Ctx(ctx).Model("account_type").
		Where("id = ?", id).
		Scan(&accountType)
	if err != nil {
		return nil, err
	}
	if accountType.Id == 0 {
		return nil, gerror.Newf("账户类型ID %d 不存在", id)
	}
	return &accountType, nil
}

// GetSymbolById 根据账户类型ID获取资产符号
func (d *accountTypeDao) GetSymbolById(ctx context.Context, id int64) (string, error) {
	accountType, err := d.GetById(ctx, id)
	if err != nil {
		return "", err
	}
	return accountType.Symbol, nil
}

// GetAllBySymbol 根据资产符号查询所有账户类型
func (d *accountTypeDao) GetAllBySymbol(ctx context.Context, symbol string) ([]*AccountType, error) {
	var accountTypes []*AccountType
	err := d.db.Ctx(ctx).Model("account_type").
		Where("symbol = ?", symbol).
		OrderAsc("id").
		Scan(&accountTypes)
	if err != nil {
		return nil, err
	}
	return accountTypes, nil
}

// GetIdByTypeAndSymbol 根据账户类型和资产符号查询账户类型ID
func (d *accountTypeDao) GetIdByTypeAndSymbol(ctx context.Context, accountType, symbol string) (int64, error) {
	result, err := d.db.Ctx(ctx).Model("account_type").
		Where("type = ? AND symbol = ?", accountType, symbol).
		Fields("id").
		Value()
	if err != nil {
		return 0, err
	}
	if result == nil || result.IsNil() {
		return 0, nil
	}
	return result.Int64(), nil
}

// Create 创建账户类型
func (d *accountTypeDao) Create(ctx context.Context, accountType *AccountType) (int64, error) {
	result, err := d.db.Ctx(ctx).Model("account_type").Data(gdb.Map{
		"asset_type_id": accountType.AssetTypeId,
		"symbol":        accountType.Symbol,
		"name":          accountType.Name,
		"type":          accountType.Type,
	}).InsertAndGetId()
	if err != nil {
		return 0, err
	}
	return result, nil
}
