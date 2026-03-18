package models

import "time"

type StockAdjustmentApprovalRequest struct {
	ID                   uint       `gorm:"primaryKey" json:"id"`
	OutletID             uint       `json:"outlet_id"`
	Status               string     `json:"status"`
	AdjustmentDate       time.Time  `json:"adjustment_date"`
	StockAdjustmentID    *uint      `json:"stock_adjustment_id"`
	RequestTotal         float64    `json:"request_total"`
	ItemCount            int        `json:"item_count"`
	Reason               string     `json:"reason"`
	RequestNote          string     `json:"request_note"`
	RequestPayload       string     `gorm:"column:request_payload" json:"-"`
	RequestedByUserID    string     `json:"requested_by_user_id"`
	RequestedByActorType string     `json:"requested_by_actor_type"`
	RequestedByName      string     `json:"requested_by_name"`
	RequestedAt          time.Time  `json:"requested_at"`
	ReviewedByUserID     string     `json:"reviewed_by_user_id"`
	ReviewedByActorType  string     `json:"reviewed_by_actor_type"`
	ReviewedByName       string     `json:"reviewed_by_name"`
	ReviewNote           string     `json:"review_note"`
	ReviewedAt           *time.Time `json:"reviewed_at"`
	ApprovedAt           *time.Time `json:"approved_at"`
	RejectedAt           *time.Time `json:"rejected_at"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

func (StockAdjustmentApprovalRequest) TableName() string {
	return "tblstock_adjustment_approval_requests"
}
