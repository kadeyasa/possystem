package models

import "time"

type Refund struct {
	ID                    uint         `gorm:"primaryKey" json:"id"`
	TransactionID         uint         `json:"transaction_id"`
	OutletID              uint         `json:"outlet_id"`
	CashierID             uint         `json:"cashier_id"`
	CashierName           string       `gorm:"column:cashier_name" json:"cashier_name"`
	RefundTotal           float64      `json:"refund_total"`
	Note                  string       `json:"note"`
	JournalEntryID        *uint        `json:"journal_entry_id"`
	AccountingSyncStatus  string       `json:"accounting_sync_status"`
	AccountingSyncError   string       `json:"accounting_sync_error"`
	AccountingSyncedAt    *time.Time   `json:"accounting_synced_at"`
	AccountingIdempotency string       `gorm:"column:accounting_idempotency_key" json:"accounting_idempotency_key"`
	CreatedAt             time.Time    `json:"created_at"`
	Items                 []RefundItem `gorm:"foreignKey:RefundID" json:"items"`
}

type RefundItem struct {
	ID        uint    `gorm:"primaryKey" json:"id"`
	RefundID  uint    `json:"refund_id"`
	ProductID uint    `json:"product_id"`
	Quantity  int     `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
	Total     float64 `json:"total"`
	Product   Product `gorm:"foreignKey:ProductID" json:"product,omitempty"`
}

func (Refund) TableName() string {
	return "tblrefunds"
}

func (RefundItem) TableName() string {
	return "tblrefund_items"
}
