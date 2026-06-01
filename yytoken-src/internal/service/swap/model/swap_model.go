package model

type SwapQuote struct {
	AmountOut string `json:"amount_out"`
	Price     string `json:"price"`
	Slippage  string `json:"slippage"`
	Fee       string `json:"fee"`
}

// SwapYYToUSDTReq YY兑换USDT请求
type SwapYYToUSDTReq struct {
	UserID    int64  `json:"userId"`
	Amount    string `json:"amount"`
	RequestID string `json:"requestId"`
}

// SwapYYToUSDTRes YY兑换USDT响应
type SwapYYToUSDTRes struct {
	USDTAmount string `json:"usdt_amount"`
	YYPrice    string `json:"yy_price"`
	Fee        string `json:"fee"`
	YYLeft     string `json:"yy_left"`
	USDTLeft   string `json:"usdt_left"`
	CreatedAt  string `json:"created_at"`
}

// GetSwapRecordsReq 查询Swap记录请求
type GetSwapRecordsReq struct {
	UserID   int64
	Page     int
	PageSize int
}

// SwapRecordItem Swap记录项
type SwapRecordItem struct {
	ID          int64  `json:"id"`
	FromSymbol  string `json:"from_symbol"`
	FromAmount  string `json:"from_amount"`
	ToSymbol    string `json:"to_symbol"`
	ToAmount    string `json:"to_amount"`
	Fee         string `json:"fee"`
	Price       string `json:"price"`
	BurnTxHash  string `json:"burn_tx_hash"`
	CreatedAt   string `json:"created_at"`
}

// GetSwapRecordsRes 查询Swap记录响应
type GetSwapRecordsRes struct {
	List     []*SwapRecordItem `json:"list"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}
