package reward

import (
	"context"
	"fmt"

	"XWFrame/internal/dao/reward"
	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/consts"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// IQuotaRepository 额度仓储接口
type IQuotaRepository interface {
	// AddQuota 增加用户额度（质押时调用）
	// maxStaticRelease: 最大静态释放金额（由调用方计算好传入，节点类型通常为质押金额×3，质押类型为质押金额×2）
	AddQuota(ctx context.Context, tx gdb.TX, userID int64, stakeAmount, powerValue, totalQuota, maxStaticRelease decimal.Decimal, packageNo string, packageID int64) error

	// DeductQuota 扣除用户额度（奖励发放时调用）
	// staticRewardAmount: 静态奖励金额（用于累加 released_static，如果为0则不更新）
	// totalRewardAmount: 总奖励金额（用于累加 total_income，如果为0则不更新）
	DeductQuota(ctx context.Context, tx gdb.TX, userID int64, amount decimal.Decimal, changeType, rewardType, remark string, relatedID int64, staticRewardAmount, totalRewardAmount decimal.Decimal) error

	// GetByUserID 根据用户ID查询额度
	GetByUserID(ctx context.Context, userID int64) (*rewardEntity.UserQuotaEntity, error)

	// GetForUpdate 使用悲观锁查询用户额度
	GetForUpdate(ctx context.Context, tx gdb.TX, userID int64) (*rewardEntity.UserQuotaEntity, error)

	// GetByUserIDs 批量查询用户额度
	GetByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*rewardEntity.UserQuotaEntity, error)

	// GetByUserIDsForUpdate 批量查询用户额度（带悲观锁）
	GetByUserIDsForUpdate(ctx context.Context, tx gdb.TX, userIDs []int64) (map[int64]*rewardEntity.UserQuotaEntity, error)

	// GetExpiredUsers 查询出局用户（remaining_quota = 0 且 total_stake_amount > 0）
	GetExpiredUsers(ctx context.Context) ([]*rewardEntity.UserQuotaEntity, error)

	// ResetCurrentStakeToZero 出局后将当前统计字段重置为0（不影响剩余额度/已用额度/历史字段）
	ResetCurrentStakeToZero(ctx context.Context, tx gdb.TX, userID int64) error

	// GetOrCreate 获取或创建用户额度
	GetOrCreate(ctx context.Context, tx gdb.TX, userID int64) (*rewardEntity.UserQuotaEntity, error)

	// AdjustQuotaForStaking 调整用户额度（质押调整时调用）
	// quotaDiff: 额度差值（新额度 - 旧额度，可为正数或负数）
	// maxStaticDiff: 最大静态释放差值（新值 - 旧值，可为正数或负数）
	// stakeAmountDiff: 质押金额差值（通常为0，因为本金不调整）
	AdjustQuotaForStaking(ctx context.Context, tx gdb.TX, userID int64, quotaDiff, maxStaticDiff, stakeAmountDiff decimal.Decimal, packageNo string, packageID int64, remark string) error

	// AddWithdrawnAmount 累加已提取金额（提现成功时调用）
	AddWithdrawnAmount(ctx context.Context, tx gdb.TX, userID int64, amount decimal.Decimal) error
}

// quotaRepository 额度仓储实现
type quotaRepository struct {
	quotaDao     reward.IUserQuotaDao
	changeLogDao reward.IQuotaChangeLogDao
}

// NewQuotaRepository 创建额度仓储实例
func NewQuotaRepository() IQuotaRepository {
	return &quotaRepository{
		quotaDao:     reward.NewUserQuotaDao(),
		changeLogDao: reward.NewQuotaChangeLogDao(),
	}
}

