package reward

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// IUserVipAdjustmentDao VIP调整记录数据访问接口
type IUserVipAdjustmentDao interface {
	// Create 创建VIP调整记录
	Create(ctx context.Context, tx gdb.TX, adjustment *reward.UserVipAdjustmentEntity) error

	// GetActiveByUserID 获取用户有效的调整记录（未过期）
	GetActiveByUserID(ctx context.Context, userID int64, now time.Time) (*reward.UserVipAdjustmentEntity, error)

	// BatchGetActiveByUserIDs 批量获取用户有效的调整记录
	BatchGetActiveByUserIDs(ctx context.Context, userIDs []int64, now time.Time) (map[int64]*reward.UserVipAdjustmentEntity, error)

	// GetByID 根据ID获取调整记录
	GetByID(ctx context.Context, id int64) (*reward.UserVipAdjustmentEntity, error)

	// Cancel 取消调整记录（将过期时间设置为当前时间）
	Cancel(ctx context.Context, tx gdb.TX, id int64, now time.Time) error

	// GetList 获取调整记录列表（分页）
	GetList(ctx context.Context, userID int64, page, pageSize int) ([]*reward.UserVipAdjustmentEntity, int, error)
}

// userVipAdjustmentDao VIP调整记录数据访问实现
type userVipAdjustmentDao struct {
	db gdb.DB
}

// NewUserVipAdjustmentDao 创建VIP调整记录数据访问实例
func NewUserVipAdjustmentDao() IUserVipAdjustmentDao {
	return &userVipAdjustmentDao{
		db: db.GetDB(),
	}
}

// Create 创建VIP调整记录
func (d *userVipAdjustmentDao) Create(ctx context.Context, tx gdb.TX, adjustment *reward.UserVipAdjustmentEntity) error {
	model := d.db.Model("user_vip_adjustment")
	if tx != nil {
		model = tx.Model("user_vip_adjustment")
	}

	result, err := model.Ctx(ctx).
		Data(map[string]interface{}{
			"user_id":            adjustment.UserID,
			"adjusted_vip_level": adjustment.AdjustedVipLevel,
			"original_vip_level": adjustment.OriginalVipLevel,
			"adjust_time":        adjustment.AdjustTime,
			"expire_time":        adjustment.ExpireTime,
			"operator_id":        adjustment.OperatorID,
			"remark":             adjustment.Remark,
		}).
		InsertAndGetId()
	if err != nil {
		return err
	}
	adjustment.Id = result
	return nil
}

// GetActiveByUserID 获取用户有效的调整记录（未过期）
func (d *userVipAdjustmentDao) GetActiveByUserID(ctx context.Context, userID int64, now time.Time) (*reward.UserVipAdjustmentEntity, error) {
	var adjustment reward.UserVipAdjustmentEntity
	err := d.db.Model("user_vip_adjustment").Ctx(ctx).
		Fields("id,user_id,adjusted_vip_level,original_vip_level,adjust_time,expire_time,operator_id,remark,created_at,updated_at").
		Where("user_id", userID).
		Where("expire_time > ?", now).
		OrderDesc("adjust_time").
		Limit(1).
		Scan(&adjustment)
	if err != nil {
		// 如果查询不到记录（sql: no rows in result set），返回 nil, nil（记录不存在，这是正常情况）
		if errors.Is(err, sql.ErrNoRows) || err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	if adjustment.Id == 0 {
		return nil, nil
	}
	return &adjustment, nil
}

// BatchGetActiveByUserIDs 批量获取用户有效的调整记录
func (d *userVipAdjustmentDao) BatchGetActiveByUserIDs(ctx context.Context, userIDs []int64, now time.Time) (map[int64]*reward.UserVipAdjustmentEntity, error) {
	if len(userIDs) == 0 {
		return make(map[int64]*reward.UserVipAdjustmentEntity), nil
	}

	// PostgreSQL最多支持65535个参数，每批最多50000个ID
	const batchSize = 50000

	result := make(map[int64]*reward.UserVipAdjustmentEntity)

	// 分批查询
	for i := 0; i < len(userIDs); i += batchSize {
		end := i + batchSize
		if end > len(userIDs) {
			end = len(userIDs)
		}

		batch := userIDs[i:end]

		var adjustments []*reward.UserVipAdjustmentEntity
		err := d.db.Model("user_vip_adjustment").Ctx(ctx).
			Fields("id,user_id,adjusted_vip_level,original_vip_level,adjust_time,expire_time,operator_id,remark,created_at,updated_at").
			Where("user_id", batch).
			Where("expire_time > ?", now).
			OrderDesc("adjust_time").
			Scan(&adjustments)
		if err != nil {
			return nil, err
		}

		// 对于每个用户，只保留最新的有效调整记录
		for _, adj := range adjustments {
			if existing, exists := result[adj.UserID]; !exists || adj.AdjustTime.After(existing.AdjustTime) {
				result[adj.UserID] = adj
			}
		}
	}

	return result, nil
}

// GetByID 根据ID获取调整记录
func (d *userVipAdjustmentDao) GetByID(ctx context.Context, id int64) (*reward.UserVipAdjustmentEntity, error) {
	var adjustment reward.UserVipAdjustmentEntity
	err := d.db.Model("user_vip_adjustment").Ctx(ctx).
		Fields("id,user_id,adjusted_vip_level,original_vip_level,adjust_time,expire_time,operator_id,remark,created_at,updated_at").
		Where("id", id).
		Scan(&adjustment)
	if err != nil {
		// 如果查询不到记录（sql: no rows in result set），返回 nil, nil（记录不存在，这是正常情况）
		if errors.Is(err, sql.ErrNoRows) || err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	if adjustment.Id == 0 {
		return nil, nil
	}
	return &adjustment, nil
}

// Cancel 取消调整记录（将过期时间设置为当前时间）
func (d *userVipAdjustmentDao) Cancel(ctx context.Context, tx gdb.TX, id int64, now time.Time) error {
	model := d.db.Model("user_vip_adjustment")
	if tx != nil {
		model = tx.Model("user_vip_adjustment")
	}

	_, err := model.Ctx(ctx).
		Where("id", id).
		Data(map[string]interface{}{
			"expire_time": now,
		}).
		Update()
	return err
}

// GetList 获取调整记录列表（分页）
func (d *userVipAdjustmentDao) GetList(ctx context.Context, userID int64, page, pageSize int) ([]*reward.UserVipAdjustmentEntity, int, error) {
	model := d.db.Model("user_vip_adjustment").Ctx(ctx)

	if userID > 0 {
		model = model.Where("user_id", userID)
	}

	// 获取总数
	count, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	var adjustments []*reward.UserVipAdjustmentEntity
	err = model.
		Fields("id,user_id,adjusted_vip_level,original_vip_level,adjust_time,expire_time,operator_id,remark,created_at,updated_at").
		OrderDesc("adjust_time").
		Page(page, pageSize).
		Scan(&adjustments)
	if err != nil {
		return nil, 0, err
	}

	return adjustments, count, nil
}
