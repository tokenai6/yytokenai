package entity

import (
	"time"
)

// TokenWhitelistEntity 代币白名单实体（始终显示在资产列表）
type TokenWhitelistEntity struct {
	Id        int64     `json:"id" orm:"id,primary"`
	Symbol    string    `json:"symbol" orm:"symbol"`       // 代币符号
	CreatedAt time.Time `json:"created_at" orm:"created_at"` // 创建时间
}

// TableName 指定表名
func (TokenWhitelistEntity) TableName() string {
	return "token_whitelist"
}
