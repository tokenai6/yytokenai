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

// RechargeParams 充值操作参数
type RechargeParams struct {
	UserID          int64           // 用户ID
	AccountTypeID   int64           // 账户类型ID
	Symbol          string          // 资产符号
	OrderNo         string          // 订单号
	RechargeAddress string          // 充值地址
	ChainType       string          // 链类型
	Amount          decimal.Decimal // 充值金额
	TxHash          string          // 交易哈希
	BlockNumber     int64           // 区块高度
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

// IRechargeRepository 充值仓储接口
type IRechargeRepository interface {
	// GetSymbolByAccountTypeId 根据账户类型ID获取资产符号
	GetSymbolByAccountTypeId(ctx context.Context, accountTypeId int64) (string, error)

	// GetRechargeRecordByTxHash 根据交易哈希获取充值记录（防重复处理）
	GetRechargeRecordByTxHash(ctx context.Context, txHash string) (*entity.RechargeRecordEntity, error)

	// GetRechargeRecordByOrderNo 根据订单号获取充值记录
	GetRechargeRecordByOrderNo(ctx context.Context, orderNo string) (*entity.RechargeRecordEntity, error)

	// GetRechargeRecordList 获取充值记录列表（支持按用户ID、资产符号、状态筛选）
	GetRechargeRecordList(ctx context.Context, req *GetRechargeListReq) (*GetRechargeListRes, error)

	// ProcessRecharge 处理充值业务（创建记录+增加余额+更新状态）
	ProcessRecharge(ctx context.Context, tx gdb.TX, params *RechargeParams) error
}

// rechargeRepository 充值仓储实现
type rechargeRepository struct {
	rechargeDao    dao.IRechargeDao
	accountTypeDao dao.IAccountTypeDao
	balanceRepo    IBalanceRepository
}

// NewRechargeRepository 创建充值仓储实例
func NewRechargeRepository() IRechargeRepository {
	return &rechargeRepository{
		rechargeDao:    dao.NewRechargeDao(),
		accountTypeDao: dao.NewAccountTypeDao(),
		balanceRepo:    NewBalanceRepository(),
	}
}

// GetSymbolByAccountTypeId 根据账户类型ID获取资产符号
// 用途：recharge_service调用，用于查询充值资产符号
func (r *rechargeRepository) GetSymbolByAccountTypeId(ctx context.Context, accountTypeId int64) (string, error) {
	return r.accountTypeDao.GetSymbolById(ctx, accountTypeId)
}

// GetRechargeRecordByTxHash 根据交易哈希获取充值记录
// 用途：recharge_service调用，用于防止重复处理同一交易
func (r *rechargeRepository) GetRechargeRecordByTxHash(ctx context.Context, txHash string) (*entity.RechargeRecordEntity, error) {
	return r.rechargeDao.GetByTxHash(ctx, txHash)
}

// GetRechargeRecordByOrderNo 根据订单号获取充值记录
// 用途：recharge_service调用，用于查询指定订单的充值记录
func (r *rechargeRepository) GetRechargeRecordByOrderNo(ctx context.Context, orderNo string) (*entity.RechargeRecordEntity, error) {
	return r.rechargeDao.GetByOrderNo(ctx, orderNo)
}

// GetRechargeRecordList 获取充值记录列表
// 用途：recharge_service调用，支持分页和条件筛选
func (r *rechargeRepository) GetRechargeRecordList(ctx context.Context, req *GetRechargeListReq) (*GetRechargeListRes, error) {
	daoReq := &dao.GetRechargeListReq{
		PageReq: req.PageReq,
		UserId:  req.UserId,
		Symbol:  req.Symbol,
		Status:  req.Status,
	}

	daoRes, err := r.rechargeDao.GetList(ctx, daoReq)
	if err != nil {
		return nil, err
	}

	return &GetRechargeListRes{
		PageRes: daoRes.PageRes,
		List:    daoRes.List,
	}, nil
}

// ProcessRecharge 处理充值业务
// 用途：recharge_service调用
// 说明：
//  1. 创建充值记录（处理中状态）
//  2. 增加用户余额
//  3. 更新充值状态为成功
func (r *rechargeRepository) ProcessRecharge(ctx context.Context, tx gdb.TX, params *RechargeParams) error {
	// 1. 创建充值记录
	record := &entity.RechargeRecordEntity{
		UserId:          params.UserID,
		AccountTypeId:   params.AccountTypeID,
		Symbol:          params.Symbol,
		OrderNo:         params.OrderNo,
		RechargeAddress: params.RechargeAddress,
		ChainType:       params.ChainType,
		Amount:          params.Amount,
		TxHash:          params.TxHash,
		BlockNumber:     params.BlockNumber,
		Confirmations:   0,
		Status:          consts.RechargeStatusProcessing,
		Remark:          "链上充值",
		CreatedAt:       gtime.Now(),
	}

	err := r.rechargeDao.Create(ctx, tx, record)
	if err != nil {
		return err
	}

	// 2. 增加余额
	remark := fmt.Sprintf("链上充值，交易哈希：%s", params.TxHash)
	err = r.balanceRepo.Deposit(ctx, tx, &BalanceOperationParams{
		UserID:         params.UserID,
		AccountTypeID:  params.AccountTypeID,
		Symbol:         params.Symbol,
		Amount:         params.Amount,
		ChangeType:     consts.ChangeTypeRecharge,
		RelatedOrderNo: params.OrderNo,
		RelatedId:      record.Id,
		Remark:         remark,
		OperatorID:     0,
		OperatorType:   consts.OperatorTypeSystem,
	})
	if err != nil {
		return err
	}

	// 3. 更新充值状态为成功
	data := map[string]interface{}{
		"status": consts.RechargeStatusSuccess,
		"remark": "充值成功",
	}
	return r.rechargeDao.UpdateById(ctx, tx, record.Id, data)
}
