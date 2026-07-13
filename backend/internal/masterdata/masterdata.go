// Package masterdata wires the master-data resource sub-packages — each a
// dto/service/handler/routes split (office, category, employee, floor, room) —
// plus the generic reference engine, under /api/v1. Shared plumbing lives in the
// common sub-package.
package masterdata

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/category"
	"github.com/ragbuaj/inventra/internal/masterdata/employee"
	"github.com/ragbuaj/inventra/internal/masterdata/floor"
	"github.com/ragbuaj/inventra/internal/masterdata/office"
	"github.com/ragbuaj/inventra/internal/masterdata/reference"
	"github.com/ragbuaj/inventra/internal/masterdata/room"
	"github.com/ragbuaj/inventra/internal/middleware"
)

// RegisterRoutes mounts all master-data endpoints. Read is open to any
// authenticated user; writes require the relevant manage permission.
func RegisterRoutes(rg *gin.RouterGroup, q *sqlc.Queries, pool *pgxpool.Pool, permSvc *authz.PermissionService, scopeSvc *authz.ScopeService, fieldSvc *authz.FieldService, aud *audit.Service, authMW gin.HandlerFunc) {
	globalManage := middleware.RequirePermission(permSvc, "masterdata.global.manage")
	officeManage := middleware.RequirePermission(permSvc, "masterdata.office.manage")

	category.RegisterRoutes(rg, category.NewHandler(q, aud), authMW, globalManage)
	reference.RegisterRoutes(rg, pool, aud, authMW, globalManage)
	office.RegisterRoutes(rg, office.NewHandler(q, scopeSvc, aud), authMW, officeManage)
	floor.RegisterRoutes(rg, floor.NewHandler(q, scopeSvc, aud), authMW, officeManage)
	room.RegisterRoutes(rg, room.NewHandler(q, scopeSvc, aud), authMW, officeManage)
	employee.RegisterRoutes(rg, employee.NewHandler(q, scopeSvc, aud, fieldSvc), authMW, officeManage)
}
