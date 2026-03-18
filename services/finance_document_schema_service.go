package services

import (
	"fmt"
	"sync"

	"gorm.io/gorm"
)

const (
	OperationalExpenseStatusPosted = "posted"

	PurchasePaymentStatusOpen        = "open"
	PurchasePaymentStatusPartialPaid = "partial_paid"
	PurchasePaymentStatusPaid        = "paid"

	VendorBillTypeInventory = "inventory"
	VendorBillTypeExpense   = "expense"
	VendorBillTypeAsset     = "asset"
	VendorBillTypeRent      = "rent"

	VendorBillStatusDraft       = "draft"
	VendorBillStatusOpen        = "open"
	VendorBillStatusPartialPaid = "partial_paid"
	VendorBillStatusPaid        = "paid"
	VendorBillStatusVoid        = "void"

	VendorPaymentStatusPosted = "posted"
	VendorPaymentStatusVoid   = "void"
)

var (
	ensurePOSFinanceSchemaOnce sync.Once
	ensurePOSFinanceSchemaErr  error
)

func EnsurePOSFinanceDocumentSchema(db *gorm.DB) error {
	ensurePOSFinanceSchemaOnce.Do(func() {
		statements := []string{
			`CREATE TABLE IF NOT EXISTS public.tbloperational_expenses (
				id BIGSERIAL PRIMARY KEY,
				outlet_id BIGINT NOT NULL,
				expense_date TIMESTAMP WITHOUT TIME ZONE NOT NULL,
				expense_category VARCHAR(64) NOT NULL,
				account_purpose VARCHAR(64),
				reference_no VARCHAR(200),
				payment_method VARCHAR(64) NOT NULL,
				amount NUMERIC NOT NULL DEFAULT 0,
				vendor_name VARCHAR(200),
				status VARCHAR(16) NOT NULL DEFAULT 'posted',
				journal_entry_id BIGINT,
				accounting_sync_status VARCHAR(16),
				accounting_sync_error TEXT,
				accounting_synced_at TIMESTAMP WITHOUT TIME ZONE,
				accounting_idempotency_key TEXT,
				note VARCHAR(200),
				created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
				updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
				deleted_at TIMESTAMP WITHOUT TIME ZONE
			)`,
			`CREATE INDEX IF NOT EXISTS idx_tbloperational_expenses_outlet_date ON tbloperational_expenses (outlet_id, expense_date DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_tbloperational_expenses_status ON tbloperational_expenses (status)`,
			`ALTER TABLE tblpurchases ADD COLUMN IF NOT EXISTS payment_method VARCHAR(64)`,
			`ALTER TABLE tblpurchases ADD COLUMN IF NOT EXISTS due_date TIMESTAMP WITHOUT TIME ZONE`,
			`ALTER TABLE tblpurchases ADD COLUMN IF NOT EXISTS paid_amount NUMERIC NOT NULL DEFAULT 0`,
			`ALTER TABLE tblpurchases ADD COLUMN IF NOT EXISTS outstanding_amount NUMERIC NOT NULL DEFAULT 0`,
			`ALTER TABLE tblpurchases ADD COLUMN IF NOT EXISTS payment_status VARCHAR(16) NOT NULL DEFAULT 'paid'`,
			`ALTER TABLE tblpurchases ADD COLUMN IF NOT EXISTS linked_vendor_bill_id BIGINT`,
			`UPDATE tblpurchases
				SET payment_method = COALESCE(NULLIF(TRIM(payment_method), ''), 'cash'),
					paid_amount = CASE
						WHEN LOWER(COALESCE(NULLIF(TRIM(payment_method), ''), 'cash')) = 'credit' THEN COALESCE(paid_amount, 0)
						ELSE COALESCE(NULLIF(paid_amount, 0), total, 0)
					END,
					outstanding_amount = CASE
						WHEN LOWER(COALESCE(NULLIF(TRIM(payment_method), ''), 'cash')) = 'credit' THEN COALESCE(NULLIF(outstanding_amount, 0), total, 0)
						ELSE 0
					END,
					payment_status = CASE
						WHEN LOWER(COALESCE(NULLIF(TRIM(payment_method), ''), 'cash')) = 'credit' AND COALESCE(NULLIF(outstanding_amount, 0), total, 0) > 0 THEN 'open'
						ELSE 'paid'
					END
				WHERE payment_method IS NULL
					OR TRIM(payment_method) = ''
					OR paid_amount IS NULL
					OR outstanding_amount IS NULL
					OR payment_status IS NULL
					OR TRIM(payment_status) = ''`,
			`CREATE INDEX IF NOT EXISTS idx_tblpurchases_outlet_date ON tblpurchases (outlet_id, purchase_date DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_tblpurchases_payment_status ON tblpurchases (payment_status)`,
			`CREATE TABLE IF NOT EXISTS public.tblvendor_bills (
				id BIGSERIAL PRIMARY KEY,
				outlet_id BIGINT NOT NULL,
				vendor_name VARCHAR(200),
				bill_no VARCHAR(200),
				bill_date TIMESTAMP WITHOUT TIME ZONE NOT NULL,
				due_date TIMESTAMP WITHOUT TIME ZONE,
				bill_type VARCHAR(32) NOT NULL,
				account_purpose VARCHAR(64),
				use_prepaid BOOLEAN NOT NULL DEFAULT FALSE,
				subtotal NUMERIC NOT NULL DEFAULT 0,
				tax_amount NUMERIC NOT NULL DEFAULT 0,
				total_amount NUMERIC NOT NULL DEFAULT 0,
				paid_amount NUMERIC NOT NULL DEFAULT 0,
				outstanding_amount NUMERIC NOT NULL DEFAULT 0,
				status VARCHAR(16) NOT NULL DEFAULT 'open',
				journal_entry_id BIGINT,
				accounting_sync_status VARCHAR(16),
				accounting_sync_error TEXT,
				accounting_synced_at TIMESTAMP WITHOUT TIME ZONE,
				accounting_idempotency_key TEXT,
				note VARCHAR(200),
				created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
				updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
				deleted_at TIMESTAMP WITHOUT TIME ZONE
			)`,
			`ALTER TABLE tblvendor_bills ADD COLUMN IF NOT EXISTS purchase_id BIGINT`,
			`CREATE INDEX IF NOT EXISTS idx_tblvendor_bills_outlet_date ON tblvendor_bills (outlet_id, bill_date DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_tblvendor_bills_status ON tblvendor_bills (status)`,
			`CREATE INDEX IF NOT EXISTS idx_tblvendor_bills_purchase_id ON tblvendor_bills (purchase_id)`,
			`CREATE TABLE IF NOT EXISTS public.tblvendor_payments (
				id BIGSERIAL PRIMARY KEY,
				outlet_id BIGINT NOT NULL,
				vendor_name VARCHAR(200),
				payment_no VARCHAR(200),
				payment_date TIMESTAMP WITHOUT TIME ZONE NOT NULL,
				payment_method VARCHAR(64) NOT NULL,
				amount NUMERIC NOT NULL DEFAULT 0,
				status VARCHAR(16) NOT NULL DEFAULT 'posted',
				journal_entry_id BIGINT,
				accounting_sync_status VARCHAR(16),
				accounting_sync_error TEXT,
				accounting_synced_at TIMESTAMP WITHOUT TIME ZONE,
				accounting_idempotency_key TEXT,
				note VARCHAR(200),
				created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
				updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
				deleted_at TIMESTAMP WITHOUT TIME ZONE
			)`,
			`CREATE INDEX IF NOT EXISTS idx_tblvendor_payments_outlet_date ON tblvendor_payments (outlet_id, payment_date DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_tblvendor_payments_status ON tblvendor_payments (status)`,
			`CREATE TABLE IF NOT EXISTS public.tblvendor_payment_allocations (
				id BIGSERIAL PRIMARY KEY,
				payment_id BIGINT NOT NULL,
				vendor_bill_id BIGINT NOT NULL,
				allocated_amount NUMERIC NOT NULL DEFAULT 0,
				created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
				updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS idx_tblvendor_payment_allocations_payment_id ON tblvendor_payment_allocations (payment_id)`,
			`CREATE INDEX IF NOT EXISTS idx_tblvendor_payment_allocations_bill_id ON tblvendor_payment_allocations (vendor_bill_id)`,
		}

		for _, statement := range statements {
			if err := db.Exec(statement).Error; err != nil {
				ensurePOSFinanceSchemaErr = fmt.Errorf("failed to ensure POS finance schema: %w", err)
				return
			}
		}
	})

	return ensurePOSFinanceSchemaErr
}

func NormalizeVendorBillType(value string) string {
	switch normalizeAccountToken(value) {
	case VendorBillTypeInventory:
		return VendorBillTypeInventory
	case VendorBillTypeExpense:
		return VendorBillTypeExpense
	case VendorBillTypeAsset:
		return VendorBillTypeAsset
	case VendorBillTypeRent:
		return VendorBillTypeRent
	default:
		return ""
	}
}

func ComputeVendorBillStatus(totalAmount, paidAmount float64) string {
	switch {
	case totalAmount <= 0:
		return VendorBillStatusDraft
	case paidAmount <= 0:
		return VendorBillStatusOpen
	case paidAmount >= totalAmount-0.0001:
		return VendorBillStatusPaid
	default:
		return VendorBillStatusPartialPaid
	}
}

func ComputePurchasePaymentStatus(totalAmount, paidAmount float64) string {
	switch {
	case totalAmount <= 0:
		return PurchasePaymentStatusPaid
	case paidAmount <= 0:
		return PurchasePaymentStatusOpen
	case paidAmount >= totalAmount-0.0001:
		return PurchasePaymentStatusPaid
	default:
		return PurchasePaymentStatusPartialPaid
	}
}
