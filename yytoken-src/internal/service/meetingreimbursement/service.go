package meetingreimbursement

import (
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"XWFrame/internal/frame/consts"
	frameModel "XWFrame/internal/frame/model"
	"XWFrame/internal/repository"
	repo "XWFrame/internal/repository/cobo"
	"XWFrame/internal/service/adminapproval"
	m "XWFrame/internal/service/meetingreimbursement/model"
	"XWFrame/internal/service/teamstats"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

const (
	configEnabled                 = "attendance_reimbursement_enabled"
	configStartAt                 = "attendance_reimbursement_start_at"
	configEndAt                   = "attendance_reimbursement_end_at"
	configMinSmallTeamPerformance = "attendance_reimbursement_min_small_team_performance"
	configMaxAmount               = "attendance_reimbursement_max_amount"
	configAcceptLimit             = "attendance_reimbursement_accept_limit"
)

var errMsgs = map[string]map[string]string{
	"not_found":                 {"zh-CN": "参会报销申请不存在", "zh-TW": "參會報銷申請不存在", "en-US": "Attendance reimbursement application not found", "ja-JP": "参加経費申請が見つかりません", "ko-KR": "참석 환급 신청을 찾을 수 없습니다", "vi-VN": "Không tìm thấy đơn hoàn trả tham dự", "th-TH": "ไม่พบคำขอเบิกค่าเข้าร่วม"},
	"invalid_email":             {"zh-CN": "邮箱格式错误", "zh-TW": "郵箱格式錯誤", "en-US": "Invalid email format", "ja-JP": "メール形式が正しくありません", "ko-KR": "이메일 형식이 잘못되었습니다", "vi-VN": "Định dạng email không hợp lệ", "th-TH": "รูปแบบอีเมลไม่ถูกต้อง"},
	"images_required":           {"zh-CN": "请上传报销凭证", "zh-TW": "請上傳報銷憑證", "en-US": "Please upload reimbursement receipts", "ja-JP": "経費証憑をアップロードしてください", "ko-KR": "환급 증빙을 업로드해 주세요", "vi-VN": "Vui lòng tải lên chứng từ hoàn trả", "th-TH": "กรุณาอัปโหลดหลักฐานการเบิกจ่าย"},
	"images_exceed_limit":       {"zh-CN": "报销凭证最多10张", "zh-TW": "報銷憑證最多10張", "en-US": "Receipts cannot exceed 10 images", "ja-JP": "証憑画像は最大10枚です", "ko-KR": "증빙 이미지는 최대 10장입니다", "vi-VN": "Chứng từ tối đa 10 ảnh", "th-TH": "หลักฐานสูงสุด 10 รูป"},
	"small_team_not_met":        {"zh-CN": "小区业绩未达到参会报销门槛", "zh-TW": "小區業績未達到參會報銷門檻", "en-US": "Small team performance below attendance reimbursement threshold", "ja-JP": "小区業績が条件を満たしていません", "ko-KR": "소구역 실적이 기준에 미달합니다", "vi-VN": "Hiệu suất nhóm nhỏ chưa đạt ngưỡng", "th-TH": "ผลงานทีมย่อยไม่ถึงเกณฑ์"},
	"approved_cannot_update":    {"zh-CN": "已通过申请禁止更新", "zh-TW": "已通過申請禁止更新", "en-US": "Approved application cannot be updated", "ja-JP": "承認済み申請は更新できません", "ko-KR": "승인된 신청은 수정할 수 없습니다", "vi-VN": "Không thể cập nhật đơn đã duyệt", "th-TH": "ไม่สามารถแก้ไขคำขอที่อนุมัติแล้ว"},
	"invalid_status":            {"zh-CN": "审批状态不合法", "zh-TW": "審批狀態不合法", "en-US": "Invalid approval status", "ja-JP": "承認状態が正しくありません", "ko-KR": "승인 상태가 유효하지 않습니다", "vi-VN": "Trạng thái phê duyệt không hợp lệ", "th-TH": "สถานะอนุมัติไม่ถูกต้อง"},
	"already_audited":           {"zh-CN": "申请已审批", "zh-TW": "申請已審批", "en-US": "Application already audited", "ja-JP": "申請は審査済みです", "ko-KR": "신청이 이미 심사되었습니다", "vi-VN": "Đơn đã được phê duyệt", "th-TH": "คำขอได้รับการตรวจสอบแล้ว"},
	"amount_invalid":            {"zh-CN": "报销额度不合法", "zh-TW": "報銷額度不合法", "en-US": "Invalid reimbursement amount", "ja-JP": "経費金額が正しくありません", "ko-KR": "환급 금액이 유효하지 않습니다", "vi-VN": "Số tiền hoàn trả không hợp lệ", "th-TH": "จำนวนเงินเบิกจ่ายไม่ถูกต้อง"},
	"amount_exceeds_limit":      {"zh-CN": "报销额度超过上限", "zh-TW": "報銷額度超過上限", "en-US": "Reimbursement amount exceeds limit", "ja-JP": "上限を超えています", "ko-KR": "상한을 초과했습니다", "vi-VN": "Số tiền vượt quá giới hạn", "th-TH": "จำนวนเงินเกินวงเงิน"},
	"accept_limit_reached":      {"zh-CN": "通过名额已满", "zh-TW": "通過名額已滿", "en-US": "Approval limit reached", "ja-JP": "承認枠が上限に達しました", "ko-KR": "승인 한도에 도달했습니다", "vi-VN": "Đã hết hạn mức phê duyệt", "th-TH": "จำนวนอนุมัติเต็มแล้ว"},
	"stake_not_adjustable":      {"zh-CN": "当前赠送质押不可调整", "zh-TW": "當前贈送質押不可調整", "en-US": "Gift stake cannot be adjusted", "ja-JP": "贈与ステークは調整できません", "ko-KR": "증정 스테이킹은 조정할 수 없습니다", "vi-VN": "Không thể điều chỉnh stake tặng", "th-TH": "ไม่สามารถปรับ stake ที่มอบให้ได้"},
	"reward_limit_too_low":      {"zh-CN": "调整后收益上限低于已发收益", "zh-TW": "調整後收益上限低於已發收益", "en-US": "Adjusted reward limit is below earned rewards", "ja-JP": "調整後の上限が獲得済み報酬を下回ります", "ko-KR": "조정 후 한도가 이미 지급된 보상보다 낮습니다", "vi-VN": "Giới hạn sau điều chỉnh thấp hơn thưởng đã nhận", "th-TH": "วงเงินหลังปรับต่ำกว่ารางวัลที่ได้รับแล้ว"},
	"disabled":                  {"zh-CN": "参会报销功能未开启", "zh-TW": "參會報銷功能未開啟", "en-US": "Attendance reimbursement is disabled", "ja-JP": "参加経費精算は無効です", "ko-KR": "참석 환급 기능이 비활성화되었습니다", "vi-VN": "Chức năng hoàn trả tham dự chưa bật", "th-TH": "ยังไม่ได้เปิดใช้งานการเบิกค่าเข้าร่วม"},
	"not_started":               {"zh-CN": "参会报销活动未开始", "zh-TW": "參會報銷活動未開始", "en-US": "Attendance reimbursement has not started", "ja-JP": "参加経費精算はまだ開始していません", "ko-KR": "참석 환급이 아직 시작되지 않았습니다", "vi-VN": "Hoàn trả tham dự chưa bắt đầu", "th-TH": "การเบิกค่าเข้าร่วมยังไม่เริ่ม"},
	"ended":                     {"zh-CN": "参会报销活动已结束", "zh-TW": "參會報銷活動已結束", "en-US": "Attendance reimbursement has ended", "ja-JP": "参加経費精算は終了しました", "ko-KR": "참석 환급이 종료되었습니다", "vi-VN": "Hoàn trả tham dự đã kết thúc", "th-TH": "การเบิกค่าเข้าร่วมสิ้นสุดแล้ว"},
	"no_active_activity":        {"zh-CN": "当前没有启用的参会报销活动", "zh-TW": "目前沒有啟用的參會報銷活動", "en-US": "No active attendance reimbursement activity", "ja-JP": "有効な参加経費活動がありません", "ko-KR": "활성화된 참석 환급 활동이 없습니다", "vi-VN": "Không có hoạt động hoàn trả tham dự đang bật", "th-TH": "ไม่มีกิจกรรมเบิกค่าเข้าร่วมที่เปิดใช้งาน"},
	"already_approved_by_admin": {"zh-CN": "您已审批过，不能重复操作", "zh-TW": "您已審批過，不能重複操作", "en-US": "You have already approved this application", "ja-JP": "この申請は既に承認済みです", "ko-KR": "이미 승인한 신청입니다", "vi-VN": "Bạn đã phê duyệt đơn này", "th-TH": "คุณได้อนุมัติคำขอนี้แล้ว"},
}

type Service interface {
	Config(ctx context.Context, userID int64) (*m.ConfigRes, error)
	Apply(ctx context.Context, req *ApplyReq) (int64, error)
	ListMy(ctx context.Context, userID int64, page, pageSize int) (*m.ListRes, error)
	Get(ctx context.Context, id int64, adminID int64) (*m.DetailRes, error)
	List(ctx context.Context, req *ListReq) (*m.ListRes, error)
	Audit(ctx context.Context, req *AuditReq) error
	AdjustStake(ctx context.Context, req *AdjustStakeReq) error
	CreateActivity(ctx context.Context, req *ActivitySaveReq) (int64, error)
	UpdateActivity(ctx context.Context, req *ActivitySaveReq) error
	ActivateActivity(ctx context.Context, id int64) error
	ListActivities(ctx context.Context, page, pageSize int) (*m.ActivityListRes, error)
}

type service struct {
	repo        repository.MeetingReimbursementRepository
	balanceRepo repo.IBalanceRepository
}

func NewService() Service {
	return &service{
		repo:        repository.NewMeetingReimbursementRepository(),
		balanceRepo: repo.NewBalanceRepository(),
	}
}

type ApplyReq struct {
	UserId           int64
	Community, Email string
	ImageUrls        []string
}
type ListReq struct {
	AdminID          int64
	UserId           int64
	WalletAddress    string
	Status           int
	Community, Email string
	Page, PageSize   int
}
type AuditReq struct {
	Id, AdminID         int64
	Status              int
	AuditRemark         string
	ReimbursementAmount decimal.Decimal
}
type AdjustStakeReq struct {
	Id, AdminID int64
	NewAmount   decimal.Decimal
	Remark      string
}

type ActivitySaveReq struct {
	Id                      int64
	Title                   string
	Content                 string
	StartAt                 string
	EndAt                   string
	MinSmallTeamPerformance decimal.Decimal
	MaxReimbursementAmount  decimal.Decimal
	AcceptLimit             int
}

func (s *service) Apply(ctx context.Context, req *ApplyReq) (int64, error) {
	if readStringConfig(ctx, configEnabled, "true") == "false" {
		return 0, codeErr(ctx, 400, "disabled")
	}
	if _, err := mail.ParseAddress(strings.TrimSpace(req.Email)); err != nil {
		return 0, codeErr(ctx, 400, "invalid_email")
	}
	if len(req.ImageUrls) == 0 {
		return 0, codeErr(ctx, 400, "images_required")
	}
	if len(req.ImageUrls) > 10 {
		return 0, codeErr(ctx, 400, "images_exceed_limit")
	}
	activity, err := s.repo.GetActiveActivity(ctx)
	if err != nil {
		return 0, err
	}
	if activity == nil {
		return 0, codeErr(ctx, 400, "no_active_activity")
	}
	if err := checkActivityPeriod(ctx, activity); err != nil {
		return 0, err
	}
	small, err := s.smallTeamPerformance(ctx, req.UserId, activity)
	if err != nil {
		return 0, err
	}
	if small.LessThan(decimal.RequireFromString(activity.MinSmallTeamPerformance)) {
		return 0, codeErr(ctx, 400, "small_team_not_met")
	}
	create := &repository.CreateMeetingReimbursementReq{ActivityId: activity.Id, UserId: req.UserId, Community: strings.TrimSpace(req.Community), Email: strings.TrimSpace(req.Email), ImageUrls: req.ImageUrls, SmallTeamPerformance: small}
	var id int64
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		current, err := s.repo.GetLatestByUserForUpdate(ctx, tx, req.UserId)
		if err != nil {
			return err
		}
		if current != nil && current.Status != repository.MeetingReimbursementApproved {
			if current.Status == repository.MeetingReimbursementPending {
				id, err = s.repo.UpdatePendingByUser(ctx, tx, req.UserId, create)
				return err
			}
			if current.Status == repository.MeetingReimbursementRejected {
				id, err = s.repo.ResetRejectedByUser(ctx, tx, req.UserId, create)
				return err
			}
		}
		id, err = s.repo.Create(ctx, tx, create)
		return err
	})
	return id, err
}

