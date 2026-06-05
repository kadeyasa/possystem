package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	invoiceOCRProviderAuto        = "auto"
	invoiceOCRProviderMacOSVision = "macos_vision"
)

type PurchaseInvoiceExtraction struct {
	Provider      string                          `json:"provider"`
	SupplierName  string                          `json:"supplier_name"`
	InvoiceNumber string                          `json:"invoice_number"`
	PurchaseDate  string                          `json:"purchase_date"`
	PaymentMethod string                          `json:"payment_method"`
	Total         float64                         `json:"total"`
	Items         []PurchaseInvoiceExtractionItem `json:"items"`
	RawText       string                          `json:"raw_text"`
	Warnings      []string                        `json:"warnings"`
}

type PurchaseInvoiceExtractionItem struct {
	Description   string   `json:"description"`
	Barcode       string   `json:"barcode,omitempty"`
	Quantity      int      `json:"quantity"`
	PurchasePrice float64  `json:"purchase_price"`
	Total         float64  `json:"total"`
	RawLines      []string `json:"raw_lines,omitempty"`
}

type purchaseInvoiceOCRResult struct {
	Provider string
	Text     string
}

var (
	receiptDatePattern          = regexp.MustCompile(`(?i)(\d{1,2}[/-]\d{1,2}[/-]\d{2,4})(?:\s+(\d{1,2}:\d{2}(?::\d{2})?))?`)
	receiptInvoiceNoPattern     = regexp.MustCompile(`(?i)(?:invoice|inv|nota|bill|ref|trx|transaction|no)\s*[:#-]?\s*([A-Z0-9./-]{3,})`)
	receiptQuantityPricePattern = regexp.MustCompile(`(?i)(\d{1,3})\s*[xX]\s*([0-9OobBsSIlG,.\s]{2,})`)
	receiptBarcodePattern       = regexp.MustCompile(`\b\d{10,16}\b`)
	receiptAmountPattern        = regexp.MustCompile(`(^|[^A-Za-z0-9])([0-9OobBsSIlG][0-9OobBsSIlG,.\s]{1,}[0-9OobBsSIlG])($|[^A-Za-z0-9])`)
	darwinHostArchOnce          sync.Once
	darwinHostArchValue         string
	darwinHostArchErr           error
)

func ExtractPurchaseInvoiceData(ctx context.Context, imagePath string) (*PurchaseInvoiceExtraction, error) {
	ocrResult, err := extractPurchaseInvoiceOCR(ctx, imagePath)
	if err != nil {
		return nil, err
	}

	result := parsePurchaseInvoiceText(ocrResult.Text)
	result.Provider = ocrResult.Provider
	return result, nil
}

func extractPurchaseInvoiceOCR(ctx context.Context, imagePath string) (*purchaseInvoiceOCRResult, error) {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("PURCHASE_INVOICE_OCR_PROVIDER")))
	if provider == "" {
		provider = invoiceOCRProviderAuto
	}

	switch provider {
	case invoiceOCRProviderAuto:
		if runtime.GOOS == "darwin" {
			return runMacOSVisionPurchaseOCR(ctx, imagePath)
		}
	case invoiceOCRProviderMacOSVision:
		return runMacOSVisionPurchaseOCR(ctx, imagePath)
	default:
		return nil, fmt.Errorf("unsupported purchase invoice OCR provider %q", provider)
	}

	return nil, errors.New("no purchase invoice OCR provider available in this environment")
}

