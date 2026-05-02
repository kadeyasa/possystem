package services

import (
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

const (
	AIRecommendationFeedbackAccepted      = "accepted"
	AIRecommendationFeedbackIgnored       = "ignored"
	AIRecommendationFeedbackCampaignUsed  = "campaign_used"
	AIRecommendationFeedbackSalesImproved = "sales_improved"
)

type AIPricingFeedbackInput struct {
	OutletID                       uint
	RecommendationType             string
	RecommendationKey              string
	FeedbackAction                 string
	ActorUserID                    string
	ActorType                      string
	ActorName                      string
	Note                           string
	EstimatedRevenueLift           float64
	ObservedRevenueLift            float64
	ObservedTransactionLiftPercent float64
	CampaignStartDate              *time.Time
	CampaignEndDate                *time.Time
}

type AIPricingFeedbackSummary struct {
	RecommendationKey                     string     `gorm:"column:recommendation_key"`
	AcceptedCount                         int        `gorm:"column:accepted_count"`
	IgnoredCount                          int        `gorm:"column:ignored_count"`
	CampaignUsedCount                     int        `gorm:"column:campaign_used_count"`
	SalesImprovedCount                    int        `gorm:"column:sales_improved_count"`
	EstimatedRevenueLiftTotal             float64    `gorm:"column:estimated_revenue_lift_total"`
	ObservedRevenueLiftTotal              float64    `gorm:"column:observed_revenue_lift_total"`
	AverageObservedTransactionLiftPercent float64    `gorm:"column:average_observed_transaction_lift_percent"`
	AutoAttributedCampaignCount           int        `gorm:"-"`
	AutoObservedRevenueLiftTotal          float64    `gorm:"-"`
	AutoAverageTransactionLiftPercent     float64    `gorm:"-"`
	LastAction                            string     `gorm:"column:last_action"`
	LastUpdatedAt                         *time.Time `gorm:"column:last_updated_at"`
}

type AIPricingFeedbackOutcomeRecord struct {
	RecommendationType string     `gorm:"column:recommendation_type"`
	RecommendationKey  string     `gorm:"column:recommendation_key"`
	OutcomeType        string     `gorm:"column:outcome_type"`
	CampaignStartDate  *time.Time `gorm:"column:campaign_start_date"`
	CampaignEndDate    *time.Time `gorm:"column:campaign_end_date"`
}

var (
	ensureAIPricingFeedbackSchemaOnce sync.Once
	ensureAIPricingFeedbackSchemaErr  error
)

func NormalizeAIRecommendationFeedbackAction(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "accepted", "accept", "approve", "diterima":
		return AIRecommendationFeedbackAccepted
	case "ignored", "ignore", "dismissed", "dismiss", "skip", "diabaikan":
		return AIRecommendationFeedbackIgnored
	case "campaign_used", "campaign-used", "campaign", "dipakai_campaign", "dipakai jadi campaign", "dipakai jadi promo":
		return AIRecommendationFeedbackCampaignUsed
	case "sales_improved", "sales-improved", "improved_sales", "improve-sales", "berhasil meningkatkan sales", "naikkan sales", "sales lift":
		return AIRecommendationFeedbackSalesImproved
	default:
		return ""
	}
}

func EnsureAIPricingFeedbackSchema(db *gorm.DB) error {
	ensureAIPricingFeedbackSchemaOnce.Do(func() {
		statements := []string{
			`CREATE TABLE IF NOT EXISTS public.tblai_pricing_feedback (
				id BIGSERIAL PRIMARY KEY,
				outlet_id BIGINT NOT NULL DEFAULT 0,
				recommendation_domain VARCHAR(32) NOT NULL DEFAULT 'pos_pricing',
				recommendation_type VARCHAR(64) NOT NULL DEFAULT '',
				recommendation_key VARCHAR(191) NOT NULL DEFAULT '',
				feedback_action VARCHAR(16) NOT NULL DEFAULT '',
				actor_user_id VARCHAR(64) NOT NULL DEFAULT '',
				actor_type VARCHAR(32) NOT NULL DEFAULT '',
				actor_name VARCHAR(150) NOT NULL DEFAULT '',
				note TEXT,
				created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW()
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_tblai_pricing_feedback_actor_unique
				ON tblai_pricing_feedback (outlet_id, recommendation_key, actor_type, actor_user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_tblai_pricing_feedback_lookup
				ON tblai_pricing_feedback (outlet_id, recommendation_key, feedback_action, updated_at DESC)`,
			`CREATE TABLE IF NOT EXISTS public.tblai_pricing_feedback_outcomes (
				id BIGSERIAL PRIMARY KEY,
				outlet_id BIGINT NOT NULL DEFAULT 0,
				recommendation_domain VARCHAR(32) NOT NULL DEFAULT 'pos_pricing',
				recommendation_type VARCHAR(64) NOT NULL DEFAULT '',
				recommendation_key VARCHAR(191) NOT NULL DEFAULT '',
				outcome_type VARCHAR(32) NOT NULL DEFAULT '',
				estimated_revenue_lift NUMERIC(16,2) NOT NULL DEFAULT 0,
				observed_revenue_lift NUMERIC(16,2) NOT NULL DEFAULT 0,
				observed_transaction_lift_percent NUMERIC(10,2) NOT NULL DEFAULT 0,
				campaign_start_date DATE NULL,
				campaign_end_date DATE NULL,
				actor_user_id VARCHAR(64) NOT NULL DEFAULT '',
				actor_type VARCHAR(32) NOT NULL DEFAULT '',
				actor_name VARCHAR(150) NOT NULL DEFAULT '',
				note TEXT,
				created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW()
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_tblai_pricing_feedback_outcome_unique
				ON tblai_pricing_feedback_outcomes (outlet_id, recommendation_key, outcome_type, actor_type, actor_user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_tblai_pricing_feedback_outcome_lookup
				ON tblai_pricing_feedback_outcomes (outlet_id, recommendation_key, outcome_type, updated_at DESC)`,
		}

		for _, statement := range statements {
			if err := db.Exec(statement).Error; err != nil {
				ensureAIPricingFeedbackSchemaErr = err
				return
			}
		}
	})

	return ensureAIPricingFeedbackSchemaErr
}

