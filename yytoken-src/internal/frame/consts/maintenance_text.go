package consts

var maintenanceTexts = map[string]map[string]string{
	"system_under_maintenance": {
		LanguageChinese:            "系统维护中，请稍后再试",
		LanguageTraditionalChinese: "系統維護中，請稍後再試",
		LanguageEnglish:            "System is under maintenance, please try again later",
		LanguageJapanese:           "システムメンテナンス中です。しばらくしてからお試しください",
		LanguageKorean:             "시스템 점검 중입니다. 잠시 후 다시 시도해 주세요",
	},
	"maintenance_mode_enabled": {
		LanguageChinese:            "维护模式已开启",
		LanguageTraditionalChinese: "維護模式已開啟",
		LanguageEnglish:            "Maintenance mode enabled",
		LanguageJapanese:           "メンテナンスモードを有効にしました",
		LanguageKorean:             "점검 모드가 활성화되었습니다",
	},
	"maintenance_mode_disabled": {
		LanguageChinese:            "维护模式已关闭",
		LanguageTraditionalChinese: "維護模式已關閉",
		LanguageEnglish:            "Maintenance mode disabled",
		LanguageJapanese:           "メンテナンスモードを無効にしました",
		LanguageKorean:             "점검 모드가 비활성화되었습니다",
	},
}

// MaintenanceText 返回维护相关文案的本地化版本
func MaintenanceText(key, locale string) string {
	return LocalizedText(maintenanceTexts[key], locale)
}
