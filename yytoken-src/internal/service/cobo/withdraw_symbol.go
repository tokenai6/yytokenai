package cobo

import "strings"

func normalizeWithdrawSymbol(symbol string) string {
	return strings.ToUpper(strings.TrimSpace(symbol))
}
