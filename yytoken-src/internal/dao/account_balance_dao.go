package dao

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
)

// IAccountBalanceDao 账户余额数据访问接口（只包含基础增删改查）
type IAccountBalanceDao interface {
	// 基础CRUD操作
	Create(ctx context.Context, tx gdb.TX, balance *entity.AccountBalanceEntity) error
	GetById(ctx context.Context, id int64) (*entity.AccountBalanceEntity, error)
	UpdateById(ctx context.Context, tx gdb.TX, id int64, data map[string]interface{}) error
	DeleteById(ctx context.Context, tx gdb.TX, id int64) error

	// 基础查询操作
	GetByUserAndAccountType(ctx context.Context, userID, accountTypeID int64) (*entity.AccountBalanceEntity, error)
	GetByUserAndAccountTypeForUpdate(ctx context.Context, tx gdb.TX, userID, accountTypeID int64) (*entity.AccountBalanceEntity, error)
	GetListByUserId(ctx context.Context, userID int64) ([]*entity.AccountBalanceEntity, error)
}

// accountBalanceDao 账户余额数据访问实现
type accountBalanceDao struct {
	db gdb.DB
}

// NewAccountBalanceDao 创建账户余额数据访问实例
func NewAccountBalanceDao() IAccountBalanceDao {
	return &accountBalanceDao{
		db: db.GetDB(),
	}
}

// getDB 获取数据库实例（支持事务）
func (d *accountBalanceDao) getDB(ctx context.Context, tx gdb.TX) gdb.DB {
	if tx != nil {
		return tx.GetDB()
	}
	return d.db.Ctx(ctx)
}

// Create 创建账户余额
func (d *accountBalanceDao) Create(ctx context.Context, tx gdb.TX, balance *entity.AccountBalanceEntity) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		// PostgreSQL 驱动不支持 LastInsertId，避免使用 InsertAndGetId。
		// balance.Id 在当前创建流程中不依赖（后续会通过带锁查询获取）。
		_, err := tx.Model("account_balance").
			FieldsEx("id", "created_at", "updated_at").
			Data(balance).
			Insert()
		return err
	})
}

// GetById 根据ID获取账户余额
func (d *accountBalanceDao) GetById(ctx context.Context, id int64) (*entity.AccountBalanceEntity, error) {
	var balance entity.AccountBalanceEntity
	err := d.db.Ctx(ctx).Model("account_balance").Where("id", id).Scan(&balance)
	if err != nil {
		return nil, err
	}
	if balance.Id == 0 {
		return nil, nil
	}
	return &balance, nil
}

// UpdateById 根据ID更新账户余额
func (d *accountBalanceDao) UpdateById(ctx context.Context, tx gdb.TX, id int64, data map[string]interface{}) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Model("account_balance").Where("id", id).Data(data).Update()
		return err
	})
}

// DeleteById 根据ID删除账户余额
func (d *accountBalanceDao) DeleteById(ctx context.Context, tx gdb.TX, id int64) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Model("account_balance").Where("id", id).Delete()
		return err
	})
}

// GetByUserAndAccountType 根据用户ID和账户类型ID查询账户余额（不加锁）
func (d *accountBalanceDao) GetByUserAndAccountType(ctx context.Context, userID, accountTypeID int64) (*entity.AccountBalanceEntity, error) {
	var balance entity.AccountBalanceEntity
	err := d.db.Ctx(ctx).Model("account_balance").
		Where("user_id = ? AND account_type_id = ?", userID, accountTypeID).
		Scan(&balance)
	if err != nil {
		return nil, err
	}
	if balance.Id == 0 {
		return nil, nil
	}
	return &balance, nil
}

// GetByUserAndAccountTypeForUpdate 查询账户余额（悲观锁 - 资金变更必须使用）
func (d *accountBalanceDao) GetByUserAndAccountTypeForUpdate(ctx context.Context, tx gdb.TX, userID, accountTypeID int64) (*entity.AccountBalanceEntity, error) {
	if tx == nil {
		return nil, gerror.New("transaction is required for GetByUserAndAccountTypeForUpdate")
	}

	var balance entity.AccountBalanceEntity
	err := tx.Model("account_balance").
		Where("user_id = ? AND account_type_id = ?", userID, accountTypeID).
		LockUpdate().
		Scan(&balance)

	if err != nil {
		// 检查是否为记录不存在
		if errors.Is(err, sql.ErrNoRows) || err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		// 检查是否为锁等待超时
		if strings.Contains(err.Error(), "lock timeout") || strings.Contains(err.Error(), "deadlock") {
			return nil, gerror.New("账户余额被锁定，请稍后重试")
		}
		return nil, err
	}

	if balance.Id == 0 {
		return nil, nil
	}

	return &balance, nil
}

// GetListByUserId 查询用户余额列表
func (d *accountBalanceDao) GetListByUserId(ctx context.Context, userID int64) ([]*entity.AccountBalanceEntity, error) {
	var list []*entity.AccountBalanceEntity
	err := d.db.Ctx(ctx).Model("account_balance").
		Where("user_id = ?", userID).
		Order("id DESC").
		Scan(&list)
	return list, err
}
