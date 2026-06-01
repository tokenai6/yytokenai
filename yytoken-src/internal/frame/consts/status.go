package consts

// 错误码规范
// - 0            表示成功
// - 4xx/5xx开头  对应通用HTTP语义错误
// - 1xx_xxx      业务领域错误（预留给具体模块），例如用户模块 100xxx
// 说明：仅集中定义错误码常量，实际错误文案交由国际化处理

const (
	// 通用
	ErrOK                 = 0     // 成功
	ErrInvalidParam       = 40001 // 参数错误
	ErrUnauthorized       = 40100 // 未认证/Token失效
	ErrForbidden          = 40300 // 无权限
	ErrNotFound           = 40400 // 资源不存在
	ErrConflict           = 40900 // 资源冲突
	ErrTooManyRequests    = 42900 // 请求过于频繁
	ErrInternal           = 50000 // 服务器内部错误
	ErrServiceUnavailable = 50300 // 服务不可用
)

// 用户模块 100xxx
const (
	ErrUserNotFound     = 100001 // 用户不存在
	ErrUserBanned       = 100002 // 用户已被封禁
	ErrLoginFailed      = 100003 // 登录失败
	ErrSignatureInvalid = 100004 // 签名无效
	ErrInviteRequired   = 100005 // 需要邀请码注册
)

// 预留：其它业务模块可按区间扩展
// - 110xxx: 订单模块
// - 120xxx: 支付模块
// - 130xxx: 权限/角色模块
