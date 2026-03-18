package models

import (
	"time"

	"gorm.io/gorm"
)

type MasterWaktu struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	OutletID    uint           `json:"outlet_id"`
	Description string         `json:"description"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (MasterWaktu) TableName() string {
	return "tblmasterwaktu"
}
