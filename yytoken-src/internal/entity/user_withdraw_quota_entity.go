package entity

import (
	"XWFrame/internal/frame/model"

	"github.com/shopspring/decimal"
)

// UserWithdrawQuotaEntity 用户提现额度实体
type UserWithdrawQuotaEntity struct {
	model.BaseEntity
	UserID         int64           `json:"user_id" gorm:"uniqueIndex:idx_user_month;not null"`
	Month          string          `json:"month" gorm:"uniqueIndex:idx_user_month;not null;comment:月份YYYY-MM"`
	WithdrawOffset    decimal.Decimal `json:"withdraw_offset" gorm:"type:decimal(28,8);default:0;comment:管理员重置时抵消的已提现金额"`
	ExtraQuota        decimal.Decimal `json:"extra_quota" gorm:"type:decimal(28,8);default:0;comment:管理员额外增加的额度"`
	TaxDeductionUsed  decimal.Decimal `json:"tax_deduction_used" gorm:"type:decimal(28,8);default:0;comment:本月已使用的税款抵扣额度"`
	Remark            string          `json:"remark" gorm:"type:varchar(255);default:'';comment:操作备注"`
}

// TableName 指定表名
func (UserWithdrawQuotaEntity) TableName() string {
	return "user_withdraw_quota"
}
