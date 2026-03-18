package controllers

import (
	"fmt"
	"strings"
	"time"

	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/services"
	"gorm.io/gorm"
)

type refundExecutionInput struct {
	TransactionID        uint
	OutletID             uint
	CashierID            uint
	CashierName          string
	Note                 string
	Items                []models.RefundItem
	RefundTotalOverride  *float64
	AccountingSyncStatus string
}

func calculateRefundExecutionTotal(input refundExecutionInput) float64 {
	if input.RefundTotalOverride != nil && *input.RefundTotalOverride > 0 {
		return *input.RefundTotalOverride
	}

	total := 0.0
	for _, item := range input.Items {
		total += item.Total
	}

	return total
}

func executeRefundDocument(tx *gorm.DB, input refundExecutionInput) (models.Refund, models.JournalEntry, error) {
	var (
		refund  models.Refund
		journal models.JournalEntry
	)

	cashierName := strings.TrimSpace(input.CashierName)
	if cashierName == "" && input.CashierID > 0 {
		cashierName = fmt.Sprintf("Cashier #%d", input.CashierID)
	}
	if cashierName == "" {
		cashierName = "System"
	}

	var originalTxn models.Transaction
	if err := tx.First(&originalTxn, input.TransactionID).Error; err != nil {
		return refund, journal, err
	}
	if originalTxn.OutletID != input.OutletID {
		return refund, journal, fmt.Errorf("refund outlet does not match original transaction outlet")
	}
	if strings.EqualFold(strings.TrimSpace(originalTxn.DocumentStatus), services.TransactionDocumentStatusVoided) {
		return refund, journal, fmt.Errorf("transaction has already been voided")
	}

	refundPlan, err := services.PrepareRefundInventoryPlan(tx, input.TransactionID, input.OutletID, input.Items)
	if err != nil {
		return refund, journal, err
	}
	lockedProducts := refundPlan.LockedProducts

	refundTotal := calculateRefundExecutionTotal(input)

	salesAccount, err := services.ResolveAccountForOutlet(tx, input.OutletID, "sale", "sales")
	if err != nil {
		return refund, journal, err
	}
	cashAccount, err := services.ResolveSaleSettlementAccount(tx, input.OutletID, originalTxn.PaymentMethod)
	if err != nil {
		return refund, journal, err
	}

	lines := []models.JournalLine{
		{
			AccountID:   salesAccount.ID,
			Debit:       refundTotal,
			Credit:      0,
			Description: "Pembatalan penjualan",
		},
		{
			AccountID:   cashAccount.ID,
			Debit:       0,
			Credit:      refundTotal,
			Description: "Pengembalian dana ke pelanggan",
		},
	}

	refundCOGS := refundPlan.TotalCost
	if refundCOGS > 0 {
		cogsAccount, found, err := services.TryResolveAccountForOutlet(tx, input.OutletID, "sale", "cogs")
		if err != nil {
			return refund, journal, err
		}
		if found {
			inventoryAccount, foundInventory, err := services.TryResolveAccountForOutlet(tx, input.OutletID, "purchase", "inventory")
			if err != nil {
				return refund, journal, err
			}
			if foundInventory {
				lines = append(lines,
					models.JournalLine{
						AccountID:   inventoryAccount.ID,
						Debit:       refundCOGS,
						Credit:      0,
						Description: "Pengembalian persediaan dari refund",
					},
					models.JournalLine{
						AccountID:   cogsAccount.ID,
						Debit:       0,
						Credit:      refundCOGS,
						Description: "Pembalikan harga pokok penjualan",
					},
				)
			}
		}
	}

	journal = models.JournalEntry{
		OutletID:     input.OutletID,
		Reference:    fmt.Sprintf("Refund Txn #%d", originalTxn.ID),
		Description:  "Refund transaksi penjualan",
		EntryDate:    time.Now(),
		JournalLines: lines,
	}
	if err := tx.Create(&journal).Error; err != nil {
		return refund, journal, err
	}

	accountingSyncStatus := input.AccountingSyncStatus
	if strings.TrimSpace(accountingSyncStatus) == "" {
		accountingSyncStatus = services.AccountingSyncStatusPending
	}

	refund = models.Refund{
		TransactionID:         input.TransactionID,
		OutletID:              input.OutletID,
		CashierID:             input.CashierID,
		CashierName:           cashierName,
		RefundTotal:           refundTotal,
		Note:                  strings.TrimSpace(input.Note),
		JournalEntryID:        &journal.ID,
		AccountingSyncStatus:  accountingSyncStatus,
		AccountingIdempotency: services.BuildAccountingIdempotencyKey("pos_refund", 0),
		CreatedAt:             time.Now(),
	}
	if err := tx.Create(&refund).Error; err != nil {
		return refund, journal, err
	}
	refund.AccountingIdempotency = services.BuildAccountingIdempotencyKey("pos_refund", refund.ID)
	if err := tx.Model(&refund).Updates(map[string]interface{}{
		"accounting_sync_status":     accountingSyncStatus,
		"accounting_idempotency_key": refund.AccountingIdempotency,
	}).Error; err != nil {
		return refund, journal, err
	}

	for _, item := range input.Items {
		item.RefundID = refund.ID
		if err := tx.Create(&item).Error; err != nil {
			return refund, journal, err
		}
	}

	for _, restoration := range refundPlan.Restorations {
		product := lockedProducts[restoration.InventoryProductID]
		if !services.IsStockTrackedItemType(product.ItemType) {
			continue
		}
		stockBefore := product.Stock
		stockAfter := stockBefore + restoration.QuantityConsumed

		if err := tx.Model(&models.Product{}).
			Where("id = ? AND outlet_id = ?", restoration.InventoryProductID, input.OutletID).
			Update("stock", stockAfter).Error; err != nil {
			return refund, journal, err
		}
		product.Stock = stockAfter

		note := fmt.Sprintf("Refund #%d for sale #%d", refund.ID, input.TransactionID)
		if restoration.ConsumptionType == services.InventoryConsumptionTypeRecipeComponent {
			soldProduct := lockedProducts[restoration.SoldProductID]
			note = fmt.Sprintf("Recipe restoration for refund #%d from %s", refund.ID, soldProduct.Name)
		}

		if err := services.AppendInventoryLedger(tx, models.InventoryLedger{
			OutletID:      input.OutletID,
			ProductID:     restoration.InventoryProductID,
			VariantID:     restoration.VariantID,
			MovementType:  services.InventoryMovementRefund,
			ReferenceType: services.InventoryReferenceRefund,
			ReferenceID:   refund.ID,
			QuantityIn:    restoration.QuantityConsumed,
			QuantityOut:   0,
			StockBefore:   stockBefore,
			StockAfter:    stockAfter,
			UnitCost:      restoration.UnitCost,
			TotalCost:     restoration.TotalCost,
			Notes:         note,
		}); err != nil {
			return refund, journal, err
		}
	}

	return refund, journal, nil
}
