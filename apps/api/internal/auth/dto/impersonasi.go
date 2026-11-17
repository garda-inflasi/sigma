package dto

import (
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/internal/auth/usecase"
)

// Kode galat mesin untuk impersonasi (kolom `code` amplop galat); lihat docs/impersonasi.md.
const (
	KodeAlasanWajib           = "ALASAN_WAJIB"
	KodeImpersonasiNonaktif   = "IMPERSONASI_NONAKTIF"
	KodeImpersonasiDitolak    = "IMPERSONASI_DITOLAK"
	KodeTargetTidakAda        = "TARGET_TIDAK_ADA"
	KodeTargetAmbigu          = "TARGET_AMBIGU"
	KodeTidakAdaImpersonasi   = "TIDAK_ADA_IMPERSONASI"
	KodeVersiKebijakanBerubah = "VERSI_KEBIJAKAN_BERUBAH"
	KodeKebijakanTidakValid   = "KEBIJAKAN_TIDAK_VALID"
	KodeIzinEksklusif         = "IZIN_EKSKLUSIF"
)

// ── Permintaan ────────────────────────────────────────────────────────

// MulaiImpersonasiRequest memulai (atau mengganti) impersonasi. Salah satu sasaran wajib; target_pengguna_id didahulukan.
type MulaiImpersonasiRequest struct {
	TargetPenggunaID string `json:"target_pengguna_id"`
	TargetPeran      string `json:"target_peran"`
	Alasan           string `json:"alasan"`
}

// PihakKebijakanRequest adalah satu pihak pada kebijakan: sebuah peran atau sebuah akun.
type PihakKebijakanRequest struct {
	Tipe       string `json:"tipe"`
	Peran      string `json:"peran"`
	PenggunaID string `json:"pengguna_id"`
}

// PelakuKebijakanRequest adalah satu pelaku beserta sasaran yang diizinkan baginya. Aktif tidak dikirim berarti aktif.
type PelakuKebijakanRequest struct {
	PihakKebijakanRequest
	Aktif  *bool                   `json:"aktif"`
	Target []PihakKebijakanRequest `json:"target"`
}

// KebijakanRequest mengganti seluruh kebijakan; Versi adalah versi yang dibaca klien (0 bila belum pernah disimpan).
type KebijakanRequest struct {
	Versi           int64                    `json:"versi"`
	Aktif           bool                     `json:"aktif"`
	WajibAlasan     bool                     `json:"wajib_alasan"`
	DurasiMaksMenit *int                     `json:"durasi_maks_menit"`
	Pelaku          []PelakuKebijakanRequest `json:"pelaku"`
}

// ToInput memetakan permintaan ke masukan usecase.
func (r KebijakanRequest) ToInput() usecase.KebijakanInput {
	pihak := func(p PihakKebijakanRequest) domain.PihakKebijakan {
		return domain.PihakKebijakan{Tipe: p.Tipe, Peran: p.Peran, PenggunaID: p.PenggunaID}
	}
	pelaku := make([]domain.PelakuKebijakan, len(r.Pelaku))
	for i, p := range r.Pelaku {
		aktif := p.Aktif == nil || *p.Aktif
		target := make([]domain.PihakKebijakan, len(p.Target))
		for j, t := range p.Target {
			target[j] = pihak(t)
		}
		pelaku[i] = domain.PelakuKebijakan{PihakKebijakan: pihak(p.PihakKebijakanRequest), Aktif: aktif, Target: target}
	}
	return usecase.KebijakanInput{
		Versi: r.Versi, Aktif: r.Aktif, WajibAlasan: r.WajibAlasan, DurasiMaksMenit: r.DurasiMaksMenit, Pelaku: pelaku,
	}
}

// RiwayatImpersonasiRequest adalah parameter query riwayat impersonasi.
type RiwayatImpersonasiRequest struct {
	Status string `form:"status"`
	Limit  int    `form:"limit"`
	Cursor string `form:"cursor"`
}

// ── Respons ───────────────────────────────────────────────────────────

// PihakKebijakanResponse adalah satu pihak pada kebijakan. Kolom pengguna_* hanya terisi untuk jenis PENGGUNA.
type PihakKebijakanResponse struct {
	Tipe          string  `json:"tipe"`
	Peran         *string `json:"peran"`
	PenggunaID    *string `json:"pengguna_id"`
	PenggunaNama  *string `json:"pengguna_nama"`
	PenggunaEmail *string `json:"pengguna_email"`
	PenggunaPeran *string `json:"pengguna_peran"`
}

// PelakuKebijakanResponse adalah satu pelaku beserta sasarannya.
type PelakuKebijakanResponse struct {
	PihakKebijakanResponse
	Aktif  bool                     `json:"aktif"`
	Target []PihakKebijakanResponse `json:"target"`
}

