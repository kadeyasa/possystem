package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"gorm.io/gorm"
)

type accountSelector struct {
	TransactionType string
	Purpose         string
}

type accountCategoryExpectation struct {
	label      string
	categories map[string]struct{}
}

func BuildAccountMappingRecordQuery(tx *gorm.DB) *gorm.DB {
	return tx.
		Table("tblaccount_mappings AS mappings").
		Select(`
			mappings.id AS mapping_id,
			mappings.account_id AS id,
			COALESCE(NULLIF(accounts.name, ''), mappings.account_id) AS name,
			COALESCE(NULLIF(accounts.category, ''), '') AS category,
			mappings.is_active AS is_active,
			mappings.outlet_id AS outlet_id,
			mappings.transaction_type AS transaction_type,
			mappings.purpose AS purpose
		`).
		Joins("LEFT JOIN tblaccounts AS accounts ON accounts.id = mappings.account_id")
}

func normalizeAccountToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func makeAccountCategoryExpectation(label string, categories ...string) accountCategoryExpectation {
	allowed := make(map[string]struct{}, len(categories))
	for _, category := range categories {
		token := normalizeAccountToken(category)
		if token == "" {
			continue
		}
		allowed[token] = struct{}{}
	}

	return accountCategoryExpectation{
		label:      label,
		categories: allowed,
	}
}

func ValidateAccountCategoryForMapping(category, transactionType, purpose string) error {
	transactionType = normalizeAccountToken(transactionType)
	purpose = normalizeAccountToken(purpose)
	category = normalizeAccountToken(category)

	var expectation accountCategoryExpectation
	switch transactionType {
	case "sale":
		switch purpose {
		case "sales":
			expectation = makeAccountCategoryExpectation("revenue", "revenue", "pendapatan")
		case "cash", "qris", "transfer":
			expectation = makeAccountCategoryExpectation("asset", "asset", "aset")
		case "tax":
			expectation = makeAccountCategoryExpectation("liability", "liability", "utang")
		case "cogs":
			expectation = makeAccountCategoryExpectation("expense", "expense", "beban")
		case "discount":
			expectation = makeAccountCategoryExpectation("expense or contra revenue", "expense", "beban", "kontra-pendapatan")
		}
	case "fee":
		switch purpose {
		case "expense":
			expectation = makeAccountCategoryExpectation("expense", "expense", "beban")
		case "balance":
			expectation = makeAccountCategoryExpectation("liability", "liability", "utang")
		}
	case "purchase":
		switch purpose {
		case "inventory":
			expectation = makeAccountCategoryExpectation("asset", "asset", "aset")
		case "credit":
			expectation = makeAccountCategoryExpectation("liability", "liability", "utang")
		case "cash", "qris", "transfer":
			expectation = makeAccountCategoryExpectation("asset", "asset", "aset")
		}
	}

	if len(expectation.categories) == 0 || category == "" {
		return nil
	}
	if _, ok := expectation.categories[category]; ok {
		return nil
	}

	return fmt.Errorf(
		"account category %q is invalid for mapping %s:%s; expected %s",
		category,
		transactionType,
		purpose,
		expectation.label,
	)
}

func TryResolveAccountForOutlet(tx *gorm.DB, outletID uint, transactionType, purpose string) (models.Account, bool, error) {
	// Schema setup must run on the base connection, not inside the caller's
	// business transaction. Otherwise a rollback can undo the CREATE TABLE
	// while sync.Once still marks the schema as initialized.
	schemaDB := tx
	if database.DB != nil {
		schemaDB = database.DB
	}

	if err := EnsureAccountMappingSchema(schemaDB); err != nil {
		return models.Account{}, false, err
	}

	transactionType = normalizeAccountToken(transactionType)
	purpose = normalizeAccountToken(purpose)
	if transactionType == "" || purpose == "" {
		return models.Account{}, false, fmt.Errorf("transaction_type and purpose are required")
	}

	var account models.Account
	err := BuildAccountMappingRecordQuery(tx).
		Where("mappings.outlet_id = ? AND mappings.transaction_type = ? AND mappings.purpose = ? AND mappings.is_active = true",
			outletID, transactionType, purpose).
		Take(&account).Error
	if err == nil {
		return account, true, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Account{}, false, err
	}

	err = BuildAccountMappingRecordQuery(tx).
		Where("mappings.outlet_id = 0 AND mappings.transaction_type = ? AND mappings.purpose = ? AND mappings.is_active = true",
			transactionType, purpose).
		Take(&account).Error
	if err == nil {
		return account, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Account{}, false, nil
	}

	return models.Account{}, false, err
}

