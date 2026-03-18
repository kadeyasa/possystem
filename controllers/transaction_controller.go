package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/services"
	"github.com/kadeyasa/possystem/utils"
	"gorm.io/gorm"
)

type TransactionInput struct {
	OutletID      uint                     `json:"outlet_id"`
	CashierID     uint                     `json:"cashier_id"`
	CashierName   string                   `json:"cashier_name"`
	Total         float64                  `json:"total"`
	Tax           float64                  `json:"tax"`
	Discount      float64                  `json:"discount"`
	PaymentMethod string                   `json:"payment_method"` // "cash" or "credit"
	Note          string                   `json:"note"`
	Status        uint                     `json:"status"`
	Items         []models.TransactionItem `json:"items"`
}

func applyNonVoidedTransactionFilter(query *gorm.DB) *gorm.DB {
	return query.Where("COALESCE(document_status, ?) <> ?", services.TransactionDocumentStatusPosted, services.TransactionDocumentStatusVoided)
}

func applyTransactionDateFilters(query *gorm.DB, startDate, endDate string) *gorm.DB {
	if parsed, err := time.Parse("2006-01-02", strings.TrimSpace(startDate)); err == nil {
		query = query.Where("created_at >= CAST(? AS timestamp)", parsed.Format("2006-01-02"))
	}
	if parsed, err := time.Parse("2006-01-02", strings.TrimSpace(endDate)); err == nil {
		query = query.Where("created_at < CAST(? AS timestamp)", parsed.AddDate(0, 0, 1).Format("2006-01-02"))
	}
	return query
}

// GET: /transactions
func GetAllTransactions(c *gin.Context) {
	if err := services.EnsurePOSApprovalSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var transactions []models.Transaction
	query := database.DB.
		Preload("Items.Product").
		Preload("Items.Variant").
		Model(&models.Transaction{})

	if strings.ToLower(strings.TrimSpace(c.Query("include_voided"))) != "true" {
		query = applyNonVoidedTransactionFilter(query)
	}
	if outletID := strings.TrimSpace(c.Query("outlet_id")); outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}
	query = applyTransactionDateFilters(query, c.Query("start_date"), c.Query("end_date"))
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		like := "%" + search + "%"
		query = query.Where(
			"CAST(id AS TEXT) ILIKE ? OR COALESCE(cashier_name, '') ILIKE ? OR COALESCE(payment_method, '') ILIKE ? OR COALESCE(note, '') ILIKE ?",
			like, like, like, like,
		)
	}

	if err := query.Order("created_at desc, id desc").Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, transactions)
}

// GET: /transactions/:id
func GetTransactionByID(c *gin.Context) {
	id := c.Param("id")
	var transaction models.Transaction

	if err := database.DB.
		Preload("Items.Product").
		Preload("Items.Variant"). // <- ini penting
		First(&transaction, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	}

	c.JSON(http.StatusOK, transaction)
}

// GET: /transactions/daily?date=2025-07-01
func GetTransactionsDaily(c *gin.Context) {
	if err := services.EnsurePOSApprovalSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	dateParam := c.Query("date")
	OutletId := c.Query("outlet_id")
	date, err := time.Parse("2006-01-02", dateParam)
	if err != nil {
		utils.Log.Warnf("❌ Bind JSON failed: %v", date)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format. Use YYYY-MM-DD"})
		return
	}
	if OutletId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Outlet ID"})
		return
	}
	start := date
	end := date.AddDate(0, 0, 1)

	var transactions []models.Transaction
	if err := applyNonVoidedTransactionFilter(database.DB.Where("created_at >= ? AND created_at < ? AND outlet_id = ?", start, end, OutletId)).
		Preload("Items").
		Preload("Items.Product").
		Preload("Items.Variant").
		Order("created_at desc").
		Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, transactions)
}

