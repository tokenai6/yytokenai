package entity

import (
	"XWFrame/internal/frame/model"
	"time"

	"github.com/shopspring/decimal"
)

// UserEntity 用户实体
type UserEntity struct {
	model.BaseEntity
	WalletAddress       string          `json:"wallet_address" g:"uniqueIndex;size:42;not null"`
	ParentInviteCode    string          `json:"parent_invite_code" g:"size:42"`
	ParentWalletAddress string          `json:"parent_wallet_address" g:"size:42"`
	InviteCode          string          `json:"invite_code" g:"size:42"`
	Status              int             `json:"status" g:"default:1;comment:状态 1-启用 0-禁用 -1-封禁"`
	IsVertex            bool            `json:"is_vertex" g:"default:false;comment:是否为顶点账号"`
	IsServiceCenter     bool            `json:"is_service_center" g:"default:false;comment:是否为服务中心"`
	ServiceCenterRate   decimal.Decimal `json:"service_center_rate" g:"default:0;comment:服务中心点位比例"`
	Remark              string          `json:"remark" gorm:"size:100;comment:备注"`
	Branch              string          `json:"branch" gorm:"size:255;comment:分支"`
	LastLoginAt         *time.Time      `json:"last_login_at"`
	CanWithdraw         bool            `json:"can_withdraw" g:"default:true;comment:是否可以提现"`
	VipLevel            int             `json:"vip_level" g:"default:0;comment:当前等级"`
	TeamId              *int64          `json:"team_id" g:"comment:所属团队ID"`
	TeamName            string          `json:"team_name" g:"size:50;comment:团队名称"`
	IsTest              int             `json:"is_test" g:"default:0;comment:是否测试用户 0-否 1-是"`
	NodeExempt          bool            `json:"node_exempt" g:"default:false;comment:是否豁免赠送节点业绩限制"`
	UseFullPerf         bool            `json:"use_full_perf" g:"default:false;comment:团队奖是否使用完整团队业绩（不扣除大区业绩）"`
	CanSwap             bool            `json:"can_swap" g:"default:true;comment:是否允许兑换"`
	CanTransfer         bool            `json:"can_transfer" g:"default:true;comment:是否允许转账"`
}
