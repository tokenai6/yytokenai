package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"XWFrame/internal/frame/db"
	meetingModel "XWFrame/internal/service/meeting/model"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

const (
	MeetingApprovalPending  = 1
	MeetingApprovalApproved = 2
	MeetingApprovalRejected = 3

	MeetingMaterialNotSubmitted = 0
	MeetingMaterialPending      = 1
	MeetingMaterialApproved     = 2
	MeetingMaterialRejected     = 3
)

type MeetingRepository interface {
	CreateApplication(ctx context.Context, req *CreateMeetingApplicationReq) (int64, error)
	UpsertApplicationByUser(ctx context.Context, req *CreateMeetingApplicationReq) (int64, error)
	Audit(ctx context.Context, req *AuditMeetingApplicationReq) error
	AuditTx(ctx context.Context, tx gdb.TX, req *AuditMeetingApplicationReq) error
	UpdateReimbursementAmountTx(ctx context.Context, tx gdb.TX, id int64, amount decimal.Decimal) error
	SubmitMaterials(ctx context.Context, userID int64, images *meetingModel.MeetingMaterialImages) (int64, error)
	AuditMaterialsTx(ctx context.Context, tx gdb.TX, req *AuditMeetingApplicationReq) error
	GetById(ctx context.Context, id int64) (*MeetingApplicationRow, error)
	GetByIdForUpdate(ctx context.Context, tx gdb.TX, id int64) (*MeetingApplicationRow, error)
	List(ctx context.Context, req *MeetingApplicationListReq) ([]*MeetingApplicationRow, int, error)
	ListByUser(ctx context.Context, userID int64, page int, pageSize int) ([]*MeetingApplicationRow, int, error)
	CreateReimbursementTransfer(ctx context.Context, tx gdb.TX, req *CreateReimbursementTransferReq) (int64, error)
	GetReimbursementTransferByMeetingID(ctx context.Context, meetingID int64) (*ReimbursementTransferRow, error)
	UpdateReimbursementTransferFirst(ctx context.Context, tx gdb.TX, id int64, orderNo string, transferredAt time.Time) error
	UpdateReimbursementTransferSecond(ctx context.Context, tx gdb.TX, id int64, orderNo string, transferredAt time.Time) error
	UpdateMeetingAcceptanceStatus(ctx context.Context, tx gdb.TX, id int64, status int) error
	Delete(ctx context.Context, id int64) error
}

type meetingRepository struct{}

func NewMeetingRepository() MeetingRepository {
	return &meetingRepository{}
}

type CreateMeetingApplicationReq struct {
	UserId              int64
	Title               string
	MeetingTime         time.Time
	Address             string
	ExpectedPeople      int
	SupportLecturer     bool
	LecturerCount       int
	Community           string
	OrganizerName       string
	OrganizerPhone      string
	Email               string
	ImageUrls           []string
	ReimbursementImages *meetingModel.ReimbursementImages
}

type AuditMeetingApplicationReq struct {
	Id                  int64
	AdminID             int64
	Status              int
	AuditRemark         string
	ReimbursementAmount decimal.Decimal
}

type MeetingApplicationListReq struct {
	UserId    int64
	Title     string
	Status    int
	Community string
	Email     string
	Page      int
	PageSize  int
}

