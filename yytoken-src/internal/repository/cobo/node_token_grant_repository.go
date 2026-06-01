package cobo

import (
	"context"
	"time"

	"XWFrame/internal/entity/cobo"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

type nodeTokenGrantRepository struct{}

func NewNodeTokenGrantRepository() INodeTokenGrantRepository {
	return &nodeTokenGrantRepository{}
}

func (r *nodeTokenGrantRepository) CreateTx(ctx context.Context, tx gdb.TX, entity *cobo.NodeTokenGrantEntity) error {
	now := time.Now()
	entity.CreatedAt = now
	entity.UpdatedAt = now
	_, err := tx.Model(entity.TableName()).Ctx(ctx).FieldsEx("id").Data(entity).Insert()
	return err
}

func (r *nodeTokenGrantRepository) GetByUserID(ctx context.Context, userID int64, tokenType string, page, pageSize int) ([]*cobo.NodeTokenGrantEntity, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	var (
		rows  []*cobo.NodeTokenGrantEntity
		total int64
	)

	m := g.DB().Model("cobo_node_token_grant").Ctx(ctx).Where("user_id = ?", userID)
	if tokenType != "" {
		m = m.Where("token_type = ?", tokenType)
	}
	count, err := m.Count()
	if err != nil {
		return nil, 0, err
	}
	total = int64(count)

	err = m.Order("id DESC").Limit(pageSize).Offset((page - 1) * pageSize).Scan(&rows)
	if err != nil {
		return nil, 0, err
	}

	return rows, total, nil
}

func (r *nodeTokenGrantRepository) GetPendingTotalByUserID(ctx context.Context, userID int64) (decimal.Decimal, error) {
	var total decimal.Decimal
	value, err := g.DB().Model("cobo_node_token_grant").Ctx(ctx).
		Fields("COALESCE(SUM(grant_amount::numeric), 0)").
		Where("user_id = ?", userID).
		Where("token_type = ?", "YYAI").
		Where("status = ?", 0).
		Value()
	if err != nil {
		return decimal.Zero, err
	}
	total, err = decimal.NewFromString(value.String())
	if err != nil {
		return decimal.Zero, err
	}
	return total, nil
}

func (r *nodeTokenGrantRepository) GetPendingTotalByUserIDAndType(ctx context.Context, userID int64, tokenType string) (decimal.Decimal, error) {
	var total decimal.Decimal
	value, err := g.DB().Model("cobo_node_token_grant").Ctx(ctx).
		Fields("COALESCE(SUM(grant_amount::numeric), 0)").
		Where("user_id = ?", userID).
		Where("token_type = ?", tokenType).
		Where("status = ?", 0).
		Value()
	if err != nil {
		return decimal.Zero, err
	}
	total, err = decimal.NewFromString(value.String())
	if err != nil {
		return decimal.Zero, err
	}
	return total, nil
}
