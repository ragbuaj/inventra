# Master Data + App Shell — Design-Fidelity Audit & Fix Spec

**Date:** 2026-06-24
**Trigger:** User reported the navbar/shell and the Kantor/Pegawai/Referensi pages deviate from the
`docs/design` mockups. Goal: bring them to **1:1** with the mockups (per the CLAUDE.md design-fidelity
rule), reproducing every state with mock data.

**Source of truth (open these, build to match exactly):**
- `docs/design/App Shell.dc.html`
- `docs/design/Master Data Kantor.dc.html`
- `docs/design/Master Data Pegawai.dc.html`
- `docs/design/Master Data Referensi.dc.html`

## Scope decisions (approved by user)

1. **Lantai & Ruangan (floors/rooms)** on the Kantor detail — **build now** (was deferred in the
   Phase-1 spec). Add mock + composable for floors/rooms and reproduce the section fully.
2. **Sidebar nav** — reproduce the **full Superadmin menu** structure (groups *Operasional* /
   *Administrasi*, collapsible parent groups with sub-items). Items whose pages are **not built yet**
   (Laporan, Aset→Katalog/Import/Label & Barcode, Penugasan, Maintenance, Pengajuan & Approval,
   Lokasi & Geografi, Peran & RBAC, Data Scope, Field-Permission, Audit Trail) are shown **disabled /
   "coming soon"** (no navigation, no 404). Built destinations (Dashboard, Master Data → Kantor /
   Pegawai / Referensi, Pengaturan → User) link normally.
3. **Notifications / badges / active status** — reproduce **fully with mock data**: notification
   popover with sample items, nav badge counts, and `active`/inactive indicators. Add an `active`
   field to the Office and ReferenceRow mocks; add a mock notifications source. All behind the same
   composable seam so it swaps to real data later.

## Constraints (unchanged)

Build on `U*` components, theme via semantic tokens (map the mockup's literal hex to `--ui-*` /
`color="primary"` / `text-muted` — only deviate from a token when the rendered result would visibly
differ from the mockup), i18n in both `id`/`en`, no trailing commas / 1tbs, `pnpm lint` +
`pnpm typecheck` + `pnpm test` must pass, mock-first behind `composables/api/`. Every screen ends with
a **side-by-side comparison** against its mockup and a coverage re-check (per CLAUDE.md workflow).

---

## App Shell deviations (→ `layouts/default.vue`, `AppSidebar`, `AppTopbar`, shell child components)

Structural:
- **Topbar breadcrumb + page-title block** missing — add the two-line block (breadcrumb line
  `Inventra › <parent>` + `h1` page title) between the sidebar-toggle and the search. Wire/ mount
  `AppBreadcrumb` (currently dead code).
- **Language switcher** is a dropdown — replace with an inline segmented **ID / EN** control (both
  options always visible; active segment `bg-base`, container `bg-muted` pill, `rounded-[9px]`).
- **Notification popover** missing — `NotificationBell` must open a ~330px panel (header + "Mark read"
  + scrollable rows of icon/text/time + "View all" footer), driven by a mock notifications source;
  show the unread count badge.
- **Sidebar collapsible groups + sub-items** — replace flat links with parent groups (Aset, Master
  Data, Pengaturan) that expand to indented children with a rotating chevron; section labels
  *Operasional* / *Administrasi*.
- **Sidebar bottom user strip** — add a `border-top` footer: avatar (initials) + name + scope label
  (hidden when collapsed).
- **Nav item badges** — optional count badge per item (e.g. Pengajuan & Approval = 8) from mock data.
- **Search centering** — wrap `GlobalSearch` in `flex-1 justify-center`; `max-width:420px`; add the
  `⌘K` shortcut pill inside the input.
- **User menu** — trigger is a pill (`rounded-full`, border, 30px avatar, chevron; no name text in the
  trigger); panel adds a **role/scope** section (shield badge + role + scope) and a **"Pengaturan
  Akun"** item alongside Profil and the red Keluar.

Visual:
- Sidebar widths **264px** expanded / **76px** collapsed (currently w-60 / w-16).
- Active nav item **3px left accent bar** (`inset 3px 0 0 var(--ui-primary)`) + `primary-soft` bg.
- Section label letter-spacing `.14em` + `pt-[14px]`; nav hover `bg-muted` (not `bg-elevated`).
- Topbar toggle / theme / bell buttons: outlined 36×36 `rounded-[9px]` (not borderless ghost);
  bell badge positioned per mockup. Right-cluster `gap-2`. Topbar `z-30`.
