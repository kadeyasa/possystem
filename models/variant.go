package models

import "time"

type Variant struct {
	ID            int64      `gorm:"primaryKey;column:id" json:"id"`
	OutletID      int64      `gorm:"primaryKey;column:outlet_id" json:"outlet_id"`
	ItemID        int64      `gorm:"primaryKey;column:item_id" json:"item_id"`
	Metode        string     `gorm:"column:metode" json:"metode"`
	Waktu         string     `gorm:"column:waktu" json:"waktu"`
	Durasi        string     `gorm:"column:durasi" json:"durasi"`
	HargaOnline   float64    `gorm:"column:harga_online" json:"harga_online"`
	HargaOffline  float64    `gorm:"column:harga_offline" json:"harga_offline"`
	BiayaProduksi float64    `gorm:"column:biaya_produksi" json:"biaya_produksi"`
	CreatedAt     *time.Time `gorm:"column:created_at" json:"created_at"` // nullable
	DeletedAt     *time.Time `gorm:"column:deleted_at" json:"deleted_at"` // nullable
}

// TableName sets the insert table name for this struct type
func (Variant) TableName() string {
	return "tblvariants"
}
