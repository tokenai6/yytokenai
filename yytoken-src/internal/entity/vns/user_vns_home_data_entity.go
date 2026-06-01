package vns

import (
	"XWFrame/internal/frame/model"
)

// UserVnsHomeDataEntity VNS首页数据实体
type UserVnsHomeDataEntity struct {
	model.BaseEntity
	UserId                   int64 `json:"user_id" g:"not null;uniqueIndex"`
	VipLevel                 int   `json:"vip_level" g:"not null;default:0"`
	AdminLevel               int   `json:"admin_level" g:"not null;default:0"`
	TeamTotalPerformance     int   `json:"team_total_performance" g:"not null;default:0"`
	NextLevelUpgradeRequired int   `json:"next_level_upgrade_required" g:"not null;default:0"`
	TeamMemberCount          int   `json:"team_member_count" g:"not null;default:0"`
	DirectMemberCount        int   `json:"direct_member_count" g:"not null;default:0"`
	MaxDistrictCount         int   `json:"max_district_count" g:"not null;default:0"`
	MinDistrictCount         int   `json:"min_district_count" g:"not null;default:0"`
	MinDistrictPerformance   int   `json:"min_district_performance" g:"not null;default:0"`
	RewardShare              int   `json:"reward_share" g:"not null;default:0"`
	VsBalance                int   `json:"vs_balance" g:"not null;default:0"`
	UsdtBalance              int   `json:"usdt_balance" g:"not null;default:0"`
}

// TableName 指定表名
func (UserVnsHomeDataEntity) TableName() string {
	return "user_vns_home_data"
}
