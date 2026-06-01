package reward

import (
	"context"

	rewardEntity "XWFrame/internal/entity/reward"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// IApgPriceStabilizeLogDao APG价格调控日志DAO接口
type IApgPriceStabilizeLogDao interface {
	// Create 创建调控日志
	Create(ctx context.Context, tx gdb.TX, entity *rewardEntity.ApgPriceStabilizeLogEntity) error

	// GetByTxHash 通过交易哈希查询
	GetByTxHash(ctx context.Context, txHash string) (*rewardEntity.ApgPriceStabilizeLogEntity, error)

	// UpdateStatus 更新状态
	UpdateStatus(ctx context.Context, tx gdb.TX, id int64, status string, errorMsg string) error

	// GetLatest 获取最新记录
	GetLatest(ctx context.Context) (*rewardEntity.ApgPriceStabilizeLogEntity, error)

	// GetPendingRecords 获取pending状态的记录
	GetPendingRecords(ctx context.Context) ([]*rewardEntity.ApgPriceStabilizeLogEntity, error)

	// UpdateReceiptInfo 更新交易回执信息
	UpdateReceiptInfo(ctx context.Context, tx gdb.TX, txHash string, blockNumber, gasUsed, gasPrice int64) error

	// UpdatePriceAfter 更新调控后价格
	UpdatePriceAfter(ctx context.Context, tx gdb.TX, txHash string, priceAfter decimal.Decimal) error
}

type apgPriceStabilizeLogDao struct{}

func NewApgPriceStabilizeLogDao() IApgPriceStabilizeLogDao {
	return &apgPriceStabilizeLogDao{}
}

// Create 创建调控日志
func (d *apgPriceStabilizeLogDao) Create(ctx context.Context, tx gdb.TX, entity *rewardEntity.ApgPriceStabilizeLogEntity) error {
	var db gdb.DB
	if tx != nil {
		db = tx.GetDB()
	} else {
		db = g.DB()
	}

	result, err := db.Model("apg_price_stabilize_log").Ctx(ctx).
		FieldsEx("id").
		InsertAndGetId(entity)
	if err != nil {
		return err
	}
	entity.Id = result
	return nil
}

// GetByTxHash 通过交易哈希查询
func (d *apgPriceStabilizeLogDao) GetByTxHash(ctx context.Context, txHash string) (*rewardEntity.ApgPriceStabilizeLogEntity, error) {
	var entity rewardEntity.ApgPriceStabilizeLogEntity
	err := g.DB().Model("apg_price_stabilize_log").Ctx(ctx).
		Where("tx_hash = ?", txHash).
		Scan(&entity)

	if err != nil {
		return nil, err
	}

	return &entity, nil
}

// UpdateStatus 更新状态
func (d *apgPriceStabilizeLogDao) UpdateStatus(ctx context.Context, tx gdb.TX, id int64, status string, errorMsg string) error {
	var db gdb.DB
	if tx != nil {
		db = tx.GetDB()
	} else {
		db = g.DB()
	}

	data := g.Map{
		"status": status,
	}

	if errorMsg != "" {
		data["error_msg"] = errorMsg
	}

	_, err := db.Model("apg_price_stabilize_log").Ctx(ctx).
		Where("id = ?", id).
		Update(data)

	return err
}

// GetLatest 获取最新记录
func (d *apgPriceStabilizeLogDao) GetLatest(ctx context.Context) (*rewardEntity.ApgPriceStabilizeLogEntity, error) {
	var entity rewardEntity.ApgPriceStabilizeLogEntity
	err := g.DB().Model("apg_price_stabilize_log").Ctx(ctx).
		OrderDesc("created_at").
		Limit(1).
		Scan(&entity)

	if err != nil {
		return nil, err
	}

	return &entity, nil
}

// GetPendingRecords 获取pending状态的记录
func (d *apgPriceStabilizeLogDao) GetPendingRecords(ctx context.Context) ([]*rewardEntity.ApgPriceStabilizeLogEntity, error) {
	var entities []*rewardEntity.ApgPriceStabilizeLogEntity
	err := g.DB().Model("apg_price_stabilize_log").Ctx(ctx).
		Where("status = ?", "pending").
		OrderAsc("created_at").
		Scan(&entities)

	if err != nil {
		return nil, err
	}

	return entities, nil
}

// UpdateReceiptInfo 更新交易回执信息
func (d *apgPriceStabilizeLogDao) UpdateReceiptInfo(ctx context.Context, tx gdb.TX, txHash string, blockNumber, gasUsed, gasPrice int64) error {
	var db gdb.DB
	if tx != nil {
		db = tx.GetDB()
	} else {
		db = g.DB()
	}

	data := g.Map{
		"status":        "success",
		"block_number":  blockNumber,
		"gas_used":      gasUsed,
		"gas_price":     gasPrice,
		"confirmed_at":  "NOW()",
	}

	_, err := db.Model("apg_price_stabilize_log").Ctx(ctx).
		Where("tx_hash = ?", txHash).
		Update(data)

	return err
}

// UpdatePriceAfter 更新调控后价格
func (d *apgPriceStabilizeLogDao) UpdatePriceAfter(ctx context.Context, tx gdb.TX, txHash string, priceAfter decimal.Decimal) error {
	var db gdb.DB
	if tx != nil {
		db = tx.GetDB()
	} else {
		db = g.DB()
	}

	_, err := db.Model("apg_price_stabilize_log").Ctx(ctx).
		Where("tx_hash = ?", txHash).
		Update(g.Map{"apg_price_after": priceAfter})

	return err
}
