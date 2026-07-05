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
