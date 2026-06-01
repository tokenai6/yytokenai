package model

import (
	"github.com/shopspring/decimal"
)

// CreateWithdrawReq 创建提现请求
type CreateWithdrawReq struct {
	UserID         int64           `json:"user_id" dc:"用户ID"`
	Symbol         string          `json:"symbol" dc:"币种"`
	Amount         decimal.Decimal `json:"amount" dc:"金额"`
	Chain          string          `json:"chain" dc:"链"`
	ReceiveAddress string          `json:"receive_address" dc:"接收地址（可选）"`
	Password       string          `json:"password" dc:"用户密码（如果用户已设置密码则必填）"`
}

// CreateWithdrawRes 创建提现响应
type CreateWithdrawRes struct {
	OrderNo            string `json:"order_no" dc:"订单号"`
	Status             int    `json:"status" dc:"状态 0=待审核"`
	Message            string `json:"message" dc:"提示信息"`
	Amount             string `json:"amount" dc:"提现金额"`
	FeeAmount          string `json:"fee_amount" dc:"手续费金额"`
	TaxAmount          string `json:"tax_amount" dc:"实际税收（USDT）"`
	TaxDeductionAmount string `json:"tax_deduction_amount" dc:"税款抵扣金额（节点额度）"`
	TaxYyaiUsdtAmount  string `json:"tax_yyai_usdt_amount" dc:"税款兑换YYAI的USDT等值"`
	TaxYyaiAmount      string `json:"tax_yyai_amount" dc:"兑换发放的YYAI数量"`
	ActualAmount       string `json:"actual_amount" dc:"实际到账金额（已扣除手续费与税款）"`
}

// GetWithdrawListReq 获取提现列表请求
type GetWithdrawListReq struct {
	UserID   int64  `json:"user_id" dc:"用户ID"`
	Symbol   string `json:"symbol" dc:"币种（可选，不传返回全部）"`
	Page     int    `json:"page" dc:"页码"`
	PageSize int    `json:"page_size" dc:"每页数量"`
}

// GetWithdrawListRes 获取提现列表响应
type GetWithdrawListRes struct {
	Page     int             `json:"page" dc:"页码"`
	PageSize int             `json:"page_size" dc:"每页数量"`
	Total    int             `json:"total" dc:"总数"`
	Pages    int             `json:"pages" dc:"总页数"`
	List     []*WithdrawItem `json:"list" dc:"提现列表"`
}

// WithdrawItem 提现记录项
type WithdrawItem struct {
	OrderNo            string `json:"order_no" dc:"订单号"`
	Symbol             string `json:"symbol" dc:"币种"`
	Amount             string `json:"amount" dc:"提现金额"`
	FeeAmount          string `json:"fee_amount" dc:"手续费金额"`
	TaxAmount          string `json:"tax_amount" dc:"实际税收（USDT）"`
	TaxDeductionAmount string `json:"tax_deduction_amount" dc:"税款抵扣金额（节点额度）"`
	TaxYyaiUsdtAmount  string `json:"tax_yyai_usdt_amount" dc:"税款兑换YYAI的USDT等值"`
	TaxYyaiAmount      string `json:"tax_yyai_amount" dc:"兑换发放的YYAI数量"`
	ActualAmount       string `json:"actual_amount" dc:"实际到账金额"`
	ToAddress          string `json:"to_address" dc:"目标地址"`
	Status             int    `json:"status" dc:"状态"`
	StatusText         string `json:"status_text" dc:"状态文本"`
	TxHash             string `json:"tx_hash" dc:"交易哈希"`
	CreatedAt          int64  `json:"created_at" dc:"创建时间戳"`
}

// GetWithdrawConfigRes 获取提现配置响应（字典：symbol -> 配置）
type GetWithdrawConfigRes map[string]*WithdrawSymbolConfig

// WithdrawSymbolConfig 单个币种提现配置
type WithdrawSymbolConfig struct {
	MinAmount string `json:"min_amount" dc:"最小提现金额"`
}

// AuditWithdrawReq 审核提现请求
type AuditWithdrawReq struct {
	ID      int64  `json:"id" dc:"提现ID"`
	Status  int    `json:"status" dc:"状态 1=通过 2=拒绝"`
	AdminID int64  `json:"admin_id" dc:"审核人ID"`
	Remark  string `json:"remark" dc:"审核备注"`
}

// WithdrawStatusText 提现状态文本
func WithdrawStatusText(status int) string {
	switch status {
	case 0:
		return "待审核"
	case 1:
		return "审核通过"
	case 2:
		return "审核拒绝"
	case 3:
		return "处理中"
	case 4:
		return "成功"
	case 5:
		return "失败"
	default:
		return "未知"
	}
}
