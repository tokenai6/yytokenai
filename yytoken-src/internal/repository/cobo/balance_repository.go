package cobo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"XWFrame/internal/entity/cobo"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// balanceRepository 余额仓储实现
type balanceRepository struct{}

// NewBalanceRepository 创建余额仓储
func NewBalanceRepository() IBalanceRepository {
	return &balanceRepository{}
}

// GetByUserID 根据用户ID获取余额
func (r *balanceRepository) GetByUserID(ctx context.Context, userID int64, symbol string) (*cobo.BalanceEntity, error) {
	var entity cobo.BalanceEntity
	err := g.DB().Model("cobo_balance").
		Ctx(ctx).
		Where("user_id = ? AND symbol = ?", userID, symbol).
		Scan(&entity)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || isNoRowsErr(err) {
			return nil, nil
		}
		return nil, err
	}
	return &entity, nil
}

// GetByUserIDForUpdate 根据用户ID获取余额并加锁（用于事务中）
func (r *balanceRepository) GetByUserIDForUpdate(ctx context.Context, tx gdb.TX, userID int64, symbol string) (*cobo.BalanceEntity, error) {
	if tx == nil {
		return nil, errors.New("transaction is required for GetByUserIDForUpdate")
	}

	var entity cobo.BalanceEntity
	err := tx.Model("cobo_balance").
		Ctx(ctx).
		Where("user_id = ? AND symbol = ?", userID, symbol).
		LockUpdate().
		Scan(&entity)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || isNoRowsErr(err) {
			return nil, nil
		}
		return nil, err
	}
	return &entity, nil
}

// Create 创建余额记录
func (r *balanceRepository) Create(ctx context.Context, entity *cobo.BalanceEntity) error {
	_, err := g.DB().Exec(ctx, `
		INSERT INTO cobo_balance (user_id, symbol, available_amount, frozen_amount, created_at, updated_at)
		VALUES (?, ?, ?, ?, NOW(), NOW())
		ON CONFLICT (user_id, symbol) DO NOTHING
	`, entity.UserID, entity.Symbol, entity.AvailableAmount, entity.FrozenAmount)
	return err
}

// UpdateBalance 更新余额（原子操作）
func (r *balanceRepository) UpdateBalance(ctx context.Context, userID int64, symbol string, availableDelta, frozenDelta decimal.Decimal) error {
	return r.updateBalanceInternal(ctx, nil, userID, symbol, availableDelta, frozenDelta)
}

// UpdateBalanceTx 在事务中更新余额
func (r *balanceRepository) UpdateBalanceTx(ctx context.Context, tx gdb.TX, userID int64, symbol string, availableDelta, frozenDelta decimal.Decimal) error {
	return r.updateBalanceInternal(ctx, tx, userID, symbol, availableDelta, frozenDelta)
}

// updateBalanceInternal 内部更新余额实现
func (r *balanceRepository) updateBalanceInternal(ctx context.Context, tx gdb.TX, userID int64, symbol string, availableDelta, frozenDelta decimal.Decimal) error {
	// 使用SQL直接更新，避免竞态条件
	sql := `
		INSERT INTO cobo_balance (user_id, symbol, available_amount, frozen_amount, created_at, updated_at)
		VALUES (?, ?, ?, ?, NOW(), NOW())
		ON CONFLICT (user_id, symbol)
		DO UPDATE SET
			available_amount = cobo_balance.available_amount + ?,
			frozen_amount = cobo_balance.frozen_amount + ?,
			updated_at = NOW()
	`
	var err error
	if tx != nil {
		_, err = tx.Exec(sql, userID, symbol, availableDelta, frozenDelta, availableDelta, frozenDelta)
	} else {
		_, err = g.DB().Exec(ctx, sql, userID, symbol, availableDelta, frozenDelta, availableDelta, frozenDelta)
	}
	return err
}

// GetOrCreate 获取或创建余额记录
func (r *balanceRepository) GetOrCreate(ctx context.Context, userID int64, symbol string) (*cobo.BalanceEntity, error) {
	entity, err := r.GetByUserID(ctx, userID, symbol)
	if err != nil {
		return nil, err
	}

	if entity == nil {
		seed := &cobo.BalanceEntity{
			UserID:          userID,
			Symbol:          symbol,
			AvailableAmount: decimal.Zero,
			FrozenAmount:    decimal.Zero,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		if err := r.Create(ctx, seed); err != nil {
			return nil, err
		}

		entity, err = r.GetByUserID(ctx, userID, symbol)
		if err != nil {
			return nil, err
		}
		if entity == nil {
			return seed, nil
		}
	}

	return entity, nil
}

// isNoRowsErr 判断是否为空记录错误
func isNoRowsErr(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, sql.ErrNoRows) || contains(err.Error(), "no rows in result set")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
