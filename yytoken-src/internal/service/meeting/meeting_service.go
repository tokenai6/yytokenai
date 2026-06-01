package meeting

import (
	"context"
	"encoding/json"
	"net/mail"
	"strings"
	"time"

	"XWFrame/internal/frame/consts"
	"XWFrame/internal/frame/model"
	"XWFrame/internal/repository"
	"XWFrame/internal/service/adminapproval"
	coboSvc "XWFrame/internal/service/cobo"
	meetingModel "XWFrame/internal/service/meeting/model"
	"XWFrame/internal/service/teamstats"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

const meetingApplicationMinSmallTeamPerformanceKey = "meeting_application_min_small_team_performance"

// meetingErrMsgs 会议模块面向客户端的错误文案多语言表
// 新增 key 时必须覆盖全部 7 种语言：zh-CN / zh-TW / en-US / ja-JP / ko-KR / vi-VN / th-TH
var meetingErrMsgs = map[string]map[string]string{
	"invalid_meeting_time": {
		"zh-CN": "会议时间格式错误",
		"zh-TW": "會議時間格式錯誤",
		"en-US": "Invalid meeting time format",
		"ja-JP": "会議時間の形式が正しくありません",
		"ko-KR": "회의 시간 형식이 잘못되었습니다",
		"vi-VN": "Định dạng thờigian họp không hợp lệ",
		"th-TH": "รูปแบบเวลาประชุมไม่ถูกต้อง",
	},
	"user_not_logged_in": {
		"zh-CN": "请登录喔",
		"zh-TW": "請登入喔",
		"en-US": "Please log in",
		"ja-JP": "ログインしてください",
		"ko-KR": "로그인해 주세요",
		"vi-VN": "Vui lòng đăng nhập",
		"th-TH": "กรุณาเข้าสู่ระบบ",
	},
	"invalid_email": {
		"zh-CN": "邮箱格式错误",
		"zh-TW": "郵箱格式錯誤",
		"en-US": "Invalid email format",
		"ja-JP": "メールアドレスの形式が正しくありません",
		"ko-KR": "이메일 형식이 잘못되었습니다",
		"vi-VN": "Định dạng email không hợp lệ",
		"th-TH": "รูปแบบอีเมลไม่ถูกต้อง",
	},
	"meeting_images_exceed_limit": {
		"zh-CN": "会议图片最多5张",
		"zh-TW": "會議圖片最多5張",
		"en-US": "Meeting images cannot exceed 5",
		"ja-JP": "会議画像は最大5枚までです",
		"ko-KR": "회의 이미지는 최대 5장까지 가능합니다",
		"vi-VN": "Hình ảnh họp tối đa 5 ảnh",
		"th-TH": "รูปภาพประชุมสูงสุด 5 รูป",
	},
	"lecturer_count_required": {
		"zh-CN": "请输入讲师人数",
		"zh-TW": "請輸入講師人數",
		"en-US": "Lecturer count is required",
		"ja-JP": "講師人数を入力してください",
		"ko-KR": "강사 인원을 입력해 주세요",
		"vi-VN": "Vui lòng nhập số lượng giảng viên",
		"th-TH": "กรุณาระบุจำนวนวิทยากร",
	},
	"small_team_performance_not_met": {
		"zh-CN": "小区业绩未达到会议申请门槛",
		"zh-TW": "小區業績未達到會議申請門檻",
		"en-US": "Small team performance below threshold",
		"ja-JP": "小区業績が会議申請の条件を満たしていません",
		"ko-KR": "소구역 실적이 회의 신청 기준에 미달합니다",
		"vi-VN": "Hiệu suất nhóm nhỏ chưa đạt ngưỡng đăng ký họp",
		"th-TH": "ผลงานทีมย่อยไม่ถึงเกณฑ์การขอประชุม",
	},
	"invalid_approval_status": {
		"zh-CN": "审批状态不合法",
		"zh-TW": "審批狀態不合法",
		"en-US": "Invalid approval status",
		"ja-JP": "承認ステータスが正しくありません",
		"ko-KR": "승인 상태가 유효하지 않습니다",
		"vi-VN": "Trạng thái phê duyệt không hợp lệ",
		"th-TH": "สถานะการอนุมัติไม่ถูกต้อง",
	},
	"meeting_application_not_found": {
		"zh-CN": "会议申请不存在",
		"zh-TW": "會議申請不存在",
		"en-US": "Meeting application not found",
		"ja-JP": "会議申請が存在しません",
		"ko-KR": "회의 신청을 찾을 수 없습니다",
		"vi-VN": "Không tìm thấy đơn đăng ký họp",
		"th-TH": "ไม่พบการขอประชุม",
	},
	"meeting_already_audited": {
		"zh-CN": "会议申请已审批",
		"zh-TW": "會議申請已審批",
		"en-US": "Meeting application already audited",
		"ja-JP": "会議申請は既に審査済みです",
		"ko-KR": "회의 신청이 이미 심사되었습니다",
		"vi-VN": "Đơn đăng ký họp đã được phê duyệt",
		"th-TH": "การขอประชุมได้รับการอนุมัติแล้ว",
	},
	"invalid_threshold_config": {
		"zh-CN": "会议申请门槛配置异常",
		"zh-TW": "會議申請門檻配置異常",
		"en-US": "Invalid meeting application threshold config",
		"ja-JP": "会議申請条件の設定が正しくありません",
		"ko-KR": "회의 신청 기준 설정이 잘못되었습니다",
		"vi-VN": "Cấu hình ngưỡng đăng ký họp không hợp lệ",
		"th-TH": "การตั้งค่าเกณฑ์การขอประชุมไม่ถูกต้อง",
	},
	"reimbursement_amount_exceeds_limit": {
		"zh-CN": "报销额度超过上限",
		"zh-TW": "報銷額度超過上限",
		"en-US": "Reimbursement amount exceeds the limit",
		"ja-JP": "経費限度額を超えています",
		"ko-KR": "비용 상한을 초과했습니다",
		"vi-VN": "Số tiền hoàn trả vượt quá giới hạn",
		"th-TH": "วงเงินเบิกจ่ายเกินกำหนด",
	},
	"meeting_not_approved": {
		"zh-CN": "会议未通过，无法验收",
		"zh-TW": "會議未通過，無法驗收",
		"en-US": "Meeting not approved, cannot confirm acceptance",
		"ja-JP": "会議が承認されていないため、検収できません",
		"ko-KR": "회의가 승인되지 않아 검수할 수 없습니다",
		"vi-VN": "Cuộc họp chưa được phê duyệt, không thể nghiệm thu",
		"th-TH": "การประชุมยังไม่ผ่านการอนุมัติ ไม่สามารถยืนยันการรับมอบได้",
	},
	"meeting_already_accepted": {
		"zh-CN": "已验收，无需重复操作",
		"zh-TW": "已驗收，無需重複操作",
		"en-US": "Already accepted, no need to repeat",
		"ja-JP": "既に検収済みです",
		"ko-KR": "이미 검수되었습니다",
		"vi-VN": "Đã nghiệm thu, không cần thao tác lại",
		"th-TH": "รับมอบแล้ว ไม่ต้องดำเนินการซ้ำ",
	},
	"reimbursement_transfer_not_found": {
		"zh-CN": "报销转账记录不存在",
		"zh-TW": "報銷轉賬記錄不存在",
		"en-US": "Reimbursement transfer record not found",
		"ja-JP": "経費振替記録が存在しません",
		"ko-KR": "비용 이체 기록을 찾을 수 없습니다",
		"vi-VN": "Không tìm thấy bản ghi chuyển tiền hoàn trả",
		"th-TH": "ไม่พบรายการโอนเงินเบิกจ่าย",
	},
	"invalid_reimbursement_amount": {
		"zh-CN": "报销额度不合法",
		"zh-TW": "報銷額度不合法",
		"en-US": "Invalid reimbursement amount",
		"ja-JP": "経費金額が正しくありません",
		"ko-KR": "비용 금액이 유효하지 않습니다",
		"vi-VN": "Số tiền hoàn trả không hợp lệ",
		"th-TH": "วงเงินเบิกจ่ายไม่ถูกต้อง",
	},
	"meeting_cannot_delete_approved": {
		"zh-CN": "已审批的会议申请不可删除",
		"zh-TW": "已審批的會議申請不可刪除",
		"en-US": "Approved or rejected meeting applications cannot be deleted",
		"ja-JP": "承認済みの会議申請は削除できません",
		"ko-KR": "승인된 회의 신청은 삭제할 수 없습니다",
		"vi-VN": "Không thể xóa đơn đăng ký họp đã được phê duyệt",
		"th-TH": "ไม่สามารถลบการขอประชุมที่ได้รับการอนุมัติแล้ว",
	},
	"meeting_material_requires_approved": {
		"zh-CN": "初次审批通过后才能上传会议资料",
		"zh-TW": "初次審批通過後才能上傳會議資料",
		"en-US": "Meeting materials can be uploaded only after initial approval",
		"ja-JP": "初回承認後にのみ会議資料をアップロードできます",
		"ko-KR": "1차 승인 후에만 회의 자료를 업로드할 수 있습니다",
		"vi-VN": "Chỉ có thể tải tài liệu cuộc họp lên sau khi phê duyệt lần đầu",
		"th-TH": "อัปโหลดเอกสารการประชุมได้หลังผ่านการอนุมัติครั้งแรกเท่านั้น",
	},
	"meeting_material_images_required": {
		"zh-CN": "请上传完整会议资料图片",
		"zh-TW": "請上傳完整會議資料圖片",
		"en-US": "Please upload all required meeting material images",
		"ja-JP": "必要な会議資料画像をすべてアップロードしてください",
		"ko-KR": "필수 회의 자료 이미지를 모두 업로드해 주세요",
		"vi-VN": "Vui lòng tải lên đầy đủ hình ảnh tài liệu cuộc họp",
		"th-TH": "กรุณาอัปโหลดรูปภาพเอกสารการประชุมให้ครบถ้วน",
	},
	"meeting_material_not_pending": {
		"zh-CN": "会议资料不在待审核状态",
		"zh-TW": "會議資料不在待審核狀態",
		"en-US": "Meeting materials are not pending review",
		"ja-JP": "会議資料は審査待ち状態ではありません",
		"ko-KR": "회의 자료가 심사 대기 상태가 아닙니다",
		"vi-VN": "Tài liệu cuộc họp không ở trạng thái chờ duyệt",
		"th-TH": "เอกสารการประชุมไม่ได้อยู่ในสถานะรอตรวจสอบ",
	},
	"meeting_material_already_approved": {
		"zh-CN": "会议资料已审核通过，无需重复提交",
		"zh-TW": "會議資料已審核通過，無需重複提交",
		"en-US": "Meeting materials already approved, no need to resubmit",
		"ja-JP": "会議資料は既に承認済みです",
		"ko-KR": "회의 자료가 이미 승인되었습니다",
		"vi-VN": "Tài liệu cuộc họp đã được phê duyệt",
		"th-TH": "เอกสารการประชุมได้รับการอนุมัติแล้ว",
	},
	"already_approved_by_admin": {
		"zh-CN": "您已审批过，不能重复操作",
		"zh-TW": "您已審批過，不能重複操作",
		"en-US": "You have already approved this application",
		"ja-JP": "この申請は既に承認済みです",
		"ko-KR": "이미 승인한 신청입니다",
		"vi-VN": "Bạn đã phê duyệt đơn này",
		"th-TH": "คุณได้อนุมัติคำขอนี้แล้ว",
	},
}

func meetingErr(ctx context.Context, key string) error {
	locale := consts.LocaleFromCtx(ctx)
	m := meetingErrMsgs[key]
	msg := consts.LocalizedText(m, locale)
	if msg == "" {
		return gerror.New(key)
	}
	return gerror.New(msg)
}

type Service interface {
	Apply(ctx context.Context, req *ApplyMeetingReq) (int64, error)
	SubmitMaterials(ctx context.Context, req *SubmitMeetingMaterialsReq) (int64, error)
	GetApplicationConfig(ctx context.Context, userID int64) (*meetingModel.MeetingApplicationConfigRes, error)
	Audit(ctx context.Context, req *AuditMeetingReq) error
	AuditMaterials(ctx context.Context, req *AuditMeetingReq) error
	ConfirmAcceptance(ctx context.Context, meetingApplicationID, adminID int64) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64, adminID int64) (*meetingModel.MeetingApplicationDetailRes, error)
	List(ctx context.Context, req *ListMeetingReq) (*meetingModel.MeetingApplicationListRes, error)
	ListMy(ctx context.Context, userID int64, page int, pageSize int) (*meetingModel.MeetingApplicationListRes, error)
}

