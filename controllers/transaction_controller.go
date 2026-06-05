package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/services"
	"github.com/kadeyasa/possystem/utils"
	"gorm.io/gorm"
)

type TransactionInput struct {
	OutletID           uint                     `json:"outlet_id"`
	CashierID          uint                     `json:"cashier_id"`
	CashierName        string                   `json:"cashier_name"`
	CustomerID         *uint                    `json:"customer_id"`
	CustomerName       string                   `json:"customer_name"`
	Subtotal           float64                  `json:"subtotal"`
	DiscountPercent    float64                  `json:"discount_percent"`
	Total              float64                  `json:"total"`
	ServicePercent     float64                  `json:"service_percent"`
	Service            float64                  `json:"service"`
	TaxPercent         float64                  `json:"tax_percent"`
	Tax                float64                  `json:"tax"`
	Discount           float64                  `json:"discount"`
	GrandTotal         float64                  `json:"grand_total"`
	PaymentMethod      string                   `json:"payment_method"` // "cash" or "credit"
	RefundCreditCode   string                   `json:"refund_credit_code"`
	RefundCreditAmount float64                  `json:"refund_credit_amount"`
	Note               string                   `json:"note"`
	Status             uint                     `json:"status"`
	Items              []models.TransactionItem `json:"items"`
}

type resolvedTransactionAmounts struct {
	Subtotal        float64
	DiscountPercent float64
	Discount        float64
	ServicePercent  float64
	Service         float64
	TaxPercent      float64
	Tax             float64
	GrandTotal      float64
}

func clampTransactionAmount(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}

func clampTransactionPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func resolveTransactionAmounts(input TransactionInput) resolvedTransactionAmounts {
	subtotal := clampTransactionAmount(input.Subtotal)
	if subtotal == 0 {
		subtotal = clampTransactionAmount(input.Total)
	}

	discountPercent := clampTransactionPercent(input.DiscountPercent)
	discount := clampTransactionAmount(input.Discount)
	if discount == 0 && discountPercent > 0 {
		discount = subtotal * (discountPercent / 100.0)
	}

	netSubtotal := subtotal - discount
	if netSubtotal < 0 {
		netSubtotal = 0
	}

	servicePercent := clampTransactionPercent(input.ServicePercent)
	service := clampTransactionAmount(input.Service)
	if service == 0 && servicePercent > 0 {
		service = netSubtotal * (servicePercent / 100.0)
	}

	taxPercent := clampTransactionPercent(input.TaxPercent)
	tax := clampTransactionAmount(input.Tax)
	if tax == 0 && taxPercent > 0 {
		tax = (netSubtotal + service) * (taxPercent / 100.0)
	}

	grandTotal := clampTransactionAmount(input.GrandTotal)
	if grandTotal == 0 {
		grandTotal = netSubtotal + service + tax
	}

	return resolvedTransactionAmounts{
		Subtotal:        subtotal,
		DiscountPercent: discountPercent,
		Discount:        discount,
		ServicePercent:  servicePercent,
		Service:         service,
		TaxPercent:      taxPercent,
		Tax:             tax,
		GrandTotal:      grandTotal,
	}
}

type transactionApprovalLockSnapshot struct {
	RequestType string
	Status      string
	Message     string
	priority    int
}

func applyNonVoidedTransactionFilter(query *gorm.DB) *gorm.DB {
	return query.Where("COALESCE(document_status, ?) <> ?", services.TransactionDocumentStatusPosted, services.TransactionDocumentStatusVoided)
}

func buildTransactionApprovalLockSnapshot(request models.POSApprovalRequest) (transactionApprovalLockSnapshot, bool) {
	switch {
	case strings.EqualFold(strings.TrimSpace(request.Status), services.POSApprovalStatusPending):
		return transactionApprovalLockSnapshot{
			RequestType: request.RequestType,
			Status:      services.POSApprovalStatusPending,
			Message:     "Transaksi ini sedang menunggu approval.",
			priority:    3,
		}, true
	case strings.EqualFold(strings.TrimSpace(request.Status), services.POSApprovalStatusApproved) &&
		strings.EqualFold(strings.TrimSpace(request.RequestType), services.POSApprovalRequestTypeVoid):
		return transactionApprovalLockSnapshot{
			RequestType: services.POSApprovalRequestTypeVoid,
			Status:      services.POSApprovalStatusApproved,
			Message:     "Transaksi ini sudah di-void lewat approval.",
			priority:    2,
		}, true
	default:
		return transactionApprovalLockSnapshot{}, false
	}
}

