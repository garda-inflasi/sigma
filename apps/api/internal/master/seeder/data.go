package seeder

import "github.com/garda-inflasi/api/internal/master/domain"

// Data referensi Aceh. Kode wilayah mengikuti BPS; kode komoditas, nama, dan
// koordinat sama dengan apps/web/src/lib/data/reference.ts (dijaga tes fixture
// packages/contracts/master-reference.json). Koordinat bersifat perkiraan
// (ibu kota/pasar) untuk geofence dan peta; ganti dengan data geospasial resmi
// bila tersedia.

// ProvinsiAceh adalah satu-satunya provinsi cakupan program.
var ProvinsiAceh = domain.Provinsi{Kode: "11", Nama: "ACEH"}

// kabupatenAceh: 18 kabupaten dan 5 kota se-Aceh.
var kabupatenAceh = []domain.Kabupaten{
	{Kode: "1101", Nama: "KABUPATEN SIMEULUE", Jenis: "Kabupaten", Ibukota: "Sinabang", Lat: 2.48, Lng: 96.38},
	{Kode: "1102", Nama: "KABUPATEN ACEH SINGKIL", Jenis: "Kabupaten", Ibukota: "Singkil", Lat: 2.28, Lng: 97.79},
	{Kode: "1103", Nama: "KABUPATEN ACEH SELATAN", Jenis: "Kabupaten", Ibukota: "Tapaktuan", Lat: 3.26, Lng: 97.18},
	{Kode: "1104", Nama: "KABUPATEN ACEH TENGGARA", Jenis: "Kabupaten", Ibukota: "Kutacane", Lat: 3.49, Lng: 97.8},
	{Kode: "1105", Nama: "KABUPATEN ACEH TIMUR", Jenis: "Kabupaten", Ibukota: "Idi Rayeuk", Lat: 4.96, Lng: 97.77},
	{Kode: "1106", Nama: "KABUPATEN ACEH TENGAH", Jenis: "Kabupaten", Ibukota: "Takengon", Lat: 4.62, Lng: 96.85},
	{Kode: "1107", Nama: "KABUPATEN ACEH BARAT", Jenis: "Kabupaten", Ibukota: "Meulaboh", Lat: 4.14, Lng: 96.13},
	{Kode: "1108", Nama: "KABUPATEN ACEH BESAR", Jenis: "Kabupaten", Ibukota: "Kota Jantho", Lat: 5.3, Lng: 95.63},
	{Kode: "1109", Nama: "KABUPATEN PIDIE", Jenis: "Kabupaten", Ibukota: "Sigli", Lat: 5.38, Lng: 95.96},
	{Kode: "1110", Nama: "KABUPATEN BIREUEN", Jenis: "Kabupaten", Ibukota: "Bireuen", Lat: 5.2, Lng: 96.7},
	{Kode: "1111", Nama: "KABUPATEN ACEH UTARA", Jenis: "Kabupaten", Ibukota: "Lhoksukon", Lat: 5.05, Lng: 97.32},
	{Kode: "1112", Nama: "KABUPATEN ACEH BARAT DAYA", Jenis: "Kabupaten", Ibukota: "Blangpidie", Lat: 3.74, Lng: 96.84},
	{Kode: "1113", Nama: "KABUPATEN GAYO LUES", Jenis: "Kabupaten", Ibukota: "Blangkejeren", Lat: 3.98, Lng: 97.35},
	{Kode: "1114", Nama: "KABUPATEN ACEH TAMIANG", Jenis: "Kabupaten", Ibukota: "Karang Baru", Lat: 4.29, Lng: 98.05},
	{Kode: "1115", Nama: "KABUPATEN NAGAN RAYA", Jenis: "Kabupaten", Ibukota: "Suka Makmue", Lat: 4.13, Lng: 96.47},
	{Kode: "1116", Nama: "KABUPATEN ACEH JAYA", Jenis: "Kabupaten", Ibukota: "Calang", Lat: 4.63, Lng: 95.58},
	{Kode: "1117", Nama: "KABUPATEN BENER MERIAH", Jenis: "Kabupaten", Ibukota: "Simpang Tiga Redelong", Lat: 4.73, Lng: 96.87},
	{Kode: "1118", Nama: "KABUPATEN PIDIE JAYA", Jenis: "Kabupaten", Ibukota: "Meureudu", Lat: 5.25, Lng: 96.25},
	{Kode: "1171", Nama: "KOTA BANDA ACEH", Jenis: "Kota", Ibukota: "Banda Aceh", Lat: 5.55, Lng: 95.32},
	{Kode: "1172", Nama: "KOTA SABANG", Jenis: "Kota", Ibukota: "Sabang", Lat: 5.89, Lng: 95.32},
	{Kode: "1173", Nama: "KOTA LANGSA", Jenis: "Kota", Ibukota: "Langsa", Lat: 4.47, Lng: 97.97},
	{Kode: "1174", Nama: "KOTA LHOKSEUMAWE", Jenis: "Kota", Ibukota: "Lhokseumawe", Lat: 5.18, Lng: 97.15},
	{Kode: "1175", Nama: "KOTA SUBULUSSALAM", Jenis: "Kota", Ibukota: "Subulussalam", Lat: 2.64, Lng: 98.0},
}

