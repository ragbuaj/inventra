# WAF (Coraza + OWASP CRS) — Fase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Menambahkan Web Application Firewall (Coraza + OWASP Core Rule Set) ke reverse-proxy Caddy pada stack produksi, dengan rollout aman DetectionOnly → Blocking, tanpa memutus alur aplikasi.

**Architecture:** Caddy resmi tidak membawa Coraza, jadi kita build image Caddy kustom via `xcaddy --with github.com/corazawaf/coraza-caddy/v2`. Modul v2 punya field `load_owasp_crs` yang membundel CRS ke dalam binary (referensi `@coraza.conf-recommended`, `@crs-setup.conf.example`, `@owasp_crs/*.conf`) — tak perlu vendoring file rule. WAF berjalan in-process di reverse-proxy yang sudah ada; tuning via file exclusions yang di-bind-mount.

**Tech Stack:** Caddy 2 (custom build), Coraza v2 (coraza-caddy), OWASP CRS 4 (bundled), Docker Compose, `traefik/whoami` (upstream uji), bash + curl (smoke test).

## Global Constraints

- **Self-hosted, tanpa akun/dependensi eksternal** — WAF sepenuhnya di dalam stack (verbatim dari spec bagian 2.2).
- **Rollout bertahap wajib:** DetectionOnly → tuning exclusions → Blocking (spec bagian 4).
- **Jangan putuskan alur aplikasi:** login, CRUD aset, **upload lampiran ke MinIO**, payload JSON harus tetap lolos (spec bagian 4, bagian 7).
- **Directive Caddy:** `order coraza_waf first` wajib ada di global options (syntax modul).
- **Endpoint app tetap:** Caddy hanya merutekan `/api/*` & `/health` ke backend, sisanya ke frontend (jangan ubah routing).
- **Build custom Caddy ringan** (Go build Caddy, bukan Nuxt) — aman di 4 GB; `build:` lokal di compose (integrasi CI/GHCR = follow-up setelah CD PR #52 merge).

---

### Task 1: Image Caddy kustom (Coraza) + wiring compose

Membuktikan binary Caddy kustom (dengan modul Coraza) ter-build dan tetap menyajikan aplikasi — **belum** mengaktifkan WAF.

**Files:**
- Create: `ops/caddy/Dockerfile`
- Move: `ops/Caddyfile` → `ops/caddy/Caddyfile` (isi tetap, akan diedit di Task 3)
- Modify: `docker-compose.prod.yml` (service `caddy`: `image:` → `build:`, perbarui path mount Caddyfile)

**Interfaces:**
- Produces: image Caddy kustom berisi modul `coraza_waf`; service `caddy` di compose dibangun dari `./ops/caddy`.

- [ ] **Step 1: Tulis Dockerfile Caddy kustom**

Create `ops/caddy/Dockerfile`:

```dockerfile
# syntax=docker/dockerfile:1

# --- build stage: Caddy + modul Coraza WAF ---
FROM caddy:2-builder-alpine AS build
RUN xcaddy build --with github.com/corazawaf/coraza-caddy/v2

# --- runtime ---
FROM caddy:2-alpine
COPY --from=build /usr/bin/caddy /usr/bin/caddy
```

- [ ] **Step 2: Pindahkan Caddyfile ke ops/caddy/**

Run:
```bash
git mv ops/Caddyfile ops/caddy/Caddyfile
```
Expected: file berpindah, isi tidak berubah.

- [ ] **Step 3: Arahkan compose ke image kustom**

Modify `docker-compose.prod.yml` — ganti blok service `caddy` menjadi:

```yaml
  caddy:
    build:
      context: ./ops/caddy
    container_name: inventra-caddy
    restart: unless-stopped
    depends_on:
      - frontend
      - backend
    ports:
      - "80:80"
      - "443:443"
    environment:
      DOMAIN: ${DOMAIN}
      ACME_EMAIL: ${ACME_EMAIL}
    volumes:
      - ./ops/caddy/Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy-data:/data
      - caddy-config:/config
```

- [ ] **Step 4: Build image & verifikasi modul Coraza ada**

Run:
```bash
docker build -t inventra-caddy-waf ./ops/caddy
docker run --rm inventra-caddy-waf caddy list-modules | grep -i -E "coraza|waf"
```
Expected: build sukses; grep menampilkan baris modul WAF (mis. `http.handlers.waf`). Bila kosong → build gagal memuat modul, periksa Step 1.

- [ ] **Step 5: Verifikasi config produksi tetap valid**

Run:
```bash
docker run --rm -e DOMAIN=example.com -e ACME_EMAIL=a@b.com \
  -v "$(pwd)/ops/caddy/Caddyfile:/etc/caddy/Caddyfile:ro" \
  inventra-caddy-waf caddy validate --config /etc/caddy/Caddyfile
```
Expected: `Valid configuration`.

- [ ] **Step 6: Commit**

```bash
git add ops/caddy/Dockerfile ops/caddy/Caddyfile docker-compose.prod.yml
git commit -m "feat(waf): custom Caddy image with Coraza module"
```

---

### Task 2: Harness uji lokal + WAF blocking (smoke test)

Membangun harness terisolasi (HTTP, tanpa TLS) untuk membuktikan CRS **memblokir serangan** dan **meloloskan trafik sah** — deterministik, bisa dijalankan di mesin mana pun.

**Files:**
- Create: `ops/caddy/test/Caddyfile.test`
- Create: `ops/caddy/test/docker-compose.test.yml`
- Create: `ops/waf-smoketest.sh`

**Interfaces:**
- Consumes: image Caddy kustom (Task 1, `build: ../` context).
- Produces: `ops/waf-smoketest.sh <base-url>` — exit 0 bila semua assertion lolos.

- [ ] **Step 1: Tulis Caddyfile uji (blocking)**

Create `ops/caddy/test/Caddyfile.test`:

```
{
	auto_https off
	order coraza_waf first
}

:8080 {
	coraza_waf {
		load_owasp_crs
		directives `
			Include @coraza.conf-recommended
			Include @crs-setup.conf.example
			Include @owasp_crs/*.conf
			SecRuleEngine On
		`
	}
	reverse_proxy whoami:80
}
```

- [ ] **Step 2: Tulis compose harness uji**

Create `ops/caddy/test/docker-compose.test.yml`:

```yaml
# Harness uji WAF terisolasi (HTTP :8080, tanpa TLS).
#   docker compose -f ops/caddy/test/docker-compose.test.yml up -d --build
#   ops/waf-smoketest.sh http://localhost:8080
#   docker compose -f ops/caddy/test/docker-compose.test.yml down
services:
  caddy:
    build:
      context: ..
    # Host port 18080 (bukan 8080) agar tak bentrok dengan backend app yang
    # mungkin sedang jalan di 8080. Port internal container tetap 8080.
    ports:
      - "18080:8080"
    volumes:
      - ./Caddyfile.test:/etc/caddy/Caddyfile:ro
    depends_on:
      - whoami

  whoami:
    image: traefik/whoami:latest
```

- [ ] **Step 3: Tulis skrip smoke test**

Create `ops/waf-smoketest.sh`:

```bash
#!/usr/bin/env bash
# Smoke-test WAF: serangan diblokir (403), trafik sah lolos (2xx/3xx).
#   ops/waf-smoketest.sh [base-url]   (default http://localhost:8080)
set -uo pipefail

BASE="${1:-http://localhost:8080}"
fail=0

check() {
  local desc="$1" expect="$2" url="$3"
  local code
  code="$(curl -s -o /dev/null -w '%{http_code}' --path-as-is "$url")"
  if [ "$code" = "$expect" ]; then
    echo "PASS  [$code] $desc"
  else
    echo "FAIL  [$code != $expect] $desc"
    fail=1
  fi
}

echo "== Serangan (harus 403) =="
check "SQLi in query"        403 "$BASE/?id=1%27%20OR%20%271%27%3D%271%27--"
check "XSS in query"         403 "$BASE/?q=%3Cscript%3Ealert(1)%3C%2Fscript%3E"
check "Path traversal"       403 "$BASE/../../../../etc/passwd"

echo "== Trafik sah (harus 200) =="
check "homepage"             200 "$BASE/"
check "normal query"         200 "$BASE/?page=2&sort=name"

if [ "$fail" -ne 0 ]; then
  echo "SMOKE TEST GAGAL"; exit 1
fi
echo "SMOKE TEST LULUS"
```

Run:
```bash
chmod +x ops/waf-smoketest.sh
```

- [ ] **Step 4: Jalankan harness & smoke test (verifikasi blokir)**

Run:
```bash
docker compose -f ops/caddy/test/docker-compose.test.yml up -d --build
sleep 5
ops/waf-smoketest.sh http://localhost:18080
```
Expected: `SMOKE TEST LULUS` — tiga serangan `403`, dua request sah `200`.

- [ ] **Step 5: Bereskan harness**

Run:
```bash
docker compose -f ops/caddy/test/docker-compose.test.yml down
```
Expected: kontainer uji berhenti & terhapus.

- [ ] **Step 6: Commit**

```bash
git add ops/caddy/test/Caddyfile.test ops/caddy/test/docker-compose.test.yml ops/waf-smoketest.sh
git commit -m "test(waf): isolated local harness + attack smoke test"
```

---

### Task 3: Aktifkan WAF di produksi (DetectionOnly) + scaffolding exclusions

Mengaktifkan WAF pada Caddyfile produksi dalam mode **DetectionOnly** (mencatat, tidak memblokir) dan menyiapkan file exclusions untuk tuning berdasarkan trafik nyata.

**Files:**
- Modify: `ops/caddy/Caddyfile` (tambah global `order` + blok `coraza_waf` DetectionOnly)
- Create: `ops/caddy/coraza-exclusions.conf`
- Modify: `docker-compose.prod.yml` (mount exclusions ke service caddy)

**Interfaces:**
- Consumes: image Caddy kustom (Task 1).
- Produces: WAF aktif DetectionOnly di produksi; file `coraza-exclusions.conf` yang di-Include setelah rule CRS (tempat `SecRuleRemoveById`/`SecRuleUpdateTargetById`).

- [ ] **Step 1: Tulis file exclusions awal**

Create `ops/caddy/coraza-exclusions.conf`:

```
# Exclusions WAF kustom — di-Include SETELAH rule OWASP CRS dimuat.
# Tambahkan aturan di sini saat DetectionOnly memunculkan false-positive pada
# alur aplikasi yang sah (lihat prosedur tuning di Step 5).
#
# Contoh bentuk (JANGAN aktifkan tanpa bukti FP di log):
#   SecRuleRemoveById 920420          # nonaktifkan satu rule global
#   SecRuleUpdateTargetById 942100 "!ARGS:filter"   # kecualikan satu argumen
#   SecRuleRemoveByTag "attack-protocol"            # nonaktifkan per-tag
#
# Awalnya kosong (belum ada exclusion) — diisi berdasarkan observasi.
```

- [ ] **Step 2: Aktifkan coraza_waf di Caddyfile produksi (DetectionOnly)**

Modify `ops/caddy/Caddyfile` menjadi:

```
{
	email {$ACME_EMAIL}
	order coraza_waf first
}

{$DOMAIN} {
	encode gzip zstd

	coraza_waf {
		load_owasp_crs
		directives `
			Include @coraza.conf-recommended
			Include @crs-setup.conf.example
			Include @owasp_crs/*.conf
			Include /etc/caddy/coraza-exclusions.conf
			SecRuleEngine DetectionOnly
		`
	}

	@api path /api/* /health
	handle @api {
		reverse_proxy backend:8080
	}

	handle {
		reverse_proxy frontend:3000
	}
}
```

- [ ] **Step 3: Mount exclusions ke container caddy**

Modify `docker-compose.prod.yml` — pada `volumes:` service `caddy`, tambahkan mount exclusions (di atas `caddy-data`):

```yaml
    volumes:
      - ./ops/caddy/Caddyfile:/etc/caddy/Caddyfile:ro
      - ./ops/caddy/coraza-exclusions.conf:/etc/caddy/coraza-exclusions.conf:ro
      - caddy-data:/data
      - caddy-config:/config
```

- [ ] **Step 4: Verifikasi config produksi valid (dengan WAF DetectionOnly)**

Run:
```bash
docker build -t inventra-caddy-waf ./ops/caddy
docker run --rm -e DOMAIN=example.com -e ACME_EMAIL=a@b.com \
  -v "$(pwd)/ops/caddy/Caddyfile:/etc/caddy/Caddyfile:ro" \
  -v "$(pwd)/ops/caddy/coraza-exclusions.conf:/etc/caddy/coraza-exclusions.conf:ro" \
  inventra-caddy-waf caddy validate --config /etc/caddy/Caddyfile
```
Expected: `Valid configuration` (mengkonfirmasi `@` includes + exclusions ter-resolve).

- [ ] **Step 5: Dokumentasikan prosedur tuning (di ADR nanti dijadikan referensi)**

Tidak ada perubahan file di step ini — ini prosedur yang dijalankan operator SETELAH deploy DetectionOnly:
1. Deploy stack; jalankan alur aplikasi nyata: login, buat/edit aset, **upload lampiran**, ekspor, query filter.
2. Kumpulkan audit log Coraza dari log caddy:
   `docker compose -f docker-compose.prod.yml logs caddy | grep -i coraza`
3. Untuk tiap rule yang terpicu pada request SAH, tambahkan exclusion di `coraza-exclusions.conf` (mis. `SecRuleRemoveById <id>` atau `SecRuleUpdateTargetById <id> "!ARGS:<nama>"`).
4. Ulangi sampai alur sah bersih di DetectionOnly. Baru lanjut Task 4 (Blocking).

- [ ] **Step 6: Commit**

```bash
git add ops/caddy/Caddyfile ops/caddy/coraza-exclusions.conf docker-compose.prod.yml
git commit -m "feat(waf): enable Coraza+CRS on prod Caddy in DetectionOnly mode"
```

---

### Task 4: Beralih ke Blocking + ADR + dokumentasi

Setelah tuning DetectionOnly bersih, aktifkan enforcement (Blocking), catat keputusan di ADR, dan perbarui dokumentasi.

**Files:**
- Modify: `ops/caddy/Caddyfile` (`SecRuleEngine DetectionOnly` → `On`)
- Create: `docs/adr/0012-waf.md`
- Modify: `docs/DEPLOYMENT.md` (bagian WAF)
- Modify: `docs/PROGRESS.md` (tandai Fase 1 WAF selesai)

**Interfaces:**
- Consumes: WAF DetectionOnly ter-tuning (Task 3).
- Produces: WAF enforcing (Blocking) di produksi; ADR-0012; dokumentasi.

- [ ] **Step 1: Aktifkan enforcement (Blocking)**

Modify `ops/caddy/Caddyfile` — pada blok `coraza_waf`, ubah baris terakhir directive:

```
			SecRuleEngine On
```
(dari `SecRuleEngine DetectionOnly`).

- [ ] **Step 2: Verifikasi blocking end-to-end di harness lokal**

Run (harness Task 2 sudah `SecRuleEngine On`, jadi ini regresi):
```bash
docker compose -f ops/caddy/test/docker-compose.test.yml up -d --build
sleep 5
ops/waf-smoketest.sh http://localhost:18080
docker compose -f ops/caddy/test/docker-compose.test.yml down
```
Expected: `SMOKE TEST LULUS`.

- [ ] **Step 3: Tulis ADR-0012**

Create `docs/adr/0012-waf.md`:

```markdown
# 12. Web Application Firewall — Coraza + OWASP CRS di Caddy

Tanggal: 2026-07-06

## Status

Accepted

## Konteks

Aplikasi bank-grade terekspos internet pada satu VPS. Perlu lapisan filter
serangan aplikasi (SQLi, XSS, path traversal, dsb.) di depan backend, tanpa
menambah dependensi/akun eksternal dan tanpa membebani VPS 4 GB secara berlebih.

## Keputusan

Memakai **Coraza** (WAF Go, kompatibel ModSecurity) sebagai modul di **Caddy**
(reverse-proxy yang sudah ada), memuat **OWASP Core Rule Set** via field
`load_owasp_crs` (CRS ter-bundle di binary). Image Caddy dibangun kustom dengan
`xcaddy --with github.com/corazawaf/coraza-caddy/v2`. Rollout **DetectionOnly →
Blocking** setelah tuning exclusions terhadap trafik nyata.

## Alternatif yang ditolak

- **Cloudflare free (edge WAF):** butuh akun + memindah nameserver domain;
  menambah dependensi eksternal. Bisa ditambahkan sebagai lapisan edge nanti.
- **fail2ban / rate-limit saja:** bukan WAF; tak memahami payload L7.

## Konsekuensi

- (+) Self-hosted, reproducible, in-process (tanpa hop/kontainer tambahan).
- (+) Aturan CRS standar industri, dapat di-tune per-rule.
- (−) Image Caddy harus dibangun kustom; perlu tuning exclusions untuk hindari
  false-positive (mis. upload multipart, body JSON).
```

- [ ] **Step 4: Perbarui DEPLOYMENT.md**

Modify `docs/DEPLOYMENT.md` — tambahkan sub-bagian di bawah "## 12. Troubleshooting" (sebelum "## Referensi perintah cepat"):

```markdown
---

## WAF (Coraza + OWASP CRS)

Reverse-proxy Caddy menjalankan WAF Coraza dengan OWASP CRS (image Caddy kustom
di `ops/caddy/`). Mode diatur oleh `SecRuleEngine` di `ops/caddy/Caddyfile`:
`DetectionOnly` (mencatat) atau `On` (memblokir, default produksi).

**Tuning false-positive** — bila alur sah terblokir (mis. upload lampiran):
1. `docker compose -f docker-compose.prod.yml logs caddy | grep -i coraza` untuk
   menemukan rule id yang terpicu.
2. Tambahkan exclusion di `ops/caddy/coraza-exclusions.conf`
   (mis. `SecRuleRemoveById <id>`), lalu redeploy.

**Uji WAF lokal (tanpa menyentuh produksi):**
```bash
docker compose -f ops/caddy/test/docker-compose.test.yml up -d --build
ops/waf-smoketest.sh http://localhost:8080
docker compose -f ops/caddy/test/docker-compose.test.yml down
```
```

- [ ] **Step 5: Perbarui PROGRESS.md**

Modify `docs/PROGRESS.md` — tandai item WAF Fase 1 selesai (`[ ]` → `[x]`) dengan catatan singkat, dan refresh blok "Next session" bila ada. (Sesuaikan dengan struktur file saat itu; bila belum ada entri WAF, tambahkan satu baris di bagian yang relevan.)

- [ ] **Step 6: Commit**

```bash
git add ops/caddy/Caddyfile docs/adr/0012-waf.md docs/DEPLOYMENT.md docs/PROGRESS.md
git commit -m "feat(waf): enforce blocking + ADR-0012 + docs"
```

---

## Catatan deploy (di luar task, dijalankan operator)

Setelah semua task merged:
1. Deploy dengan WAF **DetectionOnly** dulu (Task 3 state) di produksi, jalankan tuning (Task 3 Step 5).
2. Baru merge/deploy Task 4 (Blocking) setelah log DetectionOnly bersih dari FP.
3. Redeploy: `git pull && docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --build`.