func attachRefundableItemsToTransactions(tx *gorm.DB, transactions []models.Transaction) error {
	if len(transactions) == 0 {
		return nil
	}

	transactionIDs := make([]uint, 0, len(transactions))
	for _, transaction := range transactions {
		if transaction.ID > 0 {
			transactionIDs = append(transactionIDs, transaction.ID)
		}
	}
	if len(transactionIDs) == 0 {
		return nil
	}

	type refundedQtyRow struct {
		TransactionID uint
		ProductID     uint
		Quantity      int
	}

	var refundedRows []refundedQtyRow
	if err := tx.Table("tblrefund_items AS ri").
		Select("r.transaction_id AS transaction_id, ri.product_id AS product_id, COALESCE(SUM(ri.quantity), 0) AS quantity").
		Joins("JOIN tblrefunds AS r ON r.id = ri.refund_id").
		Where("r.transaction_id IN ?", transactionIDs).
		Group("r.transaction_id, ri.product_id").
		Scan(&refundedRows).Error; err != nil {
		return err
	}

	refundedMap := make(map[string]int, len(refundedRows))
	for _, row := range refundedRows {
		refundedMap[fmt.Sprintf("%d:%d", row.TransactionID, row.ProductID)] = row.Quantity
	}

	for index := range transactions {
		type soldAggregate struct {
			productID   uint
			productName string
			quantity    int
			total       float64
			unitPrice   float64
		}

		grouped := make(map[uint]*soldAggregate)
		order := make([]uint, 0)
		for _, item := range transactions[index].Items {
			entry, exists := grouped[item.ProductID]
			if !exists {
				entry = &soldAggregate{
					productID:   item.ProductID,
					productName: strings.TrimSpace(item.Product.Name),
				}
				if entry.productName == "" {
					entry.productName = fmt.Sprintf("Product #%d", item.ProductID)
				}
				grouped[item.ProductID] = entry
				order = append(order, item.ProductID)
			}
			entry.quantity += item.Quantity
			entry.total += item.Total
			if entry.quantity > 0 {
				entry.unitPrice = entry.total / float64(entry.quantity)
			}
		}

		summaries := make([]models.RefundableItemSummary, 0, len(order))
		for _, productID := range order {
			entry := grouped[productID]
			refundedQty := refundedMap[fmt.Sprintf("%d:%d", transactions[index].ID, productID)]
			refundableQty := entry.quantity - refundedQty
			if refundableQty < 0 {
				refundableQty = 0
			}
			summaries = append(summaries, models.RefundableItemSummary{
				ProductID:          productID,
				ProductName:        entry.productName,
				SoldQuantity:       entry.quantity,
				RefundedQuantity:   refundedQty,
				RefundableQuantity: refundableQty,
				UnitPrice:          entry.unitPrice,
				Total:              entry.total,
			})
		}
		transactions[index].RefundableItems = summaries
	}

	return nil
}

func attachApprovalLocksToTransactions(tx *gorm.DB, transactions []models.Transaction) error {
	if len(transactions) == 0 {
		return nil
	}

	transactionIDs := make([]uint, 0, len(transactions))
	for _, transaction := range transactions {
		if transaction.ID > 0 {
			transactionIDs = append(transactionIDs, transaction.ID)
		}
	}
	if len(transactionIDs) == 0 {
		return nil
	}

	var requests []models.POSApprovalRequest
	if err := tx.
		Where("transaction_id IN ?", transactionIDs).
		Where("status IN ?", []string{services.POSApprovalStatusPending, services.POSApprovalStatusApproved}).
		Order("requested_at DESC, id DESC").
		Find(&requests).Error; err != nil {
		return err
	}

	lockByTransactionID := make(map[uint]transactionApprovalLockSnapshot)
	for _, request := range requests {
		candidate, shouldLock := buildTransactionApprovalLockSnapshot(request)
		if !shouldLock {
			continue
		}

		existing, exists := lockByTransactionID[request.TransactionID]
		if !exists || candidate.priority > existing.priority {
			lockByTransactionID[request.TransactionID] = candidate
		}
	}

	for index := range transactions {
		lock, exists := lockByTransactionID[transactions[index].ID]
		if !exists {
			continue
		}

		transactions[index].ApprovalLocked = true
		transactions[index].ApprovalLockType = lock.RequestType
		transactions[index].ApprovalLockStatus = lock.Status
		transactions[index].ApprovalLockMessage = lock.Message
	}

	return nil
}