type service struct {
	repo           repository.MeetingRepository
	directTransfer coboSvc.DirectTransferService
}

func NewService() Service {
	return &service{
		repo:           repository.NewMeetingRepository(),
		directTransfer: coboSvc.NewDirectTransferService(),
	}
}

type ApplyMeetingReq struct {
	UserId              int64
	Title               string
	MeetingTime         string
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
	Remark              string
}

type AuditMeetingReq struct {
	Id                  int64
	AdminID             int64
	Status              int
	AuditRemark         string
	ReimbursementAmount decimal.Decimal
}

type SubmitMeetingMaterialsReq struct {
	UserId int64
	Images *meetingModel.MeetingMaterialImages
}

type ListMeetingReq struct {
	AdminID   int64
	UserId    int64
	Title     string
	Status    int
	Community string
	Email     string
	Page      int
	PageSize  int
}

func (s *service) Apply(ctx context.Context, req *ApplyMeetingReq) (int64, error) {
	meetingTime, err := parseMeetingTime(req.MeetingTime)
	if err != nil {
		return 0, gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "invalid_meeting_time").Error())
	}
	if req.UserId <= 0 {
		return 0, gerror.NewCode(gcode.New(401, "", nil), meetingErr(ctx, "user_not_logged_in").Error())
	}
	if _, err := mail.ParseAddress(strings.TrimSpace(req.Email)); err != nil {
		return 0, gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "invalid_email").Error())
	}
	if len(req.ImageUrls) > 5 {
		return 0, gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "meeting_images_exceed_limit").Error())
	}
	if !req.SupportLecturer {
		req.LecturerCount = 0
	} else if req.LecturerCount <= 0 {
		return 0, gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "lecturer_count_required").Error())
	}
	if req.ReimbursementImages == nil {
		req.ReimbursementImages = &meetingModel.ReimbursementImages{}
	}
	stats, err := s.getApplicationStats(ctx, req.UserId)
	if err != nil {
		return 0, err
	}
	if stats.SmallTeamPerformance.LessThan(stats.MinSmallTeamPerformance) {
		return 0, gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "small_team_performance_not_met").Error())
	}
	createReq := &repository.CreateMeetingApplicationReq{
		UserId:              req.UserId,
		Title:               strings.TrimSpace(req.Title),
		MeetingTime:         meetingTime,
		Address:             strings.TrimSpace(req.Address),
		ExpectedPeople:      req.ExpectedPeople,
		SupportLecturer:     req.SupportLecturer,
		LecturerCount:       req.LecturerCount,
		Community:           strings.TrimSpace(req.Community),
		OrganizerName:       strings.TrimSpace(req.OrganizerName),
		OrganizerPhone:      strings.TrimSpace(req.OrganizerPhone),
		Email:               strings.TrimSpace(req.Email),
		ImageUrls:           req.ImageUrls,
		ReimbursementImages: req.ReimbursementImages,
	}
	return s.repo.UpsertApplicationByUser(ctx, createReq)
}

