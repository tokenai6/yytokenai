package model

// FlowType 流水类型
const (
	FlowTypeRecharge      = "recharge"         // 充值
	FlowTypeWithdraw      = "withdraw"         // 提现
	FlowTypeReward        = "reward"           // 奖励（兜底）
	FlowTypeRefund        = "refund"           // 退款（兜底）
	FlowTypePayout        = "payout"           // 中奖派发
	FlowTypeCompensation  = "compensation"     // 补偿释放
	FlowTypeNodePurchase  = "node_purchase"    // 节点购买
	FlowTypeMatchReward   = "match_reward"     // 拼团奖励
	FlowTypeMatchRefund   = "match_refund"     // 拼团退款
	FlowTypeDirectReward  = "direct_reward"    // 直推奖励
	FlowTypeMatchJoin       = "match_join"        // 拼团参与扣款
	FlowTypeGroupTeamReward = "group_team_reward" // 拼团团队奖励
	FlowTypeSwap            = "swap"              // 兑换
	FlowTypeTransfer        = "transfer"          // 转账
)

// FlowRecordItem 流水记录项
type FlowRecordItem struct {
	ID           string `json:"id" dc:"记录唯一标识（格式：类型:原ID）"`
	Type         string `json:"type" dc:"流水类型：recharge=充值, withdraw=提现, reward=奖励"`
	TypeText     string `json:"type_text,omitempty" dc:"类型文本"`
	Amount       string `json:"amount" dc:"金额（提现和奖励为正，充值为正）"`
	Symbol       string `json:"symbol" dc:"币种"`
	Status       int    `json:"status" dc:"状态"`
	StatusText   string `json:"status_text,omitempty" dc:"状态文本"`
	TxHash       string `json:"tx_hash" dc:"交易哈希"`
	Description  string `json:"description,omitempty" dc:"描述信息"`
	CreatedAt    int64  `json:"created_at" dc:"创建时间戳"`

	// 充值特有字段
	Confirmations int `json:"confirmations,omitempty" dc:"确认数（仅充值）"`

	// 提现特有字段
	FeeRate      string `json:"fee_rate,omitempty" dc:"手续费费率（仅提现）"`
	FeeAmount    string `json:"fee_amount,omitempty" dc:"手续费金额（仅提现）"`
	ActualAmount string `json:"actual_amount,omitempty" dc:"实际到账金额（仅提现）"`
	ToAddress    string `json:"to_address,omitempty" dc:"目标地址（仅提现）"`

	// 奖励特有字段
	RewardType          string `json:"reward_type,omitempty" dc:"奖励类型（仅奖励）"`
	SourceWalletAddress string `json:"source_wallet_address,omitempty" dc:"来源用户钱包地址（仅奖励）"`
	PurchaseAmount      string `json:"purchase_amount,omitempty" dc:"触发奖励的购买金额（仅奖励）"`
}

// GetFlowRecordsReq 获取流水记录请求
type GetFlowRecordsReq struct {
	UserID   int64  `json:"user_id" dc:"用户ID"`
	Page     int    `json:"page" dc:"页码，默认1"`
	PageSize int    `json:"page_size" dc:"每页数量，默认20"`
	FlowType string `json:"flow_type" dc:"流水类型筛选：recharge/withdraw/reward，空表示全部"`
}

// GetFlowRecordsRes 获取流水记录响应
type GetFlowRecordsRes struct {
	Page     int               `json:"page" dc:"页码"`
	PageSize int               `json:"page_size" dc:"每页数量"`
	Total    int64             `json:"total" dc:"总数"`
	Pages    int               `json:"pages" dc:"总页数"`
	List     []*FlowRecordItem `json:"list" dc:"流水记录列表"`
}
