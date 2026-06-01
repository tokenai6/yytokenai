package model

import frameModel "XWFrame/internal/frame/model"

type ReimbursementImages struct {
	Flight   []string `json:"flight"`
	Train    []string `json:"train"`
	SelfPaid []string `json:"self_paid"`
}

type MeetingMaterialImages struct {
	Venue    []string `json:"venue"`
	CheckIn  []string `json:"check_in"`
	Host     []string `json:"host"`
	Lecture  []string `json:"lecture"`
	Applause []string `json:"applause"`
	QA       []string `json:"qa"`
	Dinner   []string `json:"dinner"`
	Deal     []string `json:"deal"`
	Bill     []string `json:"bill"`
}

type MeetingApplicationItem struct {
	Id                    int64                      `json:"id"`
	UserId                int64                      `json:"user_id"`
	WalletAddress         string                     `json:"wallet_address,omitempty"`
	SmallTeamPerformance  string                     `json:"small_team_performance"`
	Title                 string                     `json:"title"`
	MeetingTime           string                     `json:"meeting_time"`
	Address               string                     `json:"address"`
	ExpectedPeople        int                        `json:"expected_people"`
	SupportLecturer       bool                       `json:"support_lecturer"`
	LecturerCount         int                        `json:"lecturer_count"`
	Community             string                     `json:"community"`
	OrganizerName         string                     `json:"organizer_name"`
	OrganizerPhone        string                     `json:"organizer_phone"`
	Email                 string                     `json:"email"`
	ImageUrls             []string                   `json:"image_urls"`
	ReimbursementImages   *ReimbursementImages       `json:"reimbursement_images"`
	MaterialImages        *MeetingMaterialImages     `json:"material_images"`
	MaterialStatus        int                        `json:"material_status"`
	MaterialAuditBy       int64                      `json:"material_audit_by"`
	MaterialAuditRemark   string                     `json:"material_audit_remark"`
	MaterialAuditAt       string                     `json:"material_audit_at"`
	MaterialSubmittedAt   string                     `json:"material_submitted_at"`
	ApprovalStatus        int                        `json:"approval_status"`
	ReimbursementAmount   string                     `json:"reimbursement_amount"`
	AcceptanceStatus      int                        `json:"acceptance_status"`
	ReimbursementTransfer *ReimbursementTransferItem `json:"reimbursement_transfer,omitempty"`
	AuditBy               int64                      `json:"audit_by"`
	AuditRemark           string                     `json:"audit_remark"`
	AuditAt               string                     `json:"audit_at"`
	CreatedAt             string                     `json:"created_at"`
	UpdatedAt             string                     `json:"updated_at"`
	ApprovalCount         int                        `json:"approval_count"`
	CurrentAdminApproved  bool                       `json:"current_admin_approved"`
	// 前端状态字段（用两个文本状态替代多个 boolean）
	FirstAuditStatus  string `json:"first_audit_status"`  // pending | approved | rejected
	SecondAuditStatus string `json:"second_audit_status"` // not_submitted | pending | approved | rejected
}

type MeetingApplicationListRes struct {
	frameModel.PageRes
	List []*MeetingApplicationItem `json:"list"`
}

type MeetingApplicationDetailRes struct {
	*MeetingApplicationItem
}

type MeetingApplicationConfigRes struct {
	SmallTeamPerformance    string `json:"small_team_performance"`
	MinSmallTeamPerformance string `json:"min_small_team_performance"`
	CanApply                bool   `json:"can_apply"`
}

type ReimbursementTransferItem struct {
	TotalAmount         string `json:"total_amount"`
	FirstAmount         string `json:"first_amount"`
	SecondAmount        string `json:"second_amount"`
	FirstTransferredAt  string `json:"first_transferred_at"`
	SecondTransferredAt string `json:"second_transferred_at"`
	Status              int    `json:"status"`
	FirstOrderNo        string `json:"first_order_no"`
	SecondOrderNo       string `json:"second_order_no"`
}

type UploadImageRes struct {
	Url string `json:"url"`
}
