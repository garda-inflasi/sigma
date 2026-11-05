package seeder

import (
	"math"
	"time"

	"github.com/garda-inflasi/api/internal/kpi/domain"
)

// periodeTargetMulai adalah awal berlaku target bawaan (awal program).
var periodeTargetMulai = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

type kpiSeed struct {
	Kode, Kelompok, Nama, Deskripsi, Rumus, Satuan, Arah string
	// SumberQuery kosong = belum ada penghitung otomatis (menunggu model atau data lain, 16-B).
	SumberQuery string
	// Target nil = target belum ditetapkan.
	Target *float64
}

// Target mengikuti PRD §12 (12.1–12.5). Baseline sengaja tidak diisi: itu
// keputusan pemangku kepentingan (docs/todos/16-kpi.md). Ambang waspada
// bawaan 10% dari target (NAIK: 90% target, TURUN: 110% target).
var kpiSeeds = []kpiSeed{
	// ── Data & sistem
	{Kode: "DATA-PRODUSEN-TERVERIFIKASI", Kelompok: domain.KelompokDataSistem, Nama: "Kelengkapan produsen terverifikasi", Satuan: "%", Arah: domain.ArahNaik,
		Deskripsi:   "Porsi produsen (kode pedagang) yang entri harganya lolos validasi.",
		Rumus:       "100 × produsen berentri DIVALIDASI/DIPUBLIKASI ÷ semua produsen berentri, pada bulan itu",
		SumberQuery: domain.QProdusenTerverifikasi, Target: new(95.0)},
	{Kode: "DATA-KETEPATAN", Kelompok: domain.KelompokDataSistem, Nama: "Ketepatan data harga", Satuan: "%", Arah: domain.ArahNaik,
		Deskripsi:   "Porsi entri harga yang sudah diputuskan dan lolos validasi. Target belum ditetapkan.",
		Rumus:       "100 × entri DIVALIDASI/DIPUBLIKASI ÷ entri DIVALIDASI/DIPUBLIKASI/DITOLAK, pada bulan itu",
		SumberQuery: domain.QKetepatanData},
	{Kode: "DATA-KELENGKAPAN-HARIAN", Kelompok: domain.KelompokDataSistem, Nama: "Kelengkapan entri harian", Satuan: "%", Arah: domain.ArahNaik,
		Deskripsi:   "Rata-rata harian pasangan pasar aktif × komoditas pilot yang punya entri tervalidasi.",
		Rumus:       "rata-rata harian dari 100 × pasangan (pasar aktif, komoditas pilot) berentri tervalidasi ÷ seluruh pasangan",
		SumberQuery: domain.QKelengkapanHarian, Target: new(95.0)},
	{Kode: "DATA-AKURASI-PRODUKSI", Kelompok: domain.KelompokDataSistem, Nama: "Akurasi proyeksi produksi", Satuan: "%", Arah: domain.ArahNaik,
		Deskripsi: "Akurasi model proyeksi produksi. Menunggu model dan data evaluasi (16-B).",
		Rumus:     "100 − MAPE proyeksi produksi", Target: new(98.0)},
	{Kode: "DATA-MAPE-HARGA", Kelompok: domain.KelompokDataSistem, Nama: "MAPE prediksi harga", Satuan: "%", Arah: domain.ArahTurun,
		Deskripsi: "Galat persentase absolut rata-rata model prediksi harga. Menunggu data evaluasi model (16-B).",
		Rumus:     "rata-rata |prediksi − aktual| ÷ aktual × 100", Target: new(5.4)},
	{Kode: "DATA-F1-EWS", Kelompok: domain.KelompokDataSistem, Nama: "F1-score EWS", Satuan: "", Arah: domain.ArahNaik,
		Deskripsi: "Kinerja deteksi peringatan dini. Menunggu data evaluasi EWS (16-B).",
		Rumus:     "2 × presisi × recall ÷ (presisi + recall)", Target: new(0.88)},
	{Kode: "DATA-AUC-EWS", Kelompok: domain.KelompokDataSistem, Nama: "AUC-ROC EWS", Satuan: "", Arah: domain.ArahNaik,
		Deskripsi: "Daya pembeda model EWS. Menunggu data evaluasi EWS (16-B).",
		Rumus:     "luas di bawah kurva ROC", Target: new(0.94)},
	{Kode: "DATA-RMSE-SHELFLIFE", Kelompok: domain.KelompokDataSistem, Nama: "RMSE shelf-life", Satuan: "hari", Arah: domain.ArahTurun,
		Deskripsi: "Galat model masa simpan. Menunggu data evaluasi model (16-B).",
		Rumus:     "akar rata-rata (prediksi − aktual)² dalam hari", Target: new(1.0)},

	// ── Logistik & infrastruktur
	{Kode: "LOG-UTILISASI-CS", Kelompok: domain.KelompokLogistik, Nama: "Utilisasi cold storage", Satuan: "%", Arah: domain.ArahNaik,
		Deskripsi:   "Volume stok tersimpan terhadap kapasitas fasilitas operasional. Pendekatan: lot dianggap tersimpan sampai batas masa simpannya karena stok keluar belum dicatat.",
		Rumus:       "100 × Σ volume lot aktif pada akhir periode ÷ Σ kapasitas fasilitas OPERASIONAL",
		SumberQuery: domain.QUtilisasiColdStorage, Target: new(75.0)},
	{Kode: "LOG-RISIKO-FOODLOSS", Kelompok: domain.KelompokLogistik, Nama: "Risiko food loss stok", Satuan: "%", Arah: domain.ArahTurun,
		Deskripsi:   "Estimasi risiko food loss stok cold storage (bukan kerugian terealisasi). Target belum ditetapkan.",
		Rumus:       "100 × Σ(volume × risiko) ÷ Σ volume lot aktif pada akhir periode",
		SumberQuery: domain.QRisikoFoodLoss},
	{Kode: "LOG-PENURUNAN-FOODLOSS", Kelompok: domain.KelompokLogistik, Nama: "Penurunan food loss", Satuan: "%", Arah: domain.ArahNaik,
		Deskripsi: "Penurunan food loss terhadap baseline. Menunggu penetapan baseline dan data kerugian terealisasi (16-B).",
		Rumus:     "100 × (baseline − food loss periode) ÷ baseline", Target: new(60.0)},
	{Kode: "LOG-VOLUME-AGREGASI", Kelompok: domain.KelompokLogistik, Nama: "Volume teragregasi", Satuan: "ton", Arah: domain.ArahNaik,
		Deskripsi: "Total volume yang diagregasi per bulan. Menunggu data agregasi (16-B).",
		Rumus:     "Σ volume agregasi dalam bulan", Target: new(10000.0)},

	// ── Dampak ekonomi
	{Kode: "EKO-VOLATILITAS", Kelompok: domain.KelompokDampakEkonomi, Nama: "Volatilitas harga produsen (CV)", Satuan: "%", Arah: domain.ArahTurun,
		Deskripsi: "Koefisien variasi harga produsen. Menunggu baseline dan periode evaluasi (16-B).",
		Rumus:     "simpangan baku ÷ rata-rata harga × 100", Target: new(7.8)},
	{Kode: "EKO-MARGIN", Kelompok: domain.KelompokDampakEkonomi, Nama: "Margin produsen–konsumen", Satuan: "%", Arah: domain.ArahTurun,
		Deskripsi: "Selisih harga konsumen dan produsen. Menunggu baseline dan periode evaluasi (16-B).",
		Rumus:     "(harga konsumen − harga produsen) ÷ harga konsumen × 100", Target: new(32.0)},
	{Kode: "EKO-EFISIENSI-BIAYA", Kelompok: domain.KelompokDampakEkonomi, Nama: "Efisiensi biaya logistik", Satuan: "%", Arah: domain.ArahNaik,
		Deskripsi: "Penghematan biaya logistik/rute. Menunggu data rute (16-B).",
		Rumus:     "100 × (biaya awal − biaya periode) ÷ biaya awal", Target: new(25.0)},

	// ── Tindak lanjut
	{Kode: "TL-REKOMENDASI-AI", Kelompok: domain.KelompokTindakLanjut, Nama: "Rekomendasi AI ditindaklanjuti", Satuan: "%", Arah: domain.ArahNaik,
		Deskripsi:   "Porsi intervensi berasal dari rekomendasi AI yang sudah selesai dieksekusi.",
		Rumus:       "100 × intervensi rekomendasi AI berstatus selesai ÷ semua intervensi rekomendasi AI yang dibuat pada bulan itu",
		SumberQuery: domain.QRekomendasiAI, Target: new(78.0)},
	{Kode: "TL-KOORDINASI", Kelompok: domain.KelompokTindakLanjut, Nama: "Indeks kelancaran koordinasi lintas lembaga", Satuan: "/5", Arah: domain.ArahNaik,
		Deskripsi: "Skor survei koordinasi lintas lembaga. Menunggu data survei (16-B).",
		Rumus:     "rata-rata skor survei (1–5)", Target: new(4.2)},
}

