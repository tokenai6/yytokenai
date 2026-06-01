package performance

import (
	"context"
	"encoding/json"
	"time"

	"XWFrame/internal/entity"
	rewardEntity "XWFrame/internal/entity/reward"
	frameCache "XWFrame/internal/frame/cache"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/repository/reward"
	vipAdjustmentService "XWFrame/internal/service/reward/vip_adjustment"

	"github.com/gogf/gf/v2/container/gvar"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcache"
	"github.com/shopspring/decimal"
)

// IPerformanceService 业绩计算服务接口
type IPerformanceService interface {
	// CalculateDailyPerformance 计算每日业绩
	CalculateDailyPerformance(ctx context.Context, recordTime time.Time) error

	// CalculateVIPLevels 计算VIP等级
	CalculateVIPLevels(ctx context.Context, recordTime time.Time) error

	// CalculateDailyPerformanceAndVIP 一次性计算业绩并评定VIP（避免重复读取业绩）
	CalculateDailyPerformanceAndVIP(ctx context.Context, recordTime time.Time) error

	// RepairIncrementalPerformances 按算法修复指定时间的新增业绩字段
	RepairIncrementalPerformances(ctx context.Context, recordTime time.Time) error

	// GetRealtimeTeamAndDistrictPerformance 获取用户实时的团队业绩与小区业绩
	GetRealtimeTeamAndDistrictPerformance(ctx context.Context, userID int64) (*RealtimePerformanceRes, error)

	// GetRealtimeTeamAndDistrictPerformanceByStakeType 获取用户实时的团队业绩与小区业绩（按质押类型过滤）
	// 说明：用于类似  场景的独立口径统计，例如只统计 stake_type=12，且不按出局状态过滤。
	GetRealtimeTeamAndDistrictPerformanceByStakeType(ctx context.Context, userID int64, stakeType int) (*RealtimePerformanceRes, error)

	// RefreshRealtimeTeamAndDistrictCache 刷新全部用户的实时团队/小区业绩缓存
	RefreshRealtimeTeamAndDistrictCache(ctx context.Context) error

	// ComputeDailyPerformances 仅计算当日业绩记录（不落库），用于验证和测试
	ComputeDailyPerformances(ctx context.Context, date time.Time) ([]*rewardEntity.UserPerformanceEntity, int, error)
}

// performanceService 业绩计算服务实现
type performanceService struct {
	perfRepo         reward.IPerformanceRepository
	vipRepo          reward.IVipRepository
	vipAdjustmentSvc vipAdjustmentService.IVipAdjustmentService
	memoryCache      *gcache.Cache
	redisCache       *gcache.Cache
}

const realtimePerformanceCacheTTL = 10 * time.Minute

// performanceCalcContext 承载当日业绩计算所需的上下文数据
// 该结构体在计算开始前一次性构建，避免在计算过程中重复查询数据库
type performanceCalcContext struct {
	// users 所有激活用户列表（用于遍历生成业绩记录）
	users []*entity.UserEntity

	// childrenMap 父子关系映射：父用户ID -> 直接子用户列表
	// 用于计算直推人数、直推新增业绩、遍历网体结构
	childrenMap map[int64][]*entity.UserEntity

	// parentMap 子到父的映射：子用户ID -> 父用户ID
	// 用于从子节点向上回溯到父节点（后序遍历时使用）
	parentMap map[int64]int64

	// subtreeUserIDs 子树用户ID列表：用户ID -> 该用户所有下级用户ID列表（包括直接和间接下级）
	// 用于统计团队人数、快速判断用户是否在某个用户的团队中
	subtreeUserIDs map[int64][]int64

	// subtreeStakeSum 子树质押总和：用户ID -> 该用户及其所有下级的质押金额总和
	// 用于快速计算团队业绩：团队业绩 = subtreeStakeSum[userID] - userStakeAmountMap[userID]
	subtreeStakeSum map[int64]decimal.Decimal

	//子树组合质押总和：用户ID -> 该用户及其所有下级的组合质押金额总和
	subtreeStakeCombinationSum map[int64]decimal.Decimal

	// maxChildStakeSum 最大子区质押总和：用户ID -> 该用户所有直接下级中，子树质押最大的值
	// 用于计算小区业绩：小区业绩 = 所有子区总和 - 最大子区
	maxChildStakeSum map[int64]decimal.Decimal

	// userMap 用户映射：用户ID -> 用户实体（快速查找用户信息）
	userMap map[int64]*entity.UserEntity

	// userStakeAmountMap 用户质押金额映射：用户ID -> 该用户当前总质押金额
	// 个人业绩 = userStakeAmountMap[userID]
	userStakeAmountMap map[int64]decimal.Decimal
	//用户组合质押金额映射：用户ID -> 该用户当前总组合质押金额
	userStakeCombinationAmountMap map[int64]decimal.Decimal

	// prevPersonalMap 上一批次个人业绩映射：用户ID -> 上一批次该用户的个人业绩
	// 用于计算新增个人业绩：新增个人业绩 = 当前个人业绩 - 上一批次个人业绩
	prevPersonalMap map[int64]decimal.Decimal

	// existingPerfMap 已存在业绩记录映射：用户ID -> 是否已存在当日业绩记录
	// 用于幂等性检查，避免重复计算和插入
	existingPerfMap map[int64]bool

	// prevRecordTime 上一批次业绩记录时间（用于查询上一批次业绩数据）
	prevRecordTime time.Time

	// newStakeByStart 按开始时间统计的新增质押：用户ID -> 在[上一批次时间, 当前时间]区间内开始的新质押金额总和
	// 用于计算新增个人业绩（更精确的方式，基于质押开始时间）
	newStakeByStart map[int64]decimal.Decimal

	// newStakeByActualEnd 按实际结束时间统计的出局质押：用户ID -> 在[上一批次时间, 当前时间]区间内结束的质押金额总和
	// 用于计算新增出局质押（NewExpiredStake字段）
	newStakeByActualEnd map[int64]decimal.Decimal
}

// release 主动释放上下文中占用的引用，利于长任务内存回收
func (c *performanceCalcContext) release() {
	c.users = nil
	c.childrenMap = nil
	c.parentMap = nil
	c.subtreeUserIDs = nil
	c.subtreeStakeSum = nil
	c.subtreeStakeCombinationSum = nil
	c.maxChildStakeSum = nil
	c.userMap = nil
	c.userStakeAmountMap = nil
	c.userStakeCombinationAmountMap = nil
	c.prevPersonalMap = nil
	c.existingPerfMap = nil
	c.newStakeByStart = nil
	c.newStakeByActualEnd = nil
}

// performanceRecordsResult 暂存生成的业绩记录及新增个人业绩映射
// 用于在生成基础业绩记录后，暂存数据以便后续填充新增业绩字段
type performanceRecordsResult struct {
	// records 待创建的业绩记录列表（不含新增业绩字段，后续会填充）
	records []*rewardEntity.UserPerformanceEntity

	// performanceMap 业绩记录映射：用户ID -> 业绩记录实体（快速查找和更新）
	performanceMap map[int64]*rewardEntity.UserPerformanceEntity

	// newPersonalMap 新增个人业绩映射：用户ID -> 新增个人业绩金额
	// 用于后续计算新增直推、团队、小区业绩（这些字段依赖于新增个人业绩）
	newPersonalMap map[int64]decimal.Decimal

	// skipCount 跳过的记录数量（已存在业绩记录，无需重复计算）
	skipCount int
}

// release 主动释放生成结果中的临时映射
func (r *performanceRecordsResult) release() {
	r.records = nil
	r.performanceMap = nil
	r.newPersonalMap = nil
}

// NewPerformanceService 创建业绩计算服务实例
func NewPerformanceService() IPerformanceService {
	return &performanceService{
		perfRepo:         reward.NewPerformanceRepository(),
		vipRepo:          reward.NewVipRepository(),
		vipAdjustmentSvc: vipAdjustmentService.NewVipAdjustmentService(),
		memoryCache:      frameCache.GetMemoryCache(),
		redisCache:       frameCache.GetRedisCache(),
	}
}

// CalculateDailyPerformance 计算每日业绩（业务逻辑层）
func (s *performanceService) CalculateDailyPerformance(ctx context.Context, date time.Time) error {
	performancesToCreate, skipCount, err := s.computeDailyPerformances(ctx, date)
	if err != nil {
		return err
	}

	// 7. 批量保存业绩记录（调用Repository层）
	if len(performancesToCreate) > 0 {
		g.Log().Infof(ctx, "[业绩统计] 开始批量插入: 总数=%d", len(performancesToCreate))
		err := s.perfRepo.BatchCreatePerformance(ctx, performancesToCreate)
		if err != nil {
			g.Log().Errorf(ctx, "[业绩统计] 批量插入失败: err=%v", err)
			return gerror.Wrap(err, "批量插入业绩失败")
		}
	}

	successCount := len(performancesToCreate)
	g.Log().Infof(ctx, "[业绩统计] 计算完成: 成功=%d, 跳过=%d", successCount, skipCount)
	return nil
}

// GetRealtimeTeamAndDistrictPerformance 获取用户实时的团队业绩和小区业绩，用于前端实时展示
func (s *performanceService) GetRealtimeTeamAndDistrictPerformance(ctx context.Context, userID int64) (*RealtimePerformanceRes, error) {
	if cached, ok := s.getRealtimePerformanceCache(ctx, userID); ok {
		return cached, nil
	}

	calcCtx, err := s.buildPerformanceCalcContext(ctx, time.Now())
	if err != nil {
		return nil, err
	}
	defer calcCtx.release()

	if _, exists := calcCtx.userMap[userID]; !exists {
		return nil, gerror.Newf("用户不存在: %d", userID)
	}

	result := s.calculateRealtimePerformanceResult(userID, calcCtx)
	s.setRealtimePerformanceCache(ctx, userID, result)

	return result, nil
}