// pasarAceh: 12 pasar utama (kode sama dengan mode mock web) ditambah satu pasar
// tradisional untuk tiap kabupaten/kota yang belum punya, agar seluruh 23 wilayah tercakup.
var pasarAceh = []domain.Pasar{
	{Kode: "PSR-1171-01", Nama: "Pasar Peunayong", KabupatenKode: "1171", Lat: 5.5603, Lng: 95.3237, RadiusMeter: 400},
	{Kode: "PSR-1171-02", Nama: "Pasar Al-Mahirah Lamdingin", KabupatenKode: "1171", Lat: 5.5765, Lng: 95.3352, RadiusMeter: 400},
	{Kode: "PSR-1108-01", Nama: "Pasar Induk Lambaro", KabupatenKode: "1108", Lat: 5.5085, Lng: 95.364, RadiusMeter: 600},
	{Kode: "PSR-1174-01", Nama: "Pasar Inpres Lhokseumawe", KabupatenKode: "1174", Lat: 5.179, Lng: 97.146, RadiusMeter: 450},
	{Kode: "PSR-1110-01", Nama: "Pasar Inpres Bireuen", KabupatenKode: "1110", Lat: 5.203, Lng: 96.703, RadiusMeter: 450},
	{Kode: "PSR-1109-01", Nama: "Pasar Kota Sigli", KabupatenKode: "1109", Lat: 5.386, Lng: 95.96, RadiusMeter: 450},
	{Kode: "PSR-1107-01", Nama: "Pasar Bina Usaha Meulaboh", KabupatenKode: "1107", Lat: 4.145, Lng: 96.129, RadiusMeter: 450},
	{Kode: "PSR-1106-01", Nama: "Pasar Inpres Takengon", KabupatenKode: "1106", Lat: 4.627, Lng: 96.846, RadiusMeter: 450},
	{Kode: "PSR-1173-01", Nama: "Pasar Kota Langsa", KabupatenKode: "1173", Lat: 4.47, Lng: 97.968, RadiusMeter: 450},
	{Kode: "PSR-1103-01", Nama: "Pasar Tapaktuan", KabupatenKode: "1103", Lat: 3.26, Lng: 97.183, RadiusMeter: 400},
	{Kode: "PSR-1104-01", Nama: "Pasar Kutacane", KabupatenKode: "1104", Lat: 3.493, Lng: 97.8, RadiusMeter: 400},
	{Kode: "PSR-1101-01", Nama: "Pasar Sinabang", KabupatenKode: "1101", Lat: 2.48, Lng: 96.38, RadiusMeter: 400},

	{Kode: "PSR-1102-01", Nama: "Pasar Tradisional Singkil", KabupatenKode: "1102", Lat: 2.28, Lng: 97.79, RadiusMeter: 400},
	{Kode: "PSR-1105-01", Nama: "Pasar Tradisional Idi", KabupatenKode: "1105", Lat: 4.96, Lng: 97.77, RadiusMeter: 400},
	{Kode: "PSR-1111-01", Nama: "Pasar Tradisional Lhoksukon", KabupatenKode: "1111", Lat: 5.05, Lng: 97.32, RadiusMeter: 400},
	{Kode: "PSR-1112-01", Nama: "Pasar Tradisional Blangpidie", KabupatenKode: "1112", Lat: 3.74, Lng: 96.84, RadiusMeter: 400},
	{Kode: "PSR-1113-01", Nama: "Pasar Tradisional Blangkejeren", KabupatenKode: "1113", Lat: 3.98, Lng: 97.35, RadiusMeter: 400},
	{Kode: "PSR-1114-01", Nama: "Pasar Tradisional Karang Baru", KabupatenKode: "1114", Lat: 4.29, Lng: 98.05, RadiusMeter: 400},
	{Kode: "PSR-1115-01", Nama: "Pasar Tradisional Suka Makmue", KabupatenKode: "1115", Lat: 4.13, Lng: 96.47, RadiusMeter: 400},
	{Kode: "PSR-1116-01", Nama: "Pasar Tradisional Calang", KabupatenKode: "1116", Lat: 4.63, Lng: 95.58, RadiusMeter: 400},
	{Kode: "PSR-1117-01", Nama: "Pasar Tradisional Redelong", KabupatenKode: "1117", Lat: 4.73, Lng: 96.87, RadiusMeter: 400},
	{Kode: "PSR-1118-01", Nama: "Pasar Tradisional Meureudu", KabupatenKode: "1118", Lat: 5.25, Lng: 96.25, RadiusMeter: 400},
	{Kode: "PSR-1172-01", Nama: "Pasar Tradisional Sabang", KabupatenKode: "1172", Lat: 5.89, Lng: 95.32, RadiusMeter: 400},
	{Kode: "PSR-1175-01", Nama: "Pasar Tradisional Subulussalam", KabupatenKode: "1175", Lat: 2.64, Lng: 98.0, RadiusMeter: 400},
}

