package controllers

import (
	"encoding/json"
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
	"gorm.io/gorm/clause"
)

type stockAdjustmentApprovalRequestItemPayload struct {
	ProductID     uint    `json:"product_id"`
	ProductName   string  `json:"product_name"`
	QuantityDelta int     `json:"quantity_delta"`
	StockBefore   int     `json:"stock_before"`
	StockAfter    int     `json:"stock_after"`
	UnitCost      float64 `json:"unit_cost"`
	TotalCost     float64 `json:"total_cost"`
	Note          string  `json:"note"`
}

type stockAdjustmentApprovalRequestPayload struct {
	OutletID       uint                                        `json:"outlet_id"`
	AdjustmentDate time.Time                                   `json:"adjustment_date"`
	Note           string                                      `json:"note"`
	Items          []stockAdjustmentApprovalRequestItemPayload `json:"items"`
}

type stockAdjustmentApprovalDocumentSummary struct {
	ID                    uint       `json:"id"`
	OutletID              uint       `json:"outlet_id"`
	AdjustmentDate        time.Time  `json:"adjustment_date"`
	Reason                string     `json:"reason"`
	Status                string     `json:"status"`
	Note                  string     `json:"note"`
	AccountingSyncStatus  string     `json:"accounting_sync_status"`
	AccountingSyncError   string     `json:"accounting_sync_error"`
	AccountingSyncedAt    *time.Time `json:"accounting_synced_at"`
	AccountingIdempotency string     `json:"accounting_idempotency_key"`
}

type stockAdjustmentApprovalRequestResponse struct {
	ID                   uint                                        `json:"id"`
	OutletID             uint                                        `json:"outlet_id"`
	Status               string                                      `json:"status"`
	AdjustmentDate       time.Time                                   `json:"adjustment_date"`
	StockAdjustmentID    *uint                                       `json:"stock_adjustment_id"`
	RequestTotal         float64                                     `json:"request_total"`
	ItemCount            int                                         `json:"item_count"`
	Reason               string                                      `json:"reason"`
	RequestNote          string                                      `json:"request_note"`
	ReviewNote           string                                      `json:"review_note"`
	RequestedByUserID    string                                      `json:"requested_by_user_id"`
	RequestedByActorType string                                      `json:"requested_by_actor_type"`
	RequestedByName      string                                      `json:"requested_by_name"`
	RequestedAt          time.Time                                   `json:"requested_at"`
	ReviewedByUserID     string                                      `json:"reviewed_by_user_id"`
	ReviewedByActorType  string                                      `json:"reviewed_by_actor_type"`
	ReviewedByName       string                                      `json:"reviewed_by_name"`
	ReviewedAt           *time.Time                                  `json:"reviewed_at"`
	ApprovedAt           *time.Time                                  `json:"approved_at"`
	RejectedAt           *time.Time                                  `json:"rejected_at"`
	RequestedItems       []stockAdjustmentApprovalRequestItemPayload `json:"requested_items"`
	StockAdjustment      *stockAdjustmentApprovalDocumentSummary     `json:"stock_adjustment"`
}

type reviewStockAdjustmentApprovalRequestInput struct {
	Note string `json:"note"`
}

func ensureStockAdjustmentApprovalDependencies() error {
	if err := services.EnsureInventorySchema(database.DB); err != nil {
		return err
	}
	if err := services.EnsureAccountingSyncSchema(database.DB); err != nil {
		return err
	}
	if err := services.EnsureStockAdjustmentApprovalSchema(database.DB); err != nil {
		return err
	}
	return nil
}

func parseStockAdjustmentApprovalPayload(raw string) (stockAdjustmentApprovalRequestPayload, error) {
	payload := stockAdjustmentApprovalRequestPayload{}
	if strings.TrimSpace(raw) == "" {
		return payload, nil
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return payload, err
	}
	return payload, nil
}

func buildStockAdjustmentApprovalRequestItems(prepared []preparedStockAdjustmentItem) []stockAdjustmentApprovalRequestItemPayload {
	items := make([]stockAdjustmentApprovalRequestItemPayload, 0, len(prepared))
	for _, item := range prepared {
		items = append(items, stockAdjustmentApprovalRequestItemPayload{
			ProductID:     item.productID,
			ProductName:   item.productName,
			QuantityDelta: item.delta,
			StockBefore:   item.stockBefore,
			StockAfter:    item.stockAfter,
			UnitCost:      item.unitCost,
			TotalCost:     item.totalCost,
			Note:          item.note,
		})
	}
	return items
}

