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
- (−) Servis app stack (`docker-compose.prod.yml`: Postgres, Nuxt SSR, MinIO,
  backend) sengaja **tidak** diberi `mem_limit`, sedangkan hanya tier monitoring
  yang dibatasi — di bawah tekanan memori, container monitoring adalah tier yang
  memang dikorbankan (OOM-sacrifice) lebih dulu, bukan app stack. Jaring pengaman
  adalah 2 GB swap yang sudah disiapkan role `base` (Phase 2). Operator perlu
  memantau `docker stats` dan menurunkan `mem_limit`/retensi monitoring bila app
  stack justru yang kekurangan RAM.
