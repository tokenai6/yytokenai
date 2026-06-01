package blockchain

import (
	"XWFrame/internal/frame/consts"
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// EventListenerConfig 事件监听服务配置
type EventListenerConfig struct {
	// 网络配置（以太坊标准）
	ChainID     int64  // 链ID
	ChainName   string // 链名称
	NetworkType string // 网络类型

	// RPC节点配置
	RPCEndpoints []string // RPC节点列表

	// 合约地址
	ContractUSDT                string // USDT合约地址
	ContractREX                 string // REX代币地址
	ContractAPG                 string // APG合约地址
	ContractStaking             string // 质押合约地址
	ContractMarketCapController string // 市值控制合约地址
	ContractCombinationStaking  string // 组合质押合约地址
	ContractMintStake           string // APG Mint质押合约地址
	ContractMintGroup           string // APG 拼团合约地址
	ContractTokenStoreBurn      string // TokenStoreBurn 合约地址
	ContractYYToken             string // YY Token 合约地址
	ContractYYAIToken           string // YYAI Token 合约地址

	// 节点分红地址配置
	TreasuryAddress     string // 国库合约地址
	NodeDividendAddress string // 节点收税地址

	// 代币精度
	Decimals int // 代币精度

	// 扫块配置
	Interval          time.Duration // 执行间隔
	StartBlock        uint64        // 起始区块
	ConfirmBlocks     int           // 区块确认数
	BatchSize         int           // 批量查询区块数量
	MaxBlocksPerQuery int           // 单次查询最大区块范围
	EnableReorgCheck  bool          // 启用重组检查
	ReorgDepth        int           // 重组检查深度

	// 重试配置
	MaxRetry    int           // 最大重试次数
	RetryDelay  time.Duration // 重试延迟
	AutoRestart bool          // 自动重启

	// 监控配置
	HeartbeatInterval time.Duration // 心跳间隔
	EnableMetrics     bool          // 启用监控指标

	// 超时配置
	RPCTimeout        time.Duration // RPC调用超时
	BlockQueryTimeout time.Duration // 区块查询超时
	EventQueryTimeout time.Duration // 事件查询超时

	// 限流配置
	RateLimitEnabled  bool // 启用限流
	RequestsPerSecond int  // 每秒请求数
	Burst             int  // 突发请求数
}

// LoadEventListenerConfig 加载事件监听配置（自动使用默认值）
func LoadEventListenerConfig(ctx context.Context) (*EventListenerConfig, error) {
	cfg := &EventListenerConfig{}

	// ==========================================
	// 必须配置（用户必须提供，无默认值）
	// ==========================================
	cfg.ChainID = g.Cfg().MustGet(ctx, "blockchain.network.chain_id").Int64()
	cfg.ChainName = g.Cfg().MustGet(ctx, "blockchain.network.chain_name").String()

	if cfg.ChainID == 0 || cfg.ChainName == "" {
		return nil, fmt.Errorf("blockchain.network.chain_id 和 chain_name 是必须配置")
	}

	// RPC节点列表
	cfg.RPCEndpoints = g.Cfg().MustGet(ctx, "blockchain.rpc_endpoints").Strings()
	if len(cfg.RPCEndpoints) == 0 {
		return nil, fmt.Errorf("blockchain.rpc_endpoints 是必须配置")
	}

	// 合约地址
	cfg.ContractUSDT = g.Cfg().MustGet(ctx, "blockchain.contracts.usdt_token").String()
	cfg.ContractREX = g.Cfg().MustGet(ctx, "blockchain.contracts.rex_token").String()
	cfg.ContractAPG = g.Cfg().MustGet(ctx, "blockchain.contracts.apg_token").String()
	cfg.ContractStaking = g.Cfg().MustGet(ctx, "blockchain.contracts.ecosystem_staking").String()
	cfg.ContractMarketCapController = g.Cfg().MustGet(ctx, "blockchain.contracts.market_cap_controller").String()
	cfg.ContractCombinationStaking = g.Cfg().MustGet(ctx, "blockchain.contracts.combination_staking").String()
	cfg.ContractMintStake = g.Cfg().MustGet(ctx, "blockchain.contracts.mint_stake").String()
	cfg.ContractMintGroup = g.Cfg().MustGet(ctx, "blockchain.contracts.mint_group").String()
	cfg.ContractTokenStoreBurn = g.Cfg().MustGet(ctx, "blockchain.contracts.token_store_burn").String()
	cfg.ContractYYToken = g.Cfg().MustGet(ctx, "blockchain.contracts.yy_token").String()
	cfg.ContractYYAIToken = g.Cfg().MustGet(ctx, "blockchain.contracts.yyai_token").String()

	// 节点分红地址配置
	cfg.TreasuryAddress = g.Cfg().MustGet(ctx, "blockchain.node_dividend.treasury_address").String()
	cfg.NodeDividendAddress = g.Cfg().MustGet(ctx, "blockchain.node_dividend.node_dividend_address").String()

	if cfg.ContractAPG == "" || cfg.ContractStaking == "" {
		return nil, fmt.Errorf("blockchain.contracts.apg_token 和 ecosystem_staking 是必须配置")
	}

	if cfg.TreasuryAddress == "" || cfg.NodeDividendAddress == "" {
		return nil, fmt.Errorf("blockchain.node_dividend.treasury_address 和 node_dividend_address 是必须配置")
	}

	// 起始区块（必须配置）
	cfg.StartBlock = g.Cfg().MustGet(ctx, "blockchain.block_scanner.start_block").Uint64()

	// ==========================================
	// 可选配置（使用常量作为默认值）
	// ==========================================

	// 网络类型
	cfg.NetworkType = g.Cfg().MustGet(ctx, "blockchain.network.network_type", consts.BlockchainDefaultNetworkType).String()

	// 代币精度
	cfg.Decimals = g.Cfg().MustGet(ctx, "blockchain.decimals.default", consts.BlockchainDefaultDecimals).Int()

	// 扫块配置
	cfg.Interval = time.Duration(
		g.Cfg().MustGet(ctx, "blockchain.block_scanner.interval", consts.BlockchainDefaultScanInterval).Int(),
	) * time.Second

	cfg.ConfirmBlocks = g.Cfg().MustGet(ctx,
		"blockchain.block_scanner.confirmation_blocks", consts.BlockchainDefaultConfirmationBlocks).Int()

	cfg.BatchSize = g.Cfg().MustGet(ctx,
		"blockchain.block_scanner.batch_size", consts.BlockchainDefaultBatchSize).Int()

	cfg.MaxBlocksPerQuery = g.Cfg().MustGet(ctx,
		"blockchain.block_scanner.max_blocks_per_query", consts.BlockchainDefaultMaxBlocksPerQuery).Int()

	cfg.EnableReorgCheck = g.Cfg().MustGet(ctx,
		"blockchain.block_scanner.enable_reorg_check", consts.BlockchainDefaultEnableReorgCheck).Bool()

	cfg.ReorgDepth = g.Cfg().MustGet(ctx,
		"blockchain.block_scanner.reorg_depth", consts.BlockchainDefaultReorgDepth).Int()

	// 重试配置
	cfg.MaxRetry = g.Cfg().MustGet(ctx,
		"blockchain.event_listener.max_retry", consts.BlockchainDefaultMaxRetry).Int()

	cfg.RetryDelay = time.Duration(
		g.Cfg().MustGet(ctx, "blockchain.event_listener.retry_delay", consts.BlockchainDefaultRetryDelay).Int(),
	) * time.Second

	cfg.AutoRestart = g.Cfg().MustGet(ctx,
		"blockchain.event_listener.auto_restart", consts.BlockchainDefaultAutoRestart).Bool()

	// 监控配置
	cfg.HeartbeatInterval = time.Duration(
		g.Cfg().MustGet(ctx, "blockchain.event_listener.heartbeat_interval", consts.BlockchainDefaultHeartbeatInterval).Int(),
	) * time.Second

	cfg.EnableMetrics = g.Cfg().MustGet(ctx,
		"blockchain.event_listener.enable_metrics", consts.BlockchainDefaultEnableMetrics).Bool()

	// 超时配置
	cfg.RPCTimeout = time.Duration(
		g.Cfg().MustGet(ctx, "blockchain.timeouts.rpc_call", consts.BlockchainDefaultRPCTimeout).Int(),
	) * time.Second

	cfg.BlockQueryTimeout = time.Duration(
		g.Cfg().MustGet(ctx, "blockchain.timeouts.block_query", consts.BlockchainDefaultBlockQueryTimeout).Int(),
	) * time.Second

	cfg.EventQueryTimeout = time.Duration(
		g.Cfg().MustGet(ctx, "blockchain.timeouts.event_query", consts.BlockchainDefaultEventQueryTimeout).Int(),
	) * time.Second

	// 限流配置
	cfg.RateLimitEnabled = g.Cfg().MustGet(ctx,
		"blockchain.rate_limit.enabled", consts.BlockchainDefaultRateLimitEnabled).Bool()

	cfg.RequestsPerSecond = g.Cfg().MustGet(ctx,
		"blockchain.rate_limit.requests_per_second", consts.BlockchainDefaultRequestsPerSecond).Int()

	cfg.Burst = g.Cfg().MustGet(ctx,
		"blockchain.rate_limit.burst", consts.BlockchainDefaultBurst).Int()

	return cfg, nil
}

// GetTokenDecimals 获取特定代币精度（使用默认值）
func GetTokenDecimals(ctx context.Context, tokenType string) int {
	configKey := fmt.Sprintf("blockchain.decimals.%s", tokenType)
	return g.Cfg().MustGet(ctx, configKey, consts.BlockchainDefaultDecimals).Int()
}
