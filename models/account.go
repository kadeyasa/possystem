package models

type Account struct {
	ID              string `gorm:"primaryKey" json:"id"`
	Name            string `json:"name"`
	Category        string `json:"category"`
	IsActive        bool   `json:"is_active"`
	OutletID        uint   `json:"outlet_id"`
	TransactionType string `json:"transaction_type"`
	Purpose         string `json:"purpose"`
}

func (Account) TableName() string {
	return "tblaccounts"
}
