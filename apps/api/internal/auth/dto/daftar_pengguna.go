package dto

import (
	"strconv"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
)

// FilterPenggunaRequest adalah parameter kueri penyaring daftar pengguna. Daftar (GET /v1/users) dan ekspor
// (GET /v1/users/ekspor) menyematkannya, jadi keduanya menerima filter yang persis sama dan galat yang sama:
//
//	search    potongan nama, email, peran, atau lembaga (maks 100 karakter)
//	peran     satu nilai enum peran
//	status    AKTIF | NONAKTIF | MENUNGGU
//	lapis     STRATEGIS | MANAJERIAL | OPERASIONAL | SISTEM
//	mfa       true | false  (MFA aktif / belum aktif)
//	terkunci  true | false  (login sedang terkunci / tidak)
//
// Parameter kosong atau tidak ada berarti tanpa filter. Nilai lain ditolak 400.
type FilterPenggunaRequest struct {
	Search   string `form:"search"`
	Peran    string `form:"peran"`
	Status   string `form:"status"`
	Lapis    string `form:"lapis"`
	MFA      string `form:"mfa"`
	Terkunci string `form:"terkunci"`
}

// Filter mengubah parameter kueri menjadi domain.FilterPengguna. Boolean harus persis "true" atau "false"
// (bukan "1", "0", atau kosong-berarti-false); galatnya *domain.FilterPenggunaError. Validasi nilai enum dan
// panjang kata kunci dilakukan domain.DaftarPenggunaQuery.Normal dan usecase.ListUsers.
func (r FilterPenggunaRequest) Filter() (domain.FilterPengguna, error) {
	mfa, err := parseBoolKueri("mfa", r.MFA)
	if err != nil {
		return domain.FilterPengguna{}, err
	}
	terkunci, err := parseBoolKueri("terkunci", r.Terkunci)
	if err != nil {
		return domain.FilterPengguna{}, err
	}
	return domain.FilterPengguna{
		Cari: r.Search, Peran: r.Peran, Status: r.Status, Lapis: r.Lapis, MFA: mfa, Terkunci: terkunci,
	}, nil
}

// parseBoolKueri membaca "true"/"false"; kosong = nil (tanpa filter).
func parseBoolKueri(param, nilai string) (*bool, error) {
	if nilai == "" {
		return nil, nil
	}
	if nilai != "true" && nilai != "false" {
		return nil, &domain.FilterPenggunaError{Param: param, Pesan: param + " harus true atau false, bukan " + strconv.Quote(nilai)}
	}
	b := nilai == "true"
	return &b, nil
}

// Query mengubah kueri daftar menjadi domain.DaftarPenggunaQuery (belum dinormalkan).
func (r ListUserRequest) Query() (domain.DaftarPenggunaQuery, error) {
	filter, err := r.FilterPenggunaRequest.Filter()
	if err != nil {
		return domain.DaftarPenggunaQuery{}, err
	}
	return domain.DaftarPenggunaQuery{Filter: filter, Urut: r.Sort, Arah: r.Arah, Page: r.Page, Limit: r.Limit}, nil
}

// ── Reset MFA oleh admin ──────────────────────────────────────────────

// ResetMFAResponse adalah hasil POST /v1/users/:id/reset-mfa.
type ResetMFAResponse struct {
	ID string `json:"id"`
	// MFAAktifSebelumnya false berarti pengguna memang tanpa MFA: tidak ada yang berubah dan sesinya tidak disentuh.
	MFAAktifSebelumnya bool `json:"mfa_aktif_sebelumnya"`
	// SesiDicabut adalah jumlah sesi pengguna yang diakhiri.
	SesiDicabut int `json:"sesi_dicabut"`
}

// ── Riwayat peran ─────────────────────────────────────────────────────

// RiwayatPeranRequest adalah kueri GET /v1/users/:id/riwayat-peran.
type RiwayatPeranRequest struct {
	Limit int `form:"limit" binding:"omitempty,min=1,max=200"`
}

// AktorRiwayatPeran adalah pelaku perubahan peran (snapshot di audit). ID kosong bila akunnya sudah dihapus.
type AktorRiwayatPeran struct {
	ID   *string `json:"id"`
	Nama *string `json:"nama"`
}

// PerubahanPeranResponse adalah satu perubahan peran.
type PerubahanPeranResponse struct {
	// ID adalah id baris audit sumbernya (GET /v1/audit/:id).
	ID    string    `json:"id"`
	Waktu time.Time `json:"waktu"`
	// Aksi adalah aksi audit sumber: CREATE_USER (peran awal), ASSIGN_ROLE, UPDATE_USER, ...
	Aksi string `json:"aksi"`
	// Dari kosong (null) untuk peran awal saat akun dibuat.
	Dari *string `json:"dari"`
	Ke   string  `json:"ke"`
	// Aktor kosong (null) bila aksinya tanpa pelaku terautentikasi.
	Aktor *AktorRiwayatPeran `json:"aktor"`
}

// RiwayatPeranResponse adalah riwayat peran satu pengguna, terbaru lebih dulu.
type RiwayatPeranResponse struct {
	Items []PerubahanPeranResponse `json:"items"`
	// Terpotong true bila ada entri lebih lama di luar limit.
	Terpotong bool `json:"terpotong"`
}

// ToRiwayatPeranResponse memetakan riwayat peran domain ke representasi API.
func ToRiwayatPeranResponse(items []domain.PerubahanPeran, terpotong bool) RiwayatPeranResponse {
	out := make([]PerubahanPeranResponse, len(items))
	for i, p := range items {
		out[i] = PerubahanPeranResponse{ID: p.AuditID, Waktu: p.Waktu.UTC(), Aksi: p.Aksi, Dari: p.Dari, Ke: p.Ke}
		if p.AktorID != nil || p.AktorNama != nil {
			out[i].Aktor = &AktorRiwayatPeran{ID: p.AktorID, Nama: p.AktorNama}
		}
	}
	return RiwayatPeranResponse{Items: out, Terpotong: terpotong}
}
