package models

import "time"

type InventoryLedger struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	OutletID      uint      `json:"outlet_id"`
	ProductID     uint      `json:"product_id"`
	VariantID     *int64    `json:"variant_id,omitempty"`
	MovementType  string    `gorm:"size:32" json:"movement_type"`
	ReferenceType string    `gorm:"size:32" json:"reference_type"`
	ReferenceID   uint      `json:"reference_id"`
	QuantityIn    int       `json:"quantity_in"`
	QuantityOut   int       `json:"quantity_out"`
	StockBefore   int       `json:"stock_before"`
	StockAfter    int       `json:"stock_after"`
	UnitCost      float64   `json:"unit_cost"`
	TotalCost     float64   `json:"total_cost"`
	Notes         string    `gorm:"size:200" json:"notes"`
	CreatedAt     time.Time `json:"created_at"`
	Product       Product   `gorm:"foreignKey:ProductID" json:"product,omitempty"`
}

func (InventoryLedger) TableName() string {
	return "tblinventory_ledger"
}
