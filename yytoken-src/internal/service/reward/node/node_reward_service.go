package node

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/consts"
	coboRepo "XWFrame/internal/repository/cobo"
	rewardRepo "XWFrame/internal/repository/reward"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// INodeRewardService 节点分红服务接口
type INodeRewardService interface {
	// DistributeNodeReward 发放节点分红
	DistributeNodeReward(ctx context.Context, date time.Time) error

	// DistributeFounderFeeDividend 发放创始股手续费分红
	DistributeFounderFeeDividend(ctx context.Context, date time.Time) error
}

// nodeRewardService 节点分红服务实现
type nodeRewardService struct {
	nodeRewardRepo rewardRepo.INodeRewardRepository
	balanceRepo    coboRepo.IBalanceRepository
}

// NewNodeRewardService 创建节点分红服务实例
func NewNodeRewardService() INodeRewardService {
	return &nodeRewardService{
		nodeRewardRepo: rewardRepo.NewNodeRewardRepository(),
		balanceRepo:    coboRepo.NewBalanceRepository(),
	}
}

// DistributeNodeReward 发放节点分红（业务逻辑层）
func (s *nodeRewardService) DistributeNodeReward(ctx context.Context, date time.Time) error {
	g.Log().Infof(ctx, "[NodeReward] 开始发放节点分红，日期: %s", date.Format(consts.TimeFormatDate))

	// 1. 查询上次节点分红时间
	lastDividendTime, err := s.nodeRewardRepo.GetLastNodeDividendTime(ctx)
	if err != nil {
		g.Log().Errorf(ctx, "[NodeReward] 查询上次节点分红时间失败: %v", err)
		return err
	}

	// 2. 计算查询时间范围（业务逻辑）
	var startTime time.Time
	if lastDividendTime.IsZero() {
		startTime = time.Time{} // 零时间，表示查询全部
		g.Log().Info(ctx, "[NodeReward] 首次节点分红，查询当前时间之前的所有国库USDT转账记录")
	} else {
		startTime = lastDividendTime
		g.Log().Infof(ctx, "[NodeReward] 上次节点分红时间: %s，查询从上次分红到当前时间的国库USDT转账记录", lastDividendTime.Format(consts.TimeFormatDate))
	}

	endTime := date

	// 3. 查询时间范围内的国库USDT转账总额（新）
	dividendPool, err := s.nodeRewardRepo.GetUsdtTransferTotalByDateRange(ctx, startTime, endTime)
	if err != nil {
		g.Log().Errorf(ctx, "[NodeReward] 查询国库USDT转账总额失败: %v", err)
		return err
	}

	if dividendPool.IsZero() {
		g.Log().Info(ctx, "[NodeReward] 时间范围内国库USDT转账总额为0，分红池为0，跳过节点分红发放")
		return nil
	}

	g.Log().Infof(ctx, "[NodeReward] 时间范围: %s 到 %s，国库USDT转账总额（分红池）: %s",
		startTime.Format(consts.TimeFormatDate), endTime.Format(consts.TimeFormatDate), dividendPool.String())

	// 4. 查询所有节点权益持有者
	userEquityMap, err := s.nodeRewardRepo.GetNodeEquityUsers(ctx)
	if err != nil {
		g.Log().Errorf(ctx, "[NodeReward] 查询节点权益持有者失败: %v", err)
		return err
	}

	if len(userEquityMap) == 0 {
		g.Log().Info(ctx, "[NodeReward] 没有节点权益持有者，跳过节点分红发放")
		return nil
	}

	// 5. 业务逻辑：计算全网总节点权益
	totalEquity := decimal.Zero
	for _, equity := range userEquityMap {
		totalEquity = totalEquity.Add(equity)
	}

	if totalEquity.IsZero() {
		g.Log().Info(ctx, "[NodeReward] 全网总节点权益为0，跳过节点分红发放")
		return nil
	}

	g.Log().Infof(ctx, "[NodeReward] 找到 %d 个节点权益持有者，总权益: %s", len(userEquityMap), totalEquity.String())

	// 6. 批量获取用户额度和检查幂等性
	userIDs := make([]int64, 0, len(userEquityMap))
	for userID := range userEquityMap {
		userIDs = append(userIDs, userID)
	}

	quotaMap, existsMap, err := s.batchGetQuotasAndCheckIdempotency(ctx, userIDs, date)
	if err != nil {
		return err
	}

	// 7. 计算并批量处理节点分红
	var allAssetRecords []*rewardEntity.AssetRecordEntity
	totalProcessed := 0
	totalReward := decimal.Zero

	for userID, equityValue := range userEquityMap {
		// 检查用户额度
		quota, exists := quotaMap[userID]
		if !exists || quota == nil || quota.RemainingQuota.LessThanOrEqual(decimal.Zero) {
			g.Log().Debugf(ctx, "[NodeReward] 用户 %d 已出局（额度不足），跳过", userID)
			continue
		}

		// 检查幂等性
		if existsMap[userID] {
			g.Log().Debugf(ctx, "[NodeReward] 用户 %d 在时刻 %s 的节点分红已发放，跳过", userID, date.Format(consts.TimeFormatDateTime))
			continue
		}

		// 业务逻辑：计算权益比例和分红
		equityRatio := equityValue.DivRound(totalEquity, consts.TokenPrecision)
		nodeReward := dividendPool.Mul(equityRatio)

		if nodeReward.LessThanOrEqual(decimal.Zero) {
			continue
		}

		// 检查剩余额度限制
		if nodeReward.GreaterThan(quota.RemainingQuota) {
			nodeReward = quota.RemainingQuota
		}

		if nodeReward.LessThanOrEqual(decimal.Zero) {
			continue
		}

		// 创建资金记录
		assetRecord := s.createAssetRecord(userID, nodeReward, equityValue, totalEquity, dividendPool, equityRatio, date, startTime, endTime)
		allAssetRecords = append(allAssetRecords, assetRecord)

		totalProcessed++
		totalReward = totalReward.Add(nodeReward)
	}

	if len(allAssetRecords) == 0 {
		g.Log().Info(ctx, "[NodeReward] 没有需要发放的节点分红")
		return nil
	}

	// 8. 批量执行数据库操作
	if err := s.nodeRewardRepo.BatchCreateAssetRecords(ctx, allAssetRecords); err != nil {
		g.Log().Errorf(ctx, "[NodeReward] 批量执行节点分红失败: %v", err)
		return err
	}

	g.Log().Infof(ctx, "[NodeReward] 节点分红发放完成 - 处理用户: %d, 发放金额: %s",
		totalProcessed, totalReward.String())

	return nil
}