func (s *service) SubmitMaterials(ctx context.Context, req *SubmitMeetingMaterialsReq) (int64, error) {
	if req.UserId <= 0 {
		return 0, gerror.NewCode(gcode.New(401, "", nil), meetingErr(ctx, "user_not_logged_in").Error())
	}
	if req.Images == nil || !hasAllMaterialImages(req.Images) {
		return 0, gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "meeting_material_images_required").Error())
	}
	currentList, _, err := s.repo.List(ctx, &repository.MeetingApplicationListReq{UserId: req.UserId, Page: 1, PageSize: 1})
	if err != nil {
		return 0, err
	}
	if len(currentList) == 0 {
		return 0, gerror.NewCode(gcode.New(404, "", nil), meetingErr(ctx, "meeting_application_not_found").Error())
	}
	current := currentList[0]
	if current.ApprovalStatus != repository.MeetingApprovalApproved {
		return 0, gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "meeting_material_requires_approved").Error())
	}
	if current.MaterialStatus == repository.MeetingMaterialApproved {
		return 0, gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "meeting_material_already_approved").Error())
	}
	if current.AcceptanceStatus == 1 {
		return 0, gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "meeting_already_accepted").Error())
	}
	return s.repo.SubmitMaterials(ctx, req.UserId, req.Images)
}

