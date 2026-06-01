package repository

import (
	dao "XWFrame/internal/dao"
	entity "XWFrame/internal/entity"
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
)

// NodePurchaseRepository 节点购买记录仓储
type NodePurchaseRepository struct {
	dao *dao.NodePurchaseRecordDao
}

// NewNodePurchaseRepository 创建节点购买记录仓储
func NewNodePurchaseRepository() *NodePurchaseRepository {
	return &NodePurchaseRepository{
		dao: dao.NewNodePurchaseRecordDao(),
	}
}

// GetAll 全量获取节点购买记录
func (r *NodePurchaseRepository) GetAll(ctx context.Context) ([]*entity.NodePurchaseRecordEntity, error) {
	return r.dao.GetAll(ctx)
}

// CreatePurchaseRecord 创建购买记录
func (r *NodePurchaseRepository) CreatePurchaseRecord(ctx context.Context, entity *entity.NodePurchaseRecordEntity) error {
	err := r.dao.Create(ctx, entity)
	if err != nil {
		return fmt.Errorf("创建购买记录失败: %v", err)
	}

	g.Log().Infof(ctx, "创建购买记录成功: 用户 %d, 金额 %s, 交易哈希 %s",
		entity.UserID, entity.Amount, entity.TransactionHash)
	return nil
}

// CheckEventProcessed 检查事件是否已处理
func (r *NodePurchaseRepository) CheckEventProcessed(ctx context.Context, eventID string) (bool, error) {
	record, err := r.dao.GetByEventId(ctx, eventID)
	if err != nil {
		return false, fmt.Errorf("检查事件处理状态失败: %v", err)
	}
	return record != nil, nil
}

// GetUserPurchaseRecords 获取用户购买记录
func (r *NodePurchaseRepository) GetUserPurchaseRecords(ctx context.Context, userID int64) ([]*entity.NodePurchaseRecordEntity, error) {
	records, err := r.dao.GetByUserId(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("查询用户购买记录失败: %v", err)
	}
	return records, nil
}

// GetReferralPurchaseStats 获取推荐用户购买统计
func (r *NodePurchaseRepository) GetReferralPurchaseStats(ctx context.Context, parentUserIDs []int64) ([]*entity.NodePurchaseRecordEntity, error) {
	records, err := r.dao.GetByParentUsers(ctx, parentUserIDs)
	if err != nil {
		return nil, fmt.Errorf("查询推荐用户购买记录失败: %v", err)
	}
	return records, nil
}