// GET: /transactions/weekly?date=2025-07-01
func GetTransactionsWeekly(c *gin.Context) {
	if err := services.EnsurePOSApprovalSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	dateParam := c.Query("date")
	OutletId := c.Query("outlet_id")
	date, err := time.Parse("2006-01-02", dateParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format. Use YYYY-MM-DD"})
		return
	}
	if OutletId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Outlet ID"})
		return
	}
	// Awal minggu = Senin
	offset := int(date.Weekday())
	if offset == 0 {
		offset = 6
	} else {
		offset--
	}
	start := date.AddDate(0, 0, -offset)
	end := start.AddDate(0, 0, 7)

	var transactions []models.Transaction
	if err := applyNonVoidedTransactionFilter(database.DB.Where("created_at >= ? AND created_at < ? AND outlet_id = ?", start, end, OutletId)).
		Preload("Items").
		Preload("Items.Product").
		Preload("Items.Variant").
		Order("created_at desc").
		Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, transactions)
}

// GET: /transactions/monthly?year=2025&month=7
func GetTransactionsMonthly(c *gin.Context) {
	if err := services.EnsurePOSApprovalSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	year := c.Query("year")
	month := c.Query("month")
	OutletId := c.Query("outlet_id")
	start, err := time.Parse("2006-1-2", year+"-"+month+"-1")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid year/month format"})
		return
	}
	end := start.AddDate(0, 1, 0)
	if OutletId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Outlet ID"})
		return
	}
	var transactions []models.Transaction
	if err := applyNonVoidedTransactionFilter(database.DB.Where("created_at >= ? AND created_at < ? AND outlet_id = ?", start, end, OutletId)).
		Preload("Items").
		Preload("Items.Product").
		Preload("Items.Variant").
		Order("created_at desc").
		Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, transactions)
}

