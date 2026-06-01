package cobo

import (
	"context"

	coboEntity "XWFrame/internal/entity/cobo"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/shopspring/decimal"
)

// IPerformanceDao cobo_performance 数据访问接口
type IPerformanceDao interface {
	// GetByUserID 根据用户ID获取缓存业绩
	GetByUserID(ctx context.Context, userID int64) (*coboEntity.PerformanceEntity, error)

	// GetByInviteCode 根据邀请码获取缓存业绩
	GetByInviteCode(ctx context.Context, inviteCode string) (*coboEntity.PerformanceEntity, error)

	// GetByWalletAddress 根据钱包地址获取缓存业绩
	GetByWalletAddress(ctx context.Context, walletAddress string) (*coboEntity.PerformanceEntity, error)

	// GetDirectList 获取直推用户的缓存业绩列表
	GetDirectList(ctx context.Context, parentInviteCode string, page, pageSize int) ([]*coboEntity.PerformanceEntity, int, error)

	// Upsert 插入或更新缓存业绩
	Upsert(ctx context.Context, entity *coboEntity.PerformanceEntity) error

	// BatchUpsert 批量插入或更新缓存业绩
	BatchUpsert(ctx context.Context, entities []*coboEntity.PerformanceEntity) error

	// GetLatestPurchaseID 获取 cobo_node_purchase 表的最大ID
	GetLatestPurchaseID(ctx context.Context) (int64, error)

	// GetPurchaseCount 获取 cobo_node_purchase 表的记录数（非赠送）
	GetPurchaseCount(ctx context.Context) (int, error)

	// Count 获取缓存表记录数
	Count(ctx context.Context) (int, error)

	// GetStaleUsers 获取需要更新的用户列表（超过5分钟未更新）
	GetStaleUsers(ctx context.Context, limit int) ([]*coboEntity.PerformanceEntity, error)

	// GetAffectedUsersByPurchaseRange 获取指定ID范围内新购买记录涉及的用户ID（包括购买用户及其祖先）
	GetAffectedUsersByPurchaseRange(ctx context.Context, minID, maxID int64) ([]int64, error)

	// CalculatePersonalPerformance 计算用户个人业绩（从 cobo_node_purchase 实时计算）
	CalculatePersonalPerformance(ctx context.Context, userID int64) (decimal.Decimal, int, int64, error)

	// CalculateTeamPerformance 计算用户团队业绩（从 cobo_node_purchase 实时计算）
	CalculateTeamPerformance(ctx context.Context, inviteCode string) (decimal.Decimal, int, error)

	// CalculateSmallTeamPerformance 计算用户小区业绩（团队业绩 - 最大直推分支业绩）
	CalculateSmallTeamPerformance(ctx context.Context, inviteCode string) (decimal.Decimal, error)
}

// performanceDao cobo_performance 数据访问实现
type performanceDao struct {
	db gdb.DB
}

// NewPerformanceDao 创建数据访问实例
func NewPerformanceDao() IPerformanceDao {
	return &performanceDao{
		db: db.GetDB(),
	}
}

// GetByUserID 根据用户ID获取缓存业绩
func (d *performanceDao) GetByUserID(ctx context.Context, userID int64) (*coboEntity.PerformanceEntity, error) {
	var entity coboEntity.PerformanceEntity
	err := d.db.Model("cobo_performance").Ctx(ctx).
		Where("user_id", userID).
		Scan(&entity)
	if err != nil {
		return nil, err
	}
	if entity.UserID == 0 {
		return nil, nil
	}
	return &entity, nil
}

// GetByInviteCode 根据邀请码获取缓存业绩
func (d *performanceDao) GetByInviteCode(ctx context.Context, inviteCode string) (*coboEntity.PerformanceEntity, error) {
	var entity coboEntity.PerformanceEntity
	err := d.db.Model("cobo_performance").Ctx(ctx).
		Where("invite_code", inviteCode).
		Scan(&entity)
	if err != nil {
		return nil, err
	}
	if entity.UserID == 0 {
		return nil, nil
	}
	return &entity, nil
}

// GetByWalletAddress 根据钱包地址获取缓存业绩
func (d *performanceDao) GetByWalletAddress(ctx context.Context, walletAddress string) (*coboEntity.PerformanceEntity, error) {
	var entity coboEntity.PerformanceEntity
	err := d.db.Model("cobo_performance").Ctx(ctx).
		Where("wallet_address", walletAddress).
		Scan(&entity)
	if err != nil {
		return nil, err
	}
	if entity.UserID == 0 {
		return nil, nil
	}
	return &entity, nil
}

