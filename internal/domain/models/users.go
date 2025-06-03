package models

import (
	"time"

	"gorm.io/gorm"
)

type Users struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Email     string         `json:"email" gorm:"unique;not null"`
	Password  string         `json:"-" gorm:"not null"` // "-" means don't include in JSON
	Name      string         `json:"name" gorm:"not null"`
	Role      string         `json:"role" gorm:"not null;default:'user'"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}
