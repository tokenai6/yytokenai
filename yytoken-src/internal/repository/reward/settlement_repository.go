package reward

import (
	"context"
	"sync"
	"time"

	"XWFrame/internal/dao"
	rewardDao "XWFrame/internal/dao/reward"
	rewardEntity "XWFrame/internal/entity/reward"
	repo "XWFrame/internal/repository"

	"github.com/gogf/gf/v2/database/gdb"
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

// ISettlementRepository 统一结算仓储接口（仅提供数据访问）
type ISettlementRepository interface {
	// GetPendingByDate 查询所有待结算的资金记录
	GetPendingByDate(ctx context.Context, date time.Time) ([]*rewardEntity.AssetRecordEntity, error)

	// GetSettledMapByDate 查询该日期所有已结算明细
	GetSettledMapByDate(ctx context.Context, date time.Time) (map[string]bool, error)

	// GetSettlementRecordMapByDate 批量查询该日期的所有结算记录
	GetSettlementRecordMapByDate(ctx context.Context, date time.Time) (map[int64]*rewardEntity.SettlementRecordEntity, error)

	// CreateSettlementRecord 创建结算记录
	CreateSettlementRecord(ctx context.Context, tx gdb.TX, record *rewardEntity.SettlementRecordEntity) error

	// UpdateSettlementRecord 更新结算记录
	UpdateSettlementRecord(ctx context.Context, tx gdb.TX, record *rewardEntity.SettlementRecordEntity) error

	// BatchCreateSettlementDetails 批量创建结算明细
	BatchCreateSettlementDetails(ctx context.Context, tx gdb.TX, details []*rewardEntity.SettlementDetailEntity) error

	// CreateSettlementDetail 创建结算明细
	CreateSettlementDetail(ctx context.Context, tx gdb.TX, detail *rewardEntity.SettlementDetailEntity) error

	// UpdateAssetRecordStatus 批量更新资产记录状态
	UpdateAssetRecordStatus(ctx context.Context, tx gdb.TX, ids []int64, status int) error

	// CreateAssetRecord 创建资产记录
	CreateAssetRecord(ctx context.Context, tx gdb.TX, record *rewardEntity.AssetRecordEntity) error

	// GetQuotaForUpdate 使用悲观锁查询用户额度
	GetQuotaForUpdate(ctx context.Context, tx gdb.TX, userID int64) (*rewardEntity.UserQuotaEntity, error)

	// GetQuotaByUserID 获取用户额度
	GetQuotaByUserID(ctx context.Context, userID int64) (*rewardEntity.UserQuotaEntity, error)

	// GetQuotaOrCreate 获取或创建用户额度
	GetQuotaOrCreate(ctx context.Context, tx gdb.TX, userID int64) (*rewardEntity.UserQuotaEntity, error)

	// GetAPGUserBalanceAccountTypeID 获取APG用户余额账户类型ID
	GetAPGUserBalanceAccountTypeID(ctx context.Context) int64

	// DepositBalance 增加用户可用余额
	DepositBalance(ctx context.Context, tx gdb.TX, params *BalanceDepositParams) error

	// WithdrawBalance 扣减用户可用余额
	WithdrawBalance(ctx context.Context, tx gdb.TX, params *BalanceWithdrawParams) error

	// DeductQuota 扣除用户额度
	DeductQuota(ctx context.Context, tx gdb.TX, userID int64, amount decimal.Decimal, changeType, rewardType, remark string, relatedID int64, staticRewardAmount, totalRewardAmount decimal.Decimal) error

	// UpdatePackageStatusToExpired 更新算力包状态为出局
	UpdatePackageStatusToExpired(ctx context.Context, tx gdb.TX, userID int64, status int) error
}

// BalanceDepositParams 余额充值参数
type BalanceDepositParams struct {
	UserID         int64
	AccountTypeID  int64
	Symbol         string
	Amount         decimal.Decimal
	ChangeType     string
	RelatedOrderNo string
	RelatedId      int64
	Remark         string
	OperatorType   string
}

// BalanceWithdrawParams 余额扣减参数
type BalanceWithdrawParams struct {
	UserID         int64
	AccountTypeID  int64
	Symbol         string
	Amount         decimal.Decimal
	ChangeType     string
	RelatedOrderNo string
	RelatedId      int64
	Remark         string
	OperatorType   string
}

// settlementRepository 统一结算仓储实现
type settlementRepository struct {
	assetRecordDao      rewardDao.IAssetRecordDao
	quotaRepo           IQuotaRepository
	stakingDao          rewardDao.IStakingPackageDao
	userDao             dao.IUserDao
	balanceRepo         repo.IBalanceRepository
	accountTypeRepo     repo.IAccountTypeRepository
	settlementRecordDao rewardDao.ISettlementRecordDao
	settlementDetailDao rewardDao.ISettlementDetailDao
}

// NewSettlementRepository 创建统一结算仓储实例
func NewSettlementRepository() ISettlementRepository {
	return &settlementRepository{
		assetRecordDao:      rewardDao.NewAssetRecordDao(),
		quotaRepo:           NewQuotaRepository(),
		stakingDao:          rewardDao.NewStakingPackageDao(),
		userDao:             dao.NewUserDao(),
		balanceRepo:         repo.NewBalanceRepository(),
		accountTypeRepo:     repo.NewAccountTypeRepository(),
		settlementRecordDao: rewardDao.NewSettlementRecordDao(),
		settlementDetailDao: rewardDao.NewSettlementDetailDao(),
	}
}

// GetPendingByDate 查询所有待结算的资金记录
func (r *settlementRepository) GetPendingByDate(ctx context.Context, date time.Time) ([]*rewardEntity.AssetRecordEntity, error) {
	return r.assetRecordDao.GetPendingByDate(ctx, date)
}

// GetSettledMapByDate 查询该日期所有已结算明细
func (r *settlementRepository) GetSettledMapByDate(ctx context.Context, date time.Time) (map[string]bool, error) {
	return r.settlementDetailDao.GetSettledMapByDate(ctx, date)
}

// GetSettlementRecordMapByDate 批量查询该日期的所有结算记录
func (r *settlementRepository) GetSettlementRecordMapByDate(ctx context.Context, date time.Time) (map[int64]*rewardEntity.SettlementRecordEntity, error) {
	return r.settlementRecordDao.GetMapByDate(ctx, date)
}

// CreateSettlementRecord 创建结算记录
func (r *settlementRepository) CreateSettlementRecord(ctx context.Context, tx gdb.TX, record *rewardEntity.SettlementRecordEntity) error {
	return r.settlementRecordDao.Create(ctx, tx, record)
}

// UpdateSettlementRecord 更新结算记录
func (r *settlementRepository) UpdateSettlementRecord(ctx context.Context, tx gdb.TX, record *rewardEntity.SettlementRecordEntity) error {
	return r.settlementRecordDao.Update(ctx, tx, record)
}

// BatchCreateSettlementDetails 批量创建结算明细
func (r *settlementRepository) BatchCreateSettlementDetails(ctx context.Context, tx gdb.TX, details []*rewardEntity.SettlementDetailEntity) error {
	return r.settlementDetailDao.BatchCreate(ctx, tx, details)
}

// CreateSettlementDetail 创建结算明细
func (r *settlementRepository) CreateSettlementDetail(ctx context.Context, tx gdb.TX, detail *rewardEntity.SettlementDetailEntity) error {
	return r.settlementDetailDao.Create(ctx, tx, detail)
}

// UpdateAssetRecordStatus 批量更新资产记录状态
func (r *settlementRepository) UpdateAssetRecordStatus(ctx context.Context, tx gdb.TX, ids []int64, status int) error {
	return r.assetRecordDao.UpdateStatus(ctx, tx, ids, status)
}

// CreateAssetRecord 创建资产记录
func (r *settlementRepository) CreateAssetRecord(ctx context.Context, tx gdb.TX, record *rewardEntity.AssetRecordEntity) error {
	return r.assetRecordDao.Create(ctx, tx, record)
}

// GetQuotaForUpdate 使用悲观锁查询用户额度
func (r *settlementRepository) GetQuotaForUpdate(ctx context.Context, tx gdb.TX, userID int64) (*rewardEntity.UserQuotaEntity, error) {
	return r.quotaRepo.GetForUpdate(ctx, tx, userID)
}

// GetQuotaByUserID 获取用户额度
func (r *settlementRepository) GetQuotaByUserID(ctx context.Context, userID int64) (*rewardEntity.UserQuotaEntity, error) {
	return r.quotaRepo.GetByUserID(ctx, userID)
}

// GetQuotaOrCreate 获取或创建用户额度
func (r *settlementRepository) GetQuotaOrCreate(ctx context.Context, tx gdb.TX, userID int64) (*rewardEntity.UserQuotaEntity, error) {
	return r.quotaRepo.GetOrCreate(ctx, tx, userID)
}

// GetAPGUserBalanceAccountTypeID 获取APG用户余额账户类型ID
func (r *settlementRepository) GetAPGUserBalanceAccountTypeID(ctx context.Context) int64 {
	return r.accountTypeRepo.GetAPGUserBalanceID(ctx)
}

// DepositBalance 增加用户可用余额
func (r *settlementRepository) DepositBalance(ctx context.Context, tx gdb.TX, params *BalanceDepositParams) error {
	return r.balanceRepo.Deposit(ctx, tx, &repo.BalanceOperationParams{
		UserID:         params.UserID,
		AccountTypeID:  params.AccountTypeID,
		Symbol:         params.Symbol,
		Amount:         params.Amount,
		ChangeType:     params.ChangeType,
		RelatedOrderNo: params.RelatedOrderNo,
		RelatedId:      params.RelatedId,
		Remark:         params.Remark,
		OperatorType:   params.OperatorType,
	})
}

// WithdrawBalance 扣减用户可用余额
func (r *settlementRepository) WithdrawBalance(ctx context.Context, tx gdb.TX, params *BalanceWithdrawParams) error {
	return r.balanceRepo.Withdraw(ctx, tx, &repo.BalanceOperationParams{
		UserID:         params.UserID,
		AccountTypeID:  params.AccountTypeID,
		Symbol:         params.Symbol,
		Amount:         params.Amount,
		ChangeType:     params.ChangeType,
		RelatedOrderNo: params.RelatedOrderNo,
		RelatedId:      params.RelatedId,
		Remark:         params.Remark,
		OperatorType:   params.OperatorType,
	})
}

// DeductQuota 扣除用户额度
func (r *settlementRepository) DeductQuota(ctx context.Context, tx gdb.TX, userID int64, amount decimal.Decimal, changeType, rewardType, remark string, relatedID int64, staticRewardAmount, totalRewardAmount decimal.Decimal) error {
	return r.quotaRepo.DeductQuota(ctx, tx, userID, amount, changeType, rewardType, remark, relatedID, staticRewardAmount, totalRewardAmount)
}

// UpdatePackageStatusToExpired 更新算力包状态为出局
func (r *settlementRepository) UpdatePackageStatusToExpired(ctx context.Context, tx gdb.TX, userID int64, status int) error {
	return r.stakingDao.UpdateStatusToExpired(ctx, tx, userID, status)
}
