package seeder

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/garda-inflasi/api/internal/cold-storage/domain"
)

const (
	// intervalTelemetri adalah jarak antarpembacaan demo; titikTelemetri pembacaan
	// per fasilitas menutup 48 jam terakhir. Jumlah baris demo karenanya tetap
	// (titikTelemetri per fasilitas beroperasi) berapa kali pun seeder dijalankan.
	intervalTelemetri = 30 * time.Minute
	titikTelemetri    = 96
	// prngTelemetri membuat derau pembacaan sama di setiap jalan.
	prngTelemetri = 20261105
)

// penyimpangan menambah suhu pada titik ke-k dari akhir (0 = pembacaan terbaru)
// untuk fasilitas yang dirancang mengalami lonjakan suhu dalam riwayat demo.
var penyimpangan = map[string]map[int]float64{
	// Gangguan pendingin CS-04 sekitar 10-12 jam lalu, kini sudah pulih.
	"CS-04": {20: 1.6, 21: 2.6, 22: 3.1, 23: 2.8, 24: 1.9, 25: 1.0},
}

// BuildTelemetri menyusun pembacaan demo 48 jam terakhir untuk fasilitas yang
// sudah punya telemetri. Pembacaan terbaru sama dengan suhu aktual fasilitas,
// supaya kartu fasilitas konsisten dengan riwayatnya. Grid waktu menempel pada
// kelipatan 30 menit sehingga menjalankan ulang tidak menggandakan baris.
func BuildTelemetri(now time.Time) []domain.TelemetriSuhu {
	akhir := now.UTC().Truncate(intervalTelemetri)
	var out []domain.TelemetriSuhu
	for idx, f := range fasilitasSeeds {
		if f.SuhuAktualC == nil {
			continue
		}
		rng := rand.New(rand.NewPCG(prngTelemetri, uint64(idx+1)))
		for i := range titikTelemetri {
			k := titikTelemetri - 1 - i // jarak dari pembacaan terbaru
			waktu := akhir.Add(-time.Duration(k) * intervalTelemetri)
			fase := 2 * math.Pi * float64(waktu.Hour()*60+waktu.Minute()) / (24 * 60)

			suhu := f.SuhuTargetC + 0.35*math.Sin(fase) + (rng.Float64()-0.5)*0.5 + penyimpangan[f.ID][k]
			if k == 0 {
				suhu = *f.SuhuAktualC
			}
			lembap := math.Min(100, math.Max(0, 82+4*math.Sin(fase+1)+(rng.Float64()-0.5)*4))
			out = append(out, domain.TelemetriSuhu{
				FasilitasID:   f.ID,
				Waktu:         waktu,
				SuhuC:         math.Round(suhu*100) / 100,
				KelembapanPct: &lembap,
				Sumber:        domain.SumberDemo,
			})
		}
	}
	return out
}

// BuildTransaksi menyusun satu transaksi MASUK untuk tiap lot demo, pada saat
// lot masuk, sehingga buku besar konsisten dengan volume awal lot.
func BuildTransaksi(now time.Time) []domain.TransaksiStok {
	stok := BuildStok(now)
	out := make([]domain.TransaksiStok, len(stok))
	for i, s := range stok {
		tujuan := s.ColdStorageID
		out[i] = domain.TransaksiStok{
			ID:                "TRX-DEMO-" + s.ID,
			LotID:             s.ID,
			Jenis:             domain.TransaksiMasuk,
			VolumeTon:         s.VolumeTon,
			FasilitasTujuanID: &tujuan,
			Waktu:             s.MasukPada,
			Alasan:            "Penerimaan awal (data demo)",
			Sumber:            domain.SumberDemo,
		}
	}
	return out
}
