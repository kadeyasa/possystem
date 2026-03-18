package models

import "time"

type StaffShiftClosing struct {
	ID                      uint      `gorm:"primaryKey" json:"id"`
	OutletID                uint      `gorm:"column:outlet_id;index" json:"outlet_id"`
	CashierID               uint      `gorm:"column:cashier_id;index" json:"cashier_id"`
	CashierName             string    `gorm:"column:cashier_name" json:"cashier_name"`
	PeriodStart             time.Time `gorm:"column:period_start" json:"period_start"`
	PeriodEnd               time.Time `gorm:"column:period_end" json:"period_end"`
	ClosedAt                time.Time `gorm:"column:closed_at" json:"closed_at"`
	TransactionCount        int64     `gorm:"column:transaction_count" json:"transaction_count"`
	GrossSalesAmount        float64   `gorm:"column:gross_sales_amount" json:"gross_sales_amount"`
	DiscountAmount          float64   `gorm:"column:discount_amount" json:"discount_amount"`
	TaxAmount               float64   `gorm:"column:tax_amount" json:"tax_amount"`
	NetSalesAmount          float64   `gorm:"column:net_sales_amount" json:"net_sales_amount"`
	CashTransactionCount    int64     `gorm:"column:cash_transaction_count" json:"cash_transaction_count"`
	CashSalesAmount         float64   `gorm:"column:cash_sales_amount" json:"cash_sales_amount"`
	NonCashTransactionCount int64     `gorm:"column:non_cash_transaction_count" json:"non_cash_transaction_count"`
	NonCashSalesAmount      float64   `gorm:"column:non_cash_sales_amount" json:"non_cash_sales_amount"`
	ExpectedCashAmount      float64   `gorm:"column:expected_cash_amount" json:"expected_cash_amount"`
	ActualCashAmount        float64   `gorm:"column:actual_cash_amount" json:"actual_cash_amount"`
	CashDifferenceAmount    float64   `gorm:"column:cash_difference_amount" json:"cash_difference_amount"`
	Note                    string    `gorm:"column:note" json:"note"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

func (StaffShiftClosing) TableName() string {
	return "tblstaff_shift_closings"
}

type StaffShiftClosingPreview struct {
	OutletID                uint       `json:"outlet_id"`
	CashierID               uint       `json:"cashier_id"`
	CashierName             string     `json:"cashier_name"`
	PeriodStart             time.Time  `json:"period_start"`
	PeriodEnd               time.Time  `json:"period_end"`
	LastClosedAt            *time.Time `json:"last_closed_at,omitempty"`
	TransactionCount        int64      `json:"transaction_count"`
	GrossSalesAmount        float64    `json:"gross_sales_amount"`
	DiscountAmount          float64    `json:"discount_amount"`
	TaxAmount               float64    `json:"tax_amount"`
	NetSalesAmount          float64    `json:"net_sales_amount"`
	CashTransactionCount    int64      `json:"cash_transaction_count"`
	CashSalesAmount         float64    `json:"cash_sales_amount"`
	NonCashTransactionCount int64      `json:"non_cash_transaction_count"`
	NonCashSalesAmount      float64    `json:"non_cash_sales_amount"`
	ExpectedCashAmount      float64    `json:"expected_cash_amount"`
}
