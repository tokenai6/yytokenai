package reward

import (
	"context"
	"fmt"
	"strings"
	"time"

	"XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// IAssetRecordDao 资金记录数据访问接口
type IAssetRecordDao interface {
	// Create 创建资金记录
	Create(ctx context.Context, tx gdb.TX, record *reward.AssetRecordEntity) error

	// GetLatestRecordDate 获取资金记录最新记录日期
	GetLatestRecordDate(ctx context.Context) (time.Time, error)

	// BatchCreate 批量创建资金记录
	BatchCreate(ctx context.Context, tx gdb.TX, records []*reward.AssetRecordEntity) error

	// GetPendingByDate 查询指定时刻的待结算记录
	GetPendingByDate(ctx context.Context, date time.Time) ([]*reward.AssetRecordEntity, error)

	// GetByUserAndDate 查询用户某时刻的所有资金记录
	GetByUserAndDate(ctx context.Context, userID int64, date time.Time) ([]*reward.AssetRecordEntity, error)

	// GetPagedByUserID 分页查询用户资金记录
	GetPagedByUserID(ctx context.Context, userID int64, businessType, flowType string, page, pageSize int) ([]*reward.AssetRecordEntity, int, error)

	// GetPagedByUserIDWithDateRange 分页查询用户资金记录（支持日期范围）
	GetPagedByUserIDWithDateRange(ctx context.Context, userID int64, businessType, startDate, endDate string, page, pageSize int) ([]*reward.AssetRecordEntity, int, error)

	// UpdateStatus 批量更新状态
	UpdateStatus(ctx context.Context, tx gdb.TX, ids []int64, status int) error

	// GetSumByUserAndBusiness 统计用户某业务类型的金额总和
	GetSumByUserAndBusiness(ctx context.Context, userID int64, businessType string, status int) (decimal.Decimal, error)

	// GetDailySumByBusiness 统计某时刻某业务类型的金额总和
	GetDailySumByBusiness(ctx context.Context, date time.Time, businessType string, status int) (decimal.Decimal, error)

	// GetUserDailySumGroupByBusiness 统计用户某时刻各业务类型的金额（用于奖励概览）
	GetUserDailySumGroupByBusiness(ctx context.Context, userID int64, date time.Time, status int) (map[string]decimal.Decimal, error)

	// GetAllByDateAndBusiness 查询某时刻某业务类型的所有资金记录（用于批量查询）
	GetAllByDateAndBusiness(ctx context.Context, date time.Time, businessType, flowType string) ([]*reward.AssetRecordEntity, error)

	// GetBatchUserSumByBusiness 批量查询多个用户某业务类型的金额总和
	GetBatchUserSumByBusiness(ctx context.Context, userIDs []int64, businessType string, status int, recordTime time.Time) (map[int64]decimal.Decimal, error)

	// GetAllSumsByBusiness 统计指定业务类型在指定时刻的所有用户金额总和
	GetAllSumsByBusiness(ctx context.Context, businessType string, status int, recordTime time.Time) (map[int64]decimal.Decimal, error)

	// CheckExists 检查记录是否已存在（幂等验证）
	CheckExists(ctx context.Context, userID int64, recordTime time.Time, businessType string) (bool, error)

	// BatchCheckExists 批量检查记录是否已存在（幂等验证）
	// 注意：使用完整的 record_time 时间戳进行精确匹配，支持同一天内的多次执行追踪
	// 例如：用户在 2025-10-23 20:00:00 发放的静态收益独立于 21:00:00 发放的静态收益
	BatchCheckExists(ctx context.Context, userIDs []int64, recordTime time.Time, businessType string) (map[int64]bool, error)

	// GetUserTotalIncome 获取用户累计总收益
	GetUserTotalIncome(ctx context.Context, userID int64, status int) (decimal.Decimal, error)

	// GetUserTotalRewardByType 获取用户指定业务类型的总奖励
	GetUserTotalRewardByType(ctx context.Context, userID int64, businessTypes []string) (decimal.Decimal, error)

	// GetUserTotalByBusinessAnyFlow 获取用户指定业务类型的金额总和（不限制收支方向）
	// 用途：统计削减等负值记录，与正向奖励进行抵扣
	GetUserTotalByBusinessAnyFlow(ctx context.Context, userID int64, businessType string, status int) (decimal.Decimal, error)

	// GetLastRecordTimeByBusinessType 获取指定业务类型的最后一条记录的 record_time（用于增量计算）
	GetLastRecordTimeByBusinessType(ctx context.Context, businessType string) (time.Time, error)

	// GetUserLatestRecordTime 获取用户最新的record_time（用于查询昨日数据）
	GetUserLatestRecordTime(ctx context.Context, userID int64) (time.Time, error)

	// GetUserLatestRewardRecordTime 获取用户最新的奖励类型record_time
	GetUserLatestRewardRecordTime(ctx context.Context, userID int64, rewardTypes []string) (time.Time, error)

	// GetUserNthLatestRecordTime 获取用户倒数第N个的record_time（用于查询前日数据）
	// offset: 1表示最新，2表示倒数第二个，以此类推
	GetUserNthLatestRecordTime(ctx context.Context, userID int64, offset int) (time.Time, error)

	// GetUserRecordsByOffset 按offset获取用户记录
	// userID: 用户ID，如果为0表示查询所有用户
	// businessType: 业务类型，空字符串表示所有类型
	// offset: 偏移量指针，若为nil表示不限制偏移，返回所有匹配记录；否则按指定offset返回
	// 返回所有匹配条件的记录（按record_time倒序）
	GetUserRecordsByOffset(ctx context.Context, userID int64, businessType string, offset *int) ([]*reward.AssetRecordEntity, error)

	// GetUserRewardTotalByOffset 按offset获取用户奖励类型记录的总金额
	// userID: 用户ID
	// rewardTypes: 奖励类型列表
	// offset: 偏移量，0=最新记录（昨日），1=前日，2=大前日，nil=所有记录
	// 返回指定偏移量的奖励总额
	GetUserRewardTotalByOffset(ctx context.Context, userID int64, rewardTypes []string, offset *int) (decimal.Decimal, error)

	// GetUserRewardTotalByOffsetGroupByType 按recordTime获取用户奖励类型记录的总金额（按业务类型分组）
	// userID: 用户ID
	// rewardTypes: 奖励类型列表
	// recordTime: 指定的记录时间，如果为零时间则查询所有记录
	// 返回指定时间的各奖励类型总额
	GetUserRewardTotalByOffsetGroupByType(ctx context.Context, userID int64, rewardTypes []string, recordTime time.Time) (map[string]decimal.Decimal, error)

	// GetBatchUserSumByBusinessAndDateRange 批量查询多个用户某业务类型在指定日期范围内的金额总和
	GetBatchUserSumByBusinessAndDateRange(ctx context.Context, userRecords []UserRecordTime, businessType string) (map[string]decimal.Decimal, error)

	// GetYesterdayWeightedRewardRanking 查询昨日加权奖励排行榜（按user_id分组，按amount降序，取前50）
	GetYesterdayWeightedRewardRanking(ctx context.Context, dateStr string, limit int) ([]*reward.AssetRecordEntity, error)

	// GetReleaseUSDTDetails 查询释放USDT明细信息（分页，支持日期范围和business_type IN查询）
	GetReleaseUSDTDetails(ctx context.Context, userID int64, dateStr string, businessTypes []string, businessType string, page, pageSize int) ([]*reward.AssetRecordEntity, int, error)

	// GetByID 根据ID查询资金记录
	GetByID(ctx context.Context, id int64) (*reward.AssetRecordEntity, error)
}

// assetRecordDao 资金记录数据访问实现
type assetRecordDao struct {
	db gdb.DB
}

// UserRecordTime 用户记录时间数据
type UserRecordTime struct {
	UserID     int64
	RecordTime time.Time
}

// NewAssetRecordDao 创建资金记录数据访问实例
func NewAssetRecordDao() IAssetRecordDao {
	return &assetRecordDao{
		db: db.GetDB(),
	}
}

// Create 创建资金记录
func (d *assetRecordDao) Create(ctx context.Context, tx gdb.TX, record *reward.AssetRecordEntity) error {
	model := d.db.Model("asset_record")
	if tx != nil {
		model = tx.Model("asset_record")
	}

	result, err := model.Ctx(ctx).
		FieldsEx("id", "created_at", "updated_at").
		Data(record).
		InsertAndGetId()
	if err != nil {
		return err
	}
	record.Id = result
	return nil
}

// GetLatestRecordDate 获取资金记录最新记录日期
func (d *assetRecordDao) GetLatestRecordDate(ctx context.Context) (time.Time, error) {
	var entity reward.AssetRecordEntity

	err := d.db.Model("asset_record").Ctx(ctx).
		WhereIn("business_type", []string{
			consts.AssetBusinessTypeRewardStatic,
			consts.AssetBusinessTypeRewardDistrict,
			consts.AssetBusinessTypeRewardReferral,
			consts.AssetBusinessTypeRewardWeighted,
			consts.AssetBusinessTypeRewardNode,
			consts.AssetBusinessTypeRewardReduced,
		}).
		OrderDesc("id").
		Limit(1).
		Scan(&entity)
	if err != nil {
		return time.Time{}, err
	}

	if entity.Id == 0 {
		return time.Time{}, nil
	}

	return entity.RecordTime, nil
}

// BatchCreate 批量创建资金记录
func (d *assetRecordDao) BatchCreate(ctx context.Context, tx gdb.TX, records []*reward.AssetRecordEntity) error {
	if len(records) == 0 {
		return nil
	}

	const batchSize = 6000

	for start := 0; start < len(records); start += batchSize {
		end := start + batchSize
		if end > len(records) {
			end = len(records)
		}

		model := d.db.Model("asset_record")
		if tx != nil {
			model = tx.Model("asset_record")
		}

		if _, err := model.Ctx(ctx).
			FieldsEx("id", "created_at", "updated_at").
			Data(records[start:end]).
			Insert(); err != nil {
			return err
		}
	}

	return nil
}

// GetPendingByDate 查询指定时刻的待结算记录
func (d *assetRecordDao) GetPendingByDate(ctx context.Context, date time.Time) ([]*reward.AssetRecordEntity, error) {
	var records []*reward.AssetRecordEntity
	err := d.db.Model("asset_record").Ctx(ctx).
		Where("record_time = ?", date).
		Where("status", 1). // status=1 待结算
		Scan(&records)
	return records, err
}

// GetByUserAndDate 查询用户某时刻的所有资金记录
func (d *assetRecordDao) GetByUserAndDate(ctx context.Context, userID int64, date time.Time) ([]*reward.AssetRecordEntity, error) {
	var records []*reward.AssetRecordEntity
	err := d.db.Model("asset_record").Ctx(ctx).
		Where("user_id", userID).
		Where("record_time = ?", date).
		OrderAsc("business_type").
		Scan(&records)
	return records, err
}

// GetPagedByUserID 分页查询用户资金记录
func (d *assetRecordDao) GetPagedByUserID(ctx context.Context, userID int64, businessType, flowType string, page, pageSize int) ([]*reward.AssetRecordEntity, int, error) {
	model := d.db.Model("asset_record").Ctx(ctx).
		Where("user_id", userID)

	// 业务类型过滤
	if businessType != "" && businessType != "all" {
		model = model.Where("business_type", businessType)
	}

	// 收支类型过滤
	if flowType != "" && flowType != "all" {
		model = model.Where("flow_type", flowType)
	}

	// 获取总数
	total, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	var records []*reward.AssetRecordEntity
	err = model.OrderDesc("record_time").
		OrderDesc("id").
		Page(page, pageSize).
		Scan(&records)
	if err != nil {
		return nil, 0, err
	}

	return records, total, nil
}

// GetPagedByUserIDWithDateRange 分页查询用户资金记录（支持日期范围）
func (d *assetRecordDao) GetPagedByUserIDWithDateRange(ctx context.Context, userID int64, businessType, startDate, endDate string, page, pageSize int) ([]*reward.AssetRecordEntity, int, error) {
	model := d.db.Model("asset_record").Ctx(ctx).
		Where("user_id", userID)

	// 业务类型过滤
	if businessType != "" && businessType != "all" {
		model = model.Where("business_type", businessType)
	}

	// 日期范围过滤
	if startDate != "" {
		model = model.Where("record_time >= ?", startDate+" 00:00:00")
	}
	if endDate != "" {
		model = model.Where("record_time <= ?", endDate+" 23:59:59")
	}

	// 获取总数
	total, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	var records []*reward.AssetRecordEntity
	err = model.OrderDesc("record_time").
		OrderDesc("id").
		Page(page, pageSize).
		Scan(&records)
	if err != nil {
		return nil, 0, err
	}

	return records, total, nil
}

// UpdateStatus 批量更新状态
func (d *assetRecordDao) UpdateStatus(ctx context.Context, tx gdb.TX, ids []int64, status int) error {
	if len(ids) == 0 {
		return nil
	}

	model := d.db.Model("asset_record")
	if tx != nil {
		model = tx.Model("asset_record")
	}

	_, err := model.Ctx(ctx).
		WhereIn("id", ids).
		Data(g.Map{"status": status}).
		Update()
	return err
}

// GetSumByUserAndBusiness 统计用户某业务类型的金额总和
func (d *assetRecordDao) GetSumByUserAndBusiness(ctx context.Context, userID int64, businessType string, status int) (decimal.Decimal, error) {
	model := d.db.Model("asset_record").Ctx(ctx).
		Where("user_id", userID).
		Where("business_type", businessType).
		Where("flow_type", "income") // 只统计收入

	if status > 0 {
		model = model.Where("status", status)
	}

	result, err := model.Sum("amount")
	if err != nil {
		return decimal.Zero, err
	}

	return decimal.NewFromFloat(result), nil
}

// BatchCheckExists 批量检查记录是否已存在（幂等验证）
// 注意：使用完整的 record_time 时间戳进行精确匹配，支持同一天内的多次执行追踪
// 例如：用户在 2025-10-23 20:00:00 发放的静态收益独立于 21:00:00 发放的静态收益
func (d *assetRecordDao) BatchCheckExists(ctx context.Context, userIDs []int64, recordTime time.Time, businessType string) (map[int64]bool, error) {
	if len(userIDs) == 0 {
		return make(map[int64]bool), nil
	}

	var results []struct {
		UserID int64 `json:"user_id"`
	}

	err := d.db.Model("asset_record").Ctx(ctx).
		Fields("user_id").
		WhereIn("user_id", userIDs).
		Where("record_time = ?", recordTime).
		Where("business_type", businessType).
		Scan(&results)
	if err != nil {
		return nil, err
	}

	// 构建存在映射
	existsMap := make(map[int64]bool)
	for _, result := range results {
		existsMap[result.UserID] = true
	}

	// 为所有用户ID设置默认值（不存在）
	for _, userID := range userIDs {
		if _, exists := existsMap[userID]; !exists {
			existsMap[userID] = false
		}
	}

	return existsMap, nil
}

// GetDailySumByBusiness 统计某时刻某业务类型的金额总和
func (d *assetRecordDao) GetDailySumByBusiness(ctx context.Context, date time.Time, businessType string, status int) (decimal.Decimal, error) {
	model := d.db.Model("asset_record").Ctx(ctx).
		Where("record_time = ?", date).
		Where("business_type", businessType).
		Where("flow_type", "income")

	if status > 0 {
		model = model.Where("status", status)
	}

	result, err := model.Sum("amount")
	if err != nil {
		return decimal.Zero, err
	}

	return decimal.NewFromFloat(result), nil
}

// GetUserDailySumGroupByBusiness 统计用户某时刻各业务类型的金额（用于奖励概览）
func (d *assetRecordDao) GetUserDailySumGroupByBusiness(ctx context.Context, userID int64, date time.Time, status int) (map[string]decimal.Decimal, error) {
	var results []struct {
		BusinessType string  `json:"business_type"`
		TotalAmount  float64 `json:"total_amount"`
	}

	model := d.db.Model("asset_record").Ctx(ctx).
		Fields("business_type, SUM(amount) as total_amount").
		Where("user_id", userID).
		Where("record_time = ?", date).
		Where("flow_type", "income").
		Group("business_type")

	if status > 0 {
		model = model.Where("status", status)
	}

	err := model.Scan(&results)
	if err != nil {
		return nil, err
	}

	// 转换为map
	sumMap := make(map[string]decimal.Decimal)
	for _, r := range results {
		sumMap[r.BusinessType] = decimal.NewFromFloat(r.TotalAmount)
	}

	return sumMap, nil
}

// GetUserRewardTotalByOffsetGroupByType 按recordTime获取用户奖励类型记录的总金额（按业务类型分组）
func (d *assetRecordDao) GetUserRewardTotalByOffsetGroupByType(ctx context.Context, userID int64, rewardTypes []string, recordTime time.Time) (map[string]decimal.Decimal, error) {
	// 构建查询条件
	model := d.db.Model("asset_record").Ctx(ctx).
		Fields("business_type, SUM(amount) as total_amount").
		Where("user_id", userID).
		Where("status", 2).          // 已结算
		Where("flow_type", "income") // 收入

	// 如果指定了record_time，则按时间过滤
	if !recordTime.IsZero() {
		model = model.Where("record_time", recordTime)
	}

	// 添加奖励类型过滤
	if len(rewardTypes) > 0 {
		model = model.WhereIn("business_type", rewardTypes)
	}

	// 按业务类型分组
	model = model.Group("business_type")

	// 执行查询
	var results []struct {
		BusinessType string  `json:"business_type"`
		TotalAmount  float64 `json:"total_amount"`
	}

	err := model.Scan(&results)
	if err != nil {
		return nil, err
	}

	// 转换为map
	sumMap := make(map[string]decimal.Decimal)
	for _, r := range results {
		sumMap[r.BusinessType] = decimal.NewFromFloat(r.TotalAmount)
	}

	return sumMap, nil
}

// GetUserRewardTotalByOffset 按offset获取用户奖励类型记录的总金额
func (d *assetRecordDao) GetUserRewardTotalByOffset(ctx context.Context, userID int64, rewardTypes []string, offset *int) (decimal.Decimal, error) {
	// 构建查询条件
	model := d.db.Model("asset_record").Ctx(ctx).
		Fields("user_id, record_time, SUM(amount) as total_amount").
		Where("user_id", userID).
		Where("status", 2).          // 已结算
		Where("flow_type", "income") // 收入

	// 添加奖励类型过滤
	if len(rewardTypes) > 0 {
		model = model.WhereIn("business_type", rewardTypes)
	}

	// 按用户ID和记录时间分组
	model = model.Group("user_id, record_time")

	// 按记录时间倒序排列
	model = model.OrderDesc("record_time")

	// 如果指定了offset，则使用OFFSET和LIMIT
	if offset != nil {
		model = model.Offset(*offset).Limit(1)
	}

	// 执行查询
	var results []struct {
		UserID      int64   `json:"user_id"`
		RecordTime  string  `json:"record_time"`
		TotalAmount float64 `json:"total_amount"`
	}

	err := model.Scan(&results)
	if err != nil {
		return decimal.Zero, err
	}

	// 如果没有结果，返回0
	if len(results) == 0 {
		return decimal.Zero, nil
	}

	// 返回第一个结果的总金额
	return decimal.NewFromFloat(results[0].TotalAmount), nil
}

// GetAllByDateAndBusiness 查询某时刻某业务类型的所有资金记录
// 查询当前执行周期内的记录，不限制状态（因为在统一结算前都是待结算状态）
func (d *assetRecordDao) GetAllByDateAndBusiness(ctx context.Context, date time.Time, businessType, flowType string) ([]*reward.AssetRecordEntity, error) {
	var records []*reward.AssetRecordEntity

	model := d.db.Model("asset_record").Ctx(ctx).
		Where("record_time = ?", date)

	if businessType != "" {
		model = model.Where("business_type", businessType)
	}

	if flowType != "" {
		model = model.Where("flow_type", flowType)
	}

	err := model.Scan(&records)
	return records, err
}

// GetBatchUserSumByBusiness 批量查询多个用户某业务类型的金额总和
func (d *assetRecordDao) GetBatchUserSumByBusiness(ctx context.Context, userIDs []int64, businessType string, status int, recordTime time.Time) (map[int64]decimal.Decimal, error) {
	if len(userIDs) == 0 {
		return make(map[int64]decimal.Decimal), nil
	}

	var results []struct {
		UserID      int64   `json:"user_id"`
		TotalAmount float64 `json:"total_amount"`
	}

	model := d.db.Model("asset_record").Ctx(ctx).
		Fields("user_id, SUM(amount) as total_amount").
		WhereIn("user_id", userIDs).
		Where("business_type", businessType).
		Where("record_time = ?", recordTime).
		Where("flow_type", "income").
		Group("user_id")

	if status > 0 {
		model = model.Where("status", status)
	}

	err := model.Scan(&results)
	if err != nil {
		return nil, err
	}

	// 转换为map
	sumMap := make(map[int64]decimal.Decimal)
	for _, r := range results {
		sumMap[r.UserID] = decimal.NewFromFloat(r.TotalAmount)
	}

	return sumMap, nil
}

// GetBatchUserSumByBusinessAndDateRange 批量查询多个用户某业务类型在指定日期范围内的金额总和
func (d *assetRecordDao) GetBatchUserSumByBusinessAndDateRange(ctx context.Context, userRecords []UserRecordTime, businessType string) (map[string]decimal.Decimal, error) {
	if len(userRecords) == 0 {
		return make(map[string]decimal.Decimal), nil
	}

	model := d.db.Model("asset_record").Ctx(ctx).
		Where("business_type", businessType).
		Where("flow_type", consts.AssetFlowTypeIncome)

	var conditions []string
	var args []interface{}
	for _, record := range userRecords {
		conditions = append(conditions, "(user_id = ? AND record_time = ?)")
		args = append(args, record.UserID, record.RecordTime)
	}
	if len(conditions) > 0 {
		model = model.Where(strings.Join(conditions, " OR "), args...)
	}

	var records []*reward.AssetRecordEntity
	err := model.Scan(&records)
	if err != nil {
		return nil, err
	}

	// 转换为map
	sumMap := make(map[string]decimal.Decimal)
	for _, record := range records {
		key := fmt.Sprintf("%d:%s", record.UserID, record.RecordTime.Format("2006-01-02 15:04:05"))
		if amount, exists := sumMap[key]; exists {
			sumMap[key] = amount.Add(record.Amount)
		} else {
			sumMap[key] = record.Amount
		}
	}

	return sumMap, nil
}

// GetAllSumsByBusiness 统计指定业务类型在指定时刻的所有用户金额总和
func (d *assetRecordDao) GetAllSumsByBusiness(ctx context.Context, businessType string, status int, recordTime time.Time) (map[int64]decimal.Decimal, error) {
	var results []struct {
		UserID      int64   `json:"user_id"`
		TotalAmount float64 `json:"total_amount"`
	}

	model := d.db.Model("asset_record").Ctx(ctx).
		Fields("user_id, SUM(amount) as total_amount").
		Where("business_type", businessType).
		Where("record_time = ?", recordTime).
		Where("flow_type", "income").
		Group("user_id")

	if status > 0 {
		model = model.Where("status", status)
	}

	err := model.Scan(&results)
	if err != nil {
		return nil, err
	}

	// 转换为map
	sumMap := make(map[int64]decimal.Decimal)
	for _, r := range results {
		sumMap[r.UserID] = decimal.NewFromFloat(r.TotalAmount)
	}

	return sumMap, nil
}

// CheckExists 检查记录是否已存在（幂等验证）
func (d *assetRecordDao) CheckExists(ctx context.Context, userID int64, recordTime time.Time, businessType string) (bool, error) {
	count, err := d.db.Model("asset_record").Ctx(ctx).
		Where("user_id", userID).
		Where("record_time", recordTime).
		Where("business_type", businessType).
		Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetUserTotalIncome 获取用户累计总收益
func (d *assetRecordDao) GetUserTotalIncome(ctx context.Context, userID int64, status int) (decimal.Decimal, error) {
	model := d.db.Model("asset_record").Ctx(ctx).
		Where("user_id", userID).
		Where("flow_type", "income")

	if status > 0 {
		model = model.Where("status", status)
	}

	result, err := model.Sum("amount")
	if err != nil {
		return decimal.Zero, err
	}

	return decimal.NewFromFloat(result), nil
}

// GetUserTotalRewardByType 获取用户指定业务类型的总奖励
func (d *assetRecordDao) GetUserTotalRewardByType(ctx context.Context, userID int64, businessTypes []string) (decimal.Decimal, error) {
	if len(businessTypes) == 0 {
		return decimal.Zero, nil
	}

	model := d.db.Model("asset_record").Ctx(ctx).
		Where("user_id", userID).
		WhereIn("business_type", businessTypes).
		Where("flow_type", "income").
		Where("status", 2) // 已结算

	result, err := model.Sum("amount")
	if err != nil {
		return decimal.Zero, err
	}

	return decimal.NewFromFloat(result), nil
}

// GetUserTotalByBusinessAnyFlow 获取用户指定业务类型的金额总和（不限制收支方向）
// status: 传入>0时按状态过滤，例如已结算=2
func (d *assetRecordDao) GetUserTotalByBusinessAnyFlow(ctx context.Context, userID int64, businessType string, status int) (decimal.Decimal, error) {
	model := d.db.Model("asset_record").Ctx(ctx).
		Where("user_id", userID).
		Where("business_type", businessType)

	if status > 0 {
		model = model.Where("status", status)
	}

	result, err := model.Sum("amount")
	if err != nil {
		return decimal.Zero, err
	}

	return decimal.NewFromFloat(result), nil
}

// GetLastRecordTimeByBusinessType 获取指定业务类型的最后一条记录的 record_time（用于增量计算）
func (d *assetRecordDao) GetLastRecordTimeByBusinessType(ctx context.Context, businessType string) (time.Time, error) {
	var result struct {
		RecordTime time.Time `json:"record_time"`
	}

	err := d.db.Model("asset_record").Ctx(ctx).
		Where("business_type", businessType).
		OrderDesc("record_time").
		Limit(1).
		Scan(&result)

	// 如果返回错误，返回零值时间
	// 在调用端会根据零值时间判断是否是首次执行
	if err != nil {
		return time.Time{}, nil
	}

	return result.RecordTime, nil
}

// GetUserLatestRecordTime 获取用户最新的record_time（用于查询昨日数据）
func (d *assetRecordDao) GetUserLatestRecordTime(ctx context.Context, userID int64) (time.Time, error) {
	var result struct {
		RecordTime time.Time `json:"record_time"`
	}

	err := d.db.Model("asset_record").Ctx(ctx).
		Where("user_id", userID).
		OrderDesc("record_time").
		Limit(1).
		Scan(&result)

	if err != nil {
		return time.Time{}, nil
	}

	return result.RecordTime, nil
}

// GetUserLatestRewardRecordTime 获取用户最新的奖励类型record_time
func (d *assetRecordDao) GetUserLatestRewardRecordTime(ctx context.Context, userID int64, rewardTypes []string) (time.Time, error) {
	var result struct {
		RecordTime time.Time `json:"record_time"`
	}

	model := d.db.Model("asset_record").Ctx(ctx).
		Fields("record_time").
		Where("user_id", userID).
		Where("status", 2).          // 已结算
		Where("flow_type", "income") // 收入

	// 添加奖励类型过滤
	if len(rewardTypes) > 0 {
		model = model.WhereIn("business_type", rewardTypes)
	}

	err := model.OrderDesc("record_time").
		Limit(1).
		Scan(&result)

	if err != nil {
		return time.Time{}, nil
	}

	return result.RecordTime, nil
}

// GetUserNthLatestRecordTime 获取用户倒数第N个的record_time（用于查询前日数据）
// offset: 1表示最新，2表示倒数第二个，以此类推
func (d *assetRecordDao) GetUserNthLatestRecordTime(ctx context.Context, userID int64, offset int) (time.Time, error) {
	if offset < 1 {
		offset = 1
	}

	var result struct {
		RecordTime time.Time `json:"record_time"`
	}

	err := d.db.Model("asset_record").Ctx(ctx).
		Where("user_id", userID).
		OrderDesc("record_time").
		Limit(1).
		Offset(offset - 1). // offset-1是因为OFFSET是0-based
		Scan(&result)

	if err != nil {
		return time.Time{}, nil
	}

	return result.RecordTime, nil
}

// GetUserRecordsByOffset 按offset获取用户记录
// userID: 用户ID，如果为0表示查询所有用户
// businessType: 业务类型，空字符串表示所有类型
// offset: 偏移量指针，若为nil表示不限制偏移，返回所有匹配记录；否则按指定offset返回
// 返回所有匹配条件的记录（按record_time倒序）
func (d *assetRecordDao) GetUserRecordsByOffset(ctx context.Context, userID int64, businessType string, offset *int) ([]*reward.AssetRecordEntity, error) {
	model := d.db.Model("asset_record").Ctx(ctx)

	// 如果userID不为0，则按用户ID过滤
	if userID != 0 {
		model = model.Where("user_id", userID)
	}

	// 如果businessType不为空，则按业务类型过滤
	if businessType != "" {
		model = model.Where("business_type", businessType)
	}

	var results []*reward.AssetRecordEntity

	err := model.OrderDesc("record_time").
		Scan(&results)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return []*reward.AssetRecordEntity{}, nil
	}

	// 如果offset不为nil，则从指定偏移量开始返回
	if offset != nil {
		if *offset >= len(results) {
			return []*reward.AssetRecordEntity{}, nil // 超出范围，返回空切片
		}
		records := make([]*reward.AssetRecordEntity, 0, len(results)-*offset)
		for i := *offset; i < len(results); i++ {
			records = append(records, results[i])
		}
		return records, nil
	}

	// 如果offset为nil，返回所有记录
	return results, nil
}

// GetYesterdayWeightedRewardRanking 查询昨日加权奖励排行榜（按user_id分组，按amount降序，取前50）
func (d *assetRecordDao) GetYesterdayWeightedRewardRanking(ctx context.Context, dateStr string, limit int) ([]*reward.AssetRecordEntity, error) {
	if limit <= 0 {
		limit = 50
	}

	// 先查询所有符合条件的记录（每个用户当天可能有多条记录，需要分组汇总）
	// 使用子查询或窗口函数获取每个用户的总金额，然后取金额最大的记录
	var results []*reward.AssetRecordEntity
	fmt.Println("dateStr: ", dateStr)
	err := d.db.Model("asset_record").Ctx(ctx).
		Where("business_type", consts.AssetBusinessTypeRewardWeighted).
		Where("flow_type", consts.AssetFlowTypeIncome).
		Where("record_time = ?", dateStr).
		OrderDesc("amount").
		OrderDesc("id").
		Limit(limit * 10). // 先取更多记录，然后在应用层按user_id去重并分组
		Scan(&results)

	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return []*reward.AssetRecordEntity{}, nil
	}

	// 在应用层按user_id分组，每个用户保留金额最大的记录
	userRecordMap := make(map[int64]*reward.AssetRecordEntity)
	for _, record := range results {
		if existing, exists := userRecordMap[record.UserID]; exists {
			// 如果已存在该用户记录，保留金额更大的
			if record.Amount.GreaterThan(existing.Amount) {
				userRecordMap[record.UserID] = record
			}
		} else {
			userRecordMap[record.UserID] = record
		}
	}

	// 转换为切片并按金额排序
	var sortedRecords []*reward.AssetRecordEntity
	for _, record := range userRecordMap {
		sortedRecords = append(sortedRecords, record)
	}

	// 按金额降序排序（使用快速排序）
	for i := 0; i < len(sortedRecords)-1; i++ {
		maxIdx := i
		for j := i + 1; j < len(sortedRecords); j++ {
			if sortedRecords[j].Amount.GreaterThan(sortedRecords[maxIdx].Amount) {
				maxIdx = j
			}
		}
		if maxIdx != i {
			sortedRecords[i], sortedRecords[maxIdx] = sortedRecords[maxIdx], sortedRecords[i]
		}
	}

	// 取前limit条
	if len(sortedRecords) > limit {
		sortedRecords = sortedRecords[:limit]
	}

	return sortedRecords, nil
}

