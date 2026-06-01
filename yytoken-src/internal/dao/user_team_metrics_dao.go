package dao

import (
	"context"
	"time"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/shopspring/decimal"
)

// IUserTeamMetricsDao 用户团队指标数据访问接口
type IUserTeamMetricsDao interface {
	// Create 创建用户团队指标
	Create(ctx context.Context, metrics *entity.UserTeamMetricsEntity) error

	// GetById 根据ID获取用户团队指标
	GetById(ctx context.Context, id int64) (*entity.UserTeamMetricsEntity, error)

	// GetByAddress 根据地址获取用户团队指标
	GetByAddress(ctx context.Context, address string) (*entity.UserTeamMetricsEntity, error)

	// GetByUserId 根据用户ID获取用户团队指标
	GetByUserId(ctx context.Context, userId int64) (*entity.UserTeamMetricsEntity, error)

	// GetList 分页获取用户团队指标列表
	GetList(ctx context.Context, page, pageSize int, address string) ([]*entity.UserTeamMetricsEntity, int, error)

	// UpdateById 根据ID更新用户团队指标
	UpdateById(ctx context.Context, id int64, data map[string]interface{}) error

	// DeleteById 根据ID删除用户团队指标
	DeleteById(ctx context.Context, id int64) error

	// GetAllMetrics 获取所有用户团队指标
	GetAllMetrics(ctx context.Context) ([]*entity.UserTeamMetricsEntity, error)

	// GetUserTeamNewByCycle 根据周期获取用户团队新增
	GetUserTeamNewByCycle(ctx context.Context, userId int64, cycleStartTime, cycleEndTime time.Time) (decimal.Decimal, error)

	// GetTeamStaticReleaseByCycle 获取团队成员（包括自己）的未出局总质押金额
	GetTeamStaticReleaseByCycle(ctx context.Context, userId int64, cycleStartTime, cycleEndTime time.Time) (decimal.Decimal, error)
}

// userTeamMetricsDao 用户团队指标数据访问实现
type userTeamMetricsDao struct {
	db gdb.DB
}

// NewUserTeamMetricsDao 创建用户团队指标数据访问实例
func NewUserTeamMetricsDao() IUserTeamMetricsDao {
	return &userTeamMetricsDao{
		db: db.GetDB(),
	}
}

// Create 创建用户团队指标
func (d *userTeamMetricsDao) Create(ctx context.Context, metrics *entity.UserTeamMetricsEntity) error {
	result, err := d.db.Model("user_team_metrics").Ctx(ctx).
		FieldsEx("id", "created_at", "updated_at").
		Data(metrics).
		InsertAndGetId()
	if err != nil {
		return err
	}
	metrics.Id = result
	return nil
}

// GetById 根据ID获取用户团队指标
func (d *userTeamMetricsDao) GetById(ctx context.Context, id int64) (*entity.UserTeamMetricsEntity, error) {
	var metrics entity.UserTeamMetricsEntity
	err := d.db.Model("user_team_metrics").Ctx(ctx).Where("id", id).Scan(&metrics)
	if err != nil {
		return nil, err
	}
	if metrics.Id == 0 {
		return nil, nil
	}
	return &metrics, nil
}

// GetByAddress 根据地址获取用户团队指标
func (d *userTeamMetricsDao) GetByAddress(ctx context.Context, address string) (*entity.UserTeamMetricsEntity, error) {
	var metrics entity.UserTeamMetricsEntity
	d.db.Model("user_team_metrics").Ctx(ctx).
		Where("LOWER(address) = LOWER(?)", address).
		Scan(&metrics)
	if metrics.Id == 0 {
		return nil, nil
	}
	return &metrics, nil
}

// GetByUserId 根据用户ID获取用户团队指标
func (d *userTeamMetricsDao) GetByUserId(ctx context.Context, userId int64) (*entity.UserTeamMetricsEntity, error) {
	var metrics entity.UserTeamMetricsEntity
	err := d.db.Model("user_team_metrics").Ctx(ctx).Where("user_id", userId).Scan(&metrics)
	if err != nil {
		return nil, err
	}
	if metrics.Id == 0 {
		return nil, nil
	}
	return &metrics, nil
}

// GetList 分页获取用户团队指标列表
func (d *userTeamMetricsDao) GetList(ctx context.Context, page, pageSize int, address string) ([]*entity.UserTeamMetricsEntity, int, error) {
	var metricsList []*entity.UserTeamMetricsEntity
	model := d.db.Model("user_team_metrics").Ctx(ctx)

	// 地址模糊查询
	if address != "" {
		model = model.Where("LOWER(address) LIKE LOWER(?)", "%"+address+"%")
	}

	// 获取总数
	total, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	// 分页查询（按 updated_at 降序，相同时间按 id 降序）
	err = model.Order(" id DESC").
		Page(page, pageSize).
		Scan(&metricsList)
	if err != nil {
		return nil, 0, err
	}

	return metricsList, total, nil
}

