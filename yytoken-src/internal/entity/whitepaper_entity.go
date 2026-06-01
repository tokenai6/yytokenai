package entity

import "time"

// WhitepaperEntity 白皮书实体
type WhitepaperEntity struct {
	Id        int64     `json:"id" orm:"id,primary"`
	Language  string    `json:"language" orm:"language"`
	Url       string    `json:"url" orm:"url"`
	SortOrder int       `json:"sort_order" orm:"sort_order"`
	IsEnabled bool      `json:"is_enabled" orm:"is_enabled"`
	CreatedAt time.Time `json:"created_at" orm:"created_at"`
	UpdatedAt time.Time `json:"updated_at" orm:"updated_at"`
}

func (WhitepaperEntity) TableName() string {
	return "whitepaper"
}
