#!/usr/bin/env bash
# Smoke-test WAF: serangan diblokir (403), trafik sah lolos (2xx/3xx).
#   ops/waf-smoketest.sh [base-url]   (default http://localhost:8080)
set -uo pipefail

BASE="${1:-http://localhost:8080}"
fail=0

check() {
  local desc="$1" expect="$2" url="$3"
  local code
  code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 --path-as-is "$url")"
  if [ "$code" = "$expect" ]; then
    echo "PASS  [$code] $desc"
  else
    echo "FAIL  [$code != $expect] $desc"
    fail=1
  fi
}

check_json_post() {
  local desc="$1" expect="$2" url="$3" body="$4"
  local code
  code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 -X POST \
    -H 'Content-Type: application/json' -d "$body" "$url")"
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
check_json_post "legit JSON POST (app-shaped body)" 200 "$BASE/" '{"name":"kursi kantor","qty":2}'

if [ "$fail" -ne 0 ]; then
  echo "SMOKE TEST GAGAL"; exit 1
fi
echo "SMOKE TEST LULUS"
