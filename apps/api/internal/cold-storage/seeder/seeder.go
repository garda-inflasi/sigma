// Package seeder mengisi data cold storage dalam dua lapis: fasilitas
// (referensi, aman di produksi) dan lot stok (demo, bergeser mengikuti waktu).
// Komoditas dan kabupaten berasal dari seeder master.reference.
package seeder

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/cold-storage/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReferenceSummary melaporkan baris yang ditulis oleh SeedReference.
type ReferenceSummary struct {
	Fasilitas int
}

// DemoSummary melaporkan baris yang ditulis oleh SeedDemo.
type DemoSummary struct {
	Stok      int
	Telemetri int
	Transaksi int
}

type fasilitasSeed struct {
	ID           string
	Nama         string
	Wilayah      string // nama pada tabel kabupaten, mis. "KABUPATEN ACEH BESAR"
	Lat, Lng     float64
	KapasitasTon int
	SuhuTargetC  float64
	SuhuAktualC  *float64
	Status       string
}

type stokSeed struct {
	ID         string
	Fasilitas  string
	Komoditas  string
	VolumeTon  float64
	UmurHari   int // sejak masuk
	MasaSimpan int // total masa simpan dalam rantai dingin
	Risiko     float64
	Pemilik    string
}

// Wilayah dirujuk menurut nama, bukan kode: kode kabupaten diputuskan oleh
// master wilayah, sehingga seeder tetap benar bila kode di sana berubah.
var fasilitasSeeds = []fasilitasSeed{
	{ID: "CS-01", Nama: "Cold Storage Lambaro", Wilayah: "KABUPATEN ACEH BESAR", Lat: 5.505, Lng: 95.37, KapasitasTon: 400, SuhuTargetC: 4, SuhuAktualC: new(3.5), Status: "OPERASIONAL"},
	{ID: "CS-02", Nama: "Cold Storage Perikanan Lhokseumawe", Wilayah: "KOTA LHOKSEUMAWE", Lat: 5.17, Lng: 97.13, KapasitasTon: 300, SuhuTargetC: -20, SuhuAktualC: new(-20.1), Status: "OPERASIONAL"},
	{ID: "CS-03", Nama: "Sentra Hortikultura Takengon", Wilayah: "KABUPATEN ACEH TENGAH", Lat: 4.64, Lng: 96.86, KapasitasTon: 250, SuhuTargetC: 6, SuhuAktualC: new(6.0), Status: "OPERASIONAL"},
	{ID: "CS-04", Nama: "Gudang Dingin Meulaboh", Wilayah: "KABUPATEN ACEH BARAT", Lat: 4.15, Lng: 96.14, KapasitasTon: 150, SuhuTargetC: 4, SuhuAktualC: new(4.8), Status: "PERAWATAN"},
	// Rencana: belum dibangun, tanpa telemetri maupun stok.
	{ID: "CS-05", Nama: "Gudang Dingin Langsa", Wilayah: "KOTA LANGSA", Lat: 4.48, Lng: 97.96, KapasitasTon: 120, SuhuTargetC: 4, SuhuAktualC: nil, Status: "RENCANA"},
}

