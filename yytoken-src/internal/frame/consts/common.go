package consts

// TimeConst 时间相关常量
const (
	// 时区
	TimezoneUTC8 = "Asia/Shanghai"

	// 时间格式
	TimeFormatDate     = "2006-01-02"
	TimeFormatDateTime = "2006-01-02 15:04:05"
	TimeFormatTime     = "15:04:05"
	TimeFormatUnix     = "2006-01-02T15:04:05Z"

	// 时间戳格式
	TimestampFormat = "2006-01-02 15:04:05"

	// 时间单位
	SecondsPerDay = 86400 // 每天秒数（24 * 60 * 60）
)

// BatchConst 批次处理相关常量
const (
	// 批次大小限制（避免PostgreSQL参数限制）
	MaxBatchSizeUserQuery    = 30000 // 用户查询批次大小
	MaxBatchSizeRecordInsert = 6000  // 记录插入批次大小
	// MaxTeamRecursionDepth 团队树递归最大深度，防止数据环导致无限递归
	MaxTeamRecursionDepth = 50
)

// PrecisionConst 精度相关常量
const (
	// 区块链代币精度（18位小数）
	TokenPrecision = 18

	// 金额精度（2位小数）
	AmountPrecision = 2

	// 百分比精度（4位小数）
	PercentPrecision = 4
)

// UserIDConst 特殊用户ID常量
const (
	SystemUserID = -1 // 系统账户ID（用于统计全网奖励发放总额）
)

// RewardTypeConst 奖励类型统计相关常量
var (
	// RewardTypesForStatistics 用于统计的奖励类型列表
	// 包含所有需要统计的奖励业务类型
	RewardTypesForStatistics = []string{
		AssetBusinessTypeRewardStatic,        // 静态收益
		AssetBusinessTypeRewardDistrict,      // 小区奖励
		AssetBusinessTypeRewardReferral,      // 推荐奖励
		AssetBusinessTypeRewardWeighted,      // 加权奖励
		AssetBusinessTypeRewardNode,          // 节点分红
		AssetBusinessTypeRewardServiceCenter, // 服务中心奖励
	}

	// RewardTypesForTotalRelease 用于“累计总奖励/释放总金额”的奖励类型集合
	// 说明：不包含服务中心奖励，仅统计用户自身的各类释放与奖励类型
	RewardTypesForTotalRelease = []string{
		AssetBusinessTypeRewardStatic,
		AssetBusinessTypeRewardDistrict,
		AssetBusinessTypeRewardReferral,
		AssetBusinessTypeRewardWeighted,
		AssetBusinessTypeRewardNode,
	}
)

// StatusConst 通用状态码
const (
	// 通用状态
	StatusEnabled  = 1 // 启用
	StatusDisabled = 0 // 禁用

	// 用户状态
	UserStatusActive   = 1  // 活跃
	UserStatusInactive = 0  // 非活跃
	UserStatusBanned   = -1 // 封禁

	// 订单状态
	OrderStatusPending    = 0  // 待处理
	OrderStatusProcessing = 1  // 处理中
	OrderStatusCompleted  = 2  // 已完成
	OrderStatusCancelled  = -1 // 已取消
	OrderStatusFailed     = -2 // 失败

	// 交易状态
	TransactionStatusPending   = 0  // 待确认
	TransactionStatusConfirmed = 1  // 已确认
	TransactionStatusFailed    = -1 // 失败

	// 管理员状态
	AdminStatusActive   = 1 // 活跃
	AdminStatusInactive = 0 // 非活跃

	// 权限状态
	PermissionStatusEnabled  = 1 // 启用
	PermissionStatusDisabled = 0 // 禁用
)
