# Implementation Plan — Mobile fase M8: Profil lengkap & keamanan akun

Tanggal: 2026-07-21. Spec: [`2026-07-21-mobile-m8-profile-security.md`](../specs/2026-07-21-mobile-m8-profile-security.md).
Branch rencana `feat/mobile-m8`. Frontend/mobile-only, nol backend baru.

## Overview

Enam irisan memperluas layar Profil yang ada (M0) menjadi profil lengkap + kelola akun, plus layar
Keamanan Akun dan Lupa Password. Semua memakai endpoint auth yang sudah ada dan tidak di-deny untuk
`aud=mobile`. Alur password/email berbasis link email: mobile hanya memulai, penetapan di web.

## Architecture decisions

- Perluas fitur `profile`/`account` yang ada (M0 sudah punya profil ringkas + sesi perangkat).
- Password/email: mobile memanggil `change-request`/`forgot`; TIDAK ada layar set-password native.
- Avatar: `has_avatar` boolean + ambil blob ter-autentikasi; object URL/ImageProvider di-dispose
  saat diganti. Reuse image picker M7 bila ada (QM8-2).
- Efek cabut-sesi ganti password ditangani interceptor Dio yang ada (401 refresh gagal lalu logout
  bersih) — M8 hanya memverifikasi jalurnya, bukan membangun ulang.

## Prasyarat (Phase 0)

- [ ] Mockup di-generate: Keamanan Akun (5.19), Lupa Password (5.20), edit Profil (5.11), edit Login (5.1).

### Checkpoint: Phase 0
- [ ] Empat mockup/edit tersedia sebagai acuan 1:1.

## Task list

### Phase 1: Profil

#### Task M8-1: Profil lengkap (read)
**Description:** Perluas Profil: metadata akun + detail pegawai read-only dari `GET /auth/profile`+`/me`.
**Acceptance:**
- [ ] Kartu Data Diri (nama, telepon) + blok Detail Pegawai read-only (kode, status, departemen, jabatan).
- [ ] Kartu Informasi Akun (peran, kantor, metode login, tanggal bergabung); akun tanpa pegawai menampilkan catatan, bukan grid kosong.
- [ ] Tautan ke Keamanan Akun + Pengaturan; seksi Sesi Perangkat yang ada tetap.
**Verification:** widget test (akun dengan/ tanpa pegawai) + golden light/dark; 1:1 mockup 5.11.
**Dependencies:** Phase 0.
**Files:** `mobile/lib/features/profile/` (edit: model, screen) + test.
**Scope:** M.

#### Task M8-2: Ubah data diri
**Description:** Mode Ubah pada kartu Data Diri lalu `PUT /auth/profile` `{name, phone}`.
**Acceptance:**
- [ ] Toggle Ubah/Simpan/Batal; field jadi TextField; simpan memanggil `PUT /auth/profile` lalu tampilan read-only segar dari respons.
- [ ] Validasi: nama wajib.
**Verification:** widget test (mode ubah, simpan, batal, nama kosong) + golden; 1:1 mockup.
**Dependencies:** M8-1.
**Files:** `mobile/lib/features/profile/` (edit) + test.
**Scope:** S.

#### Task M8-3: Foto profil / avatar
**Description:** Unggah/hapus/tampil avatar (`GET/POST/DELETE /auth/avatar`, `has_avatar`).
**Acceptance:**
- [ ] Unggah (kamera/galeri, JPG/PNG, batas ukuran) lalu `POST`; tampil via blob ter-autentikasi; Hapus (`DELETE`) hanya muncul bila `has_avatar`.
- [ ] Gagal ambil foto degradasi ke inisial, tak memblokir layar; ImageProvider di-dispose saat diganti.
**Verification:** widget test (tombol Hapus hanya saat has_avatar; fallback inisial) + golden; integration (unggah lalu hapus lalu has_avatar berubah).
**Dependencies:** M8-1.
**Files:** `mobile/lib/features/profile/avatar/` + test.
**Scope:** M.

### Checkpoint: Phase 1
- [ ] Profil lengkap + edit + avatar jalan; analyze/test hijau; golden; 1:1 mockup. Review.

### Phase 2: Keamanan akun

#### Task M8-4: Keamanan Akun (ganti password + ganti email)
**Description:** Layar Keamanan: ganti password (`POST /auth/password/change-request`) + ganti email (`POST /auth/email/change-request`), keduanya lalu state "cek email".
**Acceptance:**
- [ ] Sheet Ganti Password: hanya field password lama lalu kirim lalu state konfirmasi; peringatan "semua sesi keluar".
- [ ] Sheet Ganti Email: field email baru lalu kirim lalu state konfirmasi.
- [ ] Password lama salah lalu 400 inline TANPA memicu auto-logout (bedakan dari 401).
**Verification:** unit (state machine; 400 bukan logout) + widget (dua sheet, konfirmasi) + golden; 1:1 mockup 5.19.
**Dependencies:** M8-1 (tautan dari Profil).
**Files:** `mobile/lib/features/account_security/` + test.
**Scope:** M.

#### Task M8-5: Lupa Password (dari Login)
**Description:** Tautan di Login lalu layar Lupa Password (`POST /auth/password/forgot`, anti-enumerasi).
**Acceptance:**
- [ ] Tautan "Lupa password?" di Login membuka layar; input email lalu kirim lalu pesan anti-enumerasi IDENTIK apa pun inputnya.
**Verification:** widget test (pesan sama untuk email ada/tidak; tautan Login) + golden; integration (`forgot` selalu 200).
**Dependencies:** Phase 0.
**Files:** `mobile/lib/features/auth/forgot_password/` + edit Login + test.
**Scope:** S.

#### Task M8-6: Verifikasi logout bersih pasca-ganti-password
**Description:** Pastikan setelah password benar-benar diganti (via web), sesi mobile lama logout bersih.
**Acceptance:**
- [ ] Pada 401 refresh gagal (token pra-`password_changed_at`), token lokal dibersihkan lalu arahkan ke Login (tanpa loop refresh).
**Verification:** unit interceptor (401 refresh gagal lalu clear + redirect) + integration bila memungkinkan.
**Dependencies:** M8-4.
**Files:** `mobile/lib/core/network/` (verifikasi/uji; kemungkinan tanpa perubahan) + test.
**Scope:** S.

### Checkpoint: Phase 2 (Complete)
- [ ] Semua acceptance terpenuhi; analyze/test/build APK hijau; golden light+dark; integration hijau.
- [ ] Semua layar 1:1 mockup (side-by-side). PROGRESS.md dicentang + PR number. Review akhir.

## Risks and mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Pengguna bingung link membuka web, bukan app | Med | Copy eksplisit "buka link di email" (QM8-3); tanpa deep-link v1 |
| Avatar butuh dependensi image picker | Low | Reuse dari M7 bila ada (QM8-2); bila tidak, tanya |
| 400 password lama salah salah-tangani jadi logout | Med | Unit test memisahkan 400 (inline) dari 401 (logout) |
| Ganti password mencabut sesi mobile tak tertangani | Med | Task M8-6 verifikasi jalur interceptor yang ada |

## Open questions

Tidak ada yang memblokir (QM8-1 resolved: `PUT /auth/profile` = name+phone). QM8-2 (reuse image
picker) & QM8-3 (copy link) diputuskan saat implementasi/mockup, tidak menghambat.