func (s *service) Config(ctx context.Context, userID int64) (*m.ConfigRes, error) {
	activity, _ := s.repo.GetActiveActivity(ctx)
	small, err := s.smallTeamPerformance(ctx, userID, activity)
	if err != nil {
		return nil, err
	}
	maxAmount := readDecimalConfig(ctx, configMaxAmount, "5000")
	limit := readIntConfig(ctx, configAcceptLimit, 99)
	minPerf := readDecimalConfig(ctx, configMinSmallTeamPerformance, "20000")
	accepted := 0
	if activity != nil {
		maxAmount = decimal.RequireFromString(activity.MaxReimbursementAmount)
		limit = activity.AcceptLimit
		minPerf = decimal.RequireFromString(activity.MinSmallTeamPerformance)
		accepted, _ = s.repo.CountApprovedByActivity(ctx, activity.Id)
	}
	now := time.Now()
	start := readStringConfig(ctx, configStartAt, "2026-05-24 00:00:00")
	end := readStringConfig(ctx, configEndAt, "2026-06-15 23:59:59")
	if activity != nil {
		start = activity.StartAt.Format("2006-01-02 15:04:05")
		end = activity.EndAt.Format("2006-01-02 15:04:05")
	}
	return &m.ConfigRes{Enabled: readStringConfig(ctx, configEnabled, "true") != "false", Activity: convertActivity(activity), StartAt: start, EndAt: end, Now: now.Format("2006-01-02 15:04:05"), SmallTeamPerformance: utils.FormatDecimal(small), MinSmallTeamPerformance: utils.FormatDecimal(minPerf), MaxAmount: utils.FormatDecimal(maxAmount), AcceptLimit: limit, AcceptedCount: accepted, CanApply: activity != nil && !small.LessThan(minPerf) && accepted < limit}, nil
}

