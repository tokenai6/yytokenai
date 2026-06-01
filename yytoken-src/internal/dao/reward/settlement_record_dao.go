package reward

import (
	"context"
	"time"

	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// ISettlementRecordDao 结算表数据访问接口
type ISettlementRecordDao interface {
	// Create 创建结算记录
	Create(ctx context.Context, tx gdb.TX, record *rewardEntity.SettlementRecordEntity) error

	// GetByUserAndDate 查询用户某时刻的结算记录
	GetByUserAndDate(ctx context.Context, userID int64, date time.Time) (*rewardEntity.SettlementRecordEntity, error)

	// GetMapByDate 批量查询某时刻的所有结算记录，返回 map[userID]*record
	GetMapByDate(ctx context.Context, date time.Time) (map[int64]*rewardEntity.SettlementRecordEntity, error)

	// Update 更新结算记录
	Update(ctx context.Context, tx gdb.TX, record *rewardEntity.SettlementRecordEntity) error
}

// settlementRecordDao 结算表数据访问实现
type settlementRecordDao struct {
	db gdb.DB
}

// NewSettlementRecordDao 创建结算表数据访问实例
func NewSettlementRecordDao() ISettlementRecordDao {
	return &settlementRecordDao{
		db: db.GetDB(),
	}
}

// Create 创建结算记录
func (d *settlementRecordDao) Create(ctx context.Context, tx gdb.TX, record *rewardEntity.SettlementRecordEntity) error {
	model := d.db.Model("settlement_record")
	if tx != nil {
		model = tx.Model("settlement_record")
	}

	result, err := model.Ctx(ctx).
		FieldsEx("id", "created_at", "updated_at").
		Data(record).
		InsertAndGetId()
	if err != nil {
		return err
	}
	record.Id = result
	return nil
}

// GetByUserAndDate 查询用户某时刻的结算记录
func (d *settlementRecordDao) GetByUserAndDate(ctx context.Context, userID int64, date time.Time) (*rewardEntity.SettlementRecordEntity, error) {
	var record rewardEntity.SettlementRecordEntity
	err := d.db.Model("settlement_record").Ctx(ctx).
		Where("user_id", userID).
		Where("record_time = ?", date).
		Scan(&record)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	if record.Id == 0 {
		return nil, nil
	}
	return &record, nil
}

// GetMapByDate 批量查询某时刻的所有结算记录，返回 map[userID]*record
func (d *settlementRecordDao) GetMapByDate(ctx context.Context, date time.Time) (map[int64]*rewardEntity.SettlementRecordEntity, error) {
	var records []*rewardEntity.SettlementRecordEntity
	err := d.db.Model("settlement_record").Ctx(ctx).
		Where("record_time = ?", date).
		Scan(&records)
	if err != nil {
		return nil, err
	}

	recordMap := make(map[int64]*rewardEntity.SettlementRecordEntity)
	for _, record := range records {
		recordMap[record.UserID] = record
	}
	return recordMap, nil
}

// Update 更新结算记录
func (d *settlementRecordDao) Update(ctx context.Context, tx gdb.TX, record *rewardEntity.SettlementRecordEntity) error {
	model := d.db.Model("settlement_record")
	if tx != nil {
		model = tx.Model("settlement_record")
	}

	_, err := model.Ctx(ctx).
		Where("id", record.Id).
		Data(record).
		Update()
	return err
}
