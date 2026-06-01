package reward

import (
	"context"
	"database/sql"
	"errors"
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

// IStakingPackageDao 算力包数据访问接口
type IStakingPackageDao interface {
	// Create 创建算力包
	Create(ctx context.Context, tx gdb.TX, pkg *reward.StakingPackageEntity) error

	// GetActivePackages 获取运行中的算力包（status=1）
	GetActivePackages(ctx context.Context) ([]*reward.StakingPackageEntity, error)

	// GetActivePackagesByRecordTime 获取运行中的算力包，且开始时间小于等于指定时间（用于业绩统计）
	GetActivePackagesByRecordTime(ctx context.Context, recordTime time.Time) ([]*reward.StakingPackageEntity, error)

	// GetPackagesByRecordTimeAndStakeType 获取指定质押类型的算力包，且开始时间小于等于指定时间
	// 说明：不按 status 过滤（别管有没有出局），用于独立口径统计（例如只统计 stake_type=12）。
	GetPackagesByRecordTimeAndStakeType(ctx context.Context, recordTime time.Time, stakeType int) ([]*reward.StakingPackageEntity, error)

	// GetUserActivePackages 获取用户运行中的算力包
	GetUserActivePackages(ctx context.Context, userID int64) ([]*reward.StakingPackageEntity, error)

	// GetUserTotalPowerValue 获取用户总算力值
	GetUserTotalPowerValue(ctx context.Context, userID int64) (decimal.Decimal, error)

	// GetByUserID 获取用户所有算力包
	GetByUserID(ctx context.Context, userID int64) ([]*reward.StakingPackageEntity, error)

	// GetPagedByUserID 分页获取用户算力包（支持日期范围查询）
	GetPagedByUserID(ctx context.Context, userID int64, status int, startDate, endDate string, stakeTypes []int, page, pageSize int) ([]*reward.StakingPackageEntity, int, error)

	// GetPagedList 分页获取质押包列表（支持多条件查询）
	GetPagedList(ctx context.Context, req *GetStakingPackageListReq) ([]*reward.StakingPackageEntity, int, error)

	// GetExpiredByUserID 获取用户已出局的算力包（status=2/3/4）
	GetExpiredByUserID(ctx context.Context, tx gdb.TX, userID int64) ([]*reward.StakingPackageEntity, error)

	// UpdateStatusToExpired 更新算力包状态为出局
	UpdateStatusToExpired(ctx context.Context, tx gdb.TX, userID int64, status int) error

	// UpdateReleasedStatic 更新已释放静态收益、已释放天数和剩余天数
	UpdateReleasedStatic(ctx context.Context, tx gdb.TX, packageID int64, amount, daysElapsed, remainingDays decimal.Decimal) error

	// UpdateMaxStaticRelease 更新最大静态释放金额（一次性释放口径：max_static_release=stake_amount）
	UpdateMaxStaticRelease(ctx context.Context, tx gdb.TX, packageID int64, maxStaticRelease decimal.Decimal) error

	// UpdatePackageStatus 更新算力包状态
	UpdatePackageStatus(ctx context.Context, tx gdb.TX, packageID int64, status int) error

	// UpdateUserPackagesStatus 更新用户所有运行中算力包状态
	UpdateUserPackagesStatus(ctx context.Context, tx gdb.TX, userID int64, status int) error

	// GetByStakeTypeAndStatus 获取特定类型和状态的算力包（用于节点分红计算，stake_type=2为购买节点）
	GetByStakeTypeAndStatus(ctx context.Context, stakeType, status int) ([]*reward.StakingPackageEntity, error)

	// GetUserNodeEquity 获取用户节点权益总额（stake_type=2, status=1）
	GetUserNodeEquity(ctx context.Context, userID int64) (decimal.Decimal, error)

	// GetTotalNodeEquity 获取全网节点总权益（stake_type=2, status=1）
	GetTotalNodeEquity(ctx context.Context) (decimal.Decimal, error)

	// GetNodeEquityUsers 获取所有节点权益持有者及其权益（stake_type=2, status=1）
	GetNodeEquityUsers(ctx context.Context) (map[int64]decimal.Decimal, error)

	// GetTodayPurchaseCountByUserID 获取用户今日购买次数
	GetTodayPurchaseCountByUserID(ctx context.Context, userID int64) (int, error)

	// GetUserStakingStats 获取用户质押统计数据
	GetUserStakingStats(ctx context.Context, userID int64) (*UserStakingStats, error)

	// GetByTxHash 根据交易哈希查询质押包（用于幂等）
	GetByTxHash(ctx context.Context, txHash string) (*reward.StakingPackageEntity, error)

	// GetByPackageNo 根据包编号查询质押包
	GetByPackageNo(ctx context.Context, packageNo string) (*reward.StakingPackageEntity, error)

	// GetByUserIDs 根据用户ID批量查询质押包
	GetByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*reward.StakingPackageEntity, error)

	// GetStakeAmountByStartTimeRange 获取指定开始时间区间内新增的质押金额（按用户聚合）
	GetStakeAmountByStartTimeRange(ctx context.Context, start, end time.Time) (map[int64]decimal.Decimal, error)

	// GetStakeAmountByActualEndTimeRange 获取指定实际结束时间区间内到期的质押金额（按用户聚合）
	GetStakeAmountByActualEndTimeRange(ctx context.Context, start, end time.Time) (map[int64]decimal.Decimal, error)

	// SumStakeAmountByUserIDs 根据用户ID列表统计质押金额
	SumStakeAmountByUserIDs(ctx context.Context, userIDs []int64) (decimal.Decimal, error)

	// SumStakeAmountByUserIDsWithStatus 根据用户ID列表统计质押金额（仅统计运行中的质押包，status=1）
	SumStakeAmountByUserIDsWithStatus(ctx context.Context, userIDs []int64) (decimal.Decimal, error)

	// SumStakeAmountByUserIDsWithPowerMultiplier 根据用户ID列表统计质押金额（只统计 power_multiplier=1 的数据，必须有 user_id）
	SumStakeAmountByUserIDsWithPowerMultiplier(ctx context.Context, userIDs []int64) (decimal.Decimal, error)

	// GetStakeAmountMapByUserIDsWithPowerMultiplier 批量查询用户 stake_amount 汇总（按 user_id 分组，只统计 power_multiplier=1）
	GetStakeAmountMapByUserIDsWithPowerMultiplier(ctx context.Context, userIDs []int64) (map[int64]decimal.Decimal, error)

	// GetByID 根据ID查询质押包
	GetByID(ctx context.Context, packageID int64) (*reward.StakingPackageEntity, error)

	// UpdateStakingAdjustment 更新质押包调整信息（类型、倍数、额度等）
	UpdateStakingAdjustment(ctx context.Context, tx gdb.TX, packageID int64, updateData map[string]interface{}) error

	// CheckStakeEventProcessed 检查质押事件是否已处理
	CheckStakeEventProcessed(ctx context.Context, txHash string, blockNumber uint64) (bool, error)
}