func (s *service) GetApplicationConfig(ctx context.Context, userID int64) (*meetingModel.MeetingApplicationConfigRes, error) {
	stats, err := s.getApplicationStats(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &meetingModel.MeetingApplicationConfigRes{
		SmallTeamPerformance:    utils.FormatDecimal(stats.SmallTeamPerformance),
		MinSmallTeamPerformance: utils.FormatDecimal(stats.MinSmallTeamPerformance),
		CanApply:                !stats.SmallTeamPerformance.LessThan(stats.MinSmallTeamPerformance),
	}, nil
}

func (s *service) Audit(ctx context.Context, req *AuditMeetingReq) error {
	if req.Status != repository.MeetingApprovalApproved && req.Status != repository.MeetingApprovalRejected {
		return gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "invalid_approval_status").Error())
	}
	current, err := s.repo.GetById(ctx, req.Id)
	if err != nil {
		return err
	}
	if current == nil {
		return gerror.NewCode(gcode.New(404, "", nil), meetingErr(ctx, "meeting_application_not_found").Error())
	}

	approvalSvc := adminapproval.NewService()
	bizType := adminapproval.BizTypeMeeting.String()

	// 拒绝操作
	if req.Status == repository.MeetingApprovalRejected {
		return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
			currentTx, err := s.repo.GetByIdForUpdate(ctx, tx, req.Id)
			if err != nil {
				return err
			}
			if currentTx == nil {
				return gerror.NewCode(gcode.New(404, "", nil), meetingErr(ctx, "meeting_application_not_found").Error())
			}
			if currentTx.ApprovalStatus != repository.MeetingApprovalPending {
				return gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "meeting_already_audited").Error())
			}
			if err := s.repo.AuditTx(ctx, tx, &repository.AuditMeetingApplicationReq{
				Id:          req.Id,
				AdminID:     req.AdminID,
				Status:      req.Status,
				AuditRemark: strings.TrimSpace(req.AuditRemark),
			}); err != nil {
				return err
			}
			return approvalSvc.RecordApprovalTx(ctx, tx, bizType, req.Id, req.AdminID, adminapproval.ActionReject, strings.TrimSpace(req.AuditRemark))
		})
	}

	// 通过操作
	if current.ApprovalStatus == repository.MeetingApprovalRejected {
		return gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "meeting_already_audited").Error())
	}

	reimbursementAmount := req.ReimbursementAmount
	if current.ReimbursementAmount.GreaterThan(decimal.Zero) {
		reimbursementAmount = current.ReimbursementAmount
	}
	// 没有报销金额：直接通过，不需要多级审批转账
	if !reimbursementAmount.GreaterThan(decimal.Zero) {
		if current.ApprovalStatus != repository.MeetingApprovalPending {
			return gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "meeting_already_audited").Error())
		}
		return s.repo.Audit(ctx, &repository.AuditMeetingApplicationReq{
			Id:          req.Id,
			AdminID:     req.AdminID,
			Status:      req.Status,
			AuditRemark: strings.TrimSpace(req.AuditRemark),
		})
	}

	maxAmount, err := getMaxReimbursementAmount(ctx)
	if err != nil {
		return err
	}
	if reimbursementAmount.GreaterThan(maxAmount) {
		return gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "reimbursement_amount_exceeds_limit").Error())
	}

	firstAmount := reimbursementAmount.Mul(decimal.NewFromFloat(0.5)).Round(8)
	secondAmount := reimbursementAmount.Sub(firstAmount)
	shouldSubmitFirst := false
	var firstTransferID int64
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		currentTx, err := s.repo.GetByIdForUpdate(ctx, tx, req.Id)
		if err != nil {
			return err
		}
		if currentTx == nil {
			return gerror.NewCode(gcode.New(404, "", nil), meetingErr(ctx, "meeting_application_not_found").Error())
		}

		// 检查当前管理员是否已审批
		hasApproved, err := approvalSvc.HasAdminApprovedTx(ctx, tx, bizType, req.Id, req.AdminID)
		if err != nil {
			return err
		}
		if hasApproved {
			return gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "already_approved_by_admin").Error())
		}

		// 查询已有 approve 记录数
		approveCount, err := approvalSvc.GetApproveCountTx(ctx, tx, bizType, req.Id)
		if err != nil {
			return err
		}

		// 记录审批
		if currentTx.ReimbursementAmount.GreaterThan(decimal.Zero) {
			reimbursementAmount = currentTx.ReimbursementAmount
		}
		if currentTx.ApprovalStatus == repository.MeetingApprovalPending && reimbursementAmount.GreaterThan(decimal.Zero) {
			if err := s.repo.UpdateReimbursementAmountTx(ctx, tx, req.Id, reimbursementAmount); err != nil {
				return err
			}
		}
		if err := approvalSvc.RecordApprovalTx(ctx, tx, bizType, req.Id, req.AdminID, adminapproval.ActionApprove, strings.TrimSpace(req.AuditRemark)); err != nil {
			return err
		}

		// 第二个管理员同意后，才更新业务状态并创建第一批转账任务
		if approveCount >= 1 && currentTx.ApprovalStatus == repository.MeetingApprovalPending {
			if err := s.repo.AuditTx(ctx, tx, &repository.AuditMeetingApplicationReq{
				Id:                  req.Id,
				AdminID:             req.AdminID,
				Status:              req.Status,
				AuditRemark:         strings.TrimSpace(req.AuditRemark),
				ReimbursementAmount: reimbursementAmount,
			}); err != nil {
				return err
			}

			transferID, err := s.repo.CreateReimbursementTransfer(ctx, tx, &repository.CreateReimbursementTransferReq{
				MeetingApplicationId: req.Id,
				UserId:               currentTx.UserId,
				Symbol:               "USDT",
				TotalAmount:          reimbursementAmount,
				FirstAmount:          firstAmount,
				SecondAmount:         secondAmount,
			})
			if err != nil {
				return err
			}
			if _, err := s.directTransfer.EnsureTransferOrderTx(ctx, tx, &coboSvc.EnsureTransferOrderReq{BizType: bizType, BizID: req.Id, Phase: "first", UserID: currentTx.UserId, Symbol: "USDT", Amount: firstAmount, Description: "会议报销第一批（50%）"}); err != nil {
				return err
			}
			firstTransferID = transferID
			shouldSubmitFirst = true
		}

		return nil
	})
	if err != nil {
		return err
	}

	// 满足至少两个不同管理员 approve 时才调用 Cobo 转账第一批
	if shouldSubmitFirst {
		order, transferErr := s.directTransfer.SubmitTransferOrder(ctx, bizType, req.Id, "first")
		if transferErr != nil {
			g.Log().Errorf(ctx, "[Meeting] first direct transfer failed: id=%d, err=%v", req.Id, transferErr)
			return transferErr
		}
		if firstTransferID > 0 {
			if err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
				return s.repo.UpdateReimbursementTransferFirst(ctx, tx, firstTransferID, order.RequestID, time.Now())
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *service) AuditMaterials(ctx context.Context, req *AuditMeetingReq) error {
	if req.Status != repository.MeetingMaterialApproved && req.Status != repository.MeetingMaterialRejected {
		return gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "invalid_approval_status").Error())
	}

	current, err := s.repo.GetById(ctx, req.Id)
	if err != nil {
		return err
	}
	if current == nil {
		return gerror.NewCode(gcode.New(404, "", nil), meetingErr(ctx, "meeting_application_not_found").Error())
	}

	approvalSvc := adminapproval.NewService()
	bizType := "meeting_second" // 第二批使用独立的 biz_type

	// 拒绝操作
	if req.Status == repository.MeetingMaterialRejected {
		return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
			currentTx, err := s.repo.GetByIdForUpdate(ctx, tx, req.Id)
			if err != nil {
				return err
			}
			if currentTx == nil {
				return gerror.NewCode(gcode.New(404, "", nil), meetingErr(ctx, "meeting_application_not_found").Error())
			}
			if currentTx.MaterialStatus != repository.MeetingMaterialPending {
				return gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "meeting_material_not_pending").Error())
			}
			if err := s.repo.AuditMaterialsTx(ctx, tx, &repository.AuditMeetingApplicationReq{Id: req.Id, AdminID: req.AdminID, Status: req.Status, AuditRemark: strings.TrimSpace(req.AuditRemark)}); err != nil {
				return err
			}
			return approvalSvc.RecordApprovalTx(ctx, tx, bizType, req.Id, req.AdminID, adminapproval.ActionReject, strings.TrimSpace(req.AuditRemark))
		})
	}

	// 通过操作
	if current.MaterialStatus == repository.MeetingMaterialRejected {
		return gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "meeting_material_not_pending").Error())
	}
	if current.ApprovalStatus != repository.MeetingApprovalApproved {
		return gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "meeting_material_requires_approved").Error())
	}

	shouldSubmitSecond := false
	var secondTransferID int64
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		currentTx, err := s.repo.GetByIdForUpdate(ctx, tx, req.Id)
		if err != nil {
			return err
		}
		if currentTx == nil {
			return gerror.NewCode(gcode.New(404, "", nil), meetingErr(ctx, "meeting_application_not_found").Error())
		}
		if currentTx.ApprovalStatus != repository.MeetingApprovalApproved {
			return gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "meeting_material_requires_approved").Error())
		}
		if currentTx.MaterialStatus != repository.MeetingMaterialPending {
			return gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "meeting_material_not_pending").Error())
		}

		// 检查当前管理员是否已审批
		hasApproved, err := approvalSvc.HasAdminApprovedTx(ctx, tx, bizType, req.Id, req.AdminID)
		if err != nil {
			return err
		}
		if hasApproved {
			return gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "already_approved_by_admin").Error())
		}

		// 查询已有 approve 记录数
		approveCount, err := approvalSvc.GetApproveCountTx(ctx, tx, bizType, req.Id)
		if err != nil {
			return err
		}

		if err := s.repo.AuditMaterialsTx(ctx, tx, &repository.AuditMeetingApplicationReq{Id: req.Id, AdminID: req.AdminID, Status: req.Status, AuditRemark: strings.TrimSpace(req.AuditRemark)}); err != nil {
			return err
		}

		if currentTx.AcceptanceStatus != 0 {
			// 记录审批但不转账
			return approvalSvc.RecordApprovalTx(ctx, tx, bizType, req.Id, req.AdminID, adminapproval.ActionApprove, strings.TrimSpace(req.AuditRemark))
		}

		transfer, err := s.repo.GetReimbursementTransferByMeetingID(ctx, req.Id)
		if err != nil {
			return err
		}
		if transfer == nil || transfer.Status != 1 {
			if err := s.repo.UpdateMeetingAcceptanceStatus(ctx, tx, req.Id, 1); err != nil {
				return err
			}
			return approvalSvc.RecordApprovalTx(ctx, tx, bizType, req.Id, req.AdminID, adminapproval.ActionApprove, strings.TrimSpace(req.AuditRemark))
		}

		// 记录审批
		if err := approvalSvc.RecordApprovalTx(ctx, tx, bizType, req.Id, req.AdminID, adminapproval.ActionApprove, strings.TrimSpace(req.AuditRemark)); err != nil {
			return err
		}

		// 如果已有其他管理员 approve，触发第二批转账
		if approveCount >= 1 {
			if _, err := s.directTransfer.EnsureTransferOrderTx(ctx, tx, &coboSvc.EnsureTransferOrderReq{BizType: bizType, BizID: req.Id, Phase: "second", UserID: currentTx.UserId, Symbol: "USDT", Amount: transfer.SecondAmount, Description: "会议报销第二批（50%）"}); err != nil {
				return err
			}
			secondTransferID = transfer.Id
			shouldSubmitSecond = true
		}

		return nil
	})
	if err != nil {
		return err
	}

	// 满足至少两个不同管理员 approve 时才调用 Cobo 转账第二批
	if shouldSubmitSecond {
		order, transferErr := s.directTransfer.SubmitTransferOrder(ctx, bizType, req.Id, "second")
		if transferErr != nil {
			g.Log().Errorf(ctx, "[Meeting] second direct transfer failed: id=%d, err=%v", req.Id, transferErr)
			return transferErr
		}
		if err := s.finalizeSecondReimbursement(ctx, req.Id, secondTransferID, order.RequestID); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) Get(ctx context.Context, id int64, adminID int64) (*meetingModel.MeetingApplicationDetailRes, error) {
	row, err := s.repo.GetById(ctx, id)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, gerror.NewCode(gcode.New(404, "", nil), meetingErr(ctx, "meeting_application_not_found").Error())
	}
	item, err := s.convertMeeting(ctx, row, adminID)
	if err != nil {
		return nil, err
	}
	return &meetingModel.MeetingApplicationDetailRes{MeetingApplicationItem: item}, nil
}

