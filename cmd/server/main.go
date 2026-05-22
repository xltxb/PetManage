package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/xltxb/PetManage/internal/announcement"
	"github.com/xltxb/PetManage/internal/auth"
	"github.com/xltxb/PetManage/internal/config"
	"github.com/xltxb/PetManage/internal/contract"
	"github.com/xltxb/PetManage/internal/dashboard"
	"github.com/xltxb/PetManage/internal/database"
	"github.com/xltxb/PetManage/internal/dictionary"
	"github.com/xltxb/PetManage/internal/merchant"
	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/internal/operationlog"
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

	var h http.Handler = mux
	h = notFoundWrapper(h)
	h = loggingMiddleware(lgr)(h)
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
