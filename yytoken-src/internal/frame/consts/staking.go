package consts

// 算力包质押类型（业务类型）
const (
	StakeTypeLPStaking          = 1  // LP质押
	StakeTypeNodePurchase       = 2  // 购买节点
	StakeTypeExternalImport     = 3  // 导入质押
	StakeTypeExternalNode       = 4  // 导入节点
	StakeTypeCombinationStaking = 5  // 组合质押
	StakeTypeMintGame           = 10 // APG Mint游戏质押
)

// 链上质押类型（与合约枚举对应）
const (
	StakeTypeChainLPStake       = 0 // LP质押 (LP_STAKE)
	StakeTypeChainTreasuryStake = 1 // 国库质押 (TREASURY_STAKE)
)

// 算力包状态
const (
	StakingStatusRunning       = 1 // 运行中
	StakingStatusStaticExpired = 2 // 静态完成出局
	StakingStatusQuotaExpired  = 3 // 额度耗尽出局
	StakingStatusForceExpired  = 4 // 强制出局
)

// 质押购买限制配置
const (
	MaxPurchasePerDay = 30   // 每日最大购买次数（根据需求文档：每日限制购买次数30次）
	MinPurchaseAmount = 10.0 // 每次最小购买金额（USDT）（根据需求文档：每次最小购买额度10u）
)

// 质押倍数配置
const (
	StakePowerMultiplier           = 2.0  // 算力倍数（质押金额×2）
	StakeQuotaMultiplier           = 2.0  // 额度倍数（质押金额×2，动静两倍出局）
	StakeDailyYieldRate            = 0.01 // 日收益率（1%）
	NodeMaxStaticReleaseMultiplier = 3    // 节点类型最大静态释放倍数
)

// 节点分红配置
const (
	// NodeMinRewardAmount 节点最小分红金额（小于此金额不发放）
	NodeMinRewardAmount = 0.01
)

// 注意：NodeDividendPoolRate 定义在 reward_config.go 中

// VIP等级
const (
	VIPLevel0 = 0 // VIP0（无等级）
	VIPLevel1 = 1 // VIP1
	VIPLevel2 = 2 // VIP2
	VIPLevel3 = 3 // VIP3
	VIPLevel4 = 4 // VIP4
	VIPLevel5 = 5 // VIP5
	VIPLevel6 = 6 // VIP6
	VIPLevel7 = 7 // VIP7
	VIPLevel8 = 8 // VIP8
	VIPLevel9 = 9 // VIP9
)

// VIP等级状态
const (
	VIPStatusInactive = 0 // 未激活
	VIPStatusActive   = 1 // 激活中
	VIPStatusExpired  = 2 // 已过期
)

// 质押调整类型（用于API接口）
const (
	StakeAdjustTypeStaking     = "staking"     // 质押类型
	StakeAdjustTypeNode        = "node"        // 节点类型
	StakeAdjustTypeCombination = "combination" // 组合类型
)

// StakingCategory 质押类别（用于前端展示）
const (
	StakeCategoryStaking     = "staking"
	StakeCategoryNode        = "node"
	StakeCategoryCombination = "combination"
)
