package dao

import (
	"context"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"
	"XWFrame/internal/frame/model"

	"github.com/gogf/gf/v2/database/gdb"
)

// IRechargeDao 充值记录数据访问接口（只包含基础增删改查）
type IRechargeDao interface {
	// 基础CRUD操作
	Create(ctx context.Context, tx gdb.TX, record *entity.RechargeRecordEntity) error
	GetById(ctx context.Context, id int64) (*entity.RechargeRecordEntity, error)
	UpdateById(ctx context.Context, tx gdb.TX, id int64, data map[string]interface{}) error
	DeleteById(ctx context.Context, tx gdb.TX, id int64) error

	// 基础查询操作
	GetByOrderNo(ctx context.Context, orderNo string) (*entity.RechargeRecordEntity, error)
	GetByTxHash(ctx context.Context, txHash string) (*entity.RechargeRecordEntity, error)
	GetList(ctx context.Context, req *GetRechargeListReq) (*GetRechargeListRes, error)
}

// GetRechargeListReq 获取充值列表请求
type GetRechargeListReq struct {
	model.PageReq
	UserId int64  `json:"userId"`
	Symbol string `json:"symbol"`
	Status *int   `json:"status"`
}

// GetRechargeListRes 获取充值列表响应
type GetRechargeListRes struct {
	model.PageRes
	List []*entity.RechargeRecordEntity `json:"list"`
}

// rechargeDao 充值记录数据访问实现
type rechargeDao struct {
	db gdb.DB
}

// NewRechargeDao 创建充值记录数据访问实例
func NewRechargeDao() IRechargeDao {
	return &rechargeDao{
		db: db.GetDB(),
	}
}

// Create 创建充值记录
func (d *rechargeDao) Create(ctx context.Context, tx gdb.TX, record *entity.RechargeRecordEntity) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		result, err := tx.Model("recharge_record").FieldsEx("id", "created_at", "updated_at").Data(record).InsertAndGetId()
		if err != nil {
			return err
		}
		record.Id = result
		return nil
	})
}

// GetById 根据ID获取充值记录
func (d *rechargeDao) GetById(ctx context.Context, id int64) (*entity.RechargeRecordEntity, error) {
	var record entity.RechargeRecordEntity
	err := d.db.Ctx(ctx).Model("recharge_record").Where("id", id).Scan(&record)
	if err != nil {
		return nil, err
	}
	if record.Id == 0 {
		return nil, nil
	}
	return &record, nil
}

// UpdateById 根据ID更新充值记录
func (d *rechargeDao) UpdateById(ctx context.Context, tx gdb.TX, id int64, data map[string]interface{}) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Model("recharge_record").Where("id", id).Data(data).Update()
		return err
	})
}

// DeleteById 根据ID删除充值记录
func (d *rechargeDao) DeleteById(ctx context.Context, tx gdb.TX, id int64) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Model("recharge_record").Where("id", id).Delete()
		return err
	})
}

// GetByOrderNo 根据订单号查询
func (d *rechargeDao) GetByOrderNo(ctx context.Context, orderNo string) (*entity.RechargeRecordEntity, error) {
	var record entity.RechargeRecordEntity
	err := d.db.Ctx(ctx).Model("recharge_record").
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

// GetByTxHash 根据交易哈希查询
func (d *rechargeDao) GetByTxHash(ctx context.Context, txHash string) (*entity.RechargeRecordEntity, error) {
	var record entity.RechargeRecordEntity
	err := d.db.Ctx(ctx).Model("recharge_record").
		Where("tx_hash = ?", txHash).
		Scan(&record)
	if err != nil {
		return nil, err
	}
	if record.Id == 0 {
		return nil, nil
	}
	return &record, nil
}

// GetList 查询充值列表
func (d *rechargeDao) GetList(ctx context.Context, req *GetRechargeListReq) (*GetRechargeListRes, error) {
	query := d.db.Ctx(ctx).Model("recharge_record")

	// 条件筛选
	if req.UserId > 0 {
		query = query.Where("user_id = ?", req.UserId)
	}
	if req.Symbol != "" {
		query = query.Where("symbol = ?", req.Symbol)
	}
	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}
	// 添加金额过滤（小于 0.01 的订单不显示）
	query = query.Where("amount >= ?", 0.01)

	// 查询总数
	total, err := query.Count()
	if err != nil {
		return nil, err
	}

	// 查询列表
	var list []*entity.RechargeRecordEntity
	err = query.Order("id DESC").
		Limit((req.Page-1)*req.PageSize, req.PageSize).
		Scan(&list)
	if err != nil {
		return nil, err
	}

	// 计算总页数
	pages := (total + req.PageSize - 1) / req.PageSize

	return &GetRechargeListRes{
		PageRes: model.PageRes{
			Page:     req.Page,
			PageSize: req.PageSize,
			Total:    total,
			Pages:    pages,
		},
		List: list,
	}, nil
}
