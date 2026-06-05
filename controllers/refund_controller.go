package controllers

import (
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

type RefundInput struct {
	TransactionID    uint                `json:"transaction_id"`
	OutletID         uint                `json:"outlet_id"`
	CashierID        uint                `json:"cashier_id"`
	CashierName      string              `json:"cashier_name"`
	CustomerID       *uint               `json:"customer_id"`
	CustomerName     string              `json:"customer_name"`
	SettlementType   string              `json:"settlement_type"`
	SettlementMethod string              `json:"settlement_method"`
	Note             string              `json:"note"`
	Items            []models.RefundItem `json:"items"`
}

func CreateRefund(c *gin.Context) {
	var input RefundInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
	if err := services.EnsureRefundSettlementSchema(database.DB); err != nil {
		utils.Log.Errorf("❌ Failed to ensure refund settlement schema: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare refund settlement schema"})
		return
	}

	var (
		refund models.Refund
	)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		executedRefund, _, execErr := executeRefundDocument(tx, refundExecutionInput{
			TransactionID:        input.TransactionID,
			OutletID:             input.OutletID,
			CashierID:            input.CashierID,
			CashierName:          input.CashierName,
			CustomerID:           input.CustomerID,
			CustomerName:         input.CustomerName,
			SettlementType:       input.SettlementType,
			SettlementMethod:     input.SettlementMethod,
			Note:                 input.Note,
			Items:                input.Items,
			AccountingSyncStatus: services.AccountingSyncStatusPending,
		})
		if execErr != nil {
			return execErr
		}
		refund = executedRefund
		return recordDocumentAudit(tx, c, documentAuditInput{
			OutletID:     refund.OutletID,
			DocumentType: "refund",
			DocumentID:   refund.ID,
			Action:       documentAuditActionCreated,
			Summary:      "Refund dibuat langsung dari transaksi POS",
			Note:         refund.Note,
			Metadata: gin.H{
				"transaction_id":    refund.TransactionID,
				"cashier_id":        refund.CashierID,
				"cashier_name":      refund.CashierName,
				"refund_total":      refund.RefundTotal,
				"settlement_type":   refund.SettlementType,
				"settlement_method": refund.SettlementMethod,
				"store_credit_code": refund.StoreCreditCode,
			},
			After: gin.H{
				"refund": refund,
				"items":  input.Items,
			},
		})
	})

	if err != nil {
		if isInventoryValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	syncResult, syncErr := services.SyncAccountingRecord(services.AccountingRecordTypeRefund, refund.ID)
	if syncErr != nil {
		utils.Log.Warnf("refund %d synced locally but failed to post accounting journal: %v", refund.ID, syncErr)
		c.JSON(http.StatusOK, gin.H{
			"refund_id":                  refund.ID,
			"refund_number":              refund.RefundNumber,
			"message":                    "Refund berhasil diproses dan jurnal dicatat",
			"accounting_sync_status":     services.AccountingSyncStatusFailed,
			"accounting_sync_error":      syncErr.Error(),
			"accounting_idempotency_key": services.BuildAccountingIdempotencyKey("pos_refund", refund.ID),
			"settlement_type":            refund.SettlementType,
			"settlement_method":          refund.SettlementMethod,
			"store_credit_code":          refund.StoreCreditCode,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"refund_id":                  refund.ID,
		"refund_number":              refund.RefundNumber,
		"message":                    "Refund berhasil diproses dan jurnal dicatat",
		"accounting_sync_status":     syncResult.Status,
		"accounting_idempotency_key": syncResult.IdempotencyKey,
		"accounting_journal_id":      syncResult.JournalID,
		"accounting_already_posted":  syncResult.AlreadyPosted,
		"settlement_type":            refund.SettlementType,
		"settlement_method":          refund.SettlementMethod,
		"store_credit_code":          refund.StoreCreditCode,
	})
}

func GetRefundsByDay(c *gin.Context) {
	date := c.Query("date")
	var refunds []models.Refund
	database.DB.Preload("Items").
		Where("DATE(created_at) = ?", date).
		Find(&refunds)
	c.JSON(http.StatusOK, refunds)
}

func GetRefundsByMonth(c *gin.Context) {
	month := c.Query("month")
	year := c.Query("year")
	var refunds []models.Refund
	database.DB.Preload("Items").
		Where("EXTRACT(MONTH FROM created_at) = ? AND EXTRACT(YEAR FROM created_at) = ?", month, year).
		Find(&refunds)
	c.JSON(http.StatusOK, refunds)
}

func GetRefundDetail(c *gin.Context) {
	id := c.Param("id")
	var refund models.Refund
	if err := database.DB.Preload("Items").First(&refund, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Refund not found"})
		return
	}
	c.JSON(http.StatusOK, refund)
}

func GetRefundsByBarcode(c *gin.Context) {
	barcode := c.Query("barcode")
	var refunds []models.Refund
	database.DB.
		Joins("JOIN tblrefund_items ON tblrefund_items.refund_id = tblrefunds.id").
		Joins("JOIN tblproducts ON tblrefund_items.product_id = tblproducts.id").
		Where("tblproducts.barcode = ?", barcode).
		Preload("Items").
		Find(&refunds)
	c.JSON(http.StatusOK, refunds)
}

func GetRefundReport(c *gin.Context) {
	var refunds []models.Refund

	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	outletID := c.Query("outlet_id")
	cashierID := c.Query("cashier_id")
	barcode := c.Query("barcode")

	query := database.DB.Preload("Items.Product").Model(&models.Refund{})

	if startDate != "" && endDate != "" {
		sd, _ := time.Parse("2006-01-02", startDate)
		ed, _ := time.Parse("2006-01-02", endDate)
		query = query.Where("created_at BETWEEN ? AND ?", sd, ed)
	}

	if outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}

	if cashierID != "" {
		query = query.Where("cashier_id = ?", cashierID)
	}

	if barcode != "" {
		// Join dengan produk
		query = query.Joins("JOIN tblrefund_items ON tblrefund_items.refund_id = tblrefunds.id").
			Joins("JOIN tblproducts ON tblproducts.id = tblrefund_items.product_id").
			Where("tblproducts.barcode = ?", barcode)
	}

	if err := query.Order("created_at desc").Find(&refunds).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data refund"})
		return
	}

	c.JSON(http.StatusOK, refunds)
}

