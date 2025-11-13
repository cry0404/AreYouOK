package model

import (
	"time"

	"gorm.io/gorm"
)


type BaseModel struct {
	//gorm.Model 还是自定义更好一点
	ID        int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time      `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time      `gorm:"not null;default:now()" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"` //这里有部分可以软删除
}
