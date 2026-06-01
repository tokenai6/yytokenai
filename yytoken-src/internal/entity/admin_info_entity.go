package entity

import (
	"XWFrame/internal/frame/model"

	"github.com/gogf/gf/v2/os/gtime"
)

// AdminInfoEntity 管理员实体
type AdminInfoEntity struct {
	model.BaseEntity
	Username      string      `json:"username"`
	WalletAddress string      `json:"wallet_address"`
	Email         string      `json:"email"`
	Role          string      `json:"role"`
	RoleId        int64       `json:"role_id"`
	Status        int         `json:"status"`
	LastLoginAt   *gtime.Time `json:"lastLoginAt"`
	CreatedAt     *gtime.Time `json:"createdAt"`
	UpdatedAt     *gtime.Time `json:"updatedAt"`
}
