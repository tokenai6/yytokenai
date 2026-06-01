package officeapply

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"XWFrame/internal/frame/consts"
	frameModel "XWFrame/internal/frame/model"
	"XWFrame/internal/repository"
	"XWFrame/internal/service/adminapproval"
	coboSvc "XWFrame/internal/service/cobo"
	m "XWFrame/internal/service/officeapply/model"
	"XWFrame/internal/service/teamstats"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

const (
	configEnabled           = "office_apply_enabled"
	configPeriodStart       = "office_apply_period_start"
	configPeriodEnd         = "office_apply_period_end"
	configMinPerf           = "office_apply_min_small_team_performance"
	configLevel1Performance = "office_apply_level_1_performance"
	configLevel1Amount      = "office_apply_level_1_amount"
	configLevel2Performance = "office_apply_level_2_performance"
	configLevel2Amount      = "office_apply_level_2_amount"
	configLevel3Performance = "office_apply_level_3_performance"
	configLevel3Amount      = "office_apply_level_3_amount"
	configMaxImages         = "office_apply_max_images"
	configGrowthRate        = "office_apply_growth_rate"
)

var errMsgs = map[string]map[string]string{
	"not_found":                      {"zh-CN": "工作室申请不存在", "zh-TW": "工作室申請不存在", "en-US": "Office application not found", "ja-JP": "オフィス申請が見つかりません", "ko-KR": "오피스 신청을 찾을 수 없습니다", "vi-VN": "Không tìm thấy đơn đăng ký văn phòng", "th-TH": "ไม่พบคำขอ Office"},
	"disabled":                       {"zh-CN": "工作室申请功能未开启", "zh-TW": "工作室申請功能未開啟", "en-US": "Office application is disabled", "ja-JP": "オフィス申請は無効です", "ko-KR": "오피스 신청 기능이 비활성화되었습니다", "vi-VN": "Chức năng đăng ký văn phòng chưa bật", "th-TH": "ยังไม่ได้เปิดใช้งาน Office application"},
	"not_started":                    {"zh-CN": "工作室申请周期未开始", "zh-TW": "工作室申請週期未開始", "en-US": "Office application period has not started", "ja-JP": "オフィス申請期間はまだ開始されていません", "ko-KR": "오피스 신청 기간이 시작되지 않았습니다", "vi-VN": "Thời gian đăng ký văn phòng chưa bắt đầu", "th-TH": "ยังไม่เริ่มช่วงเวลาสมัคร Office"},
	"ended":                          {"zh-CN": "工作室申请周期已结束", "zh-TW": "工作室申請週期已結束", "en-US": "Office application period has ended", "ja-JP": "オフィス申請期間は終了しました", "ko-KR": "오피스 신청 기간이 종료되었습니다", "vi-VN": "Thời gian đăng ký văn phòng đã kết thúc", "th-TH": "ช่วงเวลาสมัคร Office สิ้นสุดแล้ว"},
	"leader_name_required":           {"zh-CN": "社区领袖必填", "zh-TW": "社區領袖必填", "en-US": "Community leader is required", "ja-JP": "コミュニティリーダーは必須です", "ko-KR": "커뮤니티 리더는 필수입니다", "vi-VN": "Vui lòng nhập lãnh đạo cộng đồng", "th-TH": "กรุณากรอกผู้นำชุมชน"},
	"community_address_required":     {"zh-CN": "社区地址必填", "zh-TW": "社區地址必填", "en-US": "Community address is required", "ja-JP": "コミュニティ住所は必須です", "ko-KR": "커뮤니티 주소는 필수입니다", "vi-VN": "Vui lòng nhập địa chỉ cộng đồng", "th-TH": "กรุณากรอกที่อยู่ชุมชน"},
	"contact_phone_required":         {"zh-CN": "联系电话必填", "zh-TW": "聯絡電話必填", "en-US": "Contact phone is required", "ja-JP": "連絡先電話番号は必須です", "ko-KR": "연락처는 필수입니다", "vi-VN": "Vui lòng nhập số điện thoại", "th-TH": "กรุณากรอกเบอร์ติดต่อ"},
	"images_required":                {"zh-CN": "请上传工作室场地照片", "zh-TW": "請上傳工作室場地照片", "en-US": "Please upload office photos", "ja-JP": "オフィス写真をアップロードしてください", "ko-KR": "오피스 사진을 업로드해 주세요", "vi-VN": "Vui lòng tải ảnh văn phòng", "th-TH": "กรุณาอัปโหลดรูปสำนักงาน"},
	"images_exceed_limit":            {"zh-CN": "工作室场地照片超过上限", "zh-TW": "工作室場地照片超過上限", "en-US": "Office photos exceed the limit", "ja-JP": "オフィス写真が上限を超えています", "ko-KR": "오피스 사진 수가 한도를 초과했습니다", "vi-VN": "Số ảnh văn phòng vượt quá giới hạn", "th-TH": "จำนวนรูปสำนักงานเกินขีดจำกัด"},
	"small_team_performance_not_met": {"zh-CN": "小区业绩未达到工作室申请门槛", "zh-TW": "小區業績未達到工作室申請門檻", "en-US": "Small team performance below office application threshold", "ja-JP": "小区業績がオフィス申請条件を満たしていません", "ko-KR": "소구역 실적이 오피스 신청 기준에 미달합니다", "vi-VN": "Hiệu suất nhóm nhỏ chưa đạt ngưỡng", "th-TH": "ผลงานทีมย่อยไม่ถึงเกณฑ์"},
	"performance_growth_not_met":     {"zh-CN": "再次申请需较上次审批时小区业绩新增至少50%", "zh-TW": "再次申請需較上次審批時小區業績新增至少50%", "en-US": "Small team performance must increase by at least 50% from the last approved application", "ja-JP": "再申請には前回承認時より小区実績が50%以上増加している必要があります", "ko-KR": "재신청 시 소구역 실적은 이전 승인 시점보다 최소 50% 증가해야 합니다", "vi-VN": "Hiệu suất nhóm nhỏ phải tăng ít nhất 50% so với lần phê duyệt trước", "th-TH": "การสมัครซ้ำต้องมีผลงานทีมย่อยเพิ่มขึ้นอย่างน้อย 50% จากครั้งที่อนุมัติก่อนหน้า"},
	"approved_cannot_update":         {"zh-CN": "已通过申请禁止更新", "zh-TW": "已通過申請禁止更新", "en-US": "Approved application cannot be updated", "ja-JP": "承認済み申請は更新できません", "ko-KR": "승인된 신청은 수정할 수 없습니다", "vi-VN": "Không thể cập nhật đơn đã duyệt", "th-TH": "ไม่สามารถแก้ไขคำขอที่อนุมัติแล้ว"},
	"invalid_status":                 {"zh-CN": "审批状态不合法", "zh-TW": "審批狀態不合法", "en-US": "Invalid approval status", "ja-JP": "承認状態が正しくありません", "ko-KR": "승인 상태가 유효하지 않습니다", "vi-VN": "Trạng thái phê duyệt không hợp lệ", "th-TH": "สถานะอนุมัติไม่ถูกต้อง"},
	"already_audited":                {"zh-CN": "申请已审批", "zh-TW": "申請已審批", "en-US": "Application already audited", "ja-JP": "申請は審査済みです", "ko-KR": "신청이 이미 심사되었습니다", "vi-VN": "Đơn đã được phê duyệt", "th-TH": "คำขอได้รับการตรวจสอบแล้ว"},
	"amount_invalid":                 {"zh-CN": "补贴额度不合法", "zh-TW": "補貼額度不合法", "en-US": "Invalid subsidy amount", "ja-JP": "補助金額が正しくありません", "ko-KR": "보조금 금액이 유효하지 않습니다", "vi-VN": "Số tiền trợ cấp không hợp lệ", "th-TH": "จำนวนเงินอุดหนุนไม่ถูกต้อง"},
	"amount_exceeds_limit":           {"zh-CN": "补贴额度超过当前档位上限", "zh-TW": "補貼額度超過當前檔位上限", "en-US": "Subsidy amount exceeds current level limit", "ja-JP": "補助金額が現在レベルの上限を超えています", "ko-KR": "보조금이 현재 등급 한도를 초과했습니다", "vi-VN": "Số tiền trợ cấp vượt quá giới hạn", "th-TH": "จำนวนเงินอุดหนุนเกินขีดจำกัด"},
	"already_approved_by_admin":      {"zh-CN": "您已审批过，不能重复操作", "zh-TW": "您已審批過，不能重複操作", "en-US": "You have already approved this application", "ja-JP": "この申請は既に承認済みです", "ko-KR": "이미 승인한 신청입니다", "vi-VN": "Bạn đã phê duyệt đơn này", "th-TH": "คุณได้อนุมัติคำขอนี้แล้ว"},
}