// GetReleaseUSDTDetails 查询释放USDT明细信息（分页，支持日期范围和business_type IN查询）
func (d *assetRecordDao) GetReleaseUSDTDetails(ctx context.Context, userID int64, dateStr string, businessTypes []string, businessType string, page, pageSize int) ([]*reward.AssetRecordEntity, int, error) {
	model := d.db.Model("asset_record").Ctx(ctx)

	// 用户ID过滤
	if userID > 0 {
		model = model.Where("user_id", userID)
	}

	// 日期范围过滤
	if dateStr != "" {
		startTime := dateStr + " 00:00:00"
		endTime := dateStr + " 23:59:59"
		model = model.Where("record_time >= ?", startTime).Where("record_time <= ?", endTime)
	}

	// 业务类型过滤
	if businessType != "" {
		model = model.Where("business_type", businessType)
	} else if len(businessTypes) > 0 {
		model = model.WhereIn("business_type", businessTypes)
	}

	// 获取总数
	total, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	var records []*reward.AssetRecordEntity
	err = model.OrderDesc("record_time").
		OrderDesc("id").
		Page(page, pageSize).
		Scan(&records)
	if err != nil {
		return nil, 0, err
	}

	return records, total, nil
}

// GetByID 根据ID查询资金记录
func (d *assetRecordDao) GetByID(ctx context.Context, id int64) (*reward.AssetRecordEntity, error) {
	var record reward.AssetRecordEntity
	err := d.db.Model("asset_record").Ctx(ctx).
		Where("id", id).
		Limit(1).
		Scan(&record)
	if err != nil {
		return nil, err
	}
	if record.Id == 0 {
		return nil, nil
	}
	return &record, nil
}
