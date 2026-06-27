# Spec — Structured Logging & Request Correlation (ADR-0002)

| | |
|---|---|
| **Tanggal** | 2026-06-27 |
| **ADR** | [0002](../../adr/0002-structured-logging.md) (Accepted) · config per [0003](../../adr/0003-configuration.md) · tests per [0001](../../adr/0001-go-testing-stack.md) |
| **Bagian dari** | Trio cross-cutting **A→B→C**: **A = logging (ini)** → B = rate limiting (ADR-0004) → C = Google OAuth (ADR-0009) |
| **Status** | Disetujui — siap menulis implementation plan |

## 1. Tujuan & ruang lingkup

Mengganti logging default (Gin `gin.Logger()` + stdlib `log`) dengan **structured logging `log/slog`** dan
**korelasi request end-to-end** lewat `X-Request-ID` FE↔BE, dengan **redaksi field sensitif** terpusat.
Fondasi observability yang dipakai subsistem B & C berikutnya.

**Dalam ruang lingkup:**
- Backend: paket `internal/logging` (konstruksi logger + helper context + redaksi), middleware `RequestID`
  + `RequestLogger` + `Recovery` terstruktur, wiring di `main.go`/`router.go`, ganti stdlib `log` startup.
- Frontend: propagasi header `X-Request-ID` per panggilan API di `useApiClient`.
- Config: `LOG_LEVEL`, `LOG_FORMAT`.
- Test backend + frontend.

**Di luar ruang lingkup (YAGNI / fase lain):** pengiriman client-error ke endpoint backend (`useLogger`
penuh), OpenTelemetry, log sink/agregator eksternal, sampling lanjutan. Handler `slog` menyisakan jalur
ke OTel nanti tanpa ubah call site.

## 2. Keputusan desain (disepakati)

1. **`log/slog`** (stdlib, ADR-0002); handler **`text` (dev) / `json` (prod)** dipilih dari `Env`,
   override `LOG_FORMAT`; level via `LOG_LEVEL` (default `info`). `slog.SetDefault(...)` di startup.
2. **Korelasi via `X-Request-ID`**; FE generate per panggilan, BE baca/echo + bind `request_id` ke semua log.
3. **Redaksi terpusat via `slog.HandlerOptions.ReplaceAttr`** (lebih kuat dari helper manual).
4. **Custom `Recovery` terstruktur** menggantikan `gin.Recovery()` (log panic + stack + `request_id`, 500 JSON konsisten).
5. Frontend: **propagasi `X-Request-ID` saja** (client-error shipping ditunda).
6. Nama paket: **`internal/logging`**.

## 3. Berkas

```
backend/internal/logging/logging.go        ← New, SetDefault, WithLogger/FromContext, redaction ReplaceAttr
backend/internal/middleware/requestlog.go   ← RequestID() + RequestLogger(base) gin middleware
backend/internal/middleware/recovery.go      ← Recovery(base) structured panic recovery
backend/internal/config/config.go            ← + LogLevel, LogFormat (getEnv)
backend/cmd/api/main.go                      ← build logger + SetDefault; stdlib log → slog (startup/lifecycle)
backend/internal/server/router.go            ← r.Use(RequestID, RequestLogger(log), Recovery(log), CORS) — drop gin.Logger()+gin.Recovery()
backend/.env.example                         ← + LOG_LEVEL, LOG_FORMAT
frontend/app/composables/useApiClient.ts     ← set X-Request-ID header in request()
```

Tests:
```
backend/internal/logging/logging_test.go
backend/internal/middleware/requestlog_test.go
backend/internal/middleware/recovery_test.go
frontend/test/nuxt/useApiClient.spec.ts   (// @vitest-environment nuxt — useApiClient pakai composable Nuxt)
```

## 4. Backend — paket `internal/logging`

```go
package logging

// New builds the app logger from config: json (prod) or text (dev), at LOG_LEVEL,
// with sensitive keys redacted at the handler level.
func New(cfg *config.Config) *slog.Logger

// Context helpers — request-scoped logger carrying request_id (and later user_id/role_id).
func WithLogger(ctx context.Context, l *slog.Logger) context.Context
func FromContext(ctx context.Context) *slog.Logger   // falls back to slog.Default()
```

- **Format**: `LOG_FORMAT` ∈ {`json`,`text`}; default `text` bila `cfg.Env == "development"`, selain itu `json`.
- **Level**: `LOG_LEVEL` ∈ {`debug`,`info`,`warn`,`error`}; default `info`. Parse case-insensitive; fallback `info`.
- **Redaksi (`ReplaceAttr`)**: bila `strings.ToLower(attr.Key)` ∈ set sensitif
  {`password`,`password_hash`,`token`,`access_token`,`refresh_token`,`secret`,`authorization`,`google_id`,`api_key`}
  → ganti nilai jadi string `"[REDACTED]"` (apa pun tipenya). Berlaku rekursif ke grup attr.
- `New` memanggil `slog.SetDefault(logger)` **tidak** dilakukan di sini (efek samping) — dilakukan eksplisit di `main.go`.

## 5. Backend — middleware

`internal/middleware/requestlog.go`:
- **`RequestID() gin.HandlerFunc`** — `id := c.GetHeader("X-Request-ID")`; bila kosong → `uuid.NewString()`.
  Simpan ke gin context (`c.Set(CtxRequestID, id)`) **dan** ke `c.Request` context; set response header
  `X-Request-ID: <id>`. (Gunakan generator UUID yang sudah dipakai repo; bila belum ada dep, `crypto/rand` hex.)
