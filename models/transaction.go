package models

import (
	"time"
)

type Transaction struct {
	ID                    uint                    `gorm:"primaryKey" json:"id"`
	OutletID              uint                    `json:"outlet_id"`
	CashierID             uint                    `json:"cashier_id"`
	CashierName           string                  `gorm:"column:cashier_name" json:"cashier_name"`
	CustomerID            *uint                   `gorm:"column:customer_id" json:"customer_id"`
	CustomerName          string                  `gorm:"column:customer_name" json:"customer_name"`
	Subtotal              float64                 `gorm:"column:subtotal" json:"subtotal"`
	DiscountPercent       float64                 `gorm:"column:discount_percent" json:"discount_percent"`
	Total                 float64                 `json:"total"`
	ServicePercent        float64                 `gorm:"column:service_percent" json:"service_percent"`
	Service               float64                 `gorm:"column:service" json:"service"`
	TaxPercent            float64                 `gorm:"column:tax_percent" json:"tax_percent"`
	Tax                   float64                 `json:"tax"`
	Discount              float64                 `json:"discount"`
	GrandTotal            float64                 `gorm:"column:grand_total" json:"grand_total"`
	SettlementAmount      float64                 `gorm:"column:settlement_amount" json:"settlement_amount"`
	RefundCreditAmount    float64                 `gorm:"column:refund_credit_amount" json:"refund_credit_amount"`
	RefundCreditCode      string                  `gorm:"column:refund_credit_code" json:"refund_credit_code"`
	PaymentMethod         string                  `json:"payment_method"`
	Note                  string                  `json:"note"`
	JournalEntryID        *uint                   `json:"journal_entry_id"`
	AccountingSyncStatus  string                  `json:"accounting_sync_status"`
	AccountingSyncError   string                  `json:"accounting_sync_error"`
	AccountingSyncedAt    *time.Time              `json:"accounting_synced_at"`
	AccountingIdempotency string                  `gorm:"column:accounting_idempotency_key" json:"accounting_idempotency_key"`
	CreatedAt             time.Time               `json:"created_at"`
	UpdatedAt             time.Time               `json:"updated_at"`
	Status                uint                    `json:"status"`
	DocumentStatus        string                  `gorm:"column:document_status" json:"document_status"`
	VoidedAt              *time.Time              `gorm:"column:voided_at" json:"voided_at"`
	VoidReason            string                  `gorm:"column:void_reason" json:"void_reason"`
	VoidApprovalRequestID *uint                   `gorm:"column:void_approval_request_id" json:"void_approval_request_id"`
	ApprovalLocked        bool                    `gorm:"-" json:"approval_locked"`
	ApprovalLockType      string                  `gorm:"-" json:"approval_lock_type,omitempty"`
	ApprovalLockStatus    string                  `gorm:"-" json:"approval_lock_status,omitempty"`
	ApprovalLockMessage   string                  `gorm:"-" json:"approval_lock_message,omitempty"`
	RefundableItems       []RefundableItemSummary `gorm:"-" json:"refundable_items,omitempty"`
	Items                 []TransactionItem       `gorm:"foreignKey:TransactionID" json:"items"`
}

type RefundableItemSummary struct {
	ProductID          uint    `json:"product_id"`
	ProductName        string  `json:"product_name"`
	SoldQuantity       int     `json:"sold_quantity"`
	RefundedQuantity   int     `json:"refunded_quantity"`
	RefundableQuantity int     `json:"refundable_quantity"`
	UnitPrice          float64 `json:"unit_price"`
	Total              float64 `json:"total"`
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
