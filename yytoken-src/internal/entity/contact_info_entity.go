package entity

import "time"

// ContactInfoEntity 联系方式实体
type ContactInfoEntity struct {
	Id        int64     `json:"id" orm:"id,primary"`
	Title     string    `json:"title" orm:"title"`
	Url       string    `json:"url" orm:"url"`
	IconUrl   string    `json:"icon_url" orm:"icon_url"`
	SortOrder int       `json:"sort_order" orm:"sort_order"`
	IsEnabled bool      `json:"is_enabled" orm:"is_enabled"`
	CreatedAt time.Time `json:"created_at" orm:"created_at"`
	UpdatedAt time.Time `json:"updated_at" orm:"updated_at"`
}

func (ContactInfoEntity) TableName() string {
	return "contact_info"
}
