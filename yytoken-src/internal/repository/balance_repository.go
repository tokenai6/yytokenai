package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"
	"XWFrame/internal/service/balance/model"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/shopspring/decimal"
)

// BalanceOperationParams 余额操作通用参数
type BalanceOperationParams struct {
	UserID         int64           `json:"userId"`         // 用户ID
	AccountTypeID  int64           `json:"accountTypeId"`  // 账户类型ID
	AccountType    string          `json:"accountType"`    // 账户类型标识（如apg_user_balance，可选，为空时自动查询）
	Symbol         string          `json:"symbol"`         // 资产符号
	Amount         decimal.Decimal `json:"amount"`         // 变动金额
	ChangeType     string          `json:"changeType"`     // 变动类型
	RelatedOrderNo string          `json:"relatedOrderNo"` // 关联订单号
	RelatedId      int64           `json:"relatedId"`      // 关联ID（如提现记录ID、充值记录ID等）
	Remark         string          `json:"remark"`         // 备注
	OperatorID     int64           `json:"operatorId"`     // 操作人ID
	OperatorType   string          `json:"operatorType"`   // 操作人类型
}

// GetBalanceListReq 查询余额列表请求参数（支持多字段组合查询）
type GetBalanceListReq struct {
	UserID        int64  `json:"userId"`        // 用户ID（必填）
	AccountTypeID *int64 `json:"accountTypeId"` // 账户类型ID（可选）
	Symbol        string `json:"symbol"`        // 资产符号（可选）
	MinBalance    string `json:"minBalance"`    // 最小余额（可选，用于过滤小额资产）
}

// GetBalanceChangeLogsReq 查询余额变动日志请求参数
type GetBalanceChangeLogsReq struct {
	UserID        int64  `json:"userId"`        // 用户ID（必填）
	AccountTypeID *int64 `json:"accountTypeId"` // 账户类型ID（可选）
	Symbol        string `json:"symbol"`        // 资产符号（可选）
	ChangeType    string `json:"changeType"`    // 变动类型（可选）
	Page          int    `json:"page"`          // 页码
	PageSize      int    `json:"pageSize"`      // 每页数量
}

// GetBalanceChangeLogsRes 查询余额变动日志响应
type GetBalanceChangeLogsRes struct {
	List  []*model.BalanceChangeLog `json:"list"`  // 变动日志列表
	Total int                       `json:"total"` // 总记录数
}

// IBalanceRepository 余额仓储接口
type IBalanceRepository interface {
	// 余额查询
	// GetBalance 查询单个账户余额（用户ID+账户类型ID）
	GetBalance(ctx context.Context, userID, accountTypeID int64) (*entity.AccountBalanceEntity, error)

	// GetBalanceList 查询余额列表（用户ID必填，支持账户类型、资产符号、最小余额等筛选）
	GetBalanceList(ctx context.Context, req *GetBalanceListReq) ([]*entity.AccountBalanceEntity, error)

	// GetUserBalanceBySymbol 根据用户ID、资产符号和账户类型查询余额
	GetUserBalanceBySymbol(ctx context.Context, userID int64, symbol, accountType string) (*entity.AccountBalanceEntity, error)

	// GetBalanceForUpdate 查询账户余额并加锁（用于事务中检查余额）
	GetBalanceForUpdate(ctx context.Context, tx gdb.TX, userID, accountTypeID int64) (*entity.AccountBalanceEntity, error)

	// EnsureAccountBalance 确保用户账户余额记录存在（不存在则创建0余额）
	// 说明：tx 可为 nil；若为 nil 会在独立事务中创建，避免被上层业务事务回滚。
	EnsureAccountBalance(ctx context.Context, tx gdb.TX, userID, accountTypeID int64, accountType, symbol string) error

	// 余额操作
	// FreezeBalance 冻结余额并记录日志（提现申请时调用）
	FreezeBalance(ctx context.Context, tx gdb.TX, params *BalanceOperationParams) error

	// UnfreezeBalance 解冻余额并记录日志（提现失败、审核拒绝时调用）
	UnfreezeBalance(ctx context.Context, tx gdb.TX, params *BalanceOperationParams) error

	// UnfreezeAndDeduct 解冻并扣除余额，记录日志（提现成功时调用）
	UnfreezeAndDeduct(ctx context.Context, tx gdb.TX, params *BalanceOperationParams) error

	// Deposit 入金操作，增加余额并记录日志（充值、兑换入账时调用）
	Deposit(ctx context.Context, tx gdb.TX, params *BalanceOperationParams) error

	// Withdraw 出金操作，减少余额并记录日志（兑换扣款时调用）
	Withdraw(ctx context.Context, tx gdb.TX, params *BalanceOperationParams) error

	// 日志查询
	// GetBalanceChangeLogs 查询用户余额变动日志
	GetBalanceChangeLogs(ctx context.Context, req *GetBalanceChangeLogsReq) (*GetBalanceChangeLogsRes, error)
}

