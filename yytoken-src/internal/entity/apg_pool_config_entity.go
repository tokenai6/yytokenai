package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

type ApgPoolConfig struct {
	Id             int64       `json:"id" orm:"id,primary"`
	PoolType       int         `json:"pool_type" orm:"pool_type"`
	PoolName       string      `json:"pool_name" orm:"pool_name"`
	TicketAmount   float64     `json:"ticket_amount" orm:"ticket_amount"`
	MinPlayers     int         `json:"min_players" orm:"min_players"`
	MaxBuyPerRound int         `json:"max_buy_per_round" orm:"max_buy_per_round"`
	Status         int         `json:"status" orm:"status"`
	CreatedAt      *gtime.Time `json:"created_at" orm:"created_at"`
	UpdatedAt      *gtime.Time `json:"updated_at" orm:"updated_at"`
}
