package integration

// Pembantu bersama tes integrasi ringkasan operasional 08-B: aktivitas per petugas, stok, produsen, dan logistik.
// Data terkendali disisipkan langsung ke tabel dengan angka yang bisa dihitung dengan tangan; semua waktu relatif
// terhadap hari ini menurut waktu Aceh (WIB), dengan jam yang sama dengan yang dipakai API.

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var wibTes = time.FixedZone("WIB", 7*3600)

// pukul membentuk waktu WIB pada tanggal hari (DATE) dengan jam dan menit tertentu. hari dapat berupa hasil hariIni()
// digeser dengan AddDate.
func pukul(hari time.Time, jam, menit int) time.Time {
	return time.Date(hari.Year(), hari.Month(), hari.Day(), jam, menit, 0, 0, wibTes)
}

// jagaTengahMalam menunda tes yang akan berjalan melewati pergantian hari WIB: data terkendali dan permintaan API
// harus melihat "hari ini" yang sama.
func jagaTengahMalam(t *testing.T) {
	t.Helper()
	now := time.Now().In(wibTes)
	besok := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, wibTes)
	if sisa := time.Until(besok); sisa < 30*time.Second {
		t.Logf("menunggu pergantian hari WIB (%s)", sisa.Round(time.Second))
		time.Sleep(sisa + 2*time.Second)
	}
}

// ── Bentuk respons ───────────────────────────────────────────────────

type petugasItem struct {
	ID                string     `json:"id"`
	Nama              string     `json:"nama"`
	KabupatenKode     string     `json:"kabupaten_kode"`
	KabupatenNama     string     `json:"kabupaten_nama"`
	EntriHariIni      int        `json:"entri_hari_ini"`
	TargetHarian      int        `json:"target_harian"`
	MenungguValidasi  int        `json:"menunggu_validasi"`
	DitolakJendela    int        `json:"ditolak_jendela"`
	EntriTerakhirPada *time.Time `json:"entri_terakhir_pada"`
	TugasAktif        int        `json:"tugas_aktif"`
	TugasSelesai      int        `json:"tugas_selesai"`
	TugasTerlambat    int        `json:"tugas_terlambat"`
}

type petugasResp struct {
	Tanggal     string        `json:"tanggal"`
	JendelaHari int           `json:"jendela_hari"`
	Petugas     []petugasItem `json:"petugas"`
}

type stokRingkasan struct {
	TotalTon                float64 `json:"total_ton"`
	JumlahLot               int     `json:"jumlah_lot"`
	JumlahFasilitas         int     `json:"jumlah_fasilitas"`
	KapasitasTon            int     `json:"kapasitas_ton"`
	FasilitasOperasional    int     `json:"fasilitas_operasional"`
	UtilisasiPct            float64 `json:"utilisasi_pct"`
	MendekatiKedaluwarsaTon float64 `json:"mendekati_kedaluwarsa_ton"`
	RisikoTinggiTon         float64 `json:"risiko_tinggi_ton"`
	KedaluwarsaLot          int     `json:"kedaluwarsa_lot"`
	KedaluwarsaTon          float64 `json:"kedaluwarsa_ton"`
	DiLuarOperasionalTon    float64 `json:"di_luar_operasional_ton"`
}

type stokKomoditas struct {
	KomoditasKode           string  `json:"komoditas_kode"`
	KomoditasNama           string  `json:"komoditas_nama"`
	TotalTon                float64 `json:"total_ton"`
	JumlahLot               int     `json:"jumlah_lot"`
	JumlahFasilitas         int     `json:"jumlah_fasilitas"`
	MendekatiKedaluwarsaTon float64 `json:"mendekati_kedaluwarsa_ton"`
	RisikoTinggiTon         float64 `json:"risiko_tinggi_ton"`
}

type stokResp struct {
	Tanggal               string          `json:"tanggal"`
	AmbangKedaluwarsaHari int             `json:"ambang_kedaluwarsa_hari"`
	AmbangRisiko          float64         `json:"ambang_risiko"`
	Ringkasan             stokRingkasan   `json:"ringkasan"`
	Komoditas             []stokKomoditas `json:"komoditas"`
}

func (r stokResp) kodeKomoditas() []string {
	kode := make([]string, len(r.Komoditas))
	for i, k := range r.Komoditas {
		kode[i] = k.KomoditasKode
	}
	return kode
}

func (r stokResp) cari(kode string) (stokKomoditas, bool) {
	for _, k := range r.Komoditas {
		if k.KomoditasKode == kode {
			return k, true
		}
	}
	return stokKomoditas{}, false
}