// balanceRepository 余额仓储实现
type balanceRepository struct {
	balanceDao          dao.IAccountBalanceDao   // 账户余额DAO
	balanceChangeLogDao dao.IBalanceChangeLogDao // 余额变动日志DAO
	accountTypeDao      dao.IAccountTypeDao      // 账户类型DAO
}

// NewBalanceRepository 创建余额仓储实例
func NewBalanceRepository() IBalanceRepository {
	return &balanceRepository{
		balanceDao:          dao.NewAccountBalanceDao(),
		balanceChangeLogDao: dao.NewBalanceChangeLogDao(),
		accountTypeDao:      dao.NewAccountTypeDao(),
	}
}

// createBalanceChangeLog 创建余额变动日志（内部方法）
func (r *balanceRepository) createBalanceChangeLog(ctx context.Context, tx gdb.TX, params *BalanceOperationParams, beforeBalance, afterBalance decimal.Decimal) error {
	// related_order_no 数据库字段通常为 varchar(64)：
	// - 对 tx hash(0x + 64 hex) 场景：去掉 0x 以适配长度
	// - 若仍超长：取后 64 位，避免入库失败导致事务回滚
	orderNo := strings.TrimSpace(params.RelatedOrderNo)
	if strings.HasPrefix(orderNo, "0x") || strings.HasPrefix(orderNo, "0X") {
		if len(orderNo) == 66 {
			orderNo = orderNo[2:]
		}
	}
	if len(orderNo) > 64 {
		orderNo = orderNo[len(orderNo)-64:]
	}

	log := &model.BalanceChangeLog{
		UserId:         params.UserID,
		AccountTypeId:  params.AccountTypeID,
		Symbol:         params.Symbol,
		ChangeType:     params.ChangeType,
		Amount:         params.Amount,
		BeforeBalance:  beforeBalance,
		AfterBalance:   afterBalance,
		RelatedOrderNo: orderNo,
		RelatedId:      params.RelatedId,
		Remark:         params.Remark,
		OperatorId:     params.OperatorID,
		OperatorType:   params.OperatorType,
	}
	return r.balanceChangeLogDao.CreateLog(ctx, tx, log)
}

// updateBalanceWithLock 更新余额（带悲观锁，内部方法）
func (r *balanceRepository) updateBalanceWithLock(ctx context.Context, tx gdb.TX, userID, accountTypeID int64, updateFunc func(*entity.AccountBalanceEntity) error) (*entity.AccountBalanceEntity, error) {
	var result *entity.AccountBalanceEntity

	err := db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		// 使用悲观锁查询余额
		balance, err := r.balanceDao.GetByUserAndAccountTypeForUpdate(ctx, tx, userID, accountTypeID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			// 真正的数据库错误（排除记录不存在的情况）
			return err
		}

		if balance == nil {
			return gerror.New("账户不存在")
		}

		// 执行余额更新逻辑
		if err := updateFunc(balance); err != nil {
			return err
		}

		// 更新数据库
		data := map[string]interface{}{
			"balance":        balance.Balance,
			"frozen_balance": balance.FrozenBalance,
			"total_amount":   balance.TotalAmount,
			"version":        gdb.Raw("version + 1"),
		}
		if err := r.balanceDao.UpdateById(ctx, tx, balance.Id, data); err != nil {
			return err
		}

		result = balance
		return nil
	})

	return result, err
}

