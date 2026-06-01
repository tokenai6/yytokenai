package servicecenter

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"XWFrame/internal/entity"
	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/consts"
	rewardRepo "XWFrame/internal/repository/reward"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// ServiceCenterNode 表示服务中心节点（构建奖励树时使用）
type ServiceCenterNode struct {
	UserID                 int64
	Rate                   decimal.Decimal
	Children               []*ServiceCenterNode
	RewardBase             decimal.Decimal
	RewardAmount           decimal.Decimal
	TeamNewPerformance     decimal.Decimal
	PersonalNewPerformance decimal.Decimal
	Calculated             bool
	AllocatedDetails       []ServiceCenterAllocatedDetail // 被下级分走的明细
}

// ServiceCenterAllocatedDetail 被下级分走的明细
type ServiceCenterAllocatedDetail struct {
	ChildUserID          int64           `json:"child_user_id"`          // 下级服务中心用户ID
	ChildRate            decimal.Decimal `json:"child_rate"`             // 下级点位比例（百分比）
	ChildTeamPerformance decimal.Decimal `json:"child_team_performance"` // 下级团队新增业绩
	AllocatedAmount      decimal.Decimal `json:"allocated_amount"`       // 被该下级分走的金额（下级团队新增业绩 * 下级点位比例）
}

// ServiceCenterRewardMetadata 资产记录附加信息
type ServiceCenterRewardMetadata struct {
	ServiceCenterRate  decimal.Decimal                `json:"service_center_rate"`  // 服务中心点位比例（百分比）
	TeamNewPerformance decimal.Decimal                `json:"team_new_performance"` // 团队新增业绩（伞下所有用户的新增业绩总和）
	RewardBase         decimal.Decimal                `json:"reward_base"`          // 奖励基数（团队新增业绩 * 点位比例）
	RewardAmount       decimal.Decimal                `json:"reward_amount"`        // 实际奖励金额（奖励基数 - 被下级分走的金额）
	AllocatedDetails   []ServiceCenterAllocatedDetail `json:"allocated_details"`    // 被下级分走的明细列表
}

// serviceCenterData 批量查询的数据结构
type serviceCenterData struct {
	allUsers           []*entity.UserEntity
	serviceCenterUsers []*entity.UserEntity
	performances       []*rewardEntity.UserPerformanceEntity
	existsMap          map[int64]bool
}

// calculationContext 计算时使用的上下文
type calculationContext struct {
	nodeMap            map[int64]*ServiceCenterNode
	userMap            map[int64]*entity.UserEntity
	performanceMap     map[int64]*rewardEntity.UserPerformanceEntity
	personalMap        map[int64]decimal.Decimal
	childrenMap        map[int64][]*entity.UserEntity
	childToParentMap   map[int64]int64
	serviceCenterSet   map[int64]struct{}
	nonServiceCacheMap map[int64]decimal.Decimal
}

// ServiceCenterCalculationResult 服务中心奖励计算结果
type ServiceCenterCalculationResult struct {
	RecordTime         time.Time
	NodeMap            map[int64]*ServiceCenterNode
	ExistsMap          map[int64]bool
	AllUsers           []*entity.UserEntity
	ServiceCenterUsers []*entity.UserEntity
	UserMap            map[int64]*entity.UserEntity
	PerformanceMap     map[int64]*rewardEntity.UserPerformanceEntity
	TeamNewPerformance map[int64]decimal.Decimal
	ChildrenMap        map[int64][]*entity.UserEntity
	ChildToParentMap   map[int64]int64
}

// IServiceCenterService 服务中心奖励服务接口
type IServiceCenterService interface {
	// DistributeServiceCenterReward 发放服务中心奖励
	DistributeServiceCenterReward(ctx context.Context, recordTime time.Time) error

	// PreviewServiceCenterReward 预览服务中心奖励（仅计算，不写入记录）
	PreviewServiceCenterReward(ctx context.Context, recordTime time.Time) (*ServiceCenterCalculationResult, error)
}

