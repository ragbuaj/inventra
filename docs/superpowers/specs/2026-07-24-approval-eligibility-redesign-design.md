# Redesain Aturan Eligibility Approval — Backend Design

Date: 2026-07-24
Status: Approved (keputusan dikonfirmasi dengan user; belum diimplementasikan)

## Goal

Menutup tiga celah pada aturan **eligibility** mesin maker-checker yang sekarang (`internal/approval/`),
tanpa membongkar mekanisme chain berbasis nilai (`approval_thresholds`) yang sudah bekerja dan
ter-snapshot. Redesain ini mendefinisikan ulang **siapa yang boleh menandatangani sebuah langkah**,
memisahkan sumbu **otoritas** dari sumbu **visibilitas** yang selama ini tercampur.

Ini dokumen desain (bukan rencana implementasi). Chain construction, executor registry, outbox/notifikasi,
dan jalur decide transaksional tetap seperti sekarang kecuali disebut berubah di sini.

## Masalah — tiga celah, satu akar

Aturan `eligibleToDecide` sekarang (`service.go`) memakai **data-scope** sebagai penentu wewenang:

```
bukan maker
DAN bukan approver langkah sebelumnya
DAN tierOK  (ada ancestor ber-tier == required_level, atau origin office untuk office/subtree)
DAN InScope(dataScope("requests"), tierOffice)
```

Baris terakhir itu akarnya. Data-scope bersifat **monoton** (akun global menutupi semua office),
sehingga menghasilkan tiga gejala:

1. **Konsumsi ke bawah.** Pejabat tinggi berscope luas memenuhi syarat langkah rendah — `required_level`
   hanya dibaca sebagai "scope menutupi office tier itu", dan pejabat pusat menutupi semua office. Ia
   bisa mengonsumsi langkah `office` yang sedang current.
2. **Persetujuan selevel.** Chain ditentukan nilai + tipe request, bukan oleh siapa maker-nya. Seorang
   kepala kanwil bisa jadi maker untuk request yang puncak rantainya hanya sampai `wilayah` (levelnya
   sendiri); SoD memblok dia, tapi rekan selevel yang mengetuk — bukan cek ke atas.
3. **Deadlock diam.** Bila approver satu-satunya di sebuah tier adalah maker-nya (mis. kepala kanwil
   mengajukan mutasi aset di wilayahnya), langkah itu tak bisa ditandatangani siapa pun dan request
   menggantung `pending` selamanya. Tidak ada timeout/eskalasi/delegasi.

**Diagnosis:** sistem memakai *visibilitas* (siapa boleh melihat) sebagai proksi *otoritas* (siapa boleh
menandatangani). Dua hal itu harus dipisah.

## Prinsip

Dua sumbu ortogonal:

- **Visibilitas** (`data_scope_policies`) — siapa boleh *melihat* request di daftar/inbox. **Tidak diubah.**
- **Otoritas** (baru) — siapa boleh *menandatangani* langkah tertentu. Inilah yang di-redesain.

Otoritas berdiri di atas tiga konsep:

- **Level approver** `L(u)` — diturunkan dari `masterdata.office_types.tier` kantor si user
  (`office` / `wilayah` / `pusat`). Tidak perlu kolom baru: dihitung via join office user. Karena level
  = tier kantor, semua orang di satu kantor berlevel sama.
- **Limit otorisasi** — batas wewenang rupiah per jabatan/user; menjadi **gerbang** yang dicek **hanya di
  langkah pengesah puncak**.
- **Locus** — kantor tier langkah itu; **dibekukan (snapshot) saat Submit**.

## Keputusan (dikonfirmasi user)

| # | Aspek | Keputusan |
|---|---|---|
| A | Model otoritas | Chain tetap dibentuk `approval_thresholds`; **limit otorisasi jadi gerbang** (Opsi B) |
| B | Kecocokan level | **Exact** (`L(u) == required_level`) + **substitusi** (bukan kolaps) |
| C | Konfigurasi substitusi | **Diatur admin di depan** (matriks), diterapkan engine otomatis |
| D | Locus substitusi | **Kantor yang sama** dengan pejabat yang absen (tier tak pernah bergeser) |
| E | Jabatan pengganti | **Fleksibel, boleh lebih dari satu jabatan**, termasuk pangkat lebih rendah di kantor itu |
| F | Kondisi substitusi | `on_empty` (hanya saat kursi kosong) dan `always` (ko-otoritas berdiri) |
| G | Limit pengganti pangkat-bawah | **Cap per-aturan** (`max_amount`; kosong = warisi wewenang penuh) |
| H | Cakupan gerbang limit | **Hanya langkah pengesah puncak**; langkah bawah = review |
| I | Delegasi | **Kantor yang sama saja** untuk v1 (person-to-person, mewarisi otoritas) |
| J | SLA | **Kirim notifikasi pengingat**; auto-eskalasi berbasis waktu ditunda (slot `sla_breached` disiapkan) |
| K | Superadmin/global | **Dikeluarkan dari jalur tanda tangan** (pemisahan tugas IT vs bisnis) |
| L | Perubahan struktur org | **Snapshot** tier office saat Submit; request berjalan tak tergeser re-parent |
| M | Meta-approval | Perubahan config **wajib diaudit**; maker-checker atas config = fase 2 |

