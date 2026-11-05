// Package seeder mengisi registri model dan proyeksi harga contoh (hanya demo).
// Semuanya diberi sumber 'DEMO': bukan keluaran model sungguhan. Bentuknya
// meniru mockXai di apps/web/src/lib/api/mock.ts supaya halaman prediksi tampil
// sama dengan mode mock. Produksi tidak di-seed; tabel diisi pipeline ML (11-B).
package seeder

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	masterdomain "github.com/garda-inflasi/api/internal/master/domain"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/prediksi/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// seedPRNG membuat deret acak sama di setiap jalan.
	seedPRNG = 20260930

	// KabupatenUtama adalah kabupaten yang menampilkan semua komoditas (Kota Banda Aceh);
	// kabupaten lain hanya mendapat komoditas pilot.
	KabupatenUtama = "1171"

	VersiProduksi = "v2.3.1"
	VersiKandidat = "v2.4.0-rc1"

	// namaModel sama dengan model pada mockXai.
	namaModel = "Ensemble LightGBM + SARIMAX"
	// intervalZ adalah pengali simpangan untuk rentang keyakinan 80%.
	intervalZ = 1.28
	// hariDilatih adalah umur model produksi saat seed dijalankan.
	hariDilatih = 7
)

// Horizons adalah horizon proyeksi demo, dalam hari.
var Horizons = []int{7, 14, 30}

// skenario menggambarkan keadaan pasar demo satu komoditas.
type skenario struct {
	// Pct14 adalah perubahan harga proyeksi 14 hari (persen); horizon lain berskala linear.
	Pct14 float64
	// Volatilitas adalah simpangan harian (pecahan), sama dengan Komoditas.volatilitas di web.
	Volatilitas float64
	// Faktor memakai Bobot sebagai bagian dari perubahan total (jumlah 1; negatif = menahan).
	Faktor []faktorContoh
}

type faktorContoh struct {
	Nama, Detail string
	Bobot        float64
}

var faktorUmum = []faktorContoh{
	{"Momentum harga 7 hari", "Pergerakan harga mingguan di pasar sentinel", 0.45},
	{"Permintaan musiman", "Pola permintaan bulanan menurut kalender", 0.35},
	{"Pasokan regional", "Kiriman antardaerah dibanding rata-rata", 0.2},
}

var skenarioKomoditas = map[string]skenario{
	"CBM-KRT": {Pct14: 14.2, Volatilitas: 0.034, Faktor: []faktorContoh{
		{"Curah hujan sentra Gayo", "2,1× normal selama 10 hari di Aceh Tengah & Bener Meriah", 0.45},
		{"Stok cold storage", "Stok Lambaro turun 32% dalam 5 hari", 0.26},
		{"Momentum harga 7 hari", "Kenaikan beruntun di 7 dari 11 pasar", 0.22},
		{"Permintaan musiman", "Kenduri Maulid mulai pekan depan", 0.14},
		{"Pasokan dari Sumatera Utara", "Kiriman via Langsa naik 12%", -0.07},
	}},
	"BWM": {Pct14: 7.1, Volatilitas: 0.021, Faktor: []faktorContoh{
		{"Pasokan antarpulau", "Kiriman dari Brebes tertunda cuaca", 0.6},
		{"Momentum harga 7 hari", "Naik moderat di pesisir barat", 0.34},
		{"Panen lokal Pidie", "Panen kecil mulai masuk pasar Sigli", -0.22},
		{"Ongkos angkut", "Tarif logistik Medan–Banda Aceh naik 4%", 0.28},
	}},
	"BRS-MED": {Pct14: -2.4, Volatilitas: 0.004, Faktor: []faktorContoh{
		{"Panen gadu", "Panen Pidie & Aceh Utara 60% selesai", 0.52},
		{"Stok gudang Bulog", "Cukup untuk 4,2 bulan konsumsi", 0.36},
		{"Harga gabah kering panen", "Naik 2% di tingkat petani", -0.14},
		{"Momentum harga 7 hari", "Stabil", 0.26},
	}},
	"BRS-PRM": {Pct14: 0.6, Volatilitas: 0.004},
	"CBR-RWT": {Pct14: 9.5, Volatilitas: 0.038},
	"BWP":     {Pct14: 1.2, Volatilitas: 0.01},
	"TLR":     {Pct14: 1.8, Volatilitas: 0.008},
	"DAG-AYM": {Pct14: 2.2, Volatilitas: 0.012},
	"DAG-SPI": {Pct14: 0.4, Volatilitas: 0.003},
	"IKN-TGK": {Pct14: 3.1, Volatilitas: 0.024},
	"MYK":     {Pct14: -0.2, Volatilitas: 0.003},
	"GLA":     {Pct14: 0.3, Volatilitas: 0.003},
	"KTG":     {Pct14: -5.2, Volatilitas: 0.015},
	"TMT":     {Pct14: -4.8, Volatilitas: 0.03},
}