func (s *service) List(ctx context.Context, req *ListMeetingReq) (*meetingModel.MeetingApplicationListRes, error) {
	normalizePage(&req.Page, &req.PageSize)
	rows, total, err := s.repo.List(ctx, &repository.MeetingApplicationListReq{UserId: req.UserId, Title: req.Title, Status: req.Status, Community: req.Community, Email: req.Email, Page: req.Page, PageSize: req.PageSize})
	if err != nil {
		return nil, err
	}
	return s.listRes(ctx, rows, req.Page, req.PageSize, total, req.AdminID)
}

func (s *service) ListMy(ctx context.Context, userID int64, page int, pageSize int) (*meetingModel.MeetingApplicationListRes, error) {
	normalizePage(&page, &pageSize)
	rows, total, err := s.repo.ListByUser(ctx, userID, page, pageSize)
	if err != nil {
		return nil, err
	}
	return s.listRes(ctx, rows, page, pageSize, total, 0)
}

type applicationStats struct {
	SmallTeamPerformance    decimal.Decimal
	MinSmallTeamPerformance decimal.Decimal
}

func (s *service) getApplicationStats(ctx context.Context, userID int64) (*applicationStats, error) {
	inviteCode, err := getUserInviteCode(ctx, userID)
	if err != nil {
		return nil, err
	}
	smallTeamPerformance := decimal.Zero
	if inviteCode != "" {
		agg, err := teamstats.NewTeamStatsService().GetOverviewAggByInviteCode(ctx, inviteCode)
		if err != nil {
			return nil, err
		}
		if agg != nil {
			smallTeamPerformance = agg.SmallTeamPerformance
		}
	}
	minSmallTeamPerformance, err := getMeetingApplicationMinSmallTeamPerformance(ctx)
	if err != nil {
		return nil, err
	}
	return &applicationStats{SmallTeamPerformance: smallTeamPerformance, MinSmallTeamPerformance: minSmallTeamPerformance}, nil
}

