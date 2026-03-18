package controllers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/services"
	"github.com/kadeyasa/possystem/utils"
)

const maxUploadSize = 5 << 20 // 5MB

func applyProductInventoryDefaults(product *models.Product) {
	if product == nil {
		return
	}

	switch product.ItemType {
	case services.ProductItemTypeFinishedGood, services.ProductItemTypeService:
		product.Stock = 0
	}
}

func validateProductInventoryConfig(product *models.Product) error {
	if product == nil {
		return fmt.Errorf("invalid product payload")
	}

	services.ApplyProductPurchaseDefaults(product)
	if services.IsStockTrackedItemType(product.ItemType) && strings.TrimSpace(product.Satuan) == "" {
		return fmt.Errorf("satuan stok wajib diisi untuk product inventory")
	}

	switch product.ItemType {
	case services.ProductItemTypeRawMaterial, services.ProductItemTypeResaleItem:
		if product.PurchaseConversionQty <= 0 {
			return fmt.Errorf("purchase_conversion_qty must be greater than zero")
		}
	}

	return nil
}

// Create Product
func CreateProduct(c *gin.Context) {
	var input models.Product

	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Log.Warnf("❌ Bind JSON failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := services.EnsureInventorySchema(database.DB); err != nil {
		utils.Log.Errorf("❌ Failed to ensure inventory schema: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare inventory schema"})
		return
	}

	input.ItemType = services.NormalizeProductItemType(input.ItemType)
	if input.ItemType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "item_type must be raw_material, resale_item, finished_good, or service"})
		return
	}
	applyProductInventoryDefaults(&input)
	if err := validateProductInventoryConfig(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// ✅ Check if category exists
	var category models.Category
	if err := database.DB.First(&category, input.CategoryID).Error; err != nil {
		utils.Log.Warnf("❌ Invalid category_id: %v", input.CategoryID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category_id. Category not found."})
		return
	}

	// ✅ Check if product with same code already exists
	var count int64
	if err := database.DB.Model(&models.Product{}).
		Where("code = ?", input.Code).
		Count(&count).Error; err != nil {
		utils.Log.Warnf("❌ Query error on code check: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error checking product code"})
		return
	}
	if count > 0 {
		utils.Log.Warnf("❌ Product code already exists: %v", count)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product code already exists"})
		return
	}

	// ✅ Save product
	if err := database.DB.Create(&input).Error; err != nil {
		utils.Log.Errorf("❌ Failed to save product: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	utils.Log.Infof("✅ Product created: %v", input.Name)
	c.JSON(http.StatusOK, input)
}

// Get All Products
func GetAllProducts(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")
	outlet := c.Query("outlet_id")
	page, err1 := strconv.Atoi(pageStr)
	limit, err2 := strconv.Atoi(limitStr)
	if err1 != nil || err2 != nil || page < 1 || limit < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pagination parameters"})
		return
	}
	offset := (page - 1) * limit
	var total int64
	var products []models.Product
	database.DB.Model(&models.Product{}).Count(&total)

	if err := database.DB.
		Limit(limit).Offset(offset).
		Where("outlet_id=?", outlet).
		Find(&products).Error; err != nil {
		utils.Log.Errorf("❌ Failed to fetch products: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	totalPages := int((total + int64(limit) - 1) / int64(limit)) // pembulatan ke atas

	c.JSON(http.StatusOK, gin.H{
		"data":       products,
		"page":       page,
		"limit":      limit,
		"total":      total,
		"totalPages": totalPages,
	})
}

// Get Product by ID
func GetProductByID(c *gin.Context) {
	id := c.Param("id")
	var product models.Product
	if err := database.DB.First(&product, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}
	c.JSON(http.StatusOK, product)
}

// GetProductByCat mengembalikan semua produk berdasarkan category_id
func GetProductByCat(c *gin.Context) {
	categoryID := c.Query("cat_id")

	if categoryID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Category ID is required"})
		return
	}

	var products []models.Product

	if err := database.DB.Where("category_id = ?", categoryID).Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
		return
	}

	if len(products) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No products found for this category"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": products,
	})
}

func GetProductFilter(c *gin.Context) {
	categoryID := c.Query("cat_id")
	metodeCuci := c.Query("metode")
	durasiCuci := c.Query("durasi")
	outletID := c.Query("outlet_id")

	var products []models.Product

	// Mulai query
	query := database.DB.
		Model(&models.Product{}).
		Distinct("tblproducts.*"). // ambil produk unik walau join ke banyak variant
		Joins("JOIN tblvariants v ON v.item_id = tblproducts.id")

	if outletID != "" {
		query = query.Where("tblproducts.outlet_id = ?", outletID)
		query = query.Where("(v.outlet_id = ? OR v.outlet_id IS NULL OR v.outlet_id = 0)", outletID)
	}

	// filter kategori
	if categoryID != "" && categoryID != "All" {
		query = query.Where("tblproducts.category_id = ?", categoryID)
	}

	// filter metode
	if metodeCuci != "" && metodeCuci != "null" {
		query = query.Where("v.metode = ?", metodeCuci)
	}

	// filter durasi
	if durasiCuci != "" && durasiCuci != "null" {
		query = query.Where("(v.waktu = ? OR CAST(v.durasi AS TEXT) = ?)", durasiCuci, durasiCuci)
	}

	// Ambil data + preload variants
	if outletID != "" {
		query = query.Preload("Variants", "outlet_id = ? OR outlet_id IS NULL OR outlet_id = 0", outletID)
	} else {
		query = query.Preload("Variants")
	}

	if err := query.Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, products)
}

