package controllers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/utils"
	"gorm.io/gorm"
)

// POST /customers
func CreateCustomer(c *gin.Context) {
	var input models.Customer

	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Log.Warnf("❌ Failed to bind JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input.Name = strings.Join(strings.Fields(strings.TrimSpace(input.Name)), " ")
	input.Address = strings.TrimSpace(input.Address)
	input.Email = strings.TrimSpace(input.Email)
	input.Telp = strings.TrimSpace(input.Telp)
	input.DeletedAt = nil

	if input.Name == "" || input.Telp == "" || input.OutletID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name, Telp, and OutletID are required"})
		return
	}

	if err := database.DB.Create(&input).Error; err != nil {
		utils.Log.Errorf("❌ Failed to create customer: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	utils.Log.Infof("✅ Customer created: %d - %s", input.ID, input.Name)
	c.JSON(http.StatusOK, gin.H{"message": "Customer created successfully", "data": input})
}

// GET /customers/:id
func GetCustomerByID(c *gin.Context) {
	id := c.Param("id")
	var customer models.Customer

	query := database.DB.Model(&models.Customer{}).Where("id = ?", id)
	if !parseBoolQuery(c.Query("include_deleted")) {
		query = query.Where("deleted_at IS NULL")
	}

	if err := query.First(&customer).Error; err != nil {
		utils.Log.Warnf("❌ Customer not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Customer not found"})
		return
	}

	c.JSON(http.StatusOK, customer)
}

// GET /customers/outlet
func GetCustomerByOutlet(c *gin.Context) {
	outletID := strings.TrimSpace(c.Query("outlet_id"))
	if outletID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outlet_id is required"})
		return
	}

	var customers []models.Customer
	query := database.DB.Model(&models.Customer{}).
		Where("outlet_id = ?", outletID).
		Where("deleted_at IS NULL")

	if search := strings.TrimSpace(c.Query("q")); search != "" {
		like := "%" + search + "%"
		query = query.Where(
			"name ILIKE ? OR telp ILIKE ? OR email ILIKE ? OR address ILIKE ?",
			like, like, like, like,
		)
	}

	if err := query.Order("updated_at DESC, id DESC").Find(&customers).Error; err != nil {
		utils.Log.Warnf("❌ Failed to fetch customers: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch customers"})
		return
	}

	c.JSON(http.StatusOK, customers)
}

// GET /customers
func GetAllCustomers(c *gin.Context) {
	page := parsePositiveIntQueryParam(c.Query("page"), 1, 100000)
	limit := parsePositiveIntQueryParam(c.Query("limit"), 10, 200)
	offset := (page - 1) * limit
	outletID := strings.TrimSpace(c.Query("outlet_id"))
	search := strings.TrimSpace(c.Query("q"))
	includeDeleted := parseBoolQuery(c.Query("include_deleted"))

	baseQuery := database.DB.Model(&models.Customer{})
	if outletID != "" {
		baseQuery = baseQuery.Where("outlet_id = ?", outletID)
	}
	if !includeDeleted {
		baseQuery = baseQuery.Where("deleted_at IS NULL")
	}
	if search != "" {
		like := "%" + search + "%"
		baseQuery = baseQuery.Where(
			"name ILIKE ? OR telp ILIKE ? OR email ILIKE ? OR address ILIKE ?",
			like, like, like, like,
		)
	}

	countQuery := baseQuery.Session(&gorm.Session{})
	listQuery := baseQuery.Session(&gorm.Session{})

	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		utils.Log.Errorf("❌ Failed to count customers: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count customers"})
		return
	}

	var customers []models.Customer
	if err := listQuery.
		Order("updated_at DESC, id DESC").
		Limit(limit).
		Offset(offset).
		Find(&customers).Error; err != nil {
		utils.Log.Errorf("❌ Failed to get customers: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get customers"})
		return
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}

	c.JSON(http.StatusOK, gin.H{
		"data":           customers,
		"page":           page,
		"limit":          limit,
		"total":          total,
		"totalPages":     totalPages,
		"currentPage":    page,
		"totalItems":     total,
		"includeDeleted": includeDeleted,
	})
}

// PUT /customers/:id
func UpdateCustomer(c *gin.Context) {
	id := c.Param("id")
	var customer models.Customer

	if err := database.DB.Where("id = ?", id).Where("deleted_at IS NULL").First(&customer).Error; err != nil {
		utils.Log.Warnf("❌ Customer not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Customer not found"})
		return
	}

	var input models.Customer
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Log.Warnf("❌ Failed to bind JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	name := strings.Join(strings.Fields(strings.TrimSpace(input.Name)), " ")
	address := strings.TrimSpace(input.Address)
	email := strings.TrimSpace(input.Email)
	telp := strings.TrimSpace(input.Telp)
	outletID := customer.OutletID
	if input.OutletID != 0 {
		outletID = input.OutletID
	}

	if name == "" || telp == "" || outletID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name, Telp, and OutletID are required"})
		return
	}

	updates := map[string]interface{}{
		"outlet_id":  outletID,
		"name":       name,
		"address":    address,
		"email":      email,
		"telp":       telp,
		"updated_at": time.Now(),
	}

	if err := database.DB.Model(&customer).Updates(updates).Error; err != nil {
		utils.Log.Errorf("❌ Failed to update customer: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update customer"})
		return
	}

	if err := database.DB.First(&customer, "id = ?", customer.ID).Error; err != nil {
		utils.Log.Errorf("❌ Failed to reload customer after update: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reload customer"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Customer updated successfully", "data": customer})
}

// DELETE /customers/:id
func DeleteCustomer(c *gin.Context) {
	id := c.Param("id")
	var customer models.Customer

	if err := database.DB.Where("id = ?", id).Where("deleted_at IS NULL").First(&customer).Error; err != nil {
		utils.Log.Warnf("❌ Customer not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Customer not found"})
		return
	}

	now := time.Now()
	if err := database.DB.Model(&customer).Updates(map[string]interface{}{
		"deleted_at": now,
		"updated_at": now,
	}).Error; err != nil {
		utils.Log.Errorf("❌ Failed to delete customer: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete customer"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Customer deleted", "deleted_at": now})
}