func runMacOSVisionPurchaseOCR(ctx context.Context, imagePath string) (*purchaseInvoiceOCRResult, error) {
	if runtime.GOOS != "darwin" {
		return nil, errors.New("macOS Vision OCR is only available on darwin runtime")
	}

	swiftPath, err := exec.LookPath("swift")
	if err != nil {
		return nil, errors.New("swift runtime not found for invoice OCR")
	}

	scriptPath, err := resolveInvoiceOCRSwiftScript()
	if err != nil {
		return nil, err
	}

	cacheRoot := filepath.Join(os.TempDir(), "possystem-invoice-ocr")
	homeRoot := filepath.Join(cacheRoot, "home")
	clangModuleCache := filepath.Join(homeRoot, ".cache", "clang", "ModuleCache")
	swiftModuleCache := filepath.Join(cacheRoot, "swift-module-cache")
	for _, dir := range []string{cacheRoot, homeRoot, clangModuleCache, swiftModuleCache} {
		if mkdirErr := os.MkdirAll(dir, 0o750); mkdirErr != nil {
			return nil, fmt.Errorf("failed to prepare OCR cache directory: %w", mkdirErr)
		}
	}

	command := buildMacOSVisionOCRCommand(ctx, swiftPath, scriptPath, imagePath)
	command.Env = append(os.Environ(),
		"HOME="+homeRoot,
		"XDG_CACHE_HOME="+filepath.Join(homeRoot, ".cache"),
		"CLANG_MODULE_CACHE_PATH="+clangModuleCache,
		"SWIFT_MODULE_CACHE_PATH="+swiftModuleCache,
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		errorOutput := strings.TrimSpace(stderr.String())
		if errorOutput == "" {
			errorOutput = strings.TrimSpace(stdout.String())
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			if errorOutput != "" {
				errorOutput = fmt.Sprintf("%s (%v)", errorOutput, ctxErr)
			} else {
				errorOutput = ctxErr.Error()
			}
		}
		if errorOutput == "" {
			errorOutput = err.Error()
		}
		return nil, fmt.Errorf("macOS Vision OCR failed: %s", errorOutput)
	}

	text := strings.TrimSpace(stdout.String())
	if text == "" {
		return nil, errors.New("invoice OCR returned empty text")
	}

	return &purchaseInvoiceOCRResult{
		Provider: invoiceOCRProviderMacOSVision,
		Text:     text,
	}, nil
}

func buildMacOSVisionOCRCommand(ctx context.Context, swiftPath string, scriptPath string, imagePath string) *exec.Cmd {
	if hostArch, err := detectDarwinHostArchitecture(); err == nil && strings.HasPrefix(hostArch, "arm64") {
		if archPath, archErr := exec.LookPath("arch"); archErr == nil {
			// Force native arm64 Swift on Apple Silicon even if the Go service is running under Rosetta.
			return exec.CommandContext(ctx, archPath, "-arm64", swiftPath, scriptPath, imagePath)
		}
	}

	return exec.CommandContext(ctx, swiftPath, scriptPath, imagePath)
}

func detectDarwinHostArchitecture() (string, error) {
	darwinHostArchOnce.Do(func() {
		unamePath, err := exec.LookPath("uname")
		if err != nil {
			darwinHostArchErr = err
			return
		}

		output, err := exec.Command(unamePath, "-m").Output()
		if err != nil {
			darwinHostArchErr = err
			return
		}

		darwinHostArchValue = strings.ToLower(strings.TrimSpace(string(output)))
	})

	return darwinHostArchValue, darwinHostArchErr
}

func resolveInvoiceOCRSwiftScript() (string, error) {
	candidates := []string{}

	if configured := strings.TrimSpace(os.Getenv("PURCHASE_INVOICE_OCR_SWIFT_SCRIPT")); configured != "" {
		candidates = append(candidates, configured)
	}

	candidates = append(candidates,
		filepath.Join("scripts", "invoice_ocr.swift"),
		filepath.Join(".", "scripts", "invoice_ocr.swift"),
	)

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", errors.New("invoice OCR swift script not found; expected scripts/invoice_ocr.swift")
}

