package reward

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/shopspring/decimal"
)

// IUserPerformanceDao 用户业绩数据访问接口
//
// Deprecated: 该接口已废弃，团队业绩统一使用 cobo_node_purchase 表实时计算
// 请使用 internal/dao/cobo IPerformanceDao
// 保留该接口仅用于兼容历史代码
type IUserPerformanceDao interface {
	// Create 创建用户业绩记录
	Create(ctx context.Context, tx gdb.TX, performance *reward.UserPerformanceEntity) error

	// BatchCreate 批量创建用户业绩记录
	BatchCreate(ctx context.Context, tx gdb.TX, performances []*reward.UserPerformanceEntity) error

	// GetByUserAndDate 根据用户ID和日期获取业绩
	GetByUserAndDate(ctx context.Context, userID int64, date time.Time) (*reward.UserPerformanceEntity, error)

	// GetByDate 获取指定日期的所有用户业绩
	GetByDate(ctx context.Context, date time.Time) ([]*reward.UserPerformanceEntity, error)

	// GetTopByNewDirectPerformance 获取直推新增业绩前N名（limit<=0 表示不限制）
	GetTopByNewDirectPerformance(ctx context.Context, date time.Time, limit int) ([]*reward.UserPerformanceEntity, error)

	// GetNewDirectPerformanceSum 获取前N名直推新增业绩总和
	GetNewDirectPerformanceSum(ctx context.Context, userIDs []int64, date time.Time) (decimal.Decimal, error)

	// GetByUserIDs 批量获取用户业绩
	GetByUserIDs(ctx context.Context, userIDs []int64, date time.Time) ([]*reward.UserPerformanceEntity, error)

	// UpdateToZero 清零业绩（用于出局用户）
	UpdateToZero(ctx context.Context, tx gdb.TX, userID int64, date time.Time) error

	// UpdateToZeroWithAudit 清零业绩并记录备注与元数据（用于出局用户）
	UpdateToZeroWithAudit(ctx context.Context, tx gdb.TX, userID int64, date time.Time, remark string, metadata string) error

	// GetUserLatestRecordTime 获取用户最新的record_time
	GetUserLatestRecordTime(ctx context.Context, userID int64) (time.Time, error)

	// GetUserNthLatestRecordTime 获取用户倒数第N个的record_time（offset: 1=最新，2=倒数第二个）
	GetUserNthLatestRecordTime(ctx context.Context, userID int64, offset int) (time.Time, error)

	// GetLatestRecordTimeBefore 获取指定时间之前最近的record_time
	GetLatestRecordTimeBefore(ctx context.Context, date time.Time) (time.Time, error)

	// UpdateIncrementalFields 更新指定用户、指定时间的新增业绩字段
	UpdateIncrementalFields(ctx context.Context, tx gdb.TX, userID int64, recordTime time.Time, data map[string]interface{}) error

	// GetUserRecordsByOffset 按offset获取用户业绩记录
	// userID: 用户ID，如果为0表示查询所有用户
	// offset: 偏移量指针，若为nil表示不限制偏移，返回所有匹配记录；否则按指定offset返回
	// 返回所有匹配条件的业绩记录（按record_time倒序）
	GetUserRecordsByOffset(ctx context.Context, userID int64, offset *int) ([]*reward.UserPerformanceEntity, error)

	// GetLatestPerformancePage 分页获取业绩记录（按id倒序，可按user_ids过滤）
	GetLatestPerformancePage(ctx context.Context, page, pageSize int, userIDs []int64, recordDate string) ([]*reward.UserPerformanceEntity, int, error)

	// GetByDateRange 按日期范围和user_ids分页查询业绩记录（按id倒序）
	// dateStr: 日期字符串，格式为 "2025-01-01"，如果为空则不限制日期
	// userIDs: 用户ID列表，如果为空则不限制用户
	GetByDateRange(ctx context.Context, dateStr string, userIDs []int64, page, pageSize int) ([]*reward.UserPerformanceEntity, int, error)

	// GetLatestRecordDate 获取业绩最新记录日期
	GetLatestRecordDate(ctx context.Context) (time.Time, error)
}

