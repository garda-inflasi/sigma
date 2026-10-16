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
│                              │         │  9. wait healthcheck (3/3)           │
│                              │         │ 10. compose up -d --no-build (app)   │
│                              │         │ 11. curl /healthz                    │
│                              │         │ 12. tulis .last-deploy               │
│ 13. healthcheck gagal?       │         │                                      │
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
POSTGRES_PASSWORD=GANTI_DENGAN_PASSWORD_KUAT
POSTGRES_DB=garda_inflasi
DB_HOST=postgres
DB_PORT=5432
DB_USER=garda
DB_PASSWORD=GANTI_DENGAN_PASSWORD_KUAT
DB_NAME=garda_inflasi
DATABASE_URL=postgres://garda:GANTI_DENGAN_PASSWORD_KUAT@postgres:5432/garda_inflasi?sslmode=disable

# ── Redis ───────────────────────────────────────────────────────────────
REDIS_URL=redis://redis:6379/0

# ── MongoDB ─────────────────────────────────────────────────────────────
MONGO_URI=mongodb://mongodb:27017
MONGO_DB=garda_inflasi

# ── Secrets (WAJIB diganti, minimal 32 karakter acak) ───────────────────
JWT_SECRET=GANTI_DENGAN_RANDOM_32_KARAKTER
AES_KEY=GANTI_DENGAN_RANDOM_32_KARAKTER
GARDA_JWT_SECRET=GANTI_DENGAN_RANDOM_32_KARAKTER

# ── App ─────────────────────────────────────────────────────────────────
GIN_MODE=release
APP_ADDR=:8080
APP_ALLOWED_ORIGINS=https://app.garda.id
NODE_ENV=production
GO_API_BASE_URL=http://api:8080/v1

# ── Caddy ───────────────────────────────────────────────────────────────
CADDY_HTTP_PORT=80
CADDY_HTTPS_PORT=443
ENVEOF

ssh -i ~/.ssh/garda_vm deploy@192.168.1.100 'chmod 600 ~/garda-inflasi/.env'
```

Generate secret acak:

```bash
openssl rand -base64 32
```

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

```bash
# Backup otomatis sebelum deploy
make deploy ARGS=--backup

# Backup manual
ssh -i ~/.ssh/garda_vm deploy@192.168.1.100 '
  cd ~/garda-inflasi
  set -a; . ./.env; set +a
  mkdir -p backups
  docker exec garda-postgres pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" \
    | gzip > "backups/$(date -u +%F-%H%M).sql.gz"
  ls -lh backups/ | tail -5
'
```

### Restore

```bash
ssh -i ~/.ssh/garda_vm deploy@192.168.1.100 '
  cd ~/garda-inflasi
  set -a; . ./.env; set +a
  gunzip -c backups/2026-10-16-0900.sql.gz \
    | docker exec -i garda-postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"
'
```

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
| 12 | `Healthcheck infra gagal (2/3 healthy)` | Postgres/Mongo/Redis tidak start | `make logs SERVICE=postgres`; cek kredensial `.env`; cek volume korup |
| 13 | `docker-compose: command not found` | VM punya Compose v1 | Install plugin v2: `sudo apt install docker-compose-plugin`; script hanya pakai `docker compose` |
| 14 | `Deploy lain sedang berjalan (lock: ...)` | Deploy sebelumnya crash / masih jalan | Tunggu; kalau yakin tidak ada proses: `rm -rf .deploy.lock` |
| 15 | `Build dari working tree kotor` | Ada perubahan belum di-commit | `git commit` dulu, atau `make deploy ARGS=--allow-dirty` |
| 16 | `Wajib ada .env di VM` | `.env` production belum dibuat | Lihat §3.6 — buat manual di VM (tidak pernah ditransfer) |
| 17 | `services.mongodb.build must be a string` | `compose.prod.yaml` memakai `build: null` | Gunakan `build: !reset null` (sudah diperbaiki di repo ini) |
| 18 | `Tarball korup / ukuran tidak cocok` | Transfer terputus | Ulangi `make deploy/transfer`; rsync `--partial` akan melanjutkan |
| 19 | `jq: command not found` di VM | `jq` tidak terinstall | Opsional — script fallback ke `grep`. Atau `sudo apt install jq` |
| 20 | Login gagal / redirect loop setelah deploy | HSTS aktif di HTTP, atau `NEXT_PUBLIC_API_BASE_URL` absolut | Pastikan akses via HTTPS; set `NEXT_PUBLIC_API_BASE_URL=/api` (relatif) |
| 21 | `CORS error` di browser | Origin tidak cocok (http vs https) | Set `APP_ALLOWED_ORIGINS` di `.env` VM; pakai URL relatif di frontend |
| 22 | `502 Bad Gateway` dari Caddy | Container `api`/`web` belum siap | `make logs SERVICE=web`; tunggu healthcheck; cek `docker compose ps` |

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
| `DATABASE_URL` | DSN lengkap untuk Go |
| `REDIS_URL` | DSN Redis |
| `MONGO_URI` / `MONGO_DB` | DSN MongoDB |
| `JWT_SECRET` | Signing key JWT (≥32 char acak) |
| `AES_KEY` | Enkripsi at-rest (≥32 char acak) |
| `GARDA_JWT_SECRET` | Secret untuk BFF Next.js |
| `APP_ALLOWED_ORIGINS` | CORS allowlist |
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
