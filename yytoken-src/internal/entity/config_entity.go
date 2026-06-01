package entity

import (
	"XWFrame/internal/frame/model"

	"github.com/gogf/gf/v2/os/gtime"
)

// ConfigEntity 配置实体
type ConfigEntity struct {
	model.BaseEntity
	KeyName     string      `json:"keyName"`
	KeyValue    string      `json:"keyValue"`
	Description string      `json:"description"`
	CreatedAt   *gtime.Time `json:"createdAt"`
	UpdatedAt   *gtime.Time `json:"updatedAt"`
	DeletedAt   *gtime.Time `json:"deletedAt"`
}
