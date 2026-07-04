# Wiring layar Pengajuan & Approval ke `/api/v1/requests` — Design

**Tanggal:** 2026-07-04 · **Status:** Disetujui user (brainstorming session)
**Referensi:** mockup `docs/design/Pengajuan Approval.dc.html` · backend `internal/approval` ·
PROGRESS.md butir 23 kandidat (a)

## Tujuan

Layar `/approval` (`frontend/app/pages/approval.vue`) saat ini full mock
(`mock/approval.ts` + `useApproval` fake-latency). Sisi **submit** sudah wired sejak Task 21
(`useAssetRequests` → `POST /requests`). Task ini me-wire sisi **inbox + decide**:
daftar pengajuan, detail, dan aksi approve/reject — terhadap backend approval yang sudah
lengkap dan teruji (`GET /requests`, `GET /requests/inbox`, `GET /requests/:id`,
`POST /requests/:id/approve|reject`).

## Keputusan produk (sudah dikonfirmasi user)

1. **Backend di-enrich** (bukan degrade frontend): payload + nama pengaju/approver/kantor
   diekspos; FilterView entity `requests` ikut di-wire (menutup sebagian TODO #10).
2. **Tab Pending = inbox** (`GET /requests/inbox`, hanya yang eligible di-decide);
   Approved/Rejected/Cancelled/All = `GET /requests?status=`.
3. **Lampiran**: empty-state permanen — section tetap dirender dengan teks "tidak ada
   lampiran" (request belum punya mekanisme lampiran; BAST tercipta pasca-approval di
   modul lain). Deviasi dicatat di PROGRESS.md.
4. **Tab Cancelled ditambahkan** (tab ke-5, deviasi dari 4 tab mockup — atas permintaan
   user). Dicatat di PROGRESS.md.

## 1. Backend — enrichment (modul `internal/approval`)

### Query sqlc (`db/queries/approval.sql`)

Perluas (atau tambah varian enriched dari) query yang ada — JOIN LEFT ke
`identity.users u ON u.id = requested_by_id`, `identity.roles ro ON ro.id = u.role_id`,
`masterdata.offices o ON o.id = office_id`:

- `ListRequests` + `ListInboxCandidates` + `GetRequest` → tambah kolom
  `u.name AS requested_by_name`, `ro.name AS requested_by_role`, `o.name AS office_name`.
- `ListRequestApprovals` → LEFT JOIN users pada `approver_id` → `approver_name`.

Catatan: kolom nama user adalah `identity.users.name` (bukan `full_name`); role dari
`identity.roles.name` via `users.role_id`. Semua JOIN LEFT + filter soft-delete mengikuti
konvensi query yang ada. `sqlc generate` setelah edit.

### Serialisasi & handler

- `requestToMap` (list/inbox/detail) memuat field baru: `requested_by_name`,
  `requested_by_role`, `office_name` (nullable → `*string`).
- **Hanya `GET /requests/:id`**: tambah `payload` (JSON mentah di-unmarshal ke `any`
  agar jadi objek di response, bukan string) dan `steps[]` yang kini membawa
  `approver_name`. List/inbox TIDAK memuat payload (hemat + kartu kiri tak butuh).
- **FilterView entity `requests`**: handler sudah memegang `fieldSvc`. Terapkan
  `ForEntity(roleID, "requests")` + `FilterView` pada map hasil serialisasi di `list`,
  `inbox`, dan `get`. Field keys yang dikatalogkan: `amount`, `payload`, `reason`.
  Default-allow — tanpa konfigurasi, tidak ada perubahan perilaku; role yang di-deny
  kehilangan key tsb. dari response.
- Submit, decide, cancel, executor, threshold: **tidak berubah**.

### OpenAPI + test

- `Request` schema + field enrichment; schema respons `GET /requests/:id` ditambah
  `payload` + `steps[].approver_name` (boleh via schema `RequestDetail`).
- Integration test (`//go:build integration`): (a) list/inbox/get memuat nama pengaju,
  role, kantor yang benar; (b) steps memuat approver_name setelah decide; (c) get memuat
  payload utuh; (d) FilterView men-drop `amount`/`payload` untuk role yang di-deny;
  (e) default-allow tanpa policy.

## 2. Frontend — composable & konstanta

### `composables/api/useApproval.ts` (rewrite total)

Pola `useApiClient().request<T>()` (sama dengan `useAssetRequests`/`useAudit`). DTO
Inggris snake_case:

