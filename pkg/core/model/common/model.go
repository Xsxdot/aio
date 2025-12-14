package common

import (
	"time"

	"gorm.io/gorm"
)

type Model struct {
	ID        int64          `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"deletedAt"`
}

type ModelString struct {
	ID        string         `gorm:"primaryKey json:"id"`
	CreatedAt time.Time      `json:"createdAt" annotation:"创建时间"`
	UpdatedAt time.Time      `json:"updatedAt" annotation:"更新时间"`
	DeletedAt gorm.DeletedAt `json:"deletedAt"`
}
