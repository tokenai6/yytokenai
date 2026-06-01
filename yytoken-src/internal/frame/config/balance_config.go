package config

import (
	"context"

	"XWFrame/internal/frame/consts"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// BalanceConfig 资金系统配置
type BalanceConfig struct {
	// 兑换配置
	Exchange ExchangeConfig `json:"exchange"`
	// 充值配置
	Recharge RechargeConfig `json:"recharge"`
}

// ExchangeConfig 兑换配置
type ExchangeConfig struct {
	// 手续费类型 1-固定值 2-百分比
	FeeType int `json:"feeType"`
	// 手续费值
	FeeValue decimal.Decimal `json:"feeValue"`
	// 兑换汇率映射（key: "REWARD_to_USDT", value: 汇率）
	ExchangeRates map[string]decimal.Decimal `json:"exchangeRates"`
}

// RechargeConfig 充值配置
type RechargeConfig struct {
	// 最小充值金额
	MinAmount decimal.Decimal `json:"minAmount"`
	// 需要的区块确认数
	RequiredConfirmations int `json:"requiredConfirmations"`
}

// GetBalanceConfig 获取资金系统配置
func GetBalanceConfig() *BalanceConfig {
	ctx := context.TODO()

	// 默认配置
	config := &BalanceConfig{
		Exchange: ExchangeConfig{
			FeeType:  consts.ExchangeFeeTypePercent,
			FeeValue: decimal.NewFromFloat(consts.ExchangeFeeValueDefault),
			ExchangeRates: func() map[string]decimal.Decimal {
				m := make(map[string]decimal.Decimal, len(consts.DefaultExchangeRates))
				for k, v := range consts.DefaultExchangeRates {
					m[k] = decimal.NewFromFloat(v)
				}
				return m
			}(),
		},
		Recharge: RechargeConfig{
			MinAmount:             decimal.NewFromFloat(1), // 最小充值1
			RequiredConfirmations: 12,                      // 需要12个确认
		},
	}

	// 从配置文件读取（如果存在）
	if g.Cfg().Available(ctx) {
		// 兑换配置：暂时使用常量，忽略配置文件覆盖

		// 充值配置
		if minAmount := g.Cfg().MustGet(ctx, "balance.recharge.minAmount").Float64(); minAmount > 0 {
			config.Recharge.MinAmount = decimal.NewFromFloat(minAmount)
		}
		if confirmations := g.Cfg().MustGet(ctx, "balance.recharge.requiredConfirmations").Int(); confirmations > 0 {
			config.Recharge.RequiredConfirmations = confirmations
		}
	}

	return config
}

// CalculateFee 计算手续费
func CalculateFee(amount decimal.Decimal, feeType int, feeValue decimal.Decimal) decimal.Decimal {
	if feeType == 1 {
		// 固定值
		return feeValue
	} else if feeType == 2 {
		// 百分比
		return amount.Mul(feeValue)
	}
	return decimal.Zero
}

// GetExchangeRate 获取兑换汇率
func GetExchangeRate(fromSymbol, toSymbol string) decimal.Decimal {
	cfg := GetBalanceConfig()

	// 构造配置键：from_to_to
	key := fromSymbol + "_to_" + toSymbol

	// 从配置中获取
	if rate, exists := cfg.Exchange.ExchangeRates[key]; exists {
		return rate
	}

	// 默认返回 1:1
	return decimal.NewFromInt(1)
}
