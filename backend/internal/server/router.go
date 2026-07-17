// Package server wires the HTTP router and shared middleware.
//
// Feature modules (identity, masterdata, asset, ...) register their routes here
// as they are implemented; see docs/PRD.md §7 for the module layout.
package server

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	apidocs "github.com/ragbuaj/inventra/api"
	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/asset"
	"github.com/ragbuaj/inventra/internal/assignment"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/auth"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/authzadmin"
	"github.com/ragbuaj/inventra/internal/cache"
	"github.com/ragbuaj/inventra/internal/config"
	"github.com/ragbuaj/inventra/internal/db"
	"github.com/ragbuaj/inventra/internal/depreciation"
	"github.com/ragbuaj/inventra/internal/disposal"
	"github.com/ragbuaj/inventra/internal/email"
	"github.com/ragbuaj/inventra/internal/geoip"
	"github.com/ragbuaj/inventra/internal/identity"
	"github.com/ragbuaj/inventra/internal/importer"
	"github.com/ragbuaj/inventra/internal/maintenance"
	"github.com/ragbuaj/inventra/internal/masterdata"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/masterdata/employee"
	"github.com/ragbuaj/inventra/internal/masterdata/office"
	"github.com/ragbuaj/inventra/internal/masterdata/reference"
	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/notification"
	"github.com/ragbuaj/inventra/internal/oauth"
	"github.com/ragbuaj/inventra/internal/ratelimit"
	"github.com/ragbuaj/inventra/internal/report"
	"github.com/ragbuaj/inventra/internal/search"
	"github.com/ragbuaj/inventra/internal/storage"
	"github.com/ragbuaj/inventra/internal/stockopname"
	"github.com/ragbuaj/inventra/internal/transfer"
	"github.com/ragbuaj/inventra/internal/user"
)

// Deps holds the shared infrastructure passed to feature modules.
type Deps struct {
	Cfg     *config.Config
	Pool    *pgxpool.Pool
	Redis   *redis.Client
	Log     *slog.Logger
	Limiter *ratelimit.Limiter
	Storage storage.Storage
	GeoIP   geoip.Locator
}

// Workers holds the background components the router constructs but does not
// start. They are grouped in a struct rather than added as extra return values:
// the count only grows as modules gain async work, the fields name themselves at
// the call site (five bare values would not), and a new worker becomes a new
// field instead of a signature change that churns every caller and test.
//
// Fields are nil-free once NewRouter returns; whether each one actually runs is
// main.go's decision, gated on config.
type Workers struct {
	// Import drains the bulk-import queue.
	Import *importer.Worker
	// Relay publishes the notification outbox onto the Redis Stream.
	Relay *notification.Relay
	// Consumer fans stream events out into per-user notification rows.
	Consumer *notification.Consumer
	// Sweeper enqueues maintenance-due reminders and purges past retention.
	Sweeper *notification.Sweeper
}

