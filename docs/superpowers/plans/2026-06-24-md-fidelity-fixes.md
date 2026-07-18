# Master Data + App Shell — Fidelity Fixes Implementation Plan

> **For agentic workers:** executed via superpowers:subagent-driven-development — fresh implementer per
> task, each gated by a 1:1 mockup comparison + tests. Steps use checkbox (`- [ ]`) tracking.

**Goal:** Bring the App Shell and the Kantor/Pegawai/Referensi pages to **1:1** with their
`docs/design` mockups, reproducing every state with mock data.

**Spec:** `docs/superpowers/specs/2026-06-24-md-fidelity-audit.md` (read it — it carries the per-area
deviation lists and the three scope decisions). The `.dc.html` mockups are the visual source of truth.

## Global Constraints

- Build on `U*` components; theme via semantic tokens (`color="primary"`, `text-muted`, `--ui-*`) —
  map the mockup's literal hex to tokens; deviate only when a token would render visibly differently.
- i18n in BOTH `i18n/locales/{id,en}.json` (default `id`); merge new keys into the existing objects,
  never duplicate a top-level key. NO trailing commas / 1tbs. `pnpm lint` + `pnpm typecheck` +
  `pnpm test` must pass; mock-first behind `composables/api/`.
- **Every task ends with:** (a) a side-by-side comparison of the built UI against its mockup in light
  AND dark mode, reported in the task report; (b) updated/added tests so the new structure and every
  new state is covered (proactive coverage — empty/loading/error/populated, disabled nav, toggles).
- Do not redesign or drop any mockup element on your own initiative; reproduce what the mockup shows.
- If lint complains about a stale `.nuxt/eslint.config.mjs`, run `pnpm exec nuxt prepare` first.
- No Co-Authored-By trailer.

## Tasks (run in order — shared foundation first; sequential to avoid locale/config conflicts)

### Task 1 — Shared foundation: mock data, nav model, shared-component props

**Files (create/modify):**
- `app/types/index.ts` — add `active: boolean` to `Office`; add `Floor`/`Room` types; add `NavItem`/`NavGroup` types if useful.
- `app/mock/offices.ts` — add `active` to seed rows (mix active/inactive); colored type metadata helper for tree (`tipe` → icon + bg token).
- `app/mock/floors.ts` (new) + `app/composables/api/useFloors.ts` (new) — floors + rooms fixtures keyed by office, CRUD (`listByOffice`, `create`, `remove`; rooms nested), node-safe mock module.
- `app/mock/reference.ts` — add `active` to seeded + fallback rows; keep counts derivable via `referenceStores[key].all().length`.
- `app/mock/notifications.ts` (new) + `app/composables/api/useNotifications.ts` (new) — sample notifications (icon/text/time/read) + unread count; `list()`, `markAllRead()`.
- `app/utils/nav.ts` (new) — the full Superadmin nav model: groups `operasional`/`administrasi`, items with `{ labelKey, icon, to?, permission?, badgeCount?, disabled?, children? }`. Built routes get `to`; unbuilt ones get `disabled: true` (no `to`). Reproduce the mockup's exact items/grouping (see audit bagian App Shell + the mockup).
- `app/components/DataToolbar.vue` — add `showReset?: boolean` (default true); render Reset only when true.
- `app/components/FormModal.vue` + `FormSlideover.vue` — add optional `subtitle?: string` prop rendered under the title.
- `app/components/PageHeader.vue` — title `text-2xl font-bold tracking-tight`, `mb-[22px]`.
- `app/components/TreeView.vue` — extend `TreeNode` with optional `iconBg?`/`iconColor?` (type badge) and `inactive?` (dot); render the colored badge, the inactive dot, and a `inset 3px 0 0 var(--ui-primary)` left accent on the selected node.
- i18n: add keys the above need (notification panel, nav labels for all menu items incl. disabled ones, "coming soon", floors/rooms, Aktif/Nonaktif) in both locales.
- Tests: unit for `useFloors` (CRUD + listByOffice), `useNotifications` (unread count, markAllRead), the nav model (built items have `to`, unbuilt are `disabled`), and the `active`/count derivations. Update any existing test that asserts `TreeNode`/Office shape.

- [ ] Implement per the spec; TDD the new mock modules/composables; update affected tests.
- [ ] `pnpm test`, `pnpm typecheck`, `pnpm lint` green.
- [ ] Commit: `feat(frontend): add shell/master-data mock data, nav model, shared-component props`

### Task 2 — AppSidebar 1:1 rebuild

**Files:** `app/components/AppSidebar.vue`; `i18n/locales/{id,en}.json` (as needed). Uses the Task-1 nav model.
**Build to match `docs/design/App Shell.dc.html` sidebar (audit bagian App Shell, sidebar):** sections
*Operasional* / *Administrasi*; collapsible parent groups (Aset, Master Data, Pengaturan) with
indented children + rotating chevron; disabled "coming soon" items (no nav, muted, not clickable);
optional count badges; widths 264/76; active 3px left accent + `primary-soft` bg; hover `bg-muted`;
section-label `.14em` + `pt-[14px]`; logo mark archive/box icon; app name `tracking-tight`; bottom
user strip (avatar initials + name + scope, hidden when collapsed).
- [ ] Add a `mountSuspended` runtime test: groups render, a built item links, a disabled item is not a link, expand/collapse toggles children, badge shows. Update existing sidebar coverage.
- [ ] `pnpm test` + `typecheck` + `lint` green; mockup comparison (light+dark) in report.
- [ ] Commit: `feat(frontend): rebuild app sidebar to match App Shell mockup`

