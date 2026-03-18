package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/controllers"
	"github.com/kadeyasa/possystem/middleware"
)

func SetupRoutes(router *gin.Engine) {
	v1 := router.Group("/api/categories")
	v1.Use(middleware.AuthMiddleware())
	{
		v1.POST("/", controllers.CreateCategory)
		v1.GET("/", controllers.GetAllCategories)
		v1.GET("/:id", controllers.GetCategoryByID)
		v1.PUT("/:id", controllers.UpdateCategory)
		v1.DELETE("/:id", controllers.DeleteCategory)
		v1.GET("/details", controllers.GetCategoriesByPage)
	}

	product := router.Group("/api/products")
	product.Use(middleware.AuthMiddleware())
	{
		product.POST("/", controllers.CreateProduct)
		product.POST("/uploads", controllers.UploadImage)
		product.GET("/", controllers.GetAllProducts)
		product.GET("/:id", controllers.GetProductByID)
		product.GET("/product", controllers.GetProductByCat)
		product.PUT("/:id", controllers.UpdateProduct)
		product.DELETE("/:id", controllers.DeleteProduct)
		product.GET("/filters", controllers.GetProductFilter)
	}

	productRecipes := router.Group("/api/product-recipes")
	productRecipes.Use(middleware.AuthMiddleware())
	{
		productRecipes.POST("/", controllers.UpsertProductRecipe)
		productRecipes.GET("/", controllers.GetProductRecipes)
		productRecipes.GET("/:product_id", controllers.GetProductRecipeByProduct)
		productRecipes.DELETE("/:product_id", controllers.DeleteProductRecipe)
	}

	inventoryLedger := router.Group("/api/inventory-ledger")
	inventoryLedger.Use(middleware.AuthMiddleware())
	{
		inventoryLedger.GET("/", controllers.GetInventoryLedger)
		inventoryLedger.GET("/reports/stock-movement", controllers.GetStockMovementReport)
		inventoryLedger.GET("/reports/bom-consumption", controllers.GetBOMConsumptionReport)
	}

	stockAdjustments := router.Group("/api/stock-adjustments")
	stockAdjustments.Use(middleware.AuthMiddleware())
	{
		stockAdjustments.POST("/", controllers.CreateStockAdjustmentApprovalRequest)
		stockAdjustments.GET("/", controllers.GetStockAdjustments)
		stockAdjustments.GET("/:id", controllers.GetStockAdjustmentByID)
	}

	stockAdjustmentApprovalRequests := router.Group("/api/stock-adjustments/approval-requests")
	stockAdjustmentApprovalRequests.Use(middleware.AuthMiddleware())
	{
		stockAdjustmentApprovalRequests.GET("/", controllers.GetStockAdjustmentApprovalRequests)
		stockAdjustmentApprovalRequests.POST("/:id/approve", controllers.ApproveStockAdjustmentApprovalRequest)
		stockAdjustmentApprovalRequests.POST("/:id/reject", controllers.RejectStockAdjustmentApprovalRequest)
	}

	stockOpnames := router.Group("/api/stock-opnames")
	stockOpnames.Use(middleware.AuthMiddleware())
	{
		stockOpnames.POST("/", controllers.CreateStockOpname)
		stockOpnames.GET("/", controllers.GetStockOpnames)
		stockOpnames.GET("/reports/variance", controllers.GetStockOpnameVarianceReport)
		stockOpnames.GET("/:id", controllers.GetStockOpnameByID)
	}

	items := router.Group("api/items")
	items.Use(middleware.AuthMiddleware())
	{
		items.POST("/", controllers.CreateItem)
		items.GET("/", controllers.GetItems)
		items.GET("/detail", controllers.GetItem)
		items.DELETE("/:id", controllers.DeleteItem)
		items.PUT("/:id", controllers.UpdateItem)
	}

	metode := router.Group("api/metode")
	metode.Use(middleware.AuthMiddleware())
	{
		metode.POST("/", controllers.CreateMasterMetode)
		metode.GET("/", controllers.GetAllMetode)
		metode.DELETE("/:id", controllers.DeleteMetode)
	}

	waktu := router.Group("api/waktu")
	waktu.Use(middleware.AuthMiddleware())
	{
		waktu.POST("/", controllers.CreateMasterWaktu)
		waktu.GET("/", controllers.GetAllWaktu)
		waktu.DELETE("/:id", controllers.DeleteWaktu)
	}

	purchase := router.Group("/api/purchase")
	purchase.Use(middleware.AuthMiddleware())
	{
		purchase.POST("/", controllers.CreatePurchase)
		purchase.GET("/reports", controllers.GetPurchaseReport)
		purchase.GET("/reports/vendor-price-comparison", controllers.GetVendorPriceComparisonReport)
	}

	operationalExpenses := router.Group("/api/operational-expenses")
	operationalExpenses.Use(middleware.AuthMiddleware())
	{
		operationalExpenses.POST("/", controllers.CreateOperationalExpense)
		operationalExpenses.GET("/", controllers.GetOperationalExpenses)
		operationalExpenses.GET("/:id", controllers.GetOperationalExpenseByID)
	}

	vendorBills := router.Group("/api/vendor-bills")
	vendorBills.Use(middleware.AuthMiddleware())
	vendorBills.Use(middleware.AdminOnlyMiddleware())
	{
		vendorBills.POST("/", controllers.CreateVendorBill)
		vendorBills.GET("/", controllers.GetVendorBills)
		vendorBills.GET("/reports/aging", controllers.GetVendorAgingReport)
		vendorBills.GET("/:id", controllers.GetVendorBillByID)
	}

	vendorPayments := router.Group("/api/vendor-payments")
	vendorPayments.Use(middleware.AuthMiddleware())
	vendorPayments.Use(middleware.AdminOnlyMiddleware())
	{
		vendorPayments.POST("/", controllers.CreateVendorPayment)
		vendorPayments.GET("/", controllers.GetVendorPayments)
		vendorPayments.GET("/:id", controllers.GetVendorPaymentByID)
	}

	vendors := router.Group("/api/vendors")
	vendors.Use(middleware.AuthMiddleware())
	vendors.Use(middleware.AdminOnlyMiddleware())
	{
		vendors.POST("/", controllers.CreateVendor)
		vendors.GET("/", controllers.GetVendors)
		vendors.GET("/:id", controllers.GetVendorByID)
		vendors.PUT("/:id", controllers.UpdateVendor)
	}

	accountGroup := router.Group("/api/accounts")
	accountGroup.Use(middleware.AuthMiddleware())
	{
		accountGroup.POST("/", controllers.CreateAccount)
		accountGroup.POST("/copy-mappings", controllers.CopyAccountMappings)
		accountGroup.GET("/", controllers.GetAllAccounts)            // GET all
		accountGroup.GET("/filter", controllers.GetAccountsByFilter) // GET by filter
		accountGroup.GET("/:id", controllers.GetAccountByID)         // GET by ID
		accountGroup.PUT("/:id", controllers.UpdateAccount)
		accountGroup.DELETE("/:id", controllers.DeleteAccount)
	}

	transactionGroup := router.Group("/api/transactions")
	transactionGroup.Use(middleware.AuthMiddleware())
	{
		transactionGroup.POST("/", controllers.CreateTransaction)
		transactionGroup.GET("/", controllers.GetAllTransactions)
		transactionGroup.GET("/:id", controllers.GetTransactionByID)
		transactionGroup.GET("/daily", controllers.GetTransactionsDaily)
		transactionGroup.GET("/weekly", controllers.GetTransactionsWeekly)
		transactionGroup.GET("/monthly", controllers.GetTransactionsMonthly)
		transactionGroup.GET("/salesreport", controllers.GetSalesReport)
		transactionGroup.GET("/dashboarddata", controllers.GetDashboardInfo)
		transactionGroup.PUT("/update/:id", controllers.UpdateStatus)
	}

	shiftClosings := router.Group("/api/shift-closings")
	shiftClosings.Use(middleware.AuthMiddleware())
	{
		shiftClosings.GET("/", controllers.GetStaffShiftClosings)
		shiftClosings.GET("/current", controllers.GetCurrentStaffShiftClosingPreview)
		shiftClosings.POST("/", controllers.CreateStaffShiftClosing)
	}

	orderDrafts := router.Group("/api/order-drafts")
	orderDrafts.Use(middleware.AuthMiddleware())
	{
		orderDrafts.POST("/", controllers.CreateOrderDraft)
		orderDrafts.GET("/", controllers.GetOrderDrafts)
		orderDrafts.GET("/kitchen/queue", controllers.GetKitchenOrderDraftQueue)
		orderDrafts.GET("/:id", controllers.GetOrderDraftByID)
		orderDrafts.POST("/:id/send-to-kitchen", controllers.SendOrderDraftToKitchen)
		orderDrafts.POST("/:id/kitchen-start", controllers.StartKitchenOrderDraft)
		orderDrafts.POST("/:id/kitchen-complete", controllers.CompleteKitchenOrderDraft)
		orderDrafts.POST("/:id/kitchen-cancel", controllers.CancelKitchenOrderDraft)
		orderDrafts.POST("/:id/complete", controllers.CompleteOrderDraft)
		orderDrafts.POST("/:id/cancel", controllers.CancelOrderDraft)
	}

	refunds := router.Group("/api/refunds")
	refunds.Use(middleware.AuthMiddleware())
	{
		refunds.GET("/day", controllers.GetRefundsByDay)
		refunds.GET("/month", controllers.GetRefundsByMonth)
		refunds.GET("/:id", controllers.GetRefundDetail)
		refunds.GET("/barcode", controllers.GetRefundsByBarcode)
		refunds.POST("/", controllers.CreateRefund)
		refunds.GET("/reports", controllers.GetRefundReport)
	}

	approvalRequestsSubmission := router.Group("/api/approval-requests")
	approvalRequestsSubmission.Use(middleware.AuthMiddleware())
	{
		approvalRequestsSubmission.POST("/refund", controllers.CreateRefundApprovalRequest)
		approvalRequestsSubmission.POST("/void", controllers.CreateVoidApprovalRequest)
	}

	approvalRequests := router.Group("/api/approval-requests")
	approvalRequests.Use(middleware.AuthMiddleware())
	approvalRequests.Use(middleware.ManagementOnlyMiddleware())
	{
		approvalRequests.GET("/", controllers.GetPOSApprovalRequests)
		approvalRequests.POST("/:id/approve", controllers.ApprovePOSApprovalRequest)
		approvalRequests.POST("/:id/reject", controllers.RejectPOSApprovalRequest)
	}

	customers := router.Group("/api/customers")
	customers.Use(middleware.AuthMiddleware())
	{
		customers.GET("/", controllers.GetAllCustomers)      // GET all customers
		customers.GET("/:id", controllers.GetCustomerByID)   // GET customer by ID
		customers.POST("/", controllers.CreateCustomer)      // POST create customer
		customers.PUT("/:id", controllers.UpdateCustomer)    // PUT update customer
		customers.DELETE("/:id", controllers.DeleteCustomer) // DELETE customer
		customers.GET("/outlet", controllers.GetCustomerByOutlet)
	}

	deposit := router.Group("/api/deposit")
	deposit.Use(middleware.AuthMiddleware())
	{
		deposit.POST("/", controllers.CreateDeposit)
		deposit.GET("/", controllers.GetDepositByOutlet)
		deposit.GET("/:id", controllers.GetDepositById)
		deposit.PUT("/approve/:id", controllers.ApproveDeposit)
	}

	balance := router.Group("/api/balance")
	balance.Use(middleware.AuthMiddleware())
	{
		balance.GET("/", controllers.GetUserBalance)
		balance.GET("/revenue", controllers.GetRevenue)
		balance.GET("/all", controllers.GetAllBalance)
	}

	accountingSync := router.Group("/api/accounting/sync")
	accountingSync.Use(middleware.AuthMiddleware())
	{
		accountingSync.GET("/summary", controllers.GetAccountingSyncSummary)
		accountingSync.GET("/records", controllers.GetAccountingSyncRecords)
		accountingSync.POST("/retry", controllers.RetryAccountingSync)
		accountingSync.POST("/retry-failed", controllers.RetryFailedAccountingSync)
	}

	outletfee := router.Group("/api/outletfee")
	outletfee.Use(middleware.AuthMiddleware())
	{
		outletfee.POST("/", controllers.CreateOutletFee)
		outletfee.GET("/", controllers.GetOutletFeeHandler)
	}

	reports := router.Group("/reports")
	reports.Use(middleware.AuthMiddleware())
	{
		reports.GET("/journals", controllers.GetJournalReport)
		reports.GET("/balance-sheet", controllers.GetBalanceSheet)
	}

	publicPOS := router.Group("/api/public")
	{
		publicPOS.GET("/menu-restoran", controllers.GetPublicMenuCatalog)
		publicPOS.POST("/pesanan-restoran", controllers.CreatePublicOrderDraft)
	}
	router.Static("/datauploads", "./uploads")

}
