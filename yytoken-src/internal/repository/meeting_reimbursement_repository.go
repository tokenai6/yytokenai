package repository

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

const (
	MeetingReimbursementPending  = 1
	MeetingReimbursementApproved = 2
	MeetingReimbursementRejected = 3
)

type MeetingReimbursementRepository interface {
	Create(ctx context.Context, tx gdb.TX, req *CreateMeetingReimbursementReq) (int64, error)
	UpdatePendingByUser(ctx context.Context, tx gdb.TX, userID int64, req *CreateMeetingReimbursementReq) (int64, error)
	ResetRejectedByUser(ctx context.Context, tx gdb.TX, userID int64, req *CreateMeetingReimbursementReq) (int64, error)
	GetLatestByUserForUpdate(ctx context.Context, tx gdb.TX, userID int64) (*MeetingReimbursementRow, error)
	GetByID(ctx context.Context, id int64) (*MeetingReimbursementRow, error)
	GetByIDForUpdate(ctx context.Context, tx gdb.TX, id int64) (*MeetingReimbursementRow, error)
	List(ctx context.Context, req *MeetingReimbursementListReq) ([]*MeetingReimbursementRow, int, error)
	ListByUser(ctx context.Context, userID int64, page int, pageSize int) ([]*MeetingReimbursementRow, int, error)
	CountApproved(ctx context.Context) (int, error)
	CountApprovedByActivity(ctx context.Context, activityID int64) (int, error)
	AuditApproved(ctx context.Context, tx gdb.TX, req *AuditMeetingReimbursementReq) error
	AuditRejected(ctx context.Context, tx gdb.TX, req *AuditMeetingReimbursementReq) error
	UpdateAmount(ctx context.Context, tx gdb.TX, id int64, amount decimal.Decimal, remark string, adminID int64) error
	GetActiveActivity(ctx context.Context) (*MeetingReimbursementActivityRow, error)
	CreateActivity(ctx context.Context, req *SaveMeetingReimbursementActivityReq) (int64, error)
	UpdateActivity(ctx context.Context, req *SaveMeetingReimbursementActivityReq) error
	ActivateActivity(ctx context.Context, id int64) error
	ListActivities(ctx context.Context, page, pageSize int) ([]*MeetingReimbursementActivityRow, int, error)
}

type meetingReimbursementRepository struct{}

func NewMeetingReimbursementRepository() MeetingReimbursementRepository {
	return &meetingReimbursementRepository{}
}

type CreateMeetingReimbursementReq struct {
	ActivityId           int64
	UserId               int64
	Community            string
	Email                string
	ImageUrls            []string
	SmallTeamPerformance decimal.Decimal
}

type AuditMeetingReimbursementReq struct {
	Id                  int64
	AdminID             int64
	Status              int
	AuditRemark         string
	ReimbursementAmount decimal.Decimal
	StakingV2OrderID    int64
}

type MeetingReimbursementListReq struct {
	UserId        int64
	WalletAddress string
	Status        int
	Community     string
	Email         string
	Page          int
	PageSize      int
}

