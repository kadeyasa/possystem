package models

import (
	"time"

	"gorm.io/gorm"
)

type StockOpname struct {
	ID                    uint              `gorm:"primaryKey" json:"id"`
	OutletID              uint              `json:"outlet_id"`
	OpnameDate            time.Time         `json:"opname_date"`
	Status                string            `gorm:"size:16" json:"status"`
	JournalEntryID        *uint             `json:"journal_entry_id"`
	AccountingSyncStatus  string            `json:"accounting_sync_status"`
	AccountingSyncError   string            `json:"accounting_sync_error"`
	AccountingSyncedAt    *time.Time        `json:"accounting_synced_at"`
	AccountingIdempotency string            `gorm:"column:accounting_idempotency_key" json:"accounting_idempotency_key"`
	Note                  string            `gorm:"size:200" json:"note"`
	Items                 []StockOpnameItem `gorm:"foreignKey:OpnameID" json:"items,omitempty"`
	CreatedAt             time.Time         `json:"created_at"`
	UpdatedAt             time.Time         `json:"updated_at"`
	DeletedAt             gorm.DeletedAt    `gorm:"index" json:"-"`
}

func (StockOpname) TableName() string {
	return "tblstock_opnames"
}

type StockOpnameItem struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	OpnameID      uint      `json:"opname_id"`
	ProductID     uint      `json:"product_id"`
	SystemStock   int       `json:"system_stock"`
	ActualStock   int       `json:"actual_stock"`
	DifferenceQty int       `json:"difference_qty"`
	UnitCost      float64   `json:"unit_cost"`
	TotalCost     float64   `json:"total_cost"`
	Note          string    `gorm:"size:200" json:"note"`
	Product       Product   `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (StockOpnameItem) TableName() string {
	return "tblstock_opname_items"
}
