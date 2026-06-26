# Design — Global Search, Office Location Map, Account Profile

**Date:** 2026-06-26
**Status:** Approved (design) · pending implementation plan
**Mockups:** `docs/design/Global Search.dc.html`, `docs/design/Peta Lokasi.dc.html`,
`docs/design/Profil Akun.dc.html`

Three new frontend screens are added on top of the built foundation. All three are **mock-first**
(behind composables, ready to swap to real `$fetch`), reproduce their mockup **1:1**, and ship with
unit + component + e2e tests.

## Resolved decisions

1. **Map technology — real Leaflet** (user-requested deviation from the illustrative SVG in the
   mockup). The mockup **layout** is still followed 1:1; only the map panel itself is a real
   Leaflet + OpenStreetMap map instead of the hand-drawn SVG.
2. **Data wiring — mock-first, consistent** with the rest of the frontend. User identity (name /
   email / role) comes from the real `useAuthStore()`; theme from `useColorMode()`; language from
   `useI18n()`. Everything else (search index, password change, sessions, notification prefs, map
   offices) is mock behind a composable.
3. **Office categories on the map — follow the mockup exactly:** `Pusat / Wilayah / Cabang / Outlet`
   with the mockup's four colors (mapped to semantic tokens). The map uses a **self-contained mock
   office dataset** (the 9 offices from the mockup) so it is fully faithful to the mockup. It does
   **not** reuse the existing `officeStore` (whose `tipe` is `pusat/kanwil/cabang/unit`).

### Fidelity rule (per CLAUDE.md)

Build exactly what each mockup shows — no self-initiated changes, simplifications, drops, or
substitutions. The only deviation is the Leaflet map (decision 1), explicitly requested. Where a
mockup uses a literal hex color, substitute the equivalent **semantic token** but keep structure and
intent. Every screen ends with a side-by-side comparison against its `.dc.html` in light **and** dark
mode before being called done.

## Shared conventions

- `U*` (Nuxt UI) primitives + semantic tokens (`text-muted`, `bg-default`, `--ui-primary`, …); no
  hardcoded Tailwind colors. Reproduce mockup markup faithfully where a raw composition is clearer,
  matching how existing mockup-faithful screens are built.
- All user-facing strings in `i18n/locales/{id,en}.json` via `$t` / `useI18n()`. Default locale `id`.
- Mock data seam: `app/mock/*` fixtures + `app/composables/api/use*` services using `fakeLatency()`.
- Permission-gated UI via `useCan` / `<Can>`; route gating via `definePageMeta({ middleware: 'can',
  permission })`.
- Tests: pure logic → Vitest unit (node); components → `mountSuspended` runtime (`// @vitest-environment
  nuxt`); flows → Playwright e2e using the `login()` helper. Assert real behavior (rendered text,
  resolved i18n, emitted events, navigation), broad coverage of every state/branch/edge case.

---

## 1. Global Search — Command Palette (⌘K)

### Behavior (from `Global Search.dc.html`)

A ⌘K command palette overlay. States, exactly as the mockup: **initial** (Recent Searches + Quick
Actions), **loading** (shimmer skeleton rows), **results** (grouped by entity type, each group with a
"See all (n)" action and per-row icon/title/sub/optional status badge), **no-results** (empty state).
Footer shows ↑↓ navigate · ↵ open · Esc close, plus the right-aligned note "Results limited to your
scope & permissions". Match highlighting (`<mark>`) on the typed query within result titles.

### Components & wiring

- **`app/components/GlobalSearch.vue`** (exists as a dead input) → becomes a **button** that opens the
  palette, keeping the search icon, placeholder, and ⌘K badge (the mockup's topbar uses an
  `openPalette` button, not an input).
- **`app/components/CommandPalette.vue`** (new) — the overlay + input row + body states + footer.
  Rendered globally from `app/layouts/default.vue` next to `<ConfirmDialog />` so it floats above all
  pages. Built as a teleported overlay positioned per mockup (top-aligned, `max-width:640px`).
