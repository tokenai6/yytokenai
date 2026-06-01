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
	OfficeApplyPending  = 1
	OfficeApplyApproved = 2
	OfficeApplyRejected = 3
)

type OfficeApplyRepository interface {
	Create(ctx context.Context, tx gdb.TX, req *CreateOfficeApplyReq) (int64, error)
	UpdatePending(ctx context.Context, tx gdb.TX, id int64, req *CreateOfficeApplyReq) error
	ResetRejected(ctx context.Context, tx gdb.TX, id int64, req *CreateOfficeApplyReq) error
	GetByUserForUpdate(ctx context.Context, tx gdb.TX, userID int64) (*OfficeApplyRow, error)
	GetByUser(ctx context.Context, userID int64) (*OfficeApplyRow, error)
	GetLastApprovedByUser(ctx context.Context, userID int64) (*OfficeApplyRow, error)
	GetByID(ctx context.Context, id int64) (*OfficeApplyRow, error)
	GetByIDForUpdate(ctx context.Context, tx gdb.TX, id int64) (*OfficeApplyRow, error)
	List(ctx context.Context, req *OfficeApplyListReq) ([]*OfficeApplyRow, int, error)
	UpdateSubsidyAmountTx(ctx context.Context, tx gdb.TX, id int64, amount decimal.Decimal) error
	AuditApproved(ctx context.Context, tx gdb.TX, id int64, adminID int64, amount decimal.Decimal, remark string) error
	AuditRejected(ctx context.Context, tx gdb.TX, id int64, adminID int64, remark string) error
}

type officeApplyRepository struct{}

func NewOfficeApplyRepository() OfficeApplyRepository {
	return &officeApplyRepository{}
}

type CreateOfficeApplyReq struct {
	UserId               int64
	LeaderName           string
	CommunityAddress     string
	ContactPhone         string
	SmallTeamPerformance decimal.Decimal
	EligibleLevel        int
	SubsidyAmount        decimal.Decimal
	ImageUrls            []string
}

type OfficeApplyRow struct {
	Id                                   int64           `json:"id"`
	UserId                               int64           `json:"user_id"`
	WalletAddress                        string          `json:"wallet_address"`
	LeaderName                           string          `json:"leader_name"`
	CommunityAddress                     string          `json:"community_address"`
	ContactPhone                         string          `json:"contact_phone"`
	SmallTeamPerformance                 decimal.Decimal `json:"small_team_performance"`
	PreviousApprovedSmallTeamPerformance decimal.Decimal `json:"previous_approved_small_team_performance"`
	EligibleLevel                        int             `json:"eligible_level"`
	SubsidyAmount                        decimal.Decimal `json:"subsidy_amount"`
	ImageUrlsText                        string          `json:"image_urls"`
	Status                               int             `json:"status"`
	AuditBy                              int64           `json:"audit_by"`
	AuditRemark                          string          `json:"audit_remark"`
	AuditAt                              *time.Time      `json:"audit_at"`
	CreatedAt                            time.Time       `json:"created_at"`
	UpdatedAt                            time.Time       `json:"updated_at"`
}

type OfficeApplyListReq struct {
	UserId           int64
	WalletAddress    string
	Status           int
	LeaderName       string
	CommunityAddress string
	ContactPhone     string
	Page             int
	PageSize         int
}

func (r *officeApplyRepository) Create(ctx context.Context, tx gdb.TX, req *CreateOfficeApplyReq) (int64, error) {
	data, err := officeApplyData(req)
	if err != nil {
		return 0, err
	}
	data["status"] = OfficeApplyPending
	data["created_at"] = time.Now()
	data["updated_at"] = time.Now()
	return tx.Model("office_application").Ctx(ctx).Data(data).InsertAndGetId()
}

func (r *officeApplyRepository) UpdatePending(ctx context.Context, tx gdb.TX, id int64, req *CreateOfficeApplyReq) error {
	data, err := officeApplyData(req)
	if err != nil {
		return err
	}
	data["updated_at"] = time.Now()
	_, err = tx.Model("office_application").Ctx(ctx).Where("id = ? AND status = ? AND deleted_at IS NULL", id, OfficeApplyPending).Data(data).Update()
	return err
}

func (r *officeApplyRepository) ResetRejected(ctx context.Context, tx gdb.TX, id int64, req *CreateOfficeApplyReq) error {
	data, err := officeApplyData(req)
	if err != nil {
		return err
	}
	data["status"] = OfficeApplyPending
	data["audit_by"] = nil
	data["audit_remark"] = ""
	data["audit_at"] = nil
	data["updated_at"] = time.Now()
	_, err = tx.Model("office_application").Ctx(ctx).Where("id = ? AND status = ? AND deleted_at IS NULL", id, OfficeApplyRejected).Data(data).Update()
	return err
}

