package cobo

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	coboWaas2 "github.com/CoboGlobal/cobo-waas2-go-sdk/cobo_waas2"
	"github.com/CoboGlobal/cobo-waas2-go-sdk/cobo_waas2/crypto"
)

// CoboClient Cobo钱包客户端
type CoboClient struct {
	apiClient *coboWaas2.APIClient
	ctx       context.Context
	env       int
	apiSecret string
	timeout   time.Duration
}

// Config Cobo客户端配置
type Config struct {
	APISecret string `yaml:"apiSecret" dc:"API密钥"`
	Env       string `yaml:"env" dc:"环境：dev/prod"`
	Timeout   int    `yaml:"timeout" dc:"请求超时时间(秒)"`
}

// NewCoboClient 创建Cobo客户端
func NewCoboClient(config *Config) (*CoboClient, error) {
	if config == nil {
		return nil, fmt.Errorf("配置不能为空")
	}

	apiSecret := strings.TrimSpace(config.APISecret)
	if apiSecret == "" {
		return nil, fmt.Errorf("API密钥不能为空")
	}
	if err := validateAPISecret(apiSecret); err != nil {
		return nil, fmt.Errorf("API密钥格式错误: %w", err)
	}

	// 设置环境
	var env int
	switch config.Env {
	case "prod", "production":
		env = coboWaas2.ProdEnv
	case "dev", "development":
		env = coboWaas2.DevEnv
	default:
		env = coboWaas2.DevEnv // 默认开发环境
	}

	// 创建配置和客户端
	configuration := coboWaas2.NewConfiguration()

	// 配置HTTP客户端以优化网络连接
	httpClient := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second, // 连接超时
				KeepAlive: 30 * time.Second, // 保持连接
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second, // TLS握手超时
			ResponseHeaderTimeout: 10 * time.Second, // 响应头超时
			ExpectContinueTimeout: 1 * time.Second,  // Expect: 100-continue超时
			MaxIdleConns:          100,              // 最大空闲连接数
			MaxIdleConnsPerHost:   10,               // 每个主机的最大空闲连接数
			IdleConnTimeout:       90 * time.Second, // 空闲连接超时
		},
	}

	configuration.HTTPClient = httpClient
	apiClient := coboWaas2.NewAPIClient(configuration)

	// 设置上下文
	ctx := context.Background()
	ctx = context.WithValue(ctx, coboWaas2.ContextEnv, env)

	// 使用原始API Secret格式
	ctx = context.WithValue(ctx, coboWaas2.ContextPortalSigner, crypto.Ed25519Signer{
		Secret: apiSecret,
	})

	// 设置超时时间（不应用到全局上下文）
	var timeout time.Duration
	if config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	} else {
		timeout = 30 * time.Second // 默认30秒超时
	}

	return &CoboClient{
		apiClient: apiClient,
		ctx:       ctx,
		env:       env,
		apiSecret: apiSecret,
		timeout:   timeout,
	}, nil
}

// GetAPIClient 获取API客户端
func (c *CoboClient) GetAPIClient() *coboWaas2.APIClient {
	return c.apiClient
}

// GetContext 获取上下文
func (c *CoboClient) GetContext() context.Context {
	return c.ctx
}

// getContextWithTimeout 获取带超时的上下文
func (c *CoboClient) getContextWithTimeout(ctx context.Context) context.Context {
	// 创建新的基础上下文，包含认证信息。Cobo SDK 从 context 读取 signer，
	// scheduler 传入的 ctx 自带 deadline 但没有 signer，不能直接复用。
	baseCtx := context.Background()
	baseCtx = context.WithValue(baseCtx, coboWaas2.ContextEnv, c.env)
	baseCtx = context.WithValue(baseCtx, coboWaas2.ContextPortalSigner, crypto.Ed25519Signer{
		Secret: c.apiSecret,
	})
	if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
		ctxWithDeadline, _ := context.WithDeadline(baseCtx, deadline)
		return ctxWithDeadline
	}

	// 基于新的基础上下文创建带超时的上下文
	ctxWithTimeout, _ := context.WithTimeout(baseCtx, c.timeout)
	return ctxWithTimeout
}

