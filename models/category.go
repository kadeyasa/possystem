package models

type Category struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	OutletID     uint   `json:"outlet_id"`
	CategoryCode string `gorm:"size:20" json:"category_code"`
	CategoryName string `gorm:"size:200" json:"category_name"`
	Status       bool   `json:"status"`
	Icon         string `json:"icon"`
}

func (Category) TableName() string {
	return "tblcategories"
}
