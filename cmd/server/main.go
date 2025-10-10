package main

import (
	"fmt"
	"log"
	"log/slog"
	"strings"
	"time"

	"github.com/Daneel-Li/gps-back/internal/config"
	"github.com/Daneel-Li/gps-back/internal/dao"
	"github.com/Daneel-Li/gps-back/internal/handlers"
	"github.com/Daneel-Li/gps-back/internal/services"
	"github.com/Daneel-Li/gps-back/internal/vendors/btt"
	v53 "github.com/Daneel-Li/gps-back/internal/vendors/v53"

	//	"reflect"
	"net/http"

	mux "github.com/gorilla/mux"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func setupLogging(logLevel string) {
	switch strings.ToLower(logLevel) {
	case "debug":
		slog.SetLogLoggerLevel(slog.LevelDebug)
	case "info":
		slog.SetLogLoggerLevel(slog.LevelInfo)
	case "warn":
		slog.SetLogLoggerLevel(slog.LevelWarn)
	case "error":
		slog.SetLogLoggerLevel(slog.LevelError)
	}
}

// initDatabase initializes database connection
func initDatabase(cfg *config.Config) *gorm.DB {

	sqlCfg := cfg.Mysql
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=UTC",
		sqlCfg.Username, sqlCfg.Password, sqlCfg.Host, sqlCfg.Port, sqlCfg.DBName)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		PrepareStmt: true, // Enable prepared statements to improve performance
		NowFunc: func() time.Time {
			return time.Now().UTC() // Use UTC for writing
		},
	})
	if err != nil {
		log.Fatal("Could not connect to the database", err)
	}

	// Get underlying *sql.DB object and configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Get underlying sql.DB failed", err)
	}

	// Key connection pool configuration
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetMaxIdleConns(20)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	// Add connection health check
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			if err := sqlDB.Ping(); err != nil {
				log.Printf("Database connection health check failed: %v", err)
			}
		}
	}()

	return db
}

// initServices initializes service container
func initServices(db *gorm.DB, cfg *config.Config) *services.SimpleServiceContainer {
	// Initialize map API service
	services.InitService()

	// Create data access layer
	repo := dao.NewMysqlRepository(db)

	// Create command manager
	cmdM := services.NewCommandManager()

	// Create simplified service container
	serviceContainer := services.NewSimpleServiceContainer(repo, cmdM)

	// Initialize vendor drivers
	initVendorDrivers(serviceContainer, cfg, repo)

	return serviceContainer
}

// initVendorDrivers initializes vendor drivers
func initVendorDrivers(serviceContainer *services.SimpleServiceContainer, cfg *config.Config, repo dao.Repository) {

	bttDriver := btt.NewMqttHandler(btt.MqttConfig(cfg.Mqtt), services.NewBttTopicProvider(repo))
	serviceContainer.RegisterDriver("btt", bttDriver)
	v53Driver := v53.NewV53Handler(5353)
	serviceContainer.RegisterDriver("v53", v53Driver)

	// Set message processor
	messageProcessor := handlers.NewMessageProcessor(repo, services.NewWsManager(time.Minute*10), services.NewCommandManager())
	serviceContainer.SetMessageHandler(messageProcessor)

	serviceContainer.StartAllDrivers()
}

// startServer starts HTTP server
func startServer(router *mux.Router, cfg *config.Config) {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", cfg.ServerPort),
		Handler: router,
	}

	slog.Info("Starting HTTPS server: " + server.Addr + "...")

	err := server.ListenAndServeTLS(cfg.Tls.CertPath, cfg.Tls.KeyPath)
	if err != nil {
		slog.Error("Failed to start HTTPS server: " + err.Error())
	}
}

func main() {

	cfg := config.GetConfig()

	// Set log level
	setupLogging(cfg.Loglevel)

	// Initialize database connection
	db := initDatabase(cfg)

	// Initialize service container
	serviceContainer := initServices(db, cfg)

	// Setup routes
	router := setupRoutes(serviceContainer)

	// Start HTTP server
	startServer(router, cfg)
}

