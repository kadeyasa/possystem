package controllers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/services"
)

func GetInventoryLedger(c *gin.Context) {
	if err := services.EnsureInventorySchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	limit, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "100")))
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	query := database.DB.Model(&models.InventoryLedger{}).Preload("Product")

	if outletID := strings.TrimSpace(c.Query("outlet_id")); outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}
	if productID := strings.TrimSpace(c.Query("product_id")); productID != "" {
		query = query.Where("product_id = ?", productID)
	}
	if movementType := strings.ToLower(strings.TrimSpace(c.Query("movement_type"))); movementType != "" {
		query = query.Where("LOWER(COALESCE(movement_type, '')) = ?", movementType)
	}
	if referenceType := strings.ToLower(strings.TrimSpace(c.Query("reference_type"))); referenceType != "" {
		query = query.Where("LOWER(COALESCE(reference_type, '')) = ?", referenceType)
	}
	if startDate := strings.TrimSpace(c.Query("start_date")); startDate != "" {
		if parsed, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("DATE(created_at) >= ?", parsed.Format("2006-01-02"))
		}
	}
	if endDate := strings.TrimSpace(c.Query("end_date")); endDate != "" {
		if parsed, err := time.Parse("2006-01-02", endDate); err == nil {
			query = query.Where("DATE(created_at) <= ?", parsed.Format("2006-01-02"))
		}
	}

	var ledger []models.InventoryLedger
	if err := query.Order("created_at desc, id desc").Limit(limit).Find(&ledger).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ledger)
}
