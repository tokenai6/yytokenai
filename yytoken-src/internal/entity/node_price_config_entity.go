package entity

import (
	"XWFrame/internal/frame/model"

	"github.com/shopspring/decimal"
)

// NodePriceConfigEntity 节点价格配置
type NodePriceConfigEntity struct {
	model.BaseEntity
	NodeType  int             `json:"node_type"`
	NodeLevel string          `json:"node_level"`
	Price     decimal.Decimal `json:"price"`
}
