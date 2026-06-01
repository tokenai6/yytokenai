package model

// TokenInfo 代币信息
type TokenInfo struct {
	Id              int64  `json:"id"`
	Symbol          string `json:"symbol"`
	Name            string `json:"name"`
	ContractAddress string `json:"contract_address"`
	TokenImgUrl     string `json:"token_img_url"`
	Decimals        int    `json:"decimals"`
	Shown           bool   `json:"shown"`
}
