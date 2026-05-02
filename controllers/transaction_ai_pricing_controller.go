package controllers

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/services"
	"gorm.io/gorm"
)

type posPricingAssistantSummary struct {
	TransactionsAnalyzed int     `json:"transactions_analyzed"`
	UniqueProducts       int     `json:"unique_products"`
	GrossSales           float64 `json:"gross_sales"`
	AverageTicket        float64 `json:"average_ticket"`
	TotalItemQuantity    int     `json:"total_item_quantity"`
	DominantTimeSlot     string  `json:"dominant_time_slot"`
}

type posBundlingRecommendation struct {
	RecommendationKey                         string  `json:"recommendation_key"`
	PrimaryProductID                          uint    `json:"primary_product_id"`
	PrimaryProductName                        string  `json:"primary_product_name"`
	PairedProductID                           uint    `json:"paired_product_id"`
	PairedProductName                         string  `json:"paired_product_name"`
	JointTransactionCount                     int     `json:"joint_transaction_count"`
	AttachRatePercent                         float64 `json:"attach_rate_percent"`
	AverageBundleValue                        float64 `json:"average_bundle_value"`
	SuggestedDiscountPercent                  float64 `json:"suggested_discount_percent"`
	SuggestedBundlePrice                      float64 `json:"suggested_bundle_price"`
	Reason                                    string  `json:"reason"`
	ConfidenceLabel                           string  `json:"confidence_label"`
	FeedbackAcceptedCount                     int     `json:"feedback_accepted_count,omitempty"`
	FeedbackIgnoredCount                      int     `json:"feedback_ignored_count,omitempty"`
	FeedbackCampaignUsedCount                 int     `json:"feedback_campaign_used_count,omitempty"`
	FeedbackSalesImprovedCount                int     `json:"feedback_sales_improved_count,omitempty"`
	FeedbackEstimatedRevenueLiftTotal         float64 `json:"feedback_estimated_revenue_lift_total,omitempty"`
	FeedbackObservedRevenueLiftTotal          float64 `json:"feedback_observed_revenue_lift_total,omitempty"`
	FeedbackAverageTransactionLiftPercent     float64 `json:"feedback_average_transaction_lift_percent,omitempty"`
	FeedbackAutoAttributedCampaignCount       int     `json:"feedback_auto_attributed_campaign_count,omitempty"`
	FeedbackAutoObservedRevenueLiftTotal      float64 `json:"feedback_auto_observed_revenue_lift_total,omitempty"`
	FeedbackAutoAverageTransactionLiftPercent float64 `json:"feedback_auto_average_transaction_lift_percent,omitempty"`
	FeedbackLastAction                        string  `json:"feedback_last_action,omitempty"`
	rankingScore                              float64
}

type posPromoCandidate struct {
	RecommendationKey                         string  `json:"recommendation_key"`
	StrategyKey                               string  `json:"strategy_key"`
	StrategyLabel                             string  `json:"strategy_label"`
	FocusType                                 string  `json:"focus_type"`
	FocusProductID                            uint    `json:"focus_product_id,omitempty"`
	FocusProductName                          string  `json:"focus_product_name,omitempty"`
	TimeSlotKey                               string  `json:"time_slot_key,omitempty"`
	TimeSlotLabel                             string  `json:"time_slot_label,omitempty"`
	SupportingTransactions                    int     `json:"supporting_transactions"`
	CurrentRevenue                            float64 `json:"current_revenue"`
	SuggestedDiscountPercent                  float64 `json:"suggested_discount_percent"`
	SuggestedPrice                            float64 `json:"suggested_price"`
	ExpectedLiftPercent                       float64 `json:"expected_lift_percent"`
	Reason                                    string  `json:"reason"`
	ConfidenceLabel                           string  `json:"confidence_label"`
	FeedbackAcceptedCount                     int     `json:"feedback_accepted_count,omitempty"`
	FeedbackIgnoredCount                      int     `json:"feedback_ignored_count,omitempty"`
	FeedbackCampaignUsedCount                 int     `json:"feedback_campaign_used_count,omitempty"`
	FeedbackSalesImprovedCount                int     `json:"feedback_sales_improved_count,omitempty"`
	FeedbackEstimatedRevenueLiftTotal         float64 `json:"feedback_estimated_revenue_lift_total,omitempty"`
	FeedbackObservedRevenueLiftTotal          float64 `json:"feedback_observed_revenue_lift_total,omitempty"`
	FeedbackAverageTransactionLiftPercent     float64 `json:"feedback_average_transaction_lift_percent,omitempty"`
	FeedbackAutoAttributedCampaignCount       int     `json:"feedback_auto_attributed_campaign_count,omitempty"`
	FeedbackAutoObservedRevenueLiftTotal      float64 `json:"feedback_auto_observed_revenue_lift_total,omitempty"`
	FeedbackAutoAverageTransactionLiftPercent float64 `json:"feedback_auto_average_transaction_lift_percent,omitempty"`
	FeedbackLastAction                        string  `json:"feedback_last_action,omitempty"`
	rankingScore                              float64
}

type posPricingAssistantReport struct {
	Summary                 posPricingAssistantSummary  `json:"summary"`
	BundlingRecommendations []posBundlingRecommendation `json:"bundling_recommendations"`
	PromoCandidates         []posPromoCandidate         `json:"promo_candidates"`
}

type posPricingProductAggregate struct {
	ID               uint
	Name             string
	Transactions     int
	Quantity         int
	Revenue          float64
	AverageUnitPrice float64
}

type posPricingTimeSlotAggregate struct {
	Key          string
	Label        string
	Transactions int
	Revenue      float64
}

type posPricingPairAggregate struct {
	LeftID        uint
	LeftName      string
	RightID       uint
	RightName     string
	Count         int
	CombinedValue float64
}

