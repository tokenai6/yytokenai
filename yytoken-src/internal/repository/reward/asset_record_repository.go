package reward

import (
	"context"
	"time"

	rewardDao "XWFrame/internal/dao/reward"
	rewardEntity "XWFrame/internal/entity/reward"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/shopspring/decimal"
)

// IAssetRecordRepository 资金记录仓储接口
type IAssetRecordRepository interface {
	// GetUserRecordsByOffset 按offset获取用户记录
	GetUserRecordsByOffset(ctx context.Context, userID int64, businessType string, offset *int) ([]*rewardEntity.AssetRecordEntity, error)

	// GetLatestRecordDate 获取资金记录最新记录日期
	GetLatestRecordDate(ctx context.Context) (time.Time, error)

	// GetUserTotalIncome 获取用户累计总收入
	GetUserTotalIncome(ctx context.Context, userID int64, status int) (decimal.Decimal, error)

	// GetUserTotalRewardByType 获取用户指定类型的累计奖励
	GetUserTotalRewardByType(ctx context.Context, userID int64, businessTypes []string) (decimal.Decimal, error)

	// GetUserRewardTotalByOffset 按offset获取用户奖励类型记录的总金额
	GetUserRewardTotalByOffset(ctx context.Context, userID int64, rewardTypes []string, offset *int) (decimal.Decimal, error)

	// GetUserRewardTotalByOffsetGroupByType 按recordTime获取用户奖励类型记录的总金额（按业务类型分组）
	GetUserRewardTotalByOffsetGroupByType(ctx context.Context, userID int64, rewardTypes []string, recordTime time.Time) (map[string]decimal.Decimal, error)

	// GetUserLatestRecordTime 获取用户最新的record_time
	GetUserLatestRecordTime(ctx context.Context, userID int64) (time.Time, error)

	// GetUserLatestRewardRecordTime 获取用户最新的奖励类型record_time
	GetUserLatestRewardRecordTime(ctx context.Context, userID int64, rewardTypes []string) (time.Time, error)

	// GetPagedByUserID 分页获取用户记录
	GetPagedByUserID(ctx context.Context, userID int64, businessType, recordType string, page, pageSize int) ([]*rewardEntity.AssetRecordEntity, int, error)

	// GetBatchUserSumByBusinessAndDateRange 批量查询多个用户某业务类型在指定日期范围内的金额总和
	GetBatchUserSumByBusinessAndDateRange(ctx context.Context, userRecords []UserRecordTime, businessType string) (map[string]decimal.Decimal, error)

	// Create 创建资金记录（可在事务中调用）
	Create(ctx context.Context, tx gdb.TX, record *rewardEntity.AssetRecordEntity) error

	// GetYesterdayWeightedRewardRanking 查询昨日加权奖励排行榜（按user_id分组，按amount降序，取前50）
	GetYesterdayWeightedRewardRanking(ctx context.Context, dateStr string, limit int) ([]*rewardEntity.AssetRecordEntity, error)

	// GetReleaseUSDTDetails 查询释放USDT明细信息（分页，支持日期范围和business_type IN查询）
	GetReleaseUSDTDetails(ctx context.Context, userID int64, dateStr string, businessTypes []string, businessType string, page, pageSize int) ([]*rewardEntity.AssetRecordEntity, int, error)

	// GetByID 根据ID查询资金记录
	GetByID(ctx context.Context, id int64) (*rewardEntity.AssetRecordEntity, error)
}

// UserRecordTime 用户记录时间数据
type UserRecordTime struct {
	UserID     int64
	RecordTime time.Time
}

// assetRecordRepository 资金记录仓储实现
type assetRecordRepository struct {
	assetRecordDao rewardDao.IAssetRecordDao
}

// NewAssetRecordRepository 创建资金记录仓储实例
func NewAssetRecordRepository() IAssetRecordRepository {
	return &assetRecordRepository{
		assetRecordDao: rewardDao.NewAssetRecordDao(),
	}
}

// GetUserRecordsByOffset 按offset获取用户记录
func (r *assetRecordRepository) GetUserRecordsByOffset(ctx context.Context, userID int64, businessType string, offset *int) ([]*rewardEntity.AssetRecordEntity, error) {
	return r.assetRecordDao.GetUserRecordsByOffset(ctx, userID, businessType, offset)
}

// GetLatestRecordDate 获取资金记录最新记录日期
func (r *assetRecordRepository) GetLatestRecordDate(ctx context.Context) (time.Time, error) {
	return r.assetRecordDao.GetLatestRecordDate(ctx)
}

