package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/utils"
)

func GetUserBalance(c *gin.Context) {
	outlet := c.Query("outlet_id")
	var balance models.Balance
	if err := database.DB.First(&balance, "outlet_id = ?", outlet).Error; err != nil {
		utils.Log.Warnf("❌ Customer not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Customer not found"})
		return
	}
	c.JSON(http.StatusOK, balance)
}

func GetAllBalance(c *gin.Context) {
	var balances []models.Balance
	query := database.DB.Model(&models.Balance{})
	if err := query.Find(&balances).Error; err != nil {
		utils.Log.Warnf("❌ Failed to fetch deposit: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch deposit"})
		return
	}
	// Response
	c.JSON(http.StatusOK, gin.H{
		"balances": balances,
	})
}
