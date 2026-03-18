package models

import (
	"time"

	"gorm.io/gorm"
)

type VendorBill struct {
	ID                    uint           `gorm:"primaryKey" json:"id"`
	OutletID              uint           `json:"outlet_id"`
	PurchaseID            *uint          `json:"purchase_id,omitempty"`
	VendorName            string         `gorm:"size:200" json:"vendor_name"`
	BillNo                string         `gorm:"size:200" json:"bill_no"`
	BillDate              time.Time      `json:"bill_date"`
	DueDate               *time.Time     `json:"due_date,omitempty"`
	BillType              string         `gorm:"size:32" json:"bill_type"`
	AccountPurpose        string         `gorm:"size:64" json:"account_purpose"`
	UsePrepaid            bool           `json:"use_prepaid"`
	Subtotal              float64        `json:"subtotal"`
	TaxAmount             float64        `json:"tax_amount"`
	TotalAmount           float64        `json:"total_amount"`
	PaidAmount            float64        `json:"paid_amount"`
	OutstandingAmount     float64        `json:"outstanding_amount"`
	Status                string         `gorm:"size:16" json:"status"`
	JournalEntryID        *uint          `json:"journal_entry_id"`
	AccountingSyncStatus  string         `json:"accounting_sync_status"`
	AccountingSyncError   string         `json:"accounting_sync_error"`
	AccountingSyncedAt    *time.Time     `json:"accounting_synced_at"`
	AccountingIdempotency string         `gorm:"column:accounting_idempotency_key" json:"accounting_idempotency_key"`
	Note                  string         `gorm:"size:200" json:"note"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
	DeletedAt             gorm.DeletedAt `gorm:"index" json:"-"`
}

func (VendorBill) TableName() string {
	return "tblvendor_bills"
}
