package models

import "time"

type Customer struct {
	ID        int64      `gorm:"primaryKey;column:id" json:"id"`
	OutletID  int64      `gorm:"primaryKey;column:outlet_id" json:"outlet_id"`
	Name      string     `gorm:"column:name" json:"name"`
	Address   string     `gorm:"column:address" json:"address"`
	Email     string     `gorm:"column:email" json:"email"`
	Telp      string     `gorm:"column:telp" json:"telp"`
	CreatedAt time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt *time.Time `gorm:"column:deleted_at" json:"deleted_at"` // nullable
}

// TableName sets the insert table name for this struct type
func (Customer) TableName() string {
	return "tblcustomers"
}
