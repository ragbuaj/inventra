# Catatan Kondisi Sistem dan Poin Diskusi — Approval dan Jurnal/GL

Tanggal: 2026-07-24
Sifat: dokumen komunikasi untuk pihak user/klien (bukan spesifikasi teknis)

Dokumen ini merangkum **kondisi sistem saat ini** pada dua area — (1) mekanisme persetujuan
berjenjang (approval) dan (2) pencatatan jurnal akuntansi serta rencana integrasi ke buku besar (GL)
— beserta **poin-poin yang memerlukan keputusan atau klarifikasi** dari pihak bank/klien.

---

## 1. Ringkasan singkat

- **Approval**: mekanisme persetujuan berjenjang sudah berjalan, tetapi ditemukan **tiga celah aturan
  kewenangan** yang perlu dibenahi sebelum layak dipakai lingkungan bank. Perbaikannya **sudah dirancang**,
  belum dibangun.
- **Jurnal/GL**: jurnal akuntansi **baru tersedia untuk penyusutan** dalam bentuk laporan yang bisa
  diunduh. Jurnal untuk **penghapusan (disposal), perolehan aset, dan penurunan nilai belum ada**, dan
  **belum ada pengiriman otomatis ke GL** (masih manual). Arah yang disepakati: mulai dari **penyerahan
  berkas manual**, integrasi otomatis ditunda sampai pihak bank menentukan.
- Sejumlah keputusan bersifat **akuntansi dan kebijakan wewenang** yang **bukan ranah tim pengembang** —
  memerlukan input tim keuangan/manajemen bank. Daftar lengkap ada di bagian 4.

---

## 2. Kondisi Sistem — Mekanisme Approval

### Yang sudah berjalan
- Persetujuan **berjenjang berdasarkan nilai transaksi** (maker-checker). Makin besar nilainya, makin
  tinggi tingkat pejabat yang harus menyetujui (kantor, wilayah, pusat).
- **Pemisahan tugas (SoD)**: pengaju tidak bisa menyetujui pengajuannya sendiri; satu orang tidak bisa
  menandatangani dua langkah pada pengajuan yang sama.
- Konfigurasi tingkatan dan ambang nilai **dapat diatur** (tidak ditanam mati di kode).

### Celah yang ditemukan (perlu dibenahi)
1. **Kemacetan (deadlock)**: bila seorang pejabat adalah satu-satunya yang berwenang di tingkatnya lalu
   ia sendiri yang mengajukan, pengajuan bisa **tidak bisa ditandatangani siapa pun** dan menggantung.
2. **Persetujuan selevel**: rantai persetujuan **belum dijamin melampaui** tingkat si pengaju — sehingga
   bisa terjadi pengajuan pejabat disetujui oleh rekan setingkat, bukan atasan.
3. **Pejabat tinggi memenuhi langkah rendah**: karena aturan lama memakai "cakupan wilayah" sebagai
   penentu, pejabat berwewenang luas bisa mengisi langkah yang seharusnya untuk tingkat di bawahnya.

### Arah perbaikan (sudah dirancang, belum dibangun)
- **Memisahkan "wewenang menyetujui" dari "hak melihat data"** (dulu tercampur).
- **Limit otorisasi per jabatan** (batas nilai rupiah yang boleh disetujui seseorang) sebagai penentu.
- **Penggantian saat pejabat berhalangan (substitusi)** oleh jabatan lain **di kantor yang sama**, dengan
  batas nilai yang bisa diatur.
- **Aturan saat pengajuan dibuat**: rantai otomatis diperpanjang bila puncaknya belum melampaui pengaju;
  pengajuan yang mustahil disetujui ditolak di awal (bukan dibiarkan menggantung).
- Detail teknis lengkap: `docs/superpowers/specs/2026-07-24-approval-eligibility-redesign-design.md`.

---

## 3. Kondisi Sistem — Jurnal Akuntansi dan GL

Istilah singkat: **jurnal** adalah catatan akuntansi dua sisi (debit dan kredit) yang harus seimbang;
**GL (buku besar)** adalah tempat seluruh jurnal resmi bank dikumpulkan.

### Yang sudah berjalan
- **Penyusutan (depresiasi)**: sistem sudah menghasilkan **jurnal seimbang otomatis** dan bisa **diunduh**
  sebagai Excel/PDF per periode. Kode akun beban penyusutan diambil per kategori aset; akun akumulasi
  penyusutan diambil dari satu setelan aplikasi.

