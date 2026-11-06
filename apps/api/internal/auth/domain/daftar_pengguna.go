package domain

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// Daftar pengguna di sisi server (GET /v1/users): pencarian, filter, urutan, dan paginasi dalam satu
// kontrak yang dipakai bersama oleh daftar dan ekspor CSV (GET /v1/users/ekspor). Seluruh masukan divalidasi di
// sini (Normal), jadi pemanggil mana pun, HTTP maupun usecase lain, mendapat aturan dan galat yang sama.

// Kolom yang boleh dipakai sebagai urutan. Daftar ini adalah daftar putih: nilai lain ditolak, dan SQL-nya
// (repositori) hanya mengenal nilai-nilai ini.
const (
	UrutNama          = "nama"
	UrutEmail         = "email"
	UrutPeran         = "peran"
	UrutLembaga       = "lembaga"
	UrutStatus        = "status"
	UrutTerakhirMasuk = "terakhir_masuk"
	UrutDibuatPada    = "dibuat_pada"
)

// KolomUrutPengguna adalah seluruh nilai `sort` yang sah.
var KolomUrutPengguna = []string{UrutNama, UrutEmail, UrutPeran, UrutLembaga, UrutStatus, UrutTerakhirMasuk, UrutDibuatPada}

// Arah urutan.
const (
	ArahNaik  = "asc"
	ArahTurun = "desc"
)

// Nilai enum basis data yang boleh menjadi filter (peran divalidasi usecase terhadap katalog peran).
var (
	StatusPengguna = []string{"AKTIF", "NONAKTIF", "MENUNGGU"}
	LapisPengguna  = []string{"STRATEGIS", "MANAJERIAL", "OPERASIONAL", "SISTEM"}
)

// Batas daftar.
const (
	// LimitBawaanPengguna dipakai bila limit tidak diberikan.
	LimitBawaanPengguna = 20
	// MaksLimitPengguna adalah batas satu halaman yang diterima HTTP (400 bila lebih).
	MaksLimitPengguna = 100
	// MaksPanjangCari membatasi kata kunci pencarian.
	MaksPanjangCari = 100
)

// FilterPenggunaError adalah masukan daftar yang salah (nilai filter/urutan tidak dikenal, boolean rusak).
// Pesannya aman ditampilkan ke klien; HTTP menjawabnya 400.
type FilterPenggunaError struct {
	// Param adalah nama parameter kueri yang salah (search, peran, status, lapis, mfa, terkunci, sort, arah).
	Param string
	Pesan string
}

func (e *FilterPenggunaError) Error() string { return e.Pesan }

func galatFilter(param, format string, args ...any) error {
	return &FilterPenggunaError{Param: param, Pesan: fmt.Sprintf(format, args...)}
}

// FilterPengguna adalah penyaring daftar pengguna. Nilai kosong (atau nil) berarti "semua". Penyaring
// digabung dengan AND; Cari mencocokkan nama, email, peran, atau lembaga.
type FilterPengguna struct {
	// Cari adalah potongan teks tak peka huruf besar-kecil pada nama, email, peran, atau lembaga.
	Cari string
	// Peran, Status, dan Lapis mencocokkan nilai enum persis.
	Peran, Status, Lapis string
	// MFA true = hanya akun dengan MFA aktif, false = hanya yang belum aktif.
	MFA *bool
	// Terkunci true = hanya akun yang login-nya sedang terkunci (lockout, 01-F), false = hanya yang tidak.
	Terkunci *bool
}

// DaftarPenggunaQuery adalah satu permintaan daftar pengguna: filter, urutan, dan halaman.
type DaftarPenggunaQuery struct {
	Filter FilterPengguna
	// Urut adalah salah satu KolomUrutPengguna; kosong = dibuat_pada.
	Urut string
	// Arah adalah ArahNaik atau ArahTurun; kosong = naik bila Urut diberikan, selain itu turun (terbaru dulu).
	Arah string
	// Page mulai dari 1 (kurang dari itu dianggap 1); Limit kosong = LimitBawaanPengguna.
	Page, Limit int
	// Sekarang adalah waktu acuan filter Terkunci; kosong = diisi pemanggil (usecase memakai jamnya).
	Sekarang time.Time
}

// Normal memvalidasi query dan mengembalikan salinan berisi nilai bawaan yang sudah diisi (urutan, arah,
// halaman, limit) dan kata kunci tanpa spasi tepi. Galatnya *FilterPenggunaError. Peran tidak diperiksa di
// sini karena katalog peran ada di usecase.
func (q DaftarPenggunaQuery) Normal() (DaftarPenggunaQuery, error) {
	q.Filter.Cari = strings.TrimSpace(q.Filter.Cari)
	if utf8.RuneCountInString(q.Filter.Cari) > MaksPanjangCari {
		return q, galatFilter("search", "search maksimal %d karakter", MaksPanjangCari)
	}
	if q.Filter.Status != "" && !slices.Contains(StatusPengguna, q.Filter.Status) {
		return q, galatFilter("status", "status tidak dikenal: %q (pilihan: %s)", q.Filter.Status, strings.Join(StatusPengguna, ", "))
	}
	if q.Filter.Lapis != "" && !slices.Contains(LapisPengguna, q.Filter.Lapis) {
		return q, galatFilter("lapis", "lapis tidak dikenal: %q (pilihan: %s)", q.Filter.Lapis, strings.Join(LapisPengguna, ", "))
	}

	urutDiberikan := q.Urut != ""
	if !urutDiberikan {
		q.Urut = UrutDibuatPada
	}
	if !slices.Contains(KolomUrutPengguna, q.Urut) {
		return q, galatFilter("sort", "sort tidak dikenal: %q (pilihan: %s)", q.Urut, strings.Join(KolomUrutPengguna, ", "))
	}
	switch q.Arah {
	case ArahNaik, ArahTurun:
	case "":
		q.Arah = ArahTurun
		if urutDiberikan {
			q.Arah = ArahNaik
		}
	default:
		return q, galatFilter("arah", "arah tidak dikenal: %q (pilihan: %s, %s)", q.Arah, ArahNaik, ArahTurun)
	}

	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 {
		q.Limit = LimitBawaanPengguna
	}
	return q, nil
}

// Offset adalah jumlah baris yang dilewati halaman ini.
func (q DaftarPenggunaQuery) Offset() int { return (q.Page - 1) * q.Limit }

// DaftarPenggunaRepository membaca daftar pengguna. Dipisah dari Repository supaya jalur baca daftar tidak
// bercampur dengan penulisan; Repository menyematkannya.
type DaftarPenggunaRepository interface {
	// ListUsers mengembalikan satu halaman pengguna sesuai q (yang sudah Normal) beserta jumlah seluruh baris
	// yang cocok dengan filter. Urutan stabil: kolom urut lalu id pada arah yang sama, jadi paginasi tidak
	// menggandakan atau melewatkan baris walau nilai urutnya kembar.
	ListUsers(ctx context.Context, q DaftarPenggunaQuery) ([]Pengguna, int64, error)
}
