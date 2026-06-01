package dao

import (
	"context"
	"time"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/frame/db"
	"XWFrame/internal/frame/model"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/shopspring/decimal"
)

// IExchangeDao 兑换记录数据访问接口（只包含基础增删改查）
type IExchangeDao interface {
	// 基础CRUD操作
	Create(ctx context.Context, tx gdb.TX, record *entity.ExchangeRecordEntity) error
	GetById(ctx context.Context, id int64) (*entity.ExchangeRecordEntity, error)
	UpdateById(ctx context.Context, tx gdb.TX, id int64, data map[string]interface{}) error
	DeleteById(ctx context.Context, tx gdb.TX, id int64) error

	// 基础查询操作
	GetByOrderNo(ctx context.Context, orderNo string) (*entity.ExchangeRecordEntity, error)
	GetList(ctx context.Context, req *GetExchangeListReq) (*GetExchangeListRes, error)

	// 查询APG卖出总额（用于节点分红计算）
	GetAPGSellTotalByDate(ctx context.Context, date time.Time) (decimal.Decimal, error)

	// 查询APG销毁总额（用于节点分红计算，支持时间范围）
	GetAPGBurnTotalByDateRange(ctx context.Context, startTime, endTime time.Time) (decimal.Decimal, error)

	// 查询上次节点分红时间
	GetLastNodeDividendTime(ctx context.Context) (time.Time, error)
}

// GetExchangeListReq 获取兑换列表请求
type GetExchangeListReq struct {
	model.PageReq
	UserId       int64 `json:"userId"`
	ExchangeType *int  `json:"exchangeType"`
}

// GetExchangeListRes 获取兑换列表响应
type GetExchangeListRes struct {
	model.PageRes
	List []*entity.ExchangeRecordEntity `json:"list"`
}

// exchangeDao 兑换记录数据访问实现
type exchangeDao struct {
	db gdb.DB
}

// NewExchangeDao 创建兑换记录数据访问实例
func NewExchangeDao() IExchangeDao {
	return &exchangeDao{
		db: db.GetDB(),
	}
}

// Create 创建兑换记录
func (d *exchangeDao) Create(ctx context.Context, tx gdb.TX, record *entity.ExchangeRecordEntity) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		result, err := tx.Model("exchange_record").FieldsEx("id", "created_at", "updated_at").Data(record).InsertAndGetId()
		if err != nil {
			return err
		}
		record.Id = result
		return nil
	})
}

// GetById 根据ID获取兑换记录
func (d *exchangeDao) GetById(ctx context.Context, id int64) (*entity.ExchangeRecordEntity, error) {
	var record entity.ExchangeRecordEntity
	err := d.db.Ctx(ctx).Model("exchange_record").Where("id", id).Scan(&record)
	if err != nil {
		return nil, err
	}
	if record.Id == 0 {
		return nil, nil
	}
	return &record, nil
}

// UpdateById 根据ID更新兑换记录
func (d *exchangeDao) UpdateById(ctx context.Context, tx gdb.TX, id int64, data map[string]interface{}) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Model("exchange_record").Where("id", id).Data(data).Update()
		return err
	})
}

// DeleteById 根据ID删除兑换记录
func (d *exchangeDao) DeleteById(ctx context.Context, tx gdb.TX, id int64) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Model("exchange_record").Where("id", id).Delete()
		return err
	})
}

// GetByOrderNo 根据订单号查询
func (d *exchangeDao) GetByOrderNo(ctx context.Context, orderNo string) (*entity.ExchangeRecordEntity, error) {
	var record entity.ExchangeRecordEntity
	err := d.db.Ctx(ctx).Model("exchange_record").
		Where("order_no = ?", orderNo).
		Scan(&record)
	if err != nil {
		return nil, err
	}
	if record.Id == 0 {
		return nil, nil
	}
	return &record, nil
}

