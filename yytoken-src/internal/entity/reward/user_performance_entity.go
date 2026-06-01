package reward

import (
	"time"

	"XWFrame/internal/frame/model"

	"github.com/shopspring/decimal"
)

// UserPerformanceEntity 用户业绩实体
//
// Deprecated: 该表已废弃，团队业绩统一使用 cobo_node_purchase 表实时计算
// 请使用 cobo_performance 缓存表或直接从 cobo_node_purchase 表查询
// 保留该表仅用于兼容历史数据
//
// 替代方案：
// - 实时查询：使用 internal/service/teamstats GetOverviewAggByInviteCode
// - 缓存查询：使用 internal/service/cobo PerformanceService
//
// 记录用户在某个时刻（record_time）的业绩数据，包括累计业绩和新增业绩
type UserPerformanceEntity struct {
	model.BaseEntity
	// UserID 用户ID
	UserID int64 `json:"user_id" gorm:"not null;index:idx_user_perf_record_date"`

	// RecordTime 业绩记录时间（精确到秒，用于区分不同的业绩批次）
	RecordTime time.Time `json:"record_time" gorm:"type:timestamp;not null;index:idx_user_perf_record_date,priority:2;index:idx_user_perf_record_time"`

	// ========== 累计业绩字段（基于当前网体结构和质押数据计算） ==========

	// PersonalPerformance 个人业绩：该用户当前总质押金额（所有运行中算力包的质押金额总和）
	PersonalPerformance decimal.Decimal `json:"personal_performance" gorm:"type:decimal(28,18);default:0"`

	// TeamPerformance 团队业绩：除去自己，下面网体的所有个人业绩之和
	// 计算公式：团队业绩 = 子树质押总和 - 自己的质押金额
	TeamPerformance decimal.Decimal `json:"team_performance" gorm:"type:decimal(28,18);default:0"`

	// TeamCombinationPerformance 团队业绩：除去自己，下面网体的所有组合质押金额之和
	TeamCombinationPerformance decimal.Decimal `json:"team_combination_performance" gorm:"type:decimal(28,18);default:0"`

	// PersonalCombinationPerformance 个人组合业绩：该用户当前总组合质押金额（所有运行中组合算力包的质押金额总和）
	PersonalCombinationPerformance decimal.Decimal `json:"personal_combination_performance" gorm:"type:decimal(28,18);default:0"`

	// DistrictPerformance 小区业绩：所有子区的团队业绩总和 - 最大子区团队业绩
	// 算法：每个直接下级形成一个区，小区业绩 = 所有区的团队业绩总和 - 最大区团队业绩
	DistrictPerformance decimal.Decimal `json:"district_performance" gorm:"type:decimal(28,18);default:0"`

	// MaxDistrictPerformance 最大区团队业绩：所有直接下级中，子树质押最大的值
	// 用于VIP等级计算和小区业绩计算
	MaxDistrictPerformance decimal.Decimal `json:"max_district_performance" gorm:"type:decimal(28,18);default:0"`

	// DirectReferralCount 直推人数：直接下级用户数量
	DirectReferralCount int `json:"direct_referral_count" gorm:"default:0"`

	// TeamMemberCount 团队人数：所有下级用户数量（包括直接和间接下级）
	TeamMemberCount int `json:"team_member_count" gorm:"default:0"`

	// TeamTotalCount 当时的团队总人数（与TeamMemberCount相同，保留用于兼容）
	TeamTotalCount int `json:"team_total_count" gorm:"default:0"`

	// ActiveMemberCount 有效人数：直推用户中，个人业绩≥阈值的用户数量（用于VIP评估）
	ActiveMemberCount int `json:"active_member_count" gorm:"default:0"`

	// ========== 新增业绩字段（基于时间区间统计，用于计算业绩变化） ==========

	// NewPersonalPerformance 新增个人业绩：在[上一批次时间, 当前时间]区间内开始的新质押金额
	// 用于统计个人业绩的增长量
	NewPersonalPerformance decimal.Decimal `json:"new_personal_performance" gorm:"type:decimal(28,18);default:0"`

	// NewDirectPerformance 新增直推业绩：所有直接下级的新增个人业绩总和
	// 用于统计直推业绩的增长量
	NewDirectPerformance decimal.Decimal `json:"new_direct_performance" gorm:"type:decimal(28,18);default:0;index:idx_user_perf_new_direct"`

	// NewTeamPerformance 新增团队业绩：所有下级的新增个人业绩总和（不包括自己）
	// 用于统计团队业绩的增长量
	NewTeamPerformance decimal.Decimal `json:"new_team_performance" gorm:"type:decimal(28,18);default:0"`

	// NewDistrictPerformance 新增小区业绩：所有子区的新增团队业绩总和 - 最大子区的新增团队业绩
	// 用于统计小区业绩的增长量
	NewDistrictPerformance decimal.Decimal `json:"new_district_performance" gorm:"type:decimal(28,18);default:0"`

	// NewExpiredStake 新增出局质押：所有下级在[上一批次时间, 当前时间]区间内结束的质押金额总和
	// 用于统计出局质押的增长量
	NewExpiredStake decimal.Decimal `json:"new_expired_stake" gorm:"type:decimal(28,18);default:0"`

	// ========== 其他字段 ==========

	// TotalPowerValue 总算力值：与PersonalPerformance相同（保留用于兼容）
	TotalPowerValue decimal.Decimal `json:"total_power_value" gorm:"type:decimal(28,18);default:0"`

	// VipLevel VIP等级：基于小区业绩评定（需要个人业绩≥阈值才能评定VIP）
	VipLevel int `json:"vip_level" gorm:"default:0"`

	// Remark 备注信息
	Remark string `json:"remark" gorm:"type:text;default:''"`

	// Metadata 元数据（JSON格式，用于存储额外的业务数据）
	Metadata string `json:"metadata" gorm:"type:jsonb;default:'{}'"`
}

// TableName 指定表名
func (UserPerformanceEntity) TableName() string {
	return "user_performance"
}

// PerformanceMetadata 业绩记录元数据
type PerformanceMetadata struct {
	EffectiveVipLevel int `json:"effective_vip_level"` // 实际生效的VIP等级（考虑调整后的，记录时的有效等级）
}