func buildStockAdjustmentApprovalDocumentSummary(adjustment models.StockAdjustment) *stockAdjustmentApprovalDocumentSummary {
	return &stockAdjustmentApprovalDocumentSummary{
		ID:                    adjustment.ID,
		OutletID:              adjustment.OutletID,
		AdjustmentDate:        adjustment.AdjustmentDate,
		Reason:                adjustment.Reason,
		Status:                adjustment.Status,
		Note:                  adjustment.Note,
		AccountingSyncStatus:  adjustment.AccountingSyncStatus,
		AccountingSyncError:   adjustment.AccountingSyncError,
		AccountingSyncedAt:    adjustment.AccountingSyncedAt,
		AccountingIdempotency: adjustment.AccountingIdempotency,
	}
}

func buildStockAdjustmentApprovalResponse(
	request models.StockAdjustmentApprovalRequest,
	adjustment *models.StockAdjustment,
) stockAdjustmentApprovalRequestResponse {
	payload, _ := parseStockAdjustmentApprovalPayload(request.RequestPayload)
	requestedByName := normalizeStoredApprovalActorName(
		request.RequestedByName,
		request.RequestedByActorType,
		request.RequestedByUserID,
	)
	reviewedByName := ""
	if strings.TrimSpace(request.ReviewedByName) != "" || strings.TrimSpace(request.ReviewedByUserID) != "" {
		reviewedByName = normalizeStoredApprovalActorName(
			request.ReviewedByName,
			request.ReviewedByActorType,
			request.ReviewedByUserID,
		)
	}

	response := stockAdjustmentApprovalRequestResponse{
		ID:                   request.ID,
		OutletID:             request.OutletID,
		Status:               request.Status,
		AdjustmentDate:       request.AdjustmentDate,
		StockAdjustmentID:    request.StockAdjustmentID,
		RequestTotal:         request.RequestTotal,
		ItemCount:            request.ItemCount,
		Reason:               request.Reason,
		RequestNote:          request.RequestNote,
		ReviewNote:           request.ReviewNote,
		RequestedByUserID:    request.RequestedByUserID,
		RequestedByActorType: request.RequestedByActorType,
		RequestedByName:      requestedByName,
		RequestedAt:          request.RequestedAt,
		ReviewedByUserID:     request.ReviewedByUserID,
		ReviewedByActorType:  request.ReviewedByActorType,
		ReviewedByName:       reviewedByName,
		ReviewedAt:           request.ReviewedAt,
		ApprovedAt:           request.ApprovedAt,
		RejectedAt:           request.RejectedAt,
		RequestedItems:       payload.Items,
	}

	if adjustment != nil {
		response.StockAdjustment = buildStockAdjustmentApprovalDocumentSummary(*adjustment)
	}

	return response
}

func CreateStockAdjustmentApprovalRequest(c *gin.Context) {
	var input StockAdjustmentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.OutletID == 0 || len(input.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outlet_id and items are required"})
		return
	}

	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reason is required"})
		return
	}

	if err := ensureStockAdjustmentApprovalDependencies(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	actor := readApprovalActorSnapshot(c)
	var request models.StockAdjustmentApprovalRequest

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		prepared, _, _, err := prepareStockAdjustmentItems(tx, input)
		if err != nil {
			return err
		}

		adjustmentDate := resolveStockAdjustmentDate(input)
		requestItems := buildStockAdjustmentApprovalRequestItems(prepared)
		payload := stockAdjustmentApprovalRequestPayload{
			OutletID:       input.OutletID,
			AdjustmentDate: adjustmentDate,
			Note:           strings.TrimSpace(input.Note),
			Items:          requestItems,
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		requestTotal := 0.0
		for _, item := range requestItems {
			requestTotal += item.TotalCost
		}

		requestedByName := normalizeStoredApprovalActorName(actor.Name, actor.ActorType, actor.UserID)
		if strings.TrimSpace(requestedByName) == "" {
			requestedByName = actor.Name
		}

		request = models.StockAdjustmentApprovalRequest{
			OutletID:             input.OutletID,
			Status:               services.POSApprovalStatusPending,
			AdjustmentDate:       adjustmentDate,
			RequestTotal:         requestTotal,
			ItemCount:            len(requestItems),
			Reason:               reason,
			RequestNote:          strings.TrimSpace(input.Note),
			RequestPayload:       string(payloadBytes),
			RequestedByUserID:    actor.UserID,
			RequestedByActorType: actor.ActorType,
			RequestedByName:      requestedByName,
			RequestedAt:          time.Now(),
		}

		return tx.Create(&request).Error
	})
	if err != nil {
		if isInventoryValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Stock adjustment request submitted for approval",
		"data":    buildStockAdjustmentApprovalResponse(request, nil),
	})
}

