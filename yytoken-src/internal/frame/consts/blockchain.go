package consts

// ============================================
// 区块链事件监听服务默认配置常量
// ============================================

const (
	// ==========================================
	// 代币精度（以太坊标准）
	// ==========================================
	BlockchainDefaultDecimals = 18 // 默认18位小数（1e18）

	// ==========================================
	// 扫块配置
	// ==========================================
	BlockchainDefaultScanInterval       = 30   // 扫块间隔（秒）
	BlockchainDefaultConfirmationBlocks = 6    // 区块确认数（BSC标准）
	BlockchainDefaultBatchSize          = 100  // 批量大小（块）
	BlockchainDefaultMaxBlocksPerQuery  = 5000 // 单次最大查询范围（块）
	BlockchainDefaultEnableReorgCheck   = true // 启用重组检查
	BlockchainDefaultReorgDepth         = 20   // 重组检查深度（块）

	// ==========================================
	// 重试配置
	// ==========================================
	BlockchainDefaultMaxRetry    = 3    // 最大重试次数
	BlockchainDefaultRetryDelay  = 5    // 重试延迟（秒）
	BlockchainDefaultAutoRestart = true // 自动重启

	// ==========================================
	// 监控配置
	// ==========================================
	BlockchainDefaultHeartbeatInterval = 300  // 心跳间隔（秒）
	BlockchainDefaultEnableMetrics     = true // 启用监控指标

	// ==========================================
	// 超时配置
	// ==========================================
	BlockchainDefaultRPCTimeout         = 30  // RPC调用超时（秒）
	BlockchainDefaultBlockQueryTimeout  = 60  // 区块查询超时（秒）
	BlockchainDefaultEventQueryTimeout  = 300 // 事件查询超时（秒）
	BlockchainDefaultTransactionTimeout = 30  // 交易查询超时（秒）

	// ==========================================
	// 限流配置
	// ==========================================
	BlockchainDefaultRateLimitEnabled  = true // 启用限流
	BlockchainDefaultRequestsPerSecond = 2    // 每秒请求数（2次/秒 = 500ms间隔）
	BlockchainDefaultBurst             = 1    // 突发请求数
)

// 默认网络类型
const BlockchainDefaultNetworkType = "ethereum"

// ==========================================
// Redis缓存Key常量
// ==========================================

const BlockchainCacheKeyPrefix = "cache:blockchain:"

// 事件监听进度缓存Key
const (
	BlockchainCacheKeyUnifiedLastBlock        = BlockchainCacheKeyPrefix + "unified:last_block"          // 统一扫块器进度
	BlockchainCacheKeyApgMintLastBlock        = BlockchainCacheKeyPrefix + "apg_mint:last_block"         // APG Mint扫块器进度
	BlockchainCacheKeyApgMintLastBlockMigrate = BlockchainCacheKeyPrefix + "apg_mint:last_block:migrate" // APG Mint扫块器进度（迁移模式）
)

// 监听器心跳缓存Key
const (
	BlockchainCacheKeyStakeListenerHeart = BlockchainCacheKeyPrefix + "stake_listener:heartbeat"
	BlockchainCacheKeyMintListenerHeart  = BlockchainCacheKeyPrefix + "mint_listener:heartbeat"
	BlockchainCacheKeyBurnListenerHeart  = BlockchainCacheKeyPrefix + "burn_listener:heartbeat"
)

// 累计销毁总量缓存Key
const BlockchainCacheKeySwapBurnTotal = BlockchainCacheKeyPrefix + "swap_burn_total"

// ==========================================
// 事件ID常量
// ==========================================

const (
	// HashPowerStaked 事件ID
	BlockchainEventIDStake = "0xe34e954a25f0f2d5436889a98539c4d2bdca2ed426c7dcd8e367af0e1a60d91f"

	// SignatureMint 事件ID
	BlockchainEventIDMint = "0xc53126bf2bc2c787c9c85624db3eb39e9dd342a9cccf62aa6135e57f0d854129"

	// SwapFeeBurned 事件ID
	BlockchainEventIDBurn = "0xd758143be2ff7953f87e9aa230b35625bfb79c7bdea0152e30ddd3b5aeda9dbd"

	// Transfer 事件ID（ERC20标准）
	BlockchainEventIDTransfer = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

	// HashPowerStaked 组合质押事件ID（双币质押）
	BlockchainEventIDCombinationStake = "0x430b491431ef15724c1bb222b8450bde9f5e488b3110dee90f6008597104b704"

	// StakeFromMintGame APG Mint质押事件ID（输家60%代币兑换U购买算力）
	// 事件签名: StakeFromMintGame(address indexed user, uint256 indexed stakeIndex, uint256 indexed groupType, uint256 groupIndex, uint256 usdtAmount, uint8 stakeType, uint256 rexPrice, uint256 timestamp)
	BlockchainEventIDMintStake = "0x273cf2083981b88e76126f4ca7c875037b0a13c68e13962f6a5fab2873c26441"

	// HashPowerGroup 拼团事件ID
	// 事件签名: HashPowerGroup(address indexed user, address indexed token, uint256 indexed index, uint256 groupType, uint256 amount, uint256 liquidity, uint256 usdtValue, uint256 tokenAmount, uint256 timestamp)
	BlockchainEventIDApgGroup = "0xcd77865ff62f39e2807f31ad54d5f11af3b8819375980d53568e33be82694e1b"

	// RefundUserLp 领取/退款事件ID
	// 事件签名: RefundUserLp(address indexed user, uint256 indexed index, address indexed token, address rewardToken, uint256 lpAmount, uint256 tokenAmount, uint256 rewardAmount, uint256 nonce)
	BlockchainEventIDApgRefund = "0x97ce1ecbdd67c0b2261f56c217618d8e03c079ea99ddbdc6a5c22d64fdc87730"

	// RefundReferralReward 推荐奖励领取事件ID
	// 事件签名: RefundReferralReward(address indexed sender, address indexed token, uint256 amount, uint256 nonce)
	BlockchainEventIDApgReferralReward = "0x6c4241698fc14ae504e2ff721d6d21b42977a311582d2355f24a79b752fa3a4e"
)

