# ADR-0007 — Frontend API composable convention (folders + DTO naming)

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-06-26 |
| **Deciders** | Ragil (owner) |
| **Maps to** | Discovered during the #3 audit — **frontend** cleanup, separate from backend #3 (ADR-0008) |

## Context and problem statement

Feature screens are **mock-first**, backed by a typed service layer in `frontend/app/composables/api/`
that will swap to real `$fetch` behind the same interface. Two convention problems surfaced:

1. **Flat folder.** ~20 `useX.ts` composables (masterdata, asset, settings, operational, reporting,
   account, search) sit in one directory with no grouping — it doesn't mirror the backend's modular
   structure and won't scale as bank-FAM modules (transfer, stock opname, disposal) land.
2. **Inconsistent DTO field naming.** Some master-data composables use **Indonesian** field keys
   (`useOffices`: `nama`, `kode`, `tipe`, `provinsi`, `kota`, `alamat`) while others
   (`referenceResources`) and the **backend API** use **English snake_case** (`name`, `code`,
   `office_type_id`, `province_id`, `city_id`, `address`). The Indonesian keys diverge from the API
   contract — wiring to the real backend would need a mapping shim, a latent bug source. (An
   English-standardization effort is already underway, e.g. the akun→account rename.)

## Decision drivers

- The mock service interface must be the **same shape as the real API** so the swap is transparent.
- Mirror backend modularity for cross-stack consistency and scale.
- i18n already handles user-facing language; **data field keys are a contract, not UI copy**.

## Decision outcome

1. **Group `composables/api/` into module subfolders** mirroring the backend modules (Nuxt
   auto-import scans recursively, so no manual-import churn):
   - `api/masterdata/` — offices, floors, employees, reference, `referenceResources`
   - `api/asset/` — assets (+ future: transfers, stockopname, disposals, documents)
   - `api/identity/` — users, rbac, dataScope, fieldPermission, audit, account
   - `api/operational/` — assignment, maintenance, approval
   - `api/reporting/` — dashboard, reports
   - `api/` (top-level) — globalSearch, notifications, officeMap (cross-cutting)
   Mirror the same grouping under `mock/` for parity.
2. **DTO field keys = backend API contract: English `snake_case`.** Rename Indonesian-keyed DTOs/types
   (starting with `useOffices` + its `Office` type and mock store) to `name`/`code`/`office_type_id`/
   `province_id`/`city_id`/`address`, etc. **i18n labels stay Indonesian** — only the data keys change.
3. **Consistent CRUD surface** per resource composable (`list`/`get`/`create`/`update`/`remove`); keep
   the generic reference engine (`useReference` + `referenceResources`) for flat reference tables.

## Consequences

- 👍 Frontend structure mirrors backend modules; the mock→real swap needs **no field mapping**; scales
  cleanly as new modules land.
- 👍 One obvious place for each resource's service + matching mock.
- 👎 A one-time refactor touching ~20 composables, their mocks, shared types, and tests → do it as a
  focused change with `pnpm lint && pnpm typecheck && pnpm test` green before merge.
- 👎 Moving files changes relative imports (`./referenceResources`) → mechanical; aliases (`~/mock/...`)
  are unaffected.

## Implementation notes

- Execute as one focused refactor (separate from this ADR). Order: (a) rename Indonesian field keys to
  the API contract (highest value — removes the shim), (b) regroup folders.
- Verify CI gates locally: `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build`.