type sisiHarga struct {
	Harga           float64 `json:"harga"`
	JumlahEntri     int     `json:"jumlah_entri"`
	JumlahKabupaten int     `json:"jumlah_kabupaten"`
}

type margin struct {
	Selisih        float64 `json:"selisih"`
	PersenProdusen float64 `json:"persen_produsen"`
	PersenKonsumen float64 `json:"persen_konsumen"`
}

type produksi struct {
	Ton     float64 `json:"ton"`
	Periode string  `json:"periode"`
	Sumber  string  `json:"sumber"`
	Demo    bool    `json:"demo"`
}

type produsenKomoditas struct {
	KomoditasKode string     `json:"komoditas_kode"`
	KomoditasNama string     `json:"komoditas_nama"`
	Satuan        string     `json:"satuan"`
	Produsen      *sisiHarga `json:"produsen"`
	Konsumen      *sisiHarga `json:"konsumen"`
	Margin        *margin    `json:"margin"`
	Produksi      *produksi  `json:"produksi"`
}

type produsenResp struct {
	Dari      string              `json:"dari"`
	Sampai    string              `json:"sampai"`
	Komoditas []produsenKomoditas `json:"komoditas"`
}

func (r produsenResp) kodeKomoditas() []string {
	kode := make([]string, len(r.Komoditas))
	for i, k := range r.Komoditas {
		kode[i] = k.KomoditasKode
	}
	return kode
}

func (r produsenResp) cari(t *testing.T, kode string) produsenKomoditas {
	t.Helper()
	for _, k := range r.Komoditas {
		if k.KomoditasKode == kode {
			return k
		}
	}
	require.Failf(t, "komoditas tidak ada", "%s tidak ada di %v", kode, r.kodeKomoditas())
	return produsenKomoditas{}
}

type gerakJenis struct {
	Jumlah    int     `json:"jumlah"`
	VolumeTon float64 `json:"volume_ton"`
}

type gerakRingkasan struct {
	Masuk  gerakJenis `json:"masuk"`
	Keluar gerakJenis `json:"keluar"`
	Pindah gerakJenis `json:"pindah"`
}

type gerakKomoditas struct {
	KomoditasKode string     `json:"komoditas_kode"`
	KomoditasNama string     `json:"komoditas_nama"`
	Masuk         gerakJenis `json:"masuk"`
	Keluar        gerakJenis `json:"keluar"`
	Pindah        gerakJenis `json:"pindah"`
}

type fasilitasLogistik struct {
	ID            string  `json:"id"`
	Kode          *string `json:"kode"`
	Nama          string  `json:"nama"`
	KabupatenKode string  `json:"kabupaten_kode"`
	KabupatenNama string  `json:"kabupaten_nama"`
}

type pergerakan struct {
	ID            string             `json:"id"`
	Waktu         time.Time          `json:"waktu"`
	Jenis         string             `json:"jenis"`
	KomoditasKode string             `json:"komoditas_kode"`
	KomoditasNama string             `json:"komoditas_nama"`
	VolumeTon     float64            `json:"volume_ton"`
	Asal          *fasilitasLogistik `json:"asal"`
	Tujuan        *fasilitasLogistik `json:"tujuan"`
}

type ruteItem struct {
	ID            string    `json:"id"`
	Kode          *string   `json:"kode"`
	AsalKode      string    `json:"asal_kode"`
	Asal          string    `json:"asal"`
	TujuanKode    string    `json:"tujuan_kode"`
	Tujuan        string    `json:"tujuan"`
	KomoditasKode string    `json:"komoditas_kode"`
	KomoditasNama string    `json:"komoditas_nama"`
	VolumeTon     float64   `json:"volume_ton"`
	JarakKm       int       `json:"jarak_km"`
	JamTempuh     float64   `json:"jam_tempuh"`
	Biaya         int64     `json:"biaya"`
	Status        string    `json:"status"`
	Sumber        string    `json:"sumber"`
	DibuatPada    time.Time `json:"dibuat_pada"`
}

type ruteRingkas struct {
	Jumlah    int        `json:"jumlah"`
	VolumeTon float64    `json:"volume_ton"`
	Biaya     int64      `json:"biaya"`
	Items     []ruteItem `json:"items"`
}

type logistikResp struct {
	Dari      string           `json:"dari"`
	Sampai    string           `json:"sampai"`
	Ringkasan gerakRingkasan   `json:"ringkasan"`
	Komoditas []gerakKomoditas `json:"komoditas"`
	Terbaru   []pergerakan     `json:"terbaru"`
	Rute      ruteRingkas      `json:"rute"`
}

func (r logistikResp) idTerbaru() []string {
	id := make([]string, len(r.Terbaru))
	for i, p := range r.Terbaru {
		id[i] = p.ID
	}
	return id
}

