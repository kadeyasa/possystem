package controllers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/services"
	"gorm.io/gorm"
)

type VendorInput struct {
	OutletID               uint   `json:"outlet_id"`
	VendorName             string `json:"vendor_name"`
	ContactName            string `json:"contact_name"`
	Phone                  string `json:"phone"`
	Email                  string `json:"email"`
	Address                string `json:"address"`
	DefaultPaymentTermDays int    `json:"default_payment_term_days"`
	IsActive               *bool  `json:"is_active"`
	Note                   string `json:"note"`
}

func normalizeVendorMasterName(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func normalizeVendorText(value string) string {
	return strings.TrimSpace(value)
}

func parseBoolQuery(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y":
		return true
	default:
		return false
	}
}

func CreateVendor(c *gin.Context) {
	if err := services.EnsureVendorMasterSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var input VendorInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vendorName := normalizeVendorMasterName(input.VendorName)
	if input.OutletID == 0 || vendorName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outlet_id and vendor_name are required"})
		return
	}
	if input.DefaultPaymentTermDays < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "default_payment_term_days must be zero or greater"})
		return
	}

	var existing models.Vendor
	err := database.DB.
		Where("outlet_id = ? AND LOWER(TRIM(vendor_name)) = ?", input.OutletID, strings.ToLower(vendorName)).
		First(&existing).Error
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Vendor dengan nama tersebut sudah ada di outlet ini"})
		return
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	vendor := models.Vendor{
		OutletID:               input.OutletID,
		VendorName:             vendorName,
		ContactName:            normalizeVendorMasterName(input.ContactName),
		Phone:                  normalizeVendorText(input.Phone),
		Email:                  normalizeVendorText(input.Email),
		Address:                normalizeVendorText(input.Address),
		DefaultPaymentTermDays: input.DefaultPaymentTermDays,
		IsActive:               isActive,
		Note:                   normalizeVendorText(input.Note),
	}

	if err := database.DB.Create(&vendor).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Vendor created successfully", "data": vendor})
}

func GetVendors(c *gin.Context) {
	if err := services.EnsureVendorMasterSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	query := database.DB.Model(&models.Vendor{})

	if outletID := strings.TrimSpace(c.Query("outlet_id")); outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}
	if !parseBoolQuery(c.Query("include_inactive")) {
		query = query.Where("is_active = ?", true)
	}
	if search := strings.TrimSpace(c.Query("q")); search != "" {
		like := "%" + search + "%"
		query = query.Where(
			"vendor_name ILIKE ? OR contact_name ILIKE ? OR phone ILIKE ? OR email ILIKE ? OR note ILIKE ?",
			like, like, like, like, like,
		)
	}

	var vendors []models.Vendor
	if err := query.Order("is_active desc, vendor_name asc, id asc").Find(&vendors).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, vendors)
}

func GetVendorByID(c *gin.Context) {
	if err := services.EnsureVendorMasterSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var vendor models.Vendor
	if err := database.DB.First(&vendor, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Vendor not found"})
		return
	}

	c.JSON(http.StatusOK, vendor)
}

func UpdateVendor(c *gin.Context) {
	if err := services.EnsureVendorMasterSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var vendor models.Vendor
	if err := database.DB.First(&vendor, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Vendor not found"})
		return
	}

	var input VendorInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	nextOutletID := vendor.OutletID
	if input.OutletID != 0 {
		nextOutletID = input.OutletID
	}

	vendorName := normalizeVendorMasterName(input.VendorName)
	if vendorName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "vendor_name is required"})
		return
	}
	if input.DefaultPaymentTermDays < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "default_payment_term_days must be zero or greater"})
		return
	}

	var existing models.Vendor
	err := database.DB.
		Where("id <> ? AND outlet_id = ? AND LOWER(TRIM(vendor_name)) = ?", vendor.ID, nextOutletID, strings.ToLower(vendorName)).
		First(&existing).Error
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Vendor dengan nama tersebut sudah ada di outlet ini"})
		return
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	isActive := vendor.IsActive
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	updates := map[string]interface{}{
		"outlet_id":                 nextOutletID,
		"vendor_name":               vendorName,
		"contact_name":              normalizeVendorMasterName(input.ContactName),
		"phone":                     normalizeVendorText(input.Phone),
		"email":                     normalizeVendorText(input.Email),
		"address":                   normalizeVendorText(input.Address),
		"default_payment_term_days": input.DefaultPaymentTermDays,
		"is_active":                 isActive,
		"note":                      normalizeVendorText(input.Note),
	}

	if err := database.DB.Model(&vendor).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := database.DB.First(&vendor, vendor.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Vendor updated successfully", "data": vendor})
}
