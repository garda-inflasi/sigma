package usecase

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"gorm.io/gorm"
)

// Impor pengguna dari CSV (02-D). Server yang mengurai dan memeriksa; klien hanya mengirim teksnya.
//
//   - Format: header `nama,email,peran,lembaga,kabupaten_kode` (kabupaten_kode boleh tidak ada kolomnya; urutan
//     kolom bebas), UTF-8 dengan atau tanpa BOM, pemisah koma atau titik koma (dideteksi dari header), CRLF atau
//     LF, sel berkutip boleh memuat pemisah dan baris baru. Baris yang seluruhnya kosong dilewati.
//   - Batas: ImporMaksByte dan ImporMaksBaris baris data per berkas (lebih dari itu ditolak seluruhnya).
//   - Mode uji (dry run) mengurai dan memeriksa tanpa menulis apa pun; mode komit memeriksa ulang dengan data
//     terkini lalu membuat akun MENUNGGU untuk baris yang sah.
//   - Semantik transaksi: SEMUA baris sah ditulis dalam SATU transaksi (akun + undangan), atau tidak satu pun.
//     Baris yang tidak sah tidak pernah dibuat dan dilaporkan per baris; mereka tidak menggagalkan baris lain.
//     Bila di antara pemeriksaan dan penulisan email sebuah baris keburu didaftarkan pihak lain, penulisan
//     dibatalkan seluruhnya (ErrImporBentrok) dan impor diulang.
//   - Email undangan dikirim SESUDAH komit, di latar belakang dengan konkurensi terbatas (ratusan email
//     serempak lewat SMTP akan menahan permintaan berpuluh detik). Kegagalan kirim tercatat di log dan audit
//     (KIRIM_UNDANGAN, hasil GAGAL); akunnya tetap ada dan admin memakai "Kirim ulang undangan".

const (
	// ImporMaksBaris adalah jumlah maksimum baris data per berkas.
	ImporMaksBaris = 500
	// ImporMaksByte adalah ukuran maksimum teks CSV.
	ImporMaksByte = 512 * 1024
	// imporKonkurensiEmail membatasi pengiriman email undangan serempak di latar belakang.
	imporKonkurensiEmail = 4
)

// Status satu baris impor.
const (
	ImporValid  = "valid"
	ImporGalat  = "galat"
	ImporDibuat = "dibuat"
)

var (
	// ErrImporTerlaluBesar: teks CSV melebihi ImporMaksByte.
	ErrImporTerlaluBesar = errors.New("berkas impor terlalu besar")
	// ErrImporBentrok: ada email yang didaftarkan pihak lain di antara pemeriksaan dan penulisan; tidak ada
	// akun yang dibuat.
	ErrImporBentrok = errors.New("email sudah didaftarkan pihak lain saat impor berjalan")
)

// kolomImpor adalah kolom yang dikenal; empat pertama wajib ada di header.
var kolomImpor = []string{"nama", "email", "peran", "lembaga", "kabupaten_kode"}

const jumlahKolomWajibImpor = 4

// BarisImpor adalah hasil satu baris data berkas.
type BarisImpor struct {
	// Baris adalah nomor baris di berkas (header = 1), cocok dengan nomor di spreadsheet.
	Baris         int
	Nama          string
	Email         string
	Peran         string
	Lembaga       string
	KabupatenKode string
	Status        string
	Galat         []string
	// ID akun yang dibuat (hanya Status ImporDibuat).
	ID string
}

// HasilImpor adalah ringkasan satu impor.
type HasilImpor struct {
	DryRun bool
	Total  int
	// Valid adalah jumlah baris yang lolos pemeriksaan (dibuat bila bukan dry run).
	Valid  int
	Galat  int
	Dibuat int
	Baris  []BarisImpor
	// EmailAktif false bila SMTP tidak dikonfigurasi: undangan dibuat tetapi tidak ada email yang dikirim.
	EmailAktif bool
}

type barisMentah struct {
	nomor int
	sel   map[string]string
	// kelebihan true bila ada sel berisi di luar kolom header.
	kelebihan bool
}