// AddQuota 增加用户额度（质押时调用）
// maxStaticRelease: 最大静态释放金额（由调用方计算好传入，节点类型通常为质押金额×3，质押类型为质押金额×2）
func (r *quotaRepository) AddQuota(ctx context.Context, tx gdb.TX, userID int64, stakeAmount, powerValue, totalQuota, maxStaticRelease decimal.Decimal, packageNo string, packageID int64) error {
	// 1. 获取或创建用户额度
	quota, err := r.GetOrCreate(ctx, tx, userID)
	if err != nil {
		return gerror.Wrap(err, "获取用户额度失败")
	}

	// 记录变更前的额度
	quotaBefore := quota.TotalQuota

	// 3. 更新额度
	updateData := map[string]interface{}{
		"total_stake_amount":         gdb.Raw(fmt.Sprintf("total_stake_amount + %s", stakeAmount.String())),
		"max_static_release":         gdb.Raw(fmt.Sprintf("max_static_release + %s", maxStaticRelease.String())),
		"total_quota":                gdb.Raw(fmt.Sprintf("total_quota + %s", totalQuota.String())),
		"remaining_quota":            gdb.Raw(fmt.Sprintf("remaining_quota + %s", totalQuota.String())),
		"history_total_quota":        gdb.Raw(fmt.Sprintf("history_total_quota + %s", totalQuota.String())),
		"history_total_stake_amount": gdb.Raw(fmt.Sprintf("history_total_stake_amount + %s", stakeAmount.String())),
		"history_stake_count":        gdb.Raw("history_stake_count + 1"),
	}

	err = r.quotaDao.UpdateQuota(ctx, tx, userID, updateData)
	if err != nil {
		return gerror.Wrap(err, "更新用户额度失败")
	}

	// 4. 创建额度变更日志
	changeLog := &rewardEntity.QuotaChangeLogEntity{
		UserID:         userID,
		ChangeType:     consts.QuotaChangeTypeAdd,
		ChangeAmount:   totalQuota,
		QuotaBefore:    quotaBefore,
		QuotaAfter:     quotaBefore.Add(totalQuota),
		RelatedOrderNo: packageNo,
		RelatedID:      packageID,
		RewardType:     consts.QuotaRewardTypeStakeIncreaseQuota,
		Remark:         fmt.Sprintf("质押购买增加额度：%s USDT", stakeAmount.String()),
	}

	err = r.changeLogDao.Create(ctx, tx, changeLog)
	if err != nil {
		return gerror.Wrap(err, "创建额度变更日志失败")
	}

	// g.Log().Infof(ctx, "[额度变更] 用户 %d 质押 %s USDT，增加额度 %s，变更前 %s，变更后 %s",
	// 	userID, stakeAmount.String(), totalQuota.String(), quotaBefore.String(), quotaBefore.Add(totalQuota).String())

	return nil
}

// DeductQuota 扣除用户额度（奖励发放时调用）
// staticRewardAmount: 静态奖励金额（用于累加 released_static，如果为0则不更新）
// totalRewardAmount: 总奖励金额（用于累加 total_income，如果为0则不更新）
func (r *quotaRepository) DeductQuota(ctx context.Context, tx gdb.TX, userID int64, amount decimal.Decimal, changeType, rewardType, remark string, relatedID int64, staticRewardAmount, totalRewardAmount decimal.Decimal) error {
	// 1. 获取用户额度（使用悲观锁）
	quota, err := r.quotaDao.GetForUpdate(ctx, tx, userID)
	if err != nil {
		return gerror.Wrap(err, "获取用户额度失败")
	}
	if quota == nil {
		return gerror.Newf("用户 %d 额度不存在", userID)
	}

	// 2. 检查剩余额度
	if quota.RemainingQuota.LessThan(amount) {
		return gerror.Newf("用户 %d 剩余额度不足，剩余 %s，需要 %s", userID, quota.RemainingQuota.String(), amount.String())
	}

	// 记录变更前的额度
	quotaBefore := quota.TotalQuota

	// 3. 扣除额度并更新相关统计字段
	err = r.quotaDao.DeductQuotaWithStats(ctx, tx, userID, amount, staticRewardAmount, totalRewardAmount)
	if err != nil {
		return gerror.Wrap(err, "扣除用户额度失败")
	}

	// 4. 创建额度变更日志
	changeLog := &rewardEntity.QuotaChangeLogEntity{
		UserID:       userID,
		ChangeType:   changeType,
		ChangeAmount: amount.Neg(), // 负数表示扣除
		QuotaBefore:  quotaBefore,
		QuotaAfter:   quotaBefore.Sub(amount),
		RelatedID:    relatedID,
		RewardType:   rewardType,
		Remark:       remark,
	}

	err = r.changeLogDao.Create(ctx, tx, changeLog)
	if err != nil {
		return gerror.Wrap(err, "创建额度变更日志失败")
	}

	// g.Log().Infof(ctx, "[额度变更] 用户 %d 扣除额度 %s，变更前 %s，变更后 %s，原因：%s",
	// 	userID, amount.String(), quotaBefore.String(), quotaBefore.Sub(amount).String(), remark)

	return nil
}