// stakingPackageDao 算力包数据访问实现
type stakingPackageDao struct {
	db gdb.DB
}

// NewStakingPackageDao 创建算力包数据访问实例
func NewStakingPackageDao() IStakingPackageDao {
	return &stakingPackageDao{
		db: db.GetDB(),
	}
}

// Create 创建算力包
func (d *stakingPackageDao) Create(ctx context.Context, tx gdb.TX, pkg *reward.StakingPackageEntity) error {
	model := d.db.Model("staking_package")
	if tx != nil {
		model = tx.Model("staking_package")
	}

	result, err := model.Ctx(ctx).
		FieldsEx("id", "created_at", "updated_at").
		Data(pkg).
		InsertAndGetId()
	if err != nil {
		return err
	}
	pkg.Id = result
	return nil
}

// GetActivePackages 获取运行中的算力包（status=1）
func (d *stakingPackageDao) GetActivePackages(ctx context.Context) ([]*reward.StakingPackageEntity, error) {
	var packages []*reward.StakingPackageEntity
	err := d.db.Model("staking_package").Ctx(ctx).
		Where("status", 1).
		Scan(&packages)
	if err != nil {
		return nil, err
	}
	return packages, nil
}

// GetActivePackagesByRecordTime 获取运行中的算力包，且开始时间小于等于指定时间（用于业绩统计）
// 条件：status=1 且 start_time <= recordTime
func (d *stakingPackageDao) GetActivePackagesByRecordTime(ctx context.Context, recordTime time.Time) ([]*reward.StakingPackageEntity, error) {
	var packages []*reward.StakingPackageEntity
	err := d.db.Model("staking_package").Ctx(ctx).
		Where("status", 1).
		Where("start_time <= ?", recordTime).
		Scan(&packages)
	if err != nil {
		return nil, err
	}
	return packages, nil
}

