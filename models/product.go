package models

import (
	"time"

	"gorm.io/gorm"
)

type Product struct {
	ID                    uint           `gorm:"primaryKey" json:"id"`
	OutletID              uint           `json:"outlet_id"`
	CategoryID            uint           `json:"category_id"`         // corrected from string to uint
	Code                  string         `gorm:"size:10" json:"code"` // size adjusted to match DB
	Name                  string         `gorm:"size:200" json:"name"`
	ItemType              string         `gorm:"size:32" json:"item_type"`
	Price                 float64        `json:"price"`
	LastPurchasePrice     float64        `json:"last_purchase_price"`
	Stock                 int            `json:"stock"`
	ImageUrl              string         `json:"image_url"`
	IsActive              bool           `json:"is_active"`
	Satuan                string         `json:"satuan"`
	PurchaseUnit          string         `gorm:"size:32" json:"purchase_unit"`
	PurchaseConversionQty int            `json:"purchase_conversion_qty"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
	DeletedAt             gorm.DeletedAt `gorm:"index" json:"-"`
	Variants              []Variant      `gorm:"foreignKey:ItemID;references:ID"`
}

func (Product) TableName() string {
	return "tblproducts"
}
