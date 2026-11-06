package usecase

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/domain"
)

// Ekspor pengguna ke CSV (02-D). Data pribadi (UU PDP) keluar dari sistem, jadi:
//   - hanya pengguna.kelola, dengan filter daftar yang sama (domain.FilterPengguna);
//   - dicatat sebagai EKSPOR_DATA SEBELUM berkas diserahkan: bila audit gagal, tidak ada data yang keluar;
//   - kolomnya daftar putih tetap, tanpa rahasia (kata sandi, rahasia MFA, kode cadangan, token) dan tanpa
//     kolom dari input pengguna selain yang tertera;
//   - sel yang diawali = + - @ (atau tab/CR) diberi awalan apostrof supaya tidak dieksekusi sebagai rumus saat
//     dibuka di spreadsheet (sama dengan ekspor audit).
// Format: titik koma, UTF-8 dengan BOM (terbuka benar di Excel berlokal Indonesia), seperti ekspor audit.

const (
	// EksporMaksBaris membatasi satu ekspor; sisanya ditandai terpotong (X-Export-Truncated).
	EksporMaksBaris = 20000
	// EksporMaksID membatasi "ekspor yang dipilih" (id dikirim di query string).
	EksporMaksID = 500
)

// KolomEkspor adalah header berkas ekspor, urut.
var KolomEkspor = []string{
	"id", "nama", "email", "peran", "lapis", "lembaga", "kabupaten_kode", "status", "mfa_aktif", "terakhir_masuk", "dibuat_pada",
}

// HasilEkspor adalah berkas CSV siap serah beserta ringkasannya.
type HasilEkspor struct {
	CSV       []byte
	Baris     int
	Terpotong bool
}

// csvAman menetralkan sel yang akan dibaca spreadsheet sebagai rumus: awalan apostrof, seperti ekspor audit.
func csvAman(s string) string {
	if s == "" {
		return ""
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}

func rekamEkspor(u domain.Pengguna) []string {
	kab := ""
	if u.KabupatenKode != nil {
		kab = *u.KabupatenKode
	}
	masuk := ""
	if u.TerakhirMasuk != nil {
		masuk = u.TerakhirMasuk.UTC().Format(time.RFC3339)
	}
	return []string{
		csvAman(u.ID), csvAman(u.Nama), csvAman(u.Email), csvAman(u.Peran), csvAman(u.Lapis), csvAman(u.Lembaga),
		csvAman(kab), csvAman(u.Status), strconv.FormatBool(u.MfaAktif), masuk, u.DibuatPada.UTC().Format(time.RFC3339),
	}
}

// BuatCSVPengguna menyusun berkas CSV (BOM, titik koma) dari daftar pengguna.
func BuatCSVPengguna(users []domain.Pengguna) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\xef\xbb\xbf")
	w := csv.NewWriter(&buf)
	w.Comma = ';'
	if err := w.Write(KolomEkspor); err != nil {
		return nil, err
	}
	for _, u := range users {
		if err := w.Write(rekamEkspor(u)); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// EksporPengguna membaca pengguna sesuai filter, menyusun CSV, dan mencatat EKSPOR_DATA. Berkas diserahkan
// hanya bila audit tercatat.
func (uc *UseCase) EksporPengguna(ctx context.Context, filter domain.FilterPengguna, aktorID string) (*HasilEkspor, error) {
	if len(filter.IDs) > EksporMaksID {
		return nil, galatValidasi("Pilih paling banyak %d pengguna, atau ekspor semua hasil filter.", EksporMaksID)
	}
	users, terpotong, err := uc.repo.ListUsersUntukEkspor(ctx, filter, eksporMaksBaris)
	if err != nil {
		return nil, fmt.Errorf("ekspor pengguna: %w", err)
	}
	berkas, err := BuatCSVPengguna(users)
	if err != nil {
		return nil, fmt.Errorf("ekspor pengguna: susun CSV: %w", err)
	}

	// Ekspor adalah keluarnya data pribadi: bila tidak tercatat, tidak diserahkan.
	err = uc.audit.Write(ctx, audit.Entry{
		AktorID: aktorID, Aksi: "EKSPOR_DATA", Entitas: "pengguna", EntitasID: "ekspor", Hasil: audit.HasilBerhasil,
		Metadata: map[string]any{"baris": len(users), "terpotong": terpotong, "search": filter.Search, "jumlah_id": len(filter.IDs)},
	})
	if err != nil {
		return nil, fmt.Errorf("ekspor pengguna: catat audit: %w", err)
	}
	return &HasilEkspor{CSV: berkas, Baris: len(users), Terpotong: terpotong}, nil
}

// eksporMaksBaris adalah batas baris yang dibaca; variabel agar tes bisa menurunkannya tanpa membuat 20 ribu akun.
var eksporMaksBaris = EksporMaksBaris
