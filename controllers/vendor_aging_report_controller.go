package controllers

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/database"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/services"
)

type vendorAgingSummary struct {
	TotalBills             int     `json:"total_bills"`
	TotalAmount            float64 `json:"total_amount"`
	TotalPaidAmount        float64 `json:"total_paid_amount"`
	TotalOutstandingAmount float64 `json:"total_outstanding_amount"`
	UniqueVendors          int     `json:"unique_vendors"`
	OverdueBills           int     `json:"overdue_bills"`
	OverdueAmount          float64 `json:"overdue_amount"`
	DueTodayAmount         float64 `json:"due_today_amount"`
	CurrentAmount          float64 `json:"current_amount"`
	AverageAgeDays         int     `json:"average_age_days"`
	MaxAgeDays             int     `json:"max_age_days"`
}

type vendorAgingBreakdownItem struct {
	Key    string  `json:"key"`
	Label  string  `json:"label"`
	Count  int     `json:"count"`
	Amount float64 `json:"amount"`
}

type vendorAgingTimelineItem struct {
	PeriodKey         string  `json:"period_key"`
	PeriodLabel       string  `json:"period_label"`
	TotalBills        int     `json:"total_bills"`
	OutstandingAmount float64 `json:"outstanding_amount"`
	OverdueBills      int     `json:"overdue_bills"`
	OverdueAmount     float64 `json:"overdue_amount"`
}

type vendorAgingReportRow struct {
	ID                   uint       `json:"id"`
	OutletID             uint       `json:"outlet_id"`
	PurchaseID           *uint      `json:"purchase_id,omitempty"`
	VendorName           string     `json:"vendor_name"`
	BillNo               string     `json:"bill_no"`
	BillDate             time.Time  `json:"bill_date"`
	DueDate              *time.Time `json:"due_date,omitempty"`
	BillType             string     `json:"bill_type"`
	Status               string     `json:"status"`
	TotalAmount          float64    `json:"total_amount"`
	PaidAmount           float64    `json:"paid_amount"`
	OutstandingAmount    float64    `json:"outstanding_amount"`
	AgeDays              int        `json:"age_days"`
	DaysUntilDue         int        `json:"days_until_due"`
	AgingBucket          string     `json:"aging_bucket"`
	AgingBucketLabel     string     `json:"aging_bucket_label"`
	DueStatus            string     `json:"due_status"`
	DueStatusLabel       string     `json:"due_status_label"`
	AccountingSyncStatus string     `json:"accounting_sync_status"`
	AccountingSyncedAt   *time.Time `json:"accounting_synced_at,omitempty"`
	Note                 string     `json:"note"`
}

