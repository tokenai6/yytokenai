package reward

import (
	"time"

	"github.com/shopspring/decimal"
)

// ApgPriceStabilizeLogEntity APG价格调控日志实体
type ApgPriceStabilizeLogEntity struct {
	Id int64 `json:"id" orm:"id,primary"`

	// 价格数据
	PriceChangeBps int             `json:"price_change_bps" orm:"price_change_bps"` // 涨跌幅(BP) 100=1%
	ApgPriceBefore decimal.Decimal `json:"apg_price_before" orm:"apg_price_before"` // 调控前APG价格
	ApgPriceAfter  decimal.Decimal `json:"apg_price_after" orm:"apg_price_after"`   // 调控后APG价格

	// 交易信息
	TxHash      *string `json:"tx_hash" orm:"tx_hash"`           // 交易哈希 (失败时为NULL)
	BlockNumber int64   `json:"block_number" orm:"block_number"` // 区块号
	GasUsed     int64   `json:"gas_used" orm:"gas_used"`         // 消耗的Gas
	GasPrice    int64   `json:"gas_price" orm:"gas_price"`       // Gas价格(Wei)

	// 状态信息
	Status   string `json:"status" orm:"status"`       // pending/success/failed
	ErrorMsg string `json:"error_msg" orm:"error_msg"` // 错误信息

	// 时间戳
	CreatedAt   time.Time  `json:"created_at" orm:"created_at"`
	ConfirmedAt *time.Time `json:"confirmed_at" orm:"confirmed_at"`
	UpdatedAt   time.Time  `json:"updated_at" orm:"updated_at"`
}
