package seeder

import (
	"fmt"
	"math"
	"time"

	"github.com/garda-inflasi/api/internal/eksekutif/domain"
)

// Bentuk data mengikuti mockEksekutif di web. Semua angka TIRUAN (sumber DEMO).

// yoyDemo dan mtmDemo adalah 12 bulan berurutan dari yang tertua; bulan terakhir
// (2,84% yoy, 0,41% mtm) sama dengan mockEksekutif.
var (
	yoyDemo = [12]float64{2.12, 1.94, 2.31, 2.58, 2.41, 2.2, 2.47, 2.69, 2.33, 2.51, 2.63, 2.84}
	mtmDemo = [12]float64{0.12, -0.08, 0.35, 0.28, 0.14, -0.05, 0.31, 0.26, -0.11, 0.19, 0.22, 0.41}
)

const (
	// ihkAwalDemo adalah IHK bulan tertua pada seri demo.
	ihkAwalDemo = 108.4
	// ytdPerBulanDemo: pendekatan linear inflasi tahun kalender (≈1,92% pada September).
	ytdPerBulanDemo = 0.213
)

func bulanLalu(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)
}

func bulatkan(v float64, digit int) float64 {
	p := math.Pow10(digit)
	return math.Round(v*p) / p
}

// BuildInflasi mengembalikan inflasi provinsi 12 bulan yang berakhir di bulan lalu.
func BuildInflasi(now time.Time) []domain.InflasiBulanan {
	akhir := bulanLalu(now)
	rows := make([]domain.InflasiBulanan, len(yoyDemo))
	ihk := ihkAwalDemo
	for i := range rows {
		periode := akhir.AddDate(0, i-(len(yoyDemo)-1), 0)
		if i > 0 {
			ihk *= 1 + mtmDemo[i]/100
		}
		rows[i] = domain.InflasiBulanan{
			WilayahKode: domain.WilayahProvinsi, Periode: periode,
			IHK: bulatkan(ihk, 3), InflasiMtm: mtmDemo[i], InflasiYoy: yoyDemo[i],
			InflasiYtd: bulatkan(float64(periode.Month())*ytdPerBulanDemo, 2),
			Sumber:     domain.SumberDemo,
		}
	}
	return rows
}

type andilSeed struct {
	Komoditas  string
	Andil      float64
	InflasiPct float64
}

var andilDemo = []andilSeed{
	{"CBM-KRT", 0.18, 14.2}, {"BWM", 0.06, 6.8}, {"IKN-TGK", 0.04, 3.9},
	{"BRS-MED", 0.03, 1.6}, {"TMT", -0.02, -5.4}, {"KTG", -0.01, -3.1},
}

// BuildAndil mengembalikan andil komoditas pada bulan lalu.
func BuildAndil(now time.Time) []domain.AndilInflasi {
	periode := bulanLalu(now)
	rows := make([]domain.AndilInflasi, len(andilDemo))
	for i, a := range andilDemo {
		rows[i] = domain.AndilInflasi{
			Periode: periode, WilayahKode: domain.WilayahProvinsi, KomoditasKode: a.Komoditas,
			Andil: a.Andil, InflasiPct: a.InflasiPct, Sumber: domain.SumberDemo,
		}
	}
	return rows
}

type neracaSeed struct {
	Komoditas           string
	ProduksiTon, Konsum float64
}

var neracaDemo = []neracaSeed{
	{"BRS-MED", 146200, 52400}, {"KTG", 9800, 3200}, {"TMT", 4100, 2900}, {"IKN-TGK", 7400, 6100},
	{"DAG-AYM", 6200, 6000}, {"TLR", 4800, 5600}, {"CBM-KRT", 3100, 3900}, {"BWM", 900, 2600},
}

// BuildNeraca mengembalikan neraca pangan provinsi pada bulan lalu. Stok tidak
// diketahui (NULL): tidak ada data yang mendasarinya.
func BuildNeraca(now time.Time) []domain.NeracaPangan {
	periode := bulanLalu(now)
	rows := make([]domain.NeracaPangan, len(neracaDemo))
	for i, n := range neracaDemo {
		rows[i] = domain.NeracaPangan{
			KomoditasKode: n.Komoditas, WilayahKode: domain.WilayahProvinsi, Periode: periode,
			ProduksiTon: n.ProduksiTon, KonsumsiTon: n.Konsum, Sumber: domain.SumberDemo,
		}
	}
	return rows
}

type fiskalSeed struct {
	Instrumen       string
	Pagu, Realisasi int64
}

var fiskalDemo = []fiskalSeed{
	{"Belanja tidak terduga (BTT)", 120_000_000_000, 31_200_000_000},
	{"Operasi pasar murah", 24_000_000_000, 11_200_000_000},
	{"Subsidi ongkos angkut", 18_000_000_000, 9_400_000_000},
	{"Gerakan tanam pangan", 24_000_000_000, 2_400_000_000},
}

// BuildFiskal mengembalikan ruang fiskal per akhir bulan lalu.
func BuildFiskal(now time.Time) []domain.RuangFiskal {
	perTanggal := bulanLalu(now).AddDate(0, 1, -1)
	rows := make([]domain.RuangFiskal, len(fiskalDemo))
	for i, f := range fiskalDemo {
		rows[i] = domain.RuangFiskal{
			ID:    fmt.Sprintf("DEMO-RF-%s-%d", perTanggal.Format("200601"), i+1),
			Tahun: perTanggal.Year(), Instrumen: f.Instrumen, Pagu: f.Pagu, Realisasi: f.Realisasi,
			PerTanggal: perTanggal, Sumber: domain.SumberDemo,
		}
	}
	return rows
}
