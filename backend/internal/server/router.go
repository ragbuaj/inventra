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
	"github.com/redis/go-redis/v9"

	apidocs "github.com/ragbuaj/inventra/api"
	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/asset"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/auth"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/cache"
	"github.com/ragbuaj/inventra/internal/config"
	"github.com/ragbuaj/inventra/internal/db"
	"github.com/ragbuaj/inventra/internal/identity"
	"github.com/ragbuaj/inventra/internal/masterdata"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/oauth"
	"github.com/ragbuaj/inventra/internal/ratelimit"
	"github.com/ragbuaj/inventra/internal/storage"
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
}

// NewRouter builds the Gin engine with base middleware, health, and readiness probes.
func NewRouter(d Deps) *gin.Engine {
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
		middleware.RequestLogger(d.Log),
		middleware.Recovery(d.Log),
		middleware.CORS(d.Cfg.FrontendURL),
	)

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

		identitySvc := identity.NewService(queries, tokenManager, tokenStore)
		identityHandler := identity.NewHandler(identitySvc, permSvc, scopeSvc, d.Limiter, d.Cfg.RateLimitLoginPerMin, d.Cfg.Env == "production", d.Cfg.JWTRefreshTTL, googleOAuth, d.Cfg.FrontendURL)
		identity.RegisterRoutes(api, identityHandler, requireAuth, d.Limiter, d.Cfg.RateLimitLoginIPPerMin, d.Cfg.RateLimitRefreshPerMin, d.Cfg.RateLimitLoginIPPerMin)

		userHandler := user.NewHandler(user.NewService(queries), fieldSvc, auditSvc)
		user.RegisterRoutes(api, userHandler, requireAuth, middleware.RequirePermission(permSvc, "user.manage"))

		masterdata.RegisterRoutes(api, queries, d.Pool, permSvc, scopeSvc, auditSvc, requireAuth)

		assetSvc := asset.NewService(queries, d.Pool, d.Storage, d.Cfg.AttachmentMaxBytes, d.Cfg.LabelLogoPath)
		assetHandler := asset.NewHandler(assetSvc, fieldSvc, common.ScopedDeps{Q: queries, Scope: scopeSvc}, auditSvc)
		asset.RegisterRoutes(api, assetHandler,
			requireAuth,
			middleware.RequirePermission(permSvc, "asset.view"),
			middleware.RequirePermission(permSvc, "asset.manage"),
		)

		auditHandler := audit.NewHandler(auditSvc, scopeSvc, queries)
		audit.RegisterRoutes(api, auditHandler, requireAuth, middleware.RequirePermission(permSvc, "audit.view"))

		approvalSvc := approval.NewService(queries, d.Pool, scopeSvc, d.Redis)
		approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeAssetCreate, assetSvc.CreateExecutor())
		approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeAssetDisposal, assetSvc.DisposalExecutor())
		approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeValuationExclusion, assetSvc.ExclusionExecutor())
		approvalHandler := approval.NewHandler(approvalSvc, fieldSvc, common.ScopedDeps{Q: queries, Scope: scopeSvc}, auditSvc)
		approval.RegisterRoutes(api, approvalHandler, requireAuth, permSvc)
	}

	return r
}
