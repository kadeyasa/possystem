package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/services"
	"gorm.io/gorm"
)

type StaffShiftClosingCreateRequest struct {
	OutletID         uint    `json:"outlet_id"`
	CashierID        uint    `json:"cashier_id"`
	CashierName      string  `json:"cashier_name"`
	ActualCashAmount float64 `json:"actual_cash_amount"`
	Note             string  `json:"note"`
}

type staffShiftClosingAggregate struct {
	TransactionCount        int64
	GrossSalesAmount        float64
	DiscountAmount          float64
	ServiceAmount           float64
	TaxAmount               float64
	NetSalesAmount          float64
	CashTransactionCount    int64
	CashSalesAmount         float64
	NonCashTransactionCount int64
	NonCashSalesAmount      float64
}

func GetCurrentStaffShiftClosingPreview(c *gin.Context) {
	if err := services.EnsurePOSApprovalSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := services.EnsureRefundSettlementSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := services.EnsureStaffShiftClosingSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	outletID, cashierID, cashierName, err := resolveShiftClosingContext(
		c,
		parseUintQueryParam(c.Query("outlet_id")),
		parseUintQueryParam(c.Query("cashier_id")),
		strings.TrimSpace(c.Query("cashier_name")),
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	preview, err := buildStaffShiftClosingPreview(database.DB, outletID, cashierID, cashierName, time.Now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": preview})
}

func GetStaffShiftClosings(c *gin.Context) {
	if err := services.EnsureStaffShiftClosingSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	outletID := parseUintQueryParam(c.Query("outlet_id"))
	cashierID := parseUintQueryParam(c.Query("cashier_id"))
	limit := parsePositiveIntQueryParam(c.Query("limit"), 8, 50)
	query := database.DB.Model(&models.StaffShiftClosing{})

	if outletID > 0 {
		query = query.Where("outlet_id = ?", outletID)
	}

	if isManagementActor(c) {
		if cashierID > 0 {
			query = query.Where("cashier_id = ?", cashierID)
		}
	} else {
		contextCashierID := readUintContextValue(c, "user_id")
		if contextCashierID > 0 {
			query = query.Where("cashier_id = ?", contextCashierID)
		} else if cashierID > 0 {
			query = query.Where("cashier_id = ?", cashierID)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	results := make([]models.StaffShiftClosing, 0)
	if err := query.
		Order("closed_at DESC, id DESC").
		Limit(limit).
		Find(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total":   total,
		"results": results,
	})
}

func CreateStaffShiftClosing(c *gin.Context) {
	if err := services.EnsurePOSApprovalSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := services.EnsureRefundSettlementSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := services.EnsureStaffShiftClosingSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var req StaffShiftClosingCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ActualCashAmount < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "actual_cash_amount tidak boleh negatif"})
		return
	}

	outletID, cashierID, cashierName, err := resolveShiftClosingContext(c, req.OutletID, req.CashierID, req.CashierName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var (
		created models.StaffShiftClosing
		preview *models.StaffShiftClosingPreview
	)

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		currentPreview, previewErr := buildStaffShiftClosingPreview(tx, outletID, cashierID, cashierName, time.Now())
		if previewErr != nil {
			return previewErr
		}
		preview = currentPreview

		created = models.StaffShiftClosing{
			OutletID:                outletID,
			CashierID:               cashierID,
			CashierName:             currentPreview.CashierName,
			PeriodStart:             currentPreview.PeriodStart,
			PeriodEnd:               currentPreview.PeriodEnd,
			ClosedAt:                currentPreview.PeriodEnd,
			TransactionCount:        currentPreview.TransactionCount,
			GrossSalesAmount:        currentPreview.GrossSalesAmount,
			DiscountAmount:          currentPreview.DiscountAmount,
			ServiceAmount:           currentPreview.ServiceAmount,
			TaxAmount:               currentPreview.TaxAmount,
			NetSalesAmount:          currentPreview.NetSalesAmount,
			CashTransactionCount:    currentPreview.CashTransactionCount,
			CashSalesAmount:         currentPreview.CashSalesAmount,
			NonCashTransactionCount: currentPreview.NonCashTransactionCount,
			NonCashSalesAmount:      currentPreview.NonCashSalesAmount,
			ExpectedCashAmount:      currentPreview.ExpectedCashAmount,
			ActualCashAmount:        req.ActualCashAmount,
			CashDifferenceAmount:    req.ActualCashAmount - currentPreview.ExpectedCashAmount,
			Note:                    strings.TrimSpace(req.Note),
		}

		return tx.Create(&created).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Shift closing berhasil disimpan",
		"data":    created,
		"preview": preview,
	})
}

