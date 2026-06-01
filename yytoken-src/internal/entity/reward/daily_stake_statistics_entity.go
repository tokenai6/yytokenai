package reward

import (
	"XWFrame/internal/frame/model"
	"time"

	"github.com/shopspring/decimal"
)

// DailyStakeStatisticsEntity 每日质押统计实体
type DailyStakeStatisticsEntity struct {
	model.BaseEntity
	RecordTime          time.Time       `json:"record_time" gorm:"type:timestamp;uniqueIndex;not null;comment:记录时间"`
	TotalNewStakeAmount decimal.Decimal `json:"total_new_stake_amount" gorm:"type:decimal(28,18);default:0;comment:当日新增LP质押总额"`
	TotalNewStakeCount  int             `json:"total_new_stake_count" gorm:"default:0;comment:当日新增LP质押笔数"`
	TotalNewUsers       int             `json:"total_new_users" gorm:"default:0;comment:当日新增LP质押用户数（去重）"`
	LpStakeAmount       decimal.Decimal `json:"lp_stake_amount" gorm:"type:decimal(28,18);default:0;comment:LP质押金额"`
	LpStakeCount        int             `json:"lp_stake_count" gorm:"default:0;comment:LP质押笔数"`
	NodePurchaseAmount  decimal.Decimal `json:"node_purchase_amount" gorm:"type:decimal(28,18);default:0;comment:节点购买金额（上线前预售，上线后不再新增）"`
	NodePurchaseCount   int             `json:"node_purchase_count" gorm:"default:0;comment:节点购买笔数（上线前预售，上线后不再新增）"`
}

// TableName 指定表名
func (DailyStakeStatisticsEntity) TableName() string {
	return "daily_stake_statistics"
}
