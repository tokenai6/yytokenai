package model

// SZPNDepositCallbackReq monitor充值回调请求
type SZPNDepositCallbackReq struct {
	EventID       string `json:"event_id"`
	ChainID       string `json:"chain_id"`
	Symbol        string `json:"symbol"`
	TxHash        string `json:"tx_hash"`
	LogIndex      int64  `json:"log_index"`
	FromAddress   string `json:"from_address"`
	ToAddress     string `json:"to_address"`
	Amount        string `json:"amount"`
	BlockNumber   int64  `json:"block_number"`
	Confirmations int    `json:"confirmations"`
}

// SZPNDepositCallbackRes monitor充值回调响应
type SZPNDepositCallbackRes struct {
	Accepted  bool   `json:"accepted"`
	Processed bool   `json:"processed"`
	Reason    string `json:"reason,omitempty"`
}
