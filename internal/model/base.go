package model

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel 基础模型，包含公共字段
type BaseModel struct {
	ID        int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time      `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time      `gorm:"not null;default:now()" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}
