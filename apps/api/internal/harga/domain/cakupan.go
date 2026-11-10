package domain

// Peran operasional yang jangkauan barisnya dibatasi (02-B). Kuncinya sama dengan enum peran dan tabel roles.
const (
	PeranKoordinator = "OPS_KOORDINATOR"
	PeranAgregator   = "OPS_AGREGATOR"
	PeranMahasiswa   = "OPS_MAHASISWA"
)

// perananSeluruhAceh adalah peran tingkat provinsi ke atas: strategis, manajerial, dan sistem. Mereka tidak punya
// kabupaten tugas (pengguna.kabupaten_kode NULL) dan melihat entri seluruh Aceh. Daftar ini sengaja eksplisit:
// peran yang belum dikenal TIDAK otomatis melihat semuanya (lihat CakupanPeran), dan tes integrasi memastikan
// setiap peran di tabel roles masuk salah satu golongan.
var perananSeluruhAceh = map[string]bool{
	"STRATEGIS_FMIPA": true, "STRATEGIS_BI": true, "STRATEGIS_BPS": true, "STRATEGIS_DJPB": true,
	"MANAJER_PROGRAM": true, "MANAJER_DATA": true, "MANAJER_KEBIJAKAN": true,
	"MANAJER_PEMBIAYAAN": true, "MANAJER_RISIKO": true, "MANAJER_MUTU": true,
	"ADMIN_SISTEM": true,
}

// PeranDikenal melaporkan apakah peran punya aturan jangkauan eksplisit.
func PeranDikenal(peran string) bool {
	switch peran {
	case PeranKoordinator, PeranAgregator, PeranMahasiswa:
		return true
	}
	return perananSeluruhAceh[peran]
}

// Cakupan menentukan entri harga yang boleh dibaca atau diubah pemanggil (row scope, 02-B). Entri di luar cakupan
// diperlakukan seperti tidak ada: detail dijawab 404, daftar dan ringkasan tidak memuatnya, dan aksi massal
// melaporkannya tidak_ditemukan.
//
// Entri masuk cakupan bila salah satu syarat terpenuhi: pasarnya di KabupatenKode, atau ia dicatat oleh PemilikID.
// Nilai nol berarti tertutup (tidak ada entri yang lolos), jadi cakupan yang lupa diisi tidak membuka data; membuka
// semua entri harus eksplisit lewat Semua.
type Cakupan struct {
	// Semua membuka entri seluruh Aceh.
	Semua bool
	// KabupatenKode (kode BPS) membuka entri di pasar kabupaten ini. Kosong: tidak ada batas kabupaten yang dibuka.
	KabupatenKode string
	// PemilikID (UUID pengguna) membuka entri yang dicatat pengguna ini. Kosong: tidak ada entri milik yang dibuka.
	PemilikID string
}

// CakupanSemua membuka entri seluruh Aceh.
func CakupanSemua() Cakupan { return Cakupan{Semua: true} }

// CakupanMilik hanya membuka entri yang dicatat penggunaID.
func CakupanMilik(penggunaID string) Cakupan { return Cakupan{PemilikID: penggunaID} }

// CakupanPeran menurunkan cakupan dari peran pengguna (02-B):
//   - Koordinator dan Agregator: entri di kabupaten tugasnya, ditambah entri yang ia catat sendiri (supaya entri
//     yang ia kirim dari pasar di luar kabupatennya tidak lenyap dari daftarnya). Tanpa kabupaten tugas
//     (kabupatenKode kosong) hanya entri miliknya: gagal-tutup, bukan seluruh Aceh.
//   - Mahasiswa: hanya entri yang ia catat sendiri.
//   - Peran tingkat provinsi ke atas: seluruh Aceh.
//   - Peran yang belum dikenal: hanya entri miliknya (hak paling sedikit), bukan seluruh Aceh.
func CakupanPeran(peran, kabupatenKode, penggunaID string) Cakupan {
	switch peran {
	case PeranKoordinator, PeranAgregator:
		return Cakupan{KabupatenKode: kabupatenKode, PemilikID: penggunaID}
	case PeranMahasiswa:
		return CakupanMilik(penggunaID)
	}
	if perananSeluruhAceh[peran] {
		return CakupanSemua()
	}
	return CakupanMilik(penggunaID)
}

// Mengizinkan melaporkan apakah entri di kabupatenKode yang dicatat petugasID masuk cakupan.
func (c Cakupan) Mengizinkan(kabupatenKode string, petugasID *string) bool {
	if c.Semua {
		return true
	}
	if c.KabupatenKode != "" && kabupatenKode == c.KabupatenKode {
		return true
	}
	return c.PemilikID != "" && petugasID != nil && *petugasID == c.PemilikID
}