func UpsertAIPricingFeedback(db *gorm.DB, input AIPricingFeedbackInput) error {
	if err := EnsureAIPricingFeedbackSchema(db); err != nil {
		return err
	}

	action := NormalizeAIRecommendationFeedbackAction(input.FeedbackAction)
	if action == "" || input.OutletID == 0 || strings.TrimSpace(input.RecommendationKey) == "" {
		return nil
	}

	if isAIPricingOutcomeAction(action) {
		if err := upsertAIPricingPreference(db, input, AIRecommendationFeedbackAccepted); err != nil {
			return err
		}
		return upsertAIPricingOutcome(db, input, action)
	}

	return upsertAIPricingPreference(db, input, action)
}

func upsertAIPricingPreference(db *gorm.DB, input AIPricingFeedbackInput, action string) error {
	return db.Exec(`
		INSERT INTO tblai_pricing_feedback (
			outlet_id,
			recommendation_domain,
			recommendation_type,
			recommendation_key,
			feedback_action,
			actor_user_id,
			actor_type,
			actor_name,
			note,
			created_at,
			updated_at
		)
		VALUES (?, 'pos_pricing', ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
		ON CONFLICT (outlet_id, recommendation_key, actor_type, actor_user_id)
		DO UPDATE SET
			recommendation_type = EXCLUDED.recommendation_type,
			feedback_action = EXCLUDED.feedback_action,
			actor_name = EXCLUDED.actor_name,
			note = EXCLUDED.note,
			updated_at = NOW()
	`,
		input.OutletID,
		strings.TrimSpace(input.RecommendationType),
		strings.TrimSpace(input.RecommendationKey),
		action,
		strings.TrimSpace(input.ActorUserID),
		strings.TrimSpace(input.ActorType),
		strings.TrimSpace(input.ActorName),
		strings.TrimSpace(input.Note),
	).Error
}

func upsertAIPricingOutcome(db *gorm.DB, input AIPricingFeedbackInput, outcomeType string) error {
	return db.Exec(`
		INSERT INTO tblai_pricing_feedback_outcomes (
			outlet_id,
			recommendation_domain,
			recommendation_type,
			recommendation_key,
			outcome_type,
			estimated_revenue_lift,
			observed_revenue_lift,
			observed_transaction_lift_percent,
			campaign_start_date,
			campaign_end_date,
			actor_user_id,
			actor_type,
			actor_name,
			note,
			created_at,
			updated_at
		)
		VALUES (?, 'pos_pricing', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
		ON CONFLICT (outlet_id, recommendation_key, outcome_type, actor_type, actor_user_id)
		DO UPDATE SET
			recommendation_type = EXCLUDED.recommendation_type,
			estimated_revenue_lift = EXCLUDED.estimated_revenue_lift,
			observed_revenue_lift = EXCLUDED.observed_revenue_lift,
			observed_transaction_lift_percent = EXCLUDED.observed_transaction_lift_percent,
			campaign_start_date = EXCLUDED.campaign_start_date,
			campaign_end_date = EXCLUDED.campaign_end_date,
			actor_name = EXCLUDED.actor_name,
			note = EXCLUDED.note,
			updated_at = NOW()
	`,
		input.OutletID,
		strings.TrimSpace(input.RecommendationType),
		strings.TrimSpace(input.RecommendationKey),
		outcomeType,
		input.EstimatedRevenueLift,
		input.ObservedRevenueLift,
		input.ObservedTransactionLiftPercent,
		input.CampaignStartDate,
		input.CampaignEndDate,
		strings.TrimSpace(input.ActorUserID),
		strings.TrimSpace(input.ActorType),
		strings.TrimSpace(input.ActorName),
		strings.TrimSpace(input.Note),
	).Error
}