// GetByUserID 根据用户ID查询额度
func (r *quotaRepository) GetByUserID(ctx context.Context, userID int64) (*rewardEntity.UserQuotaEntity, error) {
	quota, err := r.quotaDao.GetByUserID(ctx, userID)
	if err != nil {
		return nil, gerror.Wrapf(err, "查询用户 %d 额度失败", userID)
	}
	return quota, nil
}

// GetForUpdate 使用悲观锁查询用户额度
func (r *quotaRepository) GetForUpdate(ctx context.Context, tx gdb.TX, userID int64) (*rewardEntity.UserQuotaEntity, error) {
	quota, err := r.quotaDao.GetForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, gerror.Wrapf(err, "锁定查询用户 %d 额度失败", userID)
	}
	return quota, nil
}

// GetByUserIDs 批量查询用户额度（自动分批处理，避免PostgreSQL参数限制）
func (r *quotaRepository) GetByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*rewardEntity.UserQuotaEntity, error) {
	if len(userIDs) == 0 {
		return make(map[int64]*rewardEntity.UserQuotaEntity), nil
	}

	const maxBatchSize = 30000 // 每批次最多3万条数据
	quotaMap := make(map[int64]*rewardEntity.UserQuotaEntity)

	if len(userIDs) > maxBatchSize {
		// 分批查询
		totalBatches := (len(userIDs) + maxBatchSize - 1) / maxBatchSize
		g.Log().Infof(ctx, "[QuotaRepository] 批量查询用户额度，数量 %d 超过限制，将分 %d 批处理", len(userIDs), totalBatches)

		for i := 0; i < len(userIDs); i += maxBatchSize {
			end := i + maxBatchSize
			if end > len(userIDs) {
				end = len(userIDs)
			}
			batch := userIDs[i:end]
			batchIndex := i/maxBatchSize + 1

			batchQuotaMap, err := r.quotaDao.GetByUserIDs(ctx, batch)
			if err != nil {
				return nil, gerror.Wrapf(err, "批量查询用户额度失败（第 %d/%d 批）", batchIndex, totalBatches)
			}

			// 合并结果
			for userID, quota := range batchQuotaMap {
				quotaMap[userID] = quota
			}
		}
	} else {
		// 数据量不大，直接查询
		var err error
		quotaMap, err = r.quotaDao.GetByUserIDs(ctx, userIDs)
		if err != nil {
			return nil, gerror.Wrapf(err, "批量查询用户额度失败，用户数量: %d", len(userIDs))
		}
	}

	return quotaMap, nil
}

// GetByUserIDsForUpdate 批量查询用户额度（带悲观锁）
// 🚀 性能优化：批量结算时使用，在一个事务中锁定多个用户的额度
func (r *quotaRepository) GetByUserIDsForUpdate(ctx context.Context, tx gdb.TX, userIDs []int64) (map[int64]*rewardEntity.UserQuotaEntity, error) {
	if len(userIDs) == 0 {
		return make(map[int64]*rewardEntity.UserQuotaEntity), nil
	}

	quotaMap := make(map[int64]*rewardEntity.UserQuotaEntity)
	var quotas []*rewardEntity.UserQuotaEntity

	// 使用事务的 Model，添加 FOR UPDATE 锁
	model := tx.Model("user_quota")
	err := model.Ctx(ctx).
		Where("user_id IN (?)", userIDs).
		LockUpdate(). // 添加悲观锁（FOR UPDATE）
		Scan(&quotas)

	if err != nil {
		return nil, gerror.Wrapf(err, "批量查询用户额度（带锁）失败，用户数量: %d", len(userIDs))
	}

	// 转换为 map
	for _, quota := range quotas {
		quotaMap[quota.UserID] = quota
	}

	return quotaMap, nil
}

