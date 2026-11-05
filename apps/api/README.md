# API Go

Gin + GORM, arsitektur hexagonal per modul: `internal/<modul>/{domain,usecase,repository,delivery,dto,routes}`.
Konfigurasi lewat env (lihat `env.example` di akar repo). Alur lokal ada di [README akar](../../README.md).

## Perintah

| Perintah                               | Fungsi                                                       |
| -------------------------------------- | ------------------------------------------------------------ |
| `cmd/api`                              | server HTTP                                                  |
| `cmd/migrate up\|down\|version`        | migrasi goose dari `MIGRATIONS_DIR` (bawaan `db/migrations`) |
| `cmd/seed reference\|admin\|demo\|all` | seeder; `demo` ditolak bila `APP_ENV=production`             |
| `cmd/ingest <sumber>`                  | jalankan ingestion satu sumber sekali (mis. `SRC-BI`); juga `daftar`, `pemetaan`, `konfigurasi` |

## Menambah modul

Setiap modul baru **menambah berkas baru** di empat tempat berikut. Jangan mengedit berkas bersama
(registry, `main.go`, daftar tes) agar cabang paralel tidak bentrok saat merge. Satu-satunya
berkas bersama yang ikut berubah adalah `internal/modules/registry.go` (satu baris `Register`)
dan, bila ada paket integrasi baru, variabel `INTEGRATION_PACKAGES` di Makefile.

Kunci modul = nama direktori di `internal/` (mis. `harga`, `policy-brief`). Di nama berkas,
tanda hubung menjadi garis bawah (`policy_brief`).

### 1. Seeder (`cmd/seed/register_<modul>.go`)

Isi seeder ada di `internal/<modul>/seeder/` (fungsi biasa, tanpa mengimpor `seedkit`).
Berkas register mendaftarkannya dengan `init()`:

```go
func init() {
	seedkit.Register(seedkit.Seeder{
		Name:      "harga.demo",             // "<modul>.<lapis>", akhiran harus sama dengan Tier
		Tier:      seedkit.TierDemo,         // TierReference | TierAdmin | TierDemo
		DependsOn: []string{"master.reference"}, // keras: tidak terdaftar => galat
		After:     []string{"auth.demo"},        // lunak: hanya urutan, bila terdaftar
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := hargaseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"entri": sum.Entri}, err
		},
	})
}
```

Aturan: idempoten (upsert, jalankan dua kali tidak menambah baris), deterministik (PRNG seed
tetap; tanggal relatif terhadap `now`), `reference`/`admin` aman di produksi, `demo` tidak.
Nama kanonik: `auth.admin`, `auth.demo`, `master.reference`, `coldstorage.reference|demo`,
`harga.demo`, `lapangan.demo`, `dashboard.reference|demo`, `ews.reference|demo`,
`intervensi.reference|demo`, `spasial.demo`, `sumber.reference|demo`, `eksekutif.reference|demo`,
`brief.demo`, `kpi.reference|demo`, `prediksi.demo`.
Dependensi di lapis yang tidak dipilih dianggap sudah dijalankan (`seed demo` mengandaikan `reference` sudah ada).

### 2. Entity vs migrasi (`internal/platform/schematest`)

Tes kontrak menerapkan semua migrasi dengan goose ke PostgreSQL sementara dan membandingkan
tabel, kolom, dan tipe tiap entity dengan `information_schema`.

- `models_<modul>_test.go`: daftarkan entity ber-`TableName()`:

  ```go
  func init() { registerModels("harga", &hargadomain.EntriHarga{}) }
  ```

- `pending_<modul>_test.go`: tandai modul yang belum selaras beserta alasannya:

  ```go
  func init() { markPending("harga", "entri_harga: kolom ... tidak ada (slice 06-A)") }
  ```

Modul di luar daftar pending **harus** lulus. Menyelaraskan modul = menghapus berkas `pending_`-nya.
Tes gagal bila modul pending ternyata sudah cocok, jadi daftar tidak bisa basi.

### 3. Tes asap (`internal/platform/smoketest`)

`smoke_<modul>_test.go` menambah kasus. Tes menyalakan PostgreSQL, migrasi, membangun dan
menjalankan `cmd/seed` (reference lalu demo), merakit router penuh, lalu memanggil tiap kasus:

