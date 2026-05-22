package main

import (
	"database/sql"
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
	"github.com/xltxb/PetManage/internal/auth"
	"github.com/xltxb/PetManage/internal/category"
	"github.com/xltxb/PetManage/internal/checkout"
	"github.com/xltxb/PetManage/internal/complaint"
	"github.com/xltxb/PetManage/internal/config"
	"github.com/xltxb/PetManage/internal/contract"
	"github.com/xltxb/PetManage/internal/dashboard"
	"github.com/xltxb/PetManage/internal/database"
	"github.com/xltxb/PetManage/internal/dictionary"
	"github.com/xltxb/PetManage/internal/merchant"
	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/internal/operationlog"
	"github.com/xltxb/PetManage/internal/product"
	"github.com/xltxb/PetManage/internal/report"
	"github.com/xltxb/PetManage/internal/member"
	"github.com/xltxb/PetManage/internal/pet"
	"github.com/xltxb/PetManage/internal/supplier"
	"github.com/xltxb/PetManage/internal/risk"
	"github.com/xltxb/PetManage/internal/role"
	"github.com/xltxb/PetManage/pkg/apperrors"
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

	// Initialize supplier service.
	supplierService := supplier.NewService(db)

	// Initialize pet service.
	petService := pet.NewService(db)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/v1/auth/login", makeLoginHandler(authService))
	mux.HandleFunc("/api/v1/auth/refresh", makeRefreshHandler(authService))
	mux.HandleFunc("POST /api/v1/merchant/auth/login", makeMerchantLoginHandler(authService))
	mux.Handle("/api/v1/auth/change-password", middleware.Auth(jwtManager)(http.HandlerFunc(makeChangePasswordHandler(authService))))

	// Merchant routes (public).
	mux.HandleFunc("POST /api/v1/merchants/apply", makeMerchantApplyHandler(merchantService))
	mux.HandleFunc("GET /api/v1/merchants/apply/{id}", makeMerchantGetHandler(merchantService))

	// Merchant routes (auth-protected).
	mux.Handle("GET /api/v1/merchants", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantListHandler(merchantService))))
	mux.Handle("GET /api/v1/merchants/pending", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantPendingHandler(merchantService))))
	mux.Handle("POST /api/v1/merchants/{id}/reject", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantRejectHandler(merchantService))))
	mux.Handle("PUT /api/v1/merchants/{id}/apply", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantResubmitHandler(merchantService))))

	// Merchant status control (auth-protected).
	mux.Handle("POST /api/v1/merchants/{id}/freeze", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantFreezeHandler(merchantService))))
	mux.Handle("POST /api/v1/merchants/{id}/unfreeze", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantUnfreezeHandler(merchantService))))
	mux.Handle("POST /api/v1/merchants/{id}/close", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantCloseHandler(merchantService))))
	mux.Handle("GET /api/v1/operation-logs/merchant/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantLogsHandler(merchantService))))
	mux.Handle("GET /api/v1/operation-logs", middleware.Auth(jwtManager)(http.HandlerFunc(makeOperationLogsHandler(opLogService))))

	// Dashboard routes (auth-protected).
	mux.Handle("GET /api/v1/dashboard/overview", middleware.Auth(jwtManager)(http.HandlerFunc(makeDashboardOverviewHandler(dashboardService))))
	mux.Handle("GET /api/v1/dashboard/merchant/{id}/analysis", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantAnalysisHandler(dashboardService))))
	mux.Handle("GET /api/v1/dashboard/merchants/ranking", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantsRankingHandler(dashboardService))))

	// Merchant dashboard routes (auth-protected).
	mux.Handle("GET /api/v1/merchant/dashboard", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantDashboardHandler(merchantService))))

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
	// Member management (auth-protected, merchant-only).
	mux.Handle("POST /api/v1/merchant/members", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberCreateHandler(memberService))))
	mux.Handle("GET /api/v1/merchant/members", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberListHandler(memberService))))
	mux.Handle("GET /api/v1/merchant/members/search", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberSearchHandler(memberService))))
	mux.Handle("GET /api/v1/merchant/members/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberGetHandler(memberService))))
	mux.Handle("PUT /api/v1/merchant/members/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberUpdateHandler(memberService))))
	mux.Handle("POST /api/v1/merchant/members/{id}/toggle-status", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberToggleStatusHandler(memberService))))
	mux.Handle("POST /api/v1/merchant/members/batch-import", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberBatchImportHandler(memberService))))

	// Member QR code (auth-protected, merchant-only).
	mux.Handle("GET /api/v1/merchant/members/{id}/qrcode", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberQRCodeHandler(memberService))))
	mux.Handle("GET /api/v1/merchant/members/qrcode/scan", middleware.Auth(jwtManager)(http.HandlerFunc(makeMemberQRCodeScanHandler(memberService))))

	// Pet management (auth-protected, merchant-only).
	mux.Handle("POST /api/v1/merchant/members/{id}/pets", middleware.Auth(jwtManager)(http.HandlerFunc(makePetCreateHandler(petService))))
	mux.Handle("GET /api/v1/merchant/members/{id}/pets", middleware.Auth(jwtManager)(http.HandlerFunc(makePetListHandler(petService))))
	mux.Handle("GET /api/v1/merchant/members/{id}/pets/{petId}", middleware.Auth(jwtManager)(http.HandlerFunc(makePetGetHandler(petService))))
	mux.Handle("PUT /api/v1/merchant/members/{id}/pets/{petId}", middleware.Auth(jwtManager)(http.HandlerFunc(makePetUpdateHandler(petService))))
	mux.Handle("DELETE /api/v1/merchant/members/{id}/pets/{petId}", middleware.Auth(jwtManager)(http.HandlerFunc(makePetDeleteHandler(petService))))

	// Checkout (auth-protected, merchant-only).
	mux.Handle("POST /api/v1/merchant/checkout", middleware.Auth(jwtManager)(http.HandlerFunc(makeCheckoutHandler(checkoutService, riskService))))

	// Refund (auth-protected, merchant-only).
	mux.Handle("POST /api/v1/merchant/orders/{id}/refund", middleware.Auth(jwtManager)(http.HandlerFunc(makeRefundHandler(riskService))))

	// Risk control — rule management (auth + permission).
	mux.Handle("GET /api/v1/risk/rules",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("risk:view")(
				http.HandlerFunc(makeRiskRuleListHandler(riskService)),
			),
		),
	)
	mux.Handle("POST /api/v1/risk/rules",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("risk:manage")(
				http.HandlerFunc(makeRiskRuleCreateHandler(riskService)),
			),
		),
	)
	mux.Handle("GET /api/v1/risk/rules/{id}",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("risk:view")(
				http.HandlerFunc(makeRiskRuleGetHandler(riskService)),
			),
		),
	)
	mux.Handle("PUT /api/v1/risk/rules/{id}",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("risk:manage")(
				http.HandlerFunc(makeRiskRuleUpdateHandler(riskService)),
			),
		),
	)
	mux.Handle("DELETE /api/v1/risk/rules/{id}",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("risk:manage")(
				http.HandlerFunc(makeRiskRuleDeleteHandler(riskService)),
			),
		),
	)
	mux.Handle("POST /api/v1/risk/rules/{id}/toggle",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("risk:manage")(
				http.HandlerFunc(makeRiskRuleToggleHandler(riskService)),
			),
		),
	)

	// Risk control — alert management (auth + permission).
	mux.Handle("GET /api/v1/risk/alerts",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("risk:view")(
				http.HandlerFunc(makeRiskAlertListHandler(riskService)),
			),
		),
	)
	mux.Handle("GET /api/v1/risk/alerts/{id}",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("risk:view")(
				http.HandlerFunc(makeRiskAlertGetHandler(riskService)),
			),
		),
	)
	mux.Handle("PUT /api/v1/risk/alerts/{id}/status",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("risk:manage")(
				http.HandlerFunc(makeRiskAlertStatusHandler(riskService)),
			),
		),
	)

	// Complaint ticket management (auth-protected).
	mux.Handle("POST /api/v1/complaints", middleware.Auth(jwtManager)(http.HandlerFunc(makeComplaintCreateHandler(complaintService))))
	mux.Handle("GET /api/v1/complaints",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("complaint:view")(
				http.HandlerFunc(makeComplaintListHandler(complaintService)),
			),
		),
	)
	mux.Handle("GET /api/v1/complaints/{id}",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("complaint:view")(
				http.HandlerFunc(makeComplaintGetHandler(complaintService)),
			),
		),
	)
	mux.Handle("PUT /api/v1/complaints/{id}/assign",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("complaint:manage")(
				http.HandlerFunc(makeComplaintAssignHandler(complaintService)),
			),
		),
	)
	mux.Handle("PUT /api/v1/complaints/{id}/progress",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("complaint:manage")(
				http.HandlerFunc(makeComplaintUpdateProgressHandler(complaintService)),
			),
		),
	)
	mux.Handle("PUT /api/v1/complaints/{id}/status",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("complaint:manage")(
				http.HandlerFunc(makeComplaintUpdateStatusHandler(complaintService)),
			),
		),
	)
	mux.Handle("GET /api/v1/complaints/stats",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("complaint:view")(
				http.HandlerFunc(makeComplaintStatsHandler(complaintService)),
			),
		),
	)

	// Report export (auth-protected).
	mux.Handle("GET /api/v1/reports/operating", middleware.Auth(jwtManager)(http.HandlerFunc(makeReportOperatingHandler(reportService))))
	mux.Handle("GET /api/v1/reports/transactions", middleware.Auth(jwtManager)(http.HandlerFunc(makeReportTransactionHandler(reportService))))

	// Contract management (auth-protected).
	mux.Handle("POST /api/v1/contracts/merchant/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeContractUploadHandler(contractService))))
	mux.Handle("GET /api/v1/contracts/merchant/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeContractListHandler(contractService))))
	mux.Handle("GET /api/v1/contracts/merchant/{id}/current", middleware.Auth(jwtManager)(http.HandlerFunc(makeContractCurrentHandler(contractService))))
	mux.Handle("POST /api/v1/contracts/merchant/{id}/renew", middleware.Auth(jwtManager)(http.HandlerFunc(makeContractRenewHandler(contractService))))
	mux.Handle("GET /api/v1/contracts/reminders", middleware.Auth(jwtManager)(http.HandlerFunc(makeContractRemindersHandler(contractService))))

	// Dictionary management — Categories (auth-protected).
	mux.Handle("GET /api/v1/dict/categories", middleware.Auth(jwtManager)(http.HandlerFunc(makeDictListCategoriesHandler(dictService))))
	mux.Handle("PUT /api/v1/dict/categories/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeDictUpdateCategoryHandler(dictService))))
	mux.Handle("DELETE /api/v1/dict/categories/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeDictDeleteCategoryHandler(dictService))))
	mux.Handle("POST /api/v1/dict/categories/{id}/toggle", middleware.Auth(jwtManager)(http.HandlerFunc(makeDictToggleCategoryHandler(dictService))))

	// Dictionary management — Breeds (auth-protected).
	mux.Handle("POST /api/v1/dict/breeds", middleware.Auth(jwtManager)(http.HandlerFunc(makeDictCreateBreedHandler(dictService))))
	mux.Handle("GET /api/v1/dict/breeds", middleware.Auth(jwtManager)(http.HandlerFunc(makeDictListBreedsHandler(dictService))))
	mux.Handle("PUT /api/v1/dict/breeds/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeDictUpdateBreedHandler(dictService))))
	mux.Handle("DELETE /api/v1/dict/breeds/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeDictDeleteBreedHandler(dictService))))
	mux.Handle("POST /api/v1/dict/breeds/{id}/toggle", middleware.Auth(jwtManager)(http.HandlerFunc(makeDictToggleBreedHandler(dictService))))

	// Platform role & permission management (auth-protected).
	mux.Handle("GET /api/v1/platform/permissions", middleware.Auth(jwtManager)(http.HandlerFunc(makePermissionsHandler(roleService))))
	mux.Handle("GET /api/v1/platform/roles", middleware.Auth(jwtManager)(http.HandlerFunc(makeRoleListHandler(roleService))))
	mux.Handle("POST /api/v1/platform/roles", middleware.Auth(jwtManager)(http.HandlerFunc(makeRoleCreateHandler(roleService))))
	mux.Handle("GET /api/v1/platform/roles/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeRoleGetHandler(roleService))))
	mux.Handle("PUT /api/v1/platform/roles/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeRoleUpdateHandler(roleService))))
	mux.Handle("DELETE /api/v1/platform/roles/{id}", middleware.Auth(jwtManager)(http.HandlerFunc(makeRoleDeleteHandler(roleService))))
	mux.Handle("GET /api/v1/platform/users", middleware.Auth(jwtManager)(http.HandlerFunc(makeUserListHandler(roleService))))
	mux.Handle("POST /api/v1/platform/users", middleware.Auth(jwtManager)(http.HandlerFunc(makeUserCreateHandler(roleService))))
	mux.Handle("PUT /api/v1/platform/users/{id}/role", middleware.Auth(jwtManager)(http.HandlerFunc(makeUserAssignRoleHandler(roleService))))

	// Permission-protected routes (auth + permission check).
	// Merchant approve requires merchant:manage permission.
	mux.Handle("POST /api/v1/merchants/{id}/approve",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("merchant:manage")(
				http.HandlerFunc(makeMerchantApproveHandler(merchantService)),
			),
		),
	)
	// Dict create requires dict:manage permission.
	mux.Handle("POST /api/v1/dict/categories",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("dict:manage")(
				http.HandlerFunc(makeDictCreateCategoryHandler(dictService)),
			),
		),
	)

	// Announcement routes — platform side (auth + permission).
	mux.Handle("GET /api/v1/announcements",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("announcement:view")(
				http.HandlerFunc(makeAnnouncementListHandler(announcementService)),
			),
		),
	)
	mux.Handle("GET /api/v1/announcements/{id}",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("announcement:view")(
				http.HandlerFunc(makeAnnouncementGetHandler(announcementService)),
			),
		),
	)
	mux.Handle("POST /api/v1/announcements",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("announcement:manage")(
				http.HandlerFunc(makeAnnouncementCreateHandler(announcementService)),
			),
		),
	)
	mux.Handle("PUT /api/v1/announcements/{id}",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("announcement:manage")(
				http.HandlerFunc(makeAnnouncementUpdateHandler(announcementService)),
			),
		),
	)
	mux.Handle("DELETE /api/v1/announcements/{id}",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("announcement:manage")(
				http.HandlerFunc(makeAnnouncementDeleteHandler(announcementService)),
			),
		),
	)
	mux.Handle("POST /api/v1/announcements/{id}/pin",
		middleware.Auth(jwtManager)(
			permChecker.RequirePermission("announcement:manage")(
				http.HandlerFunc(makeAnnouncementPinHandler(announcementService)),
			),
		),
	)

	// Announcement routes — merchant side (auth only).
	mux.Handle("GET /api/v1/merchant/announcements", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantAnnouncementListHandler(announcementService))))
	mux.Handle("GET /api/v1/merchant/announcements/unread-count", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantAnnouncementUnreadCountHandler(announcementService))))
	mux.Handle("POST /api/v1/merchant/announcements/{id}/read", middleware.Auth(jwtManager)(http.HandlerFunc(makeMerchantAnnouncementReadHandler(announcementService))))

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

