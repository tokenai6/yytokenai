package entity

import (
	"time"
)

// TokenConfigEntity 代币配置实体
type TokenConfigEntity struct {
	Id                int64     `json:"id" orm:"id,primary"`
	Symbol            string    `json:"symbol" orm:"symbol"`                             // 代币符号，如 USDT
	Name              string    `json:"name" orm:"name"`                                 // 显示名称
	ContractAddress   string    `json:"contract_address" orm:"contract_address"`         // 合约地址
	Decimals          int       `json:"decimals" orm:"decimals"`                         // 精度
	TokenImgUrl       string    `json:"token_img_url" orm:"token_img_url"`               // 代币图标URL
	MinWithdrawAmount float64   `json:"min_withdraw_amount" orm:"min_withdraw_amount"`   // 最低提现金额
	SortOrder         int       `json:"sort_order" orm:"sort_order"`                     // 排序
	IsEnabled         bool      `json:"is_enabled" orm:"is_enabled"`                     // 是否启用
	DetectByHotWallet bool      `json:"detect_by_hot_wallet" orm:"detect_by_hot_wallet"` // 是否由 hotwallet 路径监听入账
	PriceByHotwallet  bool      `json:"price_by_hotwallet" orm:"price_by_hotwallet"`     // 是否由 hotwallet 路径自动抓价
	EnableHumanPrice  bool      `json:"enable_human_price" orm:"enable_human_price"`     // 是否支持人工设定价格
	Shown             bool      `json:"shown" orm:"shown"`                               // 是否在资产列表显示
	IsTicketToken     bool      `json:"is_ticket_token" orm:"is_ticket_token"`           // 是否可作为拼团门票
	DetectAddress     string    `json:"detect_address" orm:"detect_address"`             // 链上充值监听地址
	CreatedAt         time.Time `json:"created_at" orm:"created_at"`                     // 创建时间
	UpdatedAt         time.Time `json:"updated_at" orm:"updated_at"`                     // 更新时间
}

// TableName 指定表名
func (TokenConfigEntity) TableName() string {
	return "token_config"
}