type posPricingLineAggregate struct {
	ProductID uint
	Name      string
	Quantity  int
	Revenue   float64
}

type posPricingFeedbackRequest struct {
	OutletID                       uint    `json:"outlet_id"`
	RecommendationType             string  `json:"recommendation_type"`
	RecommendationKey              string  `json:"recommendation_key"`
	FeedbackAction                 string  `json:"feedback_action"`
	Note                           string  `json:"note"`
	EstimatedRevenueLift           float64 `json:"estimated_revenue_lift"`
	ObservedRevenueLift            float64 `json:"observed_revenue_lift"`
	ObservedTransactionLiftPercent float64 `json:"observed_transaction_lift_percent"`
	CampaignStartDate              string  `json:"campaign_start_date"`
	CampaignEndDate                string  `json:"campaign_end_date"`
}

func GetPOSPricingAssistant(c *gin.Context) {
	if err := services.EnsurePOSApprovalSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := services.EnsureAIPricingFeedbackSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	query := database.DB.
		Preload("Items.Product").
		Model(&models.Transaction{})
	query = applyNonVoidedTransactionFilter(query)

	var scopedOutletID uint
	if startDateStr := strings.TrimSpace(c.Query("start_date")); startDateStr != "" {
		if parsed, err := parseTransactionDateOnly(startDateStr); err == nil {
			query = query.Where("created_at >= ?", parsed)
		}
	}
	if endDateStr := strings.TrimSpace(c.Query("end_date")); endDateStr != "" {
		if parsed, err := parseTransactionDateOnly(endDateStr); err == nil {
			query = query.Where("created_at < ?", parsed.AddDate(0, 0, 1))
		}
	}
	if outletID := strings.TrimSpace(c.Query("outlet_id")); outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
		if parsed, err := strconv.ParseUint(outletID, 10, 64); err == nil {
			scopedOutletID = uint(parsed)
		}
	}
	if paymentMethod := strings.TrimSpace(c.Query("payment_method")); paymentMethod != "" {
		query = query.Where("payment_method = ?", paymentMethod)
	}

	var transactions []models.Transaction
	if err := query.Order("created_at desc").Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data pricing assistant POS"})
		return
	}

	if scopedOutletID == 0 {
		scopedOutletID = resolvePOSPricingScopedOutletID(transactions)
	}

	report := buildPOSPricingAssistantReport(transactions, nil)

	feedbackSummaries, err := services.GetAIPricingFeedbackSummaries(
		database.DB,
		scopedOutletID,
		collectPOSPricingRecommendationKeys(report),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil feedback AI pricing POS"})
		return
	}
	if scopedOutletID > 0 {
		if err := applyPOSPricingAutomaticAttribution(database.DB, scopedOutletID, feedbackSummaries); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghitung auto-attribution AI pricing POS"})
			return
		}
	}

	report = buildPOSPricingAssistantReport(transactions, feedbackSummaries)
	c.JSON(http.StatusOK, report)
}

func SubmitPOSPricingAssistantFeedback(c *gin.Context) {
	if err := services.EnsureAIPricingFeedbackSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var payload posPricingFeedbackRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payload feedback AI pricing tidak valid"})
		return
	}

	payload.RecommendationKey = strings.TrimSpace(payload.RecommendationKey)
	payload.RecommendationType = strings.TrimSpace(payload.RecommendationType)
	payload.FeedbackAction = services.NormalizeAIRecommendationFeedbackAction(payload.FeedbackAction)
	if payload.OutletID == 0 || payload.RecommendationKey == "" || payload.RecommendationType == "" || payload.FeedbackAction == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outlet_id, recommendation_type, recommendation_key, dan feedback_action wajib diisi"})
		return
	}

	campaignStartDate, err := parsePOSPricingFeedbackDateOnly(payload.CampaignStartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "campaign_start_date tidak valid"})
		return
	}
	campaignEndDate, err := parsePOSPricingFeedbackDateOnly(payload.CampaignEndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "campaign_end_date tidak valid"})
		return
	}
	if campaignStartDate != nil && campaignEndDate != nil && campaignEndDate.Before(*campaignStartDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "campaign_end_date tidak boleh lebih kecil dari campaign_start_date"})
		return
	}

	actor := readApprovalActorSnapshot(c)
	if err := services.UpsertAIPricingFeedback(database.DB, services.AIPricingFeedbackInput{
		OutletID:                       payload.OutletID,
		RecommendationType:             payload.RecommendationType,
		RecommendationKey:              payload.RecommendationKey,
		FeedbackAction:                 payload.FeedbackAction,
		ActorUserID:                    actor.UserID,
		ActorType:                      actor.ActorType,
		ActorName:                      actor.Name,
		Note:                           payload.Note,
		EstimatedRevenueLift:           payload.EstimatedRevenueLift,
		ObservedRevenueLift:            payload.ObservedRevenueLift,
		ObservedTransactionLiftPercent: payload.ObservedTransactionLiftPercent,
		CampaignStartDate:              campaignStartDate,
		CampaignEndDate:                campaignEndDate,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan feedback AI pricing POS"})
		return
	}

	summaries, err := services.GetAIPricingFeedbackSummaries(database.DB, payload.OutletID, []string{payload.RecommendationKey})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Feedback tersimpan, tetapi ringkasan feedback gagal dibaca"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Feedback AI pricing POS tersimpan",
		"data":    summaries[payload.RecommendationKey],
	})
}