type Service interface {
	Config(ctx context.Context, userID int64) (*m.ConfigRes, error)
	AdminConfig(ctx context.Context) (*m.AdminConfigRes, error)
	UpdateConfig(ctx context.Context, req *UpdateConfigReq) error
	Apply(ctx context.Context, req *ApplyReq) (int64, error)
	MyApplication(ctx context.Context, userID int64) (*m.DetailRes, error)
	List(ctx context.Context, req *ListReq) (*m.ListRes, error)
	Get(ctx context.Context, id int64, adminID int64) (*m.DetailRes, error)
	Audit(ctx context.Context, req *AuditReq) error
}

type service struct {
	repo           repository.OfficeApplyRepository
	directTransfer coboSvc.DirectTransferService
}

func NewService() Service {
	return &service{
		repo:           repository.NewOfficeApplyRepository(),
		directTransfer: coboSvc.NewDirectTransferService(),
	}
}

type ApplyReq struct {
	UserId           int64
	LeaderName       string
	CommunityAddress string
	ContactPhone     string
	SubsidyAmount    decimal.Decimal
	ImageUrls        []string
}

type ListReq struct {
	AdminID          int64
	UserId           int64
	WalletAddress    string
	Status           int
	LeaderName       string
	CommunityAddress string
	ContactPhone     string
	Page, PageSize   int
}

