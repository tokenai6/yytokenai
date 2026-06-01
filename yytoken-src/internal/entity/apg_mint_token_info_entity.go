package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

type ApgMintTokenInfo struct {
	Id              int64       `json:"id" orm:"id,primary"`
	Symbol          string      `json:"symbol" orm:"symbol"`
	Name            string      `json:"name" orm:"name"`
	ContractAddress string      `json:"contract_address" orm:"contract_address"`
	IconUrl         string      `json:"icon_url" orm:"icon_url"`
	Decimals        int         `json:"decimals" orm:"decimals"`
	Price           string      `json:"price" orm:"price"`
	SortOrder       int         `json:"sort_order" orm:"sort_order"`
	Status          int         `json:"status" orm:"status"`
	CreatedAt       *gtime.Time `json:"created_at" orm:"created_at"`
	UpdatedAt       *gtime.Time `json:"updated_at" orm:"updated_at"`
}