// DistributeFounderFeeDividend 发放创始股手续费分红（手续费池50%）
func (s *nodeRewardService) DistributeFounderFeeDividend(ctx context.Context, date time.Time) error {
	g.Log().Infof(ctx, "[FounderFeeDividend] 开始发放创始股手续费分红，日期: %s", date.Format(consts.TimeFormatDate))

	lastTime, err := s.nodeRewardRepo.GetLastRecordTimeByBusinessType(ctx, consts.AssetBusinessTypeFounderFeeDividend)
	if err != nil {
		g.Log().Errorf(ctx, "[FounderFeeDividend] 查询上次分红时间失败: %v", err)
		return err
	}

	feeTotal, err := s.nodeRewardRepo.GetWithdrawFeeTotalByDateRange(ctx, lastTime, date)
	if err != nil {
		g.Log().Errorf(ctx, "[FounderFeeDividend] 查询手续费总额失败: %v", err)
		return err
	}

	if feeTotal.LessThanOrEqual(decimal.Zero) {
		g.Log().Info(ctx, "[FounderFeeDividend] 时间范围内无提现手续费，跳过发放")
		return nil
	}

	poolRate := decimal.NewFromFloat(0.5)
	dividendPool := feeTotal.Mul(poolRate)
	if dividendPool.LessThanOrEqual(decimal.Zero) {
		g.Log().Info(ctx, "[FounderFeeDividend] 分红池为0，跳过发放")
		return nil
	}

	userEquityMap, err := s.nodeRewardRepo.GetNodeEquityUsers(ctx)
	if err != nil {
		g.Log().Errorf(ctx, "[FounderFeeDividend] 查询创始股权益失败: %v", err)
		return err
	}
	if len(userEquityMap) == 0 {
		g.Log().Info(ctx, "[FounderFeeDividend] 无创始股用户，跳过发放")
		return nil
	}

	totalEquity := decimal.Zero
	for _, equity := range userEquityMap {
		totalEquity = totalEquity.Add(equity)
	}
	if totalEquity.LessThanOrEqual(decimal.Zero) {
		g.Log().Info(ctx, "[FounderFeeDividend] 创始股总权益为0，跳过发放")
		return nil
	}

	userIDs := make([]int64, 0, len(userEquityMap))
	for userID := range userEquityMap {
		userIDs = append(userIDs, userID)
	}

	quotaMap, existsMap, err := s.batchGetQuotasAndCheckByBusiness(ctx, userIDs, date, consts.AssetBusinessTypeFounderFeeDividend)
	if err != nil {
		return err
	}

	records := make([]*rewardEntity.AssetRecordEntity, 0, len(userEquityMap))
	processedCount := 0
	totalReward := decimal.Zero

	for userID, equityValue := range userEquityMap {
		quota, exists := quotaMap[userID]
		if !exists || quota == nil || quota.RemainingQuota.LessThanOrEqual(decimal.Zero) {
			continue
		}

		if existsMap[userID] {
			continue
		}

		equityRatio := equityValue.DivRound(totalEquity, consts.TokenPrecision)
		rewardAmount := dividendPool.Mul(equityRatio)
		if rewardAmount.GreaterThan(quota.RemainingQuota) {
			rewardAmount = quota.RemainingQuota
		}
		if rewardAmount.LessThanOrEqual(decimal.Zero) {
			continue
		}

		metadata := map[string]interface{}{
			"fee_total":     feeTotal.String(),
			"pool_rate":     poolRate.String(),
			"dividend_pool": dividendPool.String(),
			"equity_value":  equityValue.String(),
			"total_equity":  totalEquity.String(),
			"equity_ratio":  equityRatio.String(),
			"period_start_at": func() string {
				if lastTime.IsZero() {
					return ""
				}
				return lastTime.Format(consts.TimeFormatDateTime)
			}(),
			"period_end_at": date.Format(consts.TimeFormatDateTime),
		}
		metadataJSON, _ := json.Marshal(metadata)

		records = append(records, &rewardEntity.AssetRecordEntity{
			UserID:       userID,
			AssetType:    consts.AssetTypeUSDTUserBalance,
			RecordTime:   date,
			Amount:       rewardAmount,
			FlowType:     consts.AssetFlowTypeIncome,
			Status:       consts.AssetRecordStatusSettled,
			BusinessType: consts.AssetBusinessTypeFounderFeeDividend,
			BusinessID:   0,
			Remark:       "创始股手续费分红",
			Metadata:     string(metadataJSON),
		})

		processedCount++
		totalReward = totalReward.Add(rewardAmount)
	}

	if len(records) == 0 {
		g.Log().Info(ctx, "[FounderFeeDividend] 无可发放记录，跳过")
		return nil
	}

	if err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		for _, record := range records {
			if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, record.UserID, "USDT", record.Amount, decimal.Zero); err != nil {
				return err
			}

			data := g.Map{
				"user_id":               record.UserID,
				"reward_type":           "founder_fee_dividend",
				"amount":                record.Amount,
				"symbol":                "USDT",
				"source_user_id":        0,
				"source_wallet_address": "",
				"purchase_package_no":   "",
				"purchase_amount":       "",
				"reward_rate":           "0.50",
				"metadata":              record.Metadata,
				"created_at":            time.Now(),
				"updated_at":            time.Now(),
			}
			if _, err := tx.Model("cobo_reward_record").Ctx(ctx).Data(data).Insert(); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		g.Log().Errorf(ctx, "[FounderFeeDividend] 发放并入账失败: %v", err)
		return err
	}

	g.Log().Infof(ctx, "[FounderFeeDividend] 发放完成，用户数: %d, 发放总额: %s, 手续费总额: %s",
		processedCount, totalReward.String(), feeTotal.String())

	return nil
}

