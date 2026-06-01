package cobo

import (
	"time"

	"github.com/shopspring/decimal"
)

// PerformanceEntity cobo_performance 团队业绩缓存实体
// 用途：缓存 cobo_node_purchase 的团队业绩数据，避免频繁实时计算
// 更新频率：每5分钟检测新充值记录后更新
type PerformanceEntity struct {
	ID                    int64           `json:"id" orm:"id"`
	UserID                int64           `json:"user_id" orm:"user_id"`                               // 用户ID
	WalletAddress         string          `json:"wallet_address" orm:"wallet_address"`                 // 用户钱包地址
	InviteCode            string          `json:"invite_code" orm:"invite_code"`                       // 用户邀请码
	PersonalPerformance   decimal.Decimal `json:"personal_performance" orm:"personal_performance"`     // 个人业绩（该用户的 cobo_node_purchase 购买金额总和，不含赠送）
	TeamPerformance       decimal.Decimal `json:"team_performance" orm:"team_performance"`             // 团队业绩（该用户所有下级的 cobo_node_purchase 购买金额总和，不含赠送）
	SmallTeamPerformance  decimal.Decimal `json:"small_team_performance" orm:"small_team_performance"` // 小区业绩（团队业绩 - 最大直推分支业绩）
	DirectCount           int             `json:"direct_count" orm:"direct_count"`                     // 直推人数
	TeamTotalCount        int             `json:"team_total_count" orm:"team_total_count"`             // 团队总人数
	PurchaseCount         int             `json:"purchase_count" orm:"purchase_count"`                 // 统计的购买记录数（用于增量更新判断）
	MaxPurchaseID         int64           `json:"max_purchase_id" orm:"max_purchase_id"`               // 已处理的最大 cobo_node_purchase.id（用于增量更新）
	UpdatedAt             time.Time       `json:"updated_at" orm:"updated_at"`                         // 更新时间
	CreatedAt             time.Time       `json:"created_at" orm:"created_at"`                         // 创建时间
}

// TableName 表名
func (e PerformanceEntity) TableName() string {
	return "cobo_performance"
}

// IsFresh 检查缓存是否新鲜（5分钟内更新过）
func (e PerformanceEntity) IsFresh() bool {
	return time.Since(e.UpdatedAt) < 5*time.Minute
}

// NeedRefresh 检查是否需要刷新（超过5分钟或有新购买记录）
func (e PerformanceEntity) NeedRefresh(latestPurchaseID int64) bool {
	// 超过5分钟需要刷新
	if time.Since(e.UpdatedAt) >= 5*time.Minute {
		return true
	}
	// 有新的购买记录需要刷新
	if latestPurchaseID > e.MaxPurchaseID {
		return true
	}
	return false
}
