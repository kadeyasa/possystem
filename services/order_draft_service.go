package services

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const (
	POSOrderDraftStatusOpen      = "open"
	POSOrderDraftStatusPaid      = "paid"
	POSOrderDraftStatusCancelled = "cancelled"

	POSOrderDraftSourceCashierHold    = "cashier_hold"
	POSOrderDraftSourceGuestSelfOrder = "guest_self_order"

	POSOrderDraftServiceModeDineIn      = "dine_in"
	POSOrderDraftServiceModeTakeAway    = "take_away"
	POSOrderDraftServiceModeDelivery    = "delivery"
	POSOrderDraftServiceModeLaundryHold = "laundry_hold"
	POSOrderDraftServiceModeCounterHold = "counter_hold"

	POSOrderDraftPaymentMethodUnassigned = "unassigned"
	POSOrderDraftPaymentMethodCash       = "cash"
	POSOrderDraftPaymentMethodQris       = "qris"

	POSOrderDraftPaymentStatusDraft                = "draft"
	POSOrderDraftPaymentStatusAwaitingCash         = "awaiting_cash"
	POSOrderDraftPaymentStatusPendingGateway       = "pending_gateway"
	POSOrderDraftPaymentStatusPendingConfiguration = "pending_configuration"
	POSOrderDraftPaymentStatusPaid                 = "paid"
	POSOrderDraftPaymentStatusCancelled            = "cancelled"

	POSOrderDraftFulfillmentStatusNotRequired    = "not_required"
	POSOrderDraftFulfillmentStatusPendingCashier = "pending_cashier"
	POSOrderDraftFulfillmentStatusQueuedKitchen  = "queued_kitchen"
	POSOrderDraftFulfillmentStatusInKitchen      = "in_kitchen"
	POSOrderDraftFulfillmentStatusReady          = "ready"
)