// GetRealtimeTeamAndDistrictPerformanceByStakeType 获取用户实时的团队业绩和小区业绩（按质押类型过滤）
// 场景： 团队业绩口径只统计 stake_type=12，且不按出局状态过滤。
func (s *performanceService) GetRealtimeTeamAndDistrictPerformanceByStakeType(ctx context.Context, userID int64, stakeType int) (*RealtimePerformanceRes, error) {
	calcCtx, err := s.buildPerformanceCalcContextByStakeType(ctx, time.Now(), stakeType)
	if err != nil {
		return nil, err
	}
	defer calcCtx.release()

	if _, exists := calcCtx.userMap[userID]; !exists {
		return nil, gerror.Newf("用户不存在: %d", userID)
	}

	return s.calculateRealtimePerformanceResult(userID, calcCtx), nil
}

// RefreshRealtimeTeamAndDistrictCache 刷新全部用户实时团队/小区业绩缓存
func (s *performanceService) RefreshRealtimeTeamAndDistrictCache(ctx context.Context) error {
	if s.memoryCache == nil {
		return gerror.New("内存缓存未初始化")
	}

	calcCtx, err := s.buildPerformanceCalcContext(ctx, time.Now())
	if err != nil {
		return err
	}
	defer calcCtx.release()

	users := calcCtx.users
	start := time.Now()
	for _, user := range users {
		result := s.calculateRealtimePerformanceResult(user.Id, calcCtx)
		s.setRealtimePerformanceCache(ctx, user.Id, result)
	}

	g.Log().Infof(ctx, "[业绩概览] 实时团队/小区业绩缓存刷新完成: 用户数=%d, 耗时=%v", len(users), time.Since(start))
	return nil
}

func (s *performanceService) calculateRealtimePerformanceResult(userID int64, calcCtx *performanceCalcContext) *RealtimePerformanceRes {
	teamPerformance := s.calculateTeamPerformance(userID, calcCtx)
	districtPerformance, maxDistrictPerformance := s.calculateDistrictPerformanceIterative(userID, calcCtx)
	teamMemberCount := len(calcCtx.subtreeUserIDs[userID])
	directReferralCount := len(calcCtx.childrenMap[userID])
	activeDirectMemberCount := s.countActiveDirectMembers(userID, calcCtx.childrenMap, calcCtx.userStakeAmountMap)

	return &RealtimePerformanceRes{
		TeamPerformance:         teamPerformance,
		DistrictPerformance:     districtPerformance,
		MaxDistrictPerformance:  maxDistrictPerformance,
		TeamMemberCount:         teamMemberCount,
		DirectReferralCount:     directReferralCount,
		ActiveDirectMemberCount: activeDirectMemberCount,
	}
}

func (s *performanceService) setRealtimePerformanceCache(ctx context.Context, userID int64, result *RealtimePerformanceRes) {
	if result == nil {
		return
	}

	cacheKey := frameCache.CacheKey{}.Performance().RealtimeTeamDistrict(userID)

	if s.memoryCache != nil {
		if err := s.memoryCache.Set(ctx, cacheKey, result, realtimePerformanceCacheTTL); err != nil {
			g.Log().Warningf(ctx, "[业绩概览] 内存缓存实时业绩失败: user_id=%d, err=%v", userID, err)
		}
	}

	if s.redisCache != nil {
		if err := s.redisCache.Set(ctx, cacheKey, result, realtimePerformanceCacheTTL); err != nil {
			g.Log().Warningf(ctx, "[业绩概览] Redis缓存实时业绩失败: user_id=%d, err=%v", userID, err)
		}
	}
}

func (s *performanceService) getRealtimePerformanceCache(ctx context.Context, userID int64) (*RealtimePerformanceRes, bool) {
	cacheKey := frameCache.CacheKey{}.Performance().RealtimeTeamDistrict(userID)

	if s.memoryCache != nil {
		if value, err := s.memoryCache.Get(ctx, cacheKey); err == nil && value != nil && !value.IsNil() {
			if cached, err := s.decodeRealtimePerformance(value); err == nil {
				return cached, true
			} else {
				g.Log().Warningf(ctx, "[业绩概览] 解析内存实时业绩缓存失败: user_id=%d, err=%v", userID, err)
			}
		}
	}

	if s.redisCache != nil {
		if value, err := s.redisCache.Get(ctx, cacheKey); err == nil && value != nil && !value.IsNil() {
			if cached, err := s.decodeRealtimePerformance(value); err == nil {
				if s.memoryCache != nil {
					if err := s.memoryCache.Set(ctx, cacheKey, cached, realtimePerformanceCacheTTL); err != nil {
						g.Log().Warningf(ctx, "[业绩概览] 回填内存实时业绩缓存失败: user_id=%d, err=%v", userID, err)
					}
				}
				return cached, true
			}
			g.Log().Warningf(ctx, "[业绩概览] 解析Redis实时业绩缓存失败: user_id=%d, err=%v", userID, err)
		}
	}

	return nil, false
}

