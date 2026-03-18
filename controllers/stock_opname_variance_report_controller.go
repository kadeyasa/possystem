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

const (
	stockOpnameVarianceTypeGain    = "gain"
	stockOpnameVarianceTypeLoss    = "loss"
	stockOpnameVarianceTypeMatched = "matched"
)

type stockOpnameVarianceSummary struct {
	TotalOpnames        int     `json:"total_opnames"`
	TotalVarianceItems  int     `json:"total_variance_items"`
	GainItems           int     `json:"gain_items"`
	LossItems           int     `json:"loss_items"`
	MatchedItems        int     `json:"matched_items"`
	TotalGainQuantity   int     `json:"total_gain_quantity"`
	TotalLossQuantity   int     `json:"total_loss_quantity"`
	NetVarianceQuantity int     `json:"net_variance_quantity"`
	TotalVarianceCost   float64 `json:"total_variance_cost"`
	AffectedProducts    int     `json:"affected_products"`
	AffectedOutlets     int     `json:"affected_outlets"`
}

type stockOpnameVarianceBreakdownItem struct {
	Key      string  `json:"key"`
	Label    string  `json:"label"`
	Count    int     `json:"count"`
	Quantity int     `json:"quantity"`
	Amount   float64 `json:"amount"`
}

type stockOpnameVarianceTrendItem struct {
	PeriodKey           string  `json:"period_key"`
	PeriodLabel         string  `json:"period_label"`
	TotalOpnames        int     `json:"total_opnames"`
	VarianceItems       int     `json:"variance_items"`
	GainItems           int     `json:"gain_items"`
	LossItems           int     `json:"loss_items"`
	MatchedItems        int     `json:"matched_items"`
	TotalGainQuantity   int     `json:"total_gain_quantity"`
	TotalLossQuantity   int     `json:"total_loss_quantity"`
	NetVarianceQuantity int     `json:"net_variance_quantity"`
	TotalVarianceCost   float64 `json:"total_variance_cost"`
}

type stockOpnameVarianceReportRow struct {
	ID                   uint       `json:"id"`
	OpnameID             uint       `json:"opname_id"`
	OutletID             uint       `json:"outlet_id"`
	ProductID            uint       `json:"product_id"`
	ProductName          string     `json:"product_name"`
	ItemType             string     `json:"item_type"`
	Unit                 string     `json:"unit"`
	SystemStock          int        `json:"system_stock"`
	ActualStock          int        `json:"actual_stock"`
	DifferenceQty        int        `json:"difference_qty"`
	VarianceType         string     `json:"variance_type"`
	UnitCost             float64    `json:"unit_cost"`
	TotalCost            float64    `json:"total_cost"`
	Note                 string     `json:"note"`
	OpnameNote           string     `json:"opname_note"`
	OpnameStatus         string     `json:"opname_status"`
	AccountingSyncStatus string     `json:"accounting_sync_status"`
	AccountingSyncedAt   *time.Time `json:"accounting_synced_at,omitempty"`
	OpnameDate           time.Time  `json:"opname_date"`
	CreatedAt            time.Time  `json:"created_at"`
}

type stockOpnameVarianceTrendBucket struct {
	Item   stockOpnameVarianceTrendItem
	Opname map[uint]struct{}
}

