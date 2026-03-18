package controllers

import "strings"

func isInventoryValidationError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "insufficient stock"):
		return true
	case strings.Contains(message, "refundable quantity"):
		return true
	case strings.Contains(message, "not part of the original transaction"):
		return true
	case strings.Contains(message, "has no refundable quantity left"):
		return true
	case strings.Contains(message, "at least one"):
		return true
	case strings.Contains(message, "quantity must be greater than zero"):
		return true
	case strings.Contains(message, "product_id is required"):
		return true
	case strings.Contains(message, "quantity_delta must not be zero"):
		return true
	case strings.Contains(message, "actual_stock must not be negative"):
		return true
	case strings.Contains(message, "not stock tracked"):
		return true
	case strings.Contains(message, "would make stock negative"):
		return true
	default:
		return false
	}
}
