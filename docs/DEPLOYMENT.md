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

- Hanya **port 80 & 443** yang terbuka ke internet. Redis/MinIO hanya bisa
  diakses antar-container; Postgres juga terikat ke **loopback VPS**
  (`127.0.0.1:5432`) supaya admin bisa membacanya lewat **SSH tunnel** —
  tetap tertutup dari internet. Lihat [ops/db/README.md](../ops/db/README.md)
  untuk role read-only `inventra_ro` dan setup MCP postgres.
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
| `MAIL_ENABLED`        | `true` agar email benar-benar dikirim; `false` = log-only  |
| `EMAIL_PROVIDER`      | `resend` (disarankan produksi) atau `smtp`; kosong = `smtp`|
| `RESEND_API_KEY`      | API key Resend (bila `EMAIL_PROVIDER=resend`) — rahasia    |
| `SMTP_FROM`           | alamat pengirim pada domain yang sudah diverifikasi Resend |

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

Frontend adalah PWA, jadi ada empat pemeriksaan tambahan yang hanya bisa dilakukan
terhadap produksi sungguhan. Lihat bagian 17.

---

## 10. Update / redeploy

> **Otomatis?** Kalau auto-deploy (CD) sudah diaktifkan (bagian 14), setiap merge ke
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

> **Pengguna tidak langsung melihat versi baru.** Frontend adalah PWA dengan
> `registerType: 'prompt'`: begitu service worker terpasang di sebuah perangkat,
> navigasi dilayani dari precache, bukan dari jaringan. Setelah deploy, pengguna
> menerima ajakan "Versi baru tersedia" dan **harus menekan Muat ulang** untuk pindah.
> Ini disengaja (lihat bagian 17), bukan cache yang macet.

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
`localhost`. Nilai ini dipakai browser pengguna, dan sejak frontend menjadi PWA ia
**dibekukan saat image dibangun**, bukan saat container dijalankan: rute `/`
di-prerender jadi HTML statis supaya service worker bisa mem-precache shell
aplikasi, sehingga berkas itu sudah membawa nilainya. Karena itu `up -d
--force-recreate frontend` saja tidak akan mengubahnya — image-nya harus dibangun
ulang.

Image dari GHCR sudah membawa nilai produksi dari CD (`build-args` di
`.github/workflows/deploy.yml`, domain dari variabel repo `PROD_DOMAIN`). Kalau
domain produksi berubah, set variabel itu lalu jalankan ulang workflow Deploy.
Bila image dibangun langsung di VPS, pastikan `DOMAIN` di `.env.prod` benar lalu:
```bash
docker compose -f docker-compose.prod.yml --env-file .env.prod build --no-cache frontend
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d frontend
```
Setelah frontend diganti, minta pengguna memuat ulang sekali supaya service worker
mengambil shell yang baru.

**Sertifikat TLS tidak terbit (log Caddy error ACME).**
DNS A record belum menunjuk ke IP VPS, atau port 80/443 tertutup. Pastikan
`dig +short <DOMAIN>` mengembalikan IP VPS dan `ufw status` mengizinkan 80/443.

**CORS ditolak.**
`FRONTEND_URL` backend harus `https://<DOMAIN>` (sudah otomatis dari `DOMAIN`).

**Tanpa domain (hanya IP).**
HTTPS otomatis tidak bisa jalan dengan IP. Untuk uji cepat, ganti blok situs di
`ops/Caddyfile` menjadi `:80 { ... }`, bangun ulang image frontend dengan
`--build-arg NUXT_PUBLIC_API_BASE=http://<IP>/api/v1` (nilainya build-time, lihat
butir "Network Error" di atas), dan set `FRONTEND_URL` backend ke `http://<IP>`. Ini **hanya untuk testing** — untuk produksi,
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

**Rollback frontend dan service worker.** Image frontend lama membawa `sw.js` dengan
manifest precache yang berbeda, jadi perangkat pengguna memperlakukannya seperti
pembaruan biasa: ajakan "Versi baru tersedia" muncul lagi, dan yang mereka pindahi
justru versi lama. Efeknya benar, hanya kalimatnya yang terasa aneh bagi pengguna.
Perangkat yang mengabaikan ajakan itu tetap menjalankan versi yang di-rollback sampai
mereka memuat ulang. Kalau rollback dilakukan karena frontend rusak parah, lihat
"Kill switch" di bagian 17.

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

Role `monitoring` (langkah bagian 16 di bawah) menyusul role `app` di `site.yml` dan
menaikkan overlay observability dengan cara yang sama (`docker_compose_v2`,
`state: present`) — file rahasia overlay (`alertmanager.yml`, `grafana.env`)
harus sudah disiapkan di server sebelum menjalankan playbook (lihat bagian 16).

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
  dari `.env.prod`, yang di-render role `app` dari Vault).