func GetStockOpnameVarianceReport(c *gin.Context) {
	if err := services.EnsureInventorySchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	query := database.DB.
		Model(&models.StockOpname{}).
		Preload("Items.Product").
		Order("opname_date DESC, id DESC")

	if outletID := strings.TrimSpace(c.Query("outlet_id")); outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}

	startDateStr := strings.TrimSpace(c.Query("start_date"))
	endDateStr := strings.TrimSpace(c.Query("end_date"))
	if startDateStr != "" {
		if startDate, err := time.Parse("2006-01-02", startDateStr); err == nil {
			query = query.Where("opname_date >= ?", startDate)
		}
	}
	if endDateStr != "" {
		if endDate, err := time.Parse("2006-01-02", endDateStr); err == nil {
			query = query.Where("opname_date < ?", endDate.AddDate(0, 0, 1))
		}
	}

	var opnames []models.StockOpname
	if err := query.Find(&opnames).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data stock opname"})
		return
	}

	productFilter := strings.TrimSpace(c.Query("product_id"))
	varianceFilter := normalizeStockOpnameVarianceFilter(c.DefaultQuery("variance_type", "variance_only"))
	search := strings.ToLower(strings.TrimSpace(c.Query("search")))

	rows := make([]stockOpnameVarianceReportRow, 0)
	for _, opname := range opnames {
		opnameDate := opname.OpnameDate
		if opnameDate.IsZero() {
			opnameDate = opname.CreatedAt
		}

		for _, item := range opname.Items {
			if productFilter != "" && strconv.FormatUint(uint64(item.ProductID), 10) != productFilter {
				continue
			}

			varianceType := getStockOpnameVarianceType(item.DifferenceQty)
			if !stockOpnameVarianceFilterMatch(varianceFilter, varianceType) {
				continue
			}

			productName := strings.TrimSpace(item.Product.Name)
			if productName == "" {
				productName = "Product #" + strconv.FormatUint(uint64(item.ProductID), 10)
			}

			row := stockOpnameVarianceReportRow{
				ID:                   item.ID,
				OpnameID:             opname.ID,
				OutletID:             opname.OutletID,
				ProductID:            item.ProductID,
				ProductName:          productName,
				ItemType:             item.Product.ItemType,
				Unit:                 item.Product.Satuan,
				SystemStock:          item.SystemStock,
				ActualStock:          item.ActualStock,
				DifferenceQty:        item.DifferenceQty,
				VarianceType:         varianceType,
				UnitCost:             item.UnitCost,
				TotalCost:            item.TotalCost,
				Note:                 strings.TrimSpace(item.Note),
				OpnameNote:           strings.TrimSpace(opname.Note),
				OpnameStatus:         opname.Status,
				AccountingSyncStatus: opname.AccountingSyncStatus,
				AccountingSyncedAt:   opname.AccountingSyncedAt,
				OpnameDate:           opnameDate,
				CreatedAt:            item.CreatedAt,
			}

			if search != "" {
				candidates := []string{
					strconv.FormatUint(uint64(row.ID), 10),
					strconv.FormatUint(uint64(row.OpnameID), 10),
					strconv.FormatUint(uint64(row.ProductID), 10),
					strconv.FormatInt(int64(row.SystemStock), 10),
					strconv.FormatInt(int64(row.ActualStock), 10),
					strconv.FormatInt(int64(row.DifferenceQty), 10),
					row.ProductName,
					row.ItemType,
					row.Unit,
					row.VarianceType,
					row.Note,
					row.OpnameNote,
					row.OpnameStatus,
					row.AccountingSyncStatus,
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
	}

	sort.Slice(rows, func(left, right int) bool {
		if rows[left].OpnameDate.Equal(rows[right].OpnameDate) {
			if rows[left].OpnameID == rows[right].OpnameID {
				return rows[left].ID > rows[right].ID
			}
			return rows[left].OpnameID > rows[right].OpnameID
		}
		return rows[left].OpnameDate.After(rows[right].OpnameDate)
	})

	c.JSON(http.StatusOK, buildStockOpnameVarianceResponse(rows))
}

func buildStockOpnameVarianceResponse(rows []stockOpnameVarianceReportRow) gin.H {
	summary := stockOpnameVarianceSummary{}
	varianceBreakdown := make(map[string]*stockOpnameVarianceBreakdownItem)
	outletBreakdown := make(map[string]*stockOpnameVarianceBreakdownItem)
	productBreakdown := make(map[string]*stockOpnameVarianceBreakdownItem)
	itemTypeBreakdown := make(map[string]*stockOpnameVarianceBreakdownItem)
	trendBreakdown := make(map[string]*stockOpnameVarianceTrendBucket)
	affectedOpnames := make(map[uint]struct{})
	affectedProducts := make(map[uint]struct{})
	affectedOutlets := make(map[uint]struct{})

	for _, row := range rows {
		affectedOpnames[row.OpnameID] = struct{}{}
		affectedProducts[row.ProductID] = struct{}{}
		affectedOutlets[row.OutletID] = struct{}{}

		summary.TotalVarianceItems++
		summary.NetVarianceQuantity += row.DifferenceQty
		summary.TotalVarianceCost += row.TotalCost

		absQty := absInt(row.DifferenceQty)

		switch row.VarianceType {
		case stockOpnameVarianceTypeGain:
			summary.GainItems++
			summary.TotalGainQuantity += row.DifferenceQty
		case stockOpnameVarianceTypeLoss:
			summary.LossItems++
			summary.TotalLossQuantity += absQty
		default:
			summary.MatchedItems++
		}

		varianceEntry := varianceBreakdown[row.VarianceType]
		if varianceEntry == nil {
			varianceEntry = &stockOpnameVarianceBreakdownItem{
				Key:   row.VarianceType,
				Label: formatStockOpnameVarianceTypeLabel(row.VarianceType),
			}
			varianceBreakdown[row.VarianceType] = varianceEntry
		}
		varianceEntry.Count++
		varianceEntry.Quantity += absQty
		varianceEntry.Amount += row.TotalCost

		outletKey := strconv.FormatUint(uint64(row.OutletID), 10)
		outletEntry := outletBreakdown[outletKey]
		if outletEntry == nil {
			outletEntry = &stockOpnameVarianceBreakdownItem{
				Key:   outletKey,
				Label: "Outlet #" + outletKey,
			}
			outletBreakdown[outletKey] = outletEntry
		}
		outletEntry.Count++
		outletEntry.Quantity += absQty
		outletEntry.Amount += row.TotalCost

		productKey := strconv.FormatUint(uint64(row.ProductID), 10)
		productEntry := productBreakdown[productKey]
		if productEntry == nil {
			productEntry = &stockOpnameVarianceBreakdownItem{
				Key:   productKey,
				Label: row.ProductName,
			}
			productBreakdown[productKey] = productEntry
		}
		productEntry.Count++
		productEntry.Quantity += absQty
		productEntry.Amount += row.TotalCost

		itemTypeKey := strings.ToLower(strings.TrimSpace(row.ItemType))
		if itemTypeKey == "" {
			itemTypeKey = "unknown"
		}
		itemTypeEntry := itemTypeBreakdown[itemTypeKey]
		if itemTypeEntry == nil {
			itemTypeEntry = &stockOpnameVarianceBreakdownItem{
				Key:   itemTypeKey,
				Label: row.ItemType,
			}
			if itemTypeEntry.Label == "" {
				itemTypeEntry.Label = "unknown"
			}
			itemTypeBreakdown[itemTypeKey] = itemTypeEntry
		}
		itemTypeEntry.Count++
		itemTypeEntry.Quantity += absQty
		itemTypeEntry.Amount += row.TotalCost

		periodKey := row.OpnameDate.Format("2006-01-02")
		trendEntry := trendBreakdown[periodKey]
		if trendEntry == nil {
			trendEntry = &stockOpnameVarianceTrendBucket{
				Item: stockOpnameVarianceTrendItem{
					PeriodKey:   periodKey,
					PeriodLabel: row.OpnameDate.Format("02 Jan 2006"),
				},
				Opname: make(map[uint]struct{}),
			}
			trendBreakdown[periodKey] = trendEntry
		}
		trendEntry.Opname[row.OpnameID] = struct{}{}
		trendEntry.Item.VarianceItems++
		trendEntry.Item.NetVarianceQuantity += row.DifferenceQty
		trendEntry.Item.TotalVarianceCost += row.TotalCost
		switch row.VarianceType {
		case stockOpnameVarianceTypeGain:
			trendEntry.Item.GainItems++
			trendEntry.Item.TotalGainQuantity += row.DifferenceQty
		case stockOpnameVarianceTypeLoss:
			trendEntry.Item.LossItems++
			trendEntry.Item.TotalLossQuantity += absQty
		default:
			trendEntry.Item.MatchedItems++
		}
	}

	summary.TotalOpnames = len(affectedOpnames)
	summary.AffectedProducts = len(affectedProducts)
	summary.AffectedOutlets = len(affectedOutlets)

	return gin.H{
		"summary":             summary,
		"variance_breakdown":  flattenStockOpnameVarianceBreakdown(varianceBreakdown),
		"outlet_breakdown":    flattenStockOpnameVarianceBreakdown(outletBreakdown),
		"product_breakdown":   flattenStockOpnameVarianceBreakdown(productBreakdown),
		"item_type_breakdown": flattenStockOpnameVarianceBreakdown(itemTypeBreakdown),
		"trends":              flattenStockOpnameVarianceTrends(trendBreakdown),
		"results":             rows,
		"total":               len(rows),
	}
}

func normalizeStockOpnameVarianceFilter(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "variance_only", "non_zero":
		return "variance_only"
	case "all":
		return "all"
	case stockOpnameVarianceTypeGain:
		return stockOpnameVarianceTypeGain
	case stockOpnameVarianceTypeLoss:
		return stockOpnameVarianceTypeLoss
	case "zero", stockOpnameVarianceTypeMatched:
		return stockOpnameVarianceTypeMatched
	default:
		return "variance_only"
	}
}

func stockOpnameVarianceFilterMatch(filter, varianceType string) bool {
	switch filter {
	case "all":
		return true
	case "variance_only":
		return varianceType == stockOpnameVarianceTypeGain || varianceType == stockOpnameVarianceTypeLoss
	case stockOpnameVarianceTypeGain, stockOpnameVarianceTypeLoss, stockOpnameVarianceTypeMatched:
		return varianceType == filter
	default:
		return varianceType == stockOpnameVarianceTypeGain || varianceType == stockOpnameVarianceTypeLoss
	}
}

func getStockOpnameVarianceType(value int) string {
	if value > 0 {
		return stockOpnameVarianceTypeGain
	}
	if value < 0 {
		return stockOpnameVarianceTypeLoss
	}
	return stockOpnameVarianceTypeMatched
}

func formatStockOpnameVarianceTypeLabel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case stockOpnameVarianceTypeGain:
		return "Surplus"
	case stockOpnameVarianceTypeLoss:
		return "Shortage"
	case stockOpnameVarianceTypeMatched:
		return "Sesuai Sistem"
	default:
		return value
	}
}

func flattenStockOpnameVarianceBreakdown(source map[string]*stockOpnameVarianceBreakdownItem) []stockOpnameVarianceBreakdownItem {
	items := make([]stockOpnameVarianceBreakdownItem, 0, len(source))
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

func flattenStockOpnameVarianceTrends(source map[string]*stockOpnameVarianceTrendBucket) []stockOpnameVarianceTrendItem {
	items := make([]stockOpnameVarianceTrendItem, 0, len(source))
	for _, bucket := range source {
		bucket.Item.TotalOpnames = len(bucket.Opname)
		items = append(items, bucket.Item)
	}
	sort.Slice(items, func(left, right int) bool {
		return items[left].PeriodKey < items[right].PeriodKey
	})
	return items
}
