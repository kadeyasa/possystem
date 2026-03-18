package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/services"
	"github.com/kadeyasa/possystem/utils"
	"gorm.io/gorm"
)

type accountMappingCopyInput struct {
	SourceOutletID uint   `json:"source_outlet_id"`
	TargetOutletID uint   `json:"target_outlet_id"`
	Mode           string `json:"mode"`
}

func normalizeAccountPayload(input *models.Account) {
	input.ID = strings.TrimSpace(input.ID)
	input.Name = strings.TrimSpace(input.Name)
	input.Category = strings.TrimSpace(input.Category)
	input.TransactionType = strings.ToLower(strings.TrimSpace(input.TransactionType))
	input.Purpose = strings.ToLower(strings.TrimSpace(input.Purpose))
}

func validateAccountPayload(input models.Account) string {
	if input.ID == "" || input.Name == "" || input.Category == "" || input.TransactionType == "" || input.Purpose == "" {
		return "ID, Name, Category, transaction_type, and purpose are required"
	}
	if err := services.ValidateAccountCategoryForMapping(input.Category, input.TransactionType, input.Purpose); err != nil {
		return err.Error()
	}

	return ""
}

func applyAccountOutletScope(query *gorm.DB, outletID string) *gorm.DB {
	outletID = strings.TrimSpace(outletID)
	if outletID == "" {
		return query
	}

	return query.Where("(mappings.outlet_id = ? OR mappings.outlet_id = 0)", outletID)
}

func getAccountMappingsBaseQuery() *gorm.DB {
	return services.BuildAccountMappingRecordQuery(database.DB)
}

func getAccountMappingRecordByMappingID(mappingID uint) (models.AccountMappingRecord, error) {
	var record models.AccountMappingRecord
	err := getAccountMappingsBaseQuery().
		Where("mappings.id = ?", mappingID).
		Take(&record).Error

	return record, err
}

func upsertAccountMaster(tx *gorm.DB, input models.Account) error {
	var existing models.Account
	err := tx.First(&existing, "id = ?", input.ID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Create(&models.Account{
			ID:       input.ID,
			Name:     input.Name,
			Category: input.Category,
			IsActive: true,
		}).Error
	}
	if err != nil {
		return err
	}

	existing.Name = input.Name
	existing.Category = input.Category
	existing.IsActive = true

	return tx.Save(&existing).Error
}

func CreateAccount(c *gin.Context) {
	if !ensureAccountingSyncAdminAccess(c) {
		return
	}
	if err := services.EnsureAccountMappingSchema(database.DB); err != nil {
		utils.Log.Errorf("❌ Failed to ensure POS account mapping schema: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare account mapping schema"})
		return
	}

	var input models.Account
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Log.Warnf("❌ Failed to bind JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	normalizeAccountPayload(&input)
	if validationError := validateAccountPayload(input); validationError != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": validationError})
		return
	}

	tx := database.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := upsertAccountMaster(tx, input); err != nil {
		tx.Rollback()
		utils.Log.Errorf("❌ Failed to upsert account master: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save account master"})
		return
	}

	var existing models.AccountMapping
	err := tx.Where(
		"outlet_id = ? AND transaction_type = ? AND purpose = ?",
		input.OutletID,
		input.TransactionType,
		input.Purpose,
	).Take(&existing).Error
	if err == nil {
		tx.Rollback()
		c.JSON(http.StatusConflict, gin.H{"error": "Mapping already exists for this outlet, transaction type, and purpose"})
		return
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		tx.Rollback()
		utils.Log.Errorf("❌ Failed to check existing mapping: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate account mapping"})
		return
	}

	mapping := models.AccountMapping{
		OutletID:        input.OutletID,
		AccountID:       input.ID,
		TransactionType: input.TransactionType,
		Purpose:         input.Purpose,
		IsActive:        input.IsActive,
	}
	if err := tx.Create(&mapping).Error; err != nil {
		tx.Rollback()
		utils.Log.Errorf("❌ Failed to create account mapping: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit account mapping"})
		return
	}

	record, err := getAccountMappingRecordByMappingID(mapping.ID)
	if err != nil {
		utils.Log.Warnf("⚠️ Account mapping created but failed to reload mapping %d: %v", mapping.ID, err)
		c.JSON(http.StatusOK, gin.H{"message": "Account mapping created successfully", "data": mapping})
		return
	}

	utils.Log.Infof("✅ Account mapping created: outlet=%d %s -> %s:%s", input.OutletID, input.ID, input.TransactionType, input.Purpose)
	c.JSON(http.StatusOK, gin.H{"message": "Account mapping created successfully", "data": record})
}