type MeetingApplicationRow struct {
	Id                      int64           `json:"id"`
	UserId                  int64           `json:"user_id"`
	WalletAddress           string          `json:"wallet_address"`
	InviteCode              string          `json:"invite_code"`
	Title                   string          `json:"title"`
	MeetingTime             time.Time       `json:"meeting_time"`
	Address                 string          `json:"address"`
	ExpectedPeople          int             `json:"expected_people"`
	SupportLecturer         bool            `json:"support_lecturer"`
	LecturerCount           int             `json:"lecturer_count"`
	Community               string          `json:"community"`
	OrganizerName           string          `json:"organizer_name"`
	OrganizerPhone          string          `json:"organizer_phone"`
	Email                   string          `json:"email"`
	ImageUrlsText           string          `json:"image_urls"`
	ReimbursementImagesText string          `json:"reimbursement_images"`
	MaterialImagesText      string          `json:"material_images"`
	MaterialStatus          int             `json:"material_status"`
	MaterialAuditBy         int64           `json:"material_audit_by"`
	MaterialAuditRemark     string          `json:"material_audit_remark"`
	MaterialAuditAt         *time.Time      `json:"material_audit_at"`
	MaterialSubmittedAt     *time.Time      `json:"material_submitted_at"`
	ApprovalStatus          int             `json:"approval_status"`
	ReimbursementAmount     decimal.Decimal `json:"reimbursement_amount"`
	AcceptanceStatus        int             `json:"acceptance_status"`
	AuditBy                 int64           `json:"audit_by"`
	AuditRemark             string          `json:"audit_remark"`
	AuditAt                 *time.Time      `json:"audit_at"`
	CreatedAt               time.Time       `json:"created_at"`
	UpdatedAt               time.Time       `json:"updated_at"`
}

func (r *meetingRepository) CreateApplication(ctx context.Context, req *CreateMeetingApplicationReq) (int64, error) {
	imageJSON, err := json.Marshal(req.ImageUrls)
	if err != nil {
		return 0, err
	}
	reimburseJSON, err := json.Marshal(req.ReimbursementImages)
	if err != nil {
		return 0, err
	}
	id, err := db.GetDB().Ctx(ctx).Model("meeting_application").Data(g.Map{
		"user_id":              req.UserId,
		"title":                req.Title,
		"meeting_time":         req.MeetingTime,
		"address":              req.Address,
		"expected_people":      req.ExpectedPeople,
		"support_lecturer":     req.SupportLecturer,
		"lecturer_count":       req.LecturerCount,
		"community":            req.Community,
		"organizer_name":       req.OrganizerName,
		"organizer_phone":      req.OrganizerPhone,
		"email":                req.Email,
		"image_urls":           string(imageJSON),
		"reimbursement_images": string(reimburseJSON),
		"approval_status":      MeetingApprovalPending,
		"created_at":           time.Now(),
		"updated_at":           time.Now(),
	}).InsertAndGetId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *meetingRepository) UpsertApplicationByUser(ctx context.Context, req *CreateMeetingApplicationReq) (int64, error) {
	var row MeetingApplicationRow
	err := db.GetDB().Ctx(ctx).Model("meeting_application").
		Fields("id", "approval_status").
		Where("user_id = ? AND deleted_at IS NULL", req.UserId).
		Order("id DESC").
		Limit(1).
		Scan(&row)
	if err != nil {
		if err == sql.ErrNoRows {
			return r.CreateApplication(ctx, req)
		}
		return 0, err
	}
	if row.Id == 0 || row.ApprovalStatus == MeetingApprovalApproved {
		return r.CreateApplication(ctx, req)
	}

	imageJSON, err := json.Marshal(req.ImageUrls)
	if err != nil {
		return 0, err
	}
	reimburseJSON, err := json.Marshal(req.ReimbursementImages)
	if err != nil {
		return 0, err
	}
	data := g.Map{
		"title":                req.Title,
		"meeting_time":         req.MeetingTime,
		"address":              req.Address,
		"expected_people":      req.ExpectedPeople,
		"support_lecturer":     req.SupportLecturer,
		"lecturer_count":       req.LecturerCount,
		"community":            req.Community,
		"organizer_name":       req.OrganizerName,
		"organizer_phone":      req.OrganizerPhone,
		"email":                req.Email,
		"image_urls":           string(imageJSON),
		"reimbursement_images": string(reimburseJSON),
		"approval_status":      MeetingApprovalPending,
		"audit_by":             0,
		"audit_remark":         "",
		"audit_at":             nil,
		"updated_at":           time.Now(),
	}
	_, err = db.GetDB().Ctx(ctx).Model("meeting_application").Where("id = ? AND deleted_at IS NULL", row.Id).Data(data).Update()
	if err != nil {
		return 0, err
	}
	return row.Id, nil
}

