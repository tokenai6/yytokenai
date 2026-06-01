package cobo

import (
	"context"

	"XWFrame/internal/entity/cobo"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/shopspring/decimal"
)

// IBalanceRepository 余额仓储接口
type IBalanceRepository interface {
	// GetByUserID 根据用户ID获取余额
	GetByUserID(ctx context.Context, userID int64, symbol string) (*cobo.BalanceEntity, error)

	// GetByUserIDForUpdate 根据用户ID获取余额并加锁（用于事务中）
	GetByUserIDForUpdate(ctx context.Context, tx gdb.TX, userID int64, symbol string) (*cobo.BalanceEntity, error)

	// Create 创建余额记录
	Create(ctx context.Context, entity *cobo.BalanceEntity) error

	// UpdateBalance 更新余额（原子操作）
	UpdateBalance(ctx context.Context, userID int64, symbol string, availableDelta, frozenDelta decimal.Decimal) error

	// UpdateBalanceTx 在事务中更新余额
	UpdateBalanceTx(ctx context.Context, tx gdb.TX, userID int64, symbol string, availableDelta, frozenDelta decimal.Decimal) error

	// GetOrCreate 获取或创建余额记录
	GetOrCreate(ctx context.Context, userID int64, symbol string) (*cobo.BalanceEntity, error)
}

// IRechargeRepository 充值记录仓储接口
type IRechargeRepository interface {
	// Create 创建充值记录
	Create(ctx context.Context, entity *cobo.RechargeEntity) error

	// GetByTxHash 根据交易哈希获取充值记录
	GetByTxHash(ctx context.Context, txHash string) (*cobo.RechargeEntity, error)

	// GetBySymbolAndTxHash 根据 symbol + tx_hash 获取充值记录
	GetBySymbolAndTxHash(ctx context.Context, symbol, txHash string) (*cobo.RechargeEntity, error)

	// GetByWebhookID 根据Webhook ID获取充值记录（幂等检查）
	GetByWebhookID(ctx context.Context, webhookID string) (*cobo.RechargeEntity, error)

	// UpdateStatus 更新充值状态
	UpdateStatus(ctx context.Context, id int64, status int, confirmations int) error

	// GetPendingWithTxHash 获取待确认且有交易哈希的充值记录
	GetPendingWithTxHash(ctx context.Context, limit int) ([]*cobo.RechargeEntity, error)

	// GetByUserID 获取用户的充值记录（分页）
	GetByUserID(ctx context.Context, userID int64, symbol string, page, pageSize int) ([]*cobo.RechargeEntity, int64, error)
}

// INodePurchaseRepository 节点购买仓储接口
type INodePurchaseRepository interface {
	// Create 创建购买记录
	Create(ctx context.Context, tx gdb.TX, entity *cobo.NodePurchaseEntity) error

	// GetByPackageNo 根据包号获取购买记录
	GetByPackageNo(ctx context.Context, packageNo string) (*cobo.NodePurchaseEntity, error)

	// GetByUserAndRequestID 根据用户ID和请求ID获取购买记录（幂等）
	GetByUserAndRequestID(ctx context.Context, userID int64, requestID string) (*cobo.NodePurchaseEntity, error)

	// GetByUserID 获取用户的购买记录（分页）
	GetByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*cobo.NodePurchaseEntity, int64, error)

	// GetRunningByUserID 获取用户运行中的节点
	GetRunningByUserID(ctx context.Context, userID int64) ([]*cobo.NodePurchaseEntity, error)

	// UpdateStatus 更新购买状态
	UpdateStatus(ctx context.Context, id int64, status int) error

	// UpdateRewards 更新奖励金额
	UpdateRewards(ctx context.Context, id int64, teamReward, leaderReward decimal.Decimal) error

	// GetDirectRewardsByUserID 获取用户收到的直推奖励记录（分页）
	// 查询条件：direct_reward_user_id = userID AND direct_reward_amount > 0
	GetDirectRewardsByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*cobo.NodePurchaseEntity, int64, error)
}

// INodeTokenGrantRepository 节点购买YY发放记录仓储接口
type INodeTokenGrantRepository interface {
	// CreateTx 事务内创建发放记录
	CreateTx(ctx context.Context, tx gdb.TX, entity *cobo.NodeTokenGrantEntity) error

	// GetByUserID 获取用户发放记录（分页）
	GetByUserID(ctx context.Context, userID int64, tokenType string, page, pageSize int) ([]*cobo.NodeTokenGrantEntity, int64, error)

	// GetPendingTotalByUserID 获取用户待发放的YY总额
	GetPendingTotalByUserID(ctx context.Context, userID int64) (decimal.Decimal, error)

	// GetPendingTotalByUserIDAndType 获取用户待发放的指定类型Token总额
	GetPendingTotalByUserIDAndType(ctx context.Context, userID int64, tokenType string) (decimal.Decimal, error)
}

// IWithdrawRepository 提现仓储接口
type IWithdrawRepository interface {
	// GetByID 根据ID获取提现申请
	GetByID(ctx context.Context, id int64) (*cobo.WithdrawEntity, error)

	// Create 创建提现申请
	Create(ctx context.Context, tx gdb.TX, entity *cobo.WithdrawEntity) error

	// GetByOrderNo 根据订单号获取提现申请
	GetByOrderNo(ctx context.Context, orderNo string) (*cobo.WithdrawEntity, error)

	// GetByUserID 获取用户的提现记录（分页）
	GetByUserID(ctx context.Context, userID int64, symbol string, page, pageSize int) ([]*cobo.WithdrawEntity, int64, error)

	// GetPendingList 获取待审核列表
	GetPendingList(ctx context.Context, page, pageSize int) ([]*cobo.WithdrawEntity, int64, error)

	// UpdateStatus 更新提现状态
	UpdateStatus(ctx context.Context, id int64, status int) error
	// UpdateStatusTx 在事务中更新提现状态
	UpdateStatusTx(ctx context.Context, tx gdb.TX, id int64, status int) error

	// Audit 审核提现
	Audit(ctx context.Context, id int64, status int, auditBy int64, remark string) error

	// AuditTx 在事务中审核提现（仅待审核状态可更新）
	AuditTx(ctx context.Context, tx gdb.TX, id int64, status int, auditBy int64, remark string) (bool, error)

	// UpdateCoboRequestID 预写入Cobo请求ID（API调用前）
	UpdateCoboRequestID(ctx context.Context, id int64, requestID string) error

	// UpdateCoboInfo 更新Cobo出金信息
	UpdateCoboInfo(ctx context.Context, id int64, requestID, response, txHash string) error

	// GetByCoboRequestID 根据Cobo请求ID获取提现申请
	GetByCoboRequestID(ctx context.Context, requestID string) (*cobo.WithdrawEntity, error)

	// GetByWebhookID 根据Webhook ID获取提现申请（幂等检查）
	GetByWebhookID(ctx context.Context, webhookID string) (*cobo.WithdrawEntity, error)

	// UpdateTxHashAndStatus 更新交易哈希和状态
	UpdateTxHashAndStatus(ctx context.Context, id int64, txHash string, status int) error

	// UpdateWebhookID 更新Webhook ID
	UpdateWebhookID(ctx context.Context, id int64, webhookID string) error
}
