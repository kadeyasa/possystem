package services

import (
	"fmt"
	"sync"

	"gorm.io/gorm"
)

var (
	ensureDocumentAuditSchemaOnce sync.Once
	ensureDocumentAuditSchemaErr  error
)

func EnsureDocumentAuditSchema(db *gorm.DB) error {
	ensureDocumentAuditSchemaOnce.Do(func() {
		statements := []string{
			`CREATE TABLE IF NOT EXISTS public.tbldocument_audit_trails (
				id BIGSERIAL PRIMARY KEY,
				outlet_id BIGINT NOT NULL DEFAULT 0,
				document_type VARCHAR(64) NOT NULL,
				document_id BIGINT NOT NULL DEFAULT 0,
				action VARCHAR(32) NOT NULL,
				summary VARCHAR(255),
				note TEXT,
				metadata_json TEXT,
				before_state_json TEXT,
				after_state_json TEXT,
				actor_user_id TEXT,
				actor_type VARCHAR(32),
				actor_name VARCHAR(255),
				request_path VARCHAR(255),
				ip_address VARCHAR(64),
				user_agent TEXT,
				created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS idx_tbldocument_audit_trails_document
				ON tbldocument_audit_trails (document_type, document_id, created_at DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_tbldocument_audit_trails_outlet_date
				ON tbldocument_audit_trails (outlet_id, created_at DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_tbldocument_audit_trails_action_date
				ON tbldocument_audit_trails (action, created_at DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_tbldocument_audit_trails_actor_date
				ON tbldocument_audit_trails (actor_user_id, created_at DESC)`,
		}

		for _, statement := range statements {
			if err := db.Exec(statement).Error; err != nil {
				ensureDocumentAuditSchemaErr = fmt.Errorf("failed to ensure document audit schema: %w", err)
				return
			}
		}
	})

	return ensureDocumentAuditSchemaErr
}
