package reward

import (
	"context"

	"XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

// IQuotaChangeLogDao 额度变动日志数据访问接口
type IQuotaChangeLogDao interface {
	// Create 创建额度变动日志
	Create(ctx context.Context, tx gdb.TX, log *reward.QuotaChangeLogEntity) error

	// GetPagedByUserID 分页获取用户额度变动日志
	// rewardTypes: 要查询的奖励类型列表（白名单），如果为空或nil则查询全部
	GetPagedByUserID(ctx context.Context, userID int64, rewardTypes []string, startDate, endDate string, page, pageSize int) ([]*reward.QuotaChangeLogEntity, int, error)

	// GetByUserID 获取用户所有额度变动日志
	GetByUserID(ctx context.Context, userID int64) ([]*reward.QuotaChangeLogEntity, error)
}

// quotaChangeLogDao 额度变动日志数据访问实现
type quotaChangeLogDao struct {
	db gdb.DB
}

// NewQuotaChangeLogDao 创建额度变动日志数据访问实例
func NewQuotaChangeLogDao() IQuotaChangeLogDao {
	return &quotaChangeLogDao{
		db: db.GetDB(),
	}
}

// Create 创建额度变动日志
func (d *quotaChangeLogDao) Create(ctx context.Context, tx gdb.TX, log *reward.QuotaChangeLogEntity) error {
	model := d.db.Model("quota_change_log")
	if tx != nil {
		model = tx.Model("quota_change_log")
	}

	result, err := model.Ctx(ctx).
		FieldsEx("id", "created_at").
		Data(log).
		InsertAndGetId()
	if err != nil {
		return err
	}
	log.Id = result
	return nil
}

// GetPagedByUserID 分页获取用户额度变动日志
// rewardTypes: 要查询的奖励类型列表（白名单），如果为空或nil则查询全部
func (d *quotaChangeLogDao) GetPagedByUserID(ctx context.Context, userID int64, rewardTypes []string, startDate, endDate string, page, pageSize int) ([]*reward.QuotaChangeLogEntity, int, error) {
	model := d.db.Model("quota_change_log").Ctx(ctx).
		Where("user_id", userID)

	// 奖励类型过滤（白名单）
	if len(rewardTypes) > 0 {
		model = model.WhereIn("reward_type", rewardTypes)
	}

	// 日期范围过滤（created_at）
	if startDate != "" {
		// 支持 YYYY-MM-DD 或完整时间
		if len(startDate) == 10 { // 仅日期
			model = model.WhereGTE("created_at", startDate+" 00:00:00")
		} else {
			model = model.WhereGTE("created_at", startDate)
		}
	}
	if endDate != "" {
		if len(endDate) == 10 { // 仅日期
			model = model.WhereLTE("created_at", endDate+" 23:59:59")
		} else {
			model = model.WhereLTE("created_at", endDate)
		}
	}

	// 获取总数（应用所有过滤条件）
	total, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	var logs []*reward.QuotaChangeLogEntity
	err = model.OrderDesc("created_at").
		Page(page, pageSize).
		Scan(&logs)
	if err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// GetByUserID 获取用户所有额度变动日志
func (d *quotaChangeLogDao) GetByUserID(ctx context.Context, userID int64) ([]*reward.QuotaChangeLogEntity, error) {
	var logs []*reward.QuotaChangeLogEntity
	err := d.db.Model("quota_change_log").Ctx(ctx).
		Where("user_id", userID).
		OrderDesc("created_at").
		Scan(&logs)
	if err != nil {
		return nil, err
	}
	return logs, nil
}
