package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/xltxb/PetManage/internal/announcement"
	"github.com/xltxb/PetManage/internal/appointment"
	"github.com/xltxb/PetManage/internal/attendance"
	"github.com/xltxb/PetManage/internal/auth"
	"github.com/xltxb/PetManage/internal/balance"
	"github.com/xltxb/PetManage/internal/category"
	"github.com/xltxb/PetManage/internal/checkout"
	"github.com/xltxb/PetManage/internal/commission"
	"github.com/xltxb/PetManage/internal/coupon"
	"github.com/xltxb/PetManage/internal/servicecard"
	"github.com/xltxb/PetManage/internal/fixedexpense"
	"github.com/xltxb/PetManage/internal/revenue"
	"github.com/xltxb/PetManage/internal/statement"
	"github.com/xltxb/PetManage/internal/complaint"
	"github.com/xltxb/PetManage/internal/config"
	"github.com/xltxb/PetManage/internal/contract"
	"github.com/xltxb/PetManage/internal/dashboard"
	"github.com/xltxb/PetManage/internal/database"
	"github.com/xltxb/PetManage/internal/dictionary"
	"github.com/xltxb/PetManage/internal/employee"
	"github.com/xltxb/PetManage/internal/inventory"
	"github.com/xltxb/PetManage/internal/merchant"
	"github.com/xltxb/PetManage/internal/merchantrole"
	"github.com/xltxb/PetManage/internal/member"
	"github.com/xltxb/PetManage/internal/memberlevel"
	"github.com/xltxb/PetManage/internal/membertag"
	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/internal/notification"
	"github.com/xltxb/PetManage/internal/operationlog"
	"github.com/xltxb/PetManage/internal/orders"
	"github.com/xltxb/PetManage/internal/payable"
	"github.com/xltxb/PetManage/internal/pet"
	"github.com/xltxb/PetManage/internal/promotion"
	"github.com/xltxb/PetManage/internal/points"
	"github.com/xltxb/PetManage/internal/product"
	"github.com/xltxb/PetManage/internal/purchase"
	"github.com/xltxb/PetManage/internal/replenishment"
	"github.com/xltxb/PetManage/internal/report"
	"github.com/xltxb/PetManage/internal/review"
	"github.com/xltxb/PetManage/internal/risk"
	"github.com/xltxb/PetManage/internal/role"
	"github.com/xltxb/PetManage/internal/schedule"
	"github.com/xltxb/PetManage/internal/servicepackage"
	"github.com/xltxb/PetManage/internal/servicemgmt"
	"github.com/xltxb/PetManage/internal/servicerecord"
	"github.com/xltxb/PetManage/internal/supplier"
	"github.com/xltxb/PetManage/internal/receipttemplate"
	"github.com/xltxb/PetManage/internal/shift"
	"github.com/xltxb/PetManage/internal/verification"
	"github.com/xltxb/PetManage/pkg/apperrors"
	cryptopkg "github.com/xltxb/PetManage/pkg/crypto"
	"github.com/xltxb/PetManage/pkg/logger"
	"go.uber.org/zap"
)

