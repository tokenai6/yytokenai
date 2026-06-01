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
	TableSharePending  = 1
	TableShareApproved = 2
	TableShareRejected = 3
)

type TableShareRepository interface {
	Create(ctx context.Context, tx gdb.TX, req *CreateTableShareReq) (int64, error)
	UpdateTodayPending(ctx context.Context, tx gdb.TX, userID int64, req *CreateTableShareReq) (int64, error)
	ResetTodayRejected(ctx context.Context, tx gdb.TX, userID int64, req *CreateTableShareReq) (int64, error)
	GetTodayByUserForUpdate(ctx context.Context, tx gdb.TX, userID int64) (*TableShareRow, error)
	GetLastApprovedByUser(ctx context.Context, userID int64) (*TableShareRow, error)
	GetByID(ctx context.Context, id int64) (*TableShareRow, error)
	GetByIDForUpdate(ctx context.Context, tx gdb.TX, id int64) (*TableShareRow, error)
	List(ctx context.Context, req *TableShareListReq) ([]*TableShareRow, int, error)
	ListByUser(ctx context.Context, userID int64, page int, pageSize int) ([]*TableShareRow, int, error)
	CountTodayByUser(ctx context.Context, userID int64) (int, error)
	UpdateShareAmountTx(ctx context.Context, tx gdb.TX, id int64, amount decimal.Decimal) error
	AuditApproved(ctx context.Context, tx gdb.TX, id int64, adminID int64, amount decimal.Decimal, remark string) error
	AuditRejected(ctx context.Context, tx gdb.TX, id int64, adminID int64, remark string) error
}

type tableShareRepository struct{}

func NewTableShareRepository() TableShareRepository {
	return &tableShareRepository{}
}

type CreateTableShareReq struct {
	UserId                 int64
	Community              string
	DiningPeopleCount      int
	SmallTeamPerformance   decimal.Decimal
	CommunicationVideoUrls []string
	SharingVideoUrls       []string
	SloganVideoUrls        []string
}

type TableShareRow struct {
	Id                                   int64           `json:"id"`
	UserId                               int64           `json:"user_id"`
	WalletAddress                        string          `json:"wallet_address"`
	Community                            string          `json:"community"`
	DiningPeopleCount                    int             `json:"dining_people_count"`
	SmallTeamPerformance                 decimal.Decimal `json:"small_team_performance"`
	PreviousApprovedSmallTeamPerformance decimal.Decimal `json:"previous_approved_small_team_performance"`
	CommunicationVideoUrlsText           string          `json:"communication_video_urls"`
	SharingVideoUrlsText                 string          `json:"sharing_video_urls"`
	SloganVideoUrlsText                  string          `json:"slogan_video_urls"`
	Status                               int             `json:"status"`
	ShareAmount                          decimal.Decimal `json:"share_amount"`
	AuditBy                              int64           `json:"audit_by"`
	AuditRemark                          string          `json:"audit_remark"`
	AuditAt                              *time.Time      `json:"audit_at"`
	CreatedAt                            time.Time       `json:"created_at"`
	UpdatedAt                            time.Time       `json:"updated_at"`
}

type TableShareListReq struct {
	UserId        int64
	WalletAddress string
	Status        int
	Community     string
	Page          int
	PageSize      int
}

func (r *tableShareRepository) Create(ctx context.Context, tx gdb.TX, req *CreateTableShareReq) (int64, error) {
	data, err := tableShareData(req)
	if err != nil {
		return 0, err
	}
	data["status"] = TableSharePending
	data["created_at"] = time.Now()
	data["updated_at"] = time.Now()
	return tx.Model("table_share_application").Ctx(ctx).Data(data).InsertAndGetId()
}