// deteksiPemisah memilih titik koma bila header lebih banyak memuatnya daripada koma (Excel berlokal Indonesia).
func deteksiPemisah(teks string) rune {
	header, _, _ := strings.Cut(teks, "\n")
	if strings.Count(header, ";") > strings.Count(header, ",") {
		return ';'
	}
	return ','
}

// uraiCSVImpor mengurai teks CSV menjadi baris mentah. Galat berkas (encoding, header, kutip, batas) menolak
// seluruh berkas; galat isi per baris diperiksa terpisah.
func uraiCSVImpor(teks string) ([]barisMentah, error) {
	teks = strings.TrimPrefix(teks, "\ufeff")
	if len(teks) > ImporMaksByte {
		return nil, ErrImporTerlaluBesar
	}
	if !utf8.ValidString(teks) || strings.ContainsRune(teks, utf8.RuneError) || strings.ContainsRune(teks, 0) {
		return nil, galatValidasi("Berkas harus berencoding UTF-8. Simpan dari spreadsheet sebagai \"CSV UTF-8\" lalu coba lagi.")
	}
	if strings.TrimSpace(teks) == "" {
		return nil, galatValidasi("Berkas kosong.")
	}

	r := csv.NewReader(strings.NewReader(teks))
	r.Comma = deteksiPemisah(teks)
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true

	header, err := r.Read()
	if err != nil {
		return nil, galatBacaCSV(err)
	}
	indeks := map[string]int{}
	var tidakDikenal []string
	for i, h := range header {
		nama := strings.ToLower(strings.TrimSpace(h))
		switch {
		case nama == "":
			// Sel header kosong (sisa kolom Excel) diabaikan.
		case !slices.Contains(kolomImpor, nama):
			tidakDikenal = append(tidakDikenal, strings.TrimSpace(h))
		default:
			if _, ganda := indeks[nama]; ganda {
				return nil, galatValidasi("Kolom %q muncul lebih dari sekali di header.", nama)
			}
			indeks[nama] = i
		}
	}
	if len(tidakDikenal) > 0 {
		return nil, galatValidasi("Kolom tidak dikenal di header: %s. Kolom yang diterima: %s.", strings.Join(tidakDikenal, ", "), strings.Join(kolomImpor, ", "))
	}
	var hilang []string
	for _, wajib := range kolomImpor[:jumlahKolomWajibImpor] {
		if _, ada := indeks[wajib]; !ada {
			hilang = append(hilang, wajib)
		}
	}
	if len(hilang) > 0 {
		return nil, galatValidasi("Kolom wajib tidak ada di header: %s. Header yang benar: %s.", strings.Join(hilang, ", "), strings.Join(kolomImpor, ","))
	}

	var baris []barisMentah
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, galatBacaCSV(err)
		}
		kosong := true
		for _, sel := range rec {
			if strings.TrimSpace(sel) != "" {
				kosong = false
				break
			}
		}
		if kosong {
			continue
		}
		if len(baris) >= ImporMaksBaris {
			return nil, galatValidasi("Berkas memuat lebih dari %d baris data. Bagi menjadi beberapa berkas.", ImporMaksBaris)
		}
		nomor, _ := r.FieldPos(0)
		b := barisMentah{nomor: nomor, sel: map[string]string{}}
		for nama, i := range indeks {
			if i < len(rec) {
				b.sel[nama] = strings.TrimSpace(rec[i])
			}
		}
		for i := len(header); i < len(rec); i++ {
			if strings.TrimSpace(rec[i]) != "" {
				b.kelebihan = true
			}
		}
		baris = append(baris, b)
	}
	if len(baris) == 0 {
		return nil, galatValidasi("Berkas hanya berisi header; tidak ada baris data.")
	}
	return baris, nil
}