func buildStaffShiftClosingPreview(db *gorm.DB, outletID, cashierID uint, cashierName string, previewEnd time.Time) (*models.StaffShiftClosingPreview, error) {
	if db == nil {
		return nil, errors.New("database is not initialized")
	}

	startOfDay := time.Date(previewEnd.Year(), previewEnd.Month(), previewEnd.Day(), 0, 0, 0, 0, previewEnd.Location())
	periodStart := startOfDay
	var lastClosedAt *time.Time

	var lastClosing models.StaffShiftClosing
	lastClosingResult := db.
		Where("outlet_id = ? AND cashier_id = ?", outletID, cashierID).
		Order("period_end DESC, id DESC").
		Limit(1).
		Find(&lastClosing)
	if lastClosingResult.Error != nil {
		return nil, lastClosingResult.Error
	}
	if lastClosingResult.RowsAffected > 0 {
		periodStart = lastClosing.PeriodEnd
		closedAt := lastClosing.ClosedAt
		lastClosedAt = &closedAt
		if strings.TrimSpace(cashierName) == "" {
			cashierName = lastClosing.CashierName
		}
	}

	if strings.TrimSpace(cashierName) == "" && cashierID > 0 {
		cashierName = fmt.Sprintf("Kasir #%d", cashierID)
	}

	aggregate := staffShiftClosingAggregate{}
	netSalesExpr := `COALESCE(NULLIF(grand_total, 0), COALESCE(NULLIF(subtotal, 0), total) - COALESCE(discount, 0) + COALESCE(service, 0) + COALESCE(tax, 0))`
	settlementAmountExpr := fmt.Sprintf(`COALESCE(settlement_amount, %s)`, netSalesExpr)
	query := fmt.Sprintf(`
		SELECT
			COUNT(*)::bigint AS transaction_count,
			COALESCE(SUM(COALESCE(NULLIF(subtotal, 0), total)), 0)::double precision AS gross_sales_amount,
			COALESCE(SUM(COALESCE(discount, 0)), 0)::double precision AS discount_amount,
			COALESCE(SUM(COALESCE(service, 0)), 0)::double precision AS service_amount,
			COALESCE(SUM(COALESCE(tax, 0)), 0)::double precision AS tax_amount,
			COALESCE(SUM(%s), 0)::double precision AS net_sales_amount,
			COUNT(*) FILTER (
				WHERE (
					LOWER(COALESCE(payment_method, '')) LIKE '%%cash%%'
					OR LOWER(COALESCE(payment_method, '')) LIKE '%%tunai%%'
				)
				  AND %s > 0
			)::bigint AS cash_transaction_count,
			COALESCE(SUM(
				CASE
					WHEN (
						LOWER(COALESCE(payment_method, '')) LIKE '%%cash%%'
						OR LOWER(COALESCE(payment_method, '')) LIKE '%%tunai%%'
					)
					  AND %s > 0
					THEN %s
					ELSE 0
				END
			), 0)::double precision AS cash_sales_amount,
			COUNT(*) FILTER (
				WHERE NOT (
					LOWER(COALESCE(payment_method, '')) LIKE '%%cash%%'
					OR LOWER(COALESCE(payment_method, '')) LIKE '%%tunai%%'
				)
				  AND %s > 0
			)::bigint AS non_cash_transaction_count,
			COALESCE(SUM(
				CASE
					WHEN NOT (
						LOWER(COALESCE(payment_method, '')) LIKE '%%cash%%'
						OR LOWER(COALESCE(payment_method, '')) LIKE '%%tunai%%'
					)
					  AND %s > 0
					THEN %s
					ELSE 0
				END
			), 0)::double precision AS non_cash_sales_amount
		FROM tbltransactions
		WHERE outlet_id = ?
		  AND cashier_id = ?
		  AND created_at >= ?
		  AND created_at <= ?
		  AND COALESCE(document_status, ?) <> ?
	`, netSalesExpr, settlementAmountExpr, settlementAmountExpr, settlementAmountExpr, settlementAmountExpr, settlementAmountExpr, settlementAmountExpr)

	if err := db.Raw(
		query,
		outletID,
		cashierID,
		periodStart,
		previewEnd,
		services.TransactionDocumentStatusPosted,
		services.TransactionDocumentStatusVoided,
	).Scan(&aggregate).Error; err != nil {
		return nil, err
	}

	return &models.StaffShiftClosingPreview{
		OutletID:                outletID,
		CashierID:               cashierID,
		CashierName:             cashierName,
		PeriodStart:             periodStart,
		PeriodEnd:               previewEnd,
		LastClosedAt:            lastClosedAt,
		TransactionCount:        aggregate.TransactionCount,
		GrossSalesAmount:        aggregate.GrossSalesAmount,
		DiscountAmount:          aggregate.DiscountAmount,
		ServiceAmount:           aggregate.ServiceAmount,
		TaxAmount:               aggregate.TaxAmount,
		NetSalesAmount:          aggregate.NetSalesAmount,
		CashTransactionCount:    aggregate.CashTransactionCount,
		CashSalesAmount:         aggregate.CashSalesAmount,
		NonCashTransactionCount: aggregate.NonCashTransactionCount,
		NonCashSalesAmount:      aggregate.NonCashSalesAmount,
		ExpectedCashAmount:      aggregate.CashSalesAmount,
	}, nil
}

