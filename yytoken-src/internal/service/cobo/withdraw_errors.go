package cobo

import (
	"XWFrame/internal/frame/consts"

	"github.com/gogf/gf/v2/errors/gerror"
)

// withdrawErrMsgs 提现相关面向客户端的错误文案多语言表
// 新增 key 时必须覆盖全部 7 种语言：zh-CN / zh-TW / en-US / ja-JP / ko-KR / vi-VN / th-TH
var withdrawErrMsgs = map[string]map[string]string{
	"not_logged_in": {
		"zh-CN": "请登录喔",
		"zh-TW": "請登入喔",
		"en-US": "Please log in",
		"ja-JP": "ログインしてください",
		"ko-KR": "로그인해 주세요",
		"vi-VN": "Vui lòng đăng nhập",
		"th-TH": "กรุณาเข้าสู่ระบบ",
	},
	"amount_invalid": {
		"zh-CN": "输入错误",
		"zh-TW": "輸入錯誤",
		"en-US": "Invalid input",
		"ja-JP": "入力エラー",
		"ko-KR": "입력 오류",
		"vi-VN": "Lỗi nhập liệu",
		"th-TH": "ข้อมูลไม่ถูกต้อง",
	},
	"amount_must_positive": {
		"zh-CN": "提现需>0",
		"zh-TW": "提現需>0",
		"en-US": "Amount must be > 0",
		"ja-JP": "出金額は0より大きく必要があります",
		"ko-KR": "출금액은 0보다 커야 합니다",
		"vi-VN": "Số tiền rút phải > 0",
		"th-TH": "จำนวนถอนต้อง > 0",
	},
	"unsupported_symbol": {
		"zh-CN": "暂不支持喔",
		"zh-TW": "暫不支援喔",
		"en-US": "Not supported",
		"ja-JP": "未対応",
		"ko-KR": "지원되지 않음",
		"vi-VN": "Chưa hỗ trợ",
		"th-TH": "ยังไม่รองรับ",
	},
	"withdraw_rate_limited": {
		"zh-CN": "请求太频繁啦",
		"zh-TW": "請求太頻繁啦",
		"en-US": "Too frequent",
		"ja-JP": "リクエストが頻繁すぎます",
		"ko-KR": "요청이 너무 빈번합니다",
		"vi-VN": "Quá thường xuyên",
		"th-TH": "ถี่เกินไป",
	},
	"withdraw_disabled": {
		"zh-CN": "提现马上开放噢",
		"zh-TW": "提現馬上開放噢",
		"en-US": "Withdrawal opening soon",
		"ja-JP": "出金はまもなく再開します",
		"ko-KR": "출금이 곧 재개됩니다",
		"vi-VN": "Rút tiền sẽ sớm mở lại",
		"th-TH": "ถอนจะเปิดเร็วๆ นี้",
	},
	"test_server_withdraw_disabled": {
		"zh-CN": "测试环境禁止提现",
		"zh-TW": "測試環境禁止提現",
		"en-US": "withdrawal is disabled on test server",
		"ja-JP": "テスト環境では出金が禁止されています",
		"ko-KR": "테스트 서버에서는 출금이 비활성화되어 있습니다",
		"vi-VN": "Rút tiền bị vô hiệu hóa trên máy chủ thử nghiệm",
		"th-TH": "การถอนถูกปิดใช้งานบนเซิร์ฟเวอร์ทดสอบ",
	},
	"symbol_withdraw_disabled": {
		"zh-CN": "提现即将开放喔",
		"zh-TW": "提現即將開放喔",
		"en-US": "Withdrawal opening soon",
		"ja-JP": "出金はまもなく再開します",
		"ko-KR": "출금이 곧 재개됩니다",
		"vi-VN": "Rút tiền sẽ sớm mở lại",
		"th-TH": "ถอนจะเปิดเร็วๆ นี้",
	},
	"amount_too_small_after_fee": {
		"zh-CN": "提现≥10U",
		"zh-TW": "提現≥10U",
		"en-US": "Amount too small",
		"ja-JP": "金額が小さすぎます",
		"ko-KR": "금액이 너무 작습니다",
		"vi-VN": "Số tiền quá nhỏ",
		"th-TH": "จำนวนเงินน้อยเกินไป",
	},
	"amount_after_fee_tax_too_small": {
		"zh-CN": "提现请≥100U",
		"zh-TW": "提現請≥100U",
		"en-US": "Amount too small",
		"ja-JP": "金額が小さすぎます",
		"ko-KR": "금액이 너무 작습니다",
		"vi-VN": "Số tiền quá nhỏ",
		"th-TH": "จำนวนเงินน้อยเกินไป",
	},
	"user_not_found": {
		"zh-CN": "用户不存在",
		"zh-TW": "用戶不存在",
		"en-US": "user not found",
		"ja-JP": "ユーザーが存在しません",
		"ko-KR": "사용자를 찾을 수 없습니다",
		"vi-VN": "Không tìm thấy người dùng",
		"th-TH": "ไม่พบผู้ใช้",
	},
	"withdraw_disabled_for_user": {
		"zh-CN": "提现已禁用噢",
		"zh-TW": "提現已禁用噢",
		"en-US": "Withdrawal disabled",
		"ja-JP": "出金は無効です",
		"ko-KR": "출금이 비활성화되었습니다",
		"vi-VN": "Rút tiền đã bị vô hiệu hóa",
		"th-TH": "การถอนถูกปิดใช้งาน",
	},
	"wallet_address_empty": {
		"zh-CN": "地址错误",
		"zh-TW": "地址錯誤",
		"en-US": "Address error",
		"ja-JP": "アドレスエラー",
		"ko-KR": "주소 오류",
		"vi-VN": "Lỗi địa chỉ",
		"th-TH": "ที่อยู่ไม่ถูกต้อง",
	},
	"invalid_receive_address": {
		"zh-CN": "格式错误",
		"zh-TW": "格式錯誤",
		"en-US": "Format error",
		"ja-JP": "形式エラー",
		"ko-KR": "형식 오류",
		"vi-VN": "Lỗi định dạng",
		"th-TH": "รูปแบบไม่ถูกต้อง",
	},
	"receive_address_not_match_wallet": {
		"zh-CN": "非法提现噢",
		"zh-TW": "非法提現噢",
		"en-US": "Invalid withdrawal",
		"ja-JP": "出金が無効です",
		"ko-KR": "출금이 무효입니다",
		"vi-VN": "Rút tiền không hợp lệ",
		"th-TH": "การถอนไม่ถูกต้อง",
	},
	"balance_insufficient": {
		"zh-CN": "余额不足",
		"zh-TW": "餘額不足",
		"en-US": "insufficient balance",
		"ja-JP": "残高が不足しています",
		"ko-KR": "잔액이 부족합니다",
		"vi-VN": "Số dư không đủ",
		"th-TH": "ยอดคงเหลือไม่เพียงพอ",
	},
	"amount_below_min": {
		"zh-CN": "金额>10U",
		"zh-TW": "金額>10U",
		"en-US": "Amount too small",
		"ja-JP": "金額が小さすぎます",
		"ko-KR": "금액이 너무 작습니다",
		"vi-VN": "Số tiền quá nhỏ",
		"th-TH": "จำนวนเงินน้อยเกินไป",
	},
	"new_user_24h_limit": {
		"zh-CN": "次日开放噢",
		"zh-TW": "次日開放噢",
		"en-US": "Available tomorrow",
		"ja-JP": "明日からご利用可能です",
		"ko-KR": "내일부터 가능합니다",
		"vi-VN": "Mở vào ngày mai",
		"th-TH": "เปิดพรุ่งนี้",
	},
	"daily_frequency_limit": {
		"zh-CN": "明日再来哦",
		"zh-TW": "明日再來哦",
		"en-US": "Come back tomorrow",
		"ja-JP": "明日お試しください",
		"ko-KR": "내일 다시 시도해 주세요",
		"vi-VN": "Hẹn gặp lại ngày mai",
		"th-TH": "ลองใหม่พรุ่งนี้",
	},
	"gift_node_performance_not_met": {
		"zh-CN": "贡献值≥10倍",
		"zh-TW": "貢獻值≥10倍",
		"en-US": "Contribution must be ≥ 10x",
		"ja-JP": "貢献値は10倍以上が必要",
		"ko-KR": "기여도는 10배 이상 필요",
		"vi-VN": "Đóng góp phải ≥ 10 lần",
		"th-TH": "ผลงานต้อง ≥ 10 เท่า",
	},
	"tax_exceeds_amount": {
		"zh-CN": "提现超额",
		"zh-TW": "提現超額",
		"en-US": "Withdrawal exceeds limit",
		"ja-JP": "出金が限度を超えています",
		"ko-KR": "출금이 한도를 초과했습니다",
		"vi-VN": "Vượt quá hạn mức rút",
		"th-TH": "ถอนเกินขีดจำกัด",
	},
	"audit_status_invalid": {
		"zh-CN": "审核状态不合法",
		"zh-TW": "審核狀態不合法",
		"en-US": "invalid audit status",
		"ja-JP": "審査ステータスが正しくありません",
		"ko-KR": "심사 상태가 유효하지 않습니다",
		"vi-VN": "Trạng thái duyệt không hợp lệ",
		"th-TH": "สถานะการตรวจสอบไม่ถูกต้อง",
	},
	"withdraw_record_not_found": {
		"zh-CN": "提现记录不存在",
		"zh-TW": "提現記錄不存在",
		"en-US": "withdraw record not found",
		"ja-JP": "出金記録が存在しません",
		"ko-KR": "출금 기록을 찾을 수 없습니다",
		"vi-VN": "Không tìm thấy bản ghi rút tiền",
		"th-TH": "ไม่พบบันทึกการถอน",
	},
	"only_pending_can_audit": {
		"zh-CN": "仅待审核状态可审核",
		"zh-TW": "僅待審核狀態可審核",
		"en-US": "only pending withdrawals can be audited",
		"ja-JP": "審査待ちの出金のみ審査できます",
		"ko-KR": "심사 대기 상태의 출금만 심사할 수 있습니다",
		"vi-VN": "Chỉ có thể duyệt khi đang chờ duyệt",
		"th-TH": "สามารถตรวจสอบได้เฉพาะรายการที่อยู่ในสถานะรอตรวจสอบเท่านั้น",
	},
	"admin_not_logged_in": {
		"zh-CN": "管理员未登录",
		"zh-TW": "管理員未登入",
		"en-US": "admin not logged in",
		"ja-JP": "管理者がログインしていません",
		"ko-KR": "관리자가 로그인되어 있지 않습니다",
		"vi-VN": "Quản trị viên chưa đăng nhập",
		"th-TH": "ผู้ดูแลระบบยังไม่ได้เข้าสู่ระบบ",
	},
	"withdraw_submitted": {
		"zh-CN": "提现广播中",
		"zh-TW": "提現廣播中",
		"en-US": "Withdrawal broadcasting",
		"ja-JP": "出金処理中",
		"ko-KR": "출금 처리 중",
		"vi-VN": "Đang xử lý rút tiền",
		"th-TH": "กำลังดำเนินการถอน",
	},
	"withdraw_config_invalid": {
		"zh-CN": "提现异常",
		"zh-TW": "提現異常",
		"en-US": "Withdrawal error",
		"ja-JP": "出金エラー",
		"ko-KR": "출금 오류",
		"vi-VN": "Lỗi rút tiền",
		"th-TH": "ข้อผิดพลาดการถอน",
	},
	"process_status_invalid": {
		"zh-CN": "当前状态不允许处理提现",
		"zh-TW": "當前狀態不允許處理提現",
		"en-US": "current status does not allow processing the withdrawal",
		"ja-JP": "現在のステータスでは出金処理を行えません",
		"ko-KR": "현재 상태에서는 출금을 처리할 수 없습니다",
		"vi-VN": "Trạng thái hiện tại không cho phép xử lý rút tiền",
		"th-TH": "สถานะปัจจุบันไม่อนุญาตให้ดำเนินการถอน",
	},
}

