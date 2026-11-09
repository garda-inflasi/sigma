package domain

import (
	"sort"

	sumberdomain "github.com/garda-inflasi/api/internal/sumber-data/domain"
)

// Kode sumber (sama dengan katalog sumber-data) dan UUID stabil barisnya, yang dipakai sumber_id.
const (
	KodeBI      = "SRC-BI"
	KodeBMKG    = "SRC-BMKG"
	KodeBPS     = "SRC-BPS"
	KodeOWM     = "SRC-OWM"
	KodeBapanas = "SRC-BAPANAS"
)

var (
	SumberBI      = sumberdomain.IDSumber(KodeBI)
	SumberBMKG    = sumberdomain.IDSumber(KodeBMKG)
	SumberBPS     = sumberdomain.IDSumber(KodeBPS)
	SumberOWM     = sumberdomain.IDSumber(KodeOWM)
	SumberBapanas = sumberdomain.IDSumber(KodeBapanas)
)

type alias struct {
	Alias, Kode string
}

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

const (
	catInduk  = "baris induk kategori, tanpa harga sendiri"
	catVarian = "varian tidak dipakai; master memakai satu varian utama per komoditas"
)

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

// catatanTitikBMKG menandai baris titik pantau BMKG bawaan seed. Operator yang mengubahnya lewat
// `gardactl ingest pemetaan` menimpa catatan ini. Seed membuang baris tebakan lama hanya bila catatannya masih
// "titik pantau awal, belum terverifikasi" (lihat db/seeds/reference/05_ingestion_mapping.sql).
const catatanTitikBMKG = "titik pantau awal; adm4 diverifikasi ke api.bmkg.go.id (2026-11-10)"

// wilayahBMKG adalah satu titik pantau (desa/kelurahan) per kabupaten/kota. Kode adm4 memakai kode Kemendagri,
// BUKAN kode BPS master: urutan kabupaten berbeda (mis. Simeulue BPS 1101 = Kemendagri 11.09; Langsa dan
// Lhokseumawe tertukar: BPS 1173/1174 = Kemendagri 11.74/11.73). Awalan adm4 karena itu tidak boleh diturunkan
// dari kode master. Setiap kode sudah dicoba ke api.bmkg.go.id; adm2 dan nama kabupatennya cocok dengan master.
// Titik dipilih sah, bukan wakil terbaik: ganti dengan kelurahan yang lebih representatif lewat
// `gardactl ingest pemetaan SRC-BMKG WILAYAH <adm4> <kode>`.
var wilayahBMKG = []alias{
	{"11.09.02.2001", "1101"}, // Simeulue: Salang / Ujung Salang
	{"11.10.01.2001", "1102"}, // Aceh Singkil: Pulau Banyak / Pulau Balai
	{"11.01.01.2001", "1103"}, // Aceh Selatan: Bakongan / Keude Bakongan
	{"11.02.01.2001", "1104"}, // Aceh Tenggara: Lawe Alas / Engkeran
	{"11.03.01.2001", "1105"}, // Aceh Timur: Darul Aman / Alue Luddin Sa
	{"11.04.02.2001", "1106"}, // Aceh Tengah: Silih Nara / Arul Gele
	{"11.05.01.2001", "1107"}, // Aceh Barat: Johan Pahlawan / Suak Indrapuri
	{"11.06.01.2001", "1108"}, // Aceh Besar: Lhoong / Meunasah Krueng Kala
	{"11.07.03.2001", "1109"}, // Pidie: Batee / Dayah Tuha
	{"11.11.01.2001", "1110"}, // Bireuen: Samalanga / Keude Aceh
	{"11.08.01.2001", "1111"}, // Aceh Utara: Baktiya / Keude Alue Ie Puteh
	{"11.12.02.2001", "1112"}, // Aceh Barat Daya: Tangan-Tangan / Adan
	{"11.13.01.2001", "1113"}, // Gayo Lues: Blangkejeren / Blangkejeren
	{"11.16.01.2001", "1114"}, // Aceh Tamiang: Manyak Payed / Simpang Lhee
	{"11.15.02.2001", "1115"}, // Nagan Raya: Seunagan / Jeuram
	{"11.14.01.2001", "1116"}, // Aceh Jaya: Teunom / Keude Teunom
	{"11.17.01.2001", "1117"}, // Bener Meriah: Pintu Rime Gayo / Blang Rakal
	{"11.18.01.2001", "1118"}, // Pidie Jaya: Meureudu / Meunasah Balek
	{"11.71.01.2001", "1171"}, // Kota Banda Aceh: Baiturrahman / Kampung Baru
	{"11.72.03.2001", "1172"}, // Kota Sabang: Sukamakmue / Iboih
	{"11.74.03.2001", "1173"}, // Kota Langsa: Langsa Kota / Teungoh
	{"11.73.01.2002", "1174"}, // Kota Lhokseumawe: Muara Dua / Alue Awe
	{"11.75.01.2001", "1175"}, // Kota Subulussalam: Simpang Kiri / Subulussalam
}

