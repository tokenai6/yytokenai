package settlement

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/consts"
	rewardRepo "XWFrame/internal/repository/reward"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// UserAssetPending 用户待结算资金汇总
type UserAssetPending struct {
	UserID      int64
	Date        time.Time
	TotalAmount decimal.Decimal
	RecordIDs   []int64
	Records     []*rewardEntity.AssetRecordEntity
}

// SettlementResult 结算结果
type SettlementResult struct {
	SuccessCount int
	ReducedCount int
	FailedCount  int
	DetailCount  int
	Error        error
}

// SafeSettledMap 线程安全的已结算记录map
type SafeSettledMap struct {
	mu   sync.RWMutex
	data map[string]bool
}

// NewSafeSettledMap 创建线程安全的已结算记录map
func NewSafeSettledMap(data map[string]bool) *SafeSettledMap {
	return &SafeSettledMap{
		data: data,
	}
}

// Get 安全读取
func (s *SafeSettledMap) Get(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[key]
}

// Set 安全写入
func (s *SafeSettledMap) Set(key string, value bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

// SafeSettlementRecordMap 线程安全的结算记录map
type SafeSettlementRecordMap struct {
	mu   sync.RWMutex
	data map[int64]*rewardEntity.SettlementRecordEntity
}

// NewSafeSettlementRecordMap 创建线程安全的结算记录map
func NewSafeSettlementRecordMap(data map[int64]*rewardEntity.SettlementRecordEntity) *SafeSettlementRecordMap {
	return &SafeSettlementRecordMap{
		data: data,
	}
}

// Get 安全读取
func (s *SafeSettlementRecordMap) Get(key int64) *rewardEntity.SettlementRecordEntity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[key]
}

// Set 安全写入
func (s *SafeSettlementRecordMap) Set(key int64, value *rewardEntity.SettlementRecordEntity) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

// WorkerPool 协程池
type WorkerPool struct {
	workerCount         int
	jobQueue            chan *UserAssetPending
	results             chan *SettlementResult
	wg                  sync.WaitGroup
	ctx                 context.Context
	settlementRepo      rewardRepo.ISettlementRepository
	settlementRecordMap *SafeSettlementRecordMap
	settledMap          *SafeSettledMap
}

// NewWorkerPool 创建协程池
func NewWorkerPool(workerCount, jobCount int) *WorkerPool {
	return &WorkerPool{
		workerCount: workerCount,
		jobQueue:    make(chan *UserAssetPending, jobCount),
		results:     make(chan *SettlementResult, jobCount),
	}
}

// Start 启动协程池
func (p *WorkerPool) Start(ctx context.Context, settlementRepo rewardRepo.ISettlementRepository, settlementRecordMap *SafeSettlementRecordMap, settledMap *SafeSettledMap) {
	p.ctx = ctx
	p.settlementRepo = settlementRepo
	p.settlementRecordMap = settlementRecordMap
	p.settledMap = settledMap

	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker()
	}

	go func() {
		p.wg.Wait()
		close(p.results)
	}()
}

// AddJob 添加任务
func (p *WorkerPool) AddJob(job *UserAssetPending) {
	p.jobQueue <- job
}

// Close 关闭协程池
func (p *WorkerPool) Close() {
	close(p.jobQueue)
}

// worker 工作协程
func (p *WorkerPool) worker() {
	defer p.wg.Done()

	for userAsset := range p.jobQueue {
		settled, reduced, detailCount, err := p.settleUserAssets(userAsset)
		result := &SettlementResult{
			SuccessCount: 0,
			ReducedCount: 0,
			FailedCount:  0,
			DetailCount:  detailCount,
		}

		if err != nil {
			result.FailedCount = 1
			result.Error = err
		} else if settled {
			result.SuccessCount = 1
			if reduced {
				result.ReducedCount = 1
			}
		}

		p.results <- result
	}
}

// ISettlementService 结算服务接口
type ISettlementService interface {
	// FinalSettlement 执行结算汇总
	FinalSettlement(ctx context.Context, recordTime time.Time) error
}

// settlementService 结算服务实现
type settlementService struct {
	settlementRepo rewardRepo.ISettlementRepository
	expireService  IExpireService
}

