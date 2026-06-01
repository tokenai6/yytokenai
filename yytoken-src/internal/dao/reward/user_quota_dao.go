package reward

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/shopspring/decimal"
)

// IUserQuotaDao 用户额度数据访问接口
type IUserQuotaDao interface {
	// Create 创建用户额度记录
	Create(ctx context.Context, tx gdb.TX, quota *reward.UserQuotaEntity) error

	// GetByUserID 根据用户ID获取额度
	GetByUserID(ctx context.Context, userID int64) (*reward.UserQuotaEntity, error)

	// GetForUpdate 锁定用户额度（悲观锁）
	GetForUpdate(ctx context.Context, tx gdb.TX, userID int64) (*reward.UserQuotaEntity, error)

	// UpdateQuota 更新用户额度
	UpdateQuota(ctx context.Context, tx gdb.TX, userID int64, data map[string]interface{}) error

	// DeductQuota 扣除额度
	DeductQuota(ctx context.Context, tx gdb.TX, userID int64, amount decimal.Decimal) error

	// DeductQuotaWithStats 扣除额度并更新统计字段
	// staticRewardAmount: 静态奖励金额（用于累加 released_static，如果为0则不更新）
	// totalRewardAmount: 总奖励金额（用于累加 total_income，如果为0则不更新）
	DeductQuotaWithStats(ctx context.Context, tx gdb.TX, userID int64, amount, staticRewardAmount, totalRewardAmount decimal.Decimal) error

	// AddWithdrawnAmount 累加已提取金额
	AddWithdrawnAmount(ctx context.Context, tx gdb.TX, userID int64, amount decimal.Decimal) error

	// GetByUserIDs 批量获取用户额度
	GetByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*reward.UserQuotaEntity, error)

	// GetExpiredUsers 获取出局用户（remaining_quota = 0 且 total_stake_amount > 0）
	GetExpiredUsers(ctx context.Context) ([]*reward.UserQuotaEntity, error)

	// BatchGetByUserIDs 批量获取用户额度
	BatchGetByUserIDs(ctx context.Context, userIDs []int64) ([]*reward.UserQuotaEntity, error)

	// ResetCurrentStakeToZero 将当前统计类字段重置为0（不影响 remaining_quota/used_quota/历史字段）
	ResetCurrentStakeToZero(ctx context.Context, tx gdb.TX, userID int64) error
}

// userQuotaDao 用户额度数据访问实现
type userQuotaDao struct {
	db gdb.DB
}

// NewUserQuotaDao 创建用户额度数据访问实例
func NewUserQuotaDao() IUserQuotaDao {
	return &userQuotaDao{
		db: db.GetDB(),
	}
}

// Create 创建用户额度记录
func (d *userQuotaDao) Create(ctx context.Context, tx gdb.TX, quota *reward.UserQuotaEntity) error {
	model := d.db.Model("user_quota")
	if tx != nil {
		model = tx.Model("user_quota")
	}

	result, err := model.Ctx(ctx).
		FieldsEx("id", "created_at", "updated_at").
		Data(quota).
		InsertAndGetId()
	if err != nil {
		return err
	}
	quota.Id = result
	return nil
}

