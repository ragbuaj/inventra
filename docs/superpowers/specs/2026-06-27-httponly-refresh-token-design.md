# Spec — Hardening Refresh Token ke Cookie httpOnly (C1)

| | |
|---|---|
| **Tanggal** | 2026-06-27 |
| **Konteks** | Subsistem C dipecah jadi **C1 = hardening httpOnly (ini)** → **C2 = Google OAuth (ADR-0009)**. C1 prasyarat handoff token OAuth yang aman. |
| **Terkait** | Auth lokal (ADR JWT/Redis), CORS (`internal/middleware/cors.go`), logging (ADR-0002) |
| **Status** | Disetujui — siap menulis implementation plan |

## 1. Tujuan & ruang lingkup

Memindahkan **refresh token** dari cookie yang **bisa dibaca JavaScript** (rentan dicuri lewat XSS) ke
**cookie `HttpOnly`, `Secure`, `SameSite` yang di-set backend**. Access token tetap di memori (Pinia).
Ini menutup kelemahan nyata: refresh token saat ini ada di `useCookie('inventra_refresh', { httpOnly:false })`
dan dikirim ke `/auth/refresh` lewat body — JS (dan XSS) bisa membacanya.

**Dalam ruang lingkup:** backend set/baca/hapus cookie refresh httpOnly pada `/auth/login|refresh|logout`;
response & body berhenti membawa refresh token; frontend berhenti menyimpan refresh di JS dan memakai
`credentials: 'include'`; plugin rehydrasi disesuaikan; OpenAPI diperbarui; test backend + frontend + e2e.

**Di luar ruang lingkup:** Google OAuth (C2); memindahkan access token ke cookie (tetap di memori, by design);
konfigurasi cookie lintas-site `SameSite=None` (catatan deploy; bisa di-config kemudian).

## 2. Keputusan desain (disepakati)

1. Cookie refresh: **`HttpOnly` · `Secure` hanya saat `Env=production` · `SameSite=Lax` · `Path=/api/v1/auth` ·
   `Max-Age=RefreshTTL` · nama `inventra_refresh`**. `Secure` env-gated agar e2e/dev (HTTP localhost) tetap mengirim cookie.
2. `tokenResponse` berhenti memuat `refresh_token` (hanya `access_token`/`token_type`/`expires_in`).
3. `/auth/refresh` & `/auth/logout` membaca refresh dari **cookie**, bukan body.
4. Frontend memakai **`credentials: 'include'`** pada panggilan auth; menghapus `useRefreshCookie`.
5. Plugin rehydrasi: **selalu coba `refresh()`** saat cold-load bila belum authed (cookie httpOnly tak terbaca JS);
   401 → tetap logout.

## 3. Berkas

```
backend/internal/identity/cookie.go        ← NEW: nama + atribut cookie + setRefreshCookie/clearRefreshCookie
backend/internal/identity/handler.go        ← login set-cookie; refresh/logout baca cookie; tak balas refresh di JSON
backend/internal/identity/dto.go            ← tokenResponse buang RefreshToken; refreshRequest/logoutRequest dihapus
backend/internal/identity/routes.go         ← tak berubah struktur (rate-limit middleware tetap)
backend/internal/server/router.go           ← teruskan secureCookie(=Env==production) + refreshTTL(=cfg.JWTRefreshTTL) ke NewHandler
backend/api/openapi.yaml                     ← /auth/login|refresh|logout: Set-Cookie; no refresh in body/response
docs/api/bruno/Auth/Login.bru                 ← hentikan ekstraksi refreshToken dari body (cookie jar Bruno)
docs/api/bruno/Auth/Refresh.bru               ← body none + tak kirim/ekstrak refresh_token (cookie jar)
docs/api/bruno/Auth/Logout.bru                ← body none (tak kirim refresh_token)
docs/api/README.md                            ← perbarui teks: refresh token kini cookie httpOnly (cookie jar Bruno)
frontend/app/composables/useAuthApi.ts       ← login/logout: credentials include, tak set refresh cookie
frontend/app/composables/useApiClient.ts      ← refreshToken: credentials include, no body, no useRefreshCookie
frontend/app/plugins/auth.client.ts           ← selalu coba refresh saat cold-load (drop cek cookie)
frontend/app/composables/useRefreshCookie.ts  ← DIHAPUS
```

