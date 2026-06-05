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

type posApprovalActorSnapshot struct {
	UserID    string
	ActorType string
	Name      string
}

type posApprovalRequestItemPayload struct {
	ProductID   uint    `json:"product_id"`
	ProductName string  `json:"product_name"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Total       float64 `json:"total"`
}

type posApprovalRequestPayload struct {
	TransactionID       uint                            `json:"transaction_id"`
	OutletID            uint                            `json:"outlet_id"`
	SettlementType      string                          `json:"settlement_type"`
	SettlementMethod    string                          `json:"settlement_method"`
	Note                string                          `json:"note"`
	RefundTotalOverride *float64                        `json:"refund_total_override,omitempty"`
	Items               []posApprovalRequestItemPayload `json:"items"`
}

type createRefundApprovalRequestItemInput struct {
	ProductID uint `json:"product_id"`
	Quantity  int  `json:"quantity"`
}

type createRefundApprovalRequestInput struct {
	TransactionID    uint                                   `json:"transaction_id"`
	OutletID         uint                                   `json:"outlet_id"`
	SettlementType   string                                 `json:"settlement_type"`
	SettlementMethod string                                 `json:"settlement_method"`
	Reason           string                                 `json:"reason"`
	Note             string                                 `json:"note"`
	RequesterName    string                                 `json:"requester_name"`
	Items            []createRefundApprovalRequestItemInput `json:"items"`
}

type createVoidApprovalRequestInput struct {
	TransactionID uint   `json:"transaction_id"`
	OutletID      uint   `json:"outlet_id"`
	Reason        string `json:"reason"`
	Note          string `json:"note"`
	RequesterName string `json:"requester_name"`
}

type reviewPOSApprovalRequestInput struct {
	Note string `json:"note"`
}

type posApprovalTransactionSummary struct {
	ID             uint                            `json:"id"`
	OutletID       uint                            `json:"outlet_id"`
	CashierID      uint                            `json:"cashier_id"`
	CashierName    string                          `json:"cashier_name"`
	Total          float64                         `json:"total"`
	Tax            float64                         `json:"tax"`
	Discount       float64                         `json:"discount"`
	PaymentMethod  string                          `json:"payment_method"`
	Note           string                          `json:"note"`
	CreatedAt      time.Time                       `json:"created_at"`
	DocumentStatus string                          `json:"document_status"`
	Items          []posApprovalRequestItemPayload `json:"items"`
}

type posApprovalRefundSummary struct {
	ID                   uint       `json:"id"`
	RefundNumber         string     `json:"refund_number"`
	RefundTotal          float64    `json:"refund_total"`
	Note                 string     `json:"note"`
	SettlementType       string     `json:"settlement_type"`
	SettlementMethod     string     `json:"settlement_method"`
	SettlementStatus     string     `json:"settlement_status"`
	StoreCreditCode      string     `json:"store_credit_code"`
	CreatedAt            time.Time  `json:"created_at"`
	AccountingSyncStatus string     `json:"accounting_sync_status"`
	AccountingSyncError  string     `json:"accounting_sync_error"`
	AccountingSyncedAt   *time.Time `json:"accounting_synced_at"`
}

type posApprovalRequestResponse struct {
	ID                        uint                            `json:"id"`
	RequestType               string                          `json:"request_type"`
	Status                    string                          `json:"status"`
	TransactionID             uint                            `json:"transaction_id"`
	OutletID                  uint                            `json:"outlet_id"`
	RefundID                  *uint                           `json:"refund_id"`
	RequestTotal              float64                         `json:"request_total"`
	ItemCount                 int                             `json:"item_count"`
	Reason                    string                          `json:"reason"`
	RequestNote               string                          `json:"request_note"`
	ReviewNote                string                          `json:"review_note"`
	RequestedByUserID         string                          `json:"requested_by_user_id"`
	RequestedByActorType      string                          `json:"requested_by_actor_type"`
	RequestedByName           string                          `json:"requested_by_name"`
	RequestedAt               time.Time                       `json:"requested_at"`
	ReviewedByUserID          string                          `json:"reviewed_by_user_id"`
	ReviewedByActorType       string                          `json:"reviewed_by_actor_type"`
	ReviewedByName            string                          `json:"reviewed_by_name"`
	ReviewedAt                *time.Time                      `json:"reviewed_at"`
	ApprovedAt                *time.Time                      `json:"approved_at"`
	RejectedAt                *time.Time                      `json:"rejected_at"`
	RequestedSettlementType   string                          `json:"requested_settlement_type"`
	RequestedSettlementMethod string                          `json:"requested_settlement_method"`
	RequestedItems            []posApprovalRequestItemPayload `json:"requested_items"`
	Transaction               *posApprovalTransactionSummary  `json:"transaction"`
	Refund                    *posApprovalRefundSummary       `json:"refund"`
}

type transactionItemAggregate struct {
	ProductID   uint
	ProductName string
	Quantity    int
	UnitPrice   float64
	Total       float64
}

func normalizeApprovalActorType(actorType, role string) string {
	normalizedActorType := strings.ToLower(strings.TrimSpace(actorType))
	if normalizedActorType != "" {
		return normalizedActorType
	}

	normalizedRole := strings.ToLower(strings.TrimSpace(role))
	switch {
	case strings.Contains(normalizedRole, "outlet"):
		return "outlet"
	case strings.Contains(normalizedRole, "admin"):
		return "admin"
	case strings.Contains(normalizedRole, "karyawan"), strings.Contains(normalizedRole, "employee"), strings.Contains(normalizedRole, "staff"):
		return "karyawan"
	default:
		return normalizedRole
	}
}

func buildApprovalActorFallbackLabel(actorType, userID string) string {
	switch strings.ToLower(strings.TrimSpace(actorType)) {
	case "outlet":
		return fmt.Sprintf("Outlet #%s", userID)
	case "admin", "administrator":
		return fmt.Sprintf("Admin #%s", userID)
	case "karyawan", "employee", "staff":
		return fmt.Sprintf("Karyawan #%s", userID)
	default:
		return fmt.Sprintf("User #%s", userID)
	}
}

func lookupApprovalActorNameByType(actorType string, userID int64) string {
	if userID <= 0 || database.DB == nil {
		return ""
	}

	resolvedName := ""
	switch actorType {
	case "karyawan", "employee", "staff":
		database.DB.Raw(`
			SELECT COALESCE(NULLIF(BTRIM(nama), ''), NULLIF(BTRIM(username), ''))
			FROM tbkaryawan
			WHERE id = ? AND deleted_at IS NULL
			LIMIT 1
		`, userID).Scan(&resolvedName)
	case "outlet":
		database.DB.Raw(`
			SELECT COALESCE(NULLIF(BTRIM(outlet_name), ''), NULLIF(BTRIM(username), ''))
			FROM tboutlet
			WHERE id = ? AND deleted_at IS NULL
			LIMIT 1
		`, userID).Scan(&resolvedName)
	case "admin", "administrator":
		database.DB.Raw(`
			SELECT NULLIF(BTRIM(username), '')
			FROM tbadmin
			WHERE id = ?
			LIMIT 1
		`, userID).Scan(&resolvedName)
	}

	return strings.TrimSpace(resolvedName)
}

func resolveApprovalActorNameFromDatabase(actorType, userID string) string {
	parsedUserID, err := strconv.ParseInt(strings.TrimSpace(userID), 10, 64)
	if err != nil || parsedUserID <= 0 || database.DB == nil {
		return ""
	}

	normalizedActorType := normalizeApprovalActorType(actorType, "")
	candidateTypes := []string{normalizedActorType}
	if normalizedActorType == "" {
		candidateTypes = []string{"karyawan", "admin", "outlet"}
	}

	for _, candidateType := range candidateTypes {
		if resolvedName := lookupApprovalActorNameByType(candidateType, parsedUserID); resolvedName != "" {
			return resolvedName
		}
	}

	if normalizedActorType != "" {
		return buildApprovalActorFallbackLabel(normalizedActorType, userID)
	}

	return ""
}

func isApprovalActorPlaceholderName(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return true
	}

	switch normalized {
	case "-", "backoffice", "user tidak dikenal", "unknown":
		return true
	}

	return strings.HasPrefix(normalized, "user #") ||
		strings.HasPrefix(normalized, "karyawan #") ||
		strings.HasPrefix(normalized, "admin #") ||
		strings.HasPrefix(normalized, "outlet #")
}

func normalizeStoredApprovalActorName(name, actorType, userID string) string {
	trimmedName := strings.TrimSpace(name)
	if !isApprovalActorPlaceholderName(trimmedName) {
		return trimmedName
	}

	if resolvedName := resolveApprovalActorNameFromDatabase(actorType, userID); resolvedName != "" {
		return resolvedName
	}

	if trimmedName != "" {
		return trimmedName
	}
	if strings.TrimSpace(userID) != "" {
		return buildApprovalActorFallbackLabel(normalizeApprovalActorType(actorType, ""), userID)
	}

	return "User tidak dikenal"
}

func resolveApprovalRequesterName(
	actor posApprovalActorSnapshot,
	providedName string,
	transaction *models.Transaction,
	fallbackActorType string,
) string {
	normalizedProvidedName := strings.TrimSpace(providedName)
	if normalizedProvidedName != "" {
		if isApprovalActorPlaceholderName(actor.Name) || strings.TrimSpace(actor.Name) == "" {
			return normalizedProvidedName
		}
	}

	resolvedName := normalizeStoredApprovalActorName(actor.Name, actor.ActorType, actor.UserID)
	if !isApprovalActorPlaceholderName(resolvedName) {
		return resolvedName
	}

	normalizedActorType := normalizeApprovalActorType(actor.ActorType, fallbackActorType)
	if normalizedActorType == "karyawan" && transaction != nil {
		cashierName := strings.TrimSpace(transaction.CashierName)
		if cashierName != "" {
			return cashierName
		}
	}

	if normalizedProvidedName != "" {
		return normalizedProvidedName
	}

	return resolvedName
}

func readApprovalActorSnapshot(c *gin.Context) posApprovalActorSnapshot {
	userID := ""
	if rawUserID, exists := c.Get("user_id"); exists {
		switch value := rawUserID.(type) {
		case string:
			userID = strings.TrimSpace(value)
		case int64:
			userID = strconv.FormatInt(value, 10)
		case int:
			userID = strconv.Itoa(value)
		}
	}

	actorType := normalizeApprovalActorType(c.GetString("actor_type"), c.GetString("role"))

	name := strings.TrimSpace(c.GetString("display_name"))
	if name == "" {
		for _, key := range []string{"full_name", "fullname", "name", "nama", "outlet_name", "username", "email"} {
			name = strings.TrimSpace(c.GetString(key))
			if name != "" {
				break
			}
		}
	}
	if name == "" {
		name = resolveApprovalActorNameFromDatabase(actorType, userID)
	}
	if name == "" && userID != "" {
		name = buildApprovalActorFallbackLabel(actorType, userID)
	}
	if name == "" {
		name = "User tidak dikenal"
	}

	return posApprovalActorSnapshot{
		UserID:    userID,
		ActorType: actorType,
		Name:      name,
	}
}

func ensurePOSApprovalDependencies() error {
	if err := services.EnsurePOSApprovalSchema(database.DB); err != nil {
		return err
	}
	if err := services.EnsureInventorySchema(database.DB); err != nil {
		return err
	}
	if err := services.EnsureAccountingSyncSchema(database.DB); err != nil {
		return err
	}
	if err := services.EnsureRefundSettlementSchema(database.DB); err != nil {
		return err
	}
	return nil
}

func parsePOSApprovalPayload(raw string) (posApprovalRequestPayload, error) {
	payload := posApprovalRequestPayload{}
	if strings.TrimSpace(raw) == "" {
		return payload, nil
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return payload, err
	}
	return payload, nil
}

func aggregateTransactionItems(transaction models.Transaction) map[uint]transactionItemAggregate {
	aggregated := make(map[uint]transactionItemAggregate)
	for _, item := range transaction.Items {
		entry := aggregated[item.ProductID]
		entry.ProductID = item.ProductID
		entry.ProductName = strings.TrimSpace(item.Product.Name)
		if entry.ProductName == "" {
			entry.ProductName = fmt.Sprintf("Product #%d", item.ProductID)
		}
		entry.Quantity += item.Quantity
		entry.Total += item.Total
		if entry.Quantity > 0 {
			entry.UnitPrice = entry.Total / float64(entry.Quantity)
		}
		aggregated[item.ProductID] = entry
	}
	return aggregated
}

func buildRefundRequestItemsFromInput(
	transaction models.Transaction,
	inputItems []createRefundApprovalRequestItemInput,
) ([]posApprovalRequestItemPayload, []models.RefundItem, error) {
	aggregated := aggregateTransactionItems(transaction)
	requestedItems := make([]posApprovalRequestItemPayload, 0, len(inputItems))
	refundItems := make([]models.RefundItem, 0, len(inputItems))
	seenProductIDs := make(map[uint]struct{})

	for _, item := range inputItems {
		if item.ProductID == 0 || item.Quantity <= 0 {
			continue
		}
		if _, exists := seenProductIDs[item.ProductID]; exists {
			return nil, nil, fmt.Errorf("duplicate product_id %d in refund request", item.ProductID)
		}

		aggregate, exists := aggregated[item.ProductID]
		if !exists || aggregate.Quantity <= 0 {
			return nil, nil, fmt.Errorf("product %d is not part of the original transaction", item.ProductID)
		}

		total := aggregate.UnitPrice * float64(item.Quantity)
		requestedItems = append(requestedItems, posApprovalRequestItemPayload{
			ProductID:   item.ProductID,
			ProductName: aggregate.ProductName,
			Quantity:    item.Quantity,
			UnitPrice:   aggregate.UnitPrice,
			Total:       total,
		})
		refundItems = append(refundItems, models.RefundItem{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			UnitPrice: aggregate.UnitPrice,
			Total:     total,
		})
		seenProductIDs[item.ProductID] = struct{}{}
	}

	if len(requestedItems) == 0 {
		return nil, nil, fmt.Errorf("minimal satu item refund wajib dipilih")
	}

	return requestedItems, refundItems, nil
}

func buildVoidRequestItems(transaction models.Transaction) ([]posApprovalRequestItemPayload, []models.RefundItem, error) {
	aggregated := aggregateTransactionItems(transaction)
	if len(aggregated) == 0 {
		return nil, nil, fmt.Errorf("transaction has no refundable items")
	}

	requestedItems := make([]posApprovalRequestItemPayload, 0, len(aggregated))
	refundItems := make([]models.RefundItem, 0, len(aggregated))
	for _, item := range aggregated {
		total := item.UnitPrice * float64(item.Quantity)
		requestedItems = append(requestedItems, posApprovalRequestItemPayload{
			ProductID:   item.ProductID,
			ProductName: item.ProductName,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			Total:       total,
		})
		refundItems = append(refundItems, models.RefundItem{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
			Total:     total,
		})
	}

	return requestedItems, refundItems, nil
}

func loadTransactionForApproval(tx *gorm.DB, transactionID uint) (models.Transaction, error) {
	var transaction models.Transaction
	err := tx.Preload("Items.Product").First(&transaction, transactionID).Error
	return transaction, err
}

func hasPendingApprovalRequest(tx *gorm.DB, transactionID uint) (bool, error) {
	var count int64
	if err := tx.Model(&models.POSApprovalRequest{}).
		Where("transaction_id = ? AND status = ?", transactionID, services.POSApprovalStatusPending).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func hasApprovedApprovalRequest(tx *gorm.DB, transactionID uint, requestType string, excludeRequestID uint) (bool, error) {
	query := tx.Model(&models.POSApprovalRequest{}).
		Where(
			"transaction_id = ? AND request_type = ? AND status = ?",
			transactionID,
			requestType,
			services.POSApprovalStatusApproved,
		)
	if excludeRequestID != 0 {
		query = query.Where("id <> ?", excludeRequestID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func hasApprovedVoidRequest(tx *gorm.DB, transactionID uint, excludeRequestID uint) (bool, error) {
	return hasApprovedApprovalRequest(tx, transactionID, services.POSApprovalRequestTypeVoid, excludeRequestID)
}

func calculateTransactionFinalTotal(transaction models.Transaction) float64 {
	return float64(transaction.Total - transaction.Discount + transaction.Tax)
}

func buildApprovalTransactionSummary(transaction models.Transaction) *posApprovalTransactionSummary {
	items := make([]posApprovalRequestItemPayload, 0, len(transaction.Items))
	for _, item := range transaction.Items {
		productName := strings.TrimSpace(item.Product.Name)
		if productName == "" {
			productName = fmt.Sprintf("Product #%d", item.ProductID)
		}
		items = append(items, posApprovalRequestItemPayload{
			ProductID:   item.ProductID,
			ProductName: productName,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			Total:       item.Total,
		})
	}

	return &posApprovalTransactionSummary{
		ID:             transaction.ID,
		OutletID:       transaction.OutletID,
		CashierID:      transaction.CashierID,
		CashierName:    transaction.CashierName,
		Total:          transaction.Total,
		Tax:            transaction.Tax,
		Discount:       transaction.Discount,
		PaymentMethod:  transaction.PaymentMethod,
		Note:           transaction.Note,
		CreatedAt:      transaction.CreatedAt,
		DocumentStatus: strings.TrimSpace(transaction.DocumentStatus),
		Items:          items,
	}
}

func buildApprovalRefundSummary(refund models.Refund) *posApprovalRefundSummary {
	return &posApprovalRefundSummary{
		ID:                   refund.ID,
		RefundNumber:         refund.RefundNumber,
		RefundTotal:          refund.RefundTotal,
		Note:                 refund.Note,
		SettlementType:       refund.SettlementType,
		SettlementMethod:     refund.SettlementMethod,
		SettlementStatus:     refund.SettlementStatus,
		StoreCreditCode:      refund.StoreCreditCode,
		CreatedAt:            refund.CreatedAt,
		AccountingSyncStatus: refund.AccountingSyncStatus,
		AccountingSyncError:  refund.AccountingSyncError,
		AccountingSyncedAt:   refund.AccountingSyncedAt,
	}
}

func buildApprovalResponse(
	request models.POSApprovalRequest,
	transaction *models.Transaction,
	refund *models.Refund,
) posApprovalRequestResponse {
	payload, _ := parsePOSApprovalPayload(request.RequestPayload)
	requestedByName := normalizeStoredApprovalActorName(
		request.RequestedByName,
		request.RequestedByActorType,
		request.RequestedByUserID,
	)
	if isApprovalActorPlaceholderName(requestedByName) && transaction != nil && normalizeApprovalActorType(request.RequestedByActorType, "") == "karyawan" {
		cashierName := strings.TrimSpace(transaction.CashierName)
		if cashierName != "" {
			requestedByName = cashierName
		}
	}
	reviewedByName := normalizeStoredApprovalActorName(
		request.ReviewedByName,
		request.ReviewedByActorType,
		request.ReviewedByUserID,
	)

	response := posApprovalRequestResponse{
		ID:                        request.ID,
		RequestType:               request.RequestType,
		Status:                    request.Status,
		TransactionID:             request.TransactionID,
		OutletID:                  request.OutletID,
		RefundID:                  request.RefundID,
		RequestTotal:              request.RequestTotal,
		ItemCount:                 request.ItemCount,
		Reason:                    request.Reason,
		RequestNote:               request.RequestNote,
		ReviewNote:                request.ReviewNote,
		RequestedByUserID:         request.RequestedByUserID,
		RequestedByActorType:      request.RequestedByActorType,
		RequestedByName:           requestedByName,
		RequestedAt:               request.RequestedAt,
		ReviewedByUserID:          request.ReviewedByUserID,
		ReviewedByActorType:       request.ReviewedByActorType,
		ReviewedByName:            reviewedByName,
		ReviewedAt:                request.ReviewedAt,
		ApprovedAt:                request.ApprovedAt,
		RejectedAt:                request.RejectedAt,
		RequestedSettlementType:   payload.SettlementType,
		RequestedSettlementMethod: payload.SettlementMethod,
		RequestedItems:            payload.Items,
	}

	if transaction != nil {
		response.Transaction = buildApprovalTransactionSummary(*transaction)
	}
	if refund != nil {
		response.Refund = buildApprovalRefundSummary(*refund)
	}

	return response
}

func CreateRefundApprovalRequest(c *gin.Context) {
	var input createRefundApprovalRequestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ensurePOSApprovalDependencies(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	actor := readApprovalActorSnapshot(c)
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reason is required"})
		return
	}

	var (
		request     models.POSApprovalRequest
		transaction models.Transaction
	)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		loadedTransaction, err := loadTransactionForApproval(tx, input.TransactionID)
		if err != nil {
			return err
		}
		if input.OutletID != 0 && loadedTransaction.OutletID != input.OutletID {
			return fmt.Errorf("request outlet does not match original transaction outlet")
		}
		if strings.EqualFold(strings.TrimSpace(loadedTransaction.DocumentStatus), services.TransactionDocumentStatusVoided) {
			return fmt.Errorf("transaction has already been voided")
		}

		hasPending, err := hasPendingApprovalRequest(tx, loadedTransaction.ID)
		if err != nil {
			return err
		}
		if hasPending {
			return fmt.Errorf("transaction already has a pending approval request")
		}

		hasVoid, err := hasApprovedVoidRequest(tx, loadedTransaction.ID, 0)
		if err != nil {
			return err
		}
		if hasVoid {
			return fmt.Errorf("transaction has already been voided")
		}
		requestedItems, refundItems, err := buildRefundRequestItemsFromInput(loadedTransaction, input.Items)
		if err != nil {
			return err
		}
		if _, err := services.PrepareRefundInventoryPlan(tx, loadedTransaction.ID, loadedTransaction.OutletID, refundItems); err != nil {
			return err
		}

		payload := posApprovalRequestPayload{
			TransactionID:  loadedTransaction.ID,
			OutletID:       loadedTransaction.OutletID,
			SettlementType: services.NormalizeRefundSettlementType(input.SettlementType),
			SettlementMethod: services.ResolveRefundSettlementMethod(
				loadedTransaction.PaymentMethod,
				input.SettlementType,
				input.SettlementMethod,
			),
			Note:  strings.TrimSpace(input.Note),
			Items: requestedItems,
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		requestTotal := 0.0
		for _, item := range requestedItems {
			requestTotal += item.Total
		}
		requestedByName := resolveApprovalRequesterName(actor, input.RequesterName, &loadedTransaction, "karyawan")

		request = models.POSApprovalRequest{
			OutletID:             loadedTransaction.OutletID,
			RequestType:          services.POSApprovalRequestTypeRefund,
			Status:               services.POSApprovalStatusPending,
			TransactionID:        loadedTransaction.ID,
			RequestTotal:         requestTotal,
			ItemCount:            len(requestedItems),
			Reason:               reason,
			RequestNote:          strings.TrimSpace(input.Note),
			RequestPayload:       string(payloadBytes),
			RequestedByUserID:    actor.UserID,
			RequestedByActorType: actor.ActorType,
			RequestedByName:      requestedByName,
			RequestedAt:          time.Now(),
		}
		if err := tx.Create(&request).Error; err != nil {
			return err
		}

		if err := recordDocumentAudit(tx, c, documentAuditInput{
			OutletID:     request.OutletID,
			DocumentType: "pos_approval_request",
			DocumentID:   request.ID,
			Action:       documentAuditActionSubmitted,
			Summary:      fmt.Sprintf("Pengajuan refund request #%d dibuat untuk transaksi #%d", request.ID, loadedTransaction.ID),
			Note:         request.RequestNote,
			Metadata: gin.H{
				"request_type":   request.RequestType,
				"transaction_id": request.TransactionID,
				"request_total":  request.RequestTotal,
				"item_count":     request.ItemCount,
			},
			After: buildApprovalResponse(request, &loadedTransaction, nil),
		}); err != nil {
			return err
		}

		transaction = loadedTransaction
		return nil
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Refund approval request created",
		"data":    buildApprovalResponse(request, &transaction, nil),
	})
}

func CreateVoidApprovalRequest(c *gin.Context) {
	var input createVoidApprovalRequestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ensurePOSApprovalDependencies(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	actor := readApprovalActorSnapshot(c)
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reason is required"})
		return
	}

	var (
		request     models.POSApprovalRequest
		transaction models.Transaction
	)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		loadedTransaction, err := loadTransactionForApproval(tx, input.TransactionID)
		if err != nil {
			return err
		}
		if input.OutletID != 0 && loadedTransaction.OutletID != input.OutletID {
			return fmt.Errorf("request outlet does not match original transaction outlet")
		}
		if strings.EqualFold(strings.TrimSpace(loadedTransaction.DocumentStatus), services.TransactionDocumentStatusVoided) {
			return fmt.Errorf("transaction has already been voided")
		}

		hasPending, err := hasPendingApprovalRequest(tx, loadedTransaction.ID)
		if err != nil {
			return err
		}
		if hasPending {
			return fmt.Errorf("transaction already has a pending approval request")
		}

		hasVoid, err := hasApprovedVoidRequest(tx, loadedTransaction.ID, 0)
		if err != nil {
			return err
		}
		if hasVoid {
			return fmt.Errorf("transaction has already been voided")
		}
		hasApprovedRefund, err := hasApprovedApprovalRequest(tx, loadedTransaction.ID, services.POSApprovalRequestTypeRefund, 0)
		if err != nil {
			return err
		}
		if hasApprovedRefund {
			return fmt.Errorf("transaction already has an approved refund request")
		}

		requestedItems, refundItems, err := buildVoidRequestItems(loadedTransaction)
		if err != nil {
			return err
		}
		if _, err := services.PrepareRefundInventoryPlan(tx, loadedTransaction.ID, loadedTransaction.OutletID, refundItems); err != nil {
			return err
		}

		overrideTotal := calculateTransactionFinalTotal(loadedTransaction)
		payload := posApprovalRequestPayload{
			TransactionID:       loadedTransaction.ID,
			OutletID:            loadedTransaction.OutletID,
			Note:                strings.TrimSpace(input.Note),
			RefundTotalOverride: &overrideTotal,
			Items:               requestedItems,
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		requestedByName := resolveApprovalRequesterName(actor, input.RequesterName, &loadedTransaction, "karyawan")

		request = models.POSApprovalRequest{
			OutletID:             loadedTransaction.OutletID,
			RequestType:          services.POSApprovalRequestTypeVoid,
			Status:               services.POSApprovalStatusPending,
			TransactionID:        loadedTransaction.ID,
			RequestTotal:         overrideTotal,
			ItemCount:            len(requestedItems),
			Reason:               reason,
			RequestNote:          strings.TrimSpace(input.Note),
			RequestPayload:       string(payloadBytes),
			RequestedByUserID:    actor.UserID,
			RequestedByActorType: actor.ActorType,
			RequestedByName:      requestedByName,
			RequestedAt:          time.Now(),
		}
		if err := tx.Create(&request).Error; err != nil {
			return err
		}

		if err := recordDocumentAudit(tx, c, documentAuditInput{
			OutletID:     request.OutletID,
			DocumentType: "pos_approval_request",
			DocumentID:   request.ID,
			Action:       documentAuditActionSubmitted,
			Summary:      fmt.Sprintf("Pengajuan void request #%d dibuat untuk transaksi #%d", request.ID, loadedTransaction.ID),
			Note:         request.RequestNote,
			Metadata: gin.H{
				"request_type":   request.RequestType,
				"transaction_id": request.TransactionID,
				"request_total":  request.RequestTotal,
				"item_count":     request.ItemCount,
			},
			After: buildApprovalResponse(request, &loadedTransaction, nil),
		}); err != nil {
			return err
		}

		transaction = loadedTransaction
		return nil
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Void approval request created",
		"data":    buildApprovalResponse(request, &transaction, nil),
	})
}

func GetPOSApprovalRequests(c *gin.Context) {
	if err := services.EnsurePOSApprovalSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	query := database.DB.Model(&models.POSApprovalRequest{})
	if outletID := strings.TrimSpace(c.Query("outlet_id")); outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}
	if requestType := strings.ToLower(strings.TrimSpace(c.Query("request_type"))); requestType != "" {
		query = query.Where("request_type = ?", requestType)
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
			"CAST(transaction_id AS TEXT) ILIKE ? OR reason ILIKE ? OR COALESCE(request_note, '') ILIKE ? OR COALESCE(requested_by_name, '') ILIKE ? OR COALESCE(reviewed_by_name, '') ILIKE ?",
			like, like, like, like, like,
		)
	}

	var requests []models.POSApprovalRequest
	if err := query.Order("requested_at DESC, id DESC").Find(&requests).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load approval requests"})
		return
	}

	transactionIDs := make([]uint, 0, len(requests))
	refundIDs := make([]uint, 0, len(requests))
	for _, request := range requests {
		if request.TransactionID > 0 {
			transactionIDs = append(transactionIDs, request.TransactionID)
		}
		if request.RefundID != nil && *request.RefundID > 0 {
			refundIDs = append(refundIDs, *request.RefundID)
		}
	}

	transactionMap := make(map[uint]models.Transaction)
	if len(transactionIDs) > 0 {
		var transactions []models.Transaction
		if err := database.DB.Preload("Items.Product").Where("id IN ?", transactionIDs).Find(&transactions).Error; err == nil {
			for _, transaction := range transactions {
				transactionMap[transaction.ID] = transaction
			}
		}
	}

	refundMap := make(map[uint]models.Refund)
	if len(refundIDs) > 0 {
		var refunds []models.Refund
		if err := database.DB.Where("id IN ?", refundIDs).Find(&refunds).Error; err == nil {
			for _, refund := range refunds {
				refundMap[refund.ID] = refund
			}
		}
	}

	results := make([]posApprovalRequestResponse, 0, len(requests))
	for _, request := range requests {
		var transaction *models.Transaction
		if loadedTransaction, exists := transactionMap[request.TransactionID]; exists {
			transaction = &loadedTransaction
		}

		var refund *models.Refund
		if request.RefundID != nil {
			if loadedRefund, exists := refundMap[*request.RefundID]; exists {
				refund = &loadedRefund
			}
		}

		results = append(results, buildApprovalResponse(request, transaction, refund))
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"total":   len(results),
	})
}

func ApprovePOSApprovalRequest(c *gin.Context) {
	var input reviewPOSApprovalRequestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ensurePOSApprovalDependencies(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	actor := readApprovalActorSnapshot(c)
	requestID, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || requestID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid approval request id"})
		return
	}

	var (
		request models.POSApprovalRequest
		refund  models.Refund
	)

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&request, requestID).Error; err != nil {
			return err
		}
		if request.Status != services.POSApprovalStatusPending {
			return fmt.Errorf("approval request has already been reviewed")
		}
		beforeRequest := request

		payload, err := parsePOSApprovalPayload(request.RequestPayload)
		if err != nil {
			return fmt.Errorf("invalid approval payload: %w", err)
		}
		items := make([]models.RefundItem, 0, len(payload.Items))
		for _, item := range payload.Items {
			items = append(items, models.RefundItem{
				ProductID: item.ProductID,
				Quantity:  item.Quantity,
				UnitPrice: item.UnitPrice,
				Total:     item.Total,
			})
		}
		if len(items) == 0 {
			return fmt.Errorf("approval request has no refund items")
		}

		transaction, err := loadTransactionForApproval(tx, request.TransactionID)
		if err != nil {
			return err
		}
		if strings.EqualFold(strings.TrimSpace(transaction.DocumentStatus), services.TransactionDocumentStatusVoided) && request.RequestType == services.POSApprovalRequestTypeVoid {
			return fmt.Errorf("transaction has already been voided")
		}
		if request.RequestType == services.POSApprovalRequestTypeVoid {
			hasVoid, err := hasApprovedVoidRequest(tx, request.TransactionID, request.ID)
			if err != nil {
				return err
			}
			if hasVoid {
				return fmt.Errorf("transaction has already been voided")
			}
		}

		requestedByName := normalizeStoredApprovalActorName(
			request.RequestedByName,
			request.RequestedByActorType,
			request.RequestedByUserID,
		)
		executedRefund, _, err := executeRefundDocument(tx, refundExecutionInput{
			TransactionID:        request.TransactionID,
			OutletID:             request.OutletID,
			CashierName:          requestedByName,
			SettlementType:       payload.SettlementType,
			SettlementMethod:     payload.SettlementMethod,
			Note:                 payload.Note,
			Items:                items,
			RefundTotalOverride:  payload.RefundTotalOverride,
			ApprovalRequestID:    &request.ID,
			AccountingSyncStatus: services.AccountingSyncStatusPending,
		})
		if err != nil {
			return err
		}
		refund = executedRefund

		now := time.Now()
		updates := map[string]interface{}{
			"requested_by_name":      requestedByName,
			"status":                 services.POSApprovalStatusApproved,
			"refund_id":              refund.ID,
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

		if request.RequestType == services.POSApprovalRequestTypeVoid {
			if err := tx.Model(&models.Transaction{}).
				Where("id = ?", request.TransactionID).
				Updates(map[string]interface{}{
					"document_status":          services.TransactionDocumentStatusVoided,
					"voided_at":                now,
					"void_reason":              request.Reason,
					"void_approval_request_id": request.ID,
				}).Error; err != nil {
				return err
			}
			transaction.DocumentStatus = services.TransactionDocumentStatusVoided
			transaction.VoidReason = request.Reason
			transaction.VoidApprovalRequestID = &request.ID
		}

		request.Status = services.POSApprovalStatusApproved
		request.RefundID = &refund.ID
		request.ReviewNote = strings.TrimSpace(input.Note)
		request.RequestedByName = requestedByName
		request.ReviewedByUserID = actor.UserID
		request.ReviewedByActorType = actor.ActorType
		request.ReviewedByName = actor.Name
		request.ReviewedAt = &now
		request.ApprovedAt = &now

		if err := recordDocumentAudit(tx, c, documentAuditInput{
			OutletID:     request.OutletID,
			DocumentType: "pos_approval_request",
			DocumentID:   request.ID,
			Action:       documentAuditActionApproved,
			Summary:      fmt.Sprintf("Approval request #%d disetujui", request.ID),
			Note:         strings.TrimSpace(input.Note),
			Metadata: gin.H{
				"request_type":   request.RequestType,
				"transaction_id": request.TransactionID,
				"refund_id":      refund.ID,
			},
			Before: buildApprovalResponse(beforeRequest, &transaction, nil),
			After:  buildApprovalResponse(request, &transaction, &refund),
		}); err != nil {
			return err
		}

		if err := recordDocumentAudit(tx, c, documentAuditInput{
			OutletID:     refund.OutletID,
			DocumentType: "refund",
			DocumentID:   refund.ID,
			Action:       documentAuditActionCreated,
			Summary:      fmt.Sprintf("Refund #%d dibuat dari approval request #%d", refund.ID, request.ID),
			Note:         refund.Note,
			Metadata: gin.H{
				"transaction_id":      refund.TransactionID,
				"approval_request_id": request.ID,
				"request_type":        request.RequestType,
				"refund_total":        refund.RefundTotal,
			},
			After: gin.H{
				"refund": refund,
				"items":  payload.Items,
			},
		}); err != nil {
			return err
		}

		if request.RequestType == services.POSApprovalRequestTypeVoid {
			if err := recordDocumentAudit(tx, c, documentAuditInput{
				OutletID:     transaction.OutletID,
				DocumentType: "transaction",
				DocumentID:   transaction.ID,
				Action:       documentAuditActionVoided,
				Summary:      fmt.Sprintf("Transaksi #%d di-void lewat approval request #%d", transaction.ID, request.ID),
				Note:         request.Reason,
				Metadata: gin.H{
					"approval_request_id": request.ID,
					"refund_id":           refund.ID,
				},
				After: gin.H{
					"transaction_id":           transaction.ID,
					"document_status":          transaction.DocumentStatus,
					"void_reason":              transaction.VoidReason,
					"void_approval_request_id": transaction.VoidApprovalRequestID,
				},
			}); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	syncResult, syncErr := services.SyncAccountingRecord(services.AccountingRecordTypeRefund, refund.ID)
	if syncErr != nil {
		c.JSON(http.StatusOK, gin.H{
			"message":                "Approval processed but accounting sync failed",
			"accounting_sync_status": services.AccountingSyncStatusFailed,
			"accounting_sync_error":  syncErr.Error(),
			"data":                   buildApprovalResponse(request, nil, &refund),
		})
		return
	}

	refund.AccountingSyncStatus = syncResult.Status
	requestData := buildApprovalResponse(request, nil, &refund)
	c.JSON(http.StatusOK, gin.H{
		"message":                    "Approval processed successfully",
		"accounting_sync_status":     syncResult.Status,
		"accounting_idempotency_key": syncResult.IdempotencyKey,
		"accounting_journal_id":      syncResult.JournalID,
		"accounting_already_posted":  syncResult.AlreadyPosted,
		"data":                       requestData,
	})
}

func RejectPOSApprovalRequest(c *gin.Context) {
	var input reviewPOSApprovalRequestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reviewNote := strings.TrimSpace(input.Note)
	if reviewNote == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "review note is required when rejecting a request"})
		return
	}

	if err := services.EnsurePOSApprovalSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	actor := readApprovalActorSnapshot(c)
	requestID, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || requestID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid approval request id"})
		return
	}

	var request models.POSApprovalRequest
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&request, requestID).Error; err != nil {
			return err
		}
		if request.Status != services.POSApprovalStatusPending {
			return fmt.Errorf("approval request has already been reviewed")
		}
		beforeRequest := request

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
		return recordDocumentAudit(tx, c, documentAuditInput{
			OutletID:     request.OutletID,
			DocumentType: "pos_approval_request",
			DocumentID:   request.ID,
			Action:       documentAuditActionRejected,
			Summary:      fmt.Sprintf("Approval request #%d ditolak", request.ID),
			Note:         reviewNote,
			Metadata: gin.H{
				"request_type":   request.RequestType,
				"transaction_id": request.TransactionID,
			},
			Before: buildApprovalResponse(beforeRequest, nil, nil),
			After:  buildApprovalResponse(request, nil, nil),
		})
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Approval request rejected",
		"data":    buildApprovalResponse(request, nil, nil),
	})
}