// UpdateById 根据ID更新用户团队指标
func (d *userTeamMetricsDao) UpdateById(ctx context.Context, id int64, data map[string]interface{}) error {
	_, err := d.db.Model("user_team_metrics").Ctx(ctx).Where("id", id).Update(data)
	return err
}

// DeleteById 根据ID删除用户团队指标
func (d *userTeamMetricsDao) DeleteById(ctx context.Context, id int64) error {
	_, err := d.db.Model("user_team_metrics").Ctx(ctx).Where("id", id).Delete()
	return err
}

// GetAllMetrics 获取所有用户团队指标
func (d *userTeamMetricsDao) GetAllMetrics(ctx context.Context) ([]*entity.UserTeamMetricsEntity, error) {
	var metricsList []*entity.UserTeamMetricsEntity
	err := d.db.Model("user_team_metrics").Ctx(ctx).Scan(&metricsList)
	if err != nil {
		return nil, err
	}
	return metricsList, nil
}

// GetUserTeamNewByCycle 根据周期获取用户团队新增
func (d *userTeamMetricsDao) GetUserTeamNewByCycle(ctx context.Context, userId int64, cycleStartTime, cycleEndTime time.Time) (decimal.Decimal, error) {
	// 1. 获取用户的所有下级用户ID
	// 查询用户邀请码
	var parent struct {
		InviteCode string `json:"invite_code"`
	}
	if err := d.db.Model("user_info").Ctx(ctx).
		Fields("invite_code").
		Where("id", userId).
		Scan(&parent); err != nil {
		return decimal.Zero, err
	}

	// 使用递归CTE查询所有下级用户ID
	// 注意：即使没有邀请码，也要包含用户自己
	descendantSQL := `
	WITH RECURSIVE user_tree AS (
		SELECT 
			u.id,
			u.invite_code
		FROM user_info u
		WHERE u.parent_invite_code = ?
		UNION ALL
		SELECT 
			ui.id,
			ui.invite_code
		FROM user_info ui
		INNER JOIN user_tree ut ON ui.parent_invite_code = ut.invite_code
	)
	SELECT id FROM user_tree;
	`

	var descendantResults []struct {
		ID int64 `json:"id"`
	}
	var descendantIDs []int64
	if parent.InviteCode != "" {
		// 有邀请码，查询所有下级用户
		if err := d.db.Ctx(ctx).Raw(descendantSQL, parent.InviteCode).Scan(&descendantResults); err != nil {
			return decimal.Zero, err
		}

		// 提取下级用户ID列表，并包含用户自己
		descendantIDs = make([]int64, 0, len(descendantResults)+1)
		// 先添加用户自己
		descendantIDs = append(descendantIDs, userId)
		// 再添加所有下级用户
		for _, result := range descendantResults {
			if result.ID > 0 {
				descendantIDs = append(descendantIDs, result.ID)
			}
		}
	} else {
		// 没有邀请码，只包含用户自己
		descendantIDs = []int64{userId}
	}

	// 2. 查询这些用户（包括自己和下级）在周期内新增的质押金额
	// 条件：start_time > cycleStartTime AND start_time <= cycleEndTime
	var result struct {
		Total decimal.Decimal `json:"total"`
	}

	err := d.db.Model("staking_package").Ctx(ctx).
		WhereIn("user_id", descendantIDs).
		Where("created_at > ?", cycleStartTime).
		Where("created_at <= ?", cycleEndTime).
		Fields("COALESCE(SUM(stake_amount), 0) AS total").
		Scan(&result)

	if err != nil {
		return decimal.Zero, err
	}

	return result.Total, nil
}

