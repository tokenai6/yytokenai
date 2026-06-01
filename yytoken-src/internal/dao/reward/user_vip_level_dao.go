package reward

import (
	"context"

	"XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// IUserVipLevelDao VIP等级数据访问接口
type IUserVipLevelDao interface {
	// Create 创建VIP等级记录
	Create(ctx context.Context, tx gdb.TX, vip *reward.UserVipLevelEntity) error

	// Update 更新VIP等级记录
	Update(ctx context.Context, tx gdb.TX, vip *reward.UserVipLevelEntity) error

	// GetCurrentVIP 获取用户当前VIP等级
	GetCurrentVIP(ctx context.Context, userID int64) (*reward.UserVipLevelEntity, error)

	// SetNotCurrent 将指定记录设为非当前
	SetNotCurrent(ctx context.Context, tx gdb.TX, id int64) error

	// GetAllCurrent 获取所有当前VIP等级
	GetAllCurrent(ctx context.Context) ([]*reward.UserVipLevelEntity, error)

	// BatchGetCurrentVIPs 批量获取用户当前VIP等级
	BatchGetCurrentVIPs(ctx context.Context, userIDs []int64) ([]*reward.UserVipLevelEntity, error)

	// GetCurrentVipLevelsByUserIDs 根据用户ID列表获取当前VIP等级记录
	GetCurrentVipLevelsByUserIDs(ctx context.Context, userIDs []int64) ([]*reward.UserVipLevelEntity, error)

	// GetUserVipLevelByOffset 按offset获取用户VIP等级记录
	// userID: 用户ID，如果为0表示查询所有用户
	// offset: 偏移量指针，若为nil表示不限制偏移，返回所有匹配记录；否则按指定offset返回
	// 返回所有匹配条件的VIP等级记录（按record_time倒序）
	GetUserVipLevelByOffset(ctx context.Context, userID int64, offset *int) ([]*reward.UserVipLevelEntity, error)

	// BatchCreate 批量创建VIP等级记录
	BatchCreate(ctx context.Context, tx gdb.TX, vips []*reward.UserVipLevelEntity) error
}

// userVipLevelDao VIP等级数据访问实现
type userVipLevelDao struct {
	db gdb.DB
}

// NewUserVipLevelDao 创建VIP等级数据访问实例
func NewUserVipLevelDao() IUserVipLevelDao {
	return &userVipLevelDao{
		db: db.GetDB(),
	}
}

// Create 创建VIP等级记录
func (d *userVipLevelDao) Create(ctx context.Context, tx gdb.TX, vip *reward.UserVipLevelEntity) error {
	model := d.db.Model("user_vip_level")
	if tx != nil {
		model = tx.Model("user_vip_level")
	}

	result, err := model.Ctx(ctx).
		Data(map[string]interface{}{
			"user_id":              vip.UserID,
			"vip_level":            vip.VipLevel,
			"district_performance": vip.DistrictPerformance,
			"reward_rate":          vip.RewardRate,
			"record_time":          vip.RecordTime,
			"prev_vip_level":       vip.PrevVipLevel,
			"is_current":           vip.IsCurrent,
		}).
		InsertAndGetId()
	if err != nil {
		return err
	}
	vip.Id = result
	return nil
}

// Update 更新VIP等级记录
func (d *userVipLevelDao) Update(ctx context.Context, tx gdb.TX, vip *reward.UserVipLevelEntity) error {
	model := d.db.Model("user_vip_level")
	if tx != nil {
		model = tx.Model("user_vip_level")
	}

	_, err := model.Ctx(ctx).
		Where("id", vip.Id).
		Data(map[string]interface{}{
			"user_id":              vip.UserID,
			"vip_level":            vip.VipLevel,
			"district_performance": vip.DistrictPerformance,
			"reward_rate":          vip.RewardRate,
			"record_time":          vip.RecordTime,
			"prev_vip_level":       vip.PrevVipLevel,
			"is_current":           vip.IsCurrent,
		}).
		Update()
	return err
}

// GetCurrentVIP 获取用户当前VIP等级
func (d *userVipLevelDao) GetCurrentVIP(ctx context.Context, userID int64) (*reward.UserVipLevelEntity, error) {
	var vip reward.UserVipLevelEntity
	err := d.db.Model("user_vip_level").Ctx(ctx).
		Fields("id,user_id,vip_level,district_performance,reward_rate,record_time,prev_vip_level,is_current,created_at,updated_at").
		Where("user_id", userID).
		Where("is_current", true).
		Scan(&vip)
	if err != nil {
		return nil, err
	}
	if vip.Id == 0 {
		return nil, nil
	}
	return &vip, nil
}

// SetNotCurrent 将指定记录设为非当前
func (d *userVipLevelDao) SetNotCurrent(ctx context.Context, tx gdb.TX, id int64) error {
	model := d.db.Model("user_vip_level")
	if tx != nil {
		model = tx.Model("user_vip_level")
	}

	_, err := model.Ctx(ctx).
		Where("id", id).
		Data(map[string]interface{}{
			"is_current": false,
		}).
		Update()
	return err
}

// GetAllCurrent 获取所有当前VIP等级
func (d *userVipLevelDao) GetAllCurrent(ctx context.Context) ([]*reward.UserVipLevelEntity, error) {
	var vips []*reward.UserVipLevelEntity
	err := d.db.Model("user_vip_level").Ctx(ctx).
		Fields("id,user_id,vip_level,district_performance,reward_rate,record_time,prev_vip_level,is_current,created_at,updated_at").
		Where("is_current", true).
		Scan(&vips)
	if err != nil {
		return nil, err
	}
	return vips, nil
}

// BatchGetCurrentVIPs 批量获取用户当前VIP等级（分批查询，避免超出PostgreSQL参数限制）
func (d *userVipLevelDao) BatchGetCurrentVIPs(ctx context.Context, userIDs []int64) ([]*reward.UserVipLevelEntity, error) {
	if len(userIDs) == 0 {
		return []*reward.UserVipLevelEntity{}, nil
	}

	// PostgreSQL最多支持65535个参数，每批最多60000个ID
	const batchSize = 50000

	var allVips []*reward.UserVipLevelEntity

	// 分批查询
	for i := 0; i < len(userIDs); i += batchSize {
		end := i + batchSize
		if end > len(userIDs) {
			end = len(userIDs)
		}

		batch := userIDs[i:end]

		var vips []*reward.UserVipLevelEntity
		err := d.db.Model("user_vip_level").Ctx(ctx).
			Fields("id,user_id,vip_level,district_performance,reward_rate,record_time,prev_vip_level,is_current,created_at,updated_at").
			Where("user_id", batch).
			Where("is_current", true).
			Scan(&vips)
		if err != nil {
			return nil, err
		}

		allVips = append(allVips, vips...)
	}

	return allVips, nil
}

// GetCurrentVipLevelsByUserIDs 根据用户ID列表获取当前VIP等级记录
func (d *userVipLevelDao) GetCurrentVipLevelsByUserIDs(ctx context.Context, userIDs []int64) ([]*reward.UserVipLevelEntity, error) {
	return d.BatchGetCurrentVIPs(ctx, userIDs)
}

// GetUserVipLevelByOffset 按offset获取用户VIP等级记录
// userID: 用户ID，如果为0表示查询所有用户
// offset: 偏移量指针，若为nil表示不限制偏移，返回所有匹配记录；否则按指定offset返回
// 返回所有匹配条件的VIP等级记录（按record_time倒序）
func (d *userVipLevelDao) GetUserVipLevelByOffset(ctx context.Context, userID int64, offset *int) ([]*reward.UserVipLevelEntity, error) {
	model := d.db.Model("user_vip_level").Ctx(ctx)

	// 如果userID大于0，则按用户ID过滤
	if userID > 0 {
		model = model.Where("user_id", userID)
	}

	var results []*reward.UserVipLevelEntity

	err := model.Fields("id,user_id,vip_level,district_performance,reward_rate,record_time,prev_vip_level,is_current,created_at,updated_at").OrderDesc("record_time").
		Scan(&results)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return []*reward.UserVipLevelEntity{}, nil
	}

	// 如果offset不为nil，则从指定偏移量开始返回
	if offset != nil {
		if *offset >= len(results) {
			return []*reward.UserVipLevelEntity{}, nil // 超出范围，返回空切片
		}
		vipLevels := make([]*reward.UserVipLevelEntity, 0, len(results)-*offset)
		for i := *offset; i < len(results); i++ {
			vipLevels = append(vipLevels, results[i])
		}
		return vipLevels, nil
	}

	// 如果offset为nil，返回所有记录
	return results, nil
}

