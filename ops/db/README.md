# Akses baca database produksi (MCP postgres)

Tujuan: membaca database produksi dari Claude Code (inspeksi skema, cek data,
analisis performa) **tanpa** membuka Postgres ke internet dan **tanpa** risiko
menulis ke data nyata.

Rangkaiannya: SSH tunnel ke VPS lalu server MCP `postgres-mcp` mode
`restricted` (read-only), memakai role database `inventra_ro` yang hanya punya
SELECT.

## 1. Sekali saja di VPS

Postgres kini terikat ke `127.0.0.1:5432` di host VPS (lihat
`docker-compose.prod.yml`) — loopback saja, tidak terjangkau dari internet.
Terapkan perubahan itu lalu buat role read-only:

```bash
# di VPS, dari direktori repo
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d postgres

RO_PASSWORD=$(openssl rand -hex 32)
docker exec -i inventra-postgres psql -U inventra -d inventra \
  -v ro_password="'$RO_PASSWORD'" < ops/db/mcp_readonly_role.sql
echo "$RO_PASSWORD"   # simpan di password manager
```

Pastikan firewall VPS tetap hanya membuka 22/80/443 (port 5432 tidak pernah
dipublikasikan ke antarmuka publik oleh binding di atas).

## 2. Sekali saja di mesin lokal

Simpan connection string sebagai environment variable (jangan ditulis ke file
repo — repo ini publik):

```powershell
setx INVENTRA_PROD_RO_URL "postgresql://inventra_ro:PASSWORD_RO@127.0.0.1:55432/inventra?sslmode=disable"
```

Buka sesi PowerShell baru setelah `setx` agar variabelnya terbaca.

`sslmode=disable` aman di sini karena koneksi tidak pernah keluar dari
terowongan SSH yang sudah terenkripsi.

## 3. Tiap kali mau dipakai: buka tunnel

```bash
ssh -N \
  -o ServerAliveInterval=30 -o ServerAliveCountMax=3 -o ExitOnForwardFailure=yes \
  -L 55432:127.0.0.1:5432 <user-ssh>@<IP-VPS>
```

Catatan: `<user-ssh>` adalah user login VPS yang sebenarnya, belum tentu
`deploy` (nama itu hanya default di `ops/ansible/group_vars/all/vars.yml` dan
baru terpakai bila playbook Ansible dijalankan). Kalau lupa, `ssh root@<IP-VPS>`
biasanya membalas dengan nama user yang benar.

Tiga opsi itu bukan hiasan: tanpa `ServerAlive*`, koneksi yang putus membuat
tunnel **beku** — port 55432 tetap mendengarkan tapi tiap koneksi menggantung
sampai timeout, sehingga `psql` tampak macet padahal kredensialnya benar. Dengan
opsi ini SSH keluar dalam ~90 detik. `ExitOnForwardFailure` membuat SSH gagal
terang-terangan bila port 55432 sudah terpakai, bukan diam-diam tersambung tanpa
forwarding.

Perintah `ssh -N` memang tidak mengeluarkan output apa pun dan tidak pernah
selesai — itu benar, bukan hang. Biarkan jendela itu terbuka dan kerjakan sisanya
di terminal lain.

Uji dari terminal lain:

```bash
# bash / Git Bash
psql "$INVENTRA_PROD_RO_URL" -c "select count(*) from asset.assets;"
```

```powershell
# PowerShell — WAJIB awalan $env:
psql $env:INVENTRA_PROD_RO_URL -c "select count(*) from asset.assets;"
```

Di PowerShell, `$INVENTRA_PROD_RO_URL` (tanpa `$env:`) adalah variabel
PowerShell yang tidak ada, jadi ia mengembang jadi string kosong dan `psql`
diam-diam menyambung ke Postgres **lokal** sebagai user Windows Anda — gejalanya
prompt password yang membingungkan, bukan error koneksi.

## 4. MCP-nya

Terdaftar di `.mcp.json` (di-gitignore, lokal saja) sebagai server
`postgres-prod`, dijalankan via `uvx postgres-mcp --access-mode=restricted`.
Mode `restricted` memaksa transaksi read-only plus statement timeout, jadi ada
dua lapis pengaman: mode MCP dan hak role database.

Restart Claude Code setelah menambah/mengubah `.mcp.json`.

## Catatan

- Tunnel mati atau tidak dibuka membuat MCP gagal connect — itu perilaku yang
  diharapkan, bukan bug.
- Untuk database dev lokal cukup pakai `localhost:5433` langsung tanpa tunnel.