// GetExpiredUsers 查询出局用户（remaining_quota = 0 且 total_stake_amount > 0）
func (r *quotaRepository) GetExpiredUsers(ctx context.Context) ([]*rewardEntity.UserQuotaEntity, error) {
	users, err := r.quotaDao.GetExpiredUsers(ctx)
	if err != nil {
		return nil, gerror.Wrap(err, "查询出局用户失败")
	}
	return users, nil
}

// ResetCurrentStakeToZero 出局后将当前统计字段重置为0（不影响 remaining_quota/used_quota/历史字段）
func (r *quotaRepository) ResetCurrentStakeToZero(ctx context.Context, tx gdb.TX, userID int64) error {
	quota, err := r.quotaDao.GetForUpdate(ctx, tx, userID)
	if err != nil {
		return gerror.Wrap(err, "查询用户额度失败")
	}
	if quota == nil {
		return gerror.Newf("用户 %d 额度不存在", userID)
	}

	before := quota.TotalQuota
	if err := r.quotaDao.ResetCurrentStakeToZero(ctx, tx, userID); err != nil {
		return gerror.Wrap(err, "重置当前统计字段失败")
	}
	g.Log().Infof(ctx, "[出局清理] 将当前统计字段重置为0: userID=%d, total_quota %s -> 0", userID, before.String())
	return nil
}

// GetOrCreate 获取或创建用户额度
func (r *quotaRepository) GetOrCreate(ctx context.Context, tx gdb.TX, userID int64) (*rewardEntity.UserQuotaEntity, error) {
	// 先尝试获取
	quota, err := r.quotaDao.GetByUserID(ctx, userID)
	if err != nil {
		return nil, gerror.Wrap(err, "查询用户额度失败")
	}

	// 如果不存在，则创建
	if quota == nil {
		quota = &rewardEntity.UserQuotaEntity{
			UserID:                  userID,
			TotalStakeAmount:        decimal.Zero,
			MaxStaticRelease:        decimal.Zero,
			ReleasedStatic:          decimal.Zero,
			TotalQuota:              decimal.Zero,
			UsedQuota:               decimal.Zero,
			RemainingQuota:          decimal.Zero,
			TotalIncome:             decimal.Zero,
			WithdrawnAmount:         decimal.Zero,
			AvailableAmount:         decimal.Zero,
			HistoryTotalQuota:       decimal.Zero,
			HistoryTotalStakeAmount: decimal.Zero,
			HistoryStakeCount:       0,
			Version:                 0,
		}

		err = r.quotaDao.Create(ctx, tx, quota)
		if err != nil {
			return nil, gerror.Wrap(err, "创建用户额度失败")
		}

		g.Log().Infof(ctx, "[额度初始化] 用户 %d 额度记录创建成功", userID)
	}

	return quota, nil
}

