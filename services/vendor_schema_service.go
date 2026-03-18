package services

import (
	"fmt"
	"sync"

	"gorm.io/gorm"
)

var (
	ensureVendorSchemaOnce sync.Once
	ensureVendorSchemaErr  error
)

func EnsureVendorMasterSchema(db *gorm.DB) error {
	ensureVendorSchemaOnce.Do(func() {
		statements := []string{
			`CREATE TABLE IF NOT EXISTS public.tblvendors (
				id BIGSERIAL PRIMARY KEY,
				outlet_id BIGINT NOT NULL,
				vendor_name VARCHAR(200) NOT NULL,
				contact_name VARCHAR(200),
				phone VARCHAR(64),
				email VARCHAR(200),
				address TEXT,
				default_payment_term_days INTEGER NOT NULL DEFAULT 0,
				is_active BOOLEAN NOT NULL DEFAULT TRUE,
				note VARCHAR(200),
				created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
				updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
				deleted_at TIMESTAMP WITHOUT TIME ZONE
			)`,
			`CREATE INDEX IF NOT EXISTS idx_tblvendors_outlet_name ON tblvendors (outlet_id, vendor_name)`,
			`CREATE INDEX IF NOT EXISTS idx_tblvendors_outlet_active ON tblvendors (outlet_id, is_active)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_tblvendors_unique_name_per_outlet
				ON tblvendors (outlet_id, LOWER(TRIM(vendor_name)))
				WHERE deleted_at IS NULL`,
		}

		for _, statement := range statements {
			if err := db.Exec(statement).Error; err != nil {
				ensureVendorSchemaErr = fmt.Errorf("failed to ensure vendor master schema: %w", err)
				return
			}
		}
	})

	return ensureVendorSchemaErr
}