// KebijakanResponse adalah kebijakan impersonasi lengkap.
type KebijakanResponse struct {
	Aktif              bool                      `json:"aktif"`
	WajibAlasan        bool                      `json:"wajib_alasan"`
	DurasiMaksMenit    *int                      `json:"durasi_maks_menit"`
	Versi              int64                     `json:"versi"`
	DiperbaruiPada     *time.Time                `json:"diperbarui_pada"`
	DiperbaruiOleh     *string                   `json:"diperbarui_oleh"`
	DiperbaruiOlehNama *string                   `json:"diperbarui_oleh_nama"`
	Pelaku             []PelakuKebijakanResponse `json:"pelaku"`
}

func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func pihakResponse(p domain.PihakKebijakan) PihakKebijakanResponse {
	return PihakKebijakanResponse{
		Tipe: p.Tipe, Peran: ptr(p.Peran), PenggunaID: ptr(p.PenggunaID),
		PenggunaNama: ptr(p.PenggunaNama), PenggunaEmail: ptr(p.PenggunaEmail), PenggunaPeran: ptr(p.PenggunaPeran),
	}
}

// ToKebijakanResponse memetakan kebijakan untuk layar admin.
func ToKebijakanResponse(k usecase.KebijakanInfo) KebijakanResponse {
	pelaku := make([]PelakuKebijakanResponse, len(k.Pelaku))
	for i, p := range k.Pelaku {
		target := make([]PihakKebijakanResponse, len(p.Target))
		for j, t := range p.Target {
			target[j] = pihakResponse(t)
		}
		pelaku[i] = PelakuKebijakanResponse{PihakKebijakanResponse: pihakResponse(p.PihakKebijakan), Aktif: p.Aktif, Target: target}
	}
	return KebijakanResponse{
		Aktif: k.Aktif, WajibAlasan: k.WajibAlasan, DurasiMaksMenit: k.DurasiMaksMenit, Versi: k.Versi,
		DiperbaruiPada: k.DiperbaruiPada, DiperbaruiOleh: k.DiperbaruiOleh, DiperbaruiOlehNama: k.DiperbaruiOlehNama, Pelaku: pelaku,
	}
}

// IdentitasImpersonasi adalah satu pihak pada impersonasi (salinan saat dimulai).
type IdentitasImpersonasi struct {
	ID    *string `json:"id"`
	Nama  string  `json:"nama"`
	Email string  `json:"email,omitempty"`
	Peran string  `json:"peran"`
}

// RiwayatImpersonasiResponse adalah satu baris riwayat impersonasi.
type RiwayatImpersonasiResponse struct {
	ID              string               `json:"id"`
	Pelaku          IdentitasImpersonasi `json:"pelaku"`
	Sasaran         IdentitasImpersonasi `json:"sasaran"`
	Alasan          *string              `json:"alasan"`
	MulaiPada       time.Time            `json:"mulai_pada"`
	KedaluwarsaPada *time.Time           `json:"kedaluwarsa_pada"`
	BerakhirPada    *time.Time           `json:"berakhir_pada"`
	AlasanBerakhir  *string              `json:"alasan_berakhir"`
	Aktif           bool                 `json:"aktif"`
	IP              *string              `json:"ip"`
	Perangkat       string               `json:"perangkat"`
}

// RiwayatImpersonasiPageResponse adalah satu halaman riwayat; next_cursor kosong berarti halaman terakhir.
type RiwayatImpersonasiPageResponse struct {
	Items      []RiwayatImpersonasiResponse `json:"items"`
	NextCursor string                       `json:"next_cursor"`
}

// ToRiwayatImpersonasiResponse memetakan satu baris riwayat.
func ToRiwayatImpersonasiResponse(r domain.RiwayatImpersonasi) RiwayatImpersonasiResponse {
	ua := ""
	if r.UserAgent != nil {
		ua = *r.UserAgent
	}
	return RiwayatImpersonasiResponse{
		ID:     r.ID,
		Pelaku: IdentitasImpersonasi{ID: r.PelakuID, Nama: r.PelakuNama, Email: r.PelakuEmail, Peran: r.PelakuPeran},
		Sasaran: IdentitasImpersonasi{
			ID: r.SasaranID, Nama: r.SasaranNama, Email: r.SasaranEmail, Peran: r.SasaranPeran,
		},
		Alasan: r.Alasan, MulaiPada: r.MulaiPada, KedaluwarsaPada: r.KedaluwarsaPada,
		BerakhirPada: r.BerakhirEfektif, AlasanBerakhir: r.AlasanAkhirEfektif, Aktif: r.Aktif,
		IP: r.IP, Perangkat: RingkasPerangkat(ua),
	}
}

