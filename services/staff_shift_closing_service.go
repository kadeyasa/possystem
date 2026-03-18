package services

import (
	"errors"

	"github.com/kadeyasa/possystem/models"
	"gorm.io/gorm"
)

func EnsureStaffShiftClosingSchema(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not initialized")
	}

	if err := db.AutoMigrate(&models.StaffShiftClosing{}); err != nil {
		return err
	}

	statements := []string{
		`CREATE INDEX IF NOT EXISTS idx_tblstaff_shift_closings_outlet_cashier_closed_at ON tblstaff_shift_closings (outlet_id, cashier_id, closed_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_tblstaff_shift_closings_cashier_period ON tblstaff_shift_closings (cashier_id, period_start, period_end)`,
		`ALTER TABLE tblstaff_shift_closings ALTER COLUMN transaction_count SET DEFAULT 0`,
		`ALTER TABLE tblstaff_shift_closings ALTER COLUMN gross_sales_amount SET DEFAULT 0`,
		`ALTER TABLE tblstaff_shift_closings ALTER COLUMN discount_amount SET DEFAULT 0`,
		`ALTER TABLE tblstaff_shift_closings ALTER COLUMN tax_amount SET DEFAULT 0`,
		`ALTER TABLE tblstaff_shift_closings ALTER COLUMN net_sales_amount SET DEFAULT 0`,
		`ALTER TABLE tblstaff_shift_closings ALTER COLUMN cash_transaction_count SET DEFAULT 0`,
		`ALTER TABLE tblstaff_shift_closings ALTER COLUMN cash_sales_amount SET DEFAULT 0`,
		`ALTER TABLE tblstaff_shift_closings ALTER COLUMN non_cash_transaction_count SET DEFAULT 0`,
		`ALTER TABLE tblstaff_shift_closings ALTER COLUMN non_cash_sales_amount SET DEFAULT 0`,
		`ALTER TABLE tblstaff_shift_closings ALTER COLUMN expected_cash_amount SET DEFAULT 0`,
		`ALTER TABLE tblstaff_shift_closings ALTER COLUMN actual_cash_amount SET DEFAULT 0`,
		`ALTER TABLE tblstaff_shift_closings ALTER COLUMN cash_difference_amount SET DEFAULT 0`,
	}

	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}

	return nil
}
