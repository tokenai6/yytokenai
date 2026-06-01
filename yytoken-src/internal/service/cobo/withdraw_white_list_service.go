package cobo

import (
	"context"

	"XWFrame/internal/entity"
	"XWFrame/internal/repository"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// WhiteListItemWithStats 白名单项（含统计信息）
type WhiteListItemWithStats struct {
	*entity.WithdrawWhiteListEntity
	TotalWithdrawCount int    `json:"total_withdraw_count"`  // 累计提现次数
	TotalWithdrawAmount string `json:"total_withdraw_amount"` // 累计提现金额
}

// WithdrawWhiteListService 提现地址白名单服务接口
type WithdrawWhiteListService interface {
	// GetList 获取白名单列表
	GetList(ctx context.Context, page, pageSize int) ([]*WhiteListItemWithStats, int, error)
	// GetByUserID 根据用户ID获取白名单记录
	GetByUserID(ctx context.Context, userID int64) (*entity.WithdrawWhiteListEntity, error)
	// Create 添加用户到白名单
	Create(ctx context.Context, userID int64, remark string) error
	// Delete 从白名单移除用户
	Delete(ctx context.Context, userID int64) error
	// IsInWhiteList 检查用户是否在白名单中
	IsInWhiteList(ctx context.Context, userID int64) bool
}

// withdrawWhiteListService 提现地址白名单服务实现
type withdrawWhiteListService struct {
	withdrawWhiteListRepo repository.IWithdrawWhiteListRepository
	userRepo              repository.IUserRepository
}

// NewWithdrawWhiteListService 创建提现地址白名单服务
func NewWithdrawWhiteListService() WithdrawWhiteListService {
	return &withdrawWhiteListService{
		withdrawWhiteListRepo: repository.NewWithdrawWhiteListRepository(),
		userRepo:              repository.NewUserRepository(),
	}
}

// GetList 获取白名单列表
func (s *withdrawWhiteListService) GetList(ctx context.Context, page, pageSize int) ([]*WhiteListItemWithStats, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	var list []*entity.WithdrawWhiteListEntity
	err := g.DB().Model("withdraw_white_list").Ctx(ctx).
		OrderDesc("created_at").
		Limit((page-1)*pageSize, pageSize).
		Scan(&list)
	if err != nil {
		return nil, 0, gerror.Wrap(err, "query withdraw white list failed")
	}

	count, err := g.DB().Model("withdraw_white_list").Ctx(ctx).Count()
	if err != nil {
		return nil, 0, gerror.Wrap(err, "count withdraw white list failed")
	}

	// 查询每个用户的提现统计
	result := make([]*WhiteListItemWithStats, 0, len(list))
	for _, item := range list {
		stats := &WhiteListItemWithStats{
			WithdrawWhiteListEntity: item,
			TotalWithdrawCount:      0,
			TotalWithdrawAmount:     "0",
		}

		// 查询累计提现次数和金额
		var statsRecord gdb.Record
		statsRecord, err = g.DB().GetOne(ctx, `
			SELECT COUNT(*) as cnt, COALESCE(SUM(amount), 0) as total_amount
			FROM cobo_withdraw_request
			WHERE user_id = ? AND status NOT IN (2, 5)
		`, item.UserID)
		if err != nil {
			g.Log().Warningf(ctx, "[WithdrawWhiteList] query withdraw stats failed: user_id=%d, err=%v", item.UserID, err)
		} else {
			stats.TotalWithdrawCount = statsRecord["cnt"].Int()
			amount := statsRecord["total_amount"].String()
			if amount != "" && amount != "0" {
				d, err := decimal.NewFromString(amount)
				if err == nil {
					stats.TotalWithdrawAmount = d.String()
				}
			}
		}

		result = append(result, stats)
	}

	return result, count, nil
}

// GetByUserID 根据用户ID获取白名单记录
func (s *withdrawWhiteListService) GetByUserID(ctx context.Context, userID int64) (*entity.WithdrawWhiteListEntity, error) {
	return s.withdrawWhiteListRepo.GetByUserID(ctx, userID)
}

// Create 添加用户到白名单
func (s *withdrawWhiteListService) Create(ctx context.Context, userID int64, remark string) error {
	// 检查用户是否存在
	user, err := s.userRepo.GetUserById(ctx, userID)
	if err != nil {
		return gerror.Wrap(err, "get user info failed")
	}
	if user == nil {
		return gerror.New("user not found")
	}

	// 检查是否已在白名单中
	existing, err := s.withdrawWhiteListRepo.GetByUserID(ctx, userID)
	if err != nil {
		return gerror.Wrap(err, "check white list failed")
	}
	if existing != nil {
		return gerror.New("user already in white list")
	}

	return s.withdrawWhiteListRepo.Create(ctx, userID, remark)
}

// Delete 从白名单移除用户
func (s *withdrawWhiteListService) Delete(ctx context.Context, userID int64) error {
	return s.withdrawWhiteListRepo.Delete(ctx, userID)
}

// IsInWhiteList 检查用户是否在白名单中
func (s *withdrawWhiteListService) IsInWhiteList(ctx context.Context, userID int64) bool {
	record, err := s.withdrawWhiteListRepo.GetByUserID(ctx, userID)
	if err != nil {
		g.Log().Debugf(ctx, "[WithdrawWhiteList] check white list failed: user_id=%d, err=%v", userID, err)
		return false
	}
	return record != nil
}
