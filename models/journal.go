package models

import (
	"time"

	"gorm.io/gorm"
)

type JournalEntry struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	OutletID     uint           `json:"outlet_id"`
	Reference    string         `gorm:"size:200" json:"reference"`
	Description  string         `gorm:"size:200" json:"description"`
	EntryDate    time.Time      `json:"entry_date"`
	JournalLines []JournalLine  `gorm:"foreignKey:JournalEntryID" json:"lines"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (JournalEntry) TableName() string {
	return "tbljournal_entries"
}

type JournalLine struct {
	ID             uint    `gorm:"primaryKey" json:"id"`
	JournalEntryID uint    `json:"journal_entry_id"`
	AccountID      string  `gorm:"size:10" json:"account_id"` // contoh: "1101" (Persediaan), "2101" (Hutang), dsb
	Debit          float64 `json:"debit"`
	Credit         float64 `json:"credit"`
	Description    string  `gorm:"size:200" json:"description"`
	Account        Account `gorm:"foreignKey:AccountID;references:ID" json:"account,omitempty"`
}

func (JournalLine) TableName() string {
	return "tbljournal_lines"
}
