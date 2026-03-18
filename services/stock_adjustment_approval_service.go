package services

import (
	"fmt"
	"sync"

	"gorm.io/gorm"
)

var (
	ensureStockAdjustmentApprovalSchemaOnce sync.Once
	ensureStockAdjustmentApprovalSchemaErr  error
)

func EnsureStockAdjustmentApprovalSchema(db *gorm.DB) error {
	ensureStockAdjustmentApprovalSchemaOnce.Do(func() {
		statements := []string{
			`CREATE TABLE IF NOT EXISTS public.tblstock_adjustment_approval_requests (
				id BIGSERIAL PRIMARY KEY,
				outlet_id BIGINT NOT NULL,
				status VARCHAR(16) NOT NULL DEFAULT 'pending',
				adjustment_date TIMESTAMP WITHOUT TIME ZONE NOT NULL,
				stock_adjustment_id BIGINT,
				request_total NUMERIC NOT NULL DEFAULT 0,
				item_count INTEGER NOT NULL DEFAULT 0,
				reason VARCHAR(64) NOT NULL,
				request_note TEXT,
				request_payload TEXT NOT NULL,
				requested_by_user_id TEXT,
				requested_by_actor_type VARCHAR(32),
				requested_by_name VARCHAR(255),
				requested_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW(),
				reviewed_by_user_id TEXT,
				reviewed_by_actor_type VARCHAR(32),
				reviewed_by_name VARCHAR(255),
				review_note TEXT,
				reviewed_at TIMESTAMP WITHOUT TIME ZONE,
				approved_at TIMESTAMP WITHOUT TIME ZONE,
				rejected_at TIMESTAMP WITHOUT TIME ZONE,
				created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS idx_tblstock_adjustment_approval_requests_status_date
				ON tblstock_adjustment_approval_requests (status, adjustment_date DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_tblstock_adjustment_approval_requests_outlet_date
				ON tblstock_adjustment_approval_requests (outlet_id, adjustment_date DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_tblstock_adjustment_approval_requests_requested_at
				ON tblstock_adjustment_approval_requests (requested_at DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_tblstock_adjustment_approval_requests_stock_adjustment_id
				ON tblstock_adjustment_approval_requests (stock_adjustment_id)`,
		}

		for _, statement := range statements {
			if err := db.Exec(statement).Error; err != nil {
				ensureStockAdjustmentApprovalSchemaErr = fmt.Errorf("failed to ensure stock adjustment approval schema: %w", err)
				return
			}
		}
	})

	return ensureStockAdjustmentApprovalSchemaErr
}