### Task 3 — AppTopbar 1:1 rebuild (+ layout padding)

**Files:** `app/components/AppTopbar.vue`, `app/components/AppBreadcrumb.vue` (mount it), `GlobalSearch.vue`, `LangSwitcher.vue`, `NotificationBell.vue`, `ThemeToggle.vue`, `UserMenu.vue`, `app/layouts/default.vue`; i18n as needed. Uses Task-1 `useNotifications`.
**Build to match the mockup topbar (audit bagian App Shell, topbar):** breadcrumb + page-title two-line
block (wire `AppBreadcrumb`); centered search `max-w-[420px]` with `⌘K` pill; inline ID/EN segmented
`LangSwitcher`; `NotificationBell` opens the ~330px popover (header + Mark read + rows + View all) with
unread badge; outlined 36px theme/bell/toggle buttons; `UserMenu` pill trigger + panel with role/scope
section + "Pengaturan Akun" + Profil + red Keluar; right cluster `gap-2`; topbar `z-30`. `default.vue`
main padding `px-8 py-7`, page canvas lighter (`--ui-bg`).
- [ ] Runtime tests: topbar renders page title + breadcrumb; lang segments toggle locale; bell opens popover and shows sample rows + unread count; user menu shows role/scope + items. Keep them behavior-asserting.
- [ ] `pnpm test` + `typecheck` + `lint` green; mockup comparison (light+dark) in report.
- [ ] Commit: `feat(frontend): rebuild app topbar to match App Shell mockup`

### Task 4 — Kantor page 1:1 rebuild

**Files:** `app/pages/master/offices.vue`; `i18n/locales/{id,en}.json`; uses Task-1 `useFloors`, Office `active`, TreeView extensions. **Build to match `docs/design/Master Data Kantor.dc.html` (audit bagian Kantor):**
full split-panel (340px tree | flex-1 detail, no PageHeader/DataToolbar framing, no card wrappers,
independent scroll); tree panel header ("Hierarki Kantor" + inline "Tambah Kantor" + in-panel search);
tree nodes with colored type badge + inactive dot + selected accent; detail header (type chip + status
chip + name 22px + kode mono); detail info card (2-col grid + full-width Alamat); **Lantai & Ruangan**
section (collapsible floor cards + room rows + dashed empty-state CTA); form gains **Induk** + **Aktif**
with 2-col rows (Induk+Jenis, Kode+Provinsi) and a subtitle.
- [ ] Rewrite `test/nuxt/master-offices.spec.ts` for the new structure and ADD coverage: tree renders seeded offices + an inactive indicator; selecting a node shows detail chips/kode; the floors section + empty state render; the form shows Induk + Aktif. Assert real text.
- [ ] `pnpm test` + `typecheck` + `lint` green; mockup comparison (light+dark) in report.
- [ ] Commit: `feat(frontend): rebuild master data kantor to match mockup (split panel + floors/rooms)`

### Task 5 — Pegawai page 1:1 rebuild

**Files:** `app/pages/master/employees.vue`; `i18n/locales/{id,en}.json`; uses `useReference` (departments/positions for the dept/jabatan selects), `useOffices` (Kantor select+filter), DataToolbar `showReset`, FormSlideover `subtitle`. **Build to match `docs/design/Master Data Pegawai.dc.html` (audit bagian Pegawai):**
columns NIP / Nama (avatar initials) / Departemen / Jabatan (pill) / Kantor / Email-Telepon / Status /
Aksi; filter bar as a `bg-base` card with search + 4 dropdowns (Kantor, Departemen, Jabatan, Status),
Reset only when a filter is active; form as `FormSlideover` (480px) with 2-col rows (NIP+Status toggle,
Dept+Jabatan selects, Email+Telepon), Kantor select + scope note, NIP mono, subtitle.
- [ ] Update/extend `test/nuxt/master-employees.spec.ts`: new columns render (Departemen, Email/Telepon), avatar initials, jabatan pill, the filter dropdowns exist, slideover form opens with selects + status toggle. Assert real text/labels.
- [ ] `pnpm test` + `typecheck` + `lint` green; mockup comparison (light+dark) in report.
- [ ] Commit: `feat(frontend): rebuild master data pegawai to match mockup (filters + slideover form)`

### Task 6 — Referensi page 1:1 rebuild

**Files:** `app/pages/master/reference.vue`; `i18n/locales/{id,en}.json`; uses `useReference` (+`active`), `referenceResources`. **Build to match `docs/design/Master Data Referensi.dc.html` (audit bagian Referensi):**
218px secondary entity-nav panel (title/subtitle + entity buttons with per-entity count badges +
active accent) replacing the toolbar `USelect`; main column = page header + search-only (no Reset) +
table + pagination; **status toggle** cell (Aktif/Nonaktif, success-tinted active); form adds an
**Aktif** toggle row + subtitle.
- [ ] Update/extend `test/nuxt/master-reference.spec.ts`: the entity panel lists resources with counts; switching entity changes the table; status renders as a toggle; the form has the Aktif toggle. Assert real text.
- [ ] `pnpm test` + `typecheck` + `lint` green; mockup comparison (light+dark) in report.
- [ ] Commit: `feat(frontend): rebuild master data referensi to match mockup (entity panel + toggles)`

## Final

- [ ] `pnpm lint && pnpm typecheck && pnpm test && pnpm build` all green.
- [ ] Whole-branch review (most capable model) against the mockups + this plan; fix Critical/Important.
- [ ] Final side-by-side check of all four areas vs their mockups in light + dark mode.
