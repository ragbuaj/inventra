# IaC (Ansible) — Fase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Mengkodifikasi seluruh setup manual server Inventra (`docs/DEPLOYMENT.md`) menjadi **playbook Ansible idempoten** sehingga VPS produksi bisa dibangun ulang identik dari nol, dengan rahasia dikelola via Ansible Vault.

**Architecture:** `ops/ansible/` berisi playbook `site.yml` yang menjalankan tiga role berurutan — `base` (paket, swap, ufw, unattended-upgrades, user+SSH hardening), `docker` (Docker Engine + compose plugin), `app` (checkout repo, render `.env.prod` dari Vault, `docker compose up` stack produksi — yang otomatis membangun image Caddy+WAF). Verifikasi lokal memakai **Ansible di dalam container** (`--syntax-check` + `ansible-lint`); `apply`/uji idempotence terhadap VPS nyata adalah langkah operator terdokumentasi.

**Tech Stack:** Ansible (ansible-core), ansible-lint, collections `community.docker` + `community.general` + `ansible.posix`, Ansible Vault, Docker (untuk menjalankan tooling Ansible secara ter-container).

## Global Constraints

- **Idempoten:** run kedua playbook = `changed=0`. Setiap task pakai modul idempoten / guard (bukan `command`/`shell` tanpa `creates`/`when`).
- **Repo publik → tidak ada rahasia di git.** Commit `*.example` (placeholder) + `.gitignore` untuk file nyata (`inventory.ini`, `group_vars/all/vault.yml`). JANGAN commit vault terenkripsi maupun kunci.
- **Verifikasi ter-container** (Ansible tidak ada di host): semua `--syntax-check`/`ansible-lint` dijalankan lewat image `ops/ansible/tools/Dockerfile` (versi ansible-core & ansible-lint di-pin).
- **Selaras dengan DEPLOYMENT.md:** langkah, nilai, dan nama (user `deploy`, path `~/inventra`, swap 2 GB, ufw 22/80/443, `docker-compose.prod.yml` + `--env-file .env.prod`) harus sama persis dengan yang sudah terbukti manual.
- **WAF sudah di dalam compose** (service `caddy` build `./ops/caddy`) — role `app` yang `compose up --build` mencakup WAF; TIDAK ada role `waf` terpisah (penyederhanaan dari spec, dicatat di ADR-0013).
- **SSH hardening aman:** set `authorized_keys` user `deploy` SEBELUM menonaktifkan password auth, agar tak mengunci diri.
- No `Co-Authored-By`/atribusi AI di commit.

---

### Task 1: Scaffolding Ansible + harness lint ter-container

Kerangka `ops/ansible/` yang lolos `--syntax-check` + `ansible-lint`, plus tooling ter-container. Role masih kosong (diisi Task 2–4).

**Files:**
- Create: `ops/ansible/ansible.cfg`
- Create: `ops/ansible/site.yml`
- Create: `ops/ansible/inventory.example.ini`
- Create: `ops/ansible/requirements.yml`
- Create: `ops/ansible/group_vars/all/vars.yml`
- Create: `ops/ansible/group_vars/all/vault.example.yml`
- Create: `ops/ansible/roles/base/tasks/main.yml` (placeholder valid)
- Create: `ops/ansible/roles/docker/tasks/main.yml` (placeholder valid)
- Create: `ops/ansible/roles/app/tasks/main.yml` (placeholder valid)
- Create: `ops/ansible/tools/Dockerfile`
- Create: `ops/ansible/lint.sh`
- Create: `ops/ansible/README.md`
- Modify: `.gitignore`

**Interfaces:**
- Produces: `ops/ansible/lint.sh` — builds the tools image and runs `ansible-playbook --syntax-check` + `ansible-lint`; exit 0 = clean. Roles `base`, `docker`, `app` (task files filled by later tasks). Vault var names consumed by `.env.prod.j2` (Task 4): `vault_domain`, `vault_acme_email`, `vault_db_password`, `vault_jwt_secret`, `vault_minio_user`, `vault_minio_password`, `vault_deploy_pubkey`.

