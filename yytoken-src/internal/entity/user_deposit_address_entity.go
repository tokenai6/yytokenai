package entity

import (
	"XWFrame/internal/frame/model"
)

// UserDepositAddressEntity 用户充值地址实体
type UserDepositAddressEntity struct {
	model.BaseEntity
	UserID  int64  `json:"user_id" g:"uniqueIndex;not null;comment:用户ID"`
	ChainID string `json:"chain_id" g:"size:20;not null;default:'BSC_BNB';comment:链ID"`
	Address string `json:"address" g:"size:100;not null;index;comment:充值地址"`
	IsValid bool   `json:"is_valid" g:"default:true;comment:是否有效"`
}
