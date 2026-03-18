package models

import (
	"time"

	"gorm.io/gorm"
)

type Purchase struct {
	ID                    uint           `gorm:"primaryKey" json:"id"`
	OutletID              uint           `json:"outlet_id"`
	SupplierName          string         `gorm:"size:200" json:"supplier_name"`
	InvoiceNumber         string         `gorm:"size:200" json:"invoice_number"`
	PurchaseDate          time.Time      `json:"purchase_date"`
	DueDate               *time.Time     `json:"due_date,omitempty"`
	Total                 float64        `json:"total"`
	PaidAmount            float64        `json:"paid_amount"`
	OutstandingAmount     float64        `json:"outstanding_amount"`
	PaymentMethod         string         `gorm:"size:64" json:"payment_method"`
	PaymentStatus         string         `gorm:"size:16" json:"payment_status"`
	LinkedVendorBillID    *uint          `json:"linked_vendor_bill_id,omitempty"`
	Note                  string         `gorm:"size:200" json:"note"`
	JournalEntryID        *uint          `json:"journal_entry_id"` // optional FK
	AccountingSyncStatus  string         `json:"accounting_sync_status"`
	AccountingSyncError   string         `json:"accounting_sync_error"`
	AccountingSyncedAt    *time.Time     `json:"accounting_synced_at"`
	AccountingIdempotency string         `gorm:"column:accounting_idempotency_key" json:"accounting_idempotency_key"`
	PurchaseItems         []PurchaseItem `gorm:"foreignKey:PurchaseID" json:"items"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Purchase) TableName() string {
	return "tblpurchases"
}
