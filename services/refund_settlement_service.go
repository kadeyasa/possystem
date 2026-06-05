package services

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/kadeyasa/possystem/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	RefundSettlementTypeCashRefund         = "cash_refund"
	RefundSettlementTypeStoreCredit        = "store_credit"
	RefundSettlementTypeExchangeRepurchase = "exchange_repurchase"

	RefundSettlementStatusCompleted           = "completed"
	RefundSettlementStatusCreditIssued        = "credit_issued"
	RefundSettlementStatusCreditPartiallyUsed = "credit_partially_used"
	RefundSettlementStatusCreditUsed          = "credit_used"

	RefundStoreCreditStatusActive = "active"
	RefundStoreCreditStatusUsed   = "used"
)

var (
	ensureRefundSettlementSchemaOnce sync.Once
	ensureRefundSettlementSchemaErr  error
)

func NormalizeRefundSettlementType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "cash", "cash_refund", "cash-refund", "refund_cash", "tunai":
		return RefundSettlementTypeCashRefund
	case "voucher", "store_credit", "store-credit", "saldo", "saldo_retur", "voucher_belanja":
		return RefundSettlementTypeStoreCredit
	case "exchange", "exchange_repurchase", "exchange-repurchase", "belanja_kembali", "rebuy", "rebuy_now":
		return RefundSettlementTypeExchangeRepurchase
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func NormalizeRefundSettlementMethod(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "tunai":
		return "cash"
	case "transfer_bank", "bank_transfer":
		return "transfer"
	case "saldo_retur", "store_credit", "voucher":
		return "refund_credit"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func ResolveRefundSettlementMethod(originalPaymentMethod, settlementType, requestedMethod string) string {
	settlementType = NormalizeRefundSettlementType(settlementType)
	method := NormalizeRefundSettlementMethod(requestedMethod)

	if settlementType == RefundSettlementTypeStoreCredit || settlementType == RefundSettlementTypeExchangeRepurchase {
		return "refund_credit"
	}
	if method != "" {
		return method
	}

	switch NormalizeRefundSettlementMethod(originalPaymentMethod) {
	case "cash", "transfer", "qris":
		return NormalizeRefundSettlementMethod(originalPaymentMethod)
	default:
		return "cash"
	}
}

func ResolveRefundSettlementStatus(settlementType string, remainingAmount float64) string {
	settlementType = NormalizeRefundSettlementType(settlementType)
	switch settlementType {
	case RefundSettlementTypeStoreCredit, RefundSettlementTypeExchangeRepurchase:
		if remainingAmount <= 0 {
			return RefundSettlementStatusCreditUsed
		}
		return RefundSettlementStatusCreditIssued
	default:
		return RefundSettlementStatusCompleted
	}
}

func BuildRefundNumber(outletID, refundID uint) string {
	return fmt.Sprintf("RT-%d-%06d", outletID, refundID)
}

func BuildRefundStoreCreditCode(outletID, refundID uint) string {
	return fmt.Sprintf("VCR-RET-%d-%06d", outletID, refundID)
}

func EnsureRefundSettlementSchema(db *gorm.DB) error {
	ensureRefundSettlementSchemaOnce.Do(func() {
		statements := []string{
			`ALTER TABLE tblrefunds ADD COLUMN IF NOT EXISTS refund_number VARCHAR(64)`,
			`ALTER TABLE tblrefunds ADD COLUMN IF NOT EXISTS customer_id BIGINT`,
			`ALTER TABLE tblrefunds ADD COLUMN IF NOT EXISTS customer_name VARCHAR(255)`,
			`ALTER TABLE tblrefunds ADD COLUMN IF NOT EXISTS settlement_type VARCHAR(32)`,
			`ALTER TABLE tblrefunds ADD COLUMN IF NOT EXISTS settlement_method VARCHAR(32)`,
			`ALTER TABLE tblrefunds ADD COLUMN IF NOT EXISTS settlement_status VARCHAR(32)`,
			`ALTER TABLE tblrefunds ADD COLUMN IF NOT EXISTS store_credit_code VARCHAR(64)`,
			`ALTER TABLE tblrefunds ADD COLUMN IF NOT EXISTS replacement_transaction_id BIGINT`,
			`ALTER TABLE tblrefunds ADD COLUMN IF NOT EXISTS approval_request_id BIGINT`,
			`ALTER TABLE tbltransactions ADD COLUMN IF NOT EXISTS settlement_amount DOUBLE PRECISION DEFAULT 0`,
			`ALTER TABLE tbltransactions ADD COLUMN IF NOT EXISTS refund_credit_amount DOUBLE PRECISION DEFAULT 0`,
			`ALTER TABLE tbltransactions ADD COLUMN IF NOT EXISTS refund_credit_code VARCHAR(64)`,
			`UPDATE tblrefunds
				SET settlement_type = COALESCE(NULLIF(BTRIM(settlement_type), ''), 'cash_refund'),
					settlement_method = COALESCE(NULLIF(BTRIM(settlement_method), ''), 'cash'),
					settlement_status = COALESCE(NULLIF(BTRIM(settlement_status), ''), 'completed')
				WHERE settlement_type IS NULL
					OR settlement_method IS NULL
					OR settlement_status IS NULL
					OR BTRIM(COALESCE(settlement_type, '')) = ''
					OR BTRIM(COALESCE(settlement_method, '')) = ''
					OR BTRIM(COALESCE(settlement_status, '')) = ''`,
			`UPDATE tbltransactions
				SET settlement_amount = COALESCE(NULLIF(settlement_amount, 0), COALESCE(NULLIF(grand_total, 0), COALESCE(NULLIF(subtotal, 0), total) - COALESCE(discount, 0) + COALESCE(service, 0) + COALESCE(tax, 0)))
				WHERE COALESCE(settlement_amount, 0) <= 0`,
			`CREATE TABLE IF NOT EXISTS public.tblrefund_store_credits (
				id BIGSERIAL PRIMARY KEY,
				refund_id BIGINT NOT NULL,
				outlet_id BIGINT NOT NULL,
				transaction_id BIGINT NOT NULL,
				customer_id BIGINT,
				customer_name VARCHAR(255),
				credit_code VARCHAR(64) NOT NULL,
				original_amount NUMERIC NOT NULL DEFAULT 0,
				remaining_amount NUMERIC NOT NULL DEFAULT 0,
				status VARCHAR(32) NOT NULL DEFAULT 'active',
				note TEXT,
				issued_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW(),
				last_used_at TIMESTAMP WITHOUT TIME ZONE,
				expires_at TIMESTAMP WITHOUT TIME ZONE,
				created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW()
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_tblrefund_store_credits_outlet_code_unique ON tblrefund_store_credits (outlet_id, credit_code)`,
			`CREATE INDEX IF NOT EXISTS idx_tblrefund_store_credits_status_customer ON tblrefund_store_credits (outlet_id, status, customer_id, issued_at DESC)`,
			`CREATE TABLE IF NOT EXISTS public.tblrefund_credit_usages (
				id BIGSERIAL PRIMARY KEY,
				refund_store_credit_id BIGINT NOT NULL,
				refund_id BIGINT,
				transaction_id BIGINT NOT NULL,
				outlet_id BIGINT NOT NULL,
				applied_amount NUMERIC NOT NULL DEFAULT 0,
				note TEXT,
				created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS idx_tblrefund_credit_usages_credit_id ON tblrefund_credit_usages (refund_store_credit_id, created_at DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_tblrefund_credit_usages_transaction_id ON tblrefund_credit_usages (transaction_id)`,
		}

		for _, statement := range statements {
			if err := db.Exec(statement).Error; err != nil {
				ensureRefundSettlementSchemaErr = err
				return
			}
		}
	})

	return ensureRefundSettlementSchemaErr
}

func ResolveRefundCreditLiabilityAccount(tx *gorm.DB, outletID uint) (models.Account, error) {
	candidates := []accountSelector{
		{TransactionType: "sale", Purpose: "return_credit"},
		{TransactionType: "sale", Purpose: "store_credit"},
		{TransactionType: "liability", Purpose: "return_credit"},
		{TransactionType: "liability", Purpose: "store_credit"},
		{TransactionType: "refund", Purpose: "store_credit"},
	}

	account, found, err := tryResolveAccountByCandidates(tx, outletID, candidates)
	if err != nil {
		return models.Account{}, err
	}
	if !found {
		return models.Account{}, fmt.Errorf("refund store credit liability account mapping not found for outlet %d", outletID)
	}
	return account, nil
}

func LockActiveRefundStoreCreditByCode(tx *gorm.DB, outletID uint, code string) (*models.RefundStoreCredit, error) {
	normalizedCode := strings.TrimSpace(code)
	if normalizedCode == "" {
		return nil, fmt.Errorf("refund credit code is required")
	}

	var credit models.RefundStoreCredit
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("outlet_id = ? AND credit_code = ?", outletID, normalizedCode).
		First(&credit).Error; err != nil {
		return nil, err
	}

	if strings.TrimSpace(credit.Status) != RefundStoreCreditStatusActive {
		return nil, fmt.Errorf("refund store credit %s is no longer active", normalizedCode)
	}
	if credit.RemainingAmount <= 0 {
		return nil, fmt.Errorf("refund store credit %s has no remaining balance", normalizedCode)
	}

	return &credit, nil
}

func ApplyRefundStoreCreditUsageTx(
	tx *gorm.DB,
	credit *models.RefundStoreCredit,
	transaction models.Transaction,
	appliedAmount float64,
	note string,
) error {
	if credit == nil {
		return fmt.Errorf("refund store credit is required")
	}
	if appliedAmount <= 0 {
		return nil
	}
	if appliedAmount > credit.RemainingAmount {
		return fmt.Errorf(
			"refund store credit %s remaining balance is %.2f and cannot cover %.2f",
			credit.CreditCode,
			credit.RemainingAmount,
			appliedAmount,
		)
	}

	now := time.Now()
	nextRemaining := credit.RemainingAmount - appliedAmount
	nextStatus := RefundStoreCreditStatusActive
	refundSettlementStatus := RefundSettlementStatusCreditPartiallyUsed
	if nextRemaining <= 0 {
		nextRemaining = 0
		nextStatus = RefundStoreCreditStatusUsed
		refundSettlementStatus = RefundSettlementStatusCreditUsed
	}

	usage := models.RefundCreditUsage{
		RefundStoreCreditID: credit.ID,
		RefundID:            ptrUint(credit.RefundID),
		TransactionID:       transaction.ID,
		OutletID:            transaction.OutletID,
		AppliedAmount:       appliedAmount,
		Note:                strings.TrimSpace(note),
		CreatedAt:           now,
	}
	if err := tx.Create(&usage).Error; err != nil {
		return err
	}

	if err := tx.Model(&models.RefundStoreCredit{}).
		Where("id = ?", credit.ID).
		Updates(map[string]interface{}{
			"remaining_amount": nextRemaining,
			"status":           nextStatus,
			"last_used_at":     now,
			"updated_at":       now,
		}).Error; err != nil {
		return err
	}

	refundUpdates := map[string]interface{}{
		"settlement_status": refundSettlementStatus,
	}
	if transaction.ID > 0 {
		refundUpdates["replacement_transaction_id"] = transaction.ID
	}
	if err := tx.Model(&models.Refund{}).
		Where("id = ?", credit.RefundID).
		Updates(refundUpdates).Error; err != nil {
		return err
	}

	credit.RemainingAmount = nextRemaining
	credit.Status = nextStatus
	credit.LastUsedAt = &now

	return nil
}

func ptrUint(value uint) *uint {
	if value == 0 {
		return nil
	}
	return &value
}