func main() {
	migrateFlag := flag.Bool("migrate", false, "Run database migrations")
	rollbackFlag := flag.Bool("rollback", false, "Rollback the most recent migration")
	statusFlag := flag.Bool("migrate-status", false, "Show migration status")
	flag.Parse()

	cfgPath := "config/config.yaml"
	if p := os.Getenv("CONFIG_PATH"); p != "" {
		cfgPath = p
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if *migrateFlag || *rollbackFlag || *statusFlag {
		handleMigration(cfg, *migrateFlag, *rollbackFlag, *statusFlag)
		return
	}

	lgr, err := logger.New(cfg.Log.Level)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer lgr.Sync()

	// Initialize encryption for sensitive data.
	if err := cryptopkg.Init(cfg.Encryption.Keys, cfg.Encryption.CurrentKeyVersion); err != nil {
		lgr.Fatal("Failed to initialize encryption", zap.Error(err))
	}

	// Connect to database.
	dsn := cfg.DSN()
	lgr.Info("Connecting to database...")
	db, err := database.Connect(dsn)
	if err != nil {
		lgr.Fatal("Database connection failed", zap.Error(err))
	}
	defer db.Close()

	// Initialize JWT manager.
	jwtManager := auth.NewJWTManager(
		cfg.JWT.Secret,
		cfg.JWT.AccessTokenTTL,
		cfg.JWT.RefreshTokenTTL,
	)

	// Initialize auth service.
	authService := auth.NewService(db, jwtManager)

	// Initialize merchant service.
	merchantService := merchant.NewService(db)

	// Initialize contract service.
	contractService := contract.NewService(db, "uploads/contracts")

	// Initialize dictionary service.
	dictService := dictionary.NewService(db)

	// Initialize role service and permission checker.
	roleService := role.NewService(db)
	permChecker := middleware.NewPermissionChecker(db)

	// Initialize announcement service.
	announcementService := announcement.NewService(db)

	// Initialize operation log service.
	opLogService := operationlog.New(db)

	// Initialize dashboard service.
	dashboardService := dashboard.NewService(db)

	// Initialize product service.
	productService := product.NewService(db)

	// Initialize checkout service.
	checkoutService := checkout.NewService(db)

	// Initialize report service.
	reportService := report.NewService(db)

	// Initialize risk control service.
	riskService := risk.NewService(db)

	// Initialize complaint service.
	complaintService := complaint.NewService(db)

	// Initialize category service.
	categoryService := category.NewService(db)

	// Initialize member service.
	memberService := member.NewService(db)
	member.SetQRCodeSecret(cfg.JWT.Secret)

	// Initialize member level service.
	memberLevelService := memberlevel.NewService(db)

	// Initialize supplier service.
	supplierService := supplier.NewService(db)

	// Initialize purchase service.
	purchaseService := purchase.NewService(db)

	// Initialize payable service.
	payableService := payable.NewService(db)

	// Initialize replenishment service.
	replenishmentService := replenishment.NewService(db)

	// Initialize inventory service.
	inventoryService := inventory.NewService(db)

	// Initialize service management service.
	serviceMgmtService := servicemgmt.NewService(db)

	// Initialize service package service.
	servicePackageService := servicepackage.NewService(db)

	// Initialize pet service.
	petService := pet.NewService(db)

	// Initialize employee service.
	employeeService := employee.NewService(db)

	// Initialize attendance service.
	attendanceService := attendance.NewService(db)

	// Initialize commission service.
	commissionService := commission.NewService(db)
		// Initialize fixed expense service.
		fixedExpenseService := fixedexpense.NewService(db)


	// Initialize revenue service.
	revenueService := revenue.NewService(db)

	// Initialize statement service.
	statementService := statement.NewService(db)

	// Initialize merchant role service.
	merchantRoleService := merchantrole.NewService(db)

	// Initialize balance (stored value) service.
	balanceService := balance.NewService(db)

	// Initialize points service.
	pointsService := points.NewService(db)

	// Initialize member tag service.
	memberTagService := membertag.NewService(db)

	// Initialize appointment service.
	appointmentService := appointment.NewService(db)
	notifService := notification.NewService(db)
	appointmentService.SetNotificationService(notifService)

	// Initialize schedule service.
	scheduleService := schedule.NewService(db)
	appointmentService.SetScheduleService(scheduleService)

	// Initialize service record service for archiving.
	serviceRecordService := servicerecord.NewService(db)
	reviewService := review.NewService(db)
	appointmentService.SetServiceRecordService(serviceRecordService)

	// Initialize orders service.
	ordersService := orders.NewService(db)

	// Initialize verification service.
	verificationService := verification.NewService(db)

	// Initialize receipt template service.
	receiptTemplateService := receipttemplate.NewService(db)

	// Initialize shift service.
	shiftService := shift.NewService(db)

	// Initialize coupon management service.
	couponService := coupon.NewService(db)

	// Initialize promotion service.
	promotionService := promotion.NewService(db)

		// Initialize service card management service.
		scService := servicecard.NewService(db)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/v1/auth/login", makeLoginHandler(authService))
	mux.HandleFunc("/api/v1/auth/refresh", makeRefreshHandler(authService))
	mux.HandleFunc("POST /api/v1/merchant/auth/login", makeMerchantLoginHandler(authService))
	mux.Handle("/api/v1/auth/change-password", middleware.Auth(jwtManager)(http.HandlerFunc(makeChangePasswordHandler(authService))))

	// Merchant routes (public).
	mux.HandleFunc("POST /api/v1/merchants/apply", makeMerchantApplyHandler(merchantService))
	mux.HandleFunc("GET /api/v1/merchants/apply/{id}", makeMerchantGetHandler(merchantService))

	// Merchant routes (platform-only, auth-protected).
	mux.Handle("GET /api/v1/merchants", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeMerchantListHandler(merchantService)))))
	mux.Handle("GET /api/v1/merchants/pending", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeMerchantPendingHandler(merchantService)))))
	mux.Handle("POST /api/v1/merchants/{id}/reject", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeMerchantRejectHandler(merchantService)))))
	mux.Handle("PUT /api/v1/merchants/{id}/apply", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeMerchantResubmitHandler(merchantService)))))

	// Merchant status control (platform-only, auth-protected).
	mux.Handle("POST /api/v1/merchants/{id}/freeze", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeMerchantFreezeHandler(merchantService)))))
	mux.Handle("POST /api/v1/merchants/{id}/unfreeze", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeMerchantUnfreezeHandler(merchantService)))))
	mux.Handle("POST /api/v1/merchants/{id}/close", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeMerchantCloseHandler(merchantService)))))
	mux.Handle("GET /api/v1/operation-logs/merchant/{id}", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeMerchantLogsHandler(merchantService)))))
	mux.Handle("GET /api/v1/operation-logs", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeOperationLogsHandler(opLogService)))))

	// Dashboard routes (platform-only, auth-protected).
	mux.Handle("GET /api/v1/dashboard/overview", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeDashboardOverviewHandler(dashboardService)))))
	mux.Handle("GET /api/v1/dashboard/merchant/{id}/analysis", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeMerchantAnalysisHandler(dashboardService)))))
	mux.Handle("GET /api/v1/dashboard/merchants/ranking", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeMerchantsRankingHandler(dashboardService)))))

	// Merchant dashboard routes (auth-protected).
	mux.Handle("GET /api/v1/merchant/dashboard", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantDashboardHandler(merchantService, petService))))

	// Merchant shop settings (auth-protected, merchant-only).
	mux.Handle("GET /api/v1/merchant/shop-settings", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantShopSettingsGetHandler(merchantService))))
	mux.Handle("PUT /api/v1/merchant/shop-settings", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantShopSettingsUpdateHandler(merchantService))))
	mux.Handle("POST /api/v1/merchant/shop-settings/logo", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantShopSettingsLogoHandler(merchantService))))

	// Product management (auth-protected, merchant-only).
	mux.Handle("POST /api/v1/merchant/products", middleware.Auth(jwtManager)(http.HandlerFunc(makeProductCreateHandler(productService))))
	mux.Handle("GET /api/v1/merchant/products", middleware.Auth(jwtManager)(http.HandlerFunc(makeProductListHandler(productService))))
	mux.Handle("POST /api/v1/merchant/products/{id}/toggle-status", middleware.Auth(jwtManager)(http.HandlerFunc(makeProductToggleStatusHandler(productService))))
	mux.Handle("GET /api/v1/merchant/products/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeProductGetHandler(productService))))
	mux.Handle("PUT /api/v1/merchant/products/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeProductUpdateHandler(productService))))
	mux.Handle("DELETE /api/v1/merchant/products/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeProductDeleteHandler(productService))))

	// SKU management (auth-protected, merchant-only).
	mux.Handle("POST /api/v1/merchant/products/{id}/skus", middleware.Auth(jwtManager)(http.HandlerFunc(makeSkuCreateHandler(productService))))
	mux.Handle("GET /api/v1/merchant/products/{id}/skus", middleware.Auth(jwtManager)(http.HandlerFunc(makeSkuListHandler(productService))))
	mux.Handle("GET /api/v1/merchant/products/{id}/skus/{skuId}", middleware.Auth(jwtManager)(http.HandlerFunc(makeSkuGetHandler(productService))))
	mux.Handle("PUT /api/v1/merchant/products/{id}/skus/{skuId}", middleware.Auth(jwtManager)(http.HandlerFunc(makeSkuUpdateHandler(productService))))
	mux.Handle("DELETE /api/v1/merchant/products/{id}/skus/{skuId}", middleware.Auth(jwtManager)(http.HandlerFunc(makeSkuDeleteHandler(productService))))
	mux.Handle("POST /api/v1/merchant/products/{id}/skus/{skuId}/toggle-status", middleware.Auth(jwtManager)(http.HandlerFunc(makeSkuToggleStatusHandler(productService))))

	// Category management (auth-protected, merchant-only).
		mux.Handle("POST /api/v1/merchant/categories", middleware.Auth(jwtManager)(http.HandlerFunc(makeCategoryCreateHandler(categoryService))))
		mux.Handle("GET /api/v1/merchant/categories", middleware.Auth(jwtManager)(http.HandlerFunc(makeCategoryListHandler(categoryService))))
		mux.Handle("PUT /api/v1/merchant/categories/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeCategoryUpdateHandler(categoryService))))
		mux.Handle("DELETE /api/v1/merchant/categories/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeCategoryDeleteHandler(categoryService))))
	// Supplier management (auth-protected, merchant-only).
	mux.Handle("POST /api/v1/merchant/suppliers", middleware.Auth(jwtManager)(http.HandlerFunc(makeSupplierCreateHandler(supplierService))))
	mux.Handle("GET /api/v1/merchant/suppliers", middleware.Auth(jwtManager)(http.HandlerFunc(makeSupplierListHandler(supplierService))))
	mux.Handle("GET /api/v1/merchant/suppliers/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeSupplierGetHandler(supplierService))))
	mux.Handle("PUT /api/v1/merchant/suppliers/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeSupplierUpdateHandler(supplierService))))
	mux.Handle("POST /api/v1/merchant/suppliers/{id}/toggle-status", middleware.Auth(jwtManager)(http.HandlerFunc(makeSupplierToggleStatusHandler(supplierService))))
	mux.Handle("POST /api/v1/merchant/suppliers/{id}/products", middleware.Auth(jwtManager)(http.HandlerFunc(makeSupplierLinkProductHandler(supplierService))))
	mux.Handle("DELETE /api/v1/merchant/suppliers/{id}/products/{productId}", middleware.Auth(jwtManager)(http.HandlerFunc(makeSupplierUnlinkProductHandler(supplierService))))
	// Purchase order management (auth-protected, merchant-only).
	mux.Handle("POST /api/v1/merchant/purchases", middleware.Auth(jwtManager)(http.HandlerFunc(makePurchaseCreateHandler(purchaseService))))
	mux.Handle("GET /api/v1/merchant/purchases", middleware.Auth(jwtManager)(http.HandlerFunc(makePurchaseListHandler(purchaseService))))
	mux.Handle("GET /api/v1/merchant/purchases/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makePurchaseGetHandler(purchaseService))))
	mux.Handle("PUT /api/v1/merchant/purchases/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makePurchaseUpdateHandler(purchaseService))))
	mux.Handle("POST /api/v1/merchant/purchases/{id}/submit", middleware.Auth(jwtManager)(http.HandlerFunc(makePurchaseSubmitHandler(purchaseService))))
	mux.Handle("POST /api/v1/merchant/purchases/{id}/confirm", middleware.Auth(jwtManager)(http.HandlerFunc(makePurchaseConfirmHandler(purchaseService))))
	mux.Handle("POST /api/v1/merchant/purchases/{id}/receive", middleware.Auth(jwtManager)(http.HandlerFunc(makePurchaseReceiveHandler(purchaseService, payableService))))
	mux.Handle("POST /api/v1/merchant/purchases/{id}/void", middleware.Auth(jwtManager)(http.HandlerFunc(makePurchaseVoidHandler(purchaseService))))
		// Accounts payable management (auth-protected, merchant-only).
		mux.Handle("GET /api/v1/merchant/payables", middleware.Auth(jwtManager)(http.HandlerFunc(makePayableListHandler(payableService))))
		mux.Handle("GET /api/v1/merchant/payables/suppliers", middleware.Auth(jwtManager)(http.HandlerFunc(makePayableSupplierSummaryHandler(payableService))))
		mux.Handle("GET /api/v1/merchant/payables/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makePayableGetHandler(payableService))))
		mux.Handle("POST /api/v1/merchant/payables/{id}/payments", middleware.Auth(jwtManager)(http.HandlerFunc(makePayableRegisterPaymentHandler(payableService))))
		mux.Handle("GET /api/v1/merchant/payables/statement", middleware.Auth(jwtManager)(http.HandlerFunc(makePayableStatementHandler(payableService))))
		mux.Handle("GET /api/v1/merchant/payables/statement/export", middleware.Auth(jwtManager)(http.HandlerFunc(makePayableStatementExportHandler(payableService))))
		// Replenishment suggestions (auth-protected, merchant-only).
		mux.Handle("GET /api/v1/merchant/replenishment/suggestions", middleware.Auth(jwtManager)(http.HandlerFunc(makeReplenishSuggestionsHandler(replenishmentService))))
		mux.Handle("POST /api/v1/merchant/replenishment/generate-po", middleware.Auth(jwtManager)(http.HandlerFunc(makeReplenishGeneratePOHandler(replenishmentService, purchaseService))))

		// Inventory warehouse management (auth-protected, merchant-only).
		mux.Handle("POST /api/v1/merchant/inventory/warehouses", middleware.Auth(jwtManager)(http.HandlerFunc(makeInventoryWarehouseCreateHandler(inventoryService))))
		mux.Handle("GET /api/v1/merchant/inventory/warehouses", middleware.Auth(jwtManager)(http.HandlerFunc(makeInventoryWarehouseListHandler(inventoryService))))

		// Inventory stock operations (auth-protected, merchant-only).
		mux.Handle("POST /api/v1/merchant/inventory/inbound", middleware.Auth(jwtManager)(http.HandlerFunc(makeInventoryInboundHandler(inventoryService))))
		mux.Handle("POST /api/v1/merchant/inventory/outbound", middleware.Auth(jwtManager)(http.HandlerFunc(makeInventoryOutboundHandler(inventoryService))))
		mux.Handle("POST /api/v1/merchant/inventory/transfer", middleware.Auth(jwtManager)(http.HandlerFunc(makeInventoryTransferHandler(inventoryService))))
		mux.Handle("POST /api/v1/merchant/inventory/loss", middleware.Auth(jwtManager)(http.HandlerFunc(makeInventoryLossHandler(inventoryService))))
		mux.Handle("POST /api/v1/merchant/inventory/surplus", middleware.Auth(jwtManager)(http.HandlerFunc(makeInventorySurplusHandler(inventoryService))))
		mux.Handle("GET /api/v1/merchant/inventory/flows", middleware.Auth(jwtManager)(http.HandlerFunc(makeInventoryFlowsHandler(inventoryService))))

		// Inventory alerts (auth-protected, merchant-only).
		mux.Handle("GET /api/v1/merchant/inventory/alerts", middleware.Auth(jwtManager)(http.HandlerFunc(makeInventoryAlertsHandler(inventoryService))))

		// Inventory count checks (auth-protected, merchant-only).
		mux.Handle("POST /api/v1/merchant/inventory/checks", middleware.Auth(jwtManager)(http.HandlerFunc(makeInventoryCheckCreateHandler(inventoryService))))
		mux.Handle("GET /api/v1/merchant/inventory/checks", middleware.Auth(jwtManager)(http.HandlerFunc(makeInventoryCheckListHandler(inventoryService))))
		mux.Handle("GET /api/v1/merchant/inventory/checks/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeInventoryCheckGetHandler(inventoryService))))
		mux.Handle("PUT /api/v1/merchant/inventory/checks/{id}/items/{itemId}", middleware.Auth(jwtManager)(http.HandlerFunc(makeInventoryCheckUpdateItemHandler(inventoryService))))
		mux.Handle("POST /api/v1/merchant/inventory/checks/{id}/submit", middleware.Auth(jwtManager)(http.HandlerFunc(makeInventoryCheckSubmitHandler(inventoryService))))
		mux.Handle("POST /api/v1/merchant/inventory/checks/{id}/confirm", middleware.Auth(jwtManager)(http.HandlerFunc(makeInventoryCheckConfirmHandler(inventoryService))))
		mux.Handle("POST /api/v1/merchant/inventory/checks/{id}/approve", middleware.Auth(jwtManager)(http.HandlerFunc(makeInventoryCheckApproveHandler(inventoryService))))

		// Service management (auth-protected, merchant-only).
	mux.Handle("POST /api/v1/merchant/service-categories", middleware.Auth(jwtManager)(http.HandlerFunc(makeServiceCategoryCreateHandler(serviceMgmtService))))
	mux.Handle("GET /api/v1/merchant/service-categories", middleware.Auth(jwtManager)(http.HandlerFunc(makeServiceCategoryListHandler(serviceMgmtService))))
	mux.Handle("PUT /api/v1/merchant/service-categories/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeServiceCategoryUpdateHandler(serviceMgmtService))))
	mux.Handle("DELETE /api/v1/merchant/service-categories/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeServiceCategoryDeleteHandler(serviceMgmtService))))
	mux.Handle("POST /api/v1/merchant/service-items", middleware.Auth(jwtManager)(http.HandlerFunc(makeServiceItemCreateHandler(serviceMgmtService))))
	mux.Handle("GET /api/v1/merchant/service-items", middleware.Auth(jwtManager)(http.HandlerFunc(makeServiceItemListHandler(serviceMgmtService))))
	mux.Handle("GET /api/v1/merchant/service-items/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeServiceItemGetHandler(serviceMgmtService))))
	mux.Handle("PUT /api/v1/merchant/service-items/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeServiceItemUpdateHandler(serviceMgmtService))))
	mux.Handle("DELETE /api/v1/merchant/service-items/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeServiceItemDeleteHandler(serviceMgmtService))))
	mux.Handle("POST /api/v1/merchant/service-items/{id}/toggle-status", middleware.Auth(jwtManager)(http.HandlerFunc(makeServiceItemToggleStatusHandler(serviceMgmtService))))
		// Service package management (auth-protected, merchant-only).
		mux.Handle("POST /api/v1/merchant/service-packages", middleware.Auth(jwtManager)(http.HandlerFunc(makePackageCreateHandler(servicePackageService))))
		mux.Handle("GET /api/v1/merchant/service-packages", middleware.Auth(jwtManager)(http.HandlerFunc(makePackageListHandler(servicePackageService))))
		mux.Handle("GET /api/v1/merchant/service-packages/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makePackageGetHandler(servicePackageService))))
		mux.Handle("PUT /api/v1/merchant/service-packages/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makePackageUpdateHandler(servicePackageService))))
		mux.Handle("DELETE /api/v1/merchant/service-packages/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makePackageDeleteHandler(servicePackageService))))
		mux.Handle("POST /api/v1/merchant/service-packages/{id}/toggle", middleware.Auth(jwtManager)(http.HandlerFunc(makePackageToggleHandler(servicePackageService))))
		mux.Handle("GET /api/v1/merchant/service-packages/{id}/items", middleware.Auth(jwtManager)(http.HandlerFunc(makePackageItemsHandler(servicePackageService))))
		// Member management (auth + merchant permission).
		mux.Handle("POST /api/v1/merchant/members",
			middleware.Auth(jwtManager)(
				permChecker.RequireMerchantPermission("member:manage")(
					http.HandlerFunc(makeMemberCreateHandler(memberService)),
				),
			),
		)
		mux.Handle("GET /api/v1/merchant/members",
			middleware.Auth(jwtManager)(
				permChecker.RequireMerchantPermission("member:view")(
					http.HandlerFunc(makeMemberListHandler(memberService)),
				),
			),
		)
		mux.Handle("GET /api/v1/merchant/members/search",
			middleware.Auth(jwtManager)(
				permChecker.RequireMerchantPermission("member:view")(
					http.HandlerFunc(makeMemberSearchHandler(memberService)),
				),
			),
		)
		mux.Handle("GET /api/v1/merchant/members/{id}",
			middleware.Auth(jwtManager)(
				permChecker.RequireMerchantPermission("member:view")(
					http.HandlerFunc(makeMemberGetHandler(memberService)),
				),
			),
		)
		mux.Handle("PUT /api/v1/merchant/members/{id}",
			middleware.Auth(jwtManager)(
				permChecker.RequireMerchantPermission("member:manage")(
					http.HandlerFunc(makeMemberUpdateHandler(memberService)),
				),
			),
		)
		mux.Handle("POST /api/v1/merchant/members/{id}/toggle-status",
			middleware.Auth(jwtManager)(
				permChecker.RequireMerchantPermission("member:manage")(
					http.HandlerFunc(makeMemberToggleStatusHandler(memberService)),
				),
			),
		)
		mux.Handle("POST /api/v1/merchant/members/batch-import",
			middleware.Auth(jwtManager)(
				permChecker.RequireMerchantPermission("member:manage")(
					http.HandlerFunc(makeMemberBatchImportHandler(memberService)),
				),
			),
		)

	// Member QR code (auth-protected, merchant-only).
	mux.Handle("GET /api/v1/merchant/members/{id}/qrcode", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberQRCodeHandler(memberService))))
	mux.Handle("GET /api/v1/merchant/members/qrcode/scan", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberQRCodeScanHandler(memberService))))

	// Member tag management (auth-protected, merchant-only).
	mux.Handle("POST /api/v1/merchant/tags", middleware.Auth(jwtManager)(http.HandlerFunc(makeTagCreateHandler(memberTagService))))
	mux.Handle("GET /api/v1/merchant/tags", middleware.Auth(jwtManager)(http.HandlerFunc(makeTagListHandler(memberTagService))))
	mux.Handle("GET /api/v1/merchant/tags/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeTagGetHandler(memberTagService))))
	mux.Handle("PUT /api/v1/merchant/tags/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeTagUpdateHandler(memberTagService))))
	mux.Handle("DELETE /api/v1/merchant/tags/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeTagDeleteHandler(memberTagService))))
	mux.Handle("POST /api/v1/merchant/tags/{id}/toggle", middleware.Auth(jwtManager)(http.HandlerFunc(makeTagToggleHandler(memberTagService))))

	// Member tag relations.
	mux.Handle("POST /api/v1/merchant/members/{id}/tags", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberAddTagsHandler(memberTagService))))
	mux.Handle("DELETE /api/v1/merchant/members/{id}/tags/{tagId}", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberRemoveTagHandler(memberTagService))))
	mux.Handle("GET /api/v1/merchant/members/{id}/tags", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberTagsHandler(memberTagService))))

	// Auto-tag rules.
	mux.Handle("POST /api/v1/merchant/tags/rules", middleware.Auth(jwtManager)(http.HandlerFunc(makeTagRuleCreateHandler(memberTagService))))
	mux.Handle("GET /api/v1/merchant/tags/rules", middleware.Auth(jwtManager)(http.HandlerFunc(makeTagRuleListHandler(memberTagService))))
	mux.Handle("GET /api/v1/merchant/tags/rules/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeTagRuleGetHandler(memberTagService))))
	mux.Handle("PUT /api/v1/merchant/tags/rules/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeTagRuleUpdateHandler(memberTagService))))
	mux.Handle("DELETE /api/v1/merchant/tags/rules/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeTagRuleDeleteHandler(memberTagService))))
	mux.Handle("POST /api/v1/merchant/tags/rules/{id}/toggle", middleware.Auth(jwtManager)(http.HandlerFunc(makeTagRuleToggleHandler(memberTagService))))
	mux.Handle("POST /api/v1/merchant/members/{id}/check-tags", middleware.Auth(jwtManager)(http.HandlerFunc(makeTagCheckApplyHandler(memberTagService))))

		// Member level management (auth-protected, merchant-only).
		mux.Handle("POST /api/v1/merchant/member-levels", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberLevelCreateHandler(memberLevelService))))
		mux.Handle("GET /api/v1/merchant/member-levels", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberLevelListHandler(memberLevelService))))
		mux.Handle("GET /api/v1/merchant/member-levels/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberLevelGetHandler(memberLevelService))))
		mux.Handle("PUT /api/v1/merchant/member-levels/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberLevelUpdateHandler(memberLevelService))))
		mux.Handle("DELETE /api/v1/merchant/member-levels/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberLevelDeleteHandler(memberLevelService))))
		mux.Handle("POST /api/v1/merchant/member-levels/{id}/toggle", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberLevelToggleHandler(memberLevelService))))
		mux.Handle("GET /api/v1/merchant/members/{id}/level", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberLevelInfoHandler(memberLevelService))))
		mux.Handle("GET /api/v1/merchant/members/{id}/level-logs", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberLevelLogsHandler(memberLevelService))))
		mux.Handle("POST /api/v1/merchant/members/{id}/check-upgrade", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberLevelCheckUpgradeHandler(memberLevelService))))

		// Stored value management (auth-protected, merchant-only).
		mux.Handle("POST /api/v1/merchant/balance/packages", middleware.Auth(jwtManager)(http.HandlerFunc(makeBalancePackageCreateHandler(balanceService))))
		mux.Handle("GET /api/v1/merchant/balance/packages", middleware.Auth(jwtManager)(http.HandlerFunc(makeBalancePackageListHandler(balanceService))))
		mux.Handle("GET /api/v1/merchant/balance/packages/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeBalancePackageGetHandler(balanceService))))
		mux.Handle("PUT /api/v1/merchant/balance/packages/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeBalancePackageUpdateHandler(balanceService))))
		mux.Handle("DELETE /api/v1/merchant/balance/packages/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeBalancePackageDeleteHandler(balanceService))))
		mux.Handle("POST /api/v1/merchant/balance/packages/{id}/toggle", middleware.Auth(jwtManager)(http.HandlerFunc(makeBalancePackageToggleHandler(balanceService))))
		mux.Handle("POST /api/v1/merchant/members/{id}/recharge", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberRechargeHandler(balanceService))))
		mux.Handle("GET /api/v1/merchant/members/{id}/balance", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberBalanceHandler(balanceService))))
		mux.Handle("GET /api/v1/merchant/members/{id}/balance-transactions", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberBalanceTransactionsHandler(balanceService))))


		// Points management (auth-protected, merchant-only).
		mux.Handle("POST /api/v1/merchant/points/rules", middleware.Auth(jwtManager)(http.HandlerFunc(makePointsRuleCreateHandler(pointsService))))
		mux.Handle("GET /api/v1/merchant/points/rules", middleware.Auth(jwtManager)(http.HandlerFunc(makePointsRuleListHandler(pointsService))))
		mux.Handle("GET /api/v1/merchant/points/rules/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makePointsRuleGetHandler(pointsService))))
		mux.Handle("PUT /api/v1/merchant/points/rules/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makePointsRuleUpdateHandler(pointsService))))
		mux.Handle("DELETE /api/v1/merchant/points/rules/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makePointsRuleDeleteHandler(pointsService))))
		mux.Handle("POST /api/v1/merchant/points/rules/{id}/toggle", middleware.Auth(jwtManager)(http.HandlerFunc(makePointsRuleToggleHandler(pointsService))))
		mux.Handle("GET /api/v1/merchant/members/{id}/points/transactions", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberPointTransactionsHandler(pointsService))))
		mux.Handle("GET /api/v1/merchant/points/expiry-alerts", middleware.Auth(jwtManager)(http.HandlerFunc(makePointsExpiryAlertsHandler(pointsService))))
	// Pet management (auth-protected, merchant-only).
	mux.Handle("POST /api/v1/merchant/members/{id}/pets", middleware.Auth(jwtManager)(http.HandlerFunc(makePetCreateHandler(petService))))
	mux.Handle("GET /api/v1/merchant/members/{id}/pets", middleware.Auth(jwtManager)(http.HandlerFunc(makePetListHandler(petService))))
	mux.Handle("GET /api/v1/merchant/members/{id}/pets/{petId}", middleware.Auth(jwtManager)(http.HandlerFunc(makePetGetHandler(petService))))
	mux.Handle("PUT /api/v1/merchant/members/{id}/pets/{petId}", middleware.Auth(jwtManager)(http.HandlerFunc(makePetUpdateHandler(petService))))
	mux.Handle("DELETE /api/v1/merchant/members/{id}/pets/{petId}", middleware.Auth(jwtManager)(http.HandlerFunc(makePetDeleteHandler(petService))))
	// Pet health reminders (auth-protected, merchant-only).
	mux.Handle("GET /api/v1/merchant/pets/health-reminders", middleware.Auth(jwtManager)(http.HandlerFunc(makeHealthRemindersHandler(petService))))
	mux.Handle("GET /api/v1/merchant/pets/health-reminders/count", middleware.Auth(jwtManager)(http.HandlerFunc(makeHealthReminderCountHandler(petService))))

	// Employee management (auth-protected, merchant-only).
	mux.Handle("POST /api/v1/merchant/employees", middleware.Auth(jwtManager)(http.HandlerFunc(makeEmployeeCreateHandler(employeeService))))
	mux.Handle("GET /api/v1/merchant/employees", middleware.Auth(jwtManager)(http.HandlerFunc(makeEmployeeListHandler(employeeService))))
	mux.Handle("GET /api/v1/merchant/employees/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeEmployeeGetHandler(employeeService))))
	mux.Handle("PUT /api/v1/merchant/employees/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeEmployeeUpdateHandler(employeeService))))
	mux.Handle("POST /api/v1/merchant/employees/{id}/resign", middleware.Auth(jwtManager)(http.HandlerFunc(makeEmployeeResignHandler(employeeService))))
	mux.Handle("POST /api/v1/merchant/employees/{id}/toggle-status", middleware.Auth(jwtManager)(http.HandlerFunc(makeEmployeeToggleStatusHandler(employeeService))))
		mux.Handle("POST /api/v1/merchant/employees/{id}/create-account", middleware.Auth(jwtManager)(http.HandlerFunc(makeEmployeeCreateAccountHandler(merchantRoleService))))
		mux.Handle("POST /api/v1/merchant/employees/{id}/assign-role", middleware.Auth(jwtManager)(http.HandlerFunc(makeEmployeeAssignRoleHandler(merchantRoleService))))
		mux.Handle("POST /api/v1/merchant/employees/{id}/disable-account", middleware.Auth(jwtManager)(http.HandlerFunc(makeEmployeeDisableAccountHandler(merchantRoleService))))

		// Attendance management (auth-protected, merchant-only).
		mux.Handle("POST /api/v1/merchant/attendance/check-in", middleware.Auth(jwtManager)(http.HandlerFunc(makeAttendanceCheckInHandler(attendanceService))))
		mux.Handle("POST /api/v1/merchant/attendance/check-out", middleware.Auth(jwtManager)(http.HandlerFunc(makeAttendanceCheckOutHandler(attendanceService))))
		mux.Handle("GET /api/v1/merchant/attendance/today", middleware.Auth(jwtManager)(http.HandlerFunc(makeAttendanceTodayHandler(attendanceService))))
		mux.Handle("POST /api/v1/merchant/attendance/leave", middleware.Auth(jwtManager)(http.HandlerFunc(makeLeaveApplyHandler(attendanceService))))
		mux.Handle("GET /api/v1/merchant/attendance/leaves", middleware.Auth(jwtManager)(http.HandlerFunc(makeLeaveListHandler(attendanceService))))
		mux.Handle("PUT /api/v1/merchant/attendance/leaves/{id}/review", middleware.Auth(jwtManager)(http.HandlerFunc(makeLeaveReviewHandler(attendanceService))))
		mux.Handle("POST /api/v1/merchant/attendance/overtime", middleware.Auth(jwtManager)(http.HandlerFunc(makeOvertimeApplyHandler(attendanceService))))
		mux.Handle("GET /api/v1/merchant/attendance/overtime", middleware.Auth(jwtManager)(http.HandlerFunc(makeOvertimeListHandler(attendanceService))))
		mux.Handle("PUT /api/v1/merchant/attendance/overtime/{id}/review", middleware.Auth(jwtManager)(http.HandlerFunc(makeOvertimeReviewHandler(attendanceService))))
		mux.Handle("GET /api/v1/merchant/attendance/stats", middleware.Auth(jwtManager)(http.HandlerFunc(makeAttendanceStatsHandler(attendanceService))))
			// Commission management (auth-protected, merchant-only).
			mux.Handle("POST /api/v1/merchant/fixed-expenses", middleware.Auth(jwtManager)(http.HandlerFunc(makeCreateFixedExpenseHandler(fixedExpenseService))))
			mux.Handle("GET /api/v1/merchant/fixed-expenses", middleware.Auth(jwtManager)(http.HandlerFunc(makeListFixedExpensesHandler(fixedExpenseService))))
			mux.Handle("PUT /api/v1/merchant/fixed-expenses/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeUpdateFixedExpenseHandler(fixedExpenseService))))
			mux.Handle("DELETE /api/v1/merchant/fixed-expenses/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeDeleteFixedExpenseHandler(fixedExpenseService))))

			mux.Handle("GET /api/v1/merchant/commission/rules", middleware.Auth(jwtManager)(http.HandlerFunc(makeCommissionRulesGetHandler(commissionService))))
			mux.Handle("PUT /api/v1/merchant/commission/rules", middleware.Auth(jwtManager)(http.HandlerFunc(makeCommissionRulesUpdateHandler(commissionService))))
			mux.Handle("POST /api/v1/merchant/commission/assign", middleware.Auth(jwtManager)(http.HandlerFunc(makeCommissionAssignHandler(commissionService))))
			mux.Handle("GET /api/v1/merchant/commission/records", middleware.Auth(jwtManager)(http.HandlerFunc(makeCommissionRecordsHandler(commissionService))))
			mux.Handle("GET /api/v1/merchant/commission/summary", middleware.Auth(jwtManager)(http.HandlerFunc(makeCommissionSummaryHandler(commissionService))))
			mux.Handle("POST /api/v1/merchant/commission/deduct", middleware.Auth(jwtManager)(http.HandlerFunc(makeCommissionDeductHandler(commissionService))))

			// Revenue statistics and income/expense details (auth-protected, merchant-only).
			mux.Handle("GET /api/v1/merchant/revenue/summary", middleware.Auth(jwtManager)(http.HandlerFunc(makeRevenueSummaryHandler(revenueService))))
			mux.Handle("GET /api/v1/merchant/revenue/transactions", middleware.Auth(jwtManager)(http.HandlerFunc(makeRevenueTransactionsHandler(revenueService))))

			// Financial statements (auth-protected, merchant-only).
			mux.Handle("GET /api/v1/merchant/statements/profit-loss", middleware.Auth(jwtManager)(http.HandlerFunc(makeProfitLossHandler(statementService))))
			mux.Handle("GET /api/v1/merchant/statements/revenue-detail", middleware.Auth(jwtManager)(http.HandlerFunc(makeRevenueDetailHandler(statementService))))
			mux.Handle("GET /api/v1/merchant/statements/product-sales", middleware.Auth(jwtManager)(http.HandlerFunc(makeProductSalesHandler(statementService))))
			mux.Handle("GET /api/v1/merchant/statements/service-performance", middleware.Auth(jwtManager)(http.HandlerFunc(makeServicePerformanceHandler(statementService))))

			// Financial statement exports (auth + report:view permission, merchant-only).
			stmtExport := middleware.Auth(jwtManager)(permChecker.RequireMerchantPermission("report:view")(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))
			_ = stmtExport
			mux.Handle("GET /api/v1/merchant/statements/profit-loss/excel", middleware.Auth(jwtManager)(permChecker.RequireMerchantPermission("report:view")(http.HandlerFunc(makeProfitLossExcelHandler(statementService)))))
			mux.Handle("GET /api/v1/merchant/statements/profit-loss/pdf", middleware.Auth(jwtManager)(permChecker.RequireMerchantPermission("report:view")(http.HandlerFunc(makeProfitLossPDFHandler(statementService)))))
			mux.Handle("GET /api/v1/merchant/statements/revenue-detail/excel", middleware.Auth(jwtManager)(permChecker.RequireMerchantPermission("report:view")(http.HandlerFunc(makeRevenueDetailExcelHandler(statementService)))))
			mux.Handle("GET /api/v1/merchant/statements/revenue-detail/pdf", middleware.Auth(jwtManager)(permChecker.RequireMerchantPermission("report:view")(http.HandlerFunc(makeRevenueDetailPDFHandler(statementService)))))
			mux.Handle("GET /api/v1/merchant/statements/product-sales/excel", middleware.Auth(jwtManager)(permChecker.RequireMerchantPermission("report:view")(http.HandlerFunc(makeProductSalesExcelHandler(statementService)))))
			mux.Handle("GET /api/v1/merchant/statements/product-sales/pdf", middleware.Auth(jwtManager)(permChecker.RequireMerchantPermission("report:view")(http.HandlerFunc(makeProductSalesPDFHandler(statementService)))))
			mux.Handle("GET /api/v1/merchant/statements/service-performance/excel", middleware.Auth(jwtManager)(permChecker.RequireMerchantPermission("report:view")(http.HandlerFunc(makeServicePerformanceExcelHandler(statementService)))))
			mux.Handle("GET /api/v1/merchant/statements/service-performance/pdf", middleware.Auth(jwtManager)(permChecker.RequireMerchantPermission("report:view")(http.HandlerFunc(makeServicePerformancePDFHandler(statementService)))))

		// Appointment management (auth-protected, merchant-only).
		mux.Handle("POST /api/v1/merchant/appointments", middleware.Auth(jwtManager)(http.HandlerFunc(makeAppointmentCreateHandler(appointmentService))))
		mux.Handle("GET /api/v1/merchant/appointments", middleware.Auth(jwtManager)(http.HandlerFunc(makeAppointmentListHandler(appointmentService))))
		mux.Handle("GET /api/v1/merchant/appointments/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeAppointmentGetHandler(appointmentService))))
		mux.Handle("POST /api/v1/merchant/appointments/{id}/confirm", middleware.Auth(jwtManager)(http.HandlerFunc(makeAppointmentConfirmHandler(appointmentService))))
		mux.Handle("POST /api/v1/merchant/appointments/{id}/reschedule", middleware.Auth(jwtManager)(http.HandlerFunc(makeAppointmentRescheduleHandler(appointmentService))))
		mux.Handle("POST /api/v1/merchant/appointments/{id}/cancel", middleware.Auth(jwtManager)(http.HandlerFunc(makeAppointmentCancelHandler(appointmentService))))
		mux.Handle("POST /api/v1/merchant/appointments/{id}/arrive", middleware.Auth(jwtManager)(http.HandlerFunc(makeAppointmentArriveHandler(appointmentService))))
		mux.Handle("POST /api/v1/merchant/appointments/{id}/start", middleware.Auth(jwtManager)(http.HandlerFunc(makeAppointmentStartHandler(appointmentService))))
		mux.Handle("POST /api/v1/merchant/appointments/{id}/complete", middleware.Auth(jwtManager)(http.HandlerFunc(makeAppointmentCompleteHandler(appointmentService))))
		mux.Handle("POST /api/v1/merchant/appointments/{id}/pickup", middleware.Auth(jwtManager)(http.HandlerFunc(makeAppointmentPickupHandler(appointmentService))))
		mux.Handle("GET /api/v1/merchant/appointments/{id}/change-logs", middleware.Auth(jwtManager)(http.HandlerFunc(makeAppointmentChangeLogsHandler(appointmentService))))

			// Service record archiving (auth-protected, merchant-only).
			mux.Handle("GET /api/v1/merchant/service-records/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeServiceRecordGetHandler(serviceRecordService))))
			mux.Handle("GET /api/v1/merchant/pets/{petId}/service-records", middleware.Auth(jwtManager)(http.HandlerFunc(makePetServiceRecordsHandler(serviceRecordService))))
			mux.Handle("GET /api/v1/merchant/members/{memberId}/service-records", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberServiceRecordsHandler(serviceRecordService))))
			mux.Handle("POST /api/v1/merchant/service-records/{id}/evaluate", middleware.Auth(jwtManager)(http.HandlerFunc(makeServiceRecordEvaluateHandler(serviceRecordService))))

		// Review management (auth-protected, merchant-only).
		mux.Handle("GET /api/v1/merchant/reviews", middleware.Auth(jwtManager)(http.HandlerFunc(makeReviewListHandler(reviewService))))
		mux.Handle("GET /api/v1/merchant/reviews/stats", middleware.Auth(jwtManager)(http.HandlerFunc(makeReviewStatsHandler(reviewService))))
		mux.Handle("GET /api/v1/merchant/reviews/employee-stats", middleware.Auth(jwtManager)(http.HandlerFunc(makeReviewEmployeeStatsHandler(reviewService))))
		mux.Handle("GET /api/v1/merchant/reviews/{type}/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeReviewGetHandler(reviewService))))
		mux.Handle("POST /api/v1/merchant/reviews/{type}/{id}/reply", middleware.Auth(jwtManager)(http.HandlerFunc(makeReviewReplyHandler(reviewService))))

		// Notification management (auth-protected, merchant-only).
		mux.Handle("GET /api/v1/merchant/notifications", middleware.Auth(jwtManager)(http.HandlerFunc(makeNotificationListHandler(notifService))))
		mux.Handle("POST /api/v1/merchant/notifications/upcoming-reminders", middleware.Auth(jwtManager)(http.HandlerFunc(makeNotificationSendUpcomingHandler(notifService))))
		mux.Handle("POST /api/v1/merchant/notifications/{id}/mark-read", middleware.Auth(jwtManager)(http.HandlerFunc(makeNotificationMarkReadHandler(notifService))))

		// Notification settings and templates (auth-protected, merchant-only).
		mux.Handle("GET /api/v1/merchant/notification/settings", middleware.Auth(jwtManager)(http.HandlerFunc(makeNotificationSettingsGetHandler(notifService))))
		mux.Handle("PUT /api/v1/merchant/notification/settings", middleware.Auth(jwtManager)(http.HandlerFunc(makeNotificationSettingsUpdateHandler(notifService))))
		mux.Handle("GET /api/v1/merchant/notification/templates", middleware.Auth(jwtManager)(http.HandlerFunc(makeNotificationTemplatesGetHandler(notifService))))
		mux.Handle("PUT /api/v1/merchant/notification/templates", middleware.Auth(jwtManager)(http.HandlerFunc(makeNotificationTemplatesUpdateHandler(notifService))))

		// Notification triggers (auth-protected).
		mux.Handle("POST /api/v1/merchant/notification/birthday", middleware.Auth(jwtManager)(http.HandlerFunc(makeNotificationBirthdayHandler(notifService))))
		mux.Handle("POST /api/v1/merchant/notification/inventory-alert", middleware.Auth(jwtManager)(http.HandlerFunc(makeNotificationInventoryAlertHandler(notifService))))
		mux.Handle("GET /api/v1/merchant/notification/send-records", middleware.Auth(jwtManager)(http.HandlerFunc(makeNotificationSendRecordsHandler(notifService))))

		// Schedule management (auth-protected, merchant-only).
		mux.Handle("GET /api/v1/merchant/schedules", middleware.Auth(jwtManager)(http.HandlerFunc(makeScheduleListHandler(scheduleService))))
		mux.Handle("PUT /api/v1/merchant/schedules", middleware.Auth(jwtManager)(http.HandlerFunc(makeScheduleUpsertHandler(scheduleService))))
		mux.Handle("POST /api/v1/merchant/schedules/batch", middleware.Auth(jwtManager)(http.HandlerFunc(makeScheduleBatchSetHandler(scheduleService))))
		mux.Handle("POST /api/v1/merchant/schedules/copy-week", middleware.Auth(jwtManager)(http.HandlerFunc(makeScheduleCopyWeekHandler(scheduleService))))
		mux.Handle("GET /api/v1/merchant/schedules/on-duty", middleware.Auth(jwtManager)(http.HandlerFunc(makeScheduleOnDutyHandler(scheduleService))))

		// Merchant role management (auth-protected, merchant-only).
		mux.Handle("POST /api/v1/merchant/roles", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantRoleCreateHandler(merchantRoleService))))
		mux.Handle("GET /api/v1/merchant/roles", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantRoleListHandler(merchantRoleService))))
		mux.Handle("GET /api/v1/merchant/roles/permissions", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantRolePermissionsHandler(merchantRoleService))))
		mux.Handle("GET /api/v1/merchant/roles/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantRoleGetHandler(merchantRoleService))))
		mux.Handle("PUT /api/v1/merchant/roles/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantRoleUpdateHandler(merchantRoleService, permChecker))))
		mux.Handle("DELETE /api/v1/merchant/roles/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantRoleDeleteHandler(merchantRoleService))))

	// Checkout (auth-protected, merchant-only).
	mux.Handle("POST /api/v1/merchant/checkout", middleware.Auth(jwtManager)(http.HandlerFunc(makeCheckoutHandler(checkoutService, riskService, memberLevelService, pointsService, memberTagService, shiftService))))

	// POS cash register (auth-protected, merchant-only).
	mux.Handle("POST /api/v1/merchant/pos/cart/calculate", middleware.Auth(jwtManager)(http.HandlerFunc(makePosCartCalculateHandler(checkoutService))))
	mux.Handle("GET /api/v1/merchant/pos/members/lookup", middleware.Auth(jwtManager)(http.HandlerFunc(makePosMemberLookupHandler(checkoutService))))
	mux.Handle("GET /api/v1/merchant/pos/coupons/verify", middleware.Auth(jwtManager)(http.HandlerFunc(makePosCouponVerifyHandler(checkoutService))))

		// Orders — listing, detail, refund (auth-protected, merchant-only).
		mux.Handle("GET /api/v1/merchant/orders", middleware.Auth(jwtManager)(middleware.RequireMerchantUser(http.HandlerFunc(makeOrderListHandler(ordersService)))))
		mux.Handle("GET /api/v1/merchant/orders/{id}", middleware.Auth(jwtManager)(middleware.RequireMerchantUser(http.HandlerFunc(makeOrderDetailHandler(ordersService)))))
		mux.Handle("POST /api/v1/merchant/orders/{id}/refund", middleware.Auth(jwtManager)(middleware.RequireMerchantUser(http.HandlerFunc(makeRefundHandler(ordersService, riskService)))))

		// Verification — coupon / service card / third-party voucher (auth-protected, merchant-only).
		mux.Handle("POST /api/v1/merchant/verification/coupon", middleware.Auth(jwtManager)(http.HandlerFunc(makeVerifyCouponHandler(verificationService))))
		mux.Handle("POST /api/v1/merchant/verification/third-party", middleware.Auth(jwtManager)(http.HandlerFunc(makeVerifyThirdPartyHandler(verificationService))))
		mux.Handle("POST /api/v1/merchant/verification/service-card", middleware.Auth(jwtManager)(http.HandlerFunc(makeVerifyServiceCardHandler(verificationService))))
		mux.Handle("GET /api/v1/merchant/verification/records", middleware.Auth(jwtManager)(http.HandlerFunc(makeVerificationRecordsHandler(verificationService))))

		// Receipt template (auth-protected, merchant-only).
		mux.Handle("GET /api/v1/merchant/receipt-template", middleware.Auth(jwtManager)(http.HandlerFunc(makeReceiptTemplateGetHandler(receiptTemplateService))))
		mux.Handle("PUT /api/v1/merchant/receipt-template", middleware.Auth(jwtManager)(http.HandlerFunc(makeReceiptTemplateUpdateHandler(receiptTemplateService))))
		mux.Handle("POST /api/v1/merchant/receipt-template/logo", middleware.Auth(jwtManager)(http.HandlerFunc(makeReceiptTemplateLogoHandler(receiptTemplateService))))
		mux.Handle("GET /api/v1/merchant/orders/{id}/receipt", middleware.Auth(jwtManager)(http.HandlerFunc(makeOrderReceiptHandler(receiptTemplateService))))

		// Shift reconciliation (auth-protected, merchant-only).
		mux.Handle("POST /api/v1/merchant/shift", middleware.Auth(jwtManager)(http.HandlerFunc(makeShiftCreateHandler(shiftService))))
		mux.Handle("GET /api/v1/merchant/shift", middleware.Auth(jwtManager)(http.HandlerFunc(makeShiftListHandler(shiftService))))
		mux.Handle("GET /api/v1/merchant/shift/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeShiftGetHandler(shiftService))))
		mux.Handle("POST /api/v1/merchant/shift/{id}/confirm", middleware.Auth(jwtManager)(http.HandlerFunc(makeShiftConfirmHandler(shiftService))))
		mux.Handle("GET /api/v1/merchant/shift/today", middleware.Auth(jwtManager)(http.HandlerFunc(makeShiftTodayHandler(shiftService))))

		// Coupon management (auth-protected, merchant-only).
		mux.Handle("POST /api/v1/merchant/coupons/templates", middleware.Auth(jwtManager)(http.HandlerFunc(makeCouponTemplateCreateHandler(couponService))))
		mux.Handle("GET /api/v1/merchant/coupons/templates", middleware.Auth(jwtManager)(http.HandlerFunc(makeCouponTemplateListHandler(couponService))))
		mux.Handle("GET /api/v1/merchant/coupons/templates/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeCouponTemplateGetHandler(couponService))))
		mux.Handle("PUT /api/v1/merchant/coupons/templates/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeCouponTemplateUpdateHandler(couponService))))
		mux.Handle("POST /api/v1/merchant/coupons/templates/{id}/toggle", middleware.Auth(jwtManager)(http.HandlerFunc(makeCouponTemplateToggleHandler(couponService))))
		mux.Handle("POST /api/v1/merchant/coupons/templates/{id}/issue", middleware.Auth(jwtManager)(http.HandlerFunc(makeCouponIssueHandler(couponService))))
		mux.Handle("GET /api/v1/merchant/coupons/codes", middleware.Auth(jwtManager)(http.HandlerFunc(makeCouponCodeListHandler(couponService))))
		mux.Handle("GET /api/v1/merchant/coupons/stats", middleware.Auth(jwtManager)(http.HandlerFunc(makeCouponStatsHandler(couponService))))

		// Promotion activity management (auth-protected, merchant-only).
		mux.Handle("POST /api/v1/merchant/promotions", middleware.Auth(jwtManager)(http.HandlerFunc(makePromotionCreateHandler(promotionService))))
		mux.Handle("GET /api/v1/merchant/promotions", middleware.Auth(jwtManager)(http.HandlerFunc(makePromotionListHandler(promotionService))))
		mux.Handle("GET /api/v1/merchant/promotions/active", middleware.Auth(jwtManager)(http.HandlerFunc(makePromotionActiveHandler(promotionService))))
		mux.Handle("GET /api/v1/merchant/promotions/stats", middleware.Auth(jwtManager)(http.HandlerFunc(makePromotionStatsHandler(promotionService))))
		mux.Handle("GET /api/v1/merchant/promotions/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makePromotionGetHandler(promotionService))))
		mux.Handle("PUT /api/v1/merchant/promotions/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makePromotionUpdateHandler(promotionService))))
		mux.Handle("DELETE /api/v1/merchant/promotions/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makePromotionDeleteHandler(promotionService))))
		mux.Handle("POST /api/v1/merchant/promotions/{id}/toggle", middleware.Auth(jwtManager)(http.HandlerFunc(makePromotionToggleHandler(promotionService))))

		// Service card management (auth-protected, merchant-only).
			mux.Handle("POST /api/v1/merchant/service-cards/templates", middleware.Auth(jwtManager)(http.HandlerFunc(makeSCTemplateCreateHandler(scService))))
			mux.Handle("GET /api/v1/merchant/service-cards/templates", middleware.Auth(jwtManager)(http.HandlerFunc(makeSCTemplateListHandler(scService))))
			mux.Handle("GET /api/v1/merchant/service-cards/templates/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeSCTemplateGetHandler(scService))))
			mux.Handle("PUT /api/v1/merchant/service-cards/templates/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeSCTemplateUpdateHandler(scService))))
			mux.Handle("POST /api/v1/merchant/service-cards/templates/{id}/toggle", middleware.Auth(jwtManager)(http.HandlerFunc(makeSCTemplateToggleHandler(scService))))
			mux.Handle("POST /api/v1/merchant/service-cards/templates/{id}/purchase", middleware.Auth(jwtManager)(http.HandlerFunc(makeSCTemplatePurchaseHandler(scService))))
			mux.Handle("GET /api/v1/merchant/service-cards/member/{memberId}", middleware.Auth(jwtManager)(http.HandlerFunc(makeSCMemberCardsHandler(scService))))
			mux.Handle("GET /api/v1/merchant/service-cards/expiring", middleware.Auth(jwtManager)(http.HandlerFunc(makeSCExpiringHandler(scService))))
			mux.Handle("GET /api/v1/merchant/service-cards/code", middleware.Auth(jwtManager)(http.HandlerFunc(makeSCCardByCodeHandler(scService))))
			mux.Handle("GET /api/v1/merchant/service-cards", middleware.Auth(jwtManager)(http.HandlerFunc(makeSCAllCardsHandler(scService))))
			mux.Handle("GET /api/v1/merchant/service-cards/usage-logs/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeSCUsageLogsHandler(scService))))

		// Risk control — rule management (platform-only auth + permission).
	mux.Handle("GET /api/v1/risk/rules",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("risk:view")(
					http.HandlerFunc(makeRiskRuleListHandler(riskService)),
				),
			),
		),
	)
	mux.Handle("POST /api/v1/risk/rules",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("risk:manage")(
					http.HandlerFunc(makeRiskRuleCreateHandler(riskService)),
				),
			),
		),
	)
	mux.Handle("GET /api/v1/risk/rules/{id}",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("risk:view")(
					http.HandlerFunc(makeRiskRuleGetHandler(riskService)),
				),
			),
		),
	)
	mux.Handle("PUT /api/v1/risk/rules/{id}",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("risk:manage")(
					http.HandlerFunc(makeRiskRuleUpdateHandler(riskService)),
				),
			),
		),
	)
	mux.Handle("DELETE /api/v1/risk/rules/{id}",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("risk:manage")(
					http.HandlerFunc(makeRiskRuleDeleteHandler(riskService)),
				),
			),
		),
	)
	mux.Handle("POST /api/v1/risk/rules/{id}/toggle",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("risk:manage")(
					http.HandlerFunc(makeRiskRuleToggleHandler(riskService)),
				),
			),
		),
	)

	// Risk control — alert management (platform-only auth + permission).
	mux.Handle("GET /api/v1/risk/alerts",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("risk:view")(
					http.HandlerFunc(makeRiskAlertListHandler(riskService)),
				),
			),
		),
	)
	mux.Handle("GET /api/v1/risk/alerts/{id}",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("risk:view")(
					http.HandlerFunc(makeRiskAlertGetHandler(riskService)),
				),
			),
		),
	)
	mux.Handle("PUT /api/v1/risk/alerts/{id}/status",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("risk:manage")(
					http.HandlerFunc(makeRiskAlertStatusHandler(riskService)),
				),
			),
		),
	)

	// Complaint ticket management (platform-only auth-protected).
	mux.Handle("POST /api/v1/complaints", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeComplaintCreateHandler(complaintService)))))
	mux.Handle("GET /api/v1/complaints",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("complaint:view")(
					http.HandlerFunc(makeComplaintListHandler(complaintService)),
				),
			),
		),
	)
	mux.Handle("GET /api/v1/complaints/{id}",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("complaint:view")(
					http.HandlerFunc(makeComplaintGetHandler(complaintService)),
				),
			),
		),
	)
	mux.Handle("PUT /api/v1/complaints/{id}/assign",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("complaint:manage")(
					http.HandlerFunc(makeComplaintAssignHandler(complaintService)),
				),
			),
		),
	)
	mux.Handle("PUT /api/v1/complaints/{id}/progress",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("complaint:manage")(
					http.HandlerFunc(makeComplaintUpdateProgressHandler(complaintService)),
				),
			),
		),
	)
	mux.Handle("PUT /api/v1/complaints/{id}/status",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("complaint:manage")(
					http.HandlerFunc(makeComplaintUpdateStatusHandler(complaintService)),
				),
			),
		),
	)
	mux.Handle("GET /api/v1/complaints/stats",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("complaint:view")(
					http.HandlerFunc(makeComplaintStatsHandler(complaintService)),
				),
			),
		),
	)

	// Report export (platform-only, auth-protected).
	mux.Handle("GET /api/v1/reports/operating", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeReportOperatingHandler(reportService)))))
	mux.Handle("GET /api/v1/reports/transactions", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeReportTransactionHandler(reportService)))))

	// Contract management (platform-only, auth-protected).
	mux.Handle("POST /api/v1/contracts/merchant/{id}", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeContractUploadHandler(contractService)))))
	mux.Handle("GET /api/v1/contracts/merchant/{id}", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeContractListHandler(contractService)))))
	mux.Handle("GET /api/v1/contracts/merchant/{id}/current", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeContractCurrentHandler(contractService)))))
	mux.Handle("POST /api/v1/contracts/merchant/{id}/renew", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeContractRenewHandler(contractService)))))
	mux.Handle("GET /api/v1/contracts/reminders", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeContractRemindersHandler(contractService)))))

	// Dictionary management — Categories (platform-only, auth-protected).
	mux.Handle("GET /api/v1/dict/categories", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeDictListCategoriesHandler(dictService)))))
	mux.Handle("PUT /api/v1/dict/categories/{id}", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeDictUpdateCategoryHandler(dictService)))))
	mux.Handle("DELETE /api/v1/dict/categories/{id}", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeDictDeleteCategoryHandler(dictService)))))
	mux.Handle("POST /api/v1/dict/categories/{id}/toggle", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeDictToggleCategoryHandler(dictService)))))

	// Dictionary management — Breeds (platform-only, auth-protected).
	mux.Handle("POST /api/v1/dict/breeds", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeDictCreateBreedHandler(dictService)))))
	mux.Handle("GET /api/v1/dict/breeds", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeDictListBreedsHandler(dictService)))))
	mux.Handle("PUT /api/v1/dict/breeds/{id}", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeDictUpdateBreedHandler(dictService)))))
	mux.Handle("DELETE /api/v1/dict/breeds/{id}", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeDictDeleteBreedHandler(dictService)))))
	mux.Handle("POST /api/v1/dict/breeds/{id}/toggle", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeDictToggleBreedHandler(dictService)))))

	// Platform role & permission management (platform-only, auth-protected).
	mux.Handle("GET /api/v1/platform/permissions", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makePermissionsHandler(roleService)))))
	mux.Handle("GET /api/v1/platform/roles", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeRoleListHandler(roleService)))))
	mux.Handle("POST /api/v1/platform/roles", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeRoleCreateHandler(roleService)))))
	mux.Handle("GET /api/v1/platform/roles/{id}", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeRoleGetHandler(roleService)))))
	mux.Handle("PUT /api/v1/platform/roles/{id}", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeRoleUpdateHandler(roleService)))))
	mux.Handle("DELETE /api/v1/platform/roles/{id}", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeRoleDeleteHandler(roleService)))))
	mux.Handle("GET /api/v1/platform/users", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeUserListHandler(roleService)))))
	mux.Handle("POST /api/v1/platform/users", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeUserCreateHandler(roleService)))))
	mux.Handle("PUT /api/v1/platform/users/{id}/role", middleware.Auth(jwtManager)(middleware.RequirePlatformUser(http.HandlerFunc(makeUserAssignRoleHandler(roleService)))))

	// Permission-protected routes (platform-only auth + permission check).
	// Merchant approve requires merchant:manage permission.
	mux.Handle("POST /api/v1/merchants/{id}/approve",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("merchant:manage")(
					http.HandlerFunc(makeMerchantApproveHandler(merchantService)),
				),
			),
		),
	)
	// Dict create requires dict:manage permission.
	mux.Handle("POST /api/v1/dict/categories",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("dict:manage")(
					http.HandlerFunc(makeDictCreateCategoryHandler(dictService)),
				),
			),
		),
	)

	// Announcement routes — platform side (platform-only auth + permission).
	mux.Handle("GET /api/v1/announcements",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("announcement:view")(
					http.HandlerFunc(makeAnnouncementListHandler(announcementService)),
				),
			),
		),
	)
	mux.Handle("GET /api/v1/announcements/{id}",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("announcement:view")(
					http.HandlerFunc(makeAnnouncementGetHandler(announcementService)),
				),
			),
		),
	)
	mux.Handle("POST /api/v1/announcements",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("announcement:manage")(
					http.HandlerFunc(makeAnnouncementCreateHandler(announcementService)),
				),
			),
		),
	)
	mux.Handle("PUT /api/v1/announcements/{id}",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("announcement:manage")(
					http.HandlerFunc(makeAnnouncementUpdateHandler(announcementService)),
				),
			),
		),
	)
	mux.Handle("DELETE /api/v1/announcements/{id}",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("announcement:manage")(
					http.HandlerFunc(makeAnnouncementDeleteHandler(announcementService)),
				),
			),
		),
	)
	mux.Handle("POST /api/v1/announcements/{id}/pin",
		middleware.Auth(jwtManager)(
			middleware.RequirePlatformUser(
				permChecker.RequirePermission("announcement:manage")(
					http.HandlerFunc(makeAnnouncementPinHandler(announcementService)),
				),
			),
		),
	)

	// Announcement routes — merchant side (auth + merchant-only).
	mux.Handle("GET /api/v1/merchant/announcements", middleware.Auth(jwtManager)(middleware.RequireMerchantUser(http.HandlerFunc(makeMerchantAnnouncementListHandler(announcementService)))))
	mux.Handle("GET /api/v1/merchant/announcements/unread-count", middleware.Auth(jwtManager)(middleware.RequireMerchantUser(http.HandlerFunc(makeMerchantAnnouncementUnreadCountHandler(announcementService)))))
	mux.Handle("POST /api/v1/merchant/announcements/{id}/read", middleware.Auth(jwtManager)(middleware.RequireMerchantUser(http.HandlerFunc(makeMerchantAnnouncementReadHandler(announcementService)))))

	// Protected routes.
	protected := http.NewServeMux()
	protected.HandleFunc("/api/v1/demo/protected", demoProtectedHandler)
	protected.HandleFunc("/api/v1/demo/validation", demoValidationHandler)
	protected.HandleFunc("/api/v1/demo/panic", demoPanicHandler)
	mux.Handle("/api/v1/demo/", middleware.Auth(jwtManager)(protected))

	inner := http.Handler(mux)
	inner = notFoundWrapper(inner)

	// Serve uploaded files.
	uploadsFS := http.FileServer(http.Dir("uploads"))
	chain := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/uploads/") {
			http.StripPrefix("/uploads/", uploadsFS).ServeHTTP(w, r)
			return
		}
		inner.ServeHTTP(w, r)
	})

	h := loggingMiddleware(lgr)(chain)
	h = middleware.RequestID(h)
	h = middleware.WithClientIP(h)
	h = middleware.Recovery(lgr)(h)

	addr := ":" + cfg.Server.Port
	lgr.Info("Pet Store Management System starting",
		zap.String("addr", addr),
		zap.String("mode", cfg.Server.Mode),
	)
	if err := http.ListenAndServe(addr, h); err != nil {
		lgr.Fatal("Server failed to start", zap.Error(err))
	}
}

