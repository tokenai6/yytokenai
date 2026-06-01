package dao

import (
	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/shopspring/decimal"
)

// ApgBurnSnapshotDao APG销毁总量快照DAO
type ApgBurnSnapshotDao struct {
	table string
	db    gdb.DB
}

// NewApgBurnSnapshotDao 创建APG销毁总量快照DAO
func NewApgBurnSnapshotDao() *ApgBurnSnapshotDao {
	return &ApgBurnSnapshotDao{
		table: "apg_burn_snapshots",
		db:    db.GetDB(),
	}
}

// Insert 插入快照记录
func (d *ApgBurnSnapshotDao) Insert(ctx context.Context, snapshot *entity.ApgBurnSnapshot) error {
	// 排除自增ID字段，让数据库自动生成
	_, err := d.db.Model(d.table).Ctx(ctx).FieldsEx("id").Insert(snapshot)
	return err
}

// GetLatestBefore 获取指定时间之前的最新快照
func (d *ApgBurnSnapshotDao) GetLatestBefore(ctx context.Context, snapshotTime time.Time) (*entity.ApgBurnSnapshot, error) {
	var snapshot entity.ApgBurnSnapshot
	err := d.db.Model(d.table).Ctx(ctx).
		Where("snapshot_time <= ?", snapshotTime).
		Order("snapshot_time DESC").
		Limit(1).
		Scan(&snapshot)

	if err != nil {
		return nil, err
	}

	return &snapshot, nil
}

// GetLatest 获取最新快照
func (d *ApgBurnSnapshotDao) GetLatest(ctx context.Context) (*entity.ApgBurnSnapshot, error) {
	var snapshot entity.ApgBurnSnapshot
	err := d.db.Model(d.table).Ctx(ctx).
		Order("snapshot_time DESC").
		Limit(1).
		Scan(&snapshot)

	if err != nil {
		return nil, err
	}

	return &snapshot, nil
}

// SaveSnapshot 保存快照（便捷方法）
func (d *ApgBurnSnapshotDao) SaveSnapshot(ctx context.Context, totalBurned decimal.Decimal, source string) error {
	snapshot := &entity.ApgBurnSnapshot{
		TotalBurned:  totalBurned,
		SnapshotTime: time.Now(),
		Source:       source,
		CreatedAt:    time.Now(),
	}

	return d.Insert(ctx, snapshot)
}
