package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/services"
)

type accountingSyncRetryInput struct {
	RecordType string `json:"record_type"`
	ID         uint   `json:"id"`
}

type accountingSyncRetryFailedInput struct {
	RecordType string `json:"record_type"`
	OutletID   uint   `json:"outlet_id"`
	Limit      int    `json:"limit"`
}

func ensureAccountingSyncAdminAccess(c *gin.Context) bool {
	actorType := strings.ToLower(strings.TrimSpace(c.GetString("actor_type")))
	role := strings.ToLower(strings.TrimSpace(c.GetString("role")))
	if actorType == "admin" || role == "administrator" {
		return true
	}

	c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
	return false
}

func GetAccountingSyncSummary(c *gin.Context) {
	if !ensureAccountingSyncAdminAccess(c) {
		return
	}

	outletID, _ := strconv.Atoi(strings.TrimSpace(c.Query("outlet_id")))
	summary, err := services.GetAccountingSyncSummary(uint(outletID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"error": "0",
		"data":  summary,
	})
}

func GetAccountingSyncRecords(c *gin.Context) {
	if !ensureAccountingSyncAdminAccess(c) {
		return
	}

	outletID, _ := strconv.Atoi(strings.TrimSpace(c.Query("outlet_id")))
	limit, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "100")))

	records, err := services.ListAccountingSyncRecords(services.AccountingSyncListFilter{
		OutletID:   uint(outletID),
		Status:     c.Query("status"),
		RecordType: c.Query("record_type"),
		Search:     c.Query("q"),
		Limit:      limit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"error":   "0",
		"records": records,
	})
}

func RetryAccountingSync(c *gin.Context) {
	if !ensureAccountingSyncAdminAccess(c) {
		return
	}

	var input accountingSyncRetryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := services.SyncAccountingRecord(input.RecordType, input.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "1",
			"message": err.Error(),
			"data":    result,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"error":   "0",
		"message": "Accounting sync retried successfully",
		"data":    result,
	})
}

func RetryFailedAccountingSync(c *gin.Context) {
	if !ensureAccountingSyncAdminAccess(c) {
		return
	}

	var input accountingSyncRetryFailedInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	records, err := services.ListAccountingSyncRecords(services.AccountingSyncListFilter{
		OutletID:   input.OutletID,
		Status:     services.AccountingSyncStatusFailed,
		RecordType: input.RecordType,
		Limit:      input.Limit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	results := make([]gin.H, 0, len(records))
	var succeeded, failed int
	for _, record := range records {
		result, syncErr := services.SyncAccountingRecord(record.RecordType, record.RecordID)
		if syncErr != nil {
			failed++
			results = append(results, gin.H{
				"record_type": record.RecordType,
				"id":          record.RecordID,
				"status":      services.AccountingSyncStatusFailed,
				"error":       syncErr.Error(),
			})
			continue
		}

		succeeded++
		results = append(results, gin.H{
			"record_type":    record.RecordType,
			"id":             record.RecordID,
			"status":         result.Status,
			"journal_id":     result.JournalID,
			"already_posted": result.AlreadyPosted,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"error":   "0",
		"message": "Bulk retry completed",
		"data": gin.H{
			"total":     len(records),
			"succeeded": succeeded,
			"failed":    failed,
			"results":   results,
		},
	})
}
