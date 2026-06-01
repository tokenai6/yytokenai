package dao

import (
	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// NodePurchaseRecordDao 节点购买记录DAO
type NodePurchaseRecordDao struct {
	db gdb.DB
}

// NewNodePurchaseRecordDao 创建节点购买记录DAO
func NewNodePurchaseRecordDao() *NodePurchaseRecordDao {
	return &NodePurchaseRecordDao{
		db: db.GetDB(),
	}
}

// GetAll 全量查询节点购买记录
func (d *NodePurchaseRecordDao) GetAll(ctx context.Context) ([]*entity.NodePurchaseRecordEntity, error) {
	var entities []*entity.NodePurchaseRecordEntity
	err := d.db.Model("node_purchase_record").Ctx(ctx).Order("id ASC").Scan(&entities)
	if err != nil {
		return nil, err
	}
	return entities, nil
}

// Create 创建购买记录
func (d *NodePurchaseRecordDao) Create(ctx context.Context, entity *entity.NodePurchaseRecordEntity) error {
	// 设置创建和更新时间
	now := time.Now()
	entity.CreatedAt = now
	entity.UpdatedAt = now

	_, err := d.db.Model("node_purchase_record").FieldsEx("id", "created_at", "updated_at").Data(entity).Insert()
	return err
}

// GetByEventId 根据event_id查询（防重）
func (d *NodePurchaseRecordDao) GetByEventId(ctx context.Context, eventID string) (*entity.NodePurchaseRecordEntity, error) {
	// 使用All方法获取结果，然后手动转换类型
	result, err := d.db.Model("node_purchase_record").Where("event_id = ?", eventID).All()
	if err != nil {
		if g.IsNil(err) {
			return nil, nil
		}
		return nil, err
	}

	if len(result) == 0 {
		return nil, nil
	}

	// 手动转换数据
	record := result[0]

	// 检查记录是否有效
	if record == nil {
		return nil, nil
	}

	purchaseEntity := &entity.NodePurchaseRecordEntity{
		UserID:          record["user_id"].Int64(),
		UserAddress:     record["user_address"].String(),
		FromAddress:     record["from_address"].String(),
		Amount:          record["amount"].String(),
		NodeLevel:       record["node_level"].String(),
		PowerMultiplier: record["power_multiplier"].Float64(),
		PowerValue:      record["power_value"].Int64(),
		TransactionHash: record["transaction_hash"].String(),
		TransactionTime: record["transaction_time"].Time(),
		EventID:         record["event_id"].String(),
		TransactionID:   record["transaction_id"].String(),
		BlockNumber:     record["block_number"].Int64(),
	}

	// 设置基础字段
	purchaseEntity.Id = record["id"].Int64()
	purchaseEntity.CreatedAt = record["created_at"].Time()
	purchaseEntity.UpdatedAt = record["updated_at"].Time()

	return purchaseEntity, nil
}

// GetByUserId 查询用户购买记录
func (d *NodePurchaseRecordDao) GetByUserId(ctx context.Context, userID int64) ([]*entity.NodePurchaseRecordEntity, error) {
	var entities []*entity.NodePurchaseRecordEntity
	err := d.db.Model("node_purchase_record").Where("user_id = ?", userID).Order("transaction_time DESC").Scan(&entities)
	if err != nil {
		return nil, err
	}
	return entities, nil
}

// GetByParentUsers 查询直推用户的购买记录
func (d *NodePurchaseRecordDao) GetByParentUsers(ctx context.Context, parentUserIDs []int64) ([]*entity.NodePurchaseRecordEntity, error) {
	var entities []*entity.NodePurchaseRecordEntity

	// 如果没有用户ID，返回空结果
	if len(parentUserIDs) == 0 {
		return entities, nil
	}

	// 构建IN查询的占位符
	placeholders := make([]string, len(parentUserIDs))
	args := make([]interface{}, len(parentUserIDs))
	for i, userID := range parentUserIDs {
		placeholders[i] = "?"
		args[i] = userID
	}

	whereClause := "user_id IN (" + strings.Join(placeholders, ",") + ")"
	err := d.db.Model("node_purchase_record").Where(whereClause, args...).Order("transaction_time DESC").Scan(&entities)
	if err != nil {
		return nil, err
	}
	return entities, nil
}