// GetPackagesByRecordTimeAndStakeType 获取指定质押类型的算力包，且开始时间小于等于指定时间
// 说明：不按 status 过滤（别管有没有出局）
func (d *stakingPackageDao) GetPackagesByRecordTimeAndStakeType(ctx context.Context, recordTime time.Time, stakeType int) ([]*reward.StakingPackageEntity, error) {
	var packages []*reward.StakingPackageEntity
	err := d.db.Model("staking_package").Ctx(ctx).
		Where("stake_type", stakeType).
		Where("start_time <= ?", recordTime).
		Scan(&packages)
	if err != nil {
		return nil, err
	}
	return packages, nil
}

// GetUserActivePackages 获取用户运行中的算力包
func (d *stakingPackageDao) GetUserActivePackages(ctx context.Context, userID int64) ([]*reward.StakingPackageEntity, error) {
	var packages []*reward.StakingPackageEntity
	err := d.db.Model("staking_package").Ctx(ctx).
		Where("user_id", userID).
		Where("status", 1).
		Scan(&packages)
	if err != nil {
		return nil, err
	}
	return packages, nil
}

// GetUserTotalPowerValue 获取用户总算力值
func (d *stakingPackageDao) GetUserTotalPowerValue(ctx context.Context, userID int64) (decimal.Decimal, error) {
	var result struct {
		Total decimal.Decimal `json:"total"`
	}

	err := d.db.Model("staking_package").Ctx(ctx).
		Where("user_id", userID).
		Where("status", 1).
		Fields("COALESCE(SUM(power_value), 0) as total").
		Scan(&result)
	if err != nil {
		return decimal.Zero, err
	}

	return result.Total, nil
}

// GetByUserID 获取用户所有算力包
func (d *stakingPackageDao) GetByUserID(ctx context.Context, userID int64) ([]*reward.StakingPackageEntity, error) {
	var packages []*reward.StakingPackageEntity
	err := d.db.Model("staking_package").Ctx(ctx).
		Where("user_id", userID).
		OrderDesc("created_at").
		Scan(&packages)
	if err != nil {
		return nil, err
	}
	return packages, nil
}

// GetPagedByUserID 分页获取用户算力包（支持日期范围查询）
func (d *stakingPackageDao) GetPagedByUserID(ctx context.Context, userID int64, status int, startDate, endDate string, stakeTypes []int, page, pageSize int) ([]*reward.StakingPackageEntity, int, error) {
	model := d.db.Model("staking_package").Ctx(ctx).
		Where("user_id", userID)

	// 如果指定状态，添加状态过滤
	if status > 0 {
		model = model.Where("status", status)
	}

	// 日期范围过滤（根据创建时间）
	if startDate != "" {
		model = model.Where("created_at >= ?", startDate+" 00:00:00")
	}
	if endDate != "" {
		model = model.Where("created_at <= ?", endDate+" 23:59:59")
	}
	if len(stakeTypes) > 0 {
		model = model.WhereIn("stake_type", stakeTypes)
	}

	// 获取总数
	total, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	var packages []*reward.StakingPackageEntity
	err = model.OrderDesc("created_at").
		Page(page, pageSize).
		Scan(&packages)
	if err != nil {
		return nil, 0, err
	}

	return packages, total, nil
}

// GetExpiredByUserID 获取用户已出局的算力包（status=2/3/4）
func (d *stakingPackageDao) GetExpiredByUserID(ctx context.Context, tx gdb.TX, userID int64) ([]*reward.StakingPackageEntity, error) {
	model := d.db.Model("staking_package")
	if tx != nil {
		model = tx.Model("staking_package")
	}

	var packages []*reward.StakingPackageEntity
	err := model.Ctx(ctx).
		Where("user_id", userID).
		WhereIn("status", []int{2, 3, 4}).
		Scan(&packages)
	if err != nil {
		return nil, err
	}
	return packages, nil
}