func GetRefundStoreCredits(c *gin.Context) {
	if err := services.EnsureRefundSettlementSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	outletID := c.Query("outlet_id")
	if outletID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outlet_id is required"})
		return
	}

	query := database.DB.Model(&models.RefundStoreCredit{}).Where("outlet_id = ?", outletID)
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	} else {
		query = query.Where("status = ?", services.RefundStoreCreditStatusActive)
	}
	if customerID := strings.TrimSpace(c.Query("customer_id")); customerID != "" {
		query = query.Where("customer_id = ?", customerID)
	}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		like := "%" + search + "%"
		query = query.Where(
			"credit_code ILIKE ? OR COALESCE(customer_name, '') ILIKE ? OR CAST(transaction_id AS TEXT) ILIKE ? OR CAST(refund_id AS TEXT) ILIKE ?",
			like, like, like, like,
		)
	}

	var credits []models.RefundStoreCredit
	if err := query.Order("issued_at DESC, id DESC").Find(&credits).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil saldo retur"})
		return
	}

	c.JSON(http.StatusOK, credits)
}

func LookupRefundStoreCredit(c *gin.Context) {
	if err := services.EnsureRefundSettlementSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	outletID := strings.TrimSpace(c.Query("outlet_id"))
	code := strings.TrimSpace(c.Query("code"))
	if outletID == "" || code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outlet_id and code are required"})
		return
	}

	var credit models.RefundStoreCredit
	if err := database.DB.
		Where("outlet_id = ? AND credit_code = ?", outletID, code).
		First(&credit).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Saldo retur tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, credit)
}