// withdrawErr 返回指定 locale 的多语言错误，未命中时回退到英文
func withdrawErr(locale, key string) error {
	locale = consts.NormalizeLanguage(locale)
	if locale == "" {
		locale = consts.LanguageDefault
	}
	m := withdrawErrMsgs[key]
	msg := consts.LocalizedText(m, locale)
	if msg == "" {
		return gerror.New(key)
	}
	return gerror.New(msg)
}

// withdrawErrKey 用于带参数的多语言错误 key，定义独立类型以避免触发 printf lint 误报
type withdrawErrKey string

// withdrawErrf 返回带参数的多语言错误
func withdrawErrf(locale string, key withdrawErrKey, args ...interface{}) error {
	locale = consts.NormalizeLanguage(locale)
	if locale == "" {
		locale = consts.LanguageDefault
	}
	m := withdrawErrMsgs[string(key)]
	msg := consts.LocalizedText(m, locale)
	if msg == "" {
		return gerror.Newf(string(key), args...)
	}
	return gerror.Newf(msg, args...)
}

// wrapRiskI18n 在保留 sentinel error 的同时给出多语言提示，便于 service 层 errors.Is 检测
func wrapRiskI18n(cause error, locale, key string, args ...interface{}) error {
	locale = consts.NormalizeLanguage(locale)
	if locale == "" {
		locale = consts.LanguageDefault
	}
	m := withdrawErrMsgs[key]
	msg := consts.LocalizedText(m, locale)
	if msg == "" {
		msg = key
	}
	if len(args) > 0 {
		return wrapRiskError(cause, msg, args...)
	}
	return wrapRiskError(cause, "%s", msg)
}

// WithdrawErr 对外暴露的多语言错误构造器，供 API 层复用 withdrawErrMsgs
func WithdrawErr(locale, key string) error {
	return withdrawErr(locale, key)
}

// WithdrawErrf 对外暴露的带参数多语言错误构造器
func WithdrawErrf(locale, key string, args ...interface{}) error {
	return withdrawErrf(locale, withdrawErrKey(key), args...)
}
