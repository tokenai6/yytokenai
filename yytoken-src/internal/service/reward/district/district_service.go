package district

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"XWFrame/internal/entity"
	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/consts"
	rewardRepo "XWFrame/internal/repository/reward"
	vipAdjustmentService "XWFrame/internal/service/reward/vip_adjustment"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// IDistrictService 小区奖励服务接口
type IDistrictService interface {
	// DistributeDistrictReward 发放小区奖励
	DistributeDistrictReward(ctx context.Context, date time.Time) error
}

// UserRewardCache 用户奖励缓存
type UserRewardCache struct {
	UserID       int64
	VipLevel     int
	RewardRate   decimal.Decimal
	RewardAmount decimal.Decimal // 实际获得的奖励金额
	Allocated    bool
}

// AllocationGroup 已分配分组
type AllocationGroup struct {
	RepresentativeUserID int64           // 代表用户ID
	StaticTotal          decimal.Decimal // 该组静态收益总和
	AllocatedRate        decimal.Decimal // 已分配比例
}

// districtService 小区奖励服务实现
type districtService struct {
	districtRepo rewardRepo.IRewardDistrictRepository
}

// NewDistrictService 创建小区奖励服务实例
func NewDistrictService() IDistrictService {
	return &districtService{
		districtRepo: rewardRepo.NewRewardDistrictRepository(),
	}
}

// DistributeDistrictReward 发放小区奖励（业务逻辑层）
func (s *districtService) DistributeDistrictReward(ctx context.Context, date time.Time) error {
	g.Log().Infof(ctx, "[小区奖励计算] 开始计算小区奖励: date=%s", date.Format(consts.TimeFormatDate))

	// 1. 获取基础数据
	allUsers, err := s.districtRepo.GetAllUsers(ctx)
	if err != nil {
		return gerror.Wrap(err, "获取所有用户失败")
	}

	allVIPs, err := s.districtRepo.GetAllCurrentVIPs(ctx)
	if err != nil {
		return gerror.Wrap(err, "获取所有VIP等级失败")
	}

	allStaticRecords, err := s.districtRepo.GetAllStaticRecords(ctx, date)
	if err != nil {
		return gerror.Wrap(err, "获取所有静态收益失败")
	}

	allPerformances, err := s.districtRepo.GetPerformanceByDate(ctx, date)
	if err != nil {
		return gerror.Wrap(err, "获取用户业绩失败")
	}

	// 2. 构建内存数据结构
	userMap, vipMap, staticRewardMap, childrenMap := s.buildDataStructures(allUsers, allVIPs, allStaticRecords)
	performanceMap := s.buildPerformanceMap(allPerformances)

	// 2.1 使用统一的VIP等级获取函数，覆盖调整后的VIP等级
	// 先保存原始计算的VIP等级映射（用于记录到metadata）
	calculatedVipLevelMap := make(map[int64]int)
	for userID, vip := range vipMap {
		if vip != nil {
			calculatedVipLevelMap[userID] = vip.VipLevel
		}
	}

	vipAdjustmentSvc := vipAdjustmentService.NewVipAdjustmentService()
	userIDToLevelMap := make(map[int64]int)
	for userID, vip := range vipMap {
		if vip != nil {
			userIDToLevelMap[userID] = vip.VipLevel
		}
	}
	effectiveVIPLevels, err := vipAdjustmentSvc.BatchGetEffectiveVIPLevels(ctx, userIDToLevelMap)
	if err != nil {
		return gerror.Wrap(err, "批量获取有效VIP等级失败")
	}
	// 更新 vipMap 中的VIP等级（用于计算奖励）
	for userID, effectiveLevel := range effectiveVIPLevels {
		if vip, exists := vipMap[userID]; exists && vip != nil {
			vip.VipLevel = effectiveLevel
		}
	}

	// 3. 构建缓存（性能优化）
	g.Log().Infof(ctx, "[小区奖励计算] 开始构建缓存...")
	childToParentMap := s.buildChildToParentMap(childrenMap)
	g.Log().Infof(ctx, "[小区奖励计算] 缓存构建完成: 用户数=%d", len(userMap))

	// 3.1 批量预取用户额度，避免循环内单查数据库
	allUserIDs := make([]int64, 0, len(userMap))
	for uid := range userMap {
		allUserIDs = append(allUserIDs, uid)
	}
	quotaMap, err := s.districtRepo.GetQuotasByUserIDs(ctx, allUserIDs)
	if err != nil {
		return gerror.Wrap(err, "批量获取用户额度失败")
	}

	// 4. 任务完成后清理缓存（使用 defer 确保即使发生错误也会清理）
	defer func() {
		// 清空所有 map 缓存
		clearMap(userMap)
		clearMap(vipMap)
		clearMap(staticRewardMap)
		clearMap(childrenMap)
		clearMap(performanceMap)
		clearMap(childToParentMap)
		clearMap(quotaMap)

		// 清空 slice
		allUsers = nil
		allVIPs = nil
		allStaticRecords = nil
		allPerformances = nil
		allUserIDs = nil

		g.Log().Debugf(ctx, "[小区奖励计算] 缓存已清理")
	}()

	// 5. 按层级自下而上计算小区奖励
	successCount, failedCount, err := s.distributeDistrictRewardByLevel(ctx, date, userMap, vipMap, staticRewardMap, childrenMap, performanceMap, childToParentMap, quotaMap, calculatedVipLevelMap)
	if err != nil {
		return err
	}

	g.Log().Infof(ctx, "[小区奖励计算] 计算完成: 成功=%d, 失败=%d", successCount, failedCount)
	return nil
}