// IExpireService 出局检查服务接口（避免循环依赖）
type IExpireService interface {
	CheckAndHandleExpire(ctx context.Context, tx gdb.TX, userID int64) error
}

// NewSettlementService 创建结算服务实例
func NewSettlementService() ISettlementService {
	return &settlementService{
		settlementRepo: rewardRepo.NewSettlementRepository(),
	}
}

// SetExpireService 设置出局检查服务（用于避免循环依赖）
func (s *settlementService) SetExpireService(expireService IExpireService) {
	s.expireService = expireService
}

// FinalSettlement 执行结算汇总 (业务逻辑)
func (s *settlementService) FinalSettlement(ctx context.Context, date time.Time) error {
	g.Log().Infof(ctx, "[统一结算] 开始执行统一结算: date=%s", date.Format(consts.TimeFormatDate))
	startTime := time.Now()

	// 1. 查询所有待结算的资金记录
	allRecords, err := s.settlementRepo.GetPendingByDate(ctx, date)
	if err != nil {
		return gerror.Wrap(err, "查询待结算资金记录失败")
	}

	if len(allRecords) == 0 {
		g.Log().Info(ctx, "[统一结算] 无待结算记录")
		return nil
	}

	g.Log().Infof(ctx, "[统一结算] 查询到待结算记录数: %d", len(allRecords))

	// 2. 批量幂等检查：查询该日期所有已结算明细
	settledMapData, err := s.settlementRepo.GetSettledMapByDate(ctx, date)
	if err != nil {
		return gerror.Wrap(err, "查询已结算明细失败")
	}

	// 创建线程安全的map
	settledMap := NewSafeSettledMap(settledMapData)
	g.Log().Infof(ctx, "[统一结算] 已结算记录数: %d", len(settledMapData))

	// 3. 批量查询该日期的所有结算记录（用于幂等检查）
	settlementRecordMapData, err := s.settlementRepo.GetSettlementRecordMapByDate(ctx, date)
	if err != nil {
		return gerror.Wrap(err, "批量查询结算记录失败")
	}

	// 创建线程安全的map
	settlementRecordMap := NewSafeSettlementRecordMap(settlementRecordMapData)
	g.Log().Infof(ctx, "[统一结算] 已有结算记录用户数: %d", len(settlementRecordMapData))

	// 4. 过滤已结算的记录
	unsettledRecords := make([]*rewardEntity.AssetRecordEntity, 0)
	skipCount := 0

	for _, record := range allRecords {
		// 只处理收入记录
		if record.FlowType != consts.AssetFlowTypeIncome {
			continue
		}

		// 跳过系统账户
		if record.UserID == consts.SystemUserID {
			continue
		}

		// 幂等检查：构建 key 并检查是否已结算
		key := fmt.Sprintf("asset_record:%d", record.Id)
		if settledMap.Get(key) {
			skipCount++
			continue // 已结算，跳过
		}

		unsettledRecords = append(unsettledRecords, record)
	}

	g.Log().Infof(ctx, "[统一结算] 跳过已结算: %d, 待结算: %d", skipCount, len(unsettledRecords))

	if len(unsettledRecords) == 0 {
		g.Log().Info(ctx, "[统一结算] 所有记录已结算，无需处理")
		return nil
	}

	// 5. 按用户分组汇总
	userAssetsMap := make(map[int64]*UserAssetPending)

	for _, record := range unsettledRecords {
		if _, exists := userAssetsMap[record.UserID]; !exists {
			userAssetsMap[record.UserID] = &UserAssetPending{
				UserID:      record.UserID,
				Date:        date,
				TotalAmount: decimal.Zero,
				RecordIDs:   make([]int64, 0),
				Records:     make([]*rewardEntity.AssetRecordEntity, 0),
			}
		}

		userAssetsMap[record.UserID].TotalAmount = userAssetsMap[record.UserID].TotalAmount.Add(record.Amount)
		userAssetsMap[record.UserID].RecordIDs = append(userAssetsMap[record.UserID].RecordIDs, record.Id)
		userAssetsMap[record.UserID].Records = append(userAssetsMap[record.UserID].Records, record)
	}

	g.Log().Infof(ctx, "[统一结算] 涉及用户数: %d", len(userAssetsMap))

	// 6. 协程池并发结算优化：使用协程池并发处理用户结算
	successCount := 0
	failedCount := 0
	reducedCount := 0
	totalDetailCount := 0

	// 将map转为slice（便于协程池处理）
	userAssetList := make([]*UserAssetPending, 0, len(userAssetsMap))
	for _, userAsset := range userAssetsMap {
		userAssetList = append(userAssetList, userAsset)
	}

	// 创建协程池（协程数量根据用户数量动态调整，最多50个协程）
	workerCount := len(userAssetList)
	if workerCount > 50 {
		workerCount = 50
	}
	if workerCount < 1 {
		workerCount = 1
	}

	pool := NewWorkerPool(workerCount, len(userAssetList))
	g.Log().Infof(ctx, "[统一结算] 启动协程池: 协程数=%d, 用户数=%d", workerCount, len(userAssetList))

	// 启动协程池
	pool.Start(ctx, s.settlementRepo, settlementRecordMap, settledMap)

	// 添加所有任务到协程池
	for _, userAsset := range userAssetList {
		pool.AddJob(userAsset)
	}

	// 关闭协程池并等待所有任务完成
	pool.Close()

	// 收集结果
	for result := range pool.results {
		successCount += result.SuccessCount
		reducedCount += result.ReducedCount
		failedCount += result.FailedCount
		totalDetailCount += result.DetailCount

		if result.Error != nil {
			g.Log().Errorf(ctx, "[统一结算-协程] 结算错误: %v", result.Error)
		}
	}

	duration := time.Since(startTime)
	g.Log().Infof(ctx, "[统一结算] 结算完成: 总记录=%d, 跳过=%d, 涉及用户=%d, 成功=%d, 削减=%d, 失败=%d, 明细数=%d, 耗时=%v",
		len(allRecords), skipCount, len(userAssetsMap), successCount, reducedCount, failedCount, totalDetailCount, duration)

	return nil
}