- **`app/composables/useCommandPalette.ts`** (new) — shared open/close state (`isOpen`, `open()`,
  `close()`, `toggle()`), so the topbar button, the global key handler, and the palette all share one
  source of truth (`useState`-backed).
- **Global keyboard handling** inside `CommandPalette.vue` (`window` keydown, added on mount, removed
  on unmount): **⌘/Ctrl+K** toggles; when open, **Esc** closes, **↑/↓** move selection across the flat
  ordered result list, **Enter** navigates to the selected item and closes. Mouse hover sets
  selection; click navigates.

### Data — `app/composables/api/useGlobalSearch.ts` (new, mock)

- `search(query: string): Promise<SearchGroup[]>` — debounced via `fakeLatency`. Aggregates across the
  existing mock stores: **assets, employees, offices, users, requests (approval)**. Returns groups in a
  fixed order, each `{ type, label, total, items: SearchItem[] }`.
- `SearchItem` = `{ type, title, sub, status?, icon, to }`. `to` is the destination route used by Enter
  / click (e.g. asset → `/assets/<tag>`, office → `/master/offices`, request → `/approval`). Where a
  detail route does not exist yet, link to the nearest existing list screen.
- **Recent searches** persisted client-side (`localStorage`, capped list); seeded empty. Selecting a
  recent entry re-runs the search.
