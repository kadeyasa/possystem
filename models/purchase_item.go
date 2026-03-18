package models

type PurchaseItem struct {
	ID                    uint    `gorm:"primaryKey" json:"id"`
	PurchaseID            uint    `json:"purchase_id"`
	ProductID             uint    `json:"product_id"`
	Quantity              int     `json:"quantity"`
	PurchaseUnit          string  `gorm:"size:32" json:"purchase_unit"`
	PurchaseConversionQty int     `json:"purchase_conversion_qty"`
	StockQuantity         int     `json:"stock_quantity"`
	PurchasePrice         float64 `json:"purchase_price"`
	UnitCostInStockUnit   float64 `json:"unit_cost_in_stock_unit"`
	Total                 float64 `json:"total"`

	Product Product `gorm:"foreignKey:ProductID" json:"product,omitempty"` // optional preload
}

func (PurchaseItem) TableName() string {
	return "tblpurchase_items"
}
