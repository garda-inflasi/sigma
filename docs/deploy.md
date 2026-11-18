
Metode deploy sudah diperbarui ke alur **Sync ke `.build-src` → Build langsung di server → Hapus `.build-src` otomatis** (Zero-Git, tanpa membiarkan source code tersisa di server), dan saat ini **full live deploy** (`bash scripts/deploy/deploy.sh`) ke server `43.173.39.71` sedang berjalan. Saya menunggu proses build, migrasi, penyalaan layanan, serta sinkronisasi Caddy Cloudflare SSL selesai untuk melanjutkan pengujian live.

Sinkronisasi Caddyfile dengan perbaikan bind-mount inode sudah berhasil dan Caddy sedang menyelesaikan tantangan DNS-01 Cloudflare untuk sertifikat TLS `app.gardainflasi.id`. Saya sedang menunggu hasil pengujian endpoint HTTPS live (`task-4871`).

Ran verify /login page and record container IDs before partial update
Ran run selective --only=web deploy (Sync → Build on VM → Clean → Up web)

Pengujian endpoint HTTPS awal (`https://app.gardainflasi.id/healthz` → `200 OK` melalui Cloudflare & Caddy) berhasil. Saat ini saya sedang menjalankan uji **update parsial per-service (`bash scripts/deploy/deploy.sh --only=web`)** (`task-4877`) untuk memverifikasi sinkronisasi → build `garda-web` di server → penghapusan `.build-src` → restart `garda-web` saja tanpa mengganggu layanan lain.

Sistem deployment **Zero-Git (Sync → Server Build → Auto-Clean Source)** serta integrasi **Caddy + Cloudflare SSL** untuk `https://app.gardainflasi.id` sudah selesai dibuat dan **langsung dijalankan serta diverifikasi live** ke server `43.173.39.71`.

### 1. Ringkasan Perubahan Arsitektur Deploy
* **Alur Server-Build + Bersih dari Source Code (`scripts/deploy/sync-source.sh`, `scripts/deploy/remote-deploy.sh`, `scripts/deploy/deploy.sh`):**
  * **Tanpa Git:** Versi rilis memakai `RELEASE_TAG` berbasis timestamp (`vYYYYMMDD-HHMMSS` atau `--tag=<versi>`) tanpa `git rev-parse`/`git status`.
  * **Sync Selektif:** [sync-source.sh](scripts/deploy/sync-source.sh) menyinkronkan file compose & migrasi ke `/home/ubuntu/garda-inflasi/` serta konteks build bersih (mengecualikan `node_modules`, `.next`, `.git`, `.env*`, test/report) ke `/home/ubuntu/garda-inflasi/.build-src/`.
  * **Build di VM & Auto-Clean:** [remote-deploy.sh](scripts/deploy/remote-deploy.sh) membangun image secara sekuensial di server (`DOCKER_BUILDKIT=1`, dengan `NODE_OPTIONS=--max-old-space-size=2048` di [web.Dockerfile](docker/web.Dockerfile)), lalu **langsung menghapus `.build-src`** (termasuk lewat `EXIT` trap jika gagal) dan me-restart hanya layanan target (`--no-build --no-deps`) menggunakan tag per-service di `.env.tags`.
* **SSH Username + Password & Bootstrap VM (`scripts/lib/ssh-wrapper.sh`, `scripts/deploy/init-server.sh`):**
  * [ssh-wrapper.sh](scripts/lib/ssh-wrapper.sh) mendukung autentikasi SSH berbasis password (`expect` / `sshpass`) untuk `ssh`, `scp`, `rsync`, dan `sudo` secara non-interaktif lewat `.env.deploy` (di-ignore di [.gitignore](.gitignore); templat di [.env.deploy.example](.env.deploy.example)).
  * [init-server.sh](scripts/deploy/init-server.sh) menambahkan `ubuntu` ke grup `docker`, menyiapkan folder `/home/ubuntu/garda-inflasi/{compose,dist,scripts,backups,data/uploads,logs}`, memverifikasi network `caddy-gardainflasi_edge`, dan membuat `.env` produksi (mode `600`).