// GetTeamStaticReleaseByCycle 获取团队成员（包括自己）的未出局总质押金额
func (d *userTeamMetricsDao) GetTeamStaticReleaseByCycle(ctx context.Context, userId int64, cycleStartTime, cycleEndTime time.Time) (decimal.Decimal, error) {
	// 1. 获取用户的所有下级用户ID
	// 查询用户邀请码
	var parent struct {
		InviteCode string `json:"invite_code"`
	}
	if err := d.db.Model("user_info").Ctx(ctx).
		Fields("invite_code").
		Where("id", userId).
		Scan(&parent); err != nil {
		return decimal.Zero, err
	}

	// 使用递归CTE查询所有下级用户ID
	// 注意：即使没有邀请码，也要包含用户自己
	descendantSQL := `
	WITH RECURSIVE user_tree AS (
		SELECT 
			u.id,
			u.invite_code
		FROM user_info u
		WHERE u.parent_invite_code = ?
		UNION ALL
		SELECT 
			ui.id,
			ui.invite_code
		FROM user_info ui
		INNER JOIN user_tree ut ON ui.parent_invite_code = ut.invite_code
	)
	SELECT id FROM user_tree;
	`

	var descendantResults []struct {
		ID int64 `json:"id"`
	}
	var descendantIDs []int64
	if parent.InviteCode != "" {
		// 有邀请码，查询所有下级用户
		if err := d.db.Ctx(ctx).Raw(descendantSQL, parent.InviteCode).Scan(&descendantResults); err != nil {
			return decimal.Zero, err
		}

		// 提取下级用户ID列表，并包含用户自己
		descendantIDs = make([]int64, 0, len(descendantResults)+1)
		// 先添加用户自己
		descendantIDs = append(descendantIDs, userId)
		// 再添加所有下级用户
		for _, result := range descendantResults {
			if result.ID > 0 {
				descendantIDs = append(descendantIDs, result.ID)
			}
		}
	} else {
		// 没有邀请码，只包含用户自己
		descendantIDs = []int64{userId}
	}

	// 2. 查询这些用户（包括自己和下级）的未出局质押包的总质押金额
	// 条件：status=1（运行中，未出局）
	if len(descendantIDs) == 0 {
		return decimal.Zero, nil
	}

	var result struct {
		Total decimal.Decimal `json:"total"`
	}

	err := d.db.Model("staking_package").Ctx(ctx).
		WhereIn("user_id", descendantIDs).
		Where("status", 1).
		Fields("COALESCE(SUM(stake_amount), 0) AS total").
		Scan(&result)

	if err != nil {
		return decimal.Zero, err
	}

	return result.Total, nil
}

// func (d *userTeamMetricsDao) GetTeamStaticReleaseByCycle(ctx context.Context, userId int64, cycleStartTime, cycleEndTime time.Time) (decimal.Decimal, error) {
// 	// 1. 获取用户的所有下级用户ID
// 	// 查询用户邀请码
// 	var parent struct {
// 		InviteCode string `json:"invite_code"`
// 	}
// 	if err := d.db.Model("user_info").Ctx(ctx).
// 		Fields("invite_code").
// 		Where("id", userId).
// 		Scan(&parent); err != nil {
// 		return decimal.Zero, err
// 	}

// 	// 使用递归CTE查询所有下级用户ID
// 	// 注意：即使没有邀请码，也要包含用户自己
// 	descendantSQL := `
// 	WITH RECURSIVE user_tree AS (
// 		SELECT
// 			u.id,
// 			u.invite_code
// 		FROM user_info u
// 		WHERE u.parent_invite_code = ?
// 		UNION ALL
// 		SELECT
// 			ui.id,
// 			ui.invite_code
// 		FROM user_info ui
// 		INNER JOIN user_tree ut ON ui.parent_invite_code = ut.invite_code
// 	)
// 	SELECT id FROM user_tree;
// 	`

// 	var descendantResults []struct {
// 		ID int64 `json:"id"`
// 	}
// 	var descendantIDs []int64
// 	if parent.InviteCode != "" {
// 		// 有邀请码，查询所有下级用户
// 		if err := d.db.Ctx(ctx).Raw(descendantSQL, parent.InviteCode).Scan(&descendantResults); err != nil {
// 			return decimal.Zero, err
// 		}

// 		// 提取下级用户ID列表，并包含用户自己
// 		descendantIDs = make([]int64, 0, len(descendantResults)+1)
// 		// 先添加用户自己
// 		descendantIDs = append(descendantIDs, userId)
// 		// 再添加所有下级用户
// 		for _, result := range descendantResults {
// 			if result.ID > 0 {
// 				descendantIDs = append(descendantIDs, result.ID)
// 			}
// 		}
// 	} else {
// 		// 没有邀请码，只包含用户自己
// 		descendantIDs = []int64{userId}
// 	}

// 	// 2. 查询这些用户（包括自己和下级）在周期内的静态奖励
// 	// 条件：business_type='reward_static' AND flow_type='income' AND record_time在周期范围内
// 	var result struct {
// 		Total decimal.Decimal `json:"total"`
// 	}

// 	err := d.db.Model("asset_record").Ctx(ctx).
// 		WhereIn("user_id", descendantIDs).
// 		Where("business_type", "reward_static").
// 		Where("flow_type", "income").
// 		Where("created_at > ?", cycleStartTime).
// 		Where("created_at <= ?", cycleEndTime).
// 		Fields("COALESCE(SUM(amount), 0) AS total").
// 		Scan(&result)

// 	if err != nil {
// 		return decimal.Zero, err
// 	}

// 	return result.Total, nil
// }
