package performance

import "github.com/shopspring/decimal"

// RealtimePerformanceRes 实时业绩响应
// 用于前端实时展示用户的团队业绩和小区业绩（不依赖历史业绩记录，基于当前网体结构和质押数据实时计算）
type RealtimePerformanceRes struct {
	// TeamPerformance 团队业绩：除去自己，下面网体的所有个人业绩之和
	TeamPerformance decimal.Decimal `json:"team_performance"`

	// DistrictPerformance 小区业绩：所有子区的团队业绩总和 - 最大子区团队业绩
	DistrictPerformance decimal.Decimal `json:"district_performance"`

	// MaxDistrictPerformance 最大区团队业绩：所有直接下级中，子树质押最大的值
	MaxDistrictPerformance decimal.Decimal `json:"max_district_performance"`

	// TeamMemberCount 团队人数：所有下级用户数量（包括直接和间接下级）
	TeamMemberCount int `json:"team_member_count"`

	// DirectReferralCount 直推人数：直接下级用户数量
	DirectReferralCount int `json:"direct_referral_count"`

	// ActiveDirectMemberCount 直推有效人数：直推中个人业绩达到阈值的数量
	ActiveDirectMemberCount int `json:"active_direct_member_count"`
}
