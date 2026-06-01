package utils_test

import (
	"testing"

	"XWFrame/pkg/utils"
)

// 将下面两个常量替换为你的私钥与消息，然后运行：
//
//	go test ./pkg/utils -v -run TestSignAndVerifyManual
const (
	privateKeyHex = "337244d3017de5c7453f6cf81bed403abec9128a40bb37a00a2eeff0cc0d26dd" // 例如：0xabc... 或不带0x的hex
	message       = "login"                                                            // 例如：Hello World
)

func TestSignAndVerifyManual(t *testing.T) {
	if privateKeyHex == "" || message == "" {
		t.Skip("请在测试文件中填写 privateKeyHex 与 message 后再运行")
	}

	addr, sig, err := utils.SignMessageWithPrivateKey(privateKeyHex, message)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}

	ok, err := utils.VerifyWalletSignature(addr, message, sig)
	if err != nil {
		t.Fatalf("验签失败: %v", err)
	}
	if !ok {
		t.Fatalf("验签未通过: addr=%s sig=%s", addr, sig)
	}

	t.Logf("地址: %s", addr)
	t.Logf("签名: %s", sig)
}
