# ADR-0008 — Backend masterdata: 4-file split (dto/service/handler/routes) per resource

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-06-26 |
| **Deciders** | Ragil (owner) |
| **Maps to** | Backlog #3 |

## Context and problem statement

`internal/masterdata/` was the **deliberate exception** to the codebase's four-file module split
(`dto.go` / `service.go` / `handler.go` / `routes.go`, as in `identity` and `user`). Each sqlc-backed
resource (offices, categories, employees, floors, rooms) lived in **one file** mixing DTO + mapper +
handler + `registerXxx` routes, with **business/scope logic inline in handlers** and **no service
layer**. As the package grows (bank-FAM enriches categories and adds modules), the mixed-responsibility,
single-package layout was inconsistent and hard to navigate.

## Decision drivers

- Consistency with the rest of the backend (the four-file split is the documented convention).
- Thin handlers; business logic + data-scope enforcement in a Gin-free service layer (testable).
- A real **folder** convention that scales as resources/modules grow.

## Considered options

1. **One masterdata package, 4 files per resource** (`offices_dto.go`, `offices_service.go`, …) —
   less churn, but ~20+ files in one package and no folders.
2. **Sub-package per resource (folder)** — `masterdata/<resource>/{dto,service,handler,routes}.go`,
   shared plumbing in `masterdata/common`, generic reference engine in `masterdata/reference`, wired by
   a thin `masterdata` aggregator. True folder convention, strong isolation.

## Decision outcome

**Chosen: Option 2 — sub-package per resource.** Final structure:

```
internal/masterdata/
├── masterdata.go              # aggregator: RegisterRoutes → delegates to each sub-package
├── common/                    # shared plumbing (leaf pkg): errors, MapDBError, WriteError,
│   ├── common.go              #   ParseUUIDPtr/UUIDPtrStr/TsStr/BoolOr/ClampInt
│   └── scope.go               #   ScopedDeps + CallerOfficeScope, InScope, SamePtr
├── office/   {dto,service,handler,routes}.go     # sqlc-backed, office-scoped
├── category/ {dto,service,handler,routes}.go     # sqlc-backed, global
├── employee/ {dto,service,handler,routes}.go     # sqlc-backed, office-scoped
├── floor/    {dto,service,handler,routes}.go     # sqlc-backed, office-scoped
├── room/     {dto,service,handler,routes}.go     # sqlc-backed, scoped via floor
└── reference/ engine.go · resources.go · routes.go   # generic CRUD engine for flat tables
```

Conventions applied per resource:
- **service.go** — `Service` + `CreateInput`/`UpdateInput` + business rules & **data-scope enforcement**
  as sentinel errors (e.g. `office.ErrParentOutOfScope`); takes resolved `(allScope, officeIDs)` params;
  **no Gin**.
- **dto.go** — `Request` (binding tags) + `Response` + `toResponse` + `Request.toInput()` (UUID parsing).
- **handler.go** — `Handler` (service + `common.ScopedDeps` + audit); resolves scope, calls service,
  records audit, serializes; an `svcError` helper routes service sentinels to HTTP status.
- **routes.go** — `RegisterRoutes(rg, handler, authMW, requireManage)`.

The generic **reference** engine keeps serving flat reference tables declaratively (it is one engine for
many tables, not a per-resource split). `NewRouter` is unchanged — it still calls
`masterdata.RegisterRoutes`, which now delegates to the sub-packages.

## Consequences

- 👍 Consistent with `identity`/`user`; thin handlers; Gin-free, testable services; scope logic in one
  obvious place per resource; scales as bank-FAM resources land.
- 👍 Verified green: `go build`, `go vet`, `go test`, `gofmt` all clean after the refactor.
- 👎 More packages and some boilerplate per resource (constructors, input mappers).
- 👎 Larger diff (moved + split files). No behavior change — routes, payloads, and scope rules are
  preserved 1:1.

## Notes

- CLAUDE.md's "masterdata is the deliberate exception" guidance is updated to describe this convention.
- New bank-FAM master resources (e.g. enriched categories, future reference tables) follow this split.
