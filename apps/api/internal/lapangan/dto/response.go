package dto

import (
	"time"

	"github.com/garda-inflasi/api/internal/lapangan/domain"
	"github.com/garda-inflasi/api/internal/lapangan/usecase"
	"github.com/garda-inflasi/api/pkg/idx"
)

// TugasLapanganResponse adalah satu tugas survei. Terisi adalah jumlah komoditas
// yang sudah tercatat terhadap Target.
type TugasLapanganResponse struct {
	// ID adalah ID BIGINT dalam string desimal ("42"). Kode berisi ID lama (TGS-<slug>-<n>) untuk tugas dari sebelum
	// migrasi 000111, NULL untuk tugas baru; tampilkan kode ?? id.
	ID        idx.ID    `json:"id"`
	Kode      *string   `json:"kode"`
	Judul     string    `json:"judul"`
	PasarID   string    `json:"pasar_id"`
	PasarNama string    `json:"pasar_nama"`
	Komoditas []string  `json:"komoditas"`
	Tenggat   time.Time `json:"tenggat"`
	Status    string    `json:"status"`
	Target    int       `json:"target"`
	Terisi    int       `json:"terisi"`
	Tipe      string    `json:"tipe"`
}

// ToTugasLapanganResponse memetakan tugas ke respons; Komoditas selalu larik (bukan null).
func ToTugasLapanganResponse(t usecase.Tugas) TugasLapanganResponse {
	komoditas := []string(t.Komoditas)
	if komoditas == nil {
		komoditas = []string{}
	}
	return TugasLapanganResponse{
		ID: t.ID, Kode: t.Kode, Judul: t.Judul, PasarID: t.PasarID, PasarNama: t.PasarNama, Komoditas: komoditas,
		Tenggat: t.Tenggat, Status: t.StatusTampil, Target: t.Target, Terisi: t.JumlahSelesai, Tipe: t.Tipe,
	}
}

// RincianKomoditasResponse adalah satu komoditas tugas; Terisi benar bila sudah ada entri tertaut.
type RincianKomoditasResponse struct {
	Kode   string `json:"kode"`
	Nama   string `json:"nama"`
	Terisi bool   `json:"terisi"`
}

// TugasDetailResponse adalah tugas (semua bidang daftar) ditambah rincian per komoditas.
type TugasDetailResponse struct {
	TugasLapanganResponse
	Rincian []RincianKomoditasResponse `json:"rincian_komoditas"`
}

// ToTugasDetailResponse memetakan detail tugas; Rincian selalu larik (bukan null).
func ToTugasDetailResponse(d usecase.Detail) TugasDetailResponse {
	rincian := make([]RincianKomoditasResponse, len(d.Rincian))
	for i, r := range d.Rincian {
		rincian[i] = RincianKomoditasResponse{Kode: r.Kode, Nama: r.Nama, Terisi: r.Terisi}
	}
	return TugasDetailResponse{TugasLapanganResponse: ToTugasLapanganResponse(d.Tugas), Rincian: rincian}
}

// BulkTugasResponse melaporkan tugas yang diproses dan yang dilewati.
type BulkTugasResponse struct {
	Berhasil int `json:"berhasil"`
	Gagal    int `json:"gagal"`
}

// AcuanResponse adalah paket data acuan offline. Versi juga dikirim sebagai ETag.
type AcuanResponse struct {
	Versi      string               `json:"versi"`
	Toleransi  float64              `json:"toleransi"`
	Komoditas  []AcuanKomoditasItem `json:"komoditas"`
	Pasar      []AcuanPasarItem     `json:"pasar"`
	HargaAcuan []usecase.HargaAcuan `json:"harga_acuan"`
}

// AcuanKomoditasItem adalah komoditas aktif dengan harga acuan dasarnya.
type AcuanKomoditasItem struct {
	Kode       string `json:"kode"`
	Nama       string `json:"nama"`
	Satuan     string `json:"satuan"`
	HargaAcuan int    `json:"harga_acuan"`
}

// AcuanPasarItem adalah pasar aktif beserta geofence-nya.
type AcuanPasarItem struct {
	ID            string  `json:"id"`
	Nama          string  `json:"nama"`
	KabupatenKode string  `json:"kabupaten_kode"`
	Lat           float64 `json:"lat"`
	Lng           float64 `json:"lng"`
	RadiusMeter   int     `json:"radius_meter"`
}

