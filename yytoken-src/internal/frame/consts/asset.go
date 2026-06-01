package consts

// 资金记录收支类型
const (
	AssetFlowTypeIncome  = "income"  // 收入
	AssetFlowTypeExpense = "expense" // 支出
)

// 资金记录状态
const (
	AssetRecordStatusPending = 1 // 待结算
	AssetRecordStatusSettled = 2 // 已结算
)

// 资金记录业务类型
const (
	AssetBusinessTypeStake               = "stake"                 // 算力购买
	AssetBusinessTypeRewardStatic        = "reward_static"         // 静态释放
	AssetBusinessTypeRewardDistrict      = "reward_district"       // VIP奖励
	AssetBusinessTypeRewardReferral      = "reward_referral"       // 层级奖
	AssetBusinessTypeRewardWeighted      = "reward_weighted"       // 直推排行奖
	AssetBusinessTypeRewardNode          = "reward_node"           // 节点分红
	AssetBusinessTypeFounderFeeDividend  = "founder_fee_dividend"  // 创始股手续费分红
	AssetBusinessTypeRewardServiceCenter = "service_center_reward" // 服务中心奖励
	AssetBusinessTypeRewardReduced       = "reward_reduced"        // 削减扣除
	AssetBusinessTypeTeamReward          = "team_reward"           // 极差奖励（团队奖励）
	AssetBusinessTypeLeaderReward        = "leader_reward"         // 领导人奖励
	AssetBusinessTypeWithdraw            = "withdraw"              // APG提现
	AssetBusinessTypeSwapBurn            = "swap_burn"             // 销毁记录
	AssetBusinessTypeMintWinner          = "apg_mint_winner"       // APG铸币赢家奖励
	AssetBusinessTypeMintReferral        = "apg_mint_referral"     // APG铸币推荐奖励
	AssetBusinessTypeEquityRecord        = "equity_record"         // 节点购买权益记录
)

// 资金类型（对应account_type表的type字段）
const (
	AssetTypeAPGUserBalance  = "apg_user_balance"     // APG用户余额
	AssetTypeVSUserBalance   = "vs_user_balance"      // VS用户余额
	AssetTypeUSDTUserBalance = "vs_usdt_user_balance" // USDT用户余额
	//vs_venus_user_balance
	AssetTypeVSVenusUserBalance = "vs_venus_user_balance" // VS Venus用户余额

)