func GetAIPricingFeedbackSummaries(db *gorm.DB, outletID uint, recommendationKeys []string) (map[string]AIPricingFeedbackSummary, error) {
	results := make(map[string]AIPricingFeedbackSummary)
	if err := EnsureAIPricingFeedbackSchema(db); err != nil {
		return results, err
	}
	if outletID == 0 || len(recommendationKeys) == 0 {
		return results, nil
	}

	normalizedKeys := make([]string, 0, len(recommendationKeys))
	seen := make(map[string]struct{}, len(recommendationKeys))
	for _, key := range recommendationKeys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalizedKeys = append(normalizedKeys, trimmed)
	}
	if len(normalizedKeys) == 0 {
		return results, nil
	}

	var rows []AIPricingFeedbackSummary
	if err := db.Raw(`
		SELECT
			recommendation_key,
			COUNT(*) FILTER (WHERE feedback_action = ?) AS accepted_count,
			COUNT(*) FILTER (WHERE feedback_action = ?) AS ignored_count,
			COALESCE((ARRAY_AGG(feedback_action ORDER BY updated_at DESC))[1], '') AS last_action,
			MAX(updated_at) AS last_updated_at
		FROM tblai_pricing_feedback
		WHERE outlet_id = ?
			AND recommendation_key IN ?
		GROUP BY recommendation_key
	`, AIRecommendationFeedbackAccepted, AIRecommendationFeedbackIgnored, outletID, normalizedKeys).Scan(&rows).Error; err != nil {
		return results, err
	}

	for _, row := range rows {
		results[row.RecommendationKey] = row
	}

	var outcomeRows []AIPricingFeedbackSummary
	if err := db.Raw(`
		SELECT
			recommendation_key,
			COUNT(*) FILTER (WHERE outcome_type = ?) AS campaign_used_count,
			COUNT(*) FILTER (WHERE outcome_type = ?) AS sales_improved_count,
			COALESCE(SUM(estimated_revenue_lift), 0) AS estimated_revenue_lift_total,
			COALESCE(SUM(observed_revenue_lift), 0) AS observed_revenue_lift_total,
			COALESCE(AVG(NULLIF(observed_transaction_lift_percent, 0)), 0) AS average_observed_transaction_lift_percent
		FROM tblai_pricing_feedback_outcomes
		WHERE outlet_id = ?
			AND recommendation_key IN ?
		GROUP BY recommendation_key
	`, AIRecommendationFeedbackCampaignUsed, AIRecommendationFeedbackSalesImproved, outletID, normalizedKeys).Scan(&outcomeRows).Error; err != nil {
		return results, err
	}

	for _, row := range outcomeRows {
		summary := results[row.RecommendationKey]
		summary.RecommendationKey = row.RecommendationKey
		summary.CampaignUsedCount = row.CampaignUsedCount
		summary.SalesImprovedCount = row.SalesImprovedCount
		summary.EstimatedRevenueLiftTotal = row.EstimatedRevenueLiftTotal
		summary.ObservedRevenueLiftTotal = row.ObservedRevenueLiftTotal
		summary.AverageObservedTransactionLiftPercent = row.AverageObservedTransactionLiftPercent
		results[row.RecommendationKey] = summary
	}

	return results, nil
}

func isAIPricingOutcomeAction(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case AIRecommendationFeedbackCampaignUsed, AIRecommendationFeedbackSalesImproved:
		return true
	default:
		return false
	}
}

func ListAIPricingFeedbackOutcomeRecords(db *gorm.DB, outletID uint, recommendationKeys []string) ([]AIPricingFeedbackOutcomeRecord, error) {
	records := make([]AIPricingFeedbackOutcomeRecord, 0)
	if err := EnsureAIPricingFeedbackSchema(db); err != nil {
		return records, err
	}
	if outletID == 0 || len(recommendationKeys) == 0 {
		return records, nil
	}

	normalizedKeys := make([]string, 0, len(recommendationKeys))
	seen := make(map[string]struct{}, len(recommendationKeys))
	for _, key := range recommendationKeys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalizedKeys = append(normalizedKeys, trimmed)
	}
	if len(normalizedKeys) == 0 {
		return records, nil
	}

	err := db.Raw(`
		SELECT
			recommendation_type,
			recommendation_key,
			outcome_type,
			campaign_start_date,
			campaign_end_date
		FROM tblai_pricing_feedback_outcomes
		WHERE outlet_id = ?
			AND recommendation_key IN ?
			AND campaign_start_date IS NOT NULL
			AND campaign_end_date IS NOT NULL
	`, outletID, normalizedKeys).Scan(&records).Error
	return records, err
}