type MeetingReimbursementRow struct {
	Id                   int64           `json:"id"`
	ActivityId           int64           `json:"activity_id"`
	ActivityTitle        string          `json:"activity_title"`
	UserId               int64           `json:"user_id"`
	WalletAddress        string          `json:"wallet_address"`
	Community            string          `json:"community"`
	Email                string          `json:"email"`
	ImageUrlsText        string          `json:"image_urls"`
	SmallTeamPerformance decimal.Decimal `json:"small_team_performance"`
	Status               int             `json:"status"`
	ReimbursementAmount  decimal.Decimal `json:"reimbursement_amount"`
	StakingV2OrderId     int64           `json:"staking_v2_order_id"`
	StakingV2Amount      decimal.Decimal `json:"staking_v2_amount"`
	StakingV2Status      int             `json:"staking_v2_status"`
	AuditBy              int64           `json:"audit_by"`
	AuditRemark          string          `json:"audit_remark"`
	AuditAt              *time.Time      `json:"audit_at"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

type MeetingReimbursementActivityRow struct {
	Id                       int64           `json:"id"`
	Title                    string          `json:"title"`
	Content                  string          `json:"content"`
	IsActive                 bool            `json:"is_active"`
	StartAt                  time.Time       `json:"start_at"`
	EndAt                    time.Time       `json:"end_at"`
	MinSmallTeamPerformance  string `json:"min_small_team_performance"`
	MaxReimbursementAmount   string `json:"max_reimbursement_amount"`
	AcceptLimit              int    `json:"accept_limit"`
	TotalReimbursementAmount string `json:"total_reimbursement_amount"`
	PendingApplicationCount  int             `json:"pending_application_count"`
	ApprovedApplicationCount int             `json:"approved_application_count"`
	CreatedAt                time.Time       `json:"created_at"`
	UpdatedAt                time.Time       `json:"updated_at"`
}

type SaveMeetingReimbursementActivityReq struct {
	Id                      int64
	Title                   string
	Content                 string
	StartAt                 time.Time
	EndAt                   time.Time
	MinSmallTeamPerformance decimal.Decimal
	MaxReimbursementAmount  decimal.Decimal
	AcceptLimit             int
}

func (r *meetingReimbursementRepository) Create(ctx context.Context, tx gdb.TX, req *CreateMeetingReimbursementReq) (int64, error) {
	imageJSON, err := json.Marshal(req.ImageUrls)
	if err != nil {
		return 0, err
	}
	return tx.Model("meeting_reimbursement_application").Ctx(ctx).Data(g.Map{
		"activity_id":            req.ActivityId,
		"user_id":                req.UserId,
		"community":              req.Community,
		"email":                  req.Email,
		"image_urls":             string(imageJSON),
		"small_team_performance": req.SmallTeamPerformance,
		"status":                 MeetingReimbursementPending,
		"created_at":             time.Now(),
		"updated_at":             time.Now(),
	}).InsertAndGetId()
}

func (r *meetingReimbursementRepository) UpdatePendingByUser(ctx context.Context, tx gdb.TX, userID int64, req *CreateMeetingReimbursementReq) (int64, error) {
	imageJSON, err := json.Marshal(req.ImageUrls)
	if err != nil {
		return 0, err
	}
	result, err := tx.Model("meeting_reimbursement_application").Ctx(ctx).
		Where("user_id = ? AND status = ? AND deleted_at IS NULL", userID, MeetingReimbursementPending).
		Data(g.Map{
			"activity_id":            req.ActivityId,
			"community":              req.Community,
			"email":                  req.Email,
			"image_urls":             string(imageJSON),
			"small_team_performance": req.SmallTeamPerformance,
			"updated_at":             time.Now(),
		}).Update()
	if err != nil {
		return 0, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return 0, nil
	}
	row, err := r.GetLatestByUserForUpdate(ctx, tx, userID)
	if err != nil || row == nil {
		return 0, err
	}
	return row.Id, nil
}

func (r *meetingReimbursementRepository) ResetRejectedByUser(ctx context.Context, tx gdb.TX, userID int64, req *CreateMeetingReimbursementReq) (int64, error) {
	imageJSON, err := json.Marshal(req.ImageUrls)
	if err != nil {
		return 0, err
	}
	result, err := tx.Model("meeting_reimbursement_application").Ctx(ctx).
		Where("user_id = ? AND status = ? AND deleted_at IS NULL", userID, MeetingReimbursementRejected).
		Data(g.Map{
			"activity_id":            req.ActivityId,
			"community":              req.Community,
			"email":                  req.Email,
			"image_urls":             string(imageJSON),
			"small_team_performance": req.SmallTeamPerformance,
			"status":                 MeetingReimbursementPending,
			"reimbursement_amount":   decimal.Zero,
			"staking_v2_order_id":    nil,
			"audit_by":               nil,
			"audit_remark":           "",
			"audit_at":               nil,
			"updated_at":             time.Now(),
		}).Update()
	if err != nil {
		return 0, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return 0, nil
	}
	row, err := r.GetLatestByUserForUpdate(ctx, tx, userID)
	if err != nil || row == nil {
		return 0, err
	}
	return row.Id, nil
}

func (r *meetingReimbursementRepository) GetLatestByUserForUpdate(ctx context.Context, tx gdb.TX, userID int64) (*MeetingReimbursementRow, error) {
	var rows []*MeetingReimbursementRow
	err := tx.Ctx(ctx).Raw(baseMeetingReimbursementSelect()+`
		WHERE a.user_id = ? AND a.deleted_at IS NULL
		ORDER BY a.id DESC
		LIMIT 1
		FOR UPDATE OF a
	`, userID).Scan(&rows)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func (r *meetingReimbursementRepository) GetByID(ctx context.Context, id int64) (*MeetingReimbursementRow, error) {
	var row MeetingReimbursementRow
	err := db.GetDB().Ctx(ctx).Raw(baseMeetingReimbursementSelect()+" WHERE a.id = ? AND a.deleted_at IS NULL", id).Scan(&row)
	if err != nil || row.Id == 0 {
		return nil, err
	}
	return &row, nil
}

func (r *meetingReimbursementRepository) GetByIDForUpdate(ctx context.Context, tx gdb.TX, id int64) (*MeetingReimbursementRow, error) {
	var rows []*MeetingReimbursementRow
	err := tx.Ctx(ctx).Raw(baseMeetingReimbursementSelect()+" WHERE a.id = ? AND a.deleted_at IS NULL FOR UPDATE OF a", id).Scan(&rows)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func (r *meetingReimbursementRepository) List(ctx context.Context, req *MeetingReimbursementListReq) ([]*MeetingReimbursementRow, int, error) {
	where, args := buildMeetingReimbursementWhere(req)
	var countRow struct {
		Total int `json:"total"`
	}
	if err := db.GetDB().Ctx(ctx).Raw("SELECT COUNT(*)::int AS total FROM meeting_reimbursement_application a LEFT JOIN user_info u ON u.id = a.user_id WHERE "+where, args...).Scan(&countRow); err != nil {
		return nil, 0, err
	}
	listArgs := append([]interface{}{}, args...)
	listArgs = append(listArgs, req.PageSize, (req.Page-1)*req.PageSize)
	var list []*MeetingReimbursementRow
	err := db.GetDB().Ctx(ctx).Raw(baseMeetingReimbursementSelect()+" WHERE "+where+" ORDER BY a.created_at DESC, a.id DESC LIMIT ? OFFSET ?", listArgs...).Scan(&list)
	return list, countRow.Total, err
}

func (r *meetingReimbursementRepository) ListByUser(ctx context.Context, userID int64, page int, pageSize int) ([]*MeetingReimbursementRow, int, error) {
	return r.List(ctx, &MeetingReimbursementListReq{UserId: userID, Page: page, PageSize: pageSize})
}

func (r *meetingReimbursementRepository) CountApproved(ctx context.Context) (int, error) {
	return db.GetDB().Ctx(ctx).Model("meeting_reimbursement_application").Where("status = ? AND deleted_at IS NULL", MeetingReimbursementApproved).Count()
}

func (r *meetingReimbursementRepository) CountApprovedByActivity(ctx context.Context, activityID int64) (int, error) {
	return db.GetDB().Ctx(ctx).Model("meeting_reimbursement_application").Where("activity_id = ? AND status = ? AND deleted_at IS NULL", activityID, MeetingReimbursementApproved).Count()
}

func (r *meetingReimbursementRepository) AuditApproved(ctx context.Context, tx gdb.TX, req *AuditMeetingReimbursementReq) error {
	_, err := tx.Model("meeting_reimbursement_application").Ctx(ctx).
		Where("id = ? AND deleted_at IS NULL", req.Id).
		Data(g.Map{
			"status":               MeetingReimbursementApproved,
			"reimbursement_amount": req.ReimbursementAmount,
			"staking_v2_order_id":  req.StakingV2OrderID,
			"audit_by":             req.AdminID,
			"audit_remark":         req.AuditRemark,
			"audit_at":             time.Now(),
			"updated_at":           time.Now(),
		}).Update()
	return err
}

func (r *meetingReimbursementRepository) AuditRejected(ctx context.Context, tx gdb.TX, req *AuditMeetingReimbursementReq) error {
	_, err := tx.Model("meeting_reimbursement_application").Ctx(ctx).
		Where("id = ? AND deleted_at IS NULL", req.Id).
		Data(g.Map{"status": MeetingReimbursementRejected, "audit_by": req.AdminID, "audit_remark": req.AuditRemark, "audit_at": time.Now(), "updated_at": time.Now()}).Update()
	return err
}

func (r *meetingReimbursementRepository) UpdateAmount(ctx context.Context, tx gdb.TX, id int64, amount decimal.Decimal, remark string, adminID int64) error {
	_, err := tx.Model("meeting_reimbursement_application").Ctx(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		Data(g.Map{"reimbursement_amount": amount, "audit_remark": strings.TrimSpace(remark), "audit_by": adminID, "updated_at": time.Now()}).Update()
	return err
}

func baseMeetingReimbursementSelect() string {
	return `
		SELECT a.id, COALESCE(a.activity_id, 0) AS activity_id, COALESCE(act.title, '') AS activity_title,
		       a.user_id, COALESCE(u.wallet_address, '') AS wallet_address,
		       a.community, a.email, a.image_urls::text AS image_urls_text,
		       a.small_team_performance, a.status, a.reimbursement_amount,
		       COALESCE(a.staking_v2_order_id, 0) AS staking_v2_order_id,
		       COALESCE(o.amount, 0) AS staking_v2_amount,
		       COALESCE(o.status, 0) AS staking_v2_status,
		       COALESCE(a.audit_by, 0) AS audit_by, COALESCE(a.audit_remark, '') AS audit_remark,
		       a.audit_at, a.created_at, a.updated_at
		FROM meeting_reimbursement_application a
		LEFT JOIN user_info u ON u.id = a.user_id
		LEFT JOIN staking_v2_order o ON o.id = a.staking_v2_order_id
		LEFT JOIN meeting_reimbursement_activity act ON act.id = a.activity_id
	`
}

func (r *meetingReimbursementRepository) GetActiveActivity(ctx context.Context) (*MeetingReimbursementActivityRow, error) {
	var row MeetingReimbursementActivityRow
	err := db.GetDB().Ctx(ctx).Raw(baseActivitySelect() + " WHERE act.is_active = TRUE AND act.deleted_at IS NULL GROUP BY act.id ORDER BY act.id DESC").Scan(&row)
	if err != nil || row.Id == 0 {
		return nil, err
	}
	return &row, nil
}

func (r *meetingReimbursementRepository) CreateActivity(ctx context.Context, req *SaveMeetingReimbursementActivityReq) (int64, error) {
	return db.GetDB().Ctx(ctx).Model("meeting_reimbursement_activity").Data(g.Map{
		"title":                      req.Title,
		"content":                    req.Content,
		"start_at":                   req.StartAt,
		"end_at":                     req.EndAt,
		"min_small_team_performance": req.MinSmallTeamPerformance,
		"max_reimbursement_amount":   req.MaxReimbursementAmount,
		"accept_limit":               req.AcceptLimit,
		"is_active":                  false,
		"created_at":                 time.Now(),
		"updated_at":                 time.Now(),
	}).InsertAndGetId()
}

func (r *meetingReimbursementRepository) UpdateActivity(ctx context.Context, req *SaveMeetingReimbursementActivityReq) error {
	_, err := db.GetDB().Ctx(ctx).Model("meeting_reimbursement_activity").Where("id = ? AND deleted_at IS NULL", req.Id).Data(g.Map{"title": req.Title, "content": req.Content, "start_at": req.StartAt, "end_at": req.EndAt, "min_small_team_performance": req.MinSmallTeamPerformance, "max_reimbursement_amount": req.MaxReimbursementAmount, "accept_limit": req.AcceptLimit, "updated_at": time.Now()}).Update()
	return err
}

func (r *meetingReimbursementRepository) ActivateActivity(ctx context.Context, id int64) error {
	return db.GetDB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Model("meeting_reimbursement_activity").Ctx(ctx).Where("deleted_at IS NULL").Data(g.Map{"is_active": false, "updated_at": time.Now()}).Update(); err != nil {
			return err
		}
		_, err := tx.Model("meeting_reimbursement_activity").Ctx(ctx).Where("id = ? AND deleted_at IS NULL", id).Data(g.Map{"is_active": true, "updated_at": time.Now()}).Update()
		return err
	})
}

func (r *meetingReimbursementRepository) ListActivities(ctx context.Context, page, pageSize int) ([]*MeetingReimbursementActivityRow, int, error) {
	count, err := db.GetDB().Ctx(ctx).Model("meeting_reimbursement_activity").Where("deleted_at IS NULL").Count()
	if err != nil {
		return nil, 0, err
	}
	var list []*MeetingReimbursementActivityRow
	err = db.GetDB().Ctx(ctx).Raw(baseActivitySelect()+" WHERE act.deleted_at IS NULL GROUP BY act.id ORDER BY act.is_active DESC, act.id DESC LIMIT ? OFFSET ?", pageSize, (page-1)*pageSize).Scan(&list)
	return list, count, err
}

func baseActivitySelect() string {
	return `
		SELECT act.id, act.title, act.content, act.is_active, act.start_at, act.end_at,
		       act.min_small_team_performance, act.max_reimbursement_amount, act.accept_limit,
		       COALESCE(SUM(CASE WHEN app.status = 2 THEN app.reimbursement_amount ELSE 0 END), 0) AS total_reimbursement_amount,
		       COUNT(app.id) FILTER (WHERE app.status = 1) AS pending_application_count,
		       COUNT(app.id) FILTER (WHERE app.status = 2) AS approved_application_count,
		       act.created_at, act.updated_at
		FROM meeting_reimbursement_activity act
		LEFT JOIN meeting_reimbursement_application app ON app.activity_id = act.id AND app.deleted_at IS NULL
	`
}

func buildMeetingReimbursementWhere(req *MeetingReimbursementListReq) (string, []interface{}) {
	where := "a.deleted_at IS NULL"
	args := []interface{}{}
	if req.UserId > 0 {
		where += " AND a.user_id = ?"
		args = append(args, req.UserId)
	}
	if strings.TrimSpace(req.WalletAddress) != "" {
		where += " AND u.wallet_address = ?"
		args = append(args, strings.ToLower(strings.TrimSpace(req.WalletAddress)))
	}
	if req.Status > 0 {
		where += " AND a.status = ?"
		args = append(args, req.Status)
	}
	if strings.TrimSpace(req.Community) != "" {
		where += " AND a.community ILIKE ?"
		args = append(args, "%"+strings.TrimSpace(req.Community)+"%")
	}
	if strings.TrimSpace(req.Email) != "" {
		where += " AND a.email ILIKE ?"
		args = append(args, "%"+strings.TrimSpace(req.Email)+"%")
	}
	return where, args
}
