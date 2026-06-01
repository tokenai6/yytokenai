package consts

import "github.com/shopspring/decimal"

// ============ 奖励系统配置常量 ============

// VIP等级奖励比例配置
var VIPRewardRates = map[int]string{
	0: "0",    // VIP0: 0%
	1: "0.30", // VIP1: 30%
	2: "0.40", // VIP2: 40%
	3: "0.50", // VIP3: 50%
	4: "0.60", // VIP4: 60%
	5: "0.70", // VIP5: 70%
	6: "0.80", // VIP6: 80%
	7: "0.85", // VIP7: 85%
	8: "0.90", // VIP8: 90%
	9: "0.95", // VIP9: 95%
}

// GetVIPRewardRate 获取VIP等级对应的奖励比例
func GetVIPRewardRate(level int) decimal.Decimal {
	if rateStr, ok := VIPRewardRates[level]; ok {
		rate, _ := decimal.NewFromString(rateStr)
		return rate
	}
	return decimal.Zero
}

// ============ 加权奖励配置 ============

// WeightedRewardPoolRate 加权奖励分红池比例（全网产出的5%）
const WeightedRewardPoolRate = "0.0015"

// WeightedRewardTop50Count 加权奖励排名前N名
const WeightedRewardTop50Count = 50

// GetWeightedRewardPoolRate 获取加权奖励分红池比例
func GetWeightedRewardPoolRate() decimal.Decimal {
	rate, _ := decimal.NewFromString(WeightedRewardPoolRate)
	return rate
}

// ============ 业绩计算配置 ============

// PowerValueToPerformanceRate 算力值转业绩比例（算力值 / 2 = 业绩）
const PowerValueToPerformanceRate = "2"

// GetPowerValueToPerformanceDivisor 获取算力值转业绩的除数
func GetPowerValueToPerformanceDivisor() decimal.Decimal {
	divisor, _ := decimal.NewFromString(PowerValueToPerformanceRate)
	return divisor
}

// ============ 业绩门槛配置 ============

// MinPerformanceThreshold 最低业绩门槛（100u）
const MinPerformanceThreshold = "100"

// GetMinPerformanceThreshold 获取最低业绩门槛
func GetMinPerformanceThreshold() decimal.Decimal {
	threshold, _ := decimal.NewFromString(MinPerformanceThreshold)
	return threshold
}

// ============ 静态收益配置（作废，使用staking_package.daily_yield_rate） ============

// StaticRewardDailyRate 静态收益日利率（1%）
const StaticRewardDailyRate_DEPRECATED = "0.01"

// GetStaticRewardDailyRate 获取静态收益日利率
func GetStaticRewardDailyRate() decimal.Decimal {
	rate, _ := decimal.NewFromString(StaticRewardDailyRate_DEPRECATED)
	return rate
}

// ============ 推荐奖励配置 ============

// ReferralRewardRate 推荐奖励比例（直推下级静态收益的1%）
const ReferralRewardRate = "0.01"

// GetReferralRewardRate 获取推荐奖励比例
func GetReferralRewardRate() decimal.Decimal {
	rate, _ := decimal.NewFromString(ReferralRewardRate)
	return rate
}

// ============ 节点分红配置 ============

// NodeDividendPoolRate 节点分红池比例（APG卖出总额的5%）
const NodeDividendPoolRate = "0.05"

// GetNodeDividendPoolRate 获取节点分红池比例
func GetNodeDividendPoolRate() decimal.Decimal {
	rate, _ := decimal.NewFromString(NodeDividendPoolRate)
	return rate
}

// ============ 出局配置 ============

// MaxStaticReleaseMultiplier 最大静态释放倍数（本金的2倍）
const MaxStaticReleaseMultiplier = "2.0"

// MaxQuotaMultiplier 最大额度倍数（本金的2倍）
const MaxQuotaMultiplier = "2.0"

// GetMaxStaticReleaseMultiplier 获取最大静态释放倍数
func GetMaxStaticReleaseMultiplier() decimal.Decimal {
	multiplier, _ := decimal.NewFromString(MaxStaticReleaseMultiplier)
	return multiplier
}

// GetMaxQuotaMultiplier 获取最大额度倍数
func GetMaxQuotaMultiplier() decimal.Decimal {
	multiplier, _ := decimal.NewFromString(MaxQuotaMultiplier)
	return multiplier
}

// ============ VIP等级评定配置 ============

// VIP等级评定所需小区业绩（从高到低排序，便于判断）
// 等级展示名称（与管理端保持一致）：
// 0 -> 无等级
// 1 -> 白钻
// 2 -> 黄钻
// 3 -> 红钻
// 4 -> 蓝钻
// 5 -> 黑钻
var VIPLevelRequirements = []struct {
	Level       int
	Requirement string
}{
	{9, "35000000"}, // VIP9: 3500万
	{8, "15000000"}, // VIP8: 1500万
	{7, "6000000"},  // VIP7: 600万
	{6, "2000000"},  // VIP6: 200万
	{5, "600000"},   // VIP5: 60万
	{4, "200000"},   // VIP4: 20万
	{3, "60000"},    // VIP3: 6万
	{2, "20000"},    // VIP2: 2万
	{1, "5000"},     // VIP1: 5千
}

// GetVIPLevel 根据小区业绩评定VIP等级
func GetVIPLevel(districtPerformance decimal.Decimal) int {
	for _, req := range VIPLevelRequirements {
		requirement, _ := decimal.NewFromString(req.Requirement)
		if districtPerformance.GreaterThanOrEqual(requirement) {
			return req.Level
		}
	}
	return 0 // VIP0
}