func ResolveAccountForOutlet(tx *gorm.DB, outletID uint, transactionType, purpose string) (models.Account, error) {
	account, found, err := TryResolveAccountForOutlet(tx, outletID, transactionType, purpose)
	if err != nil {
		return models.Account{}, err
	}
	if !found {
		return models.Account{}, fmt.Errorf("account mapping not found for outlet %d (%s:%s)", outletID, normalizeAccountToken(transactionType), normalizeAccountToken(purpose))
	}
	if err := ValidateAccountCategoryForMapping(account.Category, transactionType, purpose); err != nil {
		return models.Account{}, err
	}
	return account, nil
}

func ResolveSettlementAccountForOutlet(tx *gorm.DB, outletID uint, paymentMethod string) (models.Account, error) {
	purpose := normalizeAccountToken(paymentMethod)
	candidates := []accountSelector{
		{TransactionType: "payment", Purpose: purpose},
		{TransactionType: "settlement", Purpose: purpose},
		{TransactionType: "expense", Purpose: purpose},
		{TransactionType: "sale", Purpose: purpose},
		{TransactionType: "purchase", Purpose: purpose},
	}

	account, found, err := tryResolveAccountByCandidates(tx, outletID, candidates)
	if err != nil {
		return models.Account{}, err
	}
	if found {
		return account, nil
	}

	return models.Account{}, fmt.Errorf("settlement account mapping not found for outlet %d (%s)", outletID, purpose)
}

func ResolveSaleSettlementAccount(tx *gorm.DB, outletID uint, paymentMethod string) (models.Account, error) {
	account, err := ResolveSettlementAccountForOutlet(tx, outletID, paymentMethod)
	if err != nil {
		return models.Account{}, err
	}
	if err := ValidateAccountCategoryForMapping(account.Category, "sale", paymentMethod); err != nil {
		return models.Account{}, err
	}
	return account, nil
}

func tryResolveAccountByCandidates(tx *gorm.DB, outletID uint, candidates []accountSelector) (models.Account, bool, error) {
	for _, candidate := range candidates {
		account, found, err := TryResolveAccountForOutlet(tx, outletID, candidate.TransactionType, candidate.Purpose)
		if err != nil {
			return models.Account{}, false, err
		}
		if found {
			return account, true, nil
		}
	}

	return models.Account{}, false, nil
}

func ResolveOperationalExpenseAccount(tx *gorm.DB, outletID uint, purpose string) (models.Account, error) {
	purpose = normalizeAccountToken(purpose)
	if purpose == "" {
		purpose = "operational"
	}

	candidates := []accountSelector{
		{TransactionType: "expense", Purpose: purpose},
		{TransactionType: "operational_expense", Purpose: purpose},
		{TransactionType: "expense", Purpose: "operational"},
		{TransactionType: "expense", Purpose: "general"},
	}

	account, found, err := tryResolveAccountByCandidates(tx, outletID, candidates)
	if err != nil {
		return models.Account{}, err
	}
	if !found {
		return models.Account{}, fmt.Errorf("operational expense account mapping not found for outlet %d (%s)", outletID, purpose)
	}

	return account, nil
}

func ResolveVendorPayableAccount(tx *gorm.DB, outletID uint) (models.Account, error) {
	candidates := []accountSelector{
		{TransactionType: "payable", Purpose: "vendor"},
		{TransactionType: "payable", Purpose: "accounts_payable"},
		{TransactionType: "liability", Purpose: "accounts_payable"},
		{TransactionType: "purchase", Purpose: "credit"},
	}

	account, found, err := tryResolveAccountByCandidates(tx, outletID, candidates)
	if err != nil {
		return models.Account{}, err
	}
	if !found {
		return models.Account{}, fmt.Errorf("vendor payable account mapping not found for outlet %d", outletID)
	}

	return account, nil
}

