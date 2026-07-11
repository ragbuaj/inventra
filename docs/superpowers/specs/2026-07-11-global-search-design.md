# Global Search (`GET /api/v1/search`) — Design

**Tanggal:** 2026-07-11 · **Status:** Disetujui user (brainstorming session)
**Referensi:** PROGRESS item 39 kandidat (f) · mockup
`docs/design/Global Search.dc.html` (UI `CommandPalette.vue` sudah dibangun 1:1 — fase ini
wiring data saja) · ADR-0008 (module file split) · pola scope `all_scope`/`office_ids` di
`db/queries/*.sql`

## Tujuan

Mengganti sumber data mock pada command palette (⌘K) dengan endpoint backend riil
`GET /api/v1/search`, dengan otorisasi (permission + data scope) yang identik dengan endpoint
list per entitas yang sudah ada — lalu menghapus file `mock/*` yang menjadi yatim. Ini item
terakhir yang menahan beberapa `mock/*` sejak fase wiring (PROGRESS item 12/16/17/24).

## Keputusan arsitektur (dikonfirmasi user)

**Opsi A — "CQRS level kode"**: read model seragam (`SearchRow`) dibentuk on-the-fly oleh
5 query sqlc per entitas yang masing-masing ter-scope, + indeks **pg_trgm** agar
`ILIKE '%q%'` memakai indeks. Dipilih di atas dua alternatif yang dieksplorasi:

- **B — view `UNION ALL`**: ditolak — WHERE otorisasi 5-cabang (≥10 parameter scope) menjadi
  titik risiko keamanan tunggal, tanpa keuntungan performa nyata.
- **C — tabel proyeksi + trigger (CQRS level 2)**: ditolak untuk search — data kecil, matching
  murah, dan otorisasi tetap harus dievaluasi saat baca sehingga manfaat pre-bake hilang.
  Pola proyeksi disimpan untuk fase **Reporting & Dashboard** (agregasi mahal — di sanalah
  level 2 terbayar; lihat FR-5.3/NFR performa).

## Kontrak API

`GET /api/v1/search?q=<teks>` — `RequireAuth` saja di route; gate per entitas di service.
Ikut rate-limit global per-IP (`PerIP` di grup `/api/v1`); tidak ada limiter khusus.

- `q` wajib, **minimal 2 karakter** setelah trim; di bawah itu → `200 {"groups": []}`.
- Maks **5 item per grup** + `total` (COUNT penuh) per grup; grup kosong tidak dikirim.
- Urutan grup tetap: `assets, employees, offices, users, requests`.

```json
{
  "groups": [
    {
      "type": "assets",
      "total": 12,
      "items": [
        {
          "id": "8c9f…",
          "title": "Laptop Dell Latitude 5440",
          "subtitle": "JKT01-ELK-2026-00001",
          "status": "available",
          "asset_tag": "JKT01-ELK-2026-00001"
        }
      ]
    }
  ]
}
```

`asset_tag` hanya hadir pada item `assets` (bahan route `/assets/:tag`). Field lain
seragam: `id`, `title`, `subtitle`, `status` (nullable). DTO English snake_case sesuai
konvensi. Frontend yang memetakan `type` → ikon / `labelKey` / route.

## Otorisasi per entitas — cermin gate endpoint list yang sudah ada

Tidak ada aturan otorisasi baru. Grup yang gagal gate **dilewati tanpa error** (bukan 403) —
Staf tidak pernah tahu grup Users ada.

| Grup | Permission (programatik, `PermissionService`) | Data scope (`CallerOfficeScope`) | Kolom dicari |
|---|---|---|---|
| `assets` | `asset.view` | modul `assets` | `name`, `asset_tag`, `serial_number` |
| `employees` | — (auth saja, cermin `GET /employees`) | modul `employees` | `name`, `code` |
| `offices` | — (auth saja, cermin `GET /offices`) | modul `offices` | `name`, `code` |
| `users` | `user.manage` (praktik: Superadmin) | tanpa scope (cermin `GET /users`) | `name`, `email` |
| `requests` | — (auth saja, cermin `GET /requests`) | modul `requests` | `reason` + prefix `id` |

Scope di-resolve **per modul** (scope caller bisa berbeda antara `assets` dan `employees`);
fallback konservatif `own` mengikuti perilaku `CallerOfficeScope` yang ada.

### Catatan khusus grup `requests`

`approval.requests` tidak punya kolom judul — mock lama mencari `judul` sintetis yang tidak
ada di skema. Desain riil: matching pada `reason` (satu-satunya teks bebas) + kecocokan
**prefix `id`** (cast text); `title` dirender dari `type + office_name` (join enriched, pola
`rowTitle()` halaman Approval), `subtitle` = id pendek, `status` = status request.

## Backend

- Modul baru `internal/search` dengan split ADR-0008: `service.go` (gate permission +
  scope + orkestrasi 5 query paralel via `errgroup`, sentinel error), `dto.go`
  (`SearchRow`/`SearchGroup` + serialisasi), `handler.go` (bind `q` → service → JSON),
  `routes.go` (`GET /search`, `RequireAuth`).
