package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	"github.com/kiwit/kiwit/internal/ai"
	"github.com/kiwit/kiwit/internal/attachment"
	"github.com/kiwit/kiwit/internal/audit"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/bug"
	"github.com/kiwit/kiwit/internal/campaign"
	"github.com/kiwit/kiwit/internal/captcha"
	"github.com/kiwit/kiwit/internal/classification"
	"github.com/kiwit/kiwit/internal/docs"
	"github.com/kiwit/kiwit/internal/comment"
	"github.com/kiwit/kiwit/internal/customfield"
	"github.com/kiwit/kiwit/internal/dashboard"
	"github.com/kiwit/kiwit/internal/dataset"
	"github.com/kiwit/kiwit/internal/environment"
	"github.com/kiwit/kiwit/internal/exploratory"
	"github.com/kiwit/kiwit/internal/group"
	"github.com/kiwit/kiwit/internal/initdb"
	"github.com/kiwit/kiwit/internal/issuetracker"
	"github.com/kiwit/kiwit/internal/linkref"
	"github.com/kiwit/kiwit/internal/llm"
	"github.com/kiwit/kiwit/internal/management"
	"github.com/kiwit/kiwit/internal/markdown"
	"github.com/kiwit/kiwit/internal/menu"
	"github.com/kiwit/kiwit/internal/milestone"
	"github.com/kiwit/kiwit/internal/notification"
	"github.com/kiwit/kiwit/internal/notify"
	"github.com/kiwit/kiwit/internal/permissions"
	"github.com/kiwit/kiwit/internal/plantype"
	"github.com/kiwit/kiwit/internal/platform/config"
	"github.com/kiwit/kiwit/internal/platform/db"
	"github.com/kiwit/kiwit/internal/platform/logging"
	"github.com/kiwit/kiwit/internal/platform/secrets"
	"github.com/kiwit/kiwit/internal/plugin"
	"github.com/kiwit/kiwit/internal/property"
	"github.com/kiwit/kiwit/internal/report"
	"github.com/kiwit/kiwit/internal/requirement"
	"github.com/kiwit/kiwit/internal/search"
	"github.com/kiwit/kiwit/internal/settings"
	"github.com/kiwit/kiwit/internal/storage"
	"github.com/kiwit/kiwit/internal/telemetry"
	"github.com/kiwit/kiwit/internal/testcase"
	"github.com/kiwit/kiwit/internal/testplan"
	"github.com/kiwit/kiwit/internal/testrun"
	"github.com/kiwit/kiwit/internal/version"
	"github.com/kiwit/kiwit/internal/visual"
	"github.com/kiwit/kiwit/internal/web"
	"github.com/kiwit/kiwit/internal/webhook"
	"github.com/kiwit/kiwit/internal/xmlrpc"
)

