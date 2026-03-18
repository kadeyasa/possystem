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

func CreateOutletFee(c *gin.Context) {
	if !ensureAccountingSyncAdminAccess(c) {
		return
	}

	var input models.Outletfee
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Log.Warnf("❌ Failed to bind JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.OutletID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Outlet required"})
		return
	}

	var existing models.Outletfee
	err := database.DB.Where("outlet_id = ?", input.OutletID).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		utils.Log.Errorf("❌ Failed to load feesetting: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := database.DB.Create(&input).Error; err != nil {
			utils.Log.Errorf("❌ Failed to create feesetting: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		utils.Log.Infof("✅ Outlet Fee created: %d", input.ID)
		c.JSON(http.StatusOK, gin.H{"message": "Outlet fee setting created successfully", "data": input})
		return
	}

	existing.FeeSetting = input.FeeSetting
	if err := database.DB.Save(&existing).Error; err != nil {
		utils.Log.Errorf("❌ Failed to update feesetting: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	utils.Log.Infof("✅ Outlet Fee updated for outlet: %d", existing.OutletID)
	c.JSON(http.StatusOK, gin.H{"message": "Outlet fee setting updated successfully", "data": existing})
}

func GetOutletFeeHandler(c *gin.Context) {
	if !ensureAccountingSyncAdminAccess(c) {
		return
	}

	outletStr := c.Query("outlet_id")
	if outletStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outlet_id is required"})
		return
	}

	outletID, err := strconv.ParseInt(outletStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid outlet_id"})
		return
	}

	outletfee, err := services.GetOutletFee(outletID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "feesetting not found"})
		return
	}

	c.JSON(http.StatusOK, outletfee)
}