- **`RequestLogger(base *slog.Logger) gin.HandlerFunc`** — ambil `request_id`, buat
  `reqLog := base.With("request_id", id)`, simpan ke `c.Request` context via `logging.WithLogger`, lalu
  `c.Next()`. Setelah handler: hitung `latency_ms`, dan emit **satu** baris pada level sesuai status
  (≥500 → error, ≥400 → warn, else info): attrs `method, path, status, latency_ms, request_id`, plus
  `user_id`/`role_id` bila `c.Get(middleware.CtxUserID/CtxRoleID)` ada. **Skip** `/health` & `/health/ready`.

`internal/middleware/recovery.go`:
- **`Recovery(base *slog.Logger) gin.HandlerFunc`** — `defer recover()`; bila panic:
  `logging.FromContext(c.Request.Context()).Error("panic recovered", "error", fmt.Sprint(r), "stack", string(debug.Stack()), "path", c.FullPath())`,
  lalu `c.AbortWithStatusJSON(500, gin.H{"error": "internal server error"})`. Tidak membocorkan stack ke klien.

Urutan di `router.go`: `r.Use(RequestID(), RequestLogger(logger), Recovery(logger), CORS(...))` —
RequestID dulu (agar id tersedia untuk logger & recovery), Recovery membungkus handler.

## 6. Backend — wiring & startup

- `cmd/api/main.go`: bangun `logger := logging.New(cfg)`, `slog.SetDefault(logger)` (sehingga
  `slog.Warn` eksisting di `internal/audit/record.go` ikut terstruktur), lalu ganti seluruh
  `log.Printf/Println/Fatalf` lifecycle dengan `slog.Info/Error` (mis. "PostgreSQL connected",
  "Inventra API listening", "shutting down"). `log.Fatalf` → `slog.Error(...)` + `os.Exit(1)`.
- `internal/server/router.go`: `NewRouter` menerima/membangun logger dan memasang middleware di atas;
  hapus `gin.Logger()` & `gin.Recovery()`.

## 7. Config

`internal/config/config.go` tambah:
```go
LogLevel  string // getEnv("LOG_LEVEL", "info")
LogFormat string // getEnv("LOG_FORMAT", "")  — "" => derive from Env (text dev / json prod)
```
`.env.example`: `LOG_LEVEL=info` dan `LOG_FORMAT=` (komentar: kosong = otomatis per ENV; isi `json`/`text` untuk override).

## 8. Frontend

`app/composables/useApiClient.ts` — di `request()`, sebelum `$fetch`, tambahkan ke `headers`:
```ts
if (!headers['X-Request-ID']) headers['X-Request-ID'] = crypto.randomUUID()
```
Tidak menimpa id yang sudah diberikan caller. (Retry setelah refresh memakai `headers` yang sama → id konsisten
dalam satu logical request.) Tidak ada UI/i18n baru.

## 9. Testing (proaktif & luas)

**Backend** (`internal/logging`, capture via `slog` handler ke `bytes.Buffer`, parse JSON):
- `New`: level honor `LOG_LEVEL` (debug line muncul saat debug, tertekan saat info); format json vs text per `LOG_FORMAT`/`Env`.
- Redaksi: log dengan key `password`/`token`/`authorization`/`google_id` → nilai `[REDACTED]`; key non-sensitif tak berubah; redaksi di dalam grup attr.
- `FromContext`: mengembalikan logger yang di-`WithLogger`; fallback ke default bila tak ada.

**Backend middleware**:
- `RequestID`: generate bila header kosong; **pertahankan** id inbound; selalu echo di response header.
- `RequestLogger`: satu baris per request berisi `request_id`,`status`,`latency_ms`,`method`,`path`;
  level naik sesuai status (info/warn/error); `/health` di-skip (tidak ada baris); `user_id`/`role_id`
  ikut ter-log bila diset di context.
- `Recovery`: handler yang panic → response `500 {"error":"internal server error"}`, satu baris log
  `error` berisi `request_id` + ada `stack`; request normal tidak terpengaruh.

**Frontend** (`useApiClient`):
- `request()` menambah header `X-Request-ID` ber-UUID valid saat caller tak menyediakannya (stub `$fetch`, assert header terkirim).
- Tidak menimpa `X-Request-ID` yang sudah diberikan caller.

**Gate**: `go build ./... && go vet ./... && go test ./...` + Spectral; `pnpm lint && pnpm typecheck && pnpm test`.
OpenAPI: response header `X-Request-ID` bersifat lintas-endpoint; tidak menambah path baru sehingga
`openapi.yaml` tak wajib berubah (boleh dicatat sebagai header umum bila mudah).

## 10. Risiko & catatan

- **Jangan log body/credential**: kita hanya log metadata request; redaksi `ReplaceAttr` jaring pengaman bila struct ter-log.
- **Tidak menggagalkan request**: middleware bebas-panic; logging hanya ke stdout (tanpa mode gagal jaringan).
- **Audit writer**: `slog.Warn` di `audit/record.go` jadi terstruktur via `SetDefault`, namun belum membawa
  `request_id` (tidak memakai `FromContext`). Peningkatan opsional kecil: teruskan logger context ke audit — **ditunda** (di luar ruang lingkup), catat sebagai follow-up.
- **UUID dep**: bila repo belum punya helper UUID di backend, gunakan `crypto/rand` (hindari menambah dep hanya untuk ini); FE pakai `crypto.randomUUID()` (tersedia di runtime modern + jsdom).
