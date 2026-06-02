package router

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
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
	"pawprint/backend/internal/module/setting"
	"pawprint/backend/internal/module/settlement"
	"pawprint/backend/internal/module/wx"
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
	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr})
	idem := middleware.Idempotency(rdb)

	// Auth routes (public)
	authRepo := auth.NewRepository(db)
	authSvc := auth.NewService(authRepo, cfg.JWT.AccessSecret, cfg.JWT.RefreshSecret)
	authHandler := auth.NewHandler(authSvc)
	auth.RegisterRoutes(v1, authHandler)

	// Protected routes (require JWT + store scope)
	protected := v1.Group("")
	protected.Use(middleware.AuthRequired(authSvc))
	protected.Use(middleware.StoreScope(authSvc))

	// Notification
	notifRepo := notification.NewRepository(db)
	notifSvc := notification.NewService(notifRepo)
	notifSvc.SetFeatureFlags(cfg.FeatureSMSEnabled, cfg.FeatureWechatEnabled)
	notifHandler := notification.NewHandler(notifSvc)
	notification.RegisterRoutes(protected, notifHandler, authSvc, idem)

	// Dashboard
	dashRepo := dashboard.NewRepository(db)
	dashSvc := dashboard.NewService(dashRepo, cfg.Timezone)
	dashHandler := dashboard.NewHandler(dashSvc)
	dashboard.RegisterRoutes(protected, dashHandler, authSvc)

	// Appointment
	apptRepo := appointment.NewRepository(db)
	apptSvc := appointment.NewService(apptRepo, appointment.WithNotifier(notifSvc))
	apptHandler := appointment.NewHandler(apptSvc)
	appointment.RegisterRoutes(protected, apptHandler, authSvc, idem)

	// WeChat mini-program routes (customer-facing)
	wxRepo := wx.NewRepository(db)
	wxSvc := wx.NewService(wxRepo, apptSvc)
	wxHandler := wx.NewHandler(wxSvc)
	wx.RegisterRoutes(v1, wxHandler, idem)

	// Pet
	petRepo := pet.NewRepository(db)
	petSvc := pet.NewService(petRepo)
	petHandler := pet.NewHandler(petSvc)
	pet.RegisterRoutes(protected, petHandler, authSvc, idem)

	// Member
	memberRepo := member.NewRepository(db)
	memberSvc := member.NewService(memberRepo)
	memberHandler := member.NewHandler(memberSvc)
	member.RegisterRoutes(protected, memberHandler, authSvc, idem)

	// Inventory
	invRepo := inventory.NewRepository(db)
	invSvc := inventory.NewService(invRepo, inventory.WithNotifier(notifSvc))
	invHandler := inventory.NewHandler(invSvc)
	inventory.RegisterRoutes(protected, invHandler, authSvc, idem)

	// Settlement
	setRepo := settlement.NewRepository(db)
	setSvc := settlement.NewService(
		setRepo,
		settlement.WithMemberEffects(memberSvc),
		settlement.WithInventoryEffects(invSvc),
		settlement.WithPrintJobs(setRepo),
	)
	setHandler := settlement.NewHandler(setSvc)
	settlement.RegisterRoutes(protected, setHandler, authSvc, idem)

	// Boarding
	boardingRepo := boarding.NewRepository(db)
	boardingSvc := boarding.NewService(boardingRepo, boarding.WithSettlementCreator(setSvc))
	boardingHandler := boarding.NewHandler(boardingSvc)
	boarding.RegisterRoutes(protected, boardingHandler, authSvc, idem)

	// Analytics
	analyticsRepo := analytics.NewRepository(db)
	analyticsSvc := analytics.NewService(analyticsRepo)
	analyticsHandler := analytics.NewHandler(analyticsSvc)
	analytics.RegisterRoutes(protected, analyticsHandler, authSvc)

	// Settings
	settingRepo := setting.NewRepository(db)
	settingSvc := setting.NewService(settingRepo)
	settingHandler := setting.NewHandler(settingSvc)
	setting.RegisterRoutes(protected, settingHandler, authSvc, idem)

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
