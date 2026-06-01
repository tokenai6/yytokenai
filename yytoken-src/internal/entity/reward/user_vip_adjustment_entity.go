package reward

import (
	"XWFrame/internal/frame/model"
	"time"
)

// UserVipAdjustmentEntity VIP调整记录实体
type UserVipAdjustmentEntity struct {
	model.BaseEntity
	UserID           int64     `json:"user_id" gorm:"not null;index:idx_vip_adj_user_expire"`
	AdjustedVipLevel int       `json:"adjusted_vip_level" gorm:"not null;default:0"`
	OriginalVipLevel int       `json:"original_vip_level" gorm:"not null;default:0"`
	AdjustTime       time.Time `json:"adjust_time" gorm:"type:timestamptz;not null"`
	ExpireTime       time.Time `json:"expire_time" gorm:"type:timestamptz;not null;index:idx_vip_adj_expire"`
	OperatorID       int64     `json:"operator_id" gorm:"not null;default:0"`
	Remark           string    `json:"remark" gorm:"type:varchar(500);default:''"`
}

// TableName 指定表名
func (UserVipAdjustmentEntity) TableName() string {
	return "user_vip_adjustment"
}

// IsValid 判断调整记录是否有效
// 有效条件：未过期（取消操作会将过期时间设置为当前时间）
func (e *UserVipAdjustmentEntity) IsValid(now time.Time) bool {
	return now.Before(e.ExpireTime)
}
