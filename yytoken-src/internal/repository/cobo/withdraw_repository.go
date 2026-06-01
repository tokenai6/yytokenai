package cobo

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"XWFrame/internal/entity/cobo"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// withdrawRepository 提现仓储实现
type withdrawRepository struct{}

// NewWithdrawRepository 创建提现仓储
func NewWithdrawRepository() IWithdrawRepository {
	return &withdrawRepository{}
}

// GetByID 根据ID获取提现申请
func (r *withdrawRepository) GetByID(ctx context.Context, id int64) (*cobo.WithdrawEntity, error) {
	var entity cobo.WithdrawEntity
	err := g.DB().Model("cobo_withdraw_request").
		Ctx(ctx).
		Where("id = ?", id).
		Scan(&entity)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || isNoRowsErr(err) {
			return nil, nil
		}
		return nil, err
	}
	return &entity, nil
}

// Create 创建提现申请
func (r *withdrawRepository) Create(ctx context.Context, tx gdb.TX, entity *cobo.WithdrawEntity) error {
	entity.CreatedAt = time.Now()
	entity.UpdatedAt = time.Now()
	result, err := tx.Model("cobo_withdraw_request").
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

// GetByOrderNo 根据订单号获取提现申请
func (r *withdrawRepository) GetByOrderNo(ctx context.Context, orderNo string) (*cobo.WithdrawEntity, error) {
	var entity cobo.WithdrawEntity
	err := g.DB().Model("cobo_withdraw_request").
		Ctx(ctx).
		Where("order_no = ?", orderNo).
		Scan(&entity)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || isNoRowsErr(err) {
			return nil, nil
		}
		return nil, err
	}
	return &entity, nil
}

