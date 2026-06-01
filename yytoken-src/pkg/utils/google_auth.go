package utils

import (
	"github.com/pquerna/otp/totp"
)

// VerifyGoogleAuthCode 验证谷歌验证码
// secret: Base32编码的密钥
// code: 6位数字验证码
// 返回: 验证是否通过
func VerifyGoogleAuthCode(secret, code string) bool {
	if secret == "" || code == "" {
		return false
	}

	// 使用 TOTP 算法验证验证码
	valid := totp.Validate(code, secret)
	return valid
}