// setupRoutes sets up routes
func setupRoutes(serviceContainer *services.SimpleServiceContainer) *mux.Router {
	r := mux.NewRouter()

	// Create handler
	h := handlers.NewSimpleHandler(serviceContainer, services.NewWsManager(time.Minute*10),
		services.NewJWTService(), services.NewWechatService())
	// Set middleware
	midWares := []handlers.Middleware{
		handlers.ApiAuthCheck,
		handlers.JWTMiddleware,
	}

	r.HandleFunc("/api/v1/login", handlers.WithMidWare(h.LoginHandler, []handlers.Middleware{handlers.ApiAuthCheck}...)).Methods("POST")

	r.HandleFunc("/api/v1/devices/{device_id}", handlers.WithMidWare(h.GetDevice, midWares...)).Methods("GET")
	r.HandleFunc("/api/v1/devices", handlers.WithMidWare(h.GetDevice, midWares...)).Methods("GET")
	r.HandleFunc("/api/v1/assignments", handlers.WithMidWare(h.BindDevice, midWares...)).Methods(("POST"))
	r.HandleFunc("/api/v1/assignments/{device_id}", handlers.WithMidWare(h.UnbindDevice, midWares...)).Methods(("DELETE"))

	r.HandleFunc("/api/v1/users/{user_id}", handlers.WithMidWare(h.UpdateUser, midWares...)).Methods(("PUT"))
	r.HandleFunc("/api/v1/users/{user_id}", handlers.WithMidWare(h.GetUser, midWares...)).Methods(("GET"))

	r.HandleFunc("/api/v1/devices/{device_id}/track", handlers.WithMidWare(h.GetTrack, midWares...)).Methods("GET")
	r.HandleFunc("/api/v1/devices/{device_id}/safearea", handlers.WithMidWare(h.GetSafeRegions, midWares...)).Methods("GET")
	r.HandleFunc("/api/v1/devices/{device_id}/safearea", handlers.WithMidWare(h.PutSafeRegion, midWares...)).Methods("PUT")
	r.HandleFunc("/api/v1/devices/{device_id}/interval", handlers.WithMidWare(h.GetReportInterval, midWares...)).Methods("GET")
	r.HandleFunc("/api/v1/devices/{device_id}/autopower", handlers.WithMidWare(h.GetAutoPower, midWares...)).Methods("GET")

	r.HandleFunc("/api/v1/sharemappings", handlers.WithMidWare(h.GetShareMappings, midWares...)).Methods("GET")
	r.HandleFunc("/api/v1/sharemappings", handlers.WithMidWare(h.CreateShareMapping, midWares...)).Methods("POST")
	r.HandleFunc("/api/v1/sharemappings", handlers.WithMidWare(h.MoveShareMapping, midWares...)).Methods("DELETE")
	r.HandleFunc("/api/v1/devices/{device_id}/command", handlers.WithMidWare(h.Command, midWares...)).Methods("PUT")
	r.HandleFunc("/api/v1/devices/{device_id}/activity", handlers.WithMidWare(h.GetSteps, midWares...)).Methods("GET")
	r.HandleFunc("/api/v1/devices/{device_id}/alarms", handlers.WithMidWare(h.GetAlarms, midWares...)).Methods("GET")
	r.HandleFunc("/api/v1/devices/{device_id}/profile", handlers.WithMidWare(h.UpdateProfile, midWares...)).Methods("PUT")
	r.HandleFunc("/api/v1/devices/{device_id}/profile", handlers.WithMidWare(h.GetProfile, midWares...)).Methods("GET")
	r.HandleFunc("/api/v1/devices", handlers.WithMidWare(h.EnrollDeviceHandler, midWares...)).Methods("POST") // Register device
	// Unified avatar routes
	r.HandleFunc("/api/v1/{target:users|devices}/{id}/avatar",
		handlers.WithMidWare(h.GetAvatar, midWares...)).Methods("GET")
	r.HandleFunc("/api/v1/{target:users|devices}/{id}/avatar",
		handlers.WithMidWare(h.UploadFileHandler, midWares...)).Methods("POST")

	r.HandleFunc("/api/v1/feedbacks",
		handlers.WithMidWare(h.AddFeedback, midWares...)).Methods("POST")
	r.HandleFunc("/api/v1/feedbacks",
		handlers.WithMidWare(h.GetFeedbacks, midWares...)).Methods("GET")
	r.HandleFunc("/api/v1/upload", handlers.WithMidWare(h.UploadFileHandler, midWares...)).Methods("POST")
	r.HandleFunc("/ws", h.UpgradeWS).Methods("GET")

	r.HandleFunc("/api/v1/devices/{device_id}/paysuccess", h.PaySuccNotify).Methods("POST")
	r.HandleFunc("/api/v1/devices/{device_id}/renew",
		handlers.WithMidWare(h.Renew, midWares...)).Methods("POST")
	return r
}
