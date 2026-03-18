package controllers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/services"
	"gorm.io/gorm"
)

type StockAdjustmentItemInput struct {
	ProductID     uint   `json:"product_id"`
	QuantityDelta int    `json:"quantity_delta"`
	Note          string `json:"note"`
}

type StockAdjustmentInput struct {
	OutletID       uint                       `json:"outlet_id"`
	AdjustmentDate time.Time                  `json:"adjustment_date"`
	Reason         string                     `json:"reason"`
	Note           string                     `json:"note"`
	Items          []StockAdjustmentItemInput `json:"items"`
}

func CreateStockAdjustment(c *gin.Context) {
	var input StockAdjustmentInput
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
		adjustment models.StockAdjustment
		hasJournal bool
	)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var execErr error
		adjustment, hasJournal, execErr = executeStockAdjustmentDocument(tx, input)
		return execErr
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
			"message":                "Stock adjustment recorded successfully",
			"data":                   adjustment,
			"accounting_sync_status": services.AccountingSyncStatusSynced,
		})
		return
	}

	syncResult, syncErr := services.SyncAccountingRecord(services.AccountingRecordTypeStockAdjustment, adjustment.ID)
	if syncErr != nil {
		c.JSON(http.StatusOK, gin.H{
			"message":                    "Stock adjustment recorded successfully",
			"data":                       adjustment,
			"accounting_sync_status":     services.AccountingSyncStatusFailed,
			"accounting_sync_error":      syncErr.Error(),
			"accounting_idempotency_key": services.BuildAccountingIdempotencyKey("pos_stock_adjustment", adjustment.ID),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":                    "Stock adjustment recorded successfully",
		"data":                       adjustment,
		"accounting_sync_status":     syncResult.Status,
		"accounting_idempotency_key": syncResult.IdempotencyKey,
		"accounting_journal_id":      syncResult.JournalID,
		"accounting_already_posted":  syncResult.AlreadyPosted,
	})
}

func GetStockAdjustments(c *gin.Context) {
	if err := services.EnsureInventorySchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	query := database.DB.Model(&models.StockAdjustment{})
	if outletID := strings.TrimSpace(c.Query("outlet_id")); outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}
	if startDate := strings.TrimSpace(c.Query("start_date")); startDate != "" {
		if parsed, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("DATE(adjustment_date) >= ?", parsed.Format("2006-01-02"))
		}
	}
	if endDate := strings.TrimSpace(c.Query("end_date")); endDate != "" {
		if parsed, err := time.Parse("2006-01-02", endDate); err == nil {
			query = query.Where("DATE(adjustment_date) <= ?", parsed.Format("2006-01-02"))
		}
	}

	var adjustments []models.StockAdjustment
	if err := query.Order("adjustment_date desc, id desc").Find(&adjustments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, adjustments)
}

func GetStockAdjustmentByID(c *gin.Context) {
	if err := services.EnsureInventorySchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var adjustment models.StockAdjustment
	if err := database.DB.Preload("Items.Product").First(&adjustment, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Stock adjustment not found"})
		return
	}

	c.JSON(http.StatusOK, adjustment)
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
