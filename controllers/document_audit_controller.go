package controllers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/services"
)

func GetDocumentAuditTrails(c *gin.Context) {
	if err := services.EnsureDocumentAuditSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	query := database.DB.Model(&models.DocumentAuditTrail{})
	if outletID := strings.TrimSpace(c.Query("outlet_id")); outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}
	if documentType := strings.ToLower(strings.TrimSpace(c.Query("document_type"))); documentType != "" {
		query = query.Where("LOWER(COALESCE(document_type, '')) = ?", documentType)
	}
	if action := strings.ToLower(strings.TrimSpace(c.Query("action"))); action != "" {
		query = query.Where("LOWER(COALESCE(action, '')) = ?", action)
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
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		like := "%" + search + "%"
		query = query.Where(
			"document_type ILIKE ? OR CAST(document_id AS TEXT) ILIKE ? OR action ILIKE ? OR COALESCE(summary, '') ILIKE ? OR COALESCE(note, '') ILIKE ? OR COALESCE(actor_name, '') ILIKE ?",
			like, like, like, like, like, like,
		)
	}

	var results []models.DocumentAuditTrail
	if err := query.Order("created_at DESC, id DESC").Limit(500).Find(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load document audit trails"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"total":   len(results),
	})
}

func GetDocumentAuditTrailByID(c *gin.Context) {
	if err := services.EnsureDocumentAuditSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var audit models.DocumentAuditTrail
	if err := database.DB.First(&audit, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "document audit trail not found"})
		return
	}

	c.JSON(http.StatusOK, audit)
}