// UpdateStatusToExpired 更新算力包状态为出局
func (d *stakingPackageDao) UpdateStatusToExpired(ctx context.Context, tx gdb.TX, userID int64, status int) error {
	model := d.db.Model("staking_package")
	if tx != nil {
		model = tx.Model("staking_package")
	}

	_, err := model.Ctx(ctx).
		Where("user_id", userID).
		Where("status", 1).
		Data(map[string]interface{}{
			"status":          status,
			"actual_end_time": gdb.Raw("CURRENT_TIMESTAMP"),
		}).
		Update()
	return err
}

// UpdateReleasedStatic 更新已释放静态收益、已释放天数和剩余天数
func (d *stakingPackageDao) UpdateReleasedStatic(ctx context.Context, tx gdb.TX, packageID int64, amount, daysElapsed, remainingDays decimal.Decimal) error {
	model := d.db.Model("staking_package")
	if tx != nil {
		model = tx.Model("staking_package")
	}

	_, err := model.Ctx(ctx).
		Where("id", packageID).
		Data(g.Map{
			"released_static": amount,
			"days_elapsed":    daysElapsed,
			"remaining_days":  remainingDays,
		}).
		Update()
	return err
}

// UpdateMaxStaticRelease 更新最大静态释放金额
func (d *stakingPackageDao) UpdateMaxStaticRelease(ctx context.Context, tx gdb.TX, packageID int64, maxStaticRelease decimal.Decimal) error {
	model := d.db.Model("staking_package")
	if tx != nil {
		model = tx.Model("staking_package")
	}

	_, err := model.Ctx(ctx).
		Where("id", packageID).
		Data(g.Map{
			"max_static_release": maxStaticRelease,
		}).
		Update()
	return err
}

// UpdatePackageStatus 更新算力包状态
func (d *stakingPackageDao) UpdatePackageStatus(ctx context.Context, tx gdb.TX, packageID int64, status int) error {
	model := d.db.Model("staking_package")
	if tx != nil {
		model = tx.Model("staking_package")
	}

	updateData := g.Map{"status": status}
	if status == 2 || status == 3 || status == 4 { // 出局
		updateData["actual_end_time"] = time.Now()
	}

	_, err := model.Ctx(ctx).
		Where("id", packageID).
		Data(updateData).
		Update()
	return err
}

// UpdateUserPackagesStatus 更新用户所有运行中算力包状态
func (d *stakingPackageDao) UpdateUserPackagesStatus(ctx context.Context, tx gdb.TX, userID int64, status int) error {
	model := d.db.Model("staking_package")
	if tx != nil {
		model = tx.Model("staking_package")
	}

	updateData := g.Map{"status": status}
	if status == 2 || status == 3 || status == 4 { // 出局
		updateData["actual_end_time"] = time.Now()
	}

	_, err := model.Ctx(ctx).
		Where("user_id", userID).
		Where("status", 1). // 只更新运行中的算力包
		Data(updateData).
		Update()
	return err
}

// GetByStakeTypeAndStatus 获取特定类型和状态的算力包
func (d *stakingPackageDao) GetByStakeTypeAndStatus(ctx context.Context, stakeType, status int) ([]*reward.StakingPackageEntity, error) {
	var packages []*reward.StakingPackageEntity
	err := d.db.Model("staking_package").Ctx(ctx).
		Where("stake_type", stakeType).
		Where("status", status).
		Scan(&packages)
	if err != nil {
		return nil, err
	}
	return packages, nil
}

// GetUserNodeEquity 获取用户节点权益总额（stake_type=2购买节点, status=1）
// 注意：节点权益 = stake_amount（质押金额），不是 power_value（算力值）
func (d *stakingPackageDao) GetUserNodeEquity(ctx context.Context, userID int64) (decimal.Decimal, error) {
	var result struct {
		Total decimal.Decimal `json:"total"`
	}

	err := d.db.Model("staking_package").Ctx(ctx).
		Where("user_id", userID).
		Where("stake_type", 2).                            // 节点购买
		Where("status", 1).                                // 运行中
		Fields("COALESCE(SUM(stake_amount), 0) as total"). // 使用 stake_amount（质押金额），不是 power_value
		Scan(&result)
	if err != nil {
		return decimal.Zero, err
	}

	return result.Total, nil
}