func (s *performanceService) decodeRealtimePerformance(value *gvar.Var) (*RealtimePerformanceRes, error) {
	if value == nil || value.IsNil() {
		return nil, gerror.New("空的实时业绩缓存")
	}

	if cached, ok := value.Interface().(*RealtimePerformanceRes); ok && cached != nil {
		return cached, nil
	}

	var result RealtimePerformanceRes
	if err := value.Struct(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// CalculateDailyPerformanceAndVIP 一次性计算业绩并评定VIP
func (s *performanceService) CalculateDailyPerformanceAndVIP(ctx context.Context, date time.Time) error {
	performancesToCreate, skipCount, err := s.computeDailyPerformances(ctx, date)
	if err != nil {
		return err
	}

	if len(performancesToCreate) > 0 {
		g.Log().Infof(ctx, "[业绩统计] 开始批量插入: 总数=%d", len(performancesToCreate))
		if err := s.perfRepo.BatchCreatePerformance(ctx, performancesToCreate); err != nil {
			g.Log().Errorf(ctx, "[业绩统计] 批量插入失败: err=%v", err)
			return gerror.Wrap(err, "批量插入业绩失败")
		}
	}

	g.Log().Infof(ctx, "[业绩统计] 计算完成: 成功=%d, 跳过=%d，开始评定VIP...", len(performancesToCreate), skipCount)
	if err := s.calculateVIPLevelsFromPerformances(ctx, date, performancesToCreate); err != nil {
		return err
	}
	return nil
}

// RepairIncrementalPerformances 按算法修复指定时间的新增业绩字段
func (s *performanceService) RepairIncrementalPerformances(ctx context.Context, recordTime time.Time) error {
	g.Log().Infof(ctx, "[业绩修复] 开始修复新增业绩字段: record_time=%s", recordTime.Format(consts.TimeFormatDateTime))

	currentPerformances, err := s.perfRepo.GetPerformanceByDate(ctx, recordTime)
	if err != nil {
		return gerror.Wrap(err, "获取当前批次业绩失败")
	}
	if len(currentPerformances) == 0 {
		g.Log().Infof(ctx, "[业绩修复] 当前批次无业绩数据，跳过: record_time=%s", recordTime.Format(consts.TimeFormatDateTime))
		return nil
	}

	calcCtx, err := s.buildPerformanceCalcContext(ctx, recordTime)
	if err != nil {
		return err
	}
	defer calcCtx.release()

	directIncreaseMap, newTeamIncreaseMap, newDistrictIncreaseMap, _ := s.aggregateIncrementalPerformances(calcCtx, calcCtx.newStakeByStart)
	_, expiredTeamMap, _, _ := s.aggregateIncrementalPerformances(calcCtx, calcCtx.newStakeByActualEnd)

	updates := make([]*reward.PerformanceIncrementUpdate, 0, len(currentPerformances))

	for _, perf := range currentPerformances {
		newPersonal := calcCtx.newStakeByStart[perf.UserID]
		newDirect := directIncreaseMap[perf.UserID]
		newTeam := newTeamIncreaseMap[perf.UserID]
		newDistrict := newDistrictIncreaseMap[perf.UserID]
		newExpired := expiredTeamMap[perf.UserID]

		updates = append(updates, &reward.PerformanceIncrementUpdate{
			UserID:                 perf.UserID,
			NewPersonalPerformance: newPersonal,
			NewDirectPerformance:   newDirect,
			NewTeamPerformance:     newTeam,
			NewDistrictPerformance: newDistrict,
			NewExpiredStake:        newExpired,
		})
	}

	if err := s.perfRepo.BatchUpdateIncrementalFields(ctx, recordTime, updates); err != nil {
		return gerror.Wrap(err, "批量更新新增业绩失败")
	}

	g.Log().Infof(ctx, "[业绩修复] 修复完成: record_time=%s, 更新条数=%d", recordTime.Format(consts.TimeFormatDateTime), len(updates))
	return nil
}

// buildPerformanceCalcContext 构建当日业绩计算所需的全部数据缓存，减少后续重复查询
// 该函数一次性加载所有需要的数据到内存，避免在计算过程中重复查询数据库，大幅提升性能
// 步骤：
// 1. 加载所有激活用户和运行中的算力包
// 2. 构建网体结构（父子关系映射）
// 3. 预聚合用户质押金额
// 4. 使用拓扑排序算法计算子树聚合数据（子树用户列表、子树质押总和、最大子区质押）
// 5. 查询上一批次业绩数据（用于计算新增业绩）
// 6. 查询当日已存在业绩（幂等性检查）
// 7. 统计新增质押和出局质押（按时间区间）
func (s *performanceService) buildPerformanceCalcContext(ctx context.Context, date time.Time) (*performanceCalcContext, error) {
	// Step1: 加载所有激活用户
	allUsers, err := s.perfRepo.GetActiveUsers(ctx)
	if err != nil {
		return nil, gerror.Wrap(err, "获取所有用户失败")
	}

	// Step2: 拉取所有运行中的算力包，且开始时间小于等于统计时间，后续按用户聚合质押金额
	// 条件：status=1 且 start_time <= date（record_time）
	allPackages, err := s.perfRepo.GetActivePackagesByRecordTime(ctx, date)
	if err != nil {
		return nil, gerror.Wrap(err, "获取所有算力包失败")
	}

	// Step3: 初始化映射结构
	childrenMap := make(map[int64][]*entity.UserEntity) // 父 -> 子列表
	parentMap := make(map[int64]int64)                  // 子 -> 父ID
	childIDMap := make(map[int64][]int64)               // 父 -> 子ID列表（用于子树聚合算法）
	userMap := make(map[int64]*entity.UserEntity, len(allUsers))
	userStakeAmountMap := make(map[int64]decimal.Decimal, len(allUsers))
	userStakeCombinationAmountMap := make(map[int64]decimal.Decimal, len(allUsers))

	// Step4: 构建邀请码到用户的映射（用于快速查找父用户）
	inviteCodeToUserMap := make(map[string]*entity.UserEntity)
	for _, user := range allUsers {
		userMap[user.Id] = user
		userStakeAmountMap[user.Id] = decimal.Zero
		userStakeCombinationAmountMap[user.Id] = decimal.Zero // 初始化组合质押金额映射
		if user.InviteCode != "" {
			inviteCodeToUserMap[user.InviteCode] = user
		}
	}

	// Step5: 记录每个父子关系，构建当日的网体结构
	// 遍历所有用户，根据ParentInviteCode建立父子关系
	for _, user := range allUsers {
		if user.ParentInviteCode == "" {
			continue // 根用户（没有父用户）
		}
		if parent, exists := inviteCodeToUserMap[user.ParentInviteCode]; exists {
			childrenMap[parent.Id] = append(childrenMap[parent.Id], user)
			parentMap[user.Id] = parent.Id
			childIDMap[parent.Id] = append(childIDMap[parent.Id], user.Id)
		}
	}

	// Step6: 预聚合每个用户的总质押金额（个人业绩 = 所有运行中算力包的质押金额总和）
	combinationCount := 0
	combinationTotalAmount := decimal.Zero
	for _, pkg := range allPackages {
		userStakeAmountMap[pkg.UserID] = userStakeAmountMap[pkg.UserID].Add(pkg.StakeAmount)
		if pkg.StakeType == consts.StakeTypeCombinationStaking {
			//组合质押，累加组合质押金额
			userStakeCombinationAmountMap[pkg.UserID] = userStakeCombinationAmountMap[pkg.UserID].Add(pkg.StakeAmount)
			combinationCount++
			combinationTotalAmount = combinationTotalAmount.Add(pkg.StakeAmount)
		}
	}
	if combinationCount > 0 {
		g.Log().Infof(ctx, "[业绩统计] 发现组合质押包数量: %d, 总金额: %s", combinationCount, combinationTotalAmount.String())
		// 打印前10个有组合质押的用户
		count := 0
		for userID, amount := range userStakeCombinationAmountMap {
			if !amount.IsZero() && count < 10 {
				g.Log().Infof(ctx, "[业绩统计] 用户 %d 组合质押金额: %s", userID, amount.String())
				count++
			}
		}
	}

	// Step7: 使用拓扑排序算法计算子树聚合数据
	// 返回：子树用户ID列表、子树质押总和、最大子区质押总和
	subtreeUserIDs, subtreeStakeSum, subtreeStakeCombinationSum, maxChildStake := buildSubtreeAggregates(allUsers, childIDMap, parentMap, userStakeAmountMap, userStakeCombinationAmountMap)
	for userID, stakeCombination := range subtreeStakeCombinationSum {
		if !stakeCombination.IsZero() {
			g.Log().Infof(ctx, "[业绩统计] 用户 %d 子树组合质押金额: %s", userID, stakeCombination.String())
		}
	}

	// Step8: 查询上一批次业绩，用于计算新增个人业绩
	prevPersonalMap := make(map[int64]decimal.Decimal)
	prevRecordTime, err := s.perfRepo.GetLatestRecordTimeBefore(ctx, date)
	if err != nil {
		return nil, gerror.Wrap(err, "获取上一批次业绩时间失败")
	}
	if !prevRecordTime.IsZero() {
		prevPerformances, err := s.perfRepo.GetPerformanceByDate(ctx, prevRecordTime)
		if err != nil {
			return nil, gerror.Wrap(err, "获取上一批次业绩失败")
		}
		for _, perf := range prevPerformances {
			prevPersonalMap[perf.UserID] = perf.PersonalPerformance
		}
	}

	// Step9: 查询当日已存在的业绩记录，保证幂等（避免重复计算和插入）
	existingPerformances, err := s.perfRepo.GetPerformanceByDate(ctx, date)
	if err != nil {
		return nil, gerror.Wrap(err, "获取已有业绩失败")
	}
	existingPerfMap := make(map[int64]bool, len(existingPerformances))
	for _, perf := range existingPerformances {
		existingPerfMap[perf.UserID] = true
	}

	// Step10: 统计新增质押（按开始时间区间）
	// 用于计算新增个人业绩：统计在[上一批次时间, 当前时间]区间内开始的新质押金额
	newStakeByStart, err := s.perfRepo.GetStakeAmountByStartTimeRange(ctx, prevRecordTime, date)
	if err != nil {
		return nil, gerror.Wrap(err, "统计新增质押失败")
	}

	// Step11: 统计出局质押（按实际结束时间区间）
	// 用于计算NewExpiredStake字段：统计在[上一批次时间, 当前时间]区间内结束的质押金额
	newStakeByActualEnd, err := s.perfRepo.GetStakeAmountByActualEndTimeRange(ctx, prevRecordTime, date)
	if err != nil {
		return nil, gerror.Wrap(err, "统计出局质押失败")
	}

	return &performanceCalcContext{
		users:                         allUsers,
		childrenMap:                   childrenMap,
		parentMap:                     parentMap,
		subtreeUserIDs:                subtreeUserIDs,
		subtreeStakeSum:               subtreeStakeSum,
		subtreeStakeCombinationSum:    subtreeStakeCombinationSum,
		maxChildStakeSum:              maxChildStake,
		userMap:                       userMap,
		userStakeAmountMap:            userStakeAmountMap,
		userStakeCombinationAmountMap: userStakeCombinationAmountMap,
		prevPersonalMap:               prevPersonalMap,
		existingPerfMap:               existingPerfMap,
		prevRecordTime:                prevRecordTime,
		newStakeByStart:               newStakeByStart,
		newStakeByActualEnd:           newStakeByActualEnd,
	}, nil
}

// buildPerformanceCalcContextByStakeType 构建按质押类型过滤的业绩计算上下文（主要用于实时展示/独立口径）
// 规则：
// - 仅统计 staking_package.stake_type = stakeType
// - 不按 status 过滤（别管有没有出局）
// - start_time <= date
func (s *performanceService) buildPerformanceCalcContextByStakeType(ctx context.Context, date time.Time, stakeType int) (*performanceCalcContext, error) {
	// Step1: 加载所有激活用户
	allUsers, err := s.perfRepo.GetActiveUsers(ctx)
	if err != nil {
		return nil, gerror.Wrap(err, "获取所有用户失败")
	}

	// Step2: 拉取指定 stake_type 的算力包（不按状态过滤），且开始时间小于等于统计时间
	allPackages, err := s.perfRepo.GetPackagesByRecordTimeAndStakeType(ctx, date, stakeType)
	if err != nil {
		return nil, gerror.Wrap(err, "获取算力包失败")
	}

	// Step3: 初始化映射结构
	childrenMap := make(map[int64][]*entity.UserEntity)
	parentMap := make(map[int64]int64)
	childIDMap := make(map[int64][]int64)
	userMap := make(map[int64]*entity.UserEntity, len(allUsers))
	userStakeAmountMap := make(map[int64]decimal.Decimal, len(allUsers))
	userStakeCombinationAmountMap := make(map[int64]decimal.Decimal, len(allUsers))

	// Step4: 构建邀请码到用户的映射（用于快速查找父用户）
	inviteCodeToUserMap := make(map[string]*entity.UserEntity)
	for _, user := range allUsers {
		if user == nil {
			continue
		}
		userMap[user.Id] = user
		userStakeAmountMap[user.Id] = decimal.Zero
		userStakeCombinationAmountMap[user.Id] = decimal.Zero
		if user.InviteCode != "" {
			inviteCodeToUserMap[user.InviteCode] = user
		}
	}

	// Step5: 构建网体结构
	for _, user := range allUsers {
		if user == nil || user.ParentInviteCode == "" {
			continue
		}
		if parent, exists := inviteCodeToUserMap[user.ParentInviteCode]; exists && parent != nil {
			childrenMap[parent.Id] = append(childrenMap[parent.Id], user)
			parentMap[user.Id] = parent.Id
			childIDMap[parent.Id] = append(childIDMap[parent.Id], user.Id)
		}
	}

	// Step6: 预聚合每个用户的总质押金额（个人业绩 = 指定 stake_type 的质押金额总和）
	for _, pkg := range allPackages {
		if pkg == nil {
			continue
		}
		userStakeAmountMap[pkg.UserID] = userStakeAmountMap[pkg.UserID].Add(pkg.StakeAmount)
		// 组合质押统计在该独立口径里不需要，保持为0
	}

	// Step7: 子树聚合（团队/小区）
	subtreeUserIDs, subtreeStakeSum, subtreeStakeCombinationSum, maxChildStake := buildSubtreeAggregates(allUsers, childIDMap, parentMap, userStakeAmountMap, userStakeCombinationAmountMap)

	return &performanceCalcContext{
		users:                         allUsers,
		childrenMap:                   childrenMap,
		parentMap:                     parentMap,
		subtreeUserIDs:                subtreeUserIDs,
		subtreeStakeSum:               subtreeStakeSum,
		subtreeStakeCombinationSum:    subtreeStakeCombinationSum,
		maxChildStakeSum:              maxChildStake,
		userMap:                       userMap,
		userStakeAmountMap:            userStakeAmountMap,
		userStakeCombinationAmountMap: userStakeCombinationAmountMap,
		// 以下字段在实时计算中不会用到，保持空值/零值即可
		prevPersonalMap:     map[int64]decimal.Decimal{},
		existingPerfMap:     map[int64]bool{},
		prevRecordTime:      time.Time{},
		newStakeByStart:     map[int64]decimal.Decimal{},
		newStakeByActualEnd: map[int64]decimal.Decimal{},
	}, nil
}

// buildSubtreeAggregates 使用拓扑排序算法（自底向上）计算子树聚合数据
// 算法说明：
// 1. 使用拓扑排序，从叶子节点（没有子节点的用户）开始处理
// 2. 自底向上聚合：先计算子节点的子树数据，再计算父节点的子树数据
// 3. 对于每个节点，计算：
//   - subtreeIDs: 该节点所有下级用户ID列表（包括直接和间接下级）
//   - subtreeStake: 该节点及其所有下级的质押金额总和
//   - maxChildStake: 该节点所有直接下级中，子树质押最大的值（用于计算小区业绩）
//
// 时间复杂度：O(n)，其中n为用户数量（每个节点只处理一次）
// 空间复杂度：O(n)
func buildSubtreeAggregates(users []*entity.UserEntity, childIDMap map[int64][]int64, parentMap map[int64]int64, userStakeAmountMap map[int64]decimal.Decimal, userStakeCombinationAmountMap map[int64]decimal.Decimal) (map[int64][]int64, map[int64]decimal.Decimal, map[int64]decimal.Decimal, map[int64]decimal.Decimal) {
	// 初始化结果映射
	subtreeIDs := make(map[int64][]int64, len(users))                      // 用户ID -> 该用户所有下级用户ID列表
	subtreeStake := make(map[int64]decimal.Decimal, len(users))            // 用户ID -> 该用户及其所有下级的质押总和
	subtreeStakeCombination := make(map[int64]decimal.Decimal, len(users)) // 用户ID -> 该用户及其所有下级的组合质押总和
	maxChildStake := make(map[int64]decimal.Decimal, len(users))           // 用户ID -> 最大子区质押总和

	// 初始化所有用户的映射（确保所有用户都有对应的键，即使没有父子关系）
	for _, user := range users {
		subtreeIDs[user.Id] = []int64{}
		subtreeStake[user.Id] = decimal.Zero
		subtreeStakeCombination[user.Id] = decimal.Zero
		maxChildStake[user.Id] = decimal.Zero
	}

	// 拓扑排序所需的数据结构
	pendingChildren := make(map[int64]int, len(users)) // 用户ID -> 待处理的子节点数量
	queue := make([]int64, 0)                          // 待处理队列（叶子节点优先）

	// Step1: 初始化每个节点的待处理子节点数量，并将叶子节点（没有子节点的用户）加入队列
	for _, user := range users {
		children := childIDMap[user.Id]
		pendingChildren[user.Id] = len(children)
		if len(children) == 0 {
			// 叶子节点：没有子节点，可以直接处理
			queue = append(queue, user.Id)
		}
	}

	// Step2: 拓扑排序处理：从叶子节点开始，自底向上处理
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		children := childIDMap[current]

		if len(children) == 0 {
			// 叶子节点：没有子节点
			subtreeIDs[current] = []int64{}                                           // 没有下级用户
			subtreeStake[current] = userStakeAmountMap[current]                       // 子树质押 = 自己的质押
			subtreeStakeCombination[current] = userStakeCombinationAmountMap[current] // 子树组合质押 = 自己的组合质押
			maxChildStake[current] = decimal.Zero                                     // 没有子节点，最大子区为0
		} else {
			// 非叶子节点：需要聚合所有子节点的数据
			// 预计算总长度（优化内存分配）
			totalLen := len(children)
			for _, childID := range children {
				totalLen += len(subtreeIDs[childID])
			}

			// 初始化聚合变量
			descendants := make([]int64, 0, totalLen)                       // 所有下级用户ID列表
			totalStake := userStakeAmountMap[current]                       // 子树质押总和 = 自己的质押 + 所有子节点的子树质押
			totalStakeCombination := userStakeCombinationAmountMap[current] // 子树组合质押总和 = 自己的组合质押 + 所有子节点的子树组合质押
			maxStake := decimal.Zero                                        // 最大子区质押

			// 聚合所有子节点的数据
			for _, childID := range children {
				// 添加直接下级用户ID
				descendants = append(descendants, childID)
				// 添加间接下级用户ID（递归包含）
				if childDesc := subtreeIDs[childID]; len(childDesc) > 0 {
					descendants = append(descendants, childDesc...)
				}
				// 累加子节点的子树质押
				childStake := subtreeStake[childID]
				totalStake = totalStake.Add(childStake)
				totalStakeCombination = totalStakeCombination.Add(subtreeStakeCombination[childID])
				// 更新最大子区质押
				if childStake.GreaterThan(maxStake) {
					maxStake = childStake
				}
			}

			// 保存计算结果
			subtreeIDs[current] = descendants
			subtreeStake[current] = totalStake
			subtreeStakeCombination[current] = totalStakeCombination
			maxChildStake[current] = maxStake
		}

		// Step3: 将当前节点标记为已处理，检查父节点是否可以处理
		// 如果父节点的所有子节点都已处理完成，则将父节点加入队列
		if parentID, ok := parentMap[current]; ok {
			pendingChildren[parentID]--
			if pendingChildren[parentID] == 0 {
				// 父节点的所有子节点都已处理完成，可以处理父节点了
				queue = append(queue, parentID)
			}
		}
	}

	return subtreeIDs, subtreeStake, subtreeStakeCombination, maxChildStake
}

// generatePerformanceRecords 基于上下文生成当日业绩记录（不含新增字段），并统计跳过数量
// 该函数生成基础业绩记录，包括：
// - 个人业绩、团队业绩、小区业绩（基于当前网体结构和质押数据）
// - 直推人数、团队人数、有效人数
// - 新增个人业绩（基于时间区间统计）
// - VIP等级（基于小区业绩）
// 注意：新增直推、团队、小区业绩字段在此阶段暂设为0，后续通过fillIncrementalPerformances填充
func (s *performanceService) generatePerformanceRecords(date time.Time, calcCtx *performanceCalcContext) (*performanceRecordsResult, error) {
	result := &performanceRecordsResult{
		records:        make([]*rewardEntity.UserPerformanceEntity, 0, len(calcCtx.users)),
		performanceMap: make(map[int64]*rewardEntity.UserPerformanceEntity, len(calcCtx.users)),
		newPersonalMap: make(map[int64]decimal.Decimal, len(calcCtx.users)),
	}

	// 遍历所有用户，为每个用户生成业绩记录
	for _, user := range calcCtx.users {
		// 幂等性检查：已存在记录则跳过，避免重复计算和插入
		if calcCtx.existingPerfMap[user.Id] {
			result.skipCount++
			continue
		}

		// 个人业绩 = 当日总质押金额（所有运行中算力包的质押金额总和）
		totalStakeAmount := calcCtx.userStakeAmountMap[user.Id]
		personalPerformance := totalStakeAmount

		//个人组合业绩 = 当日总组合质押金额（所有运行中组合算力包的质押金额总和）
		totalCombinationStakeAmount := calcCtx.userStakeCombinationAmountMap[user.Id]
		personalCombinationPerformance := totalCombinationStakeAmount
		if !personalCombinationPerformance.IsZero() {
			g.Log().Infof(context.Background(), "[业绩统计] 用户 %d 个人组合业绩: %s", user.Id, personalCombinationPerformance.String())
		}

		// 团队业绩：除去自己，下面网体的所有个人业绩之和
		// 计算公式：团队业绩 = subtreeStakeSum[userID] - userStakeAmountMap[userID]
		teamPerformance := s.calculateTeamPerformance(user.Id, calcCtx)

		//团队组合业绩：除去自己，下面网体的所有组合质押金额之和
		teamCombinationPerformance := s.calculateTeamCombinationPerformance(user.Id, calcCtx)
		if !teamCombinationPerformance.IsZero() {
			g.Log().Infof(context.Background(), "[业绩统计] 用户 %d 团队组合业绩: %s", user.Id, teamCombinationPerformance.String())
		}

		// 小区业绩：每个直接下级形成一个区，小区业绩 = 所有区的团队业绩总和 - 最大区团队业绩
		// 同时返回最大区团队业绩（用于VIP等级计算）
		districtPerformance, maxDistrictPerformance := s.calculateDistrictPerformanceIterative(user.Id, calcCtx)

		// 新增个人业绩：统计在[上一批次时间, 当前时间]区间内开始的新质押金额
		// 这是基于质押开始时间的精确统计，比"当前个人业绩 - 上一批次个人业绩"更准确
		newPersonalPerformance := calcCtx.newStakeByStart[user.Id]

		// 直推人数：直接下级用户数量
		directReferralCount := len(calcCtx.childrenMap[user.Id])

		// 团队人数：所有下级用户数量（包括直接和间接下级）
		teamMemberCount := len(calcCtx.subtreeUserIDs[user.Id])

		// 有效人数：直推用户中，个人业绩≥阈值的用户数量（用于VIP评估）
		activeMemberCount := s.countActiveDirectMembers(user.Id, calcCtx.childrenMap, calcCtx.userStakeAmountMap)

		// VIP等级：基于小区业绩评定（需要个人业绩≥阈值才能评定VIP）
		vipLevel := 0
		if personalPerformance.GreaterThanOrEqual(consts.GetMinPerformanceThreshold()) {
			vipLevel = s.getVIPLevel(districtPerformance)
		}

		// 创建业绩记录实体
		performance := &rewardEntity.UserPerformanceEntity{
			UserID:                         user.Id,
			RecordTime:                     date,
			PersonalPerformance:            personalPerformance,
			PersonalCombinationPerformance: personalCombinationPerformance,
			TeamPerformance:                teamPerformance,
			TeamCombinationPerformance:     teamCombinationPerformance,
			DistrictPerformance:            districtPerformance,
			MaxDistrictPerformance:         maxDistrictPerformance,
			DirectReferralCount:            directReferralCount,
			TeamMemberCount:                teamMemberCount,
			ActiveMemberCount:              activeMemberCount,
			NewPersonalPerformance:         newPersonalPerformance,
			// 以下新增业绩字段暂设为0，后续通过fillIncrementalPerformances填充
			NewDirectPerformance:   decimal.Zero,
			NewTeamPerformance:     decimal.Zero,
			NewDistrictPerformance: decimal.Zero,
			NewExpiredStake:        calcCtx.newStakeByActualEnd[user.Id], // 出局质押已计算
			TotalPowerValue:        totalStakeAmount,
			VipLevel:               vipLevel,
			TeamTotalCount:         teamMemberCount,
		}

		result.records = append(result.records, performance)
		result.performanceMap[user.Id] = performance
		result.newPersonalMap[user.Id] = newPersonalPerformance
	}

	return result, nil
}

// fillIncrementalPerformances 使用单次后序遍历回填直推、团队、小区新增业绩
// 该函数在生成基础业绩记录后调用，填充新增业绩字段：
// - NewDirectPerformance: 新增直推业绩（所有直接下级的新增个人业绩总和）
// - NewTeamPerformance: 新增团队业绩（所有下级的新增个人业绩总和，不包括自己）
// - NewDistrictPerformance: 新增小区业绩（所有子区的新增团队业绩总和 - 最大子区的新增团队业绩）
// - NewExpiredStake: 新增出局质押（所有下级的新增出局质押总和）
//
// 算法说明：
// 1. 使用后序遍历（自底向上）聚合新增业绩
// 2. 先计算子节点的新增业绩，再计算父节点的新增业绩
// 3. 这样确保父节点计算时，所有子节点的新增业绩已经计算完成
func (s *performanceService) fillIncrementalPerformances(calcCtx *performanceCalcContext, result *performanceRecordsResult) {
	if len(result.records) == 0 {
		return
	}

	// Step1: 聚合新增业绩（基于新增个人业绩）
	// 返回：新增直推业绩、新增团队业绩、新增小区业绩
	directIncreaseMap, newTeamIncreaseMap, newDistrictIncreaseMap, _ := s.aggregateIncrementalPerformances(calcCtx, result.newPersonalMap)

	// Step2: 聚合新增出局质押（基于出局质押数据）
	// 返回：新增出局质押（团队维度）
	_, expiredTeamMap, _, _ := s.aggregateIncrementalPerformances(calcCtx, calcCtx.newStakeByActualEnd)

	// Step3: 将聚合结果填充到业绩记录中
	for userID, perf := range result.performanceMap {
		perf.NewDirectPerformance = directIncreaseMap[userID]
		perf.NewTeamPerformance = newTeamIncreaseMap[userID]
		perf.NewDistrictPerformance = newDistrictIncreaseMap[userID]
		perf.NewExpiredStake = expiredTeamMap[userID]
	}
}

// aggregateIncrementalPerformances 使用后序遍历算法聚合新增业绩
// 该函数计算新增直推、团队、小区业绩，使用拓扑排序确保自底向上计算
//
// 参数说明：
// - calcCtx: 业绩计算上下文（包含网体结构、用户数据等）
// - newPersonalMap: 新增个人业绩映射（用户ID -> 新增个人业绩金额）
//
// 返回值说明：
// - directIncreaseMap: 新增直推业绩映射（用户ID -> 所有直接下级的新增个人业绩总和）
// - newTeamIncreaseMap: 新增团队业绩映射（用户ID -> 所有下级的新增个人业绩总和，不包括自己）
// - newDistrictIncreaseMap: 新增小区业绩映射（用户ID -> 所有子区的新增团队业绩总和 - 最大子区的新增团队业绩）
// - subtreeIncreaseMap: 子树新增业绩映射（用户ID -> 该用户及其所有下级的新增个人业绩总和）
//
// 算法流程：
// 1. 计算新增直推业绩（直接遍历父子关系即可）
// 2. 使用拓扑排序生成后序遍历序列（从叶子节点到根节点）
// 3. 按后序遍历顺序计算新增团队业绩和新增小区业绩
func (s *performanceService) aggregateIncrementalPerformances(
	calcCtx *performanceCalcContext,
	newPersonalMap map[int64]decimal.Decimal,
) (map[int64]decimal.Decimal, map[int64]decimal.Decimal, map[int64]decimal.Decimal, map[int64]decimal.Decimal) {
	// Step1: 计算新增直推业绩（所有直接下级的新增个人业绩总和）
	// 注意：直推业绩只需要直接遍历父子关系，不需要后序遍历
	directIncreaseMap := make(map[int64]decimal.Decimal, len(calcCtx.childrenMap))
	for parentID, children := range calcCtx.childrenMap {
		total := decimal.Zero
		for _, child := range children {
			total = total.Add(newPersonalMap[child.Id])
		}
		directIncreaseMap[parentID] = total
	}

	// Step2: 使用拓扑排序生成后序遍历序列（从叶子节点到根节点）
	// 后序遍历确保：处理父节点时，所有子节点都已处理完成
	pendingChildren := make(map[int64]int, len(calcCtx.users)) // 用户ID -> 待处理的子节点数量
	queue := make([]int64, 0)                                  // 待处理队列（叶子节点优先）

	// 初始化：统计每个节点的子节点数量，将叶子节点加入队列
	for _, user := range calcCtx.users {
		count := len(calcCtx.childrenMap[user.Id])
		pendingChildren[user.Id] = count
		if count == 0 {
			// 叶子节点：没有子节点，可以直接处理
			queue = append(queue, user.Id)
		}
	}

	// 拓扑排序：生成后序遍历序列
	postOrder := make([]int64, 0, len(calcCtx.users))
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		postOrder = append(postOrder, current)

		// 将当前节点标记为已处理，检查父节点是否可以处理
		if parentID, ok := calcCtx.parentMap[current]; ok {
			pendingChildren[parentID]--
			if pendingChildren[parentID] == 0 {
				// 父节点的所有子节点都已处理完成，可以处理父节点了
				queue = append(queue, parentID)
			}
		}
	}

	// Step3: 按后序遍历顺序计算新增团队业绩和新增小区业绩
	subtreeIncreaseMap := make(map[int64]decimal.Decimal, len(calcCtx.users))     // 子树新增业绩（包括自己）
	newTeamIncreaseMap := make(map[int64]decimal.Decimal, len(calcCtx.users))     // 新增团队业绩（不包括自己）
	newDistrictIncreaseMap := make(map[int64]decimal.Decimal, len(calcCtx.users)) // 新增小区业绩

	// 按后序遍历顺序处理每个节点（确保子节点先于父节点处理）
	for _, userID := range postOrder {
		children := calcCtx.childrenMap[userID]

		// 初始化聚合变量
		childSubtreeTotal := decimal.Zero // 所有子节点的子树新增业绩总和
		maxDistrict := decimal.Zero       // 最大子区的新增团队业绩
		totalDistrict := decimal.Zero     // 所有子区的新增团队业绩总和

		// 获取最大区的累计团队业绩（参考 calculateDistrictPerformanceIterative 的算法）
		// 这个值已经在 buildSubtreeAggregates 中预先计算好了
		maxDistrictCumulativeStake := calcCtx.maxChildStakeSum[userID]

		// 聚合所有子节点的数据
		for _, child := range children {
			// 获取子节点的子树新增业绩（包括子节点自己及其所有下级）
			childSubtree := subtreeIncreaseMap[child.Id]
			childSubtreeTotal = childSubtreeTotal.Add(childSubtree)

			// 累计所有子区的新增团队业绩（用于计算小区业绩）
			totalDistrict = totalDistrict.Add(childSubtree)

			// 找到累计团队业绩等于最大区的那个子节点，记录它对应的新增团队业绩
			// 注意：最大区的判断基于累计团队业绩（maxChildStakeSum），但减去的是该最大区对应的新增团队业绩
			// 最大区的新增业绩不一定是最大的，所以不能用新增业绩来判断哪个是最大区
			childCumulativeStake := calcCtx.subtreeStakeSum[child.Id]
			if childCumulativeStake.Equal(maxDistrictCumulativeStake) {
				maxDistrict = childSubtree // 记录最大区对应的新增团队业绩
			}
		}

		// 计算子树新增业绩 = 自己的新增个人业绩 + 所有子节点的子树新增业绩总和
		subtreeTotal := newPersonalMap[userID].Add(childSubtreeTotal)
		subtreeIncreaseMap[userID] = subtreeTotal

		// 新增团队业绩 = 所有子节点的子树新增业绩总和（不包括自己）
		// 注意：团队业绩不包括自己的新增个人业绩
		newTeamIncreaseMap[userID] = childSubtreeTotal

		// 新增小区业绩 = 所有子区的新增团队业绩总和 - 最大区的新增团队业绩
		// 小区业绩算法：每个直接下级形成一个区，小区业绩 = 所有区的团队业绩总和 - 最大区团队业绩
		// 注意：最大区的判断基于累计团队业绩（maxChildStakeSum），但减去的是该最大区对应的新增团队业绩
		// 最大区的新增业绩不一定是最大的，所以必须用累计业绩来判断哪个是最大区
		districtIncrease := totalDistrict.Sub(maxDistrict)
		if districtIncrease.LessThan(decimal.Zero) {
			districtIncrease = decimal.Zero // 确保非负
		}
		newDistrictIncreaseMap[userID] = districtIncrease
	}

	return directIncreaseMap, newTeamIncreaseMap, newDistrictIncreaseMap, subtreeIncreaseMap
}

