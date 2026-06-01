package tableshare

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"XWFrame/internal/frame/consts"
	frameModel "XWFrame/internal/frame/model"
	"XWFrame/internal/repository"
	"XWFrame/internal/service/adminapproval"
	coboSvc "XWFrame/internal/service/cobo"
	m "XWFrame/internal/service/tableshare/model"
	"XWFrame/internal/service/teamstats"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

const (
	configEnabled                 = "table_share_enabled"
	configMinSmallTeamPerformance = "table_share_min_small_team_performance"
	configMaxAmount               = "table_share_max_amount"
	configDailyLimit              = "table_share_daily_limit"
)

var errMsgs = map[string]map[string]string{
	"not_found":                      {"zh-CN": "饭桌分享会申请不存在", "zh-TW": "飯桌分享會申請不存在", "en-US": "Table share application not found", "ja-JP": "テーブルシェア申請が見つかりません", "ko-KR": "테이블 공유 신청을 찾을 수 없습니다", "vi-VN": "Không tìm thấy đơn chia sẻ bàn ăn", "th-TH": "ไม่พบคำขอ table share"},
	"disabled":                       {"zh-CN": "饭桌分享会功能未开启", "zh-TW": "飯桌分享會功能未開啟", "en-US": "Table share is disabled", "ja-JP": "テーブルシェアは無効です", "ko-KR": "테이블 공유 기능이 비활성화되었습니다", "vi-VN": "Chức năng chia sẻ bàn ăn chưa bật", "th-TH": "ยังไม่ได้เปิดใช้งาน table share"},
	"dining_people_count_invalid":    {"zh-CN": "用餐人数不合法", "zh-TW": "用餐人數不合法", "en-US": "Invalid dining people count", "ja-JP": "食事人数が正しくありません", "ko-KR": "식사 인원이 유효하지 않습니다", "vi-VN": "Số người dùng bữa không hợp lệ", "th-TH": "จำนวนผู้รับประทานอาหารไม่ถูกต้อง"},
	"videos_required":                {"zh-CN": "请上传交流、分享、口号视频", "zh-TW": "請上傳交流、分享、口號影片", "en-US": "Please upload communication, sharing and slogan videos", "ja-JP": "交流、共有、スローガン動画をアップロードしてください", "ko-KR": "교류, 공유, 구호 영상을 업로드해 주세요", "vi-VN": "Vui lòng tải lên video giao lưu, chia sẻ và khẩu hiệu", "th-TH": "กรุณาอัปโหลดวิดีโอสื่อสาร แชร์ และสโลแกน"},
	"small_team_performance_not_met": {"zh-CN": "小区业绩未达到饭桌分享会门槛", "zh-TW": "小區業績未達到飯桌分享會門檻", "en-US": "Small team performance below table share threshold", "ja-JP": "小区業績が条件を満たしていません", "ko-KR": "소구역 실적이 기준에 미달합니다", "vi-VN": "Hiệu suất nhóm nhỏ chưa đạt ngưỡng", "th-TH": "ผลงานทีมย่อยไม่ถึงเกณฑ์"},
	"performance_increment_not_met":  {"zh-CN": "千城计划每次申请需较上次审批时小区业绩新增至少5000U", "zh-TW": "千城計劃每次申請需較上次審批時小區業績新增至少5000U", "en-US": "Each Thousand Cities application requires at least 5000U new small team performance since the last approval", "ja-JP": "千城計画の申請ごとに前回承認時より小区実績が少なくとも5000U増加している必要があります", "ko-KR": "천성 계획은 매 신청마다 이전 승인 시점보다 소구역 실적이 최소 5000U 증가해야 합니다", "vi-VN": "Mỗi đơn Thousand Cities cần tăng ít nhất 5000U hiệu suất nhóm nhỏ so với lần phê duyệt trước", "th-TH": "แผน Thousand Cities ต้องมีผลงานทีมย่อยเพิ่มขึ้นอย่างน้อย 5000U จากครั้งที่อนุมัติก่อนหน้า"},
	"approved_cannot_update":         {"zh-CN": "已通过申请禁止更新", "zh-TW": "已通過申請禁止更新", "en-US": "Approved application cannot be updated", "ja-JP": "承認済み申請は更新できません", "ko-KR": "승인된 신청은 수정할 수 없습니다", "vi-VN": "Không thể cập nhật đơn đã duyệt", "th-TH": "ไม่สามารถแก้ไขคำขอที่อนุมัติแล้ว"},
	"daily_limit_reached":            {"zh-CN": "今日申请次数已达上限", "zh-TW": "今日申請次數已達上限", "en-US": "Daily application limit reached", "ja-JP": "本日の申請上限に達しました", "ko-KR": "오늘 신청 한도에 도달했습니다", "vi-VN": "Đã đạt giới hạn đăng ký hôm nay", "th-TH": "ถึงขีดจำกัดคำขอรายวันแล้ว"},
	"invalid_status":                 {"zh-CN": "审批状态不合法", "zh-TW": "審批狀態不合法", "en-US": "Invalid approval status", "ja-JP": "承認状態が正しくありません", "ko-KR": "승인 상태가 유효하지 않습니다", "vi-VN": "Trạng thái phê duyệt không hợp lệ", "th-TH": "สถานะอนุมัติไม่ถูกต้อง"},
	"already_audited":                {"zh-CN": "申请已审批", "zh-TW": "申請已審批", "en-US": "Application already audited", "ja-JP": "申請は審査済みです", "ko-KR": "신청이 이미 심사되었습니다", "vi-VN": "Đơn đã được phê duyệt", "th-TH": "คำขอได้รับการตรวจสอบแล้ว"},
	"amount_invalid":                 {"zh-CN": "分享奖励额度不合法", "zh-TW": "分享獎勵額度不合法", "en-US": "Invalid share reward amount", "ja-JP": "報酬額が正しくありません", "ko-KR": "보상 금액이 유효하지 않습니다", "vi-VN": "Số tiền thưởng không hợp lệ", "th-TH": "จำนวนรางวัลไม่ถูกต้อง"},
	"amount_exceeds_limit":           {"zh-CN": "分享奖励额度超过上限", "zh-TW": "分享獎勵額度超過上限", "en-US": "Share reward amount exceeds limit", "ja-JP": "報酬額が上限を超えています", "ko-KR": "보상 금액이 한도를 초과했습니다", "vi-VN": "Số tiền thưởng vượt quá giới hạn", "th-TH": "จำนวนรางวัลเกินวงเงิน"},
	"already_approved_by_admin":      {"zh-CN": "您已审批过，不能重复操作", "zh-TW": "您已審批過，不能重複操作", "en-US": "You have already approved this application", "ja-JP": "この申請は既に承認済みです", "ko-KR": "이미 승인한 신청입니다", "vi-VN": "Bạn đã phê duyệt đơn này", "th-TH": "คุณได้อนุมัติคำขอนี้แล้ว"},
}

