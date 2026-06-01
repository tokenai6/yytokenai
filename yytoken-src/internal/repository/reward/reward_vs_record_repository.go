package reward

import (
	"context"

	rewardDao "XWFrame/internal/dao/reward"
	rewardEntity "XWFrame/internal/entity/reward"

	"github.com/gogf/gf/v2/database/gdb"
)

// IRewardVsRecordRepository reward_vs_record 仓储接口
type IRewardVsRecordRepository interface {
	// Create 创建记录（可在事务中调用）
	Create(ctx context.Context, tx gdb.TX, record *rewardEntity.RewardVsRecordEntity) error

	// GetPagedByUserID 分页查询用户的VS记录
	GetPagedByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*rewardEntity.RewardVsRecordEntity, int, error)

	// GetPagedList 分页查询VS记录（后台管理用）
	GetPagedList(ctx context.Context, req *rewardDao.GetRewardVsRecordPageReq) ([]*rewardEntity.RewardVsRecordEntity, int, error)
}

type rewardVsRecordRepository struct {
	dao rewardDao.IRewardVsRecordDao
}

func NewRewardVsRecordRepository() IRewardVsRecordRepository {
	return &rewardVsRecordRepository{
		dao: rewardDao.NewRewardVsRecordDao(),
	}
}

func (r *rewardVsRecordRepository) Create(ctx context.Context, tx gdb.TX, record *rewardEntity.RewardVsRecordEntity) error {
	return r.dao.Create(ctx, tx, record)
}

func (r *rewardVsRecordRepository) GetPagedByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*rewardEntity.RewardVsRecordEntity, int, error) {
	return r.dao.GetPagedByUserID(ctx, userID, page, pageSize)
}

func (r *rewardVsRecordRepository) GetPagedList(ctx context.Context, req *rewardDao.GetRewardVsRecordPageReq) ([]*rewardEntity.RewardVsRecordEntity, int, error) {
	return r.dao.GetPagedList(ctx, req)
}