// GetEnvironment 获取环境
func (c *CoboClient) GetEnvironment() int {
	return c.env
}

// IsProduction 是否为生产环境
func (c *CoboClient) IsProduction() bool {
	return c.env == coboWaas2.ProdEnv
}

// IsDevelopment 是否为开发环境
func (c *CoboClient) IsDevelopment() bool {
	return c.env == coboWaas2.DevEnv
}

// ListWallets 获取钱包列表
func (c *CoboClient) ListWallets(ctx context.Context, limit int32, before, after string) (*coboWaas2.ListWallets200Response, error) {
	// 设置默认的钱包类型和子类型
	walletType := coboWaas2.WalletType("Custodial")
	walletSubtype := coboWaas2.WalletSubtype("Asset")

	// 创建带超时的上下文
	ctxWithTimeout := c.getContextWithTimeout(ctx)

	// 使用带超时的上下文
	apiReq := c.apiClient.WalletsAPI.ListWallets(ctxWithTimeout).
		WalletType(walletType).
		WalletSubtype(walletSubtype)

	if limit > 0 {
		apiReq = apiReq.Limit(limit)
	}
	if before != "" {
		apiReq = apiReq.Before(before)
	}
	if after != "" {
		apiReq = apiReq.After(after)
	}

	// 调用API并直接返回原始响应
	resp, _, err := apiReq.Execute()
	if err != nil {
		return nil, fmt.Errorf("获取钱包列表失败: %v", err)
	}

	return resp, nil
}

// GetWalletById 获取钱包信息
func (c *CoboClient) GetWalletById(ctx context.Context, walletID string) (*coboWaas2.WalletInfo, error) {
	if walletID == "" {
		return nil, fmt.Errorf("钱包ID不能为空")
	}

	// 创建带超时的上下文
	ctxWithTimeout := c.getContextWithTimeout(ctx)

	// 调用API并直接返回原始响应
	resp, _, err := safeExecute(func() (*coboWaas2.WalletInfo, *http.Response, error) {
		return c.apiClient.WalletsAPI.GetWalletById(ctxWithTimeout, walletID).Execute()
	})
	if err != nil {
		return nil, fmt.Errorf("获取钱包信息失败: %v", err)
	}

	return resp, nil
}

func validateAPISecret(secret string) error {
	decoded, err := hex.DecodeString(secret)
	if err != nil {
		return fmt.Errorf("must be hex string: %w", err)
	}
	if len(decoded) != ed25519.SeedSize {
		return fmt.Errorf("decoded seed length must be %d bytes, got %d", ed25519.SeedSize, len(decoded))
	}
	return nil
}

func safeExecute[T any](fn func() (T, *http.Response, error)) (resp T, httpResp *http.Response, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("cobo sdk panic: %v", r)
		}
	}()
	return fn()
}

// CreateAddressRequest 创建地址请求
type CreateAddressRequest struct {
	WalletID string `json:"walletId" dc:"钱包ID"`
	ChainID  string `json:"chainId" dc:"链ID"`
	Count    int32  `json:"count,omitempty" dc:"生成数量"`
}

// CreateAddress 生成地址
func (c *CoboClient) CreateAddress(ctx context.Context, req *CreateAddressRequest) ([]coboWaas2.AddressInfo, error) {
	if req == nil {
		return nil, fmt.Errorf("请求参数不能为空")
	}

	if req.WalletID == "" {
		return nil, fmt.Errorf("钱包ID不能为空")
	}

	if req.ChainID == "" {
		return nil, fmt.Errorf("链ID不能为空")
	}

	// 设置默认数量
	count := req.Count
	if count <= 0 {
		count = 1
	}

	// 创建生成地址参数
	createAddressRequest := coboWaas2.NewCreateAddressRequest(req.ChainID, count)

	// 创建带超时的上下文
	ctxWithTimeout := c.getContextWithTimeout(ctx)

	// 调用API并直接返回原始响应
	resp, _, err := c.apiClient.WalletsAPI.CreateAddress(ctxWithTimeout, req.WalletID).CreateAddressRequest(*createAddressRequest).Execute()
	if err != nil {
		return nil, fmt.Errorf("生成地址失败: %v", err)
	}

	return resp, nil
}