func handleMigration(cfg *config.Config, migrate, rollback, status bool) {
	dsn := cfg.DSN()
	fmt.Println("Connecting to database...")
	db, err := database.Connect(dsn)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()

	m := database.NewMigrator(db, "migrations")

	switch {
	case migrate:
		fmt.Println("Running migrations...")
		if err := m.Migrate(); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
		fmt.Println("Migrations complete.")
	case rollback:
		fmt.Println("Rolling back last migration...")
		if err := m.Rollback(); err != nil {
			log.Fatalf("Rollback failed: %v", err)
		}
		fmt.Println("Rollback complete.")
	case status:
		fmt.Println("Migration status:")
		if err := m.Status(); err != nil {
			log.Fatalf("Status check failed: %v", err)
		}
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// --- Auth handlers ---

func makeLoginHandler(svc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			apperrors.WriteError(w, r, apperrors.NewNotFoundError("route not found: "+r.URL.Path))
			return
		}

		var req auth.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		if req.Username == "" || req.Password == "" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("username and password are required"))
			return
		}

		tokens, err := svc.Login(r.Context(), req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("login failed", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokens)
	}
}

func makeRefreshHandler(svc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			apperrors.WriteError(w, r, apperrors.NewNotFoundError("route not found: "+r.URL.Path))
			return
		}

		var req auth.RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		if req.RefreshToken == "" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("refresh_token is required"))
			return
		}

		tokens, err := svc.RefreshToken(r.Context(), req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("token refresh failed", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokens)
	}
}

