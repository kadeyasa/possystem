package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/utils"
)

func CreateItem(c *gin.Context) {
	var input models.Variant
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Log.Warnf("❌ Bind JSON failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	//validasi required data
	if input.ItemID == 0 || input.Metode == "" || input.Waktu == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ItemID, Metode, dan Waktu wajib diisi."})
		return
	}
	//validasi product check in product data
	var product models.Product
	if err := database.DB.First(&product, input.ItemID).Error; err != nil {
		utils.Log.Warnf("❌ Invalid Product ID: %v", input.ItemID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product_id. Product not found."})
		return
	}
	//save items in database
	if err := database.DB.Create(&input).Error; err != nil {
		utils.Log.Errorf("❌ Gagal menyimpan item: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan item"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"error":   0,
		"message": "Item berhasil dibuat",
		"data":    input,
	})
}

func GetItems(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")
	outlet := c.Query("outlet_id")
	if outlet == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Outlet ID"})
		return
	}
	page, err1 := strconv.Atoi(pageStr)
	limit, err2 := strconv.Atoi(limitStr)
	if err1 != nil || err2 != nil || page < 1 || limit < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pagination parameters"})
		return
	}
	offset := (page - 1) * limit

	var total int64
	var items []struct {
		models.Variant        // Struct utamamu
		ProductName    string `json:"product_name"`
		CategoryName   string `json:"category_name"`
		Satuan         string `json:"satuan"`
	}

	// Hitung total data (tanpa limit dan offset, tapi tetap pakai where)
	if err := database.DB.
		Table("tblvariants").
		Joins("JOIN tblproducts ON tblvariants.item_id = tblproducts.id").
		Joins("JOIN tblcategories ON tblproducts.category_id = tblcategories.id").
		Where("tblvariants.outlet_id = ?", outlet).
		Where("tblvariants.deleted_at IS NULL").
		Count(&total).Error; err != nil {
		utils.Log.Errorf("❌ Failed to count items: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count items"})
		return
	}

	// Ambil data dengan join
	if err := database.DB.
		Table("tblvariants").
		Select("tblvariants.*, tblproducts.name as product_name, tblproducts.satuan as satuan,tblcategories.category_name as category_name").
		Joins("JOIN tblproducts ON tblvariants.item_id = tblproducts.id").
		Joins("JOIN tblcategories ON tblproducts.category_id = tblcategories.id").
		Where("tblvariants.outlet_id = ?", outlet).
		Where("tblvariants.deleted_at IS NULL").
		Limit(limit).
		Offset(offset).
		Scan(&items).Error; err != nil {
		utils.Log.Errorf("❌ Failed to fetch product variants with join: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	c.JSON(http.StatusOK, gin.H{
		"data":       items,
		"page":       page,
		"limit":      limit,
		"total":      total,
		"totalPages": totalPages,
	})
}

func UpdateItem(c *gin.Context) {
	id := c.Param("id")
	var item models.Variant

	if err := database.DB.First(&item, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}
	var input models.Variant
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Log.Warnf("❌ Invalid update input: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := database.DB.Model(&item).Updates(input).Error; err != nil {
		utils.Log.Errorf("❌ Failed to update product: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	utils.Log.Infof("✅ Item updated: %v", item.ItemID)
	c.JSON(http.StatusOK, item)
}
func GetItem(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Id is required"})
		return
	}

	var item struct {
		models.Variant        // asumsi ini struct utamamu untuk tblvariants
		ProductName    string `json:"product_name"`
		CategoryName   string `json:"category_name"`
		Satuan         string `json:"satuan"`
		CatID          int64  `json:"cat_id"`
	}

	if err := database.DB.
		Table("tblvariants").
		Select("tblvariants.*, tblproducts.name AS product_name, tblproducts.satuan AS satuan,tblcategories.category_name AS category_name,tblcategories.id as cat_id").
		Joins("JOIN tblproducts ON tblvariants.item_id = tblproducts.id").
		Joins("JOIN tblcategories ON tblproducts.category_id = tblcategories.id").
		Where("tblvariants.id = ?", id).
		Where("tblvariants.deleted_at IS NULL").
		First(&item).Error; err != nil {
		utils.Log.Warnf("❌ Item not found with ID: %v", id)
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	c.JSON(http.StatusOK, item)
}

func DeleteItem(c *gin.Context) {
	id := c.Param("id")
	var item models.Variant
	if err := database.DB.First(&item, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	if err := database.DB.Delete(&item).Error; err != nil {
		utils.Log.Errorf("❌ Failed to delete item: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	utils.Log.Infof("✅ Item deleted: %v", item.ID)
	c.JSON(http.StatusOK, gin.H{"message": "Product deleted"})
}
