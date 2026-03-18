package services

import (
	"errors"

	"github.com/kadeyasa/possystem/models"
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

	if serviceMode == POSOrderDraftServiceModeLaundryHold {
		return POSOrderDraftFulfillmentStatusNotRequired
	}

	return POSOrderDraftFulfillmentStatusPendingCashier
}

func EnsurePOSOrderDraftSchema(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not initialized")
	}

	if err := db.AutoMigrate(&models.POSOrderDraft{}, &models.POSOrderDraftItem{}); err != nil {
		return err
	}

	statements := []string{
		`ALTER TABLE tblpos_order_drafts ALTER COLUMN status SET DEFAULT 'open'`,
		`ALTER TABLE tblpos_order_drafts ALTER COLUMN source SET DEFAULT 'cashier_hold'`,
		`ALTER TABLE tblpos_order_drafts ALTER COLUMN service_mode SET DEFAULT 'dine_in'`,
		`ALTER TABLE tblpos_order_drafts ADD COLUMN IF NOT EXISTS payment_method VARCHAR(50) DEFAULT 'unassigned'`,
		`ALTER TABLE tblpos_order_drafts ADD COLUMN IF NOT EXISTS payment_status VARCHAR(50) DEFAULT 'draft'`,
		`ALTER TABLE tblpos_order_drafts ADD COLUMN IF NOT EXISTS payment_gateway_provider VARCHAR(120) DEFAULT ''`,
		`ALTER TABLE tblpos_order_drafts ADD COLUMN IF NOT EXISTS payment_gateway_reference VARCHAR(120) DEFAULT ''`,
		`ALTER TABLE tblpos_order_drafts ADD COLUMN IF NOT EXISTS fulfillment_status VARCHAR(50) DEFAULT 'pending_cashier'`,
		`ALTER TABLE tblpos_order_drafts ADD COLUMN IF NOT EXISTS sent_to_kitchen_at TIMESTAMPTZ`,
		`ALTER TABLE tblpos_order_drafts ADD COLUMN IF NOT EXISTS kitchen_started_at TIMESTAMPTZ`,
		`ALTER TABLE tblpos_order_drafts ADD COLUMN IF NOT EXISTS kitchen_completed_at TIMESTAMPTZ`,
		`CREATE INDEX IF NOT EXISTS idx_tblpos_order_drafts_outlet_status ON tblpos_order_drafts (outlet_id, status, updated_at DESC)`,
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