func parseTransactionDateOnly(value string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", strings.TrimSpace(value), time.Local)
}

func applyTransactionDateFilters(query *gorm.DB, startDate, endDate string) *gorm.DB {
	if parsed, err := parseTransactionDateOnly(startDate); err == nil {
		query = query.Where("created_at >= ?", parsed)
	}
	if parsed, err := parseTransactionDateOnly(endDate); err == nil {
		query = query.Where("created_at < ?", parsed.AddDate(0, 0, 1))
	}
	return query
}

// GET: /transactions
func GetAllTransactions(c *gin.Context) {
	if err := services.EnsureAccountingSyncSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := services.EnsureRefundSettlementSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := services.EnsurePOSApprovalSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var transactions []models.Transaction
	query := database.DB.
		Preload("Items.Product").
		Preload("Items.Variant").
		Model(&models.Transaction{})

	if strings.ToLower(strings.TrimSpace(c.Query("include_voided"))) != "true" {
		query = applyNonVoidedTransactionFilter(query)
	}
	if outletID := strings.TrimSpace(c.Query("outlet_id")); outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}
	query = applyTransactionDateFilters(query, c.Query("start_date"), c.Query("end_date"))
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		like := "%" + search + "%"
		query = query.Where(
			"CAST(id AS TEXT) ILIKE ? OR COALESCE(cashier_name, '') ILIKE ? OR COALESCE(customer_name, '') ILIKE ? OR COALESCE(payment_method, '') ILIKE ? OR COALESCE(note, '') ILIKE ?",
			like, like, like, like, like,
		)
	}

	if err := query.Order("created_at desc, id desc").Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := attachRefundableItemsToTransactions(database.DB, transactions); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if strings.EqualFold(strings.TrimSpace(c.Query("include_approval_locks")), "true") {
		if err := attachApprovalLocksToTransactions(database.DB, transactions); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, transactions)
}

// GET: /transactions/:id
func GetTransactionByID(c *gin.Context) {
	if err := services.EnsureAccountingSyncSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	id := c.Param("id")
	var transaction models.Transaction

	if err := database.DB.
		Preload("Items.Product").
		Preload("Items.Variant"). // <- ini penting
		First(&transaction, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	}

	c.JSON(http.StatusOK, transaction)
}

// GET: /transactions/daily?date=2025-07-01
func GetTransactionsDaily(c *gin.Context) {
	if err := services.EnsureAccountingSyncSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := services.EnsurePOSApprovalSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	dateParam := c.Query("date")
	OutletId := c.Query("outlet_id")
	date, err := parseTransactionDateOnly(dateParam)
	if err != nil {
		utils.Log.Warnf("❌ Bind JSON failed: %v", date)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format. Use YYYY-MM-DD"})
		return
	}
	if OutletId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Outlet ID"})
		return
	}
	start := date
	end := date.AddDate(0, 0, 1)

	var transactions []models.Transaction
	if err := applyNonVoidedTransactionFilter(database.DB.Where("created_at >= ? AND created_at < ? AND outlet_id = ?", start, end, OutletId)).
		Preload("Items").
		Preload("Items.Product").
		Preload("Items.Variant").
		Order("created_at desc").
		Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, transactions)
}

// GET: /transactions/weekly?date=2025-07-01
func GetTransactionsWeekly(c *gin.Context) {
	if err := services.EnsureAccountingSyncSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := services.EnsurePOSApprovalSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	dateParam := c.Query("date")
	OutletId := c.Query("outlet_id")
	date, err := parseTransactionDateOnly(dateParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format. Use YYYY-MM-DD"})
		return
	}
	if OutletId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Outlet ID"})
		return
	}
	// Awal minggu = Senin
	offset := int(date.Weekday())
	if offset == 0 {
		offset = 6
	} else {
		offset--
	}
	start := date.AddDate(0, 0, -offset)
	end := start.AddDate(0, 0, 7)

	var transactions []models.Transaction
	if err := applyNonVoidedTransactionFilter(database.DB.Where("created_at >= ? AND created_at < ? AND outlet_id = ?", start, end, OutletId)).
		Preload("Items").
		Preload("Items.Product").
		Preload("Items.Variant").
		Order("created_at desc").
		Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, transactions)
}