// batchGetQuotasAndCheckIdempotency 批量获取用户额度和检查幂等性
func (s *nodeRewardService) batchGetQuotasAndCheckIdempotency(ctx context.Context, userIDs []int64, date time.Time) (map[int64]*rewardEntity.UserQuotaEntity, map[int64]bool, error) {
	return s.batchGetQuotasAndCheckByBusiness(ctx, userIDs, date, consts.AssetBusinessTypeRewardNode)
}

// batchGetQuotasAndCheckByBusiness 批量获取用户额度和按业务类型检查幂等性
func (s *nodeRewardService) batchGetQuotasAndCheckByBusiness(ctx context.Context, userIDs []int64, date time.Time, businessType string) (map[int64]*rewardEntity.UserQuotaEntity, map[int64]bool, error) {
	const maxBatchSize = 30000

	quotaMap := make(map[int64]*rewardEntity.UserQuotaEntity)
	existsMap := make(map[int64]bool)

	if len(userIDs) > maxBatchSize {
		// 分批处理
		for i := 0; i < len(userIDs); i += maxBatchSize {
			end := i + maxBatchSize
			if end > len(userIDs) {
				end = len(userIDs)
			}
			batch := userIDs[i:end]

			batchQuotaMap, err := s.nodeRewardRepo.BatchGetQuotas(ctx, batch)
			if err != nil {
				return nil, nil, err
			}
			for userID, quota := range batchQuotaMap {
				quotaMap[userID] = quota
			}

			batchExistsMap, err := s.nodeRewardRepo.BatchCheckExistsByBusiness(ctx, batch, date, businessType)
			if err != nil {
				return nil, nil, err
			}
			for userID, exists := range batchExistsMap {
				existsMap[userID] = exists
			}
		}
	} else {
		var err error
		quotaMap, err = s.nodeRewardRepo.BatchGetQuotas(ctx, userIDs)
		if err != nil {
			return nil, nil, err
		}

		existsMap, err = s.nodeRewardRepo.BatchCheckExistsByBusiness(ctx, userIDs, date, businessType)
		if err != nil {
			return nil, nil, err
		}
	}

	return quotaMap, existsMap, nil
}