- [ ] **Step 1: Tulis Dockerfile tooling (Ansible ter-pin)**

Create `ops/ansible/tools/Dockerfile`:

```dockerfile
# syntax=docker/dockerfile:1
# Tooling Ansible ter-container (host tidak punya ansible). Versi di-pin.
FROM python:3.12-alpine
RUN apk add --no-cache git openssh-client \
 && pip install --no-cache-dir "ansible-core==2.17.6" "ansible-lint==24.9.2"
WORKDIR /work
```

> Jika versi ter-pin gagal `pip install`, naikkan ke versi patch stabil terdekat yang terpasang sukses dan catat di report.

- [ ] **Step 2: Tulis skrip lint/syntax**

Create `ops/ansible/lint.sh`:

```bash
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
```

Run: `chmod +x ops/ansible/lint.sh`

- [ ] **Step 3: Tulis ansible.cfg + requirements + inventory contoh**

Create `ops/ansible/ansible.cfg`:

```ini
[defaults]
inventory = inventory.ini
roles_path = roles
collections_path = collections
host_key_checking = False
retry_files_enabled = False
stdout_callback = yaml
```

Create `ops/ansible/requirements.yml`:

```yaml
---
collections:
  - name: community.docker
    version: ">=3.0.0"
  - name: community.general
    version: ">=8.0.0"
  - name: ansible.posix
    version: ">=1.5.0"
```

Create `ops/ansible/inventory.example.ini`:

```ini
# Salin ke inventory.ini (di-gitignore) dan isi host VPS nyata.
[inventra]
prod ansible_host=REPLACE_WITH_VPS_IP ansible_user=deploy

[inventra:vars]
ansible_python_interpreter=/usr/bin/python3
```

- [ ] **Step 4: Tulis group_vars (mapping non-rahasia + vault contoh)**

Create `ops/ansible/group_vars/all/vars.yml`:

```yaml
---
# Konfigurasi non-rahasia + pemetaan ke variabel Vault.
deploy_user: deploy
app_dir: "/home/{{ deploy_user }}/inventra"
repo_url: "https://github.com/ragbuaj/inventra.git"
repo_version: main
swap_size_mb: 2048

# Nilai berikut berasal dari Vault (group_vars/all/vault.yml, di-gitignore).
domain: "{{ vault_domain }}"
acme_email: "{{ vault_acme_email }}"
db_password: "{{ vault_db_password }}"
jwt_secret: "{{ vault_jwt_secret }}"
minio_user: "{{ vault_minio_user }}"
minio_password: "{{ vault_minio_password }}"
deploy_pubkey: "{{ vault_deploy_pubkey }}"
```

Create `ops/ansible/group_vars/all/vault.example.yml`:

```yaml
---
# CONTOH. Salin ke vault.yml (di-gitignore), isi nilai nyata, lalu enkripsi:
#   docker run --rm -it -v "$PWD:/work" -w /work inventra-ansible-tools \
#     ansible-vault encrypt group_vars/all/vault.yml
vault_domain: "inventra.example.com"
vault_acme_email: "you@example.com"
vault_db_password: "CHANGE_ME"
vault_jwt_secret: "CHANGE_ME"
vault_minio_user: "inventra-minio"
vault_minio_password: "CHANGE_ME"
vault_deploy_pubkey: "ssh-ed25519 AAAA... deploy@laptop"
```

- [ ] **Step 5: Tulis site.yml + role placeholder valid**

Create `ops/ansible/site.yml`:

```yaml
---
- name: Provision Inventra production host
  hosts: inventra
  become: true
  roles:
    - base
    - docker
    - app
```

Create `ops/ansible/roles/base/tasks/main.yml`, `ops/ansible/roles/docker/tasks/main.yml`, `ops/ansible/roles/app/tasks/main.yml`, each with:

```yaml
---
# Diisi pada task berikutnya.
- name: Placeholder
  ansible.builtin.debug:
    msg: "role belum diimplementasikan"
```