// stokSeeds dirancang agar:
//   - volume per fasilitas = utilisasi di kartu (347/400, 240/300, 193/250, 33/150 ton);
//   - lot berisiko tinggi (risiko > 0,6) berjumlah 172,7 ton;
//   - 23 lot, cukup untuk tiga halaman dengan 10 baris per halaman;
//   - sisa masa simpan paling sedikit 5 hari (MasaSimpan − UmurHari).
var stokSeeds = []stokSeed{
	// CS-01 Lambaro — 347,0 ton
	{ID: "STK-CS01-01", Fasilitas: "CS-01", Komoditas: "CBM-KRT", VolumeTon: 34.9, UmurHari: 16, MasaSimpan: 21, Risiko: 0.68, Pemilik: "KUD Pidie Jaya"},
	{ID: "STK-CS01-02", Fasilitas: "CS-01", Komoditas: "CBR-RWT", VolumeTon: 29.4, UmurHari: 6, MasaSimpan: 18, Risiko: 0.38, Pemilik: "CV Lambaro Segar"},
	{ID: "STK-CS01-03", Fasilitas: "CS-01", Komoditas: "CBM-KRT", VolumeTon: 27.8, UmurHari: 9, MasaSimpan: 21, Risiko: 0.47, Pemilik: "Koperasi Tani Gayo Makmur"},
	{ID: "STK-CS01-04", Fasilitas: "CS-01", Komoditas: "CBM-KRT", VolumeTon: 38.2, UmurHari: 5, MasaSimpan: 21, Risiko: 0.26, Pemilik: "Kelompok Tani Bener Meriah"},
	{ID: "STK-CS01-05", Fasilitas: "CS-01", Komoditas: "CBR-RWT", VolumeTon: 18.6, UmurHari: 3, MasaSimpan: 18, Risiko: 0.21, Pemilik: "KUD Pidie Jaya"},
	{ID: "STK-CS01-06", Fasilitas: "CS-01", Komoditas: "CBM-KRT", VolumeTon: 36.0, UmurHari: 3, MasaSimpan: 21, Risiko: 0.22, Pemilik: "Kelompok Tani Bener Meriah"},
	{ID: "STK-CS01-07", Fasilitas: "CS-01", Komoditas: "BWM", VolumeTon: 52.7, UmurHari: 38, MasaSimpan: 60, Risiko: 0.55, Pemilik: "Koperasi Tani Gayo Makmur"},
	{ID: "STK-CS01-08", Fasilitas: "CS-01", Komoditas: "BWM", VolumeTon: 61.4, UmurHari: 21, MasaSimpan: 60, Risiko: 0.31, Pemilik: "CV Lambaro Segar"},
	{ID: "STK-CS01-09", Fasilitas: "CS-01", Komoditas: "BWM", VolumeTon: 48.0, UmurHari: 10, MasaSimpan: 60, Risiko: 0.17, Pemilik: "KUD Pidie Jaya"},

	// CS-02 Lhokseumawe — 240,0 ton (beku, −20 °C)
	{ID: "STK-CS02-01", Fasilitas: "CS-02", Komoditas: "IKN-TGK", VolumeTon: 64.5, UmurHari: 101, MasaSimpan: 120, Risiko: 0.72, Pemilik: "Koperasi Nelayan Samudera Pasai"},
	{ID: "STK-CS02-02", Fasilitas: "CS-02", Komoditas: "IKN-TGK", VolumeTon: 42.3, UmurHari: 106, MasaSimpan: 120, Risiko: 0.81, Pemilik: "CV Mina Lhokseumawe"},
	{ID: "STK-CS02-03", Fasilitas: "CS-02", Komoditas: "IKN-TGK", VolumeTon: 52.8, UmurHari: 70, MasaSimpan: 120, Risiko: 0.38, Pemilik: "Koperasi Nelayan Samudera Pasai"},
	{ID: "STK-CS02-04", Fasilitas: "CS-02", Komoditas: "IKN-TGK", VolumeTon: 47.3, UmurHari: 44, MasaSimpan: 120, Risiko: 0.24, Pemilik: "CV Mina Lhokseumawe"},
	{ID: "STK-CS02-05", Fasilitas: "CS-02", Komoditas: "IKN-TGK", VolumeTon: 33.1, UmurHari: 12, MasaSimpan: 120, Risiko: 0.09, Pemilik: "Koperasi Nelayan Samudera Pasai"},

	// CS-03 Takengon — 193,0 ton
	{ID: "STK-CS03-01", Fasilitas: "CS-03", Komoditas: "TMT", VolumeTon: 31.0, UmurHari: 8, MasaSimpan: 14, Risiko: 0.63, Pemilik: "Kelompok Tani Bener Meriah"},
	{ID: "STK-CS03-02", Fasilitas: "CS-03", Komoditas: "TMT", VolumeTon: 15.4, UmurHari: 8, MasaSimpan: 14, Risiko: 0.57, Pemilik: "CV Lambaro Segar"},
	{ID: "STK-CS03-03", Fasilitas: "CS-03", Komoditas: "TMT", VolumeTon: 33.1, UmurHari: 7, MasaSimpan: 14, Risiko: 0.44, Pemilik: "KUD Pidie Jaya"},
	{ID: "STK-CS03-04", Fasilitas: "CS-03", Komoditas: "CBM-KRT", VolumeTon: 14.4, UmurHari: 11, MasaSimpan: 21, Risiko: 0.54, Pemilik: "KUD Pidie Jaya"},
	{ID: "STK-CS03-05", Fasilitas: "CS-03", Komoditas: "KTG", VolumeTon: 41.3, UmurHari: 20, MasaSimpan: 75, Risiko: 0.18, Pemilik: "Koperasi Tani Gayo Makmur"},
	{ID: "STK-CS03-06", Fasilitas: "CS-03", Komoditas: "KTG", VolumeTon: 36.5, UmurHari: 31, MasaSimpan: 75, Risiko: 0.33, Pemilik: "Kelompok Tani Bener Meriah"},
	{ID: "STK-CS03-07", Fasilitas: "CS-03", Komoditas: "KTG", VolumeTon: 21.3, UmurHari: 12, MasaSimpan: 75, Risiko: 0.15, Pemilik: "KUD Pidie Jaya"},

	// CS-04 Meulaboh — 33,0 ton (dalam perawatan)
	{ID: "STK-CS04-01", Fasilitas: "CS-04", Komoditas: "TLR", VolumeTon: 12.9, UmurHari: 12, MasaSimpan: 30, Risiko: 0.41, Pemilik: "KUD Aceh Barat Sejahtera"},
	{ID: "STK-CS04-02", Fasilitas: "CS-04", Komoditas: "BWM", VolumeTon: 20.1, UmurHari: 27, MasaSimpan: 60, Risiko: 0.41, Pemilik: "KUD Aceh Barat Sejahtera"},
}

