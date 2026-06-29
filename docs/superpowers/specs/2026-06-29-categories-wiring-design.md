# Wire Kategori Aset (Asset Categories) screen to `/api/v1/categories` — Design

| | |
|---|---|
| **Tanggal** | 2026-06-29 |
| **Area** | Backend (`internal/masterdata/category`: tree endpoint) + Frontend (`pages/master/categories.vue`, `composables/api/useCategories.ts`, `components/category/CategoryFormSlideover.vue`) |
| **Backend** | `/api/v1/categories` CRUD (gate `masterdata.global.manage`, global/no-scope) + new `GET /api/v1/categories/tree` |
| **Status** | Disetujui — siap implementasi |

> Sub-proyek **ketiga** dari batch "wiring menu master" (Peta Lokasi #39, Referensi #40 selesai). Roadmap berikutnya: Pegawai → Kantor.

## 1. Konteks & lingkup

Layar **Kategori Aset** (`frontend/app/pages/master/categories.vue`) mengelola taksonomi kategori aset tetap (tabel datar ter-indentasi membentuk pohon parent/child) dengan DTO **terkaya** (PSAK 16 penyusutan + PMK 72/2023 fiskal + akun GL + batas kapitalisasi). Saat ini **mock penuh** (`useCategories` membaca `~/mock/categories`). Backend modul `category` **sudah lengkap** untuk CRUD; tipe `Category` frontend sudah **English snake_case persis cocok** dengan JSON backend (tanpa rename), enum sudah cocok, form 4-seksi + pembangunan tree (DFS client-side) sudah ada. Sub-proyek ini: (a) menambah **endpoint tree** (memuat seluruh pohon tanpa batas 100 baris), lalu (b) mewire frontend ke backend nyata + memindahkan util mock + hapus mock.

### Temuan yang menentukan desain
- **Kontrak CRUD** (`/api/v1/categories`): `GET` list (`{data,total,limit,offset}`, search/limit(1–100)/offset), `GET /:id` (objek flat), `POST` (201 flat), `PUT /:id` (200 flat), `DELETE /:id` (204). Reads `authMW`; writes `authMW + masterdata.global.manage`. **Tanpa data-scope** (global).
- **Field DTO (12)** sudah selaras penuh frontend↔backend (English keys): `name`(req), `code?`, `parent_id?`(uuid), `default_depreciation_method?`(`straight_line`|`declining_balance`), `default_useful_life_months?`(int), `default_salvage_rate?`(numeric→**string**), `asset_class`(`tangible`|`intangible`, default tangible), `default_fiscal_group?`(`kelompok_1..4`|`bangunan_permanen`|`bangunan_non_permanen`|`non_susut`), `default_fiscal_life_months?`(int), `gl_account_code?`, `capitalization_threshold?`(numeric→**string** mis. `"1000000.00"`), `is_active`. Response juga punya `created_at`/`updated_at`(string).
- **Numeric** (`default_salvage_rate` `numeric(5,4)`, `capitalization_threshold` `numeric(18,2)`) dikembalikan sebagai **string** (sqlc override). Frontend sudah memperlakukannya sebagai string; `formatThousands`/`parseThousands` menangani `.00`.
- **Tree**: backend **tak punya** query tree — hanya `ListCategories` datar (limit 1–100). Halaman saat ini fetch `limit:100` lalu membangun pohon via DFS client-side. **Batas 100 → truncation diam-diam bila kategori >100.** Keputusan: tambah endpoint tree.
- **Util di mock**: `CategoryFormSlideover.vue` + `categories.vue` mengimpor `FISCAL_GROUPS`, `isBuildingGroup`, `formatThousands`, `parseThousands` dari `~/mock/categories` — harus dipindah saat mock dihapus.
- **i18n**: `non_susut` (nilai enum sah) **tak punya** label di `masterdata.categories.fiscalGroup.*` → render raw key bila backend mengembalikannya.
- **`updated_at`**: dikembalikan backend, belum ada di tipe `Category` frontend.
- **Pelajaran #40**: mewire `useCategories` mock→HTTP membuat **setiap konsumen lain** memanggil jaringan nyata di test yang tak men-stub API (ECONNREFUSED :8080 → unhandled rejection → suite exit 1). Wajib `grep` semua konsumen `useCategories(` dan pastikan test mereka men-stub API.

### Keputusan yang disetujui
- **Endpoint tree khusus** (Q opsi 3): `GET /api/v1/categories/tree` mengembalikan **seluruh** kategori non-deleted **tanpa paginasi**. **Bentuk = datar** (`{data:[...]}`), bukan JSON nested — frontend mempertahankan DFS client-side-nya (tanpa mengubah logika tree). Halaman beralih dari `list({limit:100})` ke `tree()`.

## 2. Backend — endpoint tree

### 2.1 Query (`backend/db/queries/categories.sql`)
```sql
-- name: ListCategoryTree :many
SELECT * FROM masterdata.categories
WHERE deleted_at IS NULL
ORDER BY name;
```
Lalu `sqlc generate`.

### 2.2 Service + handler + route (`internal/masterdata/category/`)
- `service.go`: tambah `Tree(ctx) ([]sqlc.MasterdataCategory, error)` → `q.ListCategoryTree(ctx)`.
- `handler.go`: tambah `tree(c *gin.Context)` → ambil rows, serialisasi `[]Response` via `toResponse`, balas `c.JSON(200, gin.H{"data": data})` (set lengkap; tanpa total/limit/offset).
- `routes.go`: tambah `g.GET("/tree", authMW, h.tree)` **sebelum** `g.GET("/:id", ...)` (Gin v1.12 mengizinkan segmen statis `tree` berdampingan dgn param `:id`).
- openapi `backend/api/openapi.yaml`: tambah path `GET /categories/tree` (200 `{data: array of Category}`); Spectral hijau.
- Tes Go (gaya category test existing bila ada; jika tidak, integrasi seperti office): `Tree` mengembalikan semua kategori non-deleted (termasuk yang melampaui 100), parent_id passthrough, soft-deleted dikecualikan.

## 3. Frontend — composable, page, util, tipe, i18n

### 3.1 `useCategories.ts` — tulis ulang ke HTTP
Hapus impor `~/mock/*`. Via `useApiClient().request`:
- `list(query): Promise<Paginated<Category>>` — `GET /categories?search&limit&offset` (omit kosong).
- `get(id): Promise<Category>` — `GET /categories/:id`.
- `create(input: CategoryInput): Promise<Category>` — `POST /categories`.
- `update(id, input): Promise<Category>` — `PUT /categories/:id`.
- `remove(id): Promise<void>` — `DELETE /categories/:id`.
- **`tree(): Promise<Category[]>`** — `GET /categories/tree`, kembalikan `res.data`.
(Jika `list`/`get` ternyata tak diacu konsumen lain setelah halaman beralih ke `tree()`, tetap dipertahankan untuk mencerminkan kontrak backend + CRUD; plan memverifikasi pemakaian.)

### 3.2 Page `categories.vue`
- Pemuatan data: ganti `api.list({ limit: 100 })` → **`api.tree()`** (set lengkap → `allRows`). Paginasi (PAGE_SIZE=7), filter (search/kelas/golongan/aktif), dan pembangunan `orderedRows` (DFS) tetap client-side seperti sekarang.
- Tambah state **error muat + retry** bila belum ada (bungkus pemuatan dgn try/catch → `loadFailed`).
- CRUD (create/update/remove) lewat composable; setelah sukses → refresh via `tree()`.
- Guard tetap `masterdata.global.manage`.

### 3.3 Util mock → constants
Pindahkan `FISCAL_GROUPS`, `isBuildingGroup`, `formatThousands`, `parseThousands` (+ konstanta terkait bila ada) dari `mock/categories.ts` ke **`app/constants/categoryMeta.ts`**. Update importer: `categories.vue` + `components/category/CategoryFormSlideover.vue`. Hapus `mock/categories.ts` setelah tak ada importer tersisa (cek barrel `mock/index.ts`).

### 3.4 Tipe & i18n
- `~/types` `Category`: tambah `updated_at?: string | null`.
- i18n `masterdata.categories.fiscalGroup`: tambah `non_susut` (id: "Non-Penyusutan", en: "Non-Depreciable") di kedua locale. Semua string user-facing via `$t`.

## 4. Pengujian (proaktif & luas)

- **Backend** (`category`, integrasi/unit sesuai gaya existing): `Tree` mengembalikan semua kategori non-deleted (uji >100 baris untuk membuktikan tak ada cap), kecualikan soft-deleted, parent_id passthrough; CRUD existing tetap hijau.
- **Unit** (`test/unit/use-categories.spec.ts`, mock `useApiClient`; hapus tes mock lama bila ada): tiap verb membangun path/body benar (`list` query, `get/create/update/remove`, **`tree`→`GET /categories/tree` mengembalikan array**).
- **Component** (`test/nuxt/master-categories.spec.ts` — **cek apakah file sudah ada → tulis ulang, bukan buat**; stub `/categories/tree`): render baris + **indentasi child** (parent_id), filter kelas/golongan/aktif menyaring, **parent picker meng-exclude self+descendant** saat edit, **building group → metode terkunci straight_line**, create→`POST` body benar, edit/delete, numeric `capitalization_threshold` string ter-format. Assert perilaku nyata.
- **E2E** (`test/e2e` / `e2e/`): backend nyata + admin — buat kategori induk, lalu buat kategori anak memilih induk via picker (trigger+`role=option`, **bukan** `selectOption`), assert anak tampil ter-indentasi di bawah induk. Locator robust (`data-testid` bila perlu; tanpa `getByText` exact:false ambigu / `.first()` div luas / accessible-name yang memuat badge).

## 5. "Selesai"
- Backend: `go build/vet/test ./...` + `go test -tags=integration ./...` + Spectral hijau.
- Frontend: `pnpm lint/typecheck/test/build` hijau (E2E di CI). **Verifikasi exit code `pnpm test` = 0** (cek konsumen `useCategories` lain ter-stub — pelajaran #40).
- Hapus `mock/categories.ts` (cek importer + barrel; util pindah ke `constants/categoryMeta.ts`).
- Bandingkan `categories.vue` ke `docs/design/Kategori Aset.dc.html`: tabel ter-indentasi (8 kolom), filter-bar, slideover 4 seksi cocok 1:1. Tak ada deviasi yang diharapkan (semua field sudah selaras).
- `docs/PROGRESS.md`: Kategori Aset ✅ wired ke `/api/v1/categories` (+ `GET /categories/tree`); refresh "▶ Next session" → **Pegawai**.

## 6. Risiko & catatan
- **Route `/categories/tree` vs `/:id`**: daftarkan statis lebih dulu (Gin 1.12 aman).
- **Konsumen `useCategories` lain** (mis. picker kategori di form aset): grep + stub API di test mereka agar suite tetap exit 0 (pelajaran #40).
- **Numeric string**: `capitalization_threshold`/`default_salvage_rate` lewat sebagai string (sqlc numeric override); `formatThousands`/`parseThousands` (kini di constants) menanganinya, termasuk sufiks `.00` dari `numeric(18,2)`.
- **`non_susut`**: kini punya label; bila form sengaja menyembunyikan opsi `non_susut`, biarkan (hanya label tabel yang perlu, bukan opsi form).

## 7. Roadmap batch (konteks)
1. Peta Lokasi — ✅ #39.
2. Referensi — ✅ #40.
3. **Kategori Aset** — sub-proyek ini.
4. Pegawai — scope `employees` + FK picker (office/department/position); **halaman employees konsumen `useReference`** (sudah di-stub di test #40) dan akan jadi yang di-wire.
5. Kantor + Lantai + Ruangan — tree, scope, floors/rooms inline, input lat/lng.