// ComputeDailyPerformances 仅计算当日业绩记录（不落库），返回待创建的数据和跳过数量（公开方法，用于验证）
func (s *performanceService) ComputeDailyPerformances(ctx context.Context, date time.Time) ([]*rewardEntity.UserPerformanceEntity, int, error) {
	return s.computeDailyPerformances(ctx, date)
}

// computeDailyPerformances 仅计算当日业绩记录（不落库），返回待创建的数据和跳过数量
// 该函数是业绩计算的核心流程，分为三个步骤：
// Step1: 构建计算上下文（一次性加载所有需要的数据到内存）
// Step2: 生成基础业绩记录（个人业绩、团队业绩、小区业绩等）
// Step3: 填充新增业绩字段（新增直推、团队、小区业绩）
//
// 返回值：
// - []*rewardEntity.UserPerformanceEntity: 待创建的业绩记录列表
// - int: 跳过的记录数量（已存在业绩记录，无需重复计算）
// - error: 错误信息
func (s *performanceService) computeDailyPerformances(ctx context.Context, date time.Time) ([]*rewardEntity.UserPerformanceEntity, int, error) {
	g.Log().Infof(ctx, "[业绩统计] 开始计算每日业绩: date=%s", date.Format(consts.TimeFormatDate))

	// Step1: 聚合用户、质押、历史业绩等上下文数据
	// 一次性加载所有需要的数据到内存，避免在计算过程中重复查询数据库
	calcCtx, err := s.buildPerformanceCalcContext(ctx, date)
	if err != nil {
		return nil, 0, err
	}
	defer calcCtx.release() // 释放上下文占用的内存

	g.Log().Infof(ctx, "[业绩统计] 已存在业绩记录数: %d", len(calcCtx.existingPerfMap))

	// Step2: 基于上下文生成当日基础业绩记录
	// 生成个人业绩、团队业绩、小区业绩、直推人数、团队人数等基础字段
	// 注意：新增直推、团队、小区业绩字段在此阶段暂设为0
	recordResult, err := s.generatePerformanceRecords(date, calcCtx)
	if err != nil {
		return nil, 0, err
	}
	defer recordResult.release() // 释放结果占用的内存

	// 如果没有需要创建的记录，直接返回
	if len(recordResult.records) == 0 {
		return recordResult.records, recordResult.skipCount, nil
	}

	// Step3: 在内存中计算并填充新增业绩字段
	// 使用后序遍历算法，自底向上聚合新增直推、团队、小区业绩
	s.fillIncrementalPerformances(calcCtx, recordResult)

	// Step4: 批量获取有效VIP等级并填充到metadata
	if err := s.fillEffectiveVIPLevelsToMetadata(ctx, recordResult.records); err != nil {
		g.Log().Warningf(ctx, "[业绩统计] 填充有效VIP等级到metadata失败: %v", err)
		// 不中断流程，继续执行
	}

	// Step5: 入库前检查组合业绩统计情况
	s.checkCombinationPerformanceBeforeSave(ctx, recordResult.records, calcCtx)

	return recordResult.records, recordResult.skipCount, nil
}

