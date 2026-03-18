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

type stockMovementSummary struct {
	TotalMovements   int     `json:"total_movements"`
	TotalQuantityIn  int     `json:"total_quantity_in"`
	TotalQuantityOut int     `json:"total_quantity_out"`
	NetQuantity      int     `json:"net_quantity"`
	TotalCostIn      float64 `json:"total_cost_in"`
	TotalCostOut     float64 `json:"total_cost_out"`
	NetCost          float64 `json:"net_cost"`
	UniqueProducts   int     `json:"unique_products"`
	UniqueReferences int     `json:"unique_references"`
}

type stockMovementBreakdownItem struct {
	Key      string  `json:"key"`
	Label    string  `json:"label"`
	Count    int     `json:"count"`
	Quantity int     `json:"quantity"`
	Amount   float64 `json:"amount"`
}

type stockMovementTrendItem struct {
	PeriodKey        string  `json:"period_key"`
	PeriodLabel      string  `json:"period_label"`
	TotalMovements   int     `json:"total_movements"`
	TotalQuantityIn  int     `json:"total_quantity_in"`
	TotalQuantityOut int     `json:"total_quantity_out"`
	NetQuantity      int     `json:"net_quantity"`
	TotalCostIn      float64 `json:"total_cost_in"`
	TotalCostOut     float64 `json:"total_cost_out"`
}

type stockMovementReportRow struct {
	ID            uint      `json:"id"`
	OutletID      uint      `json:"outlet_id"`
	ProductID     uint      `json:"product_id"`
	ProductName   string    `json:"product_name"`
	ItemType      string    `json:"item_type"`
	MovementType  string    `json:"movement_type"`
	ReferenceType string    `json:"reference_type"`
	ReferenceID   uint      `json:"reference_id"`
	QuantityIn    int       `json:"quantity_in"`
	QuantityOut   int       `json:"quantity_out"`
	NetQuantity   int       `json:"net_quantity"`
	StockBefore   int       `json:"stock_before"`
	StockAfter    int       `json:"stock_after"`
	UnitCost      float64   `json:"unit_cost"`
	TotalCost     float64   `json:"total_cost"`
	Notes         string    `json:"notes"`
	CreatedAt     time.Time `json:"created_at"`
}

func GetStockMovementReport(c *gin.Context) {
	if err := services.EnsureInventorySchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	query := database.DB.Model(&models.InventoryLedger{}).Preload("Product").Order("created_at DESC, id DESC")

	if outletID := strings.TrimSpace(c.Query("outlet_id")); outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}
	if productID := strings.TrimSpace(c.Query("product_id")); productID != "" {
		query = query.Where("product_id = ?", productID)
	}
	if movementType := strings.ToLower(strings.TrimSpace(c.Query("movement_type"))); movementType != "" {
		query = query.Where("LOWER(COALESCE(movement_type, '')) = ?", movementType)
	}
	if referenceType := strings.ToLower(strings.TrimSpace(c.Query("reference_type"))); referenceType != "" {
		query = query.Where("LOWER(COALESCE(reference_type, '')) = ?", referenceType)
	}
	if startDate := strings.TrimSpace(c.Query("start_date")); startDate != "" {
		if parsed, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("DATE(created_at) >= ?", parsed.Format("2006-01-02"))
		}
	}
	if endDate := strings.TrimSpace(c.Query("end_date")); endDate != "" {
		if parsed, err := time.Parse("2006-01-02", endDate); err == nil {
			query = query.Where("DATE(created_at) <= ?", parsed.Format("2006-01-02"))
		}
	}

	var ledger []models.InventoryLedger
	if err := query.Find(&ledger).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data stock movement"})
		return
	}

	search := strings.ToLower(strings.TrimSpace(c.Query("search")))
	rows := make([]stockMovementReportRow, 0, len(ledger))
	for _, item := range ledger {
		productName := strings.TrimSpace(item.Product.Name)
		if productName == "" {
			productName = "Product #" + strconv.FormatUint(uint64(item.ProductID), 10)
		}

		row := stockMovementReportRow{
			ID:            item.ID,
			OutletID:      item.OutletID,
			ProductID:     item.ProductID,
			ProductName:   productName,
			ItemType:      item.Product.ItemType,
			MovementType:  item.MovementType,
			ReferenceType: item.ReferenceType,
			ReferenceID:   item.ReferenceID,
			QuantityIn:    item.QuantityIn,
			QuantityOut:   item.QuantityOut,
			NetQuantity:   item.QuantityIn - item.QuantityOut,
			StockBefore:   item.StockBefore,
			StockAfter:    item.StockAfter,
			UnitCost:      item.UnitCost,
			TotalCost:     item.TotalCost,
			Notes:         item.Notes,
			CreatedAt:     item.CreatedAt,
		}

		if search != "" {
			candidates := []string{
				strconv.FormatUint(uint64(row.ID), 10),
				strconv.FormatUint(uint64(row.ProductID), 10),
				strconv.FormatUint(uint64(row.ReferenceID), 10),
				row.ProductName,
				row.ItemType,
				row.MovementType,
				row.ReferenceType,
				row.Notes,
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
		return rows[left].CreatedAt.After(rows[right].CreatedAt)
	})

	c.JSON(http.StatusOK, buildStockMovementReportResponse(rows))
}