// GET: /transactions/monthly?year=2025&month=7
func GetTransactionsMonthly(c *gin.Context) {
	if err := services.EnsureAccountingSyncSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := services.EnsurePOSApprovalSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	year := c.Query("year")
	month := c.Query("month")
	OutletId := c.Query("outlet_id")
	start, err := time.ParseInLocation("2006-1-2", year+"-"+month+"-1", time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid year/month format"})
		return
	}
	end := start.AddDate(0, 1, 0)
	if OutletId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Outlet ID"})
		return
	}
	var transactions []models.Transaction
	if err := applyNonVoidedTransactionFilter(database.DB.Where("created_at >= ? AND created_at < ? AND outlet_id = ?", start, end, OutletId)).
		Preload("Items").
		Preload("Items.Product").
		Preload("Items.Variant").
		Order("created_at desc").
		Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, transactions)
}

func CreateTransaction(c *gin.Context) {
	var input TransactionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Log.Warnf("❌ Bind JSON failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if strings.EqualFold(strings.TrimSpace(input.PaymentMethod), "bayarnanti") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "payment_method bayarnanti must be saved as draft first and cannot post accounting directly",
		})
		return
	}

	if services.NormalizeSettlementPurpose(input.PaymentMethod) == "receivable" &&
		(input.CustomerID == nil || *input.CustomerID == 0) &&
		strings.TrimSpace(input.RefundCreditCode) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "customer_id is required for hutang customer transactions",
		})
		return
	}

	if err := services.EnsureAccountingSyncSchema(database.DB); err != nil {
		utils.Log.Errorf("❌ Failed to ensure accounting sync schema: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare accounting sync schema"})
		return
	}
	if err := services.EnsureInventorySchema(database.DB); err != nil {
		utils.Log.Errorf("❌ Failed to ensure inventory schema: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare inventory schema"})
		return
	}
	if err := services.EnsureRefundSettlementSchema(database.DB); err != nil {
		utils.Log.Errorf("❌ Failed to ensure refund settlement schema: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare refund settlement schema"})
		return
	}
	if err := services.EnsurePOSApprovalSchema(database.DB); err != nil {
		utils.Log.Errorf("❌ Failed to ensure POS approval schema: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare POS approval schema"})
		return
	}

	var (
		transaction models.Transaction
		journal     models.JournalEntry
	)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		amounts := resolveTransactionAmounts(input)
		cashierName := strings.TrimSpace(input.CashierName)
		if cashierName == "" && input.CashierID > 0 {
			cashierName = fmt.Sprintf("Cashier #%d", input.CashierID)
		}

		refundCreditCode := strings.TrimSpace(input.RefundCreditCode)
		refundCreditAmount := clampTransactionAmount(input.RefundCreditAmount)
		if refundCreditCode == "" {
			refundCreditAmount = 0
		}
		if refundCreditAmount > amounts.GrandTotal {
			return fmt.Errorf(
				"refund credit amount %.2f cannot exceed grand total %.2f",
				refundCreditAmount,
				amounts.GrandTotal,
			)
		}
		paymentSettlementAmount := amounts.GrandTotal - refundCreditAmount
		if paymentSettlementAmount < 0 {
			paymentSettlementAmount = 0
		}

		effectivePaymentMethod := strings.TrimSpace(input.PaymentMethod)
		if refundCreditAmount > 0 && paymentSettlementAmount <= 0 {
			effectivePaymentMethod = "refund_credit"
		}
		if refundCreditAmount > 0 &&
			paymentSettlementAmount > 0 &&
			services.NormalizeSettlementPurpose(effectivePaymentMethod) == "return_credit" {
			return fmt.Errorf("refund credit cannot be used as the additional payment method when there is remaining balance")
		}

		var lockedRefundCredit *models.RefundStoreCredit
		resolvedCustomerID := input.CustomerID
		if refundCreditAmount > 0 {
			var err error
			lockedRefundCredit, err = services.LockActiveRefundStoreCreditByCode(tx, input.OutletID, refundCreditCode)
			if err != nil {
				return err
			}
			if refundCreditAmount > lockedRefundCredit.RemainingAmount {
				return fmt.Errorf(
					"refund store credit %s remaining balance is %.2f",
					lockedRefundCredit.CreditCode,
					lockedRefundCredit.RemainingAmount,
				)
			}
			if resolvedCustomerID == nil && lockedRefundCredit.CustomerID != nil && *lockedRefundCredit.CustomerID > 0 {
				resolvedCustomerID = lockedRefundCredit.CustomerID
			}
		}

		customerName := strings.TrimSpace(input.CustomerName)
		if resolvedCustomerID != nil && *resolvedCustomerID > 0 {
			var customer models.Customer
			if err := tx.
				Where("id = ? AND outlet_id = ?", *resolvedCustomerID, input.OutletID).
				Take(&customer).Error; err == nil {
				customerName = strings.TrimSpace(customer.Name)
			}
		}
		if lockedRefundCredit != nil {
			if lockedRefundCredit.CustomerID != nil && *lockedRefundCredit.CustomerID > 0 {
				if resolvedCustomerID == nil || *resolvedCustomerID == 0 || *resolvedCustomerID != *lockedRefundCredit.CustomerID {
					return fmt.Errorf("refund store credit %s is tied to a different customer", lockedRefundCredit.CreditCode)
				}
			}
			if customerName == "" {
				customerName = strings.TrimSpace(lockedRefundCredit.CustomerName)
			}
		}
		if services.NormalizeSettlementPurpose(effectivePaymentMethod) == "receivable" &&
			(resolvedCustomerID == nil || *resolvedCustomerID == 0) {
			return fmt.Errorf("customer_id is required for hutang customer transactions")
		}

		salePlan, err := services.PrepareSaleInventoryPlan(tx, input.OutletID, input.Items)
		if err != nil {
			return err
		}
		lockedProducts := salePlan.LockedProducts

		salesAccount, err := services.ResolveAccountForOutlet(tx, input.OutletID, "sale", "sales")
		if err != nil {
			return err
		}
		var cashAccount models.Account
		if paymentSettlementAmount > 0 {
			cashAccount, err = services.ResolveSaleSettlementAccount(tx, input.OutletID, effectivePaymentMethod)
			if err != nil {
				return err
			}
		}
		var refundCreditAccount models.Account
		if refundCreditAmount > 0 {
			refundCreditAccount, err = services.ResolveRefundCreditLiabilityAccount(tx, input.OutletID)
			if err != nil {
				return err
			}
		}

		var taxAccount, serviceAccount, discountAccount, cogsAccount, inventoryAccount models.Account
		hasTax := amounts.Tax > 0
		hasService := amounts.Service > 0
		hasDiscount := amounts.Discount > 0
		hasCostJournal := false
		costOfGoods := 0.0

		if hasTax {
			taxAccount, err = services.ResolveAccountForOutlet(tx, input.OutletID, "sale", "tax")
			if err != nil {
				return err
			}
		}
		if hasService {
			serviceAccount, err = services.ResolveAccountForOutlet(tx, input.OutletID, "sale", "service")
			if err != nil {
				return err
			}
		}
		if hasDiscount {
			discountAccount, err = services.ResolveAccountForOutlet(tx, input.OutletID, "sale", "discount")
			if err != nil {
				return err
			}
		}

		costOfGoods = salePlan.TotalCost
		if costOfGoods > 0 {
			cogsAccount, hasCostJournal, err = services.TryResolveAccountForOutlet(tx, input.OutletID, "sale", "cogs")
			if err != nil {
				return err
			}
			if hasCostJournal {
				inventoryAccount, hasCostJournal, err = services.TryResolveAccountForOutlet(tx, input.OutletID, "purchase", "inventory")
				if err != nil {
					return err
				}
			}
		}

		journal = models.JournalEntry{
			OutletID:    input.OutletID,
			Reference:   "Revenue",
			Description: "Penjualan oleh kasir",
			EntryDate:   time.Now(),
		}

		lines := []models.JournalLine{
			{
				AccountID:   salesAccount.ID,
				Debit:       0,
				Credit:      amounts.Subtotal,
				Description: "Penjualan barang",
			},
		}
		if paymentSettlementAmount > 0 {
			lines = append([]models.JournalLine{
				{
					AccountID:   cashAccount.ID,
					Debit:       paymentSettlementAmount,
					Credit:      0,
					Description: "Penerimaan penjualan",
				},
			}, lines...)
		}
		if refundCreditAmount > 0 {
			lines = append([]models.JournalLine{
				{
					AccountID:   refundCreditAccount.ID,
					Debit:       refundCreditAmount,
					Credit:      0,
					Description: "Pemakaian saldo retur / voucher",
				},
			}, lines...)
		}
		if hasService {
			lines = append(lines, models.JournalLine{
				AccountID:   serviceAccount.ID,
				Debit:       0,
				Credit:      amounts.Service,
				Description: "Service charge restoran",
			})
		}
		if hasTax {
			lines = append(lines, models.JournalLine{
				AccountID:   taxAccount.ID,
				Debit:       0,
				Credit:      amounts.Tax,
				Description: "Pajak penjualan",
			})
		}
		if hasDiscount {
			lines = append(lines, models.JournalLine{
				AccountID:   discountAccount.ID,
				Debit:       amounts.Discount,
				Credit:      0,
				Description: "Diskon penjualan",
			})
		}
		if hasCostJournal {
			lines = append(lines,
				models.JournalLine{
					AccountID:   cogsAccount.ID,
					Debit:       costOfGoods,
					Credit:      0,
					Description: "Harga pokok penjualan",
				},
				models.JournalLine{
					AccountID:   inventoryAccount.ID,
					Debit:       0,
					Credit:      costOfGoods,
					Description: "Pengurangan persediaan",
				},
			)
		}
		journal.JournalLines = lines

		if err := tx.Create(&journal).Error; err != nil {
			return err
		}

		transaction = models.Transaction{
			OutletID:              input.OutletID,
			CashierID:             input.CashierID,
			CashierName:           cashierName,
			CustomerID:            resolvedCustomerID,
			CustomerName:          customerName,
			Subtotal:              amounts.Subtotal,
			DiscountPercent:       amounts.DiscountPercent,
			Total:                 amounts.Subtotal,
			ServicePercent:        amounts.ServicePercent,
			Service:               amounts.Service,
			TaxPercent:            amounts.TaxPercent,
			Tax:                   amounts.Tax,
			Discount:              amounts.Discount,
			GrandTotal:            amounts.GrandTotal,
			SettlementAmount:      paymentSettlementAmount,
			RefundCreditAmount:    refundCreditAmount,
			RefundCreditCode:      refundCreditCode,
			PaymentMethod:         effectivePaymentMethod,
			Note:                  input.Note,
			JournalEntryID:        &journal.ID,
			AccountingSyncStatus:  services.AccountingSyncStatusPending,
			AccountingIdempotency: services.BuildAccountingIdempotencyKey("pos_sale", 0),
			CreatedAt:             time.Now(),
			UpdatedAt:             time.Now(),
			Status:                input.Status,
			DocumentStatus:        services.TransactionDocumentStatusPosted,
		}
		if err := tx.Create(&transaction).Error; err != nil {
			return err
		}
		transaction.AccountingIdempotency = services.BuildAccountingIdempotencyKey("pos_sale", transaction.ID)
		if err := tx.Model(&transaction).Updates(map[string]interface{}{
			"accounting_sync_status":     services.AccountingSyncStatusPending,
			"accounting_idempotency_key": transaction.AccountingIdempotency,
		}).Error; err != nil {
			return err
		}

		for _, item := range input.Items {
			item.TransactionID = transaction.ID
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}

		if err := services.RecordTransactionInventoryConsumptions(tx, transaction.ID, salePlan.Consumptions); err != nil {
			return err
		}

		for _, consumption := range salePlan.Consumptions {
			product := lockedProducts[consumption.InventoryProductID]
			if !services.IsStockTrackedItemType(product.ItemType) {
				continue
			}
			stockBefore := product.Stock
			stockAfter := stockBefore - consumption.QuantityConsumed
			if err := tx.Model(&models.Product{}).
				Where("id = ? AND outlet_id = ?", consumption.InventoryProductID, input.OutletID).
				Update("stock", stockAfter).Error; err != nil {
				return err
			}
			product.Stock = stockAfter

			movementType := services.InventoryMovementSale
			note := fmt.Sprintf("POS sale transaction #%d", transaction.ID)
			if consumption.ConsumptionType == services.InventoryConsumptionTypeRecipeComponent {
				movementType = services.InventoryMovementRecipeConsume
				soldProduct := lockedProducts[consumption.SoldProductID]
				note = fmt.Sprintf("Recipe consumption for sale #%d from %s", transaction.ID, soldProduct.Name)
			}

			if err := services.AppendInventoryLedger(tx, models.InventoryLedger{
				OutletID:      input.OutletID,
				ProductID:     consumption.InventoryProductID,
				VariantID:     consumption.VariantID,
				MovementType:  movementType,
				ReferenceType: services.InventoryReferenceSale,
				ReferenceID:   transaction.ID,
				QuantityIn:    0,
				QuantityOut:   consumption.QuantityConsumed,
				StockBefore:   stockBefore,
				StockAfter:    stockAfter,
				UnitCost:      consumption.UnitCost,
				TotalCost:     consumption.TotalCost,
				Notes:         note,
			}); err != nil {
				return err
			}
		}

		if refundCreditAmount > 0 && lockedRefundCredit != nil {
			if err := services.ApplyRefundStoreCreditUsageTx(
				tx,
				lockedRefundCredit,
				transaction,
				refundCreditAmount,
				fmt.Sprintf("Pemakaian saldo retur %s untuk transaksi #%d", lockedRefundCredit.CreditCode, transaction.ID),
			); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		utils.Log.Errorf("❌ Failed to create transaction: %v", err)
		if isInventoryValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	syncResult, syncErr := services.SyncAccountingRecord(services.AccountingRecordTypeSale, transaction.ID)
	if syncErr != nil {
		utils.Log.Warnf("transaction %d synced locally but failed to post accounting journal: %v", transaction.ID, syncErr)
		c.JSON(http.StatusOK, gin.H{
			"transaction_id":             transaction.ID,
			"message":                    "Transaction completed and journal recorded",
			"accounting_sync_status":     services.AccountingSyncStatusFailed,
			"accounting_sync_error":      syncErr.Error(),
			"accounting_idempotency_key": services.BuildAccountingIdempotencyKey("pos_sale", transaction.ID),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transaction_id":             transaction.ID,
		"message":                    "Transaction completed and journal recorded",
		"accounting_sync_status":     syncResult.Status,
		"accounting_idempotency_key": syncResult.IdempotencyKey,
		"accounting_journal_id":      syncResult.JournalID,
		"accounting_already_posted":  syncResult.AlreadyPosted,
	})
}

func GetSalesReport(c *gin.Context) {
	if err := services.EnsurePOSApprovalSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var sales []models.Transaction

	// Query params
	startDateStr := c.Query("start_date") // format: YYYY-MM-DD
	endDateStr := c.Query("end_date")
	cashierID := c.Query("cashier_id")
	outletID := c.Query("outlet_id")
	paymentMethod := c.Query("payment_method")

	query := database.DB.
		Preload("Items.Product").
		Preload("Items.Variant").
		Model(&models.Transaction{})
	query = applyNonVoidedTransactionFilter(query)

	if startDateStr != "" && endDateStr != "" {
		startDate, err1 := parseTransactionDateOnly(startDateStr)
		endDate, err2 := parseTransactionDateOnly(endDateStr)
		if err1 == nil && err2 == nil {
			query = query.Where("created_at >= ? AND created_at < ?", startDate, endDate.AddDate(0, 0, 1))
		}
	}

	if cashierID != "" {
		query = query.Where("cashier_id = ?", cashierID)
	}

	if outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}

	if paymentMethod != "" {
		query = query.Where("payment_method = ?", paymentMethod)
	}

	if err := query.Order("created_at desc").Find(&sales).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data penjualan"})
		return
	}

	c.JSON(http.StatusOK, sales)
}

