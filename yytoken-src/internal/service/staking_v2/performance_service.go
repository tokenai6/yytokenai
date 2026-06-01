package staking_v2

import (
	"context"
	"fmt"
	"time"

	"XWFrame/internal/entity"
	"XWFrame/internal/repository"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// IPerformanceService staking_v2_performance 服务接口
type IPerformanceService interface {
	// GetByUserID 根据用户ID获取缓存业绩（如果不存在或过期则自动刷新）
	GetByUserID(ctx context.Context, userID int64) (*entity.StakingV2PerformanceEntity, error)

	// RefreshCache 刷新指定用户的缓存业绩
	RefreshCache(ctx context.Context, userID int64) (*entity.StakingV2PerformanceEntity, error)

	// RefreshAllCache 刷新所有缓存（全量刷新）
	RefreshAllCache(ctx context.Context) error

	// IncrementalUpdate 增量更新（只更新有变化的）
	IncrementalUpdate(ctx context.Context) error

	// NeedRefresh 检查是否需要刷新
	NeedRefresh(ctx context.Context) (bool, error)
}

// performanceService staking_v2_performance 服务实现
type performanceService struct {
	repo     repository.IStakingV2PerformanceRepository
	userRepo repository.IUserRepository
}

// NewPerformanceService 创建服务实例
func NewPerformanceService() IPerformanceService {
	return &performanceService{
		repo:     repository.NewStakingV2PerformanceRepository(),
		userRepo: repository.NewUserRepository(),
	}
}

// GetByUserID 根据用户ID获取缓存业绩（如果不存在或过期则自动刷新）
func (s *performanceService) GetByUserID(ctx context.Context, userID int64) (*entity.StakingV2PerformanceEntity, error) {
	// 先尝试从缓存获取
	e, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 如果不存在或过期，刷新缓存
	if e == nil || !e.IsFresh() {
		return s.RefreshCache(ctx, userID)
	}

	return e, nil
}

// RefreshCache 刷新指定用户的缓存业绩
func (s *performanceService) RefreshCache(ctx context.Context, userID int64) (*entity.StakingV2PerformanceEntity, error) {
	// 获取用户信息
	userInfo, err := s.userRepo.GetUserById(ctx, userID)
	if err != nil {
		return nil, err
	}
	if userInfo == nil {
		return nil, fmt.Errorf("用户不存在: %d", userID)
	}

	// 计算个人业绩
	personalPerf, orderCount, maxOrderID, err := s.repo.CalculatePersonalPerformance(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 计算团队业绩
	teamPerf, err := s.repo.CalculateTeamPerformance(ctx, userInfo.InviteCode)
	if err != nil {
		return nil, err
	}

	// 创建缓存实体
	e := &entity.StakingV2PerformanceEntity{
		UserID:              userID,
		WalletAddress:       userInfo.WalletAddress,
		InviteCode:          userInfo.InviteCode,
		PersonalPerformance: personalPerf,
		TeamPerformance:     teamPerf,
		OrderCount:          orderCount,
		MaxOrderID:          maxOrderID,
		UpdatedAt:           time.Now(),
		CreatedAt:           time.Now(),
	}

	// 保存到缓存表
	if err := s.repo.Upsert(ctx, e); err != nil {
		return nil, err
	}

	return e, nil
}

// RefreshAllCache 刷新所有缓存（全量刷新，自底向上避免重复计算）
func (s *performanceService) RefreshAllCache(ctx context.Context) error {
	startTime := time.Now()
	g.Log().Info(ctx, "[StakingV2PerformanceService] 开始全量刷新缓存（自底向上）")

	// 1. 获取所有用户关系（不限status，确保下级订单被完整统计）
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
		OrderCount          int             `json:"order_count"`
		MaxOrderID          int64           `json:"max_order_id"`
	}
	var personalPerfs []perfRow
	if err := g.DB().Ctx(ctx).Raw(`
		SELECT user_id,
			COALESCE(SUM(amount), 0) as personal_performance,
			COUNT(*) as order_count,
			COALESCE(MAX(id), 0) as max_order_id
		FROM staking_v2_order
		WHERE source_type = 1 AND is_gift = 0
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
		orderCount       int
		maxOrderID       int64
		children         []*perfNode
		teamPerf         decimal.Decimal
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
			n.orderCount = p.OrderCount
			n.maxOrderID = p.MaxOrderID
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
			g.Log().Warningf(ctx, "[StakingV2PerformanceService] 用户 %d 深度超过100，可能存在循环引用", n.userID)
			return
		}
		for _, child := range n.children {
			dfs(child, depth+1)
		}
		for _, child := range n.children {
			n.teamPerf = n.teamPerf.Add(child.personalPerf).Add(child.teamPerf)
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
		entities := make([]*entity.StakingV2PerformanceEntity, 0, end-i)
		for _, u := range users[i:end] {
			if u.Status != 1 {
				continue
			}
			n := nodeMap[u.InviteCode]
			entities = append(entities, &entity.StakingV2PerformanceEntity{
				UserID:              n.userID,
				WalletAddress:       n.walletAddress,
				InviteCode:          n.inviteCode,
				PersonalPerformance: n.personalPerf,
				TeamPerformance:     n.teamPerf,
				OrderCount:          n.orderCount,
				MaxOrderID:          n.maxOrderID,
				UpdatedAt:           time.Now(),
				CreatedAt:           time.Now(),
			})
		}
		if len(entities) == 0 {
			continue
		}
		if err := s.repo.BatchUpsert(ctx, entities); err != nil {
			g.Log().Warningf(ctx, "[StakingV2PerformanceService] 批量写入失败: %v", err)
			failCount += len(entities)
		} else {
			successCount += len(entities)
		}
		time.Sleep(50 * time.Millisecond)
	}

	g.Log().Infof(ctx, "[StakingV2PerformanceService] 全量刷新完成（自底向上），成功: %d, 失败: %d, 耗时: %v",
		successCount, failCount, time.Since(startTime))
	return nil
}

// IncrementalUpdate 增量更新（只更新有变化的）
func (s *performanceService) IncrementalUpdate(ctx context.Context) error {
	startTime := time.Now()

	// 获取最新的 order ID
	latestID, err := s.repo.GetLatestOrderID(ctx)
	if err != nil {
		return err
	}

	cacheCount, err := s.repo.Count(ctx)
	if err != nil {
		return err
	}

	if cacheCount == 0 {
		g.Log().Info(ctx, "[StakingV2PerformanceService] 缓存为空，执行全量刷新")
		return s.RefreshAllCache(ctx)
	}

	maxCachedID, err := s.getMaxCachedOrderID(ctx)
	if err != nil {
		return err
	}

	staleUsers, err := s.repo.GetStaleUsers(ctx, 1000)
	if err != nil {
		return err
	}

	// 如果没有新数据且没有过期用户，跳过
	if latestID <= maxCachedID && len(staleUsers) == 0 {
		g.Log().Debug(ctx, "[StakingV2PerformanceService] 没有新数据需要更新")
		return nil
	}

	refreshUserIDs := make([]int64, 0, len(staleUsers))
	userIDSet := make(map[int64]struct{})
	affectedCount := 0

	if latestID > maxCachedID {
		g.Log().Infof(ctx, "[StakingV2PerformanceService] 检测到新订单（latestID=%d, cachedID=%d），获取受影响用户链", latestID, maxCachedID)

		affectedUserIDs, err := s.repo.GetAffectedUsersByOrderRange(ctx, maxCachedID, latestID)
		if err != nil {
			g.Log().Warningf(ctx, "[StakingV2PerformanceService] 获取受影响用户失败: %v，回退到全量刷新", err)
			return s.RefreshAllCache(ctx)
		}

		affectedCount = len(affectedUserIDs)
		if affectedCount == 0 {
			g.Log().Warningf(ctx, "[StakingV2PerformanceService] 新订单范围内未匹配到受影响用户（latestID=%d, cachedID=%d）", latestID, maxCachedID)
			if len(staleUsers) == 0 {
				g.Log().Warning(ctx, "[StakingV2PerformanceService] 无过期用户可处理，回退到全量刷新避免空转")
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
		g.Log().Debug(ctx, "[StakingV2PerformanceService] 无需刷新用户")
		return nil
	}

	g.Log().Infof(ctx, "[StakingV2PerformanceService] 开始增量刷新，受影响用户: %d, 过期用户: %d, 去重后总数: %d",
		affectedCount, len(staleUsers), len(refreshUserIDs))

	successCount := 0
	for _, userID := range refreshUserIDs {
		_, err := s.RefreshCache(ctx, userID)
		if err != nil {
			g.Log().Warningf(ctx, "[StakingV2PerformanceService] 刷新用户 %d 缓存失败: %v", userID, err)
		} else {
			successCount++
		}
	}

	g.Log().Infof(ctx, "[StakingV2PerformanceService] 增量刷新完成，成功: %d, 失败: %d, 总耗时: %v",
		successCount, len(refreshUserIDs)-successCount, time.Since(startTime))

	return nil
}

// NeedRefresh 检查是否需要刷新
func (s *performanceService) NeedRefresh(ctx context.Context) (bool, error) {
	latestID, err := s.repo.GetLatestOrderID(ctx)
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

	maxCachedID, err := s.getMaxCachedOrderID(ctx)
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

func (s *performanceService) getMaxCachedOrderID(ctx context.Context) (int64, error) {
	var result struct {
		MaxOrderID int64 `json:"max_order_id"`
	}
	err := g.DB().Model("staking_v2_performance").Ctx(ctx).
		Fields("COALESCE(MAX(max_order_id), 0) as max_order_id").
		Scan(&result)
	if err != nil {
		return 0, err
	}
	return result.MaxOrderID, nil
}