type Service interface {
	Config(ctx context.Context, userID int64) (*m.ConfigRes, error)
	AdminConfig(ctx context.Context) (*m.AdminConfigRes, error)
	UpdateConfig(ctx context.Context, req *UpdateConfigReq) error
	Apply(ctx context.Context, req *ApplyReq) (int64, error)
	ListMy(ctx context.Context, userID int64, page, pageSize int) (*m.ListRes, error)
	List(ctx context.Context, req *ListReq) (*m.ListRes, error)
	Get(ctx context.Context, id int64, adminID int64) (*m.DetailRes, error)
	Audit(ctx context.Context, req *AuditReq) error
}

type service struct {
	repo           repository.TableShareRepository
	directTransfer coboSvc.DirectTransferService
}

func NewService() Service {
	return &service{
		repo:           repository.NewTableShareRepository(),
		directTransfer: coboSvc.NewDirectTransferService(),
	}
}

type ApplyReq struct {
	UserId                 int64
	Community              string
	DiningPeopleCount      int
	CommunicationVideoUrls []string
	SharingVideoUrls       []string
	SloganVideoUrls        []string
}

type ListReq struct {
	AdminID        int64
	UserId         int64
	WalletAddress  string
	Status         int
	Community      string
	Page, PageSize int
}

type AuditReq struct {
	Id          int64
	AdminID     int64
	Status      int
	AuditRemark string
	ShareAmount decimal.Decimal
}

type UpdateConfigReq struct {
	MinSmallTeamPerformance decimal.Decimal
	MaxAmount               decimal.Decimal
	DailyLimit              int
}

func (s *service) Config(ctx context.Context, userID int64) (*m.ConfigRes, error) {
	small, err := s.smallTeamPerformance(ctx, userID)
	if err != nil {
		return nil, err
	}
	minPerf := readDecimalConfig(ctx, configMinSmallTeamPerformance, "5000")
	dailyLimit := readIntConfig(ctx, configDailyLimit, 1)
	todayCount, _ := s.repo.CountTodayByUser(ctx, userID)
	enabled := readStringConfig(ctx, configEnabled, "true") != "false"
	return &m.ConfigRes{Enabled: enabled, SmallTeamPerformance: utils.FormatDecimal(small), MinSmallTeamPerformance: utils.FormatDecimal(minPerf), MaxAmount: utils.FormatDecimal(readDecimalConfig(ctx, configMaxAmount, "10000")), DailyLimit: dailyLimit, TodayAppliedCount: todayCount, CanApply: enabled && !small.LessThan(minPerf) && todayCount < dailyLimit}, nil
}