// serviceCenterService 服务中心奖励服务实现
type serviceCenterService struct {
	serviceCenterRepo rewardRepo.IServiceCenterRewardRepository
}

// NewServiceCenterService 创建服务中心奖励服务实例
func NewServiceCenterService() IServiceCenterService {
	return NewServiceCenterServiceWithRepo(rewardRepo.NewServiceCenterRewardRepository())
}

// NewServiceCenterServiceWithRepo 创建带自定义仓储的服务中心奖励服务实例
func NewServiceCenterServiceWithRepo(repo rewardRepo.IServiceCenterRewardRepository) IServiceCenterService {
	return &serviceCenterService{
		serviceCenterRepo: repo,
	}
}

// DistributeServiceCenterReward 发放服务中心奖励
func (s *serviceCenterService) DistributeServiceCenterReward(ctx context.Context, recordTime time.Time) error {
	g.Log().Infof(ctx, "[服务中心奖励] 开始发放，recordTime=%s", recordTime.Format(consts.TimeFormatDateTime))

	result, err := s.PreviewServiceCenterReward(ctx, recordTime)
	if err != nil {
		return err
	}

	if result == nil || len(result.ServiceCenterUsers) == 0 {
		g.Log().Infof(ctx, "[服务中心奖励] 无服务中心用户，跳过计算")
		return nil
	}

	if err := s.batchCreateRecords(ctx, recordTime, result.NodeMap, result.ExistsMap); err != nil {
		return err
	}

	g.Log().Infof(ctx, "[服务中心奖励] 发放完成")
	return nil
}

// PreviewServiceCenterReward 预览服务中心奖励（仅计算，不写入记录）
func (s *serviceCenterService) PreviewServiceCenterReward(ctx context.Context, recordTime time.Time) (*ServiceCenterCalculationResult, error) {
	data, err := s.loadBaseData(ctx, recordTime)
	if err != nil {
		return nil, err
	}

	result := &ServiceCenterCalculationResult{
		RecordTime:         recordTime,
		ExistsMap:          data.existsMap,
		AllUsers:           data.allUsers,
		ServiceCenterUsers: data.serviceCenterUsers,
		PerformanceMap:     make(map[int64]*rewardEntity.UserPerformanceEntity),
	}

	for _, perf := range data.performances {
		if perf == nil {
			continue
		}
		result.PerformanceMap[perf.UserID] = perf
	}

	if len(data.serviceCenterUsers) == 0 {
		return result, nil
	}

	ctxStruct := s.buildCalculationContext(data)
	s.buildServiceCenterTree(ctxStruct)
	s.calculateTeamNewPerformance(ctxStruct)
	s.calculateRewards(ctxStruct)

	result.NodeMap = ctxStruct.nodeMap
	result.TeamNewPerformance = make(map[int64]decimal.Decimal, len(ctxStruct.nodeMap))
	for id, node := range ctxStruct.nodeMap {
		result.TeamNewPerformance[id] = node.TeamNewPerformance
	}

	result.ChildrenMap = ctxStruct.childrenMap
	result.ChildToParentMap = ctxStruct.childToParentMap
	result.UserMap = ctxStruct.userMap

	return result, nil
}

// loadBaseData 批量查询计算所需基础数据
func (s *serviceCenterService) loadBaseData(ctx context.Context, recordTime time.Time) (*serviceCenterData, error) {
	allUsers, err := s.serviceCenterRepo.GetAllUsers(ctx)
	if err != nil {
		return nil, gerror.Wrap(err, "获取所有用户失败")
	}

	serviceCenterUsers := s.filterServiceCenterUsers(allUsers)
	if len(serviceCenterUsers) == 0 {
		return &serviceCenterData{
			allUsers:           allUsers,
			serviceCenterUsers: serviceCenterUsers,
			performances:       []*rewardEntity.UserPerformanceEntity{},
			existsMap:          map[int64]bool{},
		}, nil
	}

	g.Log().Infof(ctx, "[服务中心奖励] 服务中心用户数: %d", len(serviceCenterUsers))

	performances, err := s.serviceCenterRepo.GetPerformanceByDate(ctx, recordTime)
	if err != nil {
		return nil, gerror.Wrap(err, "获取用户业绩失败")
	}

	serviceCenterUserIDs := s.extractUserIDs(serviceCenterUsers)
	existsMap, err := s.serviceCenterRepo.BatchCheckExists(ctx, serviceCenterUserIDs, recordTime)
	if err != nil {
		return nil, gerror.Wrap(err, "批量幂等性检查失败")
	}

	return &serviceCenterData{
		allUsers:           allUsers,
		serviceCenterUsers: serviceCenterUsers,
		performances:       performances,
		existsMap:          existsMap,
	}, nil
}

