package entity

import (
	"XWFrame/internal/frame/model"
)

// AdminRoleEntity 后台角色实体
type AdminRoleEntity struct {
	model.BaseEntity
	RoleName string `json:"role_name"`
}