// buildDataStructures 构建数据结构
func (s *districtService) buildDataStructures(
	allUsers []*entity.UserEntity,
	allVIPs []*rewardEntity.UserVipLevelEntity,
	allStaticRecords []*rewardEntity.AssetRecordEntity,
) (map[int64]*entity.UserEntity, map[int64]*rewardEntity.UserVipLevelEntity, map[int64]decimal.Decimal, map[int64][]*entity.UserEntity) {
	userMap := make(map[int64]*entity.UserEntity)
	vipMap := make(map[int64]*rewardEntity.UserVipLevelEntity)
	staticRewardMap := make(map[int64]decimal.Decimal)
	childrenMap := make(map[int64][]*entity.UserEntity)

	for _, user := range allUsers {
		userMap[user.Id] = user
	}

	for _, vip := range allVIPs {
		vipMap[vip.UserID] = vip
	}

	for _, record := range allStaticRecords {
		if record.UserID > 0 {
			staticRewardMap[record.UserID] = staticRewardMap[record.UserID].Add(record.Amount)
		}
	}

	// 构建邀请码到用户的映射
	inviteCodeToUserMap := make(map[string]*entity.UserEntity)
	for _, user := range allUsers {
		if user.InviteCode != "" {
			inviteCodeToUserMap[user.InviteCode] = user
		}
	}

	// 构建父子关系映射
	for _, user := range allUsers {
		if user.ParentInviteCode != "" {
			if parent, exists := inviteCodeToUserMap[user.ParentInviteCode]; exists {
				childrenMap[parent.Id] = append(childrenMap[parent.Id], user)
			}
		}
	}

	return userMap, vipMap, staticRewardMap, childrenMap
}

// buildPerformanceMap 构建业绩映射
func (s *districtService) buildPerformanceMap(allPerformances []*rewardEntity.UserPerformanceEntity) map[int64]*rewardEntity.UserPerformanceEntity {
	performanceMap := make(map[int64]*rewardEntity.UserPerformanceEntity)
	for _, perf := range allPerformances {
		performanceMap[perf.UserID] = perf
	}
	return performanceMap
}

// buildChildToParentMap 构建反向映射：子用户ID -> 父用户ID
func (s *districtService) buildChildToParentMap(childrenMap map[int64][]*entity.UserEntity) map[int64]int64 {
	childToParentMap := make(map[int64]int64)
	for parentID, children := range childrenMap {
		for _, child := range children {
			childToParentMap[child.Id] = parentID
		}
	}
	return childToParentMap
}