// NewRouter builds the Gin engine with base middleware, health, and readiness
// probes. It also returns the background workers so main.go can start (and
// cleanly stop) their polling loops alongside the HTTP server.
func NewRouter(d Deps) (*gin.Engine, Workers) {
	if d.Cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// Trust only configured proxies so c.ClientIP() (used by rate limiting) cannot be
	// spoofed via X-Forwarded-For. Empty TRUSTED_PROXIES → trust none (direct RemoteAddr);
	// set to the load-balancer CIDR(s) in production. On a bad value, fail safe (trust none).
	if err := r.SetTrustedProxies(d.Cfg.TrustedProxies); err != nil {
		if d.Log != nil {
			d.Log.Error("invalid TRUSTED_PROXIES; trusting no proxies", "error", err)
		}
		_ = r.SetTrustedProxies(nil)
	}

	r.Use(
		middleware.RequestID(),
		middleware.Metrics(),
		middleware.RequestLogger(d.Log),
		middleware.Recovery(d.Log),
		middleware.CORS(d.Cfg.FrontendURL),
	)

	// Not routed publicly by Caddy (only /api/* and /health are) — internal scrape only.
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Liveness — process is up; no external dependencies.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "inventra-api",
			"env":     d.Cfg.Env,
		})
	})

	// Readiness — verifies PostgreSQL and Redis are reachable.
	r.GET("/health/ready", func(c *gin.Context) {
		checks := gin.H{"postgres": "ok", "redis": "ok"}
		ready := true

		if err := db.Ping(c.Request.Context(), d.Pool); err != nil {
			checks["postgres"] = err.Error()
			ready = false
		}
		if err := cache.Ping(c.Request.Context(), d.Redis); err != nil {
			checks["redis"] = err.Error()
			ready = false
		}

		code := http.StatusOK
		status := "ready"
		if !ready {
			code = http.StatusServiceUnavailable
			status = "not_ready"
		}
		c.JSON(code, gin.H{"status": status, "checks": checks})
	})

	// API documentation: OpenAPI spec + self-hosted Scalar viewer.
	r.GET("/openapi.yaml", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", apidocs.SpecYAML)
	})
	r.GET("/docs", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(apidocs.ScalarHTML))
	})
	r.GET("/docs/scalar.js", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/javascript; charset=utf-8", apidocs.ScalarJS)
	})

	// Shared wiring for feature modules.
	queries := sqlc.New(d.Pool)
	tokenManager := auth.NewTokenManager(d.Cfg)
	tokenStore := auth.NewTokenStore(d.Redis)
	requireAuth := middleware.RequireAuth(tokenManager, tokenStore)

	// The workers are constructed inside the api group below (they need
	// permSvc/scopeSvc/approvalSvc/auditSvc built there) but must be visible
	// here so they can be returned to main.go for the graceful-shutdown wiring.
	var workers Workers

	api := r.Group("/api/v1")
	api.Use(middleware.PerIP(d.Limiter, d.Cfg.RateLimitGlobalPerMin, "global", false))
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		permSvc := authz.NewPermissionService(queries, d.Redis)
		scopeSvc := authz.NewScopeService(queries, d.Redis)
		fieldSvc := authz.NewFieldService(queries, d.Redis)
		auditSvc := audit.NewService(queries)

		googleOAuth, oerr := oauth.New(context.Background(), oauth.Config{
			ClientID:     d.Cfg.GoogleClientID,
			ClientSecret: d.Cfg.GoogleClientSecret,
			RedirectURL:  d.Cfg.GoogleRedirectURL,
			Issuer:       d.Cfg.GoogleIssuer,
		}, d.Redis)
		if oerr != nil {
			d.Log.Warn("google oauth disabled (discovery failed)", "error", oerr)
		}

		mailer := email.NewMailer(email.NewSender(email.Options{
			Enabled:  d.Cfg.MailEnabled,
			Host:     d.Cfg.SMTPHost,
			Port:     d.Cfg.SMTPPort,
			Username: d.Cfg.SMTPUsername,
			Password: d.Cfg.SMTPPassword,
			From:     d.Cfg.SMTPFrom,
			FromName: d.Cfg.SMTPFromName,
			TLS:      d.Cfg.SMTPTLS,
		}, slog.Default()))
		asyncMailer := email.NewAsyncMailer(mailer, slog.Default())
		locator := d.GeoIP
		if locator == nil {
			locator = geoip.New(d.Cfg.GeoIPDBPath, d.Log)
		}
		identitySvc := identity.NewService(queries, tokenManager, tokenStore, asyncMailer, locator, d.Cfg.PasswordResetTTL, d.Cfg.FrontendURL)
		identityHandler := identity.NewHandler(identitySvc, permSvc, scopeSvc, d.Limiter, d.Cfg.RateLimitLoginPerMin, d.Cfg.Env == "production", d.Cfg.JWTRefreshTTL, googleOAuth, d.Cfg.FrontendURL, auditSvc, d.Cfg.RateLimitLoginPerMin)
		identity.RegisterRoutes(api, identityHandler, requireAuth, d.Limiter, d.Cfg.RateLimitLoginIPPerMin, d.Cfg.RateLimitRefreshPerMin, d.Cfg.RateLimitLoginIPPerMin, d.Cfg.RateLimitLoginPerMin)

		userHandler := user.NewHandler(user.NewService(queries), fieldSvc, auditSvc)
		user.RegisterRoutes(api, userHandler, requireAuth, middleware.RequirePermission(permSvc, "user.manage"))

		masterdata.RegisterRoutes(api, queries, d.Pool, permSvc, scopeSvc, fieldSvc, auditSvc, requireAuth)

		assetSvc := asset.NewService(queries, d.Pool, d.Storage, d.Cfg.AttachmentMaxBytes, d.Cfg.LabelLogoPath)
		assetHandler := asset.NewHandler(assetSvc, fieldSvc, common.ScopedDeps{Q: queries, Scope: scopeSvc}, auditSvc)
		asset.RegisterRoutes(api, assetHandler,
			requireAuth,
			middleware.RequirePermission(permSvc, "asset.view"),
			middleware.RequirePermission(permSvc, "asset.manage"),
		)

		auditHandler := audit.NewHandler(auditSvc, scopeSvc, queries)
		audit.RegisterRoutes(api, auditHandler, requireAuth, middleware.RequirePermission(permSvc, "audit.view"))

		searchSvc := search.NewService(queries)
		searchHandler := search.NewHandler(searchSvc, permSvc, common.ScopedDeps{Q: queries, Scope: scopeSvc})
		search.RegisterRoutes(api, searchHandler, requireAuth)

		approvalSvc := approval.NewService(queries, d.Pool, scopeSvc, d.Redis)
		approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeAssetCreate, assetSvc.CreateExecutor())
		depreciationSvc := depreciation.NewService(queries, d.Pool)
		disposalSvc := disposal.NewService(queries, d.Pool, approvalSvc, depreciationSvc)
		approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeAssetDisposal, disposalSvc.Executor())
		approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeValuationExclusion, assetSvc.ExclusionExecutor())
		transferSvc := transfer.NewService(queries, d.Pool, approvalSvc)
		approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeAssetTransfer, transferSvc.Executor())
		assignmentSvc := assignment.NewService(queries, d.Pool, approvalSvc)
		approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeAssignment, assignmentSvc.Executor())
		maintenanceSvc := maintenance.NewService(queries, d.Pool, approvalSvc, assetSvc)
		approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeMaintenance, maintenanceSvc.Executor())
		approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeAssetImport, assetSvc.ImportExecutor())
		approvalHandler := approval.NewHandler(approvalSvc, fieldSvc, common.ScopedDeps{Q: queries, Scope: scopeSvc}, auditSvc)
		approval.RegisterRoutes(api, approvalHandler, requireAuth, permSvc)

		transferHandler := transfer.NewHandler(transferSvc, assetSvc, scopeSvc, queries, auditSvc)
		transfer.RegisterRoutes(api, transferHandler,
			requireAuth,
			middleware.RequirePermission(permSvc, "transfer.manage"),
			middleware.RequirePermission(permSvc, "transfer.view"),
		)

		disposalHandler := disposal.NewHandler(disposalSvc, assetSvc, scopeSvc, queries, auditSvc)
		disposal.RegisterRoutes(api, disposalHandler,
			requireAuth,
			middleware.RequirePermission(permSvc, "disposal.manage"),
			middleware.RequirePermission(permSvc, "disposal.view"),
		)

		stockopnameSvc := stockopname.NewService(queries, d.Pool, disposalSvc, transferSvc, maintenanceSvc)
		stockopnameHandler := stockopname.NewHandler(stockopnameSvc, scopeSvc, queries, auditSvc)
		stockopname.RegisterRoutes(api, stockopnameHandler,
			requireAuth,
			middleware.RequirePermission(permSvc, "stockopname.manage"),
			middleware.RequirePermission(permSvc, "stockopname.view"),
		)

		assignmentHandler := assignment.NewHandler(assignmentSvc, scopeSvc, queries, auditSvc)
		assignment.RegisterRoutes(api, assignmentHandler,
			requireAuth,
			middleware.RequirePermission(permSvc, "assignment.manage"),
			middleware.RequirePermission(permSvc, "assignment.view"),
			middleware.RequirePermission(permSvc, "request.create"),
		)

		maintenanceHandler := maintenance.NewHandler(maintenanceSvc, scopeSvc, queries, auditSvc)
		maintenance.RegisterRoutes(api, maintenanceHandler,
			requireAuth,
			middleware.RequirePermission(permSvc, "maintenance.manage"),
			middleware.RequirePermission(permSvc, "maintenance.view"),
			middleware.RequirePermission(permSvc, "request.create"),
		)

		depreciationHandler := depreciation.NewHandler(depreciationSvc, fieldSvc, common.ScopedDeps{Q: queries, Scope: scopeSvc}, auditSvc)
		depreciation.RegisterRoutes(api, depreciationHandler,
			requireAuth,
			middleware.RequirePermission(permSvc, "depreciation.manage"),
			middleware.RequirePermission(permSvc, "depreciation.view"),
			middleware.RequirePermission(permSvc, "asset.view"),
		)

		reportSvc := report.NewService(queries, d.Redis)
		reportHandler := report.NewHandler(reportSvc, common.ScopedDeps{Q: queries, Scope: scopeSvc})
		report.RegisterRoutes(api, reportHandler,
			requireAuth,
			middleware.RequirePermission(permSvc, "report.view"),
			middleware.RequirePermission(permSvc, "report.export"),
		)

		notificationSvc := notification.NewService(queries)
		notificationHandler := notification.NewHandler(notificationSvc)
		notification.RegisterRoutes(api, notificationHandler, requireAuth)

		// The notification pipeline: outbox -> relay -> Redis Stream -> consumer
		// -> per-user rows, with the sweeper feeding maintenance reminders in at
		// the outbox end and purging the far end. The consumer reuses the very
		// approvalSvc and scopeSvc built above rather than second copies: those
		// carry the Redis-backed caches, and a duplicate would resolve the same
		// recipients off a cold cache. Both are mandatory here -- a nil resolver
		// or scope would make approval_pending / maintenance_due events fail and
		// pile up in the stream's pending list. The empty consumer name selects
		// the host-pid default, which is unique per process.
		workers.Relay = notification.NewRelay(queries, d.Pool, d.Redis, d.Cfg.NotificationStreamMaxLen, d.Cfg.NotificationRelayPoll)
		workers.Consumer = notification.NewConsumer(queries, d.Redis, approvalSvc, scopeSvc, "", d.Cfg.NotificationRelayPoll, d.Cfg.NotificationClaimMinIdle)
		workers.Sweeper = notification.NewSweeper(queries, d.Pool, d.Cfg.NotificationRetentionDays, d.Cfg.NotificationSweepPoll)

		authzAdminSvc := authzadmin.NewService(queries, d.Pool, permSvc, scopeSvc, fieldSvc)
		authzAdminHandler := authzadmin.NewHandler(authzAdminSvc, auditSvc)
		authzadmin.RegisterRoutes(api, authzAdminHandler, requireAuth,
			middleware.RequirePermission(permSvc, "role.manage"),
			middleware.RequirePermission(permSvc, "scope.manage"),
			middleware.RequirePermission(permSvc, "fieldperm.manage"),
			middleware.RequireAnyPermission(permSvc, "role.manage", "scope.manage", "fieldperm.manage"),
			middleware.RequireAnyPermission(permSvc, "role.manage", "scope.manage", "fieldperm.manage", "user.manage"),
		)

		importerSvc := importer.NewService(queries, d.Pool, d.Storage, d.Redis, d.Cfg.ImportMaxRows, d.Cfg.ImportMaxBytes)
		importerSvc.RegisterTarget(assetSvc.Importer())
		importerSvc.RegisterTarget(employee.NewService(queries).Importer())
		importerSvc.RegisterTarget(office.NewService(queries).Importer())
		refSvc := reference.NewService(queries)
		importerSvc.RegisterTarget(reference.NewImporter(refSvc, "provinces"))
		importerSvc.RegisterTarget(reference.NewImporter(refSvc, "cities"))
		importerSvc.RegisterTarget(reference.NewImporter(refSvc, "brands"))
		importerSvc.RegisterTarget(reference.NewImporter(refSvc, "models"))
		importerSvc.RegisterTarget(reference.NewImporter(refSvc, "units"))
		importerHandler := importer.NewHandler(importerSvc, permSvc, common.ScopedDeps{Q: queries, Scope: scopeSvc}, auditSvc)
		importer.RegisterRoutes(api, importerHandler, requireAuth)
		workers.Import = importer.NewWorker(importerSvc, d.Pool, d.Redis, approvalSvc, scopeSvc, d.Cfg.ImportWorkerPoll)
	}

	return r, workers
}