func GetDashboardInfo(c *gin.Context) {
	if err := services.EnsurePOSApprovalSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	outletID := c.Query("outlet")
	var totalSales, todaySales float64

	// Total sales keseluruhan
	err := database.DB.Model(&models.Transaction{}).
		Where("outlet_id=?", outletID).
		Where("COALESCE(document_status, ?) <> ?", services.TransactionDocumentStatusPosted, services.TransactionDocumentStatusVoided).
		Select("COALESCE(SUM(COALESCE(NULLIF(subtotal, 0), total)), 0)").
		Scan(&totalSales).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghitung total penjualan"})
		return
	}

	// Total sales hari ini
	today := time.Now().In(time.Local)
	startOfDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	endOfDay := startOfDay.AddDate(0, 0, 1)

	err = database.DB.Model(&models.Transaction{}).
		Where("outlet_id = ? AND created_at >= ? AND created_at < ?", outletID, startOfDay, endOfDay).
		Where("COALESCE(document_status, ?) <> ?", services.TransactionDocumentStatusPosted, services.TransactionDocumentStatusVoided).
		Select("COALESCE(SUM(COALESCE(NULLIF(subtotal, 0), total)), 0)").
		Scan(&todaySales).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghitung penjualan hari ini"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total_sales": totalSales,
		"today_sales": todaySales,
	})
}
func GetRevenue(c *gin.Context) {
	outletIDStr := c.Query("outlet")

	if outletIDStr == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid Outlet ID"})
		return
	}
	outlet, err := strconv.Atoi(outletIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid outlet parameters"})
		return
	}
	var todayRevenue, yesterdayRevenue, weeklyRevenue, monthlyRevenue float64

	// 🔹 Hari ini
	revenueCategoryFilter := "LOWER(COALESCE(accounts.category, '')) IN ('revenue', 'pendapatan')"

	if err := database.DB.Table("tbljournal_lines jl").
		Select("COALESCE(SUM(jl.credit),0)").
		Joins("JOIN tbljournal_entries je ON je.id = jl.journal_entry_id").
		Joins("LEFT JOIN tblaccounts accounts ON accounts.id = jl.account_id").
		Where("je.reference = ?", "Revenue").
		Where(revenueCategoryFilter).
		Where("DATE(je.created_at) = CURRENT_DATE").
		Where("je.outlet_id=?", outlet).
		Scan(&todayRevenue).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal ambil revenue harian"})
		return
	}

	// 🔹 Kemarin
	if err := database.DB.Table("tbljournal_lines jl").
		Select("COALESCE(SUM(jl.credit),0)").
		Joins("JOIN tbljournal_entries je ON je.id = jl.journal_entry_id").
		Joins("LEFT JOIN tblaccounts accounts ON accounts.id = jl.account_id").
		Where("je.reference = ?", "Revenue").
		Where(revenueCategoryFilter).
		Where("DATE(je.created_at) = CURRENT_DATE - INTERVAL '1 day'").
		Where("je.outlet_id=?", outlet).
		Scan(&yesterdayRevenue).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal ambil revenue kemarin"})
		return
	}

	// 🔹 Minggu ini
	if err := database.DB.Table("tbljournal_lines jl").
		Select("COALESCE(SUM(jl.credit),0)").
		Joins("JOIN tbljournal_entries je ON je.id = jl.journal_entry_id").
		Joins("LEFT JOIN tblaccounts accounts ON accounts.id = jl.account_id").
		Where("je.reference = ?", "Revenue").
		Where(revenueCategoryFilter).
		Where("DATE_TRUNC('week', je.created_at) = DATE_TRUNC('week', CURRENT_DATE)").
		Where("je.outlet_id=?", outlet).
		Scan(&weeklyRevenue).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal ambil revenue mingguan"})
		return
	}

	// 🔹 Bulan ini
	if err := database.DB.Table("tbljournal_lines jl").
		Select("COALESCE(SUM(jl.credit),0)").
		Joins("JOIN tbljournal_entries je ON je.id = jl.journal_entry_id").
		Joins("LEFT JOIN tblaccounts accounts ON accounts.id = jl.account_id").
		Where("je.reference = ?", "Revenue").
		Where(revenueCategoryFilter).
		Where("DATE_TRUNC('month', je.created_at) = DATE_TRUNC('month', CURRENT_DATE)").
		Where("je.outlet_id=?", outlet).
		Scan(&monthlyRevenue).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal ambil revenue bulanan"})
		return
	}
	//get transaction active
	var transactions []models.Transaction
	if err := database.DB.Model(&models.Transaction{}).
		Where("outlet_id = ?", outlet).
		Where("status = ?", 0).
		Where("COALESCE(document_status, ?) <> ?", services.TransactionDocumentStatusPosted, services.TransactionDocumentStatusVoided).
		Where("deleted_at IS NULL").
		Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal ambil transaksi aktif"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"today_revenue":     todayRevenue,
		"yesterday_revenue": yesterdayRevenue,
		"weekly_revenue":    weeklyRevenue,
		"monthly_revenue":   monthlyRevenue,
		"transactions":      transactions,
	})
}

func UpdateStatus(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID is required"})
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	// update status jadi 1
	if err := database.DB.Model(&models.Transaction{}).
		Where("id = ?", id).
		Update("status", 1).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Transaction status updated successfully"})
}