func parsePOSPricingFeedbackDateOnly(raw string) (*time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", trimmed, time.Local)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

type posPricingAutoAttributionAccumulator struct {
	CampaignCount       int
	ObservedRevenueLift float64
	TransactionLiftSum  float64
}

func buildPOSPricingAssistantReport(
	transactions []models.Transaction,
	feedbackSummaries map[string]services.AIPricingFeedbackSummary,
) posPricingAssistantReport {
	productAggregates := make(map[uint]*posPricingProductAggregate)
	timeSlotAggregates := make(map[string]*posPricingTimeSlotAggregate)
	timeSlotProductCounts := make(map[string]map[uint]int)
	pairAggregates := make(map[string]*posPricingPairAggregate)

	totalGrossSales := 0.0
	totalItemQuantity := 0

	for _, transaction := range transactions {
		transactionValue := getPOSPricingTransactionValue(transaction)
		slotKey := getPOSPricingTimeSlotKey(transaction.CreatedAt)
		slotLabel := getPOSPricingTimeSlotLabel(slotKey)

		slotEntry := timeSlotAggregates[slotKey]
		if slotEntry == nil {
			slotEntry = &posPricingTimeSlotAggregate{Key: slotKey, Label: slotLabel}
			timeSlotAggregates[slotKey] = slotEntry
		}
		slotEntry.Transactions++
		slotEntry.Revenue += transactionValue

		lineMap := make(map[uint]*posPricingLineAggregate)
		for _, item := range transaction.Items {
			if item.ProductID == 0 {
				continue
			}

			productName := strings.TrimSpace(item.Product.Name)
			if productName == "" {
				productName = fmt.Sprintf("Product #%d", item.ProductID)
			}

			line := lineMap[item.ProductID]
			if line == nil {
				line = &posPricingLineAggregate{
					ProductID: item.ProductID,
					Name:      productName,
				}
				lineMap[item.ProductID] = line
			}
			line.Quantity += item.Quantity
			line.Revenue += item.Total
			totalItemQuantity += item.Quantity
		}

		productIDs := make([]uint, 0, len(lineMap))
		for productID, line := range lineMap {
			productIDs = append(productIDs, productID)

			entry := productAggregates[productID]
			if entry == nil {
				entry = &posPricingProductAggregate{
					ID:   productID,
					Name: line.Name,
				}
				productAggregates[productID] = entry
			}
			entry.Transactions++
			entry.Quantity += line.Quantity
			entry.Revenue += line.Revenue
			if entry.Quantity > 0 {
				entry.AverageUnitPrice = entry.Revenue / float64(entry.Quantity)
			}

			slotProducts := timeSlotProductCounts[slotKey]
			if slotProducts == nil {
				slotProducts = make(map[uint]int)
				timeSlotProductCounts[slotKey] = slotProducts
			}
			slotProducts[productID] += 1
		}

		sort.Slice(productIDs, func(i, j int) bool {
			return productIDs[i] < productIDs[j]
		})
		for leftIndex := 0; leftIndex < len(productIDs); leftIndex++ {
			for rightIndex := leftIndex + 1; rightIndex < len(productIDs); rightIndex++ {
				leftLine := lineMap[productIDs[leftIndex]]
				rightLine := lineMap[productIDs[rightIndex]]
				pairKey := fmt.Sprintf("%d:%d", productIDs[leftIndex], productIDs[rightIndex])
				entry := pairAggregates[pairKey]
				if entry == nil {
					entry = &posPricingPairAggregate{
						LeftID:    productIDs[leftIndex],
						LeftName:  leftLine.Name,
						RightID:   productIDs[rightIndex],
						RightName: rightLine.Name,
					}
					pairAggregates[pairKey] = entry
				}
				entry.Count++
				entry.CombinedValue += leftLine.Revenue + rightLine.Revenue
			}
		}

		totalGrossSales += transactionValue
	}

	bundles := buildPOSBundlingRecommendations(productAggregates, pairAggregates)
	promos := buildPOSPromoCandidates(transactions, productAggregates, timeSlotAggregates, timeSlotProductCounts, bundles)
	applyPOSPricingFeedbackSummaries(bundles, promos, feedbackSummaries)
	if len(bundles) > 4 {
		bundles = bundles[:4]
	}

	dominantTimeSlot := "-"
	if len(timeSlotAggregates) > 0 {
		best := make([]*posPricingTimeSlotAggregate, 0, len(timeSlotAggregates))
		for _, item := range timeSlotAggregates {
			best = append(best, item)
		}
		sort.Slice(best, func(i, j int) bool {
			if best[i].Transactions != best[j].Transactions {
				return best[i].Transactions > best[j].Transactions
			}
			return best[i].Revenue > best[j].Revenue
		})
		dominantTimeSlot = best[0].Label
	}

	return posPricingAssistantReport{
		Summary: posPricingAssistantSummary{
			TransactionsAnalyzed: len(transactions),
			UniqueProducts:       len(productAggregates),
			GrossSales:           totalGrossSales,
			AverageTicket: func() float64 {
				if len(transactions) == 0 {
					return 0
				}
				return totalGrossSales / float64(len(transactions))
			}(),
			TotalItemQuantity: totalItemQuantity,
			DominantTimeSlot:  dominantTimeSlot,
		},
		BundlingRecommendations: bundles,
		PromoCandidates:         promos,
	}
}

func buildPOSBundlingRecommendations(
	productAggregates map[uint]*posPricingProductAggregate,
	pairAggregates map[string]*posPricingPairAggregate,
) []posBundlingRecommendation {
	recommendations := make([]posBundlingRecommendation, 0, len(pairAggregates))

	for _, pair := range pairAggregates {
		left := productAggregates[pair.LeftID]
		right := productAggregates[pair.RightID]
		if pair.Count < 2 || left == nil || right == nil {
			continue
		}

		primary := left
		paired := right
		if right.Transactions > left.Transactions || (right.Transactions == left.Transactions && right.Revenue > left.Revenue) {
			primary = right
			paired = left
		}
		if primary.Transactions <= 0 {
			continue
		}

		attachRate := (float64(pair.Count) / float64(primary.Transactions)) * 100
		averageBundleValue := pair.CombinedValue / float64(pair.Count)
		discountPercent := 5.0
		switch {
		case attachRate < 20:
			discountPercent = 10
		case attachRate < 35:
			discountPercent = 8
		}

		score := float64(pair.Count*12) + attachRate + minFloat(averageBundleValue/40000, 18)
		recommendations = append(recommendations, posBundlingRecommendation{
			RecommendationKey:        buildPOSBundlingRecommendationKey(primary.ID, paired.ID),
			PrimaryProductID:         primary.ID,
			PrimaryProductName:       primary.Name,
			PairedProductID:          paired.ID,
			PairedProductName:        paired.Name,
			JointTransactionCount:    pair.Count,
			AttachRatePercent:        attachRate,
			AverageBundleValue:       averageBundleValue,
			SuggestedDiscountPercent: discountPercent,
			SuggestedBundlePrice:     roundMoney(averageBundleValue * (1 - (discountPercent / 100))),
			Reason: fmt.Sprintf(
				"%s dan %s muncul bersama di %d transaksi. Attach rate %s%% terhadap %s cukup kuat untuk dijadikan paket bundling.",
				primary.Name,
				paired.Name,
				pair.Count,
				formatOneDecimal(attachRate),
				primary.Name,
			),
			ConfidenceLabel: resolvePOSPricingConfidenceLabel(score),
			rankingScore:    score,
		})
	}

	sort.Slice(recommendations, func(i, j int) bool {
		if recommendations[i].rankingScore != recommendations[j].rankingScore {
			return recommendations[i].rankingScore > recommendations[j].rankingScore
		}
		if recommendations[i].JointTransactionCount != recommendations[j].JointTransactionCount {
			return recommendations[i].JointTransactionCount > recommendations[j].JointTransactionCount
		}
		return recommendations[i].AttachRatePercent > recommendations[j].AttachRatePercent
	})
	return recommendations
}

func buildPOSPromoCandidates(
	transactions []models.Transaction,
	productAggregates map[uint]*posPricingProductAggregate,
	timeSlotAggregates map[string]*posPricingTimeSlotAggregate,
	timeSlotProductCounts map[string]map[uint]int,
	bundles []posBundlingRecommendation,
) []posPromoCandidate {
	candidates := make([]posPromoCandidate, 0, 3)

	if slotCandidate, ok := buildPOSTimeSlotPromoCandidate(transactions, productAggregates, timeSlotAggregates, timeSlotProductCounts); ok {
		candidates = append(candidates, slotCandidate)
	}
	if heroCandidate, ok := buildPOSMultiBuyPromoCandidate(transactions, productAggregates); ok {
		candidates = append(candidates, heroCandidate)
	}
	if bundleCandidate, ok := buildPOSBundlePromoCandidate(bundles); ok {
		candidates = append(candidates, bundleCandidate)
	}

	return candidates
}

func buildPOSTimeSlotPromoCandidate(
	transactions []models.Transaction,
	productAggregates map[uint]*posPricingProductAggregate,
	timeSlotAggregates map[string]*posPricingTimeSlotAggregate,
	timeSlotProductCounts map[string]map[uint]int,
) (posPromoCandidate, bool) {
	if len(transactions) < 6 || len(timeSlotAggregates) < 2 {
		return posPromoCandidate{}, false
	}

	slots := make([]*posPricingTimeSlotAggregate, 0, len(timeSlotAggregates))
	for _, item := range timeSlotAggregates {
		if item.Transactions > 0 {
			slots = append(slots, item)
		}
	}
	if len(slots) < 2 {
		return posPromoCandidate{}, false
	}

	sort.Slice(slots, func(i, j int) bool {
		if slots[i].Transactions != slots[j].Transactions {
			return slots[i].Transactions < slots[j].Transactions
		}
		return slots[i].Revenue < slots[j].Revenue
	})
	weakest := slots[0]
	strongest := slots[len(slots)-1]
	if weakest == nil || strongest == nil || weakest.Transactions <= 0 {
		return posPromoCandidate{}, false
	}

	totalTransactions := len(transactions)
	shareGap := ((float64(strongest.Transactions) - float64(weakest.Transactions)) / float64(totalTransactions)) * 100
	productID, productName := resolveTopPOSProductForTimeSlot(weakest.Key, timeSlotProductCounts, productAggregates)
	product := productAggregates[productID]
	if product == nil {
		return posPromoCandidate{}, false
	}

	discountPercent := 6.0
	switch {
	case shareGap > 25:
		discountPercent = 10
	case shareGap > 15:
		discountPercent = 8
	}

	score := shareGap + minFloat(float64(weakest.Transactions*8), 28)
	return posPromoCandidate{
		RecommendationKey:        buildPOSPromoRecommendationKey("time-slot-boost", "time_slot", productID, weakest.Key),
		StrategyKey:              "time-slot-boost",
		StrategyLabel:            "Promo jam sepi",
		FocusType:                "time_slot",
		FocusProductID:           productID,
		FocusProductName:         productName,
		TimeSlotKey:              weakest.Key,
		TimeSlotLabel:            weakest.Label,
		SupportingTransactions:   weakest.Transactions,
		CurrentRevenue:           weakest.Revenue,
		SuggestedDiscountPercent: discountPercent,
		SuggestedPrice:           roundMoney(product.AverageUnitPrice * (1 - (discountPercent / 100))),
		ExpectedLiftPercent:      minFloat(shareGap*0.75, 24),
		Reason: fmt.Sprintf(
			"Slot %s baru menyumbang %d transaksi, jauh di bawah slot %s. %s paling sering muncul di slot ini dan cocok dijadikan happy-hour trigger.",
			weakest.Label,
			weakest.Transactions,
			strongest.Label,
			productName,
		),
		ConfidenceLabel: resolvePOSPricingConfidenceLabel(score),
		rankingScore:    score,
	}, true
}

func buildPOSMultiBuyPromoCandidate(
	transactions []models.Transaction,
	productAggregates map[uint]*posPricingProductAggregate,
) (posPromoCandidate, bool) {
	if len(transactions) < 5 {
		return posPromoCandidate{}, false
	}

	var hero *posPricingProductAggregate
	for _, item := range productAggregates {
		if item == nil || item.Transactions < 2 || item.AverageUnitPrice <= 0 {
			continue
		}

		averageQtyPerTransaction := float64(item.Quantity) / float64(item.Transactions)
		if averageQtyPerTransaction > 1.35 {
			continue
		}

		if hero == nil || item.Transactions > hero.Transactions || (item.Transactions == hero.Transactions && item.Revenue > hero.Revenue) {
			candidate := *item
			hero = &candidate
		}
	}
	if hero == nil {
		return posPromoCandidate{}, false
	}

	discountPercent := 5.0
	if hero.Transactions >= 10 {
		discountPercent = 7
	}

	score := float64(hero.Transactions*10) + minFloat(hero.Revenue/50000, 20)
	return posPromoCandidate{
		RecommendationKey:        buildPOSPromoRecommendationKey("multi-buy-hero", "multi_buy", hero.ID, ""),
		StrategyKey:              "multi-buy-hero",
		StrategyLabel:            "Promo beli lebih banyak",
		FocusType:                "multi_buy",
		FocusProductID:           hero.ID,
		FocusProductName:         hero.Name,
		SupportingTransactions:   hero.Transactions,
		CurrentRevenue:           hero.Revenue,
		SuggestedDiscountPercent: discountPercent,
		SuggestedPrice:           roundMoney((hero.AverageUnitPrice * 2) * (1 - (discountPercent / 100))),
		ExpectedLiftPercent:      minFloat(12+(float64(hero.Transactions)*0.6), 22),
		Reason: fmt.Sprintf(
			"%s sudah sering terjual, tetapi rata-rata hanya 1 unit per nota. Skema beli 2 hemat bisa menaikkan qty tanpa memangkas margin terlalu dalam.",
			hero.Name,
		),
		ConfidenceLabel: resolvePOSPricingConfidenceLabel(score),
		rankingScore:    score,
	}, true
}

func buildPOSBundlePromoCandidate(bundles []posBundlingRecommendation) (posPromoCandidate, bool) {
	if len(bundles) == 0 {
		return posPromoCandidate{}, false
	}

	topBundle := bundles[0]
	score := float64(topBundle.JointTransactionCount*10) + topBundle.AttachRatePercent
	return posPromoCandidate{
		RecommendationKey:        buildPOSPromoRecommendationKey("bundle-activation", "cross_sell", topBundle.PrimaryProductID, ""),
		StrategyKey:              "bundle-activation",
		StrategyLabel:            "Promo pasangan favorit",
		FocusType:                "cross_sell",
		FocusProductID:           topBundle.PrimaryProductID,
		FocusProductName:         fmt.Sprintf("%s + %s", topBundle.PrimaryProductName, topBundle.PairedProductName),
		SupportingTransactions:   topBundle.JointTransactionCount,
		CurrentRevenue:           topBundle.AverageBundleValue * float64(topBundle.JointTransactionCount),
		SuggestedDiscountPercent: topBundle.SuggestedDiscountPercent,
		SuggestedPrice:           topBundle.SuggestedBundlePrice,
		ExpectedLiftPercent:      minFloat(10+(topBundle.AttachRatePercent*0.25), 26),
		Reason: fmt.Sprintf(
			"Pasangan %s dan %s sudah terbukti sering dibeli bareng. Promo bundling siap pakai akan lebih mudah dikomunikasikan ke kasir dan pelanggan.",
			topBundle.PrimaryProductName,
			topBundle.PairedProductName,
		),
		ConfidenceLabel: resolvePOSPricingConfidenceLabel(score),
		rankingScore:    score,
	}, true
}

func resolveTopPOSProductForTimeSlot(
	slotKey string,
	timeSlotProductCounts map[string]map[uint]int,
	productAggregates map[uint]*posPricingProductAggregate,
) (uint, string) {
	slotProducts := timeSlotProductCounts[slotKey]
	var (
		bestProductID uint
		bestScore     int
		bestRevenue   float64
		bestName      string
	)

	for productID, count := range slotProducts {
		if count < bestScore {
			continue
		}
		product := productAggregates[productID]
		if product == nil {
			continue
		}
		if count > bestScore || (count == bestScore && product.Revenue > bestRevenue) {
			bestProductID = productID
			bestScore = count
			bestRevenue = product.Revenue
			bestName = product.Name
		}
	}

	return bestProductID, bestName
}

func resolvePOSPricingScopedOutletID(transactions []models.Transaction) uint {
	if len(transactions) == 0 {
		return 0
	}

	firstOutletID := transactions[0].OutletID
	if firstOutletID == 0 {
		return 0
	}
	for _, transaction := range transactions[1:] {
		if transaction.OutletID != firstOutletID {
			return 0
		}
	}
	return firstOutletID
}

func applyPOSPricingAutomaticAttribution(db *gorm.DB, outletID uint, summaries map[string]services.AIPricingFeedbackSummary) error {
	if outletID == 0 || len(summaries) == 0 {
		return nil
	}

	outcomeRecords, err := services.ListAIPricingFeedbackOutcomeRecords(db, outletID, collectPOSPricingFeedbackSummaryKeys(summaries))
	if err != nil {
		return err
	}
	if len(outcomeRecords) == 0 {
		return nil
	}

	fetchedTransactions := make(map[string][]models.Transaction)
	accumulators := make(map[string]*posPricingAutoAttributionAccumulator)

	for _, record := range outcomeRecords {
		if record.CampaignStartDate == nil || record.CampaignEndDate == nil {
			continue
		}
		campaignStart := startOfPOSPricingDay(*record.CampaignStartDate)
		campaignEndExclusive := startOfPOSPricingDay(*record.CampaignEndDate).AddDate(0, 0, 1)
		if !campaignEndExclusive.After(campaignStart) {
			continue
		}

		durationDays := int(campaignEndExclusive.Sub(campaignStart).Hours() / 24)
		if durationDays <= 0 {
			durationDays = 1
		}
		baselineStart := campaignStart.AddDate(0, 0, -durationDays)
		baselineEndExclusive := campaignStart

		campaignTransactions, err := loadPOSPricingTransactionsForWindow(db, outletID, campaignStart, campaignEndExclusive, fetchedTransactions)
		if err != nil {
			return err
		}
		baselineTransactions, err := loadPOSPricingTransactionsForWindow(db, outletID, baselineStart, baselineEndExclusive, fetchedTransactions)
		if err != nil {
			return err
		}

		campaignCount, campaignRevenue := evaluatePOSPricingAttributedPerformance(record.RecommendationKey, record.RecommendationType, campaignTransactions)
		baselineCount, baselineRevenue := evaluatePOSPricingAttributedPerformance(record.RecommendationKey, record.RecommendationType, baselineTransactions)

		entry := accumulators[record.RecommendationKey]
		if entry == nil {
			entry = &posPricingAutoAttributionAccumulator{}
			accumulators[record.RecommendationKey] = entry
		}
		entry.CampaignCount++
		entry.ObservedRevenueLift += campaignRevenue - baselineRevenue
		entry.TransactionLiftSum += calculatePOSPricingLiftPercent(baselineCount, campaignCount)
	}

	for recommendationKey, accumulator := range accumulators {
		summary := summaries[recommendationKey]
		summary.AutoAttributedCampaignCount = accumulator.CampaignCount
		summary.AutoObservedRevenueLiftTotal = accumulator.ObservedRevenueLift
		if accumulator.CampaignCount > 0 {
			summary.AutoAverageTransactionLiftPercent = accumulator.TransactionLiftSum / float64(accumulator.CampaignCount)
		}
		summaries[recommendationKey] = summary
	}
	return nil
}

func collectPOSPricingFeedbackSummaryKeys(summaries map[string]services.AIPricingFeedbackSummary) []string {
	keys := make([]string, 0, len(summaries))
	for key := range summaries {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

func startOfPOSPricingDay(value time.Time) time.Time {
	localValue := value.In(time.Local)
	return time.Date(localValue.Year(), localValue.Month(), localValue.Day(), 0, 0, 0, 0, time.Local)
}

func buildPOSPricingWindowCacheKey(outletID uint, start, endExclusive time.Time) string {
	return fmt.Sprintf("%d:%s:%s", outletID, start.Format(time.RFC3339), endExclusive.Format(time.RFC3339))
}

func loadPOSPricingTransactionsForWindow(
	db *gorm.DB,
	outletID uint,
	start time.Time,
	endExclusive time.Time,
	cache map[string][]models.Transaction,
) ([]models.Transaction, error) {
	cacheKey := buildPOSPricingWindowCacheKey(outletID, start, endExclusive)
	if cached, exists := cache[cacheKey]; exists {
		return cached, nil
	}

	query := db.
		Preload("Items.Product").
		Model(&models.Transaction{}).
		Where("outlet_id = ?", outletID).
		Where("created_at >= ?", start).
		Where("created_at < ?", endExclusive)
	query = applyNonVoidedTransactionFilter(query)

	var transactions []models.Transaction
	if err := query.Order("created_at asc").Find(&transactions).Error; err != nil {
		return nil, err
	}
	cache[cacheKey] = transactions
	return transactions, nil
}

func evaluatePOSPricingAttributedPerformance(recommendationKey, recommendationType string, transactions []models.Transaction) (int, float64) {
	normalizedType := strings.ToLower(strings.TrimSpace(recommendationType))
	switch normalizedType {
	case "bundling":
		leftID, rightID, ok := parsePOSBundlingRecommendationKey(recommendationKey)
		if !ok {
			return 0, 0
		}
		return evaluatePOSBundlingAttributedPerformance(leftID, rightID, transactions)
	case "promo":
		strategyKey, focusType, focusProductID, timeSlotKey, ok := parsePOSPromoRecommendationKey(recommendationKey)
		if !ok {
			return 0, 0
		}
		return evaluatePOSPromoAttributedPerformance(strategyKey, focusType, focusProductID, timeSlotKey, transactions)
	default:
		return 0, 0
	}
}

func parsePOSBundlingRecommendationKey(value string) (uint, uint, bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 3 || parts[0] != "pos-bundling" {
		return 0, 0, false
	}
	leftID, errLeft := strconv.ParseUint(parts[1], 10, 64)
	rightID, errRight := strconv.ParseUint(parts[2], 10, 64)
	if errLeft != nil || errRight != nil {
		return 0, 0, false
	}
	return uint(leftID), uint(rightID), true
}

func parsePOSPromoRecommendationKey(value string) (string, string, uint, string, bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 5 || parts[0] != "pos-promo" {
		return "", "", 0, "", false
	}
	focusProductID, err := strconv.ParseUint(parts[3], 10, 64)
	if err != nil {
		return "", "", 0, "", false
	}
	return parts[1], parts[2], uint(focusProductID), parts[4], true
}

func evaluatePOSBundlingAttributedPerformance(leftID, rightID uint, transactions []models.Transaction) (int, float64) {
	matchingTransactions := 0
	revenue := 0.0
	for _, transaction := range transactions {
		hasLeft := false
		hasRight := false
		combinedValue := 0.0
		for _, item := range transaction.Items {
			switch item.ProductID {
			case leftID:
				hasLeft = true
				combinedValue += item.Total
			case rightID:
				hasRight = true
				combinedValue += item.Total
			}
		}
		if hasLeft && hasRight {
			matchingTransactions++
			revenue += combinedValue
		}
	}
	return matchingTransactions, revenue
}

func evaluatePOSPromoAttributedPerformance(
	strategyKey string,
	focusType string,
	focusProductID uint,
	timeSlotKey string,
	transactions []models.Transaction,
) (int, float64) {
	switch strings.ToLower(strings.TrimSpace(focusType)) {
	case "time_slot":
		return evaluatePOSTimeSlotPromoPerformance(focusProductID, timeSlotKey, transactions)
	case "multi_buy":
		return evaluatePOSMultiBuyPromoPerformance(focusProductID, transactions)
	case "cross_sell":
		return evaluatePOSFocusProductPerformance(focusProductID, transactions)
	default:
		if strings.EqualFold(strings.TrimSpace(strategyKey), "bundle-activation") {
			return evaluatePOSFocusProductPerformance(focusProductID, transactions)
		}
		return evaluatePOSFocusProductPerformance(focusProductID, transactions)
	}
}

func evaluatePOSTimeSlotPromoPerformance(focusProductID uint, timeSlotKey string, transactions []models.Transaction) (int, float64) {
	matchingTransactions := 0
	revenue := 0.0
	for _, transaction := range transactions {
		if getPOSPricingTimeSlotKey(transaction.CreatedAt) != strings.TrimSpace(timeSlotKey) {
			continue
		}
		if focusProductID == 0 {
			matchingTransactions++
			revenue += getPOSPricingTransactionValue(transaction)
			continue
		}

		productRevenue := 0.0
		productMatched := false
		for _, item := range transaction.Items {
			if item.ProductID == focusProductID {
				productMatched = true
				productRevenue += item.Total
			}
		}
		if productMatched {
			matchingTransactions++
			revenue += productRevenue
		}
	}
	return matchingTransactions, revenue
}

func evaluatePOSMultiBuyPromoPerformance(focusProductID uint, transactions []models.Transaction) (int, float64) {
	matchingTransactions := 0
	revenue := 0.0
	for _, transaction := range transactions {
		totalQty := 0
		productRevenue := 0.0
		for _, item := range transaction.Items {
			if item.ProductID == focusProductID {
				totalQty += item.Quantity
				productRevenue += item.Total
			}
		}
		if totalQty >= 2 {
			matchingTransactions++
			revenue += productRevenue
		}
	}
	return matchingTransactions, revenue
}

func evaluatePOSFocusProductPerformance(focusProductID uint, transactions []models.Transaction) (int, float64) {
	matchingTransactions := 0
	revenue := 0.0
	for _, transaction := range transactions {
		productRevenue := 0.0
		productMatched := false
		for _, item := range transaction.Items {
			if item.ProductID == focusProductID {
				productMatched = true
				productRevenue += item.Total
			}
		}
		if productMatched {
			matchingTransactions++
			revenue += productRevenue
		}
	}
	return matchingTransactions, revenue
}

func calculatePOSPricingLiftPercent(baselineCount, campaignCount int) float64 {
	if baselineCount <= 0 {
		if campaignCount <= 0 {
			return 0
		}
		return 100
	}
	return ((float64(campaignCount) - float64(baselineCount)) / float64(baselineCount)) * 100
}

func collectPOSPricingRecommendationKeys(report posPricingAssistantReport) []string {
	keys := make([]string, 0, len(report.BundlingRecommendations)+len(report.PromoCandidates))
	for _, item := range report.BundlingRecommendations {
		if strings.TrimSpace(item.RecommendationKey) != "" {
			keys = append(keys, item.RecommendationKey)
		}
	}
	for _, item := range report.PromoCandidates {
		if strings.TrimSpace(item.RecommendationKey) != "" {
			keys = append(keys, item.RecommendationKey)
		}
	}
	return keys
}

func applyPOSPricingFeedbackSummaries(
	bundles []posBundlingRecommendation,
	promos []posPromoCandidate,
	summaries map[string]services.AIPricingFeedbackSummary,
) {
	for index := range bundles {
		summary, exists := summaries[bundles[index].RecommendationKey]
		if !exists {
			continue
		}
		bundles[index].FeedbackAcceptedCount = summary.AcceptedCount
		bundles[index].FeedbackIgnoredCount = summary.IgnoredCount
		bundles[index].FeedbackCampaignUsedCount = summary.CampaignUsedCount
		bundles[index].FeedbackSalesImprovedCount = summary.SalesImprovedCount
		bundles[index].FeedbackEstimatedRevenueLiftTotal = summary.EstimatedRevenueLiftTotal
		bundles[index].FeedbackObservedRevenueLiftTotal = summary.ObservedRevenueLiftTotal
		bundles[index].FeedbackAverageTransactionLiftPercent = summary.AverageObservedTransactionLiftPercent
		bundles[index].FeedbackAutoAttributedCampaignCount = summary.AutoAttributedCampaignCount
		bundles[index].FeedbackAutoObservedRevenueLiftTotal = summary.AutoObservedRevenueLiftTotal
		bundles[index].FeedbackAutoAverageTransactionLiftPercent = summary.AutoAverageTransactionLiftPercent
		bundles[index].FeedbackLastAction = summary.LastAction
		bundles[index].rankingScore += resolvePOSPricingFeedbackAdjustment(summary)
		bundles[index].ConfidenceLabel = resolvePOSPricingConfidenceLabel(bundles[index].rankingScore)
	}
	sort.Slice(bundles, func(i, j int) bool {
		if bundles[i].rankingScore != bundles[j].rankingScore {
			return bundles[i].rankingScore > bundles[j].rankingScore
		}
		return bundles[i].JointTransactionCount > bundles[j].JointTransactionCount
	})

	for index := range promos {
		summary, exists := summaries[promos[index].RecommendationKey]
		if !exists {
			continue
		}
		promos[index].FeedbackAcceptedCount = summary.AcceptedCount
		promos[index].FeedbackIgnoredCount = summary.IgnoredCount
		promos[index].FeedbackCampaignUsedCount = summary.CampaignUsedCount
		promos[index].FeedbackSalesImprovedCount = summary.SalesImprovedCount
		promos[index].FeedbackEstimatedRevenueLiftTotal = summary.EstimatedRevenueLiftTotal
		promos[index].FeedbackObservedRevenueLiftTotal = summary.ObservedRevenueLiftTotal
		promos[index].FeedbackAverageTransactionLiftPercent = summary.AverageObservedTransactionLiftPercent
		promos[index].FeedbackAutoAttributedCampaignCount = summary.AutoAttributedCampaignCount
		promos[index].FeedbackAutoObservedRevenueLiftTotal = summary.AutoObservedRevenueLiftTotal
		promos[index].FeedbackAutoAverageTransactionLiftPercent = summary.AutoAverageTransactionLiftPercent
		promos[index].FeedbackLastAction = summary.LastAction
		promos[index].rankingScore += resolvePOSPricingFeedbackAdjustment(summary)
		promos[index].ConfidenceLabel = resolvePOSPricingConfidenceLabel(promos[index].rankingScore)
	}
	sort.Slice(promos, func(i, j int) bool {
		if promos[i].rankingScore != promos[j].rankingScore {
			return promos[i].rankingScore > promos[j].rankingScore
		}
		return promos[i].SupportingTransactions > promos[j].SupportingTransactions
	})
}

func resolvePOSPricingFeedbackAdjustment(summary services.AIPricingFeedbackSummary) float64 {
	acceptedWeight := float64(summary.AcceptedCount) * 6
	ignoredWeight := float64(summary.IgnoredCount) * 8
	campaignWeight := float64(summary.CampaignUsedCount) * 12
	salesWeight := float64(summary.SalesImprovedCount) * 18
	revenueSignal := clampPOSPricingSignal(summary.ObservedRevenueLiftTotal/250000, -18, 18) +
		clampPOSPricingSignal(summary.EstimatedRevenueLiftTotal/500000, -8, 8) +
		clampPOSPricingSignal(summary.AutoObservedRevenueLiftTotal/250000, -18, 18)
	transactionSignal := clampPOSPricingSignal(summary.AverageObservedTransactionLiftPercent/2, -12, 12) +
		clampPOSPricingSignal(summary.AutoAverageTransactionLiftPercent/2, -12, 12)
	adjustment := acceptedWeight - ignoredWeight + campaignWeight + salesWeight + revenueSignal + transactionSignal
	if strings.EqualFold(strings.TrimSpace(summary.LastAction), services.AIRecommendationFeedbackAccepted) {
		adjustment += 4
	}
	if strings.EqualFold(strings.TrimSpace(summary.LastAction), services.AIRecommendationFeedbackIgnored) {
		adjustment -= 4
	}
	if adjustment > 48 {
		return 48
	}
	if adjustment < -24 {
		return -24
	}
	return adjustment
}

func clampPOSPricingSignal(value, minimum, maximum float64) float64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func buildPOSBundlingRecommendationKey(primaryProductID, pairedProductID uint) string {
	left := primaryProductID
	right := pairedProductID
	if left > right {
		left, right = right, left
	}
	return fmt.Sprintf("pos-bundling:%d:%d", left, right)
}

func buildPOSPromoRecommendationKey(strategyKey, focusType string, focusProductID uint, timeSlotKey string) string {
	return fmt.Sprintf(
		"pos-promo:%s:%s:%d:%s",
		strings.ToLower(strings.TrimSpace(strategyKey)),
		strings.ToLower(strings.TrimSpace(focusType)),
		focusProductID,
		strings.ToLower(strings.TrimSpace(timeSlotKey)),
	)
}

func getPOSPricingTransactionValue(transaction models.Transaction) float64 {
	switch {
	case transaction.GrandTotal > 0:
		return transaction.GrandTotal
	case transaction.Total > 0:
		return transaction.Total
	default:
		return transaction.Subtotal
	}
}

func getPOSPricingTimeSlotKey(value time.Time) string {
	hour := value.In(time.Local).Hour()
	switch {
	case hour >= 5 && hour < 11:
		return "morning"
	case hour >= 11 && hour < 15:
		return "midday"
	case hour >= 15 && hour < 19:
		return "afternoon"
	default:
		return "evening"
	}
}

func getPOSPricingTimeSlotLabel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "morning":
		return "Pagi"
	case "midday":
		return "Siang"
	case "afternoon":
		return "Sore"
	case "evening":
		return "Malam"
	default:
		return "Tidak diketahui"
	}
}

func resolvePOSPricingConfidenceLabel(score float64) string {
	switch {
	case score >= 70:
		return "Tinggi"
	case score >= 45:
		return "Sedang"
	default:
		return "Eksplorasi"
	}
}

func formatOneDecimal(value float64) string {
	return strconv.FormatFloat(value, 'f', 1, 64)
}

func roundMoney(value float64) float64 {
	if value <= 0 {
		return 0
	}
	return float64(int(value + 0.5))
}

func minFloat(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}
