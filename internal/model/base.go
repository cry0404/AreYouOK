package model

import (
	"time"

	"gorm.io/gorm"
)

type BaseModel struct {
	CreatedAt time.Time      `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time      `gorm:"not null;default:now()" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
	ID        int64          `gorm:"primaryKey;autoIncrement" json:"id"`
}