// userPerformanceDao 用户业绩数据访问实现
type userPerformanceDao struct {
	db gdb.DB
}

// NewUserPerformanceDao 创建用户业绩数据访问实例
func NewUserPerformanceDao() IUserPerformanceDao {
	return &userPerformanceDao{
		db: db.GetDB(),
	}
}

// Create 创建用户业绩记录
func (d *userPerformanceDao) Create(ctx context.Context, tx gdb.TX, performance *reward.UserPerformanceEntity) error {
	model := d.db.Model("user_performance")
	if tx != nil {
		model = tx.Model("user_performance")
	}

	result, err := model.Ctx(ctx).
		FieldsEx("id", "created_at", "updated_at", "metadata").
		Data(performance).
		InsertAndGetId()
	if err != nil {
		return err
	}
	performance.Id = result
	return nil
}

// BatchCreate 批量创建用户业绩记录
// 优化：批量插入，减少数据库交互次数，性能提升50-100倍
func (d *userPerformanceDao) BatchCreate(ctx context.Context, tx gdb.TX, performances []*reward.UserPerformanceEntity) error {
	if len(performances) == 0 {
		return nil
	}

	model := d.db.Model("user_performance")
	if tx != nil {
		model = tx.Model("user_performance")
	}

	// 批量插入，每批100条
	// 注意：metadata字段需要包含在插入字段中，不能排除
	_, err := model.Ctx(ctx).
		FieldsEx("id", "created_at", "updated_at").
		Data(performances).
		Batch(100).
		Insert()

	return err
}

// GetByUserAndDate 根据用户ID和时间戳获取业绩
func (d *userPerformanceDao) GetByUserAndDate(ctx context.Context, userID int64, date time.Time) (*reward.UserPerformanceEntity, error) {
	var performance reward.UserPerformanceEntity
	err := d.db.Model("user_performance").Ctx(ctx).
		Where("user_id", userID).
		Where("record_time = ?", date).
		Scan(&performance)
	if err != nil {
		return nil, err
	}
	if performance.Id == 0 {
		return nil, nil
	}
	return &performance, nil
}

// GetByDate 获取指定时刻的所有用户业绩
func (d *userPerformanceDao) GetByDate(ctx context.Context, date time.Time) ([]*reward.UserPerformanceEntity, error) {
	var performances []*reward.UserPerformanceEntity
	err := d.db.Model("user_performance").Ctx(ctx).
		Where("record_time = ?", date).
		Scan(&performances)
	if err != nil {
		return nil, err
	}
	return performances, nil
}

// GetTopByNewDirectPerformance 获取直推新增业绩前N名（limit<=0 表示不限制）
func (d *userPerformanceDao) GetTopByNewDirectPerformance(ctx context.Context, date time.Time, limit int) ([]*reward.UserPerformanceEntity, error) {
	var performances []*reward.UserPerformanceEntity

	model := d.db.Model("user_performance").Ctx(ctx).
		Where("record_time = ?", date).
		WhereGT("new_direct_performance", 0).
		OrderDesc("new_direct_performance").
		OrderDesc("id")

	if limit > 0 {
		model = model.Limit(limit)
	}

	err := model.Scan(&performances)
	if err != nil {
		return nil, err
	}
	return performances, nil
}

// GetNewDirectPerformanceSum 获取前N名直推新增业绩总和
func (d *userPerformanceDao) GetNewDirectPerformanceSum(ctx context.Context, userIDs []int64, date time.Time) (decimal.Decimal, error) {
	var result struct {
		Total decimal.Decimal `json:"total"`
	}

	err := d.db.Model("user_performance").Ctx(ctx).
		Where("record_time = ?", date).
		WhereIn("user_id", userIDs).
		Fields("COALESCE(SUM(new_direct_performance), 0) as total").
		Scan(&result)
	if err != nil {
		return decimal.Zero, err
	}

	return result.Total, nil
}