// GetUserTotalIncome 获取用户累计总收入
func (r *assetRecordRepository) GetUserTotalIncome(ctx context.Context, userID int64, status int) (decimal.Decimal, error) {
	return r.assetRecordDao.GetUserTotalIncome(ctx, userID, status)
}

// GetUserTotalRewardByType 获取用户指定类型的累计奖励
func (r *assetRecordRepository) GetUserTotalRewardByType(ctx context.Context, userID int64, businessTypes []string) (decimal.Decimal, error) {
	return r.assetRecordDao.GetUserTotalRewardByType(ctx, userID, businessTypes)
}

// GetUserRewardTotalByOffset 按offset获取用户奖励类型记录的总金额
func (r *assetRecordRepository) GetUserRewardTotalByOffset(ctx context.Context, userID int64, rewardTypes []string, offset *int) (decimal.Decimal, error) {
	return r.assetRecordDao.GetUserRewardTotalByOffset(ctx, userID, rewardTypes, offset)
}

// GetUserRewardTotalByOffsetGroupByType 按recordTime获取用户奖励类型记录的总金额（按业务类型分组）
func (r *assetRecordRepository) GetUserRewardTotalByOffsetGroupByType(ctx context.Context, userID int64, rewardTypes []string, recordTime time.Time) (map[string]decimal.Decimal, error) {
	return r.assetRecordDao.GetUserRewardTotalByOffsetGroupByType(ctx, userID, rewardTypes, recordTime)
}

// GetUserLatestRecordTime 获取用户最新的record_time
func (r *assetRecordRepository) GetUserLatestRecordTime(ctx context.Context, userID int64) (time.Time, error) {
	return r.assetRecordDao.GetUserLatestRecordTime(ctx, userID)
}

// GetUserLatestRewardRecordTime 获取用户最新的奖励类型record_time
func (r *assetRecordRepository) GetUserLatestRewardRecordTime(ctx context.Context, userID int64, rewardTypes []string) (time.Time, error) {
	return r.assetRecordDao.GetUserLatestRewardRecordTime(ctx, userID, rewardTypes)
}

// GetPagedByUserID 分页获取用户记录
func (r *assetRecordRepository) GetPagedByUserID(ctx context.Context, userID int64, businessType, recordType string, page, pageSize int) ([]*rewardEntity.AssetRecordEntity, int, error) {
	return r.assetRecordDao.GetPagedByUserID(ctx, userID, businessType, recordType, page, pageSize)
}

// GetBatchUserSumByBusinessAndDateRange 批量查询多个用户某业务类型在指定日期范围内的金额总和
func (r *assetRecordRepository) GetBatchUserSumByBusinessAndDateRange(ctx context.Context, userRecords []UserRecordTime, businessType string) (map[string]decimal.Decimal, error) {
	daoRecords := make([]rewardDao.UserRecordTime, 0, len(userRecords))
	for _, record := range userRecords {
		daoRecords = append(daoRecords, rewardDao.UserRecordTime{
			UserID:     record.UserID,
			RecordTime: record.RecordTime,
		})
	}
	return r.assetRecordDao.GetBatchUserSumByBusinessAndDateRange(ctx, daoRecords, businessType)
}

// Create 创建资金记录（可在事务中调用）
func (r *assetRecordRepository) Create(ctx context.Context, tx gdb.TX, record *rewardEntity.AssetRecordEntity) error {
	return r.assetRecordDao.Create(ctx, tx, record)
}

// GetYesterdayWeightedRewardRanking 查询昨日加权奖励排行榜（按user_id分组，按amount降序，取前50）
func (r *assetRecordRepository) GetYesterdayWeightedRewardRanking(ctx context.Context, dateStr string, limit int) ([]*rewardEntity.AssetRecordEntity, error) {
	return r.assetRecordDao.GetYesterdayWeightedRewardRanking(ctx, dateStr, limit)
}

// GetReleaseUSDTDetails 查询释放USDT明细信息（分页，支持日期范围和business_type IN查询）
func (r *assetRecordRepository) GetReleaseUSDTDetails(ctx context.Context, userID int64, dateStr string, businessTypes []string, businessType string, page, pageSize int) ([]*rewardEntity.AssetRecordEntity, int, error) {
	return r.assetRecordDao.GetReleaseUSDTDetails(ctx, userID, dateStr, businessTypes, businessType, page, pageSize)
}

// GetByID 根据ID查询资金记录
func (r *assetRecordRepository) GetByID(ctx context.Context, id int64) (*rewardEntity.AssetRecordEntity, error) {
	return r.assetRecordDao.GetByID(ctx, id)
}
