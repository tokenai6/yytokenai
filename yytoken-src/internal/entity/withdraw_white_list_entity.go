package entity

import (
	"time"
)

// WithdrawWhiteListEntity 提现地址白名单实体
// 白名单用户允许提现到非本人绑定的钱包地址
type WithdrawWhiteListEntity struct {
	UserID    int64     `json:"user_id" orm:"user_id,primary"` // 用户ID
	CreatedAt time.Time `json:"created_at" orm:"created_at"`   // 创建时间
	Remark    string    `json:"remark" orm:"remark"`           // 备注
}

// TableName 指定表名
func (WithdrawWhiteListEntity) TableName() string {
	return "withdraw_white_list"
}
