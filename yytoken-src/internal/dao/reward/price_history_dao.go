package reward

import (
	"context"
	"time"

	"XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// IPriceHistoryDao 价格历史数据访问接口
type IPriceHistoryDao interface {
	// Create 创建价格历史记录
	Create(ctx context.Context, tx gdb.TX, price *reward.PriceHistoryEntity) error

	// GetLatest 获取最新价格
	GetLatest(ctx context.Context) (*reward.PriceHistoryEntity, error)

	// GetSeriesByRange 根据时间范围获取价格序列
	GetSeriesByRange(ctx context.Context, startTime, endTime time.Time) ([]*reward.PriceHistoryEntity, error)

	// GetLatestN 获取最新的N条价格记录
	GetLatestN(ctx context.Context, limit int) ([]*reward.PriceHistoryEntity, error)

	// GetPriceAtOrBefore 获取指定时间点或之前最近的价格记录
	GetPriceAtOrBefore(ctx context.Context, targetTime time.Time) (*reward.PriceHistoryEntity, error)

	// UpsertDailyOpenPrice 插入或更新每日开盘价记录
	UpsertDailyOpenPrice(ctx context.Context, tx gdb.TX, price *reward.PriceHistoryEntity) error

	// GetDailyOpenPrice 获取指定日期的开盘价
	GetDailyOpenPrice(ctx context.Context, date time.Time) (*reward.PriceHistoryEntity, error)
}

// priceHistoryDao 价格历史数据访问实现
type priceHistoryDao struct {
	db gdb.DB
}

// NewPriceHistoryDao 创建价格历史数据访问实例
func NewPriceHistoryDao() IPriceHistoryDao {
	return &priceHistoryDao{
		db: db.GetDB(),
	}
}

// Create 创建价格历史记录
func (d *priceHistoryDao) Create(ctx context.Context, tx gdb.TX, price *reward.PriceHistoryEntity) error {
	model := d.db.Model("price_history")
	if tx != nil {
		model = tx.Model("price_history")
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

// GetLatest 获取最新价格
func (d *priceHistoryDao) GetLatest(ctx context.Context) (*reward.PriceHistoryEntity, error) {
	var price reward.PriceHistoryEntity
	err := d.db.Model("price_history").Ctx(ctx).
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

// GetSeriesByRange 根据时间范围获取价格序列
func (d *priceHistoryDao) GetSeriesByRange(ctx context.Context, startTime, endTime time.Time) ([]*reward.PriceHistoryEntity, error) {
	var prices []*reward.PriceHistoryEntity
	err := d.db.Model("price_history").Ctx(ctx).
		Where("price_time >= ? AND price_time <= ?", startTime, endTime).
		OrderAsc("price_time").
		Scan(&prices)
	if err != nil {
		return nil, err
	}
	return prices, nil
}

// GetLatestN 获取最新的N条价格记录（按时间倒序）
func (d *priceHistoryDao) GetLatestN(ctx context.Context, limit int) ([]*reward.PriceHistoryEntity, error) {
	var prices []*reward.PriceHistoryEntity
	err := d.db.Model("price_history").Ctx(ctx).
		OrderDesc("price_time").
		Limit(limit).
		Scan(&prices)
	if err != nil {
		return nil, err
	}
	return prices, nil
}

// GetPriceAtOrBefore 获取指定时间点或之前最近的价格记录
func (d *priceHistoryDao) GetPriceAtOrBefore(ctx context.Context, targetTime time.Time) (*reward.PriceHistoryEntity, error) {
	var price reward.PriceHistoryEntity
	err := d.db.Model("price_history").Ctx(ctx).
		Where("price_time <= ?", targetTime).
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

// UpsertDailyOpenPrice 插入或更新每日开盘价记录
func (d *priceHistoryDao) UpsertDailyOpenPrice(ctx context.Context, tx gdb.TX, price *reward.PriceHistoryEntity) error {
	model := d.db.Model("price_history")
	if tx != nil {
		model = tx.Model("price_history")
	}

	var existing reward.PriceHistoryEntity
	err := model.Ctx(ctx).
		Where("price_time = ? AND source = ?", price.PriceTime, price.Source).
		Scan(&existing)

	if existing.Id > 0 {
		_, err = model.Ctx(ctx).
			Where("id = ?", existing.Id).
			Data(gdb.Map{
				"rex_price":     price.RexPrice,
				"apg_price":     price.ApgPrice,
				"exchange_rate": price.ExchangeRate,
			}).
			Update()
		return err
	}

	result, err := model.Ctx(ctx).
		Data(gdb.Map{
			"price_time":    price.PriceTime,
			"rex_price":     price.RexPrice,
			"apg_price":     price.ApgPrice,
			"exchange_rate": price.ExchangeRate,
			"source":        price.Source,
		}).
		InsertAndGetId()
	if err != nil {
		return err
	}
	price.Id = result
	return nil
}

// GetDailyOpenPrice 获取指定日期的开盘价
func (d *priceHistoryDao) GetDailyOpenPrice(ctx context.Context, date time.Time) (*reward.PriceHistoryEntity, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	endOfDay := startOfDay.Add(24 * time.Hour)

	var price reward.PriceHistoryEntity
	d.db.Model("price_history").Ctx(ctx).
		Where("source = ?", "manual_daily_open").
		Where("price_time >= ? AND price_time < ?", startOfDay, endOfDay).
		OrderDesc("price_time").
		Limit(1).
		Scan(&price)

	if price.Id == 0 {
		return nil, nil
	}
	return &price, nil
}
