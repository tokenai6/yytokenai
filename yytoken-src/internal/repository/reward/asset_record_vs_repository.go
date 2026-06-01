package reward

import (
	"context"

	rewardDao "XWFrame/internal/dao/reward"
	rewardEntity "XWFrame/internal/entity/reward"

	"github.com/gogf/gf/v2/database/gdb"
)

// IAssetRecordVsRepository asset_record_vs 仓储接口
type IAssetRecordVsRepository interface {
	// Create 创建资金记录（可在事务中调用）
	Create(ctx context.Context, tx gdb.TX, record *rewardEntity.AssetRecordEntity) error

	// GetPagedIncomeByUserID 分页查询用户收入记录（用于奖励明细）
	GetPagedIncomeByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*rewardEntity.AssetRecordEntity, int, error)

	// GetPagedInviteRewardByUserID 分页查询直推奖励记录（用于节点购买记录）
	GetPagedInviteRewardByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*rewardEntity.AssetRecordEntity, int, error)
}

type assetRecordVsRepository struct {
	dao rewardDao.IAssetRecordVsDao
}

func NewAssetRecordVsRepository() IAssetRecordVsRepository {
	return &assetRecordVsRepository{
		dao: rewardDao.NewAssetRecordVsDao(),
	}
}

func (r *assetRecordVsRepository) Create(ctx context.Context, tx gdb.TX, record *rewardEntity.AssetRecordEntity) error {
	return r.dao.Create(ctx, tx, record)
}

func (r *assetRecordVsRepository) GetPagedIncomeByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*rewardEntity.AssetRecordEntity, int, error) {
	return r.dao.GetPagedIncomeByUserID(ctx, userID, page, pageSize)
}

func (r *assetRecordVsRepository) GetPagedInviteRewardByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*rewardEntity.AssetRecordEntity, int, error) {
	return r.dao.GetPagedInviteRewardByUserID(ctx, userID, page, pageSize)
}