- **Quick actions** (exactly the mockup's three, with kbd hints shown for visual fidelity): Add Asset →
  `/assets/new`, Open Reports → `/reports`, Create Request → `/approval`. Each gated with `useCan`
  (hidden if the user lacks the permission); the kbd letters (A/L/P) are display-only.

### i18n

New `search.*` namespace: `placeholder`, `topbarPlaceholder`, `hint`, `openNow`, `recentTitle`,
`quickTitle`, quick-action labels, `seeAll` (param `n`), `emptyTitle` (param `q`), `emptySub`, footer
hints, `scopeNote`, and group labels (`assets`/`employees`/`offices`/`users`/`requests`). Status
labels reuse the existing `status.*` namespace.

### Tests

- Unit (`useGlobalSearch`): filtering matches title/sub case-insensitively; grouping + group order;
  empty query → no groups; no-match → empty; `total` per group; recent-search persistence.
- Component (`CommandPalette` via `mountSuspended`): closed → hidden; open shows initial state;
  typing → loading then results; arrow keys move selection; Enter triggers navigation; Esc closes;
  no-results renders empty state; quick actions hidden without permission.
- E2E: press ⌘K (and click the topbar trigger) → palette opens → type a known asset → result group
  appears → Enter navigates and palette closes.

---

## 2. Office Location Map — Peta Lokasi (Leaflet)

### Route & navigation

- **`app/pages/master/map.vue`** → route `/master/map`. Gate:
  `definePageMeta({ middleware: 'can', permission: 'masterdata.office.manage' })`.
- **Nav:** in `app/utils/nav.ts` the Master Data group currently has a disabled `nav.geography` child.
  Replace it with an enabled **"Peta Lokasi"** child (`labelKey: 'nav.officeMap'`, `to: '/master/map'`).
  Breadcrumb resolves to Master Data › Peta Lokasi automatically.

### Layout (1:1 with `Peta Lokasi.dc.html`)

Two-column flex inside the standard app shell:

- **Left — office list panel (`width: 312px`):** search input (by name/code), two filter selects
  **Jenis** (All + Pusat/Wilayah/Cabang/Outlet) and **Provinsi** (All + distinct provinces), a
  scrollable list of office rows (colored pin chip, name, jenis badge, code, city·province),
  loading skeleton, and an empty state when filters match nothing.
- **Right — map panel:** header with a summary strip ("n offices · n cities · n provinces") + a legend
  for the four jenis; the map area; zoom-in/out controls (top-right), a "Reset View" button
  (bottom-right) that fits all visible pins; and a **detail card** (bottom-left) shown when an office
  is selected.

### Map — `app/components/OfficeMap.client.vue` (new, client-only)

- Real **Leaflet** map with **OpenStreetMap** tiles, wrapped so it only runs client-side (SPA mode,
  `.client.vue` / `<ClientOnly>`).
- **Markers** are custom `divIcon` teardrop pins colored by jenis (reproducing the mockup pin shape:
  rounded `50% 50% 50% 0`, rotated, white border, small white glyph), with the selected pin enlarged +
  a pulse ring and an always-visible label (matching the mockup's selected/zoomed label behavior).
- Selecting a **list row** → `flyTo` that marker + opens the detail card; clicking a **marker** → same
  selection. **Reset View** → `fitBounds` over visible pins, clears selection. Zoom buttons map to
  Leaflet `zoomIn/zoomOut`; the default Leaflet zoom control is hidden in favor of the mockup-styled
  controls.
- Emits `select(id)` / `clear()` and accepts the filtered office list + selected id as props, so all
  filter/selection state lives in the page and the map stays a presentational client component.

### Data — `app/mock/officeMap.ts` + `app/composables/api/useOfficeMap.ts` (new)

- Self-contained dataset of the **9 mockup offices** (`Kantor Pusat`, `Kanwil DKI Jakarta`, `Cabang
  Jakarta Selatan`, `Cabang Jakarta Pusat`, `Outlet Blok M`, `Outlet Kemang`, `Cabang Bekasi`, `Cabang
  Tangerang`, `Outlet Depok`), each with `{ id, nama, kode, jenis, kota, prov, alamat, aset, lat, lng }`.
  The mockup's illustrative `x/y` coordinates are replaced with **real Jabodetabek lat/lng** for the
  matching locations.
- `jenis: 'Pusat' | 'Wilayah' | 'Cabang' | 'Outlet'`. A `jenisMeta` map provides label (i18n),
  semantic color token, and pin/legend styling — the mockup's four colors via tokens
  (`--pin-pusat`→primary/green, `--pin-wilayah`→info/blue, `--pin-cabang`→warning/amber,
  `--pin-outlet`→neutral/slate).
- `useOfficeMap().list()` returns the dataset (mock latency). Filtering (search + jenis + provinsi) and
  the summary counts are derived in the page via `computed`.

### Detail card actions

- **"Lihat Kantor"** → `navigateTo('/master/offices')` (office detail editor). **"Buka di Maps"** →
  opens an external Google Maps URL built from the office `lat/lng` in a new tab.

### i18n

New `map.*` namespace: page title/breadcrumb, usage note, search placeholder, filter "all" labels,
empty-list and empty-map titles/subtitles, summary strip (params), legend labels, reset/zoom tooltips,
detail-card action labels, and "registered assets".

### Tests

- Unit: filter logic (search by name/code, jenis filter, provinsi filter, combined); summary counts
  (offices/cities/provinces) over the filtered set; Google Maps URL builder.
- Component (`mountSuspended`): list renders rows; selecting a row sets selection + opens detail card;
  empty filter → empty states (list + map overlay); legend shows four jenis. The Leaflet component is
  client-only, so the page test asserts list/filter/detail behavior with the map stubbed.
- E2E: open `/master/map` → filter by jenis → list narrows → select an office → detail card shows its
  data; Reset View clears selection.

---

## 3. Account Profile — Profil Akun

### Route & entry points

- **`app/pages/akun.vue`** → route `/akun`. Available to any authenticated user (default route
  middleware; no extra permission). Tabs reflected in the URL via `?tab=profil|keamanan|pref` so the
  topbar menu can deep-link.
- **`app/components/UserMenu.vue`:** wire the existing "Profil Saya" → `navigateTo('/akun')` and
  "Pengaturan Akun" → `navigateTo('/akun?tab=pref')` (both currently just close the popover).

### Tabs (1:1 with `Profil Akun.dc.html`)

Header: avatar (initials from `auth.user`), name, role badge, email + assigned office. Three tabs:

- **Profil:** Profile-photo block (Upload / Remove buttons — mock, with hint "JPG/PNG, max 2 MB"); a
  Personal Data form (Full Name [required], Phone, Email — **email locked & greyed when login method is
  Google**, with the lock note); a read-only Account Information card (Role, Assigned Office, Linked
  Employee, Login Method [with email/Google icon], Joined Date). **Save Changes** → success toast.
- **Keamanan:** if login method is **email** → Change Password form (Current / New / Confirm) with a
  **strength meter** (4 bars + label) and validation (required fields, confirm-must-match); on success
  → toast + clear. If **Google** → an info card ("signs in via Google; password managed in Google").
  Below: **Sessions & Devices** list (mock: current + other devices, each with device, location·time,
  icon, and a Revoke action on non-current sessions) + a **"Log out of all devices"** action → toast.
- **Preferensi:** Language toggle (Indonesia / English → `useI18n().setLocale`); Theme selector
  (Light / Dark / **System** → `useColorMode().preference`, with the active card highlighted);
  Notifications toggles (Approval decisions, Maintenance reminders, Asset assignments — mock prefs,
  persisted client-side).

### Data — `app/composables/api/useAccount.ts` (new, mock) + helpers

- `getProfile()` — merges real `auth.user` (name/email/role/office) with mock-only fields (phone,
  linked employee, joined date, `loginMethod`). `loginMethod` defaults to `email` (derived; no backend
  field yet).
- `updateProfile(input)`, `changePassword(input)`, `listSessions()`, `revokeSession(id)`,
  `logoutAllOthers()`, `getNotificationPrefs()` / `setNotificationPrefs(prefs)` — all mock with
  `fakeLatency`, notification prefs persisted in `localStorage`.
- **`app/utils/passwordStrength.ts`** (new, pure) — score 0–4 from length / case mix / digit / symbol,
  with a localized label; unit-tested independently and reused by the meter.
- Toasts via the existing `useToast()`; theme via `useColorMode()`; language via `useI18n()`.

### i18n

New `account.*` namespace covering: page title, change-photo, the three tab labels, all section
headings/labels/hints (photo, personal data, account info, password, sessions, appearance, language,
theme, notifications), strength labels, Google notice, validation messages, and toast titles/messages.
Reuse `common.*` (save/cancel) and `theme.*` where they already exist.

### Tests

- Unit: `passwordStrength` scoring across weak→very-strong inputs and boundaries; `useAccount`
  validation (empty name rejected; confirm-mismatch rejected; notification-pref persistence).
- Component (`mountSuspended`): tab switching shows the right panel; Profil save with empty name shows
  the required error; Keamanan shows password form for email login and the Google card for Google
  login; password strength bars/label update with input; confirm-mismatch error renders; Preferensi
  theme select updates color mode and language toggle switches locale (assert resolved strings).
- E2E: from the user menu open `/akun`; switch tabs; submit Change Password with mismatch → inline
  error, then a valid change → success toast; toggle theme/language and assert the UI reflects it.

---

## New files / touch list

**New components:** `CommandPalette.vue`, `OfficeMap.client.vue`.
**New composables:** `useCommandPalette.ts`, `api/useGlobalSearch.ts`, `api/useOfficeMap.ts`,
`api/useAccount.ts`.
**New pages:** `pages/master/map.vue`, `pages/akun.vue`.
**New mock/util:** `mock/officeMap.ts`, `utils/passwordStrength.ts`.
**Edited:** `components/GlobalSearch.vue` (input→button trigger), `layouts/default.vue` (mount
palette), `components/UserMenu.vue` (wire profile links), `utils/nav.ts` (geography→office map),
`i18n/locales/{id,en}.json` (new namespaces + nav key), `app/types.ts` (search/map/account types).
**Dependencies:** add `leaflet` + `@types/leaflet`.

## Out of scope

- No backend modules (search index, profile/password/session endpoints, office coordinates). Those land
  when the corresponding backend phases are built; the composable interfaces are shaped to swap to
  `$fetch` without changing pages.
- Real photo upload/storage (MinIO), real device/session tracking, and real notification delivery are
  mock-only here.
