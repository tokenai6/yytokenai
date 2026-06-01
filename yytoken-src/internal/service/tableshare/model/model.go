package model

import frameModel "XWFrame/internal/frame/model"

type ApplicationItem struct {
	Id                                   int64    `json:"id"`
	UserId                               int64    `json:"user_id"`
	WalletAddress                        string   `json:"wallet_address"`
	Community                            string   `json:"community"`
	DiningPeopleCount                    int      `json:"dining_people_count"`
	SmallTeamPerformance                 string   `json:"small_team_performance"`
	PreviousApprovedSmallTeamPerformance string   `json:"previous_approved_small_team_performance"`
	CommunicationVideoUrls               []string `json:"communication_video_urls"`
	SharingVideoUrls                     []string `json:"sharing_video_urls"`
	SloganVideoUrls                      []string `json:"slogan_video_urls"`
	Status                               int      `json:"status"`
	ShareAmount                          string   `json:"share_amount"`
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
	Enabled                 bool   `json:"enabled"`
	SmallTeamPerformance    string `json:"small_team_performance"`
	MinSmallTeamPerformance string `json:"min_small_team_performance"`
	MaxAmount               string `json:"max_amount"`
	DailyLimit              int    `json:"daily_limit"`
	TodayAppliedCount       int    `json:"today_applied_count"`
	CanApply                bool   `json:"can_apply"`
}

type AdminConfigRes struct {
	Enabled                 bool   `json:"enabled"`
	MinSmallTeamPerformance string `json:"min_small_team_performance"`
	MaxAmount               string `json:"max_amount"`
	DailyLimit              int    `json:"daily_limit"`
}