func CreateTransaction(c *gin.Context) {
	var input TransactionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Log.Warnf("❌ Bind JSON failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if strings.EqualFold(strings.TrimSpace(input.PaymentMethod), "bayarnanti") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "payment_method bayarnanti must be saved as draft first and cannot post accounting directly",
		})
		return
	}

	if err := services.EnsureAccountingSyncSchema(database.DB); err != nil {
		utils.Log.Errorf("❌ Failed to ensure accounting sync schema: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare accounting sync schema"})
		return
	}
	if err := services.EnsureInventorySchema(database.DB); err != nil {
		utils.Log.Errorf("❌ Failed to ensure inventory schema: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare inventory schema"})
		return
	}
	if err := services.EnsurePOSApprovalSchema(database.DB); err != nil {
		utils.Log.Errorf("❌ Failed to ensure POS approval schema: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare POS approval schema"})
		return
	}

	var (
		transaction models.Transaction
		journal     models.JournalEntry
	)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		cashierName := strings.TrimSpace(input.CashierName)
		if cashierName == "" && input.CashierID > 0 {
			cashierName = fmt.Sprintf("Cashier #%d", input.CashierID)
		}

		salePlan, err := services.PrepareSaleInventoryPlan(tx, input.OutletID, input.Items)
		if err != nil {
			return err
		}
		lockedProducts := salePlan.LockedProducts

		salesAccount, err := services.ResolveAccountForOutlet(tx, input.OutletID, "sale", "sales")
		if err != nil {
			return err
		}
		cashAccount, err := services.ResolveSaleSettlementAccount(tx, input.OutletID, input.PaymentMethod)
		if err != nil {
			return err
		}

		var taxAccount, discountAccount, cogsAccount, inventoryAccount models.Account
		hasTax := input.Tax > 0
		hasDiscount := input.Discount > 0
		hasCostJournal := false
		costOfGoods := 0.0

		if hasTax {
			taxAccount, err = services.ResolveAccountForOutlet(tx, input.OutletID, "sale", "tax")
			if err != nil {
				return err
			}
		}
		if hasDiscount {
			discountAccount, err = services.ResolveAccountForOutlet(tx, input.OutletID, "sale", "discount")
			if err != nil {
				return err
			}
		}

		costOfGoods = salePlan.TotalCost
		if costOfGoods > 0 {
			cogsAccount, hasCostJournal, err = services.TryResolveAccountForOutlet(tx, input.OutletID, "sale", "cogs")
			if err != nil {
				return err
			}
			if hasCostJournal {
				inventoryAccount, hasCostJournal, err = services.TryResolveAccountForOutlet(tx, input.OutletID, "purchase", "inventory")
				if err != nil {
					return err
				}
			}
		}

		outletFee, err := services.GetOutletFeeTx(tx, int64(input.OutletID))
		if err != nil {
			return fmt.Errorf("failed to get outlet fee: %w", err)
		}
		totalFee := (float64(outletFee.FeeSetting) / 100.0) * input.Total

		finalTotal := input.Total - input.Discount + input.Tax
		journal = models.JournalEntry{
			OutletID:    input.OutletID,
			Reference:   "Revenue",
			Description: "Penjualan oleh kasir",
			EntryDate:   time.Now(),
		}

		lines := []models.JournalLine{
			{
				AccountID:   cashAccount.ID,
				Debit:       finalTotal,
				Credit:      0,
				Description: "Penerimaan penjualan",
			},
			{
				AccountID:   salesAccount.ID,
				Debit:       0,
				Credit:      input.Total,
				Description: "Penjualan barang",
			},
		}
		if hasTax {
			lines = append(lines, models.JournalLine{
				AccountID:   taxAccount.ID,
				Debit:       0,
				Credit:      input.Tax,
				Description: "Pajak penjualan",
			})
		}
		if hasDiscount {
			lines = append(lines, models.JournalLine{
				AccountID:   discountAccount.ID,
				Debit:       input.Discount,
				Credit:      0,
				Description: "Diskon penjualan",
			})
		}
		if hasCostJournal {
			lines = append(lines,
				models.JournalLine{
					AccountID:   cogsAccount.ID,
					Debit:       costOfGoods,
					Credit:      0,
					Description: "Harga pokok penjualan",
				},
				models.JournalLine{
					AccountID:   inventoryAccount.ID,
					Debit:       0,
					Credit:      costOfGoods,
					Description: "Pengurangan persediaan",
				},
			)
		}
		lines, _, err = services.TryAppendOutletFeeJournalLines(tx, input.OutletID, input.Total, lines)
		if err != nil {
			return err
		}
		journal.JournalLines = lines

		if err := tx.Create(&journal).Error; err != nil {
			return err
		}

		transaction = models.Transaction{
			OutletID:              input.OutletID,
			CashierID:             input.CashierID,
			CashierName:           cashierName,
			Total:                 input.Total,
			Tax:                   input.Tax,
			Discount:              input.Discount,
			PaymentMethod:         input.PaymentMethod,
			Note:                  input.Note,
			JournalEntryID:        &journal.ID,
			AccountingSyncStatus:  services.AccountingSyncStatusPending,
			AccountingIdempotency: services.BuildAccountingIdempotencyKey("pos_sale", 0),
			CreatedAt:             time.Now(),
			UpdatedAt:             time.Now(),
			Status:                input.Status,
			DocumentStatus:        services.TransactionDocumentStatusPosted,
		}
		if err := tx.Create(&transaction).Error; err != nil {
			return err
		}
		transaction.AccountingIdempotency = services.BuildAccountingIdempotencyKey("pos_sale", transaction.ID)
		if err := tx.Model(&transaction).Updates(map[string]interface{}{
			"accounting_sync_status":     services.AccountingSyncStatusPending,
			"accounting_idempotency_key": transaction.AccountingIdempotency,
		}).Error; err != nil {
			return err
		}

		for _, item := range input.Items {
			item.TransactionID = transaction.ID
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}

		if err := services.RecordTransactionInventoryConsumptions(tx, transaction.ID, salePlan.Consumptions); err != nil {
			return err
		}

		for _, consumption := range salePlan.Consumptions {
			product := lockedProducts[consumption.InventoryProductID]
			if !services.IsStockTrackedItemType(product.ItemType) {
				continue
			}
			stockBefore := product.Stock
			stockAfter := stockBefore - consumption.QuantityConsumed
			if err := tx.Model(&models.Product{}).
				Where("id = ? AND outlet_id = ?", consumption.InventoryProductID, input.OutletID).
				Update("stock", stockAfter).Error; err != nil {
				return err
			}
			product.Stock = stockAfter

			movementType := services.InventoryMovementSale
			note := fmt.Sprintf("POS sale transaction #%d", transaction.ID)
			if consumption.ConsumptionType == services.InventoryConsumptionTypeRecipeComponent {
				movementType = services.InventoryMovementRecipeConsume
				soldProduct := lockedProducts[consumption.SoldProductID]
				note = fmt.Sprintf("Recipe consumption for sale #%d from %s", transaction.ID, soldProduct.Name)
			}

			if err := services.AppendInventoryLedger(tx, models.InventoryLedger{
				OutletID:      input.OutletID,
				ProductID:     consumption.InventoryProductID,
				VariantID:     consumption.VariantID,
				MovementType:  movementType,
				ReferenceType: services.InventoryReferenceSale,
				ReferenceID:   transaction.ID,
				QuantityIn:    0,
				QuantityOut:   consumption.QuantityConsumed,
				StockBefore:   stockBefore,
				StockAfter:    stockAfter,
				UnitCost:      consumption.UnitCost,
				TotalCost:     consumption.TotalCost,
				Notes:         note,
			}); err != nil {
				return err
			}
		}

		if err := services.CreateOrUpdateBalanceTx(tx, services.BalanceInput{
			OutletID: input.OutletID,
			Amount:   -totalFee,
			Remarks:  "Transaction fee deduction",
		}); err != nil {
			return fmt.Errorf("failed to deduct balance: %w", err)
		}

		return nil
	})

	if err != nil {
		utils.Log.Errorf("❌ Failed to create transaction: %v", err)
		if isInventoryValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	syncResult, syncErr := services.SyncAccountingRecord(services.AccountingRecordTypeSale, transaction.ID)
	if syncErr != nil {
		utils.Log.Warnf("transaction %d synced locally but failed to post accounting journal: %v", transaction.ID, syncErr)
		c.JSON(http.StatusOK, gin.H{
			"transaction_id":             transaction.ID,
			"message":                    "Transaction completed and journal recorded",
			"accounting_sync_status":     services.AccountingSyncStatusFailed,
			"accounting_sync_error":      syncErr.Error(),
			"accounting_idempotency_key": services.BuildAccountingIdempotencyKey("pos_sale", transaction.ID),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transaction_id":             transaction.ID,
		"message":                    "Transaction completed and journal recorded",
		"accounting_sync_status":     syncResult.Status,
		"accounting_idempotency_key": syncResult.IdempotencyKey,
		"accounting_journal_id":      syncResult.JournalID,
		"accounting_already_posted":  syncResult.AlreadyPosted,
	})
}

func GetSalesReport(c *gin.Context) {
	if err := services.EnsurePOSApprovalSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var sales []models.Transaction

	// Query params
	startDateStr := c.Query("start_date") // format: YYYY-MM-DD
	endDateStr := c.Query("end_date")
	cashierID := c.Query("cashier_id")
	outletID := c.Query("outlet_id")
	paymentMethod := c.Query("payment_method")

	query := database.DB.
		Preload("Items.Product").
		Preload("Items.Variant").
		Model(&models.Transaction{})
	query = applyNonVoidedTransactionFilter(query)

	if startDateStr != "" && endDateStr != "" {
		startDate, err1 := time.Parse("2006-01-02", startDateStr)
		endDate, err2 := time.Parse("2006-01-02", endDateStr)
		if err1 == nil && err2 == nil {
			query = query.Where("created_at BETWEEN ? AND ?", startDate, endDate)
		}
	}

	if cashierID != "" {
		query = query.Where("cashier_id = ?", cashierID)
	}

	if outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}

	if paymentMethod != "" {
		query = query.Where("payment_method = ?", paymentMethod)
	}

	if err := query.Order("created_at desc").Find(&sales).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data penjualan"})
		return
	}

	c.JSON(http.StatusOK, sales)
}