// GetTotalNodeEquity 获取全网节点总权益（stake_type=2购买节点, status=1）
// 注意：节点权益 = stake_amount（质押金额），不是 power_value（算力值）
func (d *stakingPackageDao) GetTotalNodeEquity(ctx context.Context) (decimal.Decimal, error) {
	var result struct {
		Total decimal.Decimal `json:"total"`
	}

	err := d.db.Model("staking_package").Ctx(ctx).
		Where("stake_type", 2).                            // 节点购买
		Where("status", 1).                                // 运行中
		Fields("COALESCE(SUM(stake_amount), 0) as total"). // 使用 stake_amount（质押金额），不是 power_value
		Scan(&result)
	if err != nil {
		return decimal.Zero, err
	}

	return result.Total, nil
}

// GetNodeEquityUsers 获取所有节点权益持有者及其权益（stake_type=2, status=1）
func (d *stakingPackageDao) GetNodeEquityUsers(ctx context.Context) (map[int64]decimal.Decimal, error) {
	var results []struct {
		UserID int64           `json:"user_id"`
		Total  decimal.Decimal `json:"total"`
	}

	err := d.db.Model("staking_package").Ctx(ctx).
		WhereIn("stake_type", []int{2, 4}).                         // 节点购买、节点加速
		Where("status", 1).                                         // 运行中
		Fields("user_id, COALESCE(SUM(stake_amount), 0) as total"). // 使用 stake_amount（质押金额）
		Group("user_id").
		Scan(&results)
	if err != nil {
		return nil, err
	}

	// 转换为map格式
	userEquityMap := make(map[int64]decimal.Decimal)
	for _, result := range results {
		userEquityMap[result.UserID] = result.Total
	}

	return userEquityMap, nil
}

// GetStakeAmountByStartTimeRange 获取指定开始时间区间内新增的质押金额（按用户聚合）
func (d *stakingPackageDao) GetStakeAmountByStartTimeRange(ctx context.Context, start, end time.Time) (map[int64]decimal.Decimal, error) {
	result := make(map[int64]decimal.Decimal)
	if end.IsZero() {
		return result, nil
	}

	var rows []struct {
		UserID int64           `json:"user_id"`
		Total  decimal.Decimal `json:"total"`
	}

	query := d.db.Model("staking_package").Ctx(ctx).
		Where("start_time <= ?", end).
		Fields("user_id, COALESCE(SUM(stake_amount), 0) AS total").
		Group("user_id")

	if !start.IsZero() {
		query = query.Where("start_time > ?", start)
	}

	if err := query.Scan(&rows); err != nil {
		return nil, err
	}

	for _, row := range rows {
		result[row.UserID] = row.Total
	}
	return result, nil
}

// GetStakeAmountByActualEndTimeRange 获取指定实际结束时间区间内到期的质押金额（按用户聚合）
func (d *stakingPackageDao) GetStakeAmountByActualEndTimeRange(ctx context.Context, start, end time.Time) (map[int64]decimal.Decimal, error) {
	result := make(map[int64]decimal.Decimal)
	if end.IsZero() {
		return result, nil
	}

	var rows []struct {
		UserID int64           `json:"user_id"`
		Total  decimal.Decimal `json:"total"`
	}

	query := d.db.Model("staking_package").Ctx(ctx).
		Where("actual_end_time IS NOT NULL").
		Where("actual_end_time <= ?", end).
		Fields("user_id, COALESCE(SUM(stake_amount), 0) AS total").
		Group("user_id")

	if !start.IsZero() {
		query = query.Where("actual_end_time > ?", start)
	}

	if err := query.Scan(&rows); err != nil {
		return nil, err
	}

	for _, row := range rows {
		result[row.UserID] = row.Total
	}
	return result, nil
}

// GetTodayPurchaseCountByUserID 获取用户今日购买次数
func (d *stakingPackageDao) GetTodayPurchaseCountByUserID(ctx context.Context, userID int64) (int, error) {
	// 获取今日开始时间（00:00:00）
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// 查询今日创建的记录数
	count, err := d.db.Model("staking_package").Ctx(ctx).
		Where("user_id", userID).
		WhereGTE("created_at", todayStart).
		Count()
	if err != nil {
		return 0, err
	}

	return count, nil
}

