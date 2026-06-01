package vns

import (
	"context"
	"database/sql"
	"errors"

	"XWFrame/internal/entity/vns"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// IUserVnsHomeDataDao VNS首页数据访问接口
type IUserVnsHomeDataDao interface {
	// GetByUserID 根据用户ID获取VNS首页数据
	GetByUserID(ctx context.Context, userID int64) (*vns.UserVnsHomeDataEntity, error)

	// GetByUserIDForUpdate 根据用户ID获取VNS首页数据并加锁（用于事务）
	GetByUserIDForUpdate(ctx context.Context, tx gdb.TX, userID int64) (*vns.UserVnsHomeDataEntity, error)

	// Create 创建VNS首页数据
	Create(ctx context.Context, tx gdb.TX, data *vns.UserVnsHomeDataEntity) error

	// Update 更新VNS首页数据
	Update(ctx context.Context, tx gdb.TX, userID int64, data map[string]interface{}) error

	// Upsert 插入或更新VNS首页数据（如果不存在则创建，存在则更新）
	Upsert(ctx context.Context, tx gdb.TX, data *vns.UserVnsHomeDataEntity) error

	// GetBaseFieldsByUserIDs 批量查询 VNS首页数据基础字段（仅 user_id/admin_level/reward_share/vip_level）
	GetBaseFieldsByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*vns.UserVnsHomeDataEntity, error)

	// GetPagedList 分页查询VNS首页数据
	GetPagedList(ctx context.Context, req *GetVnsHomeDataPageReq) ([]*vns.UserVnsHomeDataEntity, int, error)
}

// GetVnsHomeDataPageReq 分页查询请求
type GetVnsHomeDataPageReq struct {
	Page    int
	PageSize int
	UserIDs []int64
}

// userVnsHomeDataDao VNS首页数据访问实现
type userVnsHomeDataDao struct {
	db gdb.DB
}

// NewUserVnsHomeDataDao 创建VNS首页数据访问实例
func NewUserVnsHomeDataDao() IUserVnsHomeDataDao {
	return &userVnsHomeDataDao{
		db: db.GetDB(),
	}
}

// GetByUserID 根据用户ID获取VNS首页数据
func (d *userVnsHomeDataDao) GetByUserID(ctx context.Context, userID int64) (*vns.UserVnsHomeDataEntity, error) {
	var data vns.UserVnsHomeDataEntity
	err := d.db.Model("user_vns_home_data").Ctx(ctx).
		Where("user_id", userID).
		Scan(&data)
	if err != nil {
		// 如果查询不到记录，返回 nil, nil（记录不存在，这是正常情况）
		if errors.Is(err, sql.ErrNoRows) || err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	if data.Id == 0 {
		return nil, nil
	}
	return &data, nil
}

func (d *userVnsHomeDataDao) GetBaseFieldsByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*vns.UserVnsHomeDataEntity, error) {
	result := make(map[int64]*vns.UserVnsHomeDataEntity)
	if len(userIDs) == 0 {
		return result, nil
	}

	// 分批 IN，避免 PostgreSQL 参数上限
	const batchSize = 5000

	for i := 0; i < len(userIDs); i += batchSize {
		end := i + batchSize
		if end > len(userIDs) {
			end = len(userIDs)
		}
		batch := userIDs[i:end]

		var list []*vns.UserVnsHomeDataEntity
		if err := d.db.Model("user_vns_home_data").Ctx(ctx).
			Fields("user_id", "admin_level", "reward_share", "vip_level").
			WhereIn("user_id", batch).
			Scan(&list); err != nil {
			return nil, err
		}

		for _, item := range list {
			if item == nil || item.UserId <= 0 {
				continue
			}
			result[item.UserId] = item
		}
	}

	return result, nil
}

// GetByUserIDForUpdate 根据用户ID获取VNS首页数据并加锁（用于事务）
func (d *userVnsHomeDataDao) GetByUserIDForUpdate(ctx context.Context, tx gdb.TX, userID int64) (*vns.UserVnsHomeDataEntity, error) {
	if tx == nil {
		return nil, errors.New("transaction is required for GetByUserIDForUpdate")
	}

	var data vns.UserVnsHomeDataEntity
	err := tx.Model("user_vns_home_data").
		Where("user_id", userID).
		LockUpdate().
		Scan(&data)
	if err != nil {
		// 如果查询不到记录，返回 nil, nil（记录不存在，这是正常情况）
		if errors.Is(err, sql.ErrNoRows) || err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	if data.Id == 0 {
		return nil, nil
	}
	return &data, nil
}

// Create 创建VNS首页数据
func (d *userVnsHomeDataDao) Create(ctx context.Context, tx gdb.TX, data *vns.UserVnsHomeDataEntity) error {
	model := d.db.Model("user_vns_home_data")
	if tx != nil {
		model = tx.Model("user_vns_home_data")
	}

	// PostgreSQL 驱动不支持 LastInsertId，避免使用 InsertAndGetId。
	_, err := model.Ctx(ctx).
		FieldsEx("id", "created_at", "updated_at").
		Data(data).
		Insert()
	if err != nil {
		return err
	}
	return nil
}

// Update 更新VNS首页数据
func (d *userVnsHomeDataDao) Update(ctx context.Context, tx gdb.TX, userID int64, data map[string]interface{}) error {
	model := d.db.Model("user_vns_home_data")
	if tx != nil {
		model = tx.Model("user_vns_home_data")
	}

	_, err := model.Ctx(ctx).
		Where("user_id", userID).
		Data(data).
		Update()
	return err
}

// Upsert 插入或更新VNS首页数据（如果不存在则创建，存在则更新）
func (d *userVnsHomeDataDao) Upsert(ctx context.Context, tx gdb.TX, data *vns.UserVnsHomeDataEntity) error {
	// 先查询是否存在
	existing, err := d.GetByUserID(ctx, data.UserId)
	if err != nil {
		return err
	}

	if existing == nil {
		// 不存在，创建
		return d.Create(ctx, tx, data)
	} else {
		// 存在，更新
		updateData := map[string]interface{}{
			"vip_level":                   data.VipLevel,
			"admin_level":                 data.AdminLevel,
			"team_total_performance":      data.TeamTotalPerformance,
			"next_level_upgrade_required": data.NextLevelUpgradeRequired,
			"team_member_count":           data.TeamMemberCount,
			"direct_member_count":         data.DirectMemberCount,
			"max_district_count":          data.MaxDistrictCount,
			"min_district_count":          data.MinDistrictCount,
			"min_district_performance":    data.MinDistrictPerformance,
			"reward_share":                data.RewardShare,
			"vs_balance":                  data.VsBalance,
			"usdt_balance":                data.UsdtBalance,
		}
		if err := d.Update(ctx, tx, data.UserId, updateData); err != nil {
			return err
		}
		data.Id = existing.Id
		return nil
	}
}

// GetPagedList 分页查询VNS首页数据
func (d *userVnsHomeDataDao) GetPagedList(ctx context.Context, req *GetVnsHomeDataPageReq) ([]*vns.UserVnsHomeDataEntity, int, error) {
	model := d.db.Model("user_vns_home_data").Ctx(ctx)

	// 如果有指定用户ID，添加过滤条件
	if len(req.UserIDs) > 0 {
		model = model.WhereIn("user_id", req.UserIDs)
	}

	// 查询总数
	total, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	var list []*vns.UserVnsHomeDataEntity
	err = model.
		Order("team_total_performance DESC, id DESC").
		Page(req.Page, req.PageSize).
		Scan(&list)
	if err != nil {
		return nil, 0, err
	}

	return list, total, nil
}