// ── Pemanggilan ──────────────────────────────────────────────────────

const awalan = "/v1/dashboard/operasional"

func (s *suite) petugasDari(t *testing.T, email, query string) petugasResp {
	t.Helper()
	return ambilData[petugasResp](t, s, awalan+"/aktivitas-petugas"+query, email)
}

func (s *suite) stokDari(t *testing.T, email, query string) stokResp {
	t.Helper()
	return ambilData[stokResp](t, s, awalan+"/stok"+query, email)
}

func (s *suite) produsenDari(t *testing.T, email, query string) produsenResp {
	t.Helper()
	return ambilData[produsenResp](t, s, awalan+"/produsen"+query, email)
}

func (s *suite) logistikDari(t *testing.T, email, query string) logistikResp {
	t.Helper()
	return ambilData[logistikResp](t, s, awalan+"/logistik"+query, email)
}

// ambilData memanggil GET path sebagai email, memastikan 200, dan mengurai isi data.
func ambilData[T any](t *testing.T, s *suite, path, email string) T {
	t.Helper()
	w := s.call(t, http.MethodGet, path, email, nil)
	require.Equal(t, http.StatusOK, w.Code, "%s: %s", path, w.Body.String())
	return data[T](t, w)
}

// mentah mengembalikan isi data apa adanya (untuk memeriksa bentuk JSON: larik [] bukan null, bidang yang tidak boleh ada).
func (s *suite) mentah(t *testing.T, path, email string) json.RawMessage {
	t.Helper()
	w := s.call(t, http.MethodGet, path, email, nil)
	require.Equal(t, http.StatusOK, w.Code, "%s: %s", path, w.Body.String())
	return data[json.RawMessage](t, w)
}

// ── Data terkendali ──────────────────────────────────────────────────

// uuidAkun mengembalikan UUID stabil akun demo menurut slug (mahasiswa_1, koordinator_lapangan, ...).
func (s *suite) uuidAkun(t *testing.T, slug string) string {
	t.Helper()
	var id string
	require.NoError(t, s.db.Raw(`SELECT garda_uuid_stabil('pengguna', ?)::text`, slug).Scan(&id).Error)
	return id
}

// sisipEntriPasti menyisipkan satu entri harga milik akun slug. Status DITOLAK otomatis membawa alasan (wajib).
func (s *suite) sisipEntriPasti(t *testing.T, slug, pasar, komoditas, tipe, status string, harga int, waktu time.Time) {
	t.Helper()
	alasan := any(nil)
	if status == "DITOLAK" {
		alasan = "harga tidak wajar"
	}
	require.NoError(t, s.db.Exec(`
		INSERT INTO entri_harga (komoditas_kode, pasar_id, tipe, harga, satuan, status, dicatat_pada, petugas_id, alasan_penolakan)
		VALUES (?, ?, ?::tipe_harga, ?, 'kg', ?::status_entri, ?, garda_uuid_stabil('pengguna', ?), ?)`,
		komoditas, pasar, tipe, harga, status, waktu, slug, alasan).Error)
}

// sisipHargaTetap menyisipkan entri harga tanpa petugas pada pukul 09.00 WIB hari tersebut untuk tiap harga, sebagai
// data pasar yang tervalidasi.
func (s *suite) sisipHargaTetap(t *testing.T, hari time.Time, pasar, komoditas, tipe, status string, harga ...int) {
	t.Helper()
	for i, h := range harga {
		require.NoError(t, s.db.Exec(`
			INSERT INTO entri_harga (komoditas_kode, pasar_id, tipe, harga, satuan, status, dicatat_pada)
			VALUES (?, ?, ?::tipe_harga, ?, 'kg', ?::status_entri, ?)`,
			komoditas, pasar, tipe, h, status, pukul(hari, 9, i)).Error)
	}
}

// sisipTugasPasti menyisipkan satu tugas lapangan milik akun slug di pasar Banda Aceh.
func (s *suite) sisipTugasPasti(t *testing.T, slug, kode, status string, tenggat time.Time, jumlahSelesai int, selesaiPada *time.Time) {
	t.Helper()
	require.NoError(t, s.db.Exec(`
		INSERT INTO tugas_lapangan (kode, judul, pasar_id, petugas_id, komoditas, tipe, tenggat, status, target, jumlah_selesai, selesai_pada)
		VALUES (?, ?, 'PSR-1171-01', garda_uuid_stabil('pengguna', ?), ARRAY['TMT']::text[], 'KONSUMEN', ?, ?::status_tugas, 4, ?, ?)`,
		kode, "Tugas "+kode, slug, tenggat, status, jumlahSelesai, selesaiPada).Error)
}

