package models

import (
	"time"

	"gorm.io/gorm"
)

type OperationalExpense struct {
	ID                    uint           `gorm:"primaryKey" json:"id"`
	OutletID              uint           `json:"outlet_id"`
	ExpenseDate           time.Time      `json:"expense_date"`
	ExpenseCategory       string         `gorm:"size:64" json:"expense_category"`
	AccountPurpose        string         `gorm:"size:64" json:"account_purpose"`
	ReferenceNo           string         `gorm:"size:200" json:"reference_no"`
	PaymentMethod         string         `gorm:"size:64" json:"payment_method"`
	Amount                float64        `json:"amount"`
	VendorName            string         `gorm:"size:200" json:"vendor_name"`
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

func (OperationalExpense) TableName() string {
	return "tbloperational_expenses"
}