// ==========================================
// 监听器名称常量
// ==========================================

const (
	BlockchainListenerNameStake               = "StakeEventListener"
	BlockchainListenerNameMint                = "MintEventListener"
	BlockchainListenerNameBurn                = "BurnEventListener"
	BlockchainListenerNameNodeDividendUsdt    = "NodeDividendUsdtListener"
	BlockchainListenerNameCombinationStake    = "CombinationStakeEventListener"
	BlockchainListenerNameMintStake           = "MintStakeEventListener"
	BlockchainListenerNameApgGroup            = "ApgGroupEventListener"
	BlockchainListenerNameApgRefund           = "ApgRefundEventListener"
	BlockchainListenerNameApgReferralReward   = "ApgReferralRewardEventListener"
)

// ==========================================
// 处理器名称常量（用于容错性重试）
// ==========================================

const (
	BlockchainHandlerNameStake               = "stake_listener"
	BlockchainHandlerNameMint                = "mint_listener"
	BlockchainHandlerNameBurn                = "burn_listener"
	BlockchainHandlerNameNodeDividendUsdt    = "node_dividend_usdt_listener"
	BlockchainHandlerNameCombinationStake    = "combination_stake_listener"
	BlockchainHandlerNameMintStake           = "mint_stake_listener"
	BlockchainHandlerNameApgGroup            = "apg_group_listener"
	BlockchainHandlerNameApgRefund           = "apg_refund_listener"
	BlockchainHandlerNameApgReferralReward   = "apg_referral_reward_listener"
)

// ==========================================
// 事件扫块容错性默认配置常量
// ==========================================

const (
	// ==========================================
	// 重试机制配置
	// ==========================================
	FaultToleranceDefaultRetryEnabled                  = true // 启用重试机制
	FaultToleranceDefaultImmediateRetryEnabled         = true // 启用立即重试
	FaultToleranceDefaultImmediateRetryMaxAttempts     = 1    // 立即重试最大次数
	FaultToleranceDefaultImmediateRetryTimeout         = 30   // 立即重试超时时间（秒）
	FaultToleranceDefaultScheduledRetryEnabled         = true // 启用定时重试
	FaultToleranceDefaultManualRetryEnabled            = true // 启用手动重试
	FaultToleranceDefaultBatchRetryEnabled             = true // 启用批量重试
	FaultToleranceDefaultBatchRetryMaxSize             = 100  // 批量重试最大数量
	FaultToleranceDefaultRecoveryRetryEnabled          = true // 启用系统恢复重试
	FaultToleranceDefaultRecoveryRetryTriggerOnStartup = true // 系统启动时触发恢复重试

	// ==========================================
	// 熔断器配置
	// ==========================================
	FaultToleranceDefaultCircuitBreakerEnabled  = true // 启用熔断器
	FaultToleranceDefaultFailureThreshold       = 50   // 失败阈值（百分比）
	FaultToleranceDefaultSuccessThreshold       = 80   // 成功阈值（百分比）
	FaultToleranceDefaultCircuitBreakerTimeout  = 300  // 熔断超时时间（秒）
	FaultToleranceDefaultRequestVolumeThreshold = 100  // 请求量阈值
	FaultToleranceDefaultSleepWindow            = 30   // 睡眠窗口（秒）
	FaultToleranceDefaultTestRequestCount       = 10   // 测试请求数

	// 分层熔断配置
	FaultToleranceDefaultNetworkLayerEnabled           = true  // 启用网络层熔断
	FaultToleranceDefaultNetworkLayerFailureThreshold  = 60    // 网络层失败阈值
	FaultToleranceDefaultNetworkLayerTimeout           = 180   // 网络层超时时间
	FaultToleranceDefaultServiceLayerEnabled           = true  // 启用服务层熔断
	FaultToleranceDefaultServiceLayerFailureThreshold  = 40    // 服务层失败阈值
	FaultToleranceDefaultServiceLayerTimeout           = 300   // 服务层超时时间
	FaultToleranceDefaultBusinessLayerEnabled          = false // 业务层熔断默认关闭
	FaultToleranceDefaultBusinessLayerFailureThreshold = 30    // 业务层失败阈值
	FaultToleranceDefaultBusinessLayerTimeout          = 600   // 业务层超时时间

	// ==========================================
	// 失败事件管理配置
	// ==========================================
	FaultToleranceDefaultFailedEventsTTL                 = 604800 // 失败事件保留时间（秒，7天）
	FaultToleranceDefaultFailedEventsMaxRetries          = 0      // 最大重试次数（0=无限制）
	FaultToleranceDefaultFailedEventsAutoRetryEnabled    = true   // 是否启用自动重试
	FaultToleranceDefaultFailedEventsAutoRetryInterval   = 10     // 自动重试间隔（秒）
	FaultToleranceDefaultFailedEventsAutoRetryLimit      = 3      // 自动重试次数限制
	FaultToleranceDefaultFailedEventsStatusCheckInterval = 60     // 状态检查间隔（秒）
	FaultToleranceDefaultFailedEventsCleanupInterval     = 3600   // 清理间隔（秒）
	FaultToleranceDefaultFailedEventsCleanupBatchSize    = 1000   // 清理批量大小

)

