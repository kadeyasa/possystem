package controllers

import (
	"math"
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
	vendorPriceModeLatest  = "latest_price"
	vendorPriceModeAverage = "average_price"
	vendorPriceModeLowest  = "lowest_price"
)

const vendorPriceHighSpreadThreshold = 10.0

type vendorPriceComparisonSummary struct {
	ProductsCompared    int     `json:"products_compared"`
	ActiveVendors       int     `json:"active_vendors"`
	CheapestVendorWins  int     `json:"cheapest_vendor_wins"`
	PotentialSavings    float64 `json:"potential_savings"`
	SingleSourceProducts int    `json:"single_source_products"`
	HighSpreadProducts  int     `json:"high_spread_products"`
}

type vendorPriceComparisonBreakdownItem struct {
	Key      string  `json:"key"`
	Label    string  `json:"label"`
	Count    int     `json:"count"`
	Amount   float64 `json:"amount"`
	Quantity int     `json:"quantity,omitempty"`
}

type vendorPriceComparisonProductRow struct {
	ProductID           uint      `json:"product_id"`
	ProductName         string    `json:"product_name"`
	CategoryID          uint      `json:"category_id"`
	CategoryName        string    `json:"category_name"`
	ItemType            string    `json:"item_type"`
	Unit                string    `json:"unit"`
	PreferredVendor     string    `json:"preferred_vendor"`
	PreferredPrice      float64   `json:"preferred_price"`
	SecondVendor        string    `json:"second_vendor"`
	SecondPrice         float64   `json:"second_price"`
	PriceSpread         float64   `json:"price_spread"`
	SpreadPercentage    float64   `json:"spread_percentage"`
	LastPurchasedVendor string    `json:"last_purchased_vendor"`
	LastPurchasedPrice  float64   `json:"last_purchased_price"`
	AverageMarketPrice  float64   `json:"average_market_price"`
	VendorCount         int       `json:"vendor_count"`
	PurchaseCount       int       `json:"purchase_count"`
	TotalQuantityBought int       `json:"total_quantity_bought"`
	LastPurchaseDate    time.Time `json:"last_purchase_date"`
	PotentialSavings    float64   `json:"potential_savings"`
	RecentPurchaseQty   int       `json:"recent_purchase_qty"`
	RiskFlag            string    `json:"risk_flag"`
	RiskLabel           string    `json:"risk_label"`
}

type vendorPriceComparisonVendorDetailRow struct {
	ProductID            uint      `json:"product_id"`
	ProductName          string    `json:"product_name"`
	CategoryName         string    `json:"category_name"`
	ItemType             string    `json:"item_type"`
	Unit                 string    `json:"unit"`
	VendorName           string    `json:"vendor_name"`
	ComparisonPrice      float64   `json:"comparison_price"`
	LatestPrice          float64   `json:"latest_price"`
	AveragePrice         float64   `json:"average_price"`
	LowestPrice          float64   `json:"lowest_price"`
	HighestPrice         float64   `json:"highest_price"`
	PurchaseCount        int       `json:"purchase_count"`
	TotalQuantity        int       `json:"total_quantity"`
	LastPurchaseDate     time.Time `json:"last_purchase_date"`
	PaymentMix           []string  `json:"payment_mix"`
	PaymentMixLabel      string    `json:"payment_mix_label"`
	OutletCoverageCount  int       `json:"outlet_coverage_count"`
	OutletIDs            []uint    `json:"outlet_ids"`
	Rank                 int       `json:"rank"`
	IsPreferred          bool      `json:"is_preferred"`
}

type vendorPriceComparisonVendorAggregate struct {
	Key               string
	Label             string
	PurchaseCount     int
	TotalQuantity     int
	TotalAmount       float64
	LatestPrice       float64
	LowestPrice       float64
	HighestPrice      float64
	LatestPurchaseAt  time.Time
	LatestPurchaseID  uint
	LastPurchaseQty   int
	PaymentMethods    map[string]struct{}
	OutletIDs         map[uint]struct{}
}

