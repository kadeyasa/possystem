package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/utils"
)

// Create
func CreateCategory(c *gin.Context) {
	var input models.Category
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Log.Warnf("❌ Bind JSON failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := database.DB.Create(&input).Error; err != nil {
		utils.Log.Errorf("❌ DB insert error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	utils.Log.Infof("✅ Category created: %v", input.CategoryName)
	c.JSON(http.StatusOK, input)
}

// Read (List All)
func GetAllCategories(c *gin.Context) {
	outletID := c.Query("outlet_id")
	var categories []models.Category
	if outletID != "" {
		if err := database.DB.Where("outlet_id = ?", outletID).Find(&categories).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data"})
			return
		}
	} else {
		// Tanpa filter outlet_id
		database.DB.Find(&categories)
	}
	c.JSON(http.StatusOK, categories)
}

func GetCategoriesByPage(c *gin.Context) {
	outletID := c.Query("outlet_id")
	pageStr := c.Query("page")
	limitStr := c.Query("limit")
	page, err1 := strconv.Atoi(pageStr)
	limit, err2 := strconv.Atoi(limitStr)
	if outletID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Outlet"})
		return
	}
	if err1 != nil || err2 != nil || page < 1 || limit < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pagination parameters"})
		return
	}
	offset := (page - 1) * limit
	var total int64
	var categories []models.Category
	database.DB.Model(&models.Category{}).Where("outlet_id = ?", outletID).Count(&total)

	if outletID != "" {
		if err := database.DB.Limit(limit).Offset(offset).Where("outlet_id = ?", outletID).Find(&categories).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data"})
			return
		}
	}
	totalPages := int((total + int64(limit) - 1) / int64(limit)) // pembulatan ke atas
	c.JSON(http.StatusOK, gin.H{
		"data":       categories,
		"page":       page,
		"limit":      limit,
		"total":      total,
		"totalPages": totalPages,
	})
}

// Read by ID
func GetCategoryByID(c *gin.Context) {
	id := c.Param("id")
	var category models.Category
	if err := database.DB.First(&category, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	}
	c.JSON(http.StatusOK, category)
}

// Update
func UpdateCategory(c *gin.Context) {
	id := c.Param("id")
	var category models.Category
	if err := database.DB.First(&category, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	}

	var input models.Category
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	database.DB.Model(&category).Updates(input)
	c.JSON(http.StatusOK, category)
}

// Delete
func DeleteCategory(c *gin.Context) {
	id := c.Param("id")
	var category models.Category
	if err := database.DB.First(&category, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	}
	database.DB.Delete(&category)
	c.JSON(http.StatusOK, gin.H{"message": "Category deleted"})
}
