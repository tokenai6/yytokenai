package consts

// 资金系统 - 兑换配置常量（先以常量固化，后续再开放配置）

// ExchangeFeeTypePercent 百分比手续费类型标识
const ExchangeFeeTypePercent = 2

// ExchangeFeeValueDefault 默认手续费比例（0.005 = 0.5%）
const ExchangeFeeValueDefault = 0.005

// DefaultExchangeRates 默认兑换汇率（from_to_to -> rate）
var DefaultExchangeRates = map[string]float64{
	"APG_to_USDT": 1,
	"USDT_to_APG": 1,
}

// 资金变动类型
const (
	// 充值相关
	ChangeTypeRecharge       = "recharge"        // 充值
	ChangeTypeRechargeRefund = "recharge_refund" // 充值退款

	// 提现相关
	ChangeTypeWithdrawFreeze     = "withdraw_freeze"      // 提现冻结
	ChangeTypeWithdraw           = "withdraw"             // 提现扣除
	ChangeTypeWithdrawUnfreeze   = "withdraw_unfreeze"    // 提现解冻
	ChangeTypeWithdrawSuccess    = "withdraw_success"     // 提现成功
	ChangeTypeWithdrawFailRefund = "withdraw_fail_refund" // 提现失败退款

	// 提现税款相关
	ChangeTypeWithdrawTaxPay           = "withdraw_tax_pay"            // 提现税款扣除
	ChangeTypeWithdrawTaxReceive       = "withdraw_tax_receive"        // 提现税款入账（接收方）
	ChangeTypeWithdrawTaxRefundPay     = "withdraw_tax_refund_pay"     // 提现税款退还（扣回接收方）
	ChangeTypeWithdrawTaxRefundReceive = "withdraw_tax_refund_receive" // 提现税款退还（退回用户）
	ChangeTypeWithdrawTaxYYAI          = "withdraw_tax_yyai"           // 提现税款兑换YYAI发放
	ChangeTypeWithdrawTaxYYAIRollback  = "withdraw_tax_yyai_rollback"  // 提现税款YYAI回滚

	// US Stock Reward Pool
	ChangeTypeUSStockRewardPool = "us_stock_reward_pool" // US Stock Reward Pool 每日结算

	// 兑换相关
	ChangeTypeExchangeIn       = "exchange_in"       // 兑换兑入
	ChangeTypeExchangeOut      = "exchange_out"      // 兑换兑出
	ChangeTypeExchangeFreeze   = "exchange_freeze"   // 兑换冻结
	ChangeTypeExchangeUnfreeze = "exchange_unfreeze" // 兑换解冻

	// 奖励相关
	ChangeTypeReward = "reward" // 奖励
	ChangeTypeBonus  = "bonus"  // 分红

	// 奖励相关（新增：细分奖励类型）
	ChangeTypeRewardSettlement = "reward_settlement" // 奖励结算（统一结算）
	ChangeTypeRewardReduced    = "reward_reduced"    // 奖励削减（额度不足）

	// 节点购买相关
	ChangeTypeNodePurchase             = "node_purchase"               // 节点购买
	ChangeTypeNodePurchaseDirectReward = "node_purchase_direct_reward" // 节点直推奖励

	// 拼团相关
	ChangeTypeGroupMatchJoinUSDT               = "group_match_join_usdt"                    // 拼团参与-本金
	ChangeTypeGroupMatchJoinTicket             = "group_match_join_ticket"                  // 拼团参与-门票
	ChangeTypeGroupMatchSettleWinner           = "group_match_settle_winner"                // 拼团中奖结算
	ChangeTypeGroupMatchSettleTicketRefund     = "group_match_settle_ticket_refund"         // 拼团门票退还
	ChangeTypeGroupMatchSettleTicketBurnToVtx  = "group_match_settle_ticket_burn_to_vertex" // 拼团门票销毁
	ChangeTypeGroupMatchSettleFlowRefund       = "group_match_settle_flow_refund"           // 拼团流团退款
	ChangeTypeGroupMatchLoserCompRelease       = "group_match_loser_comp_release"           // 拼团败者补偿释放
	ChangeTypeGroupMatchLeadershipReward       = "group_match_leadership_reward"            // 拼团领导池奖励
	ChangeTypeGroupMatchLeadershipWeightReward = "group_match_leadership_weight_reward"     // 拼团领导池权重奖励
	ChangeTypeGroupMatchTeamReward             = "group_match_team_reward"                  // 拼团团队奖励

	// 质押相关
	ChangeTypeStakingV2Stake                = "staking_v2_stake"                   // 质押
	ChangeTypeStakingV2ReferralDirect       = "staking_v2_referral_direct"         // 质押直推奖
	ChangeTypeStakingV2ReferralIndirect     = "staking_v2_referral_indirect"       // 质押间推奖
	ChangeTypeStakingV2ReferralBurnToVertex = "staking_v2_referral_burn_to_vertex" // 质押推荐销毁
	ChangeTypeStakingV2LeaderReward         = "staking_v2_leader_reward"           // 质押领导奖励
	ChangeTypeTripleToStakeV2               = "triple_to_stakev2"                  // 三倍券转质押
	ChangeTypeYYAIToBalance                 = "yyai_to_balance"                    // YYAI发放到余额

	// 手续费相关
	ChangeTypeFee  = "fee"  // 手续费
	ChangeTypeBurn = "burn" // 销毁

	// 其他
	ChangeTypeTransferIn   = "transfer_in"   // 转入
	ChangeTypeTransferOut  = "transfer_out"  // 转出
	ChangeTypeAdjust       = "adjust"        // 调整（管理员，旧）
	ChangeTypeSystemAdjust = "system_adjust" // 系统调整

	// 会议报销
	ChangeTypeMeetingReimbursementFirst  = "meeting_reimbursement_first"  // 会议报销第一批（50%）
	ChangeTypeMeetingReimbursementSecond = "meeting_reimbursement_second" // 会议报销第二批（50%）
	ChangeTypeTableShareReward           = "table_share_reward"           // 饭桌分享会奖励
	ChangeTypeOfficeApplySubsidy         = "office_apply_subsidy"         // 工作室申请补贴
)

