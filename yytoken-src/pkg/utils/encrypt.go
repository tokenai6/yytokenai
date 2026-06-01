package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// GenerateUUID 生成UUID
func GenerateUUID() string {
	return uuid.New().String()
}

// GenerateSnowflakeID 生成雪花ID
func GenerateSnowflakeID() int64 {
	// 简化的雪花ID实现
	timestamp := time.Now().UnixNano() / 1e6 // 毫秒时间戳
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	random := int64(randomBytes[0]) % 1000 // 随机数
	return timestamp*1000 + random
}

// HashPassword 密码加密
func HashPassword(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedBytes), nil
}

// VerifyPassword 密码验证
func VerifyPassword(password, hashedPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// GenerateJWTToken 生成JWT token
func GenerateJWTToken(secret string, claims map[string]interface{}) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(claims))
	return token.SignedString([]byte(secret))
}

// VerifyWalletSignature 验证钱包签名
func VerifyWalletSignature(address, message, signature string) (bool, error) {
	if signature == "" {
		return false, fmt.Errorf("empty signature")
	}

	signatureHex := signature
	if strings.HasPrefix(signatureHex, "0x") || strings.HasPrefix(signatureHex, "0X") {
		signatureHex = signatureHex[2:]
	}
	if len(signatureHex) == 0 {
		return false, fmt.Errorf("invalid signature format")
	}

	// 1. 解码签名
	sigBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false, fmt.Errorf("failed to decode signature: %v", err)
	}

	// 2. 检查签名长度（65字节：r(32) + s(32) + v(1)）
	if len(sigBytes) != 65 {
		return false, fmt.Errorf("invalid signature length: %d", len(sigBytes))
	}

	// 3. 调整 recovery id (v值)
	// 以太坊签名的v值可能是27/28或0/1，需要统一转换为0/1
	if sigBytes[64] >= 27 {
		sigBytes[64] -= 27
	}

	// 4. 使用EIP-191标准计算消息哈希
	// 添加以太坊签名消息前缀
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(message))
	hash := crypto.Keccak256Hash([]byte(prefix + message))

	// 5. 恢复公钥
	pubKey, err := crypto.SigToPub(hash.Bytes(), sigBytes)
	if err != nil {
		return false, fmt.Errorf("failed to recover public key: %v", err)
	}

	// 6. 获取签名者地址
	signerAddress := crypto.PubkeyToAddress(*pubKey)

	// 7. 比较地址（统一转为小写比较）
	expectedAddr := strings.ToLower(address)
	actualAddr := strings.ToLower(signerAddress.Hex())

	return expectedAddr == actualAddr, nil
}

// SignMessageWithPrivateKey 使用以太坊私钥对消息进行EIP-191签名
// 返回签名地址和0x前缀的签名hex
func SignMessageWithPrivateKey(privateKeyHex, message string) (string, string, error) {
	// 1. 解析私钥
	if strings.HasPrefix(privateKeyHex, "0x") || strings.HasPrefix(privateKeyHex, "0X") {
		privateKeyHex = privateKeyHex[2:]
	}
	priv, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return "", "", fmt.Errorf("私钥解析失败: %v", err)
	}

	// 2. 计算EIP-191前缀哈希
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(message))
	hash := crypto.Keccak256Hash([]byte(prefix + message))

	// 3. 执行签名（返回65字节，末位v为0/1）
	sig, err := crypto.Sign(hash.Bytes(), priv)
	if err != nil {
		return "", "", fmt.Errorf("签名失败: %v", err)
	}

	// 4. 计算地址
	pubKey := priv.PublicKey
	addr := crypto.PubkeyToAddress(pubKey).Hex()

	// 5. 编码签名为0x前缀hex
	sigHex := "0x" + hex.EncodeToString(sig)

	return addr, sigHex, nil
}

// MaskWalletAddress 钱包地址脱敏
func MaskWalletAddress(address string) string {
	if len(address) < 10 {
		return address
	}
	return address[:6] + "****" + address[len(address)-4:]
}

// CalculateHash 计算哈希值
func CalculateHash(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// TokenHash 计算 token 的短哈希（SHA256 前 8 字节 hex，共 16 字符）
// 用作多设备 session 在 Redis 中的独立 key 后缀
func TokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:8])
}

// FormatTokenAmount 格式化代币数量（考虑精度）
func FormatTokenAmount(amount string, precision int) (string, error) {
	// 将字符串转换为big.Int
	amountBig, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return "", fmt.Errorf("invalid amount: %s", amount)
	}

	// 计算精度因子
	precisionFactor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(precision)), nil)

	// 转换为浮点数格式
	quotient := new(big.Int).Div(amountBig, precisionFactor)
	remainder := new(big.Int).Mod(amountBig, precisionFactor)

	// 格式化小数部分
	remainderStr := remainder.String()
	for len(remainderStr) < precision {
		remainderStr = "0" + remainderStr
	}

	return quotient.String() + "." + remainderStr, nil
}

// ParseTokenAmount 解析代币数量（考虑精度）
func ParseTokenAmount(amountStr string, precision int) (string, error) {
	// 解析浮点数
	amountFloat, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return "", err
	}

	// 计算精度因子
	precisionFactor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(precision)), nil))

	// 转换为整数
	amountBig := new(big.Float).SetFloat64(amountFloat)
	amountBig.Mul(amountBig, precisionFactor)

	// 转换为字符串
	amountInt, _ := amountBig.Int(nil)
	return amountInt.String(), nil
}

// GenerateRandomString 生成随机字符串
func GenerateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[num.Int64()]
	}
	return string(b)
}

// IsValidEthereumAddress 验证以太坊地址格式
func IsValidEthereumAddress(address string) bool {
	if len(address) != 42 {
		return false
	}
	if address[:2] != "0x" {
		return false
	}

	// 检查是否为有效的十六进制
	for _, c := range address[2:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}

	return true
}

// IsValidInviteCode 验证邀请码格式（8-16位大写字母+数字）
func IsValidInviteCode(inviteCode string) bool {
	if len(inviteCode) < 8 || len(inviteCode) > 16 {
		return false
	}

	// 检查是否只包含大写字母和数字
	for _, c := range inviteCode {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}

	return true
}

// GenerateInviteCode 生成10位邀请码（大写字母+数字）
func GenerateInviteCode() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 10)
	for i := range b {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[num.Int64()]
	}
	return string(b)
}