---

## Arsitektur

### 1. Predikat eligibility baru — `signEligible(u, step, req)`

Menggantikan `eligibleToDecide`. Dipakai oleh jalur `Decide` dan filter `Inbox` (satu sumber kebenaran).

```
T      = step.tier_office_id            -- snapshot saat Submit (bagian 3)
R      = step.required_level
isTop  = step adalah langkah pengesah puncak chain

-- Himpunan penandatangan NORMAL
base = { u :
    has(u, "request.decide")
    DAN u.office_id == T                 -- locus; akun global tanpa office otomatis gugur (keputusan K)
    DAN L(u) == R                        -- exact; otomatis benar karena u berada di kantor T
    DAN u.id != req.requested_by_id      -- SoD 1
    DAN u.id bukan approver langkah sebelumnya  -- SoD 2
    DAN (NOT isTop OR limit(u) >= req.amount)    -- gerbang limit HANYA di puncak (keputusan H)
}

-- Ko-otoritas berdiri (condition = 'always')
always = { u : u.office_id == T
               DAN u.role IN rules(req.type, R, 'always')
               DAN SoD ok
               DAN (NOT isTop OR limit(u) >= req.amount) }

eligible = base UNION always

-- Bila kosong: picu substitusi 'on_empty', urut prioritas
if eligible kosong:
    for rule in rules(req.type, R, 'on_empty') ORDER BY priority:
        sub = { u : u.office_id == T
                    DAN u.role == rule.substitute_role_id
                    DAN SoD ok
                    DAN (NOT isTop OR min(limit(u), COALESCE(rule.max_amount, limit_pejabat_utama)) >= req.amount) }
        if sub tidak kosong:
            eligible = sub                 -- tandai substituted; catat rule (bagian 4)
            break
```

Sifat penting yang dijamin engine (tak bisa dilanggar konfigurasi):

- **Tak pernah turun tier.** Pengganti selalu di kantor `T` (tier `R`), jadi levelnya pasti `R`.
- **SoD tetap.** Bukan maker, bukan approver langkah sebelumnya.
- **Gerbang limit tetap** di langkah puncak, termasuk untuk pengganti (dibatasi `max_amount`).
- **Substitusi mengisi satu langkah**, bukan menyerap langkah lain (bukan kolaps).
- **Setiap substitusi tercatat.**

### 2. Aturan saat Submit

Ditambahkan ke `Service.Submit`:

1. **Perpanjangan chain relatif-maker** (menutup celah 2). Setelah `MatchThresholdSteps` membangun chain
   nilai, hitung level efektif maker `Lm`. Bila puncak chain `<= Lm`, perpanjang chain dengan langkah-langkah
   sampai satu level **strictly di atas** `Lm`. Jadi kepala kanwil yang mengajukan otomatis memperoleh
   langkah puncak minimal `pusat`.
2. **Cek solvability** (menutup celah 3). Untuk tiap langkah, harus ada minimal satu calon `signEligible`
   yang bukan maker (mempertimbangkan base, always, dan on_empty). Bila tak ada, **tolak di Submit** dengan
   error konfigurasi (`ErrUnsolvableChain` -> 422) alih-alih membiarkan request menggantung.
3. **Snapshot tier office** (keputusan L). Jalankan `resolveTierOffice` sekali per langkah saat Submit dan
   simpan hasilnya di `request_approvals.tier_office_id`. Jalur `Decide`/`Inbox` membaca snapshot ini, tidak
   menghitung ancestor secara live. Perubahan hierarki org setelah Submit tidak menggeser request yang
   sedang berjalan; sekaligus memurahkan decide-path.

### 3. Substitusi — matriks yang diatur admin

```sql
CREATE TABLE approval.substitution_policies (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  request_type       shared.request_type,               -- NULL = berlaku untuk semua tipe
  primary_level      shared.approver_level NOT NULL,     -- level langkah yang kursinya kosong
  substitute_role_id uuid NOT NULL REFERENCES identity.roles (id),  -- jabatan di kantor yang SAMA
  condition          text NOT NULL,                      -- 'on_empty' | 'always' | (fase 2: 'sla_breached')
  max_amount         numeric(18,2),                      -- NULL = warisi wewenang penuh pejabat utama
  priority           int NOT NULL DEFAULT 1,
  is_active          boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
-- locus tidak butuh kolom: selalu = kantor tier T langkah itu.
```

Beberapa jabatan pengganti = beberapa baris untuk `(request_type, primary_level, condition)`. Contoh:
Kepala Unit tak tersedia, `substitute_role_id = Admin Unit`, `condition = on_empty`, `max_amount = 10jt` —
Admin Unit di kantor yang sama boleh menggantikan sampai 10 juta.

### 4. Limit otorisasi