// UserStakingStats 用户质押统计数据
type UserStakingStats struct {
	TotalStakeAmount   decimal.Decimal `json:"total_stake_amount"`   // 质押总金额
	ActiveStakeAmount  decimal.Decimal `json:"active_stake_amount"`  // 有效质押金额（status=1）
	TotalPower         decimal.Decimal `json:"total_power"`          // 总算力
	ReleasedStatic     decimal.Decimal `json:"released_static"`      // 静态释放
	TotalReleasedPower decimal.Decimal `json:"total_released_power"` // 总释放算力
}

// GetUserStakingStats 获取用户质押统计数据
func (d *stakingPackageDao) GetUserStakingStats(ctx context.Context, userID int64) (*UserStakingStats, error) {
	var result struct {
		TotalStakeAmount  decimal.Decimal `json:"total_stake_amount"`
		ActiveStakeAmount decimal.Decimal `json:"active_stake_amount"`
		TotalPower        decimal.Decimal `json:"total_power"`
		ReleasedStatic    decimal.Decimal `json:"released_static"`
	}

	// 使用单个查询一次性获取所有统计数据，提高性能
	// 将常量值直接嵌入 SQL 字符串中
	fields := fmt.Sprintf(`
		COALESCE(SUM(stake_amount), 0) as total_stake_amount,
		COALESCE(SUM(CASE WHEN status = %d THEN stake_amount ELSE 0 END), 0) as active_stake_amount,
		COALESCE(SUM(power_value), 0) as total_power,
		COALESCE(SUM(released_static), 0) as released_static
	`, consts.StakingStatusRunning)

	err := d.db.Model("staking_package").Ctx(ctx).
		Where("user_id", userID).
		Fields(fields).
		Scan(&result)
	if err != nil {
		return nil, err
	}

	// 查询总释放算力：统计 asset_record 表中所有奖励类型的收入总和
	businessTypes := []string{
		consts.AssetBusinessTypeRewardStatic,
		consts.AssetBusinessTypeRewardDistrict,
		consts.AssetBusinessTypeRewardReferral,
		consts.AssetBusinessTypeRewardWeighted,
		consts.AssetBusinessTypeRewardNode,
		consts.AssetBusinessTypeRewardReduced,
	}

	var totalReleasedPower decimal.Decimal
	if len(businessTypes) > 0 {
		sumResult, err := d.db.Model("asset_record").Ctx(ctx).
			Where("user_id", userID).
			WhereIn("business_type", businessTypes).
			Sum("amount")
		if err != nil {
			return nil, err
		}
		totalReleasedPower = decimal.NewFromFloat(sumResult)
	}

	stats := &UserStakingStats{
		TotalStakeAmount:   result.TotalStakeAmount,
		ActiveStakeAmount:  result.ActiveStakeAmount,
		TotalPower:         result.TotalPower,
		ReleasedStatic:     result.ReleasedStatic,
		TotalReleasedPower: totalReleasedPower,
	}

	return stats, nil
}

// GetByTxHash 根据交易哈希查询质押包（用于幂等）
func (d *stakingPackageDao) GetByTxHash(ctx context.Context, txHash string) (*reward.StakingPackageEntity, error) {
	var pkg reward.StakingPackageEntity
	err := d.db.Model("staking_package").Ctx(ctx).
		Where("tx_hash", txHash).
		Limit(1).
		Scan(&pkg)
	if err != nil {
		// 如果查询不到记录（sql: no rows in result set），返回 nil, nil（记录不存在，这是正常情况）
		if errors.Is(err, sql.ErrNoRows) || err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	if pkg.Id == 0 {
		return nil, nil
	}
	return &pkg, nil
}

// GetByPackageNo 根据包编号查询质押包
func (d *stakingPackageDao) GetByPackageNo(ctx context.Context, packageNo string) (*reward.StakingPackageEntity, error) {
	if strings.TrimSpace(packageNo) == "" {
		return nil, nil
	}

	var pkg reward.StakingPackageEntity
	err := d.db.Model("staking_package").Ctx(ctx).
		Where("package_no", packageNo).
		Limit(1).
		Scan(&pkg)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	if pkg.Id == 0 {
		return nil, nil
	}
	return &pkg, nil
}

// GetByUserIDs 根据用户ID批量查询质押包
func (d *stakingPackageDao) GetByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*reward.StakingPackageEntity, error) {
	if len(userIDs) == 0 {
		return map[int64]*reward.StakingPackageEntity{}, nil
	}

	var packages []*reward.StakingPackageEntity
	err := d.db.Model("staking_package").Ctx(ctx).
		WhereIn("user_id", userIDs).
		OrderDesc("created_at").
		Scan(&packages)
	if err != nil {
		return nil, err
	}

	result := make(map[int64]*reward.StakingPackageEntity)
	for _, pkg := range packages {
		result[pkg.UserID] = pkg
	}
	return result, nil
}

