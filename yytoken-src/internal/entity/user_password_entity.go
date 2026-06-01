package entity

type UserPasswordEntity struct {
	ID           int64  `json:"id" gorm:"primaryKey"`
	UserID      int64  `json:"user_id" gorm:"uniqueIndex;not null"`
	PasswordHash string `json:"password_hash" gorm:"size:255;not null"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func (UserPasswordEntity) TableName() string {
	return "user_password"
}