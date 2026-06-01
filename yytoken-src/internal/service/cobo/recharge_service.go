package cobo

import (
	"context"
	"strings"

	"XWFrame/internal/dao"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/repository"
	repo "XWFrame/internal/repository/cobo"
	"XWFrame/internal/service/cobo/model"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/errors/gerror"
)

// RechargeService 充值记录服务接口
type RechargeService interface {
	// GetRechargeRecords 获取用户充值记录
	GetRechargeRecords(ctx context.Context, req *model.GetRechargeRecordsReq) (*model.GetRechargeRecordsRes, error)

	// GetDepositAddress 获取指定币种充值地址
	GetDepositAddress(ctx context.Context, req *model.GetDepositAddressReq) (*model.GetDepositAddressRes, error)
}

type rechargeService struct {
	rechargeRepo          repo.IRechargeRepository
	tokenConfigRepo       repository.ITokenConfigRepository
	userDepositAddressDao *dao.UserDepositAddressDao
}

// NewRechargeService 创建充值服务
func NewRechargeService() RechargeService {
	return &rechargeService{
		rechargeRepo:          repo.NewRechargeRepository(),
		tokenConfigRepo:       repository.NewTokenConfigRepository(),
		userDepositAddressDao: dao.NewUserDepositAddressDao(),
	}
}

// GetRechargeRecords 获取用户充值记录
func (s *rechargeService) GetRechargeRecords(ctx context.Context, req *model.GetRechargeRecordsReq) (*model.GetRechargeRecordsRes, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}

	pageSize := req.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	entities, total, err := s.rechargeRepo.GetByUserID(ctx, req.UserID, req.Symbol, page, pageSize)
	if err != nil {
		return nil, gerror.Wrap(err, "获取充值记录失败")
	}

	list := make([]*model.RechargeRecordItem, 0, len(entities))
	for _, entity := range entities {
		if entity == nil {
			continue
		}

		statusText := model.RechargeStatusText(entity.Status)
		list = append(list, &model.RechargeRecordItem{
			ID:            entity.ID,
			TxHash:        entity.TxHash,
			Amount:        entity.Amount.String(),
			Symbol:        entity.Symbol,
			Status:        entity.Status,
			StatusText:    statusText,
			StatusDesc:    statusText,
			Confirmations: entity.Confirmations,
			CreatedAt:     utils.DBTimestampToUnix(entity.CreatedAt),
		})
	}

	pages := total / int64(pageSize)
	if total%int64(pageSize) > 0 {
		pages++
	}

	return &model.GetRechargeRecordsRes{
		Page:     page,
		PageSize: pageSize,
		Total:    int(total),
		Pages:    int(pages),
		List:     list,
	}, nil
}

// GetDepositAddress 获取指定币种充值地址
func (s *rechargeService) GetDepositAddress(ctx context.Context, req *model.GetDepositAddressReq) (*model.GetDepositAddressRes, error) {
	symbol := strings.ToUpper(strings.TrimSpace(req.Symbol))
	if symbol == "" {
		symbol = "SZPN"
	}

	// USDT/BOX/JU 走 Cobo 托管，每个用户有独立充值地址
	if symbol == "USDT" || symbol == "BOX" || symbol == "JU" {
		if req.UserID == 0 {
			return nil, gerror.New("未登录")
		}
		depositAddr, err := s.userDepositAddressDao.GetByUserId(ctx, req.UserID)
		if err != nil {
			return nil, gerror.Wrapf(err, "获取%s充值地址失败", symbol)
		}
		if depositAddr == nil || !depositAddr.IsValid {
			return nil, gerror.Newf("%s充值地址未生成", symbol)
		}
		address := strings.ToLower(strings.TrimSpace(depositAddr.Address))
		if !utils.IsValidEthereumAddress(address) {
			return nil, gerror.Newf("%s充值地址无效", symbol)
		}
		return &model.GetDepositAddressRes{
			Symbol:  symbol,
			ChainID: depositAddr.ChainID,
			Address: address,
		}, nil
	}

	tokenConfig, err := s.tokenConfigRepo.GetBySymbol(ctx, symbol)
	if err != nil {
		return nil, gerror.Wrapf(err, "获取%s代币配置失败", symbol)
	}
	if tokenConfig == nil || !tokenConfig.IsEnabled {
		return nil, gerror.Newf("unsupported symbol: %s", symbol)
	}

	address := strings.ToLower(strings.TrimSpace(tokenConfig.DetectAddress))
	if address == "" {
		return nil, gerror.Newf("%s充值地址未配置", symbol)
	}
	if !utils.IsValidEthereumAddress(address) {
		return nil, gerror.Newf("%s充值地址配置无效", symbol)
	}

	return &model.GetDepositAddressRes{
		Symbol:  symbol,
		ChainID: consts.SZPNDepositChainID,
		Address: address,
	}, nil
}