Tests:
```
backend/internal/identity/cookie_test.go atau handler test  ← Set-Cookie attrs, refresh-dari-cookie, logout-clear
frontend/test/nuxt/useAuthApi.spec.ts / useApiClient.spec.ts ← credentials include + no JS refresh cookie (update yang ada)
frontend/e2e/login.spec.ts                                   ← tetap hijau (cookie-based)
```

## 4. Backend — cookie helper

```go
// cookie.go
const refreshCookieName = "inventra_refresh"

// setRefreshCookie writes the refresh token as an HttpOnly cookie scoped to the
// auth endpoints. Secure is enabled only in production (dev/CI run over plain HTTP).
func setRefreshCookie(c *gin.Context, token string, ttl time.Duration, secure bool) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(refreshCookieName, token, int(ttl.Seconds()), "/api/v1/auth", "", secure, true)
}

func clearRefreshCookie(c *gin.Context, secure bool) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(refreshCookieName, "", -1, "/api/v1/auth", "", secure, true)
}
```

`NewHandler` mendapat dua nilai kecil: **`secureCookie bool`** (`= cfg.Env == "production"`) dan
**`refreshTTL time.Duration`** (`= cfg.JWTRefreshTTL`), keduanya dihitung di `router.go` dari `cfg` dan diteruskan
saat konstruksi (mengikuti pola handler yang sudah menerima `limiter`/`loginPerMin`). Identity tak perlu meng-impor
seluruh `config`.

## 5. Backend — handler

- **`login`**: setelah `pair, user, _ := svc.Login(...)`, panggil `setRefreshCookie(c, pair.RefreshToken, refreshTTL, secure)`,
  lalu `c.JSON(200, newTokenResponse(pair))` di mana `tokenResponse` **tidak** lagi memuat `refresh_token`.
- **`refresh`**: `rt, err := c.Cookie(refreshCookieName)`; bila kosong/err → `401 {"error":"missing refresh token"}`.
  `pair, err := svc.Refresh(ctx, rt)`; sukses → `setRefreshCookie(...)` (rotasi) + `c.JSON(200, newTokenResponse(pair))`.
  Hapus `ShouldBindJSON(&refreshRequest)`.
- **`logout`**: `rt, _ := c.Cookie(refreshCookieName)`; `svc.Logout(ctx, jti, exp, rt)` (rt boleh kosong);
  `clearRefreshCookie(c, secure)`; `c.JSON(200, {"status":"logged_out"})`. Hapus body parse.

`svc.Login/Refresh/Logout` **tidak berubah** — hanya sumber refresh token (cookie vs body) yang berpindah ke handler.

## 6. Frontend

- **`useAuthApi.login`**:
  ```ts
  const res = await $fetch<{ access_token: string }>(`${base}/auth/login`, {
    method: 'POST', body: { email, password }, credentials: 'include'
  })
  auth.setToken(res.access_token)
  await fetchMe()
  ```
  (Tak ada `refreshCookie.value = ...`.)
- **`useAuthApi.logout`**: `await client.request('/auth/logout', { method: 'POST', credentials: 'include' })` lalu
  `auth.clear()` (tanpa menyentuh cookie JS).
- **`useApiClient.refreshToken`**:
  ```ts
  const res = await $fetch<{ access_token: string }>(`${base}/auth/refresh`, {
    method: 'POST', credentials: 'include'
  })
  auth.setToken(res.access_token)
  return true
  ```
  Pada 401 di `request()`: `auth.clear()` + redirect `/login` (tak menyentuh cookie JS).
- **`plugins/auth.client.ts`**:
  ```ts
  const auth = useAuthStore()
  if (auth.isAuthenticated) return
  const authApi = useAuthApi()
  try { if (await authApi.refresh()) await authApi.fetchMe() } catch { /* stay logged out */ }
  ```
- **Hapus** `useRefreshCookie.ts`. Pastikan `credentials: 'include'` pada semua panggilan auth (`/auth/login`,
  `/auth/refresh`, `/auth/logout`) — endpoint lain memakai header `Authorization` dan tak butuh cookie.

## 6b. OpenAPI & koleksi Bruno (konsumen `refresh_token`)

Pencarian menemukan konsumen yang mengandalkan `refresh_token` di body/response — semua diperbarui di C1:

- **`backend/api/openapi.yaml`**: `TokenResponse` buang field/`required` `refresh_token` (sisakan `access_token`,
  `token_type`, `expires_in`). Operasi `POST /auth/refresh` & `POST /auth/logout` **buang `requestBody`** (refresh dari
  cookie httpOnly); hapus skema `RefreshRequest` & `LogoutRequest` yang menjadi tak terpakai (hindari warning Spectral);
  tambahkan deskripsi `Set-Cookie inventra_refresh` pada response login/refresh dan penghapusannya pada logout. Spectral hijau.
