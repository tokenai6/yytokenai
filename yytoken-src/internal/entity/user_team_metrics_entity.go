package entity

import (
	"XWFrame/internal/frame/model"
	"time"

	"github.com/shopspring/decimal"
)

// UserTeamMetricsEntity 用户团队指标实体
type UserTeamMetricsEntity struct {
	model.BaseEntity
	Id                      int64           `json:"id" g:"column:id;primary_key;auto_increment"`
	UserId                  int64           `json:"user_id" g:"column:user_id;not null"`
	Address                 string          `json:"address" g:"column:address;size:42;unique;not null"`
	LastCycleTeamRelease    decimal.Decimal `json:"last_cycle_team_release" g:"column:last_cycle_team_release;type:decimal(38,18);default:0"`
	LastCycleTeamNew        decimal.Decimal `json:"last_cycle_team_new" g:"column:last_cycle_team_new;type:decimal(38,18);default:0"`
	CurrentCycleTeamRelease decimal.Decimal `json:"current_cycle_team_release" g:"column:current_cycle_team_release;type:decimal(38,18);default:0"`
	CurrentCycleTeamNew     decimal.Decimal `json:"current_cycle_team_new" g:"column:current_cycle_team_new;type:decimal(38,18);default:0"`
	TeamReleaseRatio        decimal.Decimal `json:"team_release_ratio" g:"column:team_release_ratio;type:decimal(5,2);default:0"`
	NextRatioChangeDate     *time.Time      `json:"next_ratio_change_date" g:"column:next_ratio_change_date;type:date"`
	CycleEndDate            *time.Time      `json:"cycle_end_date" g:"column:cycle_end_date;type:timestamptz"`
}
