# Spec — Mobile fase M8: Profil lengkap & keamanan akun

Tanggal: 2026-07-21. Status: draf, menunggu review pemilik produk. Branch rencana `feat/mobile-m8`.

Dasar scope: keputusan produk 2026-07-21 (vault `Keputusan/Produk/Mobile v1 Profil Lengkap dan
Keamanan Akun.md`), PRD mobile v1.1 FR-M6 (diperluas) + FR-M1.5, roadmap fase M8. Mockup: prompt
`docs/mobile/DESIGN_BRIEF.md` bagian 5.19-5.20 + edit 5.11 (Profil) & 5.1 (Login).

## 1. Objective

Memperluas profil mobile dari "profil ringkas" menjadi **profil lengkap + kelola akun**: lihat &
sunting data diri, kelola foto profil, ganti password/email, dan lupa password — paritas kunci
dengan web, memakai jalur **berbasis link email** yang sama.

Pengguna: semua peran. Sukses = pengguna dapat mengelola identitas dan keamanan akunnya dari HP:
menyunting data diri + avatar, memulai ganti password/email (diselesaikan via link email di web),
dan memulihkan akses lewat lupa password dari layar Login.

Non-goal M8: menyetel password/email langsung di native (selalu lewat link email lalu halaman web);
deep-link native (v1 link membuka web); administrasi user/RBAC (tetap web); mode offline.

## 2. Tech stack & commands

Sama dengan M7 (Flutter `mobile/`, Riverpod, Dio, freezed, go_router). Menyentuh alur auth yang
ada (M0/ADR-0017). Kemungkinan butuh penanganan gambar avatar (unggah multipart + tampil blob) —
cek apakah dependensi image picker sudah ada dari M7 (Lapor Kerusakan lampiran foto); reuse.

```
Analyze : (cd mobile) flutter analyze
Test    : flutter test
Build   : flutter build apk --debug
```

## 3. Kontrak API yang dikonsumsi (semua sudah ada — nol backend baru; tidak di-deny untuk aud=mobile)

| Kemampuan | Endpoint | Auth | Catatan |
|---|---|---|---|
| Profil lengkap (FR-M6.1) | `GET /auth/profile`, `GET /auth/me` | authed | metadata akun + detail pegawai read-only |
| Ubah data diri (FR-M6.2) | `PUT /auth/profile` | authed | field yang boleh diedit (nama, telepon, dsb.) |
| Avatar (FR-M6.2) | `GET/POST/DELETE /auth/avatar` | authed | `has_avatar` boolean; gambar via blob ter-autentikasi; JPG/PNG, `AVATAR_MAX_BYTES` |
| Ganti password (FR-M6.3) | `POST /auth/password/change-request` | authed | verifikasi password lama lalu kirim link ke `/reset-password` |
| Ganti email (FR-M6.3) | `POST /auth/email/change-request` | authed | kirim link verifikasi ke email baru; konfirmasi via `/verify-email` (web) |
| Lupa password (FR-M1.5) | `POST /auth/password/forgot` | publik | anti-enumerasi, **selalu 200** |
| Sesi perangkat (FR-M6.4) | `GET /auth/sessions`, `DELETE /auth/sessions/:id`, `POST /auth/sessions/revoke-others` | authed | sudah dirender v1 (M0) |

Catatan keamanan (mengikuti keputusan web [[Keamanan Akun via Email]] + [[Ganti Password Berbasis
Link]]): mobile hanya **memulai** ganti password/email; penetapan/konfirmasi diselesaikan di halaman
web via link email. **Ganti password mencabut semua sesi** (token-epoch `password_changed_at`) —
termasuk sesi mobile.

## 4. Perilaku per layar

### 4.1 Profil (edit 5.11)
- `GET /auth/profile`/`/auth/me`: kartu Data Diri (field editable read-only saat default, jadi
  TextField saat mode Ubah lalu `PUT /auth/profile`) berisi blok Detail Pegawai read-only (kode,
  status, departemen, jabatan); kartu Informasi Akun read-only (peran, kantor, metode login, tanggal
  bergabung); akun tanpa tautan pegawai menampilkan catatan.
- Avatar: unggah/ganti (`POST /auth/avatar`, JPG/PNG, batas ukuran), hapus (`DELETE`, tombol hanya
  muncul bila `has_avatar`), tampil via blob ter-autentikasi (object URL di-dispose saat diganti).
- Tautan ke Keamanan Akun (5.19) dan Pengaturan; seksi Sesi Perangkat (list/cabut/logout-semua,
  dialog konfirmasi) tetap seperti M0.

