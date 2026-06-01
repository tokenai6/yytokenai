package utils

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"

	"XWFrame/internal/frame/config"

	"github.com/CoboGlobal/cobo-waas2-go-sdk/cobo_waas2/crypto"
	"github.com/gogf/gf/v2/frame/g"
)

// VerifyCoboSignature 验证Cobo webhook签名
func VerifyCoboSignature(r *http.Request) error {
	ctx := r.Context()

	// 1. 获取 headers
	signature := r.Header.Get("BIZ_RESP_SIGNATURE")
	bizTimestamp := r.Header.Get("BIZ_TIMESTAMP")

	g.Log().Infof(ctx, "Headers -> BIZ_RESP_SIGNATURE: %s", signature)
	g.Log().Infof(ctx, "Headers -> BIZ_TIMESTAMP: %s", bizTimestamp)

	if signature == "" || bizTimestamp == "" {
		return fmt.Errorf("missing required headers: BIZ_RESP_SIGNATURE=%v, BIZ_TIMESTAMP=%v", signature == "", bizTimestamp == "")
	}

	// 2. 读取 body
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %v", err)
	}

	if len(rawBody) == 0 {
		g.Log().Warning(ctx, "Empty request body received")
	} else {
		g.Log().Infof(ctx, "Raw Request Body: %s", string(rawBody))
	}

	// 重置 Body 供后续处理使用
	r.Body = io.NopCloser(bytes.NewReader(rawBody))

	// 3. 构造验证消息
	message := fmt.Sprintf("%s|%s", rawBody, bizTimestamp)
	g.Log().Infof(ctx, "Message for signing: %q", message)

	// 4. 获取公钥
	coboConfig, err := config.GetCoboConfig(ctx)
	if err != nil {
		return fmt.Errorf("获取Cobo配置失败: %v", err)
	}

	var publicKeyHex string
	if coboConfig.Env == "prod" || coboConfig.Env == "production" {
		publicKeyHex = coboConfig.PubKeyProd
	} else {
		publicKeyHex = coboConfig.PubKeyDev
	}

	if publicKeyHex == "" {
		return fmt.Errorf("environment '%s' public key not found", coboConfig.Env)
	}

	publicKey, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return fmt.Errorf("failed to decode public key: %v", err)
	}

	// 5. 验证签名
	if !verifySignature(publicKey, signature, message) {
		return fmt.Errorf("signature verification failed")
	}

	g.Log().Info(ctx, "Request verified successfully")
	return nil
}

// verifySignature 验证 ED25519 签名
func verifySignature(publicKey ed25519.PublicKey, signature, message string) bool {
	return VerifyED25519Signature(publicKey, signature, message)
}

// VerifyED25519Signature 验证 ED25519 签名（导出函数）
func VerifyED25519Signature(publicKey ed25519.PublicKey, signature, message string) bool {
	signatureBytes, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}

	doubleHash := crypto.Hash256x2(message)

	// 执行验证
	valid := ed25519.Verify(publicKey, doubleHash, signatureBytes)
	return valid
}