const day = 24 * time.Hour

// SeedReference mengisi lima fasilitas. Idempoten: baris di-upsert menurut ID.
// Aman dijalankan di produksi.
//
// Prasyarat: seeder master.reference sudah dijalankan (kabupaten ada).
func SeedReference(ctx context.Context, db *gorm.DB) (ReferenceSummary, error) {
	var sum ReferenceSummary
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		kode, err := kodeKabupaten(tx)
		if err != nil {
			return err
		}
		fasilitas := BuildFasilitas(kode)
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"nama", "kabupaten_kode", "lat", "lng", "kapasitas_ton", "suhu_target_c", "status"}),
		}).Create(&fasilitas).Error; err != nil {
			return fmt.Errorf("seed fasilitas: %w", err)
		}
		sum.Fasilitas = len(fasilitas)
		return nil
	})
	return sum, err
}

// SeedDemo mengisi 23 lot stok contoh, telemetri 48 jam terakhir, dan transaksi
// masuk tiap lot. Idempoten: baris di-upsert menurut kunci, jadi menjalankannya
// ulang menyegarkan tanggal relatif terhadap now tanpa menggandakan data
// (telemetri demo di luar jendela 48 jam dibuang). Hanya untuk demo, bukan produksi.
//
// Prasyarat: SeedReference sudah dijalankan.
func SeedDemo(ctx context.Context, db *gorm.DB, now time.Time) (DemoSummary, error) {
	var sum DemoSummary
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		stok := BuildStok(now)
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"cold_storage_id", "komoditas_kode", "volume_ton", "masuk_pada", "kedaluwarsa_pada", "pemilik", "risiko_food_loss", "status"}),
		}).Create(&stok).Error; err != nil {
			return fmt.Errorf("seed stok: %w", err)
		}
		sum.Stok = len(stok)

		n, err := seedTelemetri(tx, now)
		if err != nil {
			return err
		}
		sum.Telemetri = n

		trx := BuildTransaksi(now)
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"lot_id", "jenis", "volume_ton", "fasilitas_tujuan_id", "waktu", "alasan", "sumber"}),
		}).Create(&trx).Error; err != nil {
			return fmt.Errorf("seed transaksi: %w", err)
		}
		sum.Transaksi = len(trx)
		return nil
	})
	return sum, err
}

