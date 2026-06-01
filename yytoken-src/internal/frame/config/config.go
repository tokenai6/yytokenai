package config

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
)

// CoboConfig Cobo钱包配置
type CoboConfig struct {
	Env             string             `yaml:"env" dc:"环境：dev/prod"`
	APISecret       string             `yaml:"apiSecret" dc:"API密钥"`
	WalletID        string             `yaml:"walletId" dc:"钱包ID"`
	Timeout         int                `yaml:"timeout" dc:"超时时间(秒)"`
	PubKeyDev       string             `yaml:"pubKeyDev" dc:"开发环境公钥"`
	PubKeyProd      string             `yaml:"pubKeyProd" dc:"生产环境公钥"`
	SupportedTokens []string           `yaml:"supportedTokens" dc:"支持的代币列表"`
	Webhook         CoboWebhookConfig  `yaml:"webhook" dc:"Webhook配置"`
	Recharge        CoboRechargeConfig `yaml:"recharge" dc:"充值配置"`
	Withdraw        CoboWithdrawConfig `yaml:"withdraw" dc:"提现风控配置"`
}

// CoboWebhookConfig Webhook配置
type CoboWebhookConfig struct {
	Enabled     bool     `yaml:"enabled" dc:"是否启用"`
	CallbackURL string   `yaml:"callback_url" dc:"回调URL"`
	Path        string   `yaml:"path" dc:"接收路径"`
	Secret      string   `yaml:"secret" dc:"验签密钥"`
	AllowedIPs  []string `yaml:"allowed_ips" dc:"IP白名单"`
}

// CoboRechargeConfig 充值配置
type CoboRechargeConfig struct {
	MinConfirmations int  `yaml:"min_confirmations" dc:"最小确认数"`
	AutoConfirm      bool `yaml:"auto_confirm" dc:"自动入账"`
}

// RiskRulesConfig 风控规则配置
type RiskRulesConfig struct {
	DailyMaxCount          int     `yaml:"daily_max_count" dc:"每日最多提现次数"`
	DailyMaxAmount         float64 `yaml:"daily_max_amount" dc:"每日最多提现金额"`
	HourlyMaxCount         int     `yaml:"hourly_max_count" dc:"每小时最多提现次数"`
	MinAmount              float64 `yaml:"min_amount" dc:"最小提现金额"`
	MaxAmount              float64 `yaml:"max_amount" dc:"最大提现金额"`
	NewUserDays            int     `yaml:"new_user_days" dc:"新用户限制天数"`
	NewUserDailyMax        float64 `yaml:"new_user_daily_max" dc:"新用户每日限额"`
	ConsecutiveSmallCount  int     `yaml:"consecutive_small_count" dc:"连续小额次数阈值"`
	ConsecutiveSmallAmount float64 `yaml:"consecutive_small_amount" dc:"小额定义金额"`
}

// CoboWithdrawConfig 提现风控配置
type CoboWithdrawConfig struct {
	WalletID             string          `yaml:"wallet_id" dc:"提现钱包ID(可选，未配置时复用cobo.walletId)"`
	AutoApproveThreshold float64         `yaml:"auto_approve_threshold" dc:"自动审核阈值"`
	FeeRate              float64         `yaml:"fee_rate" dc:"提现手续费费率（如0.1=10%）"`
	GasPriceWei          string          `yaml:"gas_price_wei" dc:"EVM legacy gas price (wei)"`
	GasTokenID           string          `yaml:"gas_token_id" dc:"手续费代币ID"`
	RiskRules            RiskRulesConfig `yaml:"risk_rules" dc:"风控规则"`
}

// GoogleAuthConfig 谷歌验证码配置
type GoogleAuthConfig struct {
	Enabled bool   `yaml:"enabled" dc:"是否启用谷歌验证码"`
	Secret  string `yaml:"secret" dc:"16位密钥（Base32编码）"`
}

var (
	GlobalConfig     *gcfg.Config
	CustomConfigFile string
)

// Init 初始化配置
func Init(ctx context.Context) error {
	// 确定配置文件名称
	configFile := getConfigFileName()

	// 使用Go-Frame的配置管理
	cfg := g.Cfg()

	// 设置主配置文件
	if adapter, ok := cfg.GetAdapter().(*gcfg.AdapterFile); ok {
		adapter.SetFileName(configFile)
	}

	g.Log().Info(ctx, fmt.Sprintf("使用配置文件: %s", configFile))

	GlobalConfig = cfg

	g.Log().Info(ctx, "配置系统初始化完成")
	return nil
}

// getConfigFileName 获取配置文件名称
// 优先级: CustomConfigFile > config.local.yaml > config.yaml
func getConfigFileName() string {
	if CustomConfigFile != "" {
		return CustomConfigFile
	}
	// 检查是否存在本地配置文件
	if fileExists("config.local.yaml") {
		return "config.local.yaml"
	}
	return "config.yaml"
}

// fileExists 检查文件是否存在
func fileExists(filename string) bool {
	adapter := g.Cfg().GetAdapter()
	if fileAdapter, ok := adapter.(*gcfg.AdapterFile); ok {
		available := fileAdapter.Available(context.Background(), filename)
		return available
	}
	return false
}