// distributeDistrictRewardByLevel 按层级自下而上计算小区奖励
func (s *districtService) distributeDistrictRewardByLevel(
	ctx context.Context,
	date time.Time,
	userMap map[int64]*entity.UserEntity,
	vipMap map[int64]*rewardEntity.UserVipLevelEntity,
	staticRewardMap map[int64]decimal.Decimal,
	childrenMap map[int64][]*entity.UserEntity,
	performanceMap map[int64]*rewardEntity.UserPerformanceEntity,
	childToParentMap map[int64]int64,
	quotaMap map[int64]*rewardEntity.UserQuotaEntity,
	calculatedVipLevelMap map[int64]int,
) (int, int, error) {
	// 1. 构建用户层级映射（自下而上）
	levelMap := s.buildUserLevelMap(userMap, childrenMap, childToParentMap)

	// 2. 初始化奖励缓存
	rewardCache := make(map[int64]*UserRewardCache)
	// 2.1 初始化后代缓存，避免重复递归
	descendantsCache := make(map[int64][]*entity.UserEntity)

	// 2.2 方法返回前清理内部缓存
	defer func() {
		clearMap(rewardCache)
		clearMap(descendantsCache)
		levelMap = nil
	}()

	// 3. 从最深层开始计算（自下而上）
	successCount := 0
	failedCount := 0

	for level := len(levelMap) - 1; level >= 0; level-- {
		users := levelMap[level]
		g.Log().Infof(ctx, "[小区奖励计算] 处理第%d层用户: %d个", level, len(users))

		for _, user := range users {
			userVIP, hasVIP := vipMap[user.Id]
			if !hasVIP || userVIP.VipLevel < 1 {
				continue // 跳过VIP0用户
			}

			// 检查个人业绩门槛（100u）
			perf, hasPerf := performanceMap[user.Id]
			if !hasPerf || perf.PersonalPerformance.LessThan(consts.GetMinPerformanceThreshold()) {
				continue
			}

			// 检查用户额度是否已出局（使用批量预取的额度缓存）
			quota := quotaMap[user.Id]
			if quota == nil || quota.RemainingQuota.LessThanOrEqual(decimal.Zero) {
				continue
			}

			// 计算并分配奖励（使用缓存）
			hasReward, rewardAmount, err := s.calculateAndDistributeWithCache(ctx, user.Id, date, userMap, vipMap, staticRewardMap, childrenMap, rewardCache, childToParentMap, descendantsCache, calculatedVipLevelMap)
			if err != nil {
				g.Log().Errorf(ctx, "[小区奖励计算] 计算失败: userID=%d, err=%v", user.Id, err)
				failedCount++
				continue
			}

			// 只有当用户实际获得了奖励时，才记录到rewardCache中
			// 叶子节点（没有下级）因为不会分走奖励，所以不应该进入rewardCache
			// 这样可以避免上级计算时误减奖励
			if hasReward {
				rewardCache[user.Id] = &UserRewardCache{
					UserID:       user.Id,
					VipLevel:     userVIP.VipLevel,
					RewardRate:   s.getVIPRate(userVIP.VipLevel),
					RewardAmount: rewardAmount,
					Allocated:    true,
				}
				successCount++
			}
		}
	}

	return successCount, failedCount, nil
}

// buildUserLevelMap 构建用户层级映射（自下而上）
func (s *districtService) buildUserLevelMap(
	userMap map[int64]*entity.UserEntity,
	childrenMap map[int64][]*entity.UserEntity,
	childToParentMap map[int64]int64,
) map[int][]*entity.UserEntity {
	levelMap := make(map[int][]*entity.UserEntity)
	visited := make(map[int64]bool)

	// 使用BFS计算每个用户的层级
	queue := make([]struct {
		user  *entity.UserEntity
		level int
	}, 0)

	// 找到所有根节点（没有父节点的用户）
	for _, user := range userMap {
		if _, ok := childToParentMap[user.Id]; !ok {
			queue = append(queue, struct {
				user  *entity.UserEntity
				level int
			}{user, 0})
		}
	}

	// BFS遍历
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current.user.Id] {
			continue
		}
		visited[current.user.Id] = true

		// 添加到对应层级
		levelMap[current.level] = append(levelMap[current.level], current.user)

		// 添加子节点到下一层
		children := childrenMap[current.user.Id]
		for _, child := range children {
			queue = append(queue, struct {
				user  *entity.UserEntity
				level int
			}{child, current.level + 1})
		}
	}

	return levelMap
}