// galatBacaCSV menerjemahkan galat encoding/csv menjadi pesan yang bisa ditindaklanjuti.
func galatBacaCSV(err error) error {
	var pe *csv.ParseError
	if errors.As(err, &pe) {
		switch {
		case errors.Is(pe.Err, csv.ErrQuote), errors.Is(pe.Err, csv.ErrBareQuote):
			return galatValidasi("Berkas CSV tidak dapat dibaca pada baris %d: tanda kutip (\") tidak berpasangan.", pe.Line)
		default:
			return galatValidasi("Berkas CSV tidak dapat dibaca pada baris %d.", pe.Line)
		}
	}
	return galatValidasi("Berkas CSV tidak dapat dibaca.")
}

// periksaBarisImpor merapikan dan memeriksa semua baris: isian, duplikat dalam berkas, email yang sudah
// terdaftar, dan kode kabupaten. Urutan hasil sama dengan berkas.
func (uc *UseCase) periksaBarisImpor(ctx context.Context, mentah []barisMentah) ([]BarisImpor, error) {
	hasil := make([]BarisImpor, len(mentah))
	pertama := map[string]int{} // email -> nomor baris pertama
	var emailCek, kabupatenCek []string

	for i, m := range mentah {
		in, galat := bersihkanAkunUndangan(UndangInput{
			Nama: m.sel["nama"], Email: m.sel["email"], Peran: m.sel["peran"],
			Lembaga: m.sel["lembaga"], KabupatenKode: m.sel["kabupaten_kode"],
		})
		if m.kelebihan {
			galat = append(galat, "Jumlah kolom lebih banyak daripada header.")
		}
		if in.Email != "" && emailValid(in.Email) {
			if awal, ada := pertama[in.Email]; ada {
				galat = append(galat, fmt.Sprintf("Email sama dengan baris %d.", awal))
			} else {
				pertama[in.Email] = m.nomor
				emailCek = append(emailCek, in.Email)
			}
		}
		if in.KabupatenKode != "" && kodeKabupatenValid(in.KabupatenKode) && !slices.Contains(kabupatenCek, in.KabupatenKode) {
			kabupatenCek = append(kabupatenCek, in.KabupatenKode)
		}
		hasil[i] = BarisImpor{
			Baris: m.nomor, Nama: in.Nama, Email: in.Email, Peran: in.Peran, Lembaga: in.Lembaga,
			KabupatenKode: in.KabupatenKode, Galat: galat,
		}
	}

	terdaftar, err := uc.repo.EmailTerdaftar(ctx, emailCek)
	if err != nil {
		return nil, fmt.Errorf("impor pengguna: %w", err)
	}
	kabupatenAda, err := uc.repo.KabupatenAda(ctx, kabupatenCek)
	if err != nil {
		return nil, fmt.Errorf("impor pengguna: %w", err)
	}
	for i := range hasil {
		b := &hasil[i]
		// Hanya baris yang memegang email pertama dilaporkan "sudah terdaftar"; salinan dalam berkas sudah
		// ditandai sebagai duplikat.
		if terdaftar[b.Email] && pertama[b.Email] == b.Baris {
			b.Galat = append(b.Galat, "Email sudah terdaftar.")
		}
		if b.KabupatenKode != "" && kodeKabupatenValid(b.KabupatenKode) && !kabupatenAda[b.KabupatenKode] {
			b.Galat = append(b.Galat, fmt.Sprintf("Kode kabupaten tidak dikenal: %s.", b.KabupatenKode))
		}
		b.Status = ImporValid
		if len(b.Galat) > 0 {
			b.Status = ImporGalat
		}
	}
	return hasil, nil
}

