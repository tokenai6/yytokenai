package utils

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// FormatDecimal 格式化小数，始终使用截断（truncate），禁止四舍五入
// 规则：
//   - 若 price ≥ 1：显示为 整数部分.后两位（如 123.45）
//   - 若 0.000001 < price < 1：显示最多4位有效数字，格式为常规小数（不用科学计数法），必要时补零
//   - 若 price < 0.000001：显示成0（包括负数）
//   - 若 price === 0：显示 0
func FormatDecimal(d decimal.Decimal) string {
	// 检查是否为0
	if d.IsZero() {
		return "0"
	}

	// 获取绝对值用于判断
	absValue := d.Abs()
	absFloat, _ := absValue.Float64()

	// 如果 price < 0.000001：显示成0
	const minThreshold = 0.000001
	if absFloat < minThreshold {
		return "0"
	}

	// 如果 price ≥ 1：显示为 整数部分.后两位
	if absFloat >= 1.0 {
		// 截断到2位小数（使用 Truncate，不四舍五入）
		truncated := d.Truncate(2)
		return truncated.StringFixed(2)
	}

	// 如果 0.000001 < price < 1：显示最多4位有效数字
	// 需要计算需要保留的小数位数以保证4位有效数字
	decimalPlaces := calculateDecimalPlacesForSignificantDigits(absFloat, 4)

	// 截断到指定小数位数
	truncated := d.Truncate(int32(decimalPlaces))

	// 格式化为字符串，确保有足够的小数位数
	result := truncated.StringFixed(int32(decimalPlaces))

	// 移除末尾不必要的0和小数点
	result = strings.TrimRight(result, "0")
	result = strings.TrimRight(result, ".")

	// 如果结果为空或只有符号，返回0
	if result == "" || result == "-" {
		return "0"
	}

	return result
}

// calculateDecimalPlacesForSignificantDigits 计算需要的小数位数以保证指定数量的有效数字
// 对于小于1的数，需要找到第一个非零数字的位置
// 例如：0.00123 -> 第一个非零数字在位置3（小数点后第3位）
// 需要保留4位有效数字，所以需要保留到小数点后 3 + 4 - 1 = 6位
func calculateDecimalPlacesForSignificantDigits(f float64, significantDigits int) int {
	if f == 0 {
		return 0
	}

	// 将浮点数转换为字符串，保留足够的小数位数
	decimalStr := fmt.Sprintf("%.20f", f)

	// 找到小数点位置
	dotIndex := strings.Index(decimalStr, ".")
	if dotIndex == -1 {
		return 0
	}

	// 从小数点后开始查找第一个非零数字
	firstNonZeroIndex := -1
	for i := dotIndex + 1; i < len(decimalStr); i++ {
		if decimalStr[i] != '0' {
			firstNonZeroIndex = i - dotIndex
			break
		}
	}

	if firstNonZeroIndex == -1 {
		// 全是0，返回0
		return 0
	}

	// 小数位数 = 第一个非零数字位置 + 有效数字位数 - 1
	decimalPlaces := firstNonZeroIndex + significantDigits - 1

	// 最多保留10位小数，避免过长
	if decimalPlaces > 10 {
		decimalPlaces = 10
	}

	// 最少保留1位小数
	if decimalPlaces < 1 {
		decimalPlaces = 1
	}

	return decimalPlaces
}
