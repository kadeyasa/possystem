package services

import (
	"fmt"
	"sync"

	"gorm.io/gorm"
)

var (
	ensureAccountMappingSchemaOnce sync.Once
	ensureAccountMappingSchemaErr  error
)

func EnsureAccountMappingSchema(db *gorm.DB) error {
	ensureAccountMappingSchemaOnce.Do(func() {
		statements := []string{
			`CREATE TABLE IF NOT EXISTS public.tblaccount_mappings (
				id BIGSERIAL PRIMARY KEY,
				outlet_id BIGINT NOT NULL DEFAULT 0,
				account_id VARCHAR(10) NOT NULL,
				transaction_type VARCHAR(50) NOT NULL,
				purpose VARCHAR(50) NOT NULL,
				is_active BOOLEAN NOT NULL DEFAULT TRUE,
				created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
				updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW()
			)`,
			`UPDATE tblaccount_mappings
				SET transaction_type = LOWER(BTRIM(COALESCE(transaction_type, ''))),
					purpose = LOWER(BTRIM(COALESCE(purpose, '')))
				WHERE transaction_type IS DISTINCT FROM LOWER(BTRIM(COALESCE(transaction_type, '')))
					OR purpose IS DISTINCT FROM LOWER(BTRIM(COALESCE(purpose, '')))`,
			`DELETE FROM tblaccount_mappings
				WHERE id IN (
					SELECT id
					FROM (
						SELECT id,
							ROW_NUMBER() OVER (
								PARTITION BY outlet_id, transaction_type, purpose
								ORDER BY updated_at DESC NULLS LAST, id DESC
							) AS row_num
						FROM tblaccount_mappings
					) ranked
					WHERE ranked.row_num > 1
				)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_tblaccount_mappings_scope ON tblaccount_mappings (outlet_id, transaction_type, purpose)`,
			`CREATE INDEX IF NOT EXISTS idx_tblaccount_mappings_account ON tblaccount_mappings (account_id)`,
			`CREATE INDEX IF NOT EXISTS idx_tblaccount_mappings_outlet_active ON tblaccount_mappings (outlet_id, is_active)`,
			`INSERT INTO tblaccount_mappings (outlet_id, account_id, transaction_type, purpose, is_active, created_at, updated_at)
				SELECT
					COALESCE(outlet_id, 0) AS outlet_id,
					id AS account_id,
					LOWER(BTRIM(transaction_type)) AS transaction_type,
					LOWER(BTRIM(purpose)) AS purpose,
					COALESCE(is_active, TRUE) AS is_active,
					NOW(),
					NOW()
				FROM tblaccounts
				WHERE COALESCE(BTRIM(transaction_type), '') <> ''
					AND COALESCE(BTRIM(purpose), '') <> ''
				ON CONFLICT (outlet_id, transaction_type, purpose) DO NOTHING`,
		}

		for _, statement := range statements {
			if err := db.Exec(statement).Error; err != nil {
				ensureAccountMappingSchemaErr = fmt.Errorf("failed to ensure POS account mapping schema: %w", err)
				return
			}
		}
	})

	return ensureAccountMappingSchemaErr
}
