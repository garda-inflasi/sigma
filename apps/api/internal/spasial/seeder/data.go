package seeder

// komoditasNeraca: komoditas yang dipantau di peta dan tonase surplus/defisit
// per poin persen deviasi harga (angka demo).
var komoditasNeraca = []struct {
	kode       string
	tonPerPoin float64
}{
	{"BRS-MED", 40},
	{"CBM-KRT", 6},
	{"CBR-RWT", 3},
	{"BWM", 5},
	{"TLR", 8},
	{"DAG-AYM", 7},
	{"KTG", 10},
	{"TMT", 9},
}

type titik struct{ komoditas, wilayah string }

// titikPanas selaras dengan alert EWS demo (internal/ews/seeder) dan pola mockMapMarkers:
// deviasi positif = harga naik (defisit), negatif = harga turun (surplus).
var titikPanas = map[titik]float64{
	{"CBM-KRT", "1171"}: 21.4,
	{"CBM-KRT", "1108"}: 19.2,
	{"CBM-KRT", "1174"}: 16.4,
	{"CBR-RWT", "1174"}: 15.6,
	{"BWM", "1107"}:     12.8,
	{"BWM", "1109"}:     9.4,
	{"TLR", "1110"}:     8.7,
	{"DAG-AYM", "1173"}: 8.2,
	{"KTG", "1106"}:     -18.0,
	{"TMT", "1117"}:     -14.6,
}

// sentraSurplus: asal rute, bergiliran (Takengon, Bener Meriah, Pidie).
var sentraSurplus = []struct{ kode, komoditas string }{
	{"1106", "CBM-KRT"},
	{"1117", "CBM-KRT"},
	{"1109", "BWM"},
}

const (
	komponenSurplus    = "surplus_musiman"
	komponenAkses      = "akses_jalan"
	komponenPermintaan = "permintaan"
)

// bobotSkor: bobot komponen skor kandidat; jumlahnya 1.
var bobotSkor = map[string]float64{
	komponenSurplus:    0.4,
	komponenAkses:      0.3,
	komponenPermintaan: 0.3,
}

var kandidatSeeds = []struct {
	id, kabupaten, lokasi, status, alasan string
	komponen                              map[string]float64
}{
	{
		"KC-1173", "1173", "Kota Langsa", "RENCANA",
		"Simpul timur menuju Sumatera Utara; permintaan tinggi dan akses jalan nasional baik.",
		map[string]float64{komponenSurplus: 85, komponenAkses: 80, komponenPermintaan: 80},
	},
	{
		"KC-1110", "1110", "Kab. Bireuen", "USULAN_BARU",
		"Persimpangan jalur Takengon-pesisir; surplus hortikultura musiman cukup besar.",
		map[string]float64{komponenSurplus: 80, komponenAkses: 75, komponenPermintaan: 75},
	},
	{
		"KC-1109", "1109", "Kab. Pidie", "USULAN_BARU",
		"Sentra bawang merah; akses baik tetapi permintaan lokal sedang.",
		map[string]float64{komponenSurplus: 70, komponenAkses: 75, komponenPermintaan: 68},
	},
}