- [ ] **Step 6: README + .gitignore**

Create `ops/ansible/README.md`:

```markdown
# Ansible — provisioning VPS Inventra

Mengkodifikasi setup di `docs/DEPLOYMENT.md`. Host tidak butuh Ansible — tooling
berjalan di container (`tools/Dockerfile`).

## Sekali-pakai
```bash
cp inventory.example.ini inventory.ini            # isi IP VPS
cp group_vars/all/vault.example.yml group_vars/all/vault.yml  # isi rahasia, lalu enkripsi
```

## Verifikasi (tanpa menyentuh server)
```bash
ops/ansible/lint.sh          # syntax-check + ansible-lint
```

## Apply ke VPS (langkah operator — butuh SSH + vault password)
```bash
docker run --rm -it -v "$PWD:/work" -w /work -v ~/.ssh:/root/.ssh:ro \
  inventra-ansible-tools ansible-playbook -i inventory.ini site.yml --ask-vault-pass --check   # dry-run
# hilangkan --check untuk apply sungguhan; jalankan 2x → run kedua changed=0 (idempotence)
```
```

Modify `.gitignore` — tambahkan di bawah bagian environment files:

```
# Ansible — jangan commit host & rahasia nyata (repo publik)
ops/ansible/inventory.ini
ops/ansible/group_vars/all/vault.yml
ops/ansible/collections/
```

- [ ] **Step 7: Verifikasi skeleton lolos lint**

Run:
```bash
ops/ansible/lint.sh
```
Expected: build image sukses, `ALL CHECKS PASSED` (syntax-check + ansible-lint bersih pada skeleton).

- [ ] **Step 8: Commit**

```bash
git add ops/ansible .gitignore
git commit -m "feat(iac): ansible scaffolding + containerized lint harness"
```

---

### Task 2: Role `base` (paket, swap, ufw, unattended-upgrades, user, SSH)

**Files:**
- Modify: `ops/ansible/roles/base/tasks/main.yml`
- Create: `ops/ansible/roles/base/handlers/main.yml`

**Interfaces:**
- Consumes: `deploy_user`, `swap_size_mb`, `deploy_pubkey` (Task 1).
- Produces: user `deploy` dengan sudo + authorized_keys; swap aktif; ufw mengizinkan 22/80/443; password SSH nonaktif.

- [ ] **Step 1: Tulis handler restart ssh**

Create `ops/ansible/roles/base/handlers/main.yml`:

```yaml
---
- name: Restart ssh
  ansible.builtin.service:
    name: ssh
    state: restarted
```

- [ ] **Step 2: Tulis tasks base**

Replace `ops/ansible/roles/base/tasks/main.yml` with:

```yaml
---
- name: Update apt cache
  ansible.builtin.apt:
    update_cache: true
    cache_valid_time: 3600

- name: Install base packages
  ansible.builtin.apt:
    name:
      - ufw
      - unattended-upgrades
      - ca-certificates
      - curl
      - gnupg
      - git
    state: present

- name: Create swapfile (idempotent via creates)
  ansible.builtin.command:
    cmd: "fallocate -l {{ swap_size_mb }}M /swapfile"
    creates: /swapfile
  register: swap_created

- name: Secure swapfile perms
  ansible.builtin.file:
    path: /swapfile
    owner: root
    group: root
    mode: "0600"

- name: Make swap
  ansible.builtin.command: mkswap /swapfile
  when: swap_created is changed
  changed_when: true

- name: Enable swap in fstab
  ansible.posix.mount:
    path: none
    src: /swapfile
    fstype: swap
    opts: sw
    state: present

- name: Activate swap now
  ansible.builtin.command: swapon /swapfile
  when: swap_created is changed
  changed_when: true

- name: Enable unattended-upgrades
  ansible.builtin.copy:
    dest: /etc/apt/apt.conf.d/20auto-upgrades
    content: |
      APT::Periodic::Update-Package-Lists "1";
      APT::Periodic::Unattended-Upgrade "1";
    owner: root
    group: root
    mode: "0644"

- name: Create deploy user
  ansible.builtin.user:
    name: "{{ deploy_user }}"
    groups: sudo
    append: true
    shell: /bin/bash
    create_home: true

- name: Install deploy authorized key
  ansible.posix.authorized_key:
    user: "{{ deploy_user }}"
    key: "{{ deploy_pubkey }}"
    state: present

- name: Allow OpenSSH through ufw
  community.general.ufw:
    rule: allow
    name: OpenSSH

- name: Allow HTTP/HTTPS through ufw
  community.general.ufw:
    rule: allow
    port: "{{ item }}"
    proto: tcp
  loop: ["80", "443"]

- name: Enable ufw with default deny incoming
  community.general.ufw:
    state: enabled
    policy: deny
    direction: incoming

- name: Harden SSH (key-only, no root)
  ansible.builtin.copy:
    dest: /etc/ssh/sshd_config.d/10-hardening.conf
    content: |
      PasswordAuthentication no
      PermitRootLogin no
    owner: root
    group: root
    mode: "0644"
  notify: Restart ssh
```