func (s *service) Audit(ctx context.Context, req *AuditReq) error {
	if req.Status != repository.MeetingReimbursementApproved && req.Status != repository.MeetingReimbursementRejected {
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
	bizType := adminapproval.BizTypeMeetingReimbursement.String()

	// 拒绝操作
	if req.Status == repository.MeetingReimbursementRejected {
		return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
			currentTx, err := s.repo.GetByIDForUpdate(ctx, tx, req.Id)
			if err != nil {
				return err
			}
			if currentTx == nil {
				return codeErr(ctx, 404, "not_found")
			}
			if currentTx.Status != repository.MeetingReimbursementPending {
				return codeErr(ctx, 400, "already_audited")
			}
			if err := s.repo.AuditRejected(ctx, tx, &repository.AuditMeetingReimbursementReq{Id: req.Id, AdminID: req.AdminID, AuditRemark: strings.TrimSpace(req.AuditRemark)}); err != nil {
				return err
			}
			return approvalSvc.RecordApprovalTx(ctx, tx, bizType, req.Id, req.AdminID, adminapproval.ActionReject, strings.TrimSpace(req.AuditRemark))
		})
	}

	// 通过操作
	if current.Status == repository.MeetingReimbursementRejected {
		return codeErr(ctx, 400, "already_audited")
	}

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

		reimbursementAmount := req.ReimbursementAmount
		if currentTx.ReimbursementAmount.GreaterThan(decimal.Zero) {
			reimbursementAmount = currentTx.ReimbursementAmount
		}
		if !reimbursementAmount.GreaterThan(decimal.Zero) {
			return codeErr(ctx, 400, "amount_invalid")
		}
		activity, err := s.repo.GetActiveActivity(ctx)
		if err != nil {
			return err
		}
		if activity == nil || currentTx.ActivityId != activity.Id {
			return codeErr(ctx, 400, "no_active_activity")
		}
		maxAmount := decimal.RequireFromString(activity.MaxReimbursementAmount)
		if reimbursementAmount.GreaterThan(maxAmount) {
			return codeErr(ctx, 400, "amount_exceeds_limit")
		}
		approved, err := s.repo.CountApprovedByActivity(ctx, activity.Id)
		if err != nil {
			return err
		}
		if approved >= activity.AcceptLimit {
			return codeErr(ctx, 400, "accept_limit_reached")
		}

		if currentTx.Status == repository.MeetingReimbursementPending {
			if err := s.repo.UpdateAmount(ctx, tx, req.Id, reimbursementAmount, req.AuditRemark, req.AdminID); err != nil {
				return err
			}
		}

		// 记录审批
		if err := approvalSvc.RecordApprovalTx(ctx, tx, bizType, req.Id, req.AdminID, adminapproval.ActionApprove, strings.TrimSpace(req.AuditRemark)); err != nil {
			return err
		}

		// 如果已有其他管理员 approve，触发赠送质押
		if approveCount >= 1 {
			orderID, err := s.createGiftStakeTx(ctx, tx, currentTx.UserId, req.AdminID, req.Id, reimbursementAmount, req.AuditRemark)
			if err != nil {
				return err
			}
			if err := s.repo.AuditApproved(ctx, tx, &repository.AuditMeetingReimbursementReq{Id: req.Id, AdminID: req.AdminID, AuditRemark: strings.TrimSpace(req.AuditRemark), ReimbursementAmount: reimbursementAmount, StakingV2OrderID: orderID}); err != nil {
				return err
			}
		}

		return nil
	})
	return err
}

