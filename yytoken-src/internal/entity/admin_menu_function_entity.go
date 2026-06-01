package entity

import (
	"XWFrame/internal/frame/model"
)

// AdminMenuFunctionEntity 后台菜单/功能实体
type AdminMenuFunctionEntity struct {
	model.BaseEntity
	ParentId       int64  `json:"parent_id"`
	ResourceName   string `json:"resource_name"`
	PageUrl        string `json:"page_url"`
	Route          string `json:"route"`
	FunctionKey    string `json:"function_key"`
	IsMenuFunction int    `json:"is_menu_function"`
	Icon           string `json:"icon"`
	Sort           int    `json:"sort"`
}