// createAssetRecord 创建资金记录（业务逻辑）
func (s *nodeRewardService) createAssetRecord(userID int64, nodeReward, equityValue, totalEquity, dividendPool decimal.Decimal, equityRatio decimal.Decimal, date, startTime, endTime time.Time) *rewardEntity.AssetRecordEntity {
	metadata := map[string]interface{}{
		"equity_value":        equityValue.String(),
		"total_equity":        totalEquity.String(),
		"equity_ratio":        equityRatio.String(),
		"dividend_pool":       dividendPool.String(),
		"usdt_transfer_total": dividendPool.String(),
		"query_period":        fmt.Sprintf("%s to %s", startTime.Format(consts.TimeFormatDate), endTime.Format(consts.TimeFormatDate)),
	}
	metadataJSON, _ := json.Marshal(metadata)

	return &rewardEntity.AssetRecordEntity{
		UserID:       userID,
		AssetType:    consts.AssetTypeAPGUserBalance,
		RecordTime:   date,
		Amount:       nodeReward,
		FlowType:     consts.AssetFlowTypeIncome,
		Status:       consts.AssetRecordStatusPending,
		BusinessType: consts.AssetBusinessTypeRewardNode,
		BusinessID:   0,
		Remark:       "节点分红",
		Metadata:     string(metadataJSON),
	}
}
