package repository

import (
	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
)

// NodeRepository 节点信息仓储
type NodeRepository struct {
	dao *dao.NodeDao
}

// NewNodeRepository 创建节点信息仓储
func NewNodeRepository() *NodeRepository {
	return &NodeRepository{
		dao: dao.NewNodeDao(),
	}
}

// GetNodeInfo 获取节点信息
func (r *NodeRepository) GetNodeInfo(ctx context.Context, id int64) (*entity.NodeEntity, error) {
	node, err := r.dao.GetById(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("查询节点信息失败: %v", err)
	}
	return node, nil
}

// GetAllNodes 获取所有节点
func (r *NodeRepository) GetAllNodes(ctx context.Context) ([]*entity.NodeEntity, error) {
	nodes, err := r.dao.GetList(ctx, 0, 0) // 0,0 表示获取所有记录
	if err != nil {
		return nil, fmt.Errorf("查询所有节点失败: %v", err)
	}
	return nodes, nil
}

// GetOnSaleNode 获取在售节点
func (r *NodeRepository) GetOnSaleNode(ctx context.Context) (*entity.NodeEntity, error) {
	node, err := r.dao.GetOnSaleNode(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询在售节点失败: %v", err)
	}
	return node, nil
}

// IncreaseSoldShares 增加已销售份额
func (r *NodeRepository) IncreaseSoldShares(ctx context.Context, id int64, newSoldShares int64) error {
	err := r.dao.UpdateSoldShares(ctx, id, newSoldShares)
	if err != nil {
		return fmt.Errorf("更新已销售份额失败: %v", err)
	}

	g.Log().Infof(ctx, "节点 %d 已销售份额更新为 %d", id, newSoldShares)
	return nil
}

// UpdateNodeStatus 更新节点状态
func (r *NodeRepository) UpdateNodeStatus(ctx context.Context, id int64, status int) error {
	err := r.dao.UpdateStatus(ctx, id, status)
	if err != nil {
		return fmt.Errorf("更新节点状态失败: %v", err)
	}

	g.Log().Infof(ctx, "节点 %d 状态更新为 %d", id, status)
	return nil
}
