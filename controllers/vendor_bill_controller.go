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

type VendorBillInput struct {
	OutletID       uint       `json:"outlet_id"`
	VendorName     string     `json:"vendor_name"`
	BillNo         string     `json:"bill_no"`
	BillDate       time.Time  `json:"bill_date"`
	DueDate        *time.Time `json:"due_date"`
	BillType       string     `json:"bill_type"`
	AccountPurpose string     `json:"account_purpose"`
	UsePrepaid     bool       `json:"use_prepaid"`
	Subtotal       float64    `json:"subtotal"`
	TaxAmount      float64    `json:"tax_amount"`
	TotalAmount    float64    `json:"total_amount"`
	Note           string     `json:"note"`
}

func CreateVendorBill(c *gin.Context) {
	var input VendorBillInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Log.Warnf("❌ Bind JSON failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.OutletID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outlet_id is required"})
		return
	}

	billType := services.NormalizeVendorBillType(input.BillType)
	if billType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bill_type must be expense, asset, or rent"})
		return
	}
	if billType == services.VendorBillTypeInventory {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Vendor Bill inventory dinonaktifkan. Gunakan Purchase untuk stok masuk, lalu gunakan Vendor Bill hanya untuk expense, asset, atau rent.",
		})
		return
	}

	totalAmount := input.TotalAmount
	if totalAmount <= 0 {
		totalAmount = input.Subtotal + input.TaxAmount
	}
	if totalAmount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "total_amount must be greater than zero"})
		return
	}

	if err := services.EnsureAccountingSyncSchema(database.DB); err != nil {
		utils.Log.Errorf("❌ Failed to ensure finance schema: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare finance schema"})
		return
	}

	if billNo := strings.TrimSpace(input.BillNo); billNo != "" {
		conflictQuery := database.DB.
			Model(&models.Purchase{}).
			Where("outlet_id = ? AND invoice_number = ?", input.OutletID, billNo)

		if vendorName := strings.TrimSpace(input.VendorName); vendorName != "" {
			conflictQuery = conflictQuery.Where("LOWER(COALESCE(supplier_name, '')) = ?", strings.ToLower(vendorName))
		}

		var existingPurchase models.Purchase
		if err := conflictQuery.First(&existingPurchase).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{
				"error": fmt.Sprintf("Bill no %s sudah tercatat sebagai Purchase. Jangan buat Vendor Bill untuk invoice stok yang sama.", billNo),
			})
			return
		}
	}

	var (
		bill    models.VendorBill
		journal models.JournalEntry
	)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		debitAccount, err := services.ResolveVendorBillPostingAccount(tx, input.OutletID, billType, input.AccountPurpose, input.UsePrepaid)
		if err != nil {
			return err
		}
		payableAccount, err := services.ResolveVendorPayableAccount(tx, input.OutletID)
		if err != nil {
			return err
		}

		billDate := input.BillDate
		if billDate.IsZero() {
			billDate = time.Now()
		}

		description := fmt.Sprintf("Vendor bill - %s", billType)
		if vendorName := strings.TrimSpace(input.VendorName); vendorName != "" {
			description = fmt.Sprintf("Vendor bill - %s (%s)", vendorName, billType)
		}

		journal = models.JournalEntry{
			OutletID:    input.OutletID,
			Reference:   "Vendor Bill",
			Description: description,
			EntryDate:   billDate,
			JournalLines: []models.JournalLine{
				{
					AccountID:   debitAccount.ID,
					Debit:       totalAmount,
					Credit:      0,
					Description: "Pengakuan vendor bill",
				},
				{
					AccountID:   payableAccount.ID,
					Debit:       0,
					Credit:      totalAmount,
					Description: "Hutang usaha",
				},
			},
		}
		if err := tx.Create(&journal).Error; err != nil {
			return err
		}

		bill = models.VendorBill{
			OutletID:              input.OutletID,
			VendorName:            strings.TrimSpace(input.VendorName),
			BillNo:                strings.TrimSpace(input.BillNo),
			BillDate:              billDate,
			DueDate:               input.DueDate,
			BillType:              billType,
			AccountPurpose:        strings.TrimSpace(input.AccountPurpose),
			UsePrepaid:            input.UsePrepaid,
			Subtotal:              input.Subtotal,
			TaxAmount:             input.TaxAmount,
			TotalAmount:           totalAmount,
			PaidAmount:            0,
			OutstandingAmount:     totalAmount,
			Status:                services.VendorBillStatusOpen,
			JournalEntryID:        &journal.ID,
			AccountingSyncStatus:  services.AccountingSyncStatusPending,
			AccountingIdempotency: services.BuildAccountingIdempotencyKey("pos_vendor_bill", 0),
			Note:                  strings.TrimSpace(input.Note),
		}
		if err := tx.Create(&bill).Error; err != nil {
			return err
		}

		bill.AccountingIdempotency = services.BuildAccountingIdempotencyKey("pos_vendor_bill", bill.ID)
		if err := tx.Model(&bill).Updates(map[string]interface{}{
			"accounting_sync_status":     services.AccountingSyncStatusPending,
			"accounting_idempotency_key": bill.AccountingIdempotency,
		}).Error; err != nil {
			return err
		}

		return recordDocumentAudit(tx, c, documentAuditInput{
			OutletID:     bill.OutletID,
			DocumentType: "vendor_bill",
			DocumentID:   bill.ID,
			Action:       documentAuditActionCreated,
			Summary:      fmt.Sprintf("Vendor bill #%d dibuat untuk %s", bill.ID, bill.VendorName),
			Note:         bill.Note,
			Metadata: gin.H{
				"bill_no":         bill.BillNo,
				"bill_type":       bill.BillType,
				"account_purpose": bill.AccountPurpose,
				"use_prepaid":     bill.UsePrepaid,
			},
			After: bill,
		})
	})
	if err != nil {
		utils.Log.Errorf("❌ Failed to create vendor bill: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	syncResult, syncErr := services.SyncAccountingRecord(services.AccountingRecordTypeVendorBill, bill.ID)
	if syncErr != nil {
		utils.Log.Warnf("vendor bill %d synced locally but failed to post accounting journal: %v", bill.ID, syncErr)
		c.JSON(http.StatusOK, gin.H{
			"message":                    "Vendor bill recorded successfully",
			"data":                       bill,
			"accounting_sync_status":     services.AccountingSyncStatusFailed,
			"accounting_sync_error":      syncErr.Error(),
			"accounting_idempotency_key": services.BuildAccountingIdempotencyKey("pos_vendor_bill", bill.ID),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":                    "Vendor bill recorded successfully",
		"data":                       bill,
		"accounting_sync_status":     syncResult.Status,
		"accounting_idempotency_key": syncResult.IdempotencyKey,
		"accounting_journal_id":      syncResult.JournalID,
		"accounting_already_posted":  syncResult.AlreadyPosted,
	})
}

func GetVendorBills(c *gin.Context) {
	if err := services.EnsurePOSFinanceDocumentSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	query := database.DB.Model(&models.VendorBill{})

	if outletID := strings.TrimSpace(c.Query("outlet_id")); outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}
	if status := strings.ToLower(strings.TrimSpace(c.Query("status"))); status != "" {
		query = query.Where("LOWER(COALESCE(status, '')) = ?", status)
	}
	if billType := strings.ToLower(strings.TrimSpace(c.Query("bill_type"))); billType != "" {
		query = query.Where("LOWER(COALESCE(bill_type, '')) = ?", billType)
	}
	if startDate := strings.TrimSpace(c.Query("start_date")); startDate != "" {
		if parsed, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("DATE(bill_date) >= ?", parsed.Format("2006-01-02"))
		}
	}
	if endDate := strings.TrimSpace(c.Query("end_date")); endDate != "" {
		if parsed, err := time.Parse("2006-01-02", endDate); err == nil {
			query = query.Where("DATE(bill_date) <= ?", parsed.Format("2006-01-02"))
		}
	}
	if search := strings.TrimSpace(c.Query("q")); search != "" {
		like := "%" + search + "%"
		query = query.Where("bill_no ILIKE ? OR vendor_name ILIKE ? OR note ILIKE ?", like, like, like)
	}

	var bills []models.VendorBill
	if err := query.Order("bill_date desc, id desc").Find(&bills).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, bills)
}

func GetVendorBillByID(c *gin.Context) {
	if err := services.EnsurePOSFinanceDocumentSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var bill models.VendorBill
	if err := database.DB.First(&bill, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Vendor bill not found"})
		return
	}

	c.JSON(http.StatusOK, bill)
}
