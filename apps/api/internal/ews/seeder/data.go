package seeder

import (
	"math"

	"github.com/garda-inflasi/api/internal/ews/domain"
)

// Jenis alert (enum jenis_ews).
const (
	jenisLonjakan = "LONJAKAN_HARGA"
	jenisStok     = "KELANGKAAN_STOK"
	jenisCuaca    = "ANOMALI_CUACA"
	jenisSurplus  = "OVER_SUPPLY"
	jenisData     = "ANOMALI_DATA"
)

// keteranganAmbang menandai bahwa angka awal bukan hasil kalibrasi.
const keteranganAmbang = "Ambang awal, harus dikalibrasi bersama BI/BPS."

// Ambang bersifat sama untuk komoditas yang berkarakter serupa. Satuan: lonjakan
// harga = % di atas rata-rata jendela; kelangkaan stok = % penurunan stok jendela;
// anomali cuaca = curah hujan kumulatif jendela (mm).
type ambang struct{ waspada, siaga, kritis float64 }

type aturanSeed struct {
	komoditas string
	jenis     string
	ambang    ambang
	jendela   int
}

// aturanSeeds: tiga komoditas pilot fase 1 x tiga jenis alert terukur.
var aturanSeeds = []aturanSeed{
	{"BRS-MED", jenisLonjakan, ambang{3, 5, 8}, 7},
	{"BRS-MED", jenisStok, ambang{20, 35, 50}, 5},
	{"BRS-MED", jenisCuaca, ambang{150, 220, 300}, 10},
	{"CBM-KRT", jenisLonjakan, ambang{8, 12, 18}, 7},
	{"CBM-KRT", jenisStok, ambang{25, 40, 55}, 5},
	{"CBM-KRT", jenisCuaca, ambang{100, 140, 200}, 10},
	{"BWM", jenisLonjakan, ambang{8, 12, 18}, 7},
	{"BWM", jenisStok, ambang{25, 40, 55}, 5},
	{"BWM", jenisCuaca, ambang{100, 140, 200}, 10},
}

// ambangBawaan dipakai komoditas tanpa aturan sendiri (lonjakan harga, hortikultura).
var ambangBawaan = ambang{8, 12, 18}

// aturanKode: kode tampilan aturan (kolom kode, ID lama) = kode jenis pendek + kode komoditas.
func aturanKode(jenis, komoditas string) string {
	short := map[string]string{jenisLonjakan: "LH", jenisStok: "KS", jenisCuaca: "CU"}[jenis]
	return "ATR-" + short + "-" + komoditas
}

type alertSeed struct {
	kode      string
	jenis     string
	komoditas string // kosong = tanpa komoditas
	kabupaten string
	nilai     float64
	judul     string
	pesan     string
	menitLalu int
	status    string
	skor      int
	sumber    string
}

const (
	sumberModel  = "Model EWS v2.3 + entri lapangan"
	pesanSurplus = "Harga turun di bawah koridor. Pertimbangkan penyerapan ke cold storage."
)

func lonjakan(kode, komoditas, nama, kabupaten string, nilai float64, harga string, menit int, status string) alertSeed {
	return alertSeed{
		kode: kode, jenis: jenisLonjakan, komoditas: komoditas, kabupaten: kabupaten, nilai: nilai,
		judul:     "Lonjakan harga " + nama,
		pesan:     "Harga konsumen naik di atas rata-rata 7 hari. Harga terakhir Rp" + harga + "/kg.",
		menitLalu: menit, status: status, skor: min(98, int(40+nilai*2.4+0.5)), sumber: sumberModel,
	}
}

func surplus(kode, komoditas, nama, kabupaten string, nilai float64, menit int) alertSeed {
	return alertSeed{
		kode: kode, jenis: jenisSurplus, komoditas: komoditas, kabupaten: kabupaten, nilai: nilai,
		judul: "Indikasi surplus " + nama, pesan: pesanSurplus,
		menitLalu: menit, status: domain.StatusBaru, skor: int(30 + math.Abs(nilai)*2 + 0.5), sumber: "Model EWS v2.3",
	}
}

// alertSeeds meniru mockEws (apps/web/src/lib/api/mock.ts): 12 lonjakan harga
// (4 baru, 5 diakui, 3 dieskalasi), 2 indikasi surplus, dan tiga alert tetap.
var alertSeeds = []alertSeed{
	lonjakan("EWS-1000", "CBM-KRT", "cabai merah keriting", "1171", 21.4, "66.500", 4, domain.StatusBaru),
	lonjakan("EWS-1001", "CBM-KRT", "cabai merah keriting", "1108", 19.2, "64.800", 27, domain.StatusBaru),
	lonjakan("EWS-1002", "CBR-RWT", "cabai rawit merah", "1174", 15.6, "71.200", 50, domain.StatusBaru),
	lonjakan("EWS-1003", "BWM", "bawang merah", "1107", 12.8, "49.700", 73, domain.StatusBaru),
	lonjakan("EWS-1004", "CBM-KRT", "cabai merah keriting", "1106", 11.3, "58.400", 96, domain.StatusDiakui),
	lonjakan("EWS-1005", "BWM", "bawang merah", "1109", 9.4, "47.300", 119, domain.StatusDiakui),
	lonjakan("EWS-1006", "TLR", "telur ayam ras", "1110", 8.7, "32.100", 142, domain.StatusDiakui),
	lonjakan("EWS-1007", "DAG-AYM", "daging ayam ras", "1173", 8.2, "39.600", 165, domain.StatusDiakui),
	lonjakan("EWS-1008", "BRS-MED", "beras medium", "1174", 4.2, "14.500", 188, domain.StatusDiakui),
	lonjakan("EWS-1009", "CBR-RWT", "cabai rawit merah", "1171", 13.9, "68.900", 211, domain.StatusDieskalasi),
	lonjakan("EWS-1010", "CBM-KRT", "cabai merah keriting", "1174", 16.4, "62.700", 234, domain.StatusDieskalasi),
	lonjakan("EWS-1011", "BWM", "bawang merah", "1171", 10.2, "46.800", 257, domain.StatusDieskalasi),
	surplus("EWS-1012", "KTG", "kentang", "1106", -18.0, 280),
	surplus("EWS-1013", "TMT", "tomat", "1117", -14.6, 311),
	{
		kode: "EWS-0901", jenis: jenisCuaca, komoditas: "CBM-KRT", kabupaten: "1106", nilai: 148,
		judul:     "Curah hujan ekstrem di sentra Gayo",
		pesan:     "Curah hujan 2,1× normal selama 10 hari. Risiko gagal panen cabai & tomat meningkat.",
		menitLalu: 95, status: domain.StatusDiakui, skor: 81, sumber: "BMKG + OpenWeather",
	},
	{
		kode: "EWS-0902", jenis: jenisStok, komoditas: "CBM-KRT", kabupaten: "1108", nilai: -32,
		judul:     "Stok cabai di Cold Storage Lambaro menipis",
		pesan:     "Stok turun 32% dalam 5 hari; cukup untuk ±4 hari permintaan Banda Aceh.",
		menitLalu: 140, status: domain.StatusBaru, skor: 74, sumber: "Telemetri cold storage",
	},
	{
		kode: "EWS-0903", jenis: jenisData, kabupaten: "1104", nilai: 3,
		judul:     "Entri harga di luar geofence",
		pesan:     "3 entri tercatat lebih dari 1 km dari Pasar Kutacane. Perlu verifikasi koordinator.",
		menitLalu: 210, status: domain.StatusSelesai, skor: 22, sumber: "Validasi otomatis",
	},
}
