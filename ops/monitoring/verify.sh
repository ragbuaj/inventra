#!/usr/bin/env bash
# Validasi konfigurasi monitoring tanpa menjalankan stack penuh.
set -euo pipefail
cd "$(dirname "$0")/../.."   # repo root

echo "== docker compose config =="
DOMAIN=x ACME_EMAIL=x DB_PASSWORD=x JWT_SECRET=x MINIO_ROOT_USER=x MINIO_ROOT_PASSWORD=x \
  docker compose -f docker-compose.prod.yml -f docker-compose.monitoring.yml config >/dev/null
echo "compose OK"

echo "== promtool check config =="
docker run --rm --entrypoint promtool -v "$PWD/ops/monitoring/prometheus:/p" prom/prometheus:v3.1.0 \
  check config /p/prometheus.yml

if compgen -G "ops/monitoring/prometheus/rules/*.yml" >/dev/null; then
  echo "== promtool check rules =="
  docker run --rm --entrypoint promtool -v "$PWD/ops/monitoring/prometheus:/p" prom/prometheus:v3.1.0 \
    check rules /p/rules/*.yml
fi

if [ -f ops/monitoring/alertmanager/alertmanager.yml ]; then
  echo "== amtool check-config =="
  docker run --rm --entrypoint amtool -v "$PWD/ops/monitoring/alertmanager:/a" prom/alertmanager:v0.28.0 \
    check-config /a/alertmanager.yml
fi
echo "ALL MONITORING CHECKS PASSED"
