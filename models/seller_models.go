package models

import (
	"time"

	"gorm.io/gorm"
)

type Seller struct {
	gorm.Model
	Username  string `gorm:"unique;not null"`
	Email     string `gorm:"unique;not null"`
	Number    string `gorm:"unique;not null"`
	Password  string `gorm:"not null"`
	Role      string `gorm:"default:'user'"`
	Status    string `gorm:"default:'active'"`
	CreatedAt time.Time
}
