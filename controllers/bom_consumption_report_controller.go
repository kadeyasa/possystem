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

type bomConsumptionSummary struct {
	TotalTransactions     int     `json:"total_transactions"`
	TotalRows             int     `json:"total_rows"`
	TotalSoldQuantity     int     `json:"total_sold_quantity"`
	TotalConsumedQuantity int     `json:"total_consumed_quantity"`
	TotalConsumptionCost  float64 `json:"total_consumption_cost"`
	ActiveFinishedGoods   int     `json:"active_finished_goods"`
	ActiveIngredients     int     `json:"active_ingredients"`
	ActiveOutlets         int     `json:"active_outlets"`
	AverageCostPerSale    float64 `json:"average_cost_per_sale"`
}

type bomConsumptionBreakdownItem struct {
	Key      string  `json:"key"`
	Label    string  `json:"label"`
	Count    int     `json:"count"`
	Quantity int     `json:"quantity"`
	Amount   float64 `json:"amount"`
}

type bomConsumptionTrendItem struct {
	PeriodKey            string  `json:"period_key"`
	PeriodLabel          string  `json:"period_label"`
	TotalTransactions    int     `json:"total_transactions"`
	TotalRows            int     `json:"total_rows"`
	TotalSoldQuantity    int     `json:"total_sold_quantity"`
	TotalConsumedQty     int     `json:"total_consumed_quantity"`
	TotalConsumptionCost float64 `json:"total_consumption_cost"`
}

type bomConsumptionReportRow struct {
	ID                    uint      `json:"id"`
	TransactionID         uint      `json:"transaction_id"`
	OutletID              uint      `json:"outlet_id"`
	SoldProductID         uint      `json:"sold_product_id"`
	SoldProductName       string    `json:"sold_product_name"`
	IngredientProductID   uint      `json:"ingredient_product_id"`
	IngredientProductName string    `json:"ingredient_product_name"`
	IngredientItemType    string    `json:"ingredient_item_type"`
	IngredientUnit        string    `json:"ingredient_unit"`
	ConsumptionType       string    `json:"consumption_type"`
	SoldQuantity          int       `json:"sold_quantity"`
	QuantityPerUnit       int       `json:"quantity_per_unit"`
	QuantityConsumed      int       `json:"quantity_consumed"`
	UnitCost              float64   `json:"unit_cost"`
	TotalCost             float64   `json:"total_cost"`
	PaymentMethod         string    `json:"payment_method"`
	CashierName           string    `json:"cashier_name"`
	TransactionTotal      float64   `json:"transaction_total"`
	TransactionCreatedAt  time.Time `json:"transaction_created_at"`
	TransactionNote       string    `json:"transaction_note"`
	Note                  string    `json:"note"`
}

type bomConsumptionTrendBucket struct {
	Item         bomConsumptionTrendItem
	Transactions map[uint]struct{}
}

