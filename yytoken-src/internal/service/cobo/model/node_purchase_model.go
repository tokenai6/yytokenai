package model

import (
	"strconv"
	"strings"
)

// PageReq 分页请求
type PageReq struct {
	Page     int `json:"page" dc:"页码，默认1"`
	PageSize int `json:"page_size" dc:"每页数量，默认20"`
}

// PageRes 分页响应
type PageRes struct {
	Page     int `json:"page" dc:"页码"`
	PageSize int `json:"page_size" dc:"每页数量"`
	Total    int `json:"total" dc:"总数"`
	Pages    int `json:"pages" dc:"总页数"`
}

// GetBalanceReq 获取余额请求
type GetBalanceReq struct {
	UserID int64  `json:"user_id"`
	Symbol string `json:"symbol"` // 默认USDT
}

// GetBalanceRes 获取余额响应
type GetBalanceRes struct {
	Available string `json:"available" dc:"可用余额"`
	Frozen    string `json:"frozen" dc:"冻结金额"`
	Symbol    string `json:"symbol" dc:"币种"`
}

// BuyNodeReq 购买节点请求
type BuyNodeReq struct {
	UserID    int64  `json:"user_id"`
	NodeType  int    `json:"node_type" v:"required|in:1,2,3,4,5,6#节点类型不能为空|节点类型不合法" dc:"节点类型 1-6"`
	RequestID string `json:"request_id" dc:"幂等请求ID"`
	Locale    string `json:"locale" dc:"语言环境(可选)"`
}

// BuyNodeRes 购买节点响应
type BuyNodeRes struct {
	PackageNo    string `json:"package_no" dc:"包号"`
	NodeType     int    `json:"node_type" dc:"节点类型"`
	Amount       string `json:"amount" dc:"购买金额"`
	PowerValue   string `json:"power_value" dc:"算力值"`
	BalanceAfter string `json:"balance_after" dc:"购买后余额"`
	CreatedAt    int64  `json:"created_at" dc:"创建时间戳"`
}

// GetNodeTokenGrantRecordsReq 获取YYAI发放记录请求
type GetNodeTokenGrantRecordsReq struct {
	UserID    int64  `json:"user_id"`
	Page      int    `json:"page"`
	PageSize  int    `json:"page_size"`
	TokenType string `json:"token_type"`
}

// NodeTokenGrantRecordItem Token发放记录项 (YYAI/Triple)
type NodeTokenGrantRecordItem struct {
	ID          int64  `json:"id"`
	PurchaseID  int64  `json:"purchase_id"`
	TokenType   string `json:"token_type" dc:"发放类型: YYAI/Triple"`
	TokenSymbol string `json:"token_symbol"`
	TokenChain  string `json:"token_chain"`
	GrantAmount string `json:"grant_amount"`
	Status      int    `json:"status"`
	CreatedAt   int64  `json:"created_at"`
	// YYAI价格相关字段
	YyaiPrice  string `json:"yyai_price" dc:"YYAI价格(USD)"`
	UsdtAmount string `json:"usdt_amount" dc:"购买金额(USDT)"`
	YyaiAmount string `json:"yyai_amount" dc:"实际YYAI发放数量"`
}

// GetNodeTokenGrantRecordsRes 获取YYAI发放记录响应
type GetNodeTokenGrantRecordsRes struct {
	Page     int                        `json:"page"`
	PageSize int                        `json:"page_size"`
	Total    int                        `json:"total"`
	Pages    int                        `json:"pages"`
	List     []*NodeTokenGrantRecordItem `json:"list"`
}

// NodeTokenGrantStatusText YYAI发放状态文本
func NodeTokenGrantStatusText(status int) string {
	switch status {
	case 0:
		return "待发放"
	case 1:
		return "已发放"
	case 2:
		return "发放失败"
	default:
		return "未知"
	}
}

// NodeTypeConfig 节点类型配置
type NodeTypeConfig struct {
	Type            int     `json:"type"`             // 节点类型 1-4
	Name            string  `json:"name"`             // 节点名称
	Amount          float64 `json:"amount"`           // 购买金额
	PowerMultiplier float64 `json:"power_multiplier"` // 算力倍数
}

// DefaultNodeTypes 默认节点类型配置
var DefaultNodeTypes = []NodeTypeConfig{
	{Type: 1, Name: "节点1", Amount: 1000, PowerMultiplier: 3.0},
	{Type: 2, Name: "节点2", Amount: 5000, PowerMultiplier: 4.0},
	{Type: 3, Name: "节点3", Amount: 10000, PowerMultiplier: 5.0},
	{Type: 4, Name: "节点4", Amount: 30000, PowerMultiplier: 6.0},
	{Type: 5, Name: "节点5", Amount: 100000, PowerMultiplier: 7.0},
	{Type: 6, Name: "体验节点", Amount: 100, PowerMultiplier: 0.0},
}

// GetNodeTypeConfig 获取指定节点类型配置
func GetNodeTypeConfig(nodeType int) *NodeTypeConfig {
	for _, t := range DefaultNodeTypes {
		if t.Type == nodeType {
			return &t
		}
	}
	return nil
}

// GetNodeTypeConfigByLevel 根据节点等级获取配置（如 NODE1）
func GetNodeTypeConfigByLevel(nodeLevel string) *NodeTypeConfig {
	normalized := strings.ToUpper(strings.TrimSpace(nodeLevel))
	if !strings.HasPrefix(normalized, "NODE") {
		return nil
	}

	nodeType, err := strconv.Atoi(strings.TrimPrefix(normalized, "NODE"))
	if err != nil {
		return nil
	}

	return GetNodeTypeConfig(nodeType)
}
