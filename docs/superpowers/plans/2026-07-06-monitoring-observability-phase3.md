# Monitoring & Observability — Fase 3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Menambahkan stack observability self-hosted (Prometheus + exporters + Alertmanager→Telegram + Loki/Promtail + Grafana) untuk stack produksi Inventra, plus instrumentasi RED di backend, dituning agar muat di VPS 4 GB.

**Architecture:** Overlay `docker-compose.monitoring.yml` (toggleable) yang join jaringan `inventra-net` milik stack app. Prometheus men-scrape node-exporter (host), cAdvisor (kontainer), backend `/metrics` (RED), postgres/redis exporter, dan blackbox-exporter (uptime + kedaluwarsa TLS). Alertmanager mengirim ke Telegram. Loki+Promtail mengumpulkan log kontainer. Grafana (datasource+dashboard as-code) diekspos via subdomain `monitoring.<domain>` di Caddy (blok situs terpisah, tanpa WAF, login Grafana). Semua service lain internal-only. Rahasia (token Telegram, admin Grafana) via pola `*.example` + gitignore.

**Tech Stack:** Prometheus, Alertmanager, Grafana, Loki, Promtail, node-exporter, cAdvisor, blackbox-exporter, postgres_exporter, redis_exporter; Go `prometheus/client_golang`; Docker Compose; `promtool`/`amtool` (validasi ter-container).

## Global Constraints

- **Batasan RAM 4 GB:** setiap service monitoring WAJIB punya `mem_limit`; Prometheus retensi `15d` + cap ukuran; Loki retensi pendek.
- **Keamanan akses:** HANYA Grafana yang publik (subdomain, di belakang Caddy + login Grafana). Prometheus/Alertmanager/exporters TANPA port host (internal-only di `inventra-net`).
- **`/metrics` backend TIDAK publik:** Caddy hanya merutekan `/api/*` & `/health`; Prometheus scrape backend via jaringan internal.
- **Repo publik → tanpa rahasia di git:** `alertmanager.yml` & env Grafana nyata di-gitignore; hanya `*.example` di-commit.
- **Kardinalitas metrics terkendali:** label route pakai pola rute Gin (`c.FullPath()`), bukan path mentah.
- **Selaras stack app:** join `networks: inventra-net` (external), scrape service by name (`backend:8080`, `postgres:5432`, `redis:6379`).
- No `Co-Authored-By` trailers.

---

### Task 1: Instrumentasi RED backend (`/metrics`) — TDD

**Files:**
- Create: `backend/internal/observability/metrics.go`
- Create: `backend/internal/middleware/metrics.go`
- Create: `backend/internal/middleware/metrics_test.go`
- Modify: `backend/internal/server/router.go`
- Modify: `backend/go.mod` / `backend/go.sum`

**Interfaces:**
- Produces: `observability.Collectors()` registry helper; `middleware.Metrics()` gin middleware; `GET /metrics` handler on the engine (outside `/api`).

- [ ] **Step 1: Tambah dependency**

Run (from `backend/`):
```bash
go get github.com/prometheus/client_golang@v1.20.5
```
Expected: `go.mod`/`go.sum` updated.

- [ ] **Step 2: Tulis metrics collectors**

Create `backend/internal/observability/metrics.go`:

```go
// Package observability holds Prometheus metrics for the API. The collectors
// auto-register on the default registry at package init via promauto, so they
// are registered exactly once no matter how many times NewRouter is called
// (avoids duplicate-registration panics in tests).
package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// RequestsTotal counts HTTP requests by method, route template, and status.
var RequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests processed, by method, route, and status code.",
	},
	[]string{"method", "route", "status"},
)

// RequestDuration observes request latency by method and route template.
var RequestDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency in seconds, by method and route.",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"method", "route"},
)
```

- [ ] **Step 3: Tulis failing test untuk middleware**