// seedTelemetri menulis pembacaan demo dan menurunkan suhu/kelembapan aktual
// fasilitas dari pembacaan terbarunya.
func seedTelemetri(tx *gorm.DB, now time.Time) (int, error) {
	rows := BuildTelemetri(now)
	if len(rows) == 0 {
		return 0, nil
	}
	awal := rows[0].Waktu
	for _, r := range rows {
		if r.Waktu.Before(awal) {
			awal = r.Waktu
		}
	}
	// Jendela bergeser mengikuti now; baris demo yang tertinggal dibuang agar jumlahnya tetap.
	if err := tx.Exec("DELETE FROM telemetri_suhu WHERE sumber = ? AND waktu < ?", domain.SumberDemo, awal).Error; err != nil {
		return 0, fmt.Errorf("bersihkan telemetri demo lama: %w", err)
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "fasilitas_id"}, {Name: "waktu"}},
		DoUpdates: clause.AssignmentColumns([]string{"suhu_c", "kelembapan_pct", "sumber"}),
	}).CreateInBatches(&rows, 200).Error; err != nil {
		return 0, fmt.Errorf("seed telemetri: %w", err)
	}

	terbaru := map[string]domain.TelemetriSuhu{}
	for _, r := range rows {
		if cur, ok := terbaru[r.FasilitasID]; !ok || r.Waktu.After(cur.Waktu) {
			terbaru[r.FasilitasID] = r
		}
	}
	for id, r := range terbaru {
		if err := tx.Model(&domain.FasilitasColdStorage{}).Where("id = ?", id).Updates(map[string]any{
			"suhu_aktual_c": r.SuhuC, "kelembapan_aktual_pct": r.KelembapanPct, "telemetri_pada": r.Waktu,
		}).Error; err != nil {
			return 0, fmt.Errorf("perbarui suhu aktual %s: %w", id, err)
		}
	}
	return len(rows), nil
}

// kodeKabupaten memetakan nama wilayah tiap fasilitas ke kode kabupaten di master.
func kodeKabupaten(tx *gorm.DB) (map[string]string, error) {
	kode := make(map[string]string, len(fasilitasSeeds))
	for _, f := range fasilitasSeeds {
		var k string
		if err := tx.Raw("SELECT kode FROM kabupaten WHERE UPPER(nama) = UPPER(?) LIMIT 1", f.Wilayah).Scan(&k).Error; err != nil {
			return nil, fmt.Errorf("cari kabupaten %q: %w", f.Wilayah, err)
		}
		if k == "" {
			return nil, fmt.Errorf("kabupaten %q tidak ditemukan; jalankan seeder master.reference lebih dulu (make db/seed/reference)", f.Wilayah)
		}
		kode[f.Wilayah] = k
	}
	return kode, nil
}

// BuildFasilitas menyusun baris cold_storage; kodeByWilayah memetakan nama
// wilayah ke kode kabupaten.
func BuildFasilitas(kodeByWilayah map[string]string) []domain.FasilitasColdStorage {
	out := make([]domain.FasilitasColdStorage, len(fasilitasSeeds))
	for i, f := range fasilitasSeeds {
		out[i] = domain.FasilitasColdStorage{
			ID:            f.ID,
			Nama:          f.Nama,
			KabupatenKode: kodeByWilayah[f.Wilayah],
			Lat:           f.Lat,
			Lng:           f.Lng,
			KapasitasTon:  f.KapasitasTon,
			SuhuTargetC:   f.SuhuTargetC,
			SuhuAktualC:   f.SuhuAktualC,
			Status:        f.Status,
		}
	}
	return out
}

// BuildStok menyusun baris stok_cold_storage dengan tanggal relatif terhadap now.
func BuildStok(now time.Time) []domain.StokColdStorage {
	now = now.UTC()
	out := make([]domain.StokColdStorage, len(stokSeeds))
	for i, s := range stokSeeds {
		masuk := now.Add(-time.Duration(s.UmurHari) * day)
		out[i] = domain.StokColdStorage{
			ID:              s.ID,
			ColdStorageID:   s.Fasilitas,
			KomoditasKode:   s.Komoditas,
			VolumeTon:       s.VolumeTon,
			MasukPada:       masuk,
			KedaluwarsaPada: masuk.Add(time.Duration(s.MasaSimpan) * day),
			Pemilik:         s.Pemilik,
			RisikoFoodLoss:  s.Risiko,
		}
	}
	return out
}