// settleUserAssets 结算单个用户的所有资产 (业务逻辑)
func (p *WorkerPool) settleUserAssets(userAsset *UserAssetPending) (settled bool, reduced bool, detailCount int, err error) {
	// 在事务中处理
	err = g.DB().Transaction(p.ctx, func(ctx context.Context, tx gdb.TX) error {
		// 1. 从预查询的map中获取或创建结算记录
		settlementRecord := p.settlementRecordMap.Get(userAsset.UserID)

		if settlementRecord == nil {
			// 不存在则创建
			settlementRecord = &rewardEntity.SettlementRecordEntity{
				UserID:           userAsset.UserID,
				SettlementTime:   userAsset.Date,
				SettlementAmount: decimal.Zero,
				DetailCount:      0,
			}
			if err := p.settlementRepo.CreateSettlementRecord(ctx, tx, settlementRecord); err != nil {
				return gerror.Wrap(err, "创建结算记录失败")
			}
			// 创建后加入map，避免同一批次的其他结算重复创建
			p.settlementRecordMap.Set(userAsset.UserID, settlementRecord)
		}

		// 2. 查询用户额度（加悲观锁）
		// 注意：服务中心奖励不占额度，如果只有服务中心奖励，可以没有额度记录
		// 所以先查询额度，但暂时不创建（等后面判断是否有其他奖励再决定是否需要创建）
		quota, err := p.settlementRepo.GetQuotaForUpdate(ctx, tx, userAsset.UserID)
		if err != nil {
			return gerror.Wrap(err, "查询用户额度失败")
		}

		// 3. 创建结算明细（记录每笔来源）
		// 3.1 过滤已存在的记录（使用事务外预查询的settledMap，双重保护）
		// 区分服务中心奖励和其他奖励
		settlementDetails := make([]*rewardEntity.SettlementDetailEntity, 0, len(userAsset.Records))
		skipInTxCount := 0
		serviceCenterRewardAmount := decimal.Zero // 服务中心奖励总额（全额发放）
		otherRewardAmount := decimal.Zero         // 其他奖励总额（参与削减计算）
		staticRewardAmount := decimal.Zero        // 静态奖励总额（用于累加 released_static）

		for _, record := range userAsset.Records {
			// 构建 key 并检查是否已结算（使用事务外的批量查询结果）
			key := fmt.Sprintf("asset_record:%d", record.Id)
			if p.settledMap.Get(key) {
				skipInTxCount++
				continue // 已存在，跳过（不应该发生，但做双重保护）
			}

			// 区分服务中心奖励和其他奖励
			isServiceCenterReward := record.BusinessType == consts.AssetBusinessTypeRewardServiceCenter
			if isServiceCenterReward {
				// 服务中心奖励：全额累加，不参与削减计算
				serviceCenterRewardAmount = serviceCenterRewardAmount.Add(record.Amount)
			} else {
				// 其他奖励：参与削减计算
				otherRewardAmount = otherRewardAmount.Add(record.Amount)
			}

			// 统计静态奖励金额（用于累加 released_static）
			if record.BusinessType == consts.AssetBusinessTypeRewardStatic {
				staticRewardAmount = staticRewardAmount.Add(record.Amount)
			}

			detail := &rewardEntity.SettlementDetailEntity{
				SettlementID:   settlementRecord.Id,
				UserID:         userAsset.UserID,
				SettlementTime: userAsset.Date,
				SourceTable:    "asset_record",
				SourceID:       record.Id,
				Amount:         record.Amount,
				BusinessType:   record.BusinessType,
				Status:         consts.AssetRecordStatusSettled, // 2-已结算
			}
			settlementDetails = append(settlementDetails, detail)
		}

		if skipInTxCount > 0 {
			g.Log().Warningf(ctx, "[统一结算] 事务内检测到已结算记录: userID=%d, 跳过=%d", userAsset.UserID, skipInTxCount)
		}

		if len(settlementDetails) == 0 {
			g.Log().Infof(ctx, "[统一结算] 用户所有记录已结算: userID=%d", userAsset.UserID)
			return nil // 所有记录已结算，跳过
		}

		// 3.2 批量创建结算明细
		if err := p.settlementRepo.BatchCreateSettlementDetails(ctx, tx, settlementDetails); err != nil {
			return gerror.Wrap(err, "批量创建结算明细失败")
		}

		detailCount = len(settlementDetails)

		// 4. 检查额度是否充足（只基于其他奖励，排除服务中心奖励）
		reducedAmount := decimal.Zero
		actualOtherRewardAmount := otherRewardAmount                          // 其他奖励的实际发放金额
		totalRewardAmount := serviceCenterRewardAmount.Add(otherRewardAmount) // 总奖励金额（服务中心全额+其他奖励全额）

		// 如果只有服务中心奖励，不需要检查额度（服务中心奖励不占额度）
		if otherRewardAmount.GreaterThan(decimal.Zero) {
			// 有其他奖励，需要检查额度
			if quota == nil {
				// 额度记录不存在，需要先创建
				quota, err = p.settlementRepo.GetQuotaOrCreate(ctx, tx, userAsset.UserID)
				if err != nil {
					return gerror.Wrap(err, "创建用户额度失败")
				}
				g.Log().Infof(ctx, "[统一结算] 自动创建用户额度记录: userID=%d", userAsset.UserID)
				// 创建后需要重新锁定查询（确保后续操作的一致性）
				quota, err = p.settlementRepo.GetQuotaForUpdate(ctx, tx, userAsset.UserID)
				if err != nil {
					return gerror.Wrap(err, "重新查询用户额度失败")
				}
				if quota == nil {
					return gerror.Newf("创建用户额度后查询失败: userID=%d", userAsset.UserID)
				}
			}

			if otherRewardAmount.GreaterThan(quota.RemainingQuota) {
				// 额度不足，需要削减（只削减其他奖励，服务中心奖励不受影响）
				g.Log().Warningf(ctx, "[统一结算] 用户额度不足需削减: userID=%d, 其他奖励应发=%s, 剩余额度=%s, 服务中心奖励=%s(全额)",
					userAsset.UserID, otherRewardAmount.String(), quota.RemainingQuota.String(), serviceCenterRewardAmount.String())

				reducedAmount = otherRewardAmount.Sub(quota.RemainingQuota)
				actualOtherRewardAmount = quota.RemainingQuota
				reduced = true
			} else {
				// 额度充足，所有奖励全额发放
				if serviceCenterRewardAmount.GreaterThan(decimal.Zero) {
					g.Log().Infof(ctx, "[统一结算] 用户额度充足: userID=%d, 其他奖励=%s, 服务中心奖励=%s(全额)",
						userAsset.UserID, otherRewardAmount.String(), serviceCenterRewardAmount.String())
				}
			}
		} else if serviceCenterRewardAmount.GreaterThan(decimal.Zero) {
			// 只有服务中心奖励，无需检查额度，全额发放
			g.Log().Infof(ctx, "[统一结算] 用户仅服务中心奖励: userID=%d, 服务中心奖励=%s(全额，不占额度)",
				userAsset.UserID, serviceCenterRewardAmount.String())
		}

		// 5. 更新结算记录（累加实际处理的奖励金额和明细数）
		// 注意：这里先记录完整奖励金额，削减金额会在后面单独处理
		settlementRecord.SettlementAmount = settlementRecord.SettlementAmount.Add(totalRewardAmount)
		settlementRecord.DetailCount += detailCount
		if err := p.settlementRepo.UpdateSettlementRecord(ctx, tx, settlementRecord); err != nil {
			return gerror.Wrap(err, "更新结算记录失败")
		}

		// 6. 更新所有待结算记录状态为已结算（只更新实际处理的）
		actualRecordIDs := make([]int64, 0, len(settlementDetails))
		for _, detail := range settlementDetails {
			actualRecordIDs = append(actualRecordIDs, detail.SourceID)
		}
		if err := p.settlementRepo.UpdateAssetRecordStatus(ctx, tx, actualRecordIDs, consts.AssetRecordStatusSettled); err != nil {
			return gerror.Wrap(err, "更新资金记录状态失败")
		}

		// 7. 增加用户可用余额（先增加完整奖励金额）
		apgUserBalanceID := p.settlementRepo.GetAPGUserBalanceAccountTypeID(ctx)
		orderNo := fmt.Sprintf("SETTLE_%s_%d", userAsset.Date.Format("20060102"), userAsset.UserID)

		if err := p.settlementRepo.DepositBalance(ctx, tx, &rewardRepo.BalanceDepositParams{
			UserID:         userAsset.UserID,
			AccountTypeID:  apgUserBalanceID,
			Symbol:         "APG",
			Amount:         totalRewardAmount, // 先增加完整奖励金额
			ChangeType:     consts.ChangeTypeRewardSettlement,
			RelatedOrderNo: orderNo,
			RelatedId:      settlementRecord.Id,
			Remark:         fmt.Sprintf("奖励结算-可用余额（%s）", userAsset.Date.Format(consts.TimeFormatDate)),
			OperatorType:   consts.OperatorTypeSystem,
		}); err != nil {
			return gerror.Wrap(err, "增加可用余额失败")
		}

		// 8. 如果有削减，创建削减记录、结算明细并扣除余额（只针对其他奖励）
		if reducedAmount.GreaterThan(decimal.Zero) {
			// 8.1 创建削减记录（负值金额）- 削减金额只针对其他奖励
			reducedRecord, err := p.createReducedRecord(ctx, tx, userAsset.UserID, userAsset.Date, otherRewardAmount, actualOtherRewardAmount, reducedAmount)
			if err != nil {
				return gerror.Wrap(err, "创建削减记录失败")
			}

			// 8.2 为削减记录创建结算明细（完整追溯）
			reducedDetail := &rewardEntity.SettlementDetailEntity{
				SettlementID:   settlementRecord.Id,
				UserID:         userAsset.UserID,
				SettlementTime: userAsset.Date,
				SourceTable:    "asset_record",
				SourceID:       reducedRecord.Id,     // 削减记录的ID
				Amount:         reducedRecord.Amount, // 负值
				BusinessType:   consts.AssetBusinessTypeRewardReduced,
				Status:         consts.AssetRecordStatusSettled, // 2-已结算
			}
			if err := p.settlementRepo.CreateSettlementDetail(ctx, tx, reducedDetail); err != nil {
				return gerror.Wrap(err, "创建削减结算明细失败")
			}

			// 8.3 更新结算记录（累加削减金额和明细数）
			// 削减金额是负值，累加后自动减少settlement_amount
			settlementRecord.SettlementAmount = settlementRecord.SettlementAmount.Add(reducedRecord.Amount) // 负值累加
			settlementRecord.DetailCount += 1
			if err := p.settlementRepo.UpdateSettlementRecord(ctx, tx, settlementRecord); err != nil {
				return gerror.Wrap(err, "更新结算记录失败（削减）")
			}

			// 8.4 从余额扣除超出额度的部分
			if err := p.settlementRepo.WithdrawBalance(ctx, tx, &rewardRepo.BalanceWithdrawParams{
				UserID:         userAsset.UserID,
				AccountTypeID:  apgUserBalanceID,
				Symbol:         "APG",
				Amount:         reducedAmount,
				ChangeType:     consts.ChangeTypeRewardReduced,
				RelatedOrderNo: orderNo,
				RelatedId:      reducedRecord.Id,
				Remark:         fmt.Sprintf("额度不足-奖励削减（%s）", userAsset.Date.Format(consts.TimeFormatDate)),
				OperatorType:   consts.OperatorTypeSystem,
			}); err != nil {
				return gerror.Wrap(err, "扣除削减金额失败")
			}

			detailCount += 1 // 削减记录也算一条明细
		}

		// 9. 扣除用户额度并更新统计字段（只扣除其他奖励的额度，服务中心奖励不占额度）
		// 计算实际发放的静态奖励金额（如果被削减，需要按比例计算）
		actualStaticRewardAmount := decimal.Zero
		if staticRewardAmount.GreaterThan(decimal.Zero) && otherRewardAmount.GreaterThan(decimal.Zero) {
			// 如果有削减，按比例计算实际发放的静态奖励金额
			if reducedAmount.GreaterThan(decimal.Zero) {
				// 削减比例 = 实际发放金额 / 应发金额
				ratio := actualOtherRewardAmount.Div(otherRewardAmount)
				actualStaticRewardAmount = staticRewardAmount.Mul(ratio)
			} else {
				// 没有削减，全额发放
				actualStaticRewardAmount = staticRewardAmount
			}
		} else if staticRewardAmount.GreaterThan(decimal.Zero) {
			// 只有静态奖励，全额发放
			actualStaticRewardAmount = staticRewardAmount
		}

		// 计算实际发放的总奖励金额（服务中心奖励全额 + 其他奖励实际发放金额）
		actualTotalRewardAmount := serviceCenterRewardAmount.Add(actualOtherRewardAmount)

		if actualOtherRewardAmount.GreaterThan(decimal.Zero) {
			// 有其他奖励，需要扣除额度并更新统计字段
			changeRemark := fmt.Sprintf("奖励结算扣除额度（%s），实发金额 %s USDT",
				userAsset.Date.Format(consts.TimeFormatDate), actualOtherRewardAmount.String())
			if reducedAmount.GreaterThan(decimal.Zero) {
				changeRemark = fmt.Sprintf("奖励结算扣除额度（%s），应发 %s USDT，实发 %s USDT，削减 %s USDT（服务中心奖励 %s 全额不占额度）",
					userAsset.Date.Format(consts.TimeFormatDate), otherRewardAmount.String(), actualOtherRewardAmount.String(), reducedAmount.String(), serviceCenterRewardAmount.String())
			} else if serviceCenterRewardAmount.GreaterThan(decimal.Zero) {
				changeRemark = fmt.Sprintf("奖励结算扣除额度（%s），实发 %s USDT（服务中心奖励 %s 全额不占额度）",
					userAsset.Date.Format(consts.TimeFormatDate), actualOtherRewardAmount.String(), serviceCenterRewardAmount.String())
			}

			if err := p.settlementRepo.DeductQuota(ctx, tx, userAsset.UserID, actualOtherRewardAmount, consts.QuotaChangeTypeDeduct, consts.QuotaRewardTypeReleaseQuota, changeRemark, settlementRecord.Id, actualStaticRewardAmount, actualTotalRewardAmount); err != nil {
				return gerror.Wrap(err, "扣除额度失败")
			}

			// g.Log().Debugf(ctx, "[统一结算] 扣除额度: userID=%d, 其他奖励扣除=%s, 服务中心奖励=%s(全额不占额度)",
			// 	userAsset.UserID, actualOtherRewardAmount.String(), serviceCenterRewardAmount.String())
		} else if serviceCenterRewardAmount.GreaterThan(decimal.Zero) {
			// 只有服务中心奖励，无需扣除额度，但需要更新 total_income
			// 需要先获取或创建额度记录
			if quota == nil {
				quota, err = p.settlementRepo.GetQuotaOrCreate(ctx, tx, userAsset.UserID)
				if err != nil {
					return gerror.Wrap(err, "创建用户额度失败")
				}
			}
			// 只更新 total_income，不扣除额度
			if err := p.settlementRepo.DeductQuota(ctx, tx, userAsset.UserID, decimal.Zero, consts.QuotaChangeTypeDeduct, consts.QuotaRewardTypeReleaseQuota, fmt.Sprintf("奖励结算（%s，仅服务中心奖励，不占额度）", userAsset.Date.Format(consts.TimeFormatDate)), settlementRecord.Id, decimal.Zero, actualTotalRewardAmount); err != nil {
				return gerror.Wrap(err, "更新总收益失败")
			}
			g.Log().Debugf(ctx, "[统一结算] 无需扣除额度: userID=%d（仅服务中心奖励=%s，不占额度）", userAsset.UserID, serviceCenterRewardAmount.String())
		}

		settled = true

		// 计算总应发金额（用于日志）
		// totalShouldAmount := serviceCenterRewardAmount.Add(otherRewardAmount)
		//
		// actualTotalAmount := serviceCenterRewardAmount.Add(actualOtherRewardAmount) // 实际发放金额
		// g.Log().Infof(ctx, "[统一结算] 用户结算成功: userID=%d, 总应发=%s(服务中心=%s全额+其他=%s), 实发=%s, 削减=%s(仅其他奖励), 明细数=%d",
		// 	userAsset.UserID, totalShouldAmount.String(), serviceCenterRewardAmount.String(), otherRewardAmount.String(),
		// 	actualTotalAmount.String(), reducedAmount.String(), detailCount)

		return nil
	})

	return settled, reduced, detailCount, err
}