// FreezeBalance 冻结余额并记录变动日志
// 用途：withdraw_repository调用（提现申请时冻结余额）
// 说明：执行冻结操作（可用→冻结）并记录日志
func (r *balanceRepository) FreezeBalance(ctx context.Context, tx gdb.TX, params *BalanceOperationParams) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		// 账户不存在则创建（余额为0），避免新资产首次提现直接报“账户不存在”
		existingBalance, err := r.balanceDao.GetByUserAndAccountTypeForUpdate(ctx, tx, params.UserID, params.AccountTypeID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if existingBalance == nil {
			accountType := params.AccountType
			if accountType == "" {
				accountTypeEntity, err := r.accountTypeDao.GetById(ctx, params.AccountTypeID)
				if err != nil {
					return gerror.Wrapf(err, "查询账户类型失败: accountTypeID=%d", params.AccountTypeID)
				}
				if accountTypeEntity != nil {
					accountType = accountTypeEntity.Type
				}
			}

			newBalance := &entity.AccountBalanceEntity{
				UserId:        params.UserID,
				AccountTypeId: params.AccountTypeID,
				AccountType:   accountType,
				Symbol:        params.Symbol,
				Balance:       decimal.Zero,
				FrozenBalance: decimal.Zero,
				TotalAmount:   decimal.Zero,
				Version:       0,
			}
			if err := r.balanceDao.Create(ctx, tx, newBalance); err != nil {
				return err
			}
		}

		// 使用带锁的更新方法
		balance, err := r.updateBalanceWithLock(ctx, tx, params.UserID, params.AccountTypeID, func(balance *entity.AccountBalanceEntity) error {
			// 防止“同一个 account_type_id 被复用到不同 symbol”导致误扣
			if params.Symbol != "" && balance.Symbol != "" && balance.Symbol != params.Symbol {
				return gerror.Newf("账户资产不匹配：account_type_id=%d, db_symbol=%s, req_symbol=%s", params.AccountTypeID, balance.Symbol, params.Symbol)
			}
			// 检查可用余额是否足够
			if balance.Balance.LessThan(params.Amount) {
				return gerror.New("余额不足")
			}
			// 可用余额转为冻结余额
			balance.Balance = balance.Balance.Sub(params.Amount)
			balance.FrozenBalance = balance.FrozenBalance.Add(params.Amount)
			return nil
		})
		if err != nil {
			return err
		}

		// 记录变动日志
		return r.createBalanceChangeLog(ctx, tx, params, balance.Balance.Add(params.Amount), balance.Balance)
	})
}

// UnfreezeBalance 解冻余额并记录变动日志
// 用途：withdraw_repository调用（提现失败或审核拒绝时解冻余额）
// 说明：执行解冻操作（冻结→可用）并记录日志
func (r *balanceRepository) UnfreezeBalance(ctx context.Context, tx gdb.TX, params *BalanceOperationParams) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		// 使用带锁的更新方法
		balance, err := r.updateBalanceWithLock(ctx, tx, params.UserID, params.AccountTypeID, func(balance *entity.AccountBalanceEntity) error {
			// 检查冻结余额是否足够
			if balance.FrozenBalance.LessThan(params.Amount) {
				return gerror.New("冻结余额不足")
			}
			// 冻结余额转为可用余额
			balance.FrozenBalance = balance.FrozenBalance.Sub(params.Amount)
			balance.Balance = balance.Balance.Add(params.Amount)
			return nil
		})
		if err != nil {
			return err
		}

		// 记录变动日志
		return r.createBalanceChangeLog(ctx, tx, params, balance.Balance.Sub(params.Amount), balance.Balance)
	})
}

// UnfreezeAndDeduct 解冻余额并出金，记录变动日志
// 用途：withdraw_repository调用（提现成功时扣除冻结余额）
// 说明：直接扣除冻结余额和总额，并记录日志
func (r *balanceRepository) UnfreezeAndDeduct(ctx context.Context, tx gdb.TX, params *BalanceOperationParams) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		// 使用带锁的更新方法
		balance, err := r.updateBalanceWithLock(ctx, tx, params.UserID, params.AccountTypeID, func(balance *entity.AccountBalanceEntity) error {
			// 检查冻结余额是否足够
			if balance.FrozenBalance.LessThan(params.Amount) {
				return gerror.New("冻结余额不足")
			}
			// 扣除冻结余额和总额
			balance.FrozenBalance = balance.FrozenBalance.Sub(params.Amount)
			balance.TotalAmount = balance.TotalAmount.Sub(params.Amount)
			return nil
		})
		if err != nil {
			return err
		}

		// 记录变动日志
		return r.createBalanceChangeLog(ctx, tx, params, balance.FrozenBalance.Add(params.Amount), balance.FrozenBalance)
	})
}

