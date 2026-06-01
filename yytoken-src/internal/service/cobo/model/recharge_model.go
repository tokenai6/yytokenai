package model

// GetRechargeRecordsReq 获取充值记录请求
type GetRechargeRecordsReq struct {
	UserID   int64  `json:"user_id" dc:"用户ID"`
	Symbol   string `json:"symbol" dc:"币种，默认USDT"`
	Page     int    `json:"page" dc:"页码"`
	PageSize int    `json:"page_size" dc:"每页数量"`
}

// RechargeRecordItem 充值记录项
type RechargeRecordItem struct {
	ID            int64  `json:"id" dc:"充值记录ID"`
	TxHash        string `json:"tx_hash" dc:"交易哈希"`
	Amount        string `json:"amount" dc:"金额"`
	Symbol        string `json:"symbol" dc:"币种"`
	Status        int    `json:"status" dc:"状态"`
	StatusText    string `json:"status_text" dc:"状态文本"`
	StatusDesc    string `json:"status_desc" dc:"状态文本(兼容前端)"`
	Confirmations int    `json:"confirmations" dc:"确认数"`
	CreatedAt     int64  `json:"created_at" dc:"创建时间戳"`
}

// GetRechargeRecordsRes 获取充值记录响应
type GetRechargeRecordsRes struct {
	Page     int                   `json:"page" dc:"页码"`
	PageSize int                   `json:"page_size" dc:"每页数量"`
	Total    int                   `json:"total" dc:"总数"`
	Pages    int                   `json:"pages" dc:"总页数"`
	List     []*RechargeRecordItem `json:"list" dc:"充值记录列表"`
}

// GetDepositAddressReq 获取充值地址请求
type GetDepositAddressReq struct {
	UserID int64  `json:"user_id" dc:"用户ID"`
	Symbol string `json:"symbol" dc:"币种，默认 SZPN"`
}

// GetDepositAddressRes 获取充值地址响应
type GetDepositAddressRes struct {
	Symbol  string `json:"symbol" dc:"币种"`
	ChainID string `json:"chain_id" dc:"链ID"`
	Address string `json:"address" dc:"充值地址"`
}

// RechargeStatusText 充值状态文本
func RechargeStatusText(status int) string {
	switch status {
	case 0:
		return "待确认"
	case 1:
		return "已确认"
	case 2:
		return "失败"
	case 3:
		return "已拒绝"
	default:
		return "未知"
	}
}
