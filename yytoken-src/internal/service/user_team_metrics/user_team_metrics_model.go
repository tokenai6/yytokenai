package user_team_metrics

import (
	"time"

	"github.com/shopspring/decimal"
)

// CreateMetricsReq 创建用户团队指标请求
type CreateMetricsReq struct {
	Address string `json:"address" v:"required#地址不能为空"`
}

// CreateMetricsRes 创建用户团队指标响应
type CreateMetricsRes struct {
	Id int64 `json:"id"`
}

// GetMetricsListReq 获取用户团队指标列表请求
type GetMetricsListReq struct {
	Address  string `json:"address"`   // 地址（模糊查询）
	Page     int    `json:"page"`      // 页码
	PageSize int    `json:"page_size"` // 每页数量
}

// GetMetricsListRes 获取用户团队指标列表响应
type GetMetricsListRes struct {
	List     []*MetricsItem `json:"list"`      // 列表数据
	Total    int            `json:"total"`     // 总记录数
	Page     int            `json:"page"`      // 当前页码
	PageSize int            `json:"page_size"` // 每页数量
	Pages    int            `json:"pages"`     // 总页数
}

// MetricsItem 用户团队指标项
type MetricsItem struct {
	Id                      int64           `json:"id"`
	UserId                  int64           `json:"user_id"`
	Address                 string          `json:"address"`
	LastCycleTeamRelease    decimal.Decimal `json:"last_cycle_team_release"`
	LastCycleTeamNew        decimal.Decimal `json:"last_cycle_team_new"`
	CurrentCycleTeamRelease decimal.Decimal `json:"current_cycle_team_release"`
	CurrentCycleTeamNew     decimal.Decimal `json:"current_cycle_team_new"`
	TeamReleaseRatio        decimal.Decimal `json:"team_release_ratio"`
	NextRatioChangeDate     *time.Time      `json:"next_ratio_change_date"`
	CycleEndDate            *time.Time      `json:"cycle_end_date"`
	UpdatedAt               time.Time       `json:"updated_at"`
}

// GetMetricsDetailRes 获取用户团队指标详情响应
type GetMetricsDetailRes struct {
	Id                      int64           `json:"id"`
	UserId                  int64           `json:"user_id"`
	Address                 string          `json:"address"`
	LastCycleTeamRelease    decimal.Decimal `json:"last_cycle_team_release"`
	LastCycleTeamNew        decimal.Decimal `json:"last_cycle_team_new"`
	CurrentCycleTeamRelease decimal.Decimal `json:"current_cycle_team_release"`
	CurrentCycleTeamNew     decimal.Decimal `json:"current_cycle_team_new"`
	TeamReleaseRatio        decimal.Decimal `json:"team_release_ratio"`
	NextRatioChangeDate     *time.Time      `json:"next_ratio_change_date"`
	UpdatedAt               time.Time       `json:"updated_at"`
	CycleEndDate            *time.Time      `json:"cycle_end_date"`
}

// DeleteMetricsRes 删除用户团队指标响应
type DeleteMetricsRes struct {
	Message string `json:"message"`
}