```ts
interface ApprovalRequestRow {
  id: string; type: RequestType; status: RequestStatus; amount: string | null
  current_step: number; office_id: string | null; office_name: string | null
  target_id: string | null; reason: string | null
  requested_by_id: string; requested_by_name: string | null; requested_by_role: string | null
  decided_by_id: string | null; decision_note: string | null; created_at: string | null
}
interface ApprovalStep {
  step_order: number; required_level: string
  approver_id: string | null; approver_name: string | null
  decision: RequestStatus; note: string | null; decided_at: string | null
}
interface ApprovalRequestDetail extends ApprovalRequestRow {
  payload: Record<string, unknown> | null   // absent bila dimask FilterView
  steps: ApprovalStep[]
}
```

Fungsi: `inbox()`, `list({ status?, type?, limit?, offset? })` (bentuk
`{data,total,limit,offset}`), `get(id)`, `approve(id, note?)`, `reject(id, note?)`.

Catatan kontrak: `DecideRequest` backend mewajibkan field `decision`
(`oneof=approve reject`) bila body dikirim (hanya body kosong/EOF yang ditoleransi) —
jadi `approve(id, note)` mengirim `{ decision: 'approve', note }` dan `reject` mengirim
`{ decision: 'reject', note }`, meskipun endpoint-nya sudah spesifik per aksi.

### `constants/approvalMeta.ts` (baru)

Menggantikan meta yang selama ini diimpor page dari `mock/approval.ts`:

- `RequestType = 'asset_create' | 'asset_disposal' | 'asset_transfer' | 'valuation_exclusion'`
  — `assignment`/`maintenance` ada di enum backend tapi belum punya alur submit; TIDAK
  dimasukkan filter sampai modulnya jadi (catatan di PROGRESS.md; tipe mockup
  `peminjaman`/`maintenance` per definisi belum bisa tampil).
- `TYPE_META` per tipe riil: ikon + tone + sensitive
  (asset_create → package/info; asset_disposal → trash-2/error/sensitive;
  asset_transfer → arrow-right-left/primary; valuation_exclusion → coins/warning/sensitive).
- `STATUS_TONE` + status `cancelled` (neutral); `STATUS_FILTERS` =
  `['pending','approved','rejected','cancelled','all']`.

`mock/approval.ts` **tetap ada** — masih dipakai `useGlobalSearch` (+ testnya). Hanya
page/composable yang lepas darinya. Ekspor meta yang tak lagi dipakai page boleh tetap
tinggal di mock (dipakai test mock) — jangan sampai memutus konsumen lain.

### Mapper payload → tampilan (`utils/approvalPayload.ts`, baru)

`payloadToView(type, detail, lookups)` → `{ layout: 'summary' | 'diff', rows }`, unit-testable:

- **asset_create** → layout *summary* dari `AssetCreatePayload`: nama aset, kategori
  (resolve nama via lookup), kelas aset, biaya perolehan (rupiah), tanggal beli, serial,
  vendor/PO/funding bila ada.
- **asset_disposal** → layout *diff*: status aset (aktif → disposed), metode, tanggal,
  proceeds / book_value_at_disposal (rupiah), bast_no. (Shape: `DisposalPayload`.)
- **asset_transfer** → layout *summary*: kantor asal → kantor tujuan (resolve nama),
  ruangan tujuan, alasan. (Shape: `TransferPayload`.)
- **valuation_exclusion** → layout *diff* **statis** (tidak butuh payload — request tipe
  ini memang tidak membawa payload; executor hanya memakai `reason` + `target_id`):
  status valuasi (dihitung → dikecualikan).
- **asset_disposal**: baris "status aset (aktif → disposed)" juga statis (payload tidak
  membawa status aset saat ini).
- Untuk tipe yang butuh payload (`asset_create`, `asset_transfer`, field-field
  `asset_disposal`): payload null/malformed/dimask → `rows: []`; UI merender keterangan
  "data tidak tersedia" (i18n) di section Data.

Resolve nama FK di klien memakai lookup yang sudah ada (kategori via `useCategories`
tree, kantor via `GET /offices?limit=100` seperti pola layar Pegawai); gagal resolve →
tampilkan ID mentah (fallback aman).

## 3. Frontend — halaman `approval.vue`

- **Gate**: `definePageMeta({ middleware: 'can', permission: 'request.decide' })`
  (menggantikan placeholder `masterdata.office.manage`); item nav sidebar untuk layar ini
  disesuaikan ke permission yang sama.
- **Panel kiri**: tab Pending → `inbox()`; Approved/Rejected/Cancelled → `list({status})`;
  All → `list()`. Filter tipe = client-side param `type` ke `list()` (untuk inbox, filter
  klien). Kartu: ikon tipe, judul (dibangun dari tipe + nama aset/ringkasan payload TIDAK
  tersedia di list — pakai `"{label tipe} · {office_name}"` + pengaju + tanggal + badge
  status). Pagination list mengikuti `{limit,offset}` bila jumlah > 1 halaman (limit 20,
  tombol muat-lagi sederhana) — mockup tidak menampilkan pagination, pertahankan struktur.