// sisipPengguna menyisipkan akun lapangan tambahan (tanpa kata sandi: hanya untuk daftar petugas, bukan login).
func (s *suite) sisipPengguna(t *testing.T, nama, email, peran, status string, kabupaten any) {
	t.Helper()
	require.NoError(t, s.db.Exec(`
		INSERT INTO pengguna (nama, email, peran, lapis, lembaga, kabupaten_kode, status)
		VALUES (?, ?, ?::peran, 'OPERASIONAL', 'FMIPA USK', ?, ?::status_akun)`,
		nama, email, peran, kabupaten, status).Error)
}

// sisipLotPasti menyisipkan satu lot di fasilitas (kode) dan mengembalikan ID-nya.
func (s *suite) sisipLotPasti(t *testing.T, fasilitas, komoditas string, ton float64, masuk, kedaluwarsa time.Time, risiko float64, status string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, s.db.Raw(`
		INSERT INTO stok_cold_storage (cold_storage_id, komoditas_kode, volume_ton, masuk_pada, pemilik, risiko_food_loss, kedaluwarsa_pada, status)
		SELECT id, ?, ?, ?, 'KUD Uji', ?, ?, ? FROM cold_storage WHERE kode = ?
		RETURNING id`, komoditas, ton, masuk, risiko, kedaluwarsa, status, fasilitas).Scan(&id).Error)
	require.NotZero(t, id, "fasilitas %s tidak ada", fasilitas)
	return id
}

// sisipTransaksiPasti menyisipkan satu baris buku besar. asal atau tujuan dikosongkan sesuai jenis (CHECK lokasi).
func (s *suite) sisipTransaksiPasti(t *testing.T, lot int64, jenis string, ton float64, asal, tujuan string, waktu time.Time, olehSlug, alasan string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, s.db.Raw(`
		INSERT INTO transaksi_stok (lot_id, jenis, volume_ton, fasilitas_asal_id, fasilitas_tujuan_id, oleh_id, waktu, alasan, sumber)
		VALUES (?, ?, ?, (SELECT id FROM cold_storage WHERE kode = NULLIF(?, '')), (SELECT id FROM cold_storage WHERE kode = NULLIF(?, '')),
		        CASE WHEN ?::text = '' THEN NULL ELSE garda_uuid_stabil('pengguna', ?::text) END, ?, ?, 'APLIKASI')
		RETURNING id`, lot, jenis, ton, asal, tujuan, olehSlug, olehSlug, waktu, alasan).Scan(&id).Error)
	return id
}

// sisipRutePasti menyisipkan satu rute distribusi.
func (s *suite) sisipRutePasti(t *testing.T, kode, asal, tujuan, komoditas string, ton float64, biaya int64, status string) {
	t.Helper()
	require.NoError(t, s.db.Exec(`
		INSERT INTO rute_distribusi (kode, asal_kode, tujuan_kode, komoditas_kode, volume_ton, jarak_km, jam_tempuh, biaya, status, sumber)
		VALUES (?, ?, ?, ?, ?, 100, 2.5, ?, ?, 'DEMO')`, kode, asal, tujuan, komoditas, ton, biaya, status).Error)
}

// sisipNeracaPasti menyisipkan satu baris neraca_pangan (konsumsi dibuat 1000 ton: kolom itu wajib positif).
func (s *suite) sisipNeracaPasti(t *testing.T, komoditas, wilayah string, periode time.Time, produksiTon float64, sumber string) {
	t.Helper()
	require.NoError(t, s.db.Exec(`
		INSERT INTO neraca_pangan (komoditas_kode, wilayah_kode, periode, produksi_ton, konsumsi_ton, sumber)
		VALUES (?, ?, ?::date, ?, 1000, ?)`, komoditas, wilayah, tanggal(periode), produksiTon, sumber).Error)
}

// awalBulan mengembalikan tanggal 1 bulan yang berselisih n bulan dari bulan hari ini (n negatif: ke belakang).
func awalBulan(n int) time.Time {
	h := hariIni()
	return time.Date(h.Year(), h.Month()+time.Month(n), 1, 0, 0, 0, 0, time.UTC)
}

// terpisah memastikan tes memakai basis data sendiri (data terkendali tidak boleh mengotori suite bersama).
func terpisah(t *testing.T, nama string, denganDemo bool) *suite {
	t.Helper()
	perluDocker(t)
	jagaTengahMalam(t)
	return newSuite(t, nama, denganDemo)
}

// kunciPeta mengembalikan nama kunci sebuah objek JSON.
func kunciPeta(m map[string]any) []string {
	kunci := make([]string, 0, len(m))
	for k := range m {
		kunci = append(kunci, k)
	}
	return kunci
}