// Keep migration structs relation-free so repeated schema checks do not try to
// synthesize foreign keys against older databases that are missing primary-key
// metadata on tblpos_order_drafts.
type posOrderDraftSchema struct {
	ID                      uint  `gorm:"primaryKey"`
	OutletID                uint  `gorm:"index"`
	TransactionID           *uint `gorm:"column:transaction_id"`
	CashierID               uint
	CashierName             string     `gorm:"column:cashier_name"`
	CustomerName            string     `gorm:"column:customer_name"`
	CustomerPhone           string     `gorm:"column:customer_phone"`
	CustomerID              *uint      `gorm:"column:customer_id;index"`
	OrderLabel              string     `gorm:"column:order_label"`
	TableLabel              string     `gorm:"column:table_label"`
	ServiceMode             string     `gorm:"column:service_mode"`
	Source                  string     `gorm:"column:source"`
	Status                  string     `gorm:"column:status"`
	PaymentMethod           string     `gorm:"column:payment_method"`
	PaymentStatus           string     `gorm:"column:payment_status"`
	PaymentGatewayProvider  string     `gorm:"column:payment_gateway_provider"`
	PaymentGatewayReference string     `gorm:"column:payment_gateway_reference"`
	FulfillmentStatus       string     `gorm:"column:fulfillment_status"`
	SentToKitchenAt         *time.Time `gorm:"column:sent_to_kitchen_at"`
	KitchenStartedAt        *time.Time `gorm:"column:kitchen_started_at"`
	KitchenCompletedAt      *time.Time `gorm:"column:kitchen_completed_at"`
	Note                    string     `gorm:"column:note"`
	Subtotal                float64    `gorm:"column:subtotal"`
	DiscountPercent         float64    `gorm:"column:discount_percent"`
	Discount                float64    `gorm:"column:discount"`
	ServicePercent          float64    `gorm:"column:service_percent"`
	Service                 float64    `gorm:"column:service"`
	TaxPercent              float64    `gorm:"column:tax_percent"`
	Tax                     float64    `gorm:"column:tax"`
	Total                   float64    `gorm:"column:total"`
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

func (posOrderDraftSchema) TableName() string {
	return "tblpos_order_drafts"
}

type posOrderDraftItemSchema struct {
	ID           uint    `gorm:"primaryKey"`
	OrderDraftID uint    `gorm:"column:order_draft_id;index"`
	ProductID    uint    `gorm:"column:product_id"`
	VariantID    *int64  `gorm:"column:variant_id"`
	ProductName  string  `gorm:"column:product_name"`
	Quantity     int     `gorm:"column:quantity"`
	UnitPrice    float64 `gorm:"column:unit_price"`
	Total        float64 `gorm:"column:total"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (posOrderDraftItemSchema) TableName() string {
	return "tblpos_order_draft_items"
}

func NormalizePOSOrderDraftStatus(value string) string {
	switch normalizeAccountToken(value) {
	case POSOrderDraftStatusPaid:
		return POSOrderDraftStatusPaid
	case POSOrderDraftStatusCancelled:
		return POSOrderDraftStatusCancelled
	default:
		return POSOrderDraftStatusOpen
	}
}

func NormalizePOSOrderDraftSource(value string) string {
	switch normalizeAccountToken(value) {
	case POSOrderDraftSourceGuestSelfOrder:
		return POSOrderDraftSourceGuestSelfOrder
	default:
		return POSOrderDraftSourceCashierHold
	}
}

func NormalizePOSOrderDraftServiceMode(value string) string {
	switch normalizeAccountToken(value) {
	case POSOrderDraftServiceModeTakeAway:
		return POSOrderDraftServiceModeTakeAway
	case POSOrderDraftServiceModeDelivery:
		return POSOrderDraftServiceModeDelivery
	case POSOrderDraftServiceModeLaundryHold:
		return POSOrderDraftServiceModeLaundryHold
	case POSOrderDraftServiceModeCounterHold:
		return POSOrderDraftServiceModeCounterHold
	default:
		return POSOrderDraftServiceModeDineIn
	}
}

func NormalizePOSOrderDraftPaymentMethod(value string) string {
	switch normalizeAccountToken(value) {
	case POSOrderDraftPaymentMethodCash:
		return POSOrderDraftPaymentMethodCash
	case POSOrderDraftPaymentMethodQris:
		return POSOrderDraftPaymentMethodQris
	default:
		return POSOrderDraftPaymentMethodUnassigned
	}
}

func NormalizePOSOrderDraftPaymentStatus(value string) string {
	switch normalizeAccountToken(value) {
	case POSOrderDraftPaymentStatusAwaitingCash:
		return POSOrderDraftPaymentStatusAwaitingCash
	case POSOrderDraftPaymentStatusPendingGateway:
		return POSOrderDraftPaymentStatusPendingGateway
	case POSOrderDraftPaymentStatusPendingConfiguration:
		return POSOrderDraftPaymentStatusPendingConfiguration
	case POSOrderDraftPaymentStatusPaid:
		return POSOrderDraftPaymentStatusPaid
	case POSOrderDraftPaymentStatusCancelled:
		return POSOrderDraftPaymentStatusCancelled
	default:
		return POSOrderDraftPaymentStatusDraft
	}
}

func ResolvePOSOrderDraftPaymentStatus(method, requested string) string {
	method = NormalizePOSOrderDraftPaymentMethod(method)
	requested = NormalizePOSOrderDraftPaymentStatus(requested)

	if requested != POSOrderDraftPaymentStatusDraft {
		return requested
	}

	switch method {
	case POSOrderDraftPaymentMethodCash:
		return POSOrderDraftPaymentStatusAwaitingCash
	case POSOrderDraftPaymentMethodQris:
		return POSOrderDraftPaymentStatusPendingGateway
	default:
		return POSOrderDraftPaymentStatusDraft
	}
}

func NormalizePOSOrderDraftFulfillmentStatus(value string) string {
	switch normalizeAccountToken(value) {
	case POSOrderDraftFulfillmentStatusPendingCashier:
		return POSOrderDraftFulfillmentStatusPendingCashier
	case POSOrderDraftFulfillmentStatusQueuedKitchen:
		return POSOrderDraftFulfillmentStatusQueuedKitchen
	case POSOrderDraftFulfillmentStatusInKitchen:
		return POSOrderDraftFulfillmentStatusInKitchen
	case POSOrderDraftFulfillmentStatusReady:
		return POSOrderDraftFulfillmentStatusReady
	default:
		return POSOrderDraftFulfillmentStatusNotRequired
	}
}

func ResolvePOSOrderDraftFulfillmentStatus(serviceMode, requested string) string {
	serviceMode = NormalizePOSOrderDraftServiceMode(serviceMode)
	requested = NormalizePOSOrderDraftFulfillmentStatus(requested)

	if requested != POSOrderDraftFulfillmentStatusNotRequired {
		return requested
	}

	if serviceMode == POSOrderDraftServiceModeLaundryHold || serviceMode == POSOrderDraftServiceModeCounterHold {
		return POSOrderDraftFulfillmentStatusNotRequired
	}

	return POSOrderDraftFulfillmentStatusPendingCashier
}

func EnsurePOSOrderDraftSchema(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not initialized")
	}

	if err := db.AutoMigrate(&posOrderDraftSchema{}, &posOrderDraftItemSchema{}); err != nil {
		return err
	}

	statements := []string{
		buildPrimaryKeyConstraintStatement("public.tblpos_order_drafts", "tblpos_order_drafts_pkey"),
		buildPrimaryKeyConstraintStatement("public.tblpos_order_draft_items", "tblpos_order_draft_items_pkey"),
		`ALTER TABLE tblpos_order_drafts ALTER COLUMN status SET DEFAULT 'open'`,
		`ALTER TABLE tblpos_order_drafts ALTER COLUMN source SET DEFAULT 'cashier_hold'`,
		`ALTER TABLE tblpos_order_drafts ALTER COLUMN service_mode SET DEFAULT 'dine_in'`,
		`ALTER TABLE tblpos_order_drafts ADD COLUMN IF NOT EXISTS customer_id BIGINT`,
		`ALTER TABLE tblpos_order_drafts ADD COLUMN IF NOT EXISTS payment_method VARCHAR(50) DEFAULT 'unassigned'`,
		`ALTER TABLE tblpos_order_drafts ADD COLUMN IF NOT EXISTS payment_status VARCHAR(50) DEFAULT 'draft'`,
		`ALTER TABLE tblpos_order_drafts ADD COLUMN IF NOT EXISTS payment_gateway_provider VARCHAR(120) DEFAULT ''`,
		`ALTER TABLE tblpos_order_drafts ADD COLUMN IF NOT EXISTS payment_gateway_reference VARCHAR(120) DEFAULT ''`,
		`ALTER TABLE tblpos_order_drafts ADD COLUMN IF NOT EXISTS fulfillment_status VARCHAR(50) DEFAULT 'pending_cashier'`,
		`ALTER TABLE tblpos_order_drafts ADD COLUMN IF NOT EXISTS sent_to_kitchen_at TIMESTAMPTZ`,
		`ALTER TABLE tblpos_order_drafts ADD COLUMN IF NOT EXISTS kitchen_started_at TIMESTAMPTZ`,
		`ALTER TABLE tblpos_order_drafts ADD COLUMN IF NOT EXISTS kitchen_completed_at TIMESTAMPTZ`,
		`ALTER TABLE tblpos_order_drafts ADD COLUMN IF NOT EXISTS service_percent DOUBLE PRECISION DEFAULT 0`,
		`ALTER TABLE tblpos_order_drafts ADD COLUMN IF NOT EXISTS service DOUBLE PRECISION DEFAULT 0`,
		`CREATE INDEX IF NOT EXISTS idx_tblpos_order_drafts_outlet_status ON tblpos_order_drafts (outlet_id, status, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_tblpos_order_drafts_customer ON tblpos_order_drafts (customer_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tblpos_order_drafts_source ON tblpos_order_drafts (source, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_tblpos_order_drafts_payment_status ON tblpos_order_drafts (payment_status, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_tblpos_order_drafts_fulfillment_status ON tblpos_order_drafts (fulfillment_status, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_tblpos_order_drafts_transaction_id ON tblpos_order_drafts (transaction_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tblpos_order_draft_items_order_draft ON tblpos_order_draft_items (order_draft_id)`,
		`ALTER TABLE tblpos_order_draft_items ADD COLUMN IF NOT EXISTS variant_id BIGINT`,
		`CREATE INDEX IF NOT EXISTS idx_tblpos_order_draft_items_variant ON tblpos_order_draft_items (variant_id)`,
	}

	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}

	return nil
}

func buildPrimaryKeyConstraintStatement(tableName, constraintName string) string {
	return fmt.Sprintf(`
DO $$
BEGIN
	IF to_regclass('%[1]s') IS NOT NULL
		AND NOT EXISTS (
			SELECT 1
			FROM pg_constraint
			WHERE conrelid = '%[1]s'::regclass
				AND contype = 'p'
		) THEN
		ALTER TABLE %[1]s ADD CONSTRAINT %[2]s PRIMARY KEY (id);
	END IF;
END $$;`, tableName, constraintName)
}