// groupSubordinatesByAllocation 按已分配情况对下级进行分组
func (s *districtService) groupSubordinatesByAllocation(
	userID int64,
	staticRewardMap map[int64]decimal.Decimal,
	childrenMap map[int64][]*entity.UserEntity,
	rewardCache map[int64]*UserRewardCache,
	vipMap map[int64]*rewardEntity.UserVipLevelEntity,
	currentUserVipLevel int,
	childToParentMap map[int64]int64,
	descendantsCache map[int64][]*entity.UserEntity,
) []AllocationGroup {
	groups := make([]AllocationGroup, 0)
	groupMap := make(map[string]*AllocationGroup) // 用已分配比例作为key

	// 获取符合规则的下级用户：
	// 规则：若直接下级为VIP且等级不低于当前用户，则不纳入其间接下级（但保留该直接下级本身）。
	allSubordinates := s.getEligibleDescendants(userID, childrenMap, vipMap, currentUserVipLevel, descendantsCache)

	for _, subordinate := range allSubordinates {
		staticReward, hasStatic := staticRewardMap[subordinate.Id]
		if !hasStatic || staticReward.IsZero() {
			continue
		}

		// 计算该用户的已分配比例
		allocatedRate := s.calculateUserAllocatedRate(subordinate.Id, userID, rewardCache, childrenMap, childToParentMap)

		// 生成分组key
		groupKey := allocatedRate.String()

		// 查找或创建分组
		if group, exists := groupMap[groupKey]; exists {
			group.StaticTotal = group.StaticTotal.Add(staticReward)
		} else {
			groupMap[groupKey] = &AllocationGroup{
				RepresentativeUserID: subordinate.Id,
				StaticTotal:          staticReward,
				AllocatedRate:        allocatedRate,
			}
		}
	}

	// 转换为切片
	for _, group := range groupMap {
		groups = append(groups, *group)
	}

	return groups
}

// getEligibleDescendants 获取符合平级过滤规则的所有下级用户。
// 规则：
// - 永远包含所有直接下级；
// - 若某直接下级的VIP等级 >= 当前用户VIP等级，则不继续深入该分支；
// - 否则，递归加入其所有后代。
func (s *districtService) getEligibleDescendants(
	userID int64,
	childrenMap map[int64][]*entity.UserEntity,
	vipMap map[int64]*rewardEntity.UserVipLevelEntity,
	currentUserVipLevel int,
	descendantsCache map[int64][]*entity.UserEntity,
) []*entity.UserEntity {
	var result []*entity.UserEntity

	directChildren := childrenMap[userID]
	for _, child := range directChildren {
		// 始终包含直接下级
		result = append(result, child)

		// 若直接下级VIP等级 >= 当前用户，则不深入该分支
		if vip, ok := vipMap[child.Id]; ok && vip.VipLevel >= currentUserVipLevel {
			continue
		}

		// 否则递归收集该分支的所有后代（带缓存）
		descendants := s.getAllDescendants(child.Id, nil, childrenMap, descendantsCache)
		if len(descendants) > 0 {
			result = append(result, descendants...)
		}
	}

	return result
}

