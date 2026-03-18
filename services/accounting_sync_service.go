package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/utils"
	"gorm.io/gorm"
)

const (
	AccountingSyncStatusPending = "pending"
	AccountingSyncStatusSynced  = "synced"
	AccountingSyncStatusFailed  = "failed"

	AccountingRecordTypeSale               = "sale"
	AccountingRecordTypeRefund             = "refund"
	AccountingRecordTypePurchase           = "purchase"
	AccountingRecordTypeOperationalExpense = "operational_expense"
	AccountingRecordTypeVendorBill         = "vendor_bill"
	AccountingRecordTypeVendorPayment      = "vendor_payment"
	AccountingRecordTypeStockAdjustment    = "stock_adjustment"
	AccountingRecordTypeStockOpname        = "stock_opname"

	accountingSourceSystemPOS = "pos"
	accountingBusinessUnitPOS = "pos"

	outletFeeExpenseLineDescription = "Potongan fee outlet"
	outletFeeBalanceLineDescription = "Pengurang saldo outlet"
	saleCOGSLineDescription         = "Harga pokok penjualan"
	saleInventoryLineDescription    = "Pengurangan persediaan"
)

type AccountingSyncRequest struct {
	TableName         string
	RecordID          uint
	OutletID          uint
	Description       string
	OccurredAt        time.Time
	SourceType        string
	SourceID          string
	ExternalReference string
	LegacyType        string
	Metadata          map[string]interface{}
	JournalLines      []models.JournalLine
	IdempotencyKey    string
	RecordSyncKey     string
}

type AccountingSyncResult struct {
	Status         string `json:"status"`
	Error          string `json:"error,omitempty"`
	IdempotencyKey string `json:"idempotency_key"`
	JournalID      int64  `json:"journal_id,omitempty"`
	AlreadyPosted  bool   `json:"already_posted,omitempty"`
}

type AccountingSyncListFilter struct {
	OutletID   uint
	Status     string
	RecordType string
	Search     string
	Limit      int
}

type AccountingSyncListItem struct {
	RecordType        string     `json:"record_type"`
	RecordID          uint       `json:"record_id"`
	OutletID          uint       `json:"outlet_id"`
	Amount            float64    `json:"amount"`
	Description       string     `json:"description"`
	ExternalReference string     `json:"external_reference"`
	SyncStatus        string     `json:"sync_status"`
	SyncError         string     `json:"sync_error"`
	SyncedAt          *time.Time `json:"synced_at"`
	OccurredAt        time.Time  `json:"occurred_at"`
	IdempotencyKey    string     `json:"idempotency_key"`
	JournalEntryID    *uint      `json:"journal_entry_id"`
}

type AccountingSyncTypeSummary struct {
	RecordType string `json:"record_type"`
	Total      int64  `json:"total"`
	Synced     int64  `json:"synced"`
	Pending    int64  `json:"pending"`
	Failed     int64  `json:"failed"`
}

type AccountingSyncSummary struct {
	Total         int64                       `json:"total"`
	Synced        int64                       `json:"synced"`
	Pending       int64                       `json:"pending"`
	Failed        int64                       `json:"failed"`
	NeedsRetry    int64                       `json:"needs_retry"`
	ByRecordType  []AccountingSyncTypeSummary `json:"by_record_type" gorm:"-"`
	LastUpdatedAt time.Time                   `json:"last_updated_at"`
}

type accountingPostLine struct {
	AccountID string  `json:"account_id"`
	Entry     string  `json:"entry"`
	Amount    float64 `json:"amount"`
}

