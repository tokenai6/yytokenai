package dao

import (
	"context"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/shopspring/decimal"
)

// IStakingV2PerformanceDao staking_v2_performance 数据访问接口
type IStakingV2PerformanceDao interface {
	// GetByUserID 根据用户ID获取缓存业绩
	GetByUserID(ctx context.Context, userID int64) (*entity.StakingV2PerformanceEntity, error)

	// GetByInviteCode 根据邀请码获取缓存业绩
	GetByInviteCode(ctx context.Context, inviteCode string) (*entity.StakingV2PerformanceEntity, error)

	// GetByWalletAddress 根据钱包地址获取缓存业绩
	GetByWalletAddress(ctx context.Context, walletAddress string) (*entity.StakingV2PerformanceEntity, error)

	// Upsert 插入或更新缓存业绩
	Upsert(ctx context.Context, entity *entity.StakingV2PerformanceEntity) error

	// BatchUpsert 批量插入或更新缓存业绩
	BatchUpsert(ctx context.Context, entities []*entity.StakingV2PerformanceEntity) error

	// GetLatestOrderID 获取 staking_v2_order 表的最大ID（source_type=1）
	GetLatestOrderID(ctx context.Context) (int64, error)

	// Count 获取缓存表记录数
	Count(ctx context.Context) (int, error)

	// GetStaleUsers 获取需要更新的用户列表（超过5分钟未更新）
	GetStaleUsers(ctx context.Context, limit int) ([]*entity.StakingV2PerformanceEntity, error)

	// GetAffectedUsersByOrderRange 获取指定ID范围内新订单涉及的用户ID（包括购买用户及其祖先）
	GetAffectedUsersByOrderRange(ctx context.Context, minID, maxID int64) ([]int64, error)

	// CalculatePersonalPerformance 计算用户个人业绩（从 staking_v2_order 实时计算）
	CalculatePersonalPerformance(ctx context.Context, userID int64) (decimal.Decimal, int, int64, error)

	// CalculateTeamPerformance 计算用户团队业绩（从 staking_v2_order 实时计算）
	CalculateTeamPerformance(ctx context.Context, inviteCode string) (decimal.Decimal, error)
}

// stakingV2PerformanceDao staking_v2_performance 数据访问实现
type stakingV2PerformanceDao struct {
	db gdb.DB
}

// NewStakingV2PerformanceDao 创建数据访问实例
func NewStakingV2PerformanceDao() IStakingV2PerformanceDao {
	return &stakingV2PerformanceDao{
		db: db.GetDB(),
	}
}

// GetByUserID 根据用户ID获取缓存业绩
func (d *stakingV2PerformanceDao) GetByUserID(ctx context.Context, userID int64) (*entity.StakingV2PerformanceEntity, error) {
	var e entity.StakingV2PerformanceEntity
	err := d.db.Model("staking_v2_performance").Ctx(ctx).
		Where("user_id", userID).
		Scan(&e)
	if err != nil {
		return nil, err
	}
	if e.UserID == 0 {
		return nil, nil
	}
	return &e, nil
}

// GetByInviteCode 根据邀请码获取缓存业绩
func (d *stakingV2PerformanceDao) GetByInviteCode(ctx context.Context, inviteCode string) (*entity.StakingV2PerformanceEntity, error) {
	var e entity.StakingV2PerformanceEntity
	err := d.db.Model("staking_v2_performance").Ctx(ctx).
		Where("invite_code", inviteCode).
		Scan(&e)
	if err != nil {
		return nil, err
	}
	if e.UserID == 0 {
		return nil, nil
	}
	return &e, nil
}

// GetByWalletAddress 根据钱包地址获取缓存业绩
func (d *stakingV2PerformanceDao) GetByWalletAddress(ctx context.Context, walletAddress string) (*entity.StakingV2PerformanceEntity, error) {
	var e entity.StakingV2PerformanceEntity
	err := d.db.Model("staking_v2_performance").Ctx(ctx).
		Where("wallet_address", walletAddress).
		Scan(&e)
	if err != nil {
		return nil, err
	}
	if e.UserID == 0 {
		return nil, nil
	}
	return &e, nil
}

// Upsert 插入或更新缓存业绩
func (d *stakingV2PerformanceDao) Upsert(ctx context.Context, e *entity.StakingV2PerformanceEntity) error {
	_, err := d.db.Exec(ctx, `
		INSERT INTO staking_v2_performance (
			user_id, wallet_address, invite_code,
			personal_performance, team_performance,
			order_count, max_order_id,
			updated_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id) DO UPDATE SET
			wallet_address = EXCLUDED.wallet_address,
			invite_code = EXCLUDED.invite_code,
			personal_performance = EXCLUDED.personal_performance,
			team_performance = EXCLUDED.team_performance,
			order_count = EXCLUDED.order_count,
			max_order_id = EXCLUDED.max_order_id,
			updated_at = CURRENT_TIMESTAMP
	`, e.UserID, e.WalletAddress, e.InviteCode,
		e.PersonalPerformance, e.TeamPerformance,
		e.OrderCount, e.MaxOrderID)

	return err
}