func resolveShiftClosingContext(c *gin.Context, requestedOutletID, requestedCashierID uint, requestedCashierName string) (uint, uint, string, error) {
	outletID := requestedOutletID
	if outletID == 0 {
		return 0, 0, "", errors.New("outlet_id wajib diisi")
	}

	cashierID := requestedCashierID
	cashierName := strings.TrimSpace(requestedCashierName)
	if !isManagementActor(c) {
		contextCashierID := readUintContextValue(c, "user_id")
		if contextCashierID > 0 {
			cashierID = contextCashierID
		}
		if cashierName == "" {
			cashierName = strings.TrimSpace(c.GetString("display_name"))
		}
	}
	if cashierID == 0 {
		return 0, 0, "", errors.New("cashier_id tidak valid")
	}
	if cashierName == "" {
		cashierName = fmt.Sprintf("Kasir #%d", cashierID)
	}

	return outletID, cashierID, cashierName, nil
}

func isManagementActor(c *gin.Context) bool {
	actorType := strings.ToLower(strings.TrimSpace(c.GetString("actor_type")))
	role := strings.ToLower(strings.TrimSpace(c.GetString("role")))
	return actorType == "admin" ||
		actorType == "administrator" ||
		actorType == "outlet" ||
		role == "admin" ||
		role == "administrator"
}

func readUintContextValue(c *gin.Context, key string) uint {
	value, exists := c.Get(key)
	if !exists {
		return 0
	}

	switch typed := value.(type) {
	case uint:
		return typed
	case int:
		if typed > 0 {
			return uint(typed)
		}
	case int64:
		if typed > 0 {
			return uint(typed)
		}
	case float64:
		if typed > 0 {
			return uint(typed)
		}
	case string:
		parsed, err := strconv.ParseUint(strings.TrimSpace(typed), 10, 64)
		if err == nil && parsed > 0 {
			return uint(parsed)
		}
	}

	return 0
}

func parseUintQueryParam(value string) uint {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return uint(parsed)
}

func parsePositiveIntQueryParam(value string, defaultValue, maxValue int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return defaultValue
	}
	if parsed > maxValue {
		return maxValue
	}
	return parsed
}
