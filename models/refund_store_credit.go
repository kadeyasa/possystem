package models

import "time"

type RefundStoreCredit struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	RefundID        uint       `gorm:"column:refund_id" json:"refund_id"`
	OutletID        uint       `gorm:"column:outlet_id" json:"outlet_id"`
	TransactionID   uint       `gorm:"column:transaction_id" json:"transaction_id"`
	CustomerID      *uint      `gorm:"column:customer_id" json:"customer_id"`
	CustomerName    string     `gorm:"column:customer_name" json:"customer_name"`
	CreditCode      string     `gorm:"column:credit_code" json:"credit_code"`
	OriginalAmount  float64    `gorm:"column:original_amount" json:"original_amount"`
	RemainingAmount float64    `gorm:"column:remaining_amount" json:"remaining_amount"`
	Status          string     `gorm:"column:status" json:"status"`
	Note            string     `gorm:"column:note" json:"note"`
	IssuedAt        time.Time  `gorm:"column:issued_at" json:"issued_at"`
	LastUsedAt      *time.Time `gorm:"column:last_used_at" json:"last_used_at"`
	ExpiresAt       *time.Time `gorm:"column:expires_at" json:"expires_at"`
	CreatedAt       time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (RefundStoreCredit) TableName() string {
	return "tblrefund_store_credits"
}

type RefundCreditUsage struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	RefundStoreCreditID uint      `gorm:"column:refund_store_credit_id" json:"refund_store_credit_id"`
	RefundID            *uint     `gorm:"column:refund_id" json:"refund_id"`
	TransactionID       uint      `gorm:"column:transaction_id" json:"transaction_id"`
	OutletID            uint      `gorm:"column:outlet_id" json:"outlet_id"`
	AppliedAmount       float64   `gorm:"column:applied_amount" json:"applied_amount"`
	Note                string    `gorm:"column:note" json:"note"`
	CreatedAt           time.Time `gorm:"column:created_at" json:"created_at"`
}

func (RefundCreditUsage) TableName() string {
	return "tblrefund_credit_usages"
}
