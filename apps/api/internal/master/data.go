package master

import "github.com/garda-inflasi/api/internal/master/domain"

// ProvinsiAceh adalah rujukan provinsi cakupan Garda Inflasi.
var ProvinsiAceh = domain.Provinsi{Kode: "11", Nama: "ACEH"}

var kabupatenAceh = []domain.Kabupaten{
	{Kode: "1101", Nama: "KABUPATEN SIMEULUE", Jenis: "Kabupaten", Ibukota: "Sinabang", Lat: 2.48, Lng: 96.38},
	{Kode: "1102", Nama: "KABUPATEN ACEH SINGKIL", Jenis: "Kabupaten", Ibukota: "Singkil", Lat: 2.28, Lng: 97.79},
	{Kode: "1103", Nama: "KABUPATEN ACEH SELATAN", Jenis: "Kabupaten", Ibukota: "Tapaktuan", Lat: 3.26, Lng: 97.18},
	{Kode: "1104", Nama: "KABUPATEN ACEH TENGGARA", Jenis: "Kabupaten", Ibukota: "Kutacane", Lat: 3.49, Lng: 97.80},
	{Kode: "1105", Nama: "KABUPATEN ACEH TIMUR", Jenis: "Kabupaten", Ibukota: "Idi Rayeuk", Lat: 4.96, Lng: 97.77},
	{Kode: "1106", Nama: "KABUPATEN ACEH TENGAH", Jenis: "Kabupaten", Ibukota: "Takengon", Lat: 4.62, Lng: 96.85},
	{Kode: "1107", Nama: "KABUPATEN ACEH BARAT", Jenis: "Kabupaten", Ibukota: "Meulaboh", Lat: 4.14, Lng: 96.13},
	{Kode: "1108", Nama: "KABUPATEN ACEH BESAR", Jenis: "Kabupaten", Ibukota: "Kota Jantho", Lat: 5.30, Lng: 95.63},
	{Kode: "1109", Nama: "KABUPATEN PIDIE", Jenis: "Kabupaten", Ibukota: "Sigli", Lat: 5.38, Lng: 95.96},
	{Kode: "1110", Nama: "KABUPATEN BIREUEN", Jenis: "Kabupaten", Ibukota: "Bireuen", Lat: 5.20, Lng: 96.70},
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
	{Kode: "1175", Nama: "KOTA SUBULUSSALAM", Jenis: "Kota", Ibukota: "Subulussalam", Lat: 2.64, Lng: 98.00},
}

