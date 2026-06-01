package price

import (
	"context"
	"time"

	"XWFrame/internal/dao/reward"
	rewardEntity "XWFrame/internal/entity/reward"

	"github.com/gogf/gf/v2/errors/gerror"
)

// IPriceRepository 价格仓储接口
type IPriceRepository interface {
	// GetLatestN 获取最新的N条价格记录
	GetLatestN(ctx context.Context, limit int) ([]*rewardEntity.PriceHistoryEntity, error)

	// GetPriceAtOrBefore 获取指定时间点或之前最近的价格记录
	GetPriceAtOrBefore(ctx context.Context, targetTime time.Time) (*rewardEntity.PriceHistoryEntity, error)

	// GetDailyOpenPrice 获取指定日期的开盘价
	GetDailyOpenPrice(ctx context.Context, date time.Time) (*rewardEntity.PriceHistoryEntity, error)
}

// priceRepository 价格仓储实现
type priceRepository struct {
	priceDao reward.IPriceHistoryDao
}

// NewPriceRepository 创建价格仓储实例
func NewPriceRepository() IPriceRepository {
	return &priceRepository{
		priceDao: reward.NewPriceHistoryDao(),
	}
}

// GetLatestN 获取最新的N条价格记录
func (r *priceRepository) GetLatestN(ctx context.Context, limit int) ([]*rewardEntity.PriceHistoryEntity, error) {
	prices, err := r.priceDao.GetLatestN(ctx, limit)
	if err != nil {
		return nil, gerror.Wrapf(err, "获取价格曲线失败")
	}
	return prices, nil
}

// GetPriceAtOrBefore 获取指定时间点或之前最近的价格记录
func (r *priceRepository) GetPriceAtOrBefore(ctx context.Context, targetTime time.Time) (*rewardEntity.PriceHistoryEntity, error) {
	price, err := r.priceDao.GetPriceAtOrBefore(ctx, targetTime)
	if err != nil {
		return nil, gerror.Wrapf(err, "获取指定时间价格失败")
	}
	return price, nil
}

// GetDailyOpenPrice 获取指定日期的开盘价
func (r *priceRepository) GetDailyOpenPrice(ctx context.Context, date time.Time) (*rewardEntity.PriceHistoryEntity, error) {
	price, err := r.priceDao.GetDailyOpenPrice(ctx, date)
	if err != nil {
		return nil, gerror.Wrapf(err, "获取开盘价失败")
	}
	return price, nil
}
