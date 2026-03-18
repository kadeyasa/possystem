package models

import "time"

type AccountMapping struct {
	ID              uint      `gorm:"primaryKey;column:id" json:"mapping_id"`
	OutletID        uint      `gorm:"column:outlet_id;not null;default:0" json:"outlet_id"`
	AccountID       string    `gorm:"column:account_id;size:10;not null" json:"id"`
	TransactionType string    `gorm:"column:transaction_type;size:50;not null" json:"transaction_type"`
	Purpose         string    `gorm:"column:purpose;size:50;not null" json:"purpose"`
	IsActive        bool      `gorm:"column:is_active;not null;default:true" json:"is_active"`
	CreatedAt       time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AccountMapping) TableName() string {
	return "tblaccount_mappings"
}

type AccountMappingRecord struct {
	MappingID       uint   `gorm:"column:mapping_id" json:"mapping_id"`
	ID              string `gorm:"column:id" json:"id"`
	Name            string `gorm:"column:name" json:"name"`
	Category        string `gorm:"column:category" json:"category"`
	IsActive        bool   `gorm:"column:is_active" json:"is_active"`
	OutletID        uint   `gorm:"column:outlet_id" json:"outlet_id"`
	TransactionType string `gorm:"column:transaction_type" json:"transaction_type"`
	Purpose         string `gorm:"column:purpose" json:"purpose"`
}
