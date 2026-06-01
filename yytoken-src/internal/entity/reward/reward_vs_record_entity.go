package reward

import (
	"XWFrame/internal/frame/model"

	"github.com/shopspring/decimal"
)

// RewardVsRecordEntity 用户购买/直推 VS 奖励记录实体
type RewardVsRecordEntity struct {
	model.BaseEntity
	UserID int64           `json:"user_id" gorm:"not null;index"`
	Amount decimal.Decimal `json:"amount" gorm:"type:decimal(28,18);not null"`
	Types  int             `json:"types" gorm:"not null;default:0;index"` // 0=购买，1=直推
	Claim  bool            `json:"claim" gorm:"not null;default:false;index"`
}

func (RewardVsRecordEntity) TableName() string {
	return "reward_vs_record"
}

