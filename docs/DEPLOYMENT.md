# Deployment — Inventra ke VPS (Biznetgio NEO Lite, Ubuntu 24.04)

Panduan deploy Inventra ke satu VPS (2 vCPU / 4 GB RAM) memakai Docker Compose +
Caddy (HTTPS otomatis). Semua service (PostgreSQL, Redis, MinIO, backend Go,
frontend Nuxt, reverse proxy) berjalan di satu mesin.

> **Perlu install Claude di server? Tidak.** Claude Code adalah alat bantu
> _development_ di komputer Anda. Server produksi cukup Docker + kode aplikasi.
> Jangan pasang Claude/CLI AI di server publik.

---

## 0. Arsitektur & spesifikasi

```
Internet ──443/80──▶ Caddy ──┬─ /            ─▶ frontend (Nuxt, :3000)
                             └─ /api/*, /health ─▶ backend (Go, :8080)
                                                     │
                          jaringan internal Docker   ├─▶ postgres :5432
                          (tidak diekspos ke publik) ├─▶ redis    :6379
                                                     └─▶ minio    :9000
```

- Hanya **port 80 & 443** yang terbuka ke internet. DB/Redis/MinIO hanya bisa
  diakses antar-container.
- Caddy mengurus sertifikat TLS Let's Encrypt secara otomatis (butuh domain).
- Catatan RAM: **build image Nuxt butuh ~4 GB heap**. Di VPS 4 GB, build tanpa
  swap bisa gagal (OOM/"killed"). Langkah 3 menambahkan swap — jangan dilewati.

---

## 1. Prasyarat

- Sudah punya **domain** (mis. `inventra.example.com`). Tanpa domain, HTTPS
  otomatis tidak jalan — lihat _Troubleshooting → Tanpa domain_.
- Akses SSH root/sudo ke VPS.
- **DNS**: buat record **A** dari domain Anda ke **IP publik VPS** sebelum mulai,
  supaya Let's Encrypt bisa memverifikasi saat stack naik.
  ```
  inventra.example.com.   A   <IP-PUBLIK-VPS>
  ```

---

## 2. Login & pengamanan dasar server

SSH ke server, buat user non-root, aktifkan firewall.

```bash
ssh root@<IP-PUBLIK-VPS>

# User non-root dengan sudo (ganti "deploy" sesuai selera)
adduser deploy
usermod -aG sudo deploy

# Firewall: izinkan SSH + HTTP/HTTPS saja
apt update && apt install -y ufw
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
ufw enable

# Lanjut sebagai user deploy
su - deploy
```

---

## 3. Tambah swap 2 GB (WAJIB di VPS 4 GB)

Mencegah proses build Nuxt ter-kill karena kehabisan memori.

```bash
sudo fallocate -l 2G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
free -h        # verifikasi kolom Swap terisi 2.0Gi
```

---

## 4. Install Docker Engine + Compose plugin

```bash
# Repo resmi Docker untuk Ubuntu
sudo apt update
sudo apt install -y ca-certificates curl gnupg
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | \
  sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
  https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo $VERSION_CODENAME) stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# Jalankan docker tanpa sudo
sudo usermod -aG docker $USER
newgrp docker          # atau logout/login ulang
docker --version && docker compose version
```

---

## 5. Ambil kode aplikasi

```bash
sudo apt install -y git
git clone <URL-REPO-ANDA> inventra
cd inventra
git checkout main       # gunakan branch rilis; hindari deploy dari branch fitur
```

> File `docker-compose.prod.yml`, `ops/Caddyfile`, dan `.env.prod.example` sudah
> ada di repo. Bila belum ada, tarik commit terbaru dulu.

---

## 6. Konfigurasi rahasia (`.env.prod`)

```bash
cp .env.prod.example .env.prod
nano .env.prod
```

Isi minimal:

