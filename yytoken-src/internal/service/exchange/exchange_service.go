package exchange

import (
	"context"
	"fmt"

	"XWFrame/internal/frame/config"
	"XWFrame/internal/frame/consts"
	frameModel "XWFrame/internal/frame/model"
	"XWFrame/internal/repository"
	"XWFrame/internal/service/exchange/model"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/gogf/gf/v2/util/grand"
	"github.com/shopspring/decimal"
)

// IExchangeService 兑换服务接口
type IExchangeService interface {
	// Exchange 兑换
	Exchange(ctx context.Context, req *model.ExchangeReq) error

	// GetList 获取兑换列表
	GetList(ctx context.Context, req *model.ExchangeListReq) (*model.ExchangeListRes, error)

	// GetRate 获取兑换汇率（指定兑换对）
	GetRate(ctx context.Context, fromSymbol, toSymbol string) (*model.ExchangeRateRes, error)

	// GetAllRates 获取所有兑换汇率
	GetAllRates(ctx context.Context) (map[string]decimal.Decimal, error)
}

// exchangeService 兑换服务实现
type exchangeService struct {
	exchangeRepo repository.IExchangeRepository
}

var exchangeServiceInstance IExchangeService

// GetExchangeService 获取兑换服务实例
func GetExchangeService() IExchangeService {
	if exchangeServiceInstance == nil {
		exchangeServiceInstance = &exchangeService{
			exchangeRepo: repository.NewExchangeRepository(),
		}
	}
	return exchangeServiceInstance
}

// Exchange 兑换
func (s *exchangeService) Exchange(ctx context.Context, req *model.ExchangeReq) error {
	// 1. 查询账户类型获取Symbol
	fromSymbol, err := s.exchangeRepo.GetSymbolByAccountTypeId(ctx, req.FromAccountTypeId)
	if err != nil {
		return err
	}
	toSymbol, err := s.exchangeRepo.GetSymbolByAccountTypeId(ctx, req.ToAccountTypeId)
	if err != nil {
		return err
	}

	// 2. 获取配置
	cfg := config.GetBalanceConfig()

	// 3. 获取兑换汇率（根据兑换对）
	exchangeRate := config.GetExchangeRate(fromSymbol, toSymbol)
	toAmount := req.FromAmount.Mul(exchangeRate)

	// 4. 计算手续费
	fee := config.CalculateFee(req.FromAmount, cfg.Exchange.FeeType, cfg.Exchange.FeeValue)
	feeRate := cfg.Exchange.FeeValue

	// 5. 生成兑换订单号
	orderNo := s.generateOrderNo("EXC")

	// 6. 在事务中处理兑换
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 调用repository处理兑换业务
		return s.exchangeRepo.ProcessExchange(ctx, tx, &repository.ExchangeParams{
			UserID:            req.UserId,
			FromAccountTypeID: req.FromAccountTypeId,
			ToAccountTypeID:   req.ToAccountTypeId,
			FromSymbol:        fromSymbol,
			ToSymbol:          toSymbol,
			FromAmount:        req.FromAmount,
			ToAmount:          toAmount,
			ExchangeRate:      exchangeRate,
			Fee:               fee,
			FeeRate:           feeRate,
			OrderNo:           orderNo,
			OperatorID:        req.UserId,
			OperatorType:      consts.OperatorTypeUser,
		})
	})
}

// GetList 获取兑换列表
func (s *exchangeService) GetList(ctx context.Context, req *model.ExchangeListReq) (*model.ExchangeListRes, error) {
	repoReq := &repository.GetExchangeListReq{
		PageReq: frameModel.PageReq{
			Page:     req.Page,
			PageSize: req.PageSize,
		},
		UserId:       req.UserId,
		ExchangeType: req.ExchangeType,
	}

	repoRes, err := s.exchangeRepo.GetExchangeRecordList(ctx, repoReq)
	if err != nil {
		return nil, err
	}

	// 转换entity到service model
	list := make([]*model.ExchangeRecord, 0, len(repoRes.List))
	for _, item := range repoRes.List {
		list = append(list, &model.ExchangeRecord{
			Id:                item.Id,
			UserId:            item.UserId,
			OrderNo:           item.OrderNo,
			ExchangeType:      item.ExchangeType,
			FromAccountTypeId: item.FromAccountTypeId,
			FromSymbol:        item.FromSymbol,
			FromAmount:        item.FromAmount,
			ToAccountTypeId:   item.ToAccountTypeId,
			ToSymbol:          item.ToSymbol,
			ToAmount:          item.ToAmount,
			ExchangeRate:      item.ExchangeRate,
			Fee:               item.Fee,
			FeeRate:           item.FeeRate,
			Status:            item.Status,
			Remark:            item.Remark,
			CreatedAt:         item.CreatedAt,
			UpdatedAt:         item.UpdatedAt,
		})
	}

	return &model.ExchangeListRes{
		List:     list,
		Total:    repoRes.Total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// GetRate 获取兑换汇率（指定兑换对）
func (s *exchangeService) GetRate(ctx context.Context, fromSymbol, toSymbol string) (*model.ExchangeRateRes, error) {
	cfg := config.GetBalanceConfig()

	// 获取指定兑换对的汇率
	exchangeRate := config.GetExchangeRate(fromSymbol, toSymbol)

	return &model.ExchangeRateRes{
		FromSymbol:   fromSymbol,
		ToSymbol:     toSymbol,
		ExchangeRate: exchangeRate,
		FeeRate:      cfg.Exchange.FeeValue,
	}, nil
}

// GetAllRates 获取所有兑换汇率
func (s *exchangeService) GetAllRates(ctx context.Context) (map[string]decimal.Decimal, error) {
	cfg := config.GetBalanceConfig()
	return cfg.Exchange.ExchangeRates, nil
}

// generateOrderNo 生成订单号
func (s *exchangeService) generateOrderNo(prefix string) string {
	return fmt.Sprintf("%s%s%s", prefix, gtime.Now().Format("YmdHis"), grand.S(6))
}
