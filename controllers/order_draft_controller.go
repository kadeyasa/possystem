package controllers

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/services"
	"gorm.io/gorm"
)

type orderDraftItemInput struct {
	ProductID   uint    `json:"product_id"`
	VariantID   *int64  `json:"variant_id"`
	ProductName string  `json:"product_name"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Total       float64 `json:"total"`
}

type orderDraftInput struct {
	ID                      uint                  `json:"id"`
	OutletID                uint                  `json:"outlet_id"`
	TransactionID           *uint                 `json:"transaction_id"`
	CashierID               uint                  `json:"cashier_id"`
	CashierName             string                `json:"cashier_name"`
	CustomerName            string                `json:"customer_name"`
	CustomerPhone           string                `json:"customer_phone"`
	OrderLabel              string                `json:"order_label"`
	TableLabel              string                `json:"table_label"`
	ServiceMode             string                `json:"service_mode"`
	Source                  string                `json:"source"`
	PaymentMethod           string                `json:"payment_method"`
	PaymentStatus           string                `json:"payment_status"`
	PaymentGatewayProvider  string                `json:"payment_gateway_provider"`
	PaymentGatewayReference string                `json:"payment_gateway_reference"`
	FulfillmentStatus       string                `json:"fulfillment_status"`
	Note                    string                `json:"note"`
	Subtotal                float64               `json:"subtotal"`
	DiscountPercent         float64               `json:"discount_percent"`
	Discount                float64               `json:"discount"`
	TaxPercent              float64               `json:"tax_percent"`
	Tax                     float64               `json:"tax"`
	Total                   float64               `json:"total"`
	Items                   []orderDraftItemInput `json:"items"`
}

type orderDraftStatusInput struct {
	TransactionID uint `json:"transaction_id"`
}

type publicMenuCatalogResponse struct {
	OutletID       uint                        `json:"outlet_id"`
	Categories     []models.Category           `json:"categories"`
	Products       []models.Product            `json:"products"`
	OccupiedTables []publicRestaurantTableSlot `json:"occupied_tables"`
}

type publicRestaurantTableSlot struct {
	Value             string `json:"value"`
	Label             string `json:"label"`
	FulfillmentStatus string `json:"fulfillment_status,omitempty"`
	OrderLabel        string `json:"order_label,omitempty"`
}

type publicPaymentGatewayResponse struct {
	Provider    string `json:"provider,omitempty"`
	Status      string `json:"status"`
	Reference   string `json:"reference,omitempty"`
	CheckoutURL string `json:"checkout_url,omitempty"`
	Message     string `json:"message,omitempty"`
}

const kitchenEmployeeRoleCode = 3

func isRestaurantOrderDraftServiceMode(value string) bool {
	return services.NormalizePOSOrderDraftServiceMode(value) != services.POSOrderDraftServiceModeLaundryHold
}

func isKitchenManagedOrderDraftStatus(value string) bool {
	status := services.NormalizePOSOrderDraftFulfillmentStatus(value)
	return status == services.POSOrderDraftFulfillmentStatusQueuedKitchen ||
		status == services.POSOrderDraftFulfillmentStatusInKitchen ||
		status == services.POSOrderDraftFulfillmentStatusReady
}

func readPOSRoleCode(c *gin.Context) int {
	rawValue, exists := c.Get("role_code")
	if !exists {
		return 0
	}

	switch typedValue := rawValue.(type) {
	case int:
		return typedValue
	case int64:
		return int(typedValue)
	case float64:
		return int(typedValue)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typedValue))
		if err == nil {
			return parsed
		}
	}

	return 0
}

func readPOSRoleText(c *gin.Context) string {
	for _, key := range []string{"jabatan", "role"} {
		if rawValue, exists := c.Get(key); exists {
			if value, ok := rawValue.(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}

	return ""
}

func isKitchenPOSActor(c *gin.Context) bool {
	if readPOSRoleCode(c) == kitchenEmployeeRoleCode {
		return true
	}

	roleText := strings.ToLower(strings.TrimSpace(readPOSRoleText(c)))
	return strings.Contains(roleText, "kitchen") ||
		strings.Contains(roleText, "dapur") ||
		strings.Contains(roleText, "chef") ||
		strings.Contains(roleText, "cook")
}

func parseOrderDraftFulfillmentStatuses(value string) []string {
	parts := strings.Split(value, ",")
	statuses := make([]string, 0, len(parts))
	seen := map[string]struct{}{}

	for _, part := range parts {
		normalized := services.NormalizePOSOrderDraftFulfillmentStatus(part)
		if normalized == services.POSOrderDraftFulfillmentStatusNotRequired {
			if strings.TrimSpace(part) != "" {
				continue
			}
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		statuses = append(statuses, normalized)
	}

	return statuses
}

func clampOrderDraftAmount(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}

func buildOrderDraftItems(inputItems []orderDraftItemInput) ([]models.POSOrderDraftItem, float64, error) {
	items := make([]models.POSOrderDraftItem, 0, len(inputItems))
	subtotal := 0.0

	for _, inputItem := range inputItems {
		if inputItem.ProductID == 0 || inputItem.Quantity <= 0 {
			continue
		}

		total := inputItem.Total
		if total <= 0 {
			total = float64(inputItem.Quantity) * inputItem.UnitPrice
		}
		total = clampOrderDraftAmount(total)
		subtotal += total

		productName := strings.TrimSpace(inputItem.ProductName)
		if productName == "" {
			productName = fmt.Sprintf("Produk #%d", inputItem.ProductID)
		}

		items = append(items, models.POSOrderDraftItem{
			ProductID:   inputItem.ProductID,
			VariantID:   inputItem.VariantID,
			ProductName: productName,
			Quantity:    inputItem.Quantity,
			UnitPrice:   clampOrderDraftAmount(inputItem.UnitPrice),
			Total:       total,
		})
	}

	if len(items) == 0 {
		return nil, 0, fmt.Errorf("minimal satu item wajib dipilih")
	}

	return items, subtotal, nil
}

func resolveOrderDraftCashierName(c *gin.Context, input orderDraftInput) string {
	if providedName := strings.TrimSpace(input.CashierName); providedName != "" {
		return providedName
	}

	actor := readApprovalActorSnapshot(c)
	if !isApprovalActorPlaceholderName(actor.Name) {
		return strings.TrimSpace(actor.Name)
	}

	if input.CashierID > 0 {
		return fmt.Sprintf("Kasir #%d", input.CashierID)
	}

	return "Kasir Aktif"
}

func buildOrderDraftLabel(input orderDraftInput, source string) string {
	if label := strings.TrimSpace(input.OrderLabel); label != "" {
		return label
	}
	if tableLabel := strings.TrimSpace(input.TableLabel); tableLabel != "" {
		return fmt.Sprintf("Meja %s", tableLabel)
	}
	if customerName := strings.TrimSpace(input.CustomerName); customerName != "" {
		return fmt.Sprintf("Pesanan %s", customerName)
	}
	if source == services.POSOrderDraftSourceGuestSelfOrder {
		return "Self Order Tamu"
	}
	return "Pesanan Meja"
}

func normalizeRestaurantTableLabel(value string) string {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return ""
	}

	lowerValue := strings.ToLower(trimmedValue)
	if strings.HasPrefix(lowerValue, "meja ") {
		withoutPrefix := strings.TrimSpace(trimmedValue[5:])
		if withoutPrefix != "" {
			return withoutPrefix
		}
	}

	return trimmedValue
}

func formatRestaurantTableLabel(value string) string {
	trimmedValue := normalizeRestaurantTableLabel(value)
	if trimmedValue == "" {
		return ""
	}

	if _, err := strconv.Atoi(trimmedValue); err == nil {
		return fmt.Sprintf("Meja %s", trimmedValue)
	}

	lowerValue := strings.ToLower(trimmedValue)
	if strings.HasPrefix(lowerValue, "meja ") {
		return trimmedValue
	}

	return fmt.Sprintf("Meja %s", trimmedValue)
}

func loadPublicOccupiedTables(tx *gorm.DB, outletID uint) ([]publicRestaurantTableSlot, error) {
	var drafts []models.POSOrderDraft
	if err := tx.
		Model(&models.POSOrderDraft{}).
		Select("id", "table_label", "order_label", "fulfillment_status", "updated_at", "created_at").
		Where("outlet_id = ?", outletID).
		Where("status = ?", services.POSOrderDraftStatusOpen).
		Where("service_mode <> ?", services.POSOrderDraftServiceModeLaundryHold).
		Where("COALESCE(BTRIM(table_label), '') <> ''").
		Order("updated_at DESC, id DESC").
		Find(&drafts).Error; err != nil {
		return nil, err
	}

	tableSlotsByValue := make(map[string]publicRestaurantTableSlot)
	for _, draft := range drafts {
		normalizedValue := normalizeRestaurantTableLabel(draft.TableLabel)
		if normalizedValue == "" {
			continue
		}

		if _, exists := tableSlotsByValue[normalizedValue]; exists {
			continue
		}

		tableSlotsByValue[normalizedValue] = publicRestaurantTableSlot{
			Value:             normalizedValue,
			Label:             formatRestaurantTableLabel(normalizedValue),
			FulfillmentStatus: strings.TrimSpace(draft.FulfillmentStatus),
			OrderLabel:        strings.TrimSpace(draft.OrderLabel),
		}
	}

	tableSlots := make([]publicRestaurantTableSlot, 0, len(tableSlotsByValue))
	for _, slot := range tableSlotsByValue {
		tableSlots = append(tableSlots, slot)
	}

	return tableSlots, nil
}

func isRestaurantTableCurrentlyOccupied(tx *gorm.DB, outletID uint, tableLabel string, excludeDraftID uint) (bool, error) {
	normalizedTable := normalizeRestaurantTableLabel(tableLabel)
	if normalizedTable == "" {
		return false, nil
	}

	var drafts []models.POSOrderDraft
	if err := tx.
		Model(&models.POSOrderDraft{}).
		Select("id", "table_label").
		Where("outlet_id = ?", outletID).
		Where("status = ?", services.POSOrderDraftStatusOpen).
		Where("service_mode <> ?", services.POSOrderDraftServiceModeLaundryHold).
		Where("COALESCE(BTRIM(table_label), '') <> ''").
		Find(&drafts).Error; err != nil {
		return false, err
	}

	for _, draft := range drafts {
		if excludeDraftID > 0 && draft.ID == excludeDraftID {
			continue
		}
		if normalizeRestaurantTableLabel(draft.TableLabel) == normalizedTable {
			return true, nil
		}
	}

	return false, nil
}

func preloadOrderDraft(tx *gorm.DB, draftID uint) (models.POSOrderDraft, error) {
	var draft models.POSOrderDraft
	err := tx.Preload("Items").First(&draft, draftID).Error
	return draft, err
}

func buildPublicGatewayReference(draft models.POSOrderDraft) string {
	return fmt.Sprintf("RESTO-%d-%d", draft.OutletID, draft.ID)
}

func buildPublicGatewayCheckoutURL(template string, draft models.POSOrderDraft, reference string) string {
	replacer := strings.NewReplacer(
		"{ORDER_ID}", strconv.FormatUint(uint64(draft.ID), 10),
		"{OUTLET_ID}", strconv.FormatUint(uint64(draft.OutletID), 10),
		"{AMOUNT}", strconv.FormatFloat(draft.Total, 'f', 0, 64),
		"{REFERENCE}", url.QueryEscape(reference),
		"{CUSTOMER_NAME}", url.QueryEscape(strings.TrimSpace(draft.CustomerName)),
		"{TABLE_LABEL}", url.QueryEscape(strings.TrimSpace(draft.TableLabel)),
	)

	return replacer.Replace(template)
}

func preparePublicOrderPayment(input *orderDraftInput) (string, *publicPaymentGatewayResponse) {
	paymentMethod := services.NormalizePOSOrderDraftPaymentMethod(input.PaymentMethod)
	if paymentMethod == services.POSOrderDraftPaymentMethodUnassigned {
		paymentMethod = services.POSOrderDraftPaymentMethodCash
	}

	input.PaymentMethod = paymentMethod
	input.PaymentGatewayProvider = strings.TrimSpace(input.PaymentGatewayProvider)
	input.PaymentGatewayReference = strings.TrimSpace(input.PaymentGatewayReference)

	if paymentMethod == services.POSOrderDraftPaymentMethodQris {
		checkoutURLTemplate := strings.TrimSpace(os.Getenv("PUBLIC_QRIS_CHECKOUT_URL_TEMPLATE"))
		if input.PaymentGatewayProvider == "" {
			input.PaymentGatewayProvider = strings.TrimSpace(os.Getenv("PUBLIC_QRIS_PROVIDER"))
		}
		if input.PaymentGatewayProvider == "" {
			input.PaymentGatewayProvider = "qris_gateway"
		}
		if checkoutURLTemplate == "" {
			input.PaymentStatus = services.POSOrderDraftPaymentStatusPendingConfiguration
			return "", &publicPaymentGatewayResponse{
				Provider: input.PaymentGatewayProvider,
				Status:   services.POSOrderDraftPaymentStatusPendingConfiguration,
				Message:  "Gateway QRIS belum dikonfigurasi. Pesanan tetap tersimpan agar bisa ditindaklanjuti kasir.",
			}
		}

		input.PaymentStatus = services.POSOrderDraftPaymentStatusPendingGateway
		return checkoutURLTemplate, &publicPaymentGatewayResponse{
			Provider: input.PaymentGatewayProvider,
			Status:   services.POSOrderDraftPaymentStatusPendingGateway,
			Message:  "Link QRIS siap dibuat setelah pesanan disimpan.",
		}
	}

	input.PaymentStatus = services.POSOrderDraftPaymentStatusAwaitingCash
	input.PaymentGatewayProvider = ""
	input.PaymentGatewayReference = ""
	return "", nil
}

func saveOrderDraftRecord(tx *gorm.DB, existingID uint, input orderDraftInput, cashierName, source string) (models.POSOrderDraft, error) {
	items, computedSubtotal, err := buildOrderDraftItems(input.Items)
	if err != nil {
		return models.POSOrderDraft{}, err
	}

	subtotal := computedSubtotal
	if input.Subtotal > 0 {
		subtotal = clampOrderDraftAmount(input.Subtotal)
	}

	discountPercent := clampOrderDraftAmount(input.DiscountPercent)
	discount := clampOrderDraftAmount(input.Discount)
	if discount == 0 && discountPercent > 0 {
		discount = subtotal * (discountPercent / 100.0)
	}

	taxPercent := clampOrderDraftAmount(input.TaxPercent)
	tax := clampOrderDraftAmount(input.Tax)
	if tax == 0 && taxPercent > 0 {
		tax = subtotal * (taxPercent / 100.0)
	}

	total := clampOrderDraftAmount(input.Total)
	if total == 0 {
		total = subtotal - discount + tax
	}

	orderDraft := models.POSOrderDraft{}
	if existingID > 0 {
		if err := tx.First(&orderDraft, existingID).Error; err != nil {
			return models.POSOrderDraft{}, err
		}
		if orderDraft.Status != services.POSOrderDraftStatusOpen {
			return models.POSOrderDraft{}, fmt.Errorf("draft order sudah tidak aktif")
		}
		if strings.TrimSpace(orderDraft.Source) != "" {
			source = orderDraft.Source
		}
	}

	paymentMethod := services.NormalizePOSOrderDraftPaymentMethod(input.PaymentMethod)
	paymentStatus := services.NormalizePOSOrderDraftPaymentStatus(input.PaymentStatus)
	paymentGatewayProvider := strings.TrimSpace(input.PaymentGatewayProvider)
	paymentGatewayReference := strings.TrimSpace(input.PaymentGatewayReference)
	fulfillmentStatus := services.NormalizePOSOrderDraftFulfillmentStatus(input.FulfillmentStatus)

	if existingID > 0 {
		if paymentMethod == services.POSOrderDraftPaymentMethodUnassigned && strings.TrimSpace(orderDraft.PaymentMethod) != "" {
			paymentMethod = orderDraft.PaymentMethod
		}
		if paymentStatus == services.POSOrderDraftPaymentStatusDraft && strings.TrimSpace(orderDraft.PaymentStatus) != "" {
			paymentStatus = orderDraft.PaymentStatus
		}
		if paymentGatewayProvider == "" {
			paymentGatewayProvider = strings.TrimSpace(orderDraft.PaymentGatewayProvider)
		}
		if paymentGatewayReference == "" {
			paymentGatewayReference = strings.TrimSpace(orderDraft.PaymentGatewayReference)
		}
		if fulfillmentStatus == services.POSOrderDraftFulfillmentStatusNotRequired && strings.TrimSpace(orderDraft.FulfillmentStatus) != "" {
			fulfillmentStatus = orderDraft.FulfillmentStatus
		}
	}

	paymentStatus = services.ResolvePOSOrderDraftPaymentStatus(paymentMethod, paymentStatus)

	orderDraft.OutletID = input.OutletID
	orderDraft.TransactionID = input.TransactionID
	orderDraft.CashierID = input.CashierID
	orderDraft.CashierName = cashierName
	orderDraft.CustomerName = strings.TrimSpace(input.CustomerName)
	orderDraft.CustomerPhone = strings.TrimSpace(input.CustomerPhone)
	orderDraft.OrderLabel = buildOrderDraftLabel(input, source)
	orderDraft.TableLabel = strings.TrimSpace(input.TableLabel)
	orderDraft.ServiceMode = services.NormalizePOSOrderDraftServiceMode(input.ServiceMode)
	orderDraft.Source = services.NormalizePOSOrderDraftSource(source)
	orderDraft.Status = services.POSOrderDraftStatusOpen
	fulfillmentStatus = services.ResolvePOSOrderDraftFulfillmentStatus(orderDraft.ServiceMode, fulfillmentStatus)
	orderDraft.FulfillmentStatus = fulfillmentStatus
	orderDraft.PaymentMethod = paymentMethod
	orderDraft.PaymentStatus = paymentStatus
	orderDraft.PaymentGatewayProvider = paymentGatewayProvider
	orderDraft.PaymentGatewayReference = paymentGatewayReference
	orderDraft.Note = strings.TrimSpace(input.Note)
	orderDraft.Subtotal = subtotal
	orderDraft.DiscountPercent = discountPercent
	orderDraft.Discount = discount
	orderDraft.TaxPercent = taxPercent
	orderDraft.Tax = tax
	orderDraft.Total = total

	if existingID > 0 {
		if err := tx.Model(&orderDraft).Select(
			"OutletID",
			"TransactionID",
			"CashierID",
			"CashierName",
			"CustomerName",
			"CustomerPhone",
			"OrderLabel",
			"TableLabel",
			"ServiceMode",
			"Source",
			"Status",
			"FulfillmentStatus",
			"PaymentMethod",
			"PaymentStatus",
			"PaymentGatewayProvider",
			"PaymentGatewayReference",
			"Note",
			"Subtotal",
			"DiscountPercent",
			"Discount",
			"TaxPercent",
			"Tax",
			"Total",
		).Updates(&orderDraft).Error; err != nil {
			return models.POSOrderDraft{}, err
		}
		if err := tx.Where("order_draft_id = ?", orderDraft.ID).Delete(&models.POSOrderDraftItem{}).Error; err != nil {
			return models.POSOrderDraft{}, err
		}
	} else {
		if err := tx.Create(&orderDraft).Error; err != nil {
			return models.POSOrderDraft{}, err
		}
	}

	for _, item := range items {
		item.OrderDraftID = orderDraft.ID
		if err := tx.Create(&item).Error; err != nil {
			return models.POSOrderDraft{}, err
		}
	}

	return preloadOrderDraft(tx, orderDraft.ID)
}

func CreateOrderDraft(c *gin.Context) {
	var input orderDraftInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.OutletID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outlet_id is required"})
		return
	}
	if err := services.EnsurePOSOrderDraftSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	cashierName := resolveOrderDraftCashierName(c, input)

	var savedDraft models.POSOrderDraft
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		savedDraft, err = saveOrderDraftRecord(tx, input.ID, input, cashierName, services.POSOrderDraftSourceCashierHold)
		return err
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Draft order berhasil disimpan",
		"data":    savedDraft,
	})
}

func GetOrderDrafts(c *gin.Context) {
	if err := services.EnsurePOSOrderDraftSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	query := database.DB.Model(&models.POSOrderDraft{}).Preload("Items")
	if outletID := strings.TrimSpace(c.Query("outlet_id")); outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}
	if status := strings.ToLower(strings.TrimSpace(c.Query("status"))); status != "" && status != "all" {
		query = query.Where("status = ?", services.NormalizePOSOrderDraftStatus(status))
	} else if status == "" {
		query = query.Where("status = ?", services.POSOrderDraftStatusOpen)
	}
	if source := strings.ToLower(strings.TrimSpace(c.Query("source"))); source != "" && source != "all" {
		query = query.Where("source = ?", services.NormalizePOSOrderDraftSource(source))
	}
	if fulfillmentStatusQuery := strings.TrimSpace(c.Query("fulfillment_status")); fulfillmentStatusQuery != "" && fulfillmentStatusQuery != "all" {
		fulfillmentStatuses := parseOrderDraftFulfillmentStatuses(fulfillmentStatusQuery)
		if len(fulfillmentStatuses) == 1 {
			query = query.Where("fulfillment_status = ?", fulfillmentStatuses[0])
		} else if len(fulfillmentStatuses) > 1 {
			query = query.Where("fulfillment_status IN ?", fulfillmentStatuses)
		}
	}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		like := "%" + search + "%"
		query = query.Where(
			"COALESCE(order_label, '') ILIKE ? OR COALESCE(table_label, '') ILIKE ? OR COALESCE(customer_name, '') ILIKE ? OR COALESCE(cashier_name, '') ILIKE ? OR COALESCE(note, '') ILIKE ?",
			like, like, like, like, like,
		)
	}

	var drafts []models.POSOrderDraft
	if err := query.Order("updated_at DESC, id DESC").Find(&drafts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load order drafts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results": drafts,
		"total":   len(drafts),
	})
}

func GetOrderDraftByID(c *gin.Context) {
	if err := services.EnsurePOSOrderDraftSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	draftID, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || draftID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order draft id"})
		return
	}

	var draft models.POSOrderDraft
	if err := database.DB.Preload("Items").First(&draft, draftID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order draft not found"})
		return
	}

	c.JSON(http.StatusOK, draft)
}

func CompleteOrderDraft(c *gin.Context) {
	var input orderDraftStatusInput
	if err := c.ShouldBindJSON(&input); err != nil && strings.TrimSpace(err.Error()) != "EOF" {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := services.EnsurePOSOrderDraftSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	draftID, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || draftID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order draft id"})
		return
	}

	if err := database.DB.Model(&models.POSOrderDraft{}).
		Where("id = ? AND status = ?", draftID, services.POSOrderDraftStatusOpen).
		Updates(map[string]interface{}{
			"status":         services.POSOrderDraftStatusPaid,
			"payment_status": services.POSOrderDraftPaymentStatusPaid,
			"transaction_id": input.TransactionID,
		}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to complete order draft"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Draft order ditandai selesai"})
}

func CancelOrderDraft(c *gin.Context) {
	if err := services.EnsurePOSOrderDraftSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	draftID, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || draftID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order draft id"})
		return
	}

	var draft models.POSOrderDraft
	if err := database.DB.First(&draft, draftID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order draft not found"})
		return
	}
	if draft.Status != services.POSOrderDraftStatusOpen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order draft sudah tidak aktif"})
		return
	}
	if isRestaurantOrderDraftServiceMode(draft.ServiceMode) && isKitchenManagedOrderDraftStatus(draft.FulfillmentStatus) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Pesanan yang sudah masuk kitchen hanya bisa dibatalkan dari kitchen",
		})
		return
	}

	if err := database.DB.Model(&models.POSOrderDraft{}).
		Where("id = ? AND status = ?", draftID, services.POSOrderDraftStatusOpen).
		Updates(map[string]interface{}{
			"status":         services.POSOrderDraftStatusCancelled,
			"payment_status": services.POSOrderDraftPaymentStatusCancelled,
		}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to cancel order draft"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Draft order dibatalkan"})
}

func CancelKitchenOrderDraft(c *gin.Context) {
	if err := services.EnsurePOSOrderDraftSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !isKitchenPOSActor(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Hanya role kitchen yang dapat membatalkan order dari kitchen"})
		return
	}

	draftID, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || draftID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order draft id"})
		return
	}

	var draft models.POSOrderDraft
	if err := database.DB.First(&draft, draftID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order draft not found"})
		return
	}
	if draft.Status != services.POSOrderDraftStatusOpen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order draft sudah tidak aktif"})
		return
	}
	if !isRestaurantOrderDraftServiceMode(draft.ServiceMode) || !isKitchenManagedOrderDraftStatus(draft.FulfillmentStatus) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order ini belum berada di kitchen"})
		return
	}

	if err := database.DB.Model(&models.POSOrderDraft{}).
		Where("id = ? AND status = ?", draftID, services.POSOrderDraftStatusOpen).
		Updates(map[string]interface{}{
			"status":         services.POSOrderDraftStatusCancelled,
			"payment_status": services.POSOrderDraftPaymentStatusCancelled,
		}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to cancel kitchen order"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order kitchen dibatalkan"})
}

func GetKitchenOrderDraftQueue(c *gin.Context) {
	if err := services.EnsurePOSOrderDraftSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	outletID, err := strconv.ParseUint(strings.TrimSpace(c.Query("outlet_id")), 10, 64)
	if err != nil || outletID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outlet_id is required"})
		return
	}

	fulfillmentStatuses := parseOrderDraftFulfillmentStatuses(c.DefaultQuery("fulfillment_status", "queued_kitchen,in_kitchen,ready"))
	if len(fulfillmentStatuses) == 0 {
		fulfillmentStatuses = []string{
			services.POSOrderDraftFulfillmentStatusQueuedKitchen,
			services.POSOrderDraftFulfillmentStatusInKitchen,
			services.POSOrderDraftFulfillmentStatusReady,
		}
	}

	var drafts []models.POSOrderDraft
	if err := database.DB.
		Model(&models.POSOrderDraft{}).
		Preload("Items").
		Where("outlet_id = ?", outletID).
		Where("status = ?", services.POSOrderDraftStatusOpen).
		Where("service_mode <> ?", services.POSOrderDraftServiceModeLaundryHold).
		Where("fulfillment_status IN ?", fulfillmentStatuses).
		Order("COALESCE(sent_to_kitchen_at, updated_at) ASC, id ASC").
		Find(&drafts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load kitchen queue"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results": drafts,
		"total":   len(drafts),
	})
}

func SendOrderDraftToKitchen(c *gin.Context) {
	if err := services.EnsurePOSOrderDraftSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	draftID, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || draftID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order draft id"})
		return
	}

	var draft models.POSOrderDraft
	if err := database.DB.Preload("Items").First(&draft, draftID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order draft not found"})
		return
	}
	if draft.Status != services.POSOrderDraftStatusOpen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order draft sudah tidak aktif"})
		return
	}
	if !isRestaurantOrderDraftServiceMode(draft.ServiceMode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order draft ini tidak memakai alur kitchen"})
		return
	}

	now := time.Now()
	updates := map[string]interface{}{
		"fulfillment_status": services.POSOrderDraftFulfillmentStatusQueuedKitchen,
	}
	if draft.SentToKitchenAt == nil {
		updates["sent_to_kitchen_at"] = now
	}

	if err := database.DB.Model(&models.POSOrderDraft{}).Where("id = ?", draftID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send order to kitchen"})
		return
	}

	updatedDraft, err := preloadOrderDraft(database.DB, uint(draftID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reload order draft"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Pesanan dikirim ke kitchen",
		"data":    updatedDraft,
	})
}

func StartKitchenOrderDraft(c *gin.Context) {
	if err := services.EnsurePOSOrderDraftSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	draftID, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || draftID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order draft id"})
		return
	}

	var draft models.POSOrderDraft
	if err := database.DB.First(&draft, draftID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order draft not found"})
		return
	}
	if draft.Status != services.POSOrderDraftStatusOpen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order draft sudah tidak aktif"})
		return
	}

	now := time.Now()
	updates := map[string]interface{}{
		"fulfillment_status": services.POSOrderDraftFulfillmentStatusInKitchen,
	}
	if draft.SentToKitchenAt == nil {
		updates["sent_to_kitchen_at"] = now
	}
	if draft.KitchenStartedAt == nil {
		updates["kitchen_started_at"] = now
	}

	if err := database.DB.Model(&models.POSOrderDraft{}).Where("id = ?", draftID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update kitchen status"})
		return
	}

	updatedDraft, err := preloadOrderDraft(database.DB, uint(draftID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reload order draft"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Order kitchen dimulai",
		"data":    updatedDraft,
	})
}

func CompleteKitchenOrderDraft(c *gin.Context) {
	if err := services.EnsurePOSOrderDraftSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	draftID, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || draftID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order draft id"})
		return
	}

	var draft models.POSOrderDraft
	if err := database.DB.First(&draft, draftID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order draft not found"})
		return
	}
	if draft.Status != services.POSOrderDraftStatusOpen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order draft sudah tidak aktif"})
		return
	}

	now := time.Now()
	updates := map[string]interface{}{
		"fulfillment_status":   services.POSOrderDraftFulfillmentStatusReady,
		"kitchen_completed_at": now,
	}
	if draft.KitchenStartedAt == nil {
		updates["kitchen_started_at"] = now
	}
	if draft.SentToKitchenAt == nil {
		updates["sent_to_kitchen_at"] = now
	}

	if err := database.DB.Model(&models.POSOrderDraft{}).Where("id = ?", draftID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to complete kitchen order"})
		return
	}

	updatedDraft, err := preloadOrderDraft(database.DB, uint(draftID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reload order draft"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Order kitchen diselesaikan",
		"data":    updatedDraft,
	})
}

func GetPublicMenuCatalog(c *gin.Context) {
	if err := services.EnsurePOSOrderDraftSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	outletID, err := strconv.ParseUint(strings.TrimSpace(c.Query("outlet_id")), 10, 64)
	if err != nil || outletID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outlet_id is required"})
		return
	}

	var categories []models.Category
	if err := database.DB.
		Where("outlet_id = ? AND status = ?", outletID, true).
		Order("category_name ASC").
		Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load menu categories"})
		return
	}

	var products []models.Product
	if err := database.DB.
		Where("outlet_id = ? AND is_active = ? AND item_type IN ?", outletID, true, []string{"resale_item", "finished_good"}).
		Order("name ASC").
		Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load menu products"})
		return
	}

	occupiedTables, err := loadPublicOccupiedTables(database.DB, uint(outletID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load occupied tables"})
		return
	}

	c.JSON(http.StatusOK, publicMenuCatalogResponse{
		OutletID:       uint(outletID),
		Categories:     categories,
		Products:       products,
		OccupiedTables: occupiedTables,
	})
}

func CreatePublicOrderDraft(c *gin.Context) {
	var input orderDraftInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.OutletID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outlet_id is required"})
		return
	}
	if err := services.EnsurePOSOrderDraftSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	input.TableLabel = normalizeRestaurantTableLabel(input.TableLabel)
	input.ServiceMode = services.NormalizePOSOrderDraftServiceMode(input.ServiceMode)
	if input.ServiceMode == services.POSOrderDraftServiceModeDineIn && input.TableLabel == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Pilih nomor meja yang kosong sebelum mengirim pesanan"})
		return
	}

	checkoutURLTemplate, paymentGateway := preparePublicOrderPayment(&input)
	cashierName := "Guest Self Order"
	if strings.TrimSpace(input.CashierName) != "" {
		cashierName = strings.TrimSpace(input.CashierName)
	}

	var savedDraft models.POSOrderDraft
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if input.TableLabel != "" {
			isOccupied, err := isRestaurantTableCurrentlyOccupied(tx, input.OutletID, input.TableLabel, 0)
			if err != nil {
				return err
			}
			if isOccupied {
				return fmt.Errorf("%s sedang dipakai. Silakan pilih meja kosong lain atau hubungi kasir", formatRestaurantTableLabel(input.TableLabel))
			}
		}

		var err error
		savedDraft, err = saveOrderDraftRecord(tx, 0, input, cashierName, services.POSOrderDraftSourceGuestSelfOrder)
		return err
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if services.NormalizePOSOrderDraftPaymentMethod(savedDraft.PaymentMethod) == services.POSOrderDraftPaymentMethodQris {
		reference := buildPublicGatewayReference(savedDraft)
		savedDraft.PaymentGatewayReference = reference
		if paymentGateway == nil {
			paymentGateway = &publicPaymentGatewayResponse{}
		}
		paymentGateway.Reference = reference
		paymentGateway.Provider = savedDraft.PaymentGatewayProvider

		updates := map[string]interface{}{
			"payment_gateway_reference": reference,
		}
		if strings.TrimSpace(savedDraft.PaymentGatewayProvider) != "" {
			updates["payment_gateway_provider"] = savedDraft.PaymentGatewayProvider
		}
		_ = database.DB.Model(&models.POSOrderDraft{}).Where("id = ?", savedDraft.ID).Updates(updates).Error

		if paymentGateway != nil && strings.TrimSpace(checkoutURLTemplate) != "" {
			paymentGateway.CheckoutURL = buildPublicGatewayCheckoutURL(checkoutURLTemplate, savedDraft, reference)
			paymentGateway.Status = services.POSOrderDraftPaymentStatusPendingGateway
			paymentGateway.Message = "Lanjutkan pembayaran QRIS melalui gateway yang sudah dikonfigurasi outlet."
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "Pesanan tamu berhasil dikirim",
		"data":            savedDraft,
		"payment_gateway": paymentGateway,
	})
}