// calculateUserAllocatedRate 计算单个用户的已分配比例
// 返回从 userID（下级）到 currentUserID（上级）路径上的最大已分配比例
//
// 极差宗旨：
// 1. 当前节点伞下所有节点的静态收益都需要参与小区奖励计算
// 2. 如果下面有奖励被分走了，就需要减去
//
// 对于直接下级：已分配比例为0（排除它自己），上级可以从它的静态收益中获得奖励
// 对于间接下级：查找路径上的最大已分配比例，如果有奖励被分走了，需要减去
func (s *districtService) calculateUserAllocatedRate(
	userID int64,
	currentUserID int64,
	rewardCache map[int64]*UserRewardCache,
	childrenMap map[int64][]*entity.UserEntity,
	childToParentMap map[int64]int64,
) decimal.Decimal {
	maxAllocatedRate := decimal.Zero

	// 判断 userID 是否是 currentUserID 的直接下级
	isDirectChild := false
	directChildren := childrenMap[currentUserID]
	for _, child := range directChildren {
		if child.Id == userID {
			isDirectChild = true
			break
		}
	}

	// 使用预构建的从子到父的映射（用于向上查找路径）

	// 查找从 userID 到 currentUserID 路径上的最大已分配比例
	if isDirectChild {
		// 对于直接下级：
		// - 已分配比例为0（排除它自己），这样上级可以从它的静态收益中获得奖励
		// - 即使直接下级和上级是平级，上级也应该能从直接下级的静态收益中获得奖励
		// - 从 userID 的父节点开始查找（跳过 userID 本身）
		parentID, hasParent := childToParentMap[userID]
		if hasParent && parentID == currentUserID {
			// 如果父节点就是当前用户，说明路径上只有 userID -> currentUserID
			// 由于跳过了 userID 本身，已分配比例为0
			maxAllocatedRate = decimal.Zero
		} else if hasParent {
			// 如果还有其他中间节点，查找中间节点的已分配比例
			s.findMaxAllocatedRateUpward(userID, parentID, currentUserID, childToParentMap, rewardCache, &maxAllocatedRate)
		}
		// 如果没有父节点，已分配比例保持为0
	} else {
		// 对于间接下级：正常查找（包含从 userID 到 currentUserID 路径上的所有节点）
		// 这样可以找到路径上所有已分配奖励的最大比例，如果下面有奖励被分走了，就减去
		s.findMaxAllocatedRateUpward(userID, userID, currentUserID, childToParentMap, rewardCache, &maxAllocatedRate)
	}

	return maxAllocatedRate
}

// findMaxAllocatedRateUpward 向上查找路径上的最大已分配比例
// startUserID: 起始节点（用于判断是否排除该节点本身）
// currentUserID: 当前遍历到的节点
// targetUserID: 目标节点（上级用户）
func (s *districtService) findMaxAllocatedRateUpward(
	startUserID int64,
	currentUserID int64,
	targetUserID int64,
	childToParentMap map[int64]int64,
	rewardCache map[int64]*UserRewardCache,
	maxRate *decimal.Decimal,
) {
	if currentUserID == targetUserID {
		return // 到达目标用户，停止搜索
	}

	// 如果当前节点不是起始节点，才检查它的已分配比例
	// 这样可以排除直接下级本身的已分配比例（因为上级应该能从直接下级的静态收益中获得奖励）
	if currentUserID != startUserID {
		if cache, exists := rewardCache[currentUserID]; exists && cache.Allocated {
			if cache.RewardRate.GreaterThan(*maxRate) {
				*maxRate = cache.RewardRate
			}
		}
	}

	// 向上查找父节点
	parentID, hasParent := childToParentMap[currentUserID]
	if !hasParent {
		return // 没有父节点，停止搜索
	}

	// 递归向上查找
	s.findMaxAllocatedRateUpward(startUserID, parentID, targetUserID, childToParentMap, rewardCache, maxRate)
}

