package dto

import (
	"time"

	"github.com/garda-inflasi/api/internal/cold-storage/domain"
)

type FasilitasColdStorageResponse struct {
	ID            string `json:"id"`
	Nama          string `json:"nama"`
	KabupatenKode string `json:"kabupaten_kode"`
	// KabupatenNama adalah nama resmi pada master wilayah, mis. "KABUPATEN ACEH BESAR".
	KabupatenNama string  `json:"kabupaten_nama"`
	Lat           float64 `json:"lat"`
	Lng           float64 `json:"lng"`
	KapasitasTon  int     `json:"kapasitas_ton"`
	// TerisiTon adalah jumlah volume seluruh lot di fasilitas ini.
	TerisiTon       float64 `json:"terisi_ton"`
	UtilisasiPersen float64 `json:"utilisasi_persen"`
	SuhuTargetC     float64 `json:"suhu_target_c"`
	// SuhuAktualC bernilai null bila fasilitas belum memiliki telemetri.
	SuhuAktualC *float64 `json:"suhu_aktual_c"`
	Status      string   `json:"status"`
	// BatasMinC dan BatasMaksC adalah batas alarm suhu efektif (suhu_min_c/suhu_maks_c
	// fasilitas, atau target ± toleransi bawaan).
	BatasMinC  float64 `json:"batas_min_c"`
	BatasMaksC float64 `json:"batas_maks_c"`
	// AlarmSuhu benar bila suhu aktual berada di luar batas alarm.
	AlarmSuhu bool `json:"alarm_suhu"`
	// KelembapanAktualPct dan TelemetriPada berasal dari pembacaan terakhir; null bila belum ada.
	KelembapanAktualPct *float64   `json:"kelembapan_aktual_pct"`
	TelemetriPada       *time.Time `json:"telemetri_pada"`
}

type StokColdStorageResponse struct {
	ID              string    `json:"id"`
	ColdStorageID   string    `json:"cold_storage_id"`
	ColdStorageNama string    `json:"cold_storage_nama"`
	KomoditasKode   string    `json:"komoditas_kode"`
	KomoditasNama   string    `json:"komoditas_nama"`
	VolumeTon       float64   `json:"volume_ton"`
	MasukPada       time.Time `json:"masuk_pada"`
	KedaluwarsaPada time.Time `json:"kedaluwarsa_pada"`
	// SisaMasaSimpanHari dihitung saat permintaan; 0 berarti sudah melewati batas.
	SisaMasaSimpanHari int `json:"sisa_masa_simpan_hari"`
	// RisikoFoodLoss berupa pecahan 0–1.
	RisikoFoodLoss float64 `json:"risiko_food_loss"`
	Pemilik        string  `json:"pemilik"`
	// Status lot: AKTIF atau PRIORITAS_JUAL (lot HABIS tidak ditampilkan).
	Status string `json:"status"`
}

// PageMetaResponse menjelaskan halaman stok yang dikembalikan.
type PageMetaResponse struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
	// VolumeHasilTon menjumlahkan seluruh hasil filter (semua halaman).
	VolumeHasilTon float64 `json:"volume_hasil_ton"`
}

// RingkasanColdStorageResponse mencakup seluruh stok, tidak terpengaruh filter.
type RingkasanColdStorageResponse struct {
	TotalLot           int64   `json:"total_lot"`
	TotalVolumeTon     float64 `json:"total_volume_ton"`
	RisikoTinggiLot    int64   `json:"risiko_tinggi_lot"`
	RisikoTinggiTon    float64 `json:"risiko_tinggi_ton"`
	AmbangRisikoTinggi float64 `json:"ambang_risiko_tinggi"`
}

type ColdStorageResponse struct {
	Fasilitas []FasilitasColdStorageResponse `json:"fasilitas"`
	Stok      []StokColdStorageResponse      `json:"stok"`
	Meta      PageMetaResponse               `json:"meta"`
	Ringkasan RingkasanColdStorageResponse   `json:"ringkasan"`
}

type BulkColdStorageResponse struct {
	Berhasil  int     `json:"berhasil"`
	Gagal     int     `json:"gagal"`
	VolumeTon float64 `json:"volume_ton"`
	// Rincian adalah hasil per lot menurut urutan permintaan.
	Rincian []BarisBulkResponse `json:"rincian"`
}

type BarisBulkResponse struct {
	ID       string `json:"id"`
	Berhasil bool   `json:"berhasil"`
	// Pesan menjelaskan kegagalan; kosong bila berhasil.
	Pesan string `json:"pesan"`
}

type TitikSuhuResponse struct {
	Waktu         time.Time `json:"waktu"`
	SuhuC         float64   `json:"suhu_c"`
	KelembapanPct *float64  `json:"kelembapan_pct"`
	Sumber        string    `json:"sumber"`
	DiLuarRentang bool      `json:"di_luar_rentang"`
}

