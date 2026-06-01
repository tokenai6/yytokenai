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

// IUserDailyStakeStatisticsDao 用户每日质押统计数据访问接口
type IUserDailyStakeStatisticsDao interface {
	// Create 创建用户每日质押统计
	Create(ctx context.Context, tx gdb.TX, stats *reward.UserDailyStakeStatisticsEntity) error

	// BatchCreate 批量创建
	BatchCreate(ctx context.Context, tx gdb.TX, statsList []*reward.UserDailyStakeStatisticsEntity) error

	// GetByUserAndDate 根据用户和日期查询
	GetByUserAndDate(ctx context.Context, userID int64, date time.Time) (*reward.UserDailyStakeStatisticsEntity, error)

	// GetByDate 根据日期查询所有用户统计
	GetByDate(ctx context.Context, date time.Time) ([]*reward.UserDailyStakeStatisticsEntity, error)

	// CalculateUserStats 计算每个用户的统计数据
	CalculateUserStats(ctx context.Context, date time.Time) ([]*reward.UserDailyStakeStatisticsEntity, error)
}

// userDailyStakeStatisticsDao 用户每日质押统计数据访问实现
type userDailyStakeStatisticsDao struct {
	db gdb.DB
}

// NewUserDailyStakeStatisticsDao 创建用户每日质押统计数据访问实例
func NewUserDailyStakeStatisticsDao() IUserDailyStakeStatisticsDao {
	return &userDailyStakeStatisticsDao{
		db: db.GetDB(),
	}
}

// Create 创建用户每日质押统计
func (d *userDailyStakeStatisticsDao) Create(ctx context.Context, tx gdb.TX, stats *reward.UserDailyStakeStatisticsEntity) error {
	model := d.db.Model("user_daily_stake_statistics")
	if tx != nil {
		model = tx.Model("user_daily_stake_statistics")
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

// BatchCreate 批量创建
func (d *userDailyStakeStatisticsDao) BatchCreate(ctx context.Context, tx gdb.TX, statsList []*reward.UserDailyStakeStatisticsEntity) error {
	if len(statsList) == 0 {
		return nil
	}

	model := d.db.Model("user_daily_stake_statistics")
	if tx != nil {
		model = tx.Model("user_daily_stake_statistics")
	}

	batchSize := consts.MaxBatchSizeRecordInsert
	for start := 0; start < len(statsList); start += batchSize {
		end := start + batchSize
		if end > len(statsList) {
			end = len(statsList)
		}

		batch := statsList[start:end]
		if _, err := model.Ctx(ctx).
			FieldsEx("id", "created_at", "updated_at").
			Data(batch).
			Insert(); err != nil {
			return err
		}
	}

	return nil
}

// GetByUserAndDate 根据用户和时刻查询
func (d *userDailyStakeStatisticsDao) GetByUserAndDate(ctx context.Context, userID int64, date time.Time) (*reward.UserDailyStakeStatisticsEntity, error) {
	var stats reward.UserDailyStakeStatisticsEntity
	err := d.db.Model("user_daily_stake_statistics").Ctx(ctx).
		Where("user_id", userID).
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

// GetByDate 根据时刻查询所有用户统计
func (d *userDailyStakeStatisticsDao) GetByDate(ctx context.Context, date time.Time) ([]*reward.UserDailyStakeStatisticsEntity, error) {
	var statsList []*reward.UserDailyStakeStatisticsEntity
	err := d.db.Model("user_daily_stake_statistics").Ctx(ctx).
		Where("record_time = ?", date).
		Scan(&statsList)
	if err != nil {
		return nil, err
	}
	return statsList, nil
}

// CalculateUserStats 计算每个用户的统计数据
func (d *userDailyStakeStatisticsDao) CalculateUserStats(ctx context.Context, date time.Time) ([]*reward.UserDailyStakeStatisticsEntity, error) {
	type Result struct {
		UserID      int64           `json:"user_id"`
		StakeAmount decimal.Decimal `json:"stake_amount"`
		StakeCount  int             `json:"stake_count"`
		QuotaAmount decimal.Decimal `json:"quota_amount"`
	}

	var results []Result
	sql := `
		SELECT 
			user_id,
			COALESCE(SUM(stake_amount), 0) as stake_amount,
			COUNT(*) as stake_count,
			COALESCE(SUM(total_quota), 0) as quota_amount
		FROM staking_package
		WHERE DATE(start_time) = ?
		GROUP BY user_id
	`

	err := d.db.Ctx(ctx).Raw(sql, date.Format(consts.TimeFormatDate)).Scan(&results)
	if err != nil {
		return nil, err
	}

	statsList := make([]*reward.UserDailyStakeStatisticsEntity, 0, len(results))
	for _, res := range results {
		statsList = append(statsList, &reward.UserDailyStakeStatisticsEntity{
			UserID:         res.UserID,
			RecordTime:     date,
			NewStakeAmount: res.StakeAmount,
			NewStakeCount:  res.StakeCount,
			NewQuotaAmount: res.QuotaAmount,
		})
	}

	return statsList, nil
}