func makeChangePasswordHandler(svc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			apperrors.WriteError(w, r, apperrors.NewNotFoundError("route not found: "+r.URL.Path))
			return
		}

		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		var req auth.ChangePasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		resp, err := svc.ChangePassword(r.Context(), claims.UserID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("password change failed", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// --- Merchant auth handlers ---

func makeMerchantLoginHandler(svc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req auth.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		if req.Username == "" || req.Password == "" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("username and password are required"))
			return
		}

		resp, err := svc.MerchantLogin(r.Context(), req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("login failed", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// --- Demo handlers ---

func demoValidationHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		apperrors.WriteError(w, r, apperrors.NewValidationError("field 'name' is required"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"name": name})
}

func demoProtectedHandler(w http.ResponseWriter, r *http.Request) {
	claims := middleware.UserClaimsFromContext(r.Context())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"msg":      "authorized",
		"user_id":  claims.UserID,
		"username": claims.Username,
	})
}

func demoPanicHandler(w http.ResponseWriter, r *http.Request) {
	panic("simulated internal error")
}

// --- Middleware ---

// notFoundWrapper returns a standardized JSON 404 for unmatched routes.
func notFoundWrapper(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pattern := next.(*http.ServeMux).Handler(r)
		if pattern == "" {
			appErr := apperrors.NewNotFoundError("route not found: " + r.URL.Path)
			apperrors.WriteError(w, r, appErr)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(lgr *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(wrapped, r)

			lgr.Info("request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", wrapped.statusCode),
				zap.Duration("duration", time.Since(start)),
			)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// --- Merchant handlers ---

func makeMerchantApplyHandler(svc *merchant.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req merchant.ApplyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		resp, err := svc.Apply(r.Context(), req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("application failed", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}
}

func makeMerchantGetHandler(svc *merchant.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid application id"))
			return
		}

		detail, err := svc.GetByID(r.Context(), id)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to retrieve application", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(detail)
	}
}

func makeMerchantListHandler(svc *merchant.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

		params := merchant.ListParams{
			Keyword:  r.URL.Query().Get("keyword"),
			Status:   r.URL.Query().Get("status"),
			Page:     page,
			PageSize: pageSize,
		}

		resp, err := svc.List(r.Context(), params)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list merchants", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func makeMerchantPendingHandler(svc *merchant.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apps, err := svc.ListPending(r.Context())
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list pending applications", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"applications": apps,
			"total":        len(apps),
		})
	}
}