func parsePurchaseInvoiceText(rawText string) *PurchaseInvoiceExtraction {
	lines := normalizeReceiptLines(rawText)
	result := &PurchaseInvoiceExtraction{
		RawText: strings.Join(lines, "\n"),
		Items:   extractReceiptItems(lines),
	}

	result.SupplierName = guessReceiptSupplier(lines)
	result.InvoiceNumber = guessReceiptInvoiceNumber(lines)
	result.PurchaseDate = guessReceiptPurchaseDate(lines)
	result.PaymentMethod = guessReceiptPaymentMethod(lines)
	result.Total = guessReceiptTotal(lines)
	if result.Total <= 0 {
		result.Total = sumReceiptItems(result.Items)
	}

	if strings.TrimSpace(result.SupplierName) == "" {
		result.Warnings = append(result.Warnings, "Nama supplier tidak terbaca jelas. Periksa hasil scan sebelum simpan.")
	}
	if strings.TrimSpace(result.InvoiceNumber) == "" {
		result.Warnings = append(result.Warnings, "Nomor invoice tidak ditemukan. Isi manual bila supplier menyertakannya.")
	}
	if len(result.Items) == 0 {
		result.Warnings = append(result.Warnings, "Item pembelian belum berhasil diekstrak. Pastikan foto invoice cukup jelas.")
	}
	if result.Total > 0 && len(result.Items) > 0 {
		itemTotal := sumReceiptItems(result.Items)
		if itemTotal > 0 && absFloat(result.Total-itemTotal) > 1 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Total scan Rp %.0f berbeda dengan subtotal item Rp %.0f. Tinjau ulang hasil OCR.", result.Total, itemTotal))
		}
	}

	return result
}

