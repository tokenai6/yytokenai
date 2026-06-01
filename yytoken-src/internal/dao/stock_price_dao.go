package dao

import (
	"context"
	"time"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// IStockPriceDao 股票价格数据访问接口
type IStockPriceDao interface {
	// Create 创建股票价格记录
	Create(ctx context.Context, tx gdb.TX, price *entity.StockPriceEntity) error

	// GetLatestBySymbol 获取指定股票的最新价格
	GetLatestBySymbol(ctx context.Context, symbol string) (*entity.StockPriceEntity, error)

	// GetByTimeRange 根据时间范围获取价格记录
	GetByTimeRange(ctx context.Context, symbol string, startTime, endTime time.Time) ([]*entity.StockPriceEntity, error)

	// GetLatestN 获取最新的N条价格记录
	GetLatestN(ctx context.Context, symbol string, limit int) ([]*entity.StockPriceEntity, error)
}

// stockPriceDao 股票价格数据访问实现
type stockPriceDao struct {
	db gdb.DB
}

// NewStockPriceDao 创建股票价格数据访问实例
func NewStockPriceDao() IStockPriceDao {
	return &stockPriceDao{
		db: db.GetDB(),
	}
}

// Create 创建股票价格记录
func (d *stockPriceDao) Create(ctx context.Context, tx gdb.TX, price *entity.StockPriceEntity) error {
	model := d.db.Model("stock_price")
	if tx != nil {
		model = tx.Model("stock_price")
	}

	result, err := model.Ctx(ctx).
		FieldsEx("id", "created_at").
		Data(price).
		InsertAndGetId()
	if err != nil {
		return err
	}
	price.Id = result
	return nil
}

// GetLatestBySymbol 获取指定股票的最新价格
func (d *stockPriceDao) GetLatestBySymbol(ctx context.Context, symbol string) (*entity.StockPriceEntity, error) {
	var price entity.StockPriceEntity
	err := d.db.Model("stock_price").Ctx(ctx).
		Where("symbol = ?", symbol).
		OrderDesc("price_time").
		Limit(1).
		Scan(&price)
	if err != nil {
		return nil, err
	}
	if price.Id == 0 {
		return nil, nil
	}
	return &price, nil
}

// GetByTimeRange 根据时间范围获取价格记录
func (d *stockPriceDao) GetByTimeRange(ctx context.Context, symbol string, startTime, endTime time.Time) ([]*entity.StockPriceEntity, error) {
	var prices []*entity.StockPriceEntity
	err := d.db.Model("stock_price").Ctx(ctx).
		Where("symbol = ?", symbol).
		Where("price_time >= ? AND price_time <= ?", startTime, endTime).
		OrderAsc("price_time").
		Scan(&prices)
	if err != nil {
		return nil, err
	}
	return prices, nil
}

// GetLatestN 获取最新的N条价格记录
func (d *stockPriceDao) GetLatestN(ctx context.Context, symbol string, limit int) ([]*entity.StockPriceEntity, error) {
	var prices []*entity.StockPriceEntity
	err := d.db.Model("stock_price").Ctx(ctx).
		Where("symbol = ?", symbol).
		OrderDesc("price_time").
		Limit(limit).
		Scan(&prices)
	if err != nil {
		return nil, err
	}
	return prices, nil
}