// ListAddressesRequest 获取地址列表请求
type ListAddressesRequest struct {
	WalletID string `json:"walletId" dc:"钱包ID"`
	ChainID  string `json:"chainId,omitempty" dc:"链ID"`
	Limit    int32  `json:"limit,omitempty" dc:"限制数量"`
	Before   string `json:"before,omitempty" dc:"分页前"`
	After    string `json:"after,omitempty" dc:"分页后"`
}

// ListAddresses 获取地址列表
func (c *CoboClient) ListAddresses(ctx context.Context, req *ListAddressesRequest) (*coboWaas2.ListAddresses200Response, error) {
	if req == nil {
		return nil, fmt.Errorf("请求参数不能为空")
	}

	if req.WalletID == "" {
		return nil, fmt.Errorf("钱包ID不能为空")
	}

	// 创建带超时的上下文
	ctxWithTimeout := c.getContextWithTimeout(ctx)

	// 构建请求
	apiReq := c.apiClient.WalletsAPI.ListAddresses(ctxWithTimeout, req.WalletID)

	if req.ChainID != "" {
		apiReq = apiReq.ChainIds(req.ChainID)
	}
	if req.Limit > 0 {
		apiReq = apiReq.Limit(req.Limit)
	}
	if req.Before != "" {
		apiReq = apiReq.Before(req.Before)
	}
	if req.After != "" {
		apiReq = apiReq.After(req.After)
	}

	// 调用API并直接返回原始响应
	resp, _, err := apiReq.Execute()
	if err != nil {
		return nil, fmt.Errorf("获取地址列表失败: %v", err)
	}

	return resp, nil
}

// ListWebhookEndpoints 获取Webhook端点列表
func (c *CoboClient) ListWebhookEndpoints(ctx context.Context, limit int32, before, after string) (*coboWaas2.ListWebhookEndpoints200Response, error) {
	// 创建带超时的上下文
	ctxWithTimeout := c.getContextWithTimeout(ctx)

	// 构建请求
	apiReq := c.apiClient.DevelopersWebhooksAPI.ListWebhookEndpoints(ctxWithTimeout)

	if limit > 0 {
		apiReq = apiReq.Limit(limit)
	}
	if before != "" {
		apiReq = apiReq.Before(before)
	}
	if after != "" {
		apiReq = apiReq.After(after)
	}

	// 调用API并直接返回原始响应
	resp, _, err := apiReq.Execute()
	if err != nil {
		return nil, fmt.Errorf("获取Webhook端点列表失败: %v", err)
	}

	return resp, nil
}

// GetWebhookEventById 获取Webhook事件信息
func (c *CoboClient) GetWebhookEventById(ctx context.Context, eventID, endpointID string) (*coboWaas2.WebhookEvent, error) {
	if eventID == "" {
		return nil, fmt.Errorf("事件ID不能为空")
	}
	if endpointID == "" {
		return nil, fmt.Errorf("端点ID不能为空")
	}

	// 创建带超时的上下文
	ctxWithTimeout := c.getContextWithTimeout(ctx)

	// 调用API并直接返回原始响应
	resp, _, err := c.apiClient.DevelopersWebhooksAPI.GetWebhookEventById(ctxWithTimeout, eventID, endpointID).Execute()
	if err != nil {
		return nil, fmt.Errorf("获取Webhook事件信息失败: %v", err)
	}

	return resp, nil
}

// ListWebhookEventDefinitions 获取Webhook事件定义列表
func (c *CoboClient) ListWebhookEventDefinitions(ctx context.Context, limit int32, before, after string) ([]coboWaas2.ListWebhookEventDefinitions200ResponseInner, error) {
	// 创建带超时的上下文
	ctxWithTimeout := c.getContextWithTimeout(ctx)

	// 构建请求 - 这个API不支持分页参数
	apiReq := c.apiClient.DevelopersWebhooksAPI.ListWebhookEventDefinitions(ctxWithTimeout)

	// 调用API并直接返回原始响应
	resp, _, err := apiReq.Execute()
	if err != nil {
		return nil, fmt.Errorf("获取Webhook事件定义列表失败: %v", err)
	}

	return resp, nil
}

