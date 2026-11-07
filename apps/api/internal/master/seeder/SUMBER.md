# Sumber data referensi master

## kecamatan_aceh.csv

Kecamatan se-Aceh (kode BPS 7 digit) untuk tabel `kecamatan`, ditanam seeder `master.reference` (`go:embed`).

- **Sumber:** Badan Pusat Statistik (BPS), API bridging wilayah `sig.bps.go.id`
  (`https://sig.bps.go.id/rest-bridging/getwilayah?level=kecamatan&parent=<kode kabupaten>&periode_merge=<periode>`).
  Periode, tanggal pengambilan, jumlah baris, dan catatan sumber ada di baris komentar `#` di kepala berkas.
- **Isi:** `kode` (kode BPS), `nama` (`nama_dagri` bila ada, selain itu `nama_bps` berhuruf judul), `kabupaten_kode`
  (4 digit awal `kode`), `kode_dagri` (padanan Kemendagri, bisa kosong). Terurut menurut `kode`.
- **Lisensi dan syarat:** halaman Syarat dan Ketentuan BPS (<https://www.bps.go.id/en/term-of-use>, butir 13)
  membolehkan pemakaian konten untuk keperluan komersial maupun nonkomersial dengan mencantumkan sumber
  ("BPS-Statistics Indonesia") dan tanpa kesan didukung atau berafiliasi dengan BPS; BPS juga dapat membatasi akses
  bila pola aksesnya tidak wajar. Halaman itu menyebut situs web, bukan API bridging secara khusus, jadi
  **periksa kembali syarat BPS terbaru sebelum memakai data ini di luar proyek ini**. Tidak ada lisensi lain yang
  diklaim di sini. Pencantuman sumber berlaku: tampilkan "Sumber: BPS" di mana data kecamatan ditayangkan.
- **Nama dari `nama_dagri`:** sebagian kecil nama Kemendagri tampak salah ketik atau singkatan di sumbernya
  (mis. `1105161` "Indra Makmu", `1111080` "T. Jambo Aye", `1114040` "Kota Kualasinpang"). Berkas tidak disunting tangan;
  koreksi dilakukan di sumber, atau ubah aturan nama di `cmd/impor-kecamatan` (mis. pakai `nama_bps`).

### Menyegarkan

```
make db/impor/kecamatan      # perlu internet; berurutan, jeda 1 detik, satu kali ulang, User-Agent generik
git diff apps/api/internal/master/seeder/kecamatan_aceh.csv
cd apps/api && go test ./internal/master/seeder/ ./cmd/impor-kecamatan/
make db/seed/reference       # upsert menurut kode; baris yang tidak lagi ada di BPS tidak dihapus
```

Skrip memeriksa tujuh digit, awalan sama dengan kabupaten, tanpa kode ganda, nama tidak kosong, dan setiap
kabupaten/kota punya kecamatan; berkas lama tidak berubah bila ada yang gagal. Ganti periode lewat
`go run ./cmd/impor-kecamatan -periode <periode_merge>`. Berkas hasil di-commit bersama tinjauan diff-nya
(pemekaran atau perubahan nama kecamatan tampak di sana).
