package controllers

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/services"
	"gorm.io/gorm"
)

type StockOpnameItemInput struct {
	ProductID   uint   `json:"product_id"`
	ActualStock int    `json:"actual_stock"`
	Note        string `json:"note"`
}

type StockOpnameInput struct {
	OutletID   uint                   `json:"outlet_id"`
	OpnameDate time.Time              `json:"opname_date"`
	Note       string                 `json:"note"`
	Items      []StockOpnameItemInput `json:"items"`
}

func CreateStockOpname(c *gin.Context) {
	var input StockOpnameInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.OutletID == 0 || len(input.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outlet_id and items are required"})
		return
	}

	if err := services.EnsureAccountingSyncSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare accounting sync schema"})
		return
	}
	if err := services.EnsureInventorySchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare inventory schema"})
		return
	}

	var (
		opname     models.StockOpname
		journal    models.JournalEntry
		hasJournal bool
	)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		productIDs := make([]uint, 0, len(input.Items))
		seen := make(map[uint]struct{}, len(input.Items))
		for _, item := range input.Items {
			if item.ProductID == 0 {
				return fmt.Errorf("product_id is required for every opname item")
			}
			if item.ActualStock < 0 {
				return fmt.Errorf("actual_stock must not be negative for product %d", item.ProductID)
			}
			if _, ok := seen[item.ProductID]; !ok {
				seen[item.ProductID] = struct{}{}
				productIDs = append(productIDs, item.ProductID)
			}
		}
		sort.Slice(productIDs, func(i, j int) bool { return productIDs[i] < productIDs[j] })

		lockedProducts, err := services.LockProductsForInventory(tx, input.OutletID, productIDs)
		if err != nil {
			return err
		}

		type preparedItem struct {
			product       *models.Product
			productID     uint
			systemStock   int
			actualStock   int
			differenceQty int
			unitCost      float64
			totalCost     float64
			note          string
		}

		prepared := make([]preparedItem, 0, len(input.Items))
		var totalGainValue, totalLossValue float64
		for _, item := range input.Items {
			product := lockedProducts[item.ProductID]
			if !services.IsStockTrackedItemType(product.ItemType) {
				return fmt.Errorf("product %s (%d) is not stock tracked", strings.TrimSpace(product.Name), product.ID)
			}

			systemStock := product.Stock
			differenceQty := item.ActualStock - systemStock
			unitCost, err := services.ResolveInventoryUnitCost(tx, input.OutletID, item.ProductID, nil)
			if err != nil {
				return err
			}
			totalCost := unitCost * float64(absInt(differenceQty))
			if differenceQty > 0 {
				totalGainValue += totalCost
			} else if differenceQty < 0 {
				totalLossValue += totalCost
			}

			prepared = append(prepared, preparedItem{
				product:       product,
				productID:     item.ProductID,
				systemStock:   systemStock,
				actualStock:   item.ActualStock,
				differenceQty: differenceQty,
				unitCost:      unitCost,
				totalCost:     totalCost,
				note:          strings.TrimSpace(item.Note),
			})
		}

		opnameDate := input.OpnameDate
		if opnameDate.IsZero() {
			opnameDate = time.Now()
		}

		if totalGainValue > 0 || totalLossValue > 0 {
			inventoryAccount, err := services.ResolveInventoryAssetAccount(tx, input.OutletID)
			if err != nil {
				return err
			}

			lines := make([]models.JournalLine, 0, 4)
			if totalLossValue > 0 {
				lossAccount, err := services.ResolveStockAdjustmentLossAccount(tx, input.OutletID)
				if err != nil {
					return err
				}
				lines = append(lines,
					models.JournalLine{
						AccountID:   lossAccount.ID,
						Debit:       totalLossValue,
						Credit:      0,
						Description: "Stock opname loss",
					},
					models.JournalLine{
						AccountID:   inventoryAccount.ID,
						Debit:       0,
						Credit:      totalLossValue,
						Description: "Inventory decrease from stock opname",
					},
				)
			}
			if totalGainValue > 0 {
				gainAccount, err := services.ResolveStockAdjustmentGainAccount(tx, input.OutletID)
				if err != nil {
					return err
				}
				lines = append(lines,
					models.JournalLine{
						AccountID:   inventoryAccount.ID,
						Debit:       totalGainValue,
						Credit:      0,
						Description: "Inventory increase from stock opname",
					},
					models.JournalLine{
						AccountID:   gainAccount.ID,
						Debit:       0,
						Credit:      totalGainValue,
						Description: "Stock opname gain",
					},
				)
			}

			journal = models.JournalEntry{
				OutletID:     input.OutletID,
				Reference:    "Stock Opname",
				Description:  "Penyesuaian stok hasil opname",
				EntryDate:    opnameDate,
				JournalLines: lines,
			}
			if err := tx.Create(&journal).Error; err != nil {
				return err
			}
			hasJournal = true
		}

		opname = models.StockOpname{
			OutletID:   input.OutletID,
			OpnameDate: opnameDate,
			Status:     services.StockDocumentStatusPosted,
			Note:       strings.TrimSpace(input.Note),
		}
		if hasJournal {
			opname.JournalEntryID = &journal.ID
			opname.AccountingSyncStatus = services.AccountingSyncStatusPending
			opname.AccountingIdempotency = services.BuildAccountingIdempotencyKey("pos_stock_opname", 0)
		} else {
			opname.AccountingSyncStatus = services.AccountingSyncStatusSynced
		}
		if err := tx.Create(&opname).Error; err != nil {
			return err
		}

		if hasJournal {
			opname.AccountingIdempotency = services.BuildAccountingIdempotencyKey("pos_stock_opname", opname.ID)
			if err := tx.Model(&opname).Updates(map[string]interface{}{
				"accounting_sync_status":     services.AccountingSyncStatusPending,
				"accounting_idempotency_key": opname.AccountingIdempotency,
			}).Error; err != nil {
				return err
			}
		}

		for _, item := range prepared {
			opnameItem := models.StockOpnameItem{
				OpnameID:      opname.ID,
				ProductID:     item.productID,
				SystemStock:   item.systemStock,
				ActualStock:   item.actualStock,
				DifferenceQty: item.differenceQty,
				UnitCost:      item.unitCost,
				TotalCost:     item.totalCost,
				Note:          item.note,
			}
			if err := tx.Create(&opnameItem).Error; err != nil {
				return err
			}

			if item.differenceQty != 0 {
				if err := tx.Model(&models.Product{}).
					Where("id = ? AND outlet_id = ?", item.productID, input.OutletID).
					Update("stock", item.actualStock).Error; err != nil {
					return err
				}
				item.product.Stock = item.actualStock

				movementType := services.InventoryMovementOpnameGain
				qtyIn := item.differenceQty
				qtyOut := 0
				if item.differenceQty < 0 {
					movementType = services.InventoryMovementOpnameLoss
					qtyIn = 0
					qtyOut = absInt(item.differenceQty)
				}

				if err := services.AppendInventoryLedger(tx, models.InventoryLedger{
					OutletID:      input.OutletID,
					ProductID:     item.productID,
					MovementType:  movementType,
					ReferenceType: services.InventoryReferenceOpname,
					ReferenceID:   opname.ID,
					QuantityIn:    qtyIn,
					QuantityOut:   qtyOut,
					StockBefore:   item.systemStock,
					StockAfter:    item.actualStock,
					UnitCost:      item.unitCost,
					TotalCost:     item.totalCost,
					Notes:         fmt.Sprintf("Stock opname #%d", opname.ID),
				}); err != nil {
					return err
				}
			}
		}

		if err := recordDocumentAudit(tx, c, documentAuditInput{
			OutletID:     opname.OutletID,
			DocumentType: "stock_opname",
			DocumentID:   opname.ID,
			Action:       documentAuditActionCreated,
			Summary:      fmt.Sprintf("Stock opname #%d diposting", opname.ID),
			Note:         opname.Note,
			Metadata: gin.H{
				"item_count":              len(input.Items),
				"accounting_sync_status":  opname.AccountingSyncStatus,
				"accounting_idempotency":  opname.AccountingIdempotency,
			},
			After: gin.H{
				"opname": opname,
				"items":  input.Items,
			},
		}); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		if isInventoryValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !hasJournal {
		c.JSON(http.StatusOK, gin.H{
			"message":                "Stock opname recorded successfully",
			"data":                   opname,
			"accounting_sync_status": services.AccountingSyncStatusSynced,
		})
		return
	}

	syncResult, syncErr := services.SyncAccountingRecord(services.AccountingRecordTypeStockOpname, opname.ID)
	if syncErr != nil {
		c.JSON(http.StatusOK, gin.H{
			"message":                    "Stock opname recorded successfully",
			"data":                       opname,
			"accounting_sync_status":     services.AccountingSyncStatusFailed,
			"accounting_sync_error":      syncErr.Error(),
			"accounting_idempotency_key": services.BuildAccountingIdempotencyKey("pos_stock_opname", opname.ID),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":                    "Stock opname recorded successfully",
		"data":                       opname,
		"accounting_sync_status":     syncResult.Status,
		"accounting_idempotency_key": syncResult.IdempotencyKey,
		"accounting_journal_id":      syncResult.JournalID,
		"accounting_already_posted":  syncResult.AlreadyPosted,
	})
}

func GetStockOpnames(c *gin.Context) {
	if err := services.EnsureInventorySchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	query := database.DB.Model(&models.StockOpname{})
	if outletID := strings.TrimSpace(c.Query("outlet_id")); outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}
	if startDate := strings.TrimSpace(c.Query("start_date")); startDate != "" {
		if parsed, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("DATE(opname_date) >= ?", parsed.Format("2006-01-02"))
		}
	}
	if endDate := strings.TrimSpace(c.Query("end_date")); endDate != "" {
		if parsed, err := time.Parse("2006-01-02", endDate); err == nil {
			query = query.Where("DATE(opname_date) <= ?", parsed.Format("2006-01-02"))
		}
	}

	var opnames []models.StockOpname
	if err := query.Order("opname_date desc, id desc").Find(&opnames).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, opnames)
}

func GetStockOpnameByID(c *gin.Context) {
	if err := services.EnsureInventorySchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var opname models.StockOpname
	if err := database.DB.Preload("Items.Product").First(&opname, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Stock opname not found"})
		return
	}

	c.JSON(http.StatusOK, opname)
}