- **`docs/api/bruno/Auth/Login.bru`**: hapus `bru.setEnvVar("refreshToken", res.body.refresh_token)` (tetap simpan
  `accessToken`). **`Refresh.bru`**: `body: none`, hapus body `refresh_token` dan ekstraksi `refreshToken`. **`Logout.bru`**:
  `body: none`. Bruno punya **cookie jar** (aktif default) yang menangkap `Set-Cookie` dan mengirim ulang otomatis ke path
  cocok — `HttpOnly` tak menghalangi Bruno (itu batasan JS-browser), dan `Secure=false` di dev jalan via HTTP. Jadi alur
  Login→Refresh→Logout tetap berfungsi tanpa menangani refresh token manual.
- **`docs/api/README.md`**: perbarui kalimat yang menyebut menyimpan `refreshToken` sebagai env var → refresh token kini
  cookie httpOnly yang ditangani cookie jar Bruno.

## 7. CORS

Tidak ada perubahan kode CORS: `Access-Control-Allow-Credentials: true` sudah di-set dan origin di-echo eksak
(syarat untuk credentials). `SameSite=Lax` + same-site `localhost:3000↔:8080` → cookie terkirim pada XHR
`credentials:'include'`. Konfirmasi preflight `/auth/refresh` (POST, header `Content-Type`/`X-Request-ID` sudah
di allow-list) lolos.

## 8. Testing (proaktif & luas)

**Backend** (`internal/identity`, httptest; gunakan handler dengan service nyata atau stub seperlunya):
- `login` → response **tanpa** `refresh_token`; `Set-Cookie inventra_refresh` ada, ber-`HttpOnly`, `Path=/api/v1/auth`,
  `Max-Age>0`; `Secure` mengikuti env (false saat Env≠production).
- `refresh` tanpa cookie → 401; dengan cookie valid → access baru + `Set-Cookie` baru (rotasi).
- `logout` → `Set-Cookie` dengan Max-Age≤0 (cookie kedaluwarsa) + 200.
- (Helper murni) `setRefreshCookie`/`clearRefreshCookie` menghasilkan atribut yang benar; `Secure` true hanya saat secure=true.

**Frontend** (`test/nuxt`):
- `login` memanggil `/auth/login` dengan `credentials:'include'` dan hanya `setToken` (tak menyentuh cookie JS);
- `refreshToken` memanggil `/auth/refresh` dengan `credentials:'include'` tanpa body;
- `logout` memanggil `/auth/logout` dengan `credentials:'include'`;
- Update/spec yang sebelumnya mengandalkan `useRefreshCookie` (hapus referensi). Tidak ada referensi `useRefreshCookie` tersisa.

**E2E (kritis)**: `login.spec.ts` (login → dashboard) tetap **hijau**; jalankan/verifikasi di stack nyata sebelum klaim
selesai (Secure=false di dev agar cookie terkirim via HTTP). Ini titik risiko utama — wajib diverifikasi end-to-end.

**Gate**: `go build/vet/test ./...` + Spectral; `pnpm lint/typecheck/test/build`.

## 9. Risiko & catatan

- **`Secure` harus env-gated.** Di CI/dev (HTTP) cookie `Secure` tak akan terkirim → login patah. `Secure=(Env=="production")`.
  Ini pelajaran langsung dari bug CORS sebelumnya: ubah penanganan auth → **verifikasi e2e** sebelum merge.
- **`SameSite=Lax` & same-site.** Cukup untuk `localhost` dan deploy subdomain same-site. Deploy benar-benar lintas-site
  butuh `SameSite=None; Secure` — di luar ruang lingkup, dicatat sebagai opsi config masa depan.
- **Plugin rehydrasi** kini selalu mencoba `/auth/refresh` saat cold-load anonim (satu request → 401). Dapat diabaikan;
  optimasi cookie-hint non-sensitif ditunda (YAGNI).
- **Tidak ada migrasi DB / dependency baru.** Murni perubahan handler + frontend + dto + openapi.
- **OpenAPI**: hapus `refresh_token` dari schema response login/refresh & dari request refresh/logout; tambah dokumentasi
  cookie (`Set-Cookie`) — jaga Spectral hijau.
