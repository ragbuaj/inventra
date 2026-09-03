#!/usr/bin/env bash
# Validasi konfigurasi monitoring tanpa menjalankan stack penuh.
set -euo pipefail
cd "$(dirname "$0")/../.."   # repo root

echo "== docker compose config =="
DOMAIN=x ACME_EMAIL=x DB_PASSWORD=x JWT_SECRET=x MINIO_ROOT_USER=x MINIO_ROOT_PASSWORD=x \
  docker compose -f docker-compose.prod.yml -f docker-compose.monitoring.yml config >/dev/null
echo "compose OK"

# Target blackbox datang dari berkas file_sd yang dirender saat runtime oleh layanan
# `prometheus-targets` (docker-compose.monitoring.yml) dari DOMAIN di .env.prod. Berkas
# itu belum ada saat pemeriksaan ini, jadi kita render tiruannya di jalur yang sama
# supaya promtool benar-benar memvalidasi isinya.
#
# Direktorinya sengaja DI DALAM repo, bukan dari `mktemp -d` di /tmp: di Docker Desktop
# (Windows/macOS) sumber bind-mount yang berawalan `/tmp` diselesaikan di dalam VM
# Linux-nya, bukan di mesin host, sehingga mount-nya berhasil tapi isinya kosong dan
# pemeriksaan ini diam-diam jadi tidak ada gunanya.
sd_dir="ops/monitoring/.verify-targets"
mkdir -p "$sd_dir"
trap 'rm -rf "$sd_dir"' EXIT
printf '[{"targets":["https://example.invalid/health"]}]' > "$sd_dir/blackbox.json"

echo "== promtool check config =="
# promtool hanya MEMPERINGATKAN untuk berkas file_sd yang hilang dan tetap keluar 0, jadi
# salah ketik pada jalurnya bisa lolos diam-diam. Keluarannya diperiksa eksplisit.
promtool_out="$(docker run --rm --entrypoint promtool \
  -v "$PWD/ops/monitoring/prometheus:/p" \
  -v "$PWD/$sd_dir:/etc/prometheus/targets:ro" \
  prom/prometheus:v3.1.0 \
  check config /p/prometheus.yml 2>&1)"
echo "$promtool_out"
if echo "$promtool_out" | grep -q "does not exist"; then
  echo "GAGAL: promtool tidak menemukan berkas file_sd yang dirujuk prometheus.yml." >&2
  echo "       Cocokkan jalurnya dengan mount di docker-compose.monitoring.yml." >&2
  exit 1
fi

if compgen -G "ops/monitoring/prometheus/rules/*.yml" >/dev/null; then
  echo "== promtool check rules =="
  docker run --rm --entrypoint sh -v "$PWD/ops/monitoring/prometheus:/p" prom/prometheus:v3.1.0 \
    -c 'promtool check rules /p/rules/*.yml'
fi

if [ -f ops/monitoring/alertmanager/alertmanager.yml ]; then
  echo "== amtool check-config =="
  docker run --rm --entrypoint amtool -v "$PWD/ops/monitoring/alertmanager:/a" prom/alertmanager:v0.28.0 \
    check-config /a/alertmanager.yml
fi
echo "ALL MONITORING CHECKS PASSED"
