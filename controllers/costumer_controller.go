package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/utils"
)

// POST /customers
func CreateCustomer(c *gin.Context) {
	var input models.Customer

	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Log.Warnf("❌ Failed to bind JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

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

	if err := database.DB.First(&customer, "id = ?", id).Error; err != nil {
		utils.Log.Warnf("❌ Customer not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Customer not found"})
		return
	}

	c.JSON(http.StatusOK, customer)
}

// GET /customers/:id
func GetCustomerByOutlet(c *gin.Context) {
	id := c.Query("outlet_id")
	var customers []models.Customer

	if err := database.DB.Where("outlet_id = ?", id).Find(&customers).Error; err != nil {
		utils.Log.Warnf("❌ Failed to fetch customers: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch customers"})
		return
	}

	c.JSON(http.StatusOK, customers)
}

// GET /customers
func GetAllCustomers(c *gin.Context) {
	var customers []models.Customer

	if err := database.DB.Find(&customers).Error; err != nil {
		utils.Log.Errorf("❌ Failed to get customers: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get customers"})
		return
	}

	c.JSON(http.StatusOK, customers)
}

// PUT /customers/:id
func UpdateCustomer(c *gin.Context) {
	id := c.Param("id")
	var customer models.Customer

	if err := database.DB.First(&customer, "id = ?", id).Error; err != nil {
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

	if err := database.DB.Model(&customer).Updates(input).Error; err != nil {
		utils.Log.Errorf("❌ Failed to update customer: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update customer"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Customer updated successfully", "data": customer})
}

// DELETE /customers/:id
func DeleteCustomer(c *gin.Context) {
	id := c.Param("id")
	var customer models.Customer

	if err := database.DB.First(&customer, "id = ?", id).Error; err != nil {
		utils.Log.Warnf("❌ Customer not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Customer not found"})
		return
	}

	if err := database.DB.Delete(&customer).Error; err != nil {
		utils.Log.Errorf("❌ Failed to delete customer: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete customer"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Customer deleted"})
}
