package dao

import (
	"context"
	"strings"

	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/shopspring/decimal"
)

// IAssetTypeDao 资产类型数据访问接口
type IAssetTypeDao interface {
	GetByIdForUpdate(ctx context.Context, tx gdb.TX, id int64) (*AssetType, error)
	GetBySymbol(ctx context.Context, symbol string) (*AssetType, error)
	Create(ctx context.Context, assetType *AssetType) (int64, error)
	IncrementBurnedAmount(ctx context.Context, tx gdb.TX, assetTypeId int64, amount decimal.Decimal) error
	IncrementTotalAmount(ctx context.Context, tx gdb.TX, assetTypeId int64, amount decimal.Decimal) error
	DecrementTotalAmount(ctx context.Context, tx gdb.TX, assetTypeId int64, amount decimal.Decimal) error
}

// AssetType 资产类型
type AssetType struct {
	Id           int64           `json:"id"`
	Code         string          `json:"code"`
	Name         string          `json:"name"`
	Symbol       string          `json:"symbol"`
	Decimals     int             `json:"decimals"`
	Icon         string          `json:"icon"`
	TotalAmount  decimal.Decimal `json:"totalAmount"`
	BurnedAmount decimal.Decimal `json:"burnedAmount"`
}

// assetTypeDao 资产类型数据访问实现
type assetTypeDao struct {
	db gdb.DB
}

// NewAssetTypeDao 创建资产类型数据访问实例
func NewAssetTypeDao() IAssetTypeDao {
	return &assetTypeDao{
		db: db.GetDB(),
	}
}

// GetByIdForUpdate 根据ID查询（悲观锁）
func (d *assetTypeDao) GetByIdForUpdate(ctx context.Context, tx gdb.TX, id int64) (*AssetType, error) {
	if tx == nil {
		return nil, gerror.New("transaction is required for GetByIdForUpdate")
	}

	var assetType AssetType
	err := tx.Model("asset_type").
		Where("id = ?", id).
		LockUpdate().
		Scan(&assetType)

	if err != nil {
		// 检查是否为锁等待超时
		if strings.Contains(err.Error(), "lock timeout") || strings.Contains(err.Error(), "deadlock") {
			return nil, gerror.New("资产类型被锁定，请稍后重试")
		}
		return nil, err
	}

	if assetType.Id == 0 {
		return nil, gerror.New("资产类型不存在")
	}

	return &assetType, nil
}

// GetBySymbol 根据符号查询
func (d *assetTypeDao) GetBySymbol(ctx context.Context, symbol string) (*AssetType, error) {
	var assetType AssetType
	err := d.db.Ctx(ctx).Model("asset_type").
		Where("symbol = ?", symbol).
		Scan(&assetType)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, nil
		}
		return nil, err
	}
	if assetType.Id == 0 {
		return nil, nil
	}
	return &assetType, nil
}

// Create 创建资产类型
func (d *assetTypeDao) Create(ctx context.Context, assetType *AssetType) (int64, error) {
	result, err := d.db.Ctx(ctx).Model("asset_type").Data(gdb.Map{
		"code":          assetType.Code,
		"name":          assetType.Name,
		"symbol":        assetType.Symbol,
		"decimals":      assetType.Decimals,
		"icon":          assetType.Icon,
		"total_amount":  0,
		"burned_amount": 0,
	}).InsertAndGetId()
	if err != nil {
		return 0, err
	}
	return result, nil
}

// IncrementBurnedAmount 增加销毁量（内部使用悲观锁）
func (d *assetTypeDao) IncrementBurnedAmount(ctx context.Context, tx gdb.TX, assetTypeId int64, amount decimal.Decimal) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		// 使用悲观锁查询（确保资产类型存在）
		_, err := d.GetByIdForUpdate(ctx, tx, assetTypeId)
		if err != nil {
			return err
		}

		// 累加销毁量
		_, err = tx.Model("asset_type").
			Where("id = ?", assetTypeId).
			Increment("burned_amount", amount)

		return err
	})
}

// IncrementTotalAmount 增加总发行量
func (d *assetTypeDao) IncrementTotalAmount(ctx context.Context, tx gdb.TX, assetTypeId int64, amount decimal.Decimal) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Model("asset_type").
			Where("id = ?", assetTypeId).
			Increment("total_amount", amount)
		return err
	})
}

// DecrementTotalAmount 减少总发行量（可选，销毁时使用）
func (d *assetTypeDao) DecrementTotalAmount(ctx context.Context, tx gdb.TX, assetTypeId int64, amount decimal.Decimal) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Model("asset_type").
			Where("id = ?", assetTypeId).
			Decrement("total_amount", amount)
		return err
	})
}
