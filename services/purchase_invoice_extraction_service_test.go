package services

import (
	"strings"
	"testing"
)

func TestParsePurchaseInvoiceText(t *testing.T) {
	rawText := strings.Join([]string{
		"CU RAHAYD",
		"RANAYU MINT MARKET",
		"NРMP : 002.663-761.9-908 .000",
		"TANGGAL PENGURHAN 02/12/2010",
		"JLN NGURAH RAI NO. 8 JEMBRANA - BALI",
		"BENG-RENG WAPER CARAMEL 17*25GR",
		"8996001354131 2X36,375",
		"72,750",
		"SOYJOY STRAMRERRY 3OGR",
		"8497035600430 5X8,650",
		"43,250",
		"SOYJOY RAISIN PEAMUT 30ÊN",
		"8997035563476 5xB,650",
		"43,250",
		"SOYJOY ALMOND&COKLAT JOGR",
		"8947035600928 5xB,650",
		"43,250",
		"total belanja 4 Item",
		"Fotongan",
		"Potongan Foin",
		"Charge",
		"lotal",
		"CASH",
		"202,500",
		"202,500",
		"202, 500",
		"22/04/2026 08:05",
		"1RS2/94",
	}, "\n")

	result := parsePurchaseInvoiceText(rawText)
	if result == nil {
		t.Fatal("expected parsed result")
	}
	if !strings.Contains(strings.ToUpper(result.SupplierName), "MARKET") {
		t.Fatalf("expected supplier to include MARKET, got %q", result.SupplierName)
	}
	if result.PurchaseDate != "2026-04-22" {
		t.Fatalf("expected purchase date 2026-04-22, got %q", result.PurchaseDate)
	}
	if result.PaymentMethod != "cash" {
		t.Fatalf("expected payment method cash, got %q", result.PaymentMethod)
	}
	if result.Total != 202500 {
		t.Fatalf("expected total 202500, got %.0f", result.Total)
	}
	if len(result.Items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(result.Items))
	}

	firstItem := result.Items[0]
	if firstItem.Quantity != 2 {
		t.Fatalf("expected first item quantity 2, got %d", firstItem.Quantity)
	}
	if firstItem.PurchasePrice != 36375 {
		t.Fatalf("expected first item purchase price 36375, got %.0f", firstItem.PurchasePrice)
	}
	if firstItem.Total != 72750 {
		t.Fatalf("expected first item total 72750, got %.0f", firstItem.Total)
	}
}
