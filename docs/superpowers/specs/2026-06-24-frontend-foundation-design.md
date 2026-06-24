# Inventra Frontend — Foundation & Implementation Design

**Status:** Approved (2026-06-24)
**Scope of this doc:** Overall frontend architecture + reusable component inventory + phasing.
Phase 1 (Foundation) is detailed here and becomes the first implementation plan.

Reference designs: 22 high-fidelity mockups in [`docs/design/`](../../design/) (custom `.dc.html`
export format — used as **visual reference only**; UI is rebuilt with Nuxt UI `U*` components per
[CLAUDE.md](../../../CLAUDE.md)). Screen list & per-screen briefs: [`docs/DESIGN_BRIEF.md`](../../DESIGN_BRIEF.md).

## Decisions (locked)

1. **Mock data via a service layer** — every module exposes a service whose interface equals the
   backend contract (`{ data, total, limit, offset }`). Today it reads fixtures; later only the
   service implementation swaps to real `$fetch`. UI and pages never change.
2. **Authentication is real** — wired to the live backend `/auth/*`. Login (email/password),
   token storage, auto-refresh on 401, route guard, and `/auth/me` + `/auth/permissions` for
   role-based menu/buttons. Google OAuth & password reset are deferred (not yet in backend).
3. **Phase 1 first** — foundation (app shell, design tokens, global reusable components, real login)
   before any feature screens.
4. **Charts:** `@unovis/vue` (Nuxt UI-aligned, lightweight, dark-mode aware).

## Design system (from mockups, must match)

- Primary green (`#16a34a` light / `#22c55e` dark), neutral slate. Font Inter; mono JetBrains Mono
  for code/labels. Rounded-lg, generous spacing, light + dark mode via CSS tokens.
- Semantic tokens only (`--primary`, `--text-muted`, success/warning/error) — set in
  `app/assets/css/main.css` and mapped to Nuxt UI color props. No hardcoded Tailwind colors.
- All user-facing strings via i18n (`id` default, `en`), `$t('key')`.

## Architecture & folder structure

**Principle: the UI never knows whether data is real or mocked.** A module's page calls its service;
the service returns the backend-shaped payload. Mock implementation lives behind the same signature.

```
frontend/app/
├── layouts/
│   ├── auth.vue            # login layout (brand panel + form)
│   └── default.vue         # app shell (AppSidebar + AppTopbar + UMain)
├── middleware/
│   ├── auth.global.ts      # redirect to /login when unauthenticated
│   └── can.ts              # per-route permission guard (route meta.permission)
├── components/             # GLOBAL reusable (auto-import) — see inventory below
│   └── <module>/           # MODULE-scoped components (asset/, approval/, ...)
├── composables/
│   ├── useApiClient.ts     # $fetch base: inject Bearer, refresh-on-401, toast on error
│   ├── useAuthApi.ts       # REAL → /auth/login|refresh|logout|me|permissions
│   ├── useCan.ts           # permission check for menu items & action buttons
│   └── api/                # MOCK services per module (contract === backend)
│       ├── useAssetApi.ts
│       └── ...
├── mock/                   # fixtures (JSON/TS) + helpers: paginate(), filterBy(), fakeLatency()
├── stores/                 # Pinia: auth, ui (sidebar/theme), notifications
├── utils/                  # formatRupiah, formatDate, statusMeta maps
└── pages/                  # thin — component composition only
```

### Auth flow (real)

- `authStore` (Pinia) holds access token + user + permissions. Refresh token in a cookie.
- `useAuthApi.login()` → POST `/auth/login`; stores tokens, then fetches `/auth/me` +
  `/auth/permissions`.
- `useApiClient` attaches `Authorization: Bearer`; on 401 it calls `/auth/refresh` once and retries,
  else logs out → `/login`.
- `auth.global.ts` redirects unauthenticated users to `/login` (allow-list: `/login`).
- `can.ts` reads `route.meta.permission` and 403s/redirects if `useCan` fails.
- Menu items and action buttons render conditionally via `useCan(key)`.

### Mock service contract (example)

```ts
// composables/api/useAssetApi.ts  (mock today, real later — signature is the seam)
export function useAssetApi() {
  return {
    list: (q: AssetQuery): Promise<Paginated<Asset>> => /* mock: paginate(filterBy(fixtures, q)) */,
    get:  (id: string): Promise<Asset> => ...,
    create: (input: AssetInput): Promise<Asset> => ...,
    update: (id, input): Promise<Asset> => ...,
    remove: (id): Promise<void> => ...
  }
}
// Swap to real later = replace bodies with useApiClient().$fetch('/assets', ...). UI unchanged.
```

## Reusable component inventory

### Global — `app/components/` (cross-module, auto-imported)