// GetDirectList 获取直推用户的缓存业绩列表
func (d *performanceDao) GetDirectList(ctx context.Context, parentInviteCode string, page, pageSize int) ([]*coboEntity.PerformanceEntity, int, error) {
	// 先查询总数
	total, err := d.db.Model("user_info").Ctx(ctx).
		Where("parent_invite_code", parentInviteCode).
		Count()
	if err != nil {
		return nil, 0, err
	}

	// 查询直推用户的缓存业绩
	sql := `
		SELECT cp.*
		FROM cobo_performance cp
		INNER JOIN user_info u ON u.id = cp.user_id
		WHERE u.parent_invite_code = ?
		ORDER BY cp.personal_performance DESC
		LIMIT ? OFFSET ?
	`

	var list []*coboEntity.PerformanceEntity
	offset := (page - 1) * pageSize
	err = d.db.Ctx(ctx).Raw(sql, parentInviteCode, pageSize, offset).Scan(&list)
	if err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// Upsert 插入或更新缓存业绩
func (d *performanceDao) Upsert(ctx context.Context, entity *coboEntity.PerformanceEntity) error {
	_, err := d.db.Exec(ctx, `
		INSERT INTO cobo_performance (
			user_id, wallet_address, invite_code,
			personal_performance, team_performance, small_team_performance,
			direct_count, team_total_count,
			purchase_count, max_purchase_id,
			updated_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id) DO UPDATE SET
			wallet_address = EXCLUDED.wallet_address,
			invite_code = EXCLUDED.invite_code,
			personal_performance = EXCLUDED.personal_performance,
			team_performance = EXCLUDED.team_performance,
			small_team_performance = EXCLUDED.small_team_performance,
			direct_count = EXCLUDED.direct_count,
			team_total_count = EXCLUDED.team_total_count,
			purchase_count = EXCLUDED.purchase_count,
			max_purchase_id = EXCLUDED.max_purchase_id,
			updated_at = CURRENT_TIMESTAMP
	`, entity.UserID, entity.WalletAddress, entity.InviteCode,
		entity.PersonalPerformance, entity.TeamPerformance, entity.SmallTeamPerformance,
		entity.DirectCount, entity.TeamTotalCount,
		entity.PurchaseCount, entity.MaxPurchaseID)

	return err
}

// BatchUpsert 批量插入或更新缓存业绩
func (d *performanceDao) BatchUpsert(ctx context.Context, entities []*coboEntity.PerformanceEntity) error {
	if len(entities) == 0 {
		return nil
	}

	// 使用事务批量处理
	err := d.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		for _, entity := range entities {
			_, err := tx.Exec(`
				INSERT INTO cobo_performance (
					user_id, wallet_address, invite_code,
					personal_performance, team_performance, small_team_performance,
					direct_count, team_total_count,
					purchase_count, max_purchase_id,
					updated_at, created_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
				ON CONFLICT (user_id) DO UPDATE SET
					wallet_address = EXCLUDED.wallet_address,
					invite_code = EXCLUDED.invite_code,
					personal_performance = EXCLUDED.personal_performance,
					team_performance = EXCLUDED.team_performance,
					small_team_performance = EXCLUDED.small_team_performance,
					direct_count = EXCLUDED.direct_count,
					team_total_count = EXCLUDED.team_total_count,
					purchase_count = EXCLUDED.purchase_count,
					max_purchase_id = EXCLUDED.max_purchase_id,
					updated_at = CURRENT_TIMESTAMP
			`, entity.UserID, entity.WalletAddress, entity.InviteCode,
				entity.PersonalPerformance, entity.TeamPerformance, entity.SmallTeamPerformance,
				entity.DirectCount, entity.TeamTotalCount,
				entity.PurchaseCount, entity.MaxPurchaseID)
			if err != nil {
				return err
			}
		}
		return nil
	})

	return err
}

// GetLatestPurchaseID 获取 cobo_node_purchase 表的最大ID
func (d *performanceDao) GetLatestPurchaseID(ctx context.Context) (int64, error) {
	var result struct {
		MaxID int64 `json:"max_id"`
	}
	err := d.db.Model("cobo_node_purchase").Ctx(ctx).
		Fields("COALESCE(MAX(id), 0) as max_id").
		Scan(&result)
	if err != nil {
		return 0, err
	}
	return result.MaxID, nil
}

// GetPurchaseCount 获取 cobo_node_purchase 表的记录数（非赠送）
func (d *performanceDao) GetPurchaseCount(ctx context.Context) (int, error) {
	count, err := d.db.Model("cobo_node_purchase").Ctx(ctx).
		Where("is_gift", 0).
		Count()
	return count, err
}