// GetByUserID 根据用户ID获取额度
func (d *userQuotaDao) GetByUserID(ctx context.Context, userID int64) (*reward.UserQuotaEntity, error) {
	var quota reward.UserQuotaEntity
	err := d.db.Model("user_quota").Ctx(ctx).
		Where("user_id", userID).
		Scan(&quota)
	if err != nil {
		// 如果查询不到记录，返回 nil, nil（记录不存在）
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if quota.Id == 0 {
		return nil, nil
	}
	return &quota, nil
}

// GetForUpdate 锁定用户额度（悲观锁）
func (d *userQuotaDao) GetForUpdate(ctx context.Context, tx gdb.TX, userID int64) (*reward.UserQuotaEntity, error) {
	var quota reward.UserQuotaEntity

	model := d.db.Model("user_quota")
	if tx != nil {
		model = tx.Model("user_quota")
	}

	err := model.Ctx(ctx).
		Where("user_id", userID).
		LockUpdate().
		Scan(&quota)
	if err != nil {
		// 如果查询不到记录，返回 nil, nil（记录不存在）
		// 注意：服务中心奖励不占额度，如果只有服务中心奖励，可以没有额度记录
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if quota.Id == 0 {
		return nil, nil
	}
	return &quota, nil
}

// UpdateQuota 更新用户额度
func (d *userQuotaDao) UpdateQuota(ctx context.Context, tx gdb.TX, userID int64, data map[string]interface{}) error {
	model := d.db.Model("user_quota")
	if tx != nil {
		model = tx.Model("user_quota")
	}

	_, err := model.Ctx(ctx).
		Where("user_id", userID).
		Data(data).
		Update()
	return err
}

// DeductQuota 扣除额度
func (d *userQuotaDao) DeductQuota(ctx context.Context, tx gdb.TX, userID int64, amount decimal.Decimal) error {
	sql := "UPDATE user_quota SET used_quota = used_quota + ?, remaining_quota = remaining_quota - ?, updated_at = CURRENT_TIMESTAMP WHERE user_id = ?"

	if tx != nil {
		_, err := tx.Exec(sql, amount, amount, userID)
		return err
	}

	_, err := d.db.Exec(ctx, sql, amount, amount, userID)
	return err
}

// DeductQuotaWithStats 扣除额度并更新统计字段
// staticRewardAmount: 静态奖励金额（用于累加 released_static，如果为0则不更新）
// totalRewardAmount: 总奖励金额（用于累加 total_income，如果为0则不更新）
func (d *userQuotaDao) DeductQuotaWithStats(ctx context.Context, tx gdb.TX, userID int64, amount, staticRewardAmount, totalRewardAmount decimal.Decimal) error {
	// 构建 SQL，根据参数动态添加字段更新
	parts := []string{
		"used_quota = used_quota + ?",
		"remaining_quota = remaining_quota - ?",
	}
	args := []interface{}{amount, amount}

	if staticRewardAmount.GreaterThan(decimal.Zero) {
		parts = append(parts, "released_static = released_static + ?")
		args = append(args, staticRewardAmount)
	}

	if totalRewardAmount.GreaterThan(decimal.Zero) {
		parts = append(parts, "total_income = total_income + ?")
		args = append(args, totalRewardAmount)
	}

	parts = append(parts, "updated_at = CURRENT_TIMESTAMP")
	args = append(args, userID)

	sql := "UPDATE user_quota SET " + strings.Join(parts, ", ") + " WHERE user_id = ?"

	if tx != nil {
		_, err := tx.Exec(sql, args...)
		return err
	}

	_, err := d.db.Exec(ctx, sql, args...)
	return err
}

// AddWithdrawnAmount 累加已提取金额
func (d *userQuotaDao) AddWithdrawnAmount(ctx context.Context, tx gdb.TX, userID int64, amount decimal.Decimal) error {
	sql := "UPDATE user_quota SET withdrawn_amount = withdrawn_amount + ?, updated_at = CURRENT_TIMESTAMP WHERE user_id = ?"

	if tx != nil {
		_, err := tx.Exec(sql, amount, userID)
		return err
	}

	_, err := d.db.Exec(ctx, sql, amount, userID)
	return err
}

// GetByUserIDs 批量获取用户额度
func (d *userQuotaDao) GetByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*reward.UserQuotaEntity, error) {
	if len(userIDs) == 0 {
		return make(map[int64]*reward.UserQuotaEntity), nil
	}

	var quotas []*reward.UserQuotaEntity
	err := d.db.Model("user_quota").Ctx(ctx).
		WhereIn("user_id", userIDs).
		Scan(&quotas)
	if err != nil {
		return nil, err
	}

	// 转换为map
	quotaMap := make(map[int64]*reward.UserQuotaEntity)
	for _, quota := range quotas {
		quotaMap[quota.UserID] = quota
	}

	return quotaMap, nil

}

// GetExpiredUsers 获取出局用户（静态完成出局：released_static >= max_static_release 且 max_static_release > 0）
func (d *userQuotaDao) GetExpiredUsers(ctx context.Context) ([]*reward.UserQuotaEntity, error) {
	var quotas []*reward.UserQuotaEntity
	err := d.db.Model("user_quota").Ctx(ctx).
		Where("max_static_release > ?", 0).
		Where("released_static >= max_static_release").
		Where("total_stake_amount > ?", 0).
		Scan(&quotas)
	if err != nil {
		return nil, err
	}
	return quotas, nil
}

// BatchGetByUserIDs 批量获取用户额度
func (d *userQuotaDao) BatchGetByUserIDs(ctx context.Context, userIDs []int64) ([]*reward.UserQuotaEntity, error) {
	var quotas []*reward.UserQuotaEntity
	err := d.db.Model("user_quota").Ctx(ctx).
		WhereIn("user_id", userIDs).
		Scan(&quotas)
	if err != nil {
		return nil, err
	}
	return quotas, nil
}

// DeductCurrentStake 扣减用户当前额度字段（用于出局清理）
// 已废弃：出局清理阶段不再通过DAO扣减额度统计字段
// ResetCurrentStakeToZero 将当前统计类字段重置为0（不影响剩余额度/已用额度/历史字段）
func (d *userQuotaDao) ResetCurrentStakeToZero(ctx context.Context, tx gdb.TX, userID int64) error {
	sql := `UPDATE user_quota SET
        total_stake_amount = 0,
        max_static_release = 0,
        total_quota = 0,
        updated_at = CURRENT_TIMESTAMP
    WHERE user_id = ?`

	if tx != nil {
		_, err := tx.Exec(sql, userID)
		return err
	}
	_, err := d.db.Exec(ctx, sql, userID)
	return err
}