// Update Product
func UpdateProduct(c *gin.Context) {
	id := c.Param("id")
	var product models.Product
	if err := database.DB.First(&product, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	var input models.Product
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Log.Warnf("❌ Invalid update input: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := services.EnsureInventorySchema(database.DB); err != nil {
		utils.Log.Errorf("❌ Failed to ensure inventory schema: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare inventory schema"})
		return
	}

	if strings.TrimSpace(input.ItemType) == "" {
		input.ItemType = services.NormalizeProductItemType(product.ItemType)
	} else {
		input.ItemType = services.NormalizeProductItemType(input.ItemType)
		if input.ItemType == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "item_type must be raw_material, resale_item, finished_good, or service"})
			return
		}
	}

	if input.ItemType != services.ProductItemTypeFinishedGood {
		var recipeCount int64
		if err := database.DB.Model(&models.ProductRecipe{}).
			Where("outlet_id = ? AND product_id = ?", product.OutletID, product.ID).
			Count(&recipeCount).Error; err != nil {
			utils.Log.Errorf("❌ Failed to validate product recipe: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate product recipe"})
			return
		}
		if recipeCount > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Product dengan recipe/BOM harus tetap bertipe finished_good"})
			return
		}
	}
	applyProductInventoryDefaults(&input)
	if err := validateProductInventoryConfig(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{
		"outlet_id":               input.OutletID,
		"category_id":             input.CategoryID,
		"code":                    input.Code,
		"name":                    input.Name,
		"item_type":               input.ItemType,
		"price":                   input.Price,
		"last_purchase_price":     input.LastPurchasePrice,
		"stock":                   input.Stock,
		"image_url":               input.ImageUrl,
		"is_active":               input.IsActive,
		"satuan":                  input.Satuan,
		"purchase_unit":           input.PurchaseUnit,
		"purchase_conversion_qty": input.PurchaseConversionQty,
	}

	if err := database.DB.Model(&product).Updates(updates).Error; err != nil {
		utils.Log.Errorf("❌ Failed to update product: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	outletID := input.OutletID
	if outletID == 0 {
		outletID = product.OutletID
	}
	switch input.ItemType {
	case services.ProductItemTypeFinishedGood:
		if _, err := services.SyncFinishedGoodRecipeCost(database.DB, outletID, product.ID); err != nil {
			utils.Log.Errorf("❌ Failed to sync finished good recipe cost: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sync finished good HPP"})
			return
		}
	case services.ProductItemTypeRawMaterial, services.ProductItemTypeResaleItem:
		if err := services.SyncFinishedGoodRecipeCostsForIngredients(database.DB, outletID, []uint{product.ID}); err != nil {
			utils.Log.Errorf("❌ Failed to sync dependent recipe cost: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sync dependent recipe HPP"})
			return
		}
	}

	utils.Log.Infof("✅ Product updated: %v", product.Name)
	c.JSON(http.StatusOK, product)
}

// Delete Product
func DeleteProduct(c *gin.Context) {
	id := c.Param("id")
	var product models.Product
	if err := database.DB.First(&product, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	if err := database.DB.Delete(&product).Error; err != nil {
		utils.Log.Errorf("❌ Failed to delete product: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	utils.Log.Infof("✅ Product deleted: %v", product.Name)
	c.JSON(http.StatusOK, gin.H{"message": "Product deleted"})
}

func UploadImage(c *gin.Context) {

	// Ambil file dari form field bernama "image"
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No image is received"})
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true}

	if !allowedExts[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file extension"})
		return
	}

	if file.Size > maxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large (max 5MB)"})
		return
	}
	// Optional: Validasi MIME
	openedFile, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot open file"})
		return
	}
	defer openedFile.Close()

	buffer := make([]byte, 512)
	_, err = openedFile.Read(buffer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot read file"})
		return
	}

	contentType := http.DetectContentType(buffer)
	if !strings.HasPrefix(contentType, "image/") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only image files are allowed"})
		return
	}

	// Cek atau buat folder uploads
	uploadPath := "uploads"
	if _, err := os.Stat(uploadPath); os.IsNotExist(err) {
		err := os.MkdirAll(uploadPath, 0750)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload directory"})
			return
		}
	}

	// Buat nama unik untuk file
	safeName := filepath.Base(file.Filename)
	filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), safeName)
	filePath := filepath.Join(uploadPath, filename)

	// Simpan file ke disk
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save image"})
		return
	}

	// Sukses upload
	c.JSON(http.StatusOK, gin.H{
		"message":  "Image uploaded successfully",
		"filename": filename,
		"path":     filePath,
	})
}