func buildStockMovementReportResponse(rows []stockMovementReportRow) gin.H {
	summary := stockMovementSummary{}
	movementBreakdown := make(map[string]*stockMovementBreakdownItem)
	referenceBreakdown := make(map[string]*stockMovementBreakdownItem)
	productBreakdown := make(map[string]*stockMovementBreakdownItem)
	itemTypeBreakdown := make(map[string]*stockMovementBreakdownItem)
	trendBreakdown := make(map[string]*stockMovementTrendItem)
	uniqueProducts := make(map[uint]struct{})
	uniqueReferences := make(map[string]struct{})

	for _, row := range rows {
		summary.TotalMovements++
		summary.TotalQuantityIn += row.QuantityIn
		summary.TotalQuantityOut += row.QuantityOut
		summary.NetQuantity += row.NetQuantity
		if row.QuantityIn > 0 {
			summary.TotalCostIn += row.TotalCost
		}
		if row.QuantityOut > 0 {
			summary.TotalCostOut += row.TotalCost
		}

		uniqueProducts[row.ProductID] = struct{}{}
		uniqueReferences[strings.ToLower(strings.TrimSpace(row.ReferenceType))+":"+strconv.FormatUint(uint64(row.ReferenceID), 10)] = struct{}{}

		movementKey := strings.ToLower(strings.TrimSpace(row.MovementType))
		movementEntry := movementBreakdown[movementKey]
		if movementEntry == nil {
			movementEntry = &stockMovementBreakdownItem{
				Key:   movementKey,
				Label: row.MovementType,
			}
			movementBreakdown[movementKey] = movementEntry
		}
		movementEntry.Count++
		movementEntry.Quantity += row.NetQuantity
		movementEntry.Amount += row.TotalCost

		referenceKey := strings.ToLower(strings.TrimSpace(row.ReferenceType))
		referenceEntry := referenceBreakdown[referenceKey]
		if referenceEntry == nil {
			referenceEntry = &stockMovementBreakdownItem{
				Key:   referenceKey,
				Label: row.ReferenceType,
			}
			referenceBreakdown[referenceKey] = referenceEntry
		}
		referenceEntry.Count++
		referenceEntry.Quantity += row.NetQuantity
		referenceEntry.Amount += row.TotalCost

		productKey := strings.ToLower(strings.TrimSpace(row.ProductName))
		productEntry := productBreakdown[productKey]
		if productEntry == nil {
			productEntry = &stockMovementBreakdownItem{
				Key:   productKey,
				Label: row.ProductName,
			}
			productBreakdown[productKey] = productEntry
		}
		productEntry.Count++
		productEntry.Quantity += row.NetQuantity
		productEntry.Amount += row.TotalCost

		itemTypeKey := strings.ToLower(strings.TrimSpace(row.ItemType))
		if itemTypeKey == "" {
			itemTypeKey = "unknown"
		}
		itemTypeEntry := itemTypeBreakdown[itemTypeKey]
		if itemTypeEntry == nil {
			itemTypeEntry = &stockMovementBreakdownItem{
				Key:   itemTypeKey,
				Label: row.ItemType,
			}
			if itemTypeEntry.Label == "" {
				itemTypeEntry.Label = "Unknown"
			}
			itemTypeBreakdown[itemTypeKey] = itemTypeEntry
		}
		itemTypeEntry.Count++
		itemTypeEntry.Quantity += row.NetQuantity
		itemTypeEntry.Amount += row.TotalCost

		periodKey := row.CreatedAt.Format("2006-01-02")
		trendEntry := trendBreakdown[periodKey]
		if trendEntry == nil {
			trendEntry = &stockMovementTrendItem{
				PeriodKey:   periodKey,
				PeriodLabel: row.CreatedAt.Format("02 Jan 2006"),
			}
			trendBreakdown[periodKey] = trendEntry
		}
		trendEntry.TotalMovements++
		trendEntry.TotalQuantityIn += row.QuantityIn
		trendEntry.TotalQuantityOut += row.QuantityOut
		trendEntry.NetQuantity += row.NetQuantity
		if row.QuantityIn > 0 {
			trendEntry.TotalCostIn += row.TotalCost
		}
		if row.QuantityOut > 0 {
			trendEntry.TotalCostOut += row.TotalCost
		}
	}

	summary.UniqueProducts = len(uniqueProducts)
	summary.UniqueReferences = len(uniqueReferences)
	summary.NetCost = summary.TotalCostIn - summary.TotalCostOut

	return gin.H{
		"summary":             summary,
		"movement_breakdown":  flattenStockMovementBreakdown(movementBreakdown),
		"reference_breakdown": flattenStockMovementBreakdown(referenceBreakdown),
		"product_breakdown":   flattenStockMovementBreakdown(productBreakdown),
		"item_type_breakdown": flattenStockMovementBreakdown(itemTypeBreakdown),
		"trends":              flattenStockMovementTrends(trendBreakdown),
		"results":             rows,
		"total":               len(rows),
	}
}

func flattenStockMovementBreakdown(source map[string]*stockMovementBreakdownItem) []stockMovementBreakdownItem {
	items := make([]stockMovementBreakdownItem, 0, len(source))
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

func flattenStockMovementTrends(source map[string]*stockMovementTrendItem) []stockMovementTrendItem {
	items := make([]stockMovementTrendItem, 0, len(source))
	for _, item := range source {
		items = append(items, *item)
	}
	sort.Slice(items, func(left, right int) bool {
		return items[left].PeriodKey < items[right].PeriodKey
	})
	return items
}