func (s *serviceCenterService) filterServiceCenterUsers(allUsers []*entity.UserEntity) []*entity.UserEntity {
	result := make([]*entity.UserEntity, 0)
	for _, user := range allUsers {
		if user.IsServiceCenter && user.ServiceCenterRate.GreaterThan(decimal.Zero) {
			result = append(result, user)
		}
	}
	return result
}

func (s *serviceCenterService) extractUserIDs(users []*entity.UserEntity) []int64 {
	ids := make([]int64, 0, len(users))
	for _, user := range users {
		ids = append(ids, user.Id)
	}
	return ids
}

// buildCalculationContext 构建计算上下文
func (s *serviceCenterService) buildCalculationContext(data *serviceCenterData) *calculationContext {
	ctxStruct := &calculationContext{
		nodeMap:            make(map[int64]*ServiceCenterNode),
		userMap:            make(map[int64]*entity.UserEntity),
		performanceMap:     make(map[int64]*rewardEntity.UserPerformanceEntity),
		personalMap:        make(map[int64]decimal.Decimal),
		childrenMap:        make(map[int64][]*entity.UserEntity),
		childToParentMap:   make(map[int64]int64),
		serviceCenterSet:   make(map[int64]struct{}),
		nonServiceCacheMap: make(map[int64]decimal.Decimal),
	}

	for _, user := range data.allUsers {
		ctxStruct.userMap[user.Id] = user
	}

	for _, perf := range data.performances {
		if perf == nil {
			continue
		}
		ctxStruct.performanceMap[perf.UserID] = perf

		personal := perf.NewPersonalPerformance
		if personal.LessThan(decimal.Zero) {
			personal = decimal.Zero
		}
		if personal.GreaterThan(decimal.Zero) {
			ctxStruct.personalMap[perf.UserID] = personal
		}
	}

	inviteCodeToUserMap := make(map[string]*entity.UserEntity)
	for _, user := range data.allUsers {
		if user.InviteCode != "" {
			inviteCodeToUserMap[user.InviteCode] = user
		}
	}
	for _, user := range data.allUsers {
		if user.ParentInviteCode == "" {
			continue
		}
		parent, ok := inviteCodeToUserMap[user.ParentInviteCode]
		if !ok {
			continue
		}
		ctxStruct.childrenMap[parent.Id] = append(ctxStruct.childrenMap[parent.Id], user)
		ctxStruct.childToParentMap[user.Id] = parent.Id
	}

	for _, user := range data.serviceCenterUsers {
		rate := user.ServiceCenterRate.DivRound(decimal.NewFromInt(100), consts.TokenPrecision)
		node := &ServiceCenterNode{
			UserID:                 user.Id,
			Rate:                   rate,
			Children:               make([]*ServiceCenterNode, 0),
			PersonalNewPerformance: ctxStruct.personalMap[user.Id],
		}
		ctxStruct.nodeMap[user.Id] = node
		ctxStruct.serviceCenterSet[user.Id] = struct{}{}
	}

	return ctxStruct
}