// Deposit 入金操作：增加余额并记录日志
// 用途：exchange_repository、recharge_repository调用
// 说明：增加可用余额和总额，如果账户不存在则自动创建，并记录日志
func (r *balanceRepository) Deposit(ctx context.Context, tx gdb.TX, params *BalanceOperationParams) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		// 使用悲观锁查询账户
		existingBalance, err := r.balanceDao.GetByUserAndAccountTypeForUpdate(ctx, tx, params.UserID, params.AccountTypeID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			// 真正的数据库错误（排除记录不存在的情况）
			return err
		}

		// 账户不存在则创建
		if existingBalance == nil {
			// 通过 accountTypeDao 获取 account_type
			accountType := params.AccountType
			if accountType == "" {
				accountTypeEntity, err := r.accountTypeDao.GetById(ctx, params.AccountTypeID)
				if err != nil {
					return gerror.Wrapf(err, "查询账户类型失败: accountTypeID=%d", params.AccountTypeID)
				}
				if accountTypeEntity != nil {
					accountType = accountTypeEntity.Type
				}
			}

			newBalance := &entity.AccountBalanceEntity{
				UserId:        params.UserID,
				AccountTypeId: params.AccountTypeID,
				AccountType:   accountType,
				Symbol:        params.Symbol,
				Balance:       params.Amount,
				FrozenBalance: decimal.Zero,
				TotalAmount:   params.Amount,
				Version:       0,
			}
			if err := r.balanceDao.Create(ctx, tx, newBalance); err != nil {
				return err
			}
			// 记录变动日志（从0到amount）
			return r.createBalanceChangeLog(ctx, tx, params, decimal.Zero, params.Amount)
		}

		// 账户已存在则增加余额
		beforeBalance := existingBalance.Balance
		existingBalance.Balance = existingBalance.Balance.Add(params.Amount)
		existingBalance.TotalAmount = existingBalance.TotalAmount.Add(params.Amount)

		data := map[string]interface{}{
			"balance":      existingBalance.Balance,
			"total_amount": existingBalance.TotalAmount,
			"version":      gdb.Raw("version + 1"),
		}
		if err := r.balanceDao.UpdateById(ctx, tx, existingBalance.Id, data); err != nil {
			return err
		}

		// 记录变动日志
		return r.createBalanceChangeLog(ctx, tx, params, beforeBalance, existingBalance.Balance)
	})
}

// Withdraw 出金操作：减少余额并记录日志
// 用途：exchange_repository调用
// 说明：减少可用余额和总额，会检查余额是否足够，并记录日志
func (r *balanceRepository) Withdraw(ctx context.Context, tx gdb.TX, params *BalanceOperationParams) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		// 使用带锁的更新方法
		balance, err := r.updateBalanceWithLock(ctx, tx, params.UserID, params.AccountTypeID, func(balance *entity.AccountBalanceEntity) error {
			// 检查可用余额是否足够
			if balance.Balance.LessThan(params.Amount) {
				return gerror.New("余额不足")
			}
			// 减少可用余额和总额
			balance.Balance = balance.Balance.Sub(params.Amount)
			balance.TotalAmount = balance.TotalAmount.Sub(params.Amount)
			return nil
		})
		if err != nil {
			return err
		}

		// 记录变动日志
		return r.createBalanceChangeLog(ctx, tx, params, balance.Balance.Add(params.Amount), balance.Balance)
	})
}

// GetBalance 查询单个账户余额
// 用途：查询指定用户的指定账户类型余额
// 参数：
//   - userID: 用户ID（必填）
//   - accountTypeID: 账户类型ID（必填）
//
// 返回：单个账户余额实体
// 示例：GetBalance(ctx, 123, 1) // 查询用户123的账户类型1的余额
func (r *balanceRepository) GetBalance(ctx context.Context, userID, accountTypeID int64) (*entity.AccountBalanceEntity, error) {
	return r.balanceDao.GetByUserAndAccountType(ctx, userID, accountTypeID)
}

// GetBalanceForUpdate 查询账户余额并加锁（用于事务中检查余额）
func (r *balanceRepository) GetBalanceForUpdate(ctx context.Context, tx gdb.TX, userID, accountTypeID int64) (*entity.AccountBalanceEntity, error) {
	return r.balanceDao.GetByUserAndAccountTypeForUpdate(ctx, tx, userID, accountTypeID)
}