// BatchUpsert 批量插入或更新缓存业绩
func (d *stakingV2PerformanceDao) BatchUpsert(ctx context.Context, entities []*entity.StakingV2PerformanceEntity) error {
	if len(entities) == 0 {
		return nil
	}

	err := d.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		for _, e := range entities {
			_, err := tx.Exec(`
				INSERT INTO staking_v2_performance (
					user_id, wallet_address, invite_code,
					personal_performance, team_performance,
					order_count, max_order_id,
					updated_at, created_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
				ON CONFLICT (user_id) DO UPDATE SET
					wallet_address = EXCLUDED.wallet_address,
					invite_code = EXCLUDED.invite_code,
					personal_performance = EXCLUDED.personal_performance,
					team_performance = EXCLUDED.team_performance,
					order_count = EXCLUDED.order_count,
					max_order_id = EXCLUDED.max_order_id,
					updated_at = CURRENT_TIMESTAMP
			`, e.UserID, e.WalletAddress, e.InviteCode,
				e.PersonalPerformance, e.TeamPerformance,
				e.OrderCount, e.MaxOrderID)
			if err != nil {
				return err
			}
		}
		return nil
	})

	return err
}

// GetLatestOrderID 获取 staking_v2_order 表的最大ID（source_type=1）
func (d *stakingV2PerformanceDao) GetLatestOrderID(ctx context.Context) (int64, error) {
	var result struct {
		MaxID int64 `json:"max_id"`
	}
	err := d.db.Model("staking_v2_order").Ctx(ctx).
		Fields("COALESCE(MAX(id), 0) as max_id").
		Where("source_type", 1).
		Where("is_gift", 0).
		Scan(&result)
	if err != nil {
		return 0, err
	}
	return result.MaxID, nil
}

// Count 获取缓存表记录数
func (d *stakingV2PerformanceDao) Count(ctx context.Context) (int, error) {
	count, err := d.db.Model("staking_v2_performance").Ctx(ctx).Count()
	return count, err
}

// GetStaleUsers 获取需要更新的用户列表（超过5分钟未更新）
func (d *stakingV2PerformanceDao) GetStaleUsers(ctx context.Context, limit int) ([]*entity.StakingV2PerformanceEntity, error) {
	var list []*entity.StakingV2PerformanceEntity
	err := d.db.Model("staking_v2_performance").Ctx(ctx).
		Where("updated_at < CURRENT_TIMESTAMP - INTERVAL '5 minutes'").
		Order("updated_at ASC").
		Limit(limit).
		Scan(&list)
	return list, err
}

// GetAffectedUsersByOrderRange 获取指定ID范围内新订单涉及的用户ID（包括购买用户及其祖先）
func (d *stakingV2PerformanceDao) GetAffectedUsersByOrderRange(ctx context.Context, minID, maxID int64) ([]int64, error) {
	sql := `
		WITH RECURSIVE affected_users AS (
			-- 获取新订单的用户ID
			SELECT DISTINCT user_id
			FROM staking_v2_order
			WHERE id > ? AND id <= ? AND source_type = 1 AND is_gift = 0
		),
		ancestor_chain AS (
			-- 获取这些用户的所有上级（祖先）
			SELECT u.id, u.invite_code, u.parent_invite_code, 0 as level
			FROM user_info u
			INNER JOIN affected_users au ON u.id = au.user_id

			UNION ALL

			SELECT u.id, u.invite_code, u.parent_invite_code, ac.level + 1
			FROM user_info u
			INNER JOIN ancestor_chain ac ON u.invite_code = ac.parent_invite_code
			WHERE ac.level < 20 -- 安全上限，防止循环
		)
		SELECT DISTINCT id as user_id FROM ancestor_chain
		ORDER BY id
	`

	var results []struct {
		UserID int64 `json:"user_id"`
	}
	err := d.db.Ctx(ctx).Raw(sql, minID, maxID).Scan(&results)
	if err != nil {
		return nil, err
	}

	userIDs := make([]int64, 0, len(results))
	for _, r := range results {
		userIDs = append(userIDs, r.UserID)
	}
	return userIDs, nil
}

// CalculatePersonalPerformance 计算用户个人业绩（从 staking_v2_order 实时计算）
func (d *stakingV2PerformanceDao) CalculatePersonalPerformance(ctx context.Context, userID int64) (decimal.Decimal, int, int64, error) {
	var result struct {
		TotalAmount decimal.Decimal `json:"total_amount"`
		Count       int             `json:"count"`
		MaxID       int64           `json:"max_id"`
	}

	err := d.db.Model("staking_v2_order").Ctx(ctx).
		Fields("COALESCE(SUM(amount), 0) as total_amount, COUNT(*) as count, COALESCE(MAX(id), 0) as max_id").
		Where("user_id", userID).
		Where("source_type", 1).
		Where("is_gift", 0).
		Scan(&result)

	if err != nil {
		return decimal.Zero, 0, 0, err
	}

	return result.TotalAmount, result.Count, result.MaxID, nil
}

// CalculateTeamPerformance 计算用户团队业绩（从 staking_v2_order 实时计算）
func (d *stakingV2PerformanceDao) CalculateTeamPerformance(ctx context.Context, inviteCode string) (decimal.Decimal, error) {
	var result struct {
		TotalAmount decimal.Decimal `json:"total_amount"`
	}

	sql := `
		WITH RECURSIVE team_tree AS (
			SELECT id, invite_code
			FROM user_info
			WHERE parent_invite_code = ?

			UNION ALL

			SELECT u.id, u.invite_code
			FROM user_info u
			INNER JOIN team_tree t ON u.parent_invite_code = t.invite_code
		)
		SELECT COALESCE(SUM(o.amount), 0) as total_amount
		FROM staking_v2_order o
		INNER JOIN team_tree t ON t.id = o.user_id
		WHERE o.source_type = 1 AND o.is_gift = 0
	`

	err := d.db.Ctx(ctx).Raw(sql, inviteCode).Scan(&result)
	if err != nil {
		return decimal.Zero, err
	}

	return result.TotalAmount, nil
}