// AllBalanceChangeTypes 所有资金变动类型集合（便于遍历）
var AllBalanceChangeTypes = []string{
	ChangeTypeRecharge,
	ChangeTypeRechargeRefund,
	ChangeTypeWithdrawFreeze,
	ChangeTypeWithdraw,
	ChangeTypeWithdrawUnfreeze,
	ChangeTypeWithdrawSuccess,
	ChangeTypeWithdrawFailRefund,
	ChangeTypeWithdrawTaxPay,
	ChangeTypeWithdrawTaxReceive,
	ChangeTypeWithdrawTaxRefundPay,
	ChangeTypeWithdrawTaxRefundReceive,
	ChangeTypeWithdrawTaxYYAI,
	ChangeTypeWithdrawTaxYYAIRollback,
	ChangeTypeUSStockRewardPool,
	ChangeTypeExchangeIn,
	ChangeTypeExchangeOut,
	ChangeTypeExchangeFreeze,
	ChangeTypeExchangeUnfreeze,
	ChangeTypeReward,
	ChangeTypeBonus,
	ChangeTypeRewardSettlement,
	ChangeTypeRewardReduced,
	ChangeTypeNodePurchase,
	ChangeTypeNodePurchaseDirectReward,
	ChangeTypeGroupMatchJoinUSDT,
	ChangeTypeGroupMatchJoinTicket,
	ChangeTypeGroupMatchSettleWinner,
	ChangeTypeGroupMatchSettleTicketRefund,
	ChangeTypeGroupMatchSettleTicketBurnToVtx,
	ChangeTypeGroupMatchSettleFlowRefund,
	ChangeTypeGroupMatchLoserCompRelease,
	ChangeTypeGroupMatchLeadershipReward,
	ChangeTypeGroupMatchLeadershipWeightReward,
	ChangeTypeGroupMatchTeamReward,
	ChangeTypeStakingV2Stake,
	ChangeTypeStakingV2ReferralDirect,
	ChangeTypeStakingV2ReferralIndirect,
	ChangeTypeStakingV2ReferralBurnToVertex,
	ChangeTypeStakingV2LeaderReward,
	ChangeTypeTripleToStakeV2,
	ChangeTypeYYAIToBalance,
	ChangeTypeFee,
	ChangeTypeBurn,
	ChangeTypeTransferIn,
	ChangeTypeTransferOut,
	ChangeTypeAdjust,
	ChangeTypeSystemAdjust,
	ChangeTypeMeetingReimbursementFirst,
	ChangeTypeMeetingReimbursementSecond,
	ChangeTypeTableShareReward,
	ChangeTypeOfficeApplySubsidy,
}

// 充值状态
const (
	RechargeStatusPending    = 0 // 待确认
	RechargeStatusProcessing = 1 // 充值中
	RechargeStatusSuccess    = 2 // 成功
	RechargeStatusFailed     = 3 // 失败
)

// 提现状态（使用字符串）
const (
	WithdrawStatusPendingAudit      = "pending_audit"      // 待审核（需要审核的情况）
	WithdrawStatusPendingSignature  = "pending_signature"  // 待签名（不需要审核或已审核通过）
	WithdrawStatusApproved          = "approved"           // 已审核，待签名
	WithdrawStatusProcessing        = "processing"         // 处理中（已获取签名）
	WithdrawStatusPendingCompletion = "pending_completion" // 待完成（已回写hash，等待链上确认）
	WithdrawStatusSuccess           = "success"            // 成功
	WithdrawStatusFailed            = "failed"             // 失败
	WithdrawStatusTimeout           = "timeout"            // 超时（过期自动回退）
	WithdrawStatusCancelled         = "cancelled"          // 已取消
)

// 提现超时配置
const (
	WithdrawPendingTimeoutMinutes = 15 // 待完成状态超时时间（分钟）
)

// 提现最小金额（U），单位：U
const WithdrawMinAmountU = "0.0001"

// 兑换状态
const (
	ExchangeStatusSuccess = 1 // 成功
	ExchangeStatusFailed  = 2 // 失败
)

// 兑换类型
const (
	ExchangeTypeIn  = 1 // 兑入
	ExchangeTypeOut = 2 // 兑出
)

// 操作人类型
const (
	OperatorTypeUser   = "user"   // 用户
	OperatorTypeAdmin  = "admin"  // 管理员
	OperatorTypeSystem = "system" // 系统
)

// 手续费类型
const (
	FeeTypeFixed   = 1 // 固定值
	FeeTypePercent = 2 // 百分比
)

// 链类型
const (
	ChainTypeETH  = "ETH"
	ChainTypeBSC  = "BSC"
	ChainTypeTRON = "TRON"
)

// 代币类型
const (
	TokenTypeNative = "native" // 原生代币
	TokenTypeERC20  = "ERC20"  // ERC20代币
	TokenTypeBEP20  = "BEP20"  // BEP20代币
	TokenTypeTRC20  = "TRC20"  // TRC20代币
)
