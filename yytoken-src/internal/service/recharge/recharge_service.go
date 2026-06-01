package recharge

import (
	"context"
	"fmt"

	frameModel "XWFrame/internal/frame/model"
	"XWFrame/internal/repository"
	"XWFrame/internal/service/recharge/model"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/gogf/gf/v2/util/grand"
)

// IRechargeService 充值服务接口
type IRechargeService interface {
	// ProcessRecharge 处理充值（由区块链监听服务调用）
	ProcessRecharge(ctx context.Context, req *model.RechargeReq) error

	// GetList 获取充值列表
	GetList(ctx context.Context, req *model.RechargeListReq) (*model.RechargeListRes, error)

	// GetByOrderNo 根据订单号查询
	GetByOrderNo(ctx context.Context, orderNo string) (*model.RechargeRecord, error)
}

// rechargeService 充值服务实现
type rechargeService struct {
	rechargeRepo repository.IRechargeRepository
}

var rechargeServiceInstance IRechargeService

// GetRechargeService 获取充值服务实例
func GetRechargeService() IRechargeService {
	if rechargeServiceInstance == nil {
		rechargeServiceInstance = &rechargeService{
			rechargeRepo: repository.NewRechargeRepository(),
		}
	}
	return rechargeServiceInstance
}

// ProcessRecharge 处理充值
func (s *rechargeService) ProcessRecharge(ctx context.Context, req *model.RechargeReq) error {
	// 1. 查询账户类型获取Symbol
	symbol, err := s.rechargeRepo.GetSymbolByAccountTypeId(ctx, req.AccountTypeId)
	if err != nil {
		return err
	}

	// 2. 生成充值订单号
	orderNo := s.generateOrderNo("RCH")

	// 3. 检查交易哈希是否已存在（防止重复处理）
	existsRecord, err := s.rechargeRepo.GetRechargeRecordByTxHash(ctx, req.TxHash)
	if err != nil {
		return err
	}
	if existsRecord != nil {
		return gerror.New("该交易已处理，请勿重复操作")
	}

	// 4. 在事务中处理充值
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 调用repository处理充值业务
		return s.rechargeRepo.ProcessRecharge(ctx, tx, &repository.RechargeParams{
			UserID:          req.UserId,
			AccountTypeID:   req.AccountTypeId,
			Symbol:          symbol,
			OrderNo:         orderNo,
			RechargeAddress: req.RechargeAddress,
			ChainType:       req.ChainType,
			Amount:          req.Amount,
			TxHash:          req.TxHash,
			BlockNumber:     req.BlockNumber,
		})
	})
}

// GetList 获取充值列表
func (s *rechargeService) GetList(ctx context.Context, req *model.RechargeListReq) (*model.RechargeListRes, error) {
	repoReq := &repository.GetRechargeListReq{
		PageReq: frameModel.PageReq{
			Page:     req.Page,
			PageSize: req.PageSize,
		},
		UserId: req.UserId,
		Symbol: req.Symbol,
		Status: req.Status,
	}

	repoRes, err := s.rechargeRepo.GetRechargeRecordList(ctx, repoReq)
	if err != nil {
		return nil, err
	}

	// 转换entity到service model
	list := make([]*model.RechargeRecord, 0, len(repoRes.List))
	for _, item := range repoRes.List {
		list = append(list, &model.RechargeRecord{
			Id:              item.Id,
			UserId:          item.UserId,
			AccountTypeId:   item.AccountTypeId,
			Symbol:          item.Symbol,
			OrderNo:         item.OrderNo,
			RechargeAddress: item.RechargeAddress,
			ChainType:       item.ChainType,
			Amount:          item.Amount,
			TxHash:          item.TxHash,
			BlockNumber:     item.BlockNumber,
			Confirmations:   item.Confirmations,
			Status:          item.Status,
			Remark:          item.Remark,
			CreatedAt:       item.CreatedAt,
			UpdatedAt:       item.UpdatedAt,
		})
	}

	return &model.RechargeListRes{
		List:     list,
		Total:    repoRes.Total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// GetByOrderNo 根据订单号查询
func (s *rechargeService) GetByOrderNo(ctx context.Context, orderNo string) (*model.RechargeRecord, error) {
	entityRecord, err := s.rechargeRepo.GetRechargeRecordByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if entityRecord == nil {
		return nil, nil
	}

	// 转换entity到service model
	return &model.RechargeRecord{
		Id:              entityRecord.Id,
		UserId:          entityRecord.UserId,
		AccountTypeId:   entityRecord.AccountTypeId,
		Symbol:          entityRecord.Symbol,
		OrderNo:         entityRecord.OrderNo,
		RechargeAddress: entityRecord.RechargeAddress,
		ChainType:       entityRecord.ChainType,
		Amount:          entityRecord.Amount,
		TxHash:          entityRecord.TxHash,
		BlockNumber:     entityRecord.BlockNumber,
		Confirmations:   entityRecord.Confirmations,
		Status:          entityRecord.Status,
		Remark:          entityRecord.Remark,
		CreatedAt:       entityRecord.CreatedAt,
		UpdatedAt:       entityRecord.UpdatedAt,
	}, nil
}

// generateOrderNo 生成订单号
func (s *rechargeService) generateOrderNo(prefix string) string {
	return fmt.Sprintf("%s%s%s", prefix, gtime.Now().Format("YmdHis"), grand.S(6))
}
