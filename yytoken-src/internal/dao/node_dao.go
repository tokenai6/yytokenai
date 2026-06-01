package dao

import (
	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// NodeDao 节点信息DAO
type NodeDao struct {
	db gdb.DB
}

// NewNodeDao 创建节点信息DAO
func NewNodeDao() *NodeDao {
	return &NodeDao{
		db: db.GetDB(),
	}
}

// GetById 根据ID查询
func (d *NodeDao) GetById(ctx context.Context, id int64) (*entity.NodeEntity, error) {
	// 使用All方法获取结果，然后手动转换类型
	result, err := d.db.Model("node_info").Where("id = ?", id).All()
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
	nodeEntity := &entity.NodeEntity{
		NodeLevel:       record["node_level"].String(),
		Status:          record["status"].Int(),
		PowerMultiplier: record["power_multiplier"].Float64(),
		TotalShares:     record["total_shares"].Int64(),
		SoldShares:      record["sold_shares"].Int64(),
		MinGrowth:       record["min_growth"].Int64(),
		MaxGrowth:       record["max_growth"].Int64(),
	}

	// 设置基础字段
	nodeEntity.Id = record["id"].Int64()
	nodeEntity.CreatedAt = record["created_at"].Time()
	nodeEntity.UpdatedAt = record["updated_at"].Time()

	// 处理可选的时间字段
	if !record["start_time"].IsNil() {
		startTime := record["start_time"].Time()
		nodeEntity.StartTime = &startTime
	}
	if !record["end_time"].IsNil() {
		endTime := record["end_time"].Time()
		nodeEntity.EndTime = &endTime
	}

	return nodeEntity, nil
}

// GetList 查询列表
func (d *NodeDao) GetList(ctx context.Context, limit, offset int) ([]*entity.NodeEntity, error) {
	// 使用All方法获取结果，然后手动转换类型
	result, err := d.db.Model("node_info").Limit(limit).Offset(offset).Order("created_at DESC").All()
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return []*entity.NodeEntity{}, nil
	}

	var entities []*entity.NodeEntity
	for _, record := range result {
		nodeEntity := &entity.NodeEntity{
			NodeLevel:       record["node_level"].String(),
			Status:          record["status"].Int(),
			PowerMultiplier: record["power_multiplier"].Float64(),
			TotalShares:     record["total_shares"].Int64(),
			SoldShares:      record["sold_shares"].Int64(),
			MinGrowth:       record["min_growth"].Int64(),
			MaxGrowth:       record["max_growth"].Int64(),
		}

		// 设置基础字段
		nodeEntity.Id = record["id"].Int64()
		nodeEntity.CreatedAt = record["created_at"].Time()
		nodeEntity.UpdatedAt = record["updated_at"].Time()

		// 处理可选的时间字段
		if !record["start_time"].IsNil() {
			startTime := record["start_time"].Time()
			nodeEntity.StartTime = &startTime
		}
		if !record["end_time"].IsNil() {
			endTime := record["end_time"].Time()
			nodeEntity.EndTime = &endTime
		}

		entities = append(entities, nodeEntity)
	}

	return entities, nil
}

// GetOnSaleNode 查询在售节点（第一个）
func (d *NodeDao) GetOnSaleNode(ctx context.Context) (*entity.NodeEntity, error) {
	// 使用All方法获取结果，然后手动转换类型
	result, err := d.db.Model("node_info").Where("status = ?", 2).All()
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

	nodeEntity := &entity.NodeEntity{
		NodeLevel:       record["node_level"].String(),
		Status:          record["status"].Int(),
		PowerMultiplier: record["power_multiplier"].Float64(),
		TotalShares:     record["total_shares"].Int64(),
		SoldShares:      record["sold_shares"].Int64(),
		MinGrowth:       record["min_growth"].Int64(),
		MaxGrowth:       record["max_growth"].Int64(),
	}

	// 设置基础字段
	nodeEntity.Id = record["id"].Int64()
	nodeEntity.CreatedAt = record["created_at"].Time()
	nodeEntity.UpdatedAt = record["updated_at"].Time()

	// 处理可选的时间字段
	if !record["start_time"].IsNil() {
		startTime := record["start_time"].Time()
		nodeEntity.StartTime = &startTime
	}
	if !record["end_time"].IsNil() {
		endTime := record["end_time"].Time()
		nodeEntity.EndTime = &endTime
	}

	return nodeEntity, nil
}

// UpdateSoldShares 更新已销售份额
func (d *NodeDao) UpdateSoldShares(ctx context.Context, id int64, soldShares int64) error {
	_, err := d.db.Model("node_info").Where("id = ?", id).Data(map[string]interface{}{"sold_shares": soldShares}).Update()
	return err
}

// UpdateStatus 更新状态
func (d *NodeDao) UpdateStatus(ctx context.Context, id int64, status int) error {
	_, err := d.db.Model("node_info").Where("id = ?", id).Data(map[string]interface{}{"status": status}).Update()
	return err
}