func (r *meetingRepository) Audit(ctx context.Context, req *AuditMeetingApplicationReq) error {
	return r.auditModel(db.GetDB().Model("meeting_application").Ctx(ctx), req)
}

func (r *meetingRepository) AuditTx(ctx context.Context, tx gdb.TX, req *AuditMeetingApplicationReq) error {
	return r.auditModel(tx.Model("meeting_application").Ctx(ctx), req)
}

func (r *meetingRepository) auditModel(m *gdb.Model, req *AuditMeetingApplicationReq) error {
	data := g.Map{
		"approval_status": req.Status,
		"audit_by":        req.AdminID,
		"audit_remark":    req.AuditRemark,
		"audit_at":        time.Now(),
		"updated_at":      time.Now(),
	}
	if req.ReimbursementAmount.GreaterThan(decimal.Zero) {
		data["reimbursement_amount"] = req.ReimbursementAmount
	}
	_, err := m.
		Where("id = ? AND deleted_at IS NULL", req.Id).
		Data(data).
		Update()
	return err
}

func (r *meetingRepository) UpdateReimbursementAmountTx(ctx context.Context, tx gdb.TX, id int64, amount decimal.Decimal) error {
	_, err := tx.Model("meeting_application").Ctx(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		Data(g.Map{
			"reimbursement_amount": amount,
			"updated_at":           time.Now(),
		}).
		Update()
	return err
}

func (r *meetingRepository) SubmitMaterials(ctx context.Context, userID int64, images *meetingModel.MeetingMaterialImages) (int64, error) {
	imageJSON, err := json.Marshal(images)
	if err != nil {
		return 0, err
	}
	var row MeetingApplicationRow
	if err := db.GetDB().Ctx(ctx).Raw(`
		SELECT id
		FROM meeting_application
		WHERE user_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
	`, userID).Scan(&row); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	if row.Id == 0 {
		return 0, nil
	}
	_, err = db.GetDB().Ctx(ctx).Model("meeting_application").Where("id = ? AND deleted_at IS NULL", row.Id).Data(g.Map{
		"material_images":       string(imageJSON),
		"material_status":       MeetingMaterialPending,
		"material_submitted_at": time.Now(),
		"material_audit_by":     0,
		"material_audit_remark": "",
		"material_audit_at":     nil,
		"updated_at":            time.Now(),
	}).Update()
	if err != nil {
		return 0, err
	}
	return row.Id, nil
}

func (r *meetingRepository) AuditMaterialsTx(ctx context.Context, tx gdb.TX, req *AuditMeetingApplicationReq) error {
	_, err := tx.Model("meeting_application").Ctx(ctx).
		Where("id = ? AND deleted_at IS NULL", req.Id).
		Data(g.Map{
			"material_status":       req.Status,
			"material_audit_by":     req.AdminID,
			"material_audit_remark": strings.TrimSpace(req.AuditRemark),
			"material_audit_at":     time.Now(),
			"updated_at":            time.Now(),
		}).Update()
	return err
}

func (r *meetingRepository) GetById(ctx context.Context, id int64) (*MeetingApplicationRow, error) {
	var row MeetingApplicationRow
	err := db.GetDB().Ctx(ctx).Raw(meetingByIDSQL(false), id).Scan(&row)
	if err != nil {
		return nil, err
	}
	if row.Id == 0 {
		return nil, nil
	}
	return &row, nil
}

func (r *meetingRepository) GetByIdForUpdate(ctx context.Context, tx gdb.TX, id int64) (*MeetingApplicationRow, error) {
	var row MeetingApplicationRow
	err := tx.Ctx(ctx).Raw(meetingByIDSQL(true), id).Scan(&row)
	if err != nil {
		return nil, err
	}
	if row.Id == 0 {
		return nil, nil
	}
	return &row, nil
}

func meetingByIDSQL(forUpdate bool) string {
	sql := `
		SELECT a.id, a.user_id, COALESCE(u.wallet_address, '') AS wallet_address, COALESCE(u.invite_code, '') AS invite_code, a.title, a.meeting_time, a.address,
		       a.expected_people, a.support_lecturer, a.lecturer_count, a.community, a.organizer_name, a.organizer_phone,
		       a.email, a.image_urls::text AS image_urls_text, a.reimbursement_images::text AS reimbursement_images_text,
		       COALESCE(a.material_images, '{}'::jsonb)::text AS material_images_text, COALESCE(a.material_status, 0) AS material_status,
		       COALESCE(a.material_audit_by, 0) AS material_audit_by, COALESCE(a.material_audit_remark, '') AS material_audit_remark,
		       a.material_audit_at, a.material_submitted_at,
		       a.approval_status, a.reimbursement_amount, a.acceptance_status, a.audit_by, a.audit_remark, a.audit_at, a.created_at, a.updated_at
		FROM meeting_application a
		LEFT JOIN user_info u ON u.id = a.user_id
		WHERE a.id = ? AND a.deleted_at IS NULL
	`
	if forUpdate {
		sql += " FOR UPDATE OF a"
	}
	return sql
}

func (r *meetingRepository) List(ctx context.Context, req *MeetingApplicationListReq) ([]*MeetingApplicationRow, int, error) {
	where, args := buildMeetingWhere(req)
	var countRow struct {
		Total int `json:"total"`
	}
	if err := db.GetDB().Ctx(ctx).Raw("SELECT COUNT(*)::int AS total FROM meeting_application a WHERE "+where, args...).Scan(&countRow); err != nil {
		return nil, 0, err
	}

	listArgs := append([]interface{}{}, args...)
	listArgs = append(listArgs, req.PageSize, (req.Page-1)*req.PageSize)
	var list []*MeetingApplicationRow
	err := db.GetDB().Ctx(ctx).Raw(`
		SELECT a.id, a.user_id, COALESCE(u.wallet_address, '') AS wallet_address, COALESCE(u.invite_code, '') AS invite_code, a.title, a.meeting_time, a.address,
		       a.expected_people, a.support_lecturer, a.lecturer_count, a.community, a.organizer_name, a.organizer_phone,
		       a.email, a.image_urls::text AS image_urls_text, a.reimbursement_images::text AS reimbursement_images_text,
		       COALESCE(a.material_images, '{}'::jsonb)::text AS material_images_text, COALESCE(a.material_status, 0) AS material_status,
		       COALESCE(a.material_audit_by, 0) AS material_audit_by, COALESCE(a.material_audit_remark, '') AS material_audit_remark,
		       a.material_audit_at, a.material_submitted_at,
		       a.approval_status, a.reimbursement_amount, a.acceptance_status, a.audit_by, a.audit_remark, a.audit_at, a.created_at, a.updated_at
		FROM meeting_application a
		LEFT JOIN user_info u ON u.id = a.user_id
		WHERE `+where+`
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT ? OFFSET ?
	`, listArgs...).Scan(&list)
	return list, countRow.Total, err
}

func (r *meetingRepository) ListByUser(ctx context.Context, userID int64, page int, pageSize int) ([]*MeetingApplicationRow, int, error) {
	return r.List(ctx, &MeetingApplicationListReq{UserId: userID, Page: page, PageSize: pageSize})
}

func buildMeetingWhere(req *MeetingApplicationListReq) (string, []interface{}) {
	where := "a.deleted_at IS NULL"
	args := []interface{}{}
	if req.UserId > 0 {
		where += " AND a.user_id = ?"
		args = append(args, req.UserId)
	}
	if req.Title != "" {
		where += " AND a.title ILIKE ?"
		args = append(args, "%"+req.Title+"%")
	}
	if req.Status > 0 {
		where += " AND a.approval_status = ?"
		args = append(args, req.Status)
	}
	if req.Community != "" {
		where += " AND a.community ILIKE ?"
		args = append(args, "%"+req.Community+"%")
	}
	if req.Email != "" {
		where += " AND a.email ILIKE ?"
		args = append(args, "%"+req.Email+"%")
	}
	return where, args
}

type CreateReimbursementTransferReq struct {
	MeetingApplicationId int64
	UserId               int64
	Symbol               string
	TotalAmount          decimal.Decimal
	FirstAmount          decimal.Decimal
	SecondAmount         decimal.Decimal
	FirstTransferredAt   time.Time
	FirstOrderNo         string
}

type ReimbursementTransferRow struct {
	Id                   int64
	MeetingApplicationId int64
	UserId               int64
	Symbol               string
	TotalAmount          decimal.Decimal
	FirstAmount          decimal.Decimal
	SecondAmount         decimal.Decimal
	FirstTransferredAt   *time.Time
	SecondTransferredAt  *time.Time
	Status               int
	FirstOrderNo         string
	SecondOrderNo        string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (r *meetingRepository) CreateReimbursementTransfer(ctx context.Context, tx gdb.TX, req *CreateReimbursementTransferReq) (int64, error) {
	data := g.Map{
		"meeting_application_id": req.MeetingApplicationId,
		"user_id":                req.UserId,
		"symbol":                 req.Symbol,
		"total_amount":           req.TotalAmount,
		"first_amount":           req.FirstAmount,
		"second_amount":          req.SecondAmount,
		"status":                 1,
		"created_at":             time.Now(),
		"updated_at":             time.Now(),
	}
	if !req.FirstTransferredAt.IsZero() {
		data["first_transferred_at"] = req.FirstTransferredAt
	}
	if req.FirstOrderNo != "" {
		data["first_order_no"] = req.FirstOrderNo
	}
	id, err := tx.Model("meeting_reimbursement_transfer").Ctx(ctx).Data(data).InsertAndGetId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *meetingRepository) GetReimbursementTransferByMeetingID(ctx context.Context, meetingID int64) (*ReimbursementTransferRow, error) {
	var row ReimbursementTransferRow
	err := db.GetDB().Ctx(ctx).Model("meeting_reimbursement_transfer").
		Where("meeting_application_id = ?", meetingID).
		Scan(&row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if row.Id == 0 {
		return nil, nil
	}
	return &row, nil
}

func (r *meetingRepository) UpdateReimbursementTransferFirst(ctx context.Context, tx gdb.TX, id int64, orderNo string, transferredAt time.Time) error {
	_, err := tx.Model("meeting_reimbursement_transfer").Ctx(ctx).
		Where("id = ?", id).
		Data(g.Map{
			"first_transferred_at": transferredAt,
			"first_order_no":       orderNo,
			"updated_at":           time.Now(),
		}).Update()
	return err
}

func (r *meetingRepository) UpdateReimbursementTransferSecond(ctx context.Context, tx gdb.TX, id int64, orderNo string, transferredAt time.Time) error {
	_, err := tx.Model("meeting_reimbursement_transfer").Ctx(ctx).
		Where("id = ?", id).
		Data(g.Map{
			"status":                2,
			"second_transferred_at": transferredAt,
			"second_order_no":       orderNo,
			"updated_at":            time.Now(),
		}).Update()
	return err
}

func (r *meetingRepository) UpdateMeetingAcceptanceStatus(ctx context.Context, tx gdb.TX, id int64, status int) error {
	_, err := tx.Model("meeting_application").Ctx(ctx).
		Where("id = ?", id).
		Data(g.Map{
			"acceptance_status": status,
			"updated_at":        time.Now(),
		}).Update()
	return err
}

func (r *meetingRepository) Delete(ctx context.Context, id int64) error {
	_, err := db.GetDB().Ctx(ctx).Model("meeting_application").
		Where("id = ? AND deleted_at IS NULL", id).
		Data(g.Map{
			"deleted_at": time.Now(),
			"updated_at": time.Now(),
		}).Update()
	return err
}
