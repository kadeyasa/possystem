package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/services"
	"gorm.io/gorm"
)

type ProductRecipeItemInput struct {
	IngredientProductID uint   `json:"ingredient_product_id"`
	QuantityRequired    int    `json:"quantity_required"`
	Note                string `json:"note"`
}

type ProductRecipeUpsertInput struct {
	OutletID  uint                     `json:"outlet_id"`
	ProductID uint                     `json:"product_id"`
	Items     []ProductRecipeItemInput `json:"items"`
}

type ProductRecipeDetailResponse struct {
	OutletID  uint                   `json:"outlet_id"`
	ProductID uint                   `json:"product_id"`
	Product   models.Product         `json:"product"`
	Items     []models.ProductRecipe `json:"items"`
}

func UpsertProductRecipe(c *gin.Context) {
	var input ProductRecipeUpsertInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.OutletID == 0 || input.ProductID == 0 || len(input.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outlet_id, product_id, and items are required"})
		return
	}

	if err := services.EnsureInventorySchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare inventory schema"})
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		products, err := services.GetProductsForInventory(tx, input.OutletID, []uint{input.ProductID})
		if err != nil {
			return err
		}
		product := products[input.ProductID]
		if services.NormalizeProductItemType(product.ItemType) != services.ProductItemTypeFinishedGood {
			return fmt.Errorf("recipe/BOM hanya bisa dipakai untuk product finished_good")
		}

		ingredientIDs := make([]uint, 0, len(input.Items))
		seenIngredients := make(map[uint]struct{}, len(input.Items))
		for _, item := range input.Items {
			if item.IngredientProductID == 0 {
				return fmt.Errorf("ingredient_product_id is required for every recipe item")
			}
			if item.IngredientProductID == input.ProductID {
				return fmt.Errorf("product tidak boleh menjadi ingredient untuk dirinya sendiri")
			}
			if item.QuantityRequired <= 0 {
				return fmt.Errorf("quantity_required must be greater than zero for ingredient %d", item.IngredientProductID)
			}
			if _, ok := seenIngredients[item.IngredientProductID]; ok {
				return fmt.Errorf("duplicate ingredient_product_id %d in recipe", item.IngredientProductID)
			}
			seenIngredients[item.IngredientProductID] = struct{}{}
			ingredientIDs = append(ingredientIDs, item.IngredientProductID)
		}

		ingredients, err := services.GetProductsForInventory(tx, input.OutletID, ingredientIDs)
		if err != nil {
			return err
		}
		for _, item := range input.Items {
			ingredient := ingredients[item.IngredientProductID]
			switch services.NormalizeProductItemType(ingredient.ItemType) {
			case services.ProductItemTypeRawMaterial, services.ProductItemTypeResaleItem:
			default:
				return fmt.Errorf("ingredient %s (%d) harus bertipe raw_material atau resale_item", strings.TrimSpace(ingredient.Name), ingredient.ID)
			}
		}

		if err := tx.Unscoped().
			Where("outlet_id = ? AND product_id = ?", input.OutletID, input.ProductID).
			Delete(&models.ProductRecipe{}).Error; err != nil {
			return err
		}

		for _, item := range input.Items {
			record := models.ProductRecipe{
				OutletID:            input.OutletID,
				ProductID:           input.ProductID,
				IngredientProductID: item.IngredientProductID,
				QuantityRequired:    item.QuantityRequired,
				Note:                strings.TrimSpace(item.Note),
			}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
		}

		if _, err := services.SyncFinishedGoodRecipeCost(tx, input.OutletID, input.ProductID); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var product models.Product
	if err := database.DB.Where("outlet_id = ? AND id = ?", input.OutletID, input.ProductID).First(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var items []models.ProductRecipe
	if err := database.DB.Where("outlet_id = ? AND product_id = ?", input.OutletID, input.ProductID).
		Preload("IngredientProduct").
		Order("ingredient_product_id asc").
		Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ProductRecipeDetailResponse{
		OutletID:  input.OutletID,
		ProductID: input.ProductID,
		Product:   product,
		Items:     items,
	})
}

func GetProductRecipes(c *gin.Context) {
	if err := services.EnsureInventorySchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare inventory schema"})
		return
	}

	outletID := c.Query("outlet_id")
	if strings.TrimSpace(outletID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outlet_id is required"})
		return
	}

	query := database.DB.Model(&models.ProductRecipe{}).
		Where("outlet_id = ?", outletID).
		Preload("Product").
		Preload("IngredientProduct")

	if productID := strings.TrimSpace(c.Query("product_id")); productID != "" {
		query = query.Where("product_id = ?", productID)
	}

	var recipes []models.ProductRecipe
	if err := query.Order("product_id asc, ingredient_product_id asc").Find(&recipes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": recipes})
}

func GetProductRecipeByProduct(c *gin.Context) {
	if err := services.EnsureInventorySchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare inventory schema"})
		return
	}

	outletIDValue := strings.TrimSpace(c.Query("outlet_id"))
	if outletIDValue == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outlet_id is required"})
		return
	}

	outletID64, err := strconv.ParseUint(outletIDValue, 10, 64)
	if err != nil || outletID64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid outlet_id"})
		return
	}

	productIDValue := c.Param("product_id")
	productID64, err := strconv.ParseUint(productIDValue, 10, 64)
	if err != nil || productID64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product_id"})
		return
	}

	var product models.Product
	if err := database.DB.Where("outlet_id = ? AND id = ?", uint(outletID64), uint(productID64)).First(&product).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	var items []models.ProductRecipe
	if err := database.DB.Where("outlet_id = ? AND product_id = ?", uint(outletID64), uint(productID64)).
		Preload("IngredientProduct").
		Order("ingredient_product_id asc").
		Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ProductRecipeDetailResponse{
		OutletID:  uint(outletID64),
		ProductID: uint(productID64),
		Product:   product,
		Items:     items,
	})
}

func DeleteProductRecipe(c *gin.Context) {
	if err := services.EnsureInventorySchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare inventory schema"})
		return
	}

	outletID := strings.TrimSpace(c.Query("outlet_id"))
	if outletID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outlet_id is required"})
		return
	}

	result := database.DB.Unscoped().
		Where("outlet_id = ? AND product_id = ?", outletID, c.Param("product_id")).
		Delete(&models.ProductRecipe{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Recipe not found"})
		return
	}

	if err := database.DB.Model(&models.Product{}).
		Where("outlet_id = ? AND id = ?", outletID, c.Param("product_id")).
		Update("last_purchase_price", 0).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Recipe deleted"})
}
