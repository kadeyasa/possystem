package services

import (
	"errors"
	"time"

	"github.com/kadeyasa/possystem/models"
	"gorm.io/gorm"
)

type BalanceInput struct {
	OutletID uint    `json:"outlet_id"`
	Amount   float64 `json:"amount"`
	Remarks  string  `json:"remarks"`
}

func CreateOrUpdateBalanceTx(tx *gorm.DB, input BalanceInput) error {
	var balance models.Balance

	if err := tx.Where("outlet_id = ?", input.OutletID).First(&balance).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Buat baru
			balance = models.Balance{
				OutletID:  input.OutletID,
				Balance:   input.Amount,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			if err := tx.Create(&balance).Error; err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		// Update saldo existing
		balance.Balance += input.Amount
		balance.UpdatedAt = time.Now()
		if err := tx.Save(&balance).Error; err != nil {
			return err
		}
	}

	// Simpan history
	h := models.BalanceHisotories{
		BalanceId: balance.ID,
		Remarks:   input.Remarks,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if input.Amount < 0 {
		h.Credit = -input.Amount
	} else {
		h.Debit = input.Amount
	}

	return tx.Create(&h).Error
}