- Target blackbox tidak lagi dikeraskan di `prometheus.yml`. Layanan sekali-jalan
  `prometheus-targets` merendernya dari `DOMAIN` di `.env.prod` — sumber yang sama yang dipakai
  Caddy dan `docker-compose.prod.yml` — ke berkas `file_sd` yang dibaca Prometheus. Compose
  gagal sebelum satu pun container naik kalau `DOMAIN` hilang. Berlaku sama untuk deploy manual
  maupun lewat Ansible, karena keduanya menjalankan compose dengan `--env-file .env.prod`.

---

## 17. PWA (service worker) — operasi & verifikasi

Sejak PR #148 frontend adalah PWA: dapat dipasang ke layar utama dan tetap menyajikan
shell aplikasi saat jaringan putus. Tiga hal berubah bagi operasi, dan ketiganya
menggigit di tempat yang tidak terduga kalau tidak diketahui.

Keputusan rancangannya ada di [ADR-0019](adr/0019-web-pwa.md); bagian ini hanya sisi
operasionalnya.

### 17.1 `NUXT_PUBLIC_API_BASE` dibekukan saat BUILD

Rute `/` di-prerender jadi HTML statis supaya service worker bisa mem-precache shell
aplikasi, dan berkas statis itu **sudah membawa nilai apiBase di dalamnya**. Karena itu:

- Mengubah env di `docker-compose.prod.yml` lalu `up -d --force-recreate frontend`
  **tidak berpengaruh**. Image-nya harus dibangun ulang.
- Image dari GHCR sudah membawa nilai produksi dari CD (`build-args` di
  `.github/workflows/deploy.yml`).
- Domainnya diambil dari variabel repo `PROD_DOMAIN` (diset 2026-09-03 ke
  `inventra.ragilbuaj.web.id`). Nilainya hostname telanjang — tanpa skema, tanpa garis
  miring di akhir — karena dipakai sebagai `https://<PROD_DOMAIN>/api/v1`.
- `deploy.yml` tetap memuat fallback yang dikeraskan supaya deploy tidak runtuh jadi
  `https:///api/v1` kalau variabelnya terhapus. Karena fallback semacam itu bisa jadi
  usang tanpa bersuara, tiap job CD mencatat nilai yang dibekukan **beserta sumbernya**
  ke log — periksa langkah "Catat API base yang akan dibekukan ke image". Kalau log
  berbunyi "fallback", variabelnya hilang dan harus diset ulang.
- Domain produksi hidup di **dua** tempat, dan pembagiannya struktural: satu untuk waktu
  build, satu untuk waktu jalan. Mengganti domain menuntut keduanya:
  1. **Build (GitHub):** `gh variable set PROD_DOMAIN --body <domain-baru>`, lalu jalankan
     ulang workflow Deploy supaya image frontend dibangun ulang dengan apiBase yang baru.
  2. **Runtime (VPS):** `DOMAIN` di `.env.prod`. Dari situ Caddy mengambil nama situs,
     `docker-compose.prod.yml` menyusun `FRONTEND_URL`, dan sejak monitoring memakai
     `file_sd`, target blackbox Prometheus ikut dirender dari sana juga.

  Keduanya tidak bisa disatukan lebih jauh tanpa membuat CD membaca konfigurasi dari VPS.
  Yang sudah dihilangkan adalah sumber ketiga: `prometheus.yml` tidak lagi mengeraskan domain.

Gejala kalau salah: pengguna melihat "Network Error" saat login padahal backend hidup.
Lihat butir pertama di bagian 12.

### 17.2 Pembaruan menunggu persetujuan pengguna

`registerType: 'prompt'`. Begitu service worker aktif di sebuah perangkat, **navigasi
dilayani dari precache**, bukan dari jaringan — jadi `cache-control: no-cache` pada HTML
tidak lagi menentukan apa yang dilihat pengguna terpasang. Alurnya:

1. Deploy menghasilkan `sw.js` baru. `/sw.js` disajikan `no-cache` (diverifikasi
   terhadap build nyata; berasal dari `routeRules` `/**` di `nuxt.config.ts`), jadi
   peramban memeriksanya saat halaman dimuat.
2. Service worker baru terpasang lalu **menunggu**. Pengguna melihat ajakan
   "Versi baru tersedia".
3. Saat pengguna menekan Muat ulang, worker baru aktif, `cleanupOutdatedCaches`
   menyapu precache lama, halaman dimuat ulang.

Konsekuensi operasional:

- **Deploy tidak serta-merta mengubah apa yang dijalankan pengguna.** Perangkat yang
  mengabaikan ajakan tetap di versi lama sampai mereka memuat ulang. Untuk perbaikan
  mendesak, sampaikan lewat kanal internal agar pengguna menekan Muat ulang.
- Muat ulang otomatis sengaja **tidak** dipakai: ia bisa membuang isian formulir aset
  yang sedang diketik.
- Tidak ada pemeriksaan pembaruan berkala (`periodicSyncForUpdates` tidak diaktifkan).
  Deteksi terjadi saat halaman dimuat.

