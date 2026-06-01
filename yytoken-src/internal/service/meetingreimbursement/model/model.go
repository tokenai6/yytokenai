package model

import frameModel "XWFrame/internal/frame/model"

type ApplicationItem struct {
	Id                   int64    `json:"id"`
	ActivityId           int64    `json:"activity_id"`
	ActivityTitle        string   `json:"activity_title"`
	UserId               int64    `json:"user_id"`
	WalletAddress        string   `json:"wallet_address"`
	Community            string   `json:"community"`
	Email                string   `json:"email"`
	ImageUrls            []string `json:"image_urls"`
	SmallTeamPerformance string   `json:"small_team_performance"`
	Status               int      `json:"status"`
	ReimbursementAmount  string   `json:"reimbursement_amount"`
	StakingV2OrderId     int64    `json:"staking_v2_order_id,omitempty"`
	StakingV2Amount      string   `json:"staking_v2_amount,omitempty"`
	StakingV2Status      int      `json:"staking_v2_status,omitempty"`
	CanAdjustStake       bool     `json:"can_adjust_stake"`
	AuditBy              int64    `json:"audit_by,omitempty"`
	AuditRemark          string   `json:"audit_remark"`
	AuditAt              string   `json:"audit_at"`
	CreatedAt            string   `json:"created_at"`
	UpdatedAt            string   `json:"updated_at"`
	ApprovalCount        int      `json:"approval_count"`
	CurrentAdminApproved bool     `json:"current_admin_approved"`
}

type ListRes struct {
	frameModel.PageRes
	AcceptedCount int                `json:"accepted_count"`
	AcceptLimit   int                `json:"accept_limit"`
	List          []*ApplicationItem `json:"list"`
}

type DetailRes struct {
	*ApplicationItem
}

type ConfigRes struct {
	Enabled                 bool          `json:"enabled"`
	Activity                *ActivityItem `json:"activity,omitempty"`
	StartAt                 string        `json:"start_at"`
	EndAt                   string        `json:"end_at"`
	Now                     string        `json:"now"`
	SmallTeamPerformance    string        `json:"small_team_performance"`
	MinSmallTeamPerformance string        `json:"min_small_team_performance"`
	MaxAmount               string        `json:"max_amount"`
	AcceptLimit             int           `json:"accept_limit"`
	AcceptedCount           int           `json:"accepted_count"`
	CanApply                bool          `json:"can_apply"`
}

type ActivityItem struct {
	Id                       int64  `json:"id"`
	Title                    string `json:"title"`
	Content                  string `json:"content"`
	IsActive                 bool   `json:"is_active"`
	StartAt                  string `json:"start_at"`
	EndAt                    string `json:"end_at"`
	MinSmallTeamPerformance  string `json:"min_small_team_performance"`
	MaxReimbursementAmount   string `json:"max_reimbursement_amount"`
	AcceptLimit              int    `json:"accept_limit"`
	TotalReimbursementAmount string `json:"total_reimbursement_amount"`
	PendingApplicationCount  int    `json:"pending_application_count"`
	ApprovedApplicationCount int    `json:"approved_application_count"`
	CreatedAt                string `json:"created_at"`
	UpdatedAt                string `json:"updated_at"`
}

type ActivityListRes struct {
	frameModel.PageRes
	List []*ActivityItem `json:"list"`
}