// createReducedRecord 创建削减记录（负值金额，表示扣减），返回创建的记录 (业务逻辑)
func (p *WorkerPool) createReducedRecord(ctx context.Context, tx gdb.TX, userID int64, date time.Time, originalAmount, actualAmount, reducedAmount decimal.Decimal) (*rewardEntity.AssetRecordEntity, error) {
	// 构建削减元数据
	reducedMetadata := rewardEntity.RewardReducedMetadata{
		OriginalAmount: originalAmount,
		ActualAmount:   actualAmount,
		ReducedAmount:  reducedAmount,
		ReduceRatio:    reducedAmount.DivRound(originalAmount, consts.TokenPrecision),
	}
	metadataJSON, _ := json.Marshal(reducedMetadata)

	// 创建削减记录（负值金额 + 支出类型）
	record := &rewardEntity.AssetRecordEntity{
		UserID:       userID,
		AssetType:    consts.AssetTypeAPGUserBalance,
		RecordTime:   date,
		Amount:       reducedAmount.Neg(),         // 负值金额（表示扣减）
		FlowType:     consts.AssetFlowTypeExpense, // 支出（削减）
		Status:       consts.AssetRecordStatusSettled,
		BusinessType: consts.AssetBusinessTypeRewardReduced,
		BusinessID:   0,
		Remark:       "额度不足-奖励削减",
		Metadata:     string(metadataJSON),
	}

	if err := p.settlementRepo.CreateAssetRecord(ctx, tx, record); err != nil {
		return nil, err
	}

	return record, nil
}
