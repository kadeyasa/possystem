package models

import (
	"time"
)

type Transaction struct {
	ID                    uint              `gorm:"primaryKey" json:"id"`
	OutletID              uint              `json:"outlet_id"`
	CashierID             uint              `json:"cashier_id"`
	CashierName           string            `gorm:"column:cashier_name" json:"cashier_name"`
	Total                 float64           `json:"total"`
	Tax                   float64           `json:"tax"`
	Discount              float64           `json:"discount"`
	PaymentMethod         string            `json:"payment_method"`
	Note                  string            `json:"note"`
	JournalEntryID        *uint             `json:"journal_entry_id"`
	AccountingSyncStatus  string            `json:"accounting_sync_status"`
	AccountingSyncError   string            `json:"accounting_sync_error"`
	AccountingSyncedAt    *time.Time        `json:"accounting_synced_at"`
	AccountingIdempotency string            `gorm:"column:accounting_idempotency_key" json:"accounting_idempotency_key"`
	CreatedAt             time.Time         `json:"created_at"`
	UpdatedAt             time.Time         `json:"updated_at"`
	Status                uint              `json:"status"`
	DocumentStatus        string            `gorm:"column:document_status" json:"document_status"`
	VoidedAt              *time.Time        `gorm:"column:voided_at" json:"voided_at"`
	VoidReason            string            `gorm:"column:void_reason" json:"void_reason"`
	VoidApprovalRequestID *uint             `gorm:"column:void_approval_request_id" json:"void_approval_request_id"`
	Items                 []TransactionItem `gorm:"foreignKey:TransactionID" json:"items"`
}

func (Transaction) TableName() string {
	return "tbltransactions"
}

type TransactionItem struct {
	ID            uint     `gorm:"primaryKey" json:"id"`
	TransactionID uint     `json:"transaction_id"`
	ProductID     uint     `json:"product_id"`
	Quantity      int      `json:"quantity"`
	UnitPrice     float64  `json:"unit_price"`
	Total         float64  `json:"total"`
	VariantID     *int64   `json:"variant_id"`                                        // nullable
	Variant       *Variant `gorm:"foreignKey:VariantID;references:ID" json:"variant"` // relasi ke struct di atas
	Product       Product  `gorm:"foreignKey:ProductID" json:"product"`
}

func (TransactionItem) TableName() string {
	return "tbltransaction_items"
}