// Count 获取缓存表记录数
func (d *performanceDao) Count(ctx context.Context) (int, error) {
	count, err := d.db.Model("cobo_performance").Ctx(ctx).Count()
	return count, err
}

// GetStaleUsers 获取需要更新的用户列表（超过5分钟未更新）
func (d *performanceDao) GetStaleUsers(ctx context.Context, limit int) ([]*coboEntity.PerformanceEntity, error) {
	var list []*coboEntity.PerformanceEntity
	err := d.db.Model("cobo_performance").Ctx(ctx).
		Where("updated_at < CURRENT_TIMESTAMP - INTERVAL '5 minutes'").
		Order("updated_at ASC").
		Limit(limit).
		Scan(&list)
	return list, err
}

// CalculatePersonalPerformance 计算用户个人业绩（从 cobo_node_purchase 实时计算）
func (d *performanceDao) CalculatePersonalPerformance(ctx context.Context, userID int64) (decimal.Decimal, int, int64, error) {
	var result struct {
		TotalAmount decimal.Decimal `json:"total_amount"`
		Count       int             `json:"count"`
		MaxID       int64           `json:"max_id"`
	}

	err := d.db.Model("cobo_node_purchase").Ctx(ctx).
		Fields("COALESCE(SUM(amount), 0) as total_amount, COUNT(*) as count, COALESCE(MAX(id), 0) as max_id").
		Where("user_id", userID).
		Where("is_gift", 0).
		Scan(&result)

	if err != nil {
		return decimal.Zero, 0, 0, err
	}

	return result.TotalAmount, result.Count, result.MaxID, nil
}

// GetAffectedUsersByPurchaseRange 获取指定ID范围内新购买记录涉及的用户ID（包括购买用户及其祖先）
func (d *performanceDao) GetAffectedUsersByPurchaseRange(ctx context.Context, minID, maxID int64) ([]int64, error) {
	sql := `
		WITH RECURSIVE affected_users AS (
			-- 获取新购买记录的用户ID
			SELECT DISTINCT user_id
			FROM cobo_node_purchase
			WHERE id > ? AND id <= ?
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
			WHERE ac.level < 20 -- 最多4层，但为了防止循环，设置一个安全上限
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

// CalculateTeamPerformance 计算用户团队业绩（从 cobo_node_purchase 实时计算）
func (d *performanceDao) CalculateTeamPerformance(ctx context.Context, inviteCode string) (decimal.Decimal, int, error) {
	var result struct {
		TotalAmount decimal.Decimal `json:"total_amount"`
		Count       int             `json:"count"`
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
		SELECT COALESCE(SUM(cnp.amount), 0) as total_amount, COUNT(*) as count
		FROM cobo_node_purchase cnp
		INNER JOIN team_tree t ON t.id = cnp.user_id
		WHERE cnp.is_gift = 0
	`

	err := d.db.Ctx(ctx).Raw(sql, inviteCode).Scan(&result)
	if err != nil {
		return decimal.Zero, 0, err
	}

	return result.TotalAmount, result.Count, nil
}

// CalculateSmallTeamPerformance 计算用户小区业绩（团队业绩 - 最大直推分支业绩）
func (d *performanceDao) CalculateSmallTeamPerformance(ctx context.Context, inviteCode string) (decimal.Decimal, error) {
	var result struct {
		SmallTeamPerformance decimal.Decimal `json:"small_team_performance"`
	}

	sql := `
		WITH RECURSIVE team_tree AS (
			SELECT id, invite_code, id AS root_id
			FROM user_info
			WHERE parent_invite_code = ?

			UNION ALL

			SELECT u.id, u.invite_code, t.root_id
			FROM user_info u
			INNER JOIN team_tree t ON u.parent_invite_code = t.invite_code
		),
		branch_performance AS (
			SELECT
				t.root_id,
				COALESCE(SUM(np.amount), 0) AS perf
			FROM team_tree t
			LEFT JOIN cobo_node_purchase np ON np.user_id = t.id AND np.is_gift = 0
			GROUP BY t.root_id
		)
		SELECT
			COALESCE(SUM(perf), 0)
			- COALESCE((SELECT MAX(perf) FROM branch_performance), 0)
			AS small_team_performance
		FROM branch_performance
	`

	err := d.db.Ctx(ctx).Raw(sql, inviteCode).Scan(&result)
	if err != nil {
		return decimal.Zero, err
	}

	return result.SmallTeamPerformance, nil
}