// calculateTeamPerformance 计算团队业绩：除去自己，下面网体的所有个人业绩之和
// 计算公式：团队业绩 = 子树质押总和 - 自己的质押金额
// 其中：
// - subtreeStakeSum[userID]: 该用户及其所有下级的质押金额总和
// - userStakeAmountMap[userID]: 该用户自己的质押金额
// - 团队业绩 = subtreeStakeSum[userID] - userStakeAmountMap[userID]
//
// 注意：如果用户没有下级，团队业绩为0
func (s *performanceService) calculateTeamPerformance(
	userID int64,
	calcCtx *performanceCalcContext,
) decimal.Decimal {
	// 如果没有下级，团队业绩为0
	if len(calcCtx.childrenMap[userID]) == 0 {
		return decimal.Zero
	}
	// 团队业绩 = 子树质押总和 - 自己的质押金额
	total := calcCtx.subtreeStakeSum[userID]
	return total.Sub(calcCtx.userStakeAmountMap[userID])
}

// calculateTeamCombinationPerformance 计算团队组合业绩：除去自己，下面网体的所有组合质押金额之和
// 计算公式：团队组合业绩 = 子树组合质押总和 - 自己的组合质押金额
// 其中：
// - subtreeStakeCombinationSum[userID]: 该用户及其所有下级的组合质押金额总和
// - userStakeCombinationAmountMap[userID]: 该用户自己的组合质押金额
// - 团队组合业绩 = subtreeStakeCombinationSum[userID] - userStakeCombinationAmountMap[userID]
//
// 注意：如果用户没有下级，团队组合业绩为0
func (s *performanceService) calculateTeamCombinationPerformance(
	userID int64,
	calcCtx *performanceCalcContext,
) decimal.Decimal {
	// 如果没有下级，团队组合业绩为0
	if len(calcCtx.childrenMap[userID]) == 0 {
		return decimal.Zero
	}
	// 团队组合业绩 = 子树组合质押总和 - 自己的组合质押金额
	total := calcCtx.subtreeStakeCombinationSum[userID]
	return total.Sub(calcCtx.userStakeCombinationAmountMap[userID])
}

