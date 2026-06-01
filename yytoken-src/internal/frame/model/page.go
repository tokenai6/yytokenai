package model

import "time"

// BaseEntity 基础实体
type BaseEntity struct {
	Id        int64     `json:"id" g:"primaryKey;autoIncrement;column:id"`
	CreatedAt time.Time `json:"created_at" g:"autoCreateTime;column:created_at"`
	UpdatedAt time.Time `json:"updated_at" g:"autoUpdateTime;column:updated_at"`
}

// PageReq 分页请求
type PageReq struct {
	Page     int `json:"page" v:"min:1#页码必须大于0"`                     // 页码，从1开始
	PageSize int `json:"page_size" v:"min:1|max:100#每页数量必须在1-100之间"` // 每页数量
}

// PageRes 分页响应
type PageRes struct {
	Page     int `json:"page"`      // 当前页码
	PageSize int `json:"page_size"` // 每页数量
	Total    int `json:"total"`     // 总数量
	Pages    int `json:"pages"`     // 总页数
}

// CommonRes 统一响应结构
type CommonRes struct {
	Code int         `json:"code"` // 状态码，0表示成功
	Msg  string      `json:"msg"`  // 响应消息
	Data interface{} `json:"data"` // 响应数据
}

// Success 成功响应
func Success(data interface{}) *CommonRes {
	return &CommonRes{
		Code: 0,
		Msg:  "success",
		Data: data,
	}
}

// Error 错误响应
func Error(code int, msg string) *CommonRes {
	return &CommonRes{
		Code: code,
		Msg:  msg,
		Data: nil,
	}
}

// ErrorWithData 带数据的错误响应
func ErrorWithData(code int, msg string, data interface{}) *CommonRes {
	return &CommonRes{
		Code: code,
		Msg:  msg,
		Data: data,
	}
}