// komoditasAceh: 14 komoditas pangan; tiga di antaranya pilot fase 1 (beras, cabai merah, bawang merah).
var komoditasAceh = []domain.Komoditas{
	{Kode: "BRS-MED", Nama: "Beras medium", Satuan: "kg", Kategori: "Pangan pokok", HargaAcuan: 13900, Pilot: true, ShelfLifeHari: 180},
	{Kode: "BRS-PRM", Nama: "Beras premium", Satuan: "kg", Kategori: "Pangan pokok", HargaAcuan: 15800, ShelfLifeHari: 180},
	{Kode: "CBM-KRT", Nama: "Cabai merah keriting", Satuan: "kg", Kategori: "Hortikultura", HargaAcuan: 52000, Pilot: true, ShelfLifeHari: 21},
	{Kode: "CBR-RWT", Nama: "Cabai rawit merah", Satuan: "kg", Kategori: "Hortikultura", HargaAcuan: 58000, ShelfLifeHari: 18},
	{Kode: "BWM", Nama: "Bawang merah", Satuan: "kg", Kategori: "Hortikultura", HargaAcuan: 41000, Pilot: true, ShelfLifeHari: 60},
	{Kode: "BWP", Nama: "Bawang putih", Satuan: "kg", Kategori: "Hortikultura", HargaAcuan: 38000, ShelfLifeHari: 120},
	{Kode: "TLR", Nama: "Telur ayam ras", Satuan: "kg", Kategori: "Peternakan", HargaAcuan: 29500, ShelfLifeHari: 30},
	{Kode: "DAG-AYM", Nama: "Daging ayam ras", Satuan: "kg", Kategori: "Peternakan", HargaAcuan: 36500, ShelfLifeHari: 90},
	{Kode: "DAG-SPI", Nama: "Daging sapi", Satuan: "kg", Kategori: "Peternakan", HargaAcuan: 148000, ShelfLifeHari: 180},
	{Kode: "IKN-TGK", Nama: "Ikan tongkol", Satuan: "kg", Kategori: "Perikanan", HargaAcuan: 38000, ShelfLifeHari: 120},
	{Kode: "MYK", Nama: "Minyak goreng curah", Satuan: "liter", Kategori: "Pangan olahan", HargaAcuan: 17500, ShelfLifeHari: 365},
	{Kode: "GLA", Nama: "Gula pasir", Satuan: "kg", Kategori: "Pangan olahan", HargaAcuan: 18000, ShelfLifeHari: 365},
	{Kode: "KTG", Nama: "Kentang", Satuan: "kg", Kategori: "Hortikultura", HargaAcuan: 16000, ShelfLifeHari: 75},
	{Kode: "TMT", Nama: "Tomat", Satuan: "kg", Kategori: "Hortikultura", HargaAcuan: 12000, ShelfLifeHari: 14},
}

// satuanKomoditas adalah katalog satuan; setiap Satuan pada komoditasAceh wajib ada di sini (FK komoditas.satuan).
var satuanKomoditas = []domain.Satuan{
	{Kode: "kg", Nama: "Kilogram"},
	{Kode: "liter", Nama: "Liter"},
}