### 4.2 Keamanan Akun (5.19)
- Baris Email (read-only) + "Ganti Email"; baris Password + "Ganti Password".
- **Ganti Password**: sheet dengan HANYA field password lama lalu `POST /auth/password/change-request`
  lalu state "Cek email Anda" (tidak ada set-password di native). Peringatan "Semua sesi keluar
  setelah password diganti".
- **Ganti Email**: sheet field email baru lalu `POST /auth/email/change-request` lalu state
  konfirmasi "Link verifikasi terkirim".
- Password lama salah lalu HTTP 400 (bukan 401, agar interceptor auth tidak auto-logout) —
  petakan ke pesan inline.

### 4.3 Lupa Password (5.20)
- Dari tautan "Lupa password?" di Login. Field email lalu `POST /auth/password/forgot` lalu state
  **anti-enumerasi**: "Jika email terdaftar, kami kirim link" (pesan SAMA untuk email ada/tidak).
- Penetapan password baru di halaman web `/reset-password` via link.

### 4.4 Login (edit 5.1)
- Tambah tautan "Lupa password?" lalu buka Lupa Password.

## 5. Efek lintas-sesi (penting)

- Setelah `change-request` password diselesaikan di web dan password berubah, refresh token mobile
  yang lama ditolak (`password_changed_at`). Klien native harus menangani: pada 401 refresh gagal
  lalu bersihkan token lokal lalu arahkan ke Login (perilaku interceptor yang ada; verifikasi memicu
  logout bersih, bukan loop refresh).
- Ganti email tidak mencabut sesi; hanya mengubah alamat setelah konfirmasi link.

## 6. Testing strategy

- **Unit**: DTO profile/avatar/sessions; state machine ganti password/email (idle lalu submitting
  lalu "cek email"); pemetaan error 400 password lama salah (bukan logout).
- **Widget**: Profil (mode baca vs Ubah, tombol Hapus avatar hanya saat has_avatar, akun tanpa
  pegawai), Keamanan Akun (sheet password hanya password lama, sheet email, state konfirmasi), Lupa
  Password (pesan anti-enumerasi identik untuk dua input), Login (tautan Lupa password).
- **Golden** light + dark: Profil (mode baca, mode Ubah, menu avatar), Keamanan Akun, Lupa Password,
  Login (dengan tautan).
- **Integration** (vs backend + seed): ubah data diri lalu profil ter-refresh; unggah lalu hapus
  avatar lalu `has_avatar` berubah; `POST /auth/password/forgot` lalu selalu 200 (email terdaftar &
  tidak); `change-request` lalu email tertangkap Mailpit (bila dijalankan). Data unik per run.

## 7. Boundaries

- **Selalu**: password/email lewat link email (mobile hanya memulai); anti-enumerasi lupa password;
  peringatan cabut-sesi pada ganti password; field sensitif tak diserialisasi (avatar via
  `has_avatar` + blob); i18n id/en; match mockup 1:1 light + dark; analyze/test/build hijau.
- **Tanya dulu**: menambah deep-link native untuk reset/verify (di luar scope v1); perubahan backend
  (spec ini nol backend); menambah dependensi.
- **Jangan**: menyetel/mengonfirmasi password/email di dalam native; menyimpan password plaintext;
  memicu auto-logout dari 400 (hanya 401 refresh-gagal yang logout).

## 8. Success criteria

- [ ] Profil menampilkan data lengkap + detail pegawai; mode Ubah menyimpan via `PUT /auth/profile`
      dan menyegarkan tampilan.
- [ ] Avatar unggah/hapus/tampil benar; tombol Hapus hanya saat ada foto.
- [ ] Ganti password: verifikasi password lama lalu kirim link lalu state "cek email"; password lama
      salah lalu 400 inline tanpa logout.
- [ ] Ganti email: kirim link verifikasi lalu state konfirmasi.
- [ ] Lupa password dari Login: pesan anti-enumerasi identik apa pun inputnya.
- [ ] Setelah password benar-benar diganti (via web), sesi mobile lama logout bersih ke Login.
- [ ] Semua layar 1:1 mockup (light + dark); analyze/test/build hijau; PROGRESS sinkron.

## 9. Open questions

- **QM8-1 (resolved)** — `PUT /auth/profile` = `{ name (wajib), phone (opsional) }`
  (`updateProfileRequest`, `internal/identity/dto.go`). `name` = display name user; `phone` = nomor
  pegawai tertaut. Hanya dua field ini yang editable dari mobile (sama seperti web).
- **QM8-2** — Reuse image picker/kompresi dari M7 (Lapor Kerusakan) untuk avatar? Tentukan saat
  implementasi agar tak dobel dependensi (implementation detail, tidak memblokir plan).
- **QM8-3** — Copy eksplisit "link membuka aplikasi web" agar pengguna tak bingung; putuskan saat
  review mockup (tidak memblokir plan).
