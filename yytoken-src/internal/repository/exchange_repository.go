package repository

import (
	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/frame/model"
	"context"
	"fmt"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

// ExchangeParams 兑换操作参数
type ExchangeParams struct {
	UserID            int64           // 用户ID
	FromAccountTypeID int64           // 源账户类型ID
	ToAccountTypeID   int64           // 目标账户类型ID
	FromSymbol        string          // 源资产符号
	ToSymbol          string          // 目标资产符号
	FromAmount        decimal.Decimal // 兑换金额（扣除前）
	ToAmount          decimal.Decimal // 兑换所得金额
	ExchangeRate      decimal.Decimal // 兑换汇率
	Fee               decimal.Decimal // 手续费
	FeeRate           decimal.Decimal // 手续费费率
	OrderNo           string          // 订单号
	OperatorID        int64           // 操作人ID
	OperatorType      string          // 操作人类型
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

// IExchangeRepository 兑换仓储接口
type IExchangeRepository interface {
	// GetSymbolByAccountTypeId 根据账户类型ID获取资产符号
	GetSymbolByAccountTypeId(ctx context.Context, accountTypeId int64) (string, error)

	// GetExchangeRecordList 获取兑换记录列表（支持按用户ID、兑换类型筛选）
	GetExchangeRecordList(ctx context.Context, req *GetExchangeListReq) (*GetExchangeListRes, error)

	// ProcessExchange 处理兑换业务（扣除源账户+增加目标账户+创建记录）
	ProcessExchange(ctx context.Context, tx gdb.TX, params *ExchangeParams) error
}

// exchangeRepository 兑换仓储实现
type exchangeRepository struct {
	exchangeDao    dao.IExchangeDao
	accountTypeDao dao.IAccountTypeDao
	balanceRepo    IBalanceRepository
}

// NewExchangeRepository 创建兑换仓储实例
func NewExchangeRepository() IExchangeRepository {
	return &exchangeRepository{
		exchangeDao:    dao.NewExchangeDao(),
		accountTypeDao: dao.NewAccountTypeDao(),
		balanceRepo:    NewBalanceRepository(),
	}
}

// GetSymbolByAccountTypeId 根据账户类型ID获取资产符号
// 用途：exchange_service调用，用于查询兑换资产符号
func (r *exchangeRepository) GetSymbolByAccountTypeId(ctx context.Context, accountTypeId int64) (string, error) {
	return r.accountTypeDao.GetSymbolById(ctx, accountTypeId)
}

// GetExchangeRecordList 获取兑换记录列表
// 用途：exchange_service调用，支持分页和条件筛选
func (r *exchangeRepository) GetExchangeRecordList(ctx context.Context, req *GetExchangeListReq) (*GetExchangeListRes, error) {
	daoReq := &dao.GetExchangeListReq{
		PageReq:      req.PageReq,
		UserId:       req.UserId,
		ExchangeType: req.ExchangeType,
	}

	daoRes, err := r.exchangeDao.GetList(ctx, daoReq)
	if err != nil {
		return nil, err
	}

	return &GetExchangeListRes{
		PageRes: daoRes.PageRes,
		List:    daoRes.List,
	}, nil
}

// ProcessExchange 处理兑换业务
// 用途：exchange_service调用
// 说明：
//  1. 创建兑换记录
//  2. 扣除源账户余额（包含手续费）
//  3. 增加目标账户余额
func (r *exchangeRepository) ProcessExchange(ctx context.Context, tx gdb.TX, params *ExchangeParams) error {
	// 1. 创建兑换记录
	record := &entity.ExchangeRecordEntity{
		UserId:            params.UserID,
		OrderNo:           params.OrderNo,
		ExchangeType:      consts.ExchangeTypeIn,
		FromAccountTypeId: params.FromAccountTypeID,
		FromSymbol:        params.FromSymbol,
		FromAmount:        params.FromAmount,
		ToAccountTypeId:   params.ToAccountTypeID,
		ToSymbol:          params.ToSymbol,
		ToAmount:          params.ToAmount,
		ExchangeRate:      params.ExchangeRate,
		Fee:               params.Fee,
		FeeRate:           params.FeeRate,
		Status:            consts.ExchangeStatusSuccess,
		Remark:            "兑换成功",
		CreatedAt:         gtime.Now(),
	}

	err := r.exchangeDao.Create(ctx, tx, record)
	if err != nil {
		return err
	}

	// 2. 扣除源账户余额（包含手续费）
	actualFromAmount := params.FromAmount.Add(params.Fee)
	remark := fmt.Sprintf("兑换：%s -> %s", params.FromSymbol, params.ToSymbol)

	err = r.balanceRepo.Withdraw(ctx, tx, &BalanceOperationParams{
		UserID:         params.UserID,
		AccountTypeID:  params.FromAccountTypeID,
		Symbol:         params.FromSymbol,
		Amount:         actualFromAmount,
		ChangeType:     consts.ChangeTypeExchangeOut,
		RelatedOrderNo: params.OrderNo,
		RelatedId:      record.Id,
		Remark:         remark,
		OperatorID:     params.OperatorID,
		OperatorType:   params.OperatorType,
	})
	if err != nil {
		return err
	}

	// 3. 增加目标账户余额
	err = r.balanceRepo.Deposit(ctx, tx, &BalanceOperationParams{
		UserID:         params.UserID,
		AccountTypeID:  params.ToAccountTypeID,
		Symbol:         params.ToSymbol,
		Amount:         params.ToAmount,
		ChangeType:     consts.ChangeTypeExchangeIn,
		RelatedOrderNo: params.OrderNo,
		RelatedId:      record.Id,
		Remark:         remark,
		OperatorID:     params.OperatorID,
		OperatorType:   params.OperatorType,
	})
	return err
}
