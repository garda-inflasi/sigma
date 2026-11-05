package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/garda-inflasi/api/internal/auth/routes"
	"github.com/garda-inflasi/api/internal/dashboard/refresher"
	ewsjobs "github.com/garda-inflasi/api/internal/ews/jobs"
	intervensiworkflow "github.com/garda-inflasi/api/internal/intervensi/workflow"
	"github.com/garda-inflasi/api/internal/modules"
	"github.com/garda-inflasi/api/internal/platform/prodcheck"
	"github.com/garda-inflasi/api/pkg/database"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/swagger"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
)

// dashboardRefreshInterval adalah jeda penyegaran berkala agregat dashboard.
const dashboardRefreshInterval = 5 * time.Minute

func main() {
	addr := os.Getenv("APP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "" {
		ginMode = "debug"
	}
	gin.SetMode(ginMode)

	// Logger
	log := logger.NewDefault()

	// Produksi menolak start bila rahasia masih bawaan/lemah (lihat prodcheck).
	if err := prodcheck.ValidateAPI(os.Getenv); err != nil {
		log.Error("konfigurasi production tidak aman, server tidak dijalankan", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	// Database
	dbCfg := database.DefaultConfig()
	db, err := database.New(dbCfg)
	if err != nil {
		log.Error("database connection failed", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	log.Info("database connected", map[string]interface{}{"host": dbCfg.Host, "db": dbCfg.DBName})

	// Token manager
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "garda-inflasi-dev-secret-key-change-in-production"
		log.Warn("using default JWT secret, set JWT_SECRET env var")
	}
	tokenMgr := token.NewManager(jwtSecret)

	// Router
	router := gin.New()
	router.Use(gin.Recovery())

	// ── Global middleware chain ──────────────────────────────────────
	router.Use(
		middleware.RequestID(),
		middleware.SecurityHeaders(ginMode == "release"),
		middleware.CSP(),
		middleware.CORSMiddleware(),
		middleware.CSRFProtection(middleware.ParseOrigins(os.Getenv("APP_ALLOWED_ORIGINS"), "http://localhost:3000")),
		middleware.AccessLog(log),
		middleware.RateLimit(100, time.Minute),
	)

	// Health check
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Swagger UI (development only)
	swagger.Register(router)

	// API v1
	v1 := router.Group("/v1")

	// Register auth module
	routes.Register(v1, &routes.Dependencies{
		DB:     db,
		Token:  tokenMgr,
		Logger: log,
	})

	// Register all other modules
	modules.RegisterAll(v1, db, log, tokenMgr)

	// Pekerjaan latar belakang berhenti bersama server.
	bgCtx, stopBackground := context.WithCancel(context.Background())
	defer stopBackground()

	// EWS dan workflow intervensi memakai komponen yang sama dengan route; alert dari evaluasi
	// manual (POST /v1/ews/evaluasi) ikut menjadi kartu intervensi otomatis.
	ews := ewsjobs.Shared(db, log)
	workflow := intervensiworkflow.Shared(db, log)
	ews.Engine.AddObserver(workflow)
	// Penerima sinyal SSE selalu di API karena API yang melayani koneksi /v1/ews/stream.
	go ews.RunStream(bgCtx)

	// Job berkala (refresh dashboard, rule engine dan notifikasi EWS, SLA intervensi) dimiliki
	// cmd/worker: compose menyetel API_BACKGROUND_JOBS=false. Tanpa worker (mis. make dev/api)
	// API menjalankannya sendiri. Penyegaran dashboard setelah harga berubah tetap di API.
	if os.Getenv("API_BACKGROUND_JOBS") != "false" {
		go refresher.Shared(db, log).Run(bgCtx, dashboardRefreshInterval)
		go ews.RunBerkala(bgCtx)
		go workflow.Run(bgCtx)
	} else {
		log.Info("job berkala dinonaktifkan di API (dimiliki cmd/worker)")
	}

	srv := &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Rewrite paths for Caddy reverse proxy compatibility
			// Caddy strips /api prefix, so /api/auth/login becomes /auth/login
			// We need to add /v1 prefix back
			path := r.URL.Path
			switch {
			case strings.HasPrefix(path, "/api/"):
				// Caddy didn't strip /api (direct access)
				r.URL.Path = "/v1" + path[4:]
			case path == "/healthz" || path == "/":
				// Health check or root — pass through as-is
			case !strings.HasPrefix(path, "/v1") && !strings.HasPrefix(path, "/swagger"):
				// Caddy stripped /api prefix, add /v1
				r.URL.Path = "/v1" + path
			}
			router.ServeHTTP(w, r)
		}),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("server starting", map[string]interface{}{"addr": addr})
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server failed", map[string]interface{}{"error": err.Error()})
			os.Exit(1)
		}
	}()

	<-quit
	log.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("server forced to shutdown", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	log.Info("server exited")
}
