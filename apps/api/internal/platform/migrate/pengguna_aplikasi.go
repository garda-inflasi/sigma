package migrate

import (
	"context"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
)

// PeranAplikasi adalah peran grup berhak minimum yang dibuat migrasi 000119.
const PeranAplikasi = "garda_aplikasi"

// SandiAplikasiMin adalah panjang minimal kata sandi pengguna DB aplikasi.
const SandiAplikasiMin = 16

var polaNamaPeran = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

// SiapkanPenggunaAplikasi membuat (atau memperbarui kata sandi) login DB yang dipakai api dan worker: anggota
// garda_aplikasi, bukan superuser, dan bukan pemilik skema, sehingga tidak bisa mengubah atau menghapus jejak audit
// maupun menjalankan DDL (03-C, 19 §7). Dijalankan cmd/migrate sesudah migrasi, dengan koneksi pemilik skema.
//
// Kata sandi dikirim sebagai verifier SCRAM-SHA-256 yang dihitung di sini, sehingga teks aslinya tidak pernah
// sampai ke server maupun log statement-nya.
func SiapkanPenggunaAplikasi(ctx context.Context, db *sql.DB, nama, sandi string) error {
	if !polaNamaPeran.MatchString(nama) {
		return fmt.Errorf("DB_APP_USER %q tidak valid (huruf kecil, angka, garis bawah)", nama)
	}
	if err := validasiSandi(sandi); err != nil {
		return err
	}

	var pemilik string
	var adaGrup bool
	if err := db.QueryRowContext(ctx,
		`SELECT current_user, EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)`, PeranAplikasi).Scan(&pemilik, &adaGrup); err != nil {
		return fmt.Errorf("periksa peran aplikasi: %w", err)
	}
	if nama == pemilik {
		return fmt.Errorf("DB_APP_USER tidak boleh sama dengan pengguna migrasi (%s): itulah pemilik skema", pemilik)
	}
	if !adaGrup {
		return fmt.Errorf("peran %s belum ada; pengguna migrasi butuh hak CREATEROLE (lihat migrasi 000119)", PeranAplikasi)
	}

	var ada, super, anggotaPemilik bool
	err := db.QueryRowContext(ctx, `
		SELECT true, rolsuper, pg_has_role(rolname, current_user, 'MEMBER')
		  FROM pg_roles WHERE rolname = $1`, nama).Scan(&ada, &super, &anggotaPemilik)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("periksa pengguna aplikasi: %w", err)
	}
	if super || anggotaPemilik {
		return fmt.Errorf("pengguna DB %s adalah superuser atau anggota pemilik skema; pilih nama lain untuk DB_APP_USER", nama)
	}

	verifier, err := verifierSCRAM(sandi)
	if err != nil {
		return err
	}
	// Nama dan verifier dikutip server (format %I/%L), bukan dirangkai di Go.
	const susun = `SELECT format($1::text, $2::text, $3::text, $4::text)`
	perintah := [][]any{
		{"ALTER ROLE %I WITH LOGIN PASSWORD %L", nama, verifier, ""},
		{"GRANT %I TO %I", PeranAplikasi, nama, ""}, // argumen lebih diabaikan format()
	}
	if !ada {
		perintah = [][]any{{"CREATE ROLE %I LOGIN PASSWORD %L IN ROLE %I", nama, verifier, PeranAplikasi}}
	}
	for _, args := range perintah {
		var stmt string
		if err := db.QueryRowContext(ctx, susun, args...).Scan(&stmt); err != nil {
			return fmt.Errorf("susun perintah peran: %w", err)
		}
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("siapkan pengguna aplikasi %s: %w", nama, err)
		}
	}
	return nil
}

// validasiSandi: cukup panjang dan ASCII tercetak, sehingga normalisasi SASLprep server sama dengan teks aslinya
// (verifier yang dihitung di klien cocok dengan yang dihitung Postgres saat login).
func validasiSandi(sandi string) error {
	if len(sandi) < SandiAplikasiMin {
		return fmt.Errorf("DB_APP_PASSWORD minimal %d karakter", SandiAplikasiMin)
	}
	for _, r := range sandi {
		if r < 0x21 || r > 0x7e {
			return errors.New("DB_APP_PASSWORD hanya boleh berisi karakter ASCII tercetak tanpa spasi")
		}
	}
	return nil
}

// iterasiSCRAM sama dengan bawaan Postgres (scram_iterations).
const iterasiSCRAM = 4096

// verifierSCRAM menyusun verifier SCRAM-SHA-256 dalam format pg_authid.rolpassword.
func verifierSCRAM(sandi string) (string, error) {
	garam := make([]byte, 16)
	if _, err := rand.Read(garam); err != nil {
		return "", fmt.Errorf("garam SCRAM: %w", err)
	}
	return susunVerifierSCRAM(sandi, garam, iterasiSCRAM)
}

func susunVerifierSCRAM(sandi string, garam []byte, iterasi int) (string, error) {
	salted, err := pbkdf2.Key(sha256.New, sandi, garam, iterasi, sha256.Size)
	if err != nil {
		return "", fmt.Errorf("PBKDF2 SCRAM: %w", err)
	}
	clientKey := hmacSHA256(salted, "Client Key")
	storedKey := sha256.Sum256(clientKey)
	serverKey := hmacSHA256(salted, "Server Key")
	b64 := base64.StdEncoding.EncodeToString
	return fmt.Sprintf("SCRAM-SHA-256$%d:%s$%s:%s", iterasi, b64(garam), b64(storedKey[:]), b64(serverKey)), nil
}

func hmacSHA256(kunci []byte, pesan string) []byte {
	m := hmac.New(sha256.New, kunci)
	m.Write([]byte(pesan))
	return m.Sum(nil)
}