Create `backend/internal/middleware/metrics_test.go`:

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestMetricsMiddlewareRecordsRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(Metrics())
	r.GET("/api/v1/things/:id", func(c *gin.Context) { c.Status(http.StatusOK) })
	// promhttp.Handler() scrapes the default gatherer, where the promauto
	// collectors in the observability package are registered.
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// exercise the instrumented route
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/things/42", nil))

	// scrape /metrics and assert the counter recorded the ROUTE TEMPLATE, not /42
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := w.Body.String()
	if !strings.Contains(body, `http_requests_total{method="GET",route="/api/v1/things/:id",status="200"} 1`) {
		t.Fatalf("counter not recorded with route template; body:\n%s", body)
	}
	if !strings.Contains(body, "http_request_duration_seconds_bucket") {
		t.Fatalf("duration histogram missing; body:\n%s", body)
	}
}
```

- [ ] **Step 4: Run test — verify it FAILS**

Run: `go test ./internal/middleware/ -run TestMetricsMiddlewareRecordsRequest`
Expected: FAIL (compile error — `Metrics` undefined).

- [ ] **Step 5: Implement middleware**

Create `backend/internal/middleware/metrics.go`:

```go
package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ragbuaj/inventra/internal/observability"
)

// Metrics records RED metrics (rate, errors, duration) per request. The route
// label uses the matched route template (c.FullPath()) to bound cardinality;
// unmatched routes and the /metrics endpoint are skipped.
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" || route == "/metrics" {
			return
		}
		observability.RequestDuration.WithLabelValues(c.Request.Method, route).
			Observe(time.Since(start).Seconds())
		observability.RequestsTotal.WithLabelValues(
			c.Request.Method, route, strconv.Itoa(c.Writer.Status()),
		).Inc()
	}
}
```

- [ ] **Step 6: Run test — verify it PASSES**

Run: `go test ./internal/middleware/ -run TestMetricsMiddlewareRecordsRequest`
Expected: PASS.

- [ ] **Step 7: Wire into router**

In `backend/internal/server/router.go`, inside `NewRouter` (near where base middleware + `/health` are registered): attach the middleware early and mount `/metrics`. Add imports `github.com/prometheus/client_golang/prometheus/promhttp` and (if not already present) `github.com/gin-gonic/gin`. No explicit registration call is needed — the `observability` collectors auto-register at package init (pulled in transitively via `middleware.Metrics`).

```go
	r.Use(middleware.Metrics())
	// Not routed publicly by Caddy (only /api/* and /health are) — internal scrape only.
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
```
(Place `r.Use(middleware.Metrics())` before the route groups so all routes are covered.)

- [ ] **Step 8: Verify build/vet/test green**

Run:
```bash
go build ./... && go vet ./... && go test ./internal/... 2>&1 | tail -20
```
Expected: build OK, vet OK, tests pass (incl. the new one). Guard: no duplicate-registration panic (metrics registered once).

- [ ] **Step 9: Commit**

```bash
git add backend/internal/observability backend/internal/middleware/metrics.go backend/internal/middleware/metrics_test.go backend/internal/server/router.go backend/go.mod backend/go.sum
git commit -m "feat(observability): RED metrics + /metrics endpoint on backend"
```

---

### Task 2: Overlay compose + Prometheus + node/cAdvisor + backend scrape

**Files:**
- Create: `docker-compose.monitoring.yml`
- Create: `ops/monitoring/prometheus/prometheus.yml`
- Create: `ops/monitoring/verify.sh`

**Interfaces:**
- Consumes: `inventra-net` (from prod stack), backend `/metrics` (Task 1).
- Produces: Prometheus at `prometheus:9090` scraping self, node-exporter, cAdvisor, backend. `verify.sh` validates configs via containerized `promtool`.

- [ ] **Step 1: Tulis prometheus.yml (core scrape)**

Create `ops/monitoring/prometheus/prometheus.yml`:

```yaml
global:
  scrape_interval: 30s
  evaluation_interval: 30s