// buildServiceCenterTree 构建服务中心层级树
func (s *serviceCenterService) buildServiceCenterTree(ctxStruct *calculationContext) {
	for id, node := range ctxStruct.nodeMap {
		parentID := s.findParentServiceCenter(id, ctxStruct.childToParentMap, ctxStruct.nodeMap)
		if parentID == 0 {
			continue
		}
		parentNode := ctxStruct.nodeMap[parentID]
		parentNode.Children = append(parentNode.Children, node)
	}
}

// calculateTeamNewPerformance 计算每个服务中心的团队新增业绩（伞下新增业绩）
func (s *serviceCenterService) calculateTeamNewPerformance(ctxStruct *calculationContext) {
	calculated := make(map[int64]bool)
	visiting := make(map[int64]bool)

	for id := range ctxStruct.nodeMap {
		s.computeServiceCenterTeam(id, ctxStruct, calculated, visiting)
	}
}

func (s *serviceCenterService) computeServiceCenterTeam(
	serviceCenterID int64,
	ctxStruct *calculationContext,
	calculated map[int64]bool,
	visiting map[int64]bool,
) decimal.Decimal {
	if calculated[serviceCenterID] {
		return ctxStruct.nodeMap[serviceCenterID].TeamNewPerformance
	}
	if visiting[serviceCenterID] {
		return ctxStruct.nodeMap[serviceCenterID].TeamNewPerformance
	}
	visiting[serviceCenterID] = true

	node := ctxStruct.nodeMap[serviceCenterID]
	total := ctxStruct.personalMap[serviceCenterID]

	for _, child := range ctxStruct.childrenMap[serviceCenterID] {
		if _, isServiceCenter := ctxStruct.serviceCenterSet[child.Id]; isServiceCenter {
			childTeam := s.computeServiceCenterTeam(child.Id, ctxStruct, calculated, visiting)
			total = total.Add(childTeam)
			continue
		}

		personal := ctxStruct.personalMap[child.Id]
		if personal.GreaterThan(decimal.Zero) {
			total = total.Add(personal)
		}

		descendant := s.collectNonServiceCenterPerformance(child.Id, ctxStruct)
		if descendant.GreaterThan(decimal.Zero) {
			total = total.Add(descendant)
		}
	}

	node.TeamNewPerformance = total
	calculated[serviceCenterID] = true
	delete(visiting, serviceCenterID)
	return total
}

// collectNonServiceCenterPerformance 统计普通用户子孙的个人新增业绩
func (s *serviceCenterService) collectNonServiceCenterPerformance(startUserID int64, ctxStruct *calculationContext) decimal.Decimal {
	if value, ok := ctxStruct.nonServiceCacheMap[startUserID]; ok {
		return value
	}

	total := decimal.Zero
	queue := []int64{startUserID}
	visited := make(map[int64]struct{})

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if _, ok := visited[current]; ok {
			continue
		}
		visited[current] = struct{}{}

		children := ctxStruct.childrenMap[current]
		if len(children) == 0 {
			continue
		}

		for _, child := range children {
			if _, isServiceCenter := ctxStruct.serviceCenterSet[child.Id]; isServiceCenter {
				personal := ctxStruct.personalMap[child.Id]
				if personal.GreaterThan(decimal.Zero) {
					total = total.Add(personal)
				}
				queue = append(queue, child.Id)
				continue
			}

			personal := ctxStruct.personalMap[child.Id]
			if personal.GreaterThan(decimal.Zero) {
				total = total.Add(personal)
			}
			queue = append(queue, child.Id)
		}
	}

	ctxStruct.nonServiceCacheMap[startUserID] = total
	return total
}

// calculateRewards 计算极差奖励
func (s *serviceCenterService) calculateRewards(ctxStruct *calculationContext) {
	hasParent := make(map[int64]bool)
	for _, node := range ctxStruct.nodeMap {
		for _, child := range node.Children {
			hasParent[child.UserID] = true
		}
	}

	for id, node := range ctxStruct.nodeMap {
		if hasParent[id] {
			continue
		}
		s.calculateRewardRecursive(node)
	}
}