func (aggregate *vendorPriceComparisonVendorAggregate) AveragePrice() float64 {
	if aggregate.TotalQuantity <= 0 {
		return 0
	}
	return aggregate.TotalAmount / float64(aggregate.TotalQuantity)
}

func (aggregate *vendorPriceComparisonVendorAggregate) ComparisonPrice(mode string) float64 {
	switch mode {
	case vendorPriceModeLatest:
		return aggregate.LatestPrice
	case vendorPriceModeLowest:
		return aggregate.LowestPrice
	default:
		return aggregate.AveragePrice()
	}
}

type vendorPriceComparisonProductAggregate struct {
	ProductID            uint
	ProductName          string
	CategoryID           uint
	CategoryName         string
	ItemType             string
	Unit                 string
	Vendors              map[string]*vendorPriceComparisonVendorAggregate
	LastPurchaseVendor   string
	LastPurchasePrice    float64
	LastPurchaseDate     time.Time
	LastPurchaseID       uint
	LastPurchaseQuantity int
	SearchVendors        map[string]struct{}
}

func GetVendorPriceComparisonReport(c *gin.Context) {
	if err := services.EnsurePOSFinanceDocumentSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	comparisonMode := normalizeVendorPriceComparisonMode(c.Query("comparison_mode"))
	minPurchaseCount := 1
	if raw := strings.TrimSpace(c.Query("min_purchase_count")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			minPurchaseCount = parsed
		}
	}

	filterProductID := parseUintQuery(c.Query("product_id"))
	filterCategoryID := parseUintQuery(c.Query("category_id"))
	filterItemType := strings.ToLower(strings.TrimSpace(c.Query("item_type")))
	search := strings.ToLower(strings.TrimSpace(c.Query("search")))

	query := database.DB.
		Model(&models.Purchase{}).
		Preload("PurchaseItems.Product").
		Order("purchase_date DESC, id DESC")

	if outletID := strings.TrimSpace(c.Query("outlet_id")); outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}
	if vendor := strings.TrimSpace(c.Query("vendor")); vendor != "" {
		query = query.Where("supplier_name ILIKE ?", "%"+vendor+"%")
	}
	if paymentMethod := strings.ToLower(strings.TrimSpace(c.Query("payment_method"))); paymentMethod != "" {
		query = query.Where("LOWER(COALESCE(payment_method, '')) = ?", paymentMethod)
	}
	if paymentStatus := strings.ToLower(strings.TrimSpace(c.Query("payment_status"))); paymentStatus != "" {
		query = query.Where("LOWER(COALESCE(payment_status, '')) = ?", paymentStatus)
	}
	if startDate := strings.TrimSpace(c.Query("start_date")); startDate != "" {
		if parsed, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("DATE(purchase_date) >= ?", parsed.Format("2006-01-02"))
		}
	}
	if endDate := strings.TrimSpace(c.Query("end_date")); endDate != "" {
		if parsed, err := time.Parse("2006-01-02", endDate); err == nil {
			query = query.Where("DATE(purchase_date) <= ?", parsed.Format("2006-01-02"))
		}
	}

	var purchases []models.Purchase
	if err := query.Find(&purchases).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil histori purchase vendor"})
		return
	}

	productAggregates := make(map[uint]*vendorPriceComparisonProductAggregate)
	categoryIDs := make(map[uint]struct{})

	for _, purchase := range purchases {
		for _, item := range purchase.PurchaseItems {
			if item.ProductID == 0 {
				continue
			}

			product := item.Product
			if filterProductID != 0 && item.ProductID != filterProductID {
				continue
			}
			if filterCategoryID != 0 && product.CategoryID != filterCategoryID {
				continue
			}
			if !vendorPriceItemTypeAllowed(product.ItemType, filterItemType) {
				continue
			}

			quantity := item.Quantity
			if quantity <= 0 {
				continue
			}

			purchasePrice := item.PurchasePrice
			if purchasePrice <= 0 && item.Total > 0 {
				purchasePrice = item.Total / float64(quantity)
			}
			if purchasePrice <= 0 {
				continue
			}

			totalAmount := item.Total
			if totalAmount <= 0 {
				totalAmount = purchasePrice * float64(quantity)
			}

			vendorKey, vendorLabel := normalizeVendorName(purchase.SupplierName)
			productName := strings.TrimSpace(product.Name)
			if productName == "" {
				productName = "Product #" + strconv.FormatUint(uint64(item.ProductID), 10)
			}
			unit := strings.TrimSpace(product.Satuan)
			if unit == "" {
				unit = "-"
			}

			productAggregate := productAggregates[item.ProductID]
			if productAggregate == nil {
				productAggregate = &vendorPriceComparisonProductAggregate{
					ProductID:     item.ProductID,
					ProductName:   productName,
					CategoryID:    product.CategoryID,
					ItemType:      product.ItemType,
					Unit:          unit,
					Vendors:       make(map[string]*vendorPriceComparisonVendorAggregate),
					SearchVendors: make(map[string]struct{}),
				}
				productAggregates[item.ProductID] = productAggregate
			}

			categoryIDs[product.CategoryID] = struct{}{}
			productAggregate.SearchVendors[vendorLabel] = struct{}{}

			vendorAggregate := productAggregate.Vendors[vendorKey]
			if vendorAggregate == nil {
				vendorAggregate = &vendorPriceComparisonVendorAggregate{
					Key:            vendorKey,
					Label:          vendorLabel,
					LowestPrice:    purchasePrice,
					HighestPrice:   purchasePrice,
					PaymentMethods: make(map[string]struct{}),
					OutletIDs:      make(map[uint]struct{}),
				}
				productAggregate.Vendors[vendorKey] = vendorAggregate
			}

			vendorAggregate.PurchaseCount++
			vendorAggregate.TotalQuantity += quantity
			vendorAggregate.TotalAmount += totalAmount
			vendorAggregate.OutletIDs[purchase.OutletID] = struct{}{}

			paymentMethod := strings.TrimSpace(purchase.PaymentMethod)
			if paymentMethod == "" {
				paymentMethod = "cash"
			}
			vendorAggregate.PaymentMethods[paymentMethod] = struct{}{}

			if purchasePrice < vendorAggregate.LowestPrice {
				vendorAggregate.LowestPrice = purchasePrice
			}
			if purchasePrice > vendorAggregate.HighestPrice {
				vendorAggregate.HighestPrice = purchasePrice
			}
			if vendorAggregate.LatestPurchaseAt.IsZero() ||
				purchase.PurchaseDate.After(vendorAggregate.LatestPurchaseAt) ||
				(purchase.PurchaseDate.Equal(vendorAggregate.LatestPurchaseAt) && purchase.ID > vendorAggregate.LatestPurchaseID) {
				vendorAggregate.LatestPurchaseAt = purchase.PurchaseDate
				vendorAggregate.LatestPurchaseID = purchase.ID
				vendorAggregate.LatestPrice = purchasePrice
				vendorAggregate.LastPurchaseQty = quantity
				if vendorAggregate.Label == "Unknown vendor" || strings.EqualFold(vendorAggregate.Label, vendorKey) {
					vendorAggregate.Label = vendorLabel
				}
			}

			if productAggregate.LastPurchaseDate.IsZero() ||
				purchase.PurchaseDate.After(productAggregate.LastPurchaseDate) ||
				(purchase.PurchaseDate.Equal(productAggregate.LastPurchaseDate) && purchase.ID > productAggregate.LastPurchaseID) {
				productAggregate.LastPurchaseDate = purchase.PurchaseDate
				productAggregate.LastPurchaseID = purchase.ID
				productAggregate.LastPurchaseVendor = vendorLabel
				productAggregate.LastPurchasePrice = purchasePrice
				productAggregate.LastPurchaseQuantity = quantity
			}
		}
	}

	categoryNames := loadCategoryNames(categoryIDs)
	for _, aggregate := range productAggregates {
		aggregate.CategoryName = resolveCategoryName(categoryNames, aggregate.CategoryID)
	}

	response := buildVendorPriceComparisonResponse(productAggregates, comparisonMode, minPurchaseCount, search)
	c.JSON(http.StatusOK, response)
}