func makeMerchantDashboardHandler(svc *merchant.Service) http.HandlerFunc {
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

		resp, err := svc.GetDashboard(r.Context(), *claims.MerchantID)
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

func makeCheckoutHandler(checkoutSvc *checkout.Service, riskSvc *risk.Service) http.HandlerFunc {
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

		// Check high-frequency risk after successful checkout.
		if req.MemberID != nil {
			_, _ = riskSvc.CheckHighFrequency(r.Context(), *claims.MerchantID, *req.MemberID)
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

// --- Refund handler ---

func makeRefundHandler(riskSvc *risk.Service) http.HandlerFunc {
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
		orderID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || orderID <= 0 {
			apperrors.WriteError(w, r, apperrors.NewValidationError("invalid order id"))
			return
		}

		db := riskSvc.GetDB()

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to begin transaction", err))
			return
		}
		defer tx.Rollback()

		// Verify order belongs to the merchant and is completed.
		var totalCents int
		err = tx.QueryRowContext(r.Context(),
			`SELECT total_cents FROM orders
			 WHERE id = $1 AND merchant_id = $2 AND status = 'completed' FOR UPDATE`,
			orderID, *claims.MerchantID,
		).Scan(&totalCents)
		if err == sql.ErrNoRows {
			apperrors.WriteError(w, r, apperrors.NewNotFoundError("order not found or already refunded"))
			return
		}
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to query order", err))
			return
		}

		// Update order status to refunded.
		_, err = tx.ExecContext(r.Context(),
			`UPDATE orders SET status = 'refunded', updated_at = NOW() WHERE id = $1`,
			orderID,
		)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to refund order", err))
			return
		}

		// Restore inventory for each order item.
		rows, err := tx.QueryContext(r.Context(),
			`SELECT product_id, quantity FROM order_items WHERE order_id = $1`, orderID,
		)
		if err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to query order items", err))
			return
		}
		type itemRestore struct {
			productID int64
			quantity  int
		}
		var items []itemRestore
		for rows.Next() {
			var ir itemRestore
			if err := rows.Scan(&ir.productID, &ir.quantity); err != nil {
				rows.Close()
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to scan order item", err))
				return
			}
			items = append(items, ir)
		}
		rows.Close()

		for _, ir := range items {
			_, err = tx.ExecContext(r.Context(),
				`UPDATE products SET stock = stock + $1, updated_at = NOW() WHERE id = $2`,
				ir.quantity, ir.productID,
			)
			if err != nil {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to restore inventory", err))
				return
			}

			// Record stock flow for refund.
			_, err = tx.ExecContext(r.Context(),
				`INSERT INTO stock_flows (merchant_id, product_id, order_id, type, quantity_change)
				 VALUES ($1, $2, $3, 'inbound', $4)`,
				*claims.MerchantID, ir.productID, orderID, ir.quantity,
			)
			if err != nil {
				apperrors.WriteError(w, r, apperrors.NewInternalError("failed to record stock flow", err))
				return
			}
		}

		if err := tx.Commit(); err != nil {
			apperrors.WriteError(w, r, apperrors.NewInternalError("failed to commit refund", err))
			return
		}

		// Check large refund risk (non-blocking, log only).
		alert, _ := riskSvc.CheckLargeRefund(r.Context(), orderID, *claims.MerchantID, totalCents)

		resp := map[string]interface{}{
			"order_id":    orderID,
			"status":      "refunded",
			"total_cents": totalCents,
		}
		if alert != nil {
			resp["risk_alert"] = map[string]interface{}{
				"id":          alert.ID,
				"alert_type":  alert.AlertType,
				"description": alert.Description,
				"status":      alert.Status,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
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

		result, err := svc.List(r.Context(), *claims.MerchantID, member.ListParams{
			Status:   r.URL.Query().Get("status"),
			Keyword:  r.URL.Query().Get("keyword"),
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