// GetByUserIDs 批量获取用户业绩
func (d *userPerformanceDao) GetByUserIDs(ctx context.Context, userIDs []int64, date time.Time) ([]*reward.UserPerformanceEntity, error) {
	if len(userIDs) == 0 {
		return []*reward.UserPerformanceEntity{}, nil
	}

	var performances []*reward.UserPerformanceEntity
	err := d.db.Model("user_performance").Ctx(ctx).
		Where("record_time = ?", date).
		WhereIn("user_id", userIDs).
		Scan(&performances)
	if err != nil {
		return nil, err
	}

	return performances, nil
}

// UpdateToZero 清零业绩（用于出局用户）
func (d *userPerformanceDao) UpdateToZero(ctx context.Context, tx gdb.TX, userID int64, date time.Time) error {
	model := d.db.Model("user_performance")
	if tx != nil {
		model = tx.Model("user_performance")
	}

	_, err := model.Ctx(ctx).
		Where("user_id", userID).
		Where("record_time = ?", date).
		Data(map[string]interface{}{
			"personal_performance":     0,
			"new_personal_performance": 0,
			"total_power_value":        0,
			"vip_level":                0,
		}).
		Update()
	return err
}

// UpdateToZeroWithAudit 清零业绩并记录备注与元数据（用于出局用户）
func (d *userPerformanceDao) UpdateToZeroWithAudit(ctx context.Context, tx gdb.TX, userID int64, date time.Time, remark string, metadata string) error {
	model := d.db.Model("user_performance")
	if tx != nil {
		model = tx.Model("user_performance")
	}

	// 安全转义元数据中的单引号，构造 JSONB 字面量（空值兜底为 '{}'）
	var metadataLiteral gdb.Raw
	if strings.TrimSpace(metadata) == "" {
		metadataLiteral = gdb.Raw("'{}'::jsonb")
	} else {
		escaped := strings.ReplaceAll(metadata, "'", "''")
		metadataLiteral = gdb.Raw("'" + escaped + "'::jsonb")
	}

	_, err := model.Ctx(ctx).
		Where("user_id", userID).
		Where("record_time = ?", date).
		Data(map[string]interface{}{
			"personal_performance":     0,
			"new_personal_performance": 0,
			"total_power_value":        0,
			"vip_level":                0,
			"remark":                   remark,
			"metadata":                 metadataLiteral,
		}).
		Update()
	return err
}

// GetUserLatestRecordTime 获取用户最新的record_time
func (d *userPerformanceDao) GetUserLatestRecordTime(ctx context.Context, userID int64) (time.Time, error) {
	var result struct {
		RecordTime time.Time `json:"record_time"`
	}

	err := d.db.Model("user_performance").Ctx(ctx).
		Where("user_id", userID).
		OrderDesc("record_time").
		Limit(1).
		Scan(&result)

	if err != nil {
		return time.Time{}, nil
	}

	return result.RecordTime, nil
}

// GetUserNthLatestRecordTime 获取用户倒数第N个的record_time（offset: 1=最新，2=倒数第二个）
func (d *userPerformanceDao) GetUserNthLatestRecordTime(ctx context.Context, userID int64, offset int) (time.Time, error) {
	if offset < 1 {
		offset = 1
	}

	var result struct {
		RecordTime time.Time `json:"record_time"`
	}

	err := d.db.Model("user_performance").Ctx(ctx).
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

// GetLatestRecordTimeBefore 获取指定时间之前最近的record_time
func (d *userPerformanceDao) GetLatestRecordTimeBefore(ctx context.Context, date time.Time) (time.Time, error) {
	var result struct {
		RecordTime time.Time `json:"record_time"`
	}

	err := d.db.Model("user_performance").Ctx(ctx).
		Where("record_time < ?", date).
		OrderDesc("record_time").
		Limit(1).
		Scan(&result)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || err.Error() == "sql: no rows in result set" {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}

	return result.RecordTime, nil
}

// UpdateIncrementalFields 更新指定用户、指定时间的新增业绩字段
func (d *userPerformanceDao) UpdateIncrementalFields(ctx context.Context, tx gdb.TX, userID int64, recordTime time.Time, data map[string]interface{}) error {
	if len(data) == 0 {
		return nil
	}

	model := d.db.Model("user_performance")
	if tx != nil {
		model = tx.Model("user_performance")
	}

	_, err := model.Ctx(ctx).
		Where("user_id", userID).
		Where("record_time = ?", recordTime).
		Data(data).
		Update()
	return err
}

// GetUserRecordsByOffset 按offset获取用户业绩记录
// userID: 用户ID，如果为0表示查询所有用户
// offset: 偏移量指针，若为nil表示不限制偏移，返回所有匹配记录；否则按指定offset返回
// 返回所有匹配条件的业绩记录（按record_time倒序）
func (d *userPerformanceDao) GetUserRecordsByOffset(ctx context.Context, userID int64, offset *int) ([]*reward.UserPerformanceEntity, error) {
	model := d.db.Model("user_performance").Ctx(ctx)

	// 如果userID大于0，则按用户ID过滤
	if userID > 0 {
		model = model.Where("user_id", userID)
	}

	var results []*reward.UserPerformanceEntity

	err := model.OrderDesc("record_time").
		Scan(&results)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return []*reward.UserPerformanceEntity{}, nil
	}

	// 如果offset不为nil，则从指定偏移量开始返回
	if offset != nil {
		if *offset >= len(results) {
			return []*reward.UserPerformanceEntity{}, nil // 超出范围，返回空切片
		}
		records := make([]*reward.UserPerformanceEntity, 0, len(results)-*offset)
		for i := *offset; i < len(results); i++ {
			records = append(records, results[i])
		}
		return records, nil
	}

	// 如果offset为nil，返回所有记录
	return results, nil
}