// GetConfig 获取全局配置
func GetConfig() *gcfg.Config {
	return GlobalConfig
}

// GetCoboConfig 获取Cobo配置
func GetCoboConfig(ctx context.Context) (*CoboConfig, error) {
	cfg := GlobalConfig
	if cfg == nil {
		cfg = g.Cfg()
	}
	if cfg == nil {
		return nil, fmt.Errorf("配置未初始化")
	}

	var coboConfig CoboConfig
	coboConfig.Env = cfg.MustGet(ctx, "cobo.env").String()
	coboConfig.APISecret = cfg.MustGet(ctx, "cobo.apiSecret").String()
	coboConfig.WalletID = cfg.MustGet(ctx, "cobo.walletId").String()
	coboConfig.Timeout = cfg.MustGet(ctx, "cobo.timeout").Int()
	coboConfig.PubKeyDev = cfg.MustGet(ctx, "cobo.pubKeyDev").String()
	coboConfig.PubKeyProd = cfg.MustGet(ctx, "cobo.pubKeyProd").String()

	// 解析支持的代币列表
	supportedTokens := cfg.MustGet(ctx, "cobo.supportedTokens").Interfaces()
	for _, token := range supportedTokens {
		if tokenStr, ok := token.(string); ok {
			coboConfig.SupportedTokens = append(coboConfig.SupportedTokens, tokenStr)
		}
	}

	// Webhook配置
	coboConfig.Webhook = CoboWebhookConfig{
		Enabled:     cfg.MustGet(ctx, "cobo.webhook.enabled").Bool(),
		CallbackURL: cfg.MustGet(ctx, "cobo.webhook.callback_url").String(),
		Path:        cfg.MustGet(ctx, "cobo.webhook.path").String(),
		Secret:      cfg.MustGet(ctx, "cobo.webhook.secret").String(),
	}

	// 充值配置（默认值）
	coboConfig.Recharge = CoboRechargeConfig{
		MinConfirmations: cfg.MustGet(ctx, "cobo.recharge.min_confirmations").Int(),
		AutoConfirm:      cfg.MustGet(ctx, "cobo.recharge.auto_confirm").Bool(),
	}
	if coboConfig.Recharge.MinConfirmations <= 0 {
		coboConfig.Recharge.MinConfirmations = 6
	}

	// 提现风控配置
	coboConfig.Withdraw = CoboWithdrawConfig{
		WalletID:             cfg.MustGet(ctx, "cobo.withdraw.wallet_id").String(),
		AutoApproveThreshold: cfg.MustGet(ctx, "cobo.withdraw.auto_approve_threshold").Float64(),
		FeeRate:              cfg.MustGet(ctx, "cobo.withdraw.fee_rate").Float64(),
		GasPriceWei:          cfg.MustGet(ctx, "cobo.withdraw.gas_price_wei").String(),
		GasTokenID:           cfg.MustGet(ctx, "cobo.withdraw.gas_token_id").String(),
		RiskRules: RiskRulesConfig{
			DailyMaxCount:          cfg.MustGet(ctx, "cobo.withdraw.risk_rules.daily_max_count").Int(),
			DailyMaxAmount:         cfg.MustGet(ctx, "cobo.withdraw.risk_rules.daily_max_amount").Float64(),
			HourlyMaxCount:         cfg.MustGet(ctx, "cobo.withdraw.risk_rules.hourly_max_count").Int(),
			MinAmount:              cfg.MustGet(ctx, "cobo.withdraw.risk_rules.min_amount").Float64(),
			MaxAmount:              cfg.MustGet(ctx, "cobo.withdraw.risk_rules.max_amount").Float64(),
			NewUserDays:            cfg.MustGet(ctx, "cobo.withdraw.risk_rules.new_user_days").Int(),
			NewUserDailyMax:        cfg.MustGet(ctx, "cobo.withdraw.risk_rules.new_user_daily_max").Float64(),
			ConsecutiveSmallCount:  cfg.MustGet(ctx, "cobo.withdraw.risk_rules.consecutive_small_count").Int(),
			ConsecutiveSmallAmount: cfg.MustGet(ctx, "cobo.withdraw.risk_rules.consecutive_small_amount").Float64(),
		},
	}
	if coboConfig.Withdraw.FeeRate <= 0 {
		coboConfig.Withdraw.FeeRate = 0
	}
	if coboConfig.Withdraw.WalletID == "" {
		coboConfig.Withdraw.WalletID = coboConfig.WalletID
	}

	return &coboConfig, nil
}

// GetGoogleAuthConfig 获取谷歌验证码配置
func GetGoogleAuthConfig(ctx context.Context) (*GoogleAuthConfig, error) {
	if GlobalConfig == nil {
		return nil, fmt.Errorf("配置未初始化")
	}

	var googleAuthConfig GoogleAuthConfig
	// MustGet 在配置不存在时，Bool() 返回 false，String() 返回空字符串
	// 这确保了配置不存在时，默认不开启谷歌验证码
	googleAuthConfig.Enabled = GlobalConfig.MustGet(ctx, "googleAuth.enabled").Bool()
	googleAuthConfig.Secret = GlobalConfig.MustGet(ctx, "googleAuth.secret").String()

	return &googleAuthConfig, nil
}
