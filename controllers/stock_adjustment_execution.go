package controllers

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/services"
	"gorm.io/gorm"
)

type preparedStockAdjustmentItem struct {
	product     *models.Product
	productID   uint
	productName string
	delta       int
	stockBefore int
	stockAfter  int
	unitCost    float64
	totalCost   float64
	note        string
}

func prepareStockAdjustmentItems(tx *gorm.DB, input StockAdjustmentInput) ([]preparedStockAdjustmentItem, float64, float64, error) {
	productIDs := make([]uint, 0, len(input.Items))
	seen := make(map[uint]struct{}, len(input.Items))
	for _, item := range input.Items {
		if item.ProductID == 0 {
			return nil, 0, 0, fmt.Errorf("product_id is required for every adjustment item")
		}
		if item.QuantityDelta == 0 {
			return nil, 0, 0, fmt.Errorf("quantity_delta must not be zero for product %d", item.ProductID)
		}
		if _, ok := seen[item.ProductID]; ok {
			return nil, 0, 0, fmt.Errorf("duplicate product_id %d in stock adjustment request", item.ProductID)
		}
		seen[item.ProductID] = struct{}{}
		productIDs = append(productIDs, item.ProductID)
	}
	sort.Slice(productIDs, func(i, j int) bool { return productIDs[i] < productIDs[j] })

	lockedProducts, err := services.LockProductsForInventory(tx, input.OutletID, productIDs)
	if err != nil {
		return nil, 0, 0, err
	}

	prepared := make([]preparedStockAdjustmentItem, 0, len(input.Items))
	var totalGainValue, totalLossValue float64
	for _, item := range input.Items {
		product, exists := lockedProducts[item.ProductID]
		if !exists || product == nil {
			return nil, 0, 0, fmt.Errorf("product %d not found for outlet %d", item.ProductID, input.OutletID)
		}
		if !services.IsStockTrackedItemType(product.ItemType) {
			return nil, 0, 0, fmt.Errorf("product %s (%d) is not stock tracked", strings.TrimSpace(product.Name), product.ID)
		}

		stockBefore := product.Stock
		stockAfter := stockBefore + item.QuantityDelta
		if stockAfter < 0 {
			return nil, 0, 0, fmt.Errorf("stock adjustment would make stock negative for product %s (%d)", strings.TrimSpace(product.Name), product.ID)
		}

		unitCost, err := services.ResolveInventoryUnitCost(tx, input.OutletID, item.ProductID, nil)
		if err != nil {
			return nil, 0, 0, err
		}

		totalCost := unitCost * float64(absInt(item.QuantityDelta))
		if item.QuantityDelta > 0 {
			totalGainValue += totalCost
		} else {
			totalLossValue += totalCost
		}

		productName := strings.TrimSpace(product.Name)
		if productName == "" {
			productName = fmt.Sprintf("Product #%d", product.ID)
		}

		prepared = append(prepared, preparedStockAdjustmentItem{
			product:     product,
			productID:   item.ProductID,
			productName: productName,
			delta:       item.QuantityDelta,
			stockBefore: stockBefore,
			stockAfter:  stockAfter,
			unitCost:    unitCost,
			totalCost:   totalCost,
			note:        strings.TrimSpace(item.Note),
		})
	}

	return prepared, totalGainValue, totalLossValue, nil
}

func resolveStockAdjustmentDate(input StockAdjustmentInput) time.Time {
	adjustmentDate := input.AdjustmentDate
	if adjustmentDate.IsZero() {
		adjustmentDate = time.Now()
	}
	return adjustmentDate
}

func buildStockAdjustmentJournal(tx *gorm.DB, outletID uint, adjustmentDate time.Time, totalGainValue, totalLossValue float64) (models.JournalEntry, bool, error) {
	journal := models.JournalEntry{}
	if totalGainValue <= 0 && totalLossValue <= 0 {
		return journal, false, nil
	}

	inventoryAccount, err := services.ResolveInventoryAssetAccount(tx, outletID)
	if err != nil {
		return journal, false, err
	}

	lines := make([]models.JournalLine, 0, 4)
	if totalLossValue > 0 {
		lossAccount, err := services.ResolveStockAdjustmentLossAccount(tx, outletID)
		if err != nil {
			return journal, false, err
		}
		lines = append(lines,
			models.JournalLine{
				AccountID:   lossAccount.ID,
				Debit:       totalLossValue,
				Credit:      0,
				Description: "Stock adjustment loss",
			},
			models.JournalLine{
				AccountID:   inventoryAccount.ID,
				Debit:       0,
				Credit:      totalLossValue,
				Description: "Inventory decrease from stock adjustment",
			},
		)
	}
	if totalGainValue > 0 {
		gainAccount, err := services.ResolveStockAdjustmentGainAccount(tx, outletID)
		if err != nil {
			return journal, false, err
		}
		lines = append(lines,
			models.JournalLine{
				AccountID:   inventoryAccount.ID,
				Debit:       totalGainValue,
				Credit:      0,
				Description: "Inventory increase from stock adjustment",
			},
			models.JournalLine{
				AccountID:   gainAccount.ID,
				Debit:       0,
				Credit:      totalGainValue,
				Description: "Stock adjustment gain",
			},
		)
	}

	journal = models.JournalEntry{
		OutletID:     outletID,
		Reference:    "Stock Adjustment",
		Description:  "Penyesuaian stok manual",
		EntryDate:    adjustmentDate,
		JournalLines: lines,
	}
	if err := tx.Create(&journal).Error; err != nil {
		return journal, false, err
	}

	return journal, true, nil
}

