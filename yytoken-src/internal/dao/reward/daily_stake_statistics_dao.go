package reward

import (
	"context"
	"time"

	"XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/shopspring/decimal"
)

// IDailyStakeStatisticsDao 每日质押统计数据访问接口
type IDailyStakeStatisticsDao interface {
	// Create 创建每日质押统计
	Create(ctx context.Context, tx gdb.TX, stats *reward.DailyStakeStatisticsEntity) error

	// GetByDate 根据日期查询
	GetByDate(ctx context.Context, date time.Time) (*reward.DailyStakeStatisticsEntity, error)

	// CalculateGlobalStats 计算全网统计数据
	CalculateGlobalStats(ctx context.Context, date time.Time) (*reward.DailyStakeStatisticsEntity, error)
}

// dailyStakeStatisticsDao 每日质押统计数据访问实现
type dailyStakeStatisticsDao struct {
	db gdb.DB
}

// NewDailyStakeStatisticsDao 创建每日质押统计数据访问实例
func NewDailyStakeStatisticsDao() IDailyStakeStatisticsDao {
	return &dailyStakeStatisticsDao{
		db: db.GetDB(),
	}
}

// Create 创建每日质押统计
func (d *dailyStakeStatisticsDao) Create(ctx context.Context, tx gdb.TX, stats *reward.DailyStakeStatisticsEntity) error {
	model := d.db.Model("daily_stake_statistics")
	if tx != nil {
		model = tx.Model("daily_stake_statistics")
	}

	result, err := model.Ctx(ctx).
		FieldsEx("id", "created_at", "updated_at").
		Data(stats).
		InsertAndGetId()
	if err != nil {
		return err
	}
	stats.Id = result
	return nil
}

// GetByDate 根据时刻查询
func (d *dailyStakeStatisticsDao) GetByDate(ctx context.Context, date time.Time) (*reward.DailyStakeStatisticsEntity, error) {
	var stats reward.DailyStakeStatisticsEntity
	err := d.db.Model("daily_stake_statistics").Ctx(ctx).
		Where("record_time = ?", date).
		Scan(&stats)
	if err != nil {
		return nil, err
	}
	if stats.Id == 0 {
		return nil, nil
	}
	return &stats, nil
}

// CalculateGlobalStats 计算全网统计数据
func (d *dailyStakeStatisticsDao) CalculateGlobalStats(ctx context.Context, date time.Time) (*reward.DailyStakeStatisticsEntity, error) {
	type Result struct {
		TotalCount         int             `json:"total_count"`
		TotalAmount        decimal.Decimal `json:"total_amount"`
		TotalUsers         int             `json:"total_users"`
		LpAmount           decimal.Decimal `json:"lp_amount"`
		LpCount            int             `json:"lp_count"`
		NodePurchaseAmount decimal.Decimal `json:"node_purchase_amount"`
		NodePurchaseCount  int             `json:"node_purchase_count"`
	}

	var result Result
	sql := `
		SELECT 
			COUNT(*) as total_count,
			COALESCE(SUM(stake_amount), 0) as total_amount,
			COUNT(DISTINCT user_id) as total_users,
			COALESCE(SUM(CASE WHEN stake_type = 1 THEN stake_amount ELSE 0 END), 0) as lp_amount,
			SUM(CASE WHEN stake_type = 1 THEN 1 ELSE 0 END) as lp_count,
			COALESCE(SUM(CASE WHEN stake_type = 2 THEN stake_amount ELSE 0 END), 0) as node_purchase_amount,
			SUM(CASE WHEN stake_type = 2 THEN 1 ELSE 0 END) as node_purchase_count
		FROM staking_package
		WHERE DATE(start_time) = ?
			AND stake_type = 1
	`

	err := d.db.Ctx(ctx).Raw(sql, date.Format(consts.TimeFormatDate)).Scan(&result)
	if err != nil {
		return nil, err
	}

	stats := &reward.DailyStakeStatisticsEntity{
		RecordTime:          date,
		TotalNewStakeAmount: result.TotalAmount,
		TotalNewStakeCount:  result.TotalCount,
		TotalNewUsers:       result.TotalUsers,
		LpStakeAmount:       result.LpAmount,
		LpStakeCount:        result.LpCount,
		NodePurchaseAmount:  result.NodePurchaseAmount,
		NodePurchaseCount:   result.NodePurchaseCount,
	}

	return stats, nil
}