func normalizeReceiptLines(rawText string) []string {
	lines := strings.Split(rawText, "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.Join(strings.Fields(line), " "))
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func guessReceiptSupplier(lines []string) string {
	bestLine := ""
	bestScore := -1
	for index, line := range lines {
		if looksLikeReceiptItemStart(lines, index) {
			break
		}
		if !isReceiptSupplierCandidate(line) {
			continue
		}

		score := countAlphaNumericLetters(line)
		normalized := strings.ToLower(line)
		for _, keyword := range []string{"market", "mart", "minimarket", "mini market", "toko", "store", "cv", "pt", "ud", "tb"} {
			if strings.Contains(normalized, keyword) {
				score += 12
			}
		}
		if index <= 2 {
			score += 4 - index
		}
		if hasLongDigitSequence(line) {
			score -= 8
		}
		if score > bestScore {
			bestScore = score
			bestLine = line
		}
	}
	return strings.TrimSpace(bestLine)
}

func guessReceiptInvoiceNumber(lines []string) string {
	for _, line := range lines {
		match := receiptInvoiceNoPattern.FindStringSubmatch(strings.ToUpper(line))
		if len(match) < 2 {
			continue
		}
		value := strings.TrimSpace(match[1])
		if value == "" {
			continue
		}
		if strings.EqualFold(value, "NPWP") {
			continue
		}
		return value
	}
	return ""
}

func guessReceiptPurchaseDate(lines []string) string {
	type candidate struct {
		value time.Time
		score int
	}

	var best candidate
	found := false
	for index, line := range lines {
		lowerLine := strings.ToLower(line)
		matches := receiptDatePattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			parsedDate, ok := parseReceiptDate(match[1])
			if !ok {
				continue
			}

			score := 0
			if len(match) > 2 && strings.TrimSpace(match[2]) != "" {
				score += 6
			}
			if strings.Contains(lowerLine, "tanggal") || strings.Contains(lowerLine, "date") {
				score += 3
			}
			if index >= maxInt(len(lines)-6, 0) {
				score += 2
			}
			if parsedDate.Year() >= 2020 {
				score += 1
			}

			if !found || score > best.score || (score == best.score && parsedDate.After(best.value)) {
				best = candidate{value: parsedDate, score: score}
				found = true
			}
		}
	}

	if !found {
		return ""
	}
	return best.value.Format("2006-01-02")
}

func guessReceiptPaymentMethod(lines []string) string {
	for _, line := range lines {
		normalized := strings.ToLower(line)
		switch {
		case strings.Contains(normalized, "qris"):
			return "qris"
		case strings.Contains(normalized, "transfer"):
			return "transfer"
		case strings.Contains(normalized, "kredit"), strings.Contains(normalized, "credit"):
			return "credit"
		case strings.Contains(normalized, "tunai"), strings.Contains(normalized, "cash"):
			return "cash"
		}
	}
	return ""
}

func guessReceiptTotal(lines []string) float64 {
	bestScore := -1
	bestValue := 0
	for index, line := range lines {
		normalized := strings.ToLower(line)
		if !strings.Contains(normalized, "total") && !strings.Contains(normalized, "cash") && !strings.Contains(normalized, "grand") {
			continue
		}

		amounts := extractReceiptAmounts(line)
		if len(amounts) == 0 && index+1 < len(lines) {
			amounts = extractReceiptAmounts(lines[index+1])
		}
		if len(amounts) == 0 {
			continue
		}

		score := 0
		if strings.Contains(normalized, "grand") {
			score += 4
		}
		if strings.Contains(normalized, "total") {
			score += 3
		}
		if strings.Contains(normalized, "cash") {
			score += 1
		}
		score += index

		value := amounts[len(amounts)-1]
		if score > bestScore || (score == bestScore && value > bestValue) {
			bestScore = score
			bestValue = value
		}
	}

	if bestValue <= 0 {
		return 0
	}
	return float64(bestValue)
}

func extractReceiptItems(lines []string) []PurchaseInvoiceExtractionItem {
	items := make([]PurchaseInvoiceExtractionItem, 0)
	var current *PurchaseInvoiceExtractionItem
	started := false

	finalizeCurrent := func() {
		if current == nil {
			return
		}
		normalized := finalizeReceiptItem(*current)
		if isUsefulReceiptItem(normalized) {
			items = append(items, normalized)
		}
		current = nil
	}

	for index, line := range lines {
		if isReceiptSummaryLine(line) {
			if started {
				finalizeCurrent()
				break
			}
			continue
		}

		if !started {
			if !looksLikeReceiptItemStart(lines, index) {
				continue
			}
			started = true
		}

		if current == nil {
			if !looksLikeReceiptItemLine(line) {
				continue
			}
			item := PurchaseInvoiceExtractionItem{
				Description: cleanReceiptDescription(line),
				Quantity:    0,
				RawLines:    []string{line},
			}
			enrichReceiptItemFromLine(&item, line)
			current = &item
			continue
		}

		if looksLikeReceiptItemLine(line) {
			if current.PurchasePrice > 0 || current.Total > 0 || current.Barcode != "" {
				finalizeCurrent()
				item := PurchaseInvoiceExtractionItem{
					Description: cleanReceiptDescription(line),
					Quantity:    0,
					RawLines:    []string{line},
				}
				enrichReceiptItemFromLine(&item, line)
				current = &item
				continue
			}

			current.Description = strings.TrimSpace(strings.Join([]string{current.Description, cleanReceiptDescription(line)}, " "))
			current.RawLines = append(current.RawLines, line)
			enrichReceiptItemFromLine(current, line)
			continue
		}

		current.RawLines = append(current.RawLines, line)
		enrichReceiptItemFromLine(current, line)
	}

	finalizeCurrent()
	return items
}

func finalizeReceiptItem(item PurchaseInvoiceExtractionItem) PurchaseInvoiceExtractionItem {
	item.Description = strings.TrimSpace(strings.Join(strings.Fields(item.Description), " "))
	if item.Quantity <= 0 {
		item.Quantity = 1
	}
	if item.Total <= 0 && item.PurchasePrice > 0 {
		item.Total = float64(item.Quantity) * item.PurchasePrice
	}
	if item.PurchasePrice <= 0 && item.Total > 0 && item.Quantity > 0 {
		item.PurchasePrice = item.Total / float64(item.Quantity)
	}
	return item
}

func isUsefulReceiptItem(item PurchaseInvoiceExtractionItem) bool {
	if strings.TrimSpace(item.Description) == "" {
		return false
	}
	if countAlphaNumericLetters(item.Description) < 3 {
		return false
	}
	return item.Quantity > 0 || item.PurchasePrice > 0 || item.Total > 0
}

func enrichReceiptItemFromLine(item *PurchaseInvoiceExtractionItem, line string) {
	if item == nil {
		return
	}

	if item.Barcode == "" {
		if barcode := receiptBarcodePattern.FindString(line); barcode != "" {
			item.Barcode = barcode
		}
	}

	if item.Quantity <= 0 || item.PurchasePrice <= 0 {
		qtyMatch := receiptQuantityPricePattern.FindStringSubmatch(line)
		if len(qtyMatch) >= 3 {
			if quantity, err := strconv.Atoi(strings.TrimSpace(qtyMatch[1])); err == nil && quantity > 0 && item.Quantity <= 0 {
				item.Quantity = quantity
			}
			if price, ok := parseReceiptAmount(qtyMatch[2]); ok && price > 0 && item.PurchasePrice <= 0 {
				item.PurchasePrice = float64(price)
			}
		}
	}

	amounts := extractReceiptAmounts(line)
	if len(amounts) == 0 {
		return
	}

	filtered := make([]int, 0, len(amounts))
	for _, amount := range amounts {
		if amount <= 0 {
			continue
		}
		if len(strconv.Itoa(amount)) > 8 {
			continue
		}
		filtered = append(filtered, amount)
	}
	if len(filtered) == 0 {
		return
	}

	sort.Ints(filtered)
	if item.PurchasePrice <= 0 {
		item.PurchasePrice = float64(filtered[len(filtered)-1])
	}

	candidateTotal := filtered[len(filtered)-1]
	if item.Quantity > 1 && item.PurchasePrice > 0 && absFloat(float64(candidateTotal)-item.PurchasePrice) <= 1 {
		if len(filtered) >= 2 {
			candidateTotal = filtered[len(filtered)-2]
		}
	}

	if item.Quantity > 0 && item.PurchasePrice > 0 {
		expectedTotal := float64(item.Quantity) * item.PurchasePrice
		if absFloat(float64(candidateTotal)-expectedTotal) <= maxFloat(expectedTotal*0.12, 1500) {
			item.Total = float64(candidateTotal)
			return
		}
		if item.Total <= 0 && item.Quantity == 1 && candidateTotal >= int(item.PurchasePrice) {
			item.Total = float64(candidateTotal)
		}
		return
	}

	if item.Total <= 0 || candidateTotal > int(item.Total) {
		if candidateTotal >= int(item.PurchasePrice) {
			item.Total = float64(candidateTotal)
		}
	}
}

func looksLikeReceiptItemLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if isReceiptSummaryLine(trimmed) {
		return false
	}
	if isReceiptHeaderNoise(trimmed) {
		return false
	}
	if receiptDatePattern.MatchString(trimmed) {
		return false
	}
	if receiptBarcodePattern.MatchString(trimmed) {
		return false
	}

	alphaCount := countAlphaNumericLetters(trimmed)
	if alphaCount < 4 {
		return false
	}

	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "npwp") || strings.Contains(lower, "jln") || strings.Contains(lower, "jalan") {
		return false
	}

	return true
}