- Logo mark icon = archive/box shape (mockup) not `i-lucide-package`; app name `tracking-tight`.
- `layouts/default.vue` main padding `px-8 py-7`; page canvas closer to `--ui-bg` (lighter than
  `bg-muted`). `PageHeader` title `text-2xl font-bold tracking-tight`, `mb-[22px]`.

---

## Kantor deviations (→ `pages/master/offices.vue`, `TreeView`, floors/rooms mock)

Structural:
- **Full split-panel layout** — remove the PageHeader+DataToolbar+grid framing. Left **340px** tree
  panel (`bg-base`, border-right, no `UCard`, no gap): its own header = "Hierarki Kantor" title +
  inline primary "Tambah Kantor" + a search input *inside* the panel. Right panel = `flex-1` detail
  (tinted `bg`). Panels scroll independently; no card wrappers.
- **Detail header** — type chip (colored by office type) + status chip (Aktif/Nonaktif + dot) +
  office name (22px/700) + kode (monospace, muted).
- **Detail info card** — a bordered/`rounded-[13px]`/shadow card with a 2-col grid (Jenis, Induk,
  Provinsi, Kota) + Alamat spanning full width.
- **Lantai & Ruangan section** — section header + "Tambah Lantai"; collapsible floor cards (chevron,
  icon, floor name, room count, add-room, delete-floor); room rows (name, kode, delete-room); dashed
  empty-state card with a "Tambah Lantai" CTA when none. Backed by a floors/rooms mock + composable.
- **Form** — add **Induk** (parent select) and **Aktif** toggle; lay out Induk+Jenis and Kode+Provinsi
  as 2-col rows; Kode monospace; form subtitle line.

Visual (TreeView):
- Colored **type-icon badge** per node (Pusat/Wilayah/Cabang/Outlet), **inactive dot**, **3px left
  accent** on the selected node (`primary-soft` bg).

---

## Pegawai deviations (→ `pages/master/employees.vue`)

Structural:
- **Columns** — add **Departemen** and **Email / Telepon** (email + phone stacked). Full set: NIP,
  Nama, Departemen, Jabatan, Kantor, Email/Telepon, Status, Aksi.
- **Nama cell** — 30px avatar with initials (`primary-soft`).
- **Filter bar** — a `bg-base` card (border, `rounded-[13px]`, shadow, p-14px) containing search + **4
  filter dropdowns**: Kantor, Departemen, Jabatan, Status. Reset shown **only when a filter is active**.
- **Form** — switch `FormModal` → **`FormSlideover`** (480px). Layout: NIP + Status **toggle** on one
  row; Nama full width; Departemen + Jabatan **selects** on one row; Kantor select (full width) + a
  scope note; Email + Telepon on one row. NIP monospace. Form subtitle line.

Visual:
- **Jabatan cell** rendered as a neutral pill badge.

---

## Referensi deviations (→ `pages/master/reference.vue`, reference mock)

Structural:
- **Secondary entity-nav panel** — a **218px** `bg-base` border-right panel (title "Master Data" /
  subtitle "Data referensi") listing every reference entity as a button with a **per-entity item
  count** badge and active-state styling (`primary-soft` + left accent). Remove the `USelect` entity
  switcher from the toolbar.
- **Status cell** — inline **toggle** (Aktif/Nonaktif, success-tinted when active), not plain text.
- **Form** — add an **Aktif** toggle row at the bottom of the modal body; form subtitle line.
- Reference rows gain an `active` field in the mock; counts come from the per-resource stores.

Visual:
- Remove the unconditional Reset button on this page (mockup has none).

---

## Shared-component changes (fold into the tasks that need them)

- `DataToolbar` — add a `showReset` prop (default true) so pages can hide Reset; allow filter slot use.
  (Kantor stops using DataToolbar entirely; Referensi hides Reset.)
- `PageHeader` — title `text-2xl font-bold tracking-tight`, `mb-[22px]` (used by Pegawai; Kantor/Referensi
  use in-panel headers instead).
- `FormModal` / `FormSlideover` — add an optional `subtitle` prop rendered under the title.
- `TreeView` — support a colored type badge (`iconBg`/`iconColor` on the node), an `inactive` flag
  (dot), and the selected-node left accent.
- Mock additions: `active` on Office + ReferenceRow seeds; floors/rooms fixtures + composable; a mock
  notifications source + nav badge counts.
