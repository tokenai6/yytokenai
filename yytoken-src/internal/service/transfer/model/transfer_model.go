package model

// InternalTransferReq 内部转账请求
type InternalTransferReq struct {
	FromUserID      int64  `json:"fromUserId"`
	ToWalletAddress string `json:"toWalletAddress"`
	Symbol          string `json:"symbol"`
	Amount          string `json:"amount"`
	RequestID       string `json:"requestId"`
	Password       string `json:"password"`
}

// InternalTransferRes 内部转账响应
type InternalTransferRes struct {
	TransferID      int64  `json:"transfer_id"`
	Symbol          string `json:"symbol"`
	Amount          string `json:"amount"`
	ToWalletAddress string `json:"to_wallet_address"`
	ToUserID        int64  `json:"to_user_id"`
	BalanceLeft     string `json:"balance_left"`
	CreatedAt       string `json:"created_at"`
}

// GetTransferRecordsReq 查询内部转账记录请求
type GetTransferRecordsReq struct {
	UserID   int64  `json:"userId"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Direction string `json:"direction"` // in(转入) / out(转出) / all(全部)
}

// TransferRecordItem 内部转账记录项
type TransferRecordItem struct {
	TransferID      int64  `json:"transfer_id"`
	Symbol          string `json:"symbol"`
	Amount          string `json:"amount"`
	Direction       string `json:"direction"` // in / out
	CounterpartyWalletAddress string `json:"counterparty_wallet_address"`
	CounterpartyUserID int64 `json:"counterparty_user_id"`
	CreatedAt       string `json:"created_at"`
}

// GetTransferRecordsRes 查询内部转账记录响应
type GetTransferRecordsRes struct {
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
	Total    int                  `json:"total"`
	Pages    int                  `json:"pages"`
	List     []TransferRecordItem `json:"list"`
}
