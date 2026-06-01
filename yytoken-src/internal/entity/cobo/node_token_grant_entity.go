package cobo

import (
	"time"

	"github.com/shopspring/decimal"
)

var asiaShanghaiLoc = mustLoadAsiaShanghaiLoc()

func mustLoadAsiaShanghaiLoc() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}

// NodeTokenGrantStatus 发放状态
const (
	NodeTokenGrantStatusPending = 0 // 待发放
	NodeTokenGrantStatusSent    = 1 // 已发放
	NodeTokenGrantStatusFailed  = 2 // 发放失败
)

// TokenType 发放类型
const (
	TokenTypeYYAI   = "YYAI"   // YYAI代币
	TokenTypeTriple = "Triple" // 三倍券
)

// tripleCouponRateConfigs 定义各时间阶段的三倍券赠送比例
//
// 生效规则（当天 0 点起）：
// | 节点档位 | 2026-04-20 前 | 2026-04-20 起 | 2026-05-01 起 | 2026-05-11 起 |
// |----------|---------------|---------------|---------------|---------------|
// | 1 (1000U)   | 10%  | 7%    | 4%   | 0% |
// | 2 (5000U)   | 15%  | 10.5% | 6%   | 0% |
// | 3 (10000U)  | 18%  | 12.6% | 7.2% | 0% |
// | 4 (30000U)  | 20%  | 14%   | 8%   | 0% |
// | 5 (100000U) | 25%  | 0%    | 0%   | 0% |
var tripleCouponRateConfigs = []struct {
	startAt time.Time
	rates   map[int]float64
}{
	{
		// 2026-04-20 前：沿用当前比例 1→10%, 2→15%, 3→18%, 4→20%, 5→25%
		startAt: time.Time{},
		rates: map[int]float64{
			1: 0.10,
			2: 0.15,
			3: 0.18,
			4: 0.20,
			5: 0.25,
		},
	},
	{
		// 2026-04-20 起：1→7%, 2→10.5%, 3→12.6%, 4→14%, 5→0%(停止售卖)
		startAt: time.Date(2026, 4, 20, 0, 0, 0, 0, asiaShanghaiLoc),
		rates: map[int]float64{
			1: 0.07,
			2: 0.105,
			3: 0.126,
			4: 0.14,
			5: 0.0,
		},
	},
	{
		// 2026-05-01 起：1→4%, 2→6%, 3→7.2%, 4→8%, 5→0%
		startAt: time.Date(2026, 5, 1, 0, 0, 0, 0, asiaShanghaiLoc),
		rates: map[int]float64{
			1: 0.04,
			2: 0.06,
			3: 0.072,
			4: 0.08,
			5: 0.0,
		},
	},
	{
		// 2026-05-11 起：全部停止发放三倍券
		startAt: time.Date(2026, 5, 11, 0, 0, 0, 0, asiaShanghaiLoc),
		rates: map[int]float64{
			1: 0.0,
			2: 0.0,
			3: 0.0,
			4: 0.0,
			5: 0.0,
		},
	},
}

// GetTripleCouponRate 根据购买时间获取对应节点档位的三倍券赠送比例
func GetTripleCouponRate(nodeType int, t time.Time) float64 {
	var rate float64
	for _, cfg := range tripleCouponRateConfigs {
		if !t.Before(cfg.startAt) {
			if r, ok := cfg.rates[nodeType]; ok {
				rate = r
			}
		}
	}
	return rate
}

// NodeTokenGrantEntity 节点购买Token发放记录 (YYAI/Triple)
type NodeTokenGrantEntity struct {
	ID          int64           `json:"id" orm:"id"`
	PurchaseID  int64           `json:"purchase_id" orm:"purchase_id"`
	UserID      int64           `json:"user_id" orm:"user_id"`
	Wallet      string          `json:"wallet" orm:"wallet"`
	TokenType   string          `json:"token_type" orm:"token_type"`   // 发放类型: YYAI/Triple
	TokenSymbol string          `json:"token_symbol" orm:"token_symbol"`
	TokenChain  string          `json:"token_chain" orm:"token_chain"`
	TokenAddr   string          `json:"token_addr" orm:"token_addr"`
	GrantAmount string          `json:"grant_amount" orm:"grant_amount"`
	Status      int             `json:"status" orm:"status"`
	TxHash      string          `json:"tx_hash" orm:"tx_hash"`
	ErrorMsg    string          `json:"error_msg" orm:"error_msg"`
	CreatedAt   time.Time       `json:"created_at" orm:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" orm:"updated_at"`
	// YYAI价格相关字段
	YyaiPrice   decimal.Decimal `json:"yyai_price" orm:"yyai_price"`   // 发放时YYAI价格(USD)
	UsdtAmount  decimal.Decimal `json:"usdt_amount" orm:"usdt_amount"` // 购买金额(USDT)
	YyaiAmount  decimal.Decimal `json:"yyai_amount" orm:"yyai_amount"` // 实际YYAI发放数量
}

// TableName 表名
func (e *NodeTokenGrantEntity) TableName() string {
	return "cobo_node_token_grant"
}