func executeStockAdjustmentDocument(tx *gorm.DB, input StockAdjustmentInput) (models.StockAdjustment, bool, error) {
	adjustment := models.StockAdjustment{}

	prepared, totalGainValue, totalLossValue, err := prepareStockAdjustmentItems(tx, input)
	if err != nil {
		return adjustment, false, err
	}

	adjustmentDate := resolveStockAdjustmentDate(input)
	journal, hasJournal, err := buildStockAdjustmentJournal(tx, input.OutletID, adjustmentDate, totalGainValue, totalLossValue)
	if err != nil {
		return adjustment, false, err
	}

	adjustment = models.StockAdjustment{
		OutletID:       input.OutletID,
		AdjustmentDate: adjustmentDate,
		Reason:         strings.TrimSpace(input.Reason),
		Status:         services.StockDocumentStatusPosted,
		Note:           strings.TrimSpace(input.Note),
	}
	if hasJournal {
		adjustment.JournalEntryID = &journal.ID
		adjustment.AccountingSyncStatus = services.AccountingSyncStatusPending
		adjustment.AccountingIdempotency = services.BuildAccountingIdempotencyKey("pos_stock_adjustment", 0)
	} else {
		adjustment.AccountingSyncStatus = services.AccountingSyncStatusSynced
	}
	if err := tx.Create(&adjustment).Error; err != nil {
		return adjustment, false, err
	}

	if hasJournal {
		adjustment.AccountingIdempotency = services.BuildAccountingIdempotencyKey("pos_stock_adjustment", adjustment.ID)
		if err := tx.Model(&adjustment).Updates(map[string]interface{}{
			"accounting_sync_status":     services.AccountingSyncStatusPending,
			"accounting_idempotency_key": adjustment.AccountingIdempotency,
		}).Error; err != nil {
			return adjustment, false, err
		}
	}

	for _, item := range prepared {
		adjustmentItem := models.StockAdjustmentItem{
			AdjustmentID:  adjustment.ID,
			ProductID:     item.productID,
			QuantityDelta: item.delta,
			StockBefore:   item.stockBefore,
			StockAfter:    item.stockAfter,
			UnitCost:      item.unitCost,
			TotalCost:     item.totalCost,
			Note:          item.note,
		}
		if err := tx.Create(&adjustmentItem).Error; err != nil {
			return adjustment, false, err
		}

		if err := tx.Model(&models.Product{}).
			Where("id = ? AND outlet_id = ?", item.productID, input.OutletID).
			Update("stock", item.stockAfter).Error; err != nil {
			return adjustment, false, err
		}
		item.product.Stock = item.stockAfter

		movementType := services.InventoryMovementAdjustmentIn
		qtyIn := item.delta
		qtyOut := 0
		if item.delta < 0 {
			movementType = services.InventoryMovementAdjustmentOut
			qtyIn = 0
			qtyOut = absInt(item.delta)
		}

		if err := services.AppendInventoryLedger(tx, models.InventoryLedger{
			OutletID:      input.OutletID,
			ProductID:     item.productID,
			MovementType:  movementType,
			ReferenceType: services.InventoryReferenceAdjustment,
			ReferenceID:   adjustment.ID,
			QuantityIn:    qtyIn,
			QuantityOut:   qtyOut,
			StockBefore:   item.stockBefore,
			StockAfter:    item.stockAfter,
			UnitCost:      item.unitCost,
			TotalCost:     item.totalCost,
			Notes:         fmt.Sprintf("Stock adjustment #%d", adjustment.ID),
		}); err != nil {
			return adjustment, false, err
		}
	}

	return adjustment, hasJournal, nil
}
