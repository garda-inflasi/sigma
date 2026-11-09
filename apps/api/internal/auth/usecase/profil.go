package usecase

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"gorm.io/gorm"
)

// Profil mandiri (slice 20-A, docs/todos/20-profile.md): pemilik akun mengubah nama dan telepon sendiri lewat
// PATCH /v1/auth/me. Email, peran, lapis, lembaga, kabupaten, dan status bukan kewenangannya: ProfilPatch tidak
// membawanya, jadi tidak ada jalur untuk mengubahnya dari sini.

const (
	// NamaMinKarakter dan NamaMaksKarakter membatasi nama (dihitung dalam rune, sesudah dipangkas).
	NamaMinKarakter  = 3
	NamaMaksKarakter = 100
)

// Galat validasi profil: handler memetakannya ke 400 dengan pesan Indonesia dari kontrak API.
var (
	ErrNamaTerlaluPendek  = errors.New("nama minimal 3 karakter")
	ErrNamaTerlaluPanjang = errors.New("nama maksimal 100 karakter")
	ErrTeleponTidakValid  = errors.New("nomor telepon tidak valid")
	ErrTidakAdaPerubahan  = errors.New("tidak ada perubahan")
)

// ProfilPatch adalah perubahan profil oleh pemiliknya. Nil berarti tidak diubah; Telepon "" menghapus nomor.
type ProfilPatch struct {
	Nama    *string
	Telepon *string
}

// teleponNasional adalah bagian nomor sesudah kode negara 62: 8-13 digit, digit pertama bukan nol (total E.164
// 10-15 digit). Sama dengan CHECK chk_pengguna_telepon (migrasi 000123) dan dengan aturan di sisi web.
var teleponNasional = regexp.MustCompile(`^[1-9][0-9]{7,12}$`)

// NormalisasiTelepon memeriksa nomor telepon Indonesia dan mengembalikannya dalam E.164 ("+62" + nomor nasional).
//
//  1. Dipangkas; string kosong berarti hapus nomor: kembalian "" tanpa galat (disimpan NULL).
//  2. Spasi, "-", ".", "(", dan ")" dibuang.
//  3. Awalan "+62" dipertahankan, "62" menjadi "+62", dan "0" menjadi "+62". Awalan lain tidak valid.
//  4. Bagian nasional harus 8-13 digit dengan digit pertama 1-9.
//
// Contoh: "0812-3456-7890", "62 812 3456 7890", dan "+62 (812) 3456-7890" semuanya menjadi "+6281234567890".
func NormalisasiTelepon(mentah string) (string, error) {
	mentah = strings.TrimSpace(mentah)
	if mentah == "" {
		return "", nil
	}
	bersih := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || strings.ContainsRune("-.()", r) {
			return -1
		}
		return r
	}, mentah)

	var nasional string
	switch {
	case strings.HasPrefix(bersih, "+62"):
		nasional = bersih[len("+62"):]
	case strings.HasPrefix(bersih, "62"):
		nasional = bersih[len("62"):]
	case strings.HasPrefix(bersih, "0"):
		nasional = bersih[len("0"):]
	default:
		return "", ErrTeleponTidakValid
	}
	if !teleponNasional.MatchString(nasional) {
		return "", ErrTeleponTidakValid
	}
	return "+62" + nasional, nil
}

// normalisasiNama memangkas nama dan memeriksa panjangnya dalam rune. Byte UTF-8 rusak dan NUL dibuang lebih dulu:
// PostgreSQL menolak keduanya (22021), yang akan tampil sebagai galat server.
func normalisasiNama(mentah string) (string, error) {
	nama := strings.TrimSpace(strings.ReplaceAll(strings.ToValidUTF8(mentah, ""), "\x00", ""))
	switch n := utf8.RuneCountInString(nama); {
	case n < NamaMinKarakter:
		return "", ErrNamaTerlaluPendek
	case n > NamaMaksKarakter:
		return "", ErrNamaTerlaluPanjang
	}
	return nama, nil
}

// teleponBerubah true bila nomor baru (hasil NormalisasiTelepon; "" = hapus) berbeda dari yang tersimpan.
func teleponBerubah(tersimpan *string, baru string) bool {
	if tersimpan == nil {
		return baru != ""
	}
	return *tersimpan != baru
}

// UpdateProfil mengubah nama dan/atau telepon pemilik akun. Hanya kolom yang benar-benar berbeda dari yang tersimpan
// yang ditulis (PatchUser); permintaan yang tidak mengubah apa pun tidak menulis basis data dan tidak diaudit, dan
// mengembalikan pengguna apa adanya. Perubahan dicatat sebagai UPDATE_PROFIL dengan keadaan sebelum dan sesudah
// (telepon tersamar) serta metadata telepon_diubah bila nomor berganti.
func (uc *UseCase) UpdateProfil(ctx context.Context, userID string, patch ProfilPatch) (*domain.Pengguna, error) {
	if patch.Nama == nil && patch.Telepon == nil {
		return nil, ErrTidakAdaPerubahan
	}
	var nama, telepon *string
	if patch.Nama != nil {
		n, err := normalisasiNama(*patch.Nama)
		if err != nil {
			return nil, err
		}
		nama = &n
	}
	if patch.Telepon != nil {
		t, err := NormalisasiTelepon(*patch.Telepon)
		if err != nil {
			return nil, err
		}
		telepon = &t
	}

	user, err := uc.muatPengguna(ctx, userID)
	if err != nil {
		return nil, err
	}
	var ubah domain.PenggunaPatch
	if nama != nil && *nama != user.Nama {
		ubah.Nama = nama
	}
	if telepon != nil && teleponBerubah(user.Telepon, *telepon) {
		ubah.Telepon = telepon
	}
	if ubah.Kosong() {
		return user, nil
	}

	sebelum, sesudah, err := uc.repo.PatchUser(ctx, userID, ubah)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("update profil: %w", err)
	}
	uc.writeAudit(ctx, audit.Entry{
		AktorID: userID, Aksi: "UPDATE_PROFIL", Entitas: "pengguna", EntitasID: userID,
		Hasil: audit.HasilBerhasil, Sebelum: userSnapshot(sebelum), Sesudah: userSnapshot(sesudah),
		Metadata: metadataPerubahanAkun(sebelum, sesudah, nil),
	})
	return sesudah, nil
}
