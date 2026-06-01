package entity

import (
	"time"

	"github.com/shopspring/decimal"
)

// StakingV2PerformanceEntity staking_v2_performance 团队三倍券业绩缓存实体
// 用途：缓存 staking_v2_order (source_type=1) 的团队业绩数据，避免频繁实时计算
// 更新频率：每5分钟检测新订单后更新
type StakingV2PerformanceEntity struct {
	ID                   int64           `json:"id" orm:"id"`
	UserID               int64           `json:"user_id" orm:"user_id"`                               // 用户ID
	WalletAddress        string          `json:"wallet_address" orm:"wallet_address"`                 // 用户钱包地址
	InviteCode           string          `json:"invite_code" orm:"invite_code"`                       // 用户邀请码
	PersonalPerformance  decimal.Decimal `json:"personal_performance" orm:"personal_performance"`     // 个人业绩（该用户的 staking_v2_order 手动质押金额总和）
	TeamPerformance      decimal.Decimal `json:"team_performance" orm:"team_performance"`             // 团队业绩（该用户所有下级的 staking_v2_order 手动质押金额总和）
	OrderCount           int             `json:"order_count" orm:"order_count"`                       // 统计的订单记录数
	MaxOrderID           int64           `json:"max_order_id" orm:"max_order_id"`                     // 已处理的最大 staking_v2_order.id（用于增量更新）
	UpdatedAt            time.Time       `json:"updated_at" orm:"updated_at"`                         // 更新时间
	CreatedAt            time.Time       `json:"created_at" orm:"created_at"`                         // 创建时间
}

// TableName 表名
func (e StakingV2PerformanceEntity) TableName() string {
	return "staking_v2_performance"
}

// IsFresh 检查缓存是否新鲜（5分钟内更新过）
func (e StakingV2PerformanceEntity) IsFresh() bool {
	return time.Since(e.UpdatedAt) < 5*time.Minute
}

// NeedRefresh 检查是否需要刷新（超过5分钟或有新订单）
func (e StakingV2PerformanceEntity) NeedRefresh(latestOrderID int64) bool {
	// 超过5分钟需要刷新
	if time.Since(e.UpdatedAt) >= 5*time.Minute {
		return true
	}
	// 有新的订单需要刷新
	if latestOrderID > e.MaxOrderID {
		return true
	}
	return false
}