type accountingPostPayload struct {
	OutletID          uint                   `json:"outlet_id"`
	Description       string                 `json:"description"`
	OccurredAt        string                 `json:"occurred_at"`
	SourceSystem      string                 `json:"source_system"`
	SourceType        string                 `json:"source_type"`
	SourceID          string                 `json:"source_id"`
	ExternalReference string                 `json:"external_reference"`
	BusinessUnit      string                 `json:"business_unit"`
	IdempotencyKey    string                 `json:"idempotency_key"`
	PostingStatus     string                 `json:"posting_status"`
	LegacyType        string                 `json:"legacy_type"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	Lines             []accountingPostLine   `json:"lines"`
}

type accountingPostResponse struct {
	Error   int    `json:"error"`
	Message string `json:"message"`
	Data    struct {
		JournalID      int64  `json:"journal_id"`
		IdempotencyKey string `json:"idempotency_key"`
		AlreadyPosted  bool   `json:"already_posted"`
	} `json:"data"`
}

var (
	ensureAccountingSyncSchemaOnce sync.Once
	ensureAccountingSyncSchemaErr  error
)

func EnsureAccountingSyncSchema(db *gorm.DB) error {
	if err := EnsurePOSFinanceDocumentSchema(db); err != nil {
		return err
	}

	ensureAccountingSyncSchemaOnce.Do(func() {
		statements := []string{
			`ALTER TABLE tbltransactions ADD COLUMN IF NOT EXISTS cashier_name VARCHAR(255)`,
			`ALTER TABLE tbltransactions ADD COLUMN IF NOT EXISTS accounting_sync_status VARCHAR(16)`,
			`ALTER TABLE tbltransactions ADD COLUMN IF NOT EXISTS accounting_sync_error TEXT`,
			`ALTER TABLE tbltransactions ADD COLUMN IF NOT EXISTS accounting_synced_at TIMESTAMP`,
			`ALTER TABLE tbltransactions ADD COLUMN IF NOT EXISTS accounting_idempotency_key TEXT`,
			`ALTER TABLE tblrefunds ADD COLUMN IF NOT EXISTS cashier_name VARCHAR(255)`,
			`ALTER TABLE tblrefunds ADD COLUMN IF NOT EXISTS accounting_sync_status VARCHAR(16)`,
			`ALTER TABLE tblrefunds ADD COLUMN IF NOT EXISTS accounting_sync_error TEXT`,
			`ALTER TABLE tblrefunds ADD COLUMN IF NOT EXISTS accounting_synced_at TIMESTAMP`,
			`ALTER TABLE tblrefunds ADD COLUMN IF NOT EXISTS accounting_idempotency_key TEXT`,
			`ALTER TABLE tblpurchases ADD COLUMN IF NOT EXISTS accounting_sync_status VARCHAR(16)`,
			`ALTER TABLE tblpurchases ADD COLUMN IF NOT EXISTS accounting_sync_error TEXT`,
			`ALTER TABLE tblpurchases ADD COLUMN IF NOT EXISTS accounting_synced_at TIMESTAMP`,
			`ALTER TABLE tblpurchases ADD COLUMN IF NOT EXISTS accounting_idempotency_key TEXT`,
		}

		for _, statement := range statements {
			if err := db.Exec(statement).Error; err != nil {
				ensureAccountingSyncSchemaErr = err
				return
			}
		}
	})

	return ensureAccountingSyncSchemaErr
}

func BuildAccountingIdempotencyKey(sourceType string, recordID uint) string {
	keyType := strings.ToLower(strings.TrimSpace(sourceType))
	keyType = strings.TrimPrefix(keyType, "pos_")
	keyType = strings.ReplaceAll(keyType, " ", "_")
	if keyType == "" {
		keyType = "journal"
	}
	return fmt.Sprintf("pos:%s:%d", keyType, recordID)
}

func UpdateAccountingSyncRecord(tableName string, recordID uint, status string, syncErr error, syncedAt *time.Time, idempotencyKey string) error {
	if recordID == 0 || strings.TrimSpace(tableName) == "" {
		return nil
	}

	updates := map[string]interface{}{
		"accounting_sync_status":     strings.TrimSpace(status),
		"accounting_idempotency_key": strings.TrimSpace(idempotencyKey),
	}
	if syncedAt != nil {
		updates["accounting_synced_at"] = *syncedAt
	} else {
		updates["accounting_synced_at"] = nil
	}
	if syncErr != nil {
		updates["accounting_sync_error"] = trimAccountingError(syncErr.Error())
	} else {
		updates["accounting_sync_error"] = nil
	}

	return database.DB.Table(tableName).Where("id = ?", recordID).Updates(updates).Error
}

func SyncPOSJournalEntry(req AccountingSyncRequest) (*AccountingSyncResult, error) {
	if err := EnsureAccountingSyncSchema(database.DB); err != nil {
		return nil, err
	}

	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = BuildAccountingIdempotencyKey(req.SourceType, req.RecordID)
	}
	recordSyncKey := strings.TrimSpace(req.RecordSyncKey)
	if recordSyncKey == "" {
		recordSyncKey = idempotencyKey
	}
	result := &AccountingSyncResult{
		Status:         AccountingSyncStatusPending,
		IdempotencyKey: idempotencyKey,
	}

	payload, err := buildAccountingPayload(req, idempotencyKey)
	if err != nil {
		result.Status = AccountingSyncStatusFailed
		result.Error = err.Error()
		_ = UpdateAccountingSyncRecord(req.TableName, req.RecordID, AccountingSyncStatusFailed, err, nil, recordSyncKey)
		return result, err
	}

	response, err := postAccountingPayload(payload)
	if err != nil {
		result.Status = AccountingSyncStatusFailed
		result.Error = err.Error()
		_ = UpdateAccountingSyncRecord(req.TableName, req.RecordID, AccountingSyncStatusFailed, err, nil, recordSyncKey)
		return result, err
	}

	now := time.Now()
	result.Status = AccountingSyncStatusSynced
	result.JournalID = response.Data.JournalID
	result.AlreadyPosted = response.Data.AlreadyPosted
	if err := UpdateAccountingSyncRecord(req.TableName, req.RecordID, AccountingSyncStatusSynced, nil, &now, recordSyncKey); err != nil {
		return result, err
	}

	return result, nil
}

func ComputeTransactionCOGS(tx *gorm.DB, outletID uint, items []models.TransactionItem) (float64, error) {
	plan, err := PrepareSaleInventoryPlan(tx, outletID, items)
	if err != nil {
		return 0, err
	}
	return plan.TotalCost, nil
}

func ComputeRefundCOGS(tx *gorm.DB, outletID uint, items []models.RefundItem) (float64, error) {
	var total float64
	for _, item := range items {
		unitCost, err := resolveUnitCost(tx, outletID, item.ProductID, nil)
		if err != nil {
			return 0, err
		}
		total += unitCost * float64(item.Quantity)
	}
	return total, nil
}

func normalizeAccountingRecordType(recordType string) string {
	return strings.ToLower(strings.TrimSpace(recordType))
}

func loadJournalEntryByID(journalEntryID *uint) (*models.JournalEntry, error) {
	if journalEntryID == nil || *journalEntryID == 0 {
		return nil, fmt.Errorf("journal entry is not attached")
	}

	var journal models.JournalEntry
	if err := database.DB.Preload("JournalLines").First(&journal, *journalEntryID).Error; err != nil {
		return nil, err
	}

	return &journal, nil
}

func cloneJournalLines(lines []models.JournalLine) []models.JournalLine {
	if len(lines) == 0 {
		return nil
	}

	cloned := make([]models.JournalLine, len(lines))
	copy(cloned, lines)
	return cloned
}

func hasLineDescription(lines []models.JournalLine, description string) bool {
	target := strings.ToLower(strings.TrimSpace(description))
	if target == "" {
		return false
	}

	for _, line := range lines {
		if strings.ToLower(strings.TrimSpace(line.Description)) == target {
			return true
		}
	}

	return false
}

func buildPOSSaleExternalReference(transaction models.Transaction) string {
	occurredAt := transaction.CreatedAt
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}

	return fmt.Sprintf(
		"INV-POS-%d-%s-%06d",
		transaction.OutletID,
		occurredAt.Format("20060102"),
		transaction.ID,
	)
}

func isSaleCOGSJournalLine(line models.JournalLine) bool {
	switch strings.ToLower(strings.TrimSpace(line.Description)) {
	case strings.ToLower(saleCOGSLineDescription), strings.ToLower(saleInventoryLineDescription):
		return true
	default:
		return false
	}
}

func splitSaleJournalLines(lines []models.JournalLine) ([]models.JournalLine, []models.JournalLine, error) {
	saleLines := make([]models.JournalLine, 0, len(lines))
	cogsLines := make([]models.JournalLine, 0, 2)

	for _, line := range cloneJournalLines(lines) {
		if isSaleCOGSJournalLine(line) {
			cogsLines = append(cogsLines, line)
			continue
		}
		saleLines = append(saleLines, line)
	}

	if len(cogsLines) == 1 {
		return nil, nil, fmt.Errorf("incomplete COGS journal lines for POS sale")
	}

	return saleLines, cogsLines, nil
}

func tryCreateOutletFeeJournalLines(tx *gorm.DB, outletID uint, saleAmount float64) ([]models.JournalLine, float64, bool, error) {
	if saleAmount <= 0 {
		return nil, 0, false, nil
	}

	outletFee, err := GetOutletFeeTx(tx, int64(outletID))
	if err != nil {
		return nil, 0, false, err
	}

	totalFee := (float64(outletFee.FeeSetting) / 100.0) * saleAmount
	if totalFee <= 0 {
		return nil, 0, false, nil
	}

	expenseAccount, balanceAccount, found, err := TryResolveOutletFeeAccounts(tx, outletID)
	if err != nil {
		return nil, 0, false, err
	}
	if !found {
		return nil, totalFee, false, nil
	}

	return []models.JournalLine{
		{
			AccountID:   expenseAccount.ID,
			Debit:       totalFee,
			Credit:      0,
			Description: outletFeeExpenseLineDescription,
		},
		{
			AccountID:   balanceAccount.ID,
			Debit:       0,
			Credit:      totalFee,
			Description: outletFeeBalanceLineDescription,
		},
	}, totalFee, true, nil
}

func TryAppendOutletFeeJournalLines(tx *gorm.DB, outletID uint, saleAmount float64, lines []models.JournalLine) ([]models.JournalLine, float64, error) {
	cloned := cloneJournalLines(lines)
	if hasLineDescription(cloned, outletFeeExpenseLineDescription) && hasLineDescription(cloned, outletFeeBalanceLineDescription) {
		return cloned, 0, nil
	}

	feeLines, totalFee, found, err := tryCreateOutletFeeJournalLines(tx, outletID, saleAmount)
	if err != nil {
		return nil, 0, err
	}
	if !found {
		return cloned, totalFee, nil
	}

	return append(cloned, feeLines...), totalFee, nil
}

func AppendRequiredOutletFeeJournalLines(tx *gorm.DB, outletID uint, saleAmount float64, lines []models.JournalLine) ([]models.JournalLine, float64, error) {
	cloned := cloneJournalLines(lines)
	if hasLineDescription(cloned, outletFeeExpenseLineDescription) && hasLineDescription(cloned, outletFeeBalanceLineDescription) {
		return cloned, 0, nil
	}

	feeLines, totalFee, found, err := tryCreateOutletFeeJournalLines(tx, outletID, saleAmount)
	if err != nil {
		return nil, 0, err
	}
	if totalFee > 0 && !found {
		return nil, totalFee, fmt.Errorf("outlet fee accounting mapping is not configured for outlet %d", outletID)
	}
	if !found {
		return cloned, totalFee, nil
	}

	return append(cloned, feeLines...), totalFee, nil
}

func resolveUnitCost(tx *gorm.DB, outletID uint, productID uint, variantID *int64) (float64, error) {
	var product models.Product
	if err := tx.Where("id = ? AND outlet_id = ?", productID, outletID).First(&product).Error; err != nil {
		return 0, err
	}

	if variantID != nil && *variantID > 0 {
		var variant models.Variant
		err := tx.Where("id = ? AND outlet_id = ? AND item_id = ?", *variantID, outletID, productID).First(&variant).Error
		if err == nil && variant.BiayaProduksi > 0 {
			return variant.BiayaProduksi, nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, err
		}
	}

	if product.LastPurchasePrice > 0 {
		return product.LastPurchasePrice, nil
	}

	return 0, nil
}

func buildAccountingPayload(req AccountingSyncRequest, idempotencyKey string) (*accountingPostPayload, error) {
	if req.RecordID == 0 {
		return nil, fmt.Errorf("record id is required")
	}
	if req.OutletID == 0 {
		return nil, fmt.Errorf("outlet id is required")
	}
	if strings.TrimSpace(req.SourceType) == "" || strings.TrimSpace(req.SourceID) == "" {
		return nil, fmt.Errorf("source type and source id are required")
	}

	lines := make([]accountingPostLine, 0, len(req.JournalLines))
	for _, line := range req.JournalLines {
		if line.Debit > 0 {
			lines = append(lines, accountingPostLine{
				AccountID: strings.TrimSpace(line.AccountID),
				Entry:     "debit",
				Amount:    line.Debit,
			})
		}
		if line.Credit > 0 {
			lines = append(lines, accountingPostLine{
				AccountID: strings.TrimSpace(line.AccountID),
				Entry:     "credit",
				Amount:    line.Credit,
			})
		}
	}
	if len(lines) < 2 {
		return nil, fmt.Errorf("at least two accounting lines are required")
	}

	occurredAt := req.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}

	legacyType := strings.TrimSpace(req.LegacyType)
	if legacyType == "" {
		legacyType = "general"
	}

	return &accountingPostPayload{
		OutletID:          req.OutletID,
		Description:       strings.TrimSpace(req.Description),
		OccurredAt:        occurredAt.Format(time.RFC3339),
		SourceSystem:      accountingSourceSystemPOS,
		SourceType:        strings.TrimSpace(req.SourceType),
		SourceID:          strings.TrimSpace(req.SourceID),
		ExternalReference: strings.TrimSpace(req.ExternalReference),
		BusinessUnit:      accountingBusinessUnitPOS,
		IdempotencyKey:    idempotencyKey,
		PostingStatus:     "posted",
		LegacyType:        legacyType,
		Metadata:          req.Metadata,
		Lines:             lines,
	}, nil
}

func buildSaleSyncRequests(recordID uint) ([]AccountingSyncRequest, error) {
	var transaction models.Transaction
	if err := database.DB.First(&transaction, recordID).Error; err != nil {
		return nil, err
	}

	journal, err := loadJournalEntryByID(transaction.JournalEntryID)
	if err != nil {
		return nil, err
	}

	lines, feeAmount, err := AppendRequiredOutletFeeJournalLines(database.DB, transaction.OutletID, transaction.Total, journal.JournalLines)
	if err != nil {
		return nil, err
	}

	saleLines, cogsLines, err := splitSaleJournalLines(lines)
	if err != nil {
		return nil, err
	}

	externalReference := buildPOSSaleExternalReference(transaction)
	sourceID := fmt.Sprintf("%d", transaction.ID)
	recordSyncKey := BuildAccountingIdempotencyKey("pos_sale", transaction.ID)
	requests := []AccountingSyncRequest{
		{
			TableName:         transaction.TableName(),
			RecordID:          transaction.ID,
			OutletID:          transaction.OutletID,
			Description:       fmt.Sprintf("Penjualan POS %s", externalReference),
			OccurredAt:        transaction.CreatedAt,
			SourceType:        "pos_sale",
			SourceID:          sourceID,
			ExternalReference: externalReference,
			LegacyType:        "general",
			Metadata: map[string]interface{}{
				"journal_entry_id":   journal.ID,
				"cashier_id":         transaction.CashierID,
				"cashier_name":       strings.TrimSpace(transaction.CashierName),
				"payment_method":     transaction.PaymentMethod,
				"tax":                transaction.Tax,
				"discount":           transaction.Discount,
				"status":             transaction.Status,
				"journal_group":      "sale",
				"outlet_fee_amount":  feeAmount,
				"outlet_fee_posted":  feeAmount <= 0 || hasLineDescription(saleLines, outletFeeExpenseLineDescription),
				"professional_split": true,
			},
			JournalLines:   saleLines,
			IdempotencyKey: recordSyncKey,
			RecordSyncKey:  recordSyncKey,
		},
	}

	if len(cogsLines) > 0 {
		requests = append(requests, AccountingSyncRequest{
			TableName:         transaction.TableName(),
			RecordID:          transaction.ID,
			OutletID:          transaction.OutletID,
			Description:       fmt.Sprintf("HPP POS %s", externalReference),
			OccurredAt:        transaction.CreatedAt,
			SourceType:        "pos_cogs",
			SourceID:          sourceID,
			ExternalReference: externalReference,
			LegacyType:        "expense",
			Metadata: map[string]interface{}{
				"journal_entry_id":   journal.ID,
				"cashier_id":         transaction.CashierID,
				"cashier_name":       strings.TrimSpace(transaction.CashierName),
				"payment_method":     transaction.PaymentMethod,
				"status":             transaction.Status,
				"journal_group":      "cogs",
				"professional_split": true,
			},
			JournalLines:   cogsLines,
			IdempotencyKey: BuildAccountingIdempotencyKey("pos_cogs", transaction.ID),
			RecordSyncKey:  recordSyncKey,
		})
	}

	return requests, nil
}

func buildRefundSyncRequest(recordID uint) (*AccountingSyncRequest, error) {
	var refund models.Refund
	if err := database.DB.First(&refund, recordID).Error; err != nil {
		return nil, err
	}

	journal, err := loadJournalEntryByID(refund.JournalEntryID)
	if err != nil {
		return nil, err
	}

	return &AccountingSyncRequest{
		TableName:         refund.TableName(),
		RecordID:          refund.ID,
		OutletID:          refund.OutletID,
		Description:       journal.Description,
		OccurredAt:        refund.CreatedAt,
		SourceType:        "pos_refund",
		SourceID:          fmt.Sprintf("%d", refund.ID),
		ExternalReference: fmt.Sprintf("POS-REFUND-%d", refund.ID),
		LegacyType:        "general",
		Metadata: map[string]interface{}{
			"journal_entry_id": journal.ID,
			"transaction_id":   refund.TransactionID,
			"cashier_id":       refund.CashierID,
			"cashier_name":     strings.TrimSpace(refund.CashierName),
		},
		JournalLines: cloneJournalLines(journal.JournalLines),
	}, nil
}

func buildPurchaseSyncRequest(recordID uint) (*AccountingSyncRequest, error) {
	var purchase models.Purchase
	if err := database.DB.First(&purchase, recordID).Error; err != nil {
		return nil, err
	}

	journal, err := loadJournalEntryByID(purchase.JournalEntryID)
	if err != nil {
		return nil, err
	}

	externalReference := strings.TrimSpace(purchase.InvoiceNumber)
	if externalReference == "" {
		externalReference = fmt.Sprintf("POS-PURCHASE-%d", purchase.ID)
	}

	return &AccountingSyncRequest{
		TableName:         purchase.TableName(),
		RecordID:          purchase.ID,
		OutletID:          purchase.OutletID,
		Description:       journal.Description,
		OccurredAt:        purchase.PurchaseDate,
		SourceType:        "pos_purchase",
		SourceID:          fmt.Sprintf("%d", purchase.ID),
		ExternalReference: externalReference,
		LegacyType:        "general",
		Metadata: map[string]interface{}{
			"journal_entry_id":   journal.ID,
			"supplier_name":      purchase.SupplierName,
			"payment_method":     purchase.PaymentMethod,
			"payment_status":     purchase.PaymentStatus,
			"paid_amount":        purchase.PaidAmount,
			"outstanding_amount": purchase.OutstandingAmount,
			"linked_vendor_bill": purchase.LinkedVendorBillID,
		},
		JournalLines: cloneJournalLines(journal.JournalLines),
	}, nil
}

func buildOperationalExpenseSyncRequest(recordID uint) (*AccountingSyncRequest, error) {
	var expense models.OperationalExpense
	if err := database.DB.First(&expense, recordID).Error; err != nil {
		return nil, err
	}

	journal, err := loadJournalEntryByID(expense.JournalEntryID)
	if err != nil {
		return nil, err
	}

	externalReference := strings.TrimSpace(expense.ReferenceNo)
	if externalReference == "" {
		externalReference = fmt.Sprintf("POS-OPEX-%d", expense.ID)
	}

	return &AccountingSyncRequest{
		TableName:         expense.TableName(),
		RecordID:          expense.ID,
		OutletID:          expense.OutletID,
		Description:       journal.Description,
		OccurredAt:        expense.ExpenseDate,
		SourceType:        "pos_operational_expense",
		SourceID:          fmt.Sprintf("%d", expense.ID),
		ExternalReference: externalReference,
		LegacyType:        "general",
		Metadata: map[string]interface{}{
			"journal_entry_id": journal.ID,
			"expense_category": expense.ExpenseCategory,
			"account_purpose":  expense.AccountPurpose,
			"payment_method":   expense.PaymentMethod,
			"vendor_name":      expense.VendorName,
			"status":           expense.Status,
		},
		JournalLines: cloneJournalLines(journal.JournalLines),
	}, nil
}

func buildVendorBillSyncRequest(recordID uint) (*AccountingSyncRequest, error) {
	var bill models.VendorBill
	if err := database.DB.First(&bill, recordID).Error; err != nil {
		return nil, err
	}

	journal, err := loadJournalEntryByID(bill.JournalEntryID)
	if err != nil {
		return nil, err
	}

	externalReference := strings.TrimSpace(bill.BillNo)
	if externalReference == "" {
		externalReference = fmt.Sprintf("POS-BILL-%d", bill.ID)
	}

	metadata := map[string]interface{}{
		"journal_entry_id":   journal.ID,
		"vendor_name":        bill.VendorName,
		"bill_type":          bill.BillType,
		"account_purpose":    bill.AccountPurpose,
		"use_prepaid":        bill.UsePrepaid,
		"paid_amount":        bill.PaidAmount,
		"outstanding_amount": bill.OutstandingAmount,
		"status":             bill.Status,
	}
	if bill.DueDate != nil {
		metadata["due_date"] = bill.DueDate.Format(time.RFC3339)
	}

	return &AccountingSyncRequest{
		TableName:         bill.TableName(),
		RecordID:          bill.ID,
		OutletID:          bill.OutletID,
		Description:       journal.Description,
		OccurredAt:        bill.BillDate,
		SourceType:        "pos_vendor_bill",
		SourceID:          fmt.Sprintf("%d", bill.ID),
		ExternalReference: externalReference,
		LegacyType:        "general",
		Metadata:          metadata,
		JournalLines:      cloneJournalLines(journal.JournalLines),
	}, nil
}

func buildVendorPaymentSyncRequest(recordID uint) (*AccountingSyncRequest, error) {
	var payment models.VendorPayment
	if err := database.DB.Preload("Allocations").First(&payment, recordID).Error; err != nil {
		return nil, err
	}

	journal, err := loadJournalEntryByID(payment.JournalEntryID)
	if err != nil {
		return nil, err
	}

	billIDs := make([]uint, 0, len(payment.Allocations))
	for _, allocation := range payment.Allocations {
		billIDs = append(billIDs, allocation.VendorBillID)
	}

	externalReference := strings.TrimSpace(payment.PaymentNo)
	if externalReference == "" {
		externalReference = fmt.Sprintf("POS-VPAY-%d", payment.ID)
	}

	return &AccountingSyncRequest{
		TableName:         payment.TableName(),
		RecordID:          payment.ID,
		OutletID:          payment.OutletID,
		Description:       journal.Description,
		OccurredAt:        payment.PaymentDate,
		SourceType:        "pos_vendor_payment",
		SourceID:          fmt.Sprintf("%d", payment.ID),
		ExternalReference: externalReference,
		LegacyType:        "general",
		Metadata: map[string]interface{}{
			"journal_entry_id": journal.ID,
			"vendor_name":      payment.VendorName,
			"payment_method":   payment.PaymentMethod,
			"status":           payment.Status,
			"bill_ids":         billIDs,
			"allocation_count": len(payment.Allocations),
		},
		JournalLines: cloneJournalLines(journal.JournalLines),
	}, nil
}

func buildStockAdjustmentSyncRequest(recordID uint) (*AccountingSyncRequest, error) {
	var adjustment models.StockAdjustment
	if err := database.DB.First(&adjustment, recordID).Error; err != nil {
		return nil, err
	}

	journal, err := loadJournalEntryByID(adjustment.JournalEntryID)
	if err != nil {
		return nil, err
	}

	return &AccountingSyncRequest{
		TableName:         adjustment.TableName(),
		RecordID:          adjustment.ID,
		OutletID:          adjustment.OutletID,
		Description:       journal.Description,
		OccurredAt:        adjustment.AdjustmentDate,
		SourceType:        "pos_stock_adjustment",
		SourceID:          fmt.Sprintf("%d", adjustment.ID),
		ExternalReference: fmt.Sprintf("POS-STKADJ-%d", adjustment.ID),
		LegacyType:        "general",
		Metadata: map[string]interface{}{
			"journal_entry_id": journal.ID,
			"reason":           adjustment.Reason,
			"status":           adjustment.Status,
		},
		JournalLines: cloneJournalLines(journal.JournalLines),
	}, nil
}

func buildStockOpnameSyncRequest(recordID uint) (*AccountingSyncRequest, error) {
	var opname models.StockOpname
	if err := database.DB.First(&opname, recordID).Error; err != nil {
		return nil, err
	}

	journal, err := loadJournalEntryByID(opname.JournalEntryID)
	if err != nil {
		return nil, err
	}

	return &AccountingSyncRequest{
		TableName:         opname.TableName(),
		RecordID:          opname.ID,
		OutletID:          opname.OutletID,
		Description:       journal.Description,
		OccurredAt:        opname.OpnameDate,
		SourceType:        "pos_stock_opname",
		SourceID:          fmt.Sprintf("%d", opname.ID),
		ExternalReference: fmt.Sprintf("POS-OPNAME-%d", opname.ID),
		LegacyType:        "general",
		Metadata: map[string]interface{}{
			"journal_entry_id": journal.ID,
			"status":           opname.Status,
		},
		JournalLines: cloneJournalLines(journal.JournalLines),
	}, nil
}

func BuildAccountingSyncRequests(recordType string, recordID uint) ([]AccountingSyncRequest, error) {
	switch normalizeAccountingRecordType(recordType) {
	case AccountingRecordTypeSale:
		return buildSaleSyncRequests(recordID)
	case AccountingRecordTypeRefund:
		request, err := buildRefundSyncRequest(recordID)
		if err != nil {
			return nil, err
		}
		return []AccountingSyncRequest{*request}, nil
	case AccountingRecordTypePurchase:
		request, err := buildPurchaseSyncRequest(recordID)
		if err != nil {
			return nil, err
		}
		return []AccountingSyncRequest{*request}, nil
	case AccountingRecordTypeOperationalExpense:
		request, err := buildOperationalExpenseSyncRequest(recordID)
		if err != nil {
			return nil, err
		}
		return []AccountingSyncRequest{*request}, nil
	case AccountingRecordTypeVendorBill:
		request, err := buildVendorBillSyncRequest(recordID)
		if err != nil {
			return nil, err
		}
		return []AccountingSyncRequest{*request}, nil
	case AccountingRecordTypeVendorPayment:
		request, err := buildVendorPaymentSyncRequest(recordID)
		if err != nil {
			return nil, err
		}
		return []AccountingSyncRequest{*request}, nil
	case AccountingRecordTypeStockAdjustment:
		request, err := buildStockAdjustmentSyncRequest(recordID)
		if err != nil {
			return nil, err
		}
		return []AccountingSyncRequest{*request}, nil
	case AccountingRecordTypeStockOpname:
		request, err := buildStockOpnameSyncRequest(recordID)
		if err != nil {
			return nil, err
		}
		return []AccountingSyncRequest{*request}, nil
	default:
		return nil, fmt.Errorf("unsupported accounting record type: %s", recordType)
	}
}

func BuildAccountingSyncRequest(recordType string, recordID uint) (*AccountingSyncRequest, error) {
	requests, err := BuildAccountingSyncRequests(recordType, recordID)
	if err != nil {
		return nil, err
	}
	if len(requests) == 0 {
		return nil, fmt.Errorf("no accounting sync request built for %s:%d", recordType, recordID)
	}

	return &requests[0], nil
}

func SyncAccountingRecord(recordType string, recordID uint) (*AccountingSyncResult, error) {
	requests, err := BuildAccountingSyncRequests(recordType, recordID)
	if err != nil {
		return nil, err
	}

	var primaryResult *AccountingSyncResult
	for index, request := range requests {
		result, syncErr := SyncPOSJournalEntry(request)
		if syncErr != nil {
			if primaryResult == nil {
				primaryResult = result
			} else if primaryResult != nil && primaryResult.Error == "" && result != nil {
				primaryResult.Error = result.Error
			}
			return primaryResult, syncErr
		}

		if index == 0 || primaryResult == nil {
			primaryResult = result
		}
	}

	if primaryResult == nil {
		return nil, fmt.Errorf("accounting sync did not produce any result")
	}

	return primaryResult, nil
}

func accountingSyncRowsQuery() string {
	return `
		WITH sync_rows AS (
			SELECT
				'sale'::text AS record_type,
				id AS record_id,
				outlet_id,
				total AS amount,
				COALESCE(note, '') AS description,
				CONCAT('INV-POS-', outlet_id, '-', TO_CHAR(created_at, 'YYYYMMDD'), '-', LPAD(id::text, 6, '0')) AS external_reference,
				COALESCE(NULLIF(accounting_sync_status, ''), 'pending') AS sync_status,
				COALESCE(accounting_sync_error, '') AS sync_error,
				accounting_synced_at AS synced_at,
				created_at AS occurred_at,
				COALESCE(NULLIF(accounting_idempotency_key, ''), CONCAT('pos:sale:', id)) AS idempotency_key,
				journal_entry_id
			FROM tbltransactions
			UNION ALL
			SELECT
				'refund'::text AS record_type,
				id AS record_id,
				outlet_id,
				refund_total AS amount,
				COALESCE(note, '') AS description,
				CONCAT('POS-REFUND-', id) AS external_reference,
				COALESCE(NULLIF(accounting_sync_status, ''), 'pending') AS sync_status,
				COALESCE(accounting_sync_error, '') AS sync_error,
				accounting_synced_at AS synced_at,
				created_at AS occurred_at,
				COALESCE(NULLIF(accounting_idempotency_key, ''), CONCAT('pos:refund:', id)) AS idempotency_key,
				journal_entry_id
			FROM tblrefunds
			UNION ALL
			SELECT
				'purchase'::text AS record_type,
				id AS record_id,
				outlet_id,
				total AS amount,
				COALESCE(note, '') AS description,
				COALESCE(NULLIF(invoice_number, ''), CONCAT('POS-PURCHASE-', id)) AS external_reference,
				COALESCE(NULLIF(accounting_sync_status, ''), 'pending') AS sync_status,
				COALESCE(accounting_sync_error, '') AS sync_error,
				accounting_synced_at AS synced_at,
				COALESCE(purchase_date, created_at) AS occurred_at,
				COALESCE(NULLIF(accounting_idempotency_key, ''), CONCAT('pos:purchase:', id)) AS idempotency_key,
				journal_entry_id
			FROM tblpurchases
			WHERE deleted_at IS NULL
			UNION ALL
			SELECT
				'operational_expense'::text AS record_type,
				id AS record_id,
				outlet_id,
				amount,
				COALESCE(note, '') AS description,
				COALESCE(NULLIF(reference_no, ''), CONCAT('POS-OPEX-', id)) AS external_reference,
				COALESCE(NULLIF(accounting_sync_status, ''), 'pending') AS sync_status,
				COALESCE(accounting_sync_error, '') AS sync_error,
				accounting_synced_at AS synced_at,
				COALESCE(expense_date, created_at) AS occurred_at,
				COALESCE(NULLIF(accounting_idempotency_key, ''), CONCAT('pos:operational_expense:', id)) AS idempotency_key,
				journal_entry_id
			FROM tbloperational_expenses
			WHERE deleted_at IS NULL
			UNION ALL
			SELECT
				'vendor_bill'::text AS record_type,
				id AS record_id,
				outlet_id,
				total_amount AS amount,
				COALESCE(note, '') AS description,
				COALESCE(NULLIF(bill_no, ''), CONCAT('POS-BILL-', id)) AS external_reference,
				COALESCE(NULLIF(accounting_sync_status, ''), 'pending') AS sync_status,
				COALESCE(accounting_sync_error, '') AS sync_error,
				accounting_synced_at AS synced_at,
				COALESCE(bill_date, created_at) AS occurred_at,
				COALESCE(NULLIF(accounting_idempotency_key, ''), CONCAT('pos:vendor_bill:', id)) AS idempotency_key,
				journal_entry_id
			FROM tblvendor_bills
			WHERE deleted_at IS NULL
			UNION ALL
			SELECT
				'vendor_payment'::text AS record_type,
				id AS record_id,
				outlet_id,
				amount,
				COALESCE(note, '') AS description,
				COALESCE(NULLIF(payment_no, ''), CONCAT('POS-VPAY-', id)) AS external_reference,
				COALESCE(NULLIF(accounting_sync_status, ''), 'pending') AS sync_status,
				COALESCE(accounting_sync_error, '') AS sync_error,
				accounting_synced_at AS synced_at,
				COALESCE(payment_date, created_at) AS occurred_at,
				COALESCE(NULLIF(accounting_idempotency_key, ''), CONCAT('pos:vendor_payment:', id)) AS idempotency_key,
				journal_entry_id
			FROM tblvendor_payments
			WHERE deleted_at IS NULL
			UNION ALL
			SELECT
				'stock_adjustment'::text AS record_type,
				id AS record_id,
				outlet_id,
				0::numeric AS amount,
				COALESCE(note, '') AS description,
				CONCAT('POS-STKADJ-', id) AS external_reference,
				COALESCE(NULLIF(accounting_sync_status, ''), 'pending') AS sync_status,
				COALESCE(accounting_sync_error, '') AS sync_error,
				accounting_synced_at AS synced_at,
				COALESCE(adjustment_date, created_at) AS occurred_at,
				COALESCE(NULLIF(accounting_idempotency_key, ''), CONCAT('pos:stock_adjustment:', id)) AS idempotency_key,
				journal_entry_id
			FROM tblstock_adjustments
			WHERE deleted_at IS NULL
			UNION ALL
			SELECT
				'stock_opname'::text AS record_type,
				id AS record_id,
				outlet_id,
				0::numeric AS amount,
				COALESCE(note, '') AS description,
				CONCAT('POS-OPNAME-', id) AS external_reference,
				COALESCE(NULLIF(accounting_sync_status, ''), 'pending') AS sync_status,
				COALESCE(accounting_sync_error, '') AS sync_error,
				accounting_synced_at AS synced_at,
				COALESCE(opname_date, created_at) AS occurred_at,
				COALESCE(NULLIF(accounting_idempotency_key, ''), CONCAT('pos:stock_opname:', id)) AS idempotency_key,
				journal_entry_id
			FROM tblstock_opnames
			WHERE deleted_at IS NULL
		)
	`
}

func buildAccountingSyncWhereClause(filter AccountingSyncListFilter) (string, []interface{}) {
	clauses := make([]string, 0, 4)
	args := make([]interface{}, 0, 4)

	if filter.OutletID > 0 {
		clauses = append(clauses, "outlet_id = ?")
		args = append(args, filter.OutletID)
	}
	if status := strings.ToLower(strings.TrimSpace(filter.Status)); status != "" {
		clauses = append(clauses, "LOWER(sync_status) = ?")
		args = append(args, status)
	}
	if recordType := normalizeAccountingRecordType(filter.RecordType); recordType != "" {
		clauses = append(clauses, "record_type = ?")
		args = append(args, recordType)
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		like := "%" + search + "%"
		clauses = append(clauses, "(external_reference ILIKE ? OR description ILIKE ?)")
		args = append(args, like, like)
	}

	if len(clauses) == 0 {
		return "", args
	}

	return " WHERE " + strings.Join(clauses, " AND "), args
}

func ListAccountingSyncRecords(filter AccountingSyncListFilter) ([]AccountingSyncListItem, error) {
	if err := EnsureAccountingSyncSchema(database.DB); err != nil {
		return nil, err
	}

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}

	query := accountingSyncRowsQuery() + `
		SELECT
			record_type,
			record_id,
			outlet_id,
			amount,
			description,
			external_reference,
			sync_status,
			sync_error,
			synced_at,
			occurred_at,
			idempotency_key,
			journal_entry_id
		FROM sync_rows
	`
	whereClause, args := buildAccountingSyncWhereClause(filter)
	query += whereClause + " ORDER BY occurred_at DESC, record_id DESC LIMIT ?"
	args = append(args, limit)

	var records []AccountingSyncListItem
	if err := database.DB.Raw(query, args...).Scan(&records).Error; err != nil {
		return nil, err
	}

	return records, nil
}

func GetAccountingSyncSummary(outletID uint) (*AccountingSyncSummary, error) {
	if err := EnsureAccountingSyncSchema(database.DB); err != nil {
		return nil, err
	}

	filter := AccountingSyncListFilter{OutletID: outletID}
	whereClause, args := buildAccountingSyncWhereClause(filter)

	type accountingSyncSummaryRow struct {
		Total   int64 `gorm:"column:total"`
		Synced  int64 `gorm:"column:synced"`
		Pending int64 `gorm:"column:pending"`
		Failed  int64 `gorm:"column:failed"`
	}

	var summaryRow accountingSyncSummaryRow
	summaryQuery := accountingSyncRowsQuery() + `
			SELECT
				COUNT(*) AS total,
				COALESCE(SUM(CASE WHEN LOWER(sync_status) = 'synced' THEN 1 ELSE 0 END), 0) AS synced,
				COALESCE(SUM(CASE WHEN LOWER(sync_status) = 'pending' THEN 1 ELSE 0 END), 0) AS pending,
				COALESCE(SUM(CASE WHEN LOWER(sync_status) = 'failed' THEN 1 ELSE 0 END), 0) AS failed
			FROM sync_rows
		` + whereClause
	if err := database.DB.Raw(summaryQuery, args...).Scan(&summaryRow).Error; err != nil {
		return nil, err
	}

	typeSummaries := make([]AccountingSyncTypeSummary, 0)
	typeQuery := accountingSyncRowsQuery() + `
		SELECT
			record_type,
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN LOWER(sync_status) = 'synced' THEN 1 ELSE 0 END), 0) AS synced,
			COALESCE(SUM(CASE WHEN LOWER(sync_status) = 'pending' THEN 1 ELSE 0 END), 0) AS pending,
			COALESCE(SUM(CASE WHEN LOWER(sync_status) = 'failed' THEN 1 ELSE 0 END), 0) AS failed
		FROM sync_rows
	` + whereClause + ` GROUP BY record_type ORDER BY record_type ASC`
	if err := database.DB.Raw(typeQuery, args...).Scan(&typeSummaries).Error; err != nil {
		return nil, err
	}

	summary := &AccountingSyncSummary{
		Total:         summaryRow.Total,
		Synced:        summaryRow.Synced,
		Pending:       summaryRow.Pending,
		Failed:        summaryRow.Failed,
		ByRecordType:  typeSummaries,
		LastUpdatedAt: time.Now(),
	}
	summary.NeedsRetry = summary.Pending + summary.Failed

	return summary, nil
}

func postAccountingPayload(payload *accountingPostPayload) (*accountingPostResponse, error) {
	apiURL := strings.TrimRight(strings.TrimSpace(os.Getenv("APIFITNESS_API_URL")), "/")
	if apiURL == "" {
		return nil, fmt.Errorf("APIFITNESS_API_URL is not configured")
	}

	sharedKey := strings.TrimSpace(os.Getenv("APIFITNESS_ACCOUNTING_SHARED_KEY"))
	if sharedKey == "" {
		return nil, fmt.Errorf("APIFITNESS_ACCOUNTING_SHARED_KEY is not configured")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL+"/accounting/journal/post", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Accounting-Key", sharedKey)

	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("apifitness accounting post failed (%d): %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var parsed accountingPostResponse
	if len(responseBody) == 0 {
		return &parsed, nil
	}
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		utils.Log.Warnf("failed to parse apifitness accounting response: %v", err)
		return &parsed, nil
	}

	return &parsed, nil
}

func trimAccountingError(message string) string {
	message = strings.TrimSpace(message)
	if len(message) <= 500 {
		return message
	}
	return message[:500]
}