// ToAcuanResponse memetakan data acuan; semua larik selalu terisi (bukan null).
func ToAcuanResponse(d usecase.DataAcuan) AcuanResponse {
	res := AcuanResponse{
		Versi: d.Versi, Toleransi: d.Toleransi,
		Komoditas:  make([]AcuanKomoditasItem, len(d.Komoditas)),
		Pasar:      make([]AcuanPasarItem, len(d.Pasar)),
		HargaAcuan: d.HargaAcuan,
	}
	if res.HargaAcuan == nil {
		res.HargaAcuan = []usecase.HargaAcuan{}
	}
	for i, k := range d.Komoditas {
		res.Komoditas[i] = AcuanKomoditasItem{Kode: k.Kode, Nama: k.Nama, Satuan: k.Satuan, HargaAcuan: k.HargaAcuan}
	}
	for i, p := range d.Pasar {
		res.Pasar[i] = AcuanPasarItem{ID: p.ID, Nama: p.Nama, KabupatenKode: p.KabupatenKode, Lat: p.Lat, Lng: p.Lng, RadiusMeter: p.RadiusMeter}
	}
	return res
}

// LampiranFotoResponse adalah balasan POST /v1/lapangan/harga/:id/foto.
type LampiranFotoResponse struct {
	ID         idx.ID `json:"id"`
	EntriID    idx.ID `json:"entri_id"`
	FotoURL    string `json:"foto_url"`
	MIME       string `json:"mime"`
	Ukuran     int    `json:"ukuran"`
	Checksum   string `json:"checksum"`
	DibuatPada string `json:"dibuat_pada"`
}

func ToLampiranFotoResponse(l domain.LampiranEntri, fotoURL string) LampiranFotoResponse {
	return LampiranFotoResponse{
		ID:         l.ID,
		EntriID:    l.EntriID,
		FotoURL:    fotoURL,
		MIME:       l.MIME,
		Ukuran:     l.Ukuran,
		Checksum:   l.Checksum,
		DibuatPada: l.DibuatPada.UTC().Format(time.RFC3339),
	}
}

// KodeEtikResponse adalah balasan GET /v1/lapangan/kode-etik.
type KodeEtikResponse struct {
	Versi         string  `json:"versi"`
	Judul         string  `json:"judul"`
	Isi           string  `json:"isi"`
	SudahSetuju   bool    `json:"sudah_setuju"`
	DisetujuiPada *string `json:"disetujui_pada,omitempty"`
}

func ToKodeEtikResponse(k domain.KodeEtikStatus) KodeEtikResponse {
	var t *string
	if k.DisetujuiPada != nil {
		s := k.DisetujuiPada.UTC().Format(time.RFC3339)
		t = &s
	}
	return KodeEtikResponse{
		Versi:         k.Versi,
		Judul:         k.Judul,
		Isi:           k.Isi,
		SudahSetuju:   k.SudahSetuju,
		DisetujuiPada: t,
	}
}

// PersetujuanKodeEtikResponse adalah balasan POST /v1/lapangan/kode-etik/setuju.
type PersetujuanKodeEtikResponse struct {
	ID            idx.ID `json:"id"`
	Versi         string `json:"versi"`
	DisetujuiPada string `json:"disetujui_pada"`
}

func ToPersetujuanKodeEtikResponse(p domain.PersetujuanKodeEtik) PersetujuanKodeEtikResponse {
	return PersetujuanKodeEtikResponse{
		ID:            p.ID,
		Versi:         p.Versi,
		DisetujuiPada: p.DisetujuiPada.UTC().Format(time.RFC3339),
	}
}

// KinerjaPetugasResponse adalah item balasan GET /v1/lapangan/kinerja.
type KinerjaPetugasResponse struct {
	PetugasID           string  `json:"petugas_id"`
	NamaPetugas         string  `json:"nama_petugas"`
	KabupatenKode       *string `json:"kabupaten_kode,omitempty"`
	TotalEntri          int     `json:"total_entri"`
	EntriValid          int     `json:"entri_valid"`
	EntriDitolak        int     `json:"entri_ditolak"`
	RasioPenolakanPct   float64 `json:"rasio_penolakan_pct"`
	TotalTugas          int     `json:"total_tugas"`
	TugasSelesai        int     `json:"tugas_selesai"`
	TugasTepatWaktu     int     `json:"tugas_tepat_waktu"`
	KetepatanTenggatPct float64 `json:"ketepatan_tenggat_pct"`
}

func ToKinerjaPetugasResponse(k domain.KinerjaPetugas) KinerjaPetugasResponse {
	return KinerjaPetugasResponse{
		PetugasID:           k.PetugasID,
		NamaPetugas:         k.NamaPetugas,
		KabupatenKode:       k.KabupatenKode,
		TotalEntri:          k.TotalEntri,
		EntriValid:          k.EntriValid,
		EntriDitolak:        k.EntriDitolak,
		RasioPenolakanPct:   k.RasioPenolakanPct,
		TotalTugas:          k.TotalTugas,
		TugasSelesai:        k.TugasSelesai,
		TugasTepatWaktu:     k.TugasTepatWaktu,
		KetepatanTenggatPct: k.KetepatanTenggatPct,
	}
}