// ListSupportedChains 获取支持的链列表
func (c *CoboClient) ListSupportedChains(ctx context.Context, limit int32, before, after, chainIds string) (*coboWaas2.ListSupportedChains200Response, error) {
	// 设置默认的钱包类型和子类型
	walletType := coboWaas2.WalletType("Custodial")
	walletSubtype := coboWaas2.WalletSubtype("Asset")

	// 创建带超时的上下文
	ctxWithTimeout := c.getContextWithTimeout(ctx)

	// 使用带超时的上下文
	apiReq := c.apiClient.WalletsAPI.ListSupportedChains(ctxWithTimeout).
		WalletType(walletType).
		WalletSubtype(walletSubtype).
		ChainIds(chainIds).
		Limit(limit).
		Before(before).
		After(after)

	// 调用API并直接返回原始响应
	resp, _, err := apiReq.Execute()
	if err != nil {
		return nil, fmt.Errorf("获取支持的链列表失败: %v", err)
	}

	return resp, nil
}

// ListSupportedTokens 获取支持的代币列表
func (c *CoboClient) ListSupportedTokens(ctx context.Context, limit int32, before, after, chainIds, tokenIds string) (*coboWaas2.ListSupportedTokens200Response, error) {
	// 设置默认的钱包类型和子类型
	walletType := coboWaas2.WalletType("Custodial")
	walletSubtype := coboWaas2.WalletSubtype("Asset")

	// 创建带超时的上下文
	ctxWithTimeout := c.getContextWithTimeout(ctx)

	// 使用带超时的上下文
	apiReq := c.apiClient.WalletsAPI.ListSupportedTokens(ctxWithTimeout).
		WalletType(walletType).
		WalletSubtype(walletSubtype).
		ChainIds(chainIds).
		TokenIds(tokenIds).
		Limit(limit).
		Before(before).
		After(after)

	// 调用API并直接返回原始响应
	resp, _, err := apiReq.Execute()
	if err != nil {
		return nil, fmt.Errorf("获取支持的代币列表失败: %v", err)
	}

	return resp, nil
}

// TransferRequest 转账请求
type TransferRequest struct {
	RequestID     string `json:"request_id" dc:"请求ID"`
	WalletID      string `json:"wallet_id" dc:"钱包ID"`
	SourceAddress string `json:"source_address,omitempty" dc:"转出地址(Web3钱包必填)"`
	TokenID       string `json:"token_id" dc:"代币ID"`
	ToAddress     string `json:"to_address" dc:"目标地址"`
	Amount        string `json:"amount" dc:"转账金额"`
	GasPriceWei   string `json:"gas_price_wei,omitempty" dc:"EVM legacy gas price (wei)"`
	GasTokenID    string `json:"gas_token_id,omitempty" dc:"手续费代币ID"`
	Description   string `json:"description,omitempty" dc:"转账描述"`
}

// ListTransactionsRequest 查询交易列表请求
type ListTransactionsRequest struct {
	WalletIDs           string
	Types               string
	TokenIDs            string
	Statuses            string
	Limit               int32
	After               string
	Before              string
	MinCreatedTimestamp int64
	MaxCreatedTimestamp int64
}

