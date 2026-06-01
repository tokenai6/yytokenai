package reward

import (
	"XWFrame/internal/frame/model"
	"time"

	"github.com/shopspring/decimal"
)

// UserDailyStakeStatisticsEntity 用户每日质押统计实体
type UserDailyStakeStatisticsEntity struct {
	model.BaseEntity
	UserID         int64           `json:"user_id" gorm:"not null;uniqueIndex:idx_user_daily_stake_time;comment:用户ID"`
	RecordTime     time.Time       `json:"record_time" gorm:"type:timestamp;not null;uniqueIndex:idx_user_daily_stake_time;index:idx_daily_stake_time;comment:记录时间"`
	NewStakeAmount decimal.Decimal `json:"new_stake_amount" gorm:"type:decimal(28,18);default:0;comment:当日新增LP质押金额"`
	NewStakeCount  int             `json:"new_stake_count" gorm:"default:0;comment:当日新增质押笔数"`
	NewQuotaAmount decimal.Decimal `json:"new_quota_amount" gorm:"type:decimal(28,18);default:0;comment:当日新增总额度"`
}

// TableName 指定表名
func (UserDailyStakeStatisticsEntity) TableName() string {
	return "user_daily_stake_statistics"
}
