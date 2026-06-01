package token

import (
	"context"
	"strings"

	"XWFrame/internal/repository"
	"XWFrame/internal/service/token/model"

	"github.com/gogf/gf/v2/errors/gerror"
)

// ITokenService 代币服务接口
type ITokenService interface {
	// SearchBySymbol 根据代币符号模糊搜索（忽略大小写）
	SearchBySymbol(ctx context.Context, userID int64, symbol string) ([]*model.TokenInfo, error)

	// SubmitToken 用户提交代币，仅影响当前用户的资产列表显示
	SubmitToken(ctx context.Context, userID int64, symbol string) error

	// HideToken 用户隐藏代币，支持覆盖白名单
	HideToken(ctx context.Context, userID int64, symbol string) error
}

type tokenService struct {
	tokenConfigRepo      repository.ITokenConfigRepository
	tokenWhitelistRepo   repository.ITokenWhitelistRepository
	tokenUserDisplayRepo repository.ITokenUserDisplayRepository
	tokenUserHideRepo    repository.ITokenUserHideRepository
}

var tokenServiceInstance *tokenService

func Svc() ITokenService {
	if tokenServiceInstance == nil {
		tokenServiceInstance = &tokenService{
			tokenConfigRepo:      repository.NewTokenConfigRepository(),
			tokenWhitelistRepo:   repository.NewTokenWhitelistRepository(),
			tokenUserDisplayRepo: repository.NewTokenUserDisplayRepository(),
			tokenUserHideRepo:    repository.NewTokenUserHideRepository(),
		}
	}
	return tokenServiceInstance
}

// SearchBySymbol 根据代币符号模糊搜索（忽略大小写）
func (s *tokenService) SearchBySymbol(ctx context.Context, userID int64, symbol string) ([]*model.TokenInfo, error) {
	if userID <= 0 {
		return nil, gerror.New("user not authenticated")
	}

	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, gerror.New("symbol is required")
	}

	configs, err := s.tokenConfigRepo.SearchBySymbol(ctx, symbol)
	if err != nil {
		return nil, err
	}

	whitelistItems, err := s.tokenWhitelistRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	whitelistMap := make(map[string]bool, len(whitelistItems))
	for _, item := range whitelistItems {
		whitelistMap[strings.ToUpper(item.Symbol)] = true
	}

	userShownMap, err := s.tokenUserDisplayRepo.GetShownSymbolMap(ctx, userID)
	if err != nil {
		return nil, err
	}

	userHideMap, err := s.tokenUserHideRepo.GetHiddenSymbolMap(ctx, userID)
	if err != nil {
		return nil, err
	}

	infos := make([]*model.TokenInfo, 0, len(configs))
	for _, cfg := range configs {
		symbolUpper := strings.ToUpper(cfg.Symbol)
		infos = append(infos, &model.TokenInfo{
			Id:              cfg.Id,
			Symbol:          cfg.Symbol,
			Name:            cfg.Name,
			ContractAddress: cfg.ContractAddress,
			TokenImgUrl:     cfg.TokenImgUrl,
			Decimals:        cfg.Decimals,
			Shown:           (whitelistMap[symbolUpper] || userShownMap[symbolUpper]) && !userHideMap[symbolUpper],
		})
	}
	return infos, nil
}

// SubmitToken 用户提交代币，仅影响当前用户的资产列表显示
func (s *tokenService) SubmitToken(ctx context.Context, userID int64, symbol string) error {
	if userID <= 0 {
		return gerror.New("user not authenticated")
	}

	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return gerror.New("symbol is required")
	}

	// 查找代币配置
	configs, err := s.tokenConfigRepo.SearchBySymbol(ctx, symbol)
	if err != nil {
		return err
	}

	var targetID int64
	for _, cfg := range configs {
		if strings.EqualFold(cfg.Symbol, symbol) {
			targetID = cfg.Id
			break
		}
	}

	if targetID == 0 {
		return gerror.New("token not found")
	}

	// 如果用户在 hide 列表中，先移除隐藏
	_ = s.tokenUserHideRepo.DeleteByUserIDAndSymbol(ctx, userID, symbol)

	existing, err := s.tokenUserDisplayRepo.GetByUserIDAndSymbol(ctx, userID, symbol)
	if err != nil {
		return err
	}
	if existing != nil {
		return gerror.New("token already shown")
	}

	return s.tokenUserDisplayRepo.Create(ctx, userID, symbol)
}

// HideToken 用户隐藏代币，支持覆盖白名单
func (s *tokenService) HideToken(ctx context.Context, userID int64, symbol string) error {
	if userID <= 0 {
		return gerror.New("user not authenticated")
	}

	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return gerror.New("symbol is required")
	}

	// 查找代币配置
	configs, err := s.tokenConfigRepo.SearchBySymbol(ctx, symbol)
	if err != nil {
		return err
	}

	var found bool
	for _, cfg := range configs {
		if strings.EqualFold(cfg.Symbol, symbol) {
			found = true
			break
		}
	}

	if !found {
		return gerror.New("token not found")
	}

	// 如果用户在 display 列表中，先移除显示
	_ = s.tokenUserDisplayRepo.DeleteByUserIDAndSymbol(ctx, userID, symbol)

	existing, err := s.tokenUserHideRepo.GetByUserIDAndSymbol(ctx, userID, symbol)
	if err != nil {
		return err
	}
	if existing != nil {
		return gerror.New("token already hidden")
	}

	return s.tokenUserHideRepo.Create(ctx, userID, symbol)
}