func looksLikeReceiptItemStart(lines []string, index int) bool {
	if index < 0 || index >= len(lines) {
		return false
	}

	line := lines[index]
	if !looksLikeReceiptItemLine(line) {
		return false
	}
	if strings.IndexFunc(line, func(character rune) bool {
		return character >= '0' && character <= '9'
	}) >= 0 {
		return true
	}

	for offset := 1; offset <= 2; offset++ {
		nextIndex := index + offset
		if nextIndex >= len(lines) {
			break
		}

		nextLine := lines[nextIndex]
		if receiptBarcodePattern.MatchString(nextLine) || receiptQuantityPricePattern.MatchString(nextLine) {
			return true
		}
	}

	return false
}

func cleanReceiptDescription(line string) string {
	cleaned := receiptBarcodePattern.ReplaceAllString(line, "")
	cleaned = receiptQuantityPricePattern.ReplaceAllString(cleaned, "")
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	cleaned = strings.Trim(cleaned, "-: ")
	return cleaned
}

func isReceiptSummaryLine(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	for _, keyword := range []string{
		"total belanja",
		"subtotal",
		"grand total",
		"potongan",
		"diskon",
		"charge",
		"cash",
		"tunai",
		"kembalian",
		"change",
		"terima kasih",
		"thank you",
		"poin",
	} {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func isReceiptHeaderNoise(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	for _, keyword := range []string{
		"npwp",
		"tanggal",
		"jln ",
		"jalan",
		"telp",
		"phone",
		"alamat",
		"tanggal pengukuhan",
		"ppn",
		"bkp",
		"terima kasih",
	} {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func isReceiptSupplierCandidate(line string) bool {
	if isReceiptHeaderNoise(line) || isReceiptSummaryLine(line) {
		return false
	}
	if countAlphaNumericLetters(line) < 4 {
		return false
	}
	return !receiptDatePattern.MatchString(line)
}

func extractReceiptAmounts(line string) []int {
	matches := receiptAmountPattern.FindAllStringSubmatch(line, -1)
	amounts := make([]int, 0, len(matches))
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		if amount, ok := parseReceiptAmount(match[2]); ok {
			amounts = append(amounts, amount)
		}
	}
	return amounts
}

func parseReceiptAmount(raw string) (int, bool) {
	replacer := strings.NewReplacer(
		"O", "0",
		"o", "0",
		"D", "0",
		"B", "8",
		"b", "8",
		"S", "5",
		"s", "5",
		"I", "1",
		"l", "1",
		"G", "6",
	)
	normalized := replacer.Replace(raw)
	digitsOnly := make([]rune, 0, len(normalized))
	for _, character := range normalized {
		if character >= '0' && character <= '9' {
			digitsOnly = append(digitsOnly, character)
		}
	}
	if len(digitsOnly) < 3 {
		return 0, false
	}
	value, err := strconv.Atoi(string(digitsOnly))
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func parseReceiptDate(raw string) (time.Time, bool) {
	cleaned := strings.ReplaceAll(strings.TrimSpace(raw), "-", "/")
	parts := strings.Split(cleaned, "/")
	if len(parts) != 3 {
		return time.Time{}, false
	}

	day, errDay := strconv.Atoi(parts[0])
	month, errMonth := strconv.Atoi(parts[1])
	year, errYear := strconv.Atoi(parts[2])
	if errDay != nil || errMonth != nil || errYear != nil {
		return time.Time{}, false
	}
	if year < 100 {
		year += 2000
	}
	if day <= 0 || month <= 0 || month > 12 || year < 2000 {
		return time.Time{}, false
	}

	value := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if value.Day() != day || int(value.Month()) != month || value.Year() != year {
		return time.Time{}, false
	}
	return value, true
}

func sumReceiptItems(items []PurchaseInvoiceExtractionItem) float64 {
	total := 0.0
	for _, item := range items {
		total += item.Total
	}
	return total
}

func countAlphaNumericLetters(value string) int {
	count := 0
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') {
			count++
		}
	}
	return count
}

func hasLongDigitSequence(value string) bool {
	match := regexp.MustCompile(`\d{6,}`).FindString(value)
	return match != ""
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func maxFloat(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