var wilayahBPS = []alias{
	{"aceh", "11"}, {"provinsi aceh", "11"},
	{"banda aceh", "1171"}, {"kota banda aceh", "1171"},
	{"lhokseumawe", "1174"}, {"kota lhokseumawe", "1174"},
	{"meulaboh", "1107"}, {"kota meulaboh", "1107"},
}

var komoditasBPS = []alias{
	{"beras", "BRS-MED"}, {"cabai merah", "CBM-KRT"}, {"cabai rawit", "CBR-RWT"},
	{"bawang merah", "BWM"}, {"bawang putih", "BWP"}, {"telur ayam ras", "TLR"},
	{"daging ayam ras", "DAG-AYM"}, {"daging sapi", "DAG-SPI"}, {"minyak goreng", "MYK"},
	{"gula pasir", "GLA"}, {"kentang", "KTG"}, {"tomat sayur", "TMT"}, {"tomat", "TMT"},
}

var wilayahOWM = []alias{
	{"banda aceh", "1171"},
	{"lhokseumawe", "1174"},
	{"langsa", "1173"},
	{"sabang", "1172"},
	{"subulussalam", "1175"},
	{"aceh barat", "1105"},
	{"aceh besar", "1106"},
	{"aceh selatan", "1101"},
	{"aceh tengah", "1104"},
	{"aceh utara", "1108"},
	{"pidie", "1107"},
	{"bireuen", "1111"},
}

var komoditasBapanas = []alias{
	{"beras medium", "BRS-MED"},
	{"beras premium", "BRS-PRM"},
	{"cabai merah keriting", "CBM-KRT"},
	{"cabai rawit merah", "CBR-RWT"},
	{"bawang merah", "BWM"},
	{"bawang putih bonggol", "BWP"},
	{"daging ayam ras", "DAG-AYM"},
	{"daging sapi murni", "DAG-SPI"},
	{"telur ayam ras", "TLR"},
	{"gula konsumsi", "GLA"},
	{"minyak goreng curah", "MYK"},
	{"minyak goreng kemasan sederhana", "MYK"},
}

var wilayahBapanas = func() []alias {
	kab := []string{
		"1101", "1102", "1103", "1104", "1105", "1106", "1107", "1108", "1109", "1110", "1111", "1112",
		"1113", "1114", "1115", "1116", "1117", "1118", "1171", "1172", "1173", "1174", "1175",
	}
	res := make([]alias, 0, len(kab)*2)
	for _, k := range kab {
		res = append(res, alias{k, k})
	}
	namaKhusus := []alias{
		{"kota banda aceh", "1171"},
		{"kota lhokseumawe", "1174"},
		{"kota langsa", "1173"},
		{"kota sabang", "1172"},
		{"kota subulussalam", "1175"},
	}
	res = append(res, namaKhusus...)
	return res
}()

// SemuaPemetaan mengembalikan seluruh pemetaan referensi, terurut agar deterministik.
func SemuaPemetaan() []PemetaanSumber {
	var rows []PemetaanSumber
	tambah := func(sumber, jenis string, daftar []alias, catatan string) {
		for _, a := range daftar {
			k := a.Kode
			c := catatan
			rows = append(rows, PemetaanSumber{SumberID: sumber, Jenis: jenis, Alias: a.Alias, KodeMaster: &k, Catatan: &c})
		}
	}
	tambah(SumberBI, JenisKomoditas, komoditasPIHPS, "nama baris tabel harga PIHPS")
	for _, a := range abaikanPIHPS {
		c := a.Catatan
		rows = append(rows, PemetaanSumber{SumberID: SumberBI, Jenis: JenisKomoditas, Alias: a.Alias, Diabaikan: true, Catatan: &c})
	}
	tambah(SumberBMKG, JenisWilayah, wilayahBMKG, catatanTitikBMKG)
	tambah(SumberBPS, JenisWilayah, wilayahBPS, "label wilayah tabel IHK/inflasi BPS")
	tambah(SumberBPS, JenisKomoditas, komoditasBPS, "label komoditas tabel andil inflasi BPS")
	tambah(SumberOWM, JenisWilayah, wilayahOWM, "alias wilayah OpenWeather")
	tambah(SumberBapanas, JenisKomoditas, komoditasBapanas, "komoditas Panel Harga Bapanas")
	tambah(SumberBapanas, JenisWilayah, wilayahBapanas, "wilayah Panel Harga Bapanas")

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