func (s *service) AdminConfig(ctx context.Context) (*m.AdminConfigRes, error) {
	return &m.AdminConfigRes{Enabled: readStringConfig(ctx, configEnabled, "true") != "false", MinSmallTeamPerformance: utils.FormatDecimal(readDecimalConfig(ctx, configMinSmallTeamPerformance, "5000")), MaxAmount: utils.FormatDecimal(readDecimalConfig(ctx, configMaxAmount, "10000")), DailyLimit: readIntConfig(ctx, configDailyLimit, 1)}, nil
}

func (s *service) UpdateConfig(ctx context.Context, req *UpdateConfigReq) error {
	if req.MinSmallTeamPerformance.IsNegative() || !req.MaxAmount.GreaterThan(decimal.Zero) || req.DailyLimit <= 0 {
		return codeErr(ctx, 400, "amount_invalid")
	}
	configs := map[string]string{
		configMinSmallTeamPerformance: req.MinSmallTeamPerformance.String(),
		configMaxAmount:               req.MaxAmount.String(),
		configDailyLimit:              fmt.Sprintf("%d", req.DailyLimit),
	}
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		for key, value := range configs {
			if _, err := tx.Exec("UPDATE system_config SET value = ?, updated_at = NOW() WHERE key = ?", value, key); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *service) Apply(ctx context.Context, req *ApplyReq) (int64, error) {
	if readStringConfig(ctx, configEnabled, "true") == "false" {
		return 0, codeErr(ctx, 400, "disabled")
	}
	if req.DiningPeopleCount <= 0 {
		return 0, codeErr(ctx, 400, "dining_people_count_invalid")
	}
	if len(req.CommunicationVideoUrls) == 0 || len(req.SharingVideoUrls) == 0 || len(req.SloganVideoUrls) == 0 {
		return 0, codeErr(ctx, 400, "videos_required")
	}
	small, err := s.smallTeamPerformance(ctx, req.UserId)
	if err != nil {
		return 0, err
	}
	if small.LessThan(readDecimalConfig(ctx, configMinSmallTeamPerformance, "5000")) {
		return 0, codeErr(ctx, 400, "small_team_performance_not_met")
	}
	lastApproved, err := s.repo.GetLastApprovedByUser(ctx, req.UserId)
	if err != nil {
		return 0, err
	}
	if lastApproved != nil && small.Sub(lastApproved.SmallTeamPerformance).LessThan(decimal.NewFromInt(5000)) {
		return 0, codeErr(ctx, 400, "performance_increment_not_met")
	}
	create := &repository.CreateTableShareReq{UserId: req.UserId, Community: strings.TrimSpace(req.Community), DiningPeopleCount: req.DiningPeopleCount, SmallTeamPerformance: small, CommunicationVideoUrls: req.CommunicationVideoUrls, SharingVideoUrls: req.SharingVideoUrls, SloganVideoUrls: req.SloganVideoUrls}
	var id int64
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		current, err := s.repo.GetTodayByUserForUpdate(ctx, tx, req.UserId)
		if err != nil {
			return err
		}
		if current != nil {
			if current.Status == repository.TableShareApproved {
				return codeErr(ctx, 400, "approved_cannot_update")
			}
			if current.Status == repository.TableSharePending {
				id, err = s.repo.UpdateTodayPending(ctx, tx, req.UserId, create)
				return err
			}
			if current.Status == repository.TableShareRejected {
				id, err = s.repo.ResetTodayRejected(ctx, tx, req.UserId, create)
				return err
			}
		}
		count, err := s.repo.CountTodayByUser(ctx, req.UserId)
		if err != nil {
			return err
		}
		if count >= readIntConfig(ctx, configDailyLimit, 1) {
			return codeErr(ctx, 400, "daily_limit_reached")
		}
		id, err = s.repo.Create(ctx, tx, create)
		return err
	})
	return id, err
}

func (s *service) ListMy(ctx context.Context, userID int64, page, pageSize int) (*m.ListRes, error) {
	return s.List(ctx, &ListReq{UserId: userID, Page: page, PageSize: pageSize})
}