| Component | Wraps / contains | Used by |
|---|---|---|
| **AppSidebar** | nav groups + icons, collapse, active accent, pending badge; items filtered by `useCan` | all pages |
| **AppTopbar** | breadcrumb, GlobalSearch, NotificationBell, LangSwitcher, ThemeToggle, UserMenu | all |
| **UserMenu / NotificationBell / LangSwitcher / ThemeToggle / GlobalSearch** | `UDropdownMenu`, `UButton`, `UBadge` | topbar |
| **PageHeader** | title + breadcrumb + primary-action slot | all index/detail |
| **ResourceTable** ⭐ | `UTable`: sortable headers, bulk-select checkbox, per-row action column, loading→skeleton slot, empty state, embedded pagination | asset, employee, user, audit, maintenance, master data lists |
| **DataToolbar (FilterBar)** | search `UInput` + filter `USelect` slots + reset + table/grid toggle | all lists |
| **TablePagination** | `UPagination` + "menampilkan 1–20 dari N" | all lists |
| **StatCard (KpiCard)** | `UCard`: big number, label, trend/icon | dashboard, reports |
| **StatusBadge** | `UBadge` + `status → {color,label}` map for asset status & approval status | asset, approval, assignment, maintenance |
| **FormSlideover** | `USlideover`: title, body slot, sticky "Batal/Simpan" footer, loading | asset/employee/user/office/maintenance forms |
| **FormModal** | `UModal` variant for simple entities | master data reference |
| **ConfirmDialog** + `useConfirm()` | `UModal` destructive confirm | all destructive mutations |
| **EmptyState** | icon + title + description + action slot | all lists/panels |
| **TableSkeleton / CardSkeleton** | `USkeleton` | loading states |
| **TreeView** ⭐ | recursive expand/collapse, per-level icon, child-count badge, selected node | office hierarchy (reusable) |
| **EntityAvatar** | `UAvatar` + name + sub-text | user, audit, approval |
| **`<Can>` / FieldGuard** | show/hide by permission or field-permission | sensitive buttons & columns (harga/nilai buku) |
| **AppBreadcrumb** | `UBreadcrumb` from route meta | topbar |

Global utilities (not components): `formatRupiah`, `formatDate`, `statusMeta` (color/label maps),
`useConfirm`, `useToast` (Nuxt UI built-in). Form labels use the built-in **`UFormField`** — no custom
wrapper unless a repeating pattern emerges.

### Module-scoped — `app/components/<module>/` (single-module)

| Module | Components |
|---|---|
| **asset/** | `AssetForm`, `AssetFilterBar`, `AssetCard`, `AssetGallery`, `BarcodeLabel`, `DepreciationScheduleTable`, `AssignmentHistoryTable`, `MaintenanceHistoryTable` |
| **dashboard/** | `KpiRow`, `DonutChart`, `BarChart`, `MaintenanceDuePanel`, `PendingApprovalPanel` |
| **approval/** | `ApprovalInboxItem`, `ApprovalDetailPanel`, `ApprovalTimeline`, `BeforeAfterDiff` |
| **assignment/** | `CheckoutForm`, `CheckinForm` |
| **maintenance/** | `MaintenanceForm`, `DamageReportForm`, `MaintenanceReminderBanner` |
| **masterdata/** | `ReferenceCrud` (one generic list+form for the 11 reference entities, mirroring the backend generic engine), `OfficeDetailPanel`, `FloorRoomAccordion` |
| **settings/** | `PermissionMatrix`, `ScopeMatrix`, `FieldPermissionMatrix` |
| **user/** | `UserForm` |
| **audit/** | `AuditDiffRow` |
| **import/** | `ImportStepper`, `RowValidationTable` |

## Phasing

1. **Phase 1 — Foundation (this spec):** design tokens, `useApiClient` + `useAuthApi` + `authStore`,
   auth middleware, `auth` + `default` layouts, **real Login**, AppSidebar/AppTopbar + topbar widgets,
   the **global component library (B1)**, a `/dev/components` style-guide page for verification, and
   mock-service skeleton + fixtures helpers. i18n keys for all of the above.
2. **Phase 2 — Showcase:** Dashboard + Katalog Aset + Detail Aset on the mock API.
3. **Phase 3+:** remaining modules one at a time (master data → operational → approval → settings →
   reports/import). Each gets its own spec → plan → implementation.

## Phase 1 — detailed deliverables

- **Tokens:** port the mockup CSS variables into `app/assets/css/main.css` and bind Nuxt UI colors.
- **State:** `stores/auth.ts` (token/user/permissions), `stores/ui.ts` (sidebar collapsed, theme).
- **Composables:** `useApiClient`, `useAuthApi`, `useCan`. Mock-helper module in `mock/` (`paginate`,
  `filterBy`, `fakeLatency`) — used by future module services.
- **Middleware:** `auth.global.ts`, `can.ts`.
- **Layouts:** `auth.vue` (login), `default.vue` (shell).
- **Pages:** `/login` (real), `/` dashboard placeholder (so the shell is navigable), `/dev/components`.
- **Global components (B1):** all of the inventory above, each demoed in `/dev/components`.
- **i18n:** every string in `i18n/locales/{id,en}.json`.
- **Out of scope for Phase 1:** Google OAuth, password reset, feature screens, charts (Phase 2).

## Testing & verification

- `pnpm lint` (no trailing commas, 1tbs), `pnpm typecheck`, `pnpm build` must pass (CI gates).
- Manual: login against the running backend; verify menu visibility changes by role/permission;
  toggle light/dark and id/en; `/dev/components` renders every global component in both themes.

## Open items

- None blocking. Chart library (`@unovis/vue`) is settled and only relevant from Phase 2.
