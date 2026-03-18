package controllers

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/services"
)

type purchaseReportSummary struct {
	TotalDocuments    int     `json:"total_documents"`
	TotalAmount       float64 `json:"total_amount"`
	PaidAmount        float64 `json:"paid_amount"`
	OutstandingAmount float64 `json:"outstanding_amount"`
	AverageDocument   float64 `json:"average_document"`
	TotalQuantity     int     `json:"total_quantity"`
	UniqueVendors     int     `json:"unique_vendors"`
	UniqueProducts    int     `json:"unique_products"`
	CreditDocuments   int     `json:"credit_documents"`
	OpenDocuments     int     `json:"open_documents"`
}

type purchaseReportBreakdownItem struct {
	Key      string  `json:"key"`
	Label    string  `json:"label"`
	Count    int     `json:"count"`
	Amount   float64 `json:"amount"`
	Quantity int     `json:"quantity,omitempty"`
}

type purchaseReportTrendItem struct {
	PeriodKey         string  `json:"period_key"`
	PeriodLabel       string  `json:"period_label"`
	TotalDocuments    int     `json:"total_documents"`
	TotalAmount       float64 `json:"total_amount"`
	PaidAmount        float64 `json:"paid_amount"`
	OutstandingAmount float64 `json:"outstanding_amount"`
	TotalQuantity     int     `json:"total_quantity"`
}

type purchaseReportRow struct {
	ID                   uint       `json:"id"`
	OutletID             uint       `json:"outlet_id"`
	SupplierName         string     `json:"supplier_name"`
	InvoiceNumber        string     `json:"invoice_number"`
	PurchaseDate         time.Time  `json:"purchase_date"`
	DueDate              *time.Time `json:"due_date,omitempty"`
	PaymentMethod        string     `json:"payment_method"`
	PaymentStatus        string     `json:"payment_status"`
	Total                float64    `json:"total"`
	PaidAmount           float64    `json:"paid_amount"`
	OutstandingAmount    float64    `json:"outstanding_amount"`
	LinkedVendorBillID   *uint      `json:"linked_vendor_bill_id,omitempty"`
	Note                 string     `json:"note"`
	AccountingSyncStatus string     `json:"accounting_sync_status"`
	AccountingSyncedAt   *time.Time `json:"accounting_synced_at,omitempty"`
	ItemQuantity         int        `json:"item_quantity"`
	ItemCount            int        `json:"item_count"`
	ProductNames         []string   `json:"product_names"`
}