// ListTransactions 查询交易列表
func (c *CoboClient) ListTransactions(ctx context.Context, req *ListTransactionsRequest) (*coboWaas2.ListTransactions200Response, error) {
	if req == nil {
		return nil, fmt.Errorf("请求参数不能为空")
	}

	ctxWithTimeout := c.getContextWithTimeout(ctx)
	apiReq := c.apiClient.TransactionsAPI.ListTransactions(ctxWithTimeout)

	if strings.TrimSpace(req.WalletIDs) != "" {
		apiReq = apiReq.WalletIds(strings.TrimSpace(req.WalletIDs))
	}
	if strings.TrimSpace(req.Types) != "" {
		apiReq = apiReq.Types(strings.TrimSpace(req.Types))
	}
	if strings.TrimSpace(req.TokenIDs) != "" {
		apiReq = apiReq.TokenIds(strings.TrimSpace(req.TokenIDs))
	}
	if strings.TrimSpace(req.Statuses) != "" {
		apiReq = apiReq.Statuses(strings.TrimSpace(req.Statuses))
	}
	if req.Limit > 0 {
		apiReq = apiReq.Limit(req.Limit)
	}
	if strings.TrimSpace(req.After) != "" {
		apiReq = apiReq.After(strings.TrimSpace(req.After))
	}
	if strings.TrimSpace(req.Before) != "" {
		apiReq = apiReq.Before(strings.TrimSpace(req.Before))
	}
	if req.MinCreatedTimestamp > 0 {
		apiReq = apiReq.MinCreatedTimestamp(req.MinCreatedTimestamp)
	}
	if req.MaxCreatedTimestamp > 0 {
		apiReq = apiReq.MaxCreatedTimestamp(req.MaxCreatedTimestamp)
	}

	resp, _, err := apiReq.Execute()
	if err != nil {
		return nil, fmt.Errorf("查询交易列表失败: %v", err)
	}
	return resp, nil
}

// ListTokenBalancesForWallet 获取钱包下代币余额
// tokenIDs 为空时返回钱包下所有代币余额；非空时按指定 token_id 过滤
func (c *CoboClient) ListTokenBalancesForWallet(ctx context.Context, walletID, tokenIDs string) (*coboWaas2.ListTokenBalancesForAddress200Response, error) {
	if strings.TrimSpace(walletID) == "" {
		return nil, fmt.Errorf("钱包ID不能为空")
	}

	ctxWithTimeout := c.getContextWithTimeout(ctx)
	apiReq := c.apiClient.WalletsAPI.ListTokenBalancesForWallet(ctxWithTimeout, walletID)
	if strings.TrimSpace(tokenIDs) != "" {
		apiReq = apiReq.TokenIds(strings.TrimSpace(tokenIDs))
	}

	resp, _, err := apiReq.Execute()
	if err != nil {
		return nil, fmt.Errorf("获取钱包代币余额失败: %v", err)
	}
	return resp, nil
}

// ListWalletSweepToAddresses 获取钱包自动归集目标地址列表
func (c *CoboClient) ListWalletSweepToAddresses(ctx context.Context, walletID string) (*coboWaas2.ListWalletSweepToAddresses200Response, error) {
	if walletID == "" {
		return nil, fmt.Errorf("钱包ID不能为空")
	}

	ctxWithTimeout := c.getContextWithTimeout(ctx)
	resp, _, err := c.apiClient.AutoSweepAPI.ListWalletSweepToAddresses(ctxWithTimeout).
		WalletId(walletID).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("获取归集目标地址失败: %v", err)
	}
	return resp, nil
}

// TransferResponse 转账响应
type TransferResponse struct {
	TransactionID string `json:"transaction_id" dc:"Cobo交易ID"`
	Status        string `json:"status" dc:"交易状态"`
}

