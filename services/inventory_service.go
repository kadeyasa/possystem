package services

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/kadeyasa/possystem/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ProductItemTypeRawMaterial  = "raw_material"
	ProductItemTypeResaleItem   = "resale_item"
	ProductItemTypeFinishedGood = "finished_good"
	ProductItemTypeService      = "service"

	StockDocumentStatusPosted = "posted"

	InventoryMovementPurchase      = "purchase"
	InventoryMovementSale          = "sale"
	InventoryMovementRefund        = "refund"
	InventoryMovementRecipeConsume = "recipe_consumption"
	InventoryMovementAdjustmentIn  = "adjustment_in"
	InventoryMovementAdjustmentOut = "adjustment_out"
	InventoryMovementOpnameGain    = "opname_gain"
	InventoryMovementOpnameLoss    = "opname_loss"

	InventoryReferencePurchase   = "purchase"
	InventoryReferenceSale       = "sale"
	InventoryReferenceRefund     = "refund"
	InventoryReferenceAdjustment = "stock_adjustment"
	InventoryReferenceOpname     = "stock_opname"

	InventoryConsumptionTypeDirectSale      = "direct_sale"
	InventoryConsumptionTypeRecipeComponent = "recipe_component"
)

var (
	ensureInventorySchemaOnce sync.Once
	ensureInventorySchemaErr  error
)