// frekuensi seragam: seluruh KPI dievaluasi bulanan.
const frekuensiBawaan = "BULANAN"

// Definisi mengembalikan definisi KPI dengan urutan sesuai daftar.
func Definisi() []domain.Definisi {
	rows := make([]domain.Definisi, len(kpiSeeds))
	for i, k := range kpiSeeds {
		var query *string
		if k.SumberQuery != "" {
			query = new(k.SumberQuery)
		}
		rows[i] = domain.Definisi{
			Kode: k.Kode, Kelompok: k.Kelompok, Nama: k.Nama, Deskripsi: k.Deskripsi, Rumus: k.Rumus,
			Satuan: k.Satuan, ArahBaik: k.Arah, Frekuensi: frekuensiBawaan, SumberQuery: query, Urutan: (i + 1) * 10,
		}
	}
	return rows
}

// Target mengembalikan target bawaan (hanya KPI yang punya target), tanpa baseline.
func Target() []domain.Target {
	var rows []domain.Target
	for _, k := range kpiSeeds {
		if k.Target == nil {
			continue
		}
		faktor := 0.9
		if k.Arah == domain.ArahTurun {
			faktor = 1.1
		}
		ambang := math.Round(*k.Target*faktor*1e4) / 1e4
		rows = append(rows, domain.Target{KpiKode: k.Kode, PeriodeMulai: periodeTargetMulai, Target: *k.Target, AmbangWaspada: &ambang})
	}
	return rows
}