func getUserInviteCode(ctx context.Context, userID int64) (string, error) {
	value, err := g.DB().Model("user_info").Ctx(ctx).
		Fields("invite_code").
		Where("id = ?", userID).
		Value()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value.String()), nil
}

func getMeetingApplicationMinSmallTeamPerformance(ctx context.Context) (decimal.Decimal, error) {
	value, err := g.DB().Model("system_config").Ctx(ctx).
		Fields("value").
		Where("key = ?", meetingApplicationMinSmallTeamPerformanceKey).
		Value()
	if err != nil {
		return decimal.Zero, err
	}
	if strings.TrimSpace(value.String()) == "" {
		return decimal.NewFromInt(5000), nil
	}
	threshold, err := decimal.NewFromString(strings.TrimSpace(value.String()))
	if err != nil {
		return decimal.Zero, gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "invalid_threshold_config").Error())
	}
	return threshold, nil
}

func parseMeetingTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, gerror.New("empty time")
	}
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

func (s *service) convertMeeting(ctx context.Context, row *repository.MeetingApplicationRow, adminID int64) (*meetingModel.MeetingApplicationItem, error) {
	auditAt := ""
	if row.AuditAt != nil {
		auditAt = row.AuditAt.Format("2006-01-02 15:04:05")
	}
	smallTeamPerformance := "0"
	if strings.TrimSpace(row.InviteCode) != "" {
		agg, err := teamstats.NewTeamStatsService().GetOverviewAggByInviteCode(ctx, row.InviteCode)
		if err != nil {
			return nil, err
		}
		if agg != nil {
			smallTeamPerformance = utils.FormatDecimal(agg.SmallTeamPerformance)
		}
	}
	item := &meetingModel.MeetingApplicationItem{
		Id:                   row.Id,
		UserId:               row.UserId,
		WalletAddress:        row.WalletAddress,
		SmallTeamPerformance: smallTeamPerformance,
		Title:                row.Title,
		MeetingTime:          row.MeetingTime.Format("2006-01-02 15:04:05"),
		Address:              row.Address,
		ExpectedPeople:       row.ExpectedPeople,
		SupportLecturer:      row.SupportLecturer,
		LecturerCount:        row.LecturerCount,
		Community:            row.Community,
		OrganizerName:        row.OrganizerName,
		OrganizerPhone:       row.OrganizerPhone,
		Email:                row.Email,
		ImageUrls:            parseStringSlice(row.ImageUrlsText),
		ReimbursementImages:  parseReimbursementImages(row.ReimbursementImagesText),
		MaterialImages:       parseMaterialImages(row.MaterialImagesText),
		MaterialStatus:       row.MaterialStatus,
		MaterialAuditBy:      row.MaterialAuditBy,
		MaterialAuditRemark:  row.MaterialAuditRemark,
		ApprovalStatus:       row.ApprovalStatus,
		ReimbursementAmount:  utils.FormatDecimal(row.ReimbursementAmount),
		AcceptanceStatus:     row.AcceptanceStatus,
		AuditBy:              row.AuditBy,
		AuditRemark:          row.AuditRemark,
		AuditAt:              auditAt,
		CreatedAt:            row.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:            row.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
	approvalSvc := adminapproval.NewService()
	approvalCount, err := approvalSvc.GetApproveCount(ctx, adminapproval.BizTypeMeeting.String(), row.Id)
	if err != nil {
		return nil, err
	}
	item.ApprovalCount = approvalCount
	if adminID > 0 {
		approved, err := approvalSvc.HasAdminApproved(ctx, adminapproval.BizTypeMeeting.String(), row.Id, adminID)
		if err != nil {
			return nil, err
		}
		item.CurrentAdminApproved = approved
	}
	if row.MaterialAuditAt != nil {
		item.MaterialAuditAt = row.MaterialAuditAt.Format("2006-01-02 15:04:05")
	}
	if row.MaterialSubmittedAt != nil {
		item.MaterialSubmittedAt = row.MaterialSubmittedAt.Format("2006-01-02 15:04:05")
	}

	if row.ReimbursementAmount.GreaterThan(decimal.Zero) {
		transfer, err := s.repo.GetReimbursementTransferByMeetingID(ctx, row.Id)
		if err != nil {
			return nil, err
		}
		if transfer != nil {
			firstAt := ""
			if transfer.FirstTransferredAt != nil {
				firstAt = transfer.FirstTransferredAt.Format("2006-01-02 15:04:05")
			}
			secondAt := ""
			if transfer.SecondTransferredAt != nil {
				secondAt = transfer.SecondTransferredAt.Format("2006-01-02 15:04:05")
			}
			item.ReimbursementTransfer = &meetingModel.ReimbursementTransferItem{
				TotalAmount:         utils.FormatDecimal(transfer.TotalAmount),
				FirstAmount:         utils.FormatDecimal(transfer.FirstAmount),
				SecondAmount:        utils.FormatDecimal(transfer.SecondAmount),
				FirstTransferredAt:  firstAt,
				SecondTransferredAt: secondAt,
				Status:              transfer.Status,
				FirstOrderNo:        transfer.FirstOrderNo,
				SecondOrderNo:       transfer.SecondOrderNo,
			}
		}
	}

	// 设置前端状态字段
	item.FirstAuditStatus = firstAuditStatusText(row.ApprovalStatus)
	item.SecondAuditStatus = secondAuditStatusText(row.MaterialStatus)

	return item, nil
}

func firstAuditStatusText(status int) string {
	switch status {
	case repository.MeetingApprovalPending:
		return "pending"
	case repository.MeetingApprovalApproved:
		return "approved"
	case repository.MeetingApprovalRejected:
		return "rejected"
	default:
		return "unknown"
	}
}

func secondAuditStatusText(status int) string {
	switch status {
	case repository.MeetingMaterialNotSubmitted:
		return "not_submitted"
	case repository.MeetingMaterialPending:
		return "pending"
	case repository.MeetingMaterialApproved:
		return "approved"
	case repository.MeetingMaterialRejected:
		return "rejected"
	default:
		return "unknown"
	}
}

func (s *service) listRes(ctx context.Context, rows []*repository.MeetingApplicationRow, page int, pageSize int, total int, adminID int64) (*meetingModel.MeetingApplicationListRes, error) {
	list := make([]*meetingModel.MeetingApplicationItem, 0, len(rows))
	for _, row := range rows {
		item, err := s.convertMeeting(ctx, row, adminID)
		if err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return &meetingModel.MeetingApplicationListRes{PageRes: pageRes(page, pageSize, total), List: list}, nil
}

func parseStringSlice(value string) []string {
	var list []string
	if value == "" || json.Unmarshal([]byte(value), &list) != nil {
		return []string{}
	}
	return list
}

func parseReimbursementImages(value string) *meetingModel.ReimbursementImages {
	images := &meetingModel.ReimbursementImages{}
	if value == "" {
		return images
	}
	_ = json.Unmarshal([]byte(value), images)
	return images
}

func parseMaterialImages(value string) *meetingModel.MeetingMaterialImages {
	images := &meetingModel.MeetingMaterialImages{}
	if value == "" {
		return images
	}
	_ = json.Unmarshal([]byte(value), images)
	return images
}

func hasAllMaterialImages(images *meetingModel.MeetingMaterialImages) bool {
	return len(images.Venue) > 0 &&
		len(images.CheckIn) > 0 &&
		len(images.Host) > 0 &&
		len(images.Lecture) > 0 &&
		len(images.Applause) > 0 &&
		len(images.QA) > 0 &&
		len(images.Dinner) > 0 &&
		len(images.Deal) > 0 &&
		len(images.Bill) > 0
}

func normalizePage(page *int, pageSize *int) {
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

func pageRes(page int, pageSize int, total int) model.PageRes {
	return model.PageRes{Page: page, PageSize: pageSize, Total: total, Pages: (total + pageSize - 1) / pageSize}
}

func (s *service) Delete(ctx context.Context, id int64) error {
	current, err := s.repo.GetById(ctx, id)
	if err != nil {
		return err
	}
	if current == nil {
		return gerror.NewCode(gcode.New(404, "", nil), meetingErr(ctx, "meeting_application_not_found").Error())
	}
	if current.ApprovalStatus != repository.MeetingApprovalPending {
		return gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "meeting_cannot_delete_approved").Error())
	}
	return s.repo.Delete(ctx, id)
}

func (s *service) ConfirmAcceptance(ctx context.Context, meetingApplicationID, adminID int64) error {
	current, err := s.repo.GetById(ctx, meetingApplicationID)
	if err != nil {
		return err
	}
	if current == nil {
		return gerror.NewCode(gcode.New(404, "", nil), meetingErr(ctx, "meeting_application_not_found").Error())
	}
	if current.ApprovalStatus != repository.MeetingApprovalApproved {
		return gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "meeting_not_approved").Error())
	}
	if current.AcceptanceStatus != 0 {
		return gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "meeting_already_accepted").Error())
	}

	transfer, err := s.repo.GetReimbursementTransferByMeetingID(ctx, meetingApplicationID)
	if err != nil {
		return err
	}
	if transfer == nil {
		return gerror.NewCode(gcode.New(404, "", nil), meetingErr(ctx, "reimbursement_transfer_not_found").Error())
	}
	if transfer.Status != 1 {
		return gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "meeting_already_accepted").Error())
	}

	approvalSvc := adminapproval.NewService()
	bizType := "meeting_second"

	shouldSubmitSecond := false
	var secondTransferID int64

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		currentTx, err := s.repo.GetByIdForUpdate(ctx, tx, meetingApplicationID)
		if err != nil {
			return err
		}
		if currentTx == nil {
			return gerror.NewCode(gcode.New(404, "", nil), meetingErr(ctx, "meeting_application_not_found").Error())
		}

		// 检查当前管理员是否已审批
		hasApproved, err := approvalSvc.HasAdminApprovedTx(ctx, tx, bizType, meetingApplicationID, adminID)
		if err != nil {
			return err
		}
		if hasApproved {
			return gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "already_approved_by_admin").Error())
		}

		// 查询已有 approve 记录数
		approveCount, err := approvalSvc.GetApproveCountTx(ctx, tx, bizType, meetingApplicationID)
		if err != nil {
			return err
		}

		// 记录审批
		if err := approvalSvc.RecordApprovalTx(ctx, tx, bizType, meetingApplicationID, adminID, adminapproval.ActionApprove, "确认验收"); err != nil {
			return err
		}

		// 如果已有其他管理员 approve，触发第二批转账
		if approveCount >= 1 {
			if _, err := s.directTransfer.EnsureTransferOrderTx(ctx, tx, &coboSvc.EnsureTransferOrderReq{BizType: bizType, BizID: meetingApplicationID, Phase: "second", UserID: currentTx.UserId, Symbol: "USDT", Amount: transfer.SecondAmount, Description: "会议报销第二批（50%）"}); err != nil {
				return err
			}
			secondTransferID = transfer.Id
			shouldSubmitSecond = true
		}

		return nil
	})
	if err != nil {
		return err
	}

	// 满足至少两个不同管理员 approve 时才调用 Cobo 转账第二批
	if shouldSubmitSecond {
		order, transferErr := s.directTransfer.SubmitTransferOrder(ctx, bizType, meetingApplicationID, "second")
		if transferErr != nil {
			g.Log().Errorf(ctx, "[Meeting] confirm acceptance direct transfer failed: id=%d, err=%v", meetingApplicationID, transferErr)
			return transferErr
		}
		if err := s.finalizeSecondReimbursement(ctx, meetingApplicationID, secondTransferID, order.RequestID); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) finalizeSecondReimbursement(ctx context.Context, meetingApplicationID, transferID int64, orderNo string) error {
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if err := s.repo.UpdateReimbursementTransferSecond(ctx, tx, transferID, orderNo, time.Now()); err != nil {
			return err
		}
		return s.repo.UpdateMeetingAcceptanceStatus(ctx, tx, meetingApplicationID, 1)
	})
}

func getMaxReimbursementAmount(ctx context.Context) (decimal.Decimal, error) {
	value, err := g.DB().Model("system_config").Ctx(ctx).
		Fields("value").
		Where("key = ?", "meeting_reimbursement_max_amount").
		Value()
	if err != nil {
		return decimal.Zero, err
	}
	if strings.TrimSpace(value.String()) == "" {
		return decimal.NewFromInt(50000), nil
	}
	maxAmount, err := decimal.NewFromString(strings.TrimSpace(value.String()))
	if err != nil {
		return decimal.Zero, gerror.NewCode(gcode.New(400, "", nil), meetingErr(ctx, "invalid_threshold_config").Error())
	}
	return maxAmount, nil
}