// calculateDistrictPerformanceIterative 计算小区业绩（每个直接下级形成一个区）
// 小区业绩算法说明：
// 1. 每个直接下级形成一个区
// 2. 每个区的团队业绩 = 该直接下级及其所有间接下级的个人业绩总和（即该直接下级的子树质押总和）
// 3. 小区业绩 = 所有区的团队业绩总和 - 最大区团队业绩
//
// 计算公式：
// - totalSubtree = 该用户及其所有下级的质押总和
// - selfStake = 该用户自己的质押金额
// - totalTeam = totalSubtree - selfStake（团队业绩 = 所有下级的质押总和）
// - maxChild = 最大子区的质押总和（所有直接下级中，子树质押最大的值）
// - district = totalTeam - maxChild（小区业绩 = 团队业绩 - 最大区团队业绩）
//
// 返回值：
// - districtPerformance: 小区业绩
// - maxDistrictPerformance: 最大区团队业绩（用于VIP等级计算）
func (s *performanceService) calculateDistrictPerformanceIterative(
	userID int64,
	calcCtx *performanceCalcContext,
) (districtPerformance decimal.Decimal, maxDistrictPerformance decimal.Decimal) {
	children := calcCtx.childrenMap[userID]
	// 如果没有直接下级，小区业绩为0
	if len(children) == 0 {
		return decimal.Zero, decimal.Zero
	}

	// 获取自己的质押金额
	selfStake := calcCtx.userStakeAmountMap[userID]
	// 获取子树质押总和（包括自己及其所有下级）
	totalSubtree := calcCtx.subtreeStakeSum[userID]
	// 计算团队业绩 = 子树质押总和 - 自己的质押金额
	totalTeam := totalSubtree.Sub(selfStake)
	// 获取最大子区的质押总和（所有直接下级中，子树质押最大的值）
	maxChild := calcCtx.maxChildStakeSum[userID]
	// 计算小区业绩 = 团队业绩 - 最大区团队业绩
	district := totalTeam.Sub(maxChild)
	// 确保非负
	if district.LessThan(decimal.Zero) {
		district = decimal.Zero
	}

	return district, maxChild
}

