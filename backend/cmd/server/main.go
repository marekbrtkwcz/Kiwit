package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kiwit/kiwit/internal/attachment"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/comment"
	"github.com/kiwit/kiwit/internal/issuetracker"
	"github.com/kiwit/kiwit/internal/management"
	"github.com/kiwit/kiwit/internal/notify"
	"github.com/kiwit/kiwit/internal/platform/config"
	"github.com/kiwit/kiwit/internal/platform/db"
	"github.com/kiwit/kiwit/internal/telemetry"
	"github.com/kiwit/kiwit/internal/testcase"
	"github.com/kiwit/kiwit/internal/testplan"
	"github.com/kiwit/kiwit/internal/testrun"
	"github.com/kiwit/kiwit/internal/web"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var authSvc *auth.Service
	var commentRepo comment.Repository
	var sqlDBConn *sql.DB
	var pgPoolConn *pgxpool.Pool
	var attachmentRepo attachment.Repository
	var issueRepo issuetracker.Repository
	var productRepo management.ProductRepository
	var testCaseRepo testcase.Repository
	var testPlanRepo testplan.Repository
	var testRunRepo testrun.Repository
	if strings.HasPrefix(cfg.DatabaseURL, "sqlite://") {
		sqlDB, err := db.OpenSQLite(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Error("failed to initialize sqlite", "err", err)
			os.Exit(1)
		}
		defer func() { _ = sqlDB.Close() }()
		sqlDBConn = sqlDB
		authSvc = auth.NewService(sqlDB, cfg.JWTSecret)
		commentRepo = comment.NewSQLiteRepository(sqlDB)
		attachmentRepo = attachment.NewSQLiteRepository(sqlDB)
		issueRepo = issuetracker.NewSQLiteRepository(sqlDB)
		productRepo = management.NewSQLiteRepository(sqlDB)
		testCaseRepo = testcase.NewSQLiteRepository(sqlDB)
		testPlanRepo = testplan.NewSQLiteRepository(sqlDB)
		testRunRepo = testrun.NewSQLiteRepository(sqlDB)
		logger.Info("sqlite database ready", "dsn", cfg.DatabaseURL)
	} else if strings.HasPrefix(cfg.DatabaseURL, "postgres://") || strings.HasPrefix(cfg.DatabaseURL, "postgresql://") {
		pgPool, err := db.NewPool(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Error("failed to initialize postgres", "err", err)
			os.Exit(1)
		}
		defer pgPool.Close()
		pgPoolConn = pgPool
		authSvc = auth.NewPostgresService(pgPool, cfg.JWTSecret)
		attachmentRepo = attachment.NewPostgresRepository(pgPool)
		commentRepo = comment.NewPostgresRepository(pgPool)
		issueRepo = issuetracker.NewPostgresRepository(pgPool)
		productRepo = management.NewPostgresRepository(pgPool)
		testCaseRepo = testcase.NewPostgresRepository(pgPool)
		testPlanRepo = testplan.NewPostgresRepository(pgPool)
		testRunRepo = testrun.NewPostgresRepository(pgPool)
		logger.Info("postgres database ready")
	}

	router := chi.NewMux()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	if authSvc != nil {
		router.Use(authSvc.RequireAuth)
	}

	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
			"env":    cfg.Env,
		})
	})

	api := humachi.New(router, huma.DefaultConfig("Kiwit TCMS API", "0.1.0"))
	auth.RegisterRoutes(api, authSvc)
	management.RegisterRoutes(api, productRepo)
	testcase.RegisterRoutes(api, testCaseRepo)
	testplan.RegisterRoutes(api, testPlanRepo)
	testrun.RegisterRoutes(api, testRunRepo)
	testrun.RegisterExportRoutes(router, testRunRepo)
	issuetracker.RegisterRoutes(api, issueRepo)
	telemetry.RegisterRoutes(api, sqlDBConn, pgPoolConn)
	comment.RegisterRoutes(api, commentRepo)
	notify.RegisterRoutes(api, notify.NewWebhookClient(os.Getenv("KIWIT_WEBHOOK_URL")))
	attachment.RegisterRoutes(router, attachmentRepo, "./uploads")

	router.Handle("/*", web.SPAHandler())

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ln, actualAddr, err := listenWithFallback(cfg.Addr, os.Getenv("KIWIT_ADDR") != "")
	if err != nil {
		logger.Error("failed to bind port", "err", err)
		os.Exit(1)
	}

	go func() {
		logger.Info("starting kiwit server", "addr", actualAddr, "env", cfg.Env)
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down kiwit server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
	}
}

func listenWithFallback(defaultAddr string, explicit bool) (net.Listener, string, error) {
	if explicit {
		ln, err := net.Listen("tcp", defaultAddr)
		return ln, defaultAddr, err
	}
	for port := 8080; port <= 8180; port++ {
		addr := fmt.Sprintf(":%d", port)
		if ln, err := net.Listen("tcp", addr); err == nil {
			return ln, addr, nil
		}
	}
	return nil, "", fmt.Errorf("no free port found in range 8080-8180")
}