func main() {
	cfg := config.Load()

	// Logging: level and format from env, optional rotating file output,
	// plus an in-memory ring for the admin log viewer.
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	out := io.Writer(os.Stdout)
	if cfg.LogFile != "" {
		fw, err := logging.NewRotateWriter(cfg.LogFile, int64(cfg.LogMaxSizeMB)*1024*1024, cfg.LogMaxFiles)
		if err != nil {
			fmt.Fprintln(os.Stderr, "failed to open KIWIT_LOG_FILE:", err)
			os.Exit(1)
		}
		out = io.MultiWriter(os.Stdout, fw)
	}
	// Secrets known at startup are scrubbed from all log sinks (stdout, file,
	// admin log viewer). Runtime secrets (SMTP, webhooks, LLM, reset tokens)
	// register themselves in the vault where they are read.
	secrets.DefaultVault().Register(cfg.JWTSecret, cfg.S3SecretKey, cfg.LDAPBindPassword, os.Getenv("KIWIT_LLM_API_KEY"))
	out = secrets.DefaultVault().Writer(out)
	opts := &slog.HandlerOptions{Level: level}
	var base slog.Handler
	if cfg.LogFormat == "text" {
		base = slog.NewTextHandler(out, opts)
	} else {
		base = slog.NewJSONHandler(out, opts)
	}
	logRing := logging.NewRing(2000, level)
	logger := slog.New(logging.NewFanout(base, logRing))
	slog.SetDefault(logger)

	if err := cfg.Validate(); err != nil {
		logger.Error("invalid configuration", "err", err)
		os.Exit(1)
	}

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
	} else {
		logger.Error("unsupported KIWIT_DATABASE_URL scheme (expected sqlite:// or postgres://)", "url", cfg.DatabaseURL)
		os.Exit(1)
	}

	authSvc.SetAllowRegistration(cfg.AllowRegistration)

	// Anonymous read-only access (KIWIT_ANONYMOUS_ACCESS=true) — equivalent of
	// kiwitcms AnonymousViewBackend: unauthenticated GET requests get AnonymousUser.
	anonymousAccess := strings.EqualFold(os.Getenv("KIWIT_ANONYMOUS_ACCESS"), "true")
	authSvc.SetAnonymousAccess(anonymousAccess)
	if anonymousAccess {
		logger.Info("anonymous read-only access enabled (KIWIT_ANONYMOUS_ACCESS=true)")
	}

	// Self-hosted captcha store (routes mounted after api is created below)
	captchaEnabled := strings.EqualFold(os.Getenv("KIWIT_CAPTCHA_ENABLED"), "true") ||
		(os.Getenv("KIWIT_CAPTCHA_ENABLED") == "" && cfg.AllowRegistration)
	var captchaStore *captcha.Store
	if captchaEnabled && cfg.AllowRegistration {
		captchaStore = captcha.NewStore(10*time.Minute, 2*time.Second)
		authSvc.SetCaptcha(captchaStore)
	}

	router := chi.NewMux()
	router.Use(middleware.RequestID)
	router.Use(web.RequestIDMiddleware)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(requestLogger(logger))
	rateLimiter := auth.NewRateLimiter(20, time.Minute)
	for _, cidr := range cfg.TrustedProxies {
		if cidr = strings.TrimSpace(cidr); cidr != "" {
			rateLimiter.SetTrustedProxies(cidr)
		}
	}
	router.Use(rateLimiter.LimitAuthRoutes)
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

	guard := auth.NewAccessGuard(sqlDBConn, pgPoolConn)
	emailSettings := notify.NewSettingsStore(sqlDBConn, pgPoolConn)
	emailClient := notify.NewEmailClient(notify.LoadSMTPConfigFromEnv())
	// Password reset needs the e-mail channel (or logs the link without SMTP)
	// and the public base URL for the reset link.
	authSvc.SetEmailSender(emailClient)
	authSvc.SetBaseURL(cfg.BaseURL)

	// Instance settings store — load domain from DB if set (overrides env)
	settingsStore := settings.NewStore(sqlDBConn, pgPoolConn)
	if dbDomain, err := settingsStore.Get(context.Background(), settings.KeyDomain); err == nil && dbDomain != "" {
		authSvc.SetBaseURL(dbDomain)
		logger.Info("using domain from database", "domain", dbDomain)
	}
	if cfg.LDAPEnabled() {
		authSvc.SetLDAPConfig(&auth.LDAPConfig{
			Server:             cfg.LDAPServer,
			BindDN:             cfg.LDAPBindDN,
			BindPassword:       cfg.LDAPBindPassword,
			BaseDN:             cfg.LDAPBaseDN,
			UserFilter:         cfg.LDAPUserFilter,
			UsernameAttr:       cfg.LDAPUsernameAttr,
			EmailAttr:          cfg.LDAPEmailAttr,
			StartTLS:           cfg.LDAPStartTLS,
			InsecureSkipVerify: cfg.LDAPInsecureSkipVerify,
		})
		logger.Info("LDAP authentication enabled", "server", cfg.LDAPServer, "base_dn", cfg.LDAPBaseDN)
	}

	// SSO (OIDC/OAuth2) — optional, enabled when KIWIT_OIDC_ISSUER is set.
	if cfg.OIDCEnabled() {
		redirectURL := cfg.OIDCRedirectURL
		if redirectURL == "" {
			redirectURL = cfg.BaseURL + "/api/v1/auth/sso/callback"
		}
		ssoHandler, err := auth.NewSSOHandler(ctx, auth.SSOConfig{
			Issuer:       cfg.OIDCIssuer,
			ClientID:     cfg.OIDCClientID,
			ClientSecret: cfg.OIDCClientSecret,
			RedirectURL:  redirectURL,
			ProviderName: cfg.OIDCProviderName,
			FrontendURL:  cfg.BaseURL,
		}, authSvc)
		if err != nil {
			logger.Error("failed to initialize SSO provider", "err", err)
			os.Exit(1)
		}
		ssoHandler.Register(router)
		logger.Info("SSO (OIDC) enabled", "issuer", cfg.OIDCIssuer, "provider", cfg.OIDCProviderName)
	}

	humaConfig := huma.DefaultConfig("Kiwit TCMS API", "0.1.0")
	humaConfig.Transformers = append(humaConfig.Transformers, web.ErrorRequestIDTransformer)
	api := humachi.New(router, humaConfig)
	auth.RegisterRoutes(api, authSvc)
	if captchaStore != nil {
		captcha.RegisterRoutes(api, captchaStore)
	}
	group.RegisterRoutes(api, group.NewStore(sqlDBConn, pgPoolConn))
	property.RegisterRoutes(api, property.NewStore(sqlDBConn, pgPoolConn), guard)
	plantype.RegisterRoutes(api, plantype.NewStore(sqlDBConn, pgPoolConn))
	classification.RegisterRoutes(api, classification.NewStore(sqlDBConn, pgPoolConn))
	linkref.RegisterRoutes(api, linkref.NewStore(sqlDBConn, pgPoolConn), guard)
	search.RegisterRoutes(api, search.NewStore(sqlDBConn, pgPoolConn))
	markdown.RegisterRoutes(api)
	management.RegisterRoutes(api, productRepo, guard)
	management.RegisterRelationRoutes(api, management.NewRelationStore(sqlDBConn, pgPoolConn), guard)
	auditStore := audit.NewStore(sqlDBConn, pgPoolConn)
	audit.RegisterRoutes(api, auditStore, guard)
	dashboard.RegisterRoutes(api, dashboard.NewStore(sqlDBConn, pgPoolConn))
	testcase.RegisterRoutes(api, testCaseRepo, emailSettings, emailClient, guard, auditStore)
	requirement.RegisterRoutes(api, requirement.NewStore(sqlDBConn, pgPoolConn), guard)
	milestone.RegisterRoutes(api, milestone.NewStore(sqlDBConn, pgPoolConn), guard)
	dataset.RegisterRoutes(api, dataset.NewStore(sqlDBConn, pgPoolConn), guard)
	customfield.RegisterRoutes(api, customfield.NewStore(sqlDBConn, pgPoolConn), guard)
	exploratory.RegisterRoutes(api, exploratory.NewStore(sqlDBConn, pgPoolConn), guard)
	testcase.RegisterTemplateRoutes(api, sqlDBConn, pgPoolConn)
	testplan.RegisterRoutes(api, testPlanRepo, emailSettings, emailClient, guard, auditStore)
	notifier := notify.NewWebhookClient(os.Getenv("KIWIT_WEBHOOK_URL"))
	notificationStore := notification.NewStore(sqlDBConn, pgPoolConn)
	webhookStore := webhook.NewStore(sqlDBConn, pgPoolConn)
	// Local development may need webhooks against loopback targets; production
	// keeps the SSRF dial guard on (block loopback/private/link-local IPs).
	switch strings.ToLower(strings.TrimSpace(os.Getenv("KIWIT_WEBHOOK_ALLOW_PRIVATE"))) {
	case "1", "true", "yes":
		webhook.AllowPrivateTargets = true
	}
	testrun.RegisterRoutes(api, testRunRepo, notifier, notificationStore, emailClient, webhookStore, guard, auditStore)
	testrun.RegisterExportRoutes(router, testRunRepo, guard)
	testcase.RegisterExportRoutes(router, testCaseRepo, guard)
	testcase.RegisterImportRoutes(router, testCaseRepo, guard, auditStore)
	testplan.RegisterExportRoutes(router, testPlanRepo)
	requirement.RegisterExportRoutes(router, requirement.NewStore(sqlDBConn, pgPoolConn))
	milestone.RegisterExportRoutes(router, milestone.NewStore(sqlDBConn, pgPoolConn), guard)
	bug.RegisterExportRoutes(router, bug.NewStore(sqlDBConn, pgPoolConn))
	audit.RegisterExportRoutes(router, auditStore)
	telemetry.RegisterExportRoutes(router, sqlDBConn, pgPoolConn)
	issuetracker.RegisterRoutes(api, issueRepo, guard)
	bug.RegisterRoutes(api, bug.NewStore(sqlDBConn, pgPoolConn), guard, webhookStore)
	telemetry.RegisterRoutes(api, sqlDBConn, pgPoolConn)
	comment.RegisterRoutes(api, commentRepo, guard)
	notify.RegisterRoutes(api, notifier)
	notify.RegisterEmailRoutes(api, emailSettings, emailClient, guard)
	notification.RegisterRoutes(api, notificationStore)
	webhook.RegisterRoutes(api, webhookStore)
	environment.RegisterRoutes(api, environment.NewStore(sqlDBConn, pgPoolConn))
	pluginReg := plugin.NewRegistry()
	pluginReg.InitAll(api)
	plugin.RegisterRoutes(api, pluginReg)
	var store storage.Storage
	if cfg.StorageType == "s3" {
		if cfg.S3Bucket == "" || cfg.S3AccessKey == "" || cfg.S3SecretKey == "" {
			logger.Error("KIWIT_STORAGE_TYPE=s3 requires KIWIT_S3_BUCKET, KIWIT_S3_ACCESS_KEY and KIWIT_S3_SECRET_KEY")
			os.Exit(1)
		}
		store = storage.NewS3(storage.S3Config{
			Endpoint:     cfg.S3Endpoint,
			Region:       cfg.S3Region,
			Bucket:       cfg.S3Bucket,
			AccessKey:    cfg.S3AccessKey,
			SecretKey:    cfg.S3SecretKey,
			UsePathStyle: cfg.S3UsePathStyle,
		})
		logger.Info("using S3 object storage", "endpoint", cfg.S3Endpoint, "bucket", cfg.S3Bucket)
	} else {
		store = storage.NewLocal(cfg.UploadDir)
	}
	attachment.RegisterRoutes(router, attachmentRepo, store, guard)
	visual.RegisterRoutes(api, visual.NewStore(sqlDBConn, pgPoolConn), attachmentRepo, store, guard)
	report.RegisterRoutes(api, router, report.NewStore(sqlDBConn, pgPoolConn), guard)
	campaign.RegisterRoutes(api, campaign.NewStore(sqlDBConn, pgPoolConn), guard)
	ai.RegisterRoutes(api, llm.NewServerStore(sqlDBConn, pgPoolConn), testCaseRepo, requirement.NewStore(sqlDBConn, pgPoolConn), guard)
	ai.RegisterServerRoutes(api, llm.NewServerStore(sqlDBConn, pgPoolConn))
	logging.RegisterRoutes(api, logRing)
	ai.RegisterExtraRoutes(api, llm.NewServerStore(sqlDBConn, pgPoolConn), testRunRepo, guard)
	ai.RegisterSearchRoutes(api, llm.NewServerStore(sqlDBConn, pgPoolConn), sqlDBConn, pgPoolConn)
	ai.RegisterVisualRoutes(api, llm.NewServerStore(sqlDBConn, pgPoolConn), visual.NewStore(sqlDBConn, pgPoolConn), attachmentRepo, store, guard)
	ai.RegisterExploratoryRoutes(api, llm.NewServerStore(sqlDBConn, pgPoolConn), exploratory.NewStore(sqlDBConn, pgPoolConn), guard)
	ai.RegisterRequirementRoutes(api, llm.NewServerStore(sqlDBConn, pgPoolConn), requirement.NewStore(sqlDBConn, pgPoolConn), guard)
	ai.RegisterBugRoutes(api, llm.NewServerStore(sqlDBConn, pgPoolConn), bug.NewStore(sqlDBConn, pgPoolConn), guard)
	ai.RegisterReportRoutes(api, llm.NewServerStore(sqlDBConn, pgPoolConn), report.NewStore(sqlDBConn, pgPoolConn), guard)
	ai.RegisterDatasetRoutes(api, llm.NewServerStore(sqlDBConn, pgPoolConn), testCaseRepo, dataset.NewStore(sqlDBConn, pgPoolConn), customfield.NewStore(sqlDBConn, pgPoolConn), guard)
	ai.RegisterDashboardRoutes(api, llm.NewServerStore(sqlDBConn, pgPoolConn), sqlDBConn, pgPoolConn, guard)
	xmlrpc.NewHandlerWithWebhooks(authSvc, sqlDBConn, pgPoolConn, version.VERSION, cfg.UploadDir, webhookStore).Register(router)
	initdb.Register(router, initdb.NewStore(sqlDBConn, pgPoolConn))
	menu.Register(router)
	docs.Register(router)
	permissions.Register(router, permissions.NewStore(sqlDBConn, pgPoolConn))
	settings.Register(router, settingsStore)
	version.Register(router)

	// Runtime-changeable settings: wire DB changes to live service state
	settings.OnChange(settings.KeyDomain, func(domain string) {
		authSvc.SetBaseURL(domain)
		logger.Info("domain updated at runtime", "domain", domain)
	})
	// SMTP: rebuild config from all smtp_* keys on any change
	applySMTP := func() {
		ctx := context.Background()
		host, _ := settingsStore.Get(ctx, settings.KeySMTPHost)
		port, _ := settingsStore.Get(ctx, settings.KeySMTPPort)
		user, _ := settingsStore.Get(ctx, settings.KeySMTPUser)
		pass, _ := settingsStore.Get(ctx, settings.KeySMTPPass)
		from, _ := settingsStore.Get(ctx, settings.KeySMTPFrom)
		if port == "" {
			port = "587"
		}
		if from == "" {
			from = "kiwit@localhost"
		}
		emailClient.SetConfig(notify.SMTPConfig{Host: host, Port: port, Username: user, Password: pass, From: from})
		secrets.DefaultVault().Register(pass)
	}
	for _, k := range []string{settings.KeySMTPHost, settings.KeySMTPPort, settings.KeySMTPUser, settings.KeySMTPPass, settings.KeySMTPFrom} {
		settings.OnChange(k, func(string) { applySMTP() })
	}
	settings.OnChange(settings.KeyAllowRegistration, func(v string) {
		authSvc.SetAllowRegistration(settings.BoolVal(v))
		logger.Info("allow_registration updated at runtime", "value", v)
	})
	settings.OnChange(settings.KeyCaptchaEnabled, func(v string) {
		if settings.BoolVal(v) && captchaStore != nil {
			authSvc.SetCaptcha(captchaStore)
		} else {
			authSvc.SetCaptcha(nil)
		}
		logger.Info("captcha_enabled updated at runtime", "value", v)
	})
	settings.OnChange(settings.KeyAnonymousAccess, func(v string) {
		authSvc.SetAnonymousAccess(settings.BoolVal(v))
		logger.Info("anonymous_access updated at runtime", "value", v)
	})

	// Load initial values from DB (override env defaults)
	loadSettingsFromDB := func() {
		ctx := context.Background()
		if v, err := settingsStore.Get(ctx, settings.KeyDomain); err == nil && v != "" {
			authSvc.SetBaseURL(v)
		}
		if v, err := settingsStore.Get(ctx, settings.KeyAllowRegistration); err == nil && v != "" {
			authSvc.SetAllowRegistration(settings.BoolVal(v))
		}
		if v, err := settingsStore.Get(ctx, settings.KeyCaptchaEnabled); err == nil && v != "" {
			if settings.BoolVal(v) && captchaStore != nil {
				authSvc.SetCaptcha(captchaStore)
			} else {
				authSvc.SetCaptcha(nil)
			}
		}
		if v, err := settingsStore.Get(ctx, settings.KeyAnonymousAccess); err == nil && v != "" {
			authSvc.SetAnonymousAccess(settings.BoolVal(v))
		}
		// SMTP from DB overrides env
		host, _ := settingsStore.Get(ctx, settings.KeySMTPHost)
		if host != "" {
			applySMTP()
		}
	}
	loadSettingsFromDB()

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

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			dur := time.Since(start)
			fields := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", dur.Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
				"remote_addr", r.RemoteAddr,
			}
			// Slow requests are promoted to WARN so they stand out in the log viewer.
			if dur >= time.Second {
				logger.Warn("slow request", fields...)
				return
			}
			logger.Info("http request", fields...)
		})
	}
}
