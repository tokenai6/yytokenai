package cobo

import (
	"context"
	"fmt"
	"time"

	coboEntity "XWFrame/internal/entity/cobo"
	"XWFrame/internal/repository"
	coboRepo "XWFrame/internal/repository/cobo"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// IPerformanceService cobo_performance 服务接口
type IPerformanceService interface {
	// GetByUserID 根据用户ID获取缓存业绩（如果不存在或过期则自动刷新）
	GetByUserID(ctx context.Context, userID int64) (*coboEntity.PerformanceEntity, error)

	// GetByInviteCode 根据邀请码获取缓存业绩
	GetByInviteCode(ctx context.Context, inviteCode string) (*coboEntity.PerformanceEntity, error)

	// GetDirectList 获取直推用户的缓存业绩列表
	GetDirectList(ctx context.Context, parentInviteCode string, page, pageSize int) ([]*coboEntity.PerformanceEntity, int, error)

	// RefreshCache 刷新指定用户的缓存业绩
	RefreshCache(ctx context.Context, userID int64) (*coboEntity.PerformanceEntity, error)

	// RefreshAllCache 刷新所有缓存（全量刷新）
	RefreshAllCache(ctx context.Context) error

	// IncrementalUpdate 增量更新（只更新有变化的）
	IncrementalUpdate(ctx context.Context) error

	// GetTeamOverview 获取团队概览（用于 /api/v1/team/overview）
	GetTeamOverview(ctx context.Context, userID int64) (*TeamOverviewResult, error)

	// NeedRefresh 检查是否需要刷新
	NeedRefresh(ctx context.Context) (bool, error)

	// UpdateGiftPerformancePass 更新赠送节点的业绩及格标记
	UpdateGiftPerformancePass(ctx context.Context) error
}

// TeamOverviewResult 团队概览结果
type TeamOverviewResult struct {
	TeamTotalUserCount    int
	TeamTodayNewUserCount int
	DirectCount           int
	RewardLevel           int
	BigTeamCount          int
	SmallTeamSum          int
	TeamPerformance       string
	TeamYYSum             string
}

// performanceService cobo_performance 服务实现
type performanceService struct {
	repo     coboRepo.IPerformanceRepository
	userRepo repository.IUserRepository
}

// NewPerformanceService 创建服务实例
func NewPerformanceService() IPerformanceService {
	return &performanceService{
		repo:     coboRepo.NewPerformanceRepository(),
		userRepo: repository.NewUserRepository(),
	}
}

// GetByUserID 根据用户ID获取缓存业绩（如果不存在或过期则自动刷新）
func (s *performanceService) GetByUserID(ctx context.Context, userID int64) (*coboEntity.PerformanceEntity, error) {
	// 先尝试从缓存获取
	entity, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 如果不存在或过期，刷新缓存
	if entity == nil || !entity.IsFresh() {
		return s.RefreshCache(ctx, userID)
	}

	return entity, nil
}

// GetByInviteCode 根据邀请码获取缓存业绩
func (s *performanceService) GetByInviteCode(ctx context.Context, inviteCode string) (*coboEntity.PerformanceEntity, error) {
	// 先尝试从缓存获取
	entity, err := s.repo.GetByInviteCode(ctx, inviteCode)
	if err != nil {
		return nil, err
	}

	// 如果不存在或过期，需要找到对应的 userID 刷新
	if entity == nil || !entity.IsFresh() {
		// 查询用户信息
		userInfo, err := s.userRepo.GetUserByInviteCode(ctx, inviteCode)
		if err != nil {
			return nil, err
		}
		if userInfo == nil {
			return nil, nil
		}
		return s.RefreshCache(ctx, userInfo.Id)
	}

	return entity, nil
}

// GetDirectList 获取直推用户的缓存业绩列表
func (s *performanceService) GetDirectList(ctx context.Context, parentInviteCode string, page, pageSize int) ([]*coboEntity.PerformanceEntity, int, error) {
	return s.repo.GetDirectList(ctx, parentInviteCode, page, pageSize)
}

// RefreshCache 刷新指定用户的缓存业绩
func (s *performanceService) RefreshCache(ctx context.Context, userID int64) (*coboEntity.PerformanceEntity, error) {
	// 获取用户信息
	userInfo, err := s.userRepo.GetUserById(ctx, userID)
	if err != nil {
		return nil, err
	}
	if userInfo == nil {
		return nil, fmt.Errorf("用户不存在: %d", userID)
	}

	// 计算个人业绩
	personalPerf, purchaseCount, maxPurchaseID, err := s.repo.CalculatePersonalPerformance(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 计算团队业绩
	teamPerf, _, err := s.repo.CalculateTeamPerformance(ctx, userInfo.InviteCode)
	if err != nil {
		return nil, err
	}

	// 计算小区业绩
	smallTeamPerf, err := s.repo.CalculateSmallTeamPerformance(ctx, userInfo.InviteCode)
	if err != nil {
		return nil, err
	}

	// 获取直推人数
	directCount, err := s.getDirectCount(ctx, userInfo.InviteCode)
	if err != nil {
		return nil, err
	}

	// 获取团队总人数（简化计算，可以优化）
	teamTotalCount, err := s.getTeamTotalCount(ctx, userInfo.InviteCode)
	if err != nil {
		return nil, err
	}

	// 创建缓存实体
	entity := &coboEntity.PerformanceEntity{
		UserID:               userID,
		WalletAddress:        userInfo.WalletAddress,
		InviteCode:           userInfo.InviteCode,
		PersonalPerformance:  personalPerf,
		TeamPerformance:      teamPerf,
		SmallTeamPerformance: smallTeamPerf,
		DirectCount:          directCount,
		TeamTotalCount:       teamTotalCount,
		PurchaseCount:        purchaseCount,
		MaxPurchaseID:        maxPurchaseID,
		UpdatedAt:            time.Now(),
		CreatedAt:            time.Now(),
	}

	// 保存到缓存表
	if err := s.repo.Upsert(ctx, entity); err != nil {
		return nil, err
	}

	return entity, nil
}

// RefreshAllCache 刷新所有缓存（全量刷新，自底向上避免重复计算）
func (s *performanceService) RefreshAllCache(ctx context.Context) error {
	startTime := time.Now()
	g.Log().Info(ctx, "[PerformanceService] 开始全量刷新缓存（自底向上）")

	// 1. 获取所有用户关系（不限status，确保下级购买记录被完整统计）
	type userRow struct {
		ID               int64  `json:"id"`
		WalletAddress    string `json:"wallet_address"`
		InviteCode       string `json:"invite_code"`
		ParentInviteCode string `json:"parent_invite_code"`
		Status           int    `json:"status"`
	}
	var users []userRow
	if err := g.DB().Model("user_info").Ctx(ctx).
		Fields("id, wallet_address, invite_code, parent_invite_code, status").
		Scan(&users); err != nil {
		return err
	}

	// 2. 获取所有个人业绩
	type perfRow struct {
		UserID              int64           `json:"user_id"`
		PersonalPerformance decimal.Decimal `json:"personal_performance"`
		PurchaseCount       int             `json:"purchase_count"`
		MaxPurchaseID       int64           `json:"max_purchase_id"`
	}
	var personalPerfs []perfRow
	if err := g.DB().Ctx(ctx).Raw(`
		SELECT user_id,
			COALESCE(SUM(amount), 0) as personal_performance,
			COUNT(*) as purchase_count,
			COALESCE(MAX(id), 0) as max_purchase_id
		FROM cobo_node_purchase
		WHERE is_gift = 0
		GROUP BY user_id
	`).Scan(&personalPerfs); err != nil {
		return err
	}

	// 3. 构建内存树
	type perfNode struct {
		userID           int64
		walletAddress    string
		inviteCode       string
		parentInviteCode string
		status           int
		personalPerf     decimal.Decimal
		purchaseCount    int
		maxPurchaseID    int64
		children         []*perfNode
		teamPerf         decimal.Decimal
		smallTeamPerf    decimal.Decimal
		teamTotalCount   int
	}

	nodeMap := make(map[string]*perfNode, len(users))
	idMap := make(map[int64]*perfNode, len(users))
	for _, u := range users {
		n := &perfNode{
			userID:           u.ID,
			walletAddress:    u.WalletAddress,
			inviteCode:       u.InviteCode,
			parentInviteCode: u.ParentInviteCode,
			status:           u.Status,
		}
		nodeMap[u.InviteCode] = n
		idMap[u.ID] = n
	}
	for _, p := range personalPerfs {
		if n, ok := idMap[p.UserID]; ok {
			n.personalPerf = p.PersonalPerformance
			n.purchaseCount = p.PurchaseCount
			n.maxPurchaseID = p.MaxPurchaseID
		}
	}

	// 建立父子关系
	var roots []*perfNode
	for _, n := range nodeMap {
		if n.parentInviteCode != "" {
			if parent, ok := nodeMap[n.parentInviteCode]; ok {
				parent.children = append(parent.children, n)
			} else {
				roots = append(roots, n)
			}
		} else {
			roots = append(roots, n)
		}
	}

	// 4. 后序遍历 DFS 计算
	var dfs func(n *perfNode, depth int)
	dfs = func(n *perfNode, depth int) {
		if depth > 100 {
			g.Log().Warningf(ctx, "[PerformanceService] 用户 %d 深度超过100，可能存在循环引用", n.userID)
			return
		}
		for _, child := range n.children {
			dfs(child, depth+1)
		}
		branchPerfs := make([]decimal.Decimal, 0, len(n.children))
		for _, child := range n.children {
			branchPerf := child.personalPerf.Add(child.teamPerf)
			branchPerfs = append(branchPerfs, branchPerf)
			n.teamPerf = n.teamPerf.Add(branchPerf)
			n.teamTotalCount += child.teamTotalCount + 1
		}
		if len(branchPerfs) > 1 {
			maxBranch := decimal.Zero
			for _, bp := range branchPerfs {
				if bp.GreaterThan(maxBranch) {
					maxBranch = bp
				}
			}
			n.smallTeamPerf = n.teamPerf.Sub(maxBranch)
		}
	}
	for _, root := range roots {
		dfs(root, 0)
	}

	// 5. 批量写入（只写入 status=1 的用户）
	batchSize := 100
	successCount := 0
	failCount := 0
	for i := 0; i < len(users); i += batchSize {
		end := i + batchSize
		if end > len(users) {
			end = len(users)
		}
		entities := make([]*coboEntity.PerformanceEntity, 0, end-i)
		for _, u := range users[i:end] {
			if u.Status != 1 {
				continue
			}
			n := nodeMap[u.InviteCode]
			entities = append(entities, &coboEntity.PerformanceEntity{
				UserID:               n.userID,
				WalletAddress:        n.walletAddress,
				InviteCode:           n.inviteCode,
				PersonalPerformance:  n.personalPerf,
				TeamPerformance:      n.teamPerf,
				SmallTeamPerformance: n.smallTeamPerf,
				DirectCount:          len(n.children),
				TeamTotalCount:       n.teamTotalCount,
				PurchaseCount:        n.purchaseCount,
				MaxPurchaseID:        n.maxPurchaseID,
				UpdatedAt:            time.Now(),
				CreatedAt:            time.Now(),
			})
		}
		if len(entities) == 0 {
			continue
		}
		if err := s.repo.BatchUpsert(ctx, entities); err != nil {
			g.Log().Warningf(ctx, "[PerformanceService] 批量写入失败: %v", err)
			failCount += len(entities)
		} else {
			successCount += len(entities)
		}
		time.Sleep(50 * time.Millisecond)
	}

	g.Log().Infof(ctx, "[PerformanceService] 全量刷新完成（自底向上），成功: %d, 失败: %d, 耗时: %v",
		successCount, failCount, time.Since(startTime))
	return nil
}

// IncrementalUpdate 增量更新（只更新有变化的）
func (s *performanceService) IncrementalUpdate(ctx context.Context) error {
	startTime := time.Now()

	// 获取最新的 purchase ID
	latestID, err := s.repo.GetLatestPurchaseID(ctx)
	if err != nil {
		return err
	}

	cacheCount, err := s.repo.Count(ctx)
	if err != nil {
		return err
	}

	if cacheCount == 0 {
		g.Log().Info(ctx, "[PerformanceService] 缓存为空，执行全量刷新")
		return s.RefreshAllCache(ctx)
	}

	maxCachedID, err := s.getMaxCachedPurchaseID(ctx)
	if err != nil {
		return err
	}

	staleUsers, err := s.repo.GetStaleUsers(ctx, 1000)
	if err != nil {
		return err
	}

	// 如果没有新数据且没有过期用户，跳过
	if latestID <= maxCachedID && len(staleUsers) == 0 {
		g.Log().Debug(ctx, "[PerformanceService] 没有新数据需要更新")
		return nil
	}

	refreshUserIDs := make([]int64, 0, len(staleUsers))
	userIDSet := make(map[int64]struct{})
	affectedCount := 0

	if latestID > maxCachedID {
		g.Log().Infof(ctx, "[PerformanceService] 检测到新购买记录（latestID=%d, cachedID=%d），获取受影响用户链", latestID, maxCachedID)

		affectedUserIDs, err := s.repo.GetAffectedUsersByPurchaseRange(ctx, maxCachedID, latestID)
		if err != nil {
			g.Log().Warningf(ctx, "[PerformanceService] 获取受影响用户失败: %v，回退到全量刷新", err)
			return s.RefreshAllCache(ctx)
		}

		affectedCount = len(affectedUserIDs)
		if affectedCount == 0 {
			g.Log().Warningf(ctx, "[PerformanceService] 新购买记录范围内未匹配到受影响用户（latestID=%d, cachedID=%d）", latestID, maxCachedID)
			if len(staleUsers) == 0 {
				g.Log().Warning(ctx, "[PerformanceService] 无过期用户可处理，回退到全量刷新避免空转")
				return s.RefreshAllCache(ctx)
			}
		}

		for _, userID := range affectedUserIDs {
			if _, exists := userIDSet[userID]; exists {
				continue
			}
			userIDSet[userID] = struct{}{}
			refreshUserIDs = append(refreshUserIDs, userID)
		}
	}

	for _, user := range staleUsers {
		if _, exists := userIDSet[user.UserID]; exists {
			continue
		}
		userIDSet[user.UserID] = struct{}{}
		refreshUserIDs = append(refreshUserIDs, user.UserID)
	}

	if len(refreshUserIDs) == 0 {
		g.Log().Debug(ctx, "[PerformanceService] 无需刷新用户")
		return nil
	}

	g.Log().Infof(ctx, "[PerformanceService] 开始增量刷新，受影响用户: %d, 过期用户: %d, 去重后总数: %d",
		affectedCount, len(staleUsers), len(refreshUserIDs))

	successCount := 0
	for _, userID := range refreshUserIDs {
		_, err := s.RefreshCache(ctx, userID)
		if err != nil {
			g.Log().Warningf(ctx, "[PerformanceService] 刷新用户 %d 缓存失败: %v", userID, err)
		} else {
			successCount++
		}
	}

	g.Log().Infof(ctx, "[PerformanceService] 增量刷新完成，成功: %d, 失败: %d, 总耗时: %v",
		successCount, len(refreshUserIDs)-successCount, time.Since(startTime))

	return nil
}

// GetTeamOverview 获取团队概览（用于 /api/v1/team/overview）
func (s *performanceService) GetTeamOverview(ctx context.Context, userID int64) (*TeamOverviewResult, error) {
	// 获取当前用户的缓存业绩
	selfPerf, err := s.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if selfPerf == nil {
		// 如果缓存不存在，返回空数据
		return &TeamOverviewResult{
			TeamTotalUserCount:    0,
			TeamTodayNewUserCount: 0,
			DirectCount:           0,
			RewardLevel:           2,
			BigTeamCount:          0,
			SmallTeamSum:          0,
			TeamPerformance:       "0.00",
			TeamYYSum:             "0.00",
		}, nil
	}

	// 获取团队统计（使用 teamstats service 的聚合查询）
	// 这里简化处理，实际可以复用 teamstats 的 GetOverviewAggByInviteCode
	bigTeamCount, smallTeamSum := s.calculateBigSmallTeam(ctx, selfPerf.InviteCode)

	return &TeamOverviewResult{
		TeamTotalUserCount:    selfPerf.TeamTotalCount,
		TeamTodayNewUserCount: 0, // 需要单独计算今日新增
		DirectCount:           selfPerf.DirectCount,
		RewardLevel:           2,
		BigTeamCount:          bigTeamCount,
		SmallTeamSum:          smallTeamSum,
		TeamPerformance:       selfPerf.TeamPerformance.StringFixed(2),
		TeamYYSum:             "0.00", // 需要单独查询 YY 余额
	}, nil
}

// NeedRefresh 检查是否需要刷新
func (s *performanceService) NeedRefresh(ctx context.Context) (bool, error) {
	latestID, err := s.repo.GetLatestPurchaseID(ctx)
	if err != nil {
		return false, err
	}

	count, err := s.repo.Count(ctx)
	if err != nil {
		return false, err
	}
	if count == 0 {
		return true, nil
	}

	maxCachedID, err := s.getMaxCachedPurchaseID(ctx)
	if err != nil {
		return false, err
	}
	if latestID > maxCachedID {
		return true, nil
	}

	staleUsers, err := s.repo.GetStaleUsers(ctx, 1)
	if err != nil {
		return false, err
	}

	return len(staleUsers) > 0, nil
}

func (s *performanceService) getMaxCachedPurchaseID(ctx context.Context) (int64, error) {
	var result struct {
		MaxPurchaseID int64 `json:"max_purchase_id"`
	}
	err := g.DB().Model("cobo_performance").Ctx(ctx).
		Fields("COALESCE(MAX(max_purchase_id), 0) as max_purchase_id").
		Scan(&result)
	if err != nil {
		return 0, err
	}
	return result.MaxPurchaseID, nil
}

// getDirectCount 获取直推人数
func (s *performanceService) getDirectCount(ctx context.Context, inviteCode string) (int, error) {
	count, err := g.DB().Model("user_info").Ctx(ctx).
		Where("parent_invite_code", inviteCode).
		Count()
	return count, err
}

// getTeamTotalCount 获取团队总人数
func (s *performanceService) getTeamTotalCount(ctx context.Context, inviteCode string) (int, error) {
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
		SELECT COUNT(*) as count FROM team_tree
	`

	var result struct {
		Count int `json:"count"`
	}
	err := g.DB().Ctx(ctx).Raw(sql, inviteCode).Scan(&result)
	if err != nil {
		return 0, err
	}
	return result.Count, nil
}

// getAllUserIDs 获取所有用户ID
func (s *performanceService) getAllUserIDs(ctx context.Context) ([]int64, error) {
	var results []struct {
		ID int64 `json:"id"`
	}
	err := g.DB().Model("user_info").Ctx(ctx).
		Fields("id").
		Where("status", 1).
		Scan(&results)
	if err != nil {
		return nil, err
	}

	userIDs := make([]int64, 0, len(results))
	for _, r := range results {
		userIDs = append(userIDs, r.ID)
	}
	return userIDs, nil
}

// calculateBigSmallTeam 计算大小区人数
func (s *performanceService) calculateBigSmallTeam(ctx context.Context, inviteCode string) (bigTeamCount, smallTeamSum int) {
	type result struct {
		BigTeamCount int `json:"big_team_count"`
		SmallTeamSum int `json:"small_team_sum"`
	}
	var r result
	err := g.DB().Ctx(ctx).Raw(`
		WITH RECURSIVE team_tree AS (
			SELECT u.id, u.invite_code, u.id AS root_id
			FROM user_info u
			WHERE u.parent_invite_code = ?
			UNION ALL
			SELECT c.id, c.invite_code, t.root_id
			FROM user_info c
			INNER JOIN team_tree t ON c.parent_invite_code = t.invite_code
		),
		branch_sizes AS (
			SELECT root_id, COUNT(*)::int AS cnt
			FROM team_tree
			GROUP BY root_id
		),
		branch_performance AS (
			SELECT t.root_id, COALESCE(SUM(np.amount), 0) AS perf
			FROM team_tree t
			LEFT JOIN cobo_node_purchase np ON np.user_id = t.id AND np.is_gift = 0
			GROUP BY t.root_id
		),
		big_branch AS (
			SELECT root_id
			FROM branch_performance
			ORDER BY perf DESC
			LIMIT 1
		)
		SELECT
			COALESCE((SELECT cnt FROM branch_sizes WHERE root_id = (SELECT root_id FROM big_branch)), 0)::int AS big_team_count,
			COALESCE(
				(SELECT SUM(cnt) FROM branch_sizes)
				- COALESCE((SELECT cnt FROM branch_sizes WHERE root_id = (SELECT root_id FROM big_branch)), 0),
				0
			)::int AS small_team_sum
	`, inviteCode).Scan(&r)
	if err != nil {
		g.Log().Warningf(ctx, "[PerformanceService] 计算大小区人数失败: %v", err)
		return 0, 0
	}
	return r.BigTeamCount, r.SmallTeamSum
}

// UpdateGiftPerformancePass 更新赠送节点的业绩及格标记
// 批量更新未达标赠送节点：团队业绩 >= 10倍赠送金额时标记为及格
func (s *performanceService) UpdateGiftPerformancePass(ctx context.Context) error {
	startTime := time.Now()
	g.Log().Info(ctx, "[PerformanceService] 开始更新赠送节点业绩及格标记")

	var totalPending int
	totalPending, err := g.DB().Model("cobo_node_purchase").Ctx(ctx).
		Where("is_gift = ?", 1).
		Where("gift_performance_pass = ?", false).
		Count()
	if err != nil {
		return fmt.Errorf("统计未达标赠送记录失败: %w", err)
	}

	if totalPending == 0 {
		g.Log().Debug(ctx, "[PerformanceService] 没有需要更新的赠送记录")
		return nil
	}

	var missingPerformanceResult struct {
		Cnt int `json:"cnt"`
	}
	err = g.DB().Ctx(ctx).Raw(`
		SELECT COUNT(1) AS cnt
		FROM cobo_node_purchase p
		LEFT JOIN cobo_performance perf ON perf.user_id = p.user_id
		WHERE p.is_gift = 1
		  AND p.gift_performance_pass = false
		  AND perf.user_id IS NULL
	`).Scan(&missingPerformanceResult)
	if err != nil {
		return fmt.Errorf("统计缺失业绩缓存记录失败: %w", err)
	}
	missingPerformance := missingPerformanceResult.Cnt

	result, err := g.DB().Exec(ctx, `
		UPDATE cobo_node_purchase p
		SET gift_performance_pass = true,
		    gift_team_perf_at_check = perf.team_performance,
		    updated_at = CURRENT_TIMESTAMP
		FROM cobo_performance perf
		WHERE p.user_id = perf.user_id
		  AND p.is_gift = 1
		  AND p.gift_performance_pass = false
		  AND perf.team_performance >= (p.amount * 10)
	`)
	if err != nil {
		return fmt.Errorf("批量更新赠送节点及格标记失败: %w", err)
	}

	updatedCount, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取赠送节点及格标记更新行数失败: %w", err)
	}

	failedCount := totalPending - int(updatedCount) - missingPerformance
	if failedCount < 0 {
		failedCount = 0
	}

	g.Log().Infof(ctx,
		"[PerformanceService] 赠送节点及格标记更新完成，扫描: %d, 更新: %d, 缺失业绩缓存: %d, 未达标: %d, 总耗时: %v",
		totalPending, updatedCount, missingPerformance, failedCount, time.Since(startTime))

	return nil
}