- **Detail** (fetch `get(id)` saat kartu dipilih; skeleton saat loading):
  - Header: badge tipe (+ sensitive), badge status, judul = label tipe + identitas target
    (nama aset dari payload untuk asset_create; untuk tipe lain pakai target/office).
  - Kartu pengaju: inisial dari `requested_by_name`, nama, role, kantor, tanggal submit.
  - Section **Data**: dari `payloadToView` (summary/diff per tipe).
  - Section **Alasan**: `reason` (fallback "—").
  - Section **Lampiran**: selalu empty-state (`approval.noAttach`).
  - **Timeline**: entry pertama = diajukan (`requested_by_name`, `created_at`); lalu per
    step yang sudah decided (`approver_name`, `decided_at`, note, label level); bila masih
    pending → marker kuning "menunggu persetujuan step N (level X)".
  - **Footer aksi**: bila status pending **dan** id ∈ set inbox → input catatan + tombol
    Tolak/Setujui (loading state; sukses → refresh list aktif + detail, toast). Bila
    pending tapi ∉ inbox (dilihat dari tab All) → tombol disabled + keterangan "menunggu
    approver lain / di luar wewenang Anda". Bila decided → banner hasil (approved hijau /
    rejected merah / **cancelled netral**) memakai `decided_by`/step terakhir.
  - Error decide (403 SoD/eligibility, 409 state berubah) → toast error i18n + refresh.
- **i18n**: semua label baru di `i18n/locales/{id,en}.json` (tipe riil, status cancelled,
  keterangan non-eligible, data-tidak-tersedia, dst.). Kunci lama yang tak terpakai page
  dibiarkan bila masih dipakai test mock; yang benar-benar yatim dihapus.
- `data-testid` pada elemen kunci (tab, kartu, tombol decide, note input) untuk e2e.

## 4. Testing

- **Unit (node)**: `payloadToView` per tipe — payload lengkap, minimal, null, malformed,
  masked; formatter rupiah; meta konstanta (STATUS_FILTERS memuat cancelled, dsb.).
- **Component (`mountSuspended`, stub API)**: loading skeleton; inbox kosong (empty
  state); render kartu + badge; pindah tab memanggil endpoint benar; detail summary vs
  diff; timeline pending marker; tombol disabled saat non-eligible; decide sukses
  (approve/reject) me-refresh; error 403/409 → pesan; tab Cancelled.
- **E2E real backend** (`frontend/e2e/approval.spec.ts`, data unik per run —
  lihat memori e2e-persistent-data-uniqueness): setup API prasyarat (office dsb. +
  user checker SoD seperti assets.spec.ts) → maker submit `asset_create` →
  login checker → request tampil di tab Pending (inbox) → buka detail (section Data berisi
  nama aset & biaya) → approve dengan catatan → status jadi approved + timeline memuat nama
  checker & catatan → aset tercipta. Alur kedua: submit → reject (+ catatan) → banner merah.
  Alur ketiga: submit → cancel via API sebagai maker → muncul di tab Cancelled.
- **Perhatian regresi** (memori wiring-composable-breaks-consumer-tests): konsumen
  `useApproval` hanya `approval.vue`; `useGlobalSearch` tetap di `approvalStore` (tidak
  tersentuh). Rewrite `test/nuxt/approval.spec.ts` terhadap stub `useApproval` baru;
  `test/unit/approval-mock.spec.ts` tetap (store masih hidup). Jalankan suite penuh dan
  periksa exit code.

## 5. Definition of done

1. Semua gate hijau: backend `go build/vet/test`, `go test -tags=integration ./...`,
   Spectral; frontend `pnpm lint`, `typecheck`, `test`, `build`; e2e suite penuh
   (stack Docker + admin seeded).
2. Side-by-side vs `docs/design/Pengajuan Approval.dc.html` — light & dark — 1:1 kecuali
   deviasi yang disetujui: (a) tab Cancelled (ke-5), (b) lampiran empty-state permanen,
   (c) daftar tipe filter mengikuti tipe backend riil (tanpa peminjaman/maintenance),
   (d) judul kartu list dibangun dari tipe+kantor (payload tidak diekspos di list).
3. PROGRESS.md: butir kandidat (a) ditandai selesai + catatan deviasi (a)–(d) + sisa
   TODO #10 (FilterView `employees` dst.) diperbarui; `fieldCatalog.ts` frontend
   mendapat entity `requests`.
4. OpenAPI sinkron; tidak ada mock yang dihapus yang masih punya konsumen.