### Yang belum ada
- **Jurnal penghapusan (disposal)**: nilai untung/rugi, hasil jual, dan nilai buku **sudah dihitung**,
  tetapi **belum disusun menjadi jurnal**.
- **Jurnal perolehan aset** dan **jurnal penurunan nilai (impairment)**: belum ada.
- **Peta akun lengkap**: baru tersedia untuk penyusutan; transaksi lain belum punya pemetaan kode akun.
- **Pengiriman otomatis ke GL**: belum ada — saat ini semua bersifat unduhan manual.

### Arah yang disepakati (sementara)
- **Cara pengiriman: penyerahan berkas manual** — sistem membuat berkas jurnal, tim keuangan mengunduh
  dan mengurus GL. Ini pilihan paling sederhana dan paling rendah risiko untuk saat ini.
- **Prinsip agar bisa maju tanpa menunggu keputusan bank**: pisahkan **susunan jurnal** (standar akuntansi
  PSAK — bisa dibangun sekarang) dari **kode akun** (khas tiap bank — disediakan sebagai isian kosong yang
  diisi tim keuangan bank kemudian).
- **Ditunda**: pengiriman otomatis (API/batch), rekonsiliasi otomatis — menunggu ketentuan pihak bank.

---

## 4. Poin yang Perlu Didiskusikan dengan User/Klien

### A. Keputusan akuntansi / GL (ranah tim keuangan bank)
1. **Daftar kode akun (chart of accounts)** untuk: Aset Tetap, Akumulasi Penyusutan, Beban Penyusutan
   (apakah berbeda per kategori?), Kas, Untung Pelepasan, Rugi Pelepasan, Rugi Penurunan Nilai.
2. **Aturan periode akuntansi**: jurnal masuk ke bulan apa, dan kapan periode "ditutup" sehingga tidak
   boleh ada entri mundur.
3. **Mutasi antar-kantor**: apakah menghasilkan jurnal (reklasifikasi) atau tidak menghasilkan jurnal.
4. **Basis komersial vs fiskal**: konfirmasi bahwa yang masuk GL hanya basis komersial (PSAK); basis
   fiskal (PMK 72) mengikuti jalur perpajakan yang terpisah.
5. **Format berkas** yang diterima GL bank: Excel, CSV, atau format tertentu.
6. **Cara dan frekuensi penyerahan ke GL** saat ini (manual), dan rencana ke depan (otomatis/batch) bila ada.

### B. Keputusan kebijakan wewenang / approval (ranah bisnis/manajemen)
1. **Limit otorisasi per jabatan** — siapa boleh menyetujui sampai nilai berapa.
2. **Matriks penggantian (substitusi)** — jabatan mana yang boleh menggantikan saat pejabat berhalangan,
   dan batas nilai penggantian tersebut.
3. **Kebijakan delegasi** saat cuti/dinas — untuk tahap ini direncanakan dalam lingkup kantor yang sama.
4. **Konfirmasi**: akun Superadmin (pengelola sistem) **tidak ikut** dalam alur persetujuan bisnis.
5. **Struktur jabatan tiap kantor** — memastikan tidak ada kantor yang hanya berisi satu pejabat, yang
   berpotensi membuat persetujuan macet.

---

## 5. Rekomendasi Langkah Berikutnya

1. **Bawa poin bagian 4 ke user/klien** untuk memperoleh keputusan — terutama daftar kode akun dan
   kebijakan limit wewenang.
2. **Sementara menunggu**, kerjakan bagian yang **tidak bergantung** pada keputusan bank:
   - Perbaikan aturan kewenangan approval (sudah dirancang).
   - Susunan jurnal disposal/perolehan/impairment beserta slot kode akun yang bisa diisi (mengikuti pola
     ekspor jurnal penyusutan yang sudah ada).
3. **Setelah bank menentukan** cara integrasi GL, baru bangun mekanisme pengiriman dan rekonsiliasi.

---

## 6. Referensi

- Desain teknis perbaikan approval: `docs/superpowers/specs/2026-07-24-approval-eligibility-redesign-design.md`
- Panduan mekanisme approver saat ini: `docs/PANDUAN_APPROVER.md`
- Catatan keputusan produk (vault): `Keputusan/Produk/Redesain Eligibility Approval.md`