| Variabel              | Isi                                                        |
| --------------------- | ---------------------------------------------------------- |
| `DOMAIN`              | domain Anda, mis. `inventra.example.com` (tanpa `https://`)|
| `ACME_EMAIL`          | email valid untuk notifikasi Let's Encrypt                 |
| `DB_PASSWORD`         | password DB kuat — `openssl rand -hex 24`                  |
| `JWT_SECRET`          | `openssl rand -hex 32`                                     |
| `MINIO_ROOT_USER`     | mis. `inventra-minio`                                      |
| `MINIO_ROOT_PASSWORD` | `openssl rand -hex 24`                                     |
| `GOOGLE_CLIENT_*`     | isi hanya jika memakai login Google; kosongkan bila tidak  |
| `EMAIL_PROVIDER`      | `resend` (disarankan produksi) atau `smtp`; kosong = `smtp`|
| `RESEND_API_KEY`      | API key Resend (bila `EMAIL_PROVIDER=resend`) — rahasia    |

Bangkitkan rahasia cepat:

```bash
echo "JWT_SECRET=$(openssl rand -hex 32)"
echo "DB_PASSWORD=$(openssl rand -hex 24)"
echo "MINIO_ROOT_PASSWORD=$(openssl rand -hex 24)"
```

> `.env.prod` sudah masuk `.gitignore` — jangan pernah commit file ini.

### Email (reset password, notifikasi, ganti email)

Backend mengirim email transaksional (tautan reset password, pemberitahuan ganti
password/email) lewat `EMAIL_PROVIDER`:

- **`resend` (disarankan produksi)** — Resend HTTP API (`POST api.resend.com/emails`).
  Set `RESEND_API_KEY` (rahasia, jangan di-commit) dan `SMTP_FROM` ke alamat pengirim
  pada domain yang **sudah diverifikasi** di Resend. Tahan egress ketat (hanya HTTPS
  keluar), tanpa relay SMTP. Bila `RESEND_API_KEY` kosong, sender jatuh ke mode log.
- **`smtp`** — relay SMTP mana pun (`SMTP_HOST`/`SMTP_PORT`/`SMTP_USERNAME`/
  `SMTP_PASSWORD`/`SMTP_TLS`). Dipakai dev/e2e via Mailpit (`host=mailpit:1025`).
- `MAIL_ENABLED=false` (atau host/kunci kosong) memakai **log-only sender** — email
  hanya dicatat ke log, tidak dikirim (aman untuk dev tanpa relay).

---

## 7. Build & jalankan

```bash
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --build
```

Yang terjadi berurutan: build image backend & frontend → Postgres/Redis/MinIO
naik → `migrate` menjalankan migrasi DB → backend & frontend start (backend
membuat bucket MinIO otomatis) → Caddy meminta sertifikat TLS.

Pantau:

```bash
docker compose -f docker-compose.prod.yml ps
docker compose -f docker-compose.prod.yml logs -f caddy      # cek TLS terbit
docker compose -f docker-compose.prod.yml logs -f backend
```

Build pertama bisa 5–15 menit (kompilasi Go + build Nuxt). Wajar bila lambat di
2 vCPU.

---

## 8. Seed akun admin pertama

Image backend hanya berisi binary API, jadi `createadmin` dijalankan lewat
container Go sekali-pakai yang join ke jaringan `inventra-net`:

```bash
docker run --rm --network inventra-net \
  -v "$PWD/backend:/src" -w /src \
  -e DB_HOST=postgres -e DB_PORT=5432 -e DB_USER=inventra \
  -e DB_PASSWORD="$(grep '^DB_PASSWORD=' .env.prod | cut -d= -f2-)" \
  -e DB_NAME=inventra -e DB_SSLMODE=disable \
  golang:1.25-alpine \
  go run ./cmd/createadmin -email admin@inventra.local -name "Admin" -password "GANTI-password-kuat"
```

Output sukses: `created superadmin user: id=... email=admin@inventra.local`.

> Ganti email & password. Password ini untuk login pertama; ganti dari dalam
> aplikasi setelah masuk.

