package controllers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
)

// controllers/journal_report.go
func GetJournalReport(c *gin.Context) {
	var journals []models.JournalEntry

	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	outletID := c.Query("outlet_id")

	query := database.DB.Preload("JournalLines.Account").Model(&models.JournalEntry{})

	if startDate != "" && endDate != "" {
		sd, _ := time.Parse("2006-01-02", startDate)
		ed, _ := time.Parse("2006-01-02", endDate)
		query = query.Where("entry_date BETWEEN ? AND ?", sd, ed)
	}

	if outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}

	if err := query.Order("entry_date asc").Find(&journals).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil jurnal akuntansi"})
		return
	}

	c.JSON(http.StatusOK, journals)
}

func GetBalanceSheet(c *gin.Context) {
	start := c.Query("start_date")
	end := c.Query("end_date")
	outletID := c.Query("outlet_id")

	var startDate, endDate time.Time
	var err error

	startDate, err = time.Parse("2006-01-02", start)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date format"})
		return
	}

	endDate, err = time.Parse("2006-01-02", end)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format"})
		return
	}

	// Join journal_lines -> journal_entries & accounts
	type Result struct {
		AccountID string
		Name      string
		Category  string
		Debit     float64
		Credit    float64
	}

	var results []Result
	err = database.DB.Table("tbljournal_lines").
		Select("tblaccounts.id as account_id, tblaccounts.name, tblaccounts.category, SUM(tbljournal_lines.debit) as debit, SUM(tbljournal_lines.credit) as credit").
		Joins("JOIN tbljournal_entries ON tbljournal_lines.journal_entry_id = tbljournal_entries.id").
		Joins("JOIN tblaccounts ON tbljournal_lines.account_id = tblaccounts.id").
		Where("tbljournal_entries.entry_date BETWEEN ? AND ?", startDate, endDate).
		Where("tbljournal_entries.outlet_id = ?", outletID).
		Group("tblaccounts.id, tblaccounts.name, tblaccounts.category").
		Scan(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghitung neraca"})
		return
	}

	// Klasifikasi
	assets := []gin.H{}
	liabilities := []gin.H{}
	equity := []gin.H{}
	var totalAssets, totalLiabEquity float64

	for _, r := range results {
		balance := r.Debit - r.Credit
		if r.Category == "Asset" {
			assets = append(assets, gin.H{"account_id": r.AccountID, "name": r.Name, "balance": balance})
			totalAssets += balance
		} else if r.Category == "Liability" {
			liabilities = append(liabilities, gin.H{"account_id": r.AccountID, "name": r.Name, "balance": -balance})
			totalLiabEquity += -balance
		} else if r.Category == "Equity" {
			equity = append(equity, gin.H{"account_id": r.AccountID, "name": r.Name, "balance": -balance})
			totalLiabEquity += -balance
		}
	}

	// Response
	c.JSON(http.StatusOK, gin.H{
		"periode":                  fmt.Sprintf("%s to %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")),
		"outlet_id":                outletID,
		"assets":                   assets,
		"liabilities":              liabilities,
		"equity":                   equity,
		"total_assets":             totalAssets,
		"total_liabilities_equity": totalLiabEquity,
	})
}
