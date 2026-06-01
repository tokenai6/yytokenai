package consts

import "github.com/shopspring/decimal"

// TeamDifferentialRewardRates 团队极差奖励（基于伞下APG静态产出）权益百分比配置
var TeamDifferentialRewardRates = map[int]string{
	1: "0.15", // V1: 15%
	2: "0.20", // V2: 20%
	3: "0.25", // V3: 25%
	4: "0.30", // V4: 30%
	5: "0.35", // V5: 35%
	6: "0.40", // V6: 40%
	7: "0.45", // V7: 45%
	8: "0.50", // V8: 50%
	9: "0.55", // V9: 55%
}

// GetTeamDifferentialRewardRate 获取团队极差奖励权益百分比
func GetTeamDifferentialRewardRate(level int) decimal.Decimal {
	if rateStr, ok := TeamDifferentialRewardRates[level]; ok {
		rate, _ := decimal.NewFromString(rateStr)
		return rate
	}
	return decimal.Zero
}