func (s *service) List(ctx context.Context, req *ListReq) (*m.ListRes, error) {
	page, pageSize := req.Page, req.PageSize
	normalizePage(&page, &pageSize)
	rows, total, err := s.repo.List(ctx, &repository.TableShareListReq{UserId: req.UserId, WalletAddress: req.WalletAddress, Status: req.Status, Community: req.Community, Page: page, PageSize: pageSize})
	if err != nil {
		return nil, err
	}
	list := make([]*m.ApplicationItem, 0, len(rows))
	for _, row := range rows {
		item := convert(row)
		if err := fillTableApprovalInfo(ctx, item, req.AdminID); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return &m.ListRes{PageRes: frameModel.PageRes{Page: page, PageSize: pageSize, Total: total, Pages: (total + pageSize - 1) / pageSize}, List: list}, nil
}

func (s *service) Get(ctx context.Context, id int64, adminID int64) (*m.DetailRes, error) {
	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, codeErr(ctx, 404, "not_found")
	}
	item := convert(row)
	if err := fillTableApprovalInfo(ctx, item, adminID); err != nil {
		return nil, err
	}
	return &m.DetailRes{ApplicationItem: item}, nil
}

func (s *service) Audit(ctx context.Context, req *AuditReq) error {
	if req.Status != repository.TableShareApproved && req.Status != repository.TableShareRejected {
		return codeErr(ctx, 400, "invalid_status")
	}

	current, err := s.repo.GetByID(ctx, req.Id)
	if err != nil {
		return err
	}
	if current == nil {
		return codeErr(ctx, 404, "not_found")
	}

	approvalSvc := adminapproval.NewService()
	bizType := adminapproval.BizTypeTableShare.String()

	// 拒绝操作
	if req.Status == repository.TableShareRejected {
		return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
			currentTx, err := s.repo.GetByIDForUpdate(ctx, tx, req.Id)
			if err != nil {
				return err
			}
			if currentTx == nil {
				return codeErr(ctx, 404, "not_found")
			}
			if currentTx.Status != repository.TableSharePending {
				return codeErr(ctx, 400, "already_audited")
			}
			if err := s.repo.AuditRejected(ctx, tx, req.Id, req.AdminID, strings.TrimSpace(req.AuditRemark)); err != nil {
				return err
			}
			return approvalSvc.RecordApprovalTx(ctx, tx, bizType, req.Id, req.AdminID, adminapproval.ActionReject, strings.TrimSpace(req.AuditRemark))
		})
	}

	// 通过操作
	if current.Status == repository.TableShareRejected {
		return codeErr(ctx, 400, "already_audited")
	}

	shouldSubmitTransfer := false
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		currentTx, err := s.repo.GetByIDForUpdate(ctx, tx, req.Id)
		if err != nil {
			return err
		}
		if currentTx == nil {
			return codeErr(ctx, 404, "not_found")
		}

		// 检查当前管理员是否已审批
		hasApproved, err := approvalSvc.HasAdminApprovedTx(ctx, tx, bizType, req.Id, req.AdminID)
		if err != nil {
			return err
		}
		if hasApproved {
			return codeErr(ctx, 400, "already_approved_by_admin")
		}

		// 查询已有 approve 记录数
		approveCount, err := approvalSvc.GetApproveCountTx(ctx, tx, bizType, req.Id)
		if err != nil {
			return err
		}

		if !req.ShareAmount.GreaterThan(decimal.Zero) {
			return codeErr(ctx, 400, "amount_invalid")
		}
		if req.ShareAmount.GreaterThan(readDecimalConfig(ctx, configMaxAmount, "10000")) {
			return codeErr(ctx, 400, "amount_exceeds_limit")
		}

		// 记录审批
		if currentTx.Status == repository.TableSharePending {
			if err := s.repo.UpdateShareAmountTx(ctx, tx, req.Id, req.ShareAmount); err != nil {
				return err
			}
		}
		if err := approvalSvc.RecordApprovalTx(ctx, tx, bizType, req.Id, req.AdminID, adminapproval.ActionApprove, strings.TrimSpace(req.AuditRemark)); err != nil {
			return err
		}

		// 如果已有其他管理员 approve，触发转账
		if approveCount >= 1 {
			if currentTx.Status == repository.TableSharePending {
				if err := s.repo.AuditApproved(ctx, tx, req.Id, req.AdminID, req.ShareAmount, strings.TrimSpace(req.AuditRemark)); err != nil {
					return err
				}
			}
			if _, err := s.directTransfer.EnsureTransferOrderTx(ctx, tx, &coboSvc.EnsureTransferOrderReq{BizType: bizType, BizID: req.Id, Phase: "main", UserID: currentTx.UserId, Symbol: "USDT", Amount: req.ShareAmount, Description: "饭桌分享会奖励"}); err != nil {
				return err
			}
			shouldSubmitTransfer = true
		}

		return nil
	})
	if err != nil {
		return err
	}

	// 满足至少两个不同管理员 approve 时才调用 Cobo 转账
	if shouldSubmitTransfer {
		if _, transferErr := s.directTransfer.SubmitTransferOrder(ctx, bizType, req.Id, "main"); transferErr != nil {
			g.Log().Errorf(ctx, "[TableShare] direct transfer failed: id=%d, err=%v", req.Id, transferErr)
			return transferErr
		}
	}
	return nil
}