// DemoSummary melaporkan baris yang ditulis oleh SeedDemo.
type DemoSummary struct {
	Model, Proyeksi int
}

// SeedDemo meng-upsert registri model dan proyeksi demo. Id baris tetap (tanpa
// tanggal) sehingga menjalankannya di hari lain menggeser tanggal target, bukan
// menambah baris. Prasyarat: master.reference (komoditas dan kabupaten).
func SeedDemo(ctx context.Context, db *gorm.DB, now time.Time) (DemoSummary, error) {
	models, err := BuildModels(now)
	if err != nil {
		return DemoSummary{}, err
	}
	proyeksi, err := BuildProyeksi(now)
	if err != nil {
		return DemoSummary{}, err
	}

	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"nama", "mape", "drift", "status", "dilatih_pada", "algorithm", "sumber"}),
		}).Create(&models).Error
		if err != nil {
			return fmt.Errorf("model_registry: %w", err)
		}
		err = tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"tanggal_target", "nilai", "bawah", "atas", "harga_dasar", "faktor", "sumber", "dibuat_pada",
			}),
		}).CreateInBatches(&proyeksi, 200).Error
		if err != nil {
			return fmt.Errorf("prediksi_harga: %w", err)
		}
		return nil
	})
	if err != nil {
		return DemoSummary{}, fmt.Errorf("seed prediksi demo: %w", err)
	}
	return DemoSummary{Model: len(models), Proyeksi: len(proyeksi)}, nil
}

func modelID(komoditas, versi string) string { return "MDL-" + komoditas + "-" + versi }

func proyeksiID(komoditas, kabupaten string, horizon int) string {
	return fmt.Sprintf("PRD-%s-%s-%d", komoditas, kabupaten, horizon)
}

// mapeProduksi meniru tabel registri model di mode mock: makin volatil makin besar.
func mapeProduksi(volatilitas float64) float64 {
	return math.Round((volatilitas*180+1.5)*10) / 10
}

// BuildModels menyusun registri demo: satu model produksi dan satu kandidat per komoditas.
func BuildModels(now time.Time) ([]domain.ModelRegistry, error) {
	komoditas := masterseeder.Komoditas()
	dilatih := hariKe(now, -hariDilatih)
	algoritma := namaModel

	rows := make([]domain.ModelRegistry, 0, 2*len(komoditas))
	for i, k := range komoditas {
		sk, ok := skenarioKomoditas[k.Kode]
		if !ok {
			return nil, fmt.Errorf("seed prediksi demo: komoditas %s tanpa skenario", k.Kode)
		}
		mape := mapeProduksi(sk.Volatilitas)
		rows = append(rows,
			domain.ModelRegistry{
				ID: modelID(k.Kode, VersiProduksi), KomoditasKode: k.Kode, Nama: namaModel, Versi: VersiProduksi,
				Mape: mape, Drift: math.Round(float64(i%5)*0.03*100) / 100, Status: domain.StatusProduksi,
				DilatihPada: dilatih, Algorithm: &algoritma, Metrics: "{}", FeatureImportance: "[]", Sumber: domain.SumberDemo,
			},
			domain.ModelRegistry{
				ID: modelID(k.Kode, VersiKandidat), KomoditasKode: k.Kode, Nama: namaModel, Versi: VersiKandidat,
				Mape: math.Round(mape*0.93*10) / 10, Status: domain.StatusKandidat,
				DilatihPada: hariKe(now, -1), Algorithm: &algoritma, Metrics: "{}", FeatureImportance: "[]", Sumber: domain.SumberDemo,
			},
		)
	}
	return rows, nil
}

