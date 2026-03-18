package controllers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/services"
	"github.com/kadeyasa/possystem/utils"
	"gorm.io/gorm"
)

type PurchaseInput struct {
	OutletID      uint                  `json:"outlet_id"`
	SupplierName  string                `json:"supplier_name"`
	InvoiceNumber string                `json:"invoice_number"`
	PurchaseDate  time.Time             `json:"purchase_date"`
	DueDate       *time.Time            `json:"due_date"`
	Total         float64               `json:"total"`
	Note          string                `json:"note"`
	Items         []models.PurchaseItem `json:"items"`
	PaymentMethod string                `json:"payment_method"` // "cash" or "credit"
}

func CreatePurchase(c *gin.Context) {
	var input PurchaseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Log.Warnf("❌ Bind JSON failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := services.EnsureAccountingSyncSchema(database.DB); err != nil {
		utils.Log.Errorf("❌ Failed to ensure accounting sync schema: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare accounting sync schema"})
		return
	}
	if err := services.EnsurePOSFinanceDocumentSchema(database.DB); err != nil {
		utils.Log.Errorf("❌ Failed to ensure POS finance schema: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare POS finance schema"})
		return
	}
	if err := services.EnsureInventorySchema(database.DB); err != nil {
		utils.Log.Errorf("❌ Failed to ensure inventory schema: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare inventory schema"})
		return
	}

	paymentMethod := strings.ToLower(strings.TrimSpace(input.PaymentMethod))
	if paymentMethod == "" {
		paymentMethod = "cash"
	}
	switch paymentMethod {
	case "cash", "transfer", "qris", "credit":
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "payment_method must be cash, transfer, qris, or credit",
		})
		return
	}

	if strings.TrimSpace(input.SupplierName) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "supplier_name is required"})
		return
	}
	if input.Total <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "total must be greater than zero"})
		return
	}
	if paymentMethod == "credit" && input.DueDate == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "due_date wajib diisi untuk purchase tempo agar hutang vendor bisa dipantau.",
		})
		return
	}

	if invoiceNumber := strings.TrimSpace(input.InvoiceNumber); invoiceNumber != "" {
		conflictQuery := database.DB.
			Model(&models.VendorBill{}).
			Where("outlet_id = ? AND bill_no = ?", input.OutletID, invoiceNumber)

		if vendorName := strings.TrimSpace(input.SupplierName); vendorName != "" {
			conflictQuery = conflictQuery.Where("LOWER(COALESCE(vendor_name, '')) = ?", strings.ToLower(vendorName))
		}

		var existingBill models.VendorBill
		if err := conflictQuery.First(&existingBill).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{
				"error": fmt.Sprintf("Invoice %s sudah tercatat sebagai Vendor Bill. Jangan buat Purchase untuk invoice yang sama.", invoiceNumber),
			})
			return
		}
	}

	var (
		purchase models.Purchase
		journal  models.JournalEntry
		bill     models.VendorBill
	)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		lockedProducts, err := services.ValidatePurchaseProducts(tx, input.OutletID, input.Items)
		if err != nil {
			return err
		}

		debitAccount, err := services.ResolveAccountForOutlet(tx, input.OutletID, "purchase", "inventory")
		if err != nil {
			return err
		}

		var creditAccount models.Account
		if paymentMethod == "credit" {
			creditAccount, err = services.ResolveVendorPayableAccount(tx, input.OutletID)
			if err != nil {
				return err
			}
		} else {
			creditAccount, err = services.ResolveAccountForOutlet(tx, input.OutletID, "purchase", paymentMethod)
			if err != nil {
				return err
			}
		}

		purchaseDate := input.PurchaseDate
		if purchaseDate.IsZero() {
			purchaseDate = time.Now()
		}

		journalCreditDescription := "Pembayaran pembelian"
		if paymentMethod == "credit" {
			journalCreditDescription = "Hutang pembelian inventory"
		}

		journal = models.JournalEntry{
			OutletID:    input.OutletID,
			Reference:   input.InvoiceNumber,
			Description: "Pembelian dari " + input.SupplierName,
			EntryDate:   purchaseDate,
			JournalLines: []models.JournalLine{
				{
					AccountID:   debitAccount.ID,
					Debit:       input.Total,
					Credit:      0,
					Description: "Persediaan dari pembelian",
				},
				{
					AccountID:   creditAccount.ID,
					Debit:       0,
					Credit:      input.Total,
					Description: journalCreditDescription,
				},
			},
		}

		if err := tx.Create(&journal).Error; err != nil {
			return err
		}

		paidAmount := input.Total
		outstandingAmount := 0.0
		paymentStatus := services.PurchasePaymentStatusPaid
		if paymentMethod == "credit" {
			paidAmount = 0
			outstandingAmount = input.Total
			paymentStatus = services.PurchasePaymentStatusOpen
		}

		purchase = models.Purchase{
			OutletID:              input.OutletID,
			SupplierName:          strings.TrimSpace(input.SupplierName),
			InvoiceNumber:         strings.TrimSpace(input.InvoiceNumber),
			PurchaseDate:          purchaseDate,
			DueDate:               input.DueDate,
			Total:                 input.Total,
			PaidAmount:            paidAmount,
			OutstandingAmount:     outstandingAmount,
			PaymentMethod:         paymentMethod,
			PaymentStatus:         paymentStatus,
			Note:                  strings.TrimSpace(input.Note),
			JournalEntryID:        &journal.ID,
			AccountingSyncStatus:  services.AccountingSyncStatusPending,
			AccountingIdempotency: services.BuildAccountingIdempotencyKey("pos_purchase", 0),
		}
		if err := tx.Create(&purchase).Error; err != nil {
			return err
		}
		purchase.AccountingIdempotency = services.BuildAccountingIdempotencyKey("pos_purchase", purchase.ID)
		if err := tx.Model(&purchase).Updates(map[string]interface{}{
			"accounting_sync_status":     services.AccountingSyncStatusPending,
			"accounting_idempotency_key": purchase.AccountingIdempotency,
		}).Error; err != nil {
			return err
		}

		if paymentMethod == "credit" {
			now := time.Now()
			bill = models.VendorBill{
				OutletID:              input.OutletID,
				PurchaseID:            &purchase.ID,
				VendorName:            strings.TrimSpace(input.SupplierName),
				BillNo:                strings.TrimSpace(input.InvoiceNumber),
				BillDate:              purchaseDate,
				DueDate:               input.DueDate,
				BillType:              services.VendorBillTypeInventory,
				Subtotal:              input.Total,
				TaxAmount:             0,
				TotalAmount:           input.Total,
				PaidAmount:            0,
				OutstandingAmount:     input.Total,
				Status:                services.VendorBillStatusOpen,
				JournalEntryID:        &journal.ID,
				AccountingSyncStatus:  services.AccountingSyncStatusSynced,
				AccountingSyncedAt:    &now,
				AccountingIdempotency: purchase.AccountingIdempotency,
				Note:                  strings.TrimSpace(input.Note),
			}
			if err := tx.Create(&bill).Error; err != nil {
				return err
			}
			purchase.LinkedVendorBillID = &bill.ID
			if err := tx.Model(&purchase).Update("linked_vendor_bill_id", bill.ID).Error; err != nil {
				return err
			}
		}

		purchasedProductIDs := make([]uint, 0, len(input.Items))
		for _, item := range input.Items {
			product := lockedProducts[item.ProductID]
			services.NormalizePurchaseItemForInventory(&item, product)
			if item.StockQuantity <= 0 {
				return fmt.Errorf("stock quantity must be greater than zero for product %d", item.ProductID)
			}

			item.PurchaseID = purchase.ID
			if err := tx.Create(&item).Error; err != nil {
				return err
			}

			stockBefore := product.Stock
			stockAfter := stockBefore + item.StockQuantity

			if err := tx.Model(&models.Product{}).
				Where("id = ? AND outlet_id = ?", item.ProductID, input.OutletID).
				Updates(map[string]interface{}{
					"stock":               stockAfter,
					"last_purchase_price": item.UnitCostInStockUnit,
				}).Error; err != nil {
				return err
			}
			product.Stock = stockAfter
			product.LastPurchasePrice = item.UnitCostInStockUnit

			if err := services.AppendInventoryLedger(tx, models.InventoryLedger{
				OutletID:      input.OutletID,
				ProductID:     item.ProductID,
				MovementType:  services.InventoryMovementPurchase,
				ReferenceType: services.InventoryReferencePurchase,
				ReferenceID:   purchase.ID,
				QuantityIn:    item.StockQuantity,
				QuantityOut:   0,
				StockBefore:   stockBefore,
				StockAfter:    stockAfter,
				UnitCost:      item.UnitCostInStockUnit,
				TotalCost:     item.Total,
				Notes:         fmt.Sprintf("Purchase #%d (%d %s)", purchase.ID, item.Quantity, strings.TrimSpace(item.PurchaseUnit)),
			}); err != nil {
				return err
			}

			purchasedProductIDs = append(purchasedProductIDs, item.ProductID)
		}

		if err := services.SyncFinishedGoodRecipeCostsForIngredients(tx, input.OutletID, purchasedProductIDs); err != nil {
			return err
		}

		purchaseAuditSnapshot := gin.H{
			"purchase":               purchase,
			"items":                  input.Items,
			"payment_method":         paymentMethod,
			"linked_vendor_bill_id":  purchase.LinkedVendorBillID,
			"accounting_sync_status": purchase.AccountingSyncStatus,
		}
		if err := recordDocumentAudit(tx, c, documentAuditInput{
			OutletID:     purchase.OutletID,
			DocumentType: "purchase",
			DocumentID:   purchase.ID,
			Action:       documentAuditActionCreated,
			Summary:      fmt.Sprintf("Purchase #%d dibuat untuk vendor %s", purchase.ID, purchase.SupplierName),
			Note:         purchase.Note,
			Metadata: gin.H{
				"invoice_number": purchase.InvoiceNumber,
				"payment_method": paymentMethod,
				"payment_status": purchase.PaymentStatus,
			},
			After: purchaseAuditSnapshot,
		}); err != nil {
			return err
		}

		if bill.ID > 0 {
			if err := recordDocumentAudit(tx, c, documentAuditInput{
				OutletID:     bill.OutletID,
				DocumentType: "vendor_bill",
				DocumentID:   bill.ID,
				Action:       documentAuditActionCreatedFromSource,
				Summary:      fmt.Sprintf("Vendor bill #%d dibuat dari purchase #%d", bill.ID, purchase.ID),
				Note:         bill.Note,
				Metadata: gin.H{
					"source_document_type": "purchase",
					"source_document_id":   purchase.ID,
					"bill_type":            bill.BillType,
					"payment_status":       bill.Status,
				},
				After: gin.H{
					"vendor_bill": bill,
					"purchase_id": purchase.ID,
				},
			}); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		utils.Log.Errorf("❌ Failed to create purchase: %v", err)
		if isInventoryValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	utils.Log.Infof("✅ Purchase created with invoice: %v", input.InvoiceNumber)
	syncResult, syncErr := services.SyncAccountingRecord(services.AccountingRecordTypePurchase, purchase.ID)
	if syncErr != nil {
		utils.Log.Warnf("purchase %d synced locally but failed to post accounting journal: %v", purchase.ID, syncErr)
		c.JSON(http.StatusOK, gin.H{
			"message":                    "Purchase recorded successfully",
			"data":                       purchase,
			"linked_vendor_bill_id":      purchase.LinkedVendorBillID,
			"accounting_sync_status":     services.AccountingSyncStatusFailed,
			"accounting_sync_error":      syncErr.Error(),
			"accounting_idempotency_key": services.BuildAccountingIdempotencyKey("pos_purchase", purchase.ID),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":                    "Purchase recorded successfully",
		"data":                       purchase,
		"linked_vendor_bill_id":      purchase.LinkedVendorBillID,
		"accounting_sync_status":     syncResult.Status,
		"accounting_idempotency_key": syncResult.IdempotencyKey,
		"accounting_journal_id":      syncResult.JournalID,
		"accounting_already_posted":  syncResult.AlreadyPosted,
	})
}
