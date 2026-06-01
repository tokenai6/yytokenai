package entity

import (
	"XWFrame/internal/frame/model"
	"time"
)

// NodeEntity 节点信息实体
type NodeEntity struct {
	model.BaseEntity
	NodeLevel       string     `json:"node_level" g:"size:50;not null;comment:节点等级"`
	StartTime       *time.Time `json:"start_time" g:"comment:开始时间"`
	EndTime         *time.Time `json:"end_time" g:"comment:结束时间"`
	Status          int        `json:"status" g:"default:1;comment:状态 1-待售 2-在售 3-售罄"`
	PowerMultiplier float64    `json:"power_multiplier" g:"type:decimal(10,2);not null;comment:算力倍数"`
	TotalShares     int64      `json:"total_shares" g:"not null;comment:总份额"`
	SoldShares      int64      `json:"sold_shares" g:"default:0;comment:已销售份额"`
	MinGrowth       int64      `json:"min_growth" g:"comment:最小增长"`
	MaxGrowth       int64      `json:"max_growth" g:"comment:最大增长"`
}
