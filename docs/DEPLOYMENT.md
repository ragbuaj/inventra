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

Bangkitkan rahasia cepat:

```bash
echo "JWT_SECRET=$(openssl rand -hex 32)"
echo "DB_PASSWORD=$(openssl rand -hex 24)"
echo "MINIO_ROOT_PASSWORD=$(openssl rand -hex 24)"
```

> `.env.prod` sudah masuk `.gitignore` — jangan pernah commit file ini.

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

## 13. Auto-deploy (CD) via GitHub Actions

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