func (s *serviceCenterService) calculateRewardRecursive(node *ServiceCenterNode) {
	for _, child := range node.Children {
		s.calculateRewardRecursive(child)
	}

	node.RewardBase = node.TeamNewPerformance.Mul(node.Rate)

	allocated := decimal.Zero
	node.AllocatedDetails = make([]ServiceCenterAllocatedDetail, 0)
	for _, child := range node.Children {
		if child.TeamNewPerformance.GreaterThan(decimal.Zero) && child.Rate.GreaterThan(decimal.Zero) {
			allocatedAmount := child.TeamNewPerformance.Mul(child.Rate)
			allocated = allocated.Add(allocatedAmount)

			// 记录被该下级分走的明细
			node.AllocatedDetails = append(node.AllocatedDetails, ServiceCenterAllocatedDetail{
				ChildUserID:          child.UserID,
				ChildRate:            child.Rate.Mul(decimal.NewFromInt(100)), // 转换为百分比
				ChildTeamPerformance: child.TeamNewPerformance,
				AllocatedAmount:      allocatedAmount,
			})
		}
	}

	node.RewardAmount = node.RewardBase.Sub(allocated)
	if node.RewardAmount.LessThan(decimal.Zero) {
		node.RewardAmount = decimal.Zero
	}
	node.Calculated = true
}

// batchCreateRecords 批量生成奖励记录
func (s *serviceCenterService) batchCreateRecords(
	ctx context.Context,
	recordTime time.Time,
	nodeMap map[int64]*ServiceCenterNode,
	existsMap map[int64]bool,
) error {
	recordsToCreate := make([]*rewardEntity.AssetRecordEntity, 0)

	for _, node := range nodeMap {
		if existsMap[node.UserID] {
			continue
		}
		if !node.RewardAmount.GreaterThan(decimal.Zero) {
			continue
		}

		record := s.buildAssetRecord(node, recordTime)
		recordsToCreate = append(recordsToCreate, record)
	}

	if err := s.serviceCenterRepo.BatchCreateAssetRecords(ctx, recordsToCreate); err != nil {
		return gerror.Wrap(err, "批量创建服务中心奖励记录失败")
	}

	g.Log().Infof(ctx, "[服务中心奖励] 批量创建记录成功: 数量=%d", len(recordsToCreate))
	return nil
}

// buildAssetRecord 构建资金记录
func (s *serviceCenterService) buildAssetRecord(node *ServiceCenterNode, recordTime time.Time) *rewardEntity.AssetRecordEntity {
	metadata := ServiceCenterRewardMetadata{
		ServiceCenterRate:  node.Rate.Mul(decimal.NewFromInt(100)),
		TeamNewPerformance: node.TeamNewPerformance,
		RewardBase:         node.RewardBase,
		RewardAmount:       node.RewardAmount,
		AllocatedDetails:   node.AllocatedDetails,
	}
	metadataJSON, _ := json.Marshal(metadata)

	ratePercent := node.Rate.Mul(decimal.NewFromInt(100))

	return &rewardEntity.AssetRecordEntity{
		UserID:       node.UserID,
		AssetType:    consts.AssetTypeAPGUserBalance,
		RecordTime:   recordTime,
		Amount:       node.RewardAmount,
		FlowType:     consts.AssetFlowTypeIncome,
		Status:       consts.AssetRecordStatusPending,
		BusinessType: consts.AssetBusinessTypeRewardServiceCenter,
		BusinessID:   0,
		Remark:       fmt.Sprintf("服务中心奖励-点位%s%%", utils.FormatDecimal(ratePercent)),
		Metadata:     string(metadataJSON),
	}
}

// findParentServiceCenter 查找最近的服务中心父节点
func (s *serviceCenterService) findParentServiceCenter(
	userID int64,
	childToParent map[int64]int64,
	nodeMap map[int64]*ServiceCenterNode,
) int64 {
	currentID := userID

	for {
		parentID, hasParent := childToParent[currentID]
		if !hasParent {
			break
		}
		if _, ok := nodeMap[parentID]; ok {
			return parentID
		}
		currentID = parentID
	}

	return 0
}
