package utils

import "strings"

// BuildLikePattern 把带 ^ / $ 锚点的搜索词转换成 SQL LIKE 模式。
//   ^abc   -> abc%
//   abc$   -> %abc
//   ^abc$  -> abc
//   abc    -> %abc%
//   ""     -> "" (调用方自行判空)
func BuildLikePattern(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	hasPrefix := strings.HasPrefix(s, "^")
	hasSuffix := strings.HasSuffix(s, "$")
	if hasPrefix {
		s = strings.TrimPrefix(s, "^")
	}
	if hasSuffix {
		s = strings.TrimSuffix(s, "$")
	}
	if s == "" {
		return ""
	}
	switch {
	case hasPrefix && hasSuffix:
		return s
	case hasPrefix:
		return s + "%"
	case hasSuffix:
		return "%" + s
	default:
		return "%" + s + "%"
	}
}
