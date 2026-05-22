package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/xltxb/PetManage/internal/auth"
	"github.com/xltxb/PetManage/internal/config"
	"github.com/xltxb/PetManage/internal/database"
	"github.com/xltxb/PetManage/internal/middleware"
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

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/v1/auth/login", makeLoginHandler(authService))
	mux.HandleFunc("/api/v1/auth/refresh", makeRefreshHandler(authService))

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
