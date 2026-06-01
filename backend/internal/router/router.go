package router

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"pawprint/backend/internal/config"
	"pawprint/backend/internal/middleware"
	"pawprint/backend/internal/module/auth"
	"pawprint/backend/internal/module/dashboard"
)

func Setup(db *gorm.DB, cfg *config.Config) *gin.Engine {
	r := gin.New()

	// Global middleware (order matters)
	r.Use(middleware.TraceID())
	r.Use(middleware.RequestLogger())
	r.Use(middleware.Recovery())
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.CORS())

	// Health checks (no auth required)
	r.GET("/healthz", healthCheck(db))
	r.GET("/readyz", readyCheck(db))

	// API v1 group
	v1 := r.Group("/api/v1")

	// Auth routes (public)
	authRepo := auth.NewRepository(db)
	authSvc := auth.NewService(authRepo, cfg.JWT.AccessSecret, cfg.JWT.RefreshSecret)
	authHandler := auth.NewHandler(authSvc)
	auth.RegisterRoutes(v1, authHandler)

	// Protected routes (require JWT + store scope)
	protected := v1.Group("")
	protected.Use(middleware.AuthRequired(authSvc))
	protected.Use(middleware.StoreScope(authSvc))

	// Dashboard
	dashRepo := dashboard.NewRepository(db)
	dashSvc := dashboard.NewService(dashRepo, cfg.Timezone)
	dashHandler := dashboard.NewHandler(dashSvc)
	dashboard.RegisterRoutes(protected, dashHandler)

	return r
}

func healthCheck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(503, gin.H{"status": "unhealthy", "error": "db connection lost"})
			return
		}
		if err := sqlDB.Ping(); err != nil {
			c.JSON(503, gin.H{"status": "unhealthy", "error": "db ping failed"})
			return
		}
		c.JSON(200, gin.H{"status": "healthy"})
	}
}

func readyCheck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(503, gin.H{"status": "not ready"})
			return
		}
		if err := sqlDB.Ping(); err != nil {
			c.JSON(503, gin.H{"status": "not ready", "error": "db unreachable"})
			return
		}
		var count int64
		if err := db.Raw("SELECT count(*) FROM information_schema.tables WHERE table_name = 'stores'").Scan(&count).Error; err != nil || count == 0 {
			c.JSON(503, gin.H{"status": "not ready", "error": "migrations not applied"})
			return
		}
		c.JSON(200, gin.H{"status": "ready"})
	}
}
