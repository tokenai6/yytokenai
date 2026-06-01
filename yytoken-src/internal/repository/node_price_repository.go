package repository

import (
	"context"

	"XWFrame/internal/entity"

	"github.com/gogf/gf/v2/frame/g"
)

// NodePriceRepository 节点价格配置仓储
type NodePriceRepository struct{}

// NewNodePriceRepository 创建节点价格配置仓储
func NewNodePriceRepository() *NodePriceRepository {
	return &NodePriceRepository{}
}

// GetByNodeType 根据节点类型获取价格配置
func (r *NodePriceRepository) GetByNodeType(ctx context.Context, nodeType int) (*entity.NodePriceConfigEntity, error) {
	var cfg entity.NodePriceConfigEntity
	err := g.DB().Model("node_price_config").
		Ctx(ctx).
		Where("node_type = ?", nodeType).
		Scan(&cfg)
	if err != nil {
		return nil, err
	}
	if cfg.Id == 0 {
		return nil, nil
	}
	return &cfg, nil
}

// GetAll 获取全部节点价格配置
func (r *NodePriceRepository) GetAll(ctx context.Context) ([]*entity.NodePriceConfigEntity, error) {
	list := make([]*entity.NodePriceConfigEntity, 0)
	err := g.DB().Model("node_price_config").
		Ctx(ctx).
		Order("node_type ASC").
		Scan(&list)
	if err != nil {
		return nil, err
	}
	return list, nil
}
