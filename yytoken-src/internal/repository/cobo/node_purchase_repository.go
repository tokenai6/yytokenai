package cobo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"XWFrame/internal/entity/cobo"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// nodePurchaseRepository 节点购买仓储实现
type nodePurchaseRepository struct{}

// NewNodePurchaseRepository 创建节点购买仓储
func NewNodePurchaseRepository() INodePurchaseRepository {
	return &nodePurchaseRepository{}
}

// Create 创建购买记录
func (r *nodePurchaseRepository) Create(ctx context.Context, tx gdb.TX, entity *cobo.NodePurchaseEntity) error {
	entity.CreatedAt = time.Now()
	entity.UpdatedAt = time.Now()
	id, err := tx.Model("cobo_node_purchase").
		Ctx(ctx).
		FieldsEx("id").
		Data(entity).
		InsertAndGetId()
	if err == nil {
		entity.ID = id
	}
	return err
}

// GetByPackageNo 根据包号获取购买记录
func (r *nodePurchaseRepository) GetByPackageNo(ctx context.Context, packageNo string) (*cobo.NodePurchaseEntity, error) {
	var entity cobo.NodePurchaseEntity
	err := g.DB().Model("cobo_node_purchase").
		Ctx(ctx).
		Where("package_no = ?", packageNo).
		Scan(&entity)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || isNoRowsErr(err) {
			return nil, nil
		}
		return nil, err
	}
	return &entity, nil
}

// GetByUserAndRequestID 根据用户ID和请求ID获取购买记录（幂等）
func (r *nodePurchaseRepository) GetByUserAndRequestID(ctx context.Context, userID int64, requestID string) (*cobo.NodePurchaseEntity, error) {
	if requestID == "" {
		return nil, nil
	}

	var entity cobo.NodePurchaseEntity
	err := g.DB().Model("cobo_node_purchase").
		Ctx(ctx).
		Where("user_id = ? AND request_id = ?", userID, requestID).
		Scan(&entity)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || isNoRowsErr(err) {
			return nil, nil
		}
		return nil, err
	}

	if entity.ID == 0 {
		return nil, nil
	}

	return &entity, nil
}

// GetByUserID 获取用户的购买记录（分页）
func (r *nodePurchaseRepository) GetByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*cobo.NodePurchaseEntity, int64, error) {
	var entities []*cobo.NodePurchaseEntity
	var total int64

	model := g.DB().Model("cobo_node_purchase").
		Ctx(ctx).
		Where("user_id = ?", userID)

	count, err := model.Count()
	if err != nil {
		return nil, 0, err
	}
	total = int64(count)

	err = model.
		Order("created_at DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Scan(&entities)

	if err != nil {
		return nil, 0, err
	}

	return entities, total, nil
}

// GetRunningByUserID 获取用户运行中的节点
func (r *nodePurchaseRepository) GetRunningByUserID(ctx context.Context, userID int64) ([]*cobo.NodePurchaseEntity, error) {
	var entities []*cobo.NodePurchaseEntity
	err := g.DB().Model("cobo_node_purchase").
		Ctx(ctx).
		Where("user_id = ? AND status = ?", userID, cobo.NodeStatusRunning).
		Scan(&entities)
	return entities, err
}

// UpdateStatus 更新购买状态
func (r *nodePurchaseRepository) UpdateStatus(ctx context.Context, id int64, status int) error {
	data := g.Map{
		"status":     status,
		"updated_at": time.Now(),
	}

	if status != cobo.NodeStatusRunning {
		now := time.Now()
		data["actual_end_time"] = now
	}

	_, err := g.DB().Model("cobo_node_purchase").
		Ctx(ctx).
		Data(data).
		Where("id = ?", id).
		Update()
	return err
}

// UpdateRewards 更新奖励金额
func (r *nodePurchaseRepository) UpdateRewards(ctx context.Context, id int64, teamReward, leaderReward decimal.Decimal) error {
	_, err := g.DB().Model("cobo_node_purchase").
		Ctx(ctx).
		Data(g.Map{
			"team_reward_total":   teamReward,
			"leader_reward_total": leaderReward,
			"updated_at":          time.Now(),
		}).
		Where("id = ?", id).
		Update()
	return err
}

// GetDirectRewardsByUserID 获取用户收到的直推奖励记录（分页）
func (r *nodePurchaseRepository) GetDirectRewardsByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*cobo.NodePurchaseEntity, int64, error) {
	var entities []*cobo.NodePurchaseEntity
	var total int64

	model := g.DB().Model("cobo_node_purchase").
		Ctx(ctx).
		Where("direct_reward_user_id = ? AND direct_reward_amount > 0", userID)

	count, err := model.Count()
	if err != nil {
		return nil, 0, err
	}
	total = int64(count)

	err = model.
		Order("created_at DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Scan(&entities)

	if err != nil {
		return nil, 0, err
	}

	return entities, total, nil
}