func (s *service) AdjustStake(ctx context.Context, req *AdjustStakeReq) error {
	if !req.NewAmount.GreaterThan(decimal.Zero) {
		return codeErr(ctx, 400, "amount_invalid")
	}
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		current, err := s.repo.GetByIDForUpdate(ctx, tx, req.Id)
		if err != nil {
			return err
		}
		if current == nil {
			return codeErr(ctx, 404, "not_found")
		}
		if current.Status != repository.MeetingReimbursementApproved || current.StakingV2OrderId == 0 {
			return codeErr(ctx, 400, "stake_not_adjustable")
		}
		activity, err := s.repo.GetActiveActivity(ctx)
		if err != nil {
			return err
		}
		if activity != nil && current.ActivityId == activity.Id && req.NewAmount.GreaterThan(decimal.RequireFromString(activity.MaxReimbursementAmount)) {
			return codeErr(ctx, 400, "amount_exceeds_limit")
		}
		var order struct {
			Id        int64
			Amount    decimal.Decimal
			Status    int
			IsGift    int
			RequestId string `json:"request_id"`
		}
		if err := tx.Model("staking_v2_order").Ctx(ctx).Where("id = ?", current.StakingV2OrderId).LockUpdate().Scan(&order); err != nil {
			return err
		}
		if order.Id == 0 || order.IsGift != 1 || order.Status != consts.StakingV2StatusActive || !strings.HasPrefix(order.RequestId, "ATTENDANCE-REIMBURSEMENT-") {
			return codeErr(ctx, 400, "stake_not_adjustable")
		}
		var stats struct {
			RewardLimit       decimal.Decimal `json:"reward_limit"`
			TotalRewardEarned decimal.Decimal `json:"total_reward_earned"`
		}
		if err := tx.Model("staking_v2_user_stats").Ctx(ctx).Where("user_id = ?", current.UserId).LockUpdate().Scan(&stats); err != nil {
			return err
		}
		delta := req.NewAmount.Sub(order.Amount)
		newLimit := stats.RewardLimit.Add(delta.Mul(decimal.NewFromInt(4)))
		if newLimit.LessThan(stats.TotalRewardEarned) {
			return codeErr(ctx, 400, "reward_limit_too_low")
		}
		now := time.Now()
		if _, err := tx.Model("staking_v2_order").Ctx(ctx).Where("id = ?", order.Id).Data(g.Map{"amount": req.NewAmount, "updated_at": now}).Update(); err != nil {
			return err
		}
		if _, err := tx.Model("staking_v2_user_stats").Ctx(ctx).Where("user_id = ?", current.UserId).Data(g.Map{"total_stake_amount": gdb.Raw(fmt.Sprintf("total_stake_amount + %s", delta.String())), "reward_limit": gdb.Raw(fmt.Sprintf("reward_limit + %s", delta.Mul(decimal.NewFromInt(4)).String())), "is_capped": false, "capped_at": nil, "updated_at": now}).Update(); err != nil {
			return err
		}
		if err := s.repo.UpdateAmount(ctx, tx, req.Id, req.NewAmount, req.Remark, req.AdminID); err != nil {
			return err
		}
		return s.writeBalanceLogTx(ctx, tx, current.UserId, fmt.Sprintf("ATTENDANCE-REIMBURSEMENT-ADJUST-%d", req.Id), req.Id, fmt.Sprintf("参会报销质押调整 old=%s new=%s remark=%s", order.Amount, req.NewAmount, strings.TrimSpace(req.Remark)))
	})
}

