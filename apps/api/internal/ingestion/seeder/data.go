package seeder

import (
	"sort"

	"github.com/garda-inflasi/api/internal/ingestion/domain"
)

// ID sumber (sama dengan katalog sumber-data).
const (
	sumberBI   = "SRC-BI"
	sumberBMKG = "SRC-BMKG"
	sumberBPS  = "SRC-BPS"
)

// Seluruh alias di bawah ditulis dari pengetahuan publik tentang nama di layanan
// sumber, BELUM DIVERIFIKASI terhadap respons nyata. Alias yang tidak cocok tidak
// merusak data: baris itu dilaporkan sebagai tak terpetakan di eksekusi_sumber.

type alias struct {
	Alias, Kode string
}

// komoditasPIHPS: nama baris di tabel harga PIHPS -> kode komoditas master.
var komoditasPIHPS = []alias{
	{"beras kualitas medium i", "BRS-MED"},
	{"beras kualitas super i", "BRS-PRM"},
	{"cabai merah keriting", "CBM-KRT"},
	{"cabai rawit merah", "CBR-RWT"},
	{"bawang merah ukuran sedang", "BWM"},
	{"bawang putih ukuran sedang", "BWP"},
	{"telur ayam ras segar", "TLR"},
	{"daging ayam ras segar", "DAG-AYM"},
	{"daging sapi kualitas 1", "DAG-SPI"},
	{"minyak goreng curah", "MYK"},
	{"gula pasir lokal", "GLA"},
}

// abaikanPIHPS: baris induk kategori (tanpa harga sendiri) dan varian yang tidak dipakai
// karena master memakai satu varian utama per komoditas.
var abaikanPIHPS = []struct{ Alias, Catatan string }{
	{"beras", catInduk}, {"daging ayam", catInduk}, {"daging sapi", catInduk}, {"telur ayam", catInduk},
	{"bawang merah", catInduk}, {"bawang putih", catInduk}, {"cabai merah", catInduk}, {"cabai rawit", catInduk},
	{"minyak goreng", catInduk}, {"gula pasir", catInduk},
	{"beras kualitas bawah i", catVarian}, {"beras kualitas bawah ii", catVarian},
	{"beras kualitas medium ii", catVarian}, {"beras kualitas super ii", catVarian},
	{"cabai merah besar", catVarian}, {"cabai rawit hijau", catVarian}, {"daging sapi kualitas 2", catVarian},
	{"minyak goreng kemasan bermerk 1", catVarian}, {"minyak goreng kemasan bermerk 2", catVarian},
	{"gula pasir kualitas premium", catVarian},
}

const (
	catInduk  = "baris induk kategori, tanpa harga sendiri"
	catVarian = "varian tidak dipakai; master memakai satu varian utama per komoditas"
)

// wilayahBMKG: kode adm4 BMKG (desa/kelurahan) titik pantau -> kabupaten/kota BPS.
// Titik awal adalah desa pertama pada kecamatan pertama tiap wilayah (pola kode
// Kemendagri <kab>.01.2001), BELUM TERVERIFIKASI: ganti dengan titik yang mewakili
// wilayah lewat `cmd/ingest pemetaan`. Kode yang tidak dikenal BMKG menghasilkan
// galat per permintaan di eksekusi, bukan data salah.
var wilayahBMKG = func() []alias {
	kab := []string{
		"1101", "1102", "1103", "1104", "1105", "1106", "1107", "1108", "1109", "1110", "1111", "1112",
		"1113", "1114", "1115", "1116", "1117", "1118", "1171", "1172", "1173", "1174", "1175",
	}
	res := make([]alias, len(kab))
	for i, k := range kab {
		res[i] = alias{k[:2] + "." + k[2:] + ".01.2001", k}
	}
	return res
}()

// wilayahBPS: label wilayah pada tabel IHK/inflasi BPS -> kode wilayah. Kota IHK di
// Aceh: Banda Aceh, Lhokseumawe, dan Meulaboh (Kabupaten Aceh Barat).
var wilayahBPS = []alias{
	{"aceh", "11"}, {"provinsi aceh", "11"},
	{"banda aceh", "1171"}, {"kota banda aceh", "1171"},
	{"lhokseumawe", "1174"}, {"kota lhokseumawe", "1174"},
	{"meulaboh", "1107"}, {"kota meulaboh", "1107"},
}

// komoditasBPS: label komoditas pada tabel andil inflasi BPS -> kode komoditas master.
var komoditasBPS = []alias{
	{"beras", "BRS-MED"}, {"cabai merah", "CBM-KRT"}, {"cabai rawit", "CBR-RWT"},
	{"bawang merah", "BWM"}, {"bawang putih", "BWP"}, {"telur ayam ras", "TLR"},
	{"daging ayam ras", "DAG-AYM"}, {"daging sapi", "DAG-SPI"}, {"minyak goreng", "MYK"},
	{"gula pasir", "GLA"}, {"kentang", "KTG"}, {"tomat sayur", "TMT"}, {"tomat", "TMT"},
}

// Pemetaan mengembalikan seluruh pemetaan referensi, terurut agar deterministik.
func Pemetaan() []domain.PemetaanSumber {
	var rows []domain.PemetaanSumber
	tambah := func(sumber, jenis string, daftar []alias, catatan string) {
		for _, a := range daftar {
			rows = append(rows, domain.PemetaanSumber{SumberID: sumber, Jenis: jenis, Alias: a.Alias, KodeMaster: new(a.Kode), Catatan: new(catatan)})
		}
	}
	tambah(sumberBI, domain.JenisKomoditas, komoditasPIHPS, "nama baris tabel harga PIHPS")
	for _, a := range abaikanPIHPS {
		rows = append(rows, domain.PemetaanSumber{SumberID: sumberBI, Jenis: domain.JenisKomoditas, Alias: a.Alias, Diabaikan: true, Catatan: new(a.Catatan)})
	}
	tambah(sumberBMKG, domain.JenisWilayah, wilayahBMKG, "titik pantau awal, belum terverifikasi")
	tambah(sumberBPS, domain.JenisWilayah, wilayahBPS, "label wilayah tabel IHK/inflasi BPS")
	tambah(sumberBPS, domain.JenisKomoditas, komoditasBPS, "label komoditas tabel andil inflasi BPS")

	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.SumberID != b.SumberID {
			return a.SumberID < b.SumberID
		}
		if a.Jenis != b.Jenis {
			return a.Jenis < b.Jenis
		}
		return a.Alias < b.Alias
	})
	return rows
}
