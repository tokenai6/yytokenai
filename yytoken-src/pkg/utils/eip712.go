package utils

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

// EIP712Domain EIP-712域分隔符结构
type EIP712Domain struct {
	Name              string
	Version           string
	ChainId           *big.Int
	VerifyingContract common.Address
}

// EIP712TypedData EIP-712类型化数据
type EIP712TypedData struct {
	Types       apitypes.Types
	PrimaryType string
	Domain      EIP712Domain
	Message     map[string]interface{}
}

// SignEIP712 使用EIP-712协议签名
// privateKeyHex: 私钥（支持0x前缀）
// domain: 域分隔符
// primaryType: 主要类型名称
// message: 消息数据
// types: 类型定义（不包含EIP712Domain）
func SignEIP712(privateKeyHex string, domain EIP712Domain, primaryType string, message map[string]interface{}, types apitypes.Types) (string, error) {
	// 1. 解析私钥
	if strings.HasPrefix(privateKeyHex, "0x") || strings.HasPrefix(privateKeyHex, "0X") {
		privateKeyHex = privateKeyHex[2:]
	}
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return "", fmt.Errorf("私钥解析失败: %v", err)
	}

	// 2. 添加Domain类型定义
	allTypes := make(apitypes.Types)
	allTypes["EIP712Domain"] = []apitypes.Type{
		{Name: "name", Type: "string"},
		{Name: "version", Type: "string"},
		{Name: "chainId", Type: "uint256"},
		{Name: "verifyingContract", Type: "address"},
	}

	// 复制用户提供的类型定义
	for k, v := range types {
		allTypes[k] = v
	}

	// 3. 构建TypedData Domain
	typedDataDomain := apitypes.TypedDataDomain{
		Name:              domain.Name,
		Version:           domain.Version,
		ChainId:           math.NewHexOrDecimal256(domain.ChainId.Int64()),
		VerifyingContract: domain.VerifyingContract.Hex(),
	}

	typedData := apitypes.TypedData{
		Types:       allTypes,
		PrimaryType: primaryType,
		Domain:      typedDataDomain,
		Message:     apitypes.TypedDataMessage(message),
	}

	// 4. 计算类型化数据哈希
	domainSeparator, err := typedData.HashStruct("EIP712Domain", typedDataDomain.Map())
	if err != nil {
		return "", fmt.Errorf("计算域分隔符失败: %v", err)
	}

	typedDataHash, err := typedData.HashStruct(primaryType, message)
	if err != nil {
		return "", fmt.Errorf("计算消息哈希失败: %v", err)
	}

	// 5. 构建最终签名数据：\x19\x01 + domainSeparator + typedDataHash
	rawData := []byte("\x19\x01")
	rawData = append(rawData, domainSeparator...)
	rawData = append(rawData, typedDataHash...)

	// 6. 计算Keccak256哈希
	hash := crypto.Keccak256Hash(rawData)

	// 7. 签名
	signature, err := crypto.Sign(hash.Bytes(), privateKey)
	if err != nil {
		return "", fmt.Errorf("签名失败: %v", err)
	}

	// 8. 调整v值（以太坊标准：27/28）
	if signature[64] < 27 {
		signature[64] += 27
	}

	// 9. 返回0x前缀的hex签名
	return "0x" + hex.EncodeToString(signature), nil
}

// VerifyEIP712Signature 验证EIP-712签名
func VerifyEIP712Signature(signature string, domain EIP712Domain, primaryType string, message map[string]interface{}, types apitypes.Types, expectedAddress string) (bool, error) {
	// 1. 解码签名
	signature = strings.TrimPrefix(signature, "0x")
	sigBytes, err := hex.DecodeString(signature)
	if err != nil {
		return false, fmt.Errorf("签名解码失败: %v", err)
	}

	if len(sigBytes) != 65 {
		return false, fmt.Errorf("签名长度不正确: %d", len(sigBytes))
	}

	// 2. 调整v值（转换为0/1）
	if sigBytes[64] >= 27 {
		sigBytes[64] -= 27
	}

	// 3. 构建类型定义
	allTypes := make(apitypes.Types)
	allTypes["EIP712Domain"] = []apitypes.Type{
		{Name: "name", Type: "string"},
		{Name: "version", Type: "string"},
		{Name: "chainId", Type: "uint256"},
		{Name: "verifyingContract", Type: "address"},
	}
	for k, v := range types {
		allTypes[k] = v
	}

	// 4. 构建TypedData Domain
	typedDataDomain := apitypes.TypedDataDomain{
		Name:              domain.Name,
		Version:           domain.Version,
		ChainId:           math.NewHexOrDecimal256(domain.ChainId.Int64()),
		VerifyingContract: domain.VerifyingContract.Hex(),
	}

	typedData := apitypes.TypedData{
		Types:       allTypes,
		PrimaryType: primaryType,
		Domain:      typedDataDomain,
		Message:     apitypes.TypedDataMessage(message),
	}

	// 5. 计算类型化数据哈希
	domainSeparator, err := typedData.HashStruct("EIP712Domain", typedDataDomain.Map())
	if err != nil {
		return false, fmt.Errorf("计算域分隔符失败: %v", err)
	}

	typedDataHash, err := typedData.HashStruct(primaryType, message)
	if err != nil {
		return false, fmt.Errorf("计算消息哈希失败: %v", err)
	}

	// 6. 构建最终签名数据
	rawData := []byte("\x19\x01")
	rawData = append(rawData, domainSeparator...)
	rawData = append(rawData, typedDataHash...)
	hash := crypto.Keccak256Hash(rawData)

	// 7. 恢复公钥
	pubKey, err := crypto.SigToPub(hash.Bytes(), sigBytes)
	if err != nil {
		return false, fmt.Errorf("恢复公钥失败: %v", err)
	}

	// 8. 获取签名者地址
	signerAddress := crypto.PubkeyToAddress(*pubKey)

	// 9. 比较地址
	return strings.EqualFold(signerAddress.Hex(), expectedAddress), nil
}

// GetPublicKeyFromPrivateKey 从私钥获取公钥地址
func GetPublicKeyFromPrivateKey(privateKeyHex string) (string, error) {
	if strings.HasPrefix(privateKeyHex, "0x") || strings.HasPrefix(privateKeyHex, "0X") {
		privateKeyHex = privateKeyHex[2:]
	}

	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return "", fmt.Errorf("私钥解析失败: %v", err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("无法转换公钥")
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA)
	return address.Hex(), nil
}
