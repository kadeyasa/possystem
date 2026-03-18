package models

import (
	"time"

	"gorm.io/gorm"
)

type Vendor struct {
	ID                     uint           `gorm:"primaryKey" json:"id"`
	OutletID               uint           `json:"outlet_id"`
	VendorName             string         `gorm:"size:200" json:"vendor_name"`
	ContactName            string         `gorm:"size:200" json:"contact_name"`
	Phone                  string         `gorm:"size:64" json:"phone"`
	Email                  string         `gorm:"size:200" json:"email"`
	Address                string         `gorm:"type:text" json:"address"`
	DefaultPaymentTermDays int            `json:"default_payment_term_days"`
	IsActive               bool           `json:"is_active"`
	Note                   string         `gorm:"size:200" json:"note"`
	CreatedAt              time.Time      `json:"created_at"`
	UpdatedAt              time.Time      `json:"updated_at"`
	DeletedAt              gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Vendor) TableName() string {
	return "tblvendors"
}
