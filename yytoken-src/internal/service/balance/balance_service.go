package balance

import (
	"context"

	"XWFrame/internal/frame/model"
	"XWFrame/internal/repository"
	balanceModel "XWFrame/internal/service/balance/model"
)

// IBalanceService 余额服务接口
type IBalanceService interface {
	// GetBalanceChangeLogs 查询用户余额变动日志
	GetBalanceChangeLogs(ctx context.Context, req *GetBalanceChangeLogsReq) (*GetBalanceChangeLogsRes, error)
}

// balanceService 余额服务实现
type balanceService struct {
	balanceRepo repository.IBalanceRepository
}

// NewBalanceService 创建余额服务实例
func NewBalanceService() IBalanceService {
	return &balanceService{
		balanceRepo: repository.NewBalanceRepository(),
	}
}

// GetBalanceChangeLogsReq 查询余额变动日志请求
type GetBalanceChangeLogsReq struct {
	UserID        int64  `json:"userId"`
	AccountTypeID *int64 `json:"accountTypeId"` // 可选
	Symbol        string `json:"symbol"`        // 可选
	ChangeType    string `json:"changeType"`    // 可选
	Page          int    `json:"page"`
	PageSize      int    `json:"pageSize"`
}

// GetBalanceChangeLogsRes 查询余额变动日志响应
type GetBalanceChangeLogsRes struct {
	PageInfo model.PageRes                    `json:"pageInfo"`
	List     []*balanceModel.BalanceChangeLog `json:"list"`
}

// GetBalanceChangeLogs 查询用户余额变动日志
func (s *balanceService) GetBalanceChangeLogs(ctx context.Context, req *GetBalanceChangeLogsReq) (*GetBalanceChangeLogsRes, error) {
	// 构建repository层查询请求
	repoReq := &repository.GetBalanceChangeLogsReq{
		UserID:        req.UserID,
		AccountTypeID: req.AccountTypeID,
		Symbol:        req.Symbol,
		ChangeType:    req.ChangeType,
		Page:          req.Page,
		PageSize:      req.PageSize,
	}

	// 调用repository层查询
	repoRes, err := s.balanceRepo.GetBalanceChangeLogs(ctx, repoReq)
	if err != nil {
		return nil, err
	}

	// 计算总页数
	pages := 0
	if req.PageSize > 0 {
		pages = (repoRes.Total + req.PageSize - 1) / req.PageSize
	}

	return &GetBalanceChangeLogsRes{
		PageInfo: model.PageRes{
			Page:     req.Page,
			PageSize: req.PageSize,
			Total:    repoRes.Total,
			Pages:    pages,
		},
		List: repoRes.List,
	}, nil
}
