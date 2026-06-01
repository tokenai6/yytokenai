package reward

import (
	"XWFrame/internal/frame/model"
	"time"

	"github.com/shopspring/decimal"
)

// StakingPackageEntity 算力包实体
type StakingPackageEntity struct {
	model.BaseEntity
	UserID           int64           `json:"user_id" gorm:"not null;index:idx_staking_user_status"`
	PackageNo        string          `json:"package_no" gorm:"uniqueIndex;size:64;not null"`
	StakeAmount      decimal.Decimal `json:"stake_amount" gorm:"type:decimal(28,18);not null;comment:质押金额"`
	PowerValue       decimal.Decimal `json:"power_value" gorm:"type:decimal(28,18);not null;comment:算力值（质押金额×2）"`
	StakeType        int             `json:"stake_type" gorm:"not null;default:0;comment:质押类型：0-LP质押 1-国库质押;index:idx_staking_stake_type"`
	StakeTypeChain   int             `json:"stake_type_chain" gorm:"comment:链上质押类型原值（保留链上语义）"`
	NodeLevel        string          `json:"node_level" gorm:"size:20;comment:节点等级（node1/node2/node3）"`
	PowerMultiplier  decimal.Decimal `json:"power_multiplier" gorm:"type:decimal(10,2);not null;default:2.00;comment:倍数（LP=2，节点=10/8/6）"`
	DailyYieldRate   decimal.Decimal `json:"daily_yield_rate" gorm:"type:decimal(10,6);not null;default:0.01;comment:日收益率（1%）"`
	TotalQuota       decimal.Decimal `json:"total_quota" gorm:"type:decimal(28,18);not null;comment:贡献的总额度"`
	MaxStaticRelease decimal.Decimal `json:"max_static_release" gorm:"type:decimal(28,18);not null;comment:最大静态释放（质押金额×2）"`
	ReleasedStatic   decimal.Decimal `json:"released_static" gorm:"type:decimal(28,18);not null;default:0;comment:已释放静态收益累计"`
	DaysElapsed      decimal.Decimal `json:"days_elapsed" gorm:"type:decimal(28,18);not null;default:0;comment:已释放天数（精确到秒）"`
	RemainingDays    decimal.Decimal `json:"remaining_days" gorm:"type:decimal(28,18);not null;comment:剩余天数（精确到秒）"`
	TotalDays        decimal.Decimal `json:"total_days" gorm:"type:decimal(28,18);not null;comment:理论总释放天数（最大静态释放÷每日释放）"`
	StartTime        time.Time       `json:"start_time" gorm:"not null;index:idx_staking_start_time;comment:开始时间（精确到秒）"`
	Status           int             `json:"status" gorm:"not null;default:1;comment:1-运行中 2-静态完成出局 3-额度完成出局 4-双重完成出局;index:idx_staking_status;index:idx_staking_user_status,priority:2"`
	TxHash           string          `json:"tx_hash" gorm:"size:100;index:idx_staking_tx_hash"`
	BlockNumber      int64           `json:"block_number"`
	StakeUsdt        decimal.Decimal `json:"stake_usdt" gorm:"type:decimal(28,18);not null;comment:质押USDT金额"`
	StakeToken       decimal.Decimal `json:"stake_token" gorm:"type:decimal(28,18);not null;comment:质押代币金额"`
	TokenContract    string          `json:"token_contract" gorm:"size:45;comment:质押代币合约地址"`
	TokenSymbol      string          `json:"token_symbol" gorm:"size:45;comment:质押代币简称"`
	ExpectedEndTime  *time.Time      `json:"expected_end_time" gorm:"type:timestamptz;comment:预计结束时间（开始时间+200天）"`
	ActualEndTime    *time.Time      `json:"actual_end_time" gorm:"type:timestamptz"`
}

// TableName 指定表名
func (StakingPackageEntity) TableName() string {
	return "staking_package"
}
