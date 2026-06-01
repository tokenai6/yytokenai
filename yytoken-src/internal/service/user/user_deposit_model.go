package user

import (
	"time"
)

// DepositAddressInfo 充值地址信息
type DepositAddressInfo struct {
	Address   string    `json:"address"`    // 充值地址
	ChainID   string    `json:"chain_id"`   // 链ID
	IsValid   bool      `json:"is_valid"`   // 是否有效
	CreatedAt time.Time `json:"created_at"` // 创建时间
}

// GetDepositAddressRes 获取充值地址响应
type GetDepositAddressRes struct {
	Address *DepositAddressInfo `json:"address"`
}