func ResolveVendorBillPostingAccount(tx *gorm.DB, outletID uint, billType, accountPurpose string, usePrepaid bool) (models.Account, error) {
	billType = NormalizeVendorBillType(billType)
	accountPurpose = normalizeAccountToken(accountPurpose)

	var candidates []accountSelector
	switch billType {
	case VendorBillTypeInventory:
		candidates = []accountSelector{
			{TransactionType: "purchase", Purpose: "inventory"},
			{TransactionType: "inventory", Purpose: "inventory"},
		}
	case VendorBillTypeExpense:
		if accountPurpose == "" {
			accountPurpose = "operational"
		}
		candidates = []accountSelector{
			{TransactionType: "expense", Purpose: accountPurpose},
			{TransactionType: "operational_expense", Purpose: accountPurpose},
			{TransactionType: "expense", Purpose: "operational"},
			{TransactionType: "expense", Purpose: "general"},
		}
	case VendorBillTypeAsset:
		if accountPurpose == "" {
			accountPurpose = "fixed_asset"
		}
		candidates = []accountSelector{
			{TransactionType: "asset", Purpose: accountPurpose},
			{TransactionType: "asset", Purpose: "fixed_asset"},
		}
	case VendorBillTypeRent:
		if accountPurpose == "" {
			if usePrepaid {
				accountPurpose = "prepaid"
			} else {
				accountPurpose = "rent"
			}
		}
		candidates = []accountSelector{
			{TransactionType: "rent", Purpose: accountPurpose},
			{TransactionType: "expense", Purpose: accountPurpose},
		}
		if usePrepaid {
			candidates = append(candidates,
				accountSelector{TransactionType: "rent", Purpose: "prepaid"},
				accountSelector{TransactionType: "asset", Purpose: "prepaid_rent"},
			)
		} else {
			candidates = append(candidates,
				accountSelector{TransactionType: "rent", Purpose: "expense"},
				accountSelector{TransactionType: "expense", Purpose: "rent"},
			)
		}
	default:
		return models.Account{}, fmt.Errorf("unsupported vendor bill type: %s", billType)
	}

	account, found, err := tryResolveAccountByCandidates(tx, outletID, candidates)
	if err != nil {
		return models.Account{}, err
	}
	if !found {
		return models.Account{}, fmt.Errorf("posting account mapping not found for outlet %d (%s:%s)", outletID, billType, accountPurpose)
	}

	return account, nil
}

func ResolveInventoryAssetAccount(tx *gorm.DB, outletID uint) (models.Account, error) {
	candidates := []accountSelector{
		{TransactionType: "purchase", Purpose: "inventory"},
		{TransactionType: "inventory", Purpose: "inventory"},
		{TransactionType: "inventory", Purpose: "asset"},
	}

	account, found, err := tryResolveAccountByCandidates(tx, outletID, candidates)
	if err != nil {
		return models.Account{}, err
	}
	if !found {
		return models.Account{}, fmt.Errorf("inventory asset account mapping not found for outlet %d", outletID)
	}

	return account, nil
}

func ResolveStockAdjustmentLossAccount(tx *gorm.DB, outletID uint) (models.Account, error) {
	candidates := []accountSelector{
		{TransactionType: "inventory", Purpose: "adjustment_loss"},
		{TransactionType: "expense", Purpose: "inventory_adjustment"},
		{TransactionType: "expense", Purpose: "stock_loss"},
		{TransactionType: "expense", Purpose: "operational"},
	}

	account, found, err := tryResolveAccountByCandidates(tx, outletID, candidates)
	if err != nil {
		return models.Account{}, err
	}
	if !found {
		return models.Account{}, fmt.Errorf("stock adjustment loss account mapping not found for outlet %d", outletID)
	}

	return account, nil
}

func ResolveStockAdjustmentGainAccount(tx *gorm.DB, outletID uint) (models.Account, error) {
	candidates := []accountSelector{
		{TransactionType: "inventory", Purpose: "adjustment_gain"},
		{TransactionType: "income", Purpose: "inventory_adjustment"},
		{TransactionType: "income", Purpose: "stock_gain"},
		{TransactionType: "other_income", Purpose: "inventory_adjustment"},
	}

	account, found, err := tryResolveAccountByCandidates(tx, outletID, candidates)
	if err != nil {
		return models.Account{}, err
	}
	if !found {
		return models.Account{}, fmt.Errorf("stock adjustment gain account mapping not found for outlet %d", outletID)
	}

	return account, nil
}

func TryResolveOutletFeeAccounts(tx *gorm.DB, outletID uint) (models.Account, models.Account, bool, error) {
	expenseCandidates := []accountSelector{
		{TransactionType: "fee", Purpose: "expense"},
		{TransactionType: "fee", Purpose: "outlet_fee"},
		{TransactionType: "sale", Purpose: "outlet_fee"},
		{TransactionType: "sale", Purpose: "fee_expense"},
	}
	balanceCandidates := []accountSelector{
		{TransactionType: "fee", Purpose: "balance"},
		{TransactionType: "fee", Purpose: "outlet_balance"},
		{TransactionType: "balance", Purpose: "outlet"},
		{TransactionType: "balance", Purpose: "liability"},
		{TransactionType: "deposit", Purpose: "balance"},
	}

	expenseAccount, found, err := tryResolveAccountByCandidates(tx, outletID, expenseCandidates)
	if err != nil {
		return models.Account{}, models.Account{}, false, err
	}
	if !found {
		return models.Account{}, models.Account{}, false, nil
	}

	balanceAccount, found, err := tryResolveAccountByCandidates(tx, outletID, balanceCandidates)
	if err != nil {
		return models.Account{}, models.Account{}, false, err
	}
	if !found {
		return models.Account{}, models.Account{}, false, nil
	}

	return expenseAccount, balanceAccount, true, nil
}
