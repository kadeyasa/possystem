package models

import (
	"time"

	"gorm.io/gorm"
)

type StockAdjustment struct {
	ID                    uint                  `gorm:"primaryKey" json:"id"`
	OutletID              uint                  `json:"outlet_id"`
	AdjustmentDate        time.Time             `json:"adjustment_date"`
	Reason                string                `gorm:"size:64" json:"reason"`
	Status                string                `gorm:"size:16" json:"status"`
	JournalEntryID        *uint                 `json:"journal_entry_id"`
	AccountingSyncStatus  string                `json:"accounting_sync_status"`
	AccountingSyncError   string                `json:"accounting_sync_error"`
	AccountingSyncedAt    *time.Time            `json:"accounting_synced_at"`
	AccountingIdempotency string                `gorm:"column:accounting_idempotency_key" json:"accounting_idempotency_key"`
	Note                  string                `gorm:"size:200" json:"note"`
	Items                 []StockAdjustmentItem `gorm:"foreignKey:AdjustmentID" json:"items,omitempty"`
	CreatedAt             time.Time             `json:"created_at"`
	UpdatedAt             time.Time             `json:"updated_at"`
	DeletedAt             gorm.DeletedAt        `gorm:"index" json:"-"`
}

func (StockAdjustment) TableName() string {
	return "tblstock_adjustments"
}

type StockAdjustmentItem struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	AdjustmentID  uint      `json:"adjustment_id"`
	ProductID     uint      `json:"product_id"`
	QuantityDelta int       `json:"quantity_delta"`
	StockBefore   int       `json:"stock_before"`
	StockAfter    int       `json:"stock_after"`
	UnitCost      float64   `json:"unit_cost"`
	TotalCost     float64   `json:"total_cost"`
	Note          string    `gorm:"size:200" json:"note"`
	Product       Product   `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (StockAdjustmentItem) TableName() string {
	return "tblstock_adjustment_items"
}
