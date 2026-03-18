package services

import (
	"errors"
	"fmt"

	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"gorm.io/gorm"
)

const (
	AccountMappingCopyModeFillMissing     = "fill_missing"
	AccountMappingCopyModeReplaceExisting = "replace_existing"
)

type AccountMappingCopyInput struct {
	SourceOutletID uint
	TargetOutletID uint
	Mode           string
}

type AccountMappingCopyResult struct {
	SourceOutletID uint   `json:"source_outlet_id"`
	TargetOutletID uint   `json:"target_outlet_id"`
	Mode           string `json:"mode"`
	SourceCount    int    `json:"source_count"`
	CreatedCount   int    `json:"created_count"`
	UpdatedCount   int    `json:"updated_count"`
	ReplacedCount  int    `json:"replaced_count"`
	SkippedCount   int    `json:"skipped_count"`
}

func NormalizeAccountMappingCopyMode(value string) string {
	switch normalizeAccountToken(value) {
	case AccountMappingCopyModeReplaceExisting, "replace", "overwrite", "sync":
		return AccountMappingCopyModeReplaceExisting
	default:
		return AccountMappingCopyModeFillMissing
	}
}

func makeAccountMappingKey(transactionType, purpose string) string {
	return normalizeAccountToken(transactionType) + ":" + normalizeAccountToken(purpose)
}

func upsertAccountMasterFromMappingRecord(tx *gorm.DB, input models.AccountMappingRecord) error {
	var existing models.Account
	err := tx.First(&existing, "id = ?", input.ID).Error
	if err == nil {
		existing.Name = input.Name
		existing.Category = input.Category
		existing.IsActive = true
		return tx.Save(&existing).Error
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	return tx.Create(&models.Account{
		ID:       input.ID,
		Name:     input.Name,
		Category: input.Category,
		IsActive: true,
	}).Error
}

func CopyAccountMappings(input AccountMappingCopyInput) (AccountMappingCopyResult, error) {
	result := AccountMappingCopyResult{
		SourceOutletID: input.SourceOutletID,
		TargetOutletID: input.TargetOutletID,
		Mode:           NormalizeAccountMappingCopyMode(input.Mode),
	}

	if input.SourceOutletID == 0 || input.TargetOutletID == 0 {
		return result, fmt.Errorf("source_outlet_id and target_outlet_id are required")
	}
	if input.SourceOutletID == input.TargetOutletID {
		return result, fmt.Errorf("source and target outlet must be different")
	}

	if err := EnsureAccountMappingSchema(database.DB); err != nil {
		return result, err
	}

	var sourceMappings []models.AccountMappingRecord
	if err := BuildAccountMappingRecordQuery(database.DB).
		Where("mappings.outlet_id = ? AND mappings.is_active = true", input.SourceOutletID).
		Order("mappings.transaction_type ASC, mappings.purpose ASC, mappings.id ASC").
		Scan(&sourceMappings).Error; err != nil {
		return result, err
	}
	if len(sourceMappings) == 0 {
		return result, fmt.Errorf("no active mappings found in source outlet %d", input.SourceOutletID)
	}
	result.SourceCount = len(sourceMappings)

	var targetMappings []models.AccountMappingRecord
	if err := BuildAccountMappingRecordQuery(database.DB).
		Where("mappings.outlet_id = ?", input.TargetOutletID).
		Order("mappings.transaction_type ASC, mappings.purpose ASC, mappings.id ASC").
		Scan(&targetMappings).Error; err != nil {
		return result, err
	}

	targetByKey := make(map[string]models.AccountMappingRecord, len(targetMappings))
	for _, mapping := range targetMappings {
		targetByKey[makeAccountMappingKey(mapping.TransactionType, mapping.Purpose)] = mapping
	}

	tx := database.DB.Begin()
	if tx.Error != nil {
		return result, tx.Error
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			tx.Rollback()
			panic(recovered)
		}
	}()

	for _, source := range sourceMappings {
		if err := upsertAccountMasterFromMappingRecord(tx, source); err != nil {
			tx.Rollback()
			return result, fmt.Errorf("failed to sync account master %s: %w", source.ID, err)
		}

		key := makeAccountMappingKey(source.TransactionType, source.Purpose)
		target, exists := targetByKey[key]
		if !exists {
			created := models.AccountMapping{
				OutletID:        input.TargetOutletID,
				AccountID:       source.ID,
				TransactionType: normalizeAccountToken(source.TransactionType),
				Purpose:         normalizeAccountToken(source.Purpose),
				IsActive:        source.IsActive,
			}
			if err := tx.Create(&created).Error; err != nil {
				tx.Rollback()
				return result, fmt.Errorf("failed to create mapping %s for target outlet %d: %w", key, input.TargetOutletID, err)
			}
			result.CreatedCount++
			targetByKey[key] = models.AccountMappingRecord{
				MappingID:       created.ID,
				ID:              created.AccountID,
				Name:            source.Name,
				Category:        source.Category,
				IsActive:        created.IsActive,
				OutletID:        created.OutletID,
				TransactionType: created.TransactionType,
				Purpose:         created.Purpose,
			}
			continue
		}

		if result.Mode == AccountMappingCopyModeFillMissing {
			result.SkippedCount++
			continue
		}

		if target.ID == source.ID {
			if target.IsActive == source.IsActive {
				result.SkippedCount++
				continue
			}

			var existing models.AccountMapping
			if err := tx.First(&existing, "id = ?", target.MappingID).Error; err != nil {
				tx.Rollback()
				return result, fmt.Errorf("failed to load existing mapping %s in target outlet %d: %w", key, input.TargetOutletID, err)
			}

			existing.IsActive = source.IsActive
			if err := tx.Save(&existing).Error; err != nil {
				tx.Rollback()
				return result, fmt.Errorf("failed to update existing mapping %s in target outlet %d: %w", key, input.TargetOutletID, err)
			}

			result.UpdatedCount++
			targetByKey[key] = models.AccountMappingRecord{
				MappingID:       existing.ID,
				ID:              existing.AccountID,
				Name:            source.Name,
				Category:        source.Category,
				IsActive:        existing.IsActive,
				OutletID:        existing.OutletID,
				TransactionType: existing.TransactionType,
				Purpose:         existing.Purpose,
			}
			continue
		}

		if err := tx.Delete(&models.AccountMapping{}, target.MappingID).Error; err != nil {
			tx.Rollback()
			return result, fmt.Errorf("failed to replace mapping %s in target outlet %d: %w", key, input.TargetOutletID, err)
		}

		replacement := models.AccountMapping{
			OutletID:        input.TargetOutletID,
			AccountID:       source.ID,
			TransactionType: normalizeAccountToken(source.TransactionType),
			Purpose:         normalizeAccountToken(source.Purpose),
			IsActive:        source.IsActive,
		}
		if err := tx.Create(&replacement).Error; err != nil {
			tx.Rollback()
			return result, fmt.Errorf("failed to create replacement mapping %s in target outlet %d: %w", key, input.TargetOutletID, err)
		}

		result.ReplacedCount++
		targetByKey[key] = models.AccountMappingRecord{
			MappingID:       replacement.ID,
			ID:              replacement.AccountID,
			Name:            source.Name,
			Category:        source.Category,
			IsActive:        replacement.IsActive,
			OutletID:        replacement.OutletID,
			TransactionType: replacement.TransactionType,
			Purpose:         replacement.Purpose,
		}
	}

	if err := tx.Commit().Error; err != nil {
		return result, err
	}

	return result, nil
}