// GetLatestPerformancePage 分页获取业绩记录（按id倒序，可按user_ids过滤）
func (d *userPerformanceDao) GetLatestPerformancePage(ctx context.Context, page, pageSize int, userIDs []int64, recordDate string) ([]*reward.UserPerformanceEntity, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	baseModel := d.db.Model("user_performance").Ctx(ctx)
	if len(userIDs) > 0 {
		baseModel = baseModel.WhereIn("user_id", userIDs)
	}
	if recordDate != "" {
		startTime := recordDate + " 00:00:00"
		endTime := recordDate + " 23:59:59"
		baseModel = baseModel.Where("record_time >= ?", startTime).Where("record_time <= ?", endTime)
	}

	total, err := baseModel.Count()
	if err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []*reward.UserPerformanceEntity{}, 0, nil
	}

	dataModel := d.db.Model("user_performance").Ctx(ctx)
	if len(userIDs) > 0 {
		dataModel = dataModel.WhereIn("user_id", userIDs)
	}
	if recordDate != "" {
		startTime := recordDate + " 00:00:00"
		endTime := recordDate + " 23:59:59"
		dataModel = dataModel.Where("record_time >= ?", startTime).Where("record_time <= ?", endTime)
	}

	var results []*reward.UserPerformanceEntity
	err = dataModel.OrderDesc("id").Page(page, pageSize).Scan(&results)
	if err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

// GetByDateRange 按日期范围和user_ids分页查询业绩记录（按id倒序）
func (d *userPerformanceDao) GetByDateRange(ctx context.Context, dateStr string, userIDs []int64, page, pageSize int) ([]*reward.UserPerformanceEntity, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	baseModel := d.db.Model("user_performance").Ctx(ctx)

	// 按用户ID过滤
	if len(userIDs) > 0 {
		baseModel = baseModel.WhereIn("user_id", userIDs)
	}

	// 按日期范围过滤
	if dateStr != "" {
		startTime := dateStr + " 00:00:00"
		endTime := dateStr + " 23:59:59"
		baseModel = baseModel.Where("record_time >= ?", startTime).Where("record_time <= ?", endTime)
	}

	total, err := baseModel.Clone().Count()
	if err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []*reward.UserPerformanceEntity{}, 0, nil
	}

	var results []*reward.UserPerformanceEntity
	err = baseModel.OrderDesc("id").Page(page, pageSize).Scan(&results)
	if err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

// GetLatestRecordDate 获取业绩最新记录日期
func (d *userPerformanceDao) GetLatestRecordDate(ctx context.Context) (time.Time, error) {
	var entity reward.UserPerformanceEntity

	err := d.db.Model("user_performance").Ctx(ctx).
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
