package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/utils"
)

func CreateMasterWaktu(c *gin.Context) {
	var input models.MasterWaktu
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Log.Warnf("❌ Bind JSON failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.OutletID == 0 || input.OutletID < 1 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Outlet ID Required"})
		return
	}
	if err := database.DB.Create(&input).Error; err != nil {
		utils.Log.Errorf("❌ DB insert error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	utils.Log.Infof("✅ Waktu created: %v", input.Description)
	c.JSON(http.StatusOK, input)
}

func GetAllWaktu(c *gin.Context) {
	outletID := c.Query("outlet_id")
	var Waktus []models.MasterWaktu
	if outletID != "" {
		if err := database.DB.
			Where("outlet_id = ? OR outlet_id IS NULL OR outlet_id = 0", outletID).
			Order("description ASC").
			Find(&Waktus).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data"})
			return
		}
	} else {
		// Tanpa filter outlet_id
		database.DB.Order("description ASC").Find(&Waktus)
	}
	c.JSON(http.StatusOK, Waktus)
}

func DeleteWaktu(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Id tidak boleh kosong"})
		return
	}
	var Waktu models.MasterWaktu
	if err := database.DB.First(&Waktu, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Waktu not found"})
		return
	}
	database.DB.Delete(&Waktu)
	c.JSON(http.StatusOK, gin.H{"message": "Waktu deleted"})
}
