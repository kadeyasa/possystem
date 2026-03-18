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
	"gorm.io/gorm/clause"
)

type VendorPaymentAllocationInput struct {
	VendorBillID uint    `json:"vendor_bill_id"`
	Amount       float64 `json:"amount"`
}

type VendorPaymentInput struct {
	OutletID      uint                           `json:"outlet_id"`
	VendorName    string                         `json:"vendor_name"`
	PaymentNo     string                         `json:"payment_no"`
	PaymentDate   time.Time                      `json:"payment_date"`
	PaymentMethod string                         `json:"payment_method"`
	Amount        float64                        `json:"amount"`
	Note          string                         `json:"note"`
	Allocations   []VendorPaymentAllocationInput `json:"allocations"`
}

func CreateVendorPayment(c *gin.Context) {
	var input VendorPaymentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Log.Warnf("❌ Bind JSON failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.OutletID == 0 || strings.TrimSpace(input.PaymentMethod) == "" || len(input.Allocations) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outlet_id, payment_method, and allocations are required"})
		return
	}

	if err := services.EnsureAccountingSyncSchema(database.DB); err != nil {
		utils.Log.Errorf("❌ Failed to ensure finance schema: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare finance schema"})
		return
	}

	allocationTotals := make(map[uint]float64, len(input.Allocations))
	for _, allocation := range input.Allocations {
		if allocation.VendorBillID == 0 || allocation.Amount <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "every allocation must contain vendor_bill_id and amount > 0"})
			return
		}
		allocationTotals[allocation.VendorBillID] += allocation.Amount
	}

	var (
		payment models.VendorPayment
		journal models.JournalEntry
	)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		payableAccount, err := services.ResolveVendorPayableAccount(tx, input.OutletID)
		if err != nil {
			return err
		}
		settlementAccount, err := services.ResolveSettlementAccountForOutlet(tx, input.OutletID, input.PaymentMethod)
		if err != nil {
			return err
		}

		paymentDate := input.PaymentDate
		if paymentDate.IsZero() {
			paymentDate = time.Now()
		}

		totalAllocated := 0.0
		resolvedVendorName := strings.TrimSpace(input.VendorName)

		for billID, allocatedAmount := range allocationTotals {
			var bill models.VendorBill
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND outlet_id = ?", billID, input.OutletID).
				First(&bill).Error; err != nil {
				return err
			}

			if strings.EqualFold(strings.TrimSpace(bill.Status), services.VendorBillStatusVoid) {
				return fmt.Errorf("vendor bill %d is void", bill.ID)
			}
			if bill.OutstandingAmount <= 0.0001 {
				return fmt.Errorf("vendor bill %d has no outstanding amount", bill.ID)
			}
			if allocatedAmount > bill.OutstandingAmount+0.0001 {
				return fmt.Errorf("allocation exceeds outstanding amount for vendor bill %d", bill.ID)
			}

			if resolvedVendorName == "" {
				resolvedVendorName = strings.TrimSpace(bill.VendorName)
			} else if strings.TrimSpace(bill.VendorName) != "" && !strings.EqualFold(strings.TrimSpace(bill.VendorName), resolvedVendorName) {
				return fmt.Errorf("all allocations must belong to the same vendor")
			}

			nextPaidAmount := bill.PaidAmount + allocatedAmount
			nextOutstandingAmount := bill.TotalAmount - nextPaidAmount
			if nextOutstandingAmount < 0 {
				nextOutstandingAmount = 0
			}
			nextStatus := services.ComputeVendorBillStatus(bill.TotalAmount, nextPaidAmount)

			if err := tx.Model(&bill).Updates(map[string]interface{}{
				"paid_amount":        nextPaidAmount,
				"outstanding_amount": nextOutstandingAmount,
				"status":             nextStatus,
			}).Error; err != nil {
				return err
			}

			if bill.PurchaseID != nil && *bill.PurchaseID > 0 {
				purchaseStatus := services.ComputePurchasePaymentStatus(bill.TotalAmount, nextPaidAmount)
				if err := tx.Model(&models.Purchase{}).
					Where("id = ? AND outlet_id = ?", *bill.PurchaseID, input.OutletID).
					Updates(map[string]interface{}{
						"paid_amount":        nextPaidAmount,
						"outstanding_amount": nextOutstandingAmount,
						"payment_status":     purchaseStatus,
					}).Error; err != nil {
					return err
				}
			}

			totalAllocated += allocatedAmount
		}

		if input.Amount > 0 {
			diff := input.Amount - totalAllocated
			if diff > 0.0001 || diff < -0.0001 {
				return fmt.Errorf("payment amount must equal total allocations")
			}
		}

		description := "Pembayaran hutang vendor"
		if resolvedVendorName != "" {
			description = fmt.Sprintf("Pembayaran hutang vendor - %s", resolvedVendorName)
		}

		journal = models.JournalEntry{
			OutletID:    input.OutletID,
			Reference:   "Vendor Payment",
			Description: description,
			EntryDate:   paymentDate,
			JournalLines: []models.JournalLine{
				{
					AccountID:   payableAccount.ID,
					Debit:       totalAllocated,
					Credit:      0,
					Description: "Pelunasan hutang usaha",
				},
				{
					AccountID:   settlementAccount.ID,
					Debit:       0,
					Credit:      totalAllocated,
					Description: "Pembayaran vendor",
				},
			},
		}
		if err := tx.Create(&journal).Error; err != nil {
			return err
		}

		payment = models.VendorPayment{
			OutletID:              input.OutletID,
			VendorName:            resolvedVendorName,
			PaymentNo:             strings.TrimSpace(input.PaymentNo),
			PaymentDate:           paymentDate,
			PaymentMethod:         strings.TrimSpace(input.PaymentMethod),
			Amount:                totalAllocated,
			Status:                services.VendorPaymentStatusPosted,
			JournalEntryID:        &journal.ID,
			AccountingSyncStatus:  services.AccountingSyncStatusPending,
			AccountingIdempotency: services.BuildAccountingIdempotencyKey("pos_vendor_payment", 0),
			Note:                  strings.TrimSpace(input.Note),
		}
		if err := tx.Create(&payment).Error; err != nil {
			return err
		}

		for billID, allocatedAmount := range allocationTotals {
			allocation := models.VendorPaymentAllocation{
				PaymentID:       payment.ID,
				VendorBillID:    billID,
				AllocatedAmount: allocatedAmount,
			}
			if err := tx.Create(&allocation).Error; err != nil {
				return err
			}
		}

		payment.AccountingIdempotency = services.BuildAccountingIdempotencyKey("pos_vendor_payment", payment.ID)
		if err := tx.Model(&payment).Updates(map[string]interface{}{
			"accounting_sync_status":     services.AccountingSyncStatusPending,
			"accounting_idempotency_key": payment.AccountingIdempotency,
		}).Error; err != nil {
			return err
		}

		return recordDocumentAudit(tx, c, documentAuditInput{
			OutletID:     payment.OutletID,
			DocumentType: "vendor_payment",
			DocumentID:   payment.ID,
			Action:       documentAuditActionCreated,
			Summary:      fmt.Sprintf("Vendor payment #%d dicatat untuk %s", payment.ID, payment.VendorName),
			Note:         payment.Note,
			Metadata: gin.H{
				"payment_no":     payment.PaymentNo,
				"payment_method": payment.PaymentMethod,
				"allocation_count": len(input.Allocations),
			},
			After: gin.H{
				"payment":     payment,
				"allocations": input.Allocations,
			},
		})
	})
	if err != nil {
		utils.Log.Errorf("❌ Failed to create vendor payment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	syncResult, syncErr := services.SyncAccountingRecord(services.AccountingRecordTypeVendorPayment, payment.ID)
	if syncErr != nil {
		utils.Log.Warnf("vendor payment %d synced locally but failed to post accounting journal: %v", payment.ID, syncErr)
		c.JSON(http.StatusOK, gin.H{
			"message":                    "Vendor payment recorded successfully",
			"data":                       payment,
			"accounting_sync_status":     services.AccountingSyncStatusFailed,
			"accounting_sync_error":      syncErr.Error(),
			"accounting_idempotency_key": services.BuildAccountingIdempotencyKey("pos_vendor_payment", payment.ID),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":                    "Vendor payment recorded successfully",
		"data":                       payment,
		"accounting_sync_status":     syncResult.Status,
		"accounting_idempotency_key": syncResult.IdempotencyKey,
		"accounting_journal_id":      syncResult.JournalID,
		"accounting_already_posted":  syncResult.AlreadyPosted,
	})
}

func GetVendorPayments(c *gin.Context) {
	if err := services.EnsurePOSFinanceDocumentSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	query := database.DB.Model(&models.VendorPayment{})

	if outletID := strings.TrimSpace(c.Query("outlet_id")); outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}
	if status := strings.ToLower(strings.TrimSpace(c.Query("status"))); status != "" {
		query = query.Where("LOWER(COALESCE(status, '')) = ?", status)
	}
	if startDate := strings.TrimSpace(c.Query("start_date")); startDate != "" {
		if parsed, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("DATE(payment_date) >= ?", parsed.Format("2006-01-02"))
		}
	}
	if endDate := strings.TrimSpace(c.Query("end_date")); endDate != "" {
		if parsed, err := time.Parse("2006-01-02", endDate); err == nil {
			query = query.Where("DATE(payment_date) <= ?", parsed.Format("2006-01-02"))
		}
	}
	if search := strings.TrimSpace(c.Query("q")); search != "" {
		like := "%" + search + "%"
		query = query.Where("payment_no ILIKE ? OR vendor_name ILIKE ? OR note ILIKE ?", like, like, like)
	}

	var payments []models.VendorPayment
	if err := query.Preload("Allocations").Order("payment_date desc, id desc").Find(&payments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, payments)
}

func GetVendorPaymentByID(c *gin.Context) {
	if err := services.EnsurePOSFinanceDocumentSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var payment models.VendorPayment
	if err := database.DB.Preload("Allocations.Bill").First(&payment, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Vendor payment not found"})
		return
	}

	c.JSON(http.StatusOK, payment)
}