type AuditReq struct {
	Id            int64
	AdminID       int64
	Status        int
	AuditRemark   string
	SubsidyAmount decimal.Decimal
}

type UpdateConfigReq struct {
	Enabled     bool
	PeriodStart string
	PeriodEnd   string
	Levels      []*m.LevelItem
	MaxImages   int
}

func (s *service) Config(ctx context.Context, userID int64) (*m.ConfigRes, error) {
	levels := readLevels(ctx)
	small, err := s.smallTeamPerformance(ctx, userID)
	if err != nil {
		return nil, err
	}
	level, amount := eligible(levels, small)
	app, err := s.repo.GetByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	enabled := readStringConfig(ctx, configEnabled, "true") != "false"
	return &m.ConfigRes{Enabled: enabled, PeriodStart: readStringConfig(ctx, configPeriodStart, "2026-05-22 00:00:00"), PeriodEnd: readStringConfig(ctx, configPeriodEnd, "2026-11-22 23:59:59"), SmallTeamPerformance: utils.FormatDecimal(small), EligibleLevel: level, EligibleSubsidyAmount: utils.FormatDecimal(amount), Levels: levels, MaxImages: readIntConfig(ctx, configMaxImages, 6), CanApply: enabled && level > 0, Application: convert(app)}, nil
}

func (s *service) AdminConfig(ctx context.Context) (*m.AdminConfigRes, error) {
	return &m.AdminConfigRes{Enabled: readStringConfig(ctx, configEnabled, "true") != "false", PeriodStart: readStringConfig(ctx, configPeriodStart, "2026-05-22 00:00:00"), PeriodEnd: readStringConfig(ctx, configPeriodEnd, "2026-11-22 23:59:59"), Levels: readLevels(ctx), MaxImages: readIntConfig(ctx, configMaxImages, 6)}, nil
}