// AkunSasaranResponse adalah satu akun yang dapat diimpersonasi.
type AkunSasaranResponse struct {
	ID            string  `json:"id"`
	Nama          string  `json:"nama"`
	Email         string  `json:"email"`
	Lembaga       string  `json:"lembaga"`
	KabupatenKode *string `json:"kabupaten_kode"`
}

// PeranSasaranResponse adalah satu peran beserta akun yang dapat diimpersonasi.
type PeranSasaranResponse struct {
	Peran string                `json:"peran"`
	Label string                `json:"label"`
	Lapis string                `json:"lapis"`
	Akun  []AkunSasaranResponse `json:"akun"`
}

// SasaranImpersonasiResponse adalah jawaban pemilih sasaran.
type SasaranImpersonasiResponse struct {
	Boleh           bool                   `json:"boleh"`
	Aktif           bool                   `json:"aktif"`
	WajibAlasan     bool                   `json:"wajib_alasan"`
	DurasiMaksMenit *int                   `json:"durasi_maks_menit"`
	Peran           []PeranSasaranResponse `json:"peran"`
}

// ToSasaranImpersonasiResponse memetakan daftar sasaran.
func ToSasaranImpersonasiResponse(d usecase.DaftarSasaran) SasaranImpersonasiResponse {
	peran := make([]PeranSasaranResponse, len(d.Peran))
	for i, p := range d.Peran {
		akun := make([]AkunSasaranResponse, len(p.Akun))
		for j, a := range p.Akun {
			akun[j] = AkunSasaranResponse{ID: a.ID, Nama: a.Nama, Email: a.Email, Lembaga: a.Lembaga, KabupatenKode: a.KabupatenKode}
		}
		peran[i] = PeranSasaranResponse{Peran: p.Peran, Label: p.Label, Lapis: p.Lapis, Akun: akun}
	}
	return SasaranImpersonasiResponse{
		Boleh: d.Boleh, Aktif: d.Aktif, WajibAlasan: d.WajibAlasan, DurasiMaksMenit: d.DurasiMaksMenit, Peran: peran,
	}
}

// PelakuImpersonasiMeta dan SasaranImpersonasiMeta adalah ringkasan pihak pada keadaan impersonasi yang berjalan.
type PelakuImpersonasiMeta struct {
	ID    string `json:"id"`
	Nama  string `json:"nama"`
	Email string `json:"email"`
	Peran string `json:"peran"`
}

type SasaranImpersonasiMeta struct {
	ID    string `json:"id"`
	Nama  string `json:"nama"`
	Peran string `json:"peran"`
}

// ImpersonasiMetaResponse adalah keadaan impersonasi yang sedang berjalan (GET /auth/me dan respons mulai).
type ImpersonasiMetaResponse struct {
	Aktif           bool                   `json:"aktif"`
	SesiID          string                 `json:"sesi_id"`
	Pelaku          PelakuImpersonasiMeta  `json:"pelaku"`
	Sasaran         SasaranImpersonasiMeta `json:"sasaran"`
	MulaiPada       time.Time              `json:"mulai_pada"`
	KedaluwarsaPada *time.Time             `json:"kedaluwarsa_pada"`
	Alasan          *string                `json:"alasan"`
}

// ToImpersonasiMetaResponse memetakan keadaan impersonasi; nil bila bukan sesi impersonasi.
func ToImpersonasiMetaResponse(m *usecase.ImpersonasiMeta) *ImpersonasiMetaResponse {
	if m == nil {
		return nil
	}
	return &ImpersonasiMetaResponse{
		Aktif: true, SesiID: m.SesiID,
		Pelaku:    PelakuImpersonasiMeta{ID: m.PelakuID, Nama: m.PelakuNama, Email: m.PelakuEmail, Peran: m.PelakuPeran},
		Sasaran:   SasaranImpersonasiMeta{ID: m.SasaranID, Nama: m.SasaranNama, Peran: m.SasaranPeran},
		MulaiPada: m.MulaiPada, KedaluwarsaPada: m.KedaluwarsaPada, Alasan: m.Alasan,
	}
}

// MulaiImpersonasiResponse memuat sesi impersonasi baru: BFF memasang token pada cookie dan tidak meneruskannya ke browser.
type MulaiImpersonasiResponse struct {
	AccessToken  string                   `json:"access_token"`
	RefreshToken string                   `json:"refresh_token"`
	User         UserResponse             `json:"user"`
	Impersonasi  *ImpersonasiMetaResponse `json:"impersonasi"`
}

// HentiImpersonasiResponse memuat sesi biasa pelaku yang dikembalikan.
type HentiImpersonasiResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	User         UserResponse `json:"user"`
}