* **Integrasi Caddy Existing + Cloudflare TLS (`compose.gateway.yaml`, `compose.prod.yaml`, `scripts/deploy/sync-caddy.sh`):**
  * [compose.gateway.yaml](compose.gateway.yaml) menghubungkan `garda-api` (`:8080`) dan `garda-web` (`:3000`) ke network `garda_inflasi_net` dan `caddy-gardainflasi_edge`, memindahkan `caddy` bawaan ke profil `builtin-caddy`, serta menjaga `postgres`, `redis`, `worker`, `ml` tetap internal.
  * [compose.prod.yaml](compose.prod.yaml) mendukung tag independen `API_IMAGE_TAG`, `WEB_IMAGE_TAG`, `ML_IMAGE_TAG` dan memasang volume persisten `garda_lapangan_uploads:/app/uploads` (didukung [api.Dockerfile](docker/api.Dockerfile)).
  * [sync-caddy.sh](scripts/deploy/sync-caddy.sh) memperbarui `/home/ubuntu/caddy-gardainflasi/caddy/Caddyfile` secara aman (backup otomatis, penulisan *in-place* agar inode bind-mount Docker sinkron, `caddy validate`, `caddy reload`, Cloudflare DNS-01 TLS untuk `app.gardainflasi.id` + redirect `301` dari `gardainflasi.id`).
* **Skrip Pendukung & Target Makefile:** [rollback.sh](scripts/deploy/rollback.sh), [status.sh](scripts/deploy/status.sh), [logs.sh](scripts/deploy/logs.sh), [keep-last-n-images.sh](scripts/rollback/keep-last-n-images.sh), dan [Makefile](Makefile).

---

### 2. Hasil Deploy & Pengujian Live di Server (`43.173.39.71`)
* **Full Deploy + Selective Update (`--only=web`) Berhasil:**
  * Rilis penuh `v20261118-040851` membangun `api`, `ml`, dan `web` di server, menjalankan migrasi & seed referensi/admin, serta mengaktifkan Cloudflare TLS untuk `app.gardainflasi.id`.
  * Uji update parsial `bash scripts/deploy/deploy.sh --only=web` (`v20261118-042236`) hanya me-rebuild dan me-recreate `garda-web` sementara `garda-api`, `garda-worker`, `garda-ml`, `garda-postgres`, dan `garda-redis` tetap berjalan (`Up 8 minutes (healthy)`) tanpa restart.
  * Verifikasi kebersihan server: `/home/ubuntu/garda-inflasi/.build-src` dipastikan tidak ada setelah deploy.
* **Status Container (`bash scripts/deploy/status.sh`):**
  * `garda-postgres` (`postgres:16-alpine`): `Up 8 minutes (healthy)`, `127.0.0.1:5432->5432/tcp`
  * `garda-redis` (`redis:7-alpine`): `Up 8 minutes (healthy)`, `127.0.0.1:6379->6379/tcp`
  * `garda-api` (`garda-api:v20261118-040851`): `Up 8 minutes (healthy)`
  * `garda-worker` (`garda-api:v20261118-040851`): `Up 8 minutes (healthy)`
  * `garda-ml` (`garda-ml:v20261118-040851`): `Up 8 minutes (healthy)`
  * `garda-web` (`garda-web:v20261118-042236`): `Up About a minute (healthy)`
* **Pengujian Endpoint Publik Live (Cloudflare → Caddy → Containers):**
  * `GET https://app.gardainflasi.id/healthz` → `HTTP/2 200` (`{"status":"ok",...}`)
  * `GET https://app.gardainflasi.id/` → redirect ke `https://app.gardainflasi.id/masuk?lanjut=%2F` (`HTTP 200`)
  * `GET https://app.gardainflasi.id/api/auth/me` (BFF Next.js) → `HTTP/2 401` (`application/json`)
  * `GET https://gardainflasi.id/` → `HTTP/2 301` → `https://app.gardainflasi.id/`

---

### 3. Perintah Operasional Harian (Dari Mac)
* **Deploy seluruh layanan (sync → build di VM → hapus source → migrate → up → sync Caddy):**
  ```bash
  make deploy
  ```
* **Update parsial layanan tertentu saja** (misal hanya frontend `web` atau hanya `api` + `worker`):
  ```bash
  make deploy ARGS="--only=web"
  make deploy ARGS="--only=api,worker"
  make deploy ARGS="--only=ml"
  ```
* **Cek status, log, dan rollback:**
  ```bash
  make status
  make logs SERVICE=api TAIL=100
  make rollback
  ```
