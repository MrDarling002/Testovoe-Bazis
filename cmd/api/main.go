package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/example/Testovoe-Bazis/internal/config"
	"github.com/example/Testovoe-Bazis/internal/email"
	"github.com/example/Testovoe-Bazis/internal/httpapi"
	"github.com/example/Testovoe-Bazis/internal/metrics"
	"github.com/example/Testovoe-Bazis/internal/ratelimit"
	"github.com/example/Testovoe-Bazis/internal/service"
	mysqlstore "github.com/example/Testovoe-Bazis/internal/store/mysql"
	redisstore "github.com/example/Testovoe-Bazis/internal/store/redis"
)

func main() {
	cfg := config.MustLoad()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db := mysqlstore.NewDB(cfg.DB)
	defer db.Close()

	rdb := redisstore.NewClient(cfg.Redis)
	defer rdb.Close()

	m := metrics.New()

	limiter := ratelimit.NewRedisLimiter(rdb, cfg.RateLimit.RequestsPerMinute)

	emailSender := email.NewHTTPSender(cfg.Email, m)

	userRepo := mysqlstore.NewUserRepository(db)
	teamRepo := mysqlstore.NewTeamRepository(db)
	taskRepo := mysqlstore.NewTaskRepository(db)
	analyticsRepo := mysqlstore.NewAnalyticsRepository(db)

	taskCache := redisstore.NewTaskCache(rdb, cfg.Cache.TasksTTL)

	authService := service.NewAuthService(userRepo, cfg.JWT.Secret, cfg.JWT.TTL)
	teamService := service.NewTeamService(teamRepo, userRepo, emailSender, logger)
	taskService := service.NewTaskService(taskRepo, teamRepo, taskCache)
	analyticsService := service.NewAnalyticsService(analyticsRepo)

	handler := httpapi.NewHandler(
		authService,
		teamService,
		taskService,
		analyticsService,
		m,
		limiter,
		cfg.JWT.Secret,
		logger,
	)

	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           handler.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		logger.Info("server started", "addr", cfg.Server.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Info("shutdown started")

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	logger.Info("server stopped")
}
