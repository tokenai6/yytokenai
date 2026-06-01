package reward

import (
	"context"
	"fmt"
	"time"

	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// ISettlementDetailDao 结算明细表数据访问接口
type ISettlementDetailDao interface {
	// Create 创建结算明细
	Create(ctx context.Context, tx gdb.TX, detail *rewardEntity.SettlementDetailEntity) error

	// BatchCreate 批量创建结算明细
	BatchCreate(ctx context.Context, tx gdb.TX, details []*rewardEntity.SettlementDetailEntity) error

	// GetByDate 根据日期查询所有结算明细
	GetByDate(ctx context.Context, date time.Time) ([]*rewardEntity.SettlementDetailEntity, error)

	// GetBySettlementID 根据结算ID查询明细
	GetBySettlementID(ctx context.Context, settlementID int64) ([]*rewardEntity.SettlementDetailEntity, error)

	// CheckExists 检查来源记录是否已结算
	CheckExists(ctx context.Context, sourceTable string, sourceID int64) (bool, error)

	// BatchCheckExists 批量检查来源记录是否已结算（返回已结算的Map）
	BatchCheckExists(ctx context.Context, sourceTable string, sourceIDs []int64, date time.Time) (map[int64]bool, error)

	// GetSettledMapByDate 获取某日所有已结算记录的Map（source_table:source_id → true）
	GetSettledMapByDate(ctx context.Context, date time.Time) (map[string]bool, error)
}

// settlementDetailDao 结算明细表数据访问实现
type settlementDetailDao struct {
	db gdb.DB
}

// NewSettlementDetailDao 创建结算明细表数据访问实例
func NewSettlementDetailDao() ISettlementDetailDao {
	return &settlementDetailDao{
		db: db.GetDB(),
	}
}

// Create 创建结算明细
func (d *settlementDetailDao) Create(ctx context.Context, tx gdb.TX, detail *rewardEntity.SettlementDetailEntity) error {
	model := d.db.Model("settlement_detail")
	if tx != nil {
		model = tx.Model("settlement_detail")
	}

	result, err := model.Ctx(ctx).
		FieldsEx("id", "created_at").
		Data(detail).
		InsertAndGetId()
	if err != nil {
		return err
	}
	detail.Id = result
	return nil
}

// BatchCreate 批量创建结算明细
func (d *settlementDetailDao) BatchCreate(ctx context.Context, tx gdb.TX, details []*rewardEntity.SettlementDetailEntity) error {
	if len(details) == 0 {
		return nil
	}

	model := d.db.Model("settlement_detail")
	if tx != nil {
		model = tx.Model("settlement_detail")
	}

	_, err := model.Ctx(ctx).
		FieldsEx("id", "created_at").
		Data(details).
		Insert()
	return err
}

// GetByDate 根据时刻查询所有结算明细
func (d *settlementDetailDao) GetByDate(ctx context.Context, date time.Time) ([]*rewardEntity.SettlementDetailEntity, error) {
	var details []*rewardEntity.SettlementDetailEntity
	err := d.db.Model("settlement_detail").Ctx(ctx).
		Where("record_time = ?", date).
		Scan(&details)
	if err != nil {
		return nil, err
	}
	return details, nil
}

// GetBySettlementID 根据结算ID查询明细
func (d *settlementDetailDao) GetBySettlementID(ctx context.Context, settlementID int64) ([]*rewardEntity.SettlementDetailEntity, error) {
	var details []*rewardEntity.SettlementDetailEntity
	err := d.db.Model("settlement_detail").Ctx(ctx).
		Where("settlement_id", settlementID).
		Order("id").
		Scan(&details)
	if err != nil {
		return nil, err
	}
	return details, nil
}

// CheckExists 检查来源记录是否已结算
func (d *settlementDetailDao) CheckExists(ctx context.Context, sourceTable string, sourceID int64) (bool, error) {
	count, err := d.db.Model("settlement_detail").Ctx(ctx).
		Where("source_table", sourceTable).
		Where("source_id", sourceID).
		Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// BatchCheckExists 批量检查来源记录是否已结算（返回已结算的Map）
func (d *settlementDetailDao) BatchCheckExists(ctx context.Context, sourceTable string, sourceIDs []int64, date time.Time) (map[int64]bool, error) {
	if len(sourceIDs) == 0 {
		return make(map[int64]bool), nil
	}

	var results []struct {
		SourceID int64 `json:"source_id"`
	}

	err := d.db.Model("settlement_detail").Ctx(ctx).
		Fields("source_id").
		Where("source_table", sourceTable).
		WhereIn("source_id", sourceIDs).
		Where("record_time = ?", date).
		Scan(&results)
	if err != nil {
		return nil, err
	}

	// 构建 Map
	settledMap := make(map[int64]bool)
	for _, result := range results {
		settledMap[result.SourceID] = true
	}

	return settledMap, nil
}

// GetSettledSourceIDsByDate 获取某时刻某表的所有已结算来源ID（用于构建内存Map）
func (d *settlementDetailDao) GetSettledSourceIDsByDate(ctx context.Context, sourceTable string, date time.Time) (map[int64]bool, error) {
	var results []struct {
		SourceID int64 `json:"source_id"`
	}

	err := d.db.Model("settlement_detail").Ctx(ctx).
		Fields("source_id").
		Where("source_table", sourceTable).
		Where("record_time = ?", date).
		Scan(&results)
	if err != nil {
		return nil, err
	}

	// 构建 Map（source_id → true）
	settledMap := make(map[int64]bool)
	for _, result := range results {
		settledMap[result.SourceID] = true
	}

	return settledMap, nil
}

// GetSettledMapByDate 获取某时刻所有已结算记录的Map（source_table:source_id → true）
func (d *settlementDetailDao) GetSettledMapByDate(ctx context.Context, date time.Time) (map[string]bool, error) {
	var results []struct {
		SourceTable string `json:"source_table"`
		SourceID    int64  `json:"source_id"`
	}

	err := d.db.Model("settlement_detail").Ctx(ctx).
		Fields("source_table", "source_id").
		Where("record_time = ?", date).
		Scan(&results)
	if err != nil {
		return nil, err
	}

	// 构建 Map（source_table:source_id → true）
	settledMap := make(map[string]bool)
	for _, result := range results {
		key := fmt.Sprintf("%s:%d", result.SourceTable, result.SourceID)
		settledMap[key] = true
	}

	return settledMap, nil
}