func buildVendorPriceComparisonResponse(
	productAggregates map[uint]*vendorPriceComparisonProductAggregate,
	comparisonMode string,
	minPurchaseCount int,
	search string,
) gin.H {
	summary := vendorPriceComparisonSummary{}
	vendorWinBreakdown := make(map[string]*vendorPriceComparisonBreakdownItem)
	categorySpreadBreakdown := make(map[string]*vendorPriceComparisonBreakdownItem)
	activeVendors := make(map[string]struct{})
	preferredVendors := make(map[string]struct{})

	productRows := make([]vendorPriceComparisonProductRow, 0, len(productAggregates))
	vendorDetailRows := make([]vendorPriceComparisonVendorDetailRow, 0)

	for _, productAggregate := range productAggregates {
		rankedVendors := make([]*vendorPriceComparisonVendorAggregate, 0, len(productAggregate.Vendors))
		for _, vendorAggregate := range productAggregate.Vendors {
			if vendorAggregate.PurchaseCount < minPurchaseCount {
				continue
			}
			if vendorAggregate.ComparisonPrice(comparisonMode) <= 0 {
				continue
			}
			rankedVendors = append(rankedVendors, vendorAggregate)
		}

		if len(rankedVendors) == 0 {
			continue
		}

		sort.Slice(rankedVendors, func(left, right int) bool {
			leftPrice := rankedVendors[left].ComparisonPrice(comparisonMode)
			rightPrice := rankedVendors[right].ComparisonPrice(comparisonMode)
			if !floatEquals(leftPrice, rightPrice) {
				return leftPrice < rightPrice
			}
			if rankedVendors[left].PurchaseCount != rankedVendors[right].PurchaseCount {
				return rankedVendors[left].PurchaseCount > rankedVendors[right].PurchaseCount
			}
			if !rankedVendors[left].LatestPurchaseAt.Equal(rankedVendors[right].LatestPurchaseAt) {
				return rankedVendors[left].LatestPurchaseAt.After(rankedVendors[right].LatestPurchaseAt)
			}
			return rankedVendors[left].Label < rankedVendors[right].Label
		})

		preferredVendor := rankedVendors[0]
		secondVendor := (*vendorPriceComparisonVendorAggregate)(nil)
		if len(rankedVendors) > 1 {
			secondVendor = rankedVendors[1]
		}

		totalQuantity := 0
		totalAmount := 0.0
		totalPurchaseCount := 0
		for rankIndex, vendorAggregate := range rankedVendors {
			totalQuantity += vendorAggregate.TotalQuantity
			totalAmount += vendorAggregate.TotalAmount
			totalPurchaseCount += vendorAggregate.PurchaseCount
			activeVendors[vendorAggregate.Key] = struct{}{}

			outletIDs := mapKeysUint(vendorAggregate.OutletIDs)
			paymentMix := mapKeysString(vendorAggregate.PaymentMethods)
			vendorDetailRows = append(vendorDetailRows, vendorPriceComparisonVendorDetailRow{
				ProductID:           productAggregate.ProductID,
				ProductName:         productAggregate.ProductName,
				CategoryName:        productAggregate.CategoryName,
				ItemType:            productAggregate.ItemType,
				Unit:                productAggregate.Unit,
				VendorName:          vendorAggregate.Label,
				ComparisonPrice:     vendorAggregate.ComparisonPrice(comparisonMode),
				LatestPrice:         vendorAggregate.LatestPrice,
				AveragePrice:        vendorAggregate.AveragePrice(),
				LowestPrice:         vendorAggregate.LowestPrice,
				HighestPrice:        vendorAggregate.HighestPrice,
				PurchaseCount:       vendorAggregate.PurchaseCount,
				TotalQuantity:       vendorAggregate.TotalQuantity,
				LastPurchaseDate:    vendorAggregate.LatestPurchaseAt,
				PaymentMix:          paymentMix,
				PaymentMixLabel:     strings.Join(paymentMix, ", "),
				OutletCoverageCount: len(outletIDs),
				OutletIDs:           outletIDs,
				Rank:                rankIndex + 1,
				IsPreferred:         rankIndex == 0,
			})
		}

		averageMarketPrice := 0.0
		if totalQuantity > 0 {
			averageMarketPrice = totalAmount / float64(totalQuantity)
		}

		preferredPrice := preferredVendor.ComparisonPrice(comparisonMode)
		secondPrice := 0.0
		secondVendorLabel := ""
		priceSpread := 0.0
		spreadPercentage := 0.0
		if secondVendor != nil {
			secondPrice = secondVendor.ComparisonPrice(comparisonMode)
			secondVendorLabel = secondVendor.Label
			priceSpread = secondPrice - preferredPrice
			if preferredPrice > 0 {
				spreadPercentage = (priceSpread / preferredPrice) * 100
			}
		}

		potentialSavings := 0.0
		if productAggregate.LastPurchasePrice > preferredPrice && productAggregate.LastPurchaseQuantity > 0 {
			potentialSavings = (productAggregate.LastPurchasePrice - preferredPrice) * float64(productAggregate.LastPurchaseQuantity)
		}

		riskFlag, riskLabel := computeVendorPriceRisk(productAggregate.LastPurchaseDate, len(rankedVendors), spreadPercentage, preferredVendor, secondVendor)
		row := vendorPriceComparisonProductRow{
			ProductID:           productAggregate.ProductID,
			ProductName:         productAggregate.ProductName,
			CategoryID:          productAggregate.CategoryID,
			CategoryName:        productAggregate.CategoryName,
			ItemType:            productAggregate.ItemType,
			Unit:                productAggregate.Unit,
			PreferredVendor:     preferredVendor.Label,
			PreferredPrice:      preferredPrice,
			SecondVendor:        secondVendorLabel,
			SecondPrice:         secondPrice,
			PriceSpread:         math.Max(priceSpread, 0),
			SpreadPercentage:    math.Max(spreadPercentage, 0),
			LastPurchasedVendor: productAggregate.LastPurchaseVendor,
			LastPurchasedPrice:  productAggregate.LastPurchasePrice,
			AverageMarketPrice:  averageMarketPrice,
			VendorCount:         len(rankedVendors),
			PurchaseCount:       totalPurchaseCount,
			TotalQuantityBought: totalQuantity,
			LastPurchaseDate:    productAggregate.LastPurchaseDate,
			PotentialSavings:    potentialSavings,
			RecentPurchaseQty:   productAggregate.LastPurchaseQuantity,
			RiskFlag:            riskFlag,
			RiskLabel:           riskLabel,
		}

		if search != "" && !vendorPriceProductMatchesSearch(row, productAggregate, search) {
			continue
		}

		productRows = append(productRows, row)
		preferredVendors[preferredVendor.Key] = struct{}{}

		vendorEntry := vendorWinBreakdown[preferredVendor.Key]
		if vendorEntry == nil {
			vendorEntry = &vendorPriceComparisonBreakdownItem{
				Key:   preferredVendor.Key,
				Label: preferredVendor.Label,
			}
			vendorWinBreakdown[preferredVendor.Key] = vendorEntry
		}
		vendorEntry.Count++
		vendorEntry.Amount += potentialSavings
		vendorEntry.Quantity += totalQuantity

		categoryKey := strconv.FormatUint(uint64(productAggregate.CategoryID), 10)
		if categoryKey == "0" {
			categoryKey = strings.ToLower(strings.TrimSpace(productAggregate.CategoryName))
		}
		categoryEntry := categorySpreadBreakdown[categoryKey]
		if categoryEntry == nil {
			categoryEntry = &vendorPriceComparisonBreakdownItem{
				Key:   categoryKey,
				Label: productAggregate.CategoryName,
			}
			if categoryEntry.Label == "" {
				categoryEntry.Label = "Tanpa kategori"
			}
			categorySpreadBreakdown[categoryKey] = categoryEntry
		}
		categoryEntry.Count++
		categoryEntry.Amount += potentialSavings
		categoryEntry.Quantity += totalQuantity
	}

	productIDs := make(map[uint]struct{}, len(productRows))
	for _, row := range productRows {
		productIDs[row.ProductID] = struct{}{}
		summary.ProductsCompared++
		summary.PotentialSavings += row.PotentialSavings
		if row.VendorCount <= 1 {
			summary.SingleSourceProducts++
		}
		if row.SpreadPercentage >= vendorPriceHighSpreadThreshold {
			summary.HighSpreadProducts++
		}
	}

	filteredVendorDetails := make([]vendorPriceComparisonVendorDetailRow, 0, len(vendorDetailRows))
	for _, row := range vendorDetailRows {
		if _, exists := productIDs[row.ProductID]; exists {
			filteredVendorDetails = append(filteredVendorDetails, row)
		}
	}

	sort.Slice(productRows, func(left, right int) bool {
		if !floatEquals(productRows[left].PotentialSavings, productRows[right].PotentialSavings) {
			return productRows[left].PotentialSavings > productRows[right].PotentialSavings
		}
		if !floatEquals(productRows[left].SpreadPercentage, productRows[right].SpreadPercentage) {
			return productRows[left].SpreadPercentage > productRows[right].SpreadPercentage
		}
		return productRows[left].ProductName < productRows[right].ProductName
	})

	sort.Slice(filteredVendorDetails, func(left, right int) bool {
		if filteredVendorDetails[left].ProductName == filteredVendorDetails[right].ProductName {
			return filteredVendorDetails[left].Rank < filteredVendorDetails[right].Rank
		}
		return filteredVendorDetails[left].ProductName < filteredVendorDetails[right].ProductName
	})

	summary.ActiveVendors = countActiveVendorsFromRows(filteredVendorDetails)
	summary.CheapestVendorWins = len(preferredVendors)

	topGapProducts := make([]vendorPriceComparisonProductRow, len(productRows))
	copy(topGapProducts, productRows)
	sort.Slice(topGapProducts, func(left, right int) bool {
		if !floatEquals(topGapProducts[left].PriceSpread, topGapProducts[right].PriceSpread) {
			return topGapProducts[left].PriceSpread > topGapProducts[right].PriceSpread
		}
		if !floatEquals(topGapProducts[left].PotentialSavings, topGapProducts[right].PotentialSavings) {
			return topGapProducts[left].PotentialSavings > topGapProducts[right].PotentialSavings
		}
		return topGapProducts[left].ProductName < topGapProducts[right].ProductName
	})
	if len(topGapProducts) > 10 {
		topGapProducts = topGapProducts[:10]
	}

	return gin.H{
		"summary":                   summary,
		"comparison_mode":           comparisonMode,
		"vendor_win_breakdown":      flattenVendorPriceBreakdown(vendorWinBreakdown),
		"category_spread_breakdown": flattenVendorPriceBreakdown(categorySpreadBreakdown),
		"top_price_gap_products":    topGapProducts,
		"results":                   productRows,
		"vendor_details":            filteredVendorDetails,
		"total":                     len(productRows),
	}
}

func normalizeVendorPriceComparisonMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case vendorPriceModeLatest:
		return vendorPriceModeLatest
	case vendorPriceModeLowest:
		return vendorPriceModeLowest
	default:
		return vendorPriceModeAverage
	}
}

func parseUintQuery(value string) uint {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return uint(parsed)
}

func vendorPriceItemTypeAllowed(itemType string, filterItemType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(itemType))
	if filterItemType != "" {
		return normalized == filterItemType
	}
	return normalized != "service"
}

func normalizeVendorName(name string) (string, string) {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(name)), " ")
	if trimmed == "" {
		return "unknown-vendor", "Unknown vendor"
	}
	return strings.ToLower(trimmed), trimmed
}

func loadCategoryNames(categoryIDs map[uint]struct{}) map[uint]string {
	if len(categoryIDs) == 0 {
		return map[uint]string{}
	}

	ids := make([]uint, 0, len(categoryIDs))
	for id := range categoryIDs {
		if id == 0 {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return map[uint]string{}
	}

	var categories []models.Category
	if err := database.DB.Where("id IN ?", ids).Find(&categories).Error; err != nil {
		return map[uint]string{}
	}

	result := make(map[uint]string, len(categories))
	for _, category := range categories {
		label := strings.TrimSpace(category.CategoryName)
		if label == "" {
			label = "Category #" + strconv.FormatUint(uint64(category.ID), 10)
		}
		result[category.ID] = label
	}
	return result
}

func resolveCategoryName(categoryNames map[uint]string, categoryID uint) string {
	if categoryID == 0 {
		return "Tanpa kategori"
	}
	if label, exists := categoryNames[categoryID]; exists && strings.TrimSpace(label) != "" {
		return label
	}
	return "Category #" + strconv.FormatUint(uint64(categoryID), 10)
}

func computeVendorPriceRisk(
	lastPurchaseDate time.Time,
	vendorCount int,
	spreadPercentage float64,
	preferredVendor *vendorPriceComparisonVendorAggregate,
	secondVendor *vendorPriceComparisonVendorAggregate,
) (string, string) {
	if vendorCount <= 1 {
		return "single_source", "Single source"
	}

	if !lastPurchaseDate.IsZero() && time.Since(lastPurchaseDate) > 90*24*time.Hour {
		return "stale_data", "Stale data"
	}

	if spreadPercentage >= vendorPriceHighSpreadThreshold {
		return "price_gap_high", "Price gap high"
	}

	if preferredVendor != nil && secondVendor != nil && preferredVendor.PurchaseCount >= 2 {
		return "stable_best_vendor", "Stable best vendor"
	}

	return "watchlist", "Perlu evaluasi"
}

func vendorPriceProductMatchesSearch(
	row vendorPriceComparisonProductRow,
	aggregate *vendorPriceComparisonProductAggregate,
	search string,
) bool {
	candidates := []string{
		strconv.FormatUint(uint64(row.ProductID), 10),
		row.ProductName,
		row.CategoryName,
		row.ItemType,
		row.Unit,
		row.PreferredVendor,
		row.SecondVendor,
		row.LastPurchasedVendor,
		row.RiskLabel,
	}

	for vendorLabel := range aggregate.SearchVendors {
		candidates = append(candidates, vendorLabel)
	}

	for _, candidate := range candidates {
		if strings.Contains(strings.ToLower(strings.TrimSpace(candidate)), search) {
			return true
		}
	}

	return false
}

func flattenVendorPriceBreakdown(items map[string]*vendorPriceComparisonBreakdownItem) []vendorPriceComparisonBreakdownItem {
	results := make([]vendorPriceComparisonBreakdownItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		results = append(results, *item)
	}

	sort.Slice(results, func(left, right int) bool {
		if !floatEquals(results[left].Amount, results[right].Amount) {
			return results[left].Amount > results[right].Amount
		}
		if results[left].Count != results[right].Count {
			return results[left].Count > results[right].Count
		}
		return results[left].Label < results[right].Label
	})

	return results
}

func mapKeysString(values map[string]struct{}) []string {
	results := make([]string, 0, len(values))
	for value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		results = append(results, value)
	}
	sort.Strings(results)
	return results
}

func mapKeysUint(values map[uint]struct{}) []uint {
	results := make([]uint, 0, len(values))
	for value := range values {
		if value == 0 {
			continue
		}
		results = append(results, value)
	}
	sort.Slice(results, func(left, right int) bool {
		return results[left] < results[right]
	})
	return results
}

func countActiveVendorsFromRows(rows []vendorPriceComparisonVendorDetailRow) int {
	unique := make(map[string]struct{})
	for _, row := range rows {
		key := strings.ToLower(strings.TrimSpace(row.VendorName))
		if key == "" {
			continue
		}
		unique[key] = struct{}{}
	}
	return len(unique)
}

func floatEquals(left, right float64) bool {
	return math.Abs(left-right) < 0.000001
}