func makeMerchantApproveHandler(svc *merchant.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid application id"))
			return
		}

		resp, err := svc.Approve(r.Context(), id, claims.UserID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("approval failed", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func makeMerchantRejectHandler(svc *merchant.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid application id"))
			return
		}

		var body struct {
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}
		if strings.TrimSpace(body.Reason) == "" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("rejection reason is required"))
			return
		}

		resp, err := svc.Reject(r.Context(), id, body.Reason, claims.UserID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("rejection failed", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func makeMerchantResubmitHandler(svc *merchant.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid application id"))
			return
		}

		var req merchant.ApplyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		resp, err := svc.Resubmit(r.Context(), id, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("resubmission failed", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func makeMerchantFreezeHandler(svc *merchant.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid merchant id"))
			return
		}

		var body struct {
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}
		if strings.TrimSpace(body.Reason) == "" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("freeze reason is required"))
			return
		}

		resp, err := svc.Freeze(r.Context(), id, body.Reason, claims.UserID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("freeze failed", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func makeMerchantUnfreezeHandler(svc *merchant.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid merchant id"))
			return
		}

		var body struct {
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}
		if strings.TrimSpace(body.Reason) == "" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("unfreeze reason is required"))
			return
		}

		resp, err := svc.Unfreeze(r.Context(), id, body.Reason, claims.UserID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("unfreeze failed", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func makeMerchantCloseHandler(svc *merchant.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid merchant id"))
			return
		}

		var body struct {
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}
		if strings.TrimSpace(body.Reason) == "" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("close reason is required"))
			return
		}

		resp, err := svc.Close(r.Context(), id, body.Reason, claims.UserID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("close failed", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func makeMerchantLogsHandler(svc *merchant.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid merchant id"))
			return
		}

		logs, err := svc.GetOperationLogs(r.Context(), id)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to retrieve operation logs", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"logs":  logs,
			"total": len(logs),
		})
	}
}

func makeOperationLogsHandler(svc *operationlog.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		params := operationlog.QueryParams{}

		if v := q.Get("user_id"); v != "" {
			id, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				apperrors.WriteError(w, r, apperrors.NewValidationError("invalid user_id"))
				return
			}
			params.UserID = &id
		}
		if v := q.Get("action"); v != "" {
			params.Action = &v
		}
		if v := q.Get("target_type"); v != "" {
			params.TargetType = &v
		}
		if v := q.Get("start_time"); v != "" {
			params.StartTime = &v
		}
		if v := q.Get("end_time"); v != "" {
			params.EndTime = &v
		}
		if v := q.Get("page"); v != "" {
			page, err := strconv.Atoi(v)
			if err == nil {
				params.Page = page
			}
		}
		if v := q.Get("page_size"); v != "" {
			size, err := strconv.Atoi(v)
			if err == nil {
				params.PageSize = size
			}
		}

		resp, err := svc.Query(r.Context(), params)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to query operation logs", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// --- Contract handlers ---

