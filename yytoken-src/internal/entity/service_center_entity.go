package entity

import "time"

// ServiceCenterEntity 服务中心实体
type ServiceCenterEntity struct {
	Id             int64     `json:"id" orm:"id,primary"`
	UserId         int64     `json:"user_id" orm:"user_id"`
	Name           string    `json:"name" orm:"name"`
	PersonInCharge string    `json:"person_in_charge" orm:"person_in_charge"`
	Phone          string    `json:"phone" orm:"phone"`
	Address        string    `json:"address" orm:"address"`
	WalletAddress  string    `json:"wallet_address" orm:"wallet_address"`
	CreatedAt      time.Time `json:"created_at" orm:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" orm:"updated_at"`
}

func (ServiceCenterEntity) TableName() string {
	return "service_center"
}