// ImporPengguna mengurai dan memeriksa CSV; bila bukan dryRun, membuat akun MENUNGGU untuk baris yang sah dalam
// satu transaksi, mencatat audit (satu entri per akun dan satu ringkasan), dan mengantre email undangan.
func (uc *UseCase) ImporPengguna(ctx context.Context, teksCSV string, dryRun bool, aktorID string) (*HasilImpor, error) {
	mentah, err := uraiCSVImpor(teksCSV)
	if err != nil {
		return nil, err
	}
	baris, err := uc.periksaBarisImpor(ctx, mentah)
	if err != nil {
		return nil, err
	}

	hasil := &HasilImpor{DryRun: dryRun, Total: len(baris), Baris: baris, EmailAktif: uc.emailSender.Configured()}
	for _, b := range baris {
		if b.Status == ImporValid {
			hasil.Valid++
		} else {
			hasil.Galat++
		}
	}
	if dryRun || hasil.Valid == 0 {
		return hasil, nil
	}

	sekarang := jam()
	var barus []domain.UndanganBaru
	var tokens []string
	var indeksBaris []int
	for i, b := range baris {
		if b.Status != ImporValid {
			continue
		}
		baru, raw, err := uc.penggunaUndangan(UndangInput{
			Nama: b.Nama, Email: b.Email, Peran: b.Peran, Lembaga: b.Lembaga, KabupatenKode: b.KabupatenKode,
		}, aktorID, sekarang)
		if err != nil {
			return nil, err
		}
		barus = append(barus, baru)
		tokens = append(tokens, raw)
		indeksBaris = append(indeksBaris, i)
	}
	if err := uc.repo.BuatUndangan(ctx, barus); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, ErrImporBentrok
		}
		return nil, fmt.Errorf("impor pengguna: %w", err)
	}

	for n, i := range indeksBaris {
		baris[i].Status = ImporDibuat
		baris[i].ID = barus[n].User.ID
		uc.writeAudit(ctx, audit.Entry{
			AktorID: aktorID, Aksi: "UNDANG_PENGGUNA", Entitas: "pengguna", EntitasID: barus[n].User.ID,
			Hasil: audit.HasilBerhasil, Sesudah: userSnapshot(barus[n].User),
			Metadata: map[string]any{"sumber": "impor", "baris": baris[i].Baris, "berlaku_sampai": barus[n].Undangan.ExpiresAt.UTC().Format(time.RFC3339)},
		})
	}
	hasil.Dibuat = len(barus)
	uc.writeAudit(ctx, audit.Entry{
		AktorID: aktorID, Aksi: "IMPOR_PENGGUNA", Entitas: "pengguna", EntitasID: "impor", Hasil: audit.HasilBerhasil,
		Metadata: map[string]any{"total": hasil.Total, "dibuat": hasil.Dibuat, "ditolak": hasil.Galat},
	})
	uc.log.Info("impor pengguna selesai", map[string]interface{}{"total": hasil.Total, "dibuat": hasil.Dibuat, "ditolak": hasil.Galat})

	uc.kirimUndanganLatar(ctx, aktorID, barus, tokens)
	return hasil, nil
}

// kirimUndanganLatar mengirim email undangan hasil impor di latar belakang, paling banyak
// imporKonkurensiEmail serempak. Kegagalan dicatat di log dan audit.
func (uc *UseCase) kirimUndanganLatar(ctx context.Context, aktorID string, barus []domain.UndanganBaru, tokens []string) {
	if !uc.emailSender.Configured() {
		uc.log.Warn("impor pengguna: SMTP belum dikonfigurasi, email undangan tidak dikirim", map[string]interface{}{"jumlah": len(barus)})
		return
	}
	// Konteks permintaan dibatalkan begitu jawaban terkirim; nilainya (asal permintaan untuk audit) dipertahankan.
	konteks := context.WithoutCancel(ctx)
	uc.kirimLatar.Add(1)
	go func() {
		defer uc.kirimLatar.Done()
		sem := make(chan struct{}, imporKonkurensiEmail)
		var wg sync.WaitGroup
		for n := range barus {
			user, raw := barus[n].User, tokens[n]
			sem <- struct{}{}
			wg.Add(1)
			go func() {
				defer func() { <-sem; wg.Done() }()
				if !uc.kirimUndangan(konteks, user, raw) {
					uc.writeAudit(konteks, audit.Entry{
						AktorID: aktorID, Aksi: "KIRIM_UNDANGAN", Entitas: "pengguna", EntitasID: user.ID,
						Hasil: audit.HasilGagal, Metadata: map[string]any{"sumber": "impor"},
					})
				}
			}()
		}
		wg.Wait()
	}()
}
