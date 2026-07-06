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
