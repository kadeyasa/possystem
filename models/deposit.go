package models

import "time"

type Deposit struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	OutletID  uint      `json:"outlet_id"`
	Amount    float64   `json:"amount"`
	Status    uint      `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Deposit) TableName() string {
	return "tbldeposit"
}
