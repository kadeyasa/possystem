package models

import "time"

type Balance struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	OutletID  uint      `json:"outlet_id"`
	Balance   float64   `json:"balance"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Balance) TableName() string {
	return "tblbalance"
}

type BalanceHisotories struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	BalanceId uint      `json:"balance_id"`
	Debit     float64   `json:"debit"`
	Credit    float64   `json:"credit"`
	Remarks   string    `json:"remarks"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (BalanceHisotories) TableName() string {
	return "tblbalancehistories"
}
