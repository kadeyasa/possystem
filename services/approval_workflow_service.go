package services

import (
	"fmt"
	"sync"

	"gorm.io/gorm"
)

const (
	POSApprovalRequestTypeRefund = "refund"
	POSApprovalRequestTypeVoid   = "void"

	POSApprovalStatusPending  = "pending"
	POSApprovalStatusApproved = "approved"
	POSApprovalStatusRejected = "rejected"

	TransactionDocumentStatusPosted = "posted"
	TransactionDocumentStatusVoided = "voided"
)

var (
	ensurePOSApprovalSchemaOnce sync.Once
	ensurePOSApprovalSchemaErr  error
)

func EnsurePOSApprovalSchema(db *gorm.DB) error {
	ensurePOSApprovalSchemaOnce.Do(func() {
		statements := []string{
			`ALTER TABLE tbltransactions ADD COLUMN IF NOT EXISTS document_status VARCHAR(16) NOT NULL DEFAULT 'posted'`,
			`ALTER TABLE tbltransactions ADD COLUMN IF NOT EXISTS voided_at TIMESTAMP WITHOUT TIME ZONE`,
			`ALTER TABLE tbltransactions ADD COLUMN IF NOT EXISTS void_reason TEXT`,
			`ALTER TABLE tbltransactions ADD COLUMN IF NOT EXISTS void_approval_request_id BIGINT`,
			`UPDATE tbltransactions SET document_status = 'posted' WHERE COALESCE(BTRIM(document_status), '') = ''`,
			`CREATE INDEX IF NOT EXISTS idx_tbltransactions_active_outlet_created_at_id
				ON tbltransactions (outlet_id, created_at DESC, id DESC)
				WHERE COALESCE(document_status, 'posted') <> 'voided'`,
			`CREATE TABLE IF NOT EXISTS public.tblpos_approval_requests (
				id BIGSERIAL PRIMARY KEY,
				outlet_id BIGINT NOT NULL,
				request_type VARCHAR(32) NOT NULL,
				status VARCHAR(16) NOT NULL DEFAULT 'pending',
				transaction_id BIGINT NOT NULL,
				refund_id BIGINT,
				request_total NUMERIC NOT NULL DEFAULT 0,
				item_count INTEGER NOT NULL DEFAULT 0,
				reason VARCHAR(255) NOT NULL,
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
			`CREATE INDEX IF NOT EXISTS idx_tblpos_approval_requests_status_date ON tblpos_approval_requests (status, requested_at DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_tblpos_approval_requests_type_date ON tblpos_approval_requests (request_type, requested_at DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_tblpos_approval_requests_transaction_id ON tblpos_approval_requests (transaction_id)`,
			`CREATE INDEX IF NOT EXISTS idx_tblpos_approval_requests_outlet_date ON tblpos_approval_requests (outlet_id, requested_at DESC)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_tblpos_approval_requests_pending_unique ON tblpos_approval_requests (request_type, transaction_id) WHERE status = 'pending'`,
		}

		for _, statement := range statements {
			if err := db.Exec(statement).Error; err != nil {
				ensurePOSApprovalSchemaErr = fmt.Errorf("failed to ensure POS approval schema: %w", err)
				return
			}
		}
	})

	return ensurePOSApprovalSchemaErr
}