func (s *service) createGiftStakeTx(ctx context.Context, tx gdb.TX, userID, adminID, appID int64, amount decimal.Decimal, remark string) (int64, error) {
	now := time.Now()
	requestID := fmt.Sprintf("ATTENDANCE-REIMBURSEMENT-%d", appID)
	orderID, err := tx.Model("staking_v2_order").Ctx(ctx).Data(g.Map{"user_id": userID, "request_id": requestID, "amount": amount, "source_type": consts.StakingV2SourceTypeManualStake, "status": consts.StakingV2StatusActive, "total_reward": decimal.Zero, "reward_limit_multiplier": decimal.NewFromInt(4), "is_gift": 1, "gift_by": adminID, "gift_remark": fmt.Sprintf("参会报销申请ID:%d %s", appID, strings.TrimSpace(remark)), "created_at": now, "updated_at": now}).InsertAndGetId()
	if err != nil {
		return 0, err
	}
	_, err = tx.Exec(`INSERT INTO staking_v2_user_stats (user_id, total_stake_amount, total_reward_earned, reward_limit, is_capped, created_at, updated_at) VALUES (?, ?, 0, ?, FALSE, ?, ?) ON CONFLICT (user_id) DO UPDATE SET total_stake_amount = staking_v2_user_stats.total_stake_amount + EXCLUDED.total_stake_amount, reward_limit = staking_v2_user_stats.reward_limit + EXCLUDED.reward_limit, is_capped = FALSE, capped_at = NULL, updated_at = EXCLUDED.updated_at`, userID, amount, amount.Mul(decimal.NewFromInt(4)), now, now)
	if err != nil {
		return 0, err
	}
	if err := s.writeBalanceLogTx(ctx, tx, userID, requestID, appID, fmt.Sprintf("参会报销赠送质押 amount=%s remark=%s", amount, strings.TrimSpace(remark))); err != nil {
		return 0, err
	}
	return orderID, nil
}