```sql
CREATE TABLE identity.authorization_limits (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  role_id      uuid REFERENCES identity.roles (id),      -- salah satu dari role_id / user_id diisi
  user_id      uuid REFERENCES identity.users (id),
  request_type shared.request_type,                      -- NULL = berlaku untuk semua tipe
  max_amount   numeric(18,2) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
```

`limit(u)` = limit paling spesifik yang berlaku (user mengalahkan role; tipe spesifik mengalahkan NULL).
Dipakai hanya di langkah pengesah puncak (keputusan H).

### 5. Perubahan `request_approvals`

Tambah kolom snapshot + jejak audit (WORM):

```sql
ALTER TABLE approval.request_approvals
  ADD COLUMN tier_office_id       uuid REFERENCES masterdata.offices (id),  -- snapshot locus
  ADD COLUMN acted_role_id        uuid REFERENCES identity.roles (id),      -- jabatan yang menandatangani
  ADD COLUMN substituted          boolean NOT NULL DEFAULT false,
  ADD COLUMN substitution_rule_id uuid REFERENCES approval.substitution_policies (id),
  ADD COLUMN reason               text;
```

### 6. Superadmin/global (keputusan K)

Akun tanpa office (superadmin/global) otomatis gugur `signEligible` karena `u.office_id == T` gagal.
Jadikan ini aturan eksplisit dan sengaja, bukan efek samping — didokumentasikan sebagai pemisahan tugas
IT vs bisnis. Break-glass, bila kelak dibutuhkan DRP bank, adalah mekanisme terpisah, beralarm, wajib
alasan, idealnya dual-control — bukan bagian jalur normal.

### 7. Notifikasi pengingat (keputusan J)

Manfaatkan infra notifikasi/outbox yang ada untuk mengirim **pengingat** pada approver yang eligible atas
request pending. Auto-eskalasi berbasis waktu (`condition = 'sla_breached'`) disiapkan di skema tapi mesin
penjadwalnya dibangun di fase berikutnya.

### 8. Audit config (keputusan M)

Setiap perubahan `authorization_limits` dan `substitution_policies` wajib tercatat (aktor, nilai
sebelum-sesudah, waktu) via modul audit yang ada. Maker-checker penuh atas kedua tabel ini (lewat request
type khusus seperti `authorization_limit_change` / `substitution_policy_change` yang dirutekan ke chain
tetap tinggi) = fase 2.

## Peta fase

- **v1 (menutup celah korektif):** predikat `signEligible` baru (keputusan A, B, D, E, F, G, H, K),
  perpanjangan chain relatif-maker + cek solvability + snapshot (bagian 2, keputusan L), tabel
  `substitution_policies` + `authorization_limits`, kolom baru `request_approvals`, notifikasi pengingat,
  audit config.
- **Fase 2 (slot disiapkan):** delegasi (keputusan I; kini kantor-sama), auto-eskalasi `sla_breached`
  (keputusan J), maker-checker atas perubahan config (keputusan M).

## Contoh berjalan

Cast: **Pak Budi** (Staf, maker), **Bu Sari** (Kepala Unit), **Pak Andi** (Admin Unit, kantor sama),
**Pak Dewa** (Kepala Kanwil), **Bu Rina** (Direktur Pusat).

- **Disposal 60jt di unit Pak Budi.** Chain nilai `[office, wilayah, pusat]`. Langkah office: Bu Sari
  (atau, bila kosong, Pak Andi sampai `max_amount`). Langkah wilayah: pejabat di kantor Kanwil. Langkah
  pusat (puncak): Bu Rina, dengan gerbang `limit(BuRina) >= 60jt`.
- **Pak Dewa (Kanwil) mengajukan mutasi.** Chain nilai berpuncak `wilayah` = levelnya sendiri -> aturan
  Submit memperpanjang ke `pusat`. SoD memblok Pak Dewa; puncaknya kini pusat (Bu Rina). Tidak ada
  persetujuan selevel.
- **Bu Sari cuti, tak ada delegasi.** `base(office)` kosong -> aturan `on_empty` menaruh Pak Andi (kantor
  sama) sebagai pengganti sampai `max_amount`. Bila `max_amount` di bawah nilai, aturan ini tak berlaku ->
  jatuh ke prioritas berikutnya atau di-surface saat Submit.

## Verification gates (saat implementasi nanti)

`sqlc generate` bersih; `go build ./...`; `go vet ./...`; `go test ./...` (+ job integrasi); Spectral lint
`backend/api/openapi.yaml`; update `docs/PROGRESS.md`. Uji wajib mencakup: konsumsi-ke-bawah kini ditolak
(`L != R`), locus lintas-kantor ditolak, chain diperpanjang saat maker == puncak, deadlock terdeteksi saat
Submit, substitusi `on_empty`/`always` dengan cap `max_amount`, gerbang limit hanya di puncak, snapshot
tak tergeser re-parent, superadmin ditolak menandatangani.

## Open items (di luar scope redesain ini)

- Angka limit & band threshold tetap placeholder pending kebijakan bank.
- Break-glass darurat (bila DRP memintanya) — mekanisme terpisah, belum dirancang.
- Bentuk request-type khusus untuk maker-checker config (fase 2).