### 17.3 Kill switch — melepas service worker yang rusak

Ini jebakan klasik PWA dan satu-satunya bagian di dokumen ini yang layak dibaca
**sebelum** dibutuhkan. Service worker yang rusak tetap hidup di perangkat pengguna
meski server sudah diperbaiki, karena ia yang melayani navigasi. Rollback image biasa
belum tentu menolong: perangkat baru mengambil `sw.js` pengganti kalau ia sempat
memeriksanya.

Kalau sebuah build menerbitkan service worker yang membuat aplikasi tidak bisa dipakai,
terbitkan `sw.js` yang melepas dirinya sendiri:

```javascript
// sw.js darurat — menggantikan yang rusak, lalu menghapus dirinya
self.addEventListener('install', () => self.skipWaiting())
self.addEventListener('activate', async () => {
  await self.registration.unregister()
  for (const key of await caches.keys()) await caches.delete(key)
  for (const client of await self.clients.matchAll()) client.navigate(client.url)
})
```

Sajikan berkas ini di `/sw.js` dengan `Cache-Control: no-store`, biarkan naik beberapa
jam sampai perangkat mengambilnya, baru deploy versi normal. Karena `/sw.js` sudah
disajikan `no-cache`, perangkat akan mengambilnya pada pemuatan halaman berikutnya.

**Tanya dulu sebelum menjalankan ini.** Ia menghapus seluruh Cache Storage di perangkat
pengguna dan membuat aplikasi kehilangan kemampuan luring sampai service worker normal
terpasang kembali.

### 17.4 Invarian keamanan yang harus dijaga

**Tidak ada satu pun aturan runtime caching.** Di produksi API se-origin dengan frontend
(`https://<DOMAIN>/api/v1`), jadi satu aturan runtime saja sudah cukup mengendapkan
respons API ke Cache Storage di tiap perangkat — dan kesalahan itu tidak akan pernah
terlihat saat dev, karena di dev API beda origin.

Dijaga tiga lapis, semuanya di CI: daftar-izin kunci konfigurasi
(`frontend/test/unit/pwa-workbox.spec.ts`), pemeriksaan artefak `sw.js` hasil build, dan
e2e yang membaca `caches` di peramban sungguhan lalu menuntut isinya tidak lebih dari
precache (`frontend/e2e/pwa.spec.ts`). Jangan melonggarkan ketiganya.

**Terbuka:** respons `/api/v1/*` belum menyetel `Cache-Control`, sehingga data aset masih
bisa mendarat di disk lewat cache HTTP peramban — pintu kedua yang tidak dijaga invarian
di atas. Dilacak di [isu #149](https://github.com/ragbuaj/inventra/issues/149).

### 17.5 Anggaran precache

Service worker mengunduh **seluruh** precache saat terpasang, pada kunjungan pertama
pengguna. Diukur pada build saat ini: 195 entri, 3,17 MiB di disk, sekitar **1,31 MiB
lewat kabel** (Caddy menyajikan `encode gzip zstd`; 2,31 MiB JS terkompresi jauh, 0,56 MiB
woff2 memang sudah terkompresi).

Beban sekali-pasang 1,3 MiB wajar untuk kemampuan luring, jadi precache tidak
dipersempit. E2E menjaga ambang 260 entri supaya pertumbuhannya terlihat. Reproduksi
angkanya dari `.output/public/sw.js` setelah `pnpm build`.

### 17.6 Verifikasi pasca-deploy (Tugas 11 — belum dijalankan)

Empat hal yang secara struktural tidak bisa dibuktikan sebelum ada produksi sungguhan:

| Pemeriksaan | Cara | Kalau gagal |
|---|---|---|
| `manifest.webmanifest` dan `sw.js` lolos WAF | `curl -fsS https://<DOMAIN>/manifest.webmanifest` dan `.../sw.js`, harus 200 bukan 403 | False positive OWASP CRS; perbaikannya di `ops/caddy/Caddyfile` dan itu **tanya dulu** sesuai batasan spec |
| Header `sw.js` masih `no-cache` setelah lewat Caddy | `curl -sI https://<DOMAIN>/sw.js` | Pembaruan telat terdeteksi. Sudah diverifikasi benar di lapisan Nitro (`cache-control: no-cache`, dari `routeRules` `/**`), jadi yang tersisa hanya memastikan Caddy tidak menimpanya |
| Lighthouse kategori PWA installable tanpa peringatan | DevTools Lighthouse terhadap domain produksi | Ikuti pesannya; biasanya ikon atau bidang manifest |
| Pemasangan nyata di Android dan iPhone | Chrome Android: ajakan pasang muncul lalu ikon mendarat di layar utama. Safari iOS: Bagikan lalu Tambahkan ke Layar Utama, lalu buka dari ikon dan pastikan konten tidak tertutup notch | Catat gejalanya balik ke `docs/PROGRESS.md` |

Hasil keempatnya dicatat balik ke `docs/PROGRESS.md`.

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