func (r *tableShareRepository) UpdateTodayPending(ctx context.Context, tx gdb.TX, userID int64, req *CreateTableShareReq) (int64, error) {
	data, err := tableShareData(req)
	if err != nil {
		return 0, err
	}
	data["updated_at"] = time.Now()
	result, err := tx.Model("table_share_application").Ctx(ctx).
		Where("user_id = ? AND created_date = CURRENT_DATE AND status = ? AND deleted_at IS NULL", userID, TableSharePending).
		Data(data).Update()
	if err != nil {
		return 0, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return 0, nil
	}
	row, err := r.GetTodayByUserForUpdate(ctx, tx, userID)
	if err != nil || row == nil {
		return 0, err
	}
	return row.Id, nil
}

func (r *tableShareRepository) ResetTodayRejected(ctx context.Context, tx gdb.TX, userID int64, req *CreateTableShareReq) (int64, error) {
	data, err := tableShareData(req)
	if err != nil {
		return 0, err
	}
	data["status"] = TableSharePending
	data["share_amount"] = decimal.Zero
	data["audit_by"] = nil
	data["audit_remark"] = ""
	data["audit_at"] = nil
	data["updated_at"] = time.Now()
	result, err := tx.Model("table_share_application").Ctx(ctx).
		Where("user_id = ? AND created_date = CURRENT_DATE AND status = ? AND deleted_at IS NULL", userID, TableShareRejected).
		Data(data).Update()
	if err != nil {
		return 0, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return 0, nil
	}
	row, err := r.GetTodayByUserForUpdate(ctx, tx, userID)
	if err != nil || row == nil {
		return 0, err
	}
	return row.Id, nil
}