- [ ] **Step 3: Verifikasi lint**

Run:
```bash
ops/ansible/lint.sh
```
Expected: `ALL CHECKS PASSED` (base role syntax-valid + lint-clean).

- [ ] **Step 4: Commit**

```bash
git add ops/ansible/roles/base
git commit -m "feat(iac): base role — packages, swap, ufw, users, SSH hardening"
```

---

### Task 3: Role `docker` (Docker Engine + compose plugin)

**Files:**
- Modify: `ops/ansible/roles/docker/tasks/main.yml`

**Interfaces:**
- Consumes: `deploy_user` (Task 1).
- Produces: Docker Engine + compose plugin terpasang; user `deploy` di grup `docker`.

- [ ] **Step 1: Tulis tasks docker**

Replace `ops/ansible/roles/docker/tasks/main.yml` with:

```yaml
---
- name: Create apt keyrings dir
  ansible.builtin.file:
    path: /etc/apt/keyrings
    state: directory
    mode: "0755"

- name: Add Docker GPG key
  ansible.builtin.get_url:
    url: https://download.docker.com/linux/ubuntu/gpg
    dest: /etc/apt/keyrings/docker.asc
    mode: "0644"

- name: Add Docker apt repository
  ansible.builtin.apt_repository:
    repo: >-
      deb [arch={{ ansible_architecture | replace('x86_64', 'amd64') }}
      signed-by=/etc/apt/keyrings/docker.asc]
      https://download.docker.com/linux/ubuntu {{ ansible_distribution_release }} stable
    filename: docker
    state: present

- name: Install Docker Engine + compose plugin
  ansible.builtin.apt:
    name:
      - docker-ce
      - docker-ce-cli
      - containerd.io
      - docker-buildx-plugin
      - docker-compose-plugin
    state: present
    update_cache: true

- name: Ensure docker service enabled + running
  ansible.builtin.service:
    name: docker
    state: started
    enabled: true

- name: Add deploy user to docker group
  ansible.builtin.user:
    name: "{{ deploy_user }}"
    groups: docker
    append: true
```

- [ ] **Step 2: Verifikasi lint**

Run:
```bash
ops/ansible/lint.sh
```
Expected: `ALL CHECKS PASSED`.

- [ ] **Step 3: Commit**

```bash
git add ops/ansible/roles/docker
git commit -m "feat(iac): docker role — engine + compose plugin"
```

---

### Task 4: Role `app` (repo, .env.prod dari Vault, compose up)

**Files:**
- Modify: `ops/ansible/roles/app/tasks/main.yml`
- Create: `ops/ansible/roles/app/templates/env.prod.j2`

**Interfaces:**
- Consumes: `app_dir`, `repo_url`, `repo_version`, `deploy_user`, dan var Vault (`domain`, `acme_email`, `db_password`, `jwt_secret`, `minio_user`, `minio_password`).
- Produces: repo ter-checkout di `{{ app_dir }}`, `.env.prod` ter-render (0600), stack produksi `up -d`.

