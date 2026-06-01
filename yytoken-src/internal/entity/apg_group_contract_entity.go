package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

type ApgGroupContract struct {
	Id              int64       `json:"id" orm:"id,primary"`
	ContractAddress string      `json:"contract_address" orm:"contract_address"`
	IsActive        bool        `json:"is_active" orm:"is_active"`
	ActivatedAt     *gtime.Time `json:"activated_at" orm:"activated_at"`
	DeactivatedAt   *gtime.Time `json:"deactivated_at" orm:"deactivated_at"`
	CreatedAt       *gtime.Time `json:"created_at" orm:"created_at"`
	UpdatedAt       *gtime.Time `json:"updated_at" orm:"updated_at"`
}
