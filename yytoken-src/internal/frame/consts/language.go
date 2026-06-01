package consts

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
)

// LanguageConst 语言相关常量
const (
	// 支持的语言代码
	LanguageChinese            = "zh-CN" // 简体中文
	LanguageTraditionalChinese = "zh-TW" // 繁体中文
	LanguageEnglish            = "en-US" // 英文
	LanguageJapanese           = "ja-JP" // 日文
	LanguageKorean             = "ko-KR" // 韩文
	LanguageVietnamese         = "vi-VN" // 越南文
	LanguageThai               = "th-TH" // 泰文

	// 默认语言
	LanguageDefault = LanguageChinese // 默认语言为中文

	// CtxLocaleKey 上下文中 locale 的 key
	CtxLocaleKey = "locale"
)

// LanguageNames 语言名称映射
var LanguageNames = map[string]string{
	LanguageChinese:            "中文",
	LanguageTraditionalChinese: "繁體中文",
	LanguageEnglish:            "English",
	LanguageJapanese:           "日本語",
	LanguageKorean:             "한국어",
	LanguageVietnamese:         "Tiếng Việt",
	LanguageThai:               "ไทย",
}

// SupportedLanguages 支持的语言列表
var SupportedLanguages = []string{
	LanguageChinese,
	LanguageTraditionalChinese,
	LanguageEnglish,
	LanguageJapanese,
	LanguageKorean,
	LanguageVietnamese,
	LanguageThai,
}

// IsValidLanguage 检查语言代码是否有效
func IsValidLanguage(language string) bool {
	for _, lang := range SupportedLanguages {
		if lang == language {
			return true
		}
	}
	return false
}

// GetLanguageName 获取语言名称
func GetLanguageName(language string) string {
	if name, exists := LanguageNames[language]; exists {
		return name
	}
	return LanguageNames[LanguageDefault]
}

// NormalizeLanguage 将客户端传入的 locale 字符串归一化到 SupportedLanguages 中的某一项
// 支持的输入示例：
//   - "zh-CN", "zh_cn", "zh", "zh-Hans"           -> "zh-CN"
//   - "zh-TW", "zh-HK", "zh-MO", "zh-Hant"        -> "zh-TW"
//   - "en", "en-US", "en-GB"                      -> "en-US"
//   - "ja", "ja-JP" / "ko", "ko-KR" 等             -> 对应规范代码
//   - "vi", "vi-VN" / "th", "th-TH" 等             -> 对应规范代码
//   - Accept-Language 头形式 "zh-CN,zh;q=0.9,en" -> 取第一项归一化
//
// 若输入为空或无法识别，返回 ""，调用方按需回退到 LanguageDefault。
func NormalizeLanguage(raw string) string {
	code := strings.TrimSpace(raw)
	if code == "" {
		return ""
	}
	// 处理 Accept-Language 形式：取第一项
	if idx := strings.IndexAny(code, ",;"); idx >= 0 {
		code = strings.TrimSpace(code[:idx])
	}
	lower := strings.ToLower(strings.ReplaceAll(code, "_", "-"))

	// 完整代码直接匹配
	switch lower {
	case "zh-cn", "zh-hans", "zh-hans-cn":
		return LanguageChinese
	case "zh-tw", "zh-hk", "zh-mo", "zh-hant", "zh-hant-tw", "zh-hant-hk":
		return LanguageTraditionalChinese
	case "en-us", "en-gb", "en-au", "en-ca":
		return LanguageEnglish
	case "ja-jp":
		return LanguageJapanese
	case "ko-kr":
		return LanguageKorean
	case "vi-vn":
		return LanguageVietnamese
	case "th-th":
		return LanguageThai
	}

	// 主语言降级匹配（如 "zh"、"en"）
	primary := lower
	if idx := strings.Index(primary, "-"); idx >= 0 {
		primary = primary[:idx]
	}
	switch primary {
	case "zh":
		return LanguageChinese
	case "en":
		return LanguageEnglish
	case "ja":
		return LanguageJapanese
	case "ko":
		return LanguageKorean
	case "vi":
		return LanguageVietnamese
	case "th":
		return LanguageThai
	}

	return ""
}

// LocalizedText 从 map[locale]text 中按 locale 取值，缺失时按 fallback 链回退：
// 精确 locale -> 默认语言 (zh-CN) -> 英文 (en-US) -> 空字符串
func LocalizedText(m map[string]string, locale string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[locale]; ok && v != "" {
		return v
	}
	if v, ok := m[LanguageDefault]; ok && v != "" {
		return v
	}
	if v, ok := m[LanguageEnglish]; ok {
		return v
	}
	return ""
}

// LocaleFromCtx 从请求上下文中读取已归一化的 locale
// 若中间件未设置或解析失败，返回 LanguageDefault
func LocaleFromCtx(ctx context.Context) string {
	if ctx == nil {
		return LanguageDefault
	}
	r := g.RequestFromCtx(ctx)
	if r == nil {
		return LanguageDefault
	}
	locale := r.GetCtxVar(CtxLocaleKey).String()
	if locale == "" {
		return LanguageDefault
	}
	return locale
}