func UpdateAccount(c *gin.Context) {
	if !ensureAccountingSyncAdminAccess(c) {
		return
	}
	if err := services.EnsureAccountMappingSchema(database.DB); err != nil {
		utils.Log.Errorf("❌ Failed to ensure POS account mapping schema: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare account mapping schema"})
		return
	}

	mappingID, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || mappingID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Account mapping ID is required"})
		return
	}

	var existing models.AccountMapping
	if err := database.DB.First(&existing, "id = ?", mappingID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Account mapping not found"})
		return
	}

	var input models.Account
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Log.Warnf("❌ Failed to bind JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	normalizeAccountPayload(&input)
	if validationError := validateAccountPayload(input); validationError != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": validationError})
		return
	}
	if input.ID != existing.AccountID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Account code cannot be changed. Create a new mapping and deactivate the old one instead."})
		return
	}

	tx := database.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := upsertAccountMaster(tx, input); err != nil {
		tx.Rollback()
		utils.Log.Errorf("❌ Failed to upsert account master: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save account master"})
		return
	}

	var conflict models.AccountMapping
	err = tx.Where(
		"outlet_id = ? AND transaction_type = ? AND purpose = ? AND id <> ?",
		input.OutletID,
		input.TransactionType,
		input.Purpose,
		mappingID,
	).Take(&conflict).Error
	if err == nil {
		tx.Rollback()
		c.JSON(http.StatusConflict, gin.H{"error": "Another mapping already exists for this outlet, transaction type, and purpose"})
		return
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		tx.Rollback()
		utils.Log.Errorf("❌ Failed to validate account mapping uniqueness: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate account mapping"})
		return
	}

	existing.OutletID = input.OutletID
	existing.AccountID = input.ID
	existing.TransactionType = input.TransactionType
	existing.Purpose = input.Purpose
	existing.IsActive = input.IsActive

	if err := tx.Save(&existing).Error; err != nil {
		tx.Rollback()
		utils.Log.Errorf("❌ Failed to update account mapping: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit account mapping"})
		return
	}

	record, err := getAccountMappingRecordByMappingID(existing.ID)
	if err != nil {
		utils.Log.Warnf("⚠️ Account mapping updated but failed to reload mapping %d: %v", existing.ID, err)
		c.JSON(http.StatusOK, gin.H{"message": "Account mapping updated successfully", "data": existing})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Account mapping updated successfully", "data": record})
}

func CopyAccountMappings(c *gin.Context) {
	if !ensureAccountingSyncAdminAccess(c) {
		return
	}

	var input accountMappingCopyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Log.Warnf("❌ Failed to bind account mapping copy payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mode := services.NormalizeAccountMappingCopyMode(input.Mode)
	result, err := services.CopyAccountMappings(services.AccountMappingCopyInput{
		SourceOutletID: input.SourceOutletID,
		TargetOutletID: input.TargetOutletID,
		Mode:           mode,
	})
	if err != nil {
		statusCode := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "failed") {
			statusCode = http.StatusInternalServerError
		}
		c.JSON(statusCode, gin.H{
			"error": err.Error(),
		})
		return
	}

	utils.Log.Infof(
		"✅ Copied POS account mappings: source=%d target=%d mode=%s created=%d updated=%d replaced=%d skipped=%d",
		result.SourceOutletID,
		result.TargetOutletID,
		result.Mode,
		result.CreatedCount,
		result.UpdatedCount,
		result.ReplacedCount,
		result.SkippedCount,
	)
	c.JSON(http.StatusOK, gin.H{
		"message": "Account mappings copied successfully",
		"data":    result,
	})
}

