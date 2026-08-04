package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/example/Testovoe-Bazis/internal/auth"
	"github.com/example/Testovoe-Bazis/internal/config"
	"github.com/example/Testovoe-Bazis/internal/email"
	"github.com/example/Testovoe-Bazis/internal/httpapi"
	"github.com/example/Testovoe-Bazis/internal/metrics"
	"github.com/example/Testovoe-Bazis/internal/ratelimit"
	"github.com/example/Testovoe-Bazis/internal/service"
	mysqlstore "github.com/example/Testovoe-Bazis/internal/store/mysql"
	"github.com/example/Testovoe-Bazis/internal/store/rediscache"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	if err := run(logger); err != nil {
		logger.Error("service failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := mysqlstore.NewDB(ctx, mysqlstore.DBConfig{
		DSN:             cfg.DB.DSN,
		MaxOpenConns:    cfg.DB.MaxOpenConns,
		MaxIdleConns:    cfg.DB.MaxIdleConns,
		ConnMaxLifetime: cfg.DB.ConnMaxLifetime.Std(),
	})
	if err != nil {
		return fmt.Errorf("connect mysql: %w", err)
	}
	defer db.Close()

	if err := mysqlstore.Migrate(cfg.DB.DSN); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	logger.Info("migrations applied")

	rdb, err := rediscache.NewClient(ctx, rediscache.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		return fmt.Errorf("connect redis: %w", err)
	}
	defer rdb.Close()

	m := metrics.New()
	metrics.RegisterDBStats(db.DB, "task_service")

	limiter := ratelimit.NewRedisLimiter(rdb, cfg.RateLimit.RequestsPerMinute)

	emailSender := email.NewHTTPSender(email.Config{
		BaseURL: cfg.Email.BaseURL,
		Timeout: cfg.Email.Timeout.Std(),
	}, m)

	jwtManager := auth.NewJWTManager(cfg.JWT.Secret, cfg.JWT.TTL.Std())

	userRepo := mysqlstore.NewUserRepository(db)
	teamRepo := mysqlstore.NewTeamRepository(db)
	taskRepo := mysqlstore.NewTaskRepository(db)
	analyticsRepo := mysqlstore.NewAnalyticsRepository(db)

	taskCache := rediscache.NewTaskCache(rdb, cfg.Cache.TasksTTL.Std(), m, logger)

	authService := service.NewAuthService(userRepo, jwtManager)
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
		jwtManager,
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

	errCh := make(chan error, 1)

	go func() {
		logger.Info("server started", "addr", cfg.Server.Addr)

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server: %w", err)
	case <-ctx.Done():
	}

	logger.Info("shutdown started")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	logger.Info("server stopped")

	return nil
}