// GetByUserID 获取用户的提现记录（分页）
func (r *withdrawRepository) GetByUserID(ctx context.Context, userID int64, symbol string, page, pageSize int) ([]*cobo.WithdrawEntity, int64, error) {
	var entities []*cobo.WithdrawEntity
	var total int64

	symbol = strings.ToUpper(strings.TrimSpace(symbol))

	model := g.DB().Model("cobo_withdraw_request").
		Ctx(ctx).
		Where("user_id = ?", userID)

	if symbol != "" {
		model = model.Where("symbol = ?", symbol)
	}

	count, err := model.Count()
	if err != nil {
		return nil, 0, err
	}
	total = int64(count)

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

// GetPendingList 获取待审核列表
func (r *withdrawRepository) GetPendingList(ctx context.Context, page, pageSize int) ([]*cobo.WithdrawEntity, int64, error) {
	var entities []*cobo.WithdrawEntity
	var total int64

	model := g.DB().Model("cobo_withdraw_request").
		Ctx(ctx).
		Where("status = ?", cobo.WithdrawStatusPending)

	count, err := model.Count()
	if err != nil {
		return nil, 0, err
	}
	total = int64(count)

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

// UpdateStatus 更新提现状态
func (r *withdrawRepository) UpdateStatus(ctx context.Context, id int64, status int) error {
	_, err := g.DB().Model("cobo_withdraw_request").
		Ctx(ctx).
		Data(g.Map{
			"status":     status,
			"updated_at": time.Now(),
		}).
		Where("id = ?", id).
		Update()
	return err
}

// UpdateStatusTx 在事务中更新提现状态
func (r *withdrawRepository) UpdateStatusTx(ctx context.Context, tx gdb.TX, id int64, status int) error {
	_, err := tx.Model("cobo_withdraw_request").
		Ctx(ctx).
		Data(g.Map{
			"status":     status,
			"updated_at": time.Now(),
		}).
		Where("id = ?", id).
		Update()
	return err
}

// Audit 审核提现
func (r *withdrawRepository) Audit(ctx context.Context, id int64, status int, auditBy int64, remark string) error {
	_, err := r.auditInternal(ctx, nil, id, status, auditBy, remark, false)
	return err
}

// AuditTx 在事务中审核提现
func (r *withdrawRepository) AuditTx(ctx context.Context, tx gdb.TX, id int64, status int, auditBy int64, remark string) (bool, error) {
	return r.auditInternal(ctx, tx, id, status, auditBy, remark, true)
}

func (r *withdrawRepository) auditInternal(ctx context.Context, tx gdb.TX, id int64, status int, auditBy int64, remark string, onlyPending bool) (bool, error) {
	now := time.Now()
	data := g.Map{
		"status":       status,
		"audit_by":     auditBy,
		"audit_at":     now,
		"audit_remark": remark,
		"updated_at":   now,
	}

	if status == cobo.WithdrawStatusApproved {
		data["auto_approved"] = false
	}

	var (
		result sql.Result
		err    error
	)
	if tx != nil {
		model := tx.Model("cobo_withdraw_request").
			Ctx(ctx).
			Data(data)
		if onlyPending {
			model = model.Where("id = ? AND status = ?", id, cobo.WithdrawStatusPending)
		} else {
			model = model.Where("id = ?", id)
		}
		result, err = model.Update()
	} else {
		model := g.DB().Model("cobo_withdraw_request").
			Ctx(ctx).
			Data(data)
		if onlyPending {
			model = model.Where("id = ? AND status = ?", id, cobo.WithdrawStatusPending)
		} else {
			model = model.Where("id = ?", id)
		}
		result, err = model.Update()
	}
	if err != nil {
		return false, err
	}
	if result == nil {
		return false, nil
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// UpdateCoboRequestID 预写入Cobo请求ID（API调用前，确保webhook能查到记录）
func (r *withdrawRepository) UpdateCoboRequestID(ctx context.Context, id int64, requestID string) error {
	_, err := g.DB().Model("cobo_withdraw_request").
		Ctx(ctx).
		Data(g.Map{
			"cobo_request_id": requestID,
			"updated_at":      time.Now(),
		}).
		Where("id = ?", id).
		Update()
	return err
}

// UpdateCoboInfo 更新Cobo出金信息
func (r *withdrawRepository) UpdateCoboInfo(ctx context.Context, id int64, requestID, response, txHash string) error {
	_, err := g.DB().Model("cobo_withdraw_request").
		Ctx(ctx).
		Data(g.Map{
			"cobo_request_id": requestID,
			"cobo_response":   response,
			"tx_hash":         txHash,
			"updated_at":      time.Now(),
		}).
		Where("id = ?", id).
		Update()
	return err
}

// GetByCoboRequestID 根据Cobo请求ID获取提现申请
func (r *withdrawRepository) GetByCoboRequestID(ctx context.Context, requestID string) (*cobo.WithdrawEntity, error) {
	var entity cobo.WithdrawEntity
	err := g.DB().Model("cobo_withdraw_request").
		Ctx(ctx).
		Where("cobo_request_id = ?", requestID).
		Scan(&entity)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || isNoRowsErr(err) {
			return nil, nil
		}
		return nil, err
	}
	return &entity, nil
}

// UpdateTxHashAndStatus 更新交易哈希和状态
func (r *withdrawRepository) UpdateTxHashAndStatus(ctx context.Context, id int64, txHash string, status int) error {
	_, err := g.DB().Model("cobo_withdraw_request").
		Ctx(ctx).
		Data(g.Map{
			"tx_hash":    txHash,
			"status":     status,
			"updated_at": time.Now(),
		}).
		Where("id = ?", id).
		Update()
	return err
}

// GetByWebhookID 根据Webhook ID获取提现申请（幂等检查）
func (r *withdrawRepository) GetByWebhookID(ctx context.Context, webhookID string) (*cobo.WithdrawEntity, error) {
	if webhookID == "" {
		return nil, nil
	}
	var entity cobo.WithdrawEntity
	err := g.DB().Model("cobo_withdraw_request").
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

// UpdateWebhookID 更新Webhook ID
func (r *withdrawRepository) UpdateWebhookID(ctx context.Context, id int64, webhookID string) error {
	_, err := g.DB().Model("cobo_withdraw_request").
		Ctx(ctx).
		Data(g.Map{
			"webhook_id": webhookID,
			"updated_at": time.Now(),
		}).
		Where("id = ?", id).
		Update()
	return err
}
