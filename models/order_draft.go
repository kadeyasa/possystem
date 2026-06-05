package models

import "time"

type POSOrderDraft struct {
	ID                      uint                `gorm:"primaryKey" json:"id"`
	OutletID                uint                `gorm:"index" json:"outlet_id"`
	TransactionID           *uint               `gorm:"column:transaction_id" json:"transaction_id"`
	CashierID               uint                `json:"cashier_id"`
	CashierName             string              `gorm:"column:cashier_name" json:"cashier_name"`
	CustomerName            string              `gorm:"column:customer_name" json:"customer_name"`
	CustomerPhone           string              `gorm:"column:customer_phone" json:"customer_phone"`
	CustomerID              *uint               `gorm:"column:customer_id" json:"customer_id"`
	OrderLabel              string              `gorm:"column:order_label" json:"order_label"`
	TableLabel              string              `gorm:"column:table_label" json:"table_label"`
	ServiceMode             string              `gorm:"column:service_mode" json:"service_mode"`
	Source                  string              `gorm:"column:source" json:"source"`
	Status                  string              `gorm:"column:status" json:"status"`
	PaymentMethod           string              `gorm:"column:payment_method" json:"payment_method"`
	PaymentStatus           string              `gorm:"column:payment_status" json:"payment_status"`
	PaymentGatewayProvider  string              `gorm:"column:payment_gateway_provider" json:"payment_gateway_provider"`
	PaymentGatewayReference string              `gorm:"column:payment_gateway_reference" json:"payment_gateway_reference"`
	FulfillmentStatus       string              `gorm:"column:fulfillment_status" json:"fulfillment_status"`
	SentToKitchenAt         *time.Time          `gorm:"column:sent_to_kitchen_at" json:"sent_to_kitchen_at"`
	KitchenStartedAt        *time.Time          `gorm:"column:kitchen_started_at" json:"kitchen_started_at"`
	KitchenCompletedAt      *time.Time          `gorm:"column:kitchen_completed_at" json:"kitchen_completed_at"`
	Note                    string              `gorm:"column:note" json:"note"`
	Subtotal                float64             `gorm:"column:subtotal" json:"subtotal"`
	DiscountPercent         float64             `gorm:"column:discount_percent" json:"discount_percent"`
	Discount                float64             `gorm:"column:discount" json:"discount"`
	ServicePercent          float64             `gorm:"column:service_percent" json:"service_percent"`
	Service                 float64             `gorm:"column:service" json:"service"`
	TaxPercent              float64             `gorm:"column:tax_percent" json:"tax_percent"`
	Tax                     float64             `gorm:"column:tax" json:"tax"`
	Total                   float64             `gorm:"column:total" json:"total"`
	Items                   []POSOrderDraftItem `gorm:"foreignKey:OrderDraftID" json:"items"`
	CreatedAt               time.Time           `json:"created_at"`
	UpdatedAt               time.Time           `json:"updated_at"`
}

func (POSOrderDraft) TableName() string {
	return "tblpos_order_drafts"
}

type POSOrderDraftItem struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	OrderDraftID uint      `gorm:"column:order_draft_id;index" json:"order_draft_id"`
	ProductID    uint      `gorm:"column:product_id" json:"product_id"`
	VariantID    *int64    `gorm:"column:variant_id" json:"variant_id"`
	ProductName  string    `gorm:"column:product_name" json:"product_name"`
	Quantity     int       `gorm:"column:quantity" json:"quantity"`
	UnitPrice    float64   `gorm:"column:unit_price" json:"unit_price"`
	Total        float64   `gorm:"column:total" json:"total"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (POSOrderDraftItem) TableName() string {
	return "tblpos_order_draft_items"
}
