package apg

import (
	"context"
	"strings"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

type ITokenInfoDao interface {
	GetAll(ctx context.Context) ([]*entity.ApgMintTokenInfo, error)
	GetEnabled(ctx context.Context) ([]*entity.ApgMintTokenInfo, error)
	GetById(ctx context.Context, id int) (*entity.ApgMintTokenInfo, error)
	GetBySymbol(ctx context.Context, symbol string) (*entity.ApgMintTokenInfo, error)
	GetByAddress(ctx context.Context, address string) (*entity.ApgMintTokenInfo, error)
	Upsert(ctx context.Context, token *entity.ApgMintTokenInfo) error
	UpdateIconUrl(ctx context.Context, id int64, iconUrl string) error
	BatchUpsert(ctx context.Context, tokens []*entity.ApgMintTokenInfo) error
	BatchUpdatePrices(ctx context.Context, prices map[string]string) error
	DeleteNotInAddresses(ctx context.Context, addresses []string) (int64, error)
	DeleteBySymbols(ctx context.Context, symbols []string) (int64, error)
}

type tokenInfoDao struct{}

var TokenInfo ITokenInfoDao = &tokenInfoDao{}

func (d *tokenInfoDao) GetAll(ctx context.Context) ([]*entity.ApgMintTokenInfo, error) {
	var tokens []*entity.ApgMintTokenInfo
	err := db.GetDB().Model("apg_mint_token_info").
		Order("sort_order ASC, id ASC").
		Scan(&tokens)
	return tokens, err
}

func (d *tokenInfoDao) GetEnabled(ctx context.Context) ([]*entity.ApgMintTokenInfo, error) {
	var tokens []*entity.ApgMintTokenInfo
	err := db.GetDB().Model("apg_mint_token_info").
		Where("status", 1).
		Order("sort_order ASC, id ASC").
		Scan(&tokens)
	return tokens, err
}

func (d *tokenInfoDao) GetById(ctx context.Context, id int) (*entity.ApgMintTokenInfo, error) {
	var token *entity.ApgMintTokenInfo
	err := db.GetDB().Model("apg_mint_token_info").Ctx(ctx).
		Where("id", id).
		Scan(&token)
	return token, err
}

func (d *tokenInfoDao) GetBySymbol(ctx context.Context, symbol string) (*entity.ApgMintTokenInfo, error) {
	var token *entity.ApgMintTokenInfo
	err := db.GetDB().Model("apg_mint_token_info").Ctx(ctx).
		Where("symbol", symbol).
		Scan(&token)
	if err != nil {
		return nil, err
	}
	return token, nil
}

func (d *tokenInfoDao) GetByAddress(ctx context.Context, address string) (*entity.ApgMintTokenInfo, error) {
	var token *entity.ApgMintTokenInfo
	err := db.GetDB().Model("apg_mint_token_info").Ctx(ctx).
		Where("LOWER(contract_address) = LOWER(?)", address).
		Scan(&token)
	if err != nil {
		return nil, err
	}
	return token, nil
}

func (d *tokenInfoDao) Upsert(ctx context.Context, token *entity.ApgMintTokenInfo) error {
	existing, err := d.GetByAddress(ctx, token.ContractAddress)
	if err != nil {
		return err
	}

	if existing != nil {
		_, err = db.GetDB().Model("apg_mint_token_info").Ctx(ctx).
			Where("id", existing.Id).
			Data(gdb.Map{
				"symbol":     token.Symbol,
				"name":       token.Name,
				"icon_url":   token.IconUrl,
				"decimals":   token.Decimals,
				"price":      token.Price,
				"sort_order": token.SortOrder,
				"status":     token.Status,
			}).
			Update()
	} else {
		_, err = db.GetDB().Model("apg_mint_token_info").Ctx(ctx).
			Data(gdb.Map{
				"symbol":           token.Symbol,
				"name":             token.Name,
				"contract_address": token.ContractAddress,
				"icon_url":         token.IconUrl,
				"decimals":         token.Decimals,
				"price":            token.Price,
				"sort_order":       token.SortOrder,
				"status":           token.Status,
			}).
			Insert()
	}
	return err
}

func (d *tokenInfoDao) UpdateIconUrl(ctx context.Context, id int64, iconUrl string) error {
	_, err := db.GetDB().Model("apg_mint_token_info").
		Where("id", id).
		Update(gdb.Map{
			"icon_url": iconUrl,
		})
	return err
}

func (d *tokenInfoDao) BatchUpsert(ctx context.Context, tokens []*entity.ApgMintTokenInfo) error {
	for _, token := range tokens {
		if err := d.Upsert(ctx, token); err != nil {
			return err
		}
	}
	return nil
}

func (d *tokenInfoDao) BatchUpdatePrices(ctx context.Context, prices map[string]string) error {
	for addr, price := range prices {
		_, err := db.GetDB().Model("apg_mint_token_info").Ctx(ctx).
			Where("LOWER(contract_address) = LOWER(?)", addr).
			Update(gdb.Map{"price": price})
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *tokenInfoDao) DeleteNotInAddresses(ctx context.Context, addresses []string) (int64, error) {
	if len(addresses) == 0 {
		result, err := db.GetDB().Model("apg_mint_token_info").Ctx(ctx).Delete()
		if err != nil {
			return 0, err
		}
		return result.RowsAffected()
	}

	lowerAddresses := make([]interface{}, len(addresses))
	for i, addr := range addresses {
		lowerAddresses[i] = strings.ToLower(addr)
	}

	result, err := db.GetDB().Model("apg_mint_token_info").Ctx(ctx).
		Where("LOWER(contract_address) NOT IN (?)", lowerAddresses).
		Delete()
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *tokenInfoDao) DeleteBySymbols(ctx context.Context, symbols []string) (int64, error) {
	if len(symbols) == 0 {
		return 0, nil
	}

	symbolsInterface := make([]interface{}, len(symbols))
	for i, s := range symbols {
		symbolsInterface[i] = s
	}

	result, err := db.GetDB().Model("apg_mint_token_info").Ctx(ctx).
		Where("symbol IN (?)", symbolsInterface).
		Delete()
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
