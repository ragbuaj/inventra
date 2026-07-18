# ADR-0016 — Stock opname offline-first: snapshot lokal + batch sync idempoten

- Status: Accepted
- Date: 2026-07-18
- Deciders: pemilik proyek + sesi perencanaan mobile
- Terkait: [ADR-0015](0015-mobile-companion-flutter.md) (aplikasi mobile companion). Melengkapi
  modul stock opname (spec `docs/superpowers/specs/2026-07-07-stock-opname-module-design.md`).

## Konteks

Stock opname dilakukan di lokasi yang kerap tanpa sinyal (gudang, basement, lokasi ATM). Endpoint
online per-scan sudah ada (`POST /stock-opname/sessions/:id/scan`), tetapi klien mobile harus tetap
bisa memindai saat offline dan menyetorkan hasilnya begitu koneksi kembali. Satu sesi bisa
dikerjakan beberapa petugas dengan beberapa device sekaligus, jadi dua masalah harus dijawab:
**pengiriman ulang tanpa duplikasi** (retry di jaringan buruk) dan **konflik antar-device**
(dua petugas memindai aset yang sama dengan hasil berbeda).

## Keputusan

**Snapshot lokal di device + antrean scan lokal + endpoint batch idempoten; konflik diselesaikan
first-write-wins per aset per sesi dan dilaporkan eksplisit.**

1. **Snapshot ke device.** Saat petugas membuka sesi, daftar item sesi diunduh dan disimpan di
   SQLite lokal (drift). Scan selama offline masuk **antrean lokal**, masing-masing membawa
   `client_scan_id` (UUID dibangkitkan device) dan `scanned_at` (jam device, informatif saja).
2. **Batch sync idempoten.** Saat online, klien mengirim antrean ke
   `POST /stock-opname/sessions/:id/scans/batch`. Server menyimpan `client_scan_id` yang sudah
   diproses (unik per sesi); item yang sama dikirim ulang tidak menghasilkan efek kedua —
   retry aman di jaringan putus-nyambung.
3. **Konflik: first-write-wins per aset per sesi.** Urutan ditentukan **waktu tiba di server**,
   bukan `scanned_at` device (jam device tidak dipercaya). Hasil pertama yang tiba untuk sebuah
   aset menang; kiriman berikutnya untuk aset yang sama tidak menimpa, tetapi dikembalikan di
   respons sebagai konflik per-item (hasil yang menang disertakan) sehingga device dapat
   menampilkan dan petugas dapat mengoreksi manual bila hasil yang menang keliru.
4. **Aturan domain tetap di server.** Resolusi tag ke aset, penambahan aset dalam-scope yang di
   luar snapshot (`expected=false`), dan penegakan scope/permission memakai jalur service yang
   sama dengan endpoint scan online — tidak ada logika opname yang diduplikasi di klien.
5. **Data lokal bersifat sementara.** Snapshot dan antrean dihapus dari device setelah sesi
   selesai dan tersinkron penuh; SQLite bukan arsip.

## Alternatif yang ditolak

- **Online-only.** Paling sederhana, tetapi menegasikan alasan utama scope mobile dibuka —
  keputusan produk eksplisit memilih offline-first untuk opname.
- **Last-write-wins berdasarkan timestamp device.** Jam device tidak bisa dipercaya (salah zona,
  manual, drift); hasil akhir jadi tidak deterministik dan tak bisa diaudit dengan jujur.
- **CRDT / sync engine siap pakai (PowerSync, ElectricSQL).** Kebutuhan riilnya satu tabel
  append-only per sesi dengan resolusi sederhana; engine sync dua arah generik menambah
  infrastruktur dan model konsistensi yang jauh melampaui masalahnya.
- **Kirim per-scan dengan retry (tanpa batch).** Ratusan request kecil di jaringan buruk; batch
  memberi satu kontrak idempoten, lebih sedikit round-trip, dan satu tempat pelaporan konflik.

## Konsekuensi

- Backend: tabel/kolom penyimpanan `client_scan_id` per sesi (partial unique, konvensi soft-delete
  DATABASE.md), handler batch di modul `stockopname` mengikuti pola empat-file, rate limit, dan
  OpenAPI — dibangun pada fase M5 roadmap mobile dengan spec + plan sendiri.
- Integration test wajib menutup skenario: retry kiriman yang sama (idempoten), dua device
  konflik pada aset yang sama (first-write-wins + laporan konflik), scan offline atas aset di
  luar snapshot, dan sesi yang ditutup saat masih ada antrean belum tersinkron (ditolak dengan
  jelas — bukan kehilangan data diam-diam).
- UI mobile harus selalu menampilkan status antrean (berapa scan belum tersinkron) — kegagalan
  sync tidak boleh senyap.
- Endpoint scan online per-item tetap ada dan tetap dipakai web; batch adalah jalur tambahan,
  bukan pengganti.
