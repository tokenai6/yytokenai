package reward

import (
	"context"

	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// IAssetRecordVsDao asset_record_vs 数据访问接口（只包含基础增删改查）
type IAssetRecordVsDao interface {
	// Create 创建资金记录（可在事务中调用）
	Create(ctx context.Context, tx gdb.TX, record *rewardEntity.AssetRecordEntity) error

	// GetPagedIncomeByUserID 分页查询用户收入记录（用于奖励明细）
	GetPagedIncomeByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*rewardEntity.AssetRecordEntity, int, error)

	// GetPagedInviteRewardByUserID 分页查询直推奖励记录（用于节点购买记录）
	GetPagedInviteRewardByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*rewardEntity.AssetRecordEntity, int, error)
}

type assetRecordVsDao struct {
	db gdb.DB
}

// NewAssetRecordVsDao 创建 asset_record_vs DAO
func NewAssetRecordVsDao() IAssetRecordVsDao {
	return &assetRecordVsDao{
		db: db.GetDB(),
	}
}

func (d *assetRecordVsDao) Create(ctx context.Context, tx gdb.TX, record *rewardEntity.AssetRecordEntity) error {
	model := d.db.Model("asset_record_vs")
	if tx != nil {
		model = tx.Model("asset_record_vs")
	}
	// PostgreSQL 驱动不支持 LastInsertId，避免使用 InsertAndGetId。
	_, err := model.Ctx(ctx).
		FieldsEx("id", "created_at", "updated_at").
		Data(record).
		Insert()
	if err != nil {
		return err
	}
	return nil
}

func (d *assetRecordVsDao) GetPagedIncomeByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*rewardEntity.AssetRecordEntity, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := d.db.Model("asset_record_vs").Ctx(ctx).
		Where("user_id = ?", userID).
		Where("flow_type = 'income'").
		OrderDesc("record_time")

	total, err := query.Count()
	if err != nil {
		return nil, 0, err
	}

	var list []*rewardEntity.AssetRecordEntity
	if total == 0 {
		return list, 0, nil
	}

	if err := query.Page(page, pageSize).Scan(&list); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (d *assetRecordVsDao) GetPagedInviteRewardByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*rewardEntity.AssetRecordEntity, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := d.db.Model("asset_record_vs").Ctx(ctx).
		Where("user_id = ?", userID).
		Where("flow_type = 'income'").
		// metadata jsonb 过滤：只取直推奖励（购买记录用）
		Where("metadata->>'sub_type' = ?", "invite_reward").
		OrderDesc("id")

	total, err := query.Count()
	if err != nil {
		return nil, 0, err
	}

	var list []*rewardEntity.AssetRecordEntity
	if total == 0 {
		return list, 0, nil
	}

	if err := query.Page(page, pageSize).Scan(&list); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