// calculateAndDistributeWithCache 使用缓存计算并创建小区奖励资金记录（极差算法）
// 返回值：hasReward 表示是否实际获得了奖励（totalReward > 0），rewardAmount 表示实际获得的奖励金额
// 注意：叶子节点（没有下级）不会分走奖励，因此不会进入rewardCache，不影响上级计算
func (s *districtService) calculateAndDistributeWithCache(
	ctx context.Context,
	userID int64,
	date time.Time,
	userMap map[int64]*entity.UserEntity,
	vipMap map[int64]*rewardEntity.UserVipLevelEntity,
	staticRewardMap map[int64]decimal.Decimal,
	childrenMap map[int64][]*entity.UserEntity,
	rewardCache map[int64]*UserRewardCache,
	childToParentMap map[int64]int64,
	descendantsCache map[int64][]*entity.UserEntity,
	calculatedVipLevelMap map[int64]int,
) (bool, decimal.Decimal, error) {
	userVIP := vipMap[userID]
	effectiveVipLevel := userVIP.VipLevel               // 用于计算奖励的有效VIP等级
	calculatedVipLevel := calculatedVipLevelMap[userID] // 业绩计算得到的VIP等级（原始）
	adjustedVipLevel := effectiveVipLevel               // 调整后生效的VIP等级（如果有调整，否则等于计算等级）

	userRate := s.getVIPRate(effectiveVipLevel)

	if userRate.IsZero() {
		return false, decimal.Zero, nil
	}

	// 获取所有下级用户（带缓存）
	allSubordinates := s.getAllDescendants(userID, userMap, childrenMap, descendantsCache)
	if len(allSubordinates) == 0 {
		return false, decimal.Zero, nil
	}

	totalReward := decimal.Zero
	subordinateStaticTotal := decimal.Zero
	details := make([]rewardEntity.DistrictBranchDetailAsset, 0)

	// 按分项计算奖励（实现需求推导的分项逻辑）
	// 1. 先计算所有下级的静态收益，按已分配情况分组
	allocatedGroups := s.groupSubordinatesByAllocation(userID, staticRewardMap, childrenMap, rewardCache, vipMap, userVIP.VipLevel, childToParentMap, descendantsCache)

	for _, group := range allocatedGroups {
		if group.StaticTotal.IsZero() {
			continue
		}

		subordinateStaticTotal = subordinateStaticTotal.Add(group.StaticTotal)

		// 计算该组的已分配比例
		allocatedRate := group.AllocatedRate

		// 计算当前用户可收取的比例（极差）
		availableRate := userRate.Sub(allocatedRate)
		if availableRate.GreaterThan(decimal.Zero) {
			reward := group.StaticTotal.Mul(availableRate)
			totalReward = totalReward.Add(reward)

			details = append(details, rewardEntity.DistrictBranchDetailAsset{
				BranchRootUserID:  group.RepresentativeUserID,
				BranchMaxVipLevel: s.getVipLevelByRate(allocatedRate),
				BranchMaxRate:     allocatedRate,
				BranchStaticTotal: group.StaticTotal,
				DiffRate:          availableRate,
				Reward:            reward,
			})
		}
	}

	if totalReward.IsZero() {
		return false, decimal.Zero, nil
	}

	// 幂等性检查
	exists, err := s.districtRepo.CheckExists(ctx, userID, date)
	if err != nil {
		return false, decimal.Zero, gerror.Wrap(err, "幂等验证失败")
	}
	if exists {
		g.Log().Infof(ctx, "[小区奖励计算] 记录已存在，跳过: userID=%d, date=%s", userID, date.Format(consts.TimeFormatDate))
		return false, decimal.Zero, nil
	}

	// 收集已分配奖励的用户信息
	allocatedUsers := s.collectAllocatedUsers(userID, rewardCache, staticRewardMap, childrenMap, descendantsCache, allSubordinates)

	// 构建元数据
	districtMetadata := rewardEntity.DistrictRewardMetadata{
		VipLevel:               effectiveVipLevel,  // 用于计算奖励的有效VIP等级
		CalculatedVipLevel:     calculatedVipLevel, // 业绩计算得到的VIP等级（原始）
		AdjustedVipLevel:       adjustedVipLevel,   // 调整后生效的VIP等级（如果有调整，否则等于计算等级）
		RewardRate:             userRate,
		DistrictPerformance:    userVIP.DistrictPerformance,
		SubordinateStaticTotal: subordinateStaticTotal,
		AllocatedUsers:         allocatedUsers,
		BranchDetails:          details,
	}
	metadataJSON, _ := json.Marshal(districtMetadata)

	// 创建资金记录
	record := &rewardEntity.AssetRecordEntity{
		UserID:       userID,
		AssetType:    consts.AssetTypeAPGUserBalance,
		RecordTime:   date,
		Amount:       totalReward,
		FlowType:     consts.AssetFlowTypeIncome,
		Status:       consts.AssetRecordStatusPending,
		BusinessType: consts.AssetBusinessTypeRewardDistrict,
		BusinessID:   0,
		Remark:       fmt.Sprintf("小区奖励-VIP%d", effectiveVipLevel),
		Metadata:     string(metadataJSON),
	}

	// 保存记录
	if err := s.districtRepo.CreateAssetRecord(ctx, record); err != nil {
		return false, decimal.Zero, gerror.Wrap(err, "创建资金记录失败")
	}

	// g.Log().Infof(ctx, "[小区奖励计算] 计算成功: userID=%d, VIP=%d, 奖励=%s（待结算）", userID, userVIP.VipLevel, totalReward.String())

	return true, totalReward, nil
}