// SumStakeAmountByUserIDs 根据用户ID列表统计质押金额
func (d *stakingPackageDao) SumStakeAmountByUserIDs(ctx context.Context, userIDs []int64) (decimal.Decimal, error) {
	if len(userIDs) == 0 {
		return decimal.Zero, nil
	}

	var result struct {
		Total decimal.Decimal `json:"total"`
	}
	err := d.db.Model("staking_package").Ctx(ctx).
		WhereIn("user_id", userIDs).
		Fields("COALESCE(SUM(stake_amount), 0) AS total").
		Scan(&result)
	if err != nil {
		return decimal.Zero, err
	}

	return result.Total, nil
}

// SumStakeAmountByUserIDsWithStatus 根据用户ID列表统计质押金额（仅统计运行中的质押包，status=1）
func (d *stakingPackageDao) SumStakeAmountByUserIDsWithStatus(ctx context.Context, userIDs []int64) (decimal.Decimal, error) {
	if len(userIDs) == 0 {
		return decimal.Zero, nil
	}

	var result struct {
		Total decimal.Decimal `json:"total"`
	}
	err := d.db.Model("staking_package").Ctx(ctx).
		WhereIn("user_id", userIDs).
		Where("status", 1).
		Fields("COALESCE(SUM(stake_amount), 0) AS total").
		Scan(&result)
	if err != nil {
		return decimal.Zero, err
	}

	return result.Total, nil
}

// SumStakeAmountByUserIDsWithPowerMultiplier 根据用户ID列表统计质押金额（只统计 power_multiplier=1 的数据，必须有 user_id）
func (d *stakingPackageDao) SumStakeAmountByUserIDsWithPowerMultiplier(ctx context.Context, userIDs []int64) (decimal.Decimal, error) {
	if len(userIDs) == 0 {
		return decimal.Zero, nil
	}

	// 不能直接 WhereIn(user_id, userIDs)：userIDs 太大时会触发 PostgreSQL 65535 参数上限。
	// 也不要用 ANY(?)：在部分驱动/框架参数绑定下，[]int64 可能被当成单值，导致 malformed array literal。
	// 这里采用分批 IN 汇总，稳定且兼容。
	const batchSize = 5000

	total := decimal.Zero
	for i := 0; i < len(userIDs); i += batchSize {
		end := i + batchSize
		if end > len(userIDs) {
			end = len(userIDs)
		}
		batch := userIDs[i:end]

		var result struct {
			Total decimal.Decimal `json:"total"`
		}
		err := d.db.Model("staking_package").Ctx(ctx).
			WhereIn("user_id", batch).
			Where("power_multiplier", 1).
			Where("user_id IS NOT NULL").
			Fields("COALESCE(SUM(stake_amount), 0) AS total").
			Scan(&result)
		if err != nil {
			return decimal.Zero, err
		}
		total = total.Add(result.Total)
	}

	return total, nil
}