var pasarAceh = []domain.Pasar{
	{Kode: "PSR-1171-01", Nama: "Pasar Peunayong", KabupatenKode: "1171", Lat: 5.5603, Lng: 95.3237, RadiusMeter: 400, Aktif: true},
	{Kode: "PSR-1171-02", Nama: "Pasar Al-Mahirah Lamdingin", KabupatenKode: "1171", Lat: 5.5765, Lng: 95.3352, RadiusMeter: 400, Aktif: true},
	{Kode: "PSR-1108-01", Nama: "Pasar Induk Lambaro", KabupatenKode: "1108", Lat: 5.5085, Lng: 95.3640, RadiusMeter: 600, Aktif: true},
	{Kode: "PSR-1174-01", Nama: "Pasar Inpres Lhokseumawe", KabupatenKode: "1174", Lat: 5.1790, Lng: 97.1460, RadiusMeter: 450, Aktif: true},
	{Kode: "PSR-1110-01", Nama: "Pasar Inpres Bireuen", KabupatenKode: "1110", Lat: 5.2030, Lng: 96.7030, RadiusMeter: 450, Aktif: true},
	{Kode: "PSR-1109-01", Nama: "Pasar Kota Sigli", KabupatenKode: "1109", Lat: 5.3860, Lng: 95.9600, RadiusMeter: 450, Aktif: true},
	{Kode: "PSR-1107-01", Nama: "Pasar Bina Usaha Meulaboh", KabupatenKode: "1107", Lat: 4.1450, Lng: 96.1290, RadiusMeter: 450, Aktif: true},
	{Kode: "PSR-1106-01", Nama: "Pasar Inpres Takengon", KabupatenKode: "1106", Lat: 4.6270, Lng: 96.8460, RadiusMeter: 450, Aktif: true},
	{Kode: "PSR-1173-01", Nama: "Pasar Kota Langsa", KabupatenKode: "1173", Lat: 4.4700, Lng: 97.9680, RadiusMeter: 450, Aktif: true},
	{Kode: "PSR-1103-01", Nama: "Pasar Tapaktuan", KabupatenKode: "1103", Lat: 3.2600, Lng: 97.1830, RadiusMeter: 400, Aktif: true},
	{Kode: "PSR-1104-01", Nama: "Pasar Kutacane", KabupatenKode: "1104", Lat: 3.4930, Lng: 97.8000, RadiusMeter: 400, Aktif: true},
	{Kode: "PSR-1101-01", Nama: "Pasar Sinabang", KabupatenKode: "1101", Lat: 2.4800, Lng: 96.3800, RadiusMeter: 400, Aktif: true},
	{Kode: "PSR-1102-01", Nama: "Pasar Tradisional Singkil", KabupatenKode: "1102", Lat: 2.2800, Lng: 97.7900, RadiusMeter: 400, Aktif: true},
	{Kode: "PSR-1105-01", Nama: "Pasar Tradisional Idi", KabupatenKode: "1105", Lat: 4.9600, Lng: 97.7700, RadiusMeter: 400, Aktif: true},
	{Kode: "PSR-1111-01", Nama: "Pasar Tradisional Lhoksukon", KabupatenKode: "1111", Lat: 5.0500, Lng: 97.3200, RadiusMeter: 400, Aktif: true},
	{Kode: "PSR-1112-01", Nama: "Pasar Tradisional Blangpidie", KabupatenKode: "1112", Lat: 3.7400, Lng: 96.8400, RadiusMeter: 400, Aktif: true},
	{Kode: "PSR-1113-01", Nama: "Pasar Tradisional Blangkejeren", KabupatenKode: "1113", Lat: 3.9800, Lng: 97.3500, RadiusMeter: 400, Aktif: true},
	{Kode: "PSR-1114-01", Nama: "Pasar Tradisional Karang Baru", KabupatenKode: "1114", Lat: 4.2900, Lng: 98.0500, RadiusMeter: 400, Aktif: true},
	{Kode: "PSR-1115-01", Nama: "Pasar Tradisional Suka Makmue", KabupatenKode: "1115", Lat: 4.1300, Lng: 96.4700, RadiusMeter: 400, Aktif: true},
	{Kode: "PSR-1116-01", Nama: "Pasar Tradisional Calang", KabupatenKode: "1116", Lat: 4.6300, Lng: 95.5800, RadiusMeter: 400, Aktif: true},
	{Kode: "PSR-1117-01", Nama: "Pasar Tradisional Redelong", KabupatenKode: "1117", Lat: 4.7300, Lng: 96.8700, RadiusMeter: 400, Aktif: true},
	{Kode: "PSR-1118-01", Nama: "Pasar Tradisional Meureudu", KabupatenKode: "1118", Lat: 5.2500, Lng: 96.2500, RadiusMeter: 400, Aktif: true},
	{Kode: "PSR-1172-01", Nama: "Pasar Tradisional Sabang", KabupatenKode: "1172", Lat: 5.8900, Lng: 95.3200, RadiusMeter: 400, Aktif: true},
	{Kode: "PSR-1175-01", Nama: "Pasar Tradisional Subulussalam", KabupatenKode: "1175", Lat: 2.6400, Lng: 98.0000, RadiusMeter: 400, Aktif: true},
}

