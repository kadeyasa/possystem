package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/services"
	"github.com/kadeyasa/possystem/utils"
	"gorm.io/gorm"
)

func CreateDeposit(c *gin.Context) {
	var input models.Deposit
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Log.Warnf("❌ Failed to bind JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.OutletID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OutletID are required"})
		return
	}
	if input.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Amount more than 0 are required"})
		return
	}

	if err := database.DB.Create(&input).Error; err != nil {
		utils.Log.Errorf("❌ Failed to create deposit: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	utils.Log.Infof("✅ Deposit created: %d", input.ID)
	c.JSON(http.StatusOK, gin.H{"message": "Deposit created successfully", "data": input})
}
func GetDepositByOutlet(c *gin.Context) {
	// Ambil outlet_id dari query param
	outletIDStr := c.Query("outlet_id")
	statusStr := c.Query("status")

	var deposits []models.Deposit
	// Convert string -> int
	outletID, err := strconv.Atoi(outletIDStr)

	if err != nil {
		utils.Log.Warnf("❌ Failed to fetch deposit: %v", err)
	}
	status, err := strconv.Atoi(statusStr)

	query := database.DB.Model(&models.Deposit{})

	if outletID != 0 {
		query = query.Where("outlet_id = ?", outletID)
	}

	query = query.Where("status = ?", status)
	query = query.Where("deleted_at IS NULL")

	if err := query.Find(&deposits).Error; err != nil {
		utils.Log.Warnf("❌ Failed to fetch deposit: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch deposit"})
		return
	}

	// Response
	c.JSON(http.StatusOK, gin.H{
		"outlet_id": outletID,
		"status":    status,
		"deposits":  deposits,
	})
}

func GetDepositById(c *gin.Context) {
	idStr := c.Param("id")
	iD, err := strconv.Atoi(idStr)
	if err != nil {
		utils.Log.Warnf("❌ Failed to fetch deposit ID: %v", err)
	}
	var deposits []models.Deposit
	query := database.DB.Model(&models.Deposit{})
	query = query.Where("id = ?", iD)
	if err := query.Find(&deposits).Error; err != nil {
		utils.Log.Warnf("❌ Failed to fetch deposit: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch deposit"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":       iD,
		"deposits": deposits,
	})
}
func ApproveDeposit(c *gin.Context) {
	// Ambil ID deposit dari path param
	depositIDStr := c.Param("id")
	depositID, err := strconv.Atoi(depositIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deposit_id"})
		return
	}

	// Jalankan dalam transaction
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var deposit models.Deposit
		if err := tx.First(&deposit, depositID).Error; err != nil {
			return err
		}

		// Cek apakah sudah approved
		if deposit.Status == 1 {
			return errors.New("deposit already approved")
		}

		// Update status deposit
		deposit.Status = 1
		if err := tx.Save(&deposit).Error; err != nil {
			return err
		}

		// Update balance lewat service (tapi inject tx, bukan DB global)
		if err := services.CreateOrUpdateBalanceTx(tx, services.BalanceInput{
			OutletID: deposit.OutletID,
			Amount:   deposit.Amount,
			Remarks:  "Deposit approved",
		}); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		if err.Error() == "deposit already approved" {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "deposit not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to approve deposit: " + err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "deposit approved and balance updated",
	})
}
