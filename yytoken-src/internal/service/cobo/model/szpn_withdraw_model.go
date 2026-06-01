package model

type SZPNWithdrawSubmitReq struct {
	WithdrawID int64  `json:"withdraw_id"`
	OrderNo    string `json:"order_no"`
	RequestID  string `json:"request_id"`
	Symbol     string `json:"symbol"`
	ToAddress  string `json:"to_address"`
	Amount     string `json:"amount"`
}

type SZPNWithdrawSubmitRes struct {
	Accepted bool   `json:"accepted"`
	Status   string `json:"status"`
	Reason   string `json:"reason,omitempty"`
	TxHash   string `json:"tx_hash,omitempty"`
}

type SZPNWithdrawCallbackReq struct {
	RequestID string `json:"request_id"`
	OrderNo   string `json:"order_no"`
	Status    string `json:"status"`
	TxHash    string `json:"tx_hash"`
	Reason    string `json:"reason"`
}

type SZPNWithdrawCallbackRes struct {
	Accepted  bool   `json:"accepted"`
	Processed bool   `json:"processed"`
	Reason    string `json:"reason,omitempty"`
}