func GetStockAdjustmentApprovalRequests(c *gin.Context) {
	if err := services.EnsureStockAdjustmentApprovalSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	query := database.DB.Model(&models.StockAdjustmentApprovalRequest{})
	if outletID := strings.TrimSpace(c.Query("outlet_id")); outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}
	if status := strings.ToLower(strings.TrimSpace(c.Query("status"))); status != "" {
		query = query.Where("status = ?", status)
	}
	if startDate := strings.TrimSpace(c.Query("start_date")); startDate != "" {
		if parsed, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("DATE(requested_at) >= ?", parsed.Format("2006-01-02"))
		}
	}
	if endDate := strings.TrimSpace(c.Query("end_date")); endDate != "" {
		if parsed, err := time.Parse("2006-01-02", endDate); err == nil {
			query = query.Where("DATE(requested_at) <= ?", parsed.Format("2006-01-02"))
		}
	}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		like := "%" + search + "%"
		query = query.Where(
			"CAST(id AS TEXT) ILIKE ? OR CAST(COALESCE(stock_adjustment_id, 0) AS TEXT) ILIKE ? OR reason ILIKE ? OR COALESCE(request_note, '') ILIKE ? OR COALESCE(requested_by_name, '') ILIKE ? OR COALESCE(reviewed_by_name, '') ILIKE ?",
			like, like, like, like, like, like,
		)
	}

	var requests []models.StockAdjustmentApprovalRequest
	if err := query.Order("requested_at DESC, id DESC").Find(&requests).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load stock adjustment approval requests"})
		return
	}

	adjustmentIDs := make([]uint, 0, len(requests))
	for _, request := range requests {
		if request.StockAdjustmentID != nil && *request.StockAdjustmentID > 0 {
			adjustmentIDs = append(adjustmentIDs, *request.StockAdjustmentID)
		}
	}

	adjustmentMap := make(map[uint]models.StockAdjustment)
	if len(adjustmentIDs) > 0 {
		var adjustments []models.StockAdjustment
		if err := database.DB.Where("id IN ?", adjustmentIDs).Find(&adjustments).Error; err == nil {
			for _, adjustment := range adjustments {
				adjustmentMap[adjustment.ID] = adjustment
			}
		}
	}

	results := make([]stockAdjustmentApprovalRequestResponse, 0, len(requests))
	for _, request := range requests {
		var adjustment *models.StockAdjustment
		if request.StockAdjustmentID != nil {
			if loadedAdjustment, exists := adjustmentMap[*request.StockAdjustmentID]; exists {
				adjustment = &loadedAdjustment
			}
		}
		results = append(results, buildStockAdjustmentApprovalResponse(request, adjustment))
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"total":   len(results),
	})
}

