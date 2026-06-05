package models

import "time"

type Refund struct {
	ID                       uint         `gorm:"primaryKey" json:"id"`
	RefundNumber             string       `gorm:"column:refund_number" json:"refund_number"`
	TransactionID            uint         `json:"transaction_id"`
	OutletID                 uint         `json:"outlet_id"`
	CashierID                uint         `json:"cashier_id"`
	CashierName              string       `gorm:"column:cashier_name" json:"cashier_name"`
	CustomerID               *uint        `gorm:"column:customer_id" json:"customer_id"`
	CustomerName             string       `gorm:"column:customer_name" json:"customer_name"`
	RefundTotal              float64      `json:"refund_total"`
	Note                     string       `json:"note"`
	SettlementType           string       `gorm:"column:settlement_type" json:"settlement_type"`
	SettlementMethod         string       `gorm:"column:settlement_method" json:"settlement_method"`
	SettlementStatus         string       `gorm:"column:settlement_status" json:"settlement_status"`
	StoreCreditCode          string       `gorm:"column:store_credit_code" json:"store_credit_code"`
	ReplacementTransactionID *uint        `gorm:"column:replacement_transaction_id" json:"replacement_transaction_id"`
	ApprovalRequestID        *uint        `gorm:"column:approval_request_id" json:"approval_request_id"`
	JournalEntryID           *uint        `json:"journal_entry_id"`
	AccountingSyncStatus     string       `json:"accounting_sync_status"`
	AccountingSyncError      string       `json:"accounting_sync_error"`
	AccountingSyncedAt       *time.Time   `json:"accounting_synced_at"`
	AccountingIdempotency    string       `gorm:"column:accounting_idempotency_key" json:"accounting_idempotency_key"`
	CreatedAt                time.Time    `json:"created_at"`
	Items                    []RefundItem `gorm:"foreignKey:RefundID" json:"items"`
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
