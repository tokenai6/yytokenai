package reward

import (
	"context"

	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// IRewardVsRecordDao reward_vs_record 数据访问接口（只包含基础增删改查）
type IRewardVsRecordDao interface {
	// Create 创建记录（可在事务中调用）
	Create(ctx context.Context, tx gdb.TX, record *rewardEntity.RewardVsRecordEntity) error

	// GetPagedByUserID 分页查询用户的VS记录（reward_vs_record）
	GetPagedByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*rewardEntity.RewardVsRecordEntity, int, error)

	// GetPagedList 分页查询VS记录（后台管理用）
	GetPagedList(ctx context.Context, req *GetRewardVsRecordPageReq) ([]*rewardEntity.RewardVsRecordEntity, int, error)
}

// GetRewardVsRecordPageReq 分页查询VS记录请求（后台管理用）
type GetRewardVsRecordPageReq struct {
	Page     int
	PageSize int
	UserIDs  []int64
	Types    *int  // 可选：0=购买，1=直推
	Claim    *bool // 可选：是否已领取
}

type rewardVsRecordDao struct {
	db gdb.DB
}

// NewRewardVsRecordDao 创建 reward_vs_record DAO
func NewRewardVsRecordDao() IRewardVsRecordDao {
	return &rewardVsRecordDao{
		db: db.GetDB(),
	}
}

func (d *rewardVsRecordDao) Create(ctx context.Context, tx gdb.TX, record *rewardEntity.RewardVsRecordEntity) error {
	model := d.db.Model("reward_vs_record")
	if tx != nil {
		model = tx.Model("reward_vs_record")
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

func (d *rewardVsRecordDao) GetPagedByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*rewardEntity.RewardVsRecordEntity, int, error) {
	if userID <= 0 {
		return []*rewardEntity.RewardVsRecordEntity{}, 0, nil
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := d.db.Model("reward_vs_record").Ctx(ctx).
		Where("user_id = ?", userID).
		OrderDesc("created_at").
		OrderDesc("id")

	total, err := query.Count()
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*rewardEntity.RewardVsRecordEntity{}, 0, nil
	}

	var list []*rewardEntity.RewardVsRecordEntity
	if err := query.Page(page, pageSize).Scan(&list); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (d *rewardVsRecordDao) GetPagedList(ctx context.Context, req *GetRewardVsRecordPageReq) ([]*rewardEntity.RewardVsRecordEntity, int, error) {
	if req == nil {
		req = &GetRewardVsRecordPageReq{}
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	query := d.db.Model("reward_vs_record").Ctx(ctx)

	if len(req.UserIDs) > 0 {
		query = query.WhereIn("user_id", req.UserIDs)
	}
	if req.Types != nil {
		query = query.Where("types = ?", *req.Types)
	}
	if req.Claim != nil {
		query = query.Where("claim = ?", *req.Claim)
	}

	total, err := query.Count()
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*rewardEntity.RewardVsRecordEntity{}, 0, nil
	}

	var list []*rewardEntity.RewardVsRecordEntity
	err = query.
		OrderDesc("created_at").
		OrderDesc("id").
		Page(req.Page, req.PageSize).
		Scan(&list)
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