func makeContractUploadHandler(svc *contract.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		idStr := r.PathValue("id")
		merchantID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || merchantID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid merchant id"))
			return
		}

		if err := r.ParseMultipartForm(32 << 20); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("failed to parse multipart form: "+err.Error()))
			return
		}

		contractNumber := strings.TrimSpace(r.FormValue("contract_number"))
		startDate := strings.TrimSpace(r.FormValue("start_date"))
		endDate := strings.TrimSpace(r.FormValue("end_date"))

		file, header, err := r.FormFile("file")
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("file is required"))
			return
		}
		file.Close()

		req := contract.UploadRequest{
			ContractNumber: contractNumber,
			StartDate:      startDate,
			EndDate:        endDate,
			FileHeader:     header,
		}

		resp, err := svc.Upload(r.Context(), merchantID, req, claims.UserID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("contract upload failed", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}
}

func makeContractListHandler(svc *contract.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		merchantID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || merchantID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid merchant id"))
			return
		}

		resp, err := svc.List(r.Context(), merchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list contracts", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func makeContractCurrentHandler(svc *contract.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		merchantID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || merchantID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid merchant id"))
			return
		}

		resp, err := svc.GetCurrent(r.Context(), merchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get current contract", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func makeContractRenewHandler(svc *contract.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		idStr := r.PathValue("id")
		merchantID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || merchantID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid merchant id"))
			return
		}

		if err := r.ParseMultipartForm(32 << 20); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("failed to parse multipart form: "+err.Error()))
			return
		}

		contractNumber := strings.TrimSpace(r.FormValue("contract_number"))
		startDate := strings.TrimSpace(r.FormValue("start_date"))
		endDate := strings.TrimSpace(r.FormValue("end_date"))

		file, header, err := r.FormFile("file")
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("file is required"))
			return
		}
		file.Close()

		req := contract.UploadRequest{
			ContractNumber: contractNumber,
			StartDate:      startDate,
			EndDate:        endDate,
			FileHeader:     header,
		}

		resp, err := svc.Renew(r.Context(), merchantID, req, claims.UserID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("contract renewal failed", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}
}

func makeContractRemindersHandler(svc *contract.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := svc.GetReminders(r.Context())
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get reminders", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// --- Dictionary handlers ---

func makeDictCreateCategoryHandler(svc *dictionary.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		var req dictionary.CreateCategoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		// Determine if this is a merchant-level request.
		// Platform admins (no merchant_id in claims or role_id <= super_admin) get merchantID=0.
		merchantID := int64(0)
		if claims.MerchantID != nil {
			merchantID = *claims.MerchantID
		}

		cat, err := svc.CreateCategory(r.Context(), req, merchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create category", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(cat)
	}
}

func makeDictListCategoriesHandler(svc *dictionary.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		merchantID := int64(0)
		if claims.MerchantID != nil {
			merchantID = *claims.MerchantID
		}

		cats, err := svc.ListCategories(r.Context(), merchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list categories", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"categories": cats,
		})
	}
}

func makeDictUpdateCategoryHandler(svc *dictionary.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid category id"))
			return
		}

		var req dictionary.UpdateCategoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		merchantID := int64(0)
		if claims.MerchantID != nil {
			merchantID = *claims.MerchantID
		}

		cat, err := svc.UpdateCategory(r.Context(), id, req, merchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update category", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cat)
	}
}

func makeDictDeleteCategoryHandler(svc *dictionary.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid category id"))
			return
		}

		merchantID := int64(0)
		if claims.MerchantID != nil {
			merchantID = *claims.MerchantID
		}

		if err := svc.DeleteCategory(r.Context(), id, merchantID); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to delete category", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "category deleted"})
	}
}

func makeDictToggleCategoryHandler(svc *dictionary.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid category id"))
			return
		}

		merchantID := int64(0)
		if claims.MerchantID != nil {
			merchantID = *claims.MerchantID
		}

		resp, err := svc.ToggleCategory(r.Context(), id, merchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to toggle category", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func makeDictCreateBreedHandler(svc *dictionary.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req dictionary.CreateBreedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		breed, err := svc.CreateBreed(r.Context(), req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create breed", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(breed)
	}
}

func makeDictListBreedsHandler(svc *dictionary.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		petType := r.URL.Query().Get("pet_type")

		merchantID := int64(0)
		if claims.MerchantID != nil {
			merchantID = *claims.MerchantID
		}

		resp, err := svc.ListBreeds(r.Context(), petType, merchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list breeds", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func makeDictUpdateBreedHandler(svc *dictionary.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid breed id"))
			return
		}

		var req dictionary.UpdateBreedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		breed, err := svc.UpdateBreed(r.Context(), id, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update breed", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(breed)
	}
}

func makeDictDeleteBreedHandler(svc *dictionary.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid breed id"))
			return
		}

		if err := svc.DeleteBreed(r.Context(), id); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to delete breed", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "breed deleted"})
	}
}

func makeDictToggleBreedHandler(svc *dictionary.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid breed id"))
			return
		}

		resp, err := svc.ToggleBreed(r.Context(), id)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to toggle breed", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// --- Role handlers ---

func makePermissionsHandler(svc *role.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"permissions": svc.GetAvailablePermissions(),
		})
	}
}

func makeRoleListHandler(svc *role.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roles, err := svc.ListRoles(r.Context())
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list roles", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"roles": roles,
			"total": len(roles),
		})
	}
}

func makeRoleCreateHandler(svc *role.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		var req role.CreateRoleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		resp, err := svc.CreateRole(r.Context(), req, claims.UserID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create role", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}
}

func makeRoleGetHandler(svc *role.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid role id"))
			return
		}

		rp, err := svc.GetRole(r.Context(), id)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get role", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rp)
	}
}

func makeRoleUpdateHandler(svc *role.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid role id"))
			return
		}

		var req role.UpdateRoleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		resp, err := svc.UpdateRole(r.Context(), id, req, claims.UserID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update role", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func makeRoleDeleteHandler(svc *role.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid role id"))
			return
		}

		if err := svc.DeleteRole(r.Context(), id, claims.UserID); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to delete role", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "role deleted"})
	}
}

// --- User handlers ---

func makeUserListHandler(svc *role.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := svc.ListUsers(r.Context())
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list users", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"users": users,
			"total": len(users),
		})
	}
}

func makeUserCreateHandler(svc *role.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		var req role.CreateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		resp, err := svc.CreateUser(r.Context(), req, claims.UserID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create user", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}
}

// --- Announcement handlers (platform side) ---

func makeAnnouncementCreateHandler(svc *announcement.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		var req announcement.CreateAnnouncementRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		a, err := svc.CreateAnnouncement(r.Context(), req, claims.UserID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create announcement", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(a)
	}
}

func makeAnnouncementListHandler(svc *announcement.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

		params := announcement.ListParams{
			Scope:    r.URL.Query().Get("scope"),
			Page:     page,
			PageSize: pageSize,
		}

		resp, err := svc.ListAnnouncements(r.Context(), params)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list announcements", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func makeAnnouncementGetHandler(svc *announcement.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid announcement id"))
			return
		}

		detail, err := svc.GetAnnouncement(r.Context(), id)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get announcement", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(detail)
	}
}

func makeAnnouncementUpdateHandler(svc *announcement.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid announcement id"))
			return
		}

		var req announcement.UpdateAnnouncementRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		a, err := svc.UpdateAnnouncement(r.Context(), id, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update announcement", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(a)
	}
}

func makeAnnouncementDeleteHandler(svc *announcement.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid announcement id"))
			return
		}

		if err := svc.DeleteAnnouncement(r.Context(), id); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to delete announcement", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "announcement deleted"})
	}
}

func makeAnnouncementPinHandler(svc *announcement.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid announcement id"))
			return
		}

		a, err := svc.PinAnnouncement(r.Context(), id)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to toggle pin", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(a)
	}
}

// --- Announcement handlers (merchant side) ---

func makeMerchantAnnouncementListHandler(svc *announcement.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		announcements, err := svc.GetMerchantAnnouncements(r.Context(), *claims.MerchantID, claims.UserID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list announcements", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"announcements": announcements,
			"total":         len(announcements),
		})
	}
}

func makeMerchantAnnouncementUnreadCountHandler(svc *announcement.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		count, err := svc.GetUnreadCount(r.Context(), *claims.MerchantID, claims.UserID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to count unread", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"unread_count": count,
		})
	}
}

func makeMerchantAnnouncementReadHandler(svc *announcement.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		idStr := r.PathValue("id")
		announcementID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || announcementID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid announcement id"))
			return
		}

		if err := svc.MarkAsRead(r.Context(), announcementID, claims.UserID); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to mark as read", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "marked as read"})
	}
}

func makeUserAssignRoleHandler(svc *role.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		idStr := r.PathValue("id")
		userID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || userID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid user id"))
			return
		}

		var req role.AssignRoleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		if err := svc.AssignRole(r.Context(), userID, req.RoleID, claims.UserID); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to assign role", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "role assigned successfully"})
	}
}

// --- Merchant dashboard handler ---

func makeMerchantDashboardHandler(svc *merchant.Service, petSvc *pet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		resp, err := svc.GetDashboard(r.Context(), *claims.MerchantID, petSvc)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get merchant dashboard", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// --- Merchant shop settings handlers ---

func makeMerchantShopSettingsGetHandler(svc *merchant.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		settings, err := svc.GetShopSettings(r.Context(), *claims.MerchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get shop settings", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(settings)
	}
}

func makeMerchantShopSettingsUpdateHandler(svc *merchant.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		var req merchant.UpdateShopSettingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		settings, err := svc.UpdateShopSettings(r.Context(), *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update shop settings", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(settings)
	}
}

func makeMerchantShopSettingsLogoHandler(svc *merchant.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("failed to parse multipart form: "+err.Error()))
			return
		}

		file, header, err := r.FormFile("logo")
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("logo file is required"))
			return
		}
		defer file.Close()

		// Generate unique filename: {merchantID}_{timestamp}_{originalName}
		ext := ""
		if idx := strings.LastIndex(header.Filename, "."); idx >= 0 {
			ext = header.Filename[idx:]
		}
		savedName := fmt.Sprintf("%d_%d%s", *claims.MerchantID, time.Now().UnixMilli(), ext)
		saveDir := "uploads/logos"
		if err := os.MkdirAll(saveDir, 0755); err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create upload directory", err))
			return
		}

		dst, err := os.Create(filepath.Join(saveDir, savedName))
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to save logo file", err))
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to write logo file", err))
			return
		}

		logoURL := "/uploads/logos/" + savedName
		settings, err := svc.UpdateShopLogo(r.Context(), *claims.MerchantID, logoURL)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update logo", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(settings)
	}
}

// --- Dashboard handlers ---

func makeDashboardOverviewHandler(svc *dashboard.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		period := r.URL.Query().Get("period")
		if period == "" {
			period = "all"
		}

		resp, err := svc.GetOverview(r.Context(), period)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get dashboard overview", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// --- Product handlers ---

func makeProductCreateHandler(svc *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		var req product.CreateProductRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		p, err := svc.Create(r.Context(), *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create product", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(p)
	}
}

func makeProductListHandler(svc *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		status := r.URL.Query().Get("status")
		keyword := r.URL.Query().Get("keyword")
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

		result, err := svc.List(r.Context(), *claims.MerchantID, product.ListParams{
			Status:   status,
			Keyword:  keyword,
			Page:     page,
			PageSize: pageSize,
		})
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list products", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makeProductGetHandler(svc *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid product id"))
			return
		}

		p, err := svc.GetByIDWithSKUs(r.Context(), id, *claims.MerchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get product", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(p)
	}
}

func makeProductUpdateHandler(svc *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid product id"))
			return
		}

		var req product.UpdateProductRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		p, err := svc.Update(r.Context(), id, *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update product", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(p)
	}
}

func makeProductDeleteHandler(svc *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid product id"))
			return
		}

		if err := svc.Delete(r.Context(), id, *claims.MerchantID); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to delete product", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "product deleted"})
	}
}

func makeProductToggleStatusHandler(svc *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid product id"))
			return
		}

		p, err := svc.ToggleStatus(r.Context(), id, *claims.MerchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to toggle product status", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(p)
	}
}

// --- SKU handlers ---

func makeSkuCreateHandler(svc *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		productID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || productID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid product id"))
			return
		}

		var req product.CreateSkuRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		sku, err := svc.CreateSKU(r.Context(), productID, *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create SKU", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(sku)
	}
}

func makeSkuListHandler(svc *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		productID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || productID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid product id"))
			return
		}

		skus, err := svc.ListSKUs(r.Context(), productID, *claims.MerchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list SKUs", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"skus":  skus,
			"total": len(skus),
		})
	}
}

func makeSkuGetHandler(svc *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		skuIDStr := r.PathValue("skuId")
		skuID, err := strconv.ParseInt(skuIDStr, 10, 64)
		if err != nil || skuID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid SKU id"))
			return
		}

		sku, err := svc.GetSKU(r.Context(), skuID, *claims.MerchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get SKU", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sku)
	}
}

func makeSkuUpdateHandler(svc *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		skuIDStr := r.PathValue("skuId")
		skuID, err := strconv.ParseInt(skuIDStr, 10, 64)
		if err != nil || skuID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid SKU id"))
			return
		}

		var req product.UpdateSkuRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		sku, err := svc.UpdateSKU(r.Context(), skuID, *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update SKU", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sku)
	}
}

func makeSkuDeleteHandler(svc *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		skuIDStr := r.PathValue("skuId")
		skuID, err := strconv.ParseInt(skuIDStr, 10, 64)
		if err != nil || skuID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid SKU id"))
			return
		}

		if err := svc.DeleteSKU(r.Context(), skuID, *claims.MerchantID); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to delete SKU", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "SKU deleted"})
	}
}

func makeSkuToggleStatusHandler(svc *product.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		skuIDStr := r.PathValue("skuId")
		skuID, err := strconv.ParseInt(skuIDStr, 10, 64)
		if err != nil || skuID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid SKU id"))
			return
		}

		sku, err := svc.ToggleSKUStatus(r.Context(), skuID, *claims.MerchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to toggle SKU status", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sku)
	}
}

// --- Checkout handler ---

func makeCheckoutHandler(checkoutSvc *checkout.Service, riskSvc *risk.Service, levelSvc *memberlevel.Service, pointsSvc *points.Service, tagSvc *membertag.Service, shiftSvc *shift.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}
		// Check if employee is shift-locked (must re-login after shift).
		if claims.EmployeeID != nil && *claims.EmployeeID > 0 {
			locked, err := shiftSvc.IsEmployeeShiftLocked(r.Context(), *claims.MerchantID, *claims.EmployeeID)
			if err == nil && locked {
				apperrors.WriteError(w, r, apperrors.NewForbiddenError("shift handover completed, please re-login to continue POS operations"))
				return
			}
		}

		var req checkout.CheckoutRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		resp, err := checkoutSvc.Checkout(r.Context(), *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("checkout failed", err))
			return
		}

		// Post-checkout: auto-award points based on consume rule; record deduction transactions.
		if req.MemberID != nil && *req.MemberID > 0 {
			// Record point deduction transactions for points payment.
			for _, p := range req.Payments {
				if p.Method == "points" && p.AmountCents > 0 {
					_ = pointsSvc.RecordPointsDeduction(r.Context(), *claims.MerchantID, *req.MemberID, resp.OrderID, int64(p.AmountCents), claims.UserID)
				}
			}
			// Auto-award points based on consume rule.
			_, _ = pointsSvc.EarnPointsAfterCheckout(r.Context(), *claims.MerchantID, *req.MemberID, resp.OrderID, int64(resp.PaidCents))
			// Check high-frequency risk.
			_, _ = riskSvc.CheckHighFrequency(r.Context(), *claims.MerchantID, *req.MemberID)
			// Check member level upgrade.
			_, _ = levelSvc.CheckAndUpgrade(r.Context(), *claims.MerchantID, *req.MemberID)
			// Auto-tag member based on rules.
			_, _ = tagSvc.CheckAndApplyRules(r.Context(), *claims.MerchantID, *req.MemberID)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}
}

