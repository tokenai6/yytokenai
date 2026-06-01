package entity

import (
	"XWFrame/internal/frame/model"
)

// AdminRoleMenuFunctionMappingEntity 角色与菜单功能映射实体
type AdminRoleMenuFunctionMappingEntity struct {
	model.BaseEntity
	RoleId     int64 `json:"role_id"`
	FunctionId int64 `json:"function_id"`
}