// AdjustQuotaForStaking 调整用户额度（质押调整时调用）
// quotaDiff: 额度差值（新额度 - 旧额度，可为正数或负数）
// maxStaticDiff: 最大静态释放差值（新值 - 旧值，可为正数或负数）
// stakeAmountDiff: 质押金额差值（通常为0，因为本金不调整）
func (r *quotaRepository) AdjustQuotaForStaking(ctx context.Context, tx gdb.TX, userID int64, quotaDiff, maxStaticDiff, stakeAmountDiff decimal.Decimal, packageNo string, packageID int64, remark string) error {
	// 1. 获取用户额度（使用悲观锁）
	quota, err := r.GetForUpdate(ctx, tx, userID)
	if err != nil {
		return gerror.Wrap(err, "获取用户额度失败")
	}
	if quota == nil {
		return gerror.Newf("用户 %d 额度不存在", userID)
	}

	// 2. 检查调整后剩余额度是否会导致出局（<=0）
	// 需求：不允许直接将用户修改出局，如果调整后剩余额度<=0，拒绝调整
	newRemainingQuota := quota.RemainingQuota.Add(quotaDiff)
	if newRemainingQuota.LessThanOrEqual(decimal.Zero) {
		return gerror.Newf("调整会导致用户出局，不允许修改：当前剩余额度 %s，调整差值 %s，调整后剩余额度 %s", quota.RemainingQuota.String(), quotaDiff.String(), newRemainingQuota.String())
	}

	// 记录变更前的额度
	quotaBefore := quota.TotalQuota

	// 3. 更新额度（使用 gdb.Raw 进行原子性更新）
	updateData := map[string]interface{}{}
	if !quotaDiff.IsZero() {
		// 更新总额度和剩余额度
		if quotaDiff.GreaterThan(decimal.Zero) {
			updateData["total_quota"] = gdb.Raw(fmt.Sprintf("total_quota + %s", quotaDiff.String()))
			updateData["remaining_quota"] = gdb.Raw(fmt.Sprintf("remaining_quota + %s", quotaDiff.String()))
			updateData["history_total_quota"] = gdb.Raw(fmt.Sprintf("history_total_quota + %s", quotaDiff.String()))
		} else {
			updateData["total_quota"] = gdb.Raw(fmt.Sprintf("total_quota - %s", quotaDiff.Abs().String()))
			updateData["remaining_quota"] = gdb.Raw(fmt.Sprintf("remaining_quota - %s", quotaDiff.Abs().String()))
			updateData["history_total_quota"] = gdb.Raw(fmt.Sprintf("history_total_quota - %s", quotaDiff.Abs().String()))
		}
	}

	if !maxStaticDiff.IsZero() {
		if maxStaticDiff.GreaterThan(decimal.Zero) {
			updateData["max_static_release"] = gdb.Raw(fmt.Sprintf("max_static_release + %s", maxStaticDiff.String()))
		} else {
			updateData["max_static_release"] = gdb.Raw(fmt.Sprintf("max_static_release - %s", maxStaticDiff.Abs().String()))
		}
	}

	if !stakeAmountDiff.IsZero() {
		if stakeAmountDiff.GreaterThan(decimal.Zero) {
			updateData["total_stake_amount"] = gdb.Raw(fmt.Sprintf("total_stake_amount + %s", stakeAmountDiff.String()))
			updateData["history_total_stake_amount"] = gdb.Raw(fmt.Sprintf("history_total_stake_amount + %s", stakeAmountDiff.String()))
		} else {
			updateData["total_stake_amount"] = gdb.Raw(fmt.Sprintf("total_stake_amount - %s", stakeAmountDiff.Abs().String()))
			updateData["history_total_stake_amount"] = gdb.Raw(fmt.Sprintf("history_total_stake_amount - %s", stakeAmountDiff.Abs().String()))
		}
	}

	if len(updateData) > 0 {
		err = r.quotaDao.UpdateQuota(ctx, tx, userID, updateData)
		if err != nil {
			return gerror.Wrap(err, "更新用户额度失败")
		}
	}

	// 4. 创建额度变更日志（只有额度有变化时才记录）
	if !quotaDiff.IsZero() {
		changeLog := &rewardEntity.QuotaChangeLogEntity{
			UserID:         userID,
			ChangeType:     consts.QuotaChangeTypeAdjust,
			ChangeAmount:   quotaDiff, // 正数表示增加，负数表示减少
			QuotaBefore:    quotaBefore,
			QuotaAfter:     quotaBefore.Add(quotaDiff),
			RelatedOrderNo: packageNo,
			RelatedID:      packageID,
			RewardType:     consts.QuotaRewardTypeStakeAdjust,
			Remark:         remark,
		}

		err = r.changeLogDao.Create(ctx, tx, changeLog)
		if err != nil {
			return gerror.Wrap(err, "创建额度变更日志失败")
		}
	}

	return nil
}

// AddWithdrawnAmount 累加已提取金额（提现成功时调用）
// 注意：如果用户额度记录不存在，会自动创建（因为提现成功说明用户有收益，应该有额度记录）
func (r *quotaRepository) AddWithdrawnAmount(ctx context.Context, tx gdb.TX, userID int64, amount decimal.Decimal) error {
	// 先尝试获取或创建用户额度记录
	quota, err := r.GetOrCreate(ctx, tx, userID)
	if err != nil {
		return gerror.Wrap(err, "获取或创建用户额度失败")
	}
	if quota == nil {
		return gerror.Newf("用户 %d 额度记录创建失败", userID)
	}

	// 更新已提取金额
	err = r.quotaDao.AddWithdrawnAmount(ctx, tx, userID, amount)
	if err != nil {
		return gerror.Wrap(err, "累加已提取金额失败")
	}
	return nil
}
