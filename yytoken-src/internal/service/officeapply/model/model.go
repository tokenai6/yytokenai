package model

import frameModel "XWFrame/internal/frame/model"

type LevelItem struct {
	Level          int    `json:"level"`
	MinPerformance string `json:"min_performance"`
	SubsidyAmount  string `json:"subsidy_amount"`
}

type ApplicationItem struct {
	Id                                   int64    `json:"id"`
	UserId                               int64    `json:"user_id"`
	WalletAddress                        string   `json:"wallet_address"`
	LeaderName                           string   `json:"leader_name"`
	CommunityAddress                     string   `json:"community_address"`
	ContactPhone                         string   `json:"contact_phone"`
	SmallTeamPerformance                 string   `json:"small_team_performance"`
	PreviousApprovedSmallTeamPerformance string   `json:"previous_approved_small_team_performance"`
	EligibleLevel                        int      `json:"eligible_level"`
	SubsidyAmount                        string   `json:"subsidy_amount"`
	ImageUrls                            []string `json:"image_urls"`
	Status                               int      `json:"status"`
	AuditBy                              int64    `json:"audit_by,omitempty"`
	AuditRemark                          string   `json:"audit_remark"`
	AuditAt                              string   `json:"audit_at"`
	CreatedAt                            string   `json:"created_at"`
	UpdatedAt                            string   `json:"updated_at"`
	ApprovalCount                        int      `json:"approval_count"`
	CurrentAdminApproved                 bool     `json:"current_admin_approved"`
}

type ListRes struct {
	frameModel.PageRes
	List []*ApplicationItem `json:"list"`
}

type DetailRes struct {
	*ApplicationItem
}

type ConfigRes struct {
	Enabled               bool             `json:"enabled"`
	PeriodStart           string           `json:"period_start"`
	PeriodEnd             string           `json:"period_end"`
	SmallTeamPerformance  string           `json:"small_team_performance"`
	EligibleLevel         int              `json:"eligible_level"`
	EligibleSubsidyAmount string           `json:"eligible_subsidy_amount"`
	Levels                []*LevelItem     `json:"levels"`
	MaxImages             int              `json:"max_images"`
	CanApply              bool             `json:"can_apply"`
	Application           *ApplicationItem `json:"application"`
}

type AdminConfigRes struct {
	Enabled     bool         `json:"enabled"`
	PeriodStart string       `json:"period_start"`
	PeriodEnd   string       `json:"period_end"`
	Levels      []*LevelItem `json:"levels"`
	MaxImages   int          `json:"max_images"`
}
