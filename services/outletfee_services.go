package services

import (
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"gorm.io/gorm"
)

func GetOutletFee(outletID int64) (*models.Outletfee, error) {
	var outletfee models.Outletfee
	if err := database.DB.First(&outletfee, "outlet_id = ?", outletID).Error; err != nil {
		return nil, err
	}
	return &outletfee, nil
}

func GetOutletFeeTx(tx *gorm.DB, outletID int64) (*models.Outletfee, error) {
	var outletfee models.Outletfee
	if err := tx.First(&outletfee, "outlet_id = ?", outletID).Error; err != nil {
		return nil, err
	}
	return &outletfee, nil
}
