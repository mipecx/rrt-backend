package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
	"github.com/mipecx/rrt_system/backend/internal/config"
	"github.com/redis/go-redis/v9"

	authHttp "github.com/mipecx/rrt_system/backend/internal/domain/auth/delivery/http"
	authRepo "github.com/mipecx/rrt_system/backend/internal/domain/auth/repository"
	authService "github.com/mipecx/rrt_system/backend/internal/domain/auth/service"

	incidentHttp "github.com/mipecx/rrt_system/backend/internal/domain/incidents/delivery/http"
	incidentRepo "github.com/mipecx/rrt_system/backend/internal/domain/incidents/repository"
	incidentService "github.com/mipecx/rrt_system/backend/internal/domain/incidents/service"

	rrtHttp "github.com/mipecx/rrt_system/backend/internal/domain/rrt/delivery/http"
	rrtRepo "github.com/mipecx/rrt_system/backend/internal/domain/rrt/repository"
	rrtService "github.com/mipecx/rrt_system/backend/internal/domain/rrt/service"

	"github.com/mipecx/rrt_system/backend/internal/logger"
	"github.com/mipecx/rrt_system/backend/internal/middleware"
	"github.com/mipecx/rrt_system/backend/internal/ws"

	_ "github.com/golang-migrate/migrate/v4/source/file"
)

type application struct {
	cfg *config.Config
	db  *pgxpool.Pool
	rdb *redis.Client
	log *slog.Logger
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Error loading config", "error", err)
		return
	}
	log := logger.New(cfg.Env)
	slog.SetDefault(log)

	log.Info("Config loaded successfully", slog.String("env", cfg.Env))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reqCtx := logger.AppendCtx(ctx, logger.TraceIDKey, "REQ-TX-THAI-777")

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DB.Host, cfg.DB.Port, cfg.DB.User, cfg.DB.Password, cfg.DB.Name, cfg.DB.SSLMode)

	db, err := pgxpool.New(reqCtx, dsn)
	if err != nil {
		log.ErrorContext(reqCtx, "Не удалось создать пул подключений PostgreSQL", "error", err)
		return
	}
	defer db.Close()

	if err := db.Ping(reqCtx); err != nil {
		log.ErrorContext(reqCtx, "PostgreSQL не отвечает на пинг", "error", err)
		return
	}
	log.InfoContext(reqCtx, "Успешное защищенное подключение к PostgreSQL!")

	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
	})
	defer rdb.Close()

	if err := rdb.Ping(reqCtx).Err(); err != nil {
		log.ErrorContext(reqCtx, "Redis is not responding to ping", "error", err)
		return
	}
	log.InfoContext(reqCtx, "Successful secure connection to Redis!")

	app := &application{
		cfg: cfg,
		db:  db,
		rdb: rdb,
		log: log,
	}

	aRepo := authRepo.NewRepository(db, rdb)
	iRepo := incidentRepo.NewRepository(db, rdb)
	rRepo := rrtRepo.NewRepository(db)

	// TODO: smsSender пока не внедрен в наш AuthService, пропускаем его или храним на будущее
	// smsSender := auth.NewSMSProvider(cfg.SMS)

	authMiddleware := middleware.Auth([]byte(cfg.JWT.Secret))

	authService := authService.NewService(aRepo, cfg.JWT, log)
	authHandler := authHttp.NewHandler(authService, log)

	wsHub := ws.NewHub(cfg.CORS.AllowedOrigins)
	go wsHub.Run()

	incidentService := incidentService.NewService(iRepo, rRepo, db)
	incidentHandler := incidentHttp.NewHandler(incidentService, wsHub, authMiddleware)

	rrtService := rrtService.NewService(rRepo, db, log)
	rrtHandler := rrtHttp.NewHandler(rrtService, wsHub, log, authMiddleware)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", app.healthCheckHandler)

	mux.Handle("GET /api/v1/me",
		authMiddleware(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "success"})
			}),
		),
	)

	if os.Getenv("SIMULATOR_ENABLED") == "true" && cfg.IsDev() {
		rrtHttp.StartRRTSimulator(wsHub)
	}
	mux.HandleFunc("/api/v1/ws", wsHub.HandleWS)

	authHandler.RegisterRoutes(mux)
	incidentHandler.RegisterRoutes(mux)
	rrtHandler.RegisterRoutes(mux)

	server := &http.Server{
		Addr:         cfg.HTTPPort,
		Handler:      corsMiddleware(mux, cfg.CORS.AllowedOrigins),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Info(fmt.Sprintf("The rrt_system server is running on port %s...", cfg.HTTPPort))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Critical server error", "error", err)
		}
	}()

	<-shutdown
	log.Info("Shutting down server gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("Server forced to shutdown", "error", err)
	}

	log.Info("Server stopped successfully")
}

func (app *application) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	ctx := logger.AppendCtx(r.Context(), logger.TraceIDKey, "HEALTH-CHECK-ID-123")

	app.log.InfoContext(ctx, "The system health check endpoint was called.")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "available", "system": "RRT Core Structured Logs"}`))
}

func corsMiddleware(next http.Handler, allowedOrigins []string) http.Handler {
	originSet := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[o] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && originSet[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, ngrok-skip-browser-warning, X-Requested-With, Accept")
		w.Header().Set("Access-Control-Expose-Headers", "Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