func GetVendorAgingReport(c *gin.Context) {
	if err := services.EnsurePOSFinanceDocumentSchema(database.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	query := database.DB.Model(&models.VendorBill{}).Order("COALESCE(due_date, bill_date) ASC, id DESC")

	if outletID := strings.TrimSpace(c.Query("outlet_id")); outletID != "" {
		query = query.Where("outlet_id = ?", outletID)
	}

	status := strings.ToLower(strings.TrimSpace(c.Query("status")))
	if status != "" {
		query = query.Where("LOWER(COALESCE(status, '')) = ?", status)
	} else {
		query = query.Where("LOWER(COALESCE(status, '')) <> ?", services.VendorBillStatusVoid)
		query = query.Where("COALESCE(outstanding_amount, 0) > 0")
	}

	if billType := strings.ToLower(strings.TrimSpace(c.Query("bill_type"))); billType != "" {
		query = query.Where("LOWER(COALESCE(bill_type, '')) = ?", billType)
	}

	if vendorName := strings.TrimSpace(c.Query("vendor")); vendorName != "" {
		query = query.Where("vendor_name ILIKE ?", "%"+vendorName+"%")
	}

	if startDate := strings.TrimSpace(c.Query("start_date")); startDate != "" {
		if parsed, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("DATE(bill_date) >= ?", parsed.Format("2006-01-02"))
		}
	}

	if endDate := strings.TrimSpace(c.Query("end_date")); endDate != "" {
		if parsed, err := time.Parse("2006-01-02", endDate); err == nil {
			query = query.Where("DATE(bill_date) <= ?", parsed.Format("2006-01-02"))
		}
	}

	var bills []models.VendorBill
	if err := query.Find(&bills).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data vendor aging"})
		return
	}

	today := normalizeVendorAgingDate(time.Now())
	agingBucketFilter := strings.ToLower(strings.TrimSpace(c.Query("aging_bucket")))
	search := strings.ToLower(strings.TrimSpace(c.Query("search")))

	rows := make([]vendorAgingReportRow, 0, len(bills))
	for _, bill := range bills {
		ageDays, daysUntilDue, agingBucket, agingBucketLabel, dueStatus, dueStatusLabel := computeVendorAgingMeta(bill.DueDate, bill.BillDate, today)

		row := vendorAgingReportRow{
			ID:                   bill.ID,
			OutletID:             bill.OutletID,
			PurchaseID:           bill.PurchaseID,
			VendorName:           bill.VendorName,
			BillNo:               bill.BillNo,
			BillDate:             bill.BillDate,
			DueDate:              bill.DueDate,
			BillType:             bill.BillType,
			Status:               bill.Status,
			TotalAmount:          bill.TotalAmount,
			PaidAmount:           bill.PaidAmount,
			OutstandingAmount:    bill.OutstandingAmount,
			AgeDays:              ageDays,
			DaysUntilDue:         daysUntilDue,
			AgingBucket:          agingBucket,
			AgingBucketLabel:     agingBucketLabel,
			DueStatus:            dueStatus,
			DueStatusLabel:       dueStatusLabel,
			AccountingSyncStatus: bill.AccountingSyncStatus,
			AccountingSyncedAt:   bill.AccountingSyncedAt,
			Note:                 bill.Note,
		}

		if agingBucketFilter != "" && agingBucket != agingBucketFilter {
			continue
		}

		if search != "" {
			candidates := []string{
				strconv.FormatUint(uint64(row.ID), 10),
				row.VendorName,
				row.BillNo,
				row.BillType,
				row.Status,
				row.Note,
				row.AgingBucketLabel,
				row.DueStatusLabel,
			}
			if row.PurchaseID != nil {
				candidates = append(candidates, strconv.FormatUint(uint64(*row.PurchaseID), 10))
			}

			matched := false
			for _, candidate := range candidates {
				if strings.Contains(strings.ToLower(strings.TrimSpace(candidate)), search) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		rows = append(rows, row)
	}

	sort.Slice(rows, func(left, right int) bool {
		if rows[left].AgeDays == rows[right].AgeDays {
			return rows[left].BillDate.After(rows[right].BillDate)
		}
		return rows[left].AgeDays > rows[right].AgeDays
	})

	c.JSON(http.StatusOK, buildVendorAgingResponse(rows))
}

func buildVendorAgingResponse(rows []vendorAgingReportRow) gin.H {
	summary := vendorAgingSummary{}
	vendorBreakdown := make(map[string]*vendorAgingBreakdownItem)
	billTypeBreakdown := make(map[string]*vendorAgingBreakdownItem)
	statusBreakdown := make(map[string]*vendorAgingBreakdownItem)
	agingBucketBreakdown := make(map[string]*vendorAgingBreakdownItem)
	dueTimeline := make(map[string]*vendorAgingTimelineItem)
	uniqueVendors := make(map[string]struct{})
	totalAgedDays := 0
	totalAgedRows := 0

	for _, row := range rows {
		summary.TotalBills++
		summary.TotalAmount += row.TotalAmount
		summary.TotalPaidAmount += row.PaidAmount
		summary.TotalOutstandingAmount += row.OutstandingAmount

		vendorKey := strings.ToLower(strings.TrimSpace(row.VendorName))
		if vendorKey != "" {
			uniqueVendors[vendorKey] = struct{}{}
			entry := vendorBreakdown[vendorKey]
			if entry == nil {
				entry = &vendorAgingBreakdownItem{
					Key:   vendorKey,
					Label: row.VendorName,
				}
				vendorBreakdown[vendorKey] = entry
			}
			entry.Count++
			entry.Amount += row.OutstandingAmount
		}

		billTypeKey := strings.ToLower(strings.TrimSpace(row.BillType))
		if billTypeKey == "" {
			billTypeKey = "unknown"
		}
		billTypeEntry := billTypeBreakdown[billTypeKey]
		if billTypeEntry == nil {
			billTypeEntry = &vendorAgingBreakdownItem{
				Key:   billTypeKey,
				Label: row.BillType,
			}
			if billTypeEntry.Label == "" {
				billTypeEntry.Label = "Unknown"
			}
			billTypeBreakdown[billTypeKey] = billTypeEntry
		}
		billTypeEntry.Count++
		billTypeEntry.Amount += row.OutstandingAmount

		statusKey := strings.ToLower(strings.TrimSpace(row.Status))
		if statusKey == "" {
			statusKey = "unknown"
		}
		statusEntry := statusBreakdown[statusKey]
		if statusEntry == nil {
			statusEntry = &vendorAgingBreakdownItem{
				Key:   statusKey,
				Label: row.Status,
			}
			if statusEntry.Label == "" {
				statusEntry.Label = "Unknown"
			}
			statusBreakdown[statusKey] = statusEntry
		}
		statusEntry.Count++
		statusEntry.Amount += row.OutstandingAmount

		bucketEntry := agingBucketBreakdown[row.AgingBucket]
		if bucketEntry == nil {
			bucketEntry = &vendorAgingBreakdownItem{
				Key:   row.AgingBucket,
				Label: row.AgingBucketLabel,
			}
			agingBucketBreakdown[row.AgingBucket] = bucketEntry
		}
		bucketEntry.Count++
		bucketEntry.Amount += row.OutstandingAmount

		referenceDate := row.BillDate
		if row.DueDate != nil && !row.DueDate.IsZero() {
			referenceDate = *row.DueDate
		}
		periodKey := referenceDate.Format("2006-01-02")
		timelineEntry := dueTimeline[periodKey]
		if timelineEntry == nil {
			timelineEntry = &vendorAgingTimelineItem{
				PeriodKey:   periodKey,
				PeriodLabel: referenceDate.Format("02 Jan 2006"),
			}
			dueTimeline[periodKey] = timelineEntry
		}
		timelineEntry.TotalBills++
		timelineEntry.OutstandingAmount += row.OutstandingAmount
		if row.DueStatus == "overdue" {
			timelineEntry.OverdueBills++
			timelineEntry.OverdueAmount += row.OutstandingAmount
		}

		switch row.DueStatus {
		case "overdue":
			summary.OverdueBills++
			summary.OverdueAmount += row.OutstandingAmount
		case "due_today":
			summary.DueTodayAmount += row.OutstandingAmount
		default:
			summary.CurrentAmount += row.OutstandingAmount
		}

		if row.AgeDays > 0 {
			totalAgedDays += row.AgeDays
			totalAgedRows++
			if row.AgeDays > summary.MaxAgeDays {
				summary.MaxAgeDays = row.AgeDays
			}
		}
	}

	summary.UniqueVendors = len(uniqueVendors)
	if totalAgedRows > 0 {
		summary.AverageAgeDays = totalAgedDays / totalAgedRows
	}

	return gin.H{
		"summary":                summary,
		"vendor_breakdown":       flattenVendorAgingBreakdown(vendorBreakdown),
		"bill_type_breakdown":    flattenVendorAgingBreakdown(billTypeBreakdown),
		"status_breakdown":       flattenVendorAgingBreakdown(statusBreakdown),
		"aging_bucket_breakdown": flattenVendorAgingBreakdown(agingBucketBreakdown),
		"due_timeline":           flattenVendorAgingTimeline(dueTimeline),
		"results":                rows,
		"total":                  len(rows),
	}
}

func flattenVendorAgingBreakdown(source map[string]*vendorAgingBreakdownItem) []vendorAgingBreakdownItem {
	items := make([]vendorAgingBreakdownItem, 0, len(source))
	for _, item := range source {
		items = append(items, *item)
	}
	sort.Slice(items, func(left, right int) bool {
		if items[left].Amount == items[right].Amount {
			return items[left].Count > items[right].Count
		}
		return items[left].Amount > items[right].Amount
	})
	return items
}

func flattenVendorAgingTimeline(source map[string]*vendorAgingTimelineItem) []vendorAgingTimelineItem {
	items := make([]vendorAgingTimelineItem, 0, len(source))
	for _, item := range source {
		items = append(items, *item)
	}
	sort.Slice(items, func(left, right int) bool {
		return items[left].PeriodKey < items[right].PeriodKey
	})
	return items
}

func normalizeVendorAgingDate(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func computeVendorAgingMeta(dueDate *time.Time, billDate, today time.Time) (int, int, string, string, string, string) {
	reference := normalizeVendorAgingDate(billDate)
	if dueDate != nil && !dueDate.IsZero() {
		reference = normalizeVendorAgingDate(*dueDate)
	}

	ageDays := int(today.Sub(reference).Hours() / 24)
	daysUntilDue := -ageDays

	switch {
	case ageDays < 0:
		return 0, daysUntilDue, "current", "Belum jatuh tempo", "current", "Belum jatuh tempo"
	case ageDays == 0:
		return 0, 0, "due_today", "Jatuh tempo hari ini", "due_today", "Jatuh tempo hari ini"
	case ageDays <= 30:
		return ageDays, 0, "overdue_1_30", "Overdue 1-30 hari", "overdue", "Lewat jatuh tempo"
	case ageDays <= 60:
		return ageDays, 0, "overdue_31_60", "Overdue 31-60 hari", "overdue", "Lewat jatuh tempo"
	case ageDays <= 90:
		return ageDays, 0, "overdue_61_90", "Overdue 61-90 hari", "overdue", "Lewat jatuh tempo"
	default:
		return ageDays, 0, "overdue_over_90", "Overdue > 90 hari", "overdue", "Lewat jatuh tempo"
	}
}