---

## 9. Verifikasi

```bash
curl -fsS https://<DOMAIN>/health          # → 200 dari backend
```

Buka `https://<DOMAIN>` di browser, login dengan akun admin tadi. Cek gembok
HTTPS hijau (sertifikat Let's Encrypt).

---

## 10. Update / redeploy

> **Otomatis?** Kalau auto-deploy (CD) sudah diaktifkan (§13), setiap merge ke
> `main` yang lolos CI akan otomatis ter-deploy — Anda tidak perlu menjalankan
> perintah di bawah secara manual. Bagian ini untuk redeploy manual / server
> tanpa CD.

```bash
cd ~/inventra
git pull
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --build
docker image prune -f      # bersihkan image lama
```

Migrasi DB baru otomatis dijalankan oleh service `migrate` tiap kali stack naik.

---

## 11. Backup

**Database (paling penting):**

```bash
# Backup manual ke file terkompresi
docker exec inventra-postgres pg_dump -U inventra -d inventra | gzip > \
  ~/backup-inventra-$(date +%F).sql.gz
```

Otomatiskan harian via cron (`crontab -e`):

```
0 2 * * * docker exec inventra-postgres pg_dump -U inventra -d inventra | gzip > ~/backups/inventra-$(date +\%F).sql.gz
```

**Restore:**

```bash
gunzip -c ~/backup-inventra-YYYY-MM-DD.sql.gz | \
  docker exec -i inventra-postgres psql -U inventra -d inventra
```

**File/lampiran (MinIO)** tersimpan di volume `inventra-minio`. Backup dengan
menyalin volume Docker atau memakai `mc mirror` ke storage lain.

---

## 12. Troubleshooting

**Build frontend gagal / "killed" / OOM.**
Swap belum aktif. Ulangi Langkah 3 (`free -h` harus menampilkan swap), lalu build
ulang. Alternatif: build image di komputer lokal, push ke registry, `pull` di
server (menghindari build di VPS sama sekali).

**Login gagal / "Network Error" di browser, padahal backend hidup.**
`NUXT_PUBLIC_API_BASE` harus URL publik (`https://<DOMAIN>/api/v1`), bukan
`localhost`. Nilai ini dipakai browser pengguna. Sudah diset di
`docker-compose.prod.yml`; pastikan `DOMAIN` di `.env.prod` benar, lalu:
```bash
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --force-recreate frontend
```
Jika masih memakai localhost setelah itu, rebuild frontend tanpa cache:
`docker compose -f docker-compose.prod.yml --env-file .env.prod build --no-cache frontend`.

**Sertifikat TLS tidak terbit (log Caddy error ACME).**
DNS A record belum menunjuk ke IP VPS, atau port 80/443 tertutup. Pastikan
`dig +short <DOMAIN>` mengembalikan IP VPS dan `ufw status` mengizinkan 80/443.

**CORS ditolak.**
`FRONTEND_URL` backend harus `https://<DOMAIN>` (sudah otomatis dari `DOMAIN`).

**Tanpa domain (hanya IP).**
HTTPS otomatis tidak bisa jalan dengan IP. Untuk uji cepat, ganti blok situs di
`ops/Caddyfile` menjadi `:80 { ... }` dan set `NUXT_PUBLIC_API_BASE` +
`FRONTEND_URL` ke `http://<IP>`. Ini **hanya untuk testing** — untuk produksi,
gunakan domain agar dapat HTTPS.

---

## 13. WAF (Coraza + OWASP CRS)

Reverse-proxy Caddy menjalankan WAF Coraza dengan OWASP CRS (image Caddy kustom
di `ops/caddy/`). Mode diatur oleh `SecRuleEngine` di `ops/caddy/Caddyfile`:
`DetectionOnly` (mencatat) atau `On` (memblokir, default produksi).

> **Deploy pertama kali ke environment baru:** sebelum mengaktifkan blocking,
> set dulu `SecRuleEngine DetectionOnly` di `ops/caddy/Caddyfile`, deploy, lalu
> jalankan alur nyata (login, buat/edit aset, upload lampiran, export) selagi
> memantau `docker compose -f docker-compose.prod.yml logs caddy | grep -i coraza`
> untuk menemukan rule id yang terpicu pada request yang sah. Tambahkan
> exclusion yang diperlukan ke `ops/caddy/coraza-exclusions.conf`, baru setelah
> itu set `SecRuleEngine On` dan redeploy. File exclusions yang masih kosong +
> langsung `On` berisiko men-403 alur sah (login, upload multipart ke MinIO,
> body JSON) tanpa ada jendela tuning.

**Tuning false-positive** — bila alur sah terblokir (mis. upload lampiran):
1. `docker compose -f docker-compose.prod.yml logs caddy | grep -i coraza` untuk
   menemukan rule id yang terpicu.
2. Tambahkan exclusion di `ops/caddy/coraza-exclusions.conf`
   (mis. `SecRuleRemoveById <id>`), lalu redeploy.

**Uji WAF lokal (tanpa menyentuh produksi):**
```bash
docker compose -f ops/caddy/test/docker-compose.test.yml up -d --build
ops/waf-smoketest.sh http://localhost:18080
docker compose -f ops/caddy/test/docker-compose.test.yml down
```

---

## 14. Auto-deploy (CD) via GitHub Actions

Alur setelah diaktifkan:

```
merge ke main ─▶ CI (test/lint/e2e) ─▶ workflow "Deploy":
                                          build image backend+frontend
                                          push ke GHCR
                                          SSH ke VPS ─▶ git pull ─▶ compose pull ─▶ up -d
                                        ─▶ live (otomatis)
```

Build berat (Nuxt) terjadi di runner GitHub (7 GB RAM), **bukan** di VPS. VPS
hanya `docker pull` — tanpa build, tanpa swap. Definisi pipeline ada di
[`.github/workflows/deploy.yml`](../.github/workflows/deploy.yml); ia dipicu
`workflow_run` sehingga **hanya jalan bila workflow CI sukses** di `main`.

### Setup satu kali

**a. Buat SSH key khusus CI** (di komputer lokal atau VPS):
```bash
ssh-keygen -t ed25519 -C "github-actions-deploy" -f ~/.ssh/gha_deploy -N ""
```
Tambahkan **public key** ke daftar yang diizinkan login di VPS (user `deploy`):
```bash
# jalankan di VPS, tempel isi gha_deploy.pub
echo "ssh-ed25519 AAAA...isi-public-key... github-actions-deploy" >> ~/.ssh/authorized_keys
```

**b. Tambah GitHub Secrets** (repo → Settings → Secrets and variables → Actions → New repository secret):

| Secret | Isi |
| --- | --- |
| `VPS_HOST` | IP publik VPS (atau `inventra.ragilbuaj.web.id`) |
| `VPS_USER` | `deploy` |
| `VPS_SSH_KEY` | **isi lengkap private key** `~/.ssh/gha_deploy` (termasuk baris `-----BEGIN/END-----`) |
| `VPS_PORT` | *(opsional; default 22)* |

**c. Jadikan image GHCR publik** (agar VPS bisa `pull` tanpa login). Setelah
workflow Deploy sukses pertama kali, image muncul di tab **Packages** akun Anda.
Untuk masing-masing (`inventra-backend`, `inventra-frontend`): buka package →
**Package settings** → **Change visibility** → **Public**.
> Alternatif (kalau ingin tetap privat): jalankan `docker login ghcr.io` di VPS
> dengan PAT ber-scope `read:packages`.

**d. Pastikan `~/inventra` di VPS** adalah checkout `main` yang bersih dan berisi
`.env.prod`. Karena repo publik, `git pull` tidak butuh autentikasi.

### Menjalankan & memantau

- **Otomatis**: merge/push ke `main` → tunggu CI hijau → Deploy jalan sendiri.
- **Manual**: tab **Actions → Deploy → Run workflow** (memakai `workflow_dispatch`).
- Pantau progres di tab **Actions**; verifikasi hasil dengan
  `curl -fsS https://<DOMAIN>/health`.

### Rollback

Selain `:latest`, tiap build diberi tag **commit SHA**
(`ghcr.io/ragbuaj/inventra-backend:<sha>`), jadi versi lama selalu tersedia untuk
dikembalikan. Cara termudah di VPS — tarik image versi lama lalu jalankan sebagai
`latest`:
```bash
OLD=<sha-commit-lama>
docker pull ghcr.io/ragbuaj/inventra-backend:$OLD
docker pull ghcr.io/ragbuaj/inventra-frontend:$OLD
docker tag ghcr.io/ragbuaj/inventra-backend:$OLD  ghcr.io/ragbuaj/inventra-backend:latest
docker tag ghcr.io/ragbuaj/inventra-frontend:$OLD ghcr.io/ragbuaj/inventra-frontend:latest
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d
```
Bila rollback juga menyangkut perubahan migrasi/compose, lakukan
`git checkout <commit-lama>` di `~/inventra` lebih dulu, lalu jalankan langkah di atas.

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

Role `monitoring` (langkah §16 di bawah) menyusul role `app` di `site.yml` dan
menaikkan overlay observability dengan cara yang sama (`docker_compose_v2`,
`state: present`) — file rahasia overlay (`alertmanager.yml`, `grafana.env`)
harus sudah disiapkan di server sebelum menjalankan playbook (lihat §16).

---

## 16. Monitoring & Observability

Stack observability adalah overlay toggleable (`docker-compose.monitoring.yml`):
Prometheus (metrics, retensi 15d) + exporters (node, cAdvisor, postgres, redis,
blackbox) + Alertmanager (alert → Telegram) + Loki+Promtail (log) + Grafana
(dashboard). Backend sendiri sudah terinstrumentasi RED metrics di `/metrics`
(internal-only, tidak diekspos publik).

```bash
cd ~/inventra
cp ops/monitoring/alertmanager/alertmanager.example.yml ops/monitoring/alertmanager/alertmanager.yml   # isi bot_token + chat_id
cp ops/monitoring/grafana.env.example ops/monitoring/grafana.env                                        # isi password admin + GF_SERVER_ROOT_URL
docker compose -f docker-compose.prod.yml -f docker-compose.monitoring.yml --env-file .env.prod up -d
```

- Tambahkan DNS A record `monitoring.<domain>` → IP VPS; Grafana ada di `https://monitoring.<domain>` (login admin dari grafana.env).
- Hanya Grafana yang publik; Prometheus/Alertmanager/exporters internal-only.
- Alert dikirim ke Telegram via Alertmanager. Validasi config lokal: `ops/monitoring/verify.sh`.
- Via Ansible: role `monitoring` (`ops/ansible/roles/monitoring/`) menjalankan langkah `docker compose up`
  di atas secara idempotent sebagai bagian dari `site.yml` — siapkan `alertmanager.yml`/`grafana.env`
  di server **sebelum** menjalankan playbook, karena role tidak merender rahasia overlay ini (berbeda
  dari `.env.prod`, yang di-render role `app` dari Vault). Target blackbox di `prometheus.yml` sudah
  di-hardcode ke domain publik (lihat komentar di file) — tidak perlu sed di deploy manapun.

---

## Referensi perintah cepat

```bash
# Status & log
docker compose -f docker-compose.prod.yml ps
docker compose -f docker-compose.prod.yml logs -f <service>

# Restart satu service
docker compose -f docker-compose.prod.yml --env-file .env.prod restart backend

# Matikan semua (data tetap di volume)
docker compose -f docker-compose.prod.yml down

# Matikan + hapus data (HATI-HATI: menghapus DB/MinIO)
docker compose -f docker-compose.prod.yml down -v
```