// --- Merchant analysis handlers ---

func makeMerchantAnalysisHandler(dashSvc *dashboard.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchantIDStr := r.PathValue("id")
		merchantID, err := strconv.ParseInt(merchantIDStr, 10, 64)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid merchant id"))
			return
		}

		period := r.URL.Query().Get("period")
		if period == "" {
			period = "all"
		}

		resp, err := dashSvc.GetMerchantAnalysis(r.Context(), merchantID, period)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get merchant analysis", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func makeMerchantsRankingHandler(dashSvc *dashboard.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		period := r.URL.Query().Get("period")
		if period == "" {
			period = "all"
		}

		resp, err := dashSvc.GetMerchantsRevenueRanking(r.Context(), period)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get ranking", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// --- Report handlers ---

func makeReportOperatingHandler(svc *report.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := r.URL.Query().Get("start_time")
		endTime := r.URL.Query().Get("end_time")

		data, filename, err := svc.ExportOperatingReport(r.Context(), startTime, endTime)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to export operating report", err))
			return
		}

		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Write(data)
	}
}

func makeReportTransactionHandler(svc *report.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := r.URL.Query().Get("start_time")
		endTime := r.URL.Query().Get("end_time")

		data, filename, err := svc.ExportTransactionReport(r.Context(), startTime, endTime)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to export transaction report", err))
			return
		}

		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Write(data)
	}
}


// --- Risk rule handlers ---

func makeRiskRuleListHandler(svc *risk.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rules, err := svc.ListRules(r.Context())
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list risk rules", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"rules": rules,
			"total": len(rules),
		})
	}
}

func makeRiskRuleCreateHandler(svc *risk.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req risk.CreateRuleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		rule, err := svc.CreateRule(r.Context(), &req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create rule", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(rule)
	}
}

func makeRiskRuleGetHandler(svc *risk.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid rule id"))
			return
		}

		rule, err := svc.GetRule(r.Context(), id)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get rule", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rule)
	}
}

func makeRiskRuleUpdateHandler(svc *risk.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid rule id"))
			return
		}

		var req risk.UpdateRuleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		rule, err := svc.UpdateRule(r.Context(), id, &req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update rule", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rule)
	}
}

func makeRiskRuleDeleteHandler(svc *risk.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid rule id"))
			return
		}

		if err := svc.DeleteRule(r.Context(), id); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to delete rule", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "rule deleted"})
	}
}

func makeRiskRuleToggleHandler(svc *risk.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid rule id"))
			return
		}

		rule, err := svc.ToggleRule(r.Context(), id)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to toggle rule", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rule)
	}
}

// --- Risk alert handlers ---

func makeRiskAlertListHandler(svc *risk.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		params := risk.AlertListParams{}

		if v := q.Get("merchant_id"); v != "" {
			id, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				apperrors.WriteError(w, r, apperrors.NewValidationError("invalid merchant_id"))
				return
			}
			params.MerchantID = &id
		}
		params.AlertType = q.Get("alert_type")
		params.Status = q.Get("status")

		if v := q.Get("page"); v != "" {
			page, err := strconv.Atoi(v)
			if err == nil {
				params.Page = page
			}
		}
		if v := q.Get("page_size"); v != "" {
			size, err := strconv.Atoi(v)
			if err == nil {
				params.PageSize = size
			}
		}

		resp, err := svc.ListAlerts(r.Context(), &params)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list alerts", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func makeRiskAlertGetHandler(svc *risk.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid alert id"))
			return
		}

		alert, err := svc.GetAlert(r.Context(), id)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get alert", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(alert)
	}
}

func makeRiskAlertStatusHandler(svc *risk.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid alert id"))
			return
		}

		var req risk.UpdateAlertStatusRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		alert, err := svc.UpdateAlertStatus(r.Context(), id, &req, claims.UserID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update alert status", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(alert)
	}
}

// --- Complaint ticket handlers ---

func makeComplaintCreateHandler(svc *complaint.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req complaint.CreateTicketRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		ticket, err := svc.CreateTicket(r.Context(), req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create ticket", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ticket)
	}
}

func makeComplaintListHandler(svc *complaint.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		params := complaint.ListParams{}

		if v := q.Get("merchant_id"); v != "" {
			id, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				apperrors.WriteError(w, r, apperrors.NewValidationError("invalid merchant_id"))
				return
			}
			params.MerchantID = id
		}
		params.Status = q.Get("status")
		params.ComplaintType = q.Get("complaint_type")

		if v := q.Get("assigned_to"); v != "" {
			id, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				apperrors.WriteError(w, r, apperrors.NewValidationError("invalid assigned_to"))
				return
			}
			params.AssignedTo = id
		}
		if v := q.Get("page"); v != "" {
			page, err := strconv.Atoi(v)
			if err == nil {
				params.Page = page
			}
		}
		if v := q.Get("page_size"); v != "" {
			size, err := strconv.Atoi(v)
			if err == nil {
				params.PageSize = size
			}
		}

		resp, err := svc.ListTickets(r.Context(), params)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list tickets", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func makeComplaintGetHandler(svc *complaint.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid ticket id"))
			return
		}

		ticket, err := svc.GetTicket(r.Context(), id)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get ticket", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ticket)
	}
}

func makeComplaintAssignHandler(svc *complaint.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid ticket id"))
			return
		}

		var req complaint.AssignTicketRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		ticket, err := svc.AssignTicket(r.Context(), id, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to assign ticket", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ticket)
	}
}

func makeComplaintUpdateProgressHandler(svc *complaint.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid ticket id"))
			return
		}

		var req complaint.UpdateProgressRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		ticket, err := svc.UpdateProgress(r.Context(), id, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update progress", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ticket)
	}
}

func makeComplaintUpdateStatusHandler(svc *complaint.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid ticket id"))
			return
		}

		var req complaint.UpdateStatusRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		ticket, err := svc.UpdateStatus(r.Context(), id, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update status", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ticket)
	}
}

func makeComplaintStatsHandler(svc *complaint.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := svc.GetComplaintStats(r.Context())
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get stats", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}
}

// --- Category handlers ---

func makeCategoryCreateHandler(svc *category.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		var req category.CreateCategoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		c, err := svc.Create(r.Context(), *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create category", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(c)
	}
}

func makeCategoryListHandler(svc *category.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		cats, err := svc.List(r.Context(), *claims.MerchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list categories", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"categories": cats,
		})
	}
}

func makeCategoryUpdateHandler(svc *category.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid category id"))
			return
		}

		var req category.UpdateCategoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		c, err := svc.Update(r.Context(), id, *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update category", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(c)
	}
}

func makeCategoryDeleteHandler(svc *category.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid category id"))
			return
		}

		if err := svc.Delete(r.Context(), id, *claims.MerchantID); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to delete category", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "category deleted"})
	}
}

func makeSupplierCreateHandler(svc *supplier.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		var req supplier.CreateSupplierRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		s, err := svc.Create(r.Context(), *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create supplier", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(s)
	}
}

func makeSupplierListHandler(svc *supplier.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		status := r.URL.Query().Get("status")
		keyword := r.URL.Query().Get("keyword")
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

		result, err := svc.List(r.Context(), *claims.MerchantID, supplier.ListParams{
			Status:   status,
			Keyword:  keyword,
			Page:     page,
			PageSize: pageSize,
		})
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list suppliers", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makeSupplierGetHandler(svc *supplier.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid supplier id"))
			return
		}

		detail, err := svc.GetDetail(r.Context(), id, *claims.MerchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get supplier", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(detail)
	}
}

func makeSupplierUpdateHandler(svc *supplier.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid supplier id"))
			return
		}

		var req supplier.UpdateSupplierRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		s, err := svc.Update(r.Context(), id, *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update supplier", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s)
	}
}

func makeSupplierToggleStatusHandler(svc *supplier.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid supplier id"))
			return
		}

		s, err := svc.ToggleStatus(r.Context(), id, *claims.MerchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to toggle supplier status", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s)
	}
}

func makeSupplierLinkProductHandler(svc *supplier.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid supplier id"))
			return
		}

		var req supplier.LinkProductRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		if err := svc.LinkProduct(r.Context(), id, *claims.MerchantID, req); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to link product", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"message": "product linked to supplier"})
	}
}

func makeSupplierUnlinkProductHandler(svc *supplier.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid supplier id"))
			return
		}

		pidStr := r.PathValue("productId")
		pid, err := strconv.ParseInt(pidStr, 10, 64)
		if err != nil || pid <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid product id"))
			return
		}

		if err := svc.UnlinkProduct(r.Context(), id, pid, *claims.MerchantID); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to unlink product", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "product unlinked from supplier"})
	}
}

// --- Member handlers ---

func makeMemberCreateHandler(svc *member.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		var req member.CreateMemberRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		m, err := svc.Create(r.Context(), *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create member", err))
			return
		}

		maskMember(m)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(m)
	}
}

func makeMemberListHandler(svc *member.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
		tagID, _ := strconv.ParseInt(r.URL.Query().Get("tag_id"), 10, 64)

		result, err := svc.List(r.Context(), *claims.MerchantID, member.ListParams{
			Status:   r.URL.Query().Get("status"),
			Keyword:  r.URL.Query().Get("keyword"),
			TagID:    tagID,
			Page:     page,
			PageSize: pageSize,
		})
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list members", err))
			return
		}

		maskMemberList(result)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makeMemberSearchHandler(svc *member.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		phone := r.URL.Query().Get("phone")
		members, err := svc.SearchByPhone(r.Context(), *claims.MerchantID, phone)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to search members", err))
			return
		}

		maskMemberSlice(members)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"members": members,
			"total":   len(members),
		})
	}
}

func makeMemberGetHandler(svc *member.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		detail, err := svc.GetDetail(r.Context(), id, *claims.MerchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get member", err))
			return
		}

		maskMember(&detail.Member)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(detail)
	}
}

func makeMemberUpdateHandler(svc *member.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		var req member.UpdateMemberRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		m, err := svc.Update(r.Context(), id, *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update member", err))
			return
		}

		maskMember(m)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(m)
	}
}

func makeMemberToggleStatusHandler(svc *member.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		m, err := svc.ToggleStatus(r.Context(), id, *claims.MerchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to toggle member status", err))
			return
		}

		maskMember(m)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(m)
	}
}

func makeMemberBatchImportHandler(svc *member.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		contentType := r.Header.Get("Content-Type")

		var result *member.BatchImportResult
		var resultErr error

		if strings.Contains(contentType, "multipart/form-data") {
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				apperrors.WriteError(w, r, apperrors.NewValidationError("failed to parse multipart form: "+err.Error()))
				return
			}

			file, _, err := r.FormFile("file")
			if err != nil {
				apperrors.WriteError(w, r, apperrors.NewValidationError("file is required"))
				return
			}
			defer file.Close()

			result, resultErr = svc.BatchImport(r.Context(), *claims.MerchantID, file)
		} else {
			result, resultErr = svc.BulkCreateJSON(r.Context(), *claims.MerchantID, r.Body)
		}

		if resultErr != nil {
			if appErr, ok := resultErr.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("batch import failed", resultErr))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// --- Member QR code handlers ---

func makeMemberQRCodeHandler(svc *member.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		// Verify member belongs to this merchant.
		m, err := svc.GetByID(r.Context(), id, *claims.MerchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get member", err))
			return
		}

		token := member.GenerateQRCodeToken(m.ID, m.MerchantID)
		png, err := member.RenderQRCodePNG(token)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to generate QR code", err))
			return
		}

		// Support download via ?download=1 query parameter.
		if r.URL.Query().Get("download") == "1" {
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=member_%s_qrcode.png", m.CardNo))
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(png)
	}
}

func makeMemberQRCodeScanHandler(svc *member.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		token := r.URL.Query().Get("token")
		if token == "" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("token parameter is required"))
			return
		}

		memberID, merchantID, ok := member.VerifyQRCodeToken(token)
		if !ok {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid or forged QR code token"))
			return
		}

		// Only allow scanning QR codes from the same merchant.
		if merchantID != *claims.MerchantID {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("QR code does not belong to your merchant"))
			return
		}

		m, err := svc.GetByID(r.Context(), memberID, merchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get member", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"member_id":   m.ID,
			"name":        m.Name,
			"phone":       m.Phone,
			"card_no":     m.CardNo,
			"merchant_id": m.MerchantID,
			"status":      m.Status,
		})
	}
}

// --- Pet handlers ---

func makePetCreateHandler(svc *pet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		memberID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || memberID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		var req pet.CreatePetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		p, err := svc.Create(r.Context(), *claims.MerchantID, memberID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create pet", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(p)
	}
}

func makePetListHandler(svc *pet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		memberID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || memberID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		pets, err := svc.ListByMember(r.Context(), *claims.MerchantID, memberID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list pets", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pets":  pets,
			"total": len(pets),
		})
	}
}

func makePetGetHandler(svc *pet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		memberID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || memberID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		petIDStr := r.PathValue("petId")
		petID, err := strconv.ParseInt(petIDStr, 10, 64)
		if err != nil || petID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid pet id"))
			return
		}

		p, err := svc.GetByID(r.Context(), petID, memberID, *claims.MerchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get pet", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(p)
	}
}

func makePetUpdateHandler(svc *pet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		memberID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || memberID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		petIDStr := r.PathValue("petId")
		petID, err := strconv.ParseInt(petIDStr, 10, 64)
		if err != nil || petID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid pet id"))
			return
		}

		var req pet.UpdatePetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		p, err := svc.Update(r.Context(), petID, memberID, *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update pet", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(p)
	}
}

func makePetDeleteHandler(svc *pet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		memberID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || memberID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid member id"))
			return
		}

		petIDStr := r.PathValue("petId")
		petID, err := strconv.ParseInt(petIDStr, 10, 64)
		if err != nil || petID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid pet id"))
			return
		}

		if err := svc.Delete(r.Context(), petID, memberID, *claims.MerchantID); err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to delete pet", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "pet deleted"})
	}
}

// --- Pet health reminder handlers ---