// getAllDescendants 获取所有下级用户（递归）
func (s *districtService) getAllDescendants(userID int64, userMap map[int64]*entity.UserEntity, childrenMap map[int64][]*entity.UserEntity, descendantsCache map[int64][]*entity.UserEntity) []*entity.UserEntity {
	if descendantsCache != nil {
		if cached, ok := descendantsCache[userID]; ok {
			return cached
		}
	}

	var result []*entity.UserEntity
	children := childrenMap[userID]
	for _, child := range children {
		result = append(result, child)
		descendants := s.getAllDescendants(child.Id, userMap, childrenMap, descendantsCache)
		result = append(result, descendants...)
	}

	if descendantsCache != nil {
		descendantsCache[userID] = result
	}
	return result
}

// getVIPRate 获取VIP等级对应的奖励比例
func (s *districtService) getVIPRate(vipLevel int) decimal.Decimal {
	return consts.GetVIPRewardRate(vipLevel)
}

// getVipLevelByRate 根据奖励比例获取VIP等级
func (s *districtService) getVipLevelByRate(rate decimal.Decimal) int {
	for level := 0; level <= 9; level++ {
		if s.getVIPRate(level).Equal(rate) {
			return level
		}
	}
	return 0
}

// collectAllocatedUsers 收集已分配奖励的用户信息
func (s *districtService) collectAllocatedUsers(
	userID int64,
	rewardCache map[int64]*UserRewardCache,
	staticRewardMap map[int64]decimal.Decimal,
	childrenMap map[int64][]*entity.UserEntity,
	descendantsCache map[int64][]*entity.UserEntity,
	allSubordinates []*entity.UserEntity,
) []rewardEntity.DistrictAllocatedUserDetail {
	allocatedUsers := make([]rewardEntity.DistrictAllocatedUserDetail, 0)

	// 构建下级用户ID集合，用于快速判断
	subordinateSet := make(map[int64]bool)
	for _, subordinate := range allSubordinates {
		subordinateSet[subordinate.Id] = true
	}

	// 遍历 rewardCache，找出当前用户下级中已分配奖励的用户
	for allocatedUserID, cache := range rewardCache {
		if !cache.Allocated {
			continue
		}

		// 检查该用户是否是当前用户的下级
		if !subordinateSet[allocatedUserID] {
			continue
		}

		// 计算该用户伞下的静态收益总和
		subordinateStatic := s.calculateSubordinateStatic(allocatedUserID, staticRewardMap, childrenMap, descendantsCache)

		allocatedUsers = append(allocatedUsers, rewardEntity.DistrictAllocatedUserDetail{
			UserID:            allocatedUserID,
			VipLevel:          cache.VipLevel,
			AllocatedRate:     cache.RewardRate,
			AllocatedReward:   cache.RewardAmount,
			SubordinateStatic: subordinateStatic,
		})
	}

	return allocatedUsers
}

// calculateSubordinateStatic 计算用户伞下的静态收益总和
func (s *districtService) calculateSubordinateStatic(
	userID int64,
	staticRewardMap map[int64]decimal.Decimal,
	childrenMap map[int64][]*entity.UserEntity,
	descendantsCache map[int64][]*entity.UserEntity,
) decimal.Decimal {
	total := decimal.Zero

	// 获取该用户的所有下级
	descendants := s.getAllDescendants(userID, nil, childrenMap, descendantsCache)
	for _, descendant := range descendants {
		if staticReward, exists := staticRewardMap[descendant.Id]; exists {
			total = total.Add(staticReward)
		}
	}

	return total
}

// clearMap 清空 map（辅助函数，用于释放内存）
func clearMap[K comparable, V any](m map[K]V) {
	if m == nil {
		return
	}
	for k := range m {
		delete(m, k)
	}
}