func (s *service) writeBalanceLogTx(ctx context.Context, tx gdb.TX, userID int64, orderNo string, relatedID int64, remark string) error {
	bal, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, userID, "USDT")
	if err != nil {
		return err
	}
	balance := decimal.Zero
	if bal != nil {
		balance = bal.AvailableAmount
	}
	_, err = tx.Exec(`INSERT INTO cobo_balance_change_log (user_id, symbol, change_type, amount, before_balance, after_balance, related_order_no, related_id, remark, operator_type, created_at) VALUES (?, 'USDT', ?, 0, ?, ?, ?, ?, ?, ?, ?)`, userID, consts.ChangeTypeStakingV2Stake, balance, balance, orderNo, relatedID, remark, consts.OperatorTypeSystem, time.Now())
	return err
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
	if err := fillMeetingReimbursementApprovalInfo(ctx, item, adminID); err != nil {
		return nil, err
	}
	return &m.DetailRes{ApplicationItem: item}, nil
}
func (s *service) ListMy(ctx context.Context, userID int64, page, pageSize int) (*m.ListRes, error) {
	normalizePage(&page, &pageSize)
	rows, total, err := s.repo.ListByUser(ctx, userID, page, pageSize)
	if err != nil {
		return nil, err
	}
	return s.listRes(ctx, rows, page, pageSize, total, 0)
}
func (s *service) List(ctx context.Context, req *ListReq) (*m.ListRes, error) {
	normalizePage(&req.Page, &req.PageSize)
	rows, total, err := s.repo.List(ctx, &repository.MeetingReimbursementListReq{UserId: req.UserId, WalletAddress: req.WalletAddress, Status: req.Status, Community: req.Community, Email: req.Email, Page: req.Page, PageSize: req.PageSize})
	if err != nil {
		return nil, err
	}
	return s.listRes(ctx, rows, req.Page, req.PageSize, total, req.AdminID)
}

func (s *service) CreateActivity(ctx context.Context, req *ActivitySaveReq) (int64, error) {
	saveReq, err := buildActivitySaveReq(ctx, req)
	if err != nil {
		return 0, err
	}
	return s.repo.CreateActivity(ctx, saveReq)
}

func (s *service) UpdateActivity(ctx context.Context, req *ActivitySaveReq) error {
	saveReq, err := buildActivitySaveReq(ctx, req)
	if err != nil {
		return err
	}
	return s.repo.UpdateActivity(ctx, saveReq)
}

func (s *service) ActivateActivity(ctx context.Context, id int64) error {
	return s.repo.ActivateActivity(ctx, id)
}