func (d *stakingPackageDao) GetStakeAmountMapByUserIDsWithPowerMultiplier(ctx context.Context, userIDs []int64) (map[int64]decimal.Decimal, error) {
	result := make(map[int64]decimal.Decimal)
	if len(userIDs) == 0 {
		return result, nil
	}

	const batchSize = 5000

	type row struct {
		UserID int64           `json:"user_id"`
		Total  decimal.Decimal `json:"total"`
	}

	for i := 0; i < len(userIDs); i += batchSize {
		end := i + batchSize
		if end > len(userIDs) {
			end = len(userIDs)
		}
		batch := userIDs[i:end]

		var rows []row
		if err := d.db.Model("staking_package").Ctx(ctx).
			Fields("user_id, COALESCE(SUM(stake_amount), 0) AS total").
			WhereIn("user_id", batch).
			Where("power_multiplier", 1).
			Group("user_id").
			Scan(&rows); err != nil {
			return nil, err
		}

		for _, r := range rows {
			if r.UserID <= 0 {
				continue
			}
			result[r.UserID] = result[r.UserID].Add(r.Total)
		}
	}

	return result, nil
}

// GetByID 根据ID查询质押包
func (d *stakingPackageDao) GetByID(ctx context.Context, packageID int64) (*reward.StakingPackageEntity, error) {
	var pkg reward.StakingPackageEntity
	err := d.db.Model("staking_package").Ctx(ctx).
		Where("id", packageID).
		Limit(1).
		Scan(&pkg)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	if pkg.Id == 0 {
		return nil, nil
	}
	return &pkg, nil
}

// CheckStakeEventProcessed 检查质押事件是否已处理
func (d *stakingPackageDao) CheckStakeEventProcessed(ctx context.Context, txHash string, blockNumber uint64) (bool, error) {
	count, err := d.db.Model("staking_package").Ctx(ctx).
		Where("tx_hash = ?", txHash).
		Where("block_number = ?", blockNumber).
		Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// UpdateStakingAdjustment 更新质押包调整信息（类型、倍数、额度等）
func (d *stakingPackageDao) UpdateStakingAdjustment(ctx context.Context, tx gdb.TX, packageID int64, updateData map[string]interface{}) error {
	model := d.db.Model("staking_package")
	if tx != nil {
		model = tx.Model("staking_package")
	}

	// 添加更新时间
	updateData["updated_at"] = time.Now()

	_, err := model.Ctx(ctx).
		Where("id", packageID).
		Data(updateData).
		Update()
	return err
}

// GetStakingPackageListReq 分页查询请求
type GetStakingPackageListReq struct {
	UserID        int64  `json:"user_id"`        // 用户ID（必填）
	Status        int    `json:"status"`         // 状态：1-运行中，2-历史质押（status in 2,3,4）
	StakeTypes    []int  `json:"stake_types"`    // 质押类型列表（可选，如 [2, 4]）
	TokenContract string `json:"token_contract"` // 质押合约地址
	Page          int    `json:"page"`           // 页码
	PageSize      int    `json:"page_size"`      // 每页数量
}

// GetPagedList 分页获取质押包列表
func (d *stakingPackageDao) GetPagedList(ctx context.Context, req *GetStakingPackageListReq) ([]*reward.StakingPackageEntity, int, error) {
	model := d.db.Model("staking_package").Ctx(ctx)

	// 用户ID条件
	if req.UserID > 0 {
		model = model.Where("user_id", req.UserID)
	}

	// 状态条件处理
	switch req.Status {
	case consts.StakingStatusRunning:
		// status = 1（运行中）
		model = model.Where("status", consts.StakingStatusRunning)
	case 2:
		// status in (2,3,4)（历史质押：已出局的）
		model = model.WhereIn("status", []int{consts.StakingStatusStaticExpired, consts.StakingStatusQuotaExpired, consts.StakingStatusForceExpired})
	default:
		// 如果不指定状态或指定了0，则不添加状态过滤
		if req.Status > 0 {
			model = model.Where("status", req.Status)
		}
	}
	if req.TokenContract != "" {
		if req.TokenContract == "0x0000000000000000000000000000000000000000" {
			//=null或者等于空
			model = model.Where("token_contract IS NULL OR token_contract = ''")
		} else {
			model = model.Where("token_contract", req.TokenContract)
		}
	}

	// 质押类型条件处理
	if len(req.StakeTypes) > 0 {
		model = model.WhereIn("stake_type", req.StakeTypes)
	}

	// 获取总数
	total, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	var packages []*reward.StakingPackageEntity
	err = model.OrderDesc("created_at").
		Page(req.Page, req.PageSize).
		Scan(&packages)
	if err != nil {
		return nil, 0, err
	}

	return packages, total, nil
}