// calculateDirectReferralIncrease 计算直推新增业绩
func (s *performanceService) calculateDirectReferralIncrease(
	userID int64,
	childrenMap map[int64][]*entity.UserEntity,
	stakeAmountMap map[int64]decimal.Decimal,
	prevPerfMap map[int64]decimal.Decimal,
) decimal.Decimal {
	children := childrenMap[userID]
	if len(children) == 0 {
		return decimal.Zero
	}

	totalIncrease := decimal.Zero
	for _, child := range children {
		// 当前个人业绩 = 质押金额
		currentPerf := stakeAmountMap[child.Id]

		// 前一天个人业绩
		prevPerf := prevPerfMap[child.Id]

		// 计算新增
		increase := currentPerf.Sub(prevPerf)
		if increase.GreaterThan(decimal.Zero) {
			totalIncrease = totalIncrease.Add(increase)
		}
	}

	return totalIncrease
}

// countTeamMembers 计算团队人数（使用队列迭代）
// countTeamMembers 计算团队人数（使用预计算结果）

// countActiveDirectMembers 统计直推有效人数（个人业绩≥阈值）
func (s *performanceService) countActiveDirectMembers(
	userID int64,
	childrenMap map[int64][]*entity.UserEntity,
	stakeAmountMap map[int64]decimal.Decimal,
) int {
	children := childrenMap[userID]
	if len(children) == 0 {
		return 0
	}

	threshold := consts.GetMinPerformanceThreshold()
	count := 0
	for _, child := range children {
		// 个人业绩 = 质押金额
		currentPerf := stakeAmountMap[child.Id]
		if currentPerf.GreaterThanOrEqual(threshold) {
			count++
		}
	}
	return count
}

// CalculateVIPLevels 计算VIP等级（业务逻辑层）
func (s *performanceService) CalculateVIPLevels(ctx context.Context, date time.Time) error {
	g.Log().Infof(ctx, "[VIP等级] 开始计算VIP等级: date=%s", date.Format(consts.TimeFormatDate))

	// 1. 获取所有用户的业绩数据
	performances, err := s.perfRepo.GetPerformanceByDate(ctx, date)
	if err != nil {
		return gerror.Wrap(err, "获取业绩数据失败")
	}

	if len(performances) == 0 {
		g.Log().Info(ctx, "[VIP等级] 没有业绩数据，跳过VIP等级计算")
		return nil
	}

	// 2. 批量获取当前VIP等级（一次性查询所有用户）
	userIDs := make([]int64, len(performances))
	for i, perf := range performances {
		userIDs[i] = perf.UserID
	}

	currentVIPs, err := s.vipRepo.BatchGetCurrentVIPs(ctx, userIDs)
	if err != nil {
		return gerror.Wrap(err, "批量获取当前VIP等级失败")
	}

	// 3. 构建当前VIP映射
	currentVIPMap := make(map[int64]*rewardEntity.UserVipLevelEntity)
	for _, vip := range currentVIPs {
		currentVIPMap[vip.UserID] = vip
	}

	// 4. 遍历处理：新用户收集批量创建，已存在用户直接更新
	var newVIPs []*rewardEntity.UserVipLevelEntity
	successCount := 0
	failedCount := 0
	updateCount := 0
	skipCount := 0

	for _, perf := range performances {
		// 业务逻辑：计算新的VIP等级
		vipLevel := 0
		rewardRate := decimal.Zero

		if perf.PersonalPerformance.GreaterThanOrEqual(consts.GetMinPerformanceThreshold()) {
			vipLevel = s.getVIPLevel(perf.DistrictPerformance)
			rewardRate = s.getVIPRate(vipLevel)
		}

		// 检查是否需要更新
		currentVIP, exists := currentVIPMap[perf.UserID]
		if !exists {
			// 新用户，收集批量创建
			newVIP := &rewardEntity.UserVipLevelEntity{
				UserID:              perf.UserID,
				VipLevel:            vipLevel,
				DistrictPerformance: perf.DistrictPerformance,
				RewardRate:          rewardRate,
				RecordTime:          date,
				PrevVipLevel:        nil,
				IsCurrent:           true,
			}
			newVIPs = append(newVIPs, newVIP)
		} else {
			// 检查VIP等级是否变更
			if currentVIP.VipLevel != vipLevel {
				// 需要更新，保存到Repository层处理
				prevLevel := currentVIP.VipLevel
				newVIP := &rewardEntity.UserVipLevelEntity{
					UserID:              perf.UserID,
					VipLevel:            vipLevel,
					DistrictPerformance: perf.DistrictPerformance,
					RewardRate:          rewardRate,
					RecordTime:          date,
					PrevVipLevel:        &prevLevel,
					IsCurrent:           true,
				}

				err := s.updateVIPLevel(ctx, currentVIP.Id, newVIP)
				if err != nil {
					g.Log().Errorf(ctx, "[VIP等级] 更新VIP失败: userID=%d, err=%v", perf.UserID, err)
					failedCount++
				} else {
					updateCount++
					g.Log().Infof(ctx, "[VIP等级] VIP变更: userID=%d, %d -> %d, 小区业绩=%s",
						perf.UserID, prevLevel, vipLevel, perf.DistrictPerformance.String())
				}
			} else {
				// VIP等级无变化，跳过
				skipCount++
			}
		}
	}

	// 5. 批量创建新用户的VIP记录（调用Repository层）
	if len(newVIPs) > 0 {
		g.Log().Infof(ctx, "[VIP等级] 开始批量创建新VIP记录: %d条", len(newVIPs))
		err = s.vipRepo.BatchCreateVIPs(ctx, newVIPs)
		if err != nil {
			g.Log().Errorf(ctx, "[VIP等级] 批量创建失败: %v", err)
			failedCount += len(newVIPs)
		} else {
			successCount = len(newVIPs)
			g.Log().Infof(ctx, "[VIP等级] 批量创建完成: %d条", len(newVIPs))
		}
	}

	successCount += updateCount
	g.Log().Infof(ctx, "[VIP等级] 计算完成: 成功=%d (新增=%d, 更新=%d), 跳过=%d, 失败=%d",
		successCount, len(newVIPs), updateCount, skipCount, failedCount)
	return nil
}

// calculateVIPLevelsFromPerformances 基于传入的业绩记录评定VIP，避免重复读取业绩
func (s *performanceService) calculateVIPLevelsFromPerformances(ctx context.Context, date time.Time, performances []*rewardEntity.UserPerformanceEntity) error {
	if len(performances) == 0 {
		g.Log().Info(ctx, "[VIP等级] 无需计算（无新增业绩记录）")
		return nil
	}

	// 2. 批量获取当前VIP等级（一次性查询所有用户）
	userIDs := make([]int64, len(performances))
	for i, perf := range performances {
		userIDs[i] = perf.UserID
	}

	currentVIPs, err := s.vipRepo.BatchGetCurrentVIPs(ctx, userIDs)
	if err != nil {
		return gerror.Wrap(err, "批量获取当前VIP等级失败")
	}

	// 3. 构建当前VIP映射
	currentVIPMap := make(map[int64]*rewardEntity.UserVipLevelEntity)
	for _, vip := range currentVIPs {
		currentVIPMap[vip.UserID] = vip
	}

	var newVIPs []*rewardEntity.UserVipLevelEntity
	successCount := 0
	failedCount := 0
	updateCount := 0
	skipCount := 0

	for _, perf := range performances {
		// 业务逻辑：计算新的VIP等级
		vipLevel := 0
		rewardRate := decimal.Zero

		if perf.PersonalPerformance.GreaterThanOrEqual(consts.GetMinPerformanceThreshold()) {
			vipLevel = s.getVIPLevel(perf.DistrictPerformance)
			rewardRate = s.getVIPRate(vipLevel)
		}

		// 检查是否需要更新
		currentVIP, exists := currentVIPMap[perf.UserID]
		if !exists {
			// 新用户，收集批量创建
			newVIP := &rewardEntity.UserVipLevelEntity{
				UserID:              perf.UserID,
				VipLevel:            vipLevel,
				DistrictPerformance: perf.DistrictPerformance,
				RewardRate:          rewardRate,
				RecordTime:          date,
				PrevVipLevel:        nil,
				IsCurrent:           true,
			}
			newVIPs = append(newVIPs, newVIP)
		} else {
			// 检查VIP等级是否变更
			if currentVIP.VipLevel != vipLevel {
				// 需要更新，保存到Repository层处理
				prevLevel := currentVIP.VipLevel
				newVIP := &rewardEntity.UserVipLevelEntity{
					UserID:              perf.UserID,
					VipLevel:            vipLevel,
					DistrictPerformance: perf.DistrictPerformance,
					RewardRate:          rewardRate,
					RecordTime:          date,
					PrevVipLevel:        &prevLevel,
					IsCurrent:           true,
				}

				err := s.updateVIPLevel(ctx, currentVIP.Id, newVIP)
				if err != nil {
					g.Log().Errorf(ctx, "[VIP等级] 更新VIP失败: userID=%d, err=%v", perf.UserID, err)
					failedCount++
				} else {
					updateCount++
					g.Log().Infof(ctx, "[VIP等级] VIP变更: userID=%d, %d -> %d, 小区业绩=%s",
						perf.UserID, prevLevel, vipLevel, perf.DistrictPerformance.String())
				}
			} else {
				// VIP等级无变化，跳过
				skipCount++
			}
		}
	}

	// 5. 批量创建新用户的VIP记录（调用Repository层）
	if len(newVIPs) > 0 {
		g.Log().Infof(ctx, "[VIP等级] 开始批量创建新VIP记录: %d条", len(newVIPs))
		err := s.vipRepo.BatchCreateVIPs(ctx, newVIPs)
		if err != nil {
			g.Log().Errorf(ctx, "[VIP等级] 批量创建失败: %v", err)
			failedCount += len(newVIPs)
		} else {
			successCount = len(newVIPs)
			g.Log().Infof(ctx, "[VIP等级] 批量创建完成: %d条", len(newVIPs))
		}
	}

	successCount += updateCount
	g.Log().Infof(ctx, "[VIP等级] 计算完成: 成功=%d (新增=%d, 更新=%d), 跳过=%d, 失败=%d",
		successCount, len(newVIPs), updateCount, skipCount, failedCount)
	return nil
}