func (r *officeApplyRepository) GetByUserForUpdate(ctx context.Context, tx gdb.TX, userID int64) (*OfficeApplyRow, error) {
	var rows []*OfficeApplyRow
	err := tx.Ctx(ctx).Raw(baseOfficeApplySelect()+" WHERE a.user_id = ? AND a.deleted_at IS NULL ORDER BY a.id DESC LIMIT 1 FOR UPDATE OF a", userID).Scan(&rows)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func (r *officeApplyRepository) GetByUser(ctx context.Context, userID int64) (*OfficeApplyRow, error) {
	var rows []*OfficeApplyRow
	err := db.GetDB().Ctx(ctx).Raw(baseOfficeApplySelect()+" WHERE a.user_id = ? AND a.deleted_at IS NULL ORDER BY a.id DESC LIMIT 1", userID).Scan(&rows)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func (r *officeApplyRepository) GetLastApprovedByUser(ctx context.Context, userID int64) (*OfficeApplyRow, error) {
	var rows []*OfficeApplyRow
	err := db.GetDB().Ctx(ctx).Raw(baseOfficeApplySelect()+" WHERE a.user_id = ? AND a.status = ? AND a.deleted_at IS NULL ORDER BY a.id DESC LIMIT 1", userID, OfficeApplyApproved).Scan(&rows)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func (r *officeApplyRepository) GetByID(ctx context.Context, id int64) (*OfficeApplyRow, error) {
	var row OfficeApplyRow
	err := db.GetDB().Ctx(ctx).Raw(baseOfficeApplySelect()+" WHERE a.id = ? AND a.deleted_at IS NULL", id).Scan(&row)
	if err != nil || row.Id == 0 {
		return nil, err
	}
	return &row, nil
}

func (r *officeApplyRepository) GetByIDForUpdate(ctx context.Context, tx gdb.TX, id int64) (*OfficeApplyRow, error) {
	var rows []*OfficeApplyRow
	err := tx.Ctx(ctx).Raw(baseOfficeApplySelect()+" WHERE a.id = ? AND a.deleted_at IS NULL FOR UPDATE OF a", id).Scan(&rows)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func (r *officeApplyRepository) List(ctx context.Context, req *OfficeApplyListReq) ([]*OfficeApplyRow, int, error) {
	where, args := buildOfficeApplyWhere(req)
	var countRow struct {
		Total int `json:"total"`
	}
	err := db.GetDB().Ctx(ctx).Raw("SELECT COUNT(*)::int AS total FROM office_application a LEFT JOIN user_info u ON u.id = a.user_id WHERE "+where, args...).Scan(&countRow)
	if err != nil {
		return nil, 0, err
	}
	listArgs := append([]interface{}{}, args...)
	listArgs = append(listArgs, req.PageSize, (req.Page-1)*req.PageSize)
	var list []*OfficeApplyRow
	err = db.GetDB().Ctx(ctx).Raw(baseOfficeApplySelect()+" WHERE "+where+" ORDER BY a.created_at DESC, a.id DESC LIMIT ? OFFSET ?", listArgs...).Scan(&list)
	return list, countRow.Total, err
}

func (r *officeApplyRepository) UpdateSubsidyAmountTx(ctx context.Context, tx gdb.TX, id int64, amount decimal.Decimal) error {
	_, err := tx.Model("office_application").Ctx(ctx).Where("id = ? AND deleted_at IS NULL", id).Data(g.Map{"subsidy_amount": amount, "updated_at": time.Now()}).Update()
	return err
}

func (r *officeApplyRepository) AuditApproved(ctx context.Context, tx gdb.TX, id int64, adminID int64, amount decimal.Decimal, remark string) error {
	_, err := tx.Model("office_application").Ctx(ctx).Where("id = ? AND deleted_at IS NULL", id).Data(g.Map{"status": OfficeApplyApproved, "subsidy_amount": amount, "audit_by": adminID, "audit_remark": remark, "audit_at": time.Now(), "updated_at": time.Now()}).Update()
	return err
}

func (r *officeApplyRepository) AuditRejected(ctx context.Context, tx gdb.TX, id int64, adminID int64, remark string) error {
	_, err := tx.Model("office_application").Ctx(ctx).Where("id = ? AND deleted_at IS NULL", id).Data(g.Map{"status": OfficeApplyRejected, "audit_by": adminID, "audit_remark": remark, "audit_at": time.Now(), "updated_at": time.Now()}).Update()
	return err
}

func officeApplyData(req *CreateOfficeApplyReq) (g.Map, error) {
	images, err := json.Marshal(req.ImageUrls)
	if err != nil {
		return nil, err
	}
	return g.Map{"user_id": req.UserId, "leader_name": req.LeaderName, "community_address": req.CommunityAddress, "contact_phone": req.ContactPhone, "small_team_performance": req.SmallTeamPerformance, "eligible_level": req.EligibleLevel, "subsidy_amount": req.SubsidyAmount, "image_urls": string(images)}, nil
}

func baseOfficeApplySelect() string {
	return `
		SELECT a.id, a.user_id, COALESCE(u.wallet_address, '') AS wallet_address,
		       a.leader_name, a.community_address, a.contact_phone,
		       a.small_team_performance, a.eligible_level, a.subsidy_amount,
		       COALESCE((
		           SELECT p.small_team_performance
		           FROM office_application p
		           WHERE p.user_id = a.user_id AND p.status = 2 AND p.deleted_at IS NULL AND p.id < a.id
		           ORDER BY p.id DESC
		           LIMIT 1
		       ), 0) AS previous_approved_small_team_performance,
		       a.image_urls::text AS image_urls_text, a.status,
		       COALESCE(a.audit_by, 0) AS audit_by, COALESCE(a.audit_remark, '') AS audit_remark,
		       a.audit_at, a.created_at, a.updated_at
		FROM office_application a
		LEFT JOIN user_info u ON u.id = a.user_id
	`
}

func buildOfficeApplyWhere(req *OfficeApplyListReq) (string, []interface{}) {
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
	if strings.TrimSpace(req.LeaderName) != "" {
		where += " AND a.leader_name ILIKE ?"
		args = append(args, "%"+strings.TrimSpace(req.LeaderName)+"%")
	}
	if strings.TrimSpace(req.CommunityAddress) != "" {
		where += " AND a.community_address ILIKE ?"
		args = append(args, "%"+strings.TrimSpace(req.CommunityAddress)+"%")
	}
	if strings.TrimSpace(req.ContactPhone) != "" {
		where += " AND a.contact_phone ILIKE ?"
		args = append(args, "%"+strings.TrimSpace(req.ContactPhone)+"%")
	}
	return where, args
}