func (r *balanceRepository) EnsureAccountBalance(ctx context.Context, tx gdb.TX, userID, accountTypeID int64, accountType, symbol string) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		existing, err := r.balanceDao.GetByUserAndAccountTypeForUpdate(ctx, tx, userID, accountTypeID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if existing != nil {
			return nil
		}

		b := &entity.AccountBalanceEntity{
			UserId:        userID,
			AccountTypeId: accountTypeID,
			AccountType:   accountType,
			Symbol:        symbol,
			Balance:       decimal.Zero,
			FrozenBalance: decimal.Zero,
			TotalAmount:   decimal.Zero,
			Version:       0,
		}
		return r.balanceDao.Create(ctx, tx, b)
	})
}

// GetBalanceList 查询用户余额列表
// 用途：查询用户的余额列表，支持多字段组合筛选
// 参数：
//   - UserID: 用户ID（必填）
//   - AccountTypeID: 账户类型ID（可选，不传则返回所有账户类型）
//   - Symbol: 资产符号（可选，不传则返回所有资产）
//   - MinBalance: 最小余额（可选，用于过滤小额资产）
//
// 返回：符合条件的账户余额列表
// 示例：
//  1. 查询用户所有余额：GetBalanceList(ctx, &GetBalanceListReq{UserID: 123})
//  2. 查询用户ETH余额：GetBalanceList(ctx, &GetBalanceListReq{UserID: 123, Symbol: "ETH"})
//  3. 查询余额≥0.01的账户：GetBalanceList(ctx, &GetBalanceListReq{UserID: 123, MinBalance: "0.01"})
func (r *balanceRepository) GetBalanceList(ctx context.Context, req *GetBalanceListReq) ([]*entity.AccountBalanceEntity, error) {
	// 查询用户所有余额
	balances, err := r.balanceDao.GetListByUserId(ctx, req.UserID)
	if err != nil {
		return nil, err
	}

	// 如果没有筛选条件，直接返回所有余额
	if req.AccountTypeID == nil && req.Symbol == "" && req.MinBalance == "" {
		return balances, nil
	}

	// 应用筛选条件
	var result []*entity.AccountBalanceEntity
	for _, balance := range balances {
		// 按账户类型筛选
		if req.AccountTypeID != nil && *req.AccountTypeID > 0 && balance.AccountTypeId != *req.AccountTypeID {
			continue
		}

		// 按资产符号筛选
		if req.Symbol != "" && balance.Symbol != req.Symbol {
			continue
		}

		// 按最小余额筛选
		if req.MinBalance != "" {
			minBal, err := decimal.NewFromString(req.MinBalance)
			if err == nil && balance.Balance.LessThan(minBal) {
				continue
			}
		}

		result = append(result, balance)
	}

	return result, nil
}

// GetUserBalanceBySymbol 根据用户ID、资产符号和账户类型查询余额
// 用途：asset_service调用，用于获取用户APG用户余额
func (r *balanceRepository) GetUserBalanceBySymbol(ctx context.Context, userID int64, symbol, accountType string) (*entity.AccountBalanceEntity, error) {
	// 1. 根据账户类型获取账户类型ID
	accountTypeID, err := r.accountTypeDao.GetIdByTypeAndSymbol(ctx, accountType, symbol)
	if err != nil {
		return nil, err
	}
	if accountTypeID == 0 {
		return nil, nil // 账户类型不存在
	}

	// 2. 查询用户余额
	return r.balanceDao.GetByUserAndAccountType(ctx, userID, accountTypeID)
}

// GetBalanceChangeLogs 查询用户余额变动日志
// 用途：balance_service调用
// 说明：支持按账户类型、资产符号、变动类型筛选，分页查询
func (r *balanceRepository) GetBalanceChangeLogs(ctx context.Context, req *GetBalanceChangeLogsReq) (*GetBalanceChangeLogsRes, error) {
	// 构建查询请求
	queryReq := &model.QueryBalanceLogsReq{
		UserID:        req.UserID,
		AccountTypeID: req.AccountTypeID,
		Symbol:        req.Symbol,
		ChangeType:    req.ChangeType,
		Page:          req.Page,
		PageSize:      req.PageSize,
	}

	// 查询日志列表
	list, total, err := r.balanceChangeLogDao.GetUserLogs(ctx, queryReq)
	if err != nil {
		return nil, err
	}

	return &GetBalanceChangeLogsRes{
		List:  list,
		Total: total,
	}, nil
}
