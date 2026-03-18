package services

import (
	"strings"

	"github.com/kadeyasa/possystem/models"
	"gorm.io/gorm"
)

func NormalizeInventoryUnitLabel(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func NormalizePurchaseConversionQty(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}

func ApplyProductPurchaseDefaults(product *models.Product) {
	if product == nil {
		return
	}

	product.Satuan = NormalizeInventoryUnitLabel(product.Satuan)
	product.PurchaseUnit = NormalizeInventoryUnitLabel(product.PurchaseUnit)
	product.PurchaseConversionQty = NormalizePurchaseConversionQty(product.PurchaseConversionQty)

	switch NormalizeProductItemType(product.ItemType) {
	case ProductItemTypeRawMaterial, ProductItemTypeResaleItem:
		if product.PurchaseUnit == "" {
			product.PurchaseUnit = product.Satuan
		}
	default:
		if product.PurchaseUnit == "" {
			product.PurchaseUnit = product.Satuan
		}
		product.PurchaseConversionQty = 1
	}
}

func NormalizePurchaseItemForInventory(item *models.PurchaseItem, product *models.Product) {
	if item == nil {
		return
	}

	item.PurchaseUnit = NormalizeInventoryUnitLabel(item.PurchaseUnit)
	item.PurchaseConversionQty = NormalizePurchaseConversionQty(item.PurchaseConversionQty)

	if product != nil {
		if item.PurchaseUnit == "" {
			item.PurchaseUnit = NormalizeInventoryUnitLabel(product.PurchaseUnit)
		}
		if item.PurchaseUnit == "" {
			item.PurchaseUnit = NormalizeInventoryUnitLabel(product.Satuan)
		}
	}

	if item.Total <= 0 {
		item.Total = float64(item.Quantity) * item.PurchasePrice
	}

	item.StockQuantity = item.Quantity * item.PurchaseConversionQty
	if item.StockQuantity > 0 {
		item.UnitCostInStockUnit = item.Total / float64(item.StockQuantity)
	} else {
		item.UnitCostInStockUnit = 0
	}
}

func CalculateFinishedGoodRecipeCost(tx *gorm.DB, outletID uint, productID uint) (float64, error) {
	recipesByProduct, err := LoadActiveProductRecipes(tx, outletID, []uint{productID})
	if err != nil {
		return 0, err
	}

	recipes := recipesByProduct[productID]
	if len(recipes) == 0 {
		return 0, nil
	}

	totalCost := 0.0
	for _, recipe := range recipes {
		unitCost, err := ResolveInventoryUnitCost(tx, outletID, recipe.IngredientProductID, nil)
		if err != nil {
			return 0, err
		}
		totalCost += unitCost * float64(recipe.QuantityRequired)
	}

	return totalCost, nil
}

func SyncFinishedGoodRecipeCost(tx *gorm.DB, outletID uint, productID uint) (float64, error) {
	products, err := GetProductsForInventory(tx, outletID, []uint{productID})
	if err != nil {
		return 0, err
	}

	product := products[productID]
	if NormalizeProductItemType(product.ItemType) != ProductItemTypeFinishedGood {
		return product.LastPurchasePrice, nil
	}

	totalCost, err := CalculateFinishedGoodRecipeCost(tx, outletID, productID)
	if err != nil {
		return 0, err
	}

	if err := tx.Model(&models.Product{}).
		Where("outlet_id = ? AND id = ?", outletID, productID).
		Update("last_purchase_price", totalCost).Error; err != nil {
		return 0, err
	}

	product.LastPurchasePrice = totalCost
	return totalCost, nil
}

func SyncFinishedGoodRecipeCostsForIngredients(tx *gorm.DB, outletID uint, ingredientProductIDs []uint) error {
	ingredientProductIDs = uniqueSortedProductIDs(ingredientProductIDs)
	if len(ingredientProductIDs) == 0 {
		return nil
	}

	type dependencyRow struct {
		ProductID uint
	}

	var dependencies []dependencyRow
	if err := tx.Model(&models.ProductRecipe{}).
		Select("DISTINCT product_id").
		Where("outlet_id = ? AND ingredient_product_id IN ?", outletID, ingredientProductIDs).
		Order("product_id ASC").
		Scan(&dependencies).Error; err != nil {
		return err
	}

	for _, dependency := range dependencies {
		if dependency.ProductID == 0 {
			continue
		}
		if _, err := SyncFinishedGoodRecipeCost(tx, outletID, dependency.ProductID); err != nil {
			return err
		}
	}

	return nil
}