- [ ] **Step 1: Tulis template .env.prod**

Create `ops/ansible/roles/app/templates/env.prod.j2`:

```jinja
# Dikelola oleh Ansible (role app) — JANGAN edit manual di server.
DOMAIN={{ domain }}
ACME_EMAIL={{ acme_email }}
DB_PASSWORD={{ db_password }}
JWT_SECRET={{ jwt_secret }}
MINIO_ROOT_USER={{ minio_user }}
MINIO_ROOT_PASSWORD={{ minio_password }}
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
```

- [ ] **Step 2: Tulis tasks app**

Replace `ops/ansible/roles/app/tasks/main.yml` with:

```yaml
---
- name: Checkout application repo
  become: true
  become_user: "{{ deploy_user }}"
  ansible.builtin.git:
    repo: "{{ repo_url }}"
    dest: "{{ app_dir }}"
    version: "{{ repo_version }}"
    update: true

- name: Render .env.prod from Vault
  ansible.builtin.template:
    src: env.prod.j2
    dest: "{{ app_dir }}/.env.prod"
    owner: "{{ deploy_user }}"
    group: "{{ deploy_user }}"
    mode: "0600"

- name: Bring up production stack
  become: true
  become_user: "{{ deploy_user }}"
  community.docker.docker_compose_v2:
    project_src: "{{ app_dir }}"
    files:
      - docker-compose.prod.yml
    env_files:
      - "{{ app_dir }}/.env.prod"
    state: present
    build: policy
```

- [ ] **Step 3: Verifikasi lint**

Run:
```bash
ops/ansible/lint.sh
```
Expected: `ALL CHECKS PASSED`.

- [ ] **Step 4: Verifikasi Vault round-trip (enkripsi/dekripsi bekerja)**

Run:
```bash
cd ops/ansible
docker build -q -t inventra-ansible-tools ./tools
cp group_vars/all/vault.example.yml /tmp/vault-test.yml
printf 'testpass' > /tmp/vpass
docker run --rm -v "$PWD:/work" -v /tmp:/tmp -w /work inventra-ansible-tools \
  sh -c 'ansible-vault encrypt --vault-password-file /tmp/vpass /tmp/vault-test.yml \
     && head -1 /tmp/vault-test.yml \
     && ansible-vault decrypt --vault-password-file /tmp/vpass /tmp/vault-test.yml \
     && echo VAULT_ROUNDTRIP_OK'
rm -f /tmp/vault-test.yml /tmp/vpass
cd ../..
```
Expected: baris pertama `$ANSIBLE_VAULT;1.1;AES256`, lalu `VAULT_ROUNDTRIP_OK`.

- [ ] **Step 5: Commit**

```bash
git add ops/ansible/roles/app
git commit -m "feat(iac): app role — repo checkout, .env.prod from Vault, compose up"
```

---

### Task 5: ADR-0013 + dokumentasi + verifikasi playbook penuh

**Files:**
- Create: `docs/adr/0013-iac.md`
- Modify: `docs/adr/README.md`
- Modify: `docs/DEPLOYMENT.md` (bagian IaC)
- Modify: `docs/PROGRESS.md`

**Interfaces:**
- Consumes: seluruh `ops/ansible/` (Task 1–4).
- Produces: ADR-0013, dokumentasi operator, PROGRESS tercatat.

- [ ] **Step 1: Tulis ADR-0013**

Create `docs/adr/0013-iac.md`:

```markdown
# 13. Infrastructure as Code — Ansible untuk provisioning VPS

Tanggal: 2026-07-06

## Status

Accepted

## Konteks

Setup VPS produksi (`docs/DEPLOYMENT.md`) dilakukan manual: rawan langkah
terlewat, tidak reproducible, dan sulit dibangun ulang saat pindah server.

## Keputusan

Mengkodifikasi setup sebagai **Ansible** (`ops/ansible/`), role `base` + `docker`
+ `app`, dijalankan idempoten dari control node via SSH. Rahasia dikelola
**Ansible Vault**. Tooling Ansible dijalankan **ter-container** (host tak perlu
Ansible). WAF tidak punya role terpisah — sudah ter-encode di stack compose yang
di-`up` role `app`. Role `monitoring` menyusul di Fase 3.

## Alternatif yang ditolak

- **cloud-init:** hanya berjalan saat first-boot; tak idempoten pada server hidup.
- **Skrip bash:** tak idempoten, sulit di-review sebagai konfigurasi deklaratif.

## Konsekuensi

- (+) Server reproducible; run kedua = `changed=0`.
- (+) Rahasia tak lagi plaintext di server (Vault).
- (−) Untuk repo publik, `inventory.ini` & `vault.yml` nyata TIDAK di-commit
  (hanya `*.example`); operator menyalin & mengisi lokal.
- (−) `apply`/uji idempotence butuh SSH ke VPS nyata (tak bisa di CI tanpa target).
```

- [ ] **Step 2: Update docs/adr/README.md**

Modify `docs/adr/README.md` — tambahkan baris untuk ADR-0013 (ikuti format baris ADR yang ada, mis. `| 0013 | Infrastructure as Code (Ansible) | Accepted |` atau format tabel/daftar yang dipakai file itu; baca dulu strukturnya).

- [ ] **Step 3: Tambah bagian IaC ke DEPLOYMENT.md**

Modify `docs/DEPLOYMENT.md` — tambahkan sub-bagian baru sebelum "## Referensi perintah cepat":

```markdown
---

## 15. Provisioning otomatis (Ansible / IaC)

Alih-alih langkah 2–8 manual, seluruh setup server tersedia sebagai playbook
Ansible di `ops/ansible/` (lihat `ops/ansible/README.md`). Tooling berjalan
ter-container — host cukup punya Docker.

```bash
cd ops/ansible
cp inventory.example.ini inventory.ini                          # isi IP VPS
cp group_vars/all/vault.example.yml group_vars/all/vault.yml    # isi rahasia
docker build -t inventra-ansible-tools ./tools
docker run --rm -it -v "$PWD:/work" -w /work inventra-ansible-tools \
  ansible-vault encrypt group_vars/all/vault.yml                # enkripsi vault
# Dry-run lalu apply (jalankan 2x → run kedua changed=0):
docker run --rm -it -v "$PWD:/work" -w /work -v ~/.ssh:/root/.ssh:ro \
  inventra-ansible-tools ansible-playbook -i inventory.ini site.yml --ask-vault-pass --check
```

`inventory.ini` & `vault.yml` di-gitignore (rahasia). WAF ikut ter-provision
karena role `app` menjalankan `docker compose up --build` (image Caddy+Coraza).
```

- [ ] **Step 4: Update PROGRESS.md**

Modify `docs/PROGRESS.md` — tandai Fase 2 IaC selesai (satu baris, konsisten gaya file), dan refresh blok "Next session" ke Fase 3 (Monitoring). Baca file dulu untuk menempatkan dengan tepat.

- [ ] **Step 5: Verifikasi playbook penuh lolos lint**

Run:
```bash
ops/ansible/lint.sh
```
Expected: `ALL CHECKS PASSED` pada playbook lengkap (3 role terisi).

- [ ] **Step 6: Commit**

```bash
git add docs/adr/0013-iac.md docs/adr/README.md docs/DEPLOYMENT.md docs/PROGRESS.md
git commit -m "feat(iac): ADR-0013 + operator docs + PROGRESS"
```

---

## Catatan (di luar task — langkah operator)

Verifikasi in-repo terbatas pada `--syntax-check` + `ansible-lint` (Ansible ter-container). **Uji sesungguhnya** — `ansible-playbook ... --check` lalu apply, dijalankan 2× untuk membuktikan idempotence (`changed=0`) — dilakukan operator terhadap VPS nyata (atau VM sekali-pakai), karena butuh SSH + vault password yang tidak ada di environment dev/CI. Idealnya diuji dulu di VM Ubuntu 24.04 sekali-pakai sebelum ke server produksi.