func ApproveStockAdjustmentApprovalRequest(c *gin.Context) {
	var input reviewStockAdjustmentApprovalRequestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ensureStockAdjustmentApprovalDependencies(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	actor := readApprovalActorSnapshot(c)
	requestID, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || requestID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid stock adjustment approval request id"})
		return
	}

	var (
		request    models.StockAdjustmentApprovalRequest
		adjustment models.StockAdjustment
		hasJournal bool
	)

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&request, requestID).Error; err != nil {
			return err
		}
		if request.Status != services.POSApprovalStatusPending {
			return fmt.Errorf("stock adjustment request has already been reviewed")
		}

		payload, err := parseStockAdjustmentApprovalPayload(request.RequestPayload)
		if err != nil {
			return fmt.Errorf("invalid stock adjustment approval payload: %w", err)
		}
		if len(payload.Items) == 0 {
			return fmt.Errorf("stock adjustment request has no items")
		}

		documentInput := StockAdjustmentInput{
			OutletID:       request.OutletID,
			AdjustmentDate: request.AdjustmentDate,
			Reason:         request.Reason,
			Note:           payload.Note,
			Items:          make([]StockAdjustmentItemInput, 0, len(payload.Items)),
		}
		for _, item := range payload.Items {
			documentInput.Items = append(documentInput.Items, StockAdjustmentItemInput{
				ProductID:     item.ProductID,
				QuantityDelta: item.QuantityDelta,
				Note:          item.Note,
			})
		}

		executedAdjustment, executedHasJournal, err := executeStockAdjustmentDocument(tx, documentInput)
		if err != nil {
			return err
		}
		adjustment = executedAdjustment
		hasJournal = executedHasJournal

		now := time.Now()
		requestedByName := normalizeStoredApprovalActorName(
			request.RequestedByName,
			request.RequestedByActorType,
			request.RequestedByUserID,
		)
		updates := map[string]interface{}{
			"requested_by_name":      requestedByName,
			"status":                 services.POSApprovalStatusApproved,
			"stock_adjustment_id":    adjustment.ID,
			"review_note":            strings.TrimSpace(input.Note),
			"reviewed_by_user_id":    actor.UserID,
			"reviewed_by_actor_type": actor.ActorType,
			"reviewed_by_name":       actor.Name,
			"reviewed_at":            now,
			"approved_at":            now,
			"updated_at":             now,
		}
		if err := tx.Model(&request).Updates(updates).Error; err != nil {
			return err
		}

		request.Status = services.POSApprovalStatusApproved
		request.StockAdjustmentID = &adjustment.ID
		request.ReviewNote = strings.TrimSpace(input.Note)
		request.RequestedByName = requestedByName
		request.ReviewedByUserID = actor.UserID
		request.ReviewedByActorType = actor.ActorType
		request.ReviewedByName = actor.Name
		request.ReviewedAt = &now
		request.ApprovedAt = &now

		return nil
	})
	if err != nil {
		if isInventoryValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !hasJournal {
		c.JSON(http.StatusOK, gin.H{
			"message":                "Stock adjustment approval processed successfully",
			"accounting_sync_status": services.AccountingSyncStatusSynced,
			"data":                   buildStockAdjustmentApprovalResponse(request, &adjustment),
		})
		return
	}

	syncResult, syncErr := services.SyncAccountingRecord(services.AccountingRecordTypeStockAdjustment, adjustment.ID)
	if syncErr != nil {
		adjustment.AccountingSyncStatus = services.AccountingSyncStatusFailed
		adjustment.AccountingSyncError = syncErr.Error()
		c.JSON(http.StatusOK, gin.H{
			"message":                "Stock adjustment approved but accounting sync failed",
			"accounting_sync_status": services.AccountingSyncStatusFailed,
			"accounting_sync_error":  syncErr.Error(),
			"data":                   buildStockAdjustmentApprovalResponse(request, &adjustment),
		})
		return
	}

	adjustment.AccountingSyncStatus = syncResult.Status
	adjustment.AccountingIdempotency = syncResult.IdempotencyKey
	requestData := buildStockAdjustmentApprovalResponse(request, &adjustment)
	c.JSON(http.StatusOK, gin.H{
		"message":                    "Stock adjustment approval processed successfully",
		"accounting_sync_status":     syncResult.Status,
		"accounting_idempotency_key": syncResult.IdempotencyKey,
		"accounting_journal_id":      syncResult.JournalID,
		"accounting_already_posted":  syncResult.AlreadyPosted,
		"data":                       requestData,
	})
}

func RejectStockAdjustmentApprovalRequest(c *gin.Context) {
	var input reviewStockAdjustmentApprovalRequestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reviewNote := strings.TrimSpace(input.Note)
	if reviewNote == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "review note is required when rejecting a stock adjustment request"})
		return
	}

	if err := services.EnsureStockAdjustmentApprovalSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	actor := readApprovalActorSnapshot(c)
	requestID, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || requestID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid stock adjustment approval request id"})
		return
	}

	var request models.StockAdjustmentApprovalRequest
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&request, requestID).Error; err != nil {
			return err
		}
		if request.Status != services.POSApprovalStatusPending {
			return fmt.Errorf("stock adjustment request has already been reviewed")
		}

		now := time.Now()
		requestedByName := normalizeStoredApprovalActorName(
			request.RequestedByName,
			request.RequestedByActorType,
			request.RequestedByUserID,
		)
		if err := tx.Model(&request).Updates(map[string]interface{}{
			"requested_by_name":      requestedByName,
			"status":                 services.POSApprovalStatusRejected,
			"review_note":            reviewNote,
			"reviewed_by_user_id":    actor.UserID,
			"reviewed_by_actor_type": actor.ActorType,
			"reviewed_by_name":       actor.Name,
			"reviewed_at":            now,
			"rejected_at":            now,
			"updated_at":             now,
		}).Error; err != nil {
			return err
		}

		request.Status = services.POSApprovalStatusRejected
		request.RequestedByName = requestedByName
		request.ReviewNote = reviewNote
		request.ReviewedByUserID = actor.UserID
		request.ReviewedByActorType = actor.ActorType
		request.ReviewedByName = actor.Name
		request.ReviewedAt = &now
		request.RejectedAt = &now
		return nil
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Stock adjustment request rejected",
		"data":    buildStockAdjustmentApprovalResponse(request, nil),
	})
}