var komoditasAceh = []domain.Komoditas{
	{Kode: "BRS-MED", Nama: "Beras medium", Satuan: "kg", Kategori: "Pangan pokok", HargaAcuan: 13900, Pilot: true, ShelfLifeHari: 180},
	{Kode: "BRS-PRM", Nama: "Beras premium", Satuan: "kg", Kategori: "Pangan pokok", HargaAcuan: 15800, Pilot: false, ShelfLifeHari: 180},
	{Kode: "CBM-KRT", Nama: "Cabai merah keriting", Satuan: "kg", Kategori: "Hortikultura", HargaAcuan: 52000, Pilot: true, ShelfLifeHari: 21},
	{Kode: "CBR-RWT", Nama: "Cabai rawit merah", Satuan: "kg", Kategori: "Hortikultura", HargaAcuan: 58000, Pilot: false, ShelfLifeHari: 18},
	{Kode: "BWM", Nama: "Bawang merah", Satuan: "kg", Kategori: "Hortikultura", HargaAcuan: 41000, Pilot: true, ShelfLifeHari: 60},
	{Kode: "BWP", Nama: "Bawang putih", Satuan: "kg", Kategori: "Hortikultura", HargaAcuan: 38000, Pilot: false, ShelfLifeHari: 120},
	{Kode: "TLR", Nama: "Telur ayam ras", Satuan: "kg", Kategori: "Peternakan", HargaAcuan: 29500, Pilot: false, ShelfLifeHari: 30},
	{Kode: "DAG-AYM", Nama: "Daging ayam ras", Satuan: "kg", Kategori: "Peternakan", HargaAcuan: 36500, Pilot: false, ShelfLifeHari: 90},
	{Kode: "DAG-SPI", Nama: "Daging sapi", Satuan: "kg", Kategori: "Peternakan", HargaAcuan: 148000, Pilot: false, ShelfLifeHari: 180},
	{Kode: "IKN-TGK", Nama: "Ikan tongkol", Satuan: "kg", Kategori: "Perikanan", HargaAcuan: 38000, Pilot: false, ShelfLifeHari: 120},
	{Kode: "MYK", Nama: "Minyak goreng curah", Satuan: "liter", Kategori: "Pangan olahan", HargaAcuan: 17500, Pilot: false, ShelfLifeHari: 365},
	{Kode: "GLA", Nama: "Gula pasir", Satuan: "kg", Kategori: "Pangan olahan", HargaAcuan: 18000, Pilot: false, ShelfLifeHari: 365},
	{Kode: "KTG", Nama: "Kentang", Satuan: "kg", Kategori: "Hortikultura", HargaAcuan: 16000, Pilot: false, ShelfLifeHari: 75},
	{Kode: "TMT", Nama: "Tomat", Satuan: "kg", Kategori: "Hortikultura", HargaAcuan: 12000, Pilot: false, ShelfLifeHari: 14},
}

// Kabupaten mengembalikan salinan data kabupaten/kota dengan kode provinsi terisi.
func Kabupaten() []domain.Kabupaten {
	rows := append([]domain.Kabupaten(nil), kabupatenAceh...)
	for i := range rows {
		rows[i].ProvinsiKode = ProvinsiAceh.Kode
	}
	return rows
}

// Pasar mengembalikan salinan data pasar.
func Pasar() []domain.Pasar {
	rows := append([]domain.Pasar(nil), pasarAceh...)
	for i := range rows {
		rows[i].Aktif = true
	}
	return rows
}

var satuanKomoditas = []domain.Satuan{
	{Kode: "kg", Nama: "Kilogram"},
	{Kode: "liter", Nama: "Liter"},
}

// Satuan mengembalikan salinan data satuan.
func Satuan() []domain.Satuan {
	return append([]domain.Satuan(nil), satuanKomoditas...)
}

// Kecamatan mengembalikan data kecamatan se-Aceh dari berkas CSV tersemat.
func Kecamatan() ([]domain.Kecamatan, error) {
	data, _, err := KecamatanAceh()
	return data, err
}

// Komoditas mengembalikan salinan data komoditas.
func Komoditas() []domain.Komoditas {
	return append([]domain.Komoditas(nil), komoditasAceh...)
}