func GetBOMConsumptionReport(c *gin.Context) {
	if err := services.EnsureInventorySchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	filterSoldProductID := parseUintQuery(c.Query("product_id"))
	filterIngredientID := parseUintQuery(c.Query("ingredient_product_id"))
	search := strings.ToLower(strings.TrimSpace(c.Query("search")))

	query := database.DB.
		Model(&models.TransactionInventoryConsumption{}).
		Joins("JOIN tbltransactions ON tbltransactions.id = tbltransaction_inventory_consumptions.transaction_id").
		Where("LOWER(COALESCE(tbltransaction_inventory_consumptions.consumption_type, '')) = ?", services.InventoryConsumptionTypeRecipeComponent).
		Where("LOWER(COALESCE(tbltransactions.document_status, 'posted')) <> ?", "voided").
		Preload("SoldProduct").
		Preload("InventoryProduct").
		Order("tbltransactions.created_at DESC, tbltransaction_inventory_consumptions.id DESC")

	if outletID := strings.TrimSpace(c.Query("outlet_id")); outletID != "" {
		query = query.Where("tbltransactions.outlet_id = ?", outletID)
	}
	if filterSoldProductID != 0 {
		query = query.Where("tbltransaction_inventory_consumptions.sold_product_id = ?", filterSoldProductID)
	}
	if filterIngredientID != 0 {
		query = query.Where("tbltransaction_inventory_consumptions.inventory_product_id = ?", filterIngredientID)
	}
	if paymentMethod := strings.ToLower(strings.TrimSpace(c.Query("payment_method"))); paymentMethod != "" {
		query = query.Where("LOWER(COALESCE(tbltransactions.payment_method, '')) = ?", paymentMethod)
	}
	if startDate := strings.TrimSpace(c.Query("start_date")); startDate != "" {
		if parsed, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("DATE(tbltransactions.created_at) >= ?", parsed.Format("2006-01-02"))
		}
	}
	if endDate := strings.TrimSpace(c.Query("end_date")); endDate != "" {
		if parsed, err := time.Parse("2006-01-02", endDate); err == nil {
			query = query.Where("DATE(tbltransactions.created_at) <= ?", parsed.Format("2006-01-02"))
		}
	}

	var consumptions []models.TransactionInventoryConsumption
	if err := query.Find(&consumptions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data BOM consumption"})
		return
	}

	transactionIDs := make([]uint, 0, len(consumptions))
	transactionIDSet := make(map[uint]struct{}, len(consumptions))
	for _, consumption := range consumptions {
		if _, exists := transactionIDSet[consumption.TransactionID]; exists {
			continue
		}
		transactionIDSet[consumption.TransactionID] = struct{}{}
		transactionIDs = append(transactionIDs, consumption.TransactionID)
	}

	transactionMap := make(map[uint]models.Transaction, len(transactionIDs))
	if len(transactionIDs) > 0 {
		var transactions []models.Transaction
		if err := database.DB.Where("id IN ?", transactionIDs).Find(&transactions).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil transaksi sumber BOM"})
			return
		}
		for _, transaction := range transactions {
			transactionMap[transaction.ID] = transaction
		}
	}

	rows := make([]bomConsumptionReportRow, 0, len(consumptions))
	for _, consumption := range consumptions {
		transaction := transactionMap[consumption.TransactionID]
		soldProductName := strings.TrimSpace(consumption.SoldProduct.Name)
		if soldProductName == "" {
			soldProductName = "Product #" + strconv.FormatUint(uint64(consumption.SoldProductID), 10)
		}
		ingredientProductName := strings.TrimSpace(consumption.InventoryProduct.Name)
		if ingredientProductName == "" {
			ingredientProductName = "Product #" + strconv.FormatUint(uint64(consumption.InventoryProductID), 10)
		}
		paymentMethod := strings.TrimSpace(transaction.PaymentMethod)
		if paymentMethod == "" {
			paymentMethod = "cash"
		}
		cashierName := strings.TrimSpace(transaction.CashierName)
		if cashierName == "" && transaction.CashierID != 0 {
			cashierName = "Cashier #" + strconv.FormatUint(uint64(transaction.CashierID), 10)
		}
		if cashierName == "" {
			cashierName = "Cashier belum tercatat"
		}
		transactionCreatedAt := transaction.CreatedAt
		if transactionCreatedAt.IsZero() {
			transactionCreatedAt = consumption.CreatedAt
		}

		row := bomConsumptionReportRow{
			ID:                    consumption.ID,
			TransactionID:         consumption.TransactionID,
			OutletID:              transaction.OutletID,
			SoldProductID:         consumption.SoldProductID,
			SoldProductName:       soldProductName,
			IngredientProductID:   consumption.InventoryProductID,
			IngredientProductName: ingredientProductName,
			IngredientItemType:    consumption.InventoryProduct.ItemType,
			IngredientUnit:        strings.TrimSpace(consumption.InventoryProduct.Satuan),
			ConsumptionType:       consumption.ConsumptionType,
			SoldQuantity:          consumption.SoldQuantity,
			QuantityPerUnit:       consumption.QuantityPerUnit,
			QuantityConsumed:      consumption.QuantityConsumed,
			UnitCost:              consumption.UnitCost,
			TotalCost:             consumption.TotalCost,
			PaymentMethod:         paymentMethod,
			CashierName:           cashierName,
			TransactionTotal:      transaction.Total,
			TransactionCreatedAt:  transactionCreatedAt,
			TransactionNote:       strings.TrimSpace(transaction.Note),
			Note:                  strings.TrimSpace(consumption.Note),
		}

		if search != "" {
			candidates := []string{
				strconv.FormatUint(uint64(row.ID), 10),
				strconv.FormatUint(uint64(row.TransactionID), 10),
				strconv.FormatUint(uint64(row.SoldProductID), 10),
				strconv.FormatUint(uint64(row.IngredientProductID), 10),
				row.SoldProductName,
				row.IngredientProductName,
				row.IngredientItemType,
				row.IngredientUnit,
				row.PaymentMethod,
				row.CashierName,
				row.Note,
				row.TransactionNote,
			}
			matched := false
			for _, candidate := range candidates {
				if strings.Contains(strings.ToLower(strings.TrimSpace(candidate)), search) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		rows = append(rows, row)
	}

	sort.Slice(rows, func(left, right int) bool {
		if rows[left].TransactionCreatedAt.Equal(rows[right].TransactionCreatedAt) {
			if rows[left].TransactionID == rows[right].TransactionID {
				return rows[left].ID > rows[right].ID
			}
			return rows[left].TransactionID > rows[right].TransactionID
		}
		return rows[left].TransactionCreatedAt.After(rows[right].TransactionCreatedAt)
	})

	c.JSON(http.StatusOK, buildBOMConsumptionReportResponse(rows))
}

func buildBOMConsumptionReportResponse(rows []bomConsumptionReportRow) gin.H {
	summary := bomConsumptionSummary{}
	finishedGoodBreakdown := make(map[string]*bomConsumptionBreakdownItem)
	ingredientBreakdown := make(map[string]*bomConsumptionBreakdownItem)
	outletBreakdown := make(map[string]*bomConsumptionBreakdownItem)
	ingredientTypeBreakdown := make(map[string]*bomConsumptionBreakdownItem)
	trendBreakdown := make(map[string]*bomConsumptionTrendBucket)
	transactionSet := make(map[uint]struct{})
	finishedGoodSet := make(map[uint]struct{})
	ingredientSet := make(map[uint]struct{})
	outletSet := make(map[uint]struct{})

	for _, row := range rows {
		summary.TotalRows++
		summary.TotalSoldQuantity += row.SoldQuantity
		summary.TotalConsumedQuantity += row.QuantityConsumed
		summary.TotalConsumptionCost += row.TotalCost

		transactionSet[row.TransactionID] = struct{}{}
		finishedGoodSet[row.SoldProductID] = struct{}{}
		ingredientSet[row.IngredientProductID] = struct{}{}
		if row.OutletID != 0 {
			outletSet[row.OutletID] = struct{}{}
		}

		finishedKey := strconv.FormatUint(uint64(row.SoldProductID), 10)
		finishedEntry := finishedGoodBreakdown[finishedKey]
		if finishedEntry == nil {
			finishedEntry = &bomConsumptionBreakdownItem{
				Key:   finishedKey,
				Label: row.SoldProductName,
			}
			finishedGoodBreakdown[finishedKey] = finishedEntry
		}
		finishedEntry.Count++
		finishedEntry.Quantity += row.SoldQuantity
		finishedEntry.Amount += row.TotalCost

		ingredientKey := strconv.FormatUint(uint64(row.IngredientProductID), 10)
		ingredientEntry := ingredientBreakdown[ingredientKey]
		if ingredientEntry == nil {
			ingredientEntry = &bomConsumptionBreakdownItem{
				Key:   ingredientKey,
				Label: row.IngredientProductName,
			}
			ingredientBreakdown[ingredientKey] = ingredientEntry
		}
		ingredientEntry.Count++
		ingredientEntry.Quantity += row.QuantityConsumed
		ingredientEntry.Amount += row.TotalCost

		outletKey := strconv.FormatUint(uint64(row.OutletID), 10)
		outletEntry := outletBreakdown[outletKey]
		if outletEntry == nil {
			outletEntry = &bomConsumptionBreakdownItem{
				Key:   outletKey,
				Label: "Outlet #" + outletKey,
			}
			outletBreakdown[outletKey] = outletEntry
		}
		outletEntry.Count++
		outletEntry.Quantity += row.QuantityConsumed
		outletEntry.Amount += row.TotalCost

		itemTypeKey := strings.ToLower(strings.TrimSpace(row.IngredientItemType))
		if itemTypeKey == "" {
			itemTypeKey = "unknown"
		}
		itemTypeEntry := ingredientTypeBreakdown[itemTypeKey]
		if itemTypeEntry == nil {
			itemTypeEntry = &bomConsumptionBreakdownItem{
				Key:   itemTypeKey,
				Label: row.IngredientItemType,
			}
			if itemTypeEntry.Label == "" {
				itemTypeEntry.Label = "unknown"
			}
			ingredientTypeBreakdown[itemTypeKey] = itemTypeEntry
		}
		itemTypeEntry.Count++
		itemTypeEntry.Quantity += row.QuantityConsumed
		itemTypeEntry.Amount += row.TotalCost

		periodKey := row.TransactionCreatedAt.Format("2006-01-02")
		trendEntry := trendBreakdown[periodKey]
		if trendEntry == nil {
			trendEntry = &bomConsumptionTrendBucket{
				Item: bomConsumptionTrendItem{
					PeriodKey:   periodKey,
					PeriodLabel: row.TransactionCreatedAt.Format("02 Jan 2006"),
				},
				Transactions: make(map[uint]struct{}),
			}
			trendBreakdown[periodKey] = trendEntry
		}
		trendEntry.Transactions[row.TransactionID] = struct{}{}
		trendEntry.Item.TotalRows++
		trendEntry.Item.TotalSoldQuantity += row.SoldQuantity
		trendEntry.Item.TotalConsumedQty += row.QuantityConsumed
		trendEntry.Item.TotalConsumptionCost += row.TotalCost
	}

	summary.TotalTransactions = len(transactionSet)
	summary.ActiveFinishedGoods = len(finishedGoodSet)
	summary.ActiveIngredients = len(ingredientSet)
	summary.ActiveOutlets = len(outletSet)
	if summary.TotalTransactions > 0 {
		summary.AverageCostPerSale = summary.TotalConsumptionCost / float64(summary.TotalTransactions)
	}

	return gin.H{
		"summary":                   summary,
		"finished_good_breakdown":   flattenBOMConsumptionBreakdown(finishedGoodBreakdown),
		"ingredient_breakdown":      flattenBOMConsumptionBreakdown(ingredientBreakdown),
		"outlet_breakdown":          flattenBOMConsumptionBreakdown(outletBreakdown),
		"ingredient_type_breakdown": flattenBOMConsumptionBreakdown(ingredientTypeBreakdown),
		"trends":                    flattenBOMConsumptionTrends(trendBreakdown),
		"results":                   rows,
		"total":                     len(rows),
	}
}

func flattenBOMConsumptionBreakdown(source map[string]*bomConsumptionBreakdownItem) []bomConsumptionBreakdownItem {
	items := make([]bomConsumptionBreakdownItem, 0, len(source))
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

func flattenBOMConsumptionTrends(source map[string]*bomConsumptionTrendBucket) []bomConsumptionTrendItem {
	items := make([]bomConsumptionTrendItem, 0, len(source))
	for _, bucket := range source {
		bucket.Item.TotalTransactions = len(bucket.Transactions)
		items = append(items, bucket.Item)
	}
	sort.Slice(items, func(left, right int) bool {
		return items[left].PeriodKey < items[right].PeriodKey
	})
	return items
}