// updateVIPLevel 更新VIP等级（在事务中处理）
func (s *performanceService) updateVIPLevel(ctx context.Context, oldVIPID int64, newVIP *rewardEntity.UserVipLevelEntity) error {
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 将旧记录设为非当前
		if err := s.vipRepo.SetNotCurrent(ctx, tx, oldVIPID); err != nil {
			return gerror.Wrap(err, "设置旧VIP记录为非当前失败")
		}

		// 创建新的VIP记录
		if err := s.vipRepo.CreateVIP(ctx, tx, newVIP); err != nil {
			return gerror.Wrap(err, "创建新VIP记录失败")
		}

		return nil
	})
}

// getVIPLevel 根据小区业绩评定VIP等级
func (s *performanceService) getVIPLevel(districtPerformance decimal.Decimal) int {
	return consts.GetVIPLevel(districtPerformance)
}

// getVIPRate 获取VIP等级对应的奖励比例
func (s *performanceService) getVIPRate(level int) decimal.Decimal {
	return consts.GetVIPRewardRate(level)
}

// fillEffectiveVIPLevelsToMetadata 批量获取有效VIP等级并填充到metadata
// 支持分批处理，避免数据量过大导致查询失败
func (s *performanceService) fillEffectiveVIPLevelsToMetadata(ctx context.Context, records []*rewardEntity.UserPerformanceEntity) error {
	if len(records) == 0 {
		return nil
	}

	const maxBatchSize = 10000 // 每批次最多1万个用户，避免单次查询数据量过大

	// 如果数据量不大，直接处理
	if len(records) <= maxBatchSize {
		return s.fillEffectiveVIPLevelsToMetadataBatch(ctx, records)
	}

	// 分批处理
	totalBatches := (len(records) + maxBatchSize - 1) / maxBatchSize
	g.Log().Infof(ctx, "[业绩统计] 填充有效VIP等级到metadata，记录数 %d 超过限制，将分 %d 批处理", len(records), totalBatches)

	for i := 0; i < len(records); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(records) {
			end = len(records)
		}
		batch := records[i:end]
		batchIndex := i/maxBatchSize + 1

		if err := s.fillEffectiveVIPLevelsToMetadataBatch(ctx, batch); err != nil {
			return gerror.Wrapf(err, "批量获取有效VIP等级失败（第 %d/%d 批）", batchIndex, totalBatches)
		}
	}

	return nil
}

// fillEffectiveVIPLevelsToMetadataBatch 处理单批次的有效VIP等级填充
func (s *performanceService) fillEffectiveVIPLevelsToMetadataBatch(ctx context.Context, records []*rewardEntity.UserPerformanceEntity) error {
	if len(records) == 0 {
		return nil
	}

	// 构建用户ID到计算VIP等级的映射
	userIDToLevelMap := make(map[int64]int, len(records))
	for _, perf := range records {
		userIDToLevelMap[perf.UserID] = perf.VipLevel
	}

	// 批量获取有效VIP等级
	effectiveVIPLevels, err := s.vipAdjustmentSvc.BatchGetEffectiveVIPLevels(ctx, userIDToLevelMap)
	if err != nil {
		return gerror.Wrap(err, "批量获取有效VIP等级失败")
	}

	// 填充metadata
	for _, perf := range records {
		effectiveLevel := perf.VipLevel // 默认使用计算等级
		if level, ok := effectiveVIPLevels[perf.UserID]; ok {
			effectiveLevel = level
		}

		// 构建metadata
		metadata := rewardEntity.PerformanceMetadata{
			EffectiveVipLevel: effectiveLevel,
		}
		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			g.Log().Warningf(ctx, "[业绩统计] 序列化metadata失败: userID=%d, err=%v", perf.UserID, err)
			continue
		}
		perf.Metadata = string(metadataJSON)
	}

	return nil
}

// checkCombinationPerformanceBeforeSave 入库前检查组合业绩统计情况
func (s *performanceService) checkCombinationPerformanceBeforeSave(ctx context.Context, records []*rewardEntity.UserPerformanceEntity, calcCtx *performanceCalcContext) {
	if len(records) == 0 {
		return
	}

	// 统计有组合业绩的记录数量
	personalCombinationCount := 0
	teamCombinationCount := 0
	personalCombinationTotal := decimal.Zero
	teamCombinationTotal := decimal.Zero

	// 检查上下文中的组合质押数据
	contextCombinationCount := 0
	contextCombinationTotal := decimal.Zero
	for _, amount := range calcCtx.userStakeCombinationAmountMap {
		if !amount.IsZero() {
			contextCombinationCount++
			contextCombinationTotal = contextCombinationTotal.Add(amount)
		}
	}

	// 检查上下文中的子树组合质押数据
	contextSubtreeCombinationCount := 0
	contextSubtreeCombinationTotal := decimal.Zero
	for _, amount := range calcCtx.subtreeStakeCombinationSum {
		if !amount.IsZero() {
			contextSubtreeCombinationCount++
			contextSubtreeCombinationTotal = contextSubtreeCombinationTotal.Add(amount)
		}
	}

	// 检查记录中的组合业绩
	for _, record := range records {
		if !record.PersonalCombinationPerformance.IsZero() {
			personalCombinationCount++
			personalCombinationTotal = personalCombinationTotal.Add(record.PersonalCombinationPerformance)
		}
		if !record.TeamCombinationPerformance.IsZero() {
			teamCombinationCount++
			teamCombinationTotal = teamCombinationTotal.Add(record.TeamCombinationPerformance)
		}
	}

	// 打印统计信息
	g.Log().Infof(ctx, "[业绩统计-组合业绩检查] ========== 入库前组合业绩统计检查 ==========")
	g.Log().Infof(ctx, "[业绩统计-组合业绩检查] 上下文数据:")
	g.Log().Infof(ctx, "[业绩统计-组合业绩检查]   - 个人组合质押映射: 有数据用户数=%d, 总金额=%s", contextCombinationCount, contextCombinationTotal.String())
	g.Log().Infof(ctx, "[业绩统计-组合业绩检查]   - 子树组合质押映射: 有数据用户数=%d, 总金额=%s", contextSubtreeCombinationCount, contextSubtreeCombinationTotal.String())
	g.Log().Infof(ctx, "[业绩统计-组合业绩检查] 业绩记录数据:")
	g.Log().Infof(ctx, "[业绩统计-组合业绩检查]   - 个人组合业绩: 有数据记录数=%d/%d, 总金额=%s", personalCombinationCount, len(records), personalCombinationTotal.String())
	g.Log().Infof(ctx, "[业绩统计-组合业绩检查]   - 团队组合业绩: 有数据记录数=%d/%d, 总金额=%s", teamCombinationCount, len(records), teamCombinationTotal.String())

	// 如果上下文有组合质押数据，但记录中没有，打印警告
	if contextCombinationCount > 0 && personalCombinationCount == 0 {
		g.Log().Warningf(ctx, "[业绩统计-组合业绩检查] ⚠️ 警告: 上下文中有 %d 个用户有组合质押数据，但业绩记录中个人组合业绩全为0！", contextCombinationCount)
		// 打印前10个有组合质押的用户详情
		count := 0
		for userID, amount := range calcCtx.userStakeCombinationAmountMap {
			if !amount.IsZero() && count < 10 {
				// 查找对应的业绩记录
				var foundRecord *rewardEntity.UserPerformanceEntity
				for _, record := range records {
					if record.UserID == userID {
						foundRecord = record
						break
					}
				}
				if foundRecord != nil {
					g.Log().Warningf(ctx, "[业绩统计-组合业绩检查]   用户 %d: 上下文组合质押=%s, 记录个人组合业绩=%s, 记录团队组合业绩=%s",
						userID, amount.String(), foundRecord.PersonalCombinationPerformance.String(), foundRecord.TeamCombinationPerformance.String())
				} else {
					g.Log().Warningf(ctx, "[业绩统计-组合业绩检查]   用户 %d: 上下文组合质押=%s, 但未找到对应业绩记录", userID, amount.String())
				}
				count++
			}
		}
	}

	// 如果上下文有子树组合质押数据，但记录中没有团队组合业绩，打印警告
	if contextSubtreeCombinationCount > 0 && teamCombinationCount == 0 {
		g.Log().Warningf(ctx, "[业绩统计-组合业绩检查] ⚠️ 警告: 上下文中有 %d 个用户有子树组合质押数据，但业绩记录中团队组合业绩全为0！", contextSubtreeCombinationCount)
	}

	g.Log().Infof(ctx, "[业绩统计-组合业绩检查] ==========================================")
}