func GetDashboardInfo(c *gin.Context) {
	if err := services.EnsurePOSApprovalSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	outletID := c.Query("outlet")
	var totalSales, todaySales float64

	// Total sales keseluruhan
	err := database.DB.Model(&models.Transaction{}).
		Where("outlet_id=?", outletID).
		Where("COALESCE(document_status, ?) <> ?", services.TransactionDocumentStatusPosted, services.TransactionDocumentStatusVoided).
		Select("COALESCE(SUM(total), 0)").
		Scan(&totalSales).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghitung total penjualan"})
		return
	}

	// Total sales hari ini
	today := time.Now()
	startOfDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	endOfDay := startOfDay.AddDate(0, 0, 1)

	err = database.DB.Model(&models.Transaction{}).
		Where("outlet_id = ? AND created_at >= ? AND created_at < ?", outletID, startOfDay, endOfDay).
		Where("COALESCE(document_status, ?) <> ?", services.TransactionDocumentStatusPosted, services.TransactionDocumentStatusVoided).
		Select("COALESCE(SUM(total), 0)").
		Scan(&todaySales).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghitung penjualan hari ini"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total_sales": totalSales,
		"today_sales": todaySales,
	})
}
func GetRevenue(c *gin.Context) {
	outletIDStr := c.Query("outlet")

	if outletIDStr == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid Outlet ID"})
		return
	}
	outlet, err := strconv.Atoi(outletIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid outlet parameters"})
		return
	}
	var todayRevenue, yesterdayRevenue, weeklyRevenue, monthlyRevenue float64

	// 🔹 Hari ini
	if err := database.DB.Table("tbljournal_lines jl").
		Select("COALESCE(SUM(jl.credit),0)").
		Joins("JOIN tbljournal_entries je ON je.id = jl.journal_entry_id").
		Where("je.reference = ?", "Revenue").
		Where("DATE(je.created_at) = CURRENT_DATE").
		Where("je.outlet_id=?", outlet).
		Scan(&todayRevenue).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal ambil revenue harian"})
		return
	}

	// 🔹 Kemarin
	if err := database.DB.Table("tbljournal_lines jl").
		Select("COALESCE(SUM(jl.credit),0)").
		Joins("JOIN tbljournal_entries je ON je.id = jl.journal_entry_id").
		Where("je.reference = ?", "Revenue").
		Where("DATE(je.created_at) = CURRENT_DATE - INTERVAL '1 day'").
		Where("je.outlet_id=?", outlet).
		Scan(&yesterdayRevenue).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal ambil revenue kemarin"})
		return
	}

	// 🔹 Minggu ini
	if err := database.DB.Table("tbljournal_lines jl").
		Select("COALESCE(SUM(jl.credit),0)").
		Joins("JOIN tbljournal_entries je ON je.id = jl.journal_entry_id").
		Where("je.reference = ?", "Revenue").
		Where("DATE_TRUNC('week', je.created_at) = DATE_TRUNC('week', CURRENT_DATE)").
		Where("je.outlet_id=?", outlet).
		Scan(&weeklyRevenue).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal ambil revenue mingguan"})
		return
	}

	// 🔹 Bulan ini
	if err := database.DB.Table("tbljournal_lines jl").
		Select("COALESCE(SUM(jl.credit),0)").
		Joins("JOIN tbljournal_entries je ON je.id = jl.journal_entry_id").
		Where("je.reference = ?", "Revenue").
		Where("DATE_TRUNC('month', je.created_at) = DATE_TRUNC('month', CURRENT_DATE)").
		Where("je.outlet_id=?", outlet).
		Scan(&monthlyRevenue).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal ambil revenue bulanan"})
		return
	}
	//get transaction active
	var transactions []models.Transaction
	if err := database.DB.Model(&models.Transaction{}).
		Where("outlet_id = ?", outlet).
		Where("status = ?", 0).
		Where("COALESCE(document_status, ?) <> ?", services.TransactionDocumentStatusPosted, services.TransactionDocumentStatusVoided).
		Where("deleted_at IS NULL").
		Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal ambil transaksi aktif"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"today_revenue":     todayRevenue,
		"yesterday_revenue": yesterdayRevenue,
		"weekly_revenue":    weeklyRevenue,
		"monthly_revenue":   monthlyRevenue,
		"transactions":      transactions,
	})
}

func UpdateStatus(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID is required"})
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	// update status jadi 1
	if err := database.DB.Model(&models.Transaction{}).
		Where("id = ?", id).
		Update("status", 1).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Transaction status updated successfully"})
}
