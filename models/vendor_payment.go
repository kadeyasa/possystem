package models

import (
	"time"

	"gorm.io/gorm"
)

type VendorPayment struct {
	ID                    uint                      `gorm:"primaryKey" json:"id"`
	OutletID              uint                      `json:"outlet_id"`
	VendorName            string                    `gorm:"size:200" json:"vendor_name"`
	PaymentNo             string                    `gorm:"size:200" json:"payment_no"`
	PaymentDate           time.Time                 `json:"payment_date"`
	PaymentMethod         string                    `gorm:"size:64" json:"payment_method"`
	Amount                float64                   `json:"amount"`
	Status                string                    `gorm:"size:16" json:"status"`
	JournalEntryID        *uint                     `json:"journal_entry_id"`
	AccountingSyncStatus  string                    `json:"accounting_sync_status"`
	AccountingSyncError   string                    `json:"accounting_sync_error"`
	AccountingSyncedAt    *time.Time                `json:"accounting_synced_at"`
	AccountingIdempotency string                    `gorm:"column:accounting_idempotency_key" json:"accounting_idempotency_key"`
	Note                  string                    `gorm:"size:200" json:"note"`
	Allocations           []VendorPaymentAllocation `gorm:"foreignKey:PaymentID" json:"allocations,omitempty"`
	CreatedAt             time.Time                 `json:"created_at"`
	UpdatedAt             time.Time                 `json:"updated_at"`
	DeletedAt             gorm.DeletedAt            `gorm:"index" json:"-"`
}

func (VendorPayment) TableName() string {
	return "tblvendor_payments"
}

type VendorPaymentAllocation struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	PaymentID       uint       `json:"payment_id"`
	VendorBillID    uint       `json:"vendor_bill_id"`
	AllocatedAmount float64    `json:"allocated_amount"`
	Bill            VendorBill `gorm:"foreignKey:VendorBillID" json:"bill,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (VendorPaymentAllocation) TableName() string {
	return "tblvendor_payment_allocations"
}
