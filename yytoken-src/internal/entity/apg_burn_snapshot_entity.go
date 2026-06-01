package entity

import (
	"time"

	"github.com/shopspring/decimal"
)

// ApgBurnSnapshot APG销毁总量快照实体
type ApgBurnSnapshot struct {
	Id           int64           `json:"id" orm:"id,primary"`               // 主键ID
	TotalBurned  decimal.Decimal `json:"total_burned" orm:"total_burned"`   // 累计销毁总量
	SnapshotTime time.Time       `json:"snapshot_time" orm:"snapshot_time"` // 快照时间
	Source       string          `json:"source" orm:"source"`               // 数据来源（contract=链上, local=本地汇总）
	CreatedAt    time.Time       `json:"created_at" orm:"created_at"`       // 创建时间
}