// BuildProyeksi menyusun proyeksi demo (model produksi) secara deterministik
// terhadap now: semua komoditas di KabupatenUtama, komoditas pilot di kabupaten lain.
func BuildProyeksi(now time.Time) ([]domain.PrediksiHarga, error) {
	komoditas, kabupaten := masterseeder.Komoditas(), masterseeder.Kabupaten()
	rng := rand.New(rand.NewPCG(seedPRNG, seedPRNG))
	dasar := hariKe(now, 0)

	var rows []domain.PrediksiHarga
	for _, kab := range kabupaten {
		// Faktor harga dan selisih persen kabupaten ditarik untuk setiap kabupaten
		// agar deret acak tidak bergantung pada komoditas mana yang ikut.
		faktorHarga := 0.96 + rng.Float64()*0.1
		selisihPct := (rng.Float64() - 0.5) * 2
		for _, k := range komoditas {
			if kab.Kode != KabupatenUtama && !k.Pilot {
				continue
			}
			sk, ok := skenarioKomoditas[k.Kode]
			if !ok {
				return nil, fmt.Errorf("seed prediksi demo: komoditas %s tanpa skenario", k.Kode)
			}
			rows2, err := proyeksiKomoditas(k, kab, sk, faktorHarga, selisihPct, dasar)
			if err != nil {
				return nil, err
			}
			rows = append(rows, rows2...)
		}
	}
	return rows, nil
}

func proyeksiKomoditas(k masterdomain.Komoditas, kab masterdomain.Kabupaten, sk skenario, faktorHarga, selisihPct float64, dasar time.Time) ([]domain.PrediksiHarga, error) {
	hargaDasar := int(math.Round(float64(k.HargaAcuan) * faktorHarga))
	pct14 := sk.Pct14
	if kab.Kode != KabupatenUtama {
		pct14 += selisihPct
	}
	faktor := sk.Faktor
	if faktor == nil {
		faktor = faktorUmum
	}

	rows := make([]domain.PrediksiHarga, len(Horizons))
	for i, h := range Horizons {
		pct := pct14 * float64(h) / 14
		lebar := sk.Volatilitas * 100 * math.Sqrt(float64(h)) * intervalZ // setengah rentang, persen

		dokumen, err := faktorJSON(faktor, pct)
		if err != nil {
			return nil, fmt.Errorf("seed prediksi demo: faktor %s: %w", k.Kode, err)
		}
		rows[i] = domain.PrediksiHarga{
			ID: proyeksiID(k.Kode, kab.Kode, h), KomoditasKode: k.Kode, KabupatenKode: kab.Kode,
			TanggalTarget: dasar.AddDate(0, 0, h), HorizonHari: h,
			Nilai: hargaDari(hargaDasar, pct), Bawah: hargaDari(hargaDasar, pct-lebar), Atas: hargaDari(hargaDasar, pct+lebar),
			HargaDasar: hargaDasar, ModelVersi: VersiProduksi, Faktor: dokumen, Sumber: domain.SumberDemo,
			DibuatPada: dasar,
		}
	}
	return rows, nil
}

func hargaDari(dasar int, pct float64) int {
	return int(math.Round(float64(dasar) * (1 + pct/100)))
}

// faktorJSON menghitung kontribusi tiap faktor (bobot × perubahan total, satu desimal).
func faktorJSON(faktor []faktorContoh, pct float64) (string, error) {
	out := make([]domain.Faktor, len(faktor))
	for i, f := range faktor {
		out[i] = domain.Faktor{Nama: f.Nama, Kontribusi: math.Round(f.Bobot*pct*10) / 10, Detail: f.Detail}
	}
	raw, err := json.Marshal(out)
	return string(raw), err
}

// hariKe mengembalikan awal hari (UTC) now digeser n hari.
func hariKe(now time.Time, n int) time.Time {
	y, m, d := now.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).AddDate(0, 0, n)
}