// BatchCreate 批量创建VIP等级记录（分批处理，避免超出PostgreSQL参数限制）
func (d *userVipLevelDao) BatchCreate(ctx context.Context, tx gdb.TX, vips []*reward.UserVipLevelEntity) error {
	if len(vips) == 0 {
		return nil
	}

	// 每条记录7个字段，PostgreSQL最多支持65535个参数
	// 为安全起见，每批最多500条记录（7 * 500 = 3500 < 65535）
	const batchSize = 2000

	// 分批插入
	for i := 0; i < len(vips); i += batchSize {
		end := i + batchSize
		if end > len(vips) {
			end = len(vips)
		}

		batch := vips[i:end]

		model := d.db.Model("user_vip_level")
		if tx != nil {
			model = tx.Model("user_vip_level")
		}

		// 准备批量插入数据
		data := make([]map[string]interface{}, len(batch))
		for j, vip := range batch {
			data[j] = map[string]interface{}{
				"user_id":              vip.UserID,
				"vip_level":            vip.VipLevel,
				"district_performance": vip.DistrictPerformance,
				"reward_rate":          vip.RewardRate,
				"record_time":          vip.RecordTime,
				"prev_vip_level":       vip.PrevVipLevel,
				"is_current":           vip.IsCurrent,
			}
		}

		_, err := model.Ctx(ctx).Data(data).Insert()
		if err != nil {
			return err
		}
	}

	return nil
}
