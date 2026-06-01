package cobo

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	coboWaas2 "github.com/CoboGlobal/cobo-waas2-go-sdk/cobo_waas2"
)

// setupTestClient 创建测试客户端
func setupTestClient(t *testing.T) *CoboClient {
	// 创建测试配置 - 增加超时时间
	config := &Config{
		APISecret: "663f6162cd0393e63d83d228d268b9c03d820a11ce7678d705798a756a64a450",
		Env:       "dev",
		Timeout:   60, // 增加到60秒
	}

	// 创建客户端
	client, err := NewCoboClient(config)
	if err != nil {
		t.Fatalf("创建Cobo客户端失败: %v", err)
	}

	return client
}

// retryWithBackoff 重试机制，带指数退避
func retryWithBackoff(t *testing.T, maxRetries int, operation func() error) error {
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			// 指数退避：1s, 2s, 4s, 8s...
			backoff := time.Duration(1<<uint(i-1)) * time.Second
			fmt.Fprintf(os.Stderr, "⏳ 第 %d 次重试，等待 %v...\n", i+1, backoff)
			time.Sleep(backoff)
		}

		err := operation()
		if err == nil {
			if i > 0 {
				fmt.Fprintf(os.Stderr, "✅ 重试成功！\n")
			}
			return nil
		}

		lastErr = err
		fmt.Fprintf(os.Stderr, "❌ 第 %d 次尝试失败: %v\n", i+1, err)
	}

	return fmt.Errorf("重试 %d 次后仍然失败，最后错误: %v", maxRetries, lastErr)
}

// testNetworkConnectivity 测试网络连通性
func testNetworkConnectivity(t *testing.T) {
	fmt.Fprintf(os.Stderr, "🔍 开始网络诊断...\n")

	// 测试DNS解析
	start := time.Now()
	ips, err := net.LookupIP("api.dev.cobo.com")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ DNS解析失败: %v\n", err)
		return
	}
	dnsTime := time.Since(start)
	fmt.Fprintf(os.Stderr, "✅ DNS解析成功，耗时: %v，IP地址: %v\n", dnsTime, ips)

	// 测试TCP连接
	for _, ip := range ips {
		if ip.To4() != nil { // 只测试IPv4
			start = time.Now()
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:443", ip.String()), 10*time.Second)
			if err != nil {
				fmt.Fprintf(os.Stderr, "❌ TCP连接到 %s:443 失败: %v\n", ip.String(), err)
				continue
			}
			tcpTime := time.Since(start)
			conn.Close()
			fmt.Fprintf(os.Stderr, "✅ TCP连接到 %s:443 成功，耗时: %v\n", ip.String(), tcpTime)
		}
	}

	// 测试HTTP连接
	start = time.Now()
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://api.dev.cobo.com")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ HTTP请求失败: %v\n", err)
		return
	}
	httpTime := time.Since(start)
	resp.Body.Close()
	fmt.Fprintf(os.Stderr, "✅ HTTP请求成功，状态码: %d，耗时: %v\n", resp.StatusCode, httpTime)
}

// TestListWallets 测试获取钱包列表
func TestListWallets(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	resp, err := client.ListWallets(ctx, 10, "", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 获取钱包列表失败: %v\n", err)
		return
	}

	// 转换为JSON并打印
	jsonData, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ JSON序列化失败: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "✅ 钱包列表响应:\n%s\n", string(jsonData))
}

// TestGetWalletById 测试获取钱包信息
func TestGetWalletById(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	walletInfo, err := client.GetWalletById(ctx, "3de98d53-a37d-486c-bac1-a5d512a4f4e7")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 获取钱包信息失败: %v\n", err)
		return
	}

	// 转换为JSON并打印
	jsonData, err := json.MarshalIndent(walletInfo, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ JSON序列化失败: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "✅ 钱包信息响应:\n%s\n", string(jsonData))
}

// TestCreateAddress 测试创建地址
func TestCreateAddress(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	createReq := &CreateAddressRequest{
		WalletID: "3de98d53-a37d-486c-bac1-a5d512a4f4e7",
		ChainID:  "BSC_BNB",
		Count:    1,
	}

	addresses, err := client.CreateAddress(ctx, createReq)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 创建地址失败: %v\n", err)
		return
	}

	// 转换为JSON并打印
	jsonData, err := json.MarshalIndent(addresses, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ JSON序列化失败: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "✅ 创建地址响应:\n%s\n", string(jsonData))
}

// TestListAddresses 测试获取地址列表
func TestListAddresses(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	listReq := &ListAddressesRequest{
		WalletID: "3de98d53-a37d-486c-bac1-a5d512a4f4e7",
		Limit:    10,
	}

	var addressesResp *coboWaas2.ListAddresses200Response

	// 使用重试机制
	err := retryWithBackoff(t, 3, func() error {
		var err error
		addressesResp, err = client.ListAddresses(ctx, listReq)
		return err
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 获取地址列表失败: %v\n", err)
		return
	}

	// 转换为JSON并打印
	jsonData, err := json.MarshalIndent(addressesResp, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ JSON序列化失败: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "✅ 地址列表响应:\n%s\n", string(jsonData))
}

// TestListWebhookEndpoints 测试获取Webhook端点列表
func TestListWebhookEndpoints(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	endpointsResp, err := client.ListWebhookEndpoints(ctx, 10, "", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 获取Webhook端点列表失败: %v\n", err)
		return
	}

	// 转换为JSON并打印
	jsonData, err := json.MarshalIndent(endpointsResp, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ JSON序列化失败: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "✅ Webhook端点列表响应:\n%s\n", string(jsonData))
}

// TestGetWebhookEventById 测试获取Webhook事件信息
func TestGetWebhookEventById(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	eventInfo, err := client.GetWebhookEventById(ctx, "test_event_id", "test_endpoint_id")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 获取Webhook事件信息失败: %v\n", err)
		return
	}

	// 转换为JSON并打印
	jsonData, err := json.MarshalIndent(eventInfo, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ JSON序列化失败: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "✅ Webhook事件信息响应:\n%s\n", string(jsonData))
}

// TestListWebhookEventDefinitions 测试获取Webhook事件定义列表
func TestListWebhookEventDefinitions(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	definitions, err := client.ListWebhookEventDefinitions(ctx, 10, "", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 获取Webhook事件定义列表失败: %v\n", err)
		return
	}

	// 转换为JSON并打印
	jsonData, err := json.MarshalIndent(definitions, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ JSON序列化失败: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "✅ Webhook事件定义列表响应:\n%s\n", string(jsonData))
}

// TestListSupportedChains 测试获取支持的链列表
func TestListSupportedChains(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	chainsResp, err := client.ListSupportedChains(ctx, 10, "", "", "BSC_BNB")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 获取支持的链列表失败: %v\n", err)
		return
	}

	// 转换为JSON并打印
	jsonData, err := json.MarshalIndent(chainsResp, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ JSON序列化失败: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "✅ 支持的链列表响应:\n%s\n", string(jsonData))
}

// TestListSupportedTokens 测试获取支持的代币列表
func TestListSupportedTokens(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	tokensResp, err := client.ListSupportedTokens(ctx, 10, "", "", "BNB_BSC", "USDT")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 获取支持的代币列表失败: %v\n", err)
		return
	}

	// 转换为JSON并打印
	jsonData, err := json.MarshalIndent(tokensResp, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ JSON序列化失败: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "✅ 支持的代币列表响应:\n%s\n", string(jsonData))
}

// TestNetworkConnectivity 测试网络连通性
func TestNetworkConnectivity(t *testing.T) {
	testNetworkConnectivity(t)
}
