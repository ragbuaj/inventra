#!/usr/bin/env bash
# Verifikasi playbook via Ansible ter-container: syntax-check + ansible-lint.
#   ops/ansible/lint.sh
set -euo pipefail
cd "$(dirname "$0")"

docker build -q -t inventra-ansible-tools ./tools

run() { docker run --rm -v "$PWD:/work" -w /work inventra-ansible-tools "$@"; }

echo "== installing collections =="
run ansible-galaxy collection install -r requirements.yml -p ./collections
echo "== syntax-check =="
run env ANSIBLE_COLLECTIONS_PATH=./collections \
  ansible-playbook --syntax-check -i inventory.example.ini site.yml
echo "== ansible-lint =="
run env ANSIBLE_COLLECTIONS_PATH=./collections ansible-lint
echo "ALL CHECKS PASSED"