- 5 query sqlc baru di `db/queries/search.sql`, masing-masing mengikuti pola
  `sqlc.arg(all_scope)::boolean OR office_id = ANY(sqlc.arg(office_ids)::uuid[])` +
  `ILIKE '%'||q||'%'` + `COUNT(*) OVER()` (atau query count terpisah) + `LIMIT 5`.
  Query `users` tanpa scope params (cermin `users.sql`); query `requests` join enriched
  (office name) seperti `ListRequestsEnriched`.
- **Migration baru** (nomor berikutnya saat implementasi): `CREATE EXTENSION IF NOT EXISTS
  pg_trgm` + indeks **GIN trigram** pada semua kolom yang dicari (partial `WHERE deleted_at
  IS NULL` mengikuti konvensi). Bonus: endpoint list lama yang sudah ILIKE ikut terakselerasi.
  `.down.sql` men-drop indeks (extension dibiarkan — bisa dipakai objek lain).
- Wiring di `NewRouter` mengikuti pola stockopname/assignment (service butuh `*sqlc.Queries`,
  `authz.PermissionService`, `common.ScopedDeps`).
- OpenAPI: tag `Search` + path `/api/v1/search` (hanya query param `q`; pola deskripsi
  mengikuti path `/api/v1/users`), Spectral wajib hijau.

## Frontend

- `useGlobalSearch` ditulis ulang: `$fetch` ke `/search` via helper `runtimeConfig.public.apiBase`
  yang ada; **signature `search(q): Promise<SearchGroup[]>` tidak berubah** sehingga
  `CommandPalette.vue` nyaris tak tersentuh. Mapping `type` backend → `SearchEntityType`
  UI (`assets→aset`, `employees→pegawai`, `offices→kantor`, `users→user`, `requests→pengajuan`),
  ikon + `labelKey` dari konstanta yang ada.
- Route per item: `assets` → `/assets/${asset_tag}`; `employees` → `/master/employees`;
  `offices` → `/master/offices`; `users` → `/settings/users`; `requests` → `/approval`.
- Status badge dipetakan dari enum backend (mis. `available`) ke key status yang sudah
  dipakai `StatusBadge` di layar riil — tidak menampilkan enum mentah.
- **Debounce ±250 ms** pada watcher query `CommandPalette.vue` (guard `seq` yang ada tetap
  dipertahankan); query <2 karakter tidak memanggil API.

### Pembersihan mock (blast radius terverifikasi 2026-07-11)

- **Dihapus** (yatim setelah rewiring): `mock/offices.ts`, `mock/employees.ts`,
  `mock/users.ts`, `mock/approval.ts`, + `test/unit/approval-mock.spec.ts` (menguji mock
  yang dihapus).
- **Dipertahankan**: `mock/helpers.ts` (dipakai `useDashboard`/`useReports`/`useAccount` —
  fase Reporting), `mock/assets.ts` (dipakai `pages/assets/import.vue` — fase Import),
  `mock/dashboard.ts`/`mock/reports.ts`/`mock/notifications.ts` (konsumennya belum di-wire).

## Deviasi dari mockup (disetujui user, konvensi catat-deviasi)

1. **Debounce 250 ms** — perilaku baru yang tidak ada di `Global Search.dc.html`
   (mock menembak setiap ketikan; boros terhadap backend riil).
2. **Title grup Pengajuan = `type + office`** — skema riil tidak punya kolom judul;
   mock menampilkan `judul` sintetis.

## Pengujian

- **Backend integration** (testcontainers, pola modul lain): (a) Superadmin global melihat
  kelima grup; (b) Staf scope `own` hanya melihat aset kantornya sendiri dan **tanpa** grup
  `users`; (c) scope `office_subtree` Kanwil menyaring lintas kantor; (d) grup `requests`
  match via `reason` dan via prefix id; (e) `q` 1 karakter → groups kosong; (f) limit 5 +
  `total` benar saat hasil > 5. Unit test untuk mapping DTO.
- **Frontend**: `useGlobalSearch.spec` ditulis ulang terhadap stub `$fetch` (empty query
  tidak memanggil API; mapping type/ikon/route; urutan grup; cap 5). `CommandPalette.spec`
  diperbarui + **stub API** (pelajaran terdokumentasi: konsumen composable yang di-rewire
  wajib di-stub agar suite tidak menembak `:8080`). Spec `GlobalSearch.spec`/`useCommandPalette.spec`
  tidak tersentuh (decoupled dari sumber data).
- **E2E real-backend baru** `e2e/global-search.spec.ts`: buat aset unik per run via API →
  login → Ctrl+K → ketik nama unik → grup Aset muncul → klik → mendarat di Detail Aset;
  + asersi empty-state untuk query tanpa hasil. Ikuti konvensi e2e persistent-data
  (nama unik per run, clearCookies bila ganti user).
- **Gate penuh sebelum klaim selesai**: `go build/vet/test ./...` + `-tags=integration`,
  Spectral, `pnpm lint/typecheck/test/build`, e2e.

## Di luar scope (eksplisit)

- Tombol "Lihat semua (n)" pada grup hasil tetap non-fungsional seperti sekarang
  (mockup tidak mendefinisikan targetnya; kandidat follow-up: navigasi ke list page dengan
  query terisi).
- `useNotifications`, `useDashboard`, `useReports`, import wizard — tetap mock
  (fase/modulnya sendiri).
- Ranking lintas entitas / fuzzy scoring — tidak dibutuhkan UI saat ini.
