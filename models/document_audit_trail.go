package models

import "time"

type DocumentAuditTrail struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	OutletID      uint      `json:"outlet_id"`
	DocumentType  string    `gorm:"size:64" json:"document_type"`
	DocumentID    uint      `json:"document_id"`
	Action        string    `gorm:"size:32" json:"action"`
	Summary       string    `gorm:"size:255" json:"summary"`
	Note          string    `json:"note"`
	Metadata      string    `gorm:"column:metadata_json" json:"metadata_json"`
	BeforeState   string    `gorm:"column:before_state_json" json:"before_state_json"`
	AfterState    string    `gorm:"column:after_state_json" json:"after_state_json"`
	ActorUserID   string    `json:"actor_user_id"`
	ActorType     string    `gorm:"size:32" json:"actor_type"`
	ActorName     string    `gorm:"size:255" json:"actor_name"`
	RequestPath   string    `gorm:"size:255" json:"request_path"`
	IPAddress     string    `gorm:"size:64" json:"ip_address"`
	UserAgent     string    `json:"user_agent"`
	CreatedAt     time.Time `json:"created_at"`
}

func (DocumentAuditTrail) TableName() string {
	return "tbldocument_audit_trails"
}
