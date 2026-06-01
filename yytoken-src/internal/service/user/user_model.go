package user

import (
	"time"

	"github.com/gogf/gf/v2/os/gtime"
)

// UserInfo C端用户信息（基于区块链地址）
type UserInfo struct {
	Id                  int64     `json:"-"` // 不对外暴露ID
	WalletAddress       string    `json:"wallet_address"`
	ParentInviteCode    string    `json:"parent_invite_code"`
	ParentWalletAddress string    `json:"-"`
	InviteCode          string    `json:"invite_code"`
	Status              int       `json:"status"`
	HasSetPassword      bool      `json:"has_set_password"`
	CanWithdraw         bool      `json:"can_withdraw"`
	LastLoginAt         time.Time `json:"last_login_at"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// WalletLoginReq 钱包登录请求
type WalletLoginReq struct {
	WalletAddress       string `json:"wallet_address" v:"required#钱包地址不能为空"`
	Signature           string `json:"signature" v:"required#签名不能为空"`
	Message             string `json:"message" v:"required#消息不能为空"`
	InviteCode          string `json:"invite_code"`           // 可选，用于绑定邀请关系
	ParentWalletAddress string `json:"parent_wallet_address"` // 可选，推荐人钱包地址（用于建立推荐关系）
	Nonce               string `json:"nonce" v:"required#随机校验值不能为空"`
	Timestamp           int64  `json:"timestamp" v:"required#时间戳不能为空"`
}

// WalletLoginRes 钱包登录响应
type WalletLoginRes struct {
	Token     string    `json:"token"`
	ApiSecret string    `json:"api_secret"`
	User      *UserInfo `json:"user"`
	IsNewUser bool      `json:"is_new_user"` // 是否为新用户
}

// WalletVerifyReq 钱包验证请求
type WalletVerifyReq struct {
	WalletAddress string `json:"wallet_address" v:"required#钱包地址不能为空"`
	Signature     string `json:"signature" v:"required#签名不能为空"`
	Message       string `json:"message" v:"required#消息不能为空"`
}

// WalletVerifyRes 钱包验证响应
type WalletVerifyRes struct {
	Valid bool `json:"valid"`
}

// SetPasswordReq 设置密码请求
type SetPasswordReq struct {
	Password    string `json:"password" v:"required#密码不能为空"`
	OldPassword string `json:"old_password"`
}

// SetPasswordRes 设置密码响应
type SetPasswordRes struct {
	Success bool `json:"success"`
}

// VerifyPasswordReq 验证密码请求
type VerifyPasswordReq struct {
	Password string `json:"password" v:"required#密码不能为空"`
}

// VerifyPasswordRes 验证密码响应
type VerifyPasswordRes struct {
	Valid bool `json:"valid"`
}

// GiftNodeInfo 赠送节点信息
type GiftNodeInfo struct {
	HasGiftNode         bool   `json:"has_gift_node" dc:"是否有赠送节点"`
	GiftNodePass        bool   `json:"gift_node_pass" dc:"赠送节点是否及格"`
	GiftAmount          string `json:"gift_amount" dc:"赠送总金额"`
	TeamPerformance     string `json:"team_performance" dc:"当前团队业绩"`
	RequiredPerformance string `json:"required_performance" dc:"所需团队业绩（10倍赠送金额）"`
}

// GetUserProfileRes 获取用户信息响应
type GetUserProfileRes struct {
	User            *UserInfo     `json:"user"`
	TeamMemberCount int           `json:"team_member_count" dc:"团队总人数（所有下级）"`
	DirectCount     int           `json:"direct_count" dc:"直推人数"`
	IsExited        bool          `json:"is_exited" dc:"是否已出局（Staking V2：有封顶记录且无运行中订单）"`
	IsGiftUser      bool          `json:"is_gift_user" dc:"是否仅持有赠送节点用户"`
	ActivateAmount  string        `json:"activate_amount" dc:"还需达成的团队业绩金额"`
	GiftNodeInfo    *GiftNodeInfo `json:"gift_node_info,omitempty" dc:"赠送节点及格情况"`
}

// UpdateUserProfileReq 更新用户信息请求
type UpdateUserProfileReq struct {
	// 目前C端用户只能更新基本信息，钱包地址不可更改
	Status int `json:"status"`
}

// GetSubUserPerformanceRes 获取伞下用户业绩响应
type GetSubUserPerformanceRes struct {
	TotalAmount string `json:"total_amount"` // 伞下用户总购买金额
}

// UpdateUserParentInviteCodeReq 修改用户上级邀请码请求
type UpdateUserParentInviteCodeReq struct {
	TargetUserId     int64  `json:"target_user_id" v:"required#目标用户ID不能为空"`
	NewWalletAddress string `json:"new_wallet_address" v:"required#新钱包地址不能为空"`
}

// UpdateUserParentInviteCodeRes 修改用户上级邀请码响应
type UpdateUserParentInviteCodeRes struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// GetUserDescendantsReq 根据钱包地址查询用户下级请求
type GetUserDescendantsReq struct {
	WalletAddress string `json:"wallet_address" v:"required#钱包地址不能为空"`
}

// GetUserDescendantsRes 根据钱包地址查询用户下级响应
type GetUserDescendantsRes struct {
	Descendants []UserDescendantInfo `json:"descendants"`
}

// UserDescendantInfo 用户下级信息
type UserDescendantInfo struct {
	Id                  int64                `json:"id"`
	WalletAddress       string               `json:"wallet_address"`
	ParentWalletAddress string               `json:"parent_wallet_address"`
	InviteCode          string               `json:"invite_code"`
	Level               int                  `json:"level"`
	Child               []UserDescendantInfo `json:"child"`
}

// GetUserListReq 获取用户列表请求（管理端）
type GetUserListReq struct {
	UserID           int64  `json:"user_id"`
	Page             int    `json:"page"`
	PageSize         int    `json:"page_size"`
	WalletAddress    string `json:"wallet_address"`
	DepositAddress   string `json:"deposit_address"`
	ParentUserID     int64  `json:"parent_user_id"`
	ParentInviteCode string `json:"parent_invite_code"`
	ParentAddress    string `json:"parent_address"`
	LeaderLevel      string `json:"leader_level"`
	AssetSymbol      string `json:"asset_symbol"`
	CanWithdraw      int    `json:"can_withdraw"`
	NodeExempt       int    `json:"node_exempt"`
	AdjustedOnly     int    `json:"adjusted_only"`
	CreatedAtStart   string `json:"created_at_start"`
	CreatedAtEnd     string `json:"created_at_end"`
	Status           int    `json:"status"`
}

type UserListSummary struct {
	TotalRewardUSDT     string `json:"total_reward_usdt"`
	WithdrawnRewardUSDT string `json:"withdrawn_reward_usdt"`
	RemainingRewardUSDT string `json:"remaining_reward_usdt"`
	StakeTotal          string `json:"stake_total"`
	NodePurchaseTotal   string `json:"node_purchase_total"`
	GroupRechargeUSDT   string `json:"group_recharge_usdt"`
	TicketCount         string `json:"ticket_count"`
	GiftTriple          string `json:"gift_triple"`
	SZPN                string `json:"szpn"`
	JU                  string `json:"ju"`
	YY                  string `json:"yy"`
	YYAI                string `json:"yyai"`
}

// GetUserListRes 获取用户列表响应（管理端）
type GetUserListRes struct {
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
	List     []AdminUserListItem `json:"list"`
	Summary  UserListSummary     `json:"summary"`
}

// AdminUserListItem 管理端用户列表项（完整信息）
type AdminUserListItem struct {
	Id                  int64     `json:"id"`
	WalletAddress       string    `json:"wallet_address"`
	DepositAddress      string    `json:"deposit_address"`
	ParentInviteCode    string    `json:"parent_invite_code"`
	ParentWalletAddress string    `json:"parent_wallet_address"`
	InviteCode          string    `json:"invite_code"`
	Status              int       `json:"status"`
	IsVertex            bool      `json:"is_vertex"`
	IsServiceCenter     bool      `json:"is_service_center"`
	CanWithdraw         bool      `json:"can_withdraw"`
	VipLevel            int       `json:"vip_level"`
	AdjustedVipLevel    *int      `json:"adjusted_vip_level,omitempty"`
	Leader              string    `json:"leader"`
	StakeRate           string    `json:"stake_rate"`
	TotalRewardUSDT     string    `json:"total_reward_usdt"`
	WithdrawnRewardUSDT string    `json:"withdrawn_reward_usdt"`
	RemainingRewardUSDT string    `json:"remaining_reward_usdt"`
	StakeTotal          string    `json:"stake_total"`
	NodePurchaseTotal   string    `json:"node_purchase_total"`
	GroupRechargeUSDT   string    `json:"group_recharge_usdt"`
	TicketCount         string    `json:"ticket_count"`
	GiftTriple          string    `json:"gift_triple"`
	SZPN                string    `json:"szpn"`
	JU                  string    `json:"ju"`
	YY                  string    `json:"yy"`
	YYAI                string    `json:"yyai"`
	UseFullPerf         bool      `json:"use_full_perf" dc:"团队奖是否使用完整团队业绩（不扣除大区业绩）"`
	NodeExempt          bool      `json:"node_exempt"`
	LastLoginAt         time.Time `json:"last_login_at"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// GetPerformanceStatsReq 业绩统计请求
type GetPerformanceStatsReq struct {
	Page                int    `json:"page"`
	PageSize            int    `json:"page_size"`
	WalletAddress       string `json:"wallet_address"`
	ParentWalletAddress string `json:"parent_wallet_address"`
}

// GetPerformanceStatsRes 业绩统计响应
type GetPerformanceStatsRes struct {
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
	Total    int                    `json:"total"`
	Pages    int                    `json:"pages"`
	List     []PerformanceStatsItem `json:"list"`
}

// PerformanceStatsItem 业绩统计项
type PerformanceStatsItem struct {
	WalletAddress          string `json:"wallet_address" dc:"用户钱包地址"`
	ParentWalletAddress    string `json:"parent_wallet_address" dc:"上级用户钱包地址"`
	TeamTotalPerformance   int    `json:"team_total_performance" dc:"团队总业绩"`
	TeamMemberCount        int    `json:"team_member_count" dc:"团队人数"`
	DirectMemberCount      int    `json:"direct_member_count" dc:"直推人数"`
	MaxDistrictCount       int    `json:"max_district_count" dc:"大区人数"`
	MinDistrictCount       int    `json:"min_district_count" dc:"小区人数"`
	MinDistrictPerformance int    `json:"min_district_performance" dc:"小区业绩"`
}

// GetPerformanceHistoryReq 历史业绩请求
type GetPerformanceHistoryReq struct {
	Page          int    `json:"page"`
	PageSize      int    `json:"page_size"`
	WalletAddress string `json:"wallet_address"`
	Date          string `json:"date"` // 日期格式：2025-01-01
}

// GetPerformanceHistoryRes 历史业绩响应
type GetPerformanceHistoryRes struct {
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
	Total    int                      `json:"total"`
	Pages    int                      `json:"pages"`
	List     []PerformanceHistoryItem `json:"list"`
}

// PerformanceHistoryItem 历史业绩项
type PerformanceHistoryItem struct {
	WalletAddress                  string `json:"wallet_address" dc:"用户钱包地址"`
	TeamMemberCount                int    `json:"team_member_count" dc:"小区人数（团队人数）"`
	PersonalPerformance            string `json:"personal_performance" dc:"个人业绩"`
	TeamPerformance                string `json:"team_performance" dc:"团队业绩"`
	TeamCombinationPerformance     string `json:"team_combination_performance" dc:"团队组合业绩"`
	PersonalCombinationPerformance string `json:"personal_combination_performance" dc:"个人组合业绩"`
	DistrictPerformance            string `json:"district_performance" dc:"小区业绩"`
	VipLevel                       int    `json:"vip_level" dc:"VIP等级"`
	DailyRelease                   string `json:"daily_release" dc:"当日释放（静态释放数量）"`
	RecordTime                     string `json:"record_time" dc:"统计时间"`
}

// GetYesterdayWeightedRankingRes 昨日直推排行响应
type GetYesterdayWeightedRankingRes struct {
	Total YesterdayWeightedRankingTotal  `json:"total" dc:"汇总数据"`
	List  []YesterdayWeightedRankingItem `json:"list" dc:"排行榜列表"`
}

// YesterdayWeightedRankingTotal 昨日直推排行汇总
type YesterdayWeightedRankingTotal struct {
	WeightedSum string `json:"weighted_sum" dc:"加权总和"`
	TotalReward string `json:"total_reward" dc:"总奖励"`
}

// YesterdayWeightedRankingItem 昨日直推排行项
type YesterdayWeightedRankingItem struct {
	WalletAddress string `json:"wallet_address" dc:"用户地址"`
	NewAmount     string `json:"new_amount" dc:"新增数量（直推新增业绩）"`
	Percentage    string `json:"percentage" dc:"占比（%）"`
	RewardAmount  string `json:"reward_amount" dc:"奖励数量（奖励金额）"`
}

// GetReleaseUSDTDetailsReq 查询释放USDT明细请求
type GetReleaseUSDTDetailsReq struct {
	Page          int    `json:"page"`
	PageSize      int    `json:"page_size"`
	WalletAddress string `json:"wallet_address"`
	Date          string `json:"date"` // 日期格式：2025-01-01，如果没传则默认当前日期
	BusinessType  string `json:"business_type"`
}

// GetReleaseUSDTDetailsRes 查询释放USDT明细响应
type GetReleaseUSDTDetailsRes struct {
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
	Total    int                      `json:"total"`
	Pages    int                      `json:"pages"`
	List     []ReleaseUSDTDetailsItem `json:"list"`
}

// ReleaseUSDTDetailsItem 释放USDT明细项
type ReleaseUSDTDetailsItem struct {
	Id               int64  `json:"id" dc:"记录ID"`
	WalletAddress    string `json:"wallet_address" dc:"用户地址"`
	BusinessType     string `json:"business_type" dc:"释放/奖励类型（数据库字段值）"`
	BusinessTypeDesc string `json:"business_type_desc" dc:"释放/奖励类型（中文说明）"`
	ReleaseTime      string `json:"release_time" dc:"释放时间"`
	ReleaseAmount    string `json:"release_amount" dc:"释放数量"`
	IsSettled        string `json:"is_settled" dc:"是否已结算"`
	Metadata         string `json:"metadata" dc:"元数据（json格式存储）"`
}

// GetReleaseUSDTDetailReq 获取USDT明细详情请求
type GetReleaseUSDTDetailReq struct {
	Id int64 `json:"id" v:"required|min:1#记录ID不能为空|记录ID必须大于0"`
}

// GetReleaseUSDTDetailRes 获取USDT明细详情响应
type GetReleaseUSDTDetailRes struct {
	Id               int64  `json:"id" dc:"记录ID"`
	WalletAddress    string `json:"wallet_address" dc:"用户地址"`
	BusinessType     string `json:"business_type" dc:"释放/奖励类型（数据库字段值）"`
	BusinessTypeDesc string `json:"business_type_desc" dc:"释放/奖励类型（中文说明）"`
	ReleaseTime      string `json:"release_time" dc:"释放时间"`
	ReleaseAmount    string `json:"release_amount" dc:"释放数量"`
	IsSettled        string `json:"is_settled" dc:"是否已结算"`
	Metadata         string `json:"metadata" dc:"元数据（json格式存储）"`
}

// GetNodeInfoListReq 分页显示节点信息请求
type GetNodeInfoListReq struct {
	Page          int    `json:"page"`
	PageSize      int    `json:"page_size"`
	WalletAddress string `json:"wallet_address"` // 可选
}

// GetNodeInfoListRes 分页显示节点信息响应
type GetNodeInfoListRes struct {
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Total    int            `json:"total"`
	Pages    int            `json:"pages"`
	List     []NodeInfoItem `json:"list"`
}

// NodeInfoItem 节点信息项
type NodeInfoItem struct {
	WalletAddress   string `json:"wallet_address" dc:"用户钱包地址"`
	StakeAmount     string `json:"stake_amount" dc:"节点金额"`
	PowerValue      string `json:"power_value" dc:"节点算力"`
	PowerMultiplier string `json:"power_multiplier" dc:"节点倍数"`
	TotalQuota      string `json:"total_quota" dc:"额度"`
	StakeType       int    `json:"stake_type" dc:"节点类型"`
	StakeTypeDesc   string `json:"stake_type_desc" dc:"节点类型描述"`
}

// GetWithdrawRecordsReq 查询提现记录请求
type GetWithdrawRecordsReq struct {
	Page          int    `json:"page"`
	PageSize      int    `json:"page_size"`
	WalletAddress string `json:"wallet_address"` // 可选
	Status        string `json:"status"`         // 可选：pending_audit｜pending_signature｜approved｜processing｜pending_completion｜success｜failed｜cancelled
	MinAmount     string `json:"min_amount"`     // 可选
	MaxAmount     string `json:"max_amount"`     // 可选
	Symbol        string `json:"symbol"`         // 可选：USDT｜JU｜ZPN｜YY
	TaxOnly       bool   `json:"tax_only"`       // 可选：true=仅返回已收盈利税记录
}

// GetWithdrawRecordsRes 查询提现记录响应
type GetWithdrawRecordsRes struct {
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
	Total    int                  `json:"total"`
	Pages    int                  `json:"pages"`
	List     []WithdrawRecordItem `json:"list"`
}

// GetRechargeRecordsReq 查询充值记录请求
type GetRechargeRecordsReq struct {
	Page          int    `json:"page"`
	PageSize      int    `json:"page_size"`
	WalletAddress string `json:"wallet_address"` // 可选
	TxHash        string `json:"tx_hash"`        // 可选，支持 ^前缀 / 后缀$ 锚点
	Status        string `json:"status"`         // 可选：pending｜success｜failed
	MinAmount     string `json:"min_amount"`     // 可选
	MaxAmount     string `json:"max_amount"`     // 可选
	Symbol        string `json:"symbol"`         // 可选：USDT｜JU｜ZPN｜YY
}

// GetRechargeRecordsRes 查询充值记录响应
type GetRechargeRecordsRes struct {
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
	Total    int                  `json:"total"`
	Pages    int                  `json:"pages"`
	List     []RechargeRecordItem `json:"list"`
}

// SetServiceCenterReq 设置服务中心请求
type SetServiceCenterReq struct {
	Id int64 `json:"id"`
}

// SetServiceCenterRes 设置服务中心响应
type SetServiceCenterRes struct {
	IsServiceCenter bool `json:"is_service_center"`
}

// CancelServiceCenterReq 取消服务中心请求
type CancelServiceCenterReq struct {
	Id int64 `json:"id"`
}

// CancelServiceCenterRes 取消服务中心响应
type CancelServiceCenterRes struct {
	IsServiceCenter bool `json:"is_service_center"`
}

// UpdateUserWalletAddressReq 修改用户地址请求
type UpdateUserWalletAddressReq struct {
	Id               int64  `json:"id" v:"required|min:1#用户ID不能为空|用户ID必须大于0"`
	NewWalletAddress string `json:"new_wallet_address" v:"required#新钱包地址不能为空"`
}

// ReplaceWalletAddressReq 后台替换钱包地址请求
type ReplaceWalletAddressReq struct {
	OldAddress string `json:"old_address"`
	NewAddress string `json:"new_address"`
	AllTables  bool   `json:"all_tables"`
	Remark     string `json:"remark"`
	Operator   string `json:"operator"`
}

// SetServiceCenterRateReq 设置服务中心点位比例请求
type SetServiceCenterRateReq struct {
	Id                int64   `json:"id"`
	ServiceCenterRate float64 `json:"service_center_rate"`
}

// SetServiceCenterRateRes 设置服务中心点位比例响应
type SetServiceCenterRateRes struct {
}

// GetServiceCenterUsersRes 获取服务中心用户列表响应
type GetServiceCenterUsersRes struct {
	List []ServiceCenterUserItem `json:"list"`
}

// ServiceCenterUserItem 服务中心用户项
type ServiceCenterUserItem struct {
	Id                int64  `json:"id"`
	WalletAddress     string `json:"wallet_address"`
	ServiceCenterRate string `json:"service_center_rate"`
}

// GetUserRealtimeStatsReq 获取用户实时数据请求
type GetUserRealtimeStatsReq struct {
	Id int64 `json:"id" v:"required|min:1#用户ID不能为空|用户ID必须大于0"`
}

// GetUserRealtimeStatsRes 获取用户实时数据响应
type GetUserRealtimeStatsRes struct {
	UserPurchaseAmount         string `json:"user_purchase_amount" dc:"用户购买总金额"`
	NetworkWithdrawTotal       string `json:"network_withdraw_total" dc:"网体提现总金额"`
	NetworkRealtimePerformance string `json:"network_realtime_performance" dc:"网体实时业绩"`
}

// WithdrawRecordItem 提现记录项
type WithdrawRecordItem struct {
	Id                 int64       `json:"id" dc:"提现订单ID"`
	WalletAddress      string      `json:"wallet_address" dc:"用户地址"`
	Symbol             string      `json:"symbol" dc:"资产符号"`
	Amount             string      `json:"amount" dc:"提现数量"`
	TaxAmount          string      `json:"tax_amount" dc:"盈利税金额"`
	TaxDeductionAmount string      `json:"tax_deduction_amount" dc:"税款抵扣金额"`
	TaxYyaiUsdtAmount  string      `json:"tax_yyai_usdt_amount" dc:"税款兑换YYAI的USDT等值"`
	TaxYyaiAmount      string      `json:"tax_yyai_amount" dc:"兑换发放的YYAI数量"`
	ActualAmount       string      `json:"actual_amount" dc:"实际到账金额"`
	USDTAmount         string      `json:"usdt_amount" dc:"USDT数量"`
	APGAmount          string      `json:"apg_amount" dc:"APG数量"`
	APGPrice           string      `json:"apg_price" dc:"APG价格"`
	TxHash             string      `json:"tx_hash" dc:"交易hash"`
	Status             string      `json:"status" dc:"交易状态(原值)"`
	StatusDesc         string      `json:"status_desc" dc:"交易状态中文"`
	WithdrawTime       *gtime.Time `json:"withdraw_time" dc:"提现时间"`
}

// RechargeRecordItem 充值记录项
type RechargeRecordItem struct {
	Id            int64       `json:"id" dc:"充值记录ID"`
	WalletAddress string      `json:"wallet_address" dc:"用户地址"`
	Symbol        string      `json:"symbol" dc:"资产符号"`
	Amount        string      `json:"amount" dc:"充值数量"`
	TxHash        string      `json:"tx_hash" dc:"交易hash"`
	Status        string      `json:"status" dc:"交易状态(原值)"`
	StatusDesc    string      `json:"status_desc" dc:"交易状态中文"`
	Confirmations int         `json:"confirmations" dc:"确认数"`
	RechargeTime  *gtime.Time `json:"recharge_time" dc:"充值时间"`
}

// UpdateTeamCanWithdrawReq 更新团队成员的提现权限请求
type UpdateTeamCanWithdrawReq struct {
	Id           int64  `json:"id" v:"required|min:1#用户ID不能为空|用户ID必须大于0"`
	WithdrawType string `json:"withdraw_type"` // 提现类型: team-全部提现, personal-部分提现
	CanWithdraw  *bool  `json:"can_withdraw"`  // 直接设置can_withdraw值（优先于WithdrawType）
}

// UpdateTeamStakeRateReq 更新团队成员的质押收益率请求
type UpdateTeamStakeRateReq struct {
	Id        int64   `json:"id" v:"required|min:1#用户ID不能为空|用户ID必须大于0"`
	StakeRate float64 `json:"stake_rate" v:"required|between:0,1#质押收益率不能为空|质押收益率必须在0-1之间,0.01表示1%" dc:"质押收益率"`
}

// UpdateUserNodeExemptReq 更新用户赠送节点业绩豁免状态请求
type UpdateUserNodeExemptReq struct {
	Id         int64 `json:"id" v:"required|min:1#用户ID不能为空|用户ID必须大于0"`
	NodeExempt bool  `json:"node_exempt" dc:"是否豁免赠送节点业绩限制"`
}

// GetUserDetailReq 获取用户详情请求
type GetUserDetailReq struct {
	WalletAddress string `json:"wallet_address"`
}

// UserBalanceInfo 用户余额信息
type UserBalanceInfo struct {
	Symbol          string `json:"symbol"`
	AvailableAmount string `json:"available_amount"`
	FrozenAmount    string `json:"frozen_amount"`
}

// GetUserDetailRes 获取用户详情响应
type GetUserDetailRes struct {
	Id                int64             `json:"id"`
	WalletAddress     string            `json:"wallet_address"`
	CreatedAt         string            `json:"created_at"`
	InviteCode        string            `json:"invite_code"`
	Identity          string            `json:"identity"`
	VipLevel          int               `json:"vip_level"`
	CanWithdraw       bool              `json:"can_withdraw"`
	UseFullPerf       bool              `json:"use_full_perf"`
	Leader            string            `json:"leader"`
	TotalStaking      string            `json:"total_staking"`
	TotalWithdraw     string            `json:"total_withdraw"`
	TotalReward       string            `json:"total_reward"`
	StakeRate         string            `json:"stake_rate"`
	TotalRejectCount     int               `json:"total_reject_count"`
	TotalRejectAmount    string            `json:"total_reject_amount"`
	BigTeamPerformance   string            `json:"big_team_performance"`
	SmallTeamPerformance string            `json:"small_team_performance"`
	Balances             []UserBalanceInfo `json:"balances"`
}

// UserWithdrawQuotaInfo 用户提现额度信息
type UserWithdrawQuotaInfo struct {
	Month                      string `json:"month"`                         // 当前月份 YYYY-MM
	TotalRecharge              string `json:"total_recharge"`                // 累计充值（终身）
	TotalWithdraw              string `json:"total_withdraw"`                // 累计提现（终身原始）
	WithdrawOffset             string `json:"withdraw_offset"`               // 管理员重置偏移
	EffectiveWithdraw          string `json:"effective_withdraw"`            // 有效已提现（total_withdraw - withdraw_offset）
	NodeQuota                  string `json:"node_quota"`                    // 节点税款抵扣额度总额
	TaxDeductionUsed           string `json:"tax_deduction_used"`            // 本月已使用的税款抵扣额度
	RemainingTaxDeductionQuota string `json:"remaining_tax_deduction_quota"` // 剩余可用于抵扣税款额度
	ExtraQuota                 string `json:"extra_quota"`                   // 管理员额外抵扣额度
}

// GetUserWithdrawQuotaReq 获取用户提现额度请求
type GetUserWithdrawQuotaReq struct {
	WalletAddress string `json:"wallet_address"`
}

// GetUserWithdrawQuotaRes 获取用户提现额度响应
type GetUserWithdrawQuotaRes struct {
	Quota *UserWithdrawQuotaInfo `json:"quota"`
}

// ResetUserWithdrawQuotaReq 重置用户提现额度请求
type ResetUserWithdrawQuotaReq struct {
	WalletAddress  string `json:"wallet_address"`
	WithdrawOffset string `json:"withdraw_offset"` // 指定重置偏移量，为空则自动设为当前累计提现
	ExtraQuota     string `json:"extra_quota"`     // 额外增加的额度
	Remark         string `json:"remark"`          // 备注
}

// ResetUserWithdrawQuotaRes 重置用户提现额度响应
type ResetUserWithdrawQuotaRes struct {
	Success bool `json:"success"`
}

// AdminResetUserPasswordReq 管理员重置用户密码请求
type AdminResetUserPasswordReq struct {
	WalletAddress string `json:"wallet_address"`
	Password      string `json:"password"`
}

// AdminResetUserPasswordRes 管理员重置用户密码响应
type AdminResetUserPasswordRes struct {
	Success bool `json:"success"`
}

// GetUserStakingRecordsReq 获取用户质押记录请求
type GetUserStakingRecordsReq struct {
	WalletAddress string `json:"wallet_address"`
	Page          int    `json:"page"`
	PageSize      int    `json:"page_size"`
}

// UserStakingRecordItem 用户质押记录项（基于 staking_v2_order）
type UserStakingRecordItem struct {
	Id             int64  `json:"id"`
	PackageNo      string `json:"package_no"`
	StakeTime      string `json:"stake_time"`
	Amount         string `json:"amount"`
	TotalReward    string `json:"total_reward"`
	RewardLimit    string `json:"reward_limit"`
	RestQuota      string `json:"rest_quota"`
	SourceType     string `json:"source_type"`
	SourceTypeDesc string `json:"source_type_desc"`
	StakeType      string `json:"stake_type"`      // deprecated: 兼容字段，等同 source_type
	StakeTypeDesc  string `json:"stake_type_desc"` // deprecated: 兼容字段，等同 source_type_desc
	Status         int    `json:"status"`
	StatusDesc     string `json:"status_desc"`
	IsGift         int    `json:"is_gift"`      // deprecated: staking_v2 无赠送概念，恒为 0
	IsGiftDesc     string `json:"is_gift_desc"` // deprecated: 恒为空字符串
}

// GetUserStakingRecordsRes 获取用户质押记录响应
type GetUserStakingRecordsRes struct {
	List     []UserStakingRecordItem `json:"list"`
	Total    int                     `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
}

// GetUserRewardRecordsReq 获取用户奖励明细请求
type GetUserRewardRecordsReq struct {
	WalletAddress string `json:"wallet_address"`
	Page          int    `json:"page"`
	PageSize      int    `json:"page_size"`
	BusinessType  string `json:"business_type"` // deprecated
	RewardType    string `json:"reward_type"`
}

// UserRewardRecordItem 用户奖励记录项
type UserRewardRecordItem struct {
	Id                  int64  `json:"id"`
	RewardType          string `json:"reward_type"`
	RewardTypeText      string `json:"reward_type_text"`
	Amount              string `json:"amount"`
	Symbol              string `json:"symbol"`
	SourceUserID        int64  `json:"source_user_id"`
	SourceWalletAddress string `json:"source_wallet_address"`
	PurchasePackageNo   string `json:"purchase_package_no"`
	PurchaseAmount      string `json:"purchase_amount"`
	RewardRate          string `json:"reward_rate"`
	CreatedAt           int64  `json:"created_at"`
}

// GetUserRewardRecordsRes 获取用户奖励明细响应
type GetUserRewardRecordsRes struct {
	List     []UserRewardRecordItem `json:"list"`
	Total    int                    `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

// RewardAggItem 单项奖励汇总
type RewardAggItem struct {
	Amount string `json:"amount"` // 累计金额
	Count  int64  `json:"count"`  // 记录数
}

// GetUserRewardAggReq 获取用户各项奖励汇总请求
type GetUserRewardAggReq struct {
	WalletAddress string `json:"wallet_address"`
}

// GetUserRewardAggRes 获取用户各项奖励汇总响应
type GetUserRewardAggRes struct {
	Direct       RewardAggItem `json:"direct"`        // 直推奖励
	Indirect     RewardAggItem `json:"indirect"`      // 间推奖励
	Team         RewardAggItem `json:"team"`          // 团队奖励
	Leadership   RewardAggItem `json:"leadership"`    // 领导奖励
	MatchReward  RewardAggItem `json:"match_reward"`  // 拼团赢家奖励
	USStock      RewardAggItem `json:"us_stock"`      // 美股奖励
	Total        RewardAggItem `json:"total"`         // 全部奖励合计
}

// GetUserBalanceChangeLogsReq 资金明细查询请求
type GetUserBalanceChangeLogsReq struct {
	WalletAddress string `json:"wallet_address"`
	Symbol        string `json:"symbol"`
	StartTime     string `json:"start_time"`
	EndTime       string `json:"end_time"`
	Page          int    `json:"page"`
	PageSize      int    `json:"page_size"`
}

// UserBalanceChangeLogItem 资金明细项
type UserBalanceChangeLogItem struct {
	Id             int64  `json:"id"`
	CreatedAt      string `json:"created_at"`
	ChangeType     string `json:"change_type"`
	ChangeTypeDesc string `json:"change_type_desc"`
	Symbol         string `json:"symbol"`
	Amount         string `json:"amount"`
	BeforeBalance  string `json:"before_balance"`
	AfterBalance   string `json:"after_balance"`
	RelatedOrderNo string `json:"related_order_no"`
	Remark         string `json:"remark"`
}

// GetUserBalanceChangeLogsRes 资金明细响应
type GetUserBalanceChangeLogsRes struct {
	List     []UserBalanceChangeLogItem `json:"list"`
	Total    int                        `json:"total"`
	Page     int                        `json:"page"`
	PageSize int                        `json:"page_size"`
}

// GetUserInviteRecordsReq 获取用户邀请明细请求
type GetUserInviteRecordsReq struct {
	WalletAddress string `json:"wallet_address"`
	Page          int    `json:"page"`
	PageSize      int    `json:"page_size"`
}

// UserInviteRecordItem 用户邀请记录项
type UserInviteRecordItem struct {
	Id            int64  `json:"id"`
	WalletAddress string `json:"wallet_address"`
	InviteCode    string `json:"invite_code"`
	CreatedAt     string `json:"created_at"`
	StakeAmount   string `json:"stake_amount"`
	VipLevel      int    `json:"vip_level"`
}

// GetUserInviteRecordsRes 获取用户邀请明细响应
type GetUserInviteRecordsRes struct {
	List     []UserInviteRecordItem `json:"list"`
	Total    int                    `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

// GetUserMintRecordsReq 获取用户拼团记录请求
type GetUserMintRecordsReq struct {
	WalletAddress string `json:"wallet_address"`
	Page          int    `json:"page"`
	PageSize      int    `json:"page_size"`
}

// UserMintRecordItem 用户拼团记录项
type UserMintRecordItem struct {
	Id                 int64  `json:"id"`
	MatchId            int64  `json:"match_id"`
	JoinTime           string `json:"join_time"`
	PaymentToken       string `json:"payment_token"`
	PaymentAmount      string `json:"payment_amount"`
	IsWinner           int    `json:"is_winner"`
	IsWinnerDesc       string `json:"is_winner_desc"`
	WinnerRewardAmount string `json:"winner_reward_amount"`
	GroupId            int    `json:"group_id"`
}

// GetUserMintRecordsRes 获取用户拼团记录响应
type GetUserMintRecordsRes struct {
	List     []UserMintRecordItem `json:"list"`
	Total    int                  `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

// GetUserNodePurchaseRecordsReq 获取用户节点购买记录请求
type GetUserNodePurchaseRecordsReq struct {
	WalletAddress string `json:"wallet_address"`
	Page          int    `json:"page"`
	PageSize      int    `json:"page_size"`
}

// UserNodePurchaseRecordItem 用户节点购买记录项
type UserNodePurchaseRecordItem struct {
	Id                 int64  `json:"id"`
	PackageNo          string `json:"package_no"`
	NodeType           int    `json:"node_type"`
	NodeTypeDesc       string `json:"node_type_desc"`
	Amount             string `json:"amount"`
	PowerValue         string `json:"power_value"`
	Status             int    `json:"status"`
	StatusDesc         string `json:"status_desc"`
	IsGift             int    `json:"is_gift"`
	IsGiftDesc         string `json:"is_gift_desc"`
	DirectRewardAmount string `json:"direct_reward_amount"`
	CreatedAt          string `json:"created_at"`
}

// GetUserNodePurchaseRecordsRes 获取用户节点购买记录响应
type GetUserNodePurchaseRecordsRes struct {
	List     []UserNodePurchaseRecordItem `json:"list"`
	Total    int                          `json:"total"`
	Page     int                          `json:"page"`
	PageSize int                          `json:"page_size"`
}

// GetUserRecordCountsReq 获取用户记录数量请求
type GetUserRecordCountsReq struct {
	WalletAddress string `json:"wallet_address"`
}

// GetUserRecordCountsRes 获取用户记录数量响应
type GetUserRecordCountsRes struct {
	StakingCount           int `json:"staking_count"`
	WithdrawCount          int `json:"withdraw_count"`
	InviteCount            int `json:"invite_count"`
	MintCount              int `json:"mint_count"`
	RewardCount            int `json:"reward_count"`
	CoboRewardCount        int `json:"cobo_reward_count"`
	BalanceLogCount        int `json:"balance_log_count"`
	NodePurchaseCount      int `json:"node_purchase_count"`
	TransferCount          int `json:"transfer_count"`
	SwapCount              int `json:"swap_count"`
	RechargeCount          int `json:"recharge_count"`
	YYReleaseCount         int `json:"yy_release_count"`
	LoserCompensationCount int `json:"loser_compensation_count"`
}

// SetUseFullPerfReq 设置团队奖是否使用完整团队业绩请求
type SetUseFullPerfReq struct {
	Id          int64 `json:"id" v:"required|min:1#用户ID不能为空|用户ID必须大于0" dc:"用户ID"`
	UseFullPerf bool  `json:"use_full_perf" dc:"团队奖是否使用完整团队业绩（不扣除大区业绩）"`
}

// SetUseFullPerfRes 设置团队奖是否使用完整团队业绩响应
type SetUseFullPerfRes struct {
	UseFullPerf bool `json:"use_full_perf" dc:"团队奖是否使用完整团队业绩"`
}

// GetUserYYReleaseRecordsReq 获取用户YY释放记录请求
type GetUserYYReleaseRecordsReq struct {
	WalletAddress string `json:"wallet_address"`
	Page          int    `json:"page"`
	PageSize      int    `json:"page_size"`
}

// YYReleaseRecordItem YY释放记录项
type YYReleaseRecordItem struct {
	ID               int64  `json:"id"`
	CompensationID   int64  `json:"compensation_id"`
	OrderID          int64  `json:"order_id"`
	ReleaseDate      string `json:"release_date"`
	ReleaseAmount    string `json:"release_amount"`
	ReleaseUSDTValue string `json:"release_usdt_value"`
	CreatedAt        string `json:"created_at"`
}

// GetUserYYReleaseRecordsRes 获取用户YY释放记录响应
type GetUserYYReleaseRecordsRes struct {
	List     []YYReleaseRecordItem `json:"list"`
	Total    int                   `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
}

// LoserCompensationItem 输单补偿订单项
type LoserCompensationItem struct {
	ID              int64  `json:"id"`
	OrderID         int64  `json:"order_id"`
	TokenSymbol     string `json:"token_symbol"`
	USDTValue       string `json:"usdt_value"`
	ReleaseDays     int    `json:"release_days"`
	ReleasedAmount  string `json:"released_amount"`
	PendingAmount   string `json:"pending_amount"`
	Status          int    `json:"status"`
	StartDate       string `json:"start_date"`
	CreatedAt       string `json:"created_at"`
	ReleaseLogCount int    `json:"release_log_count"`
}

// GetUserLoserCompensationsReq 获取用户补偿订单请求
type GetUserLoserCompensationsReq struct {
	WalletAddress string `json:"wallet_address"`
	Page          int    `json:"page"`
	PageSize      int    `json:"page_size"`
}

// GetUserLoserCompensationsRes 获取用户补偿订单响应
type GetUserLoserCompensationsRes struct {
	List     []LoserCompensationItem `json:"list"`
	Total    int                     `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
}

// LoserCompReleaseLogItem 补偿释放日志项
type LoserCompReleaseLogItem struct {
	ID               int64  `json:"id"`
	CompensationID   int64  `json:"compensation_id"`
	OrderID          int64  `json:"order_id"`
	ReleaseDate      string `json:"release_date"`
	ReleaseAmount    string `json:"release_amount"`
	ReleaseUSDTValue string `json:"release_usdt_value"`
	CreatedAt        string `json:"created_at"`
}

// GetUserLoserCompReleaseLogsReq 获取用户补偿释放日志请求
type GetUserLoserCompReleaseLogsReq struct {
	CompensationID int64 `json:"compensation_id"`
	Page           int   `json:"page"`
	PageSize       int   `json:"page_size"`
}

// GetUserLoserCompReleaseLogsRes 获取用户补偿释放日志响应
type GetUserLoserCompReleaseLogsRes struct {
	List     []LoserCompReleaseLogItem `json:"list"`
	Total    int                       `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
}

// AddressUpdateLogItem 地址更换日志项
type AddressUpdateLogItem struct {
	ID                 int64  `json:"id"`
	OldAddress         string `json:"old_address"`
	NewAddress         string `json:"new_address"`
	UserID             int64  `json:"user_id"`
	TeamName           string `json:"team_name"`
	Remark             string `json:"remark"`
	UpdatedTablesCount int    `json:"updated_tables_count"`
	ReferralCount      int    `json:"referral_count"`
	IsTeamLeader       bool   `json:"is_team_leader"`
	Operator           string `json:"operator"`
	CreatedAt          string `json:"created_at"`
}

// GetAddressUpdateLogsReq 获取地址更换日志请求
type GetAddressUpdateLogsReq struct {
	WalletAddress string `json:"wallet_address"`
}

// GetAddressUpdateLogsRes 获取地址更换日志响应
type GetAddressUpdateLogsRes struct {
	List []AddressUpdateLogItem `json:"list"`
}
