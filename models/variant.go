package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

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

type variantJSON struct {
	ID            int64           `json:"id"`
	OutletID      int64           `json:"outlet_id"`
	ItemID        int64           `json:"item_id"`
	Metode        string          `json:"metode"`
	Waktu         string          `json:"waktu"`
	Durasi        json.RawMessage `json:"durasi"`
	HargaOnline   float64         `json:"harga_online"`
	HargaOffline  float64         `json:"harga_offline"`
	BiayaProduksi float64         `json:"biaya_produksi"`
	CreatedAt     *time.Time      `json:"created_at"`
	DeletedAt     *time.Time      `json:"deleted_at"`
}

// UnmarshalJSON keeps the DB-backed string field compatible while accepting
// either a JSON string or number from newer POS clients.
func (v *Variant) UnmarshalJSON(data []byte) error {
	var payload variantJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	durasi, err := normalizeVariantDurasi(payload.Durasi)
	if err != nil {
		return err
	}

	v.ID = payload.ID
	v.OutletID = payload.OutletID
	v.ItemID = payload.ItemID
	v.Metode = payload.Metode
	v.Waktu = payload.Waktu
	v.Durasi = durasi
	v.HargaOnline = payload.HargaOnline
	v.HargaOffline = payload.HargaOffline
	v.BiayaProduksi = payload.BiayaProduksi
	v.CreatedAt = payload.CreatedAt
	v.DeletedAt = payload.DeletedAt

	return nil
}

func normalizeVariantDurasi(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}

	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", nil
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return strings.TrimSpace(asString), nil
	}

	var asNumber float64
	if err := json.Unmarshal(raw, &asNumber); err == nil {
		return strconv.FormatFloat(asNumber, 'f', -1, 64), nil
	}

	return "", fmt.Errorf("invalid durasi value: %s", trimmed)
}

// TableName sets the insert table name for this struct type
func (Variant) TableName() string {
	return "tblvariants"
}