// GetList 查询兑换列表
func (d *exchangeDao) GetList(ctx context.Context, req *GetExchangeListReq) (*GetExchangeListRes, error) {
	query := d.db.Ctx(ctx).Model("exchange_record")

	// 条件筛选
	if req.UserId > 0 {
		query = query.Where("user_id = ?", req.UserId)
	}
	if req.ExchangeType != nil {
		query = query.Where("exchange_type = ?", *req.ExchangeType)
	}

	// 查询总数
	total, err := query.Count()
	if err != nil {
		return nil, err
	}

	// 查询列表
	var list []*entity.ExchangeRecordEntity
	err = query.Order("id DESC").
		Limit((req.Page-1)*req.PageSize, req.PageSize).
		Scan(&list)
	if err != nil {
		return nil, err
	}

	// 计算总页数
	pages := (total + req.PageSize - 1) / req.PageSize

	return &GetExchangeListRes{
		PageRes: model.PageRes{
			Page:     req.Page,
			PageSize: req.PageSize,
			Total:    total,
			Pages:    pages,
		},
		List: list,
	}, nil
}

// GetAPGSellTotalByDate 查询APG销毁总额（用于节点分红计算）
// 根据链上设计，ApgBurnRecord中的burn_amount字段直接就是需要分红的金额
func (d *exchangeDao) GetAPGSellTotalByDate(ctx context.Context, date time.Time) (decimal.Decimal, error) {
	// 计算日期范围：当日00:00:00到23:59:59
	startTime := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endTime := time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 999999999, date.Location())

	// 从apg_burn_records表查询当日APG销毁总额（SwapFeeBurned事件）
	// burn_amount字段直接就是需要分红的金额，不需要再乘以比例
	result, err := d.db.Ctx(ctx).Model("apg_burn_records").
		Fields("COALESCE(SUM(burn_amount), 0) as total").
		Where("block_timestamp >= ?", startTime).
		Where("block_timestamp <= ?", endTime).
		Value()
	if err != nil {
		return decimal.Zero, err
	}

	// 转换为decimal类型
	totalStr := result.String()
	if totalStr == "" || totalStr == "0" {
		return decimal.Zero, nil
	}

	total, err := decimal.NewFromString(totalStr)
	if err != nil {
		return decimal.Zero, err
	}

	return total, nil
}

// GetAPGBurnTotalByDateRange 查询APG销毁总额（用于节点分红计算，支持时间范围）
func (d *exchangeDao) GetAPGBurnTotalByDateRange(ctx context.Context, startTime, endTime time.Time) (decimal.Decimal, error) {
	// 从apg_burn_records表查询指定时间范围内的APG销毁总额
	// burn_amount字段直接就是需要分红的金额，不需要再乘以比例

	query := d.db.Ctx(ctx).Model("apg_burn_records").
		Fields("COALESCE(SUM(burn_amount), 0) as total").
		Where("block_timestamp <= ?", endTime)

	// 如果startTime不为零时间，则添加起始时间条件
	// 如果startTime为零时间，则查询endTime之前的所有记录
	if !startTime.IsZero() {
		query = query.Where("block_timestamp >= ?", startTime)
	}

	result, err := query.Value()
	if err != nil {
		return decimal.Zero, err
	}

	// 转换为decimal类型
	totalStr := result.String()
	if totalStr == "" || totalStr == "0" {
		return decimal.Zero, nil
	}

	total, err := decimal.NewFromString(totalStr)
	if err != nil {
		return decimal.Zero, err
	}

	return total, nil
}

// GetLastNodeDividendTime 查询上次节点分红时间
func (d *exchangeDao) GetLastNodeDividendTime(ctx context.Context) (time.Time, error) {
	// 从asset_record表查询最近一次节点分红的record_time
	result, err := d.db.Ctx(ctx).Model("asset_record").
		Fields("MAX(record_time) as last_time").
		Where("business_type = ?", consts.AssetBusinessTypeRewardNode).
		Where("status = ?", 2). // 已结算
		Value()
	if err != nil {
		return time.Time{}, err
	}

	// 如果没有找到记录，返回零时间（表示从未分红过）
	if result.IsNil() || result.String() == "" {
		return time.Time{}, nil
	}

	// 解析时间
	lastTimeStr := result.String()
	lastTime, err := time.Parse("2006-01-02 15:04:05", lastTimeStr)
	if err != nil {
		return time.Time{}, err
	}

	return lastTime, nil
}