// GET /accounts/:id
func GetAccountByID(c *gin.Context) {
	if !ensureAccountingSyncAdminAccess(c) {
		return
	}
	if err := services.EnsureAccountMappingSchema(database.DB); err != nil {
		utils.Log.Errorf("❌ Failed to ensure POS account mapping schema: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare account mapping schema"})
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	outletID := strings.TrimSpace(c.Query("outlet_id"))

	var account models.AccountMappingRecord
	query := getAccountMappingsBaseQuery().Where("mappings.account_id = ?", id)
	query = applyAccountOutletScope(query, outletID)

	if err := query.
		Order("mappings.outlet_id DESC, mappings.id DESC").
		Take(&account).Error; err == nil {
		c.JSON(http.StatusOK, account)
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		utils.Log.Warnf("❌ Failed to get account mapping: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get account mapping"})
		return
	}

	if mappingID, err := strconv.ParseUint(id, 10, 64); err == nil && mappingID > 0 {
		record, findErr := getAccountMappingRecordByMappingID(uint(mappingID))
		if findErr == nil {
			c.JSON(http.StatusOK, record)
			return
		}
	}

	utils.Log.Warnf("❌ Account mapping not found for %s", id)
	c.JSON(http.StatusNotFound, gin.H{"error": "Account mapping not found"})
}

// GET /accounts
func GetAllAccounts(c *gin.Context) {
	if !ensureAccountingSyncAdminAccess(c) {
		return
	}
	if err := services.EnsureAccountMappingSchema(database.DB); err != nil {
		utils.Log.Errorf("❌ Failed to ensure POS account mapping schema: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare account mapping schema"})
		return
	}

	var accounts []models.AccountMappingRecord
	outletID := strings.TrimSpace(c.Query("outlet_id"))

	query := getAccountMappingsBaseQuery()
	query = applyAccountOutletScope(query, outletID)

	if err := query.
		Order("mappings.outlet_id DESC, mappings.transaction_type ASC, mappings.purpose ASC, mappings.id ASC").
		Scan(&accounts).Error; err != nil {
		utils.Log.Errorf("❌ Failed to get account mappings: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get account mappings"})
		return
	}

	c.JSON(http.StatusOK, accounts)
}

// GET /accounts/filter?transaction_type=purchase&purpose=inventory
func GetAccountsByFilter(c *gin.Context) {
	if !ensureAccountingSyncAdminAccess(c) {
		return
	}
	if err := services.EnsureAccountMappingSchema(database.DB); err != nil {
		utils.Log.Errorf("❌ Failed to ensure POS account mapping schema: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare account mapping schema"})
		return
	}

	transactionType := strings.ToLower(strings.TrimSpace(c.Query("transaction_type")))
	purpose := strings.ToLower(strings.TrimSpace(c.Query("purpose")))
	outletID := strings.TrimSpace(c.Query("outlet_id"))

	var accounts []models.AccountMappingRecord
	query := getAccountMappingsBaseQuery().Where("mappings.is_active = true")
	if transactionType != "" {
		query = query.Where("mappings.transaction_type = ?", transactionType)
	}
	if purpose != "" {
		query = query.Where("mappings.purpose = ?", purpose)
	}
	query = applyAccountOutletScope(query, outletID)

	if err := query.
		Order("mappings.outlet_id DESC, mappings.id ASC").
		Scan(&accounts).Error; err != nil {
		utils.Log.Errorf("❌ Failed to filter account mappings: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to filter account mappings"})
		return
	}

	c.JSON(http.StatusOK, accounts)
}

func DeleteAccount(c *gin.Context) {
	if !ensureAccountingSyncAdminAccess(c) {
		return
	}
	if err := services.EnsureAccountMappingSchema(database.DB); err != nil {
		utils.Log.Errorf("❌ Failed to ensure POS account mapping schema: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare account mapping schema"})
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if mappingID, err := strconv.ParseUint(id, 10, 64); err == nil && mappingID > 0 {
		if err := database.DB.Delete(&models.AccountMapping{}, mappingID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete account mapping"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Account mapping deleted"})
		return
	}

	outletID := strings.TrimSpace(c.Query("outlet_id"))
	query := database.DB.Where("account_id = ?", id)
	if outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}

	result := query.Delete(&models.AccountMapping{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete account mapping"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Account mapping not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Account mapping deleted"})
}
