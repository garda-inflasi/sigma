package dto

import (
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
)

// ── Undangan ──────────────────────────────────────────────────────────

// UndangPenggunaRequest adalah isian POST /v1/users/undang. Validasi isi (email, peran, kabupaten) ada di
// usecase supaya pesannya sama dengan impor CSV.
type UndangPenggunaRequest struct {
	Nama          string `json:"nama" binding:"required"`
	Email         string `json:"email" binding:"required"`
	Peran         string `json:"peran" binding:"required"`
	Lembaga       string `json:"lembaga" binding:"required"`
	KabupatenKode string `json:"kabupaten_kode"`
}

// UndanganResponse adalah hasil mengundang atau mengirim ulang undangan.
type UndanganResponse struct {
	User UserResponse `json:"user"`
	// BerlakuSampai adalah akhir masa berlaku tautan undangan.
	BerlakuSampai time.Time `json:"berlaku_sampai"`
	// EmailTerkirim false bila SMTP tidak dikonfigurasi, menolak, atau tidak menjawab: akun dan undangan tetap
	// ada, kirim ulang undangannya.
	EmailTerkirim bool `json:"email_terkirim"`
}

// AktivasiRequest adalah isian POST /v1/auth/aktivasi (publik).
type AktivasiRequest struct {
	Token    string `json:"token" binding:"required,max=512"`
	Password string `json:"password" binding:"required,min=8"`
}

// ── Impor ─────────────────────────────────────────────────────────────

// ImporPenggunaRequest adalah isian POST /v1/users/impor. DryRun wajib ada (true = hanya periksa).
type ImporPenggunaRequest struct {
	CSV    string `json:"csv" binding:"required"`
	DryRun *bool  `json:"dry_run" binding:"required"`
}

// ImporBarisResponse adalah hasil satu baris berkas.
type ImporBarisResponse struct {
	Baris         int      `json:"baris"`
	Nama          string   `json:"nama"`
	Email         string   `json:"email"`
	Peran         string   `json:"peran"`
	Lembaga       string   `json:"lembaga"`
	KabupatenKode string   `json:"kabupaten_kode"`
	Status        string   `json:"status"`
	Galat         []string `json:"galat"`
	ID            string   `json:"id,omitempty"`
}

// ImporPenggunaResponse adalah ringkasan dan hasil per baris sebuah impor.
type ImporPenggunaResponse struct {
	DryRun     bool                 `json:"dry_run"`
	Total      int                  `json:"total"`
	Valid      int                  `json:"valid"`
	Galat      int                  `json:"galat"`
	Dibuat     int                  `json:"dibuat"`
	EmailAktif bool                 `json:"email_aktif"`
	Baris      []ImporBarisResponse `json:"baris"`
}

// ── Ekspor ────────────────────────────────────────────────────────────

// EksporPenggunaRequest adalah query GET /v1/users/ekspor: filter yang sama dengan daftar (GET /v1/users),
// ditambah ids untuk "ekspor yang dipilih". Kriteria baru daftar pengguna ditambahkan di sini dan di
// repository/postgres/filter_pengguna.go.
type EksporPenggunaRequest struct {
	Search string `form:"search"`
	// IDs adalah id pengguna dipisah koma.
	IDs string `form:"ids"`
}

// Filter mengubah query menjadi filter domain: spasi tepi dibuang, id kosong dan ganda dibuang.
func (r EksporPenggunaRequest) Filter() domain.FilterPengguna {
	f := domain.FilterPengguna{Search: strings.TrimSpace(r.Search)}
	terlihat := map[string]bool{}
	for _, id := range strings.Split(r.IDs, ",") {
		id = strings.TrimSpace(id)
		if id == "" || terlihat[id] {
			continue
		}
		terlihat[id] = true
		f.IDs = append(f.IDs, id)
	}
	return f
}
