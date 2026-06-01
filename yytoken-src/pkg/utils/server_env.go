package utils

import "os"

// IsTestServer 判断当前服务器是否为测试环境（testyyai）
func IsTestServer() bool {
	hostname, _ := os.Hostname()
	return hostname == "testyyai"
}
