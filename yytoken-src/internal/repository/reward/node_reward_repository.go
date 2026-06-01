package reward

import (
	"context"
	"time"

	"XWFrame/internal/dao"
	rewardDao "XWFrame/internal/dao/reward"
	"XWFrame/internal/entity/cobo"
	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/consts"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// INodeRewardRepository 节点分红仓储接口（仅提供数据访问）
type INodeRewardRepository interface {
	// GetLastNodeDividendTime 获取上次节点分红时间
	GetLastNodeDividendTime(ctx context.Context) (time.Time, error)

	// GetAPGBurnTotalByDateRange 查询时间范围内的APG销毁总额（已废弃）
	GetAPGBurnTotalByDateRange(ctx context.Context, startTime, endTime time.Time) (decimal.Decimal, error)

	// GetUsdtTransferTotalByDateRange 查询时间范围内的国库USDT转账总额（新）
	GetUsdtTransferTotalByDateRange(ctx context.Context, startTime, endTime time.Time) (decimal.Decimal, error)

	// GetWithdrawFeeTotalByDateRange 查询时间范围内提现手续费总额
	GetWithdrawFeeTotalByDateRange(ctx context.Context, startTime, endTime time.Time) (decimal.Decimal, error)

	// GetNodeEquityUsers 获取所有节点权益持有者
	GetNodeEquityUsers(ctx context.Context) (map[int64]decimal.Decimal, error)

	// BatchGetQuotas 批量获取用户额度
	BatchGetQuotas(ctx context.Context, userIDs []int64) (map[int64]*rewardEntity.UserQuotaEntity, error)

	// BatchCheckExists 批量检查幂等性
	BatchCheckExists(ctx context.Context, userIDs []int64, date time.Time) (map[int64]bool, error)

	// BatchCheckExistsByBusiness 批量检查幂等性（指定业务类型）
	BatchCheckExistsByBusiness(ctx context.Context, userIDs []int64, date time.Time, businessType string) (map[int64]bool, error)

	// GetLastRecordTimeByBusinessType 获取指定业务类型最后一次记录时间
	GetLastRecordTimeByBusinessType(ctx context.Context, businessType string) (time.Time, error)

	// BatchCreateAssetRecords 批量创建资产记录
	BatchCreateAssetRecords(ctx context.Context, records []*rewardEntity.AssetRecordEntity) error
}

// nodeRewardRepository 节点分红仓储实现
type nodeRewardRepository struct {
	stakingDao      rewardDao.IStakingPackageDao
	quotaRepo       IQuotaRepository
	assetRecordDao  rewardDao.IAssetRecordDao
	exchangeDao     dao.IExchangeDao
	usdtTransferDao dao.INodeDividendUsdtTransferDao
}

// NewNodeRewardRepository 创建节点分红仓储实例
func NewNodeRewardRepository() INodeRewardRepository {
	return &nodeRewardRepository{
		stakingDao:      rewardDao.NewStakingPackageDao(),
		quotaRepo:       NewQuotaRepository(),
		assetRecordDao:  rewardDao.NewAssetRecordDao(),
		exchangeDao:     dao.NewExchangeDao(),
		usdtTransferDao: dao.NewNodeDividendUsdtTransferDao(),
	}
}

// GetLastNodeDividendTime 获取上次节点分红时间
func (r *nodeRewardRepository) GetLastNodeDividendTime(ctx context.Context) (time.Time, error) {
	return r.exchangeDao.GetLastNodeDividendTime(ctx)
}

// GetAPGBurnTotalByDateRange 查询时间范围内的APG销毁总额（已废弃）
func (r *nodeRewardRepository) GetAPGBurnTotalByDateRange(ctx context.Context, startTime, endTime time.Time) (decimal.Decimal, error) {
	return r.exchangeDao.GetAPGBurnTotalByDateRange(ctx, startTime, endTime)
}

// GetUsdtTransferTotalByDateRange 查询时间范围内的国库USDT转账总额
func (r *nodeRewardRepository) GetUsdtTransferTotalByDateRange(ctx context.Context, startTime, endTime time.Time) (decimal.Decimal, error) {
	return r.usdtTransferDao.SumAmountByDateRange(ctx, startTime, endTime)
}

// GetWithdrawFeeTotalByDateRange 查询时间范围内提现手续费总额
func (r *nodeRewardRepository) GetWithdrawFeeTotalByDateRange(ctx context.Context, startTime, endTime time.Time) (decimal.Decimal, error) {
	query := g.DB().Model("cobo_withdraw_request").
		Ctx(ctx).
		Fields("COALESCE(SUM(fee_amount), 0) as total").
		Where("status = ?", cobo.WithdrawStatusSuccess).
		Where("fee_amount > 0").
		Where("updated_at <= ?", endTime)

	if !startTime.IsZero() {
		query = query.Where("updated_at > ?", startTime)
	}

	v, err := query.Value()
	if err != nil {
		return decimal.Zero, err
	}
	if v.IsNil() || v.String() == "" {
		return decimal.Zero, nil
	}

	amount, err := decimal.NewFromString(v.String())
	if err != nil {
		return decimal.Zero, err
	}
	return amount, nil
}

// GetNodeEquityUsers 获取所有节点权益持有者
func (r *nodeRewardRepository) GetNodeEquityUsers(ctx context.Context) (map[int64]decimal.Decimal, error) {
	return r.stakingDao.GetNodeEquityUsers(ctx)
}

// BatchGetQuotas 批量获取用户额度
func (r *nodeRewardRepository) BatchGetQuotas(ctx context.Context, userIDs []int64) (map[int64]*rewardEntity.UserQuotaEntity, error) {
	return r.quotaRepo.GetByUserIDs(ctx, userIDs)
}

// BatchCheckExists 批量检查幂等性
func (r *nodeRewardRepository) BatchCheckExists(ctx context.Context, userIDs []int64, date time.Time) (map[int64]bool, error) {
	return r.assetRecordDao.BatchCheckExists(ctx, userIDs, date, consts.AssetBusinessTypeRewardNode)
}

// BatchCheckExistsByBusiness 批量检查幂等性（指定业务类型）
func (r *nodeRewardRepository) BatchCheckExistsByBusiness(ctx context.Context, userIDs []int64, date time.Time, businessType string) (map[int64]bool, error) {
	return r.assetRecordDao.BatchCheckExists(ctx, userIDs, date, businessType)
}

// GetLastRecordTimeByBusinessType 获取指定业务类型最后一次记录时间
func (r *nodeRewardRepository) GetLastRecordTimeByBusinessType(ctx context.Context, businessType string) (time.Time, error) {
	return r.assetRecordDao.GetLastRecordTimeByBusinessType(ctx, businessType)
}

// BatchCreateAssetRecords 批量创建资产记录
func (r *nodeRewardRepository) BatchCreateAssetRecords(ctx context.Context, records []*rewardEntity.AssetRecordEntity) error {
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		return r.assetRecordDao.BatchCreate(ctx, tx, records)
	})
}
