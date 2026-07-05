# Spec — Production Ops Hardening (WAF · IaC · Observability)

| | |
|---|---|
| **Tanggal** | 2026-07-06 |
| **ADR (akan dibuat)** | 0011 Observability · 0012 WAF · 0013 IaC — masing-masing di fasenya |
| **Bagian dari** | Pengerasan operasional pasca-deploy VPS (lanjutan `docs/DEPLOYMENT.md` + CD PR #52) |
| **Target server** | Biznetgio NEO Lite — 2 vCPU / 4 GB RAM, Ubuntu 24.04, Docker Compose |
| **Status** | Disetujui — siap menulis implementation plan |

## 1. Tujuan & ruang lingkup

Mengeraskan deployment produksi Inventra di satu VPS bank-grade dengan **tiga subsistem** di bawah
satu spec payung, dikerjakan **berfase**: WAF → Config management (IaC) → Monitoring & Observability.
Benang merah pengikat: **anggaran RAM 4 GB** — setiap penambahan harus dibatasi memori & retensinya.

**Dalam ruang lingkup:**
- **Fase 1 — WAF:** Coraza (ModSecurity-compatible) + OWASP Core Rule Set embedded di Caddy, rollout
  DetectionOnly → Blocking.
- **Fase 2 — IaC:** playbook Ansible idempoten yang mereproduksi seluruh setup server (kini manual di
  `DEPLOYMENT.md`), dengan rahasia terenkripsi via Ansible Vault.
- **Fase 3 — Observability:** stack SRE self-hosted (Prometheus + Grafana + Loki + exporters +
  Alertmanager), instrumentasi RED di backend, alert ke Telegram.

**Di luar ruang lingkup (fase lain / YAGNI):** distributed tracing (Tempo/OpenTelemetry), Cloudflare
edge WAF/CDN, HA/multi-node, pengiriman observability ke storage eksternal, secret manager berbasis
server (Vault/Doppler) — Ansible Vault sudah cukup untuk skala ini.

## 2. Keputusan desain (disepakati)

1. **Satu spec payung**, tiga fase berurutan: **WAF → IaC → Monitoring**.
2. **WAF = Coraza + OWASP CRS di Caddy** — self-hosted, tanpa akun/dependensi eksternal, reproducible.
3. **Config management = IaC dengan Ansible** (agentless, idempoten, jalan ulang dari laptop via SSH);
   rahasia via **Ansible Vault**.
4. **Monitoring = stack SRE self-hosted**, dituning untuk 4 GB (retensi pendek + `mem_limit`), **tanpa
   traces**.
5. **Instrumentasi = infra + aplikasi** — backend mengekspos `/metrics` (RED) + exporter Postgres & Redis.
6. **Alerting = Telegram** via Alertmanager.
7. **Akses:** hanya **Grafana** yang publik (subdomain, di belakang Caddy + login); Prometheus,
   Alertmanager, dan semua exporter **internal-only** (tanpa port host).

## 3. Arsitektur & anggaran RAM

```
Internet ─443/80─▶ Caddy (+Coraza WAF) ─┬─ /              ─▶ frontend (Nuxt)
                                        ├─ /api/*, /health ─▶ backend (Go, /metrics internal)
                                        └─ grafana.<domain> ─▶ Grafana (login)
                     jaringan internal   │
                     inventra-net        ├─ postgres ◀─ postgres_exporter
                                         ├─ redis    ◀─ redis_exporter
                                         ├─ minio
                                         ├─ node-exporter · cAdvisor · blackbox-exporter
                                         ├─ Prometheus ─▶ Alertmanager ─▶ Telegram
                                         └─ Loki ◀─ Promtail   (Grafana query metrics+logs)
```

Anggaran RAM (perkiraan, dari 4 GB + 2 GB swap):

| Kelompok | Perkiraan |
|---|---|
| App stack (PG, Redis, MinIO, Go, Nuxt, Caddy+Coraza) | ~0,8–1,1 GB |
| Monitoring (Prom, Grafana, Loki, Promtail, 3 exporter, cAdvisor, Alertmanager) | ~0,6–0,9 GB |
| OS + headroom | ~0,4–0,6 GB |
| **Total** | **~1,8–2,6 GB** |

Disiplin memori (wajib): `mem_limit`/`deploy.resources.limits` per kontainer monitoring; Prometheus
`--storage.tsdb.retention.time=15d` + `--storage.tsdb.retention.size`; retensi Loki pendek; cAdvisor
housekeeping dikecilkan.

## 4. Fase 1 — WAF (Coraza + OWASP CRS di Caddy)

**Pendekatan.** Caddy resmi tidak membawa Coraza → build **custom image** via `xcaddy` dengan plugin
`github.com/corazawaf/coraza-caddy/v2`, memuat **OWASP CRS**. WAF berjalan in-process di reverse proxy
yang sudah ada (tanpa hop/kontainer tambahan). Direktif `coraza_waf` di `Caddyfile` membungkus rute.

**Rollout bertahap (kunci menghindari false-positive):**
1. **DetectionOnly** — CRS memuat, mencatat, **tidak memblokir**. Amati trafik nyata: login, CRUD aset,
   **upload lampiran ke MinIO**, payload JSON besar, ekspor.
2. **Tuning** — susun pengecualian per-rule untuk temuan false-positive (mis. body JSON, multipart upload).
3. **Blocking** — aktifkan anomaly scoring CRS (paranoia level 1 sebagai awal), respons `403` untuk
   serangan.

**Berkas (indikatif):**
```
ops/caddy/Dockerfile             ← builder xcaddy + coraza-caddy → image Caddy kustom
ops/caddy/Caddyfile              ← + coraza_waf directive, subdomain grafana, header keamanan
ops/caddy/coraza/coraza.conf     ← config engine (SecRuleEngine DetectionOnly→On)
ops/caddy/coraza/crs/...         ← OWASP CRS (vendored/di-pin versi) + exclusions kustom
docker-compose.prod.yml          ← service caddy pakai image kustom (build: ops/caddy)
ops/waf-smoketest.sh             ← kirim SQLi/XSS/path-traversal → 403; alur sah → lolos
docs/adr/0012-waf.md             ← keputusan Coraza vs alternatif (Cloudflare, fail2ban)
```

**Verifikasi:** smoke-test payload serangan diblokir (403) setelah enforcement; regresi alur aplikasi
(login, buat/edit aset, upload lampiran) tetap lolos; audit log Coraza muncul di Loki.

## 5. Fase 2 — Config management (IaC dengan Ansible)

**Tujuan.** Mengubah seluruh langkah manual `DEPLOYMENT.md` (Docker, swap, ufw, user+SSH,
unattended-upgrades, deploy key, `.env.prod`, clone, compose up, WAF, monitoring) menjadi **playbook
idempoten** — server bisa dibangun ulang identik dari nol; run kedua = nol perubahan.

**Struktur (indikatif):**
```
ops/ansible/
  inventory.ini              ← host VPS (alamat, user deploy)
  site.yml                   ← orkestrasi role
  group_vars/all/vault.yml   ← rahasia terenkripsi (Ansible Vault): .env.prod, token Telegram, dll.
  roles/
    base/       ← paket dasar, swap, ufw, unattended-upgrades, user + authorized_keys, hardening SSH
    docker/     ← Docker Engine + compose plugin, grup docker
    app/        ← clone/pull repo, render .env.prod dari vault, docker compose up (stack prod)
    waf/        ← image Caddy kustom + config CRS (Fase 1)
    monitoring/ ← overlay compose monitoring + config (Fase 3)
docs/adr/0013-iac.md         ← Ansible vs cloud-init; keputusan & batasan
```

**Keputusan:** **Ansible** (bukan cloud-init) karena idempoten & bisa dijalankan berulang pada server
hidup, bukan hanya saat first-boot. `.env.prod` **tidak lagi** file teks polos di server melainkan
di-render dari **Ansible Vault** terenkripsi.

**Verifikasi:** `ansible-playbook --check` (dry-run) bersih; run kedua melaporkan `changed=0` (bukti
idempotence); uji end-to-end pada VM sekali-pakai bila memungkinkan.

## 6. Fase 3 — Monitoring & Observability (stack SRE)

Overlay **`docker-compose.monitoring.yml`** (bisa di-toggle terpisah dari stack app).

**Metrics — Prometheus** men-scrape:
- **node-exporter** (host: CPU, RAM, disk, load), **cAdvisor** (per-kontainer),
- **Caddy** (`admin :2019/metrics` bawaan), **backend `/metrics`** (RED per-route),
- **postgres_exporter**, **redis_exporter**,
- **blackbox-exporter** — probe HTTPS `/health` publik + **kedaluwarsa sertifikat TLS**.

**Logs — Loki + Promtail** mengumpulkan log kontainer Docker; backend sudah JSON (slog, ADR-0002) →
tereksplorasi terstruktur di Grafana. Audit log Coraza (Fase 1) juga masuk ke sini.

**Visualisasi — Grafana** dengan **datasource & dashboard as-code** (provisioning): Node Exporter Full,
cAdvisor, Caddy, Go-app RED, Postgres, Redis.

**Alerting — Alertmanager → Telegram.** Aturan awal:
- `InstanceDown` / target scrape hilang; container restart-loop.
- CPU/RAM host tinggi berkelanjutan; **disk > 85%**.
- **5xx rate** backend melonjak; **latency p99** di atas ambang.
- **Sertifikat TLS < 14 hari**; koneksi Postgres mendekati `max_connections`.
- **Umur backup DB > 25 jam** (mengikat ke rotasi backup harian).

**Instrumentasi backend (kecil, terisolasi):**
```
backend/internal/observability/metrics.go   ← registry + collector RED (client_golang)
backend/internal/middleware/metrics.go       ← middleware Gin: http_requests_total, _duration_seconds{route,method,status}
backend/internal/server/router.go            ← mount /metrics (promhttp) + middleware
backend/go.mod                                ← + prometheus/client_golang
docs/adr/0011-observability.md
```
`/metrics` **tidak** dirutekan publik oleh Caddy (hanya `/api/*` & `/health`) → hanya terjangkau
Prometheus lewat jaringan internal. `promhttp` diikat ke listener yang sama, tak terekspos internet.

**Berkas monitoring (indikatif):**
```
docker-compose.monitoring.yml
ops/monitoring/prometheus/prometheus.yml, rules/*.yml
ops/monitoring/alertmanager/alertmanager.yml        ← receiver Telegram (token via Vault)
ops/monitoring/loki/loki.yml, promtail/promtail.yml
ops/monitoring/grafana/provisioning/{datasources,dashboards}/*
```

**Keamanan akses:** hanya **Grafana** publik via `grafana.<domain>` (Caddy + login Grafana, admin
password dari Vault; anonymous off). Prometheus/Alertmanager/exporters **tanpa port host** — akses ops
via SSH tunnel bila perlu.

**Verifikasi:** semua target Prometheus `up`; dashboard termuat & berisi data; **uji satu alert
end-to-end** (mis. hentikan satu kontainer → notifikasi Telegram diterima).

## 7. Risiko & mitigasi

| Risiko | Mitigasi |
|---|---|
| **Tekanan RAM** stack monitoring di 4 GB | `mem_limit` per kontainer, retensi pendek, swap 2 GB; opsi mundur: kecilkan/kurangi Loki atau turunkan retensi |
| **False-positive CRS** memblokir alur sah (upload, JSON) | Rollout DetectionOnly → tuning exclusions → Blocking; smoke-test regresi |
| **Grafana/exporter terekspos** tak sengaja | Hanya Grafana publik + login; sisanya internal-only, diverifikasi tak ada port host |
| **Rahasia bocor** (token Telegram, .env) | Ansible Vault terenkripsi; tak ada rahasia plaintext di repo/host |
| **Build image Caddy kustom** gagal/berat | Build di CI (runner 7 GB) & push GHCR, konsisten dengan CD; VPS hanya pull |

## 8. Deliverable & dampak repo

- `ops/` bertambah: `caddy/` (Fase 1), `ansible/` (Fase 2), `monitoring/` (Fase 3).
- `docker-compose.prod.yml` (Caddy → image kustom) + `docker-compose.monitoring.yml` (overlay baru).
- Perubahan kode backend terisolasi: paket `observability` + middleware metrics + mount `/metrics`.
- 3 ADR baru: 0011 Observability, 0012 WAF, 0013 IaC.
- Update `docs/DEPLOYMENT.md` (bagian WAF, IaC, monitoring) & `docs/PROGRESS.md`.
- Skrip verifikasi per fase (`ops/waf-smoketest.sh`, cek idempotence Ansible, uji alert).

## 9. Urutan implementasi

Tiga plan implementasi terpisah (satu per fase), dikerjakan berurutan:
1. **WAF** — image Caddy kustom + CRS, DetectionOnly dulu, lalu Blocking setelah tuning.
2. **IaC** — Ansible mengkodifikasi state saat ini (termasuk WAF), Vault untuk rahasia.
3. **Monitoring** — overlay stack + instrumentasi backend + alert Telegram; di-deploy via IaC.
