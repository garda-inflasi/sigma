# Deployment — Garda Inflasi

Panduan lengkap CI/CD untuk monorepo **Garda Inflasi**.

Model deploy: **build di MacBook → save image → transfer via SSH → load & run di VM**.
VM **tidak pernah** build apa pun (`--no-build` wajib di semua `docker compose up`).

---

## Daftar Isi

1. [Arsitektur & Alur](#1-arsitektur--alur)
2. [Prasyarat](#2-prasyarat)
3. [Setup VM (sekali saja)](#3-setup-vm-sekali-saja)
4. [Setup MacBook (sekali saja)](#4-setup-macbook-sekali-saja)
5. [Konfigurasi `.env.deploy`](#5-konfigurasi-envdeploy)
6. [Layout direktori di VM](#6-layout-direktori-di-vm)
7. [Cara Pakai Ringkas](#7-cara-pakai-ringkas)
8. [Referensi Perintah](#8-referensi-perintah)
9. [Rollback](#9-rollback)
10. [Backup Database](#10-backup-database)
11. [Ganti Domain / SSL](#11-ganti-domain--ssl)
12. [Troubleshooting](#12-troubleshooting)
13. [Referensi Environment Variable](#13-referensi-environment-variable)

---

## 1. Arsitektur & Alur

```
┌──────────────────────────────┐         ┌──────────────────────────────────────┐
│  MacBook (builder)           │         │  VM (Ubuntu/Debian, Docker Engine)   │
│                              │         │                                      │
│  1. git rev-parse HEAD       │         │                                      │
│     └─> GIT_SHA=26a72a7      │         │                                      │
│                              │         │                                      │
│  2. docker buildx build      │         │                                      │
│     --platform linux/amd64   │         │                                      │
│     --load                   │         │                                      │
│     -t garda-api:26a72a7     │         │                                      │
│     -t garda-api:latest      │         │                                      │
│     (api, web, caddy)        │         │                                      │
│                              │         │                                      │
│  3. docker save | gzip -9    │         │                                      │
│     └─> dist/garda-images-   │         │                                      │
│         26a72a7.tar.gz       │         │                                      │
│                              │         │                                      │
│  4. rsync ───────────────────┼────────>│  ~/garda-inflasi/dist/               │
│     (tarball + compose +     │  SSH    │  ~/garda-inflasi/compose/            │
│      migrations + scripts)   │         │  ~/garda-inflasi/db/migrations/      │
│                              │         │                                      │
│  5. ssh 'bash -s' ───────────┼────────>│  6. gunzip -c | docker load          │
│     < remote-deploy.sh       │         │  7. docker network ensure            │
│                              │         │  8. compose up -d --no-build (infra) │
│                              │         │  9. wait healthcheck (2/2)           │
│                              │         │ 10. migrate + seed referensi (gagal  │
│                              │         │     keras: deploy berhenti)          │
│                              │         │ 11. compose up -d --no-build (app)   │
│                              │         │ 12. curl /healthz                    │
│                              │         │ 13. tulis .last-deploy               │
│ 14. healthcheck gagal?       │         │                                      │
│     └─> auto rollback        │<────────┤                                      │
└──────────────────────────────┘         └──────────────────────────────────────┘
```

**Prinsip kunci:**

| Prinsip | Implementasi |
|---|---|
| VM tidak build | `compose.prod.yaml` memakai `build: !reset null` untuk menghapus section `build:` warisan |
| Image reproducible | Tag `<git-sha>` + `latest`; working tree kotor ditolak kecuali `--allow-dirty` |
| Tidak ada registry | `docker save` + `docker load`; VM tidak butuh internet untuk pull image |
| Aman dari deploy ganda | Lock atomik `.deploy.lock` (mkdir) |
| Gagal → balik sendiri | Healthcheck gate + auto rollback |
| Migrasi nyata | Service `migrate` (image API berisi `./migrate` + `db/migrations`) jalan sebelum aplikasi naik; kegagalannya menghentikan deploy |
| Tanpa data demo | Service `seed` di produksi hanya `reference` dan `admin`; seeder `demo` menolak `APP_ENV=production` |
| Tanpa penangkap email | Service `mailpit` (dev/demo) berada di profil compose `mailpit` yang hanya dinyalakan `make docker/demo/up`; skrip deploy tidak pernah menyebutnya dan `SMTP_*` di `compose.yaml` kosong secara bawaan |
| Secret tidak bocor | `.env` **tidak pernah** ditransfer; `mask_secret` di semua log |

---

## 2. Prasyarat

### MacBook (builder)

| Tool | Versi minimum | Cek |
|---|---|---|
| Docker Desktop | 4.x dengan buildx | `docker buildx version` |
| `docker compose` | v2 plugin | `docker compose version` |
| `git` | 2.x | `git --version` |
| `rsync` | 3.x | `rsync --version` |
| `jq` | 1.6 | `jq --version` |
| `gzip` | — | `gzip --version` |

```bash
brew install rsync jq
```

### VM (target)

| Tool | Versi minimum | Cek |
|---|---|---|
| Docker Engine | 24.x | `docker version` |
| `docker compose` | v2 plugin | `docker compose version` |
| `curl` | — | `curl --version` |
| `gzip` | — | `gzip --version` |
| `jq` | opsional | `jq --version` |

> **Tidak perlu** Go, Node, pnpm, atau build tools di VM.
> `jq` opsional — `remote-deploy.sh` otomatis fallback ke `grep` bila tidak ada.

---

## 3. Setup VM (sekali saja)

### 3.1 Install Docker Engine + Compose v2

```bash
# Ubuntu / Debian
curl -fsSL https://get.docker.com | sudo sh
sudo systemctl enable --now docker

# Verifikasi (harus v2, BUKAN docker-compose v1)
docker compose version
```

### 3.2 Buat user `deploy`

```bash
sudo adduser --disabled-password --gecos "" deploy
sudo usermod -aG docker deploy
sudo mkdir -p /home/deploy/.ssh
sudo chmod 700 /home/deploy/.ssh
```

### 3.3 Pasang public key

Dari MacBook:

```bash
ssh-copy-id -i ~/.ssh/garda_vm.pub deploy@192.168.1.100
```

Atau manual — tempel isi `~/.ssh/garda_vm.pub` ke `/home/deploy/.ssh/authorized_keys` di VM:

```bash
sudo chmod 600 /home/deploy/.ssh/authorized_keys
sudo chown -R deploy:deploy /home/deploy/.ssh
```

### 3.4 Verifikasi login tanpa password

```bash
ssh -i ~/.ssh/garda_vm -p 22 deploy@192.168.1.100 'docker version --format "{{.Server.Version}}"'
```

### 3.5 Buat layout direktori

```bash
ssh -i ~/.ssh/garda_vm deploy@192.168.1.100 \
  'mkdir -p ~/garda-inflasi/{dist,compose,docker/postgres,db/migrations,backups,scripts/deploy,scripts/rollback,scripts/lib}'
```

### 3.6 Buat `.env` production di VM

> **PENTING:** `.env` **tidak pernah** ditransfer oleh script. Buat manual sekali di VM.

```bash
ssh -i ~/.ssh/garda_vm deploy@192.168.1.100 'cat > ~/garda-inflasi/.env' <<'ENVEOF'
# ── Postgres ────────────────────────────────────────────────────────────
POSTGRES_USER=garda
POSTGRES_PASSWORD=GANTI_DENGAN_HEX_ACAK
POSTGRES_DB=garda_inflasi
DB_USER=garda
DB_PASSWORD=GANTI_DENGAN_HEX_ACAK      # sama dengan POSTGRES_PASSWORD
DB_NAME=garda_inflasi

# ── Secrets (WAJIB diganti; deploy DITOLAK bila kosong/contoh/bawaan) ───
# Pakai hex agar aman di URL dan env: openssl rand -hex 24   (JWT_SECRET, passwords)
#                                     openssl rand -hex 16   (AES_KEY, TEPAT 32 karakter)
JWT_SECRET=GANTI_DENGAN_HEX_ACAK_MIN_32
AES_KEY=GANTI_DENGAN_HEX_32_KARAKTER   # menyegel rahasia MFA; migrate/seed/api memakai kunci yang sama

# ── App ─────────────────────────────────────────────────────────────────
# Origin publik aplikasi (proteksi CSRF); WAJIB bukan localhost di production.
APP_ALLOWED_ORIGINS=https://app.garda.id

# ── Caddy ───────────────────────────────────────────────────────────────
CADDY_HTTP_PORT=80
CADDY_HTTPS_PORT=443
# Domain publik + email ACME (Let's Encrypt). Tanpa ini Caddy memakai localhost + CA internal.
CADDY_SITE_ADDRESS=app.garda.id
CADDY_TLS=admin@garda.id

# ── Opsional ────────────────────────────────────────────────────────────
# COMPOSE_PROFILES=backup              # nyalakan container backup terjadwal (§10)
# JOB_DASHBOARD_REFRESH_INTERVAL=5m    # interval job worker (off = nonaktif)
# JOB_KPI_HITUNG_INTERVAL=1h
# JOB_AUTH_PEMBERSIHAN_INTERVAL=1h     # pembersihan baris auth kedaluwarsa (lihat tabel di bawah)
ENVEOF

ssh -i ~/.ssh/garda_vm deploy@192.168.1.100 'chmod 600 ~/garda-inflasi/.env'
```

Generate secret acak (hex, agar aman dalam URL DSN; `AES_KEY` harus tepat 32 karakter):

```bash
openssl rand -hex 24   # JWT_SECRET, POSTGRES_PASSWORD/DB_PASSWORD
openssl rand -hex 16   # AES_KEY
```

`APP_ENV=production` dipasang compose.prod.yaml. Dalam mode itu API menolak start bila
`JWT_SECRET`/`AES_KEY` bawaan atau lemah, `DB_PASSWORD` bawaan, atau `APP_ALLOWED_ORIGINS`
hanya localhost; `remote-deploy.sh` memeriksa hal yang sama sebelum memuat image.

---

## 4. Setup MacBook (sekali saja)

```bash
# 1. Tool pendukung
brew install rsync jq

# 2. SSH key (ed25519, tanpa passphrase agar non-interaktif)
ssh-keygen -t ed25519 -C "garda-deploy" -f ~/.ssh/garda_vm -N ""

# 3. Permission WAJIB 600 — script menolak key dengan mode lain
chmod 600 ~/.ssh/garda_vm

# 4. Template konfigurasi deploy
cp .env.deploy.example .env.deploy
chmod 600 .env.deploy

# 5. Edit sesuai VM kamu
$EDITOR .env.deploy
```

---

## 5. Konfigurasi `.env.deploy`

File ini **git-ignored** dan **tidak pernah** dikirim ke VM.

```bash
# ==== VM TARGET ====
VM_HOST=192.168.1.100          # IP atau hostname VM
VM_USER=deploy                 # user SSH
SSH_KEY_PATH=~/.ssh/garda_vm   # private key (mode 600)
SSH_PORT=22
REMOTE_DIR=/home/deploy/garda-inflasi

# ==== BUILD ====
TARGET_PLATFORM=linux/amd64    # ganti linux/arm64 untuk VM ARM
IMAGE_TAG_PREFIX=garda
KEEP_LAST_N_IMAGES=3
BUILDX_CACHE_DIR=.buildx-cache

# ==== APP BUILD ARGS ====
NEXT_PUBLIC_API_BASE_URL=/api  # WAJIB relatif — jangan absolut
NEXT_PUBLIC_APP_URL=https://app.garda.id

# ==== RUNTIME ====
PUBLIC_HOST=app.garda.id       # untuk healthcheck di `make status`
```

> **Catatan `NEXT_PUBLIC_API_BASE_URL`:** harus **relatif** (`/api`).
> URL absolut membuat `http://host` dan `https://host` menjadi origin berbeda
> dan memicu CORS error di browser.

---

## 6. Layout Direktori di VM

```
~/garda-inflasi/
├── .env                      # ← dibuat manual, TIDAK ditransfer
├── .last-deploy              # SHA yang sedang berjalan
├── .previous-deploy          # SHA untuk rollback
├── .last-deploy-time         # timestamp deploy terakhir (UTC)
├── dist/
│   └── garda-images-<sha>.tar.gz
├── compose/
│   ├── compose.infra.yaml
│   ├── compose.yaml
│   └── compose.prod.yaml
├── docker/
│   └── postgres/
│       └── init.sql
├── db/
│   └── migrations/
├── scripts/
│   ├── lib/common.sh
│   ├── deploy/remote-deploy.sh
│   └── rollback/keep-last-n-images.sh
└── backups/
    └── 2026-10-16-0900.sql.gz
```

---

## 7. Cara Pakai Ringkas

```bash
# 1. Setup VM (sekali) — lihat §3
#    install Docker, buat user deploy, pasang public key, buat .env

# 2. Setup Mac (sekali) — lihat §4
brew install rsync jq
ssh-keygen -t ed25519 -f ~/.ssh/garda_vm -N ""
chmod 600 ~/.ssh/garda_vm

# 3. Isi konfigurasi
make deploy/init-env
$EDITOR .env.deploy

# 4. Deploy
make deploy

# 5. Verifikasi
make status
curl -fsS https://app.garda.id/healthz
```

---

## 8. Referensi Perintah

| Perintah | Fungsi |
|---|---|
| `make deploy` | Full: build → save → transfer → up |
| `make deploy ARGS=--skip-build` | Pakai image lokal yang sudah ada |
| `make deploy ARGS=--skip-transfer` | Tarball sudah ada di VM |
| `make deploy ARGS=--only=api` | Hanya build & deploy service `api` |
| `make deploy ARGS=--dry-run` | Print perintah, **tidak menyentuh VM** |
| `make deploy ARGS=--allow-dirty` | Izinkan build dari working tree kotor |
| `make deploy ARGS=--backup` | `pg_dump` sebelum deploy |
| `make deploy ARGS=--no-rollback` | Matikan auto-rollback saat gagal |
| `make deploy/build` | Build image saja |
| `make deploy/save` | `docker save` → `dist/*.tar.gz` |
| `make deploy/transfer` | rsync artefak ke VM |
| `make deploy/remote` | Jalankan deploy di VM saja |
| `make deploy/prune` | Hapus image & tarball lama |
| `make rollback` | Balik ke deploy sebelumnya |
| `make rollback ARGS=--to=<sha>` | Balik ke SHA tertentu |
| `make status` | `compose ps` + `docker stats` + disk + health |
| `make logs SERVICE=api TAIL=200` | Tail log service |
| `make clean` | Hapus `dist/`, `.buildx-cache/`, `.deploy.lock` |

### Contoh alur bertahap

```bash
# Build saja (debug Dockerfile)
make deploy/build ARGS="--only=api --no-cache"

# Lihat apa yang akan dilakukan tanpa eksekusi
make deploy ARGS=--dry-run

# Deploy tanpa build ulang (image sudah ada)
make deploy ARGS=--skip-build

# Deploy dengan backup DB dulu
make deploy ARGS=--backup
```

---

## 9. Rollback

### Otomatis

Kalau `remote-deploy.sh` exit non-zero (healthcheck gagal, image korup, dsb),
`deploy.sh` **otomatis** memanggil `rollback.sh --auto`.

### Manual

```bash
# Balik ke deploy sebelumnya
make rollback

# Balik ke SHA tertentu
make rollback ARGS=--to=26a72a7

# Lihat versi yang tersedia
make status
```

### Cara kerja

1. Baca `.last-deploy` (sekarang) dan `.previous-deploy` (target).
2. Verifikasi ketiga image target ada di VM.
3. `docker tag garda-<svc>:<target> garda-<svc>:latest`.
4. `IMAGE_TAG=<target> docker compose up -d --no-build`.
5. Healthcheck HTTP.
6. Tukar marker: `.last-deploy` ← target, `.previous-deploy` ← SHA lama.

### Rollback darurat (tanpa script)

```bash
ssh -i ~/.ssh/garda_vm deploy@192.168.1.100
cd ~/garda-inflasi
PREV=$(cat .previous-deploy)
IMAGE_TAG=$PREV docker compose -f compose/compose.infra.yaml -f compose/compose.prod.yaml up -d --no-build
IMAGE_TAG=$PREV docker compose -f compose/compose.yaml -f compose/compose.prod.yaml up -d --no-build
echo "$PREV" > .last-deploy
```

---

## 10. Backup Database

Skrip di [`scripts/backup/`](../scripts/backup) (POSIX sh, dipakai juga oleh container backup):

| Skrip | Fungsi |
| --- | --- |
| `backup.sh` | `pg_dump` format custom (`-Fc`) + `.sha256` + `.manifest` (versi goose dan jumlah baris tiap tabel sebelum/sesudah dump) + retensi |
| `restore.sh` | memulihkan dump ke database **baru** (checksum diperiksa dulu; menolak menimpa database hidup tanpa konfirmasi eksplisit) |
| `verify-restore.sh` | backup baru → restore ke database sementara `garda_verify_*` → bandingkan goose version dan jumlah baris semua tabel → hapus database sementara |

Semua berjalan lewat `docker exec` ke container `garda-postgres` (ganti dengan `PG_CONTAINER`), atau langsung bila `PGHOST` diisi (mode container backup).

```bash
make db/backup                                   # ./backups/garda_inflasi-<UTC>.dump (+ .sha256, .manifest)
make db/backup/verify                            # bukti backup bisa dipulihkan (RTO terukur dicetak)
make db/backup/verify FILE=backups/x.dump        # verifikasi dump yang sudah ada
make db/restore FILE=backups/x.dump TARGET=garda_pulih   # restore ke database BARU
```

Variabel: `BACKUP_DIR` (`./backups`), `BACKUP_RETENTION_DAYS` (14), `BACKUP_KEEP_MIN` (3 dump terbaru tidak pernah dihapus), `BACKUP_MANIFEST=0` (lewati manifest bila tabel sangat besar), `VERIFY_TABLES` (batasi tabel yang dibandingkan), `VERIFY_KEEP_DB=1` (simpan database sementara untuk diselidiki). Pengguna DB perlu hak `CREATEDB` (dan superuser agar ekstensi `uuid-ossp`/`pgcrypto` terbentuk); pada produksi dengan pengguna aplikasi berhak minimum, jalankan verifikasi dengan `PGUSER` admin.

### Backup terjadwal

Service opsional `backup` (profile `backup`) di `compose.infra.yaml` menjalankan `backup.sh` tiap `BACKUP_INTERVAL_SECONDS` (bawaan 86400 = harian) ke `${BACKUP_HOST_DIR:-./backups}`. Healthcheck-nya gagal bila tidak ada dump yang lebih baru dari 2x interval.

```bash
make docker/backup/up                                   # lokal
# di VM: tambahkan COMPOSE_PROFILES=backup ke ~/garda-inflasi/.env, lalu deploy seperti biasa
make deploy ARGS=--backup                               # backup sekali sebelum deploy (memakai backup.sh yang sama)
```

### Restore

```bash
# 1. Pulihkan ke database baru, periksa isinya
make db/restore FILE=backups/garda_inflasi-20261105T071503Z.dump TARGET=garda_pulih
# 2. Bila memang harus menimpa database hidup: hentikan api/worker/web, ambil backup baru, lalu
GARDA_RESTORE_CONFIRM=garda_inflasi sh scripts/backup/restore.sh backups/<dump> --target-db garda_inflasi
```

### RPO dan RTO (sederhana)

- **RPO** = interval backup. Bawaan harian, jadi kehilangan data maksimum sampai 24 jam (turunkan `BACKUP_INTERVAL_SECONDS` untuk RPO lebih kecil; di bawah itu butuh WAL archiving/PITR, belum disediakan).
- **RTO** = waktu restore + restart aplikasi. Terukur pada data demo (database ~880 KB terkompresi, 16 ribu baris `entri_harga`): restore ke database kosong **1 detik**, seluruh `verify-restore` 2-3 detik. Pada produksi RTO tumbuh kira-kira linear dengan ukuran data; ukur ulang dengan `make db/backup/verify` setelah data nyata masuk dan catat angkanya di sini.
- Salinan backup **harus** dikopi ke luar VM (mis. `rsync` ke penyimpanan lain); backup di disk yang sama tidak melindungi dari kerusakan disk/VM. Ini belum diotomatiskan.
- Jadwalkan `make db/backup/verify` berkala (mis. mingguan) dan simpan hasilnya; backup yang tidak pernah diuji dianggap tidak ada.

### Catatan hasil uji (2026-11-05, Postgres 16.11, demo `garda-postgres`)

`make db/backup/verify` terhadap `garda_inflasi` yang sedang melayani demo: goose version 51 cocok, 42 tabel/matview cocok baris-per-baris (mis. `entri_harga` 16.142, `agregat_harga_harian` 14.559), database sementara dihapus, `garda_inflasi` tidak disentuh. Jalur negatif juga diuji: checksum rusak ditolak, manifest yang dimanipulasi menggagalkan verifikasi (database sementara tetap dihapus), restore ke database hidup tanpa konfirmasi ditolak, retensi menyisakan 3 dump terbaru dan hanya menghapus dump yang lebih tua dari 14 hari.

---

## 11. Ganti Domain / SSL

### 11.1 Domain

Edit `docker/Caddyfile`:

```caddy
app.garda.id {
    tls admin@example.com      # Let's Encrypt otomatis
    # ... blok handle yang sudah ada
}
```

Lalu:

```bash
# 1. Arahkan DNS A record app.garda.id → IP VM
# 2. Pastikan port 80 & 443 terbuka di firewall VM
# 3. Update .env.deploy
#    NEXT_PUBLIC_APP_URL=https://app.garda.id
#    PUBLIC_HOST=app.garda.id
# 4. Rebuild & deploy (Caddyfile di-bake ke image)
make deploy ARGS=--only=caddy
```

### 11.2 SSL

| Mode | Konfigurasi | Kapan dipakai |
|---|---|---|
| Let's Encrypt | `tls admin@example.com` | Domain publik, port 80/443 terbuka |
| Internal CA | `tls internal` | Development / `localhost` |
| Cert manual | `tls /path/cert.pem /path/key.pem` | Sertifikat dari CA perusahaan |

> **Penting:** HSTS hanya dikirim lewat HTTPS. Jangan aktifkan HSTS saat
> masih mengakses via HTTP — browser akan mengunci diri ke HTTPS dan login gagal.

### 11.3 Firewall

```bash
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

---

## 12. Troubleshooting

| # | Error | Penyebab | Solusi |
|---|---|---|---|
| 1 | `Permission denied (publickey)` | Public key belum ada di VM, atau key salah | `ssh-copy-id -i ~/.ssh/garda_vm.pub deploy@VM_HOST`; cek `~/.ssh/authorized_keys` di VM |
| 2 | `SSH private key ... permission 644, harus 600` | Mode file key terlalu terbuka | `chmod 600 ~/.ssh/garda_vm` |
| 3 | `SSH private key tidak ditemukan` | `SSH_KEY_PATH` salah / `~` tidak ter-expand | Pakai path absolut, atau pastikan `~/.ssh/...`; cek `.env.deploy` |
| 4 | `Konfigurasi deploy tidak lengkap (N field hilang)` | Field wajib kosong di `.env.deploy` | Script mencetak field yang hilang — isi semuanya |
| 5 | `Disk lokal kurang dari 5 GB` | `dist/` hampir penuh | `make clean`; hapus tarball lama: `make deploy/prune` |
| 6 | `Disk VM kurang dari 2x ukuran tarball` | VM kehabisan ruang | `ssh VM 'docker system prune -af'`; hapus `~/garda-inflasi/dist/*.tar.gz` lama |
| 7 | `no space left on device` saat `docker load` | `/var/lib/docker` penuh | `docker system prune -af --volumes` di VM; pertimbangkan resize disk |
| 8 | `network garda_inflasi_net not found` | Network belum dibuat | `remote-deploy.sh` otomatis membuatnya; manual: `docker network create --driver bridge --attachable garda_inflasi_net` |
| 9 | `image ... not found` saat `up` | Tarball belum di-load / SHA salah | Cek `docker images \| grep garda`; ulangi `make deploy` (tanpa `--skip-transfer`) |
| 10 | `exec format error` / container restart loop | Platform mismatch (build arm64, VM amd64) | Set `TARGET_PLATFORM=linux/amd64` di `.env.deploy`, rebuild |
| 11 | `HEALTHCHECK FAILED` setelah `up` | Service tidak siap / config salah | `make logs SERVICE=api`; cek `.env` di VM; auto-rollback sudah jalan |
| 12 | `Healthcheck infra gagal (1/2 healthy)` | Postgres/Redis tidak start | `make logs SERVICE=postgres`; cek kredensial `.env`; cek volume korup |
| 13 | `docker-compose: command not found` | VM punya Compose v1 | Install plugin v2: `sudo apt install docker-compose-plugin`; script hanya pakai `docker compose` |
| 14 | `Deploy lain sedang berjalan (lock: ...)` | Deploy sebelumnya crash / masih jalan | Tunggu; kalau yakin tidak ada proses: `rm -rf .deploy.lock` |
| 15 | `Build dari working tree kotor` | Ada perubahan belum di-commit | `git commit` dulu, atau `make deploy ARGS=--allow-dirty` |
| 16 | `Wajib ada .env di VM` | `.env` production belum dibuat | Lihat §3.6 — buat manual di VM (tidak pernah ditransfer) |
| 17 | `services.<nama>.build must be a string` | `compose.prod.yaml` memakai `build: null` | Gunakan `build: !reset null` (sudah diperbaiki di repo ini) |
| 18 | `Tarball korup / ukuran tidak cocok` | Transfer terputus | Ulangi `make deploy/transfer`; rsync `--partial` akan melanjutkan |
| 19 | `jq: command not found` di VM | `jq` tidak terinstall | Opsional — script fallback ke `grep`. Atau `sudo apt install jq` |
| 20 | Login gagal / redirect loop setelah deploy | HSTS aktif di HTTP, atau `NEXT_PUBLIC_API_BASE_URL` absolut | Pastikan akses via HTTPS; set `NEXT_PUBLIC_API_BASE_URL=/api` (relatif) |
| 21 | `CORS error` di browser | Origin tidak cocok (http vs https) | Set `APP_ALLOWED_ORIGINS` di `.env` VM; pakai URL relatif di frontend |
| 22 | `502 Bad Gateway` dari Caddy | Container `api`/`web` belum siap | `make logs SERVICE=web`; tunggu healthcheck; cek `docker compose ps` |

---

## 12b. Hardening produksi

Sudah ditegakkan (ada tes atau diuji dengan `docker compose config`):

- **Rahasia:** API dan worker menolak start bila `APP_ENV=production` dan `JWT_SECRET`/`AES_KEY`/`DB_PASSWORD` bawaan, pendek, atau berpola contoh; `APP_ALLOWED_ORIGINS` wajib origin publik (`internal/platform/prodcheck`). `remote-deploy.sh` menolak sebelum memuat image. `compose.prod.yaml` memakai `${VAR:?}` sehingga variabel kosong menghentikan compose, bukan jatuh ke bawaan dev.
- **Cookie:** `COOKIE_SECURE=true` di `compose.prod.yaml`; sesi demo lewat cookie `garda_dev_role` dimatikan saat `NODE_ENV=production` (sebelumnya selalu aktif).
- **Jaringan:** Postgres dan Redis hanya di `127.0.0.1` VM pada produksi (sebelumnya dipublikasikan ke semua antarmuka, Redis tanpa sandi). Akses admin lewat SSH tunnel.
- **TLS dan domain:** `CADDY_SITE_ADDRESS` + `CADDY_TLS` (email ACME) mengaktifkan Let's Encrypt otomatis tanpa mengubah image; bawaan tetap `localhost` + CA internal. HSTS hanya untuk HTTPS (Caddy).
- **Header keamanan:** `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Permissions-Policy`, `Cross-Origin-Opener-Policy` dari Caddy; middleware Next.js menambah header yang sama di semua respons; Go memasang CSP dan header keamanan.
- **Healthcheck:** api, web, caddy, worker (`:8081/healthz`), backup (umur dump terakhir). `remote-deploy.sh` memeriksa lewat HTTPS dengan `--resolve` bila domain diset.
- **Perbaikan skrip:** `up` pada stack infra/app kini menyebut nama service karena `compose.prod.yaml` dipakai bersama dan memuat stub service stack lain (sebelumnya `up -d` tanpa nama menaikkan `api`/`postgres` ganda di stack yang salah); `rollback.sh`, `status.sh`, `logs.sh` memakai `--project-directory` agar membaca `.env` VM (sebelumnya rollback bisa jatuh ke rahasia bawaan dev).

Belum / perlu keputusan:

- Swagger sengaja mati saat `GIN_MODE=release`; Caddy tetap memproksi `/swagger/*` (menghasilkan 404 dari API).
- CSP ketat untuk halaman Next.js belum ada (butuh nonce); CSP yang ada hanya untuk respons API.
- Rate limit API di memori per instance (100 req/menit); belum terdistribusi lewat Redis.
- Pengguna DB aplikasi masih satu dengan pemilik skema ([19 §7](todos/19-migration-seeder.md)); pengguna migrasi terpisah dan pembatasan `audit_log` belum dibuat.
- Backup belum disalin ke luar VM; belum ada WAL/PITR. Pemantauan/alert (healthcheck gagal, backup gagal) belum ada.
- Tidak ada pemindaian image/dependensi di alur build; tidak ada rotasi rahasia otomatis (ganti `JWT_SECRET` memutus semua sesi).
- `middleware.ts` memakai konvensi Next.js lama (deprecated, diganti `proxy.ts` di Next 16); masih berfungsi.

---

## 13. Referensi Environment Variable

### `.env.deploy` (Mac, git-ignored)

| Variable | Wajib | Default | Keterangan |
|---|---|---|---|
| `VM_HOST` | ✅ | — | IP / hostname VM |
| `VM_USER` | ✅ | — | User SSH |
| `SSH_KEY_PATH` | ✅ | — | Private key, mode 600 |
| `SSH_PORT` | ✅ | — | Port SSH |
| `REMOTE_DIR` | ✅ | — | Direktori deploy di VM |
| `TARGET_PLATFORM` | ❌ | `linux/amd64` | Platform image |
| `IMAGE_TAG_PREFIX` | ❌ | `garda` | Prefix nama image |
| `KEEP_LAST_N_IMAGES` | ❌ | `3` | Jumlah versi disimpan |
| `BUILDX_CACHE_DIR` | ❌ | `.buildx-cache` | Cache buildx lokal |
| `NEXT_PUBLIC_API_BASE_URL` | ❌ | `/api` | **Harus relatif** |
| `NEXT_PUBLIC_APP_URL` | ❌ | — | URL publik app |
| `PUBLIC_HOST` | ❌ | — | Untuk healthcheck `make status` |
| `SSH_CONNECT_TIMEOUT` | ❌ | `10` | Timeout koneksi SSH (detik) |
| `SSH_ALIVE_INTERVAL` | ❌ | `30` | Keepalive interval |
| `SSH_ALIVE_COUNT_MAX` | ❌ | `3` | Keepalive gagal sebelum putus |
| `HEALTH_TIMEOUT_SECONDS` | ❌ | `60` | Timeout healthcheck infra |
| `HTTP_HEALTH_RETRIES` | ❌ | `10` | Retry healthcheck HTTP |
| `HTTP_HEALTH_DELAY` | ❌ | `3` | Delay antar retry (detik) |
| `MIN_FREE_GB` | ❌ | `5` | Minimum disk lokal saat save |
| `GARDA_LOCK_STALE_SECONDS` | ❌ | `7200` | Umur lock sebelum dianggap basi |

### `.env` (VM, dibuat manual)

Lihat §3.6 untuk template lengkap. Variable kunci:

| Variable | Keterangan |
|---|---|
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | Kredensial Postgres |
| `DB_USER` / `DB_PASSWORD` / `DB_NAME` | Koneksi aplikasi (DSN dirakit compose.prod.yaml) |
| `JWT_SECRET` | Signing key JWT (≥32 char acak); dipakai juga BFF Next.js (compose menyalinnya ke `GARDA_JWT_SECRET`) |
| `AES_KEY` | Enkripsi at-rest, termasuk rahasia MFA (tepat 32 char acak; kunci contoh ditolak). `migrate`, `seed`, dan `api` memakai kunci yang sama |
| `APP_ALLOWED_ORIGINS` | Origin publik untuk CSRF (wajib, bukan localhost) |
| `CADDY_SITE_ADDRESS` / `CADDY_TLS` | Domain publik dan email ACME |
| `COMPOSE_PROFILES` | `backup` untuk backup terjadwal |
| `JOB_<NAMA>_INTERVAL` | Interval job worker, `off` menonaktifkan |
| `JOB_AUTH_PEMBERSIHAN_INTERVAL` | Jeda job `auth_pembersihan` yang menghapus baris auth mati: `jwt_blacklist` dan `refresh_tokens` kedaluwarsa, `password_reset_tokens` kedaluwarsa/terpakai, `login_attempts` lama. Bawaan `1h`; `off` menonaktifkan. Tabel tumbuh terus tanpa job ini |
| `JOB_AUTH_PEMBERSIHAN_LOGIN_HARI` | Masa simpan `login_attempts` dalam hari (bawaan `90`, minimal `1`; `off` = tidak pernah dihapus). Baris yang lebih muda dari 1 hari tidak pernah dihapus karena dipakai penghitungan penguncian akun |
| `JOB_AUTH_PEMBERSIHAN_RESET_HARI` | Masa simpan token reset sesudah kedaluwarsa/terpakai (bawaan `7`; `0` = langsung). Selama masa ini tautan lama masih dijawab "sudah kedaluwarsa", bukan "tidak valid" |
| `JOB_AUTH_PEMBERSIHAN_BATCH` | Baris per pernyataan DELETE (bawaan `1000`, maks `100000`); satu putaran memproses paling banyak 500 batch per tabel |
| `APP_ENV` | `production` di server (dipasang compose.prod.yaml): menolak rahasia bawaan dan menyalakan penegakan MFA |
| `MFA_ENFORCE` | Menimpa penegakan MFA (`true`/`false`); kosong = ikut `APP_ENV` |
| `APP_PUBLIC_URL` | Origin publik untuk tautan di email (reset kata sandi) |
| `SMTP_HOST` / `SMTP_PORT` / `SMTP_USERNAME` / `SMTP_PASSWORD` / `SMTP_FROM` | Relay email produksi. `SMTP_HOST` kosong = email tidak dikirim. `SMTP_USERNAME` kosong = tanpa autentikasi (relay internal tepercaya); isi untuk memakai AUTH PLAIN. **Jangan** memakai `COMPOSE_PROFILES=mailpit` di VM: Mailpit hanya untuk dev/demo (`make docker/demo/up`, UI di `127.0.0.1:${MAILPIT_PORT:-8025}`) |
| `API_RATE_LIMIT_PER_MENIT` | Batas permintaan API per menit per IP klien (bawaan 600) |
| `TRUSTED_PROXIES` | CIDR proxy tepercaya untuk `X-Forwarded-For` (bawaan jaringan privat tempat Caddy berada) |
| `GO_API_BASE_URL` | `http://api:8080/v1` (internal) |
| `CADDY_HTTP_PORT` / `CADDY_HTTPS_PORT` | Port publik Caddy |

---

## Lampiran — Struktur Script

```
scripts/
├── lib/
│   ├── common.sh              # logging, die, lock, retry, trap, portability
│   └── ssh-wrapper.sh         # vm_ssh / vm_scp / vm_rsync / vm_preflight
├── deploy/
│   ├── build-local.sh         # buildx build --load
│   ├── save-images.sh         # docker save | gzip -9
│   ├── transfer.sh            # rsync ke VM + pre-flight disk
│   ├── remote-deploy.sh       # ← dijalankan DI VM
│   ├── deploy.sh              # orkestrator + auto-rollback
│   ├── rollback.sh            # rollback manual / otomatis
│   ├── status.sh              # ps + stats + disk + health
│   └── logs.sh                # tail log service
└── rollback/
    └── keep-last-n-images.sh  # prune image & tarball lama
```

Semua script: `#!/usr/bin/env bash`, `set -euo pipefail`, `trap ERR` dengan `LINENO`,
kompatibel macOS (BSD) + Ubuntu (GNU), dan mendukung `--dry-run`.


# 1. Setup VM (sekali) — Docker Engine + Compose v2, user deploy, authorized_keys
#    lalu buat ~/garda-inflasi/.env manual (TIDAK pernah ditransfer)

# 2. Setup Mac (sekali)
brew install rsync jq
ssh-keygen -t ed25519 -f ~/.ssh/garda_vm -N ""
chmod 600 ~/.ssh/garda_vm

# 3. Konfigurasi
make deploy/init-env && $EDITOR .env.deploy

# 4. Deploy
make deploy

# 5. Verifikasi
make status && curl -fsS https://app.garda.id/healthz