func GetPurchaseReport(c *gin.Context) {
	if err := services.EnsurePOSFinanceDocumentSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var purchases []models.Purchase
	query := database.DB.
		Model(&models.Purchase{}).
		Preload("PurchaseItems.Product").
		Order("purchase_date DESC, id DESC")

	if outletID := strings.TrimSpace(c.Query("outlet_id")); outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}

	if supplier := strings.TrimSpace(c.Query("supplier")); supplier != "" {
		query = query.Where("supplier_name ILIKE ?", "%"+supplier+"%")
	}

	if paymentMethod := strings.ToLower(strings.TrimSpace(c.Query("payment_method"))); paymentMethod != "" {
		query = query.Where("LOWER(COALESCE(payment_method, '')) = ?", paymentMethod)
	}

	if paymentStatus := strings.ToLower(strings.TrimSpace(c.Query("payment_status"))); paymentStatus != "" {
		query = query.Where("LOWER(COALESCE(payment_status, '')) = ?", paymentStatus)
	}

	startDateStr := strings.TrimSpace(c.Query("start_date"))
	endDateStr := strings.TrimSpace(c.Query("end_date"))
	if startDateStr != "" {
		startDate, err := time.Parse("2006-01-02", startDateStr)
		if err == nil {
			query = query.Where("purchase_date >= ?", startDate)
		}
	}
	if endDateStr != "" {
		endDate, err := time.Parse("2006-01-02", endDateStr)
		if err == nil {
			query = query.Where("purchase_date < ?", endDate.AddDate(0, 0, 1))
		}
	}

	if err := query.Find(&purchases).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data pembelian"})
		return
	}

	rows := make([]purchaseReportRow, 0, len(purchases))
	for _, purchase := range purchases {
		itemQuantity := 0
		productNames := make([]string, 0, len(purchase.PurchaseItems))
		productSet := make(map[string]struct{})

		for _, item := range purchase.PurchaseItems {
			itemQuantity += item.Quantity
			productName := strings.TrimSpace(item.Product.Name)
			if productName == "" {
				productName = "Product #" + strconv.FormatUint(uint64(item.ProductID), 10)
			}
			if _, exists := productSet[productName]; exists {
				continue
			}
			productSet[productName] = struct{}{}
			productNames = append(productNames, productName)
		}

		rows = append(rows, purchaseReportRow{
			ID:                   purchase.ID,
			OutletID:             purchase.OutletID,
			SupplierName:         purchase.SupplierName,
			InvoiceNumber:        purchase.InvoiceNumber,
			PurchaseDate:         purchase.PurchaseDate,
			DueDate:              purchase.DueDate,
			PaymentMethod:        purchase.PaymentMethod,
			PaymentStatus:        purchase.PaymentStatus,
			Total:                purchase.Total,
			PaidAmount:           purchase.PaidAmount,
			OutstandingAmount:    purchase.OutstandingAmount,
			LinkedVendorBillID:   purchase.LinkedVendorBillID,
			Note:                 purchase.Note,
			AccountingSyncStatus: purchase.AccountingSyncStatus,
			AccountingSyncedAt:   purchase.AccountingSyncedAt,
			ItemQuantity:         itemQuantity,
			ItemCount:            len(purchase.PurchaseItems),
			ProductNames:         productNames,
		})
	}

	search := strings.ToLower(strings.TrimSpace(c.Query("search")))
	if search != "" {
		filtered := make([]purchaseReportRow, 0, len(rows))
		for _, row := range rows {
			candidates := []string{
				strconv.FormatUint(uint64(row.ID), 10),
				row.SupplierName,
				row.InvoiceNumber,
				row.PaymentMethod,
				row.PaymentStatus,
				row.Note,
			}
			if row.LinkedVendorBillID != nil {
				candidates = append(candidates, strconv.FormatUint(uint64(*row.LinkedVendorBillID), 10))
			}
			candidates = append(candidates, row.ProductNames...)

			matched := false
			for _, candidate := range candidates {
				if strings.Contains(strings.ToLower(strings.TrimSpace(candidate)), search) {
					matched = true
					break
				}
			}

			if matched {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
	}

	response := buildPurchaseReportResponse(rows)
	c.JSON(http.StatusOK, response)
}

func buildPurchaseReportResponse(rows []purchaseReportRow) gin.H {
	vendorBreakdown := make(map[string]*purchaseReportBreakdownItem)
	paymentMethodBreakdown := make(map[string]*purchaseReportBreakdownItem)
	paymentStatusBreakdown := make(map[string]*purchaseReportBreakdownItem)
	itemBreakdown := make(map[string]*purchaseReportBreakdownItem)
	trendBreakdown := make(map[string]*purchaseReportTrendItem)
	uniqueVendors := make(map[string]struct{})
	uniqueProducts := make(map[string]struct{})

	summary := purchaseReportSummary{}

	for _, row := range rows {
		summary.TotalDocuments++
		summary.TotalAmount += row.Total
		summary.PaidAmount += row.PaidAmount
		summary.OutstandingAmount += row.OutstandingAmount
		summary.TotalQuantity += row.ItemQuantity

		supplierKey := strings.ToLower(strings.TrimSpace(row.SupplierName))
		if supplierKey != "" {
			uniqueVendors[supplierKey] = struct{}{}
			entry := vendorBreakdown[supplierKey]
			if entry == nil {
				entry = &purchaseReportBreakdownItem{
					Key:   supplierKey,
					Label: row.SupplierName,
				}
				vendorBreakdown[supplierKey] = entry
			}
			entry.Count++
			entry.Amount += row.Total
			entry.Quantity += row.ItemQuantity
		}

		paymentMethodLabel := strings.TrimSpace(row.PaymentMethod)
		if paymentMethodLabel == "" {
			paymentMethodLabel = "cash"
		}
		paymentMethodKey := strings.ToLower(paymentMethodLabel)
		methodEntry := paymentMethodBreakdown[paymentMethodKey]
		if methodEntry == nil {
			methodEntry = &purchaseReportBreakdownItem{
				Key:   paymentMethodKey,
				Label: paymentMethodLabel,
			}
			paymentMethodBreakdown[paymentMethodKey] = methodEntry
		}
		methodEntry.Count++
		methodEntry.Amount += row.Total
		methodEntry.Quantity += row.ItemQuantity

		paymentStatusLabel := strings.TrimSpace(row.PaymentStatus)
		if paymentStatusLabel == "" {
			paymentStatusLabel = services.PurchasePaymentStatusPaid
		}
		paymentStatusKey := strings.ToLower(paymentStatusLabel)
		statusEntry := paymentStatusBreakdown[paymentStatusKey]
		if statusEntry == nil {
			statusEntry = &purchaseReportBreakdownItem{
				Key:   paymentStatusKey,
				Label: paymentStatusLabel,
			}
			paymentStatusBreakdown[paymentStatusKey] = statusEntry
		}
		statusEntry.Count++
		statusEntry.Amount += row.Total
		statusEntry.Quantity += row.ItemQuantity

		if paymentMethodKey == "credit" {
			summary.CreditDocuments++
		}
		if paymentStatusKey == services.PurchasePaymentStatusOpen || paymentStatusKey == services.PurchasePaymentStatusPartialPaid {
			summary.OpenDocuments++
		}

		for _, productName := range row.ProductNames {
			productKey := strings.ToLower(strings.TrimSpace(productName))
			if productKey == "" {
				continue
			}
			uniqueProducts[productKey] = struct{}{}
			itemEntry := itemBreakdown[productKey]
			if itemEntry == nil {
				itemEntry = &purchaseReportBreakdownItem{
					Key:   productKey,
					Label: productName,
				}
				itemBreakdown[productKey] = itemEntry
			}
			itemEntry.Count++
		}

		periodKey := row.PurchaseDate.Format("2006-01-02")
		trendEntry := trendBreakdown[periodKey]
		if trendEntry == nil {
			trendEntry = &purchaseReportTrendItem{
				PeriodKey:   periodKey,
				PeriodLabel: row.PurchaseDate.Format("02 Jan 2006"),
			}
			trendBreakdown[periodKey] = trendEntry
		}
		trendEntry.TotalDocuments++
		trendEntry.TotalAmount += row.Total
		trendEntry.PaidAmount += row.PaidAmount
		trendEntry.OutstandingAmount += row.OutstandingAmount
		trendEntry.TotalQuantity += row.ItemQuantity
	}

	summary.UniqueVendors = len(uniqueVendors)
	summary.UniqueProducts = len(uniqueProducts)
	if summary.TotalDocuments > 0 {
		summary.AverageDocument = summary.TotalAmount / float64(summary.TotalDocuments)
	}

	return gin.H{
		"summary":                  summary,
		"vendor_breakdown":         flattenPurchaseBreakdown(vendorBreakdown),
		"payment_method_breakdown": flattenPurchaseBreakdown(paymentMethodBreakdown),
		"payment_status_breakdown": flattenPurchaseBreakdown(paymentStatusBreakdown),
		"item_breakdown":           flattenPurchaseBreakdown(itemBreakdown),
		"trends":                   flattenPurchaseTrends(trendBreakdown),
		"results":                  rows,
		"total":                    len(rows),
	}
}

func flattenPurchaseBreakdown(source map[string]*purchaseReportBreakdownItem) []purchaseReportBreakdownItem {
	items := make([]purchaseReportBreakdownItem, 0, len(source))
	for _, item := range source {
		items = append(items, *item)
	}
	sort.Slice(items, func(left, right int) bool {
		if items[left].Amount == items[right].Amount {
			if items[left].Quantity == items[right].Quantity {
				return items[left].Count > items[right].Count
			}
			return items[left].Quantity > items[right].Quantity
		}
		return items[left].Amount > items[right].Amount
	})
	return items
}

func flattenPurchaseTrends(source map[string]*purchaseReportTrendItem) []purchaseReportTrendItem {
	items := make([]purchaseReportTrendItem, 0, len(source))
	for _, item := range source {
		items = append(items, *item)
	}
	sort.Slice(items, func(left, right int) bool {
		return items[left].PeriodKey < items[right].PeriodKey
	})
	return items
}
