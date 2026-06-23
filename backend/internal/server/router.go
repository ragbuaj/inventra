// Package server wires the HTTP router and shared middleware.
//
// Feature modules (identity, masterdata, asset, ...) register their routes here
// as they are implemented; see docs/PRD.md §7 for the module layout.
package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/ragbuaj/inventra/internal/cache"
	"github.com/ragbuaj/inventra/internal/config"
	"github.com/ragbuaj/inventra/internal/db"
)

// Deps holds the shared infrastructure passed to feature modules.
type Deps struct {
	Cfg   *config.Config
	Pool  *pgxpool.Pool
	Redis *redis.Client
}

// NewRouter builds the Gin engine with base middleware, health, and readiness probes.
func NewRouter(d Deps) *gin.Engine {
	if d.Cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

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

	api := r.Group("/api/v1")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})
		// Feature module routes are registered here, e.g.:
		//   identity.RegisterRoutes(api, d)
		//   asset.RegisterRoutes(api, d)
	}

	return r
}
