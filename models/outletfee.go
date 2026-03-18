package models

type Outletfee struct {
	ID         uint    `json:"id"`
	OutletID   uint    `json:"outlet_id"`
	FeeSetting float64 `json:"fee_setting"`
}

func (Outletfee) TableName() string {
	return "tbloutletfee"
}
