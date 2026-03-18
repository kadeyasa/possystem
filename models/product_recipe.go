package models

import (
	"time"

	"gorm.io/gorm"
)

type ProductRecipe struct {
	ID                  uint           `gorm:"primaryKey" json:"id"`
	OutletID            uint           `json:"outlet_id"`
	ProductID           uint           `json:"product_id"`
	IngredientProductID uint           `json:"ingredient_product_id"`
	QuantityRequired    int            `json:"quantity_required"`
	Note                string         `gorm:"size:200" json:"note"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`
	Product             Product        `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	IngredientProduct   Product        `gorm:"foreignKey:IngredientProductID" json:"ingredient_product,omitempty"`
}

func (ProductRecipe) TableName() string {
	return "tblproduct_recipes"
}

type TransactionInventoryConsumption struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	TransactionID      uint      `json:"transaction_id"`
	SoldProductID      uint      `json:"sold_product_id"`
	InventoryProductID uint      `json:"inventory_product_id"`
	VariantID          *int64    `json:"variant_id,omitempty"`
	ConsumptionType    string    `gorm:"size:32" json:"consumption_type"`
	SoldQuantity       int       `json:"sold_quantity"`
	QuantityPerUnit    int       `json:"quantity_per_unit"`
	QuantityConsumed   int       `json:"quantity_consumed"`
	UnitCost           float64   `json:"unit_cost"`
	TotalCost          float64   `json:"total_cost"`
	Note               string    `gorm:"size:200" json:"note"`
	CreatedAt          time.Time `json:"created_at"`
	SoldProduct        Product   `gorm:"foreignKey:SoldProductID" json:"sold_product,omitempty"`
	InventoryProduct   Product   `gorm:"foreignKey:InventoryProductID" json:"inventory_product,omitempty"`
}

func (TransactionInventoryConsumption) TableName() string {
	return "tbltransaction_inventory_consumptions"
}
