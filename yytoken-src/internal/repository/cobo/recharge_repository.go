package cobo

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"XWFrame/internal/entity/cobo"

	"github.com/gogf/gf/v2/frame/g"
)

// rechargeRepository 充值记录仓储实现
type rechargeRepository struct{}

// NewRechargeRepository 创建充值记录仓储
func NewRechargeRepository() IRechargeRepository {
	return &rechargeRepository{}
}

// Create 创建充值记录
func (r *rechargeRepository) Create(ctx context.Context, entity *cobo.RechargeEntity) error {
	entity.CreatedAt = time.Now()
	entity.UpdatedAt = time.Now()
	result, err := g.DB().Model("cobo_recharge_record").
		Ctx(ctx).
		OmitEmptyData().
		FieldsEx("id").
		Data(entity).
		Insert()
	if err != nil {
		return err
	}

	if id, e := result.LastInsertId(); e == nil && id > 0 {
		entity.ID = id
	}
	return err
}

// GetByTxHash 根据交易哈希获取充值记录
func (r *rechargeRepository) GetByTxHash(ctx context.Context, txHash string) (*cobo.RechargeEntity, error) {
	var entity cobo.RechargeEntity
	err := g.DB().Model("cobo_recharge_record").
		Ctx(ctx).
		Where("tx_hash = ?", txHash).
		Scan(&entity)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || isNoRowsErr(err) {
			return nil, nil
		}
		return nil, err
	}
	return &entity, nil
}

// GetBySymbolAndTxHash 根据 symbol + tx_hash 获取充值记录
func (r *rechargeRepository) GetBySymbolAndTxHash(ctx context.Context, symbol, txHash string) (*cobo.RechargeEntity, error) {
	var entity cobo.RechargeEntity
	err := g.DB().Model("cobo_recharge_record").
		Ctx(ctx).
		Where("symbol = ?", symbol).
		Where("tx_hash = ?", txHash).
		Scan(&entity)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || isNoRowsErr(err) {
			return nil, nil
		}
		return nil, err
	}
	return &entity, nil
}

// GetByWebhookID 根据Webhook ID获取充值记录
func (r *rechargeRepository) GetByWebhookID(ctx context.Context, webhookID string) (*cobo.RechargeEntity, error) {
	if webhookID == "" {
		return nil, nil
	}
	var entity cobo.RechargeEntity
	err := g.DB().Model("cobo_recharge_record").
		Ctx(ctx).
		Where("webhook_id = ?", webhookID).
		Scan(&entity)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || isNoRowsErr(err) {
			return nil, nil
		}
		return nil, err
	}
	return &entity, nil
}

// UpdateStatus 更新充值状态
func (r *rechargeRepository) UpdateStatus(ctx context.Context, id int64, status int, confirmations int) error {
	now := time.Now()
	data := g.Map{
		"status":       status,
		"confirmations": confirmations,
		"updated_at":   now,
	}

	if status == cobo.RechargeStatusConfirmed {
		data["confirmed_at"] = now
	}

	_, err := g.DB().Model("cobo_recharge_record").
		Ctx(ctx).
		Data(data).
		Where("id = ?", id).
		Update()
	return err
}

// GetPendingWithTxHash 获取待确认且有交易哈希的充值记录
func (r *rechargeRepository) GetPendingWithTxHash(ctx context.Context, limit int) ([]*cobo.RechargeEntity, error) {
	if limit <= 0 {
		limit = 100
	}

	var entities []*cobo.RechargeEntity
	err := g.DB().Model("cobo_recharge_record").
		Ctx(ctx).
		Where("status = ?", cobo.RechargeStatusPending).
		Where("tx_hash <> ''").
		Order("created_at ASC").
		Limit(limit).
		Scan(&entities)
	if err != nil {
		return nil, err
	}

	return entities, nil
}

// GetByUserID 获取用户的充值记录（分页）
func (r *rechargeRepository) GetByUserID(ctx context.Context, userID int64, symbol string, page, pageSize int) ([]*cobo.RechargeEntity, int64, error) {
	var entities []*cobo.RechargeEntity
	var total int64

	model := g.DB().Model("cobo_recharge_record").
		Ctx(ctx).
		Where("user_id = ?", userID).
		Where("amount >= ?", 0.01)

	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol != "" {
		model = model.Where("symbol = ?", symbol)
	}

	// 获取总数
	count, err := model.Count()
	if err != nil {
		return nil, 0, err
	}
	total = int64(count)

	// 获取分页数据
	err = model.
		Order("created_at DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Scan(&entities)

	if err != nil {
		return nil, 0, err
	}

	return entities, total, nil
}
