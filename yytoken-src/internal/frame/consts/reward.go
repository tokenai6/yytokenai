package consts

// 奖励记录状态
const (
	RewardStatusPending  = 0 // 待结算
	RewardStatusSettled  = 1 // 已结算（已发放）
	RewardStatusReduced  = 2 // 已削减（额度不足被削减）
	RewardStatusCanceled = 3 // 已取消（计算失败或其他原因）
)

// 额度变动-变更类型（用于 quota_change_log.change_type）
const (
	QuotaChangeTypeAdd          = "add"           // 增加额度（质押）
	QuotaChangeTypeDeduct       = "deduct"        // 扣除额度（统一结算）
	QuotaChangeTypeExpireDeduct = "expire_deduct" // 出局清理扣减
	QuotaChangeTypeAdjust       = "adjust"        // 质押调整（管理员调整质押倍数/类型）
)

// 额度变动-奖励类型（用于 quota_change_log.reward_type）
const (
	QuotaRewardTypeStakeIncreaseQuota = "stake_increase_quota" // 质押增加额度
	QuotaRewardTypeReleaseQuota       = "release_quota"        // 释放额度（统一结算扣额度）
	QuotaRewardTypeStakeAdjust        = "stake_adjust"         // 质押调整（管理员调整质押倍数/类型，C端需过滤）
)
