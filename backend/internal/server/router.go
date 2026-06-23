// Package server wires the HTTP router and shared middleware.
//
// Feature modules (identity, masterdata, asset, ...) register their routes here
// as they are implemented; see docs/PRD.md §7 for the module layout.
package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ragbuaj/inventra/internal/config"
)

// NewRouter builds the Gin engine with base middleware and health checks.
func NewRouter(cfg *config.Config) *gin.Engine {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// Liveness probe — does not depend on external services.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "inventra-api",
			"env":     cfg.Env,
		})
	})

	api := r.Group("/api/v1")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})
		// Feature module routes are registered here, e.g.:
		//   identity.RegisterRoutes(api, deps)
		//   asset.RegisterRoutes(api, deps)
	}

	return r
}