func (s *service) ListActivities(ctx context.Context, page, pageSize int) (*m.ActivityListRes, error) {
	normalizePage(&page, &pageSize)
	rows, total, err := s.repo.ListActivities(ctx, page, pageSize)
	if err != nil {
		return nil, err
	}
	list := make([]*m.ActivityItem, 0, len(rows))
	for _, row := range rows {
		list = append(list, convertActivity(row))
	}
	return &m.ActivityListRes{PageRes: frameModel.PageRes{Page: page, PageSize: pageSize, Total: total, Pages: (total + pageSize - 1) / pageSize}, List: list}, nil
}
func (s *service) listRes(ctx context.Context, rows []*repository.MeetingReimbursementRow, page, pageSize, total int, adminID int64) (*m.ListRes, error) {
	list := make([]*m.ApplicationItem, 0, len(rows))
	for _, r := range rows {
		item := convert(r)
		if err := fillMeetingReimbursementApprovalInfo(ctx, item, adminID); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	accepted, _ := s.repo.CountApproved(ctx)
	return &m.ListRes{PageRes: frameModel.PageRes{Page: page, PageSize: pageSize, Total: total, Pages: (total + pageSize - 1) / pageSize}, AcceptedCount: accepted, AcceptLimit: readIntConfig(ctx, configAcceptLimit, 99), List: list}, nil
}

type statRes struct{ small, min decimal.Decimal }

func (s *service) smallTeamPerformance(ctx context.Context, userID int64, activity *repository.MeetingReimbursementActivityRow) (decimal.Decimal, error) {
	code, err := getInviteCode(ctx, userID)
	if err != nil {
		return decimal.Zero, err
	}
	if code == "" {
		return decimal.Zero, nil
	}
	if activity != nil {
		return s.smallTeamPerformanceByPeriod(ctx, code, activity.StartAt, activity.EndAt)
	}
	agg, err := teamstats.NewTeamStatsService().GetOverviewAggByInviteCode(ctx, code)
	if err != nil {
		return decimal.Zero, err
	}
	if agg != nil {
		return agg.SmallTeamPerformance, nil
	}
	return decimal.Zero, nil
}

func (s *service) smallTeamPerformanceByPeriod(ctx context.Context, inviteCode string, startAt, endAt time.Time) (decimal.Decimal, error) {
	var r struct {
		SmallTeamPerformance string `json:"small_team_performance"`
	}
	sql := `
WITH RECURSIVE team_tree AS (
    SELECT u.id, u.invite_code, u.id AS root_id, 1 AS level
    FROM user_info u
    WHERE u.parent_invite_code = ?

    UNION ALL

    SELECT c.id, c.invite_code, t.root_id, t.level + 1
    FROM user_info c
    INNER JOIN team_tree t ON c.parent_invite_code = t.invite_code
),
branch_perf AS (
    SELECT
        t.root_id,
        COALESCE(SUM(o.amount), 0) AS perf
    FROM team_tree t
    LEFT JOIN staking_v2_order o ON o.user_id = t.id
        AND o.source_type = ?
        AND o.is_gift = 0
        AND o.created_at >= ?
        AND o.created_at <= ?
    GROUP BY t.root_id
)
SELECT
    COALESCE(SUM(perf), 0) - COALESCE(MAX(perf), 0) AS small_team_performance
FROM branch_perf
`
	if err := g.DB().Ctx(ctx).Raw(sql, inviteCode, consts.StakingV2SourceTypeManualStake, startAt, endAt).Scan(&r); err != nil {
		return decimal.Zero, err
	}
	return decimal.RequireFromString(r.SmallTeamPerformance), nil
}
func getInviteCode(ctx context.Context, userID int64) (string, error) {
	v, err := g.DB().Model("user_info").Ctx(ctx).Fields("invite_code").Where("id = ?", userID).Value()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(v.String()), nil
}
func convert(row *repository.MeetingReimbursementRow) *m.ApplicationItem {
	auditAt := ""
	if row.AuditAt != nil {
		auditAt = row.AuditAt.Format("2006-01-02 15:04:05")
	}
	return &m.ApplicationItem{Id: row.Id, ActivityId: row.ActivityId, ActivityTitle: row.ActivityTitle, UserId: row.UserId, WalletAddress: row.WalletAddress, Community: row.Community, Email: row.Email, ImageUrls: parseImages(row.ImageUrlsText), SmallTeamPerformance: utils.FormatDecimal(row.SmallTeamPerformance), Status: row.Status, ReimbursementAmount: utils.FormatDecimal(row.ReimbursementAmount), StakingV2OrderId: row.StakingV2OrderId, StakingV2Amount: utils.FormatDecimal(row.StakingV2Amount), StakingV2Status: row.StakingV2Status, CanAdjustStake: row.Status == repository.MeetingReimbursementApproved && row.StakingV2OrderId > 0 && row.StakingV2Status == consts.StakingV2StatusActive, AuditBy: row.AuditBy, AuditRemark: row.AuditRemark, AuditAt: auditAt, CreatedAt: row.CreatedAt.Format("2006-01-02 15:04:05"), UpdatedAt: row.UpdatedAt.Format("2006-01-02 15:04:05")}
}

func fillMeetingReimbursementApprovalInfo(ctx context.Context, item *m.ApplicationItem, adminID int64) error {
	if item == nil {
		return nil
	}
	approvalSvc := adminapproval.NewService()
	count, err := approvalSvc.GetApproveCount(ctx, adminapproval.BizTypeMeetingReimbursement.String(), item.Id)
	if err != nil {
		return err
	}
	item.ApprovalCount = count
	if adminID > 0 {
		approved, err := approvalSvc.HasAdminApproved(ctx, adminapproval.BizTypeMeetingReimbursement.String(), item.Id, adminID)
		if err != nil {
			return err
		}
		item.CurrentAdminApproved = approved
	}
	return nil
}

func convertActivity(row *repository.MeetingReimbursementActivityRow) *m.ActivityItem {
	if row == nil {
		return nil
	}
	return &m.ActivityItem{Id: row.Id, Title: row.Title, Content: row.Content, IsActive: row.IsActive, StartAt: row.StartAt.Format("2006-01-02 15:04:05"), EndAt: row.EndAt.Format("2006-01-02 15:04:05"), MinSmallTeamPerformance: utils.FormatDecimal(decimal.RequireFromString(row.MinSmallTeamPerformance)), MaxReimbursementAmount: utils.FormatDecimal(decimal.RequireFromString(row.MaxReimbursementAmount)), AcceptLimit: row.AcceptLimit, TotalReimbursementAmount: utils.FormatDecimal(decimal.RequireFromString(row.TotalReimbursementAmount)), PendingApplicationCount: row.PendingApplicationCount, ApprovedApplicationCount: row.ApprovedApplicationCount, CreatedAt: row.CreatedAt.Format("2006-01-02 15:04:05"), UpdatedAt: row.UpdatedAt.Format("2006-01-02 15:04:05")}
}
func parseImages(v string) []string {
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

func checkActivityPeriod(ctx context.Context, activity *repository.MeetingReimbursementActivityRow) error {
	now := time.Now()
	if now.Before(activity.StartAt) {
		return codeErr(ctx, 400, "not_started")
	}
	if now.After(activity.EndAt) {
		return codeErr(ctx, 400, "ended")
	}
	return nil
}

func buildActivitySaveReq(ctx context.Context, req *ActivitySaveReq) (*repository.SaveMeetingReimbursementActivityReq, error) {
	startAt, err := parseLocalTime(req.StartAt)
	if err != nil {
		return nil, gerror.NewCode(gcode.New(400, "", nil), "invalid start_at")
	}
	endAt, err := parseLocalTime(req.EndAt)
	if err != nil {
		return nil, gerror.NewCode(gcode.New(400, "", nil), "invalid end_at")
	}
	if !endAt.After(startAt) {
		return nil, gerror.NewCode(gcode.New(400, "", nil), "end_at must be after start_at")
	}
	if req.MinSmallTeamPerformance.IsNegative() || !req.MaxReimbursementAmount.GreaterThan(decimal.Zero) || req.AcceptLimit <= 0 {
		return nil, codeErr(ctx, 400, "amount_invalid")
	}
	return &repository.SaveMeetingReimbursementActivityReq{Id: req.Id, Title: strings.TrimSpace(req.Title), Content: strings.TrimSpace(req.Content), StartAt: startAt, EndAt: endAt, MinSmallTeamPerformance: req.MinSmallTeamPerformance, MaxReimbursementAmount: req.MaxReimbursementAmount, AcceptLimit: req.AcceptLimit}, nil
}

func parseLocalTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	layouts := []string{"2006-01-02 15:04:05", time.RFC3339, "2006-01-02T15:04:05"}
	var lastErr error
	for _, layout := range layouts {
		v, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			return v, nil
		}
		lastErr = err
	}
	return time.Time{}, lastErr
}
