package dao

import (
	"context"
	"fmt"

	"XWFrame/internal/frame/db"
	"XWFrame/internal/service/balance/model"

	"github.com/gogf/gf/v2/database/gdb"
)

// IBalanceChangeLogDao 余额变动日志数据访问接口
type IBalanceChangeLogDao interface {
	CreateLog(ctx context.Context, tx gdb.TX, log *model.BalanceChangeLog) error
	CheckOrderNoExists(ctx context.Context, tx gdb.TX, orderNo string) (bool, error)
	GetUserLogs(ctx context.Context, req *model.QueryBalanceLogsReq) ([]*model.BalanceChangeLog, int, error)
	GetLogsByOrderNo(ctx context.Context, orderNo string) ([]*model.BalanceChangeLog, error)
	DeleteByMatchIdAndChangeTypes(ctx context.Context, matchId int64, changeTypes []string) (int64, error)
}

// balanceChangeLogDao 余额变动日志数据访问实现
type balanceChangeLogDao struct {
	db gdb.DB
}

// NewBalanceChangeLogDao 创建余额变动日志数据访问实例
func NewBalanceChangeLogDao() IBalanceChangeLogDao {
	return &balanceChangeLogDao{
		db: db.GetDB(),
	}
}

// getDB 获取数据库实例（支持事务）
func (d *balanceChangeLogDao) getDB(ctx context.Context, tx gdb.TX) gdb.DB {
	if tx != nil {
		return tx.GetDB()
	}
	return d.db.Ctx(ctx)
}

// CreateLog 创建变动日志
func (d *balanceChangeLogDao) CreateLog(ctx context.Context, tx gdb.TX, log *model.BalanceChangeLog) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		// PostgreSQL 驱动不支持 LastInsertId，避免使用 InsertAndGetId。
		// 业务不依赖 log.Id，直接插入即可。
		_, err := tx.Model("balance_change_log").
			FieldsEx("id", "created_at").
			Data(log).
			Insert()
		return err
	})
}

// CheckOrderNoExists 检查订单号是否已存在（幂等性）
func (d *balanceChangeLogDao) CheckOrderNoExists(ctx context.Context, tx gdb.TX, orderNo string) (bool, error) {
	count, err := d.getDB(ctx, tx).Model("balance_change_log").
		Where("related_order_no = ?", orderNo).
		Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetUserLogs 查询用户变动日志
func (d *balanceChangeLogDao) GetUserLogs(ctx context.Context, req *model.QueryBalanceLogsReq) ([]*model.BalanceChangeLog, int, error) {
	var list []*model.BalanceChangeLog

	// 构建查询条件
	query := d.db.Ctx(ctx).Model("balance_change_log").Where("user_id = ?", req.UserID)

	// 账户类型ID过滤
	if req.AccountTypeID != nil && *req.AccountTypeID > 0 {
		query = query.Where("account_type_id = ?", *req.AccountTypeID)
	}

	// 资产符号过滤
	if req.Symbol != "" {
		query = query.Where("symbol = ?", req.Symbol)
	}

	// 变动类型过滤
	if req.ChangeType != "" {
		query = query.Where("change_type = ?", req.ChangeType)
	}

	// 查询总数
	total, err := query.Count()
	if err != nil {
		return nil, 0, err
	}

	// 查询列表
	err = query.Order("id DESC").
		Limit((req.Page-1)*req.PageSize, req.PageSize).
		Scan(&list)

	return list, total, err
}

// GetLogsByOrderNo 根据订单号查询日志
func (d *balanceChangeLogDao) GetLogsByOrderNo(ctx context.Context, orderNo string) ([]*model.BalanceChangeLog, error) {
	var list []*model.BalanceChangeLog
	err := d.db.Ctx(ctx).Model("balance_change_log").
		Where("related_order_no = ?", orderNo).
		Order("id DESC").
		Scan(&list)
	return list, err
}

// DeleteByMatchIdAndChangeTypes 删除指定场次的铸币奖励余额变动记录
func (d *balanceChangeLogDao) DeleteByMatchIdAndChangeTypes(ctx context.Context, matchId int64, changeTypes []string) (int64, error) {
	// 匹配 MINT-{matchId}-% 和 MINT-REF-{matchId}-%
	result, err := d.db.Ctx(ctx).Model("balance_change_log").
		WhereIn("change_type", changeTypes).
		Where("(related_order_no LIKE ? OR related_order_no LIKE ?)",
			fmt.Sprintf("MINT-%d-%%", matchId),
			fmt.Sprintf("MINT-REF-%d-%%", matchId)).
		Delete()
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