// ==========================================
// 重试间隔常量（秒）
// ==========================================

var FaultToleranceDefaultScheduledRetryIntervals = []int{
	60,
	120,
	300,   // 5分钟
	900,   // 15分钟
	1800,  // 30分钟
	3600,  // 1小时
	10800, // 3小时
	21600, // 6小时
	43200, // 12小时
	86400, // 1天
}

// ==========================================
// 事件优先级常量（预留，用于未来优先级队列功能）
// ==========================================

const (
	EventPriorityHigh   = "high"   // 高优先级（销毁事件、提取事件）
	EventPriorityMedium = "medium" // 中优先级（质押事件）
	EventPriorityLow    = "low"    // 低优先级（其他事件）
)

// ==========================================
// 失败事件状态常量
// ==========================================

const (
	FailedEventStatusPending   = "pending"   // 待处理
	FailedEventStatusRetrying  = "retrying"  // 重试中
	FailedEventStatusCompleted = "completed" // 已完成
	FailedEventStatusAbandoned = "abandoned" // 已放弃
)

// ==========================================
// 尝试结果常量
// ==========================================

const (
	AttemptResultSuccess = "success" // 成功
	AttemptResultFailed  = "failed"  // 失败
)

// ==========================================
// 尝试类型常量
// ==========================================

const (
	AttemptTypeImmediate = "immediate" // 立即重试
	AttemptTypeScheduled = "scheduled" // 定时重试
	AttemptTypeManual    = "manual"    // 手动重试
	AttemptTypeBatch     = "batch"     // 批量重试
	AttemptTypeRecovery  = "recovery"  // 系统恢复重试
)

// ==========================================
// 错误类型常量
// ==========================================

const (
	ErrorTypeNetwork    = "network_error"    // 网络错误
	ErrorTypeDatabase   = "database_error"   // 数据库错误
	ErrorTypeService    = "service_error"    // 服务错误
	ErrorTypeBusiness   = "business_error"   // 业务错误
	ErrorTypeTimeout    = "timeout_error"    // 超时错误
	ErrorTypeValidation = "validation_error" // 验证错误
	ErrorTypeUnknown    = "unknown_error"    // 未知错误
)

// ==========================================
// 熔断器配置常量
// ==========================================

// CircuitBreakerConfigData 熔断器配置数据
type CircuitBreakerConfigData struct {
	FailureThreshold       int // 失败阈值
	SuccessThreshold       int // 成功阈值
	Timeout                int // 超时时间（秒）
	RequestVolumeThreshold int // 请求量阈值
}

// CircuitBreakerConfigs 熔断器配置映射
var CircuitBreakerConfigs = map[string]CircuitBreakerConfigData{
	BlockchainHandlerNameBurn: {
		FailureThreshold:       50,  // 失败阈值
		SuccessThreshold:       80,  // 成功阈值
		Timeout:                300, // 超时时间（秒）
		RequestVolumeThreshold: 100, // 请求量阈值
	},
	BlockchainHandlerNameMint: {
		FailureThreshold:       40,  // 失败阈值
		SuccessThreshold:       85,  // 成功阈值
		Timeout:                180, // 超时时间（秒）
		RequestVolumeThreshold: 50,  // 请求量阈值
	},
	BlockchainHandlerNameStake: {
		FailureThreshold:       60,  // 失败阈值
		SuccessThreshold:       75,  // 成功阈值
		Timeout:                600, // 超时时间（秒）
		RequestVolumeThreshold: 200, // 请求量阈值
	},
}
