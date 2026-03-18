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

type OperationalExpenseInput struct {
	OutletID        uint      `json:"outlet_id"`
	ExpenseDate     time.Time `json:"expense_date"`
	ExpenseCategory string    `json:"expense_category"`
	AccountPurpose  string    `json:"account_purpose"`
	ReferenceNo     string    `json:"reference_no"`
	PaymentMethod   string    `json:"payment_method"`
	Amount          float64   `json:"amount"`
	VendorName      string    `json:"vendor_name"`
	Note            string    `json:"note"`
}

func CreateOperationalExpense(c *gin.Context) {
	var input OperationalExpenseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Log.Warnf("❌ Bind JSON failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.OutletID == 0 || strings.TrimSpace(input.ExpenseCategory) == "" || strings.TrimSpace(input.PaymentMethod) == "" || input.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outlet_id, expense_category, payment_method, and amount are required"})
		return
	}

	if err := services.EnsureAccountingSyncSchema(database.DB); err != nil {
		utils.Log.Errorf("❌ Failed to ensure finance schema: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare finance schema"})
		return
	}

	var (
		expense models.OperationalExpense
		journal models.JournalEntry
	)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		accountPurpose := strings.TrimSpace(input.AccountPurpose)
		if accountPurpose == "" {
			accountPurpose = strings.TrimSpace(input.ExpenseCategory)
		}

		expenseAccount, err := services.ResolveOperationalExpenseAccount(tx, input.OutletID, accountPurpose)
		if err != nil {
			return err
		}
		settlementAccount, err := services.ResolveSettlementAccountForOutlet(tx, input.OutletID, input.PaymentMethod)
		if err != nil {
			return err
		}

		expenseDate := input.ExpenseDate
		if expenseDate.IsZero() {
			expenseDate = time.Now()
		}

		description := "Biaya operasional"
		if vendorName := strings.TrimSpace(input.VendorName); vendorName != "" {
			description = fmt.Sprintf("Biaya operasional - %s", vendorName)
		}

		journal = models.JournalEntry{
			OutletID:    input.OutletID,
			Reference:   "Operational Expense",
			Description: description,
			EntryDate:   expenseDate,
			JournalLines: []models.JournalLine{
				{
					AccountID:   expenseAccount.ID,
					Debit:       input.Amount,
					Credit:      0,
					Description: "Beban operasional",
				},
				{
					AccountID:   settlementAccount.ID,
					Debit:       0,
					Credit:      input.Amount,
					Description: "Pembayaran operasional",
				},
			},
		}
		if err := tx.Create(&journal).Error; err != nil {
			return err
		}

		expense = models.OperationalExpense{
			OutletID:              input.OutletID,
			ExpenseDate:           expenseDate,
			ExpenseCategory:       strings.TrimSpace(input.ExpenseCategory),
			AccountPurpose:        accountPurpose,
			ReferenceNo:           strings.TrimSpace(input.ReferenceNo),
			PaymentMethod:         strings.TrimSpace(input.PaymentMethod),
			Amount:                input.Amount,
			VendorName:            strings.TrimSpace(input.VendorName),
			Status:                services.OperationalExpenseStatusPosted,
			JournalEntryID:        &journal.ID,
			AccountingSyncStatus:  services.AccountingSyncStatusPending,
			AccountingIdempotency: services.BuildAccountingIdempotencyKey("pos_operational_expense", 0),
			Note:                  strings.TrimSpace(input.Note),
		}
		if err := tx.Create(&expense).Error; err != nil {
			return err
		}

		expense.AccountingIdempotency = services.BuildAccountingIdempotencyKey("pos_operational_expense", expense.ID)
		return tx.Model(&expense).Updates(map[string]interface{}{
			"accounting_sync_status":     services.AccountingSyncStatusPending,
			"accounting_idempotency_key": expense.AccountingIdempotency,
		}).Error
	})
	if err != nil {
		utils.Log.Errorf("❌ Failed to create operational expense: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	syncResult, syncErr := services.SyncAccountingRecord(services.AccountingRecordTypeOperationalExpense, expense.ID)
	if syncErr != nil {
		utils.Log.Warnf("operational expense %d synced locally but failed to post accounting journal: %v", expense.ID, syncErr)
		c.JSON(http.StatusOK, gin.H{
			"message":                    "Operational expense recorded successfully",
			"data":                       expense,
			"accounting_sync_status":     services.AccountingSyncStatusFailed,
			"accounting_sync_error":      syncErr.Error(),
			"accounting_idempotency_key": services.BuildAccountingIdempotencyKey("pos_operational_expense", expense.ID),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":                    "Operational expense recorded successfully",
		"data":                       expense,
		"accounting_sync_status":     syncResult.Status,
		"accounting_idempotency_key": syncResult.IdempotencyKey,
		"accounting_journal_id":      syncResult.JournalID,
		"accounting_already_posted":  syncResult.AlreadyPosted,
	})
}

func GetOperationalExpenses(c *gin.Context) {
	if err := services.EnsurePOSFinanceDocumentSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	query := database.DB.Model(&models.OperationalExpense{})

	if outletID := strings.TrimSpace(c.Query("outlet_id")); outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}
	if status := strings.ToLower(strings.TrimSpace(c.Query("status"))); status != "" {
		query = query.Where("LOWER(COALESCE(status, '')) = ?", status)
	}
	if category := strings.ToLower(strings.TrimSpace(c.Query("expense_category"))); category != "" {
		query = query.Where("LOWER(COALESCE(expense_category, '')) = ?", category)
	}
	if startDate := strings.TrimSpace(c.Query("start_date")); startDate != "" {
		if parsed, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("DATE(expense_date) >= ?", parsed.Format("2006-01-02"))
		}
	}
	if endDate := strings.TrimSpace(c.Query("end_date")); endDate != "" {
		if parsed, err := time.Parse("2006-01-02", endDate); err == nil {
			query = query.Where("DATE(expense_date) <= ?", parsed.Format("2006-01-02"))
		}
	}
	if search := strings.TrimSpace(c.Query("q")); search != "" {
		like := "%" + search + "%"
		query = query.Where("reference_no ILIKE ? OR vendor_name ILIKE ? OR note ILIKE ?", like, like, like)
	}

	var expenses []models.OperationalExpense
	if err := query.Order("expense_date desc, id desc").Find(&expenses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, expenses)
}

func GetOperationalExpenseByID(c *gin.Context) {
	if err := services.EnsurePOSFinanceDocumentSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var expense models.OperationalExpense
	if err := database.DB.First(&expense, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Operational expense not found"})
		return
	}

	c.JSON(http.StatusOK, expense)
}