type TransaksiStokResponse struct {
	ID                string    `json:"id"`
	LotID             string    `json:"lot_id"`
	LotBaruID         *string   `json:"lot_baru_id"`
	Jenis             string    `json:"jenis"`
	VolumeTon         float64   `json:"volume_ton"`
	KomoditasKode     string    `json:"komoditas_kode"`
	KomoditasNama     string    `json:"komoditas_nama"`
	FasilitasAsalID   *string   `json:"fasilitas_asal_id"`
	FasilitasTujuanID *string   `json:"fasilitas_tujuan_id"`
	OlehNama          *string   `json:"oleh_nama"`
	Waktu             time.Time `json:"waktu"`
	Alasan            string    `json:"alasan"`
}

// DetailFasilitasResponse adalah data halaman detail satu fasilitas.
type DetailFasilitasResponse struct {
	Fasilitas FasilitasColdStorageResponse `json:"fasilitas"`
	// Telemetri berurut dari yang terlama ke terbaru.
	Telemetri []TitikSuhuResponse       `json:"telemetri"`
	Stok      []StokColdStorageResponse `json:"stok"`
	Transaksi []TransaksiStokResponse   `json:"transaksi"`
}

type AlarmSuhuResponse struct {
	FasilitasID string    `json:"fasilitas_id"`
	Waktu       time.Time `json:"waktu"`
	SuhuC       float64   `json:"suhu_c"`
	BatasMinC   float64   `json:"batas_min_c"`
	BatasMaksC  float64   `json:"batas_maks_c"`
}

type IngestTelemetriResponse struct {
	Diterima      int                 `json:"diterima"`
	Duplikat      int                 `json:"duplikat"`
	DiLuarRentang []AlarmSuhuResponse `json:"di_luar_rentang"`
}

type TransaksiHasilResponse struct {
	TransaksiID   string  `json:"transaksi_id"`
	Jenis         string  `json:"jenis"`
	LotID         string  `json:"lot_id"`
	LotBaruID     *string `json:"lot_baru_id"`
	VolumeTon     float64 `json:"volume_ton"`
	VolumeSisaTon float64 `json:"volume_sisa_ton"`
	StatusLot     string  `json:"status_lot"`
}

// ToFasilitasResponse memetakan fasilitas (turunan sudah diisi usecase).
func ToFasilitasResponse(f domain.FasilitasView) FasilitasColdStorageResponse {
	return FasilitasColdStorageResponse{
		ID: f.ID, Nama: f.Nama, KabupatenKode: f.KabupatenKode, KabupatenNama: f.KabupatenNama,
		Lat: f.Lat, Lng: f.Lng, KapasitasTon: f.KapasitasTon, TerisiTon: f.TerisiTon, UtilisasiPersen: f.UtilisasiPersen,
		SuhuTargetC: f.SuhuTargetC, SuhuAktualC: f.SuhuAktualC, Status: f.Status,
		BatasMinC: f.BatasMinC, BatasMaksC: f.BatasMaksC, AlarmSuhu: f.Alarm,
		KelembapanAktualPct: f.KelembapanAktualPct, TelemetriPada: f.TelemetriPada,
	}
}

// ToStokResponse memetakan satu lot stok.
func ToStokResponse(s domain.StokView) StokColdStorageResponse {
	return StokColdStorageResponse{
		ID: s.ID, ColdStorageID: s.ColdStorageID, ColdStorageNama: s.ColdStorageNama,
		KomoditasKode: s.KomoditasKode, KomoditasNama: s.KomoditasNama, VolumeTon: s.VolumeTon,
		MasukPada: s.MasukPada, KedaluwarsaPada: s.KedaluwarsaPada, SisaMasaSimpanHari: s.SisaMasaSimpanHari,
		RisikoFoodLoss: s.RisikoFoodLoss, Pemilik: s.Pemilik, Status: s.Status,
	}
}

// ToBulkResponse memetakan hasil aksi massal.
func ToBulkResponse(berhasil, gagal int, volumeTon float64, baris []domain.BarisBulk) BulkColdStorageResponse {
	rincian := make([]BarisBulkResponse, len(baris))
	for i, b := range baris {
		rincian[i] = BarisBulkResponse{ID: b.ID, Berhasil: b.Berhasil, Pesan: b.Pesan}
	}
	return BulkColdStorageResponse{Berhasil: berhasil, Gagal: gagal, VolumeTon: volumeTon, Rincian: rincian}
}

// ToTransaksiResponse memetakan satu baris buku besar.
func ToTransaksiResponse(t domain.TransaksiView) TransaksiStokResponse {
	return TransaksiStokResponse{
		ID: t.ID, LotID: t.LotID, LotBaruID: t.LotBaruID, Jenis: t.Jenis, VolumeTon: t.VolumeTon,
		KomoditasKode: t.KomoditasKode, KomoditasNama: t.KomoditasNama,
		FasilitasAsalID: t.FasilitasAsalID, FasilitasTujuanID: t.FasilitasTujuanID,
		OlehNama: t.OlehNama, Waktu: t.Waktu, Alasan: t.Alasan,
	}
}