```go
func init() {
	registerSmoke(smokeCase{
		Name:           "harga: daftar entri",
		Method:         http.MethodGet,
		Path:           "/v1/harga",
		Email:          "koordinator.lapangan@gardainflasi.id", // akun demo berhak: harus 200
		MinItems:       map[string]int{"items": 10},            // larik di data ("" = data itu sendiri)
		ForbiddenEmail: "strategis.bi@gardainflasi.id",         // tak berhak: harus 403
	})
}
```

Setiap kasus juga diperiksa 401 tanpa token. Semua akun demo kata sandi `Garda2024!`
(`login`, `readEnvelope`, `countItems` ada di `harness_test.go`).
`TestSeed_IsIdempotent` otomatis mencakup seeder baru (jumlah baris dua kali jalan harus sama).

### 4. Integrasi per modul

Tes yang memakai Postgres memakai `internal/platform/pgtest` (container bersama per paket,
`pgtest.Main` di `TestMain`, `pg.NewDatabase(t, nama)`, `pgtest.MigrateUp(t, db)` memakai goose
sungguhan). Jangan menulis pemisah SQL goose sendiri. Migrasi baru: nomor berikutnya dari
`db/migrations`, wajib `Up` dan `Down`, tanpa data demo, tanpa `DELETE`/`TRUNCATE`
(aturan lengkap di `docs/todos/00-cara-kerja.md` §3).

### 5. Agregat dashboard

KPI, peta panas, dan agregat harga harian adalah materialized view (`agregat_harga_harian`,
`mv_heatmap_harga`, `mv_kpi_operasional`) atas `entri_harga`. `internal/dashboard/refresher`
menyegarkannya dengan `REFRESH ... CONCURRENTLY`: tiap 5 menit dari `cmd/api` dan segera setelah
harga divalidasi atau dipublikasi. Seeder `dashboard.demo` menyegarkannya setelah data demo terisi.
Menambah view baru berarti menambahkannya ke `refresher.Views` (urut menurut ketergantungan).

### 6. Ingestion sumber data eksternal (`internal/ingestion`)

Konektor (`internal/ingestion/connector/<sumber>`) hanya merancang permintaan dan mem-parse respons;
`engine` mengambilnya lewat `Fetcher` (pembatasan laju per host, robots.txt, retry backoff, circuit
breaker), menyimpan payload mentah (bronze, `data_mentah`), menormalkan ke silver (`harga_eksternal`,
`cuaca_harian`, `inflasi_bulanan`/`andil_inflasi` untuk BPS), dan mencatat tiap run di `eksekusi_sumber`.
Gold adalah materialized view `gold_harga_harian` (ikut `refresher.Views`).

- Tes konektor memakai fixture di `connector/<sumber>/testdata`; `ingestiontest.Fetcher` menyuntikkan
  respons ke engine sehingga tidak ada tes yang memanggil internet.
- Pemetaan alias sumber ke kode master ada di `pemetaan_sumber` (seeder `ingestion.reference`, insert-only;
  ubah lewat `cmd/ingest pemetaan`). Baris yang tidak terpetakan dilaporkan di `eksekusi_sumber.ringkasan`.
- Parameter permintaan per sumber ada di `sumber_data.konfigurasi` (`cmd/ingest konfigurasi`); kunci API
  dibaca dari lingkungan (`BPS_API_KEY`), tidak disimpan di basis data.
- Konektor yang tersedia: `SRC-BI` (PIHPS), `SRC-BMKG`, `SRC-BPS` (butuh `BPS_API_KEY` dan id variabel di
  konfigurasi). Ketiganya belum diverifikasi terhadap layanan nyata; parser defensif dan dikunci fixture.
- Seeder demo `ingestion.demo` menulis eksekusi, bronze, harga, dan cuaca bertanda DEMO (pemicu `DEMO`) dan
  tidak pernah menimpa hasil ingestion nyata.
- Penjadwal di `cmd/api` mati kecuali `INGEST_SCHEDULER=on`. Tombol "Jalankan sekarang" di `/sumber-data`
  (izin `sumber.kelola`) memanggil `POST /v1/sumber-data/:id/jalankan`.
