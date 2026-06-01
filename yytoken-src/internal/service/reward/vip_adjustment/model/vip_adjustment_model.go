package model

import "time"

// CreateAdjustmentReq 创建VIP调整记录请求
type CreateAdjustmentReq struct {
	UserID           int64  `json:"user_id" v:"required#用户ID不能为空"`
	AdjustedVipLevel int    `json:"adjusted_vip_level" v:"required|between:0,9#调整后VIP等级不能为空|VIP等级必须在0-9之间"`
	OperatorID       int64  `json:"operator_id" v:"required#操作员ID不能为空"`
	Remark           string `json:"remark" dc:"备注"`
}

// CreateAdjustmentRes 创建VIP调整记录响应
type CreateAdjustmentRes struct {
	ID               int64     `json:"id" dc:"调整记录ID"`
	UserID           int64     `json:"user_id" dc:"用户ID"`
	AdjustedVipLevel int       `json:"adjusted_vip_level" dc:"调整后的VIP等级"`
	OriginalVipLevel int       `json:"original_vip_level" dc:"调整前的VIP等级"`
	AdjustTime       time.Time `json:"adjust_time" dc:"调整时间"`
	ExpireTime       time.Time `json:"expire_time" dc:"过期时间"`
}

// GetAdjustmentListReq 获取调整记录列表请求
type GetAdjustmentListReq struct {
	Page          int    `json:"page" v:"min:1#页码最小为1"`
	PageSize      int    `json:"page_size" v:"min:1|max:100#每页数量最小为1|每页数量最大为100"`
	UserID        int64  `json:"user_id" dc:"用户ID（可选）"`
	WalletAddress string `json:"wallet_address" dc:"钱包地址（可选，与user_id二选一）"`
}

// GetAdjustmentListRes 获取调整记录列表响应
type GetAdjustmentListRes struct {
	Page     int              `json:"page" dc:"页码"`
	PageSize int              `json:"page_size" dc:"每页数量"`
	Total    int              `json:"total" dc:"总记录数"`
	Pages    int              `json:"pages" dc:"总页数"`
	List     []AdjustmentItem `json:"list" dc:"调整记录列表"`
}

// AdjustmentItem 调整记录项
type AdjustmentItem struct {
	ID               int64     `json:"id" dc:"调整记录ID"`
	UserID           int64     `json:"user_id" dc:"用户ID"`
	WalletAddress    string    `json:"wallet_address" dc:"用户钱包地址"`
	AdjustedVipLevel int       `json:"adjusted_vip_level" dc:"调整后的VIP等级"`
	OriginalVipLevel int       `json:"original_vip_level" dc:"调整前的VIP等级"`
	AdjustTime       time.Time `json:"adjust_time" dc:"调整时间"`
	ExpireTime       time.Time `json:"expire_time" dc:"过期时间（取消操作会将此时间设置为当前时间）"`
	OperatorID       int64     `json:"operator_id" dc:"操作员ID"`
	OperatorName     string    `json:"operator_name" dc:"操作员用户名"`
	Remark           string    `json:"remark" dc:"备注"`
	IsValid          bool      `json:"is_valid" dc:"是否有效"`
}

// CancelAdjustmentReq 取消调整记录请求
type CancelAdjustmentReq struct {
	ID int64 `json:"id" v:"required#调整记录ID不能为空"`
}
