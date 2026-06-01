package reward

import (
	"XWFrame/internal/frame/model"
	"time"

	"github.com/shopspring/decimal"
)

// UserVipLevelEntity VIP等级实体
type UserVipLevelEntity struct {
	model.BaseEntity
	UserID              int64           `json:"user_id" gorm:"not null;index:idx_vip_user_current"`
	VipLevel            int             `json:"vip_level" gorm:"not null;default:0"`
	DistrictPerformance decimal.Decimal `json:"district_performance" gorm:"type:decimal(28,18);default:0"`
	RewardRate          decimal.Decimal `json:"reward_rate" gorm:"type:decimal(10,6);default:0"`
	RecordTime          time.Time       `json:"record_time" gorm:"type:timestamp;not null;index:idx_vip_record_date;index:idx_vip_record_time"`
	PrevVipLevel        *int            `json:"prev_vip_level"`
	IsCurrent           bool            `json:"is_current" gorm:"default:true;index:idx_vip_user_current,priority:2"`
}

// TableName 指定表名
func (UserVipLevelEntity) TableName() string {
	return "user_vip_level"
}