// Transfer 发起转账（提现）
func (c *CoboClient) Transfer(ctx context.Context, req *TransferRequest) (*TransferResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("请求参数不能为空")
	}
	if req.WalletID == "" {
		return nil, fmt.Errorf("钱包ID不能为空")
	}
	if req.TokenID == "" {
		return nil, fmt.Errorf("代币ID不能为空")
	}
	if req.ToAddress == "" {
		return nil, fmt.Errorf("目标地址不能为空")
	}
	if req.Amount == "" {
		return nil, fmt.Errorf("转账金额不能为空")
	}

	// 构建转账来源：默认使用 Custodial Asset 钱包（仅 wallet_id）
	// 若显式传入 source_address，则切换到 Web3 来源
	var source coboWaas2.TransferSource
	if req.SourceAddress != "" {
		sourceType := coboWaas2.WalletSubtype("Web3")
		web3Source := &coboWaas2.CustodialWeb3TransferSource{
			SourceType: sourceType,
			WalletId:   req.WalletID,
		}
		web3Source.SetAddress(req.SourceAddress)
		source = coboWaas2.CustodialWeb3TransferSourceAsTransferSource(web3Source)
	} else {
		sourceType := coboWaas2.WalletSubtype("Asset")
		assetSource := &coboWaas2.CustodialTransferSource{
			SourceType: sourceType,
			WalletId:   req.WalletID,
		}
		source = coboWaas2.CustodialTransferSourceAsTransferSource(assetSource)
	}

	// 构建转账目标（地址）
	destinationType := coboWaas2.TransferDestinationType("Address")
	destination := coboWaas2.AddressTransferDestinationAsTransferDestination(&coboWaas2.AddressTransferDestination{
		DestinationType: destinationType,
		AccountOutput: &coboWaas2.AddressTransferDestinationAccountOutput{
			Address: req.ToAddress,
			Amount:  req.Amount,
		},
	})

	// 构建转账参数
	transferParams := coboWaas2.NewTransferParams(
		req.RequestID,
		source,
		req.TokenID,
		destination,
	)
	if req.Description != "" {
		transferParams.SetDescription(req.Description)
	}
	if req.GasPriceWei != "" {
		gasTokenID := req.GasTokenID
		if gasTokenID == "" {
			gasTokenID = "BSC_BNB"
		}
		fee := coboWaas2.TransactionRequestEvmLegacyFeeAsTransactionRequestFee(
			coboWaas2.NewTransactionRequestEvmLegacyFee(req.GasPriceWei, coboWaas2.FEETYPE_EVM_LEGACY, gasTokenID),
		)
		transferParams.SetFee(fee)
	}

	// 创建带超时的上下文
	ctxWithTimeout := c.getContextWithTimeout(ctx)

	// 调用转账API
	resp, httpResp, err := c.apiClient.TransactionsAPI.CreateTransferTransaction(ctxWithTimeout).
		TransferParams(*transferParams).
		Execute()
	if err != nil {
		// 尝试解析错误响应
		if httpResp != nil && httpResp.Body != nil {
			return nil, fmt.Errorf("发起转账失败: %v", err)
		}
		return nil, fmt.Errorf("发起转账失败: %v", err)
	}

	return &TransferResponse{
		TransactionID: resp.GetTransactionId(),
		Status:        string(resp.GetStatus()),
	}, nil
}

// GetTransactionByID 根据ID获取交易信息
func (c *CoboClient) GetTransactionByID(ctx context.Context, transactionID string) (*coboWaas2.TransactionDetail, error) {
	if transactionID == "" {
		return nil, fmt.Errorf("交易ID不能为空")
	}

	// 创建带超时的上下文
	ctxWithTimeout := c.getContextWithTimeout(ctx)

	// 调用API
	resp, _, err := c.apiClient.TransactionsAPI.GetTransactionById(ctxWithTimeout, transactionID).Execute()
	if err != nil {
		return nil, fmt.Errorf("获取交易信息失败: %v", err)
	}

	return resp, nil
}

// GetTransactionByHash 根据交易哈希获取交易信息
func (c *CoboClient) GetTransactionByHash(ctx context.Context, txHash string) (*coboWaas2.Transaction, error) {
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return nil, fmt.Errorf("交易哈希不能为空")
	}

	ctxWithTimeout := c.getContextWithTimeout(ctx)
	resp, _, err := c.apiClient.TransactionsAPI.ListTransactions(ctxWithTimeout).
		TransactionHashes(txHash).
		Types("Deposit").
		Limit(10).
		Direction("DESC").
		Execute()
	if err != nil {
		return nil, fmt.Errorf("根据交易哈希查询交易失败: %v", err)
	}

	targetHash := strings.ToLower(txHash)
	for _, tx := range resp.GetData() {
		if tx.TransactionHash == nil {
			continue
		}
		if strings.ToLower(strings.TrimSpace(*tx.TransactionHash)) == targetHash {
			return &tx, nil
		}
	}

	return nil, nil
}
