package entity

import "time"

// TokenUserDisplayEntity 用户资产列表显示代币
type TokenUserDisplayEntity struct {
	Id        int64     `json:"id" orm:"id,primary"`
	UserId    int64     `json:"user_id" orm:"user_id"`
	Symbol    string    `json:"symbol" orm:"symbol"`
	CreatedAt time.Time `json:"created_at" orm:"created_at"`
	UpdatedAt time.Time `json:"updated_at" orm:"updated_at"`
}

// TableName 指定表名
func (TokenUserDisplayEntity) TableName() string {
	return "token_user_display"
}