func (s *service) smallTeamPerformance(ctx context.Context, userID int64) (decimal.Decimal, error) {
	code, err := getInviteCode(ctx, userID)
	if err != nil {
		return decimal.Zero, err
	}
	if code == "" {
		return decimal.Zero, nil
	}
	agg, err := teamstats.NewTeamStatsService().GetOverviewAggByInviteCode(ctx, code)
	if err != nil || agg == nil {
		return decimal.Zero, err
	}
	return agg.SmallTeamPerformance, nil
}

func getInviteCode(ctx context.Context, userID int64) (string, error) {
	v, err := g.DB().Model("user_info").Ctx(ctx).Fields("invite_code").Where("id = ?", userID).Value()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(v.String()), nil
}

func convert(row *repository.TableShareRow) *m.ApplicationItem {
	auditAt := ""
	if row.AuditAt != nil {
		auditAt = row.AuditAt.Format("2006-01-02 15:04:05")
	}
	return &m.ApplicationItem{Id: row.Id, UserId: row.UserId, WalletAddress: row.WalletAddress, Community: row.Community, DiningPeopleCount: row.DiningPeopleCount, SmallTeamPerformance: utils.FormatDecimal(row.SmallTeamPerformance), PreviousApprovedSmallTeamPerformance: utils.FormatDecimal(row.PreviousApprovedSmallTeamPerformance), CommunicationVideoUrls: parseStrings(row.CommunicationVideoUrlsText), SharingVideoUrls: parseStrings(row.SharingVideoUrlsText), SloganVideoUrls: parseStrings(row.SloganVideoUrlsText), Status: row.Status, ShareAmount: utils.FormatDecimal(row.ShareAmount), AuditBy: row.AuditBy, AuditRemark: row.AuditRemark, AuditAt: auditAt, CreatedAt: row.CreatedAt.Format("2006-01-02 15:04:05"), UpdatedAt: row.UpdatedAt.Format("2006-01-02 15:04:05")}
}

func fillTableApprovalInfo(ctx context.Context, item *m.ApplicationItem, adminID int64) error {
	if item == nil {
		return nil
	}
	approvalSvc := adminapproval.NewService()
	count, err := approvalSvc.GetApproveCount(ctx, adminapproval.BizTypeTableShare.String(), item.Id)
	if err != nil {
		return err
	}
	item.ApprovalCount = count
	if adminID > 0 {
		approved, err := approvalSvc.HasAdminApproved(ctx, adminapproval.BizTypeTableShare.String(), item.Id, adminID)
		if err != nil {
			return err
		}
		item.CurrentAdminApproved = approved
	}
	return nil
}

func parseStrings(v string) []string {
	var out []string
	if v == "" || json.Unmarshal([]byte(v), &out) != nil {
		return []string{}
	}
	return out
}

func normalizePage(page, pageSize *int) {
	if *page <= 0 {
		*page = 1
	}
	if *pageSize <= 0 {
		*pageSize = 10
	}
	if *pageSize > 100 {
		*pageSize = 100
	}
}

func codeErr(ctx context.Context, code int, key string) error {
	return gerror.NewCode(gcode.New(code, "", nil), consts.LocalizedText(errMsgs[key], consts.LocaleFromCtx(ctx)))
}

func readStringConfig(ctx context.Context, key, def string) string {
	v, err := g.DB().Model("system_config").Ctx(ctx).Fields("value").Where("key = ?", key).Value()
	if err != nil || strings.TrimSpace(v.String()) == "" {
		return def
	}
	return strings.TrimSpace(v.String())
}

func readDecimalConfig(ctx context.Context, key, def string) decimal.Decimal {
	d, err := decimal.NewFromString(readStringConfig(ctx, key, def))
	if err != nil {
		return decimal.RequireFromString(def)
	}
	return d
}

func readIntConfig(ctx context.Context, key string, def int) int {
	v := readStringConfig(ctx, key, fmt.Sprintf("%d", def))
	var out int
	if _, err := fmt.Sscanf(v, "%d", &out); err != nil || out <= 0 {
		return def
	}
	return out
}