func EnsureInventorySchema(db *gorm.DB) error {
	ensureInventorySchemaOnce.Do(func() {
		statements := []string{
			`ALTER TABLE tblproducts ADD COLUMN IF NOT EXISTS item_type VARCHAR(32)`,
			`ALTER TABLE tblproducts ADD COLUMN IF NOT EXISTS purchase_unit VARCHAR(32)`,
			`ALTER TABLE tblproducts ADD COLUMN IF NOT EXISTS purchase_conversion_qty INTEGER`,
			`UPDATE tblproducts SET item_type = 'resale_item' WHERE COALESCE(BTRIM(item_type), '') = ''`,
			`UPDATE tblproducts SET purchase_conversion_qty = 1 WHERE COALESCE(purchase_conversion_qty, 0) <= 0`,
			`UPDATE tblproducts SET purchase_unit = satuan WHERE COALESCE(BTRIM(purchase_unit), '') = '' AND COALESCE(BTRIM(satuan), '') <> ''`,
			`UPDATE tblproducts SET code = BTRIM(code) WHERE code <> BTRIM(code)`,
			`DO $$
			DECLARE constraint_name text;
			BEGIN
				FOR constraint_name IN
					SELECT conname
					FROM pg_constraint
					WHERE conrelid = 'public.tblproducts'::regclass
						AND contype = 'u'
						AND pg_get_constraintdef(oid) ILIKE '%(code)%'
						AND pg_get_constraintdef(oid) NOT ILIKE '%outlet_id%'
				LOOP
					EXECUTE format('ALTER TABLE public.tblproducts DROP CONSTRAINT %I', constraint_name);
				END LOOP;
			END $$`,
			`DO $$
			DECLARE idx record;
			BEGIN
				FOR idx IN
					SELECT indexname
					FROM pg_indexes
					WHERE schemaname = 'public'
						AND tablename = 'tblproducts'
						AND indexdef ILIKE 'CREATE UNIQUE INDEX%'
						AND indexdef ILIKE '%(code)%'
						AND indexdef NOT ILIKE '%outlet_id%'
				LOOP
					EXECUTE format('DROP INDEX IF EXISTS public.%I', idx.indexname);
				END LOOP;
			END $$`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_tblproducts_outlet_code_active_unique ON tblproducts (outlet_id, code) WHERE deleted_at IS NULL`,
			`CREATE INDEX IF NOT EXISTS idx_tblproducts_outlet_code_lookup ON tblproducts (outlet_id, code) WHERE deleted_at IS NULL`,
			`CREATE TABLE IF NOT EXISTS public.tblinventory_ledger (
					id BIGSERIAL PRIMARY KEY,
					outlet_id BIGINT NOT NULL,
				product_id BIGINT NOT NULL,
				variant_id BIGINT,
				movement_type VARCHAR(32) NOT NULL,
				reference_type VARCHAR(32) NOT NULL,
				reference_id BIGINT NOT NULL,
				quantity_in INTEGER NOT NULL DEFAULT 0,
				quantity_out INTEGER NOT NULL DEFAULT 0,
				stock_before INTEGER NOT NULL DEFAULT 0,
				stock_after INTEGER NOT NULL DEFAULT 0,
				unit_cost NUMERIC NOT NULL DEFAULT 0,
				total_cost NUMERIC NOT NULL DEFAULT 0,
				notes VARCHAR(200),
				created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS idx_tblinventory_ledger_outlet_product_date ON tblinventory_ledger (outlet_id, product_id, created_at DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_tblinventory_ledger_reference ON tblinventory_ledger (reference_type, reference_id)`,
			`CREATE INDEX IF NOT EXISTS idx_tblinventory_ledger_movement ON tblinventory_ledger (movement_type, created_at DESC)`,
			`CREATE TABLE IF NOT EXISTS public.tblproduct_recipes (
				id BIGSERIAL PRIMARY KEY,
				outlet_id BIGINT NOT NULL,
				product_id BIGINT NOT NULL,
				ingredient_product_id BIGINT NOT NULL,
				quantity_required INTEGER NOT NULL,
				note VARCHAR(200),
				created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
				updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
				deleted_at TIMESTAMP WITHOUT TIME ZONE
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_tblproduct_recipes_active_unique ON tblproduct_recipes (outlet_id, product_id, ingredient_product_id) WHERE deleted_at IS NULL`,
			`CREATE INDEX IF NOT EXISTS idx_tblproduct_recipes_product ON tblproduct_recipes (outlet_id, product_id) WHERE deleted_at IS NULL`,
			`CREATE INDEX IF NOT EXISTS idx_tblproduct_recipes_ingredient ON tblproduct_recipes (outlet_id, ingredient_product_id) WHERE deleted_at IS NULL`,
			`ALTER TABLE tblpurchase_items ADD COLUMN IF NOT EXISTS purchase_unit VARCHAR(32)`,
			`ALTER TABLE tblpurchase_items ADD COLUMN IF NOT EXISTS purchase_conversion_qty INTEGER`,
			`ALTER TABLE tblpurchase_items ADD COLUMN IF NOT EXISTS stock_quantity INTEGER`,
			`ALTER TABLE tblpurchase_items ADD COLUMN IF NOT EXISTS unit_cost_in_stock_unit NUMERIC`,
			`UPDATE tblpurchase_items SET purchase_conversion_qty = 1 WHERE COALESCE(purchase_conversion_qty, 0) <= 0`,
			`UPDATE tblpurchase_items SET stock_quantity = quantity * COALESCE(NULLIF(purchase_conversion_qty, 0), 1) WHERE COALESCE(stock_quantity, 0) <= 0 AND COALESCE(quantity, 0) > 0`,
			`UPDATE tblpurchase_items SET unit_cost_in_stock_unit = CASE WHEN COALESCE(stock_quantity, 0) > 0 THEN COALESCE(total, purchase_price * quantity, 0) / stock_quantity ELSE COALESCE(purchase_price, 0) END WHERE COALESCE(unit_cost_in_stock_unit, 0) <= 0`,
			`UPDATE tblpurchase_items SET purchase_unit = (
				SELECT COALESCE(NULLIF(BTRIM(tblproducts.purchase_unit), ''), NULLIF(BTRIM(tblproducts.satuan), ''), '')
				FROM tblproducts
				WHERE tblproducts.id = tblpurchase_items.product_id
			) WHERE COALESCE(BTRIM(purchase_unit), '') = ''`,
			`CREATE TABLE IF NOT EXISTS public.tbltransaction_inventory_consumptions (
				id BIGSERIAL PRIMARY KEY,
				transaction_id BIGINT NOT NULL,
				sold_product_id BIGINT NOT NULL,
				inventory_product_id BIGINT NOT NULL,
				variant_id BIGINT,
				consumption_type VARCHAR(32) NOT NULL,
				sold_quantity INTEGER NOT NULL DEFAULT 0,
				quantity_per_unit INTEGER NOT NULL DEFAULT 0,
				quantity_consumed INTEGER NOT NULL DEFAULT 0,
				unit_cost NUMERIC NOT NULL DEFAULT 0,
				total_cost NUMERIC NOT NULL DEFAULT 0,
				note VARCHAR(200),
				created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS idx_tbltransaction_inventory_consumptions_transaction ON tbltransaction_inventory_consumptions (transaction_id, sold_product_id)`,
			`CREATE INDEX IF NOT EXISTS idx_tbltransaction_inventory_consumptions_inventory_product ON tbltransaction_inventory_consumptions (inventory_product_id, created_at DESC)`,
			`CREATE TABLE IF NOT EXISTS public.tblstock_adjustments (
				id BIGSERIAL PRIMARY KEY,
				outlet_id BIGINT NOT NULL,
				adjustment_date TIMESTAMP WITHOUT TIME ZONE NOT NULL,
				reason VARCHAR(64),
				status VARCHAR(16) NOT NULL DEFAULT 'posted',
				journal_entry_id BIGINT,
				accounting_sync_status VARCHAR(16),
				accounting_sync_error TEXT,
				accounting_synced_at TIMESTAMP WITHOUT TIME ZONE,
				accounting_idempotency_key TEXT,
				note VARCHAR(200),
				created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
				updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
				deleted_at TIMESTAMP WITHOUT TIME ZONE
			)`,
			`CREATE INDEX IF NOT EXISTS idx_tblstock_adjustments_outlet_date ON tblstock_adjustments (outlet_id, adjustment_date DESC)`,
			`CREATE TABLE IF NOT EXISTS public.tblstock_adjustment_items (
				id BIGSERIAL PRIMARY KEY,
				adjustment_id BIGINT NOT NULL,
				product_id BIGINT NOT NULL,
				quantity_delta INTEGER NOT NULL,
				stock_before INTEGER NOT NULL DEFAULT 0,
				stock_after INTEGER NOT NULL DEFAULT 0,
				unit_cost NUMERIC NOT NULL DEFAULT 0,
				total_cost NUMERIC NOT NULL DEFAULT 0,
				note VARCHAR(200),
				created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
				updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS idx_tblstock_adjustment_items_adjustment_id ON tblstock_adjustment_items (adjustment_id)`,
			`CREATE TABLE IF NOT EXISTS public.tblstock_opnames (
				id BIGSERIAL PRIMARY KEY,
				outlet_id BIGINT NOT NULL,
				opname_date TIMESTAMP WITHOUT TIME ZONE NOT NULL,
				status VARCHAR(16) NOT NULL DEFAULT 'posted',
				journal_entry_id BIGINT,
				accounting_sync_status VARCHAR(16),
				accounting_sync_error TEXT,
				accounting_synced_at TIMESTAMP WITHOUT TIME ZONE,
				accounting_idempotency_key TEXT,
				note VARCHAR(200),
				created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
				updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
				deleted_at TIMESTAMP WITHOUT TIME ZONE
			)`,
			`CREATE INDEX IF NOT EXISTS idx_tblstock_opnames_outlet_date ON tblstock_opnames (outlet_id, opname_date DESC)`,
			`CREATE TABLE IF NOT EXISTS public.tblstock_opname_items (
				id BIGSERIAL PRIMARY KEY,
				opname_id BIGINT NOT NULL,
				product_id BIGINT NOT NULL,
				system_stock INTEGER NOT NULL DEFAULT 0,
				actual_stock INTEGER NOT NULL DEFAULT 0,
				difference_qty INTEGER NOT NULL DEFAULT 0,
				unit_cost NUMERIC NOT NULL DEFAULT 0,
				total_cost NUMERIC NOT NULL DEFAULT 0,
				note VARCHAR(200),
				created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
				updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS idx_tblstock_opname_items_opname_id ON tblstock_opname_items (opname_id)`,
		}

		for _, statement := range statements {
			if err := db.Exec(statement).Error; err != nil {
				ensureInventorySchemaErr = fmt.Errorf("failed to ensure inventory schema: %w", err)
				return
			}
		}
	})

	return ensureInventorySchemaErr
}

type PlannedInventoryConsumption struct {
	SoldProductID      uint
	InventoryProductID uint
	VariantID          *int64
	ConsumptionType    string
	SoldQuantity       int
	QuantityPerUnit    int
	QuantityConsumed   int
	UnitCost           float64
	TotalCost          float64
	Note               string
}

type SaleInventoryPlan struct {
	LockedProducts map[uint]*models.Product
	Consumptions   []PlannedInventoryConsumption
	TotalCost      float64
}

type RefundInventoryPlan struct {
	LockedProducts map[uint]*models.Product
	Restorations   []PlannedInventoryConsumption
	TotalCost      float64
}

func NormalizeProductItemType(value string) string {
	switch normalizeAccountToken(value) {
	case ProductItemTypeRawMaterial:
		return ProductItemTypeRawMaterial
	case ProductItemTypeResaleItem:
		return ProductItemTypeResaleItem
	case ProductItemTypeFinishedGood:
		return ProductItemTypeFinishedGood
	case ProductItemTypeService:
		return ProductItemTypeService
	case "":
		return ProductItemTypeResaleItem
	default:
		return ""
	}
}

func IsStockTrackedItemType(itemType string) bool {
	switch NormalizeProductItemType(itemType) {
	case ProductItemTypeRawMaterial, ProductItemTypeResaleItem, ProductItemTypeFinishedGood:
		return true
	default:
		return false
	}
}

func ResolveInventoryUnitCost(tx *gorm.DB, outletID uint, productID uint, variantID *int64) (float64, error) {
	return resolveUnitCost(tx, outletID, productID, variantID)
}

func uniqueSortedProductIDs(productIDs []uint) []uint {
	seen := make(map[uint]struct{}, len(productIDs))
	unique := make([]uint, 0, len(productIDs))
	for _, productID := range productIDs {
		if productID == 0 {
			continue
		}
		if _, ok := seen[productID]; ok {
			continue
		}
		seen[productID] = struct{}{}
		unique = append(unique, productID)
	}
	sort.Slice(unique, func(i, j int) bool { return unique[i] < unique[j] })
	return unique
}

func loadProductsForInventory(tx *gorm.DB, outletID uint, productIDs []uint, lock bool) (map[uint]*models.Product, error) {
	productIDs = uniqueSortedProductIDs(productIDs)
	if len(productIDs) == 0 {
		return nil, fmt.Errorf("at least one product is required")
	}

	query := tx.Where("outlet_id = ? AND id IN ?", outletID, productIDs).Order("id ASC")
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	var products []models.Product
	if err := query.Find(&products).Error; err != nil {
		return nil, err
	}

	if len(products) != len(productIDs) {
		found := make(map[uint]struct{}, len(products))
		for i := range products {
			found[products[i].ID] = struct{}{}
		}
		missing := make([]uint, 0)
		for _, productID := range productIDs {
			if _, ok := found[productID]; !ok {
				missing = append(missing, productID)
			}
		}
		sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })
		return nil, fmt.Errorf("product not found for outlet %d: %v", outletID, missing)
	}

	byID := make(map[uint]*models.Product, len(products))
	for i := range products {
		byID[products[i].ID] = &products[i]
	}

	return byID, nil
}

func GetProductsForInventory(tx *gorm.DB, outletID uint, productIDs []uint) (map[uint]*models.Product, error) {
	return loadProductsForInventory(tx, outletID, productIDs, false)
}

func LockProductsForInventory(tx *gorm.DB, outletID uint, productIDs []uint) (map[uint]*models.Product, error) {
	return loadProductsForInventory(tx, outletID, productIDs, true)
}

func LoadActiveProductRecipes(tx *gorm.DB, outletID uint, productIDs []uint) (map[uint][]models.ProductRecipe, error) {
	productIDs = uniqueSortedProductIDs(productIDs)
	if len(productIDs) == 0 {
		return map[uint][]models.ProductRecipe{}, nil
	}

	var recipes []models.ProductRecipe
	if err := tx.Where("outlet_id = ? AND product_id IN ?", outletID, productIDs).
		Order("product_id ASC, ingredient_product_id ASC").
		Find(&recipes).Error; err != nil {
		return nil, err
	}

	grouped := make(map[uint][]models.ProductRecipe, len(productIDs))
	for _, recipe := range recipes {
		grouped[recipe.ProductID] = append(grouped[recipe.ProductID], recipe)
	}

	return grouped, nil
}

func BuildTransactionItemQuantityMap(items []models.TransactionItem) (map[uint]int, error) {
	quantities := make(map[uint]int)
	for _, item := range items {
		if item.ProductID == 0 {
			return nil, fmt.Errorf("product_id is required for every transaction item")
		}
		if item.Quantity <= 0 {
			return nil, fmt.Errorf("quantity must be greater than zero for product %d", item.ProductID)
		}
		quantities[item.ProductID] += item.Quantity
	}
	if len(quantities) == 0 {
		return nil, fmt.Errorf("at least one transaction item is required")
	}
	return quantities, nil
}

func BuildRefundItemQuantityMap(items []models.RefundItem) (map[uint]int, error) {
	quantities := make(map[uint]int)
	for _, item := range items {
		if item.ProductID == 0 {
			return nil, fmt.Errorf("product_id is required for every refund item")
		}
		if item.Quantity <= 0 {
			return nil, fmt.Errorf("quantity must be greater than zero for product %d", item.ProductID)
		}
		quantities[item.ProductID] += item.Quantity
	}
	if len(quantities) == 0 {
		return nil, fmt.Errorf("at least one refund item is required")
	}
	return quantities, nil
}

func BuildPurchaseItemQuantityMap(items []models.PurchaseItem) (map[uint]int, error) {
	quantities := make(map[uint]int)
	for _, item := range items {
		if item.ProductID == 0 {
			return nil, fmt.Errorf("product_id is required for every purchase item")
		}
		if item.Quantity <= 0 {
			return nil, fmt.Errorf("quantity must be greater than zero for product %d", item.ProductID)
		}
		quantities[item.ProductID] += item.Quantity
	}
	if len(quantities) == 0 {
		return nil, fmt.Errorf("at least one purchase item is required")
	}
	return quantities, nil
}

func PrepareSaleInventoryPlan(tx *gorm.DB, outletID uint, items []models.TransactionItem) (*SaleInventoryPlan, error) {
	requestedQty, err := BuildTransactionItemQuantityMap(items)
	if err != nil {
		return nil, err
	}

	soldProductIDs := make([]uint, 0, len(requestedQty))
	for productID := range requestedQty {
		soldProductIDs = append(soldProductIDs, productID)
	}
	soldProductIDs = uniqueSortedProductIDs(soldProductIDs)

	soldProducts, err := GetProductsForInventory(tx, outletID, soldProductIDs)
	if err != nil {
		return nil, err
	}

	recipeProductIDs := make([]uint, 0)
	involvedProductIDs := append([]uint(nil), soldProductIDs...)
	for _, productID := range soldProductIDs {
		if NormalizeProductItemType(soldProducts[productID].ItemType) == ProductItemTypeFinishedGood {
			recipeProductIDs = append(recipeProductIDs, productID)
		}
	}

	recipesByProduct, err := LoadActiveProductRecipes(tx, outletID, recipeProductIDs)
	if err != nil {
		return nil, err
	}
	for _, recipes := range recipesByProduct {
		for _, recipe := range recipes {
			involvedProductIDs = append(involvedProductIDs, recipe.IngredientProductID)
		}
	}

	lockedProducts, err := LockProductsForInventory(tx, outletID, involvedProductIDs)
	if err != nil {
		return nil, err
	}

	requiredByInventoryProduct := make(map[uint]int)
	movements := make([]PlannedInventoryConsumption, 0, len(items))
	totalCost := 0.0

	for _, item := range items {
		product := lockedProducts[item.ProductID]
		itemType := NormalizeProductItemType(product.ItemType)
		if !IsStockTrackedItemType(itemType) {
			continue
		}

		recipes := recipesByProduct[item.ProductID]
		if itemType == ProductItemTypeFinishedGood {
			if len(recipes) == 0 {
				return nil, fmt.Errorf("finished_good %s (%d) belum punya recipe/BOM aktif", strings.TrimSpace(product.Name), product.ID)
			}

			for _, recipe := range recipes {
				if recipe.QuantityRequired <= 0 {
					return nil, fmt.Errorf("recipe quantity must be greater than zero for product %d", item.ProductID)
				}

				ingredient := lockedProducts[recipe.IngredientProductID]
				if !IsStockTrackedItemType(ingredient.ItemType) {
					return nil, fmt.Errorf("recipe ingredient %s (%d) is not stock tracked", strings.TrimSpace(ingredient.Name), ingredient.ID)
				}

				quantityConsumed := recipe.QuantityRequired * item.Quantity
				unitCost, err := ResolveInventoryUnitCost(tx, outletID, recipe.IngredientProductID, nil)
				if err != nil {
					return nil, err
				}

				requiredByInventoryProduct[recipe.IngredientProductID] += quantityConsumed
				totalCost += unitCost * float64(quantityConsumed)
				movements = append(movements, PlannedInventoryConsumption{
					SoldProductID:      item.ProductID,
					InventoryProductID: recipe.IngredientProductID,
					ConsumptionType:    InventoryConsumptionTypeRecipeComponent,
					SoldQuantity:       item.Quantity,
					QuantityPerUnit:    recipe.QuantityRequired,
					QuantityConsumed:   quantityConsumed,
					UnitCost:           unitCost,
					TotalCost:          unitCost * float64(quantityConsumed),
					Note:               strings.TrimSpace(recipe.Note),
				})
			}
			continue
		}

		unitCost, err := ResolveInventoryUnitCost(tx, outletID, item.ProductID, item.VariantID)
		if err != nil {
			return nil, err
		}

		requiredByInventoryProduct[item.ProductID] += item.Quantity
		totalCost += unitCost * float64(item.Quantity)
		movements = append(movements, PlannedInventoryConsumption{
			SoldProductID:      item.ProductID,
			InventoryProductID: item.ProductID,
			VariantID:          item.VariantID,
			ConsumptionType:    InventoryConsumptionTypeDirectSale,
			SoldQuantity:       item.Quantity,
			QuantityPerUnit:    1,
			QuantityConsumed:   item.Quantity,
			UnitCost:           unitCost,
			TotalCost:          unitCost * float64(item.Quantity),
			Note:               strings.TrimSpace(product.Name),
		})
	}

	for productID, quantity := range requiredByInventoryProduct {
		product := lockedProducts[productID]
		if product.Stock < quantity {
			return nil, fmt.Errorf("insufficient stock for product %s (%d). available=%d requested=%d", strings.TrimSpace(product.Name), product.ID, product.Stock, quantity)
		}
	}

	return &SaleInventoryPlan{
		LockedProducts: lockedProducts,
		Consumptions:   movements,
		TotalCost:      totalCost,
	}, nil
}

func ValidateSaleStock(tx *gorm.DB, outletID uint, items []models.TransactionItem) (map[uint]*models.Product, error) {
	plan, err := PrepareSaleInventoryPlan(tx, outletID, items)
	if err != nil {
		return nil, err
	}
	return plan.LockedProducts, nil
}

func validateRefundableQuantities(tx *gorm.DB, transactionID uint, requestedQty map[uint]int) error {
	type qtyRow struct {
		ProductID uint
		Quantity  int
	}

	var soldRows []qtyRow
	if err := tx.Table("tbltransaction_items").
		Select("product_id, COALESCE(SUM(quantity), 0) AS quantity").
		Where("transaction_id = ?", transactionID).
		Group("product_id").
		Scan(&soldRows).Error; err != nil {
		return err
	}

	soldByProduct := make(map[uint]int, len(soldRows))
	for _, row := range soldRows {
		soldByProduct[row.ProductID] = row.Quantity
	}

	var refundedRows []qtyRow
	if err := tx.Table("tblrefund_items ri").
		Select("ri.product_id, COALESCE(SUM(ri.quantity), 0) AS quantity").
		Joins("JOIN tblrefunds r ON r.id = ri.refund_id").
		Where("r.transaction_id = ?", transactionID).
		Group("ri.product_id").
		Scan(&refundedRows).Error; err != nil {
		return err
	}

	refundedByProduct := make(map[uint]int, len(refundedRows))
	for _, row := range refundedRows {
		refundedByProduct[row.ProductID] = row.Quantity
	}

	for productID, requested := range requestedQty {
		soldQty := soldByProduct[productID]
		if soldQty <= 0 {
			return fmt.Errorf("product %d is not part of the original transaction", productID)
		}
		remainingRefundable := soldQty - refundedByProduct[productID]
		if remainingRefundable <= 0 {
			return fmt.Errorf("product %d has no refundable quantity left", productID)
		}
		if requested > remainingRefundable {
			return fmt.Errorf("refund quantity exceeds sold quantity for product %d. refundable=%d requested=%d", productID, remainingRefundable, requested)
		}
	}

	return nil
}

func PrepareRefundInventoryPlan(tx *gorm.DB, transactionID uint, outletID uint, items []models.RefundItem) (*RefundInventoryPlan, error) {
	requestedQty, err := BuildRefundItemQuantityMap(items)
	if err != nil {
		return nil, err
	}

	if err := validateRefundableQuantities(tx, transactionID, requestedQty); err != nil {
		return nil, err
	}

	productIDs := make([]uint, 0, len(requestedQty))
	for productID := range requestedQty {
		productIDs = append(productIDs, productID)
	}
	productIDs = uniqueSortedProductIDs(productIDs)

	var recordedConsumptions []models.TransactionInventoryConsumption
	if err := tx.Where("transaction_id = ? AND sold_product_id IN ?", transactionID, productIDs).
		Order("sold_product_id ASC, inventory_product_id ASC, id ASC").
		Find(&recordedConsumptions).Error; err != nil {
		return nil, err
	}

	consumptionsBySoldProduct := make(map[uint][]models.TransactionInventoryConsumption)
	involvedProductIDs := append([]uint(nil), productIDs...)
	for _, consumption := range recordedConsumptions {
		consumptionsBySoldProduct[consumption.SoldProductID] = append(consumptionsBySoldProduct[consumption.SoldProductID], consumption)
		involvedProductIDs = append(involvedProductIDs, consumption.InventoryProductID)
	}

	lockedProducts, err := LockProductsForInventory(tx, outletID, involvedProductIDs)
	if err != nil {
		return nil, err
	}

	restorations := make([]PlannedInventoryConsumption, 0, len(items))
	totalCost := 0.0
	for _, item := range items {
		consumptions := consumptionsBySoldProduct[item.ProductID]
		if len(consumptions) > 0 {
			for _, consumption := range consumptions {
				quantityPerUnit := consumption.QuantityPerUnit
				if quantityPerUnit <= 0 {
					if consumption.SoldQuantity <= 0 || consumption.QuantityConsumed <= 0 || consumption.QuantityConsumed%consumption.SoldQuantity != 0 {
						return nil, fmt.Errorf("invalid recorded inventory consumption for transaction %d product %d", transactionID, item.ProductID)
					}
					quantityPerUnit = consumption.QuantityConsumed / consumption.SoldQuantity
				}
				quantityRestored := quantityPerUnit * item.Quantity
				if quantityRestored <= 0 {
					continue
				}
				totalCost += consumption.UnitCost * float64(quantityRestored)
				restorations = append(restorations, PlannedInventoryConsumption{
					SoldProductID:      item.ProductID,
					InventoryProductID: consumption.InventoryProductID,
					VariantID:          consumption.VariantID,
					ConsumptionType:    consumption.ConsumptionType,
					SoldQuantity:       item.Quantity,
					QuantityPerUnit:    quantityPerUnit,
					QuantityConsumed:   quantityRestored,
					UnitCost:           consumption.UnitCost,
					TotalCost:          consumption.UnitCost * float64(quantityRestored),
					Note:               strings.TrimSpace(consumption.Note),
				})
			}
			continue
		}

		product := lockedProducts[item.ProductID]
		if !IsStockTrackedItemType(product.ItemType) {
			continue
		}
		unitCost, err := ResolveInventoryUnitCost(tx, outletID, item.ProductID, nil)
		if err != nil {
			return nil, err
		}
		totalCost += unitCost * float64(item.Quantity)
		restorations = append(restorations, PlannedInventoryConsumption{
			SoldProductID:      item.ProductID,
			InventoryProductID: item.ProductID,
			ConsumptionType:    InventoryConsumptionTypeDirectSale,
			SoldQuantity:       item.Quantity,
			QuantityPerUnit:    1,
			QuantityConsumed:   item.Quantity,
			UnitCost:           unitCost,
			TotalCost:          unitCost * float64(item.Quantity),
			Note:               strings.TrimSpace(product.Name),
		})
	}

	return &RefundInventoryPlan{
		LockedProducts: lockedProducts,
		Restorations:   restorations,
		TotalCost:      totalCost,
	}, nil
}

func ValidateRefundStock(tx *gorm.DB, transactionID uint, outletID uint, items []models.RefundItem) (map[uint]*models.Product, error) {
	plan, err := PrepareRefundInventoryPlan(tx, transactionID, outletID, items)
	if err != nil {
		return nil, err
	}
	return plan.LockedProducts, nil
}

func ValidatePurchaseProducts(tx *gorm.DB, outletID uint, items []models.PurchaseItem) (map[uint]*models.Product, error) {
	requestedQty, err := BuildPurchaseItemQuantityMap(items)
	if err != nil {
		return nil, err
	}

	productIDs := make([]uint, 0, len(requestedQty))
	for productID := range requestedQty {
		productIDs = append(productIDs, productID)
	}
	sort.Slice(productIDs, func(i, j int) bool { return productIDs[i] < productIDs[j] })

	products, err := LockProductsForInventory(tx, outletID, productIDs)
	if err != nil {
		return nil, err
	}

	for _, product := range products {
		if !IsStockTrackedItemType(product.ItemType) {
			return nil, fmt.Errorf("product %s (%d) is not stock tracked and cannot be used in inventory purchase", strings.TrimSpace(product.Name), product.ID)
		}
	}

	return products, nil
}

func RecordTransactionInventoryConsumptions(tx *gorm.DB, transactionID uint, consumptions []PlannedInventoryConsumption) error {
	type aggregateKey struct {
		soldProductID      uint
		inventoryProductID uint
		consumptionType    string
	}

	grouped := make(map[aggregateKey]*models.TransactionInventoryConsumption)
	order := make([]aggregateKey, 0)
	for _, consumption := range consumptions {
		key := aggregateKey{
			soldProductID:      consumption.SoldProductID,
			inventoryProductID: consumption.InventoryProductID,
			consumptionType:    consumption.ConsumptionType,
		}
		record, ok := grouped[key]
		if !ok {
			record = &models.TransactionInventoryConsumption{
				TransactionID:      transactionID,
				SoldProductID:      consumption.SoldProductID,
				InventoryProductID: consumption.InventoryProductID,
				ConsumptionType:    consumption.ConsumptionType,
				SoldQuantity:       0,
				QuantityPerUnit:    0,
				QuantityConsumed:   0,
				UnitCost:           0,
				TotalCost:          0,
				Note:               consumption.Note,
			}
			grouped[key] = record
			order = append(order, key)
		}

		record.SoldQuantity += consumption.SoldQuantity
		record.QuantityConsumed += consumption.QuantityConsumed
		record.TotalCost += consumption.TotalCost
		if strings.TrimSpace(record.Note) == "" {
			record.Note = consumption.Note
		}
	}

	for _, key := range order {
		record := grouped[key]
		if record.QuantityConsumed > 0 {
			record.UnitCost = record.TotalCost / float64(record.QuantityConsumed)
		}
		if record.SoldQuantity > 0 && record.QuantityConsumed > 0 && record.QuantityConsumed%record.SoldQuantity == 0 {
			record.QuantityPerUnit = record.QuantityConsumed / record.SoldQuantity
		}
		record.VariantID = nil
		if err := tx.Create(record).Error; err != nil {
			return err
		}
	}

	return nil
}

func AppendInventoryLedger(tx *gorm.DB, entry models.InventoryLedger) error {
	return tx.Create(&entry).Error
}
