package reward

import (
	"XWFrame/internal/frame/model"
	"time"

	"github.com/shopspring/decimal"
)

// AssetRecordEntity 资金记录实体
type AssetRecordEntity struct {
	model.BaseEntity
	UserID       int64           `json:"user_id" gorm:"not null;uniqueIndex:idx_asset_record_unique;index:idx_asset_record_user_time"`
	AssetType    string          `json:"asset_type" gorm:"size:50;not null;index:idx_asset_record_type_time"`
	RecordTime   time.Time       `json:"record_time" gorm:"type:timestamp;not null;uniqueIndex:idx_asset_record_unique,priority:2;index:idx_asset_record_user_time,priority:2;index:idx_asset_record_time"`
	Amount       decimal.Decimal `json:"amount" gorm:"type:decimal(28,18);not null"`
	FlowType     string          `json:"flow_type" gorm:"size:20;not null;index:idx_asset_record_flow"`
	Status       int             `json:"status" gorm:"not null;default:1;index:idx_asset_record_status"`
	BusinessType string          `json:"business_type" gorm:"size:50;not null;uniqueIndex:idx_asset_record_unique,priority:3;index:idx_asset_record_business"`
	BusinessID   int64           `json:"business_id" gorm:"default:0;index:idx_asset_record_business_id"`
	Remark       string          `json:"remark" gorm:"size:200"`
	Metadata     string          `json:"metadata" gorm:"type:jsonb"`
}

// TableName 指定表名
func (AssetRecordEntity) TableName() string {
	return "asset_record"
}

// StaticRewardMetadata 静态收益元数据
type StaticRewardMetadata struct {
	PackageID   int64           `json:"package_id"`   // 算力包ID
	StakeAmount decimal.Decimal `json:"stake_amount"` // 质押金额
	PowerValue  decimal.Decimal `json:"power_value"`  // 算力值
	YieldRate   decimal.Decimal `json:"yield_rate"`   // 收益率（日利率）
}

// DistrictAllocatedUserDetail 已分配奖励的用户明细
type DistrictAllocatedUserDetail struct {
	UserID            int64           `json:"user_id"`            // 用户ID（分走奖励的用户）
	VipLevel          int             `json:"vip_level"`          // VIP等级
	AllocatedRate     decimal.Decimal `json:"allocated_rate"`     // 分走的比例（已分配比例）
	AllocatedReward   decimal.Decimal `json:"allocated_reward"`   // 分走的奖励金额
	SubordinateStatic decimal.Decimal `json:"subordinate_static"` // 该用户伞下的总静态收益
}

// DistrictRewardMetadata 小区奖励元数据
type DistrictRewardMetadata struct {
	VipLevel               int                           `json:"vip_level"`                // 用于计算奖励的有效VIP等级
	CalculatedVipLevel     int                           `json:"calculated_vip_level"`     // 业绩计算得到的VIP等级（原始）
	AdjustedVipLevel       int                           `json:"adjusted_vip_level"`       // 调整后生效的VIP等级（如果有调整，否则等于计算等级）
	RewardRate             decimal.Decimal               `json:"reward_rate"`              // 奖励比例（VIP等级对应的奖励比例）
	DistrictPerformance    decimal.Decimal               `json:"district_performance"`     // 小区业绩
	SubordinateStaticTotal decimal.Decimal               `json:"subordinate_static_total"` // 下级静态收益总和
	AllocatedUsers         []DistrictAllocatedUserDetail `json:"allocated_users"`          // 已分配奖励的用户列表（分走奖励的用户）
	BranchDetails          []DistrictBranchDetailAsset   `json:"branch_details"`           // 分支详情列表
}

// DistrictBranchDetailAsset 小区奖励分支详情（asset_record用）
type DistrictBranchDetailAsset struct {
	BranchRootUserID  int64           `json:"branch_root_user_id"`  // 分支根用户ID（代表该分支的直接下级用户ID）
	BranchMaxVipLevel int             `json:"branch_max_vip_level"` // 分支最大VIP等级（该分支中已分配奖励的最高VIP等级）
	BranchMaxRate     decimal.Decimal `json:"branch_max_rate"`      // 分支最大奖励比例（该分支中已分配奖励的最高比例）
	BranchStaticTotal decimal.Decimal `json:"branch_static_total"`  // 分支静态收益总和（该分支下所有用户的静态收益总和）
	DiffRate          decimal.Decimal `json:"diff_rate"`            // 极差比例（当前用户可收取的比例 = 用户奖励比例 - 分支最大比例）
	Reward            decimal.Decimal `json:"reward"`               // 该分支的奖励金额（分支静态收益总和 * 极差比例）
}

// ReferralRewardMetadata 推荐奖励元数据
type ReferralRewardMetadata struct {
	ReferralLevel        int             `json:"referral_level"`         // 推荐层级（1-直推，2-间推等）
	ReferralUserID       int64           `json:"referral_user_id"`       // 被推荐用户ID（产生静态收益的用户）
	ReferralStaticReward decimal.Decimal `json:"referral_static_reward"` // 被推荐用户的静态收益金额
	ReferralRate         decimal.Decimal `json:"referral_rate"`          // 推荐奖励比例
}

// WeightedRewardMetadata 加权奖励元数据
type WeightedRewardMetadata struct {
	Ranking               int             `json:"ranking"`                 // 排名（前50名）
	NewDirectPerformance  decimal.Decimal `json:"new_direct_performance"`  // 新增直推业绩
	Top50TotalPerformance decimal.Decimal `json:"top50_total_performance"` // 前50名总业绩
	NetworkTotalOutput    decimal.Decimal `json:"network_total_output"`    // 全网总产出
	WeightRatio           decimal.Decimal `json:"weight_ratio"`            // 权重比例（新增直推业绩 / 前50名总业绩）
}

// NodeRewardMetadata 节点分红元数据
type NodeRewardMetadata struct {
	NodeID       int64           `json:"node_id"`       // 节点ID
	EquityValue  decimal.Decimal `json:"equity_value"`  // 用户持有的节点权益值
	TotalEquity  decimal.Decimal `json:"total_equity"`  // 节点总权益值
	EquityRatio  decimal.Decimal `json:"equity_ratio"`  // 权益比例（用户权益值 / 节点总权益值）
	DividendPool decimal.Decimal `json:"dividend_pool"` // 分红池总额
}

// RewardReducedMetadata 奖励削减元数据
type RewardReducedMetadata struct {
	OriginalAmount decimal.Decimal `json:"original_amount"` // 原始计算金额
	ActualAmount   decimal.Decimal `json:"actual_amount"`   // 实际发放金额
	ReducedAmount  decimal.Decimal `json:"reduced_amount"`  // 削减金额
	ReduceRatio    decimal.Decimal `json:"reduce_ratio"`    // 削减比例
}
