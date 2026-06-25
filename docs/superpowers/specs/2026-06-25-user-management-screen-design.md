# User Management Screen — Design Spec

**Date:** 2026-06-25
**Phase:** Frontend feature screens (mock-first)
**Mockup (source of truth):** `docs/design/Manajemen User.dc.html`
**Route:** `/settings/users` (the sidebar already links this; the page does not exist yet — this also
fixes that broken link)

## 1. Goal

Build the **Manajemen User** (Users) screen from the mockup 1:1: a CRUD table of login accounts with
search + role/office/status filters, a row action menu (edit / reset password / activate-deactivate /
delete), pagination, a slideover create/edit form, and a delete confirmation. Mock-first, behind the
same composable interface a real `$fetch('/users')` will use later.

This screen is a close sibling of the existing `pages/master/employees.vue` and reuses the same
building blocks (`PageHeader`, filter bar, `ResourceTable`, `FormSlideover`, `useConfirm`). Follow that
page's structure.

## 2. Scope

### In scope
- `pages/settings/users.vue` — the screen, gated by `definePageMeta({ middleware: 'can', permission: 'user.manage' })`.
- `User` type, `mock/users.ts` fixtures (12 users ported 1:1 from the mockup), `useUsers` composable.
- Role/office/employee option constants (mock), role-badge color map, 3-state status, login icon.
- Full `id`/`en` i18n under `settings.users.*`.
- Vitest: unit (composable + mock) and a `mountSuspended` page test covering states/branches.

### Out of scope (later)
- Real backend wiring (`/users` CRUD exists server-side, but every other built screen is mock-first;
  this stays consistent). Reset Password is a mock action (toast only) — no email/token flow yet.
- Turning Kantor/Pegawai/Role into real office/employee/role lookups (kept as the mockup's name strings).

## 3. Data model

The mockup models office/employee/role as **plain display strings** (its `KANTOR`, `PEGAWAI`, `PERAN`
constants). Mirror that for fidelity and simplicity; a later pass swaps these for real lookups behind the
same composable.

```ts
// types/index.ts
export interface User {
  id: string
  nama: string
  email: string
  peran: string            // role name (e.g. 'Superadmin', 'Staf')
  kantor: string           // assigned office name ('' = none)
  pegawai: string          // linked employee name ('' = none)
  login: 'email' | 'google'
  status: 'active' | 'inactive' | 'suspended'
  created_at: string
}
```

`mock/users.ts`:
- `userSeed: User[]` — the 12 rows from the mockup (`USERS`), verbatim.
- `userStore = createStore<User>(userSeed)`.
- Exported constants for form/filter options: `ROLES` (`PERAN`), `KANTOR_OPTIONS`, `PEGAWAI_OPTIONS`.
- `roleBadgeColor: Record<string, BadgeColor>` — Superadmin→primary, Kepala Kanwil/Kepala Unit→info,
  Asset Manager→warning, Staf→neutral (default neutral).

`composables/api/useUsers.ts` — same shape as `useEmployees`:
- `list(query)` → `Paginated<User>` (filter on `nama`/`email`), `get`, `create(UserInput)`,
  `update(id, UserInput)`, `remove(id)`, plus `setStatus(id, status)` for the activate/deactivate action
  and `resetPassword(id)` (no-op resolving void — the real seam later). Sentinel error
  `'settings.users.errNotFound'` on missing update target.
- `UserInput` = the editable fields (`nama, email, password?, peran, kantor, pegawai, status`).
  `password` is write-only and never stored on the row (mock ignores it; real API would hash it).

## 4. UI structure (mirror the mockup)

- **PageHeader** — title `settings.users.title` ("Pengguna"/"Users"), subtitle, and a `Can` "Tambah User"
  button (`user.manage`).
- **Filter bar** (same card as employees) — search (`UInput`, name/email), three `USelect`s
  (Role / Office / Status, each with an "all" sentinel), and a Reset button shown only when any filter
  is active.
- **ResourceTable** — columns: `nama` (avatar initials + name + email), `peran` (color `UBadge`),
  `kantor`, `pegawai` (em-dash when empty, dimmed), `login` (icon + label: `i-lucide-mail` / google
  `i-simple-icons-google`), `status` (dot + `UBadge`: success/neutral/warning), and `row-actions`.
  Pagination via the table's built-in footer (`total`/`limit`/`offset`), page size 10.
- **Row actions** — a `UDropdownMenu` (3-dot trigger) with: Edit User, Reset Password (toast),
  Activate/Deactivate (toggles active↔inactive), and Delete (destructive, opens confirm). Gated by `Can`.
- **FormSlideover** — fields in mockup order: Nama\* , Email\* , Password (+ "leave empty for Google"
  note), a 2-col row of Role\* + Status, Office, Linked Employee (+ sync note). Validation: Nama and
  Email required, Email format-checked; errors surface inline / via toast on submit.
- **Empty state** — `ResourceTable`'s empty slot; copy differs for "no users" vs "no match for filters".

## 5. i18n

New `settings.users.*` keys in `id`/`en`: `title`, `subtitle`, `add`, `searchPlaceholder`,
`columns.*` (nama/peran/kantor/pegawai/login/status), `filter.*` (allRoles/allKantor/allStatus),
`status.*` (active/inactive/suspended), `login.*` (email/google), `actions.*`
(edit/resetPassword/activate/deactivate/delete), `menu`-level toasts (`passwordReset`, `statusChanged`),
`empty`/`emptyFilter`, `createTitle`/`editTitle`/`createSub`/`editSub`, `fields.*`, `placeholders.*`,
`passwordNote`, `pegawaiNote`, `deleteConfirm`, `errNotFound`, validation (`required`, `invalidEmail`).

## 6. Testing

- **Unit (node):** `useUsers` — list filters by name/email; create prepends; update patches & throws the
  sentinel on missing id; remove deletes; `setStatus` flips status; password is never persisted on the row.
  `mock/users.ts` — seed has 12 rows; `roleBadgeColor` maps each role.
- **Component (nuxt env, `mountSuspended`):** renders rows after latency; role badge + status dot text;
  login icon/label (email vs google); search narrows rows; each filter narrows rows; reset clears
  filters; pagination advances; empty state (no rows + filtered-empty); create via slideover adds a row;
  edit updates a row; activate/deactivate toggles the status; delete (confirmed) removes a row; required
  + email-format validation blocks submit. Assert real rendered/resolved text, not hollow checks.

## 7. Files

**New:** `pages/settings/users.vue`, `mock/users.ts`, `composables/api/useUsers.ts`, `test/unit/users-mock.spec.ts`, `test/nuxt/settings-users.spec.ts`.
**Modified:** `types/index.ts` (+`User`), `mock/index.ts` (re-export), `i18n/locales/{id,en}.json` (+`settings.users.*`), `docs/PROGRESS.md`.

## 8. Verification (DoD)

- `pnpm lint` · `pnpm typecheck` · `pnpm test` · `pnpm build` green.
- Live 1:1 comparison of `/settings/users` vs `docs/design/Manajemen User.dc.html` in light **and** dark
  — table, badges, row menu, slideover, confirm, empty/pagination states. Fix any deviation before done.
