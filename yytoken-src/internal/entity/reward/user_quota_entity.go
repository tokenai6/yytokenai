package reward

import (
	"XWFrame/internal/frame/model"

	"github.com/shopspring/decimal"
)

// UserQuotaEntity 用户额度实体
type UserQuotaEntity struct {
	model.BaseEntity
	UserID                  int64           `json:"user_id" gorm:"uniqueIndex;not null"`
	TotalStakeAmount        decimal.Decimal `json:"total_stake_amount" gorm:"type:decimal(28,18);default:0;comment:当前总质押金额（只包含运行中算力包）"`
	MaxStaticRelease        decimal.Decimal `json:"max_static_release" gorm:"type:decimal(28,18);default:0;comment:当前最大静态释放总额"`
	ReleasedStatic          decimal.Decimal `json:"released_static" gorm:"type:decimal(28,18);default:0;comment:已释放静态收益累计"`
	TotalQuota              decimal.Decimal `json:"total_quota" gorm:"type:decimal(28,18);default:0;comment:当前总额度"`
	UsedQuota               decimal.Decimal `json:"used_quota" gorm:"type:decimal(28,18);default:0;comment:已使用额度"`
	RemainingQuota          decimal.Decimal `json:"remaining_quota" gorm:"type:decimal(28,18);default:0;index:idx_quota_remaining;comment:剩余额度"`
	TotalIncome             decimal.Decimal `json:"total_income" gorm:"type:decimal(28,18);default:0;comment:总收益"`
	WithdrawnAmount         decimal.Decimal `json:"withdrawn_amount" gorm:"type:decimal(28,18);default:0;comment:已提取金额"`
	AvailableAmount         decimal.Decimal `json:"available_amount" gorm:"type:decimal(28,18);default:0;comment:可提取金额"`
	HistoryTotalQuota       decimal.Decimal `json:"history_total_quota" gorm:"type:decimal(28,18);default:0;comment:历史总额度（包含已出局）"`
	HistoryTotalStakeAmount decimal.Decimal `json:"history_total_stake_amount" gorm:"type:decimal(28,18);default:0;comment:历史总质押金额（包含已出局）"`
	HistoryStakeCount       int             `json:"history_stake_count" gorm:"default:0;comment:历史总质押笔数（包含已出局）"`
	Version                 int             `json:"version" gorm:"default:0;comment:乐观锁版本号"`
}

// TableName 指定表名
func (UserQuotaEntity) TableName() string {
	return "user_quota"
}