func (r *tableShareRepository) GetTodayByUserForUpdate(ctx context.Context, tx gdb.TX, userID int64) (*TableShareRow, error) {
	var rows []*TableShareRow
	err := tx.Ctx(ctx).Raw(baseTableShareSelect()+`
		WHERE a.user_id = ? AND a.created_date = CURRENT_DATE AND a.deleted_at IS NULL
		ORDER BY a.id DESC
		LIMIT 1
		FOR UPDATE OF a
	`, userID).Scan(&rows)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func (r *tableShareRepository) GetLastApprovedByUser(ctx context.Context, userID int64) (*TableShareRow, error) {
	var rows []*TableShareRow
	err := db.GetDB().Ctx(ctx).Raw(baseTableShareSelect()+`
		WHERE a.user_id = ? AND a.status = ? AND a.deleted_at IS NULL
		ORDER BY a.id DESC
		LIMIT 1
	`, userID, TableShareApproved).Scan(&rows)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func (r *tableShareRepository) GetByID(ctx context.Context, id int64) (*TableShareRow, error) {
	var row TableShareRow
	err := db.GetDB().Ctx(ctx).Raw(baseTableShareSelect()+" WHERE a.id = ? AND a.deleted_at IS NULL", id).Scan(&row)
	if err != nil || row.Id == 0 {
		return nil, err
	}
	return &row, nil
}

func (r *tableShareRepository) GetByIDForUpdate(ctx context.Context, tx gdb.TX, id int64) (*TableShareRow, error) {
	var rows []*TableShareRow
	err := tx.Ctx(ctx).Raw(baseTableShareSelect()+" WHERE a.id = ? AND a.deleted_at IS NULL FOR UPDATE OF a", id).Scan(&rows)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func (r *tableShareRepository) List(ctx context.Context, req *TableShareListReq) ([]*TableShareRow, int, error) {
	where, args := buildTableShareWhere(req)
	var countRow struct {
		Total int `json:"total"`
	}
	err := db.GetDB().Ctx(ctx).Raw("SELECT COUNT(*)::int AS total FROM table_share_application a LEFT JOIN user_info u ON u.id = a.user_id WHERE "+where, args...).Scan(&countRow)
	if err != nil {
		return nil, 0, err
	}
	listArgs := append([]interface{}{}, args...)
	listArgs = append(listArgs, req.PageSize, (req.Page-1)*req.PageSize)
	var list []*TableShareRow
	err = db.GetDB().Ctx(ctx).Raw(baseTableShareSelect()+" WHERE "+where+" ORDER BY a.created_at DESC, a.id DESC LIMIT ? OFFSET ?", listArgs...).Scan(&list)
	return list, countRow.Total, err
}

func (r *tableShareRepository) ListByUser(ctx context.Context, userID int64, page int, pageSize int) ([]*TableShareRow, int, error) {
	return r.List(ctx, &TableShareListReq{UserId: userID, Page: page, PageSize: pageSize})
}

func (r *tableShareRepository) CountTodayByUser(ctx context.Context, userID int64) (int, error) {
	return db.GetDB().Ctx(ctx).Model("table_share_application").Where("user_id = ? AND created_date = CURRENT_DATE AND deleted_at IS NULL", userID).Count()
}

func (r *tableShareRepository) UpdateShareAmountTx(ctx context.Context, tx gdb.TX, id int64, amount decimal.Decimal) error {
	_, err := tx.Model("table_share_application").Ctx(ctx).Where("id = ? AND deleted_at IS NULL", id).Data(g.Map{"share_amount": amount, "updated_at": time.Now()}).Update()
	return err
}

func (r *tableShareRepository) AuditApproved(ctx context.Context, tx gdb.TX, id int64, adminID int64, amount decimal.Decimal, remark string) error {
	_, err := tx.Model("table_share_application").Ctx(ctx).Where("id = ? AND deleted_at IS NULL", id).Data(g.Map{"status": TableShareApproved, "share_amount": amount, "audit_by": adminID, "audit_remark": remark, "audit_at": time.Now(), "updated_at": time.Now()}).Update()
	return err
}

func (r *tableShareRepository) AuditRejected(ctx context.Context, tx gdb.TX, id int64, adminID int64, remark string) error {
	_, err := tx.Model("table_share_application").Ctx(ctx).Where("id = ? AND deleted_at IS NULL", id).Data(g.Map{"status": TableShareRejected, "audit_by": adminID, "audit_remark": remark, "audit_at": time.Now(), "updated_at": time.Now()}).Update()
	return err
}

func tableShareData(req *CreateTableShareReq) (g.Map, error) {
	communication, err := json.Marshal(req.CommunicationVideoUrls)
	if err != nil {
		return nil, err
	}
	sharing, err := json.Marshal(req.SharingVideoUrls)
	if err != nil {
		return nil, err
	}
	slogan, err := json.Marshal(req.SloganVideoUrls)
	if err != nil {
		return nil, err
	}
	return g.Map{
		"user_id":                  req.UserId,
		"community":                req.Community,
		"dining_people_count":      req.DiningPeopleCount,
		"small_team_performance":   req.SmallTeamPerformance,
		"communication_video_urls": string(communication),
		"sharing_video_urls":       string(sharing),
		"slogan_video_urls":        string(slogan),
	}, nil
}

func baseTableShareSelect() string {
	return `
		SELECT a.id, a.user_id, COALESCE(u.wallet_address, '') AS wallet_address,
		       a.community, a.dining_people_count, a.small_team_performance,
		       COALESCE((
		           SELECT p.small_team_performance
		           FROM table_share_application p
		           WHERE p.user_id = a.user_id AND p.status = 2 AND p.deleted_at IS NULL AND p.id < a.id
		           ORDER BY p.id DESC
		           LIMIT 1
		       ), 0) AS previous_approved_small_team_performance,
		       a.communication_video_urls::text AS communication_video_urls_text,
		       a.sharing_video_urls::text AS sharing_video_urls_text,
		       a.slogan_video_urls::text AS slogan_video_urls_text,
		       a.status, a.share_amount, COALESCE(a.audit_by, 0) AS audit_by,
		       COALESCE(a.audit_remark, '') AS audit_remark, a.audit_at,
		       a.created_at, a.updated_at
		FROM table_share_application a
		LEFT JOIN user_info u ON u.id = a.user_id
	`
}

func buildTableShareWhere(req *TableShareListReq) (string, []interface{}) {
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
	return where, args
}