func (s *service) UpdateConfig(ctx context.Context, req *UpdateConfigReq) error {
	if req.MaxImages <= 0 || len(req.Levels) == 0 {
		return codeErr(ctx, 400, "amount_invalid")
	}
	configs := map[string]string{configEnabled: fmt.Sprintf("%t", req.Enabled), configPeriodStart: strings.TrimSpace(req.PeriodStart), configPeriodEnd: strings.TrimSpace(req.PeriodEnd), configMaxImages: fmt.Sprintf("%d", req.MaxImages)}
	for _, level := range req.Levels {
		perf, err := decimal.NewFromString(strings.TrimSpace(level.MinPerformance))
		if err != nil || perf.IsNegative() {
			return codeErr(ctx, 400, "amount_invalid")
		}
		amount, err := decimal.NewFromString(strings.TrimSpace(level.SubsidyAmount))
		if err != nil || amount.IsNegative() {
			return codeErr(ctx, 400, "amount_invalid")
		}
		switch level.Level {
		case 1:
			configs[configLevel1Performance], configs[configLevel1Amount] = perf.String(), amount.String()
		case 2:
			configs[configLevel2Performance], configs[configLevel2Amount] = perf.String(), amount.String()
		case 3:
			configs[configLevel3Performance], configs[configLevel3Amount] = perf.String(), amount.String()
		}
	}
	configs[configMinPerf] = configs[configLevel1Performance]
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		for key, value := range configs {
			if strings.TrimSpace(value) == "" {
				continue
			}
			if _, err := tx.Exec("UPDATE system_config SET value = ?, updated_at = NOW() WHERE key = ?", value, key); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *service) Apply(ctx context.Context, req *ApplyReq) (int64, error) {
	if err := s.checkOpen(ctx); err != nil {
		return 0, err
	}
	if strings.TrimSpace(req.LeaderName) == "" {
		return 0, codeErr(ctx, 400, "leader_name_required")
	}
	if strings.TrimSpace(req.CommunityAddress) == "" {
		return 0, codeErr(ctx, 400, "community_address_required")
	}
	if strings.TrimSpace(req.ContactPhone) == "" {
		return 0, codeErr(ctx, 400, "contact_phone_required")
	}
	maxImages := readIntConfig(ctx, configMaxImages, 6)
	if len(req.ImageUrls) == 0 {
		return 0, codeErr(ctx, 400, "images_required")
	}
	if len(req.ImageUrls) > maxImages {
		return 0, codeErr(ctx, 400, "images_exceed_limit")
	}
	small, err := s.smallTeamPerformance(ctx, req.UserId)
	if err != nil {
		return 0, err
	}
	level, maxAmount := eligible(readLevels(ctx), small)
	if level == 0 || small.LessThan(readDecimalConfig(ctx, configMinPerf, "4000")) {
		return 0, codeErr(ctx, 400, "small_team_performance_not_met")
	}
	lastApproved, err := s.repo.GetLastApprovedByUser(ctx, req.UserId)
	if err != nil {
		return 0, err
	}
	if lastApproved != nil {
		growthRate := readDecimalConfig(ctx, configGrowthRate, "0.5")
		if small.Sub(lastApproved.SmallTeamPerformance).LessThan(lastApproved.SmallTeamPerformance.Mul(growthRate)) {
			return 0, codeErr(ctx, 400, "performance_growth_not_met")
		}
	}
	amount := req.SubsidyAmount
	if amount.IsZero() || amount.IsNegative() {
		return 0, codeErr(ctx, 400, "amount_invalid")
	}
	if amount.GreaterThan(maxAmount) {
		return 0, codeErr(ctx, 400, "amount_exceeds_limit")
	}
	create := &repository.CreateOfficeApplyReq{UserId: req.UserId, LeaderName: strings.TrimSpace(req.LeaderName), CommunityAddress: strings.TrimSpace(req.CommunityAddress), ContactPhone: strings.TrimSpace(req.ContactPhone), SmallTeamPerformance: small, EligibleLevel: level, SubsidyAmount: amount, ImageUrls: req.ImageUrls}
	var id int64
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		current, err := s.repo.GetByUserForUpdate(ctx, tx, req.UserId)
		if err != nil {
			return err
		}
		if current == nil {
			id, err = s.repo.Create(ctx, tx, create)
			return err
		}
		id = current.Id
		switch current.Status {
		case repository.OfficeApplyApproved:
			id, err = s.repo.Create(ctx, tx, create)
			return err
		case repository.OfficeApplyPending:
			return s.repo.UpdatePending(ctx, tx, current.Id, create)
		case repository.OfficeApplyRejected:
			return s.repo.ResetRejected(ctx, tx, current.Id, create)
		default:
			return codeErr(ctx, 400, "invalid_status")
		}
	})
	return id, err
}

func (s *service) MyApplication(ctx context.Context, userID int64) (*m.DetailRes, error) {
	row, err := s.repo.GetByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, codeErr(ctx, 404, "not_found")
	}
	return &m.DetailRes{ApplicationItem: convert(row)}, nil
}