func makeHealthRemindersHandler(svc *pet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		q := r.URL.Query()
		params := pet.HealthReminderParams{
			Type: q.Get("type"),
		}
		if d, err := strconv.Atoi(q.Get("days")); err == nil && d > 0 {
			params.Days = d
		} else if params.Days <= 0 {
			params.Days = 7
		}
		if p, err := strconv.Atoi(q.Get("page")); err == nil && p > 0 {
			params.Page = p
		} else {
			params.Page = 1
		}
		if ps, err := strconv.Atoi(q.Get("page_size")); err == nil && ps > 0 {
			params.PageSize = ps
		} else {
			params.PageSize = 20
		}

		reminders, total, err := svc.GetHealthReminders(r.Context(), *claims.MerchantID, params)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get health reminders", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"reminders": reminders,
			"total":     total,
			"page":      params.Page,
			"page_size": params.PageSize,
		})
	}
}

func makeHealthReminderCountHandler(svc *pet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		days := 7
		if d, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && d > 0 {
			days = d
		}

		counts, err := svc.GetHealthReminderCounts(r.Context(), *claims.MerchantID, days)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get health reminder counts", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(counts)
	}
}

// --- Employee handlers ---

func makeEmployeeCreateHandler(svc *employee.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		var req employee.CreateEmployeeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		e, err := svc.Create(r.Context(), *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create employee", err))
			return
		}

		maskEmployee(e)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(e)
	}
}

func makeEmployeeListHandler(svc *employee.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

		params := employee.ListParams{
			Status:   r.URL.Query().Get("status"),
			Position: r.URL.Query().Get("position"),
			Keyword:  r.URL.Query().Get("keyword"),
			Page:     page,
			PageSize: pageSize,
		}

		resp, err := svc.List(r.Context(), *claims.MerchantID, params)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list employees", err))
			return
		}

		maskEmployeeList(resp)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func makeEmployeeGetHandler(svc *employee.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid employee id"))
			return
		}

		e, err := svc.GetByID(r.Context(), *claims.MerchantID, id)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get employee", err))
			return
		}

		maskEmployee(e)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(e)
	}
}

func makeEmployeeUpdateHandler(svc *employee.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid employee id"))
			return
		}

		var req employee.UpdateEmployeeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		e, err := svc.Update(r.Context(), *claims.MerchantID, id, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update employee", err))
			return
		}

		maskEmployee(e)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(e)
	}
}

func makeEmployeeResignHandler(svc *employee.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid employee id"))
			return
		}

		e, err := svc.Resign(r.Context(), *claims.MerchantID, id)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to resign employee", err))
			return
		}

		maskEmployee(e)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(e)
	}
}

func makeEmployeeToggleStatusHandler(svc *employee.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid employee id"))
			return
		}

		e, err := svc.ToggleStatus(r.Context(), *claims.MerchantID, id)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to toggle employee status", err))
			return
		}

		maskEmployee(e)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(e)
	}
}

// --- Phone masking helpers ---

func maskMember(m *member.Member) {
	m.Phone = cryptopkg.MaskPhone(m.Phone)
}

func maskMemberList(result *member.ListResult) {
	for i := range result.Members {
		result.Members[i].Phone = cryptopkg.MaskPhone(result.Members[i].Phone)
	}
}

func maskMemberSlice(members []member.Member) {
	for i := range members {
		members[i].Phone = cryptopkg.MaskPhone(members[i].Phone)
	}
}

func maskEmployee(e *employee.Employee) {
	e.Phone = cryptopkg.MaskPhone(e.Phone)
}

func maskEmployeeList(result *employee.ListResult) {
	for i := range result.Employees {
		result.Employees[i].Phone = cryptopkg.MaskPhone(result.Employees[i].Phone)
	}
}

// --- Inventory handlers ---

func getMerchantClaims(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	claims := middleware.UserClaimsFromContext(r.Context())
	if claims == nil {
		apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
		return nil, false
	}
	if claims.MerchantID == nil {
		apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
		return nil, false
	}
	return claims, true
}

func makeInventoryWarehouseCreateHandler(svc *inventory.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := getMerchantClaims(w, r)
		if !ok {
			return
		}
		var req inventory.CreateWarehouseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}
		result, err := svc.CreateWarehouse(r.Context(), *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create warehouse", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(result)
	}
}

func makeInventoryWarehouseListHandler(svc *inventory.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := getMerchantClaims(w, r)
		if !ok {
			return
		}
		result, err := svc.ListWarehouses(r.Context(), *claims.MerchantID)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list warehouses", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"warehouses": result, "total": len(result)})
	}
}

func makeInventoryInboundHandler(svc *inventory.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := getMerchantClaims(w, r)
		if !ok {
			return
		}
		var req inventory.InboundRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}
		result, err := svc.Inbound(r.Context(), *claims.MerchantID, claims.UserID, claims.Username, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("inbound failed", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(result)
	}
}

func makeInventoryOutboundHandler(svc *inventory.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := getMerchantClaims(w, r)
		if !ok {
			return
		}
		var req inventory.OutboundRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}
		result, err := svc.Outbound(r.Context(), *claims.MerchantID, claims.UserID, claims.Username, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("outbound failed", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(result)
	}
}

func makeInventoryTransferHandler(svc *inventory.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := getMerchantClaims(w, r)
		if !ok {
			return
		}
		var req inventory.TransferRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}
		out, in, err := svc.Transfer(r.Context(), *claims.MerchantID, claims.UserID, claims.Username, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("transfer failed", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"transfer_out": out, "transfer_in": in})
	}
}

func makeInventoryLossHandler(svc *inventory.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := getMerchantClaims(w, r)
		if !ok {
			return
		}
		var req inventory.LossRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}
		result, err := svc.Loss(r.Context(), *claims.MerchantID, claims.UserID, claims.Username, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("loss failed", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(result)
	}
}

func makeInventorySurplusHandler(svc *inventory.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := getMerchantClaims(w, r)
		if !ok {
			return
		}
		var req inventory.SurplusRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}
		result, err := svc.Surplus(r.Context(), *claims.MerchantID, claims.UserID, claims.Username, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("surplus failed", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(result)
	}
}

func makeInventoryFlowsHandler(svc *inventory.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := getMerchantClaims(w, r)
		if !ok {
			return
		}
		q := r.URL.Query()

		params := inventory.ListFlowsParams{
			Type:      q.Get("type"),
			StartTime: q.Get("start_time"),
			EndTime:   q.Get("end_time"),
		}

		if v := q.Get("product_id"); v != "" {
			id, err := strconv.ParseInt(v, 10, 64)
			if err == nil {
				params.ProductID = &id
			}
		}
		if v := q.Get("warehouse_id"); v != "" {
			id, err := strconv.ParseInt(v, 10, 64)
			if err == nil {
				params.WarehouseID = &id
			}
		}
		if v := q.Get("page"); v != "" {
			page, err := strconv.Atoi(v)
			if err == nil {
				params.Page = page
			}
		}
		if v := q.Get("page_size"); v != "" {
			size, err := strconv.Atoi(v)
			if err == nil {
				params.PageSize = size
			}
		}

		result, err := svc.ListFlows(r.Context(), *claims.MerchantID, params)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list flows", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// --- Replenishment handlers ---

func makeReplenishSuggestionsHandler(svc *replenishment.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		groupBySupplier := r.URL.Query().Get("group_by_supplier") == "true"

		result, err := svc.GetSuggestions(r.Context(), *claims.MerchantID, groupBySupplier)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get replenishment suggestions", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makeReplenishGeneratePOHandler(replenishSvc *replenishment.Service, purchaseSvc *purchase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("authentication required"))
			return
		}
		if claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewForbiddenError("merchant account required"))
			return
		}

		var req replenishment.GeneratePORequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		results, err := replenishSvc.GeneratePO(r.Context(), *claims.MerchantID, claims.UserID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to generate purchase order", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"purchase_orders": results,
			"total":           len(results),
		})
	}
}

// --- Inventory count check handlers ---

func makeInventoryCheckCreateHandler(svc *inventory.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := getMerchantClaims(w, r)
		if !ok {
			return
		}
		var req inventory.CreateCheckRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		check, err := svc.CreateCheck(r.Context(), *claims.MerchantID, claims.UserID, claims.Username, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to create count check", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(check)
	}
}

func makeInventoryCheckListHandler(svc *inventory.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := getMerchantClaims(w, r)
		if !ok {
			return
		}
		q := r.URL.Query()
		params := inventory.ListChecksParams{
			Status:    q.Get("status"),
			CheckType: q.Get("check_type"),
			Page:      1,
			PageSize:  20,
		}
		if v := q.Get("page"); v != "" {
			if p, err := strconv.Atoi(v); err == nil {
				params.Page = p
			}
		}
		if v := q.Get("page_size"); v != "" {
			if s, err := strconv.Atoi(v); err == nil {
				params.PageSize = s
			}
		}

		result, err := svc.ListChecks(r.Context(), *claims.MerchantID, params)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list checks", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makeInventoryCheckGetHandler(svc *inventory.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := getMerchantClaims(w, r)
		if !ok {
			return
		}
		checkID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid check id"))
			return
		}
		check, err := svc.GetCheck(r.Context(), *claims.MerchantID, checkID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get check", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(check)
	}
}

func makeInventoryCheckUpdateItemHandler(svc *inventory.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := getMerchantClaims(w, r)
		if !ok {
			return
		}
		checkID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid check id"))
			return
		}
		itemID, err := strconv.ParseInt(r.PathValue("itemId"), 10, 64)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid item id"))
			return
		}
		var req inventory.UpdateCheckItemRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		item, err := svc.UpdateCheckItem(r.Context(), *claims.MerchantID, checkID, itemID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update check item", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(item)
	}
}

func makeInventoryCheckSubmitHandler(svc *inventory.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := getMerchantClaims(w, r)
		if !ok {
			return
		}
		checkID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid check id"))
			return
		}
		check, err := svc.SubmitCheck(r.Context(), *claims.MerchantID, checkID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to submit check", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(check)
	}
}

func makeInventoryCheckConfirmHandler(svc *inventory.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := getMerchantClaims(w, r)
		if !ok {
			return
		}
		checkID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid check id"))
			return
		}
		check, err := svc.ConfirmCheck(r.Context(), *claims.MerchantID, claims.UserID, claims.Username, checkID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to confirm check", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(check)
	}
}

func makeInventoryCheckApproveHandler(svc *inventory.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := getMerchantClaims(w, r)
		if !ok {
			return
		}
		checkID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid check id"))
			return
		}
		check, err := svc.ApproveCheck(r.Context(), *claims.MerchantID, claims.UserID, claims.Username, checkID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to approve check", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(check)
	}
}

func makeInventoryAlertsHandler(svc *inventory.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := getMerchantClaims(w, r)
		if !ok {
			return
		}
		q := r.URL.Query()

		params := inventory.AlertListParams{
			AlertType: q.Get("alert_type"),
		}
		if v := q.Get("page"); v != "" {
			page, err := strconv.Atoi(v)
			if err == nil {
				params.Page = page
			}
		}
		if v := q.Get("page_size"); v != "" {
			size, err := strconv.Atoi(v)
			if err == nil {
				params.PageSize = size
			}
		}

		result, err := svc.GetAlerts(r.Context(), *claims.MerchantID, params)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get alerts", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// --- Verification handlers ---

func makeVerifyCouponHandler(svc *verification.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		var req verification.VerifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}
		if strings.TrimSpace(req.Code) == "" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("code is required"))
			return
		}

		result, err := svc.VerifyCoupon(r.Context(), *claims.MerchantID, claims.UserID, strings.TrimSpace(req.Code), req.OrderID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("coupon verification failed", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makeVerifyThirdPartyHandler(svc *verification.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		var req verification.VerifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}
		if strings.TrimSpace(req.Code) == "" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("code is required"))
			return
		}

		result, err := svc.VerifyThirdPartyVoucher(r.Context(), *claims.MerchantID, claims.UserID, strings.TrimSpace(req.Code), req.OrderID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("voucher verification failed", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makeVerifyServiceCardHandler(svc *verification.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		var req verification.VerifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}
		if strings.TrimSpace(req.Code) == "" {
			apperrors.WriteError(w, r, apperrors.NewValidationError("code is required"))
			return
		}

		result, err := svc.VerifyServiceCard(r.Context(), *claims.MerchantID, claims.UserID, strings.TrimSpace(req.Code), req.OrderID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("service card verification failed", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func makeVerificationRecordsHandler(svc *verification.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		pageSize, _ := strconv.Atoi(q.Get("page_size"))

		params := verification.ListParams{
			VerificationType: q.Get("type"),
			Code:             q.Get("code"),
			Page:             page,
			PageSize:         pageSize,
		}

		result, err := svc.ListRecords(r.Context(), *claims.MerchantID, params)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to list records", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// --- Receipt template handlers ---

func makeReceiptTemplateGetHandler(svc *receipttemplate.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		tmpl, err := svc.GetTemplate(r.Context(), *claims.MerchantID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get template", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tmpl)
	}
}

func makeReceiptTemplateUpdateHandler(svc *receipttemplate.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		var req receipttemplate.UpdateTemplateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid request body"))
			return
		}

		tmpl, err := svc.SaveTemplate(r.Context(), *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to save template", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tmpl)
	}
}

func makeReceiptTemplateLogoHandler(svc *receipttemplate.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("failed to parse form: "+err.Error()))
			return
		}

		file, header, err := r.FormFile("logo")
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewValidationError("logo file is required"))
			return
		}
		defer file.Close()

		os.MkdirAll("uploads/receipts", 0755)
		filename := fmt.Sprintf("%d_%d_%s", *claims.MerchantID, time.Now().Unix(), header.Filename)
		dstPath := filepath.Join("uploads", "receipts", filename)
		dst, err := os.Create(dstPath)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to save logo", err))
			return
		}
		defer dst.Close()
		io.Copy(dst, file)

		logoURL := "/uploads/receipts/" + filename

		current, _ := svc.GetTemplate(r.Context(), *claims.MerchantID)
		req := receipttemplate.UpdateTemplateRequest{
			LogoURL:        logoURL,
			StoreName:      current.StoreName,
			ContactPhone:   current.ContactPhone,
			ContactAddress: current.ContactAddress,
			FooterNote:     current.FooterNote,
			PaperWidth:     current.PaperWidth,
			ShowQRCode:     current.ShowQRCode,
		}

		tmpl, err := svc.SaveTemplate(r.Context(), *claims.MerchantID, req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to update logo", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tmpl)
	}
}

func makeOrderReceiptHandler(svc *receipttemplate.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.UserClaimsFromContext(r.Context())
		if claims == nil || claims.MerchantID == nil {
			apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("merchant authentication required"))
			return
		}

		idStr := r.PathValue("id")
		orderID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || orderID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid order id"))
			return
		}

		receipt, err := svc.GetOrderReceipt(r.Context(), *claims.MerchantID, orderID)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.WriteError(w, r, appErr)
				return
			}
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get receipt", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(receipt)
	}
}
