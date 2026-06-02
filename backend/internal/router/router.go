package router

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"pawprint/backend/internal/config"
	"pawprint/backend/internal/middleware"
	"pawprint/backend/internal/module/analytics"
	"pawprint/backend/internal/module/appointment"
	"pawprint/backend/internal/module/auth"
	"pawprint/backend/internal/module/boarding"
	"pawprint/backend/internal/module/dashboard"
	"pawprint/backend/internal/module/inventory"
	"pawprint/backend/internal/module/member"
	"pawprint/backend/internal/module/notification"
	"pawprint/backend/internal/module/pet"
	"pawprint/backend/internal/module/settlement"
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

	// Appointment
	apptRepo := appointment.NewRepository(db)
	apptSvc := appointment.NewService(apptRepo)
	apptHandler := appointment.NewHandler(apptSvc)
	appointment.RegisterRoutes(protected, apptHandler)

	// Boarding
	boardingRepo := boarding.NewRepository(db)
	boardingSvc := boarding.NewService(boardingRepo)
	boardingHandler := boarding.NewHandler(boardingSvc)
	boarding.RegisterRoutes(protected, boardingHandler)

	// Pet
	petRepo := pet.NewRepository(db)
	petSvc := pet.NewService(petRepo)
	petHandler := pet.NewHandler(petSvc)
	pet.RegisterRoutes(protected, petHandler)

	// Member
	memberRepo := member.NewRepository(db)
	memberSvc := member.NewService(memberRepo)
	memberHandler := member.NewHandler(memberSvc)
	member.RegisterRoutes(protected, memberHandler)

	// Inventory
	invRepo := inventory.NewRepository(db)
	invSvc := inventory.NewService(invRepo)
	invHandler := inventory.NewHandler(invSvc)
	inventory.RegisterRoutes(protected, invHandler)

	// Settlement
	setRepo := settlement.NewRepository(db)
	setSvc := settlement.NewService(setRepo)
	setHandler := settlement.NewHandler(setSvc)
	settlement.RegisterRoutes(protected, setHandler)

	// Notification
	notifRepo := notification.NewRepository(db)
	notifSvc := notification.NewService(notifRepo)
	notifHandler := notification.NewHandler(notifSvc)
	notification.RegisterRoutes(protected, notifHandler)

	// Analytics
	analyticsRepo := analytics.NewRepository(db)
	analyticsSvc := analytics.NewService(analyticsRepo)
	analyticsHandler := analytics.NewHandler(analyticsSvc)
	analytics.RegisterRoutes(protected, analyticsHandler)

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
