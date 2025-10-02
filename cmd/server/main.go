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

// initDatabase 初始化数据库连接
func initDatabase(cfg *config.Config) *gorm.DB {

	sqlCfg := cfg.Mysql
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=UTC",
		sqlCfg.Username, sqlCfg.Password, sqlCfg.Host, sqlCfg.Port, sqlCfg.DBName)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		PrepareStmt: true, // 开启预编译提升性能
		NowFunc: func() time.Time {
			return time.Now().UTC() // 写入用 UTC
		},
	})
	if err != nil {
		log.Fatal("Could not connect to the database", err)
	}

	// 获取底层*sql.DB对象并配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Get underlying sql.DB failed", err)
	}

	// 关键连接池配置
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetMaxIdleConns(20)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	// 添加连接健康检查
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

// initServices 初始化服务容器
func initServices(db *gorm.DB, cfg *config.Config) *services.SimpleServiceContainer {
	// 初始化地图API服务
	services.InitService()

	// 创建数据访问层
	repo := dao.NewMysqlRepository(db)

	// 创建命令管理器
	cmdM := services.NewCommandManager()

	// 创建简化的服务容器
	serviceContainer := services.NewSimpleServiceContainer(repo, cmdM)

	// 初始化厂商驱动
	initVendorDrivers(serviceContainer, cfg, repo)

	return serviceContainer
}

// initVendorDrivers 初始化厂商驱动
func initVendorDrivers(serviceContainer *services.SimpleServiceContainer, cfg *config.Config, repo dao.Repository) {

	bttDriver := btt.NewMqttHandler(btt.MqttConfig(cfg.Mqtt), services.NewBttTopicProvider(repo))
	serviceContainer.RegisterDriver("btt", bttDriver)
	v53Driver := v53.NewV53Handler(5353)
	serviceContainer.RegisterDriver("v53", v53Driver)

	// 设置消息处理器
	messageProcessor := handlers.NewMessageProcessor(repo, services.NewWsManager(time.Minute*10), services.NewCommandManager())
	serviceContainer.SetMessageHandler(messageProcessor)

	serviceContainer.StartAllDrivers()
}

// startServer 启动HTTP服务器
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

	// 设置日志级别
	setupLogging(cfg.Loglevel)

	// 初始化数据库连接
	db := initDatabase(cfg)

	// 初始化服务容器
	serviceContainer := initServices(db, cfg)

	// 设置路由
	router := setupRoutes(serviceContainer)

	// 启动HTTP服务器
	startServer(router, cfg)
}

// setupRoutes 设置路由
func setupRoutes(serviceContainer *services.SimpleServiceContainer) *mux.Router {
	r := mux.NewRouter()

	// 创建处理器
	h := handlers.NewSimpleHandler(serviceContainer, services.NewWsManager(time.Minute*10),
		services.NewJWTService(), services.NewWechatService())
	// 设置中间件
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
	r.HandleFunc("/api/v1/devices", handlers.WithMidWare(h.EnrollDeviceHandler, midWares...)).Methods("POST") // 注册设备
	// 统一头像路由
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