func (s *service) List(ctx context.Context, req *ListReq) (*m.ListRes, error) {
	page, pageSize := req.Page, req.PageSize
	normalizePage(&page, &pageSize)
	rows, total, err := s.repo.List(ctx, &repository.OfficeApplyListReq{UserId: req.UserId, WalletAddress: req.WalletAddress, Status: req.Status, LeaderName: req.LeaderName, CommunityAddress: req.CommunityAddress, ContactPhone: req.ContactPhone, Page: page, PageSize: pageSize})
	if err != nil {
		return nil, err
	}
	list := make([]*m.ApplicationItem, 0, len(rows))
	for _, row := range rows {
		item := convert(row)
		if err := fillOfficeApprovalInfo(ctx, item, req.AdminID); err != nil {
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
	if err := fillOfficeApprovalInfo(ctx, item, adminID); err != nil {
		return nil, err
	}
	return &m.DetailRes{ApplicationItem: item}, nil
}

func (s *service) Audit(ctx context.Context, req *AuditReq) error {
	if req.Status != repository.OfficeApplyApproved && req.Status != repository.OfficeApplyRejected {
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
	bizType := adminapproval.BizTypeOfficeApply.String()

	// 拒绝操作
	if req.Status == repository.OfficeApplyRejected {
		return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
			currentTx, err := s.repo.GetByIDForUpdate(ctx, tx, req.Id)
			if err != nil {
				return err
			}
			if currentTx == nil {
				return codeErr(ctx, 404, "not_found")
			}
			if currentTx.Status != repository.OfficeApplyPending {
				return codeErr(ctx, 400, "already_audited")
			}
			if err := s.repo.AuditRejected(ctx, tx, req.Id, req.AdminID, strings.TrimSpace(req.AuditRemark)); err != nil {
				return err
			}
			return approvalSvc.RecordApprovalTx(ctx, tx, bizType, req.Id, req.AdminID, adminapproval.ActionReject, strings.TrimSpace(req.AuditRemark))
		})
	}

	// 通过操作
	if current.Status == repository.OfficeApplyRejected {
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

		maxAmount := currentTx.SubsidyAmount
		amount := req.SubsidyAmount
		if amount.IsZero() {
			amount = maxAmount
		}
		if !amount.GreaterThan(decimal.Zero) {
			return codeErr(ctx, 400, "amount_invalid")
		}
		if amount.GreaterThan(maxAmount) {
			return codeErr(ctx, 400, "amount_exceeds_limit")
		}

		// 记录审批
		if currentTx.Status == repository.OfficeApplyPending {
			if err := s.repo.UpdateSubsidyAmountTx(ctx, tx, req.Id, amount); err != nil {
				return err
			}
		}
		if err := approvalSvc.RecordApprovalTx(ctx, tx, bizType, req.Id, req.AdminID, adminapproval.ActionApprove, strings.TrimSpace(req.AuditRemark)); err != nil {
			return err
		}

		// 如果已有其他管理员 approve，触发转账
		if approveCount >= 1 {
			if currentTx.Status == repository.OfficeApplyPending {
				if err := s.repo.AuditApproved(ctx, tx, req.Id, req.AdminID, amount, strings.TrimSpace(req.AuditRemark)); err != nil {
					return err
				}
			}
			if _, err := s.directTransfer.EnsureTransferOrderTx(ctx, tx, &coboSvc.EnsureTransferOrderReq{BizType: bizType, BizID: req.Id, Phase: "main", UserID: currentTx.UserId, Symbol: "USDT", Amount: amount, Description: "工作室申请补贴"}); err != nil {
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
			g.Log().Errorf(ctx, "[OfficeApply] direct transfer failed: id=%d, err=%v", req.Id, transferErr)
			return transferErr
		}
	}
	return nil
}

func (s *service) checkOpen(ctx context.Context) error {
	if readStringConfig(ctx, configEnabled, "true") == "false" {
		return codeErr(ctx, 400, "disabled")
	}
	now := time.Now()
	if start, ok := parseConfigTime(readStringConfig(ctx, configPeriodStart, "2026-05-22 00:00:00")); ok && now.Before(start) {
		return codeErr(ctx, 400, "not_started")
	}
	if end, ok := parseConfigTime(readStringConfig(ctx, configPeriodEnd, "2026-11-22 23:59:59")); ok && now.After(end) {
		return codeErr(ctx, 400, "ended")
	}
	return nil
}

func (s *service) smallTeamPerformance(ctx context.Context, userID int64) (decimal.Decimal, error) {
	code, err := getInviteCode(ctx, userID)
	if err != nil || code == "" {
		return decimal.Zero, err
	}
	agg, err := teamstats.NewTeamStatsService().GetOverviewAggByInviteCode(ctx, code)
	if err != nil || agg == nil {
		return decimal.Zero, err
	}
	return agg.SmallTeamPerformance, nil
}

func readLevels(ctx context.Context) []*m.LevelItem {
	return []*m.LevelItem{
		{Level: 1, MinPerformance: utils.FormatDecimal(readDecimalConfig(ctx, configLevel1Performance, "4000")), SubsidyAmount: utils.FormatDecimal(readDecimalConfig(ctx, configLevel1Amount, "800"))},
		{Level: 2, MinPerformance: utils.FormatDecimal(readDecimalConfig(ctx, configLevel2Performance, "15000")), SubsidyAmount: utils.FormatDecimal(readDecimalConfig(ctx, configLevel2Amount, "1500"))},
		{Level: 3, MinPerformance: utils.FormatDecimal(readDecimalConfig(ctx, configLevel3Performance, "30000")), SubsidyAmount: utils.FormatDecimal(readDecimalConfig(ctx, configLevel3Amount, "3000"))},
	}
}

func eligible(levels []*m.LevelItem, perf decimal.Decimal) (int, decimal.Decimal) {
	level, amount := 0, decimal.Zero
	for _, item := range levels {
		min, err := decimal.NewFromString(item.MinPerformance)
		if err != nil || perf.LessThan(min) {
			continue
		}
		parsedAmount, err := decimal.NewFromString(item.SubsidyAmount)
		if err == nil && item.Level > level {
			level, amount = item.Level, parsedAmount
		}
	}
	return level, amount
}

func getInviteCode(ctx context.Context, userID int64) (string, error) {
	v, err := g.DB().Model("user_info").Ctx(ctx).Fields("invite_code").Where("id = ?", userID).Value()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(v.String()), nil
}

func convert(row *repository.OfficeApplyRow) *m.ApplicationItem {
	if row == nil {
		return nil
	}
	auditAt := ""
	if row.AuditAt != nil {
		auditAt = row.AuditAt.Format("2006-01-02 15:04:05")
	}
	return &m.ApplicationItem{Id: row.Id, UserId: row.UserId, WalletAddress: row.WalletAddress, LeaderName: row.LeaderName, CommunityAddress: row.CommunityAddress, ContactPhone: row.ContactPhone, SmallTeamPerformance: utils.FormatDecimal(row.SmallTeamPerformance), PreviousApprovedSmallTeamPerformance: utils.FormatDecimal(row.PreviousApprovedSmallTeamPerformance), EligibleLevel: row.EligibleLevel, SubsidyAmount: utils.FormatDecimal(row.SubsidyAmount), ImageUrls: parseStrings(row.ImageUrlsText), Status: row.Status, AuditBy: row.AuditBy, AuditRemark: row.AuditRemark, AuditAt: auditAt, CreatedAt: row.CreatedAt.Format("2006-01-02 15:04:05"), UpdatedAt: row.UpdatedAt.Format("2006-01-02 15:04:05")}
}

func fillOfficeApprovalInfo(ctx context.Context, item *m.ApplicationItem, adminID int64) error {
	if item == nil {
		return nil
	}
	approvalSvc := adminapproval.NewService()
	count, err := approvalSvc.GetApproveCount(ctx, adminapproval.BizTypeOfficeApply.String(), item.Id)
	if err != nil {
		return err
	}
	item.ApprovalCount = count
	if adminID > 0 {
		approved, err := approvalSvc.HasAdminApproved(ctx, adminapproval.BizTypeOfficeApply.String(), item.Id, adminID)
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

func parseConfigTime(v string) (time.Time, bool) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	t, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(v), loc)
	return t, err == nil
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

func LocalizedError(ctx context.Context, code int, key string) error {
	return codeErr(ctx, code, key)
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