rule_files:
  - /etc/prometheus/rules/*.yml

alerting:
  alertmanagers:
    - static_configs:
        - targets: ["alertmanager:9093"]

scrape_configs:
  - job_name: prometheus
    static_configs:
      - targets: ["localhost:9090"]

  - job_name: node
    static_configs:
      - targets: ["node-exporter:9100"]

  - job_name: cadvisor
    static_configs:
      - targets: ["cadvisor:8080"]

  - job_name: backend
    metrics_path: /metrics
    static_configs:
      - targets: ["backend:8080"]
```

- [ ] **Step 2: Tulis overlay compose (Prometheus + node + cAdvisor)**

Create `docker-compose.monitoring.yml`:

```yaml
# Overlay observability (toggleable). Jalankan bersama stack prod:
#   docker compose -f docker-compose.prod.yml -f docker-compose.monitoring.yml --env-file .env.prod up -d
# Join jaringan inventra-net milik stack app (harus sudah ada).
services:
  prometheus:
    image: prom/prometheus:v3.1.0
    container_name: inventra-prometheus
    restart: unless-stopped
    command:
      - "--config.file=/etc/prometheus/prometheus.yml"
      - "--storage.tsdb.retention.time=15d"
      - "--storage.tsdb.retention.size=1GB"
    volumes:
      - ./ops/monitoring/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - ./ops/monitoring/prometheus/rules:/etc/prometheus/rules:ro
      - prometheus-data:/prometheus
    mem_limit: 400m

  node-exporter:
    image: prom/node-exporter:v1.8.2
    container_name: inventra-node-exporter
    restart: unless-stopped
    command:
      - "--path.rootfs=/host"
    pid: host
    volumes:
      - /:/host:ro,rslave
    mem_limit: 64m

  cadvisor:
    image: gcr.io/cadvisor/cadvisor:v0.49.1
    container_name: inventra-cadvisor
    restart: unless-stopped
    command:
      - "--housekeeping_interval=30s"
      - "--docker_only=true"
    volumes:
      - /:/rootfs:ro
      - /var/run:/var/run:ro
      - /sys:/sys:ro
      - /var/lib/docker/:/var/lib/docker:ro
    devices:
      - /dev/kmsg
    mem_limit: 200m

volumes:
  prometheus-data:

networks:
  default:
    name: inventra-net
    external: true
```

- [ ] **Step 3: Tulis verify.sh (promtool ter-container)**

Create `ops/monitoring/verify.sh`:

```bash
#!/usr/bin/env bash
# Validasi konfigurasi monitoring tanpa menjalankan stack penuh.
set -euo pipefail
cd "$(dirname "$0")/../.."   # repo root

echo "== docker compose config =="
DOMAIN=x ACME_EMAIL=x DB_PASSWORD=x JWT_SECRET=x MINIO_ROOT_USER=x MINIO_ROOT_PASSWORD=x \
  docker compose -f docker-compose.prod.yml -f docker-compose.monitoring.yml config >/dev/null
echo "compose OK"

echo "== promtool check config =="
docker run --rm -v "$PWD/ops/monitoring/prometheus:/p" prom/prometheus:v3.1.0 \
  promtool check config /p/prometheus.yml

if compgen -G "ops/monitoring/prometheus/rules/*.yml" >/dev/null; then
  echo "== promtool check rules =="
  docker run --rm -v "$PWD/ops/monitoring/prometheus:/p" prom/prometheus:v3.1.0 \
    promtool check rules /p/rules/*.yml
fi

if [ -f ops/monitoring/alertmanager/alertmanager.yml ]; then
  echo "== amtool check-config =="
  docker run --rm -v "$PWD/ops/monitoring/alertmanager:/a" prom/alertmanager:v0.28.0 \
    amtool check-config /a/alertmanager.yml
fi
echo "ALL MONITORING CHECKS PASSED"
```
Run: `chmod +x ops/monitoring/verify.sh`

> `promtool check config` warns that `alertmanager:9093`/rule files aren't reachable at lint time — that's expected (offline); it still validates syntax.

- [ ] **Step 4: Verify**

Run: `ops/monitoring/verify.sh`
Expected: `compose OK`, `promtool check config` SUCCESS, `ALL MONITORING CHECKS PASSED`.

- [ ] **Step 5: Commit**

```bash
git add docker-compose.monitoring.yml ops/monitoring/prometheus/prometheus.yml ops/monitoring/verify.sh
git commit -m "feat(monitoring): compose overlay + Prometheus + node/cadvisor scrape"
```

---

### Task 3: Exporters DB/cache/uptime (postgres, redis, blackbox)

**Files:**
- Modify: `docker-compose.monitoring.yml`
- Modify: `ops/monitoring/prometheus/prometheus.yml`
- Create: `ops/monitoring/blackbox/blackbox.yml`

**Interfaces:**
- Consumes: `postgres`/`redis` services (prod stack), `${DOMAIN}` (env).
- Produces: postgres_exporter, redis_exporter, blackbox-exporter scraped by Prometheus.

- [ ] **Step 1: Tulis blackbox config**

Create `ops/monitoring/blackbox/blackbox.yml`:

```yaml
modules:
  http_2xx:
    prober: http
    timeout: 10s
    http:
      preferred_ip_protocol: ip4
      fail_if_not_ssl: true
```

- [ ] **Step 2: Tambah exporter ke overlay compose**

In `docker-compose.monitoring.yml`, add under `services:` (before `volumes:`):

```yaml
  postgres-exporter:
    image: quay.io/prometheuscommunity/postgres-exporter:v0.16.0
    container_name: inventra-postgres-exporter
    restart: unless-stopped
    environment:
      DATA_SOURCE_NAME: "postgresql://inventra:${DB_PASSWORD}@postgres:5432/inventra?sslmode=disable"
    mem_limit: 64m

  redis-exporter:
    image: oliver006/redis_exporter:v1.66.0
    container_name: inventra-redis-exporter
    restart: unless-stopped
    command: ["--redis.addr=redis://redis:6379"]
    mem_limit: 32m

  blackbox-exporter:
    image: prom/blackbox-exporter:v0.25.0
    container_name: inventra-blackbox-exporter
    restart: unless-stopped
    volumes:
      - ./ops/monitoring/blackbox/blackbox.yml:/etc/blackbox_exporter/config.yml:ro
    mem_limit: 32m
```

- [ ] **Step 3: Tambah scrape jobs**

Append to `scrape_configs:` in `ops/monitoring/prometheus/prometheus.yml`:

```yaml
  - job_name: postgres
    static_configs:
      - targets: ["postgres-exporter:9187"]

  - job_name: redis
    static_configs:
      - targets: ["redis-exporter:9121"]

  - job_name: blackbox-https
    metrics_path: /probe
    params:
      module: [http_2xx]
    static_configs:
      - targets: ["https://${DOMAIN}/health"]
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: blackbox-exporter:9115
```

> Note: `${DOMAIN}` in prometheus.yml is a literal here (Prometheus doesn't expand env by default). Prometheus is started with `--config.expand-external-labels` off; instead we template this at deploy via the operator, OR keep the literal `${DOMAIN}` and enable expansion. To keep it simple and verifiable, replace `${DOMAIN}` with the real domain is a deploy step — document in Task 7. For lint, `promtool check config` accepts the literal string as a target.

- [ ] **Step 4: Verify**

Run: `ops/monitoring/verify.sh`
Expected: `ALL MONITORING CHECKS PASSED` (compose valid with new exporters; promtool accepts new jobs).

- [ ] **Step 5: Commit**

```bash
git add docker-compose.monitoring.yml ops/monitoring/prometheus/prometheus.yml ops/monitoring/blackbox/blackbox.yml
git commit -m "feat(monitoring): postgres/redis/blackbox exporters + scrape"
```

---

### Task 4: Alert rules + Alertmanager → Telegram

**Files:**
- Create: `ops/monitoring/prometheus/rules/alerts.yml`
- Create: `ops/monitoring/alertmanager/alertmanager.example.yml`
- Modify: `docker-compose.monitoring.yml`
- Modify: `.gitignore`

**Interfaces:**
- Consumes: metrics from Tasks 2–3.
- Produces: Alertmanager at `alertmanager:9093` routing to Telegram; alert rules loaded by Prometheus.

- [ ] **Step 1: Tulis alert rules**

Create `ops/monitoring/prometheus/rules/alerts.yml`:

```yaml
groups:
  - name: inventra
    rules:
      - alert: InstanceDown
        expr: up == 0
        for: 2m
        labels: {severity: critical}
        annotations:
          summary: "Target {{ $labels.job }} ({{ $labels.instance }}) down"

      - alert: HostHighCPU
        expr: 100 - (avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100) > 90
        for: 10m
        labels: {severity: warning}
        annotations:
          summary: "CPU > 90% for 10m on {{ $labels.instance }}"

      - alert: HostHighMemory
        expr: (1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes) * 100 > 90
        for: 10m
        labels: {severity: warning}
        annotations:
          summary: "Memory > 90% for 10m on {{ $labels.instance }}"

      - alert: HostDiskLow
        expr: (1 - node_filesystem_avail_bytes{fstype!~"tmpfs|overlay"} / node_filesystem_size_bytes) * 100 > 85
        for: 10m
        labels: {severity: warning}
        annotations:
          summary: "Disk > 85% on {{ $labels.instance }} ({{ $labels.mountpoint }})"

      - alert: BackendHigh5xx
        expr: sum(rate(http_requests_total{status=~"5.."}[5m])) / clamp_min(sum(rate(http_requests_total[5m])), 0.001) > 0.05
        for: 5m
        labels: {severity: critical}
        annotations:
          summary: "Backend 5xx rate > 5% for 5m"

      - alert: BackendHighLatency
        expr: histogram_quantile(0.99, sum by (le) (rate(http_request_duration_seconds_bucket[5m]))) > 2
        for: 10m
        labels: {severity: warning}
        annotations:
          summary: "Backend p99 latency > 2s for 10m"

      - alert: TLSCertExpiringSoon
        expr: (probe_ssl_earliest_cert_expiry - time()) / 86400 < 14
        for: 1h
        labels: {severity: warning}
        annotations:
          summary: "TLS cert for {{ $labels.instance }} expires in < 14 days"
```

- [ ] **Step 2: Tulis alertmanager contoh (Telegram)**

Create `ops/monitoring/alertmanager/alertmanager.example.yml`:

```yaml
# Salin ke alertmanager.yml (di-gitignore), isi bot_token + chat_id nyata.
# Buat bot via @BotFather; chat_id via getUpdates atau @userinfobot.
route:
  receiver: telegram
  group_by: ["alertname"]
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 3h

receivers:
  - name: telegram
    telegram_configs:
      - bot_token: "REPLACE_WITH_BOT_TOKEN"
        chat_id: 000000000
        parse_mode: HTML
        send_resolved: true
```

- [ ] **Step 3: Tambah Alertmanager ke overlay compose**

In `docker-compose.monitoring.yml` add under `services:`:

```yaml
  alertmanager:
    image: prom/alertmanager:v0.28.0
    container_name: inventra-alertmanager
    restart: unless-stopped
    command: ["--config.file=/etc/alertmanager/alertmanager.yml"]
    volumes:
      - ./ops/monitoring/alertmanager/alertmanager.yml:/etc/alertmanager/alertmanager.yml:ro
      - alertmanager-data:/alertmanager
    mem_limit: 64m
```
And add `alertmanager-data:` under `volumes:`.

- [ ] **Step 4: gitignore real alertmanager config**

Modify `.gitignore` — add under a monitoring section:

```
# Monitoring — jangan commit rahasia nyata (repo publik)
ops/monitoring/alertmanager/alertmanager.yml
ops/monitoring/grafana.env
```

- [ ] **Step 5: Verify (rules + a real alertmanager.yml for amtool)**

Run:
```bash
cp ops/monitoring/alertmanager/alertmanager.example.yml ops/monitoring/alertmanager/alertmanager.yml
ops/monitoring/verify.sh
```
Expected: `promtool check rules` SUCCESS, `amtool check-config` SUCCESS, `ALL MONITORING CHECKS PASSED`. (The copied `alertmanager.yml` is gitignored, so it won't be committed — it's only for the amtool check.)

- [ ] **Step 6: Commit**

```bash
git add ops/monitoring/prometheus/rules/alerts.yml ops/monitoring/alertmanager/alertmanager.example.yml docker-compose.monitoring.yml .gitignore
git commit -m "feat(monitoring): alert rules + Alertmanager Telegram receiver"
```

---

### Task 5: Log aggregation (Loki + Promtail)

**Files:**
- Create: `ops/monitoring/loki/loki.yml`
- Create: `ops/monitoring/promtail/promtail.yml`
- Modify: `docker-compose.monitoring.yml`

**Interfaces:**
- Produces: Loki at `loki:3100`; Promtail ships container logs to it.

- [ ] **Step 1: Tulis loki config**

Create `ops/monitoring/loki/loki.yml`:

```yaml
auth_enabled: false
server:
  http_listen_port: 3100
common:
  instance_addr: 127.0.0.1
  path_prefix: /loki
  storage:
    filesystem:
      chunks_directory: /loki/chunks
      rules_directory: /loki/rules
  replication_factor: 1
  ring:
    kvstore:
      store: inmemory
schema_config:
  configs:
    - from: 2024-01-01
      store: tsdb
      object_store: filesystem
      schema: v13
      index:
        prefix: index_
        period: 24h
limits_config:
  retention_period: 168h
  reject_old_samples: true
  reject_old_samples_max_age: 168h
compactor:
  working_directory: /loki/compactor
  retention_enabled: true
  delete_request_store: filesystem
```

- [ ] **Step 2: Tulis promtail config (log kontainer via docker SD)**

Create `ops/monitoring/promtail/promtail.yml`:

```yaml
server:
  http_listen_port: 9080
positions:
  filename: /tmp/positions.yaml
clients:
  - url: http://loki:3100/loki/api/v1/push
scrape_configs:
  - job_name: docker
    docker_sd_configs:
      - host: unix:///var/run/docker.sock
        refresh_interval: 30s
    relabel_configs:
      - source_labels: ["__meta_docker_container_name"]
        regex: "/(.*)"
        target_label: container
      - source_labels: ["__meta_docker_container_log_stream"]
        target_label: stream
```

- [ ] **Step 3: Tambah Loki + Promtail ke overlay compose**

In `docker-compose.monitoring.yml` add under `services:`:

```yaml
  loki:
    image: grafana/loki:3.3.2
    container_name: inventra-loki
    restart: unless-stopped
    command: ["-config.file=/etc/loki/loki.yml"]
    volumes:
      - ./ops/monitoring/loki/loki.yml:/etc/loki/loki.yml:ro
      - loki-data:/loki
    mem_limit: 200m

  promtail:
    image: grafana/promtail:3.3.2
    container_name: inventra-promtail
    restart: unless-stopped
    command: ["-config.file=/etc/promtail/promtail.yml"]
    volumes:
      - ./ops/monitoring/promtail/promtail.yml:/etc/promtail/promtail.yml:ro
      - /var/run/docker.sock:/var/run/docker.sock:ro
    depends_on:
      - loki
    mem_limit: 128m
```
And add `loki-data:` under `volumes:`.

- [ ] **Step 4: Verify (compose + loki config)**

Run:
```bash
ops/monitoring/verify.sh
docker run --rm -v "$PWD/ops/monitoring/loki:/l" grafana/loki:3.3.2 \
  -config.file=/l/loki.yml -verify-config
```
Expected: `ALL MONITORING CHECKS PASSED` and Loki prints config-valid (no fatal). If `-verify-config` isn't supported by the tag, run `-print-config-stderr` and confirm it parses without error; note which you used.

- [ ] **Step 5: Commit**

```bash
git add ops/monitoring/loki/loki.yml ops/monitoring/promtail/promtail.yml docker-compose.monitoring.yml
git commit -m "feat(monitoring): Loki + Promtail log aggregation"
```

---

### Task 6: Grafana (provisioning + dashboard) + eksposur Caddy

**Files:**
- Create: `ops/monitoring/grafana/provisioning/datasources/datasources.yml`
- Create: `ops/monitoring/grafana/provisioning/dashboards/dashboards.yml`
- Create: `ops/monitoring/grafana/dashboards/backend-red.json`
- Create: `ops/monitoring/grafana.env.example`
- Modify: `docker-compose.monitoring.yml`
- Modify: `ops/caddy/Caddyfile`

**Interfaces:**
- Consumes: Prometheus + Loki datasources; `${DOMAIN}` for the Grafana subdomain.
- Produces: Grafana at `grafana:3000`, exposed at `monitoring.<domain>` via Caddy.

- [ ] **Step 1: Datasource provisioning**

Create `ops/monitoring/grafana/provisioning/datasources/datasources.yml`:

```yaml
apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
  - name: Loki
    type: loki
    access: proxy
    url: http://loki:3100
```

- [ ] **Step 2: Dashboard provider + one RED dashboard**

Create `ops/monitoring/grafana/provisioning/dashboards/dashboards.yml`:

```yaml
apiVersion: 1
providers:
  - name: inventra
    folder: Inventra
    type: file
    options:
      path: /var/lib/grafana/dashboards
```

Create `ops/monitoring/grafana/dashboards/backend-red.json`:

```json
{
  "uid": "inventra-red",
  "title": "Inventra — Backend RED",
  "schemaVersion": 39,
  "version": 1,
  "time": {"from": "now-6h", "to": "now"},
  "panels": [
    {
      "id": 1, "type": "timeseries", "title": "Request rate (req/s)",
      "gridPos": {"h": 8, "w": 12, "x": 0, "y": 0},
      "datasource": {"type": "prometheus", "uid": "${DS_PROMETHEUS}"},
      "targets": [{"expr": "sum(rate(http_requests_total[5m]))", "legendFormat": "rps"}]
    },
    {
      "id": 2, "type": "timeseries", "title": "5xx error rate",
      "gridPos": {"h": 8, "w": 12, "x": 12, "y": 0},
      "datasource": {"type": "prometheus", "uid": "${DS_PROMETHEUS}"},
      "targets": [{"expr": "sum(rate(http_requests_total{status=~\"5..\"}[5m]))", "legendFormat": "5xx/s"}]
    },
    {
      "id": 3, "type": "timeseries", "title": "p99 latency (s)",
      "gridPos": {"h": 8, "w": 24, "x": 0, "y": 8},
      "datasource": {"type": "prometheus", "uid": "${DS_PROMETHEUS}"},
      "targets": [{"expr": "histogram_quantile(0.99, sum by (le) (rate(http_request_duration_seconds_bucket[5m])))", "legendFormat": "p99"}]
    }
  ]
}
```

- [ ] **Step 3: Grafana env contoh**

Create `ops/monitoring/grafana.env.example`:

```
# Salin ke grafana.env (di-gitignore), isi password admin kuat.
GF_SECURITY_ADMIN_PASSWORD=CHANGE_ME
GF_SERVER_ROOT_URL=https://monitoring.inventra.example.com
GF_USERS_ALLOW_SIGN_UP=false
GF_ANALYTICS_REPORTING_ENABLED=false
```

- [ ] **Step 4: Tambah Grafana ke overlay compose**

In `docker-compose.monitoring.yml` add under `services:`:

```yaml
  grafana:
    image: grafana/grafana:11.4.0
    container_name: inventra-grafana
    restart: unless-stopped
    env_file:
      - path: ./ops/monitoring/grafana.env
        required: false
    volumes:
      - ./ops/monitoring/grafana/provisioning:/etc/grafana/provisioning:ro
      - ./ops/monitoring/grafana/dashboards:/var/lib/grafana/dashboards:ro
      - grafana-data:/var/lib/grafana
    depends_on:
      - prometheus
    mem_limit: 150m
```
And add `grafana-data:` under `volumes:`.

- [ ] **Step 5: Ekspos Grafana via Caddy (subdomain, tanpa WAF)**

In `ops/caddy/Caddyfile`, add a NEW site block AFTER the existing `{$DOMAIN} { ... }` block (a separate site, so Coraza WAF does NOT apply to Grafana):

```
monitoring.{$DOMAIN} {
	encode gzip zstd
	reverse_proxy grafana:3000
}
```

- [ ] **Step 6: Verify**

Run:
```bash
ops/monitoring/verify.sh
docker run --rm -e DOMAIN=example.com -e ACME_EMAIL=a@b.com \
  -v "$(pwd)/ops/caddy/Caddyfile:/etc/caddy/Caddyfile:ro" \
  -v "$(pwd)/ops/caddy/coraza-exclusions.conf:/etc/caddy/coraza-exclusions.conf:ro" \
  inventra-caddy-waf caddy validate --config /etc/caddy/Caddyfile
python -c "import json;json.load(open('ops/monitoring/grafana/dashboards/backend-red.json'));print('dashboard JSON valid')"
```
Expected: monitoring checks pass; `Valid configuration` (Caddyfile with the new monitoring site); `dashboard JSON valid`. (Build `inventra-caddy-waf` first via `docker build -t inventra-caddy-waf ./ops/caddy` if not present.)

- [ ] **Step 7: Commit**

```bash
git add ops/monitoring/grafana ops/monitoring/grafana.env.example docker-compose.monitoring.yml ops/caddy/Caddyfile
git commit -m "feat(monitoring): Grafana provisioning + dashboard + Caddy monitoring subdomain"
```

---

### Task 7: Ansible monitoring role + ADR-0011 + docs + full verify

**Files:**
- Create: `ops/ansible/roles/monitoring/tasks/main.yml`
- Modify: `ops/ansible/site.yml`
- Create: `docs/adr/0011-observability.md`
- Modify: `docs/adr/README.md`
- Modify: `docs/DEPLOYMENT.md`
- Modify: `docs/PROGRESS.md`

**Interfaces:**
- Consumes: the whole monitoring stack (Tasks 2–6) + the Ansible scaffolding (Phase 2).
- Produces: `monitoring` role bringing up the overlay; ADR-0011; operator docs.

- [ ] **Step 1: Tulis monitoring role**

Create `ops/ansible/roles/monitoring/tasks/main.yml`:

```yaml
---
- name: Bring up monitoring overlay
  become: true
  community.docker.docker_compose_v2:
    project_src: "{{ app_dir }}"
    files:
      - docker-compose.prod.yml
      - docker-compose.monitoring.yml
    env_files:
      - "{{ app_dir }}/.env.prod"
    state: present
```

Modify `ops/ansible/site.yml` — append `monitoring` after `app`:

```yaml
  roles:
    - base
    - docker
    - app
    - monitoring
```

- [ ] **Step 2: Tulis ADR-0011**

Create `docs/adr/0011-observability.md`:

```markdown
# 11. Observability — stack SRE self-hosted (Prometheus/Grafana/Loki)

Tanggal: 2026-07-06

## Status

Accepted

## Konteks

Aplikasi bank-grade di satu VPS 4 GB butuh visibilitas metrics, log, dan alert
tanpa akun eksternal dan tanpa membebani RAM berlebih.

## Keputusan

Stack self-hosted sebagai overlay compose toggleable: Prometheus (retensi 15d,
mem_limit), exporters (node, cAdvisor, postgres, redis, blackbox), Alertmanager →
Telegram, Loki+Promtail (log), Grafana (datasource+dashboard as-code). Backend
diinstrumentasi RED via `/metrics` (internal-only). Hanya Grafana publik
(subdomain, tanpa WAF, login). Rahasia via `*.example`+gitignore. Traces (Tempo)
dikecualikan (YAGNI).

## Konsekuensi

- (+) Metrics/logs/alert standar industri, reproducible, di dalam stack.
- (−) Menambah ~0.6–0.9 GB RAM; dibatasi mem_limit + retensi pendek.
- (−) Firing alert end-to-end & scrape nyata butuh VPS (langkah operator).
```

- [ ] **Step 3: Update adr/README.md** — add the ADR-0011 row in the file's existing format (read it first; 0011 was reserved for this).

- [ ] **Step 4: Tambah bagian Monitoring ke DEPLOYMENT.md** (before "## Referensi perintah cepat", numbered §16):

```markdown
---

## 16. Monitoring & Observability

Stack observability adalah overlay toggleable (`docker-compose.monitoring.yml`).

```bash
cd ~/inventra
cp ops/monitoring/alertmanager/alertmanager.example.yml ops/monitoring/alertmanager/alertmanager.yml   # isi bot_token + chat_id
cp ops/monitoring/grafana.env.example ops/monitoring/grafana.env                                        # isi password admin + GF_SERVER_ROOT_URL
sed -i "s#\${DOMAIN}#inventra.ragilbuaj.web.id#g" ops/monitoring/prometheus/prometheus.yml              # isi domain nyata untuk blackbox
docker compose -f docker-compose.prod.yml -f docker-compose.monitoring.yml --env-file .env.prod up -d
```

- Tambahkan DNS A record `monitoring.<domain>` → IP VPS; Grafana ada di `https://monitoring.<domain>` (login admin dari grafana.env).
- Hanya Grafana yang publik; Prometheus/Alertmanager/exporters internal-only.
- Alert dikirim ke Telegram via Alertmanager. Validasi config lokal: `ops/monitoring/verify.sh`.
```

- [ ] **Step 5: Update PROGRESS.md** — tick Fase 3 done; refresh "Next session" (ops-hardening trilogy complete). Read the file first.

- [ ] **Step 6: Full verify**

Run:
```bash
ops/monitoring/verify.sh
cd backend && go build ./... && go test ./internal/middleware/ -run TestMetrics && cd ..
cd ops/ansible && MSYS_NO_PATHCONV=1 bash lint.sh && cd ../..
```
Expected: monitoring checks pass; backend builds + metrics test passes; ansible lint `ALL CHECKS PASSED` (with the new monitoring role).

- [ ] **Step 7: Commit**

```bash
git add ops/ansible/roles/monitoring ops/ansible/site.yml docs/adr/0011-observability.md docs/adr/README.md docs/DEPLOYMENT.md docs/PROGRESS.md
git commit -m "feat(monitoring): ansible monitoring role + ADR-0011 + docs"
```

---

## Catatan (di luar task — langkah operator)

Verifikasi in-repo = go test (instrumentasi) + `promtool`/`amtool`/`docker compose config` + `caddy validate` + ansible lint. **Uji sesungguhnya** — Prometheus benar-benar men-scrape target (`up==1`), dashboard Grafana berisi data, dan satu alert benar-benar terkirim ke Telegram (mis. hentikan satu kontainer) — dilakukan operator terhadap stack yang berjalan (idealnya VM Ubuntu 24.04 sekali-pakai dulu, lalu VPS). Perhatikan tekanan RAM di 4 GB: pantau `docker stats`; turunkan retensi/`mem_limit` bila perlu.
