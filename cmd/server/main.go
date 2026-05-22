package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

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

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/v1/demo/validation", demoValidationHandler)
	mux.HandleFunc("/api/v1/demo/protected", demoProtectedHandler)
	mux.HandleFunc("/api/v1/demo/panic", demoPanicHandler)

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

// --- Demo handlers for error handling verification ---

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
	token := r.Header.Get("Authorization")
	if token == "" {
		apperrors.WriteError(w, r, apperrors.NewUnauthorizedError("missing authorization token"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"msg": "authorized"})
}

func demoPanicHandler(w http.ResponseWriter, r *http.Request) {
	panic("simulated internal error")
}

// --- Logging middleware ---

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
